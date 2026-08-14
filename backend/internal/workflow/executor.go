package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

// StepRequest, bir adımın çalıştırılması için gereken her şey.
type StepRequest struct {
	ProjectID uuid.UUID
	Node      Node
	// Prompt, şablonu çözümlenmiş talimat.
	Prompt string
}

// StepOutcome, adımın sonucu.
//
// Hata döndürülse bile RunID dolu olabilir: çalıştırma kaydı oluşmuş ama iş
// başarısız olmuş olabilir ve o kayıt adıma bağlanmalıdır.
type StepOutcome struct {
	RunID  uuid.UUID
	Output string
	Diff   string
	Branch string
	// URL, adımın ürettiği adres (açılan PR, yazılan yorum). Sonraki adımlar
	// `{{ steps.<adım>.url }}` ile okur.
	URL string
}

// StepRunner, bir adımı çalıştırıp BİTMESİNİ bekleyen şey.
//
// Arayüz olarak duruyor ki motor gerçek container'lar olmadan test edilebilsin;
// motorun sıra, paralellik ve hata mantığı en çok hata barındıran kısım.
type StepRunner interface {
	RunStep(ctx context.Context, req StepRequest) (StepOutcome, error)
}

// Publisher, akış ilerlemesini canlı yayına verir.
type Publisher interface {
	Publish(topic uuid.UUID, level, message string)
}

// Executor, bir akış çalışmasını yürütür.
type Executor struct {
	store    *Store
	handlers Handlers
	bus      Publisher

	mu      sync.Mutex
	cancels map[uuid.UUID]context.CancelFunc
	wg      sync.WaitGroup

	/*
		onFinished, bir akış çalışması SON DURUMUNA yazıldıktan sonra çağrılır.
		nil olabilir.

		Toplu çalıştırma kuyruğu (spec 023) için var ve sırası hayati: kuyruk
		"slot boşaldı" sinyalini zaten alıyor ama o sinyal buradan ÖNCE geliyor
		(slotu bırakan `defer finish()`, `FinishRun`'dan önce koşuyor). O anda
		çalışma hâlâ `running` göründüğü için kuyruk bitişi göremiyor ve öğeyi
		ancak dakikalık emniyet turunda kapatıyordu — yani sigorta, mekanizmanın
		yerine geçmişti.

		Bağımlılık yönü korunuyor: `workflow` paketi kuyruğu tanımıyor, bağ
		`main.go`'da kuruluyor. BLOKLAMAMALI.
	*/
	onFinished func(runID uuid.UUID)

	shutdownCtx context.Context
	shutdown    context.CancelFunc
}

// SetOnRunFinished, çalışma bittiğinde çağrılacak kancayı bağlar.
//
// `Start` çağrılmadan ÖNCE, kurulum sırasında bir kez ayarlanır.
func (e *Executor) SetOnRunFinished(fn func(runID uuid.UUID)) { e.onFinished = fn }

// finished, son durum YAZILDIKTAN SONRA kancayı çağırır.
//
// Üç kapanış yolunun (başarı, hata, iptal) hepsinden geçer; biri unutulursa
// kuyruk o çalışmanın bittiğini yalnızca emniyet turunda öğrenir.
func (e *Executor) finished(runID uuid.UUID) {
	if e.onFinished != nil {
		e.onFinished(runID)
	}
}

// NewExecutor yeni motor üretir.
//
// Düğüm türü başına çalıştırıcı dışarıdan gelir: motor "bir adım nasıl
// çalıştırılır" sorusunu bilmez, yalnızca sırayı, paralelliği ve hata
// davranışını bilir.
func NewExecutor(store *Store, handlers Handlers, bus Publisher) *Executor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Executor{
		store: store, handlers: handlers, bus: bus,
		cancels:     map[uuid.UUID]context.CancelFunc{},
		shutdownCtx: ctx,
		shutdown:    cancel,
	}
}

// Start, akış çalışmasını arka planda başlatır ve hemen döner.
//
// HTTP isteği akışın bitmesini bekleyemez: bir akış dakikalarca sürebilir.
// Kullanıcı ilerleme ekranına geçer, canlı akışı oradan izler.
func (e *Executor) Start(run Run, graph Graph) {
	runCtx, cancel := context.WithCancel(e.shutdownCtx)

	e.mu.Lock()
	e.cancels[run.ID] = cancel
	e.mu.Unlock()

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer cancel()
		defer func() {
			e.mu.Lock()
			delete(e.cancels, run.ID)
			e.mu.Unlock()
		}()

		e.execute(runCtx, run, graph)
	}()
}

// execute, akışı seviye seviye yürütür.
func (e *Executor) execute(ctx context.Context, run Run, graph Graph) {
	// Yazma işlemleri iptal edilmiş context ile çalışmaz; ayrı context.
	writeCtx := context.WithoutCancel(ctx)

	if err := e.store.MarkRunStarted(writeCtx, run.ID); err != nil {
		slog.ErrorContext(ctx, "akış başlatılamadı", "workflow_run_id", run.ID, "error", err)
	}
	e.emit(run.ID, "info", fmt.Sprintf("akış başladı — %d adım", len(run.Steps)))

	levels, err := graph.Levels()
	if err != nil {
		e.fail(writeCtx, run.ID, err)
		return
	}

	// Adım kayıtlarına düğüm kimliğinden erişim.
	steps := make(map[string]Step, len(run.Steps))
	for _, st := range run.Steps {
		steps[st.NodeID] = st
	}

	// Bağlam adımlar ilerledikçe birikir; sonraki seviyeler onu görür.
	tctx := Context{
		Input:   run.Input,
		Trigger: run.TriggerPayload,
		Steps:   map[string]StepResult{},
	}
	var mu sync.Mutex

	for _, level := range levels {
		nodes := executableOf(level)
		if len(nodes) == 0 {
			continue
		}

		// Aynı seviyedeki adımlar AYNI ANDA çalışır (spec 007 kabul kriteri).
		// errgroup yerine elle: ilk hatayı saklayıp diğerlerinin bitmesini
		// beklemek istiyoruz — yarıda kesilen bir adım container bırakabilir.
		var (
			wg      sync.WaitGroup
			firstMu sync.Mutex
			first   error
		)

		for _, node := range nodes {
			step, ok := steps[node.ID]
			if !ok {
				continue
			}

			wg.Add(1)
			go func(node Node, step Step) {
				defer wg.Done()

				mu.Lock()
				snapshot := Context{Input: tctx.Input, Trigger: tctx.Trigger,
					Steps: copySteps(tctx.Steps)}
				mu.Unlock()

				result, err := e.runStep(ctx, writeCtx, run, node, step, snapshot)
				if err != nil {
					firstMu.Lock()
					if first == nil {
						first = err
					}
					firstMu.Unlock()
					return
				}

				mu.Lock()
				tctx.Steps[node.ID] = result
				mu.Unlock()
			}(node, step)
		}
		wg.Wait()

		// İPTAL ÖNCE bakılır: kullanıcı akışı durdurduğunda adımlar da hata
		// döndürür, ama bu bir başarısızlık değildir. Sıra ters olsaydı
		// kullanıcının durdurduğu her akış "başarısız" görünürdü.
		if ctx.Err() != nil {
			e.stopped(writeCtx, run.ID)
			return
		}
		if first != nil {
			// Spec 007 K2: bir adım başarısız olunca akış tamamen durur.
			e.fail(writeCtx, run.ID, first)
			return
		}
	}

	if _, err := e.store.SkipPending(writeCtx, run.ID, StepSkipped); err != nil {
		slog.ErrorContext(ctx, "kalan adımlar işaretlenemedi", "error", err)
	}
	if err := e.store.FinishRun(writeCtx, run.ID, RunSucceeded, nil); err != nil {
		slog.ErrorContext(ctx, "akış sonuçlandırılamadı", "error", err)
	}
	e.emit(run.ID, "info", "akış tamamlandı")
	e.finished(run.ID)
}

// runStep, tek bir adımı çalıştırır ve sonucunu kaydeder.
func (e *Executor) runStep(ctx, writeCtx context.Context, run Run, node Node,
	step Step, tctx Context,
) (StepResult, error) {
	handler, ok := e.handlers[node.Kind]
	if !ok {
		err := fmt.Errorf("%q türü bu kurulumda çalıştırılamıyor", node.Kind)
		_ = e.store.FinishStep(writeCtx, step.ID, StepFailed, StepOutcome{}, err)
		e.emit(run.ID, "error", fmt.Sprintf("%s: %s", label(node), err))
		return StepResult{}, err
	}

	// Durum ve başlangıç zamanı İŞ BAŞLAMADAN yazılır: kullanıcı adımın
	// çalıştığını anında görsün ve süre gerçek süreyi göstersin.
	if err := e.store.MarkStepRunning(writeCtx, step.ID); err != nil {
		slog.ErrorContext(ctx, "adım başlatılamadı", "error", err)
	}
	e.emit(run.ID, "info", fmt.Sprintf("%s çalışıyor", label(node)))

	outcome, runErr := handler.Execute(ctx, ExecInput{
		Run: run, Node: node, Context: tctx,
	})

	// Çalıştırma kaydı oluştuysa hata halinde de adıma bağlanır: kullanıcı
	// başarısız adımın çıktısını ve maliyetini görebilmeli.
	if outcome.RunID != uuid.Nil {
		if err := e.store.LinkStepRun(writeCtx, step.ID, outcome.RunID); err != nil {
			slog.ErrorContext(ctx, "adım çalıştırmaya bağlanamadı", "error", err)
		}
	}

	if runErr != nil {
		status := StepFailed
		if errors.Is(runErr, context.Canceled) || ctx.Err() != nil {
			status = StepCancelled
		}
		_ = e.store.FinishStep(writeCtx, step.ID, status, outcome, runErr)
		e.emit(run.ID, "error", fmt.Sprintf("%s başarısız — %s", label(node), runErr))
		return StepResult{}, fmt.Errorf("%s: %w", label(node), runErr)
	}

	// Agent adımının çıktısı çalıştırma kaydında; burada yalnızca agent
	// OLMAYAN adımların sonucu saklanır.
	result := outcome
	if outcome.RunID != uuid.Nil {
		result = StepOutcome{}
	}
	if err := e.store.FinishStep(writeCtx, step.ID, StepSucceeded, result, nil); err != nil {
		slog.ErrorContext(ctx, "adım sonuçlandırılamadı", "error", err)
	}
	e.emit(run.ID, "info", fmt.Sprintf("%s bitti", label(node)))

	return StepResult{
		Output: outcome.Output,
		Diff:   outcome.Diff,
		Branch: outcome.Branch,
		URL:    outcome.URL,
	}, nil
}

// fail, akışı hatayla kapatır ve kalan adımları atlanmış işaretler.
func (e *Executor) fail(ctx context.Context, runID uuid.UUID, cause error) {
	n, err := e.store.SkipPending(ctx, runID, StepSkipped)
	if err != nil {
		slog.ErrorContext(ctx, "kalan adımlar işaretlenemedi", "error", err)
	}
	if err := e.store.FinishRun(ctx, runID, RunFailed, cause); err != nil {
		slog.ErrorContext(ctx, "akış sonuçlandırılamadı", "error", err)
	}
	if n > 0 {
		e.emit(runID, "warn", fmt.Sprintf("%d adım atlandı", n))
	}
	e.emit(runID, "error", "akış durdu: "+cause.Error())
	e.finished(runID)
}

// stopped, iptal edilmiş bir akışı kapatır.
//
// KULLANICI mı durdurdu, SUNUCU mu kapandı — ikisi ayrı durumdur. İkisine de
// "durduruldu" demek, sunucu yeniden başladığında kullanıcıya "sen durdurdun"
// demek olurdu ve geçmiş yanlış okunurdu.
func (e *Executor) stopped(ctx context.Context, runID uuid.UUID) {
	status, reason, note := RunCancelled, "akış durduruldu", "akış durduruldu"
	if e.shutdownCtx.Err() != nil {
		status = RunInterrupted
		reason = "sunucu kapandığı için akış kesildi"
		note = reason
	}

	if _, err := e.store.SkipPending(ctx, runID, StepCancelled); err != nil {
		slog.ErrorContext(ctx, "kalan adımlar işaretlenemedi", "error", err)
	}
	if err := e.store.FinishRun(ctx, runID, status, errors.New(reason)); err != nil {
		slog.ErrorContext(ctx, "akış sonuçlandırılamadı", "error", err)
	}
	e.emit(runID, "warn", note)
	e.finished(runID)
}

// Cancel, çalışan bir akışı durdurur.
//
// Süren adımın context'i de düşer; runner container'ı temizler.
func (e *Executor) Cancel(runID uuid.UUID) error {
	e.mu.Lock()
	cancel, ok := e.cancels[runID]
	e.mu.Unlock()

	if !ok {
		return ErrNotRunning
	}
	cancel()
	return nil
}

// Shutdown, çalışan akışları iptal eder ve bitmelerini bekler.
func (e *Executor) Shutdown() {
	e.shutdown()
	e.wg.Wait()
}

func (e *Executor) emit(runID uuid.UUID, level, message string) {
	if e.bus != nil {
		e.bus.Publish(runID, level, message)
	}
}

// executableOf, seviyedeki çalıştırılacak düğümleri süzer.
//
// Tür listesi KAYIT DEFTERİNDEN geliyor: yeni bir tür eklemek burayı
// değiştirmeyi gerektirmiyor.
func executableOf(level []Node) []Node {
	out := make([]Node, 0, len(level))
	for _, n := range level {
		if spec, ok := Spec(n.Kind); ok && spec.Executable {
			out = append(out, n)
		}
	}
	return out
}

// label, olay mesajlarında adımın okunur adı.
func label(n Node) string {
	if n.Name != "" {
		return n.Name
	}
	return n.ID
}

func copySteps(in map[string]StepResult) map[string]StepResult {
	out := make(map[string]StepResult, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
