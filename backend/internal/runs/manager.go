package runs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/events"
	"github.com/agent-coder/backend/internal/runner"
)

// ErrTooManyRuns, eşzamanlı çalıştırma sınırı dolu.
var ErrTooManyRuns = errors.New("eşzamanlı çalıştırma sınırı doldu")

// Limits, ayarlardan okunan çalışma parametreleri.
//
// Fonksiyon olarak taşınırlar, değer olarak değil: ayar değiştiğinde
// yeniden başlatma gerekmesin diye her kullanımda yeniden okunurlar.
type Limits struct {
	MaxConcurrent func() int
	Timeout       func() time.Duration
	CPUCores      func() int
	MemoryGB      func() int
	CloneDepth    func() int
}

// Manager, çalıştırmaları yürütür ve eşzamanlılığı sınırlar.
type Manager struct {
	store  *Store
	runner runner.Runner
	bus    *events.Bus
	limits Limits

	mu      sync.Mutex
	cond    *sync.Cond
	active  int
	cancels map[uuid.UUID]context.CancelFunc

	// wg, arka planda süren çalıştırmaları izler; Shutdown hepsini bekler.
	wg sync.WaitGroup

	shutdownCtx context.Context
	shutdown    context.CancelFunc
}

// NewManager yeni yönetici üretir.
func NewManager(store *Store, r runner.Runner, bus *events.Bus, limits Limits) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		store: store, runner: r, bus: bus, limits: limits,
		cancels:     make(map[uuid.UUID]context.CancelFunc),
		shutdownCtx: ctx,
		shutdown:    cancel,
	}
	m.cond = sync.NewCond(&m.mu)
	return m
}

// StartInput, başlatılacak çalıştırmanın tüm girdisi.
type StartInput struct {
	Create   CreateInput
	Repo     runner.RepoSpec
	Agent    runner.AgentSpec
	Provider runner.ProviderSpec
}

// Start, çalıştırmayı kaydeder ve arka planda başlatır.
//
// İstek yanıtını BEKLETMEZ: kullanıcı hemen ilerleme ekranına geçer.
// Sınır doluysa çalıştırma sıraya alınmaz, reddedilir — kullanıcı kaç iş
// çalıştığını görüp karar verir (spec 003 H3).
func (m *Manager) Start(ctx context.Context, in StartInput) (Run, error) {
	// Çalıştırma context'i isteğe DEĞİL, yöneticiye bağlıdır: HTTP yanıtı
	// döndüğünde iş iptal olmamalı.
	run, runCtx, finish, err := m.begin(ctx, in, m.shutdownCtx)
	if err != nil {
		return Run{}, err
	}

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer finish()
		m.execute(runCtx, run, in)
	}()

	return run, nil
}

// Execute, çalıştırmayı yürütür ve BİTMESİNİ BEKLER.
//
// Workflow motoru için var: bir adımın çıktısı bir sonraki adımın talimatına
// girdiğinden, motorun beklemesi gerekiyor. Gövde `Start` ile ortaktır —
// eşzamanlılık sayacı, zaman aşımı ve iptal aynı yerden geçer, böylece akış
// adımları da genel sınıra tabi olur (spec 007 kabul kriteri).
//
// `Start`'tan tek farkı context'in kaynağı: burada çağıranın context'i işi
// kontrol eder, çünkü akış iptal edilince süren adım da düşmelidir.
func (m *Manager) Execute(ctx context.Context, in StartInput) (Run, error) {
	run, runCtx, finish, err := m.begin(ctx, in, ctx)
	if err != nil {
		return Run{}, err
	}

	m.wg.Add(1)
	defer m.wg.Done()
	defer finish()

	m.execute(runCtx, run, in)

	// Sonuç veritabanından okunur: execute yalnızca yazar, döndürmez.
	// İptal edilmiş context'le okuma yapılamaz, bu yüzden WithoutCancel.
	final, err := m.store.Get(context.WithoutCancel(ctx), run.ID)
	if err != nil {
		return Run{}, fmt.Errorf("çalıştırma sonucu okunamadı: %w", err)
	}
	return final, nil
}

// begin, slot ayırır, kaydı oluşturur ve çalıştırma context'ini kurar.
//
// Dönen `finish` her yolda çağrılmalıdır: slotu bırakır ve iptal kaydını siler.
func (m *Manager) begin(ctx context.Context, in StartInput, parent context.Context) (
	Run, context.Context, func(), error,
) {
	// Slot önce ayrılır: kayıt oluşturup sonra reddetmek, veritabanında
	// hiç başlamamış "pending" kayıtlar bırakırdı.
	if !m.tryAcquire() {
		return Run{}, nil, nil, fmt.Errorf("%w: şu an %d iş çalışıyor", ErrTooManyRuns, m.Active())
	}

	run, err := m.store.Create(ctx, in.Create)
	if err != nil {
		m.release()
		return Run{}, nil, nil, fmt.Errorf("çalıştırma kaydedilemedi: %w", err)
	}

	runCtx, cancel := context.WithCancel(parent)

	// Kapatma sinyali parent'tan gelmiyor olabilir (Execute'ta parent çağıranın
	// context'i). Container'ların temizlenmesi için kapatma HER durumda işi
	// düşürmeli — yoksa Shutdown boşuna bekler ve sahipsiz container kalır.
	stopWatch := context.AfterFunc(m.shutdownCtx, cancel)

	m.mu.Lock()
	m.cancels[run.ID] = cancel
	m.mu.Unlock()

	finish := func() {
		stopWatch()
		cancel()
		m.mu.Lock()
		delete(m.cancels, run.ID)
		m.mu.Unlock()
		m.release()
	}
	return run, runCtx, finish, nil
}

// execute, tek bir çalıştırmayı yürütür ve sonucunu kaydeder.
func (m *Manager) execute(ctx context.Context, run Run, in StartInput) {
	// Kayıt yazma işlemleri iptal edilmiş context ile çalışmaz; ayrı context.
	writeCtx := context.WithoutCancel(ctx)

	if err := m.store.MarkRunning(writeCtx, run.ID); err != nil {
		slog.ErrorContext(ctx, "çalıştırma başlatılamadı", "run_id", run.ID, "error", err)
	}
	m.emit(writeCtx, run.ID, "info", "çalıştırma sıraya alındı")

	in.Repo.CloneDepth = m.limits.CloneDepth()

	req := runner.Request{
		RunID:    run.ID,
		Repo:     in.Repo,
		Agent:    in.Agent,
		Provider: in.Provider,
		Model:    run.ModelID,
		Task:     run.Task,
		Timeout:  m.limits.Timeout(),
		Limits: runner.Limits{
			CPUCores: m.limits.CPUCores(),
			MemoryGB: m.limits.MemoryGB(),
		},
	}

	result, err := m.runner.Run(ctx, req, func(e runner.Event) {
		m.emit(writeCtx, run.ID, string(e.Level), e.Message)
	})

	status := classify(err)

	// Maliyet BAŞARISIZLIKTA DA kaydedilir: harcama gerçekleşmiştir.
	if storeErr := m.store.Finish(writeCtx, run.ID, status, result, err); storeErr != nil {
		slog.ErrorContext(ctx, "çalıştırma sonucu kaydedilemedi",
			"run_id", run.ID, "error", storeErr)
	}

	m.emitTerminal(writeCtx, run.ID, status, err)

	slog.InfoContext(ctx, "çalıştırma bitti",
		"run_id", run.ID, "status", status,
		"maliyet", costOf(result), "hata", err)
}

// classify, runner hatasını çalıştırma durumuna çevirir.
func classify(err error) Status {
	switch {
	case err == nil:
		return StatusSucceeded
	case errors.Is(err, runner.ErrCancelled):
		return StatusCancelled
	case errors.Is(err, runner.ErrTimeout):
		return StatusTimeout
	default:
		return StatusFailed
	}
}

func costOf(r *runner.Result) float64 {
	if r == nil {
		return 0
	}
	return r.CostUSD
}

// emit, olayı hem veritabanına yazar hem canlı dinleyicilere yayınlar.
func (m *Manager) emit(ctx context.Context, runID uuid.UUID, level, message string) {
	seq, ts, err := m.store.AppendEvent(ctx, runID, level, message)
	if err != nil {
		slog.WarnContext(ctx, "olay kaydedilemedi", "run_id", runID, "error", err)
		return
	}
	m.bus.Publish(runID, events.Event{
		Seq: seq, Level: level, Message: message,
		TS: ts.UTC().Format(time.RFC3339),
	})
}

// emitTerminal, bitiş olayını yayınlar; dinleyiciler bağlantıyı buna göre kapatır.
func (m *Manager) emitTerminal(ctx context.Context, runID uuid.UUID, status Status, cause error) {
	message := "çalıştırma tamamlandı"
	level := "info"

	if cause != nil {
		message = userMessage(status, cause)
		level = "error"
	}

	seq, ts, err := m.store.AppendEvent(ctx, runID, level, message)
	if err != nil {
		slog.WarnContext(ctx, "bitiş olayı kaydedilemedi", "run_id", runID, "error", err)
	}

	m.bus.Publish(runID, events.Event{
		Seq: seq, Level: level, Message: message,
		TS:       ts.UTC().Format(time.RFC3339),
		Terminal: true, Status: string(status),
	})
}

// userMessage, teknik hatayı kullanıcının anlayacağı cümleye çevirir.
func userMessage(status Status, cause error) string {
	switch {
	case status == StatusCancelled:
		return "çalıştırma iptal edildi"
	case status == StatusTimeout:
		return "süre sınırı aşıldı, çalışma durduruldu"
	case errors.Is(cause, runner.ErrRepoAccess):
		return "depoya erişilemedi: " + cause.Error()
	case errors.Is(cause, runner.ErrProviderAuth):
		return "sağlayıcı anahtarı geçersiz — ayarlardan güncelleyin"
	// Sürücü hatası model hatasının ÖNÜNDE: ikisi de mesaj gönderimi
	// sırasında çıkıyor ama sebepleri ve çözümleri bambaşka. Sürücü
	// yüklenemediyse modelde bir sorun yok, ortamda var.
	case errors.Is(cause, runner.ErrProviderDriver):
		return "sağlayıcı sürücüsü yüklenemedi: " + cause.Error() +
			" — kurumsal ağdaysanız RUNNER_EXTRA_CA_CERT ile kök sertifikayı tanıtın"
	case errors.Is(cause, runner.ErrModel):
		return "model çağrısı başarısız: " + cause.Error()
	case errors.Is(cause, runner.ErrSandbox):
		return "çalışma ortamı hazırlanamadı: " + cause.Error()
	default:
		return "çalıştırma başarısız: " + cause.Error()
	}
}

// Cancel, çalışan bir işi durdurur.
func (m *Manager) Cancel(id uuid.UUID) error {
	m.mu.Lock()
	cancel, ok := m.cancels[id]
	m.mu.Unlock()

	if !ok {
		return ErrNotRunning
	}
	cancel()
	return nil
}

// Running, işin BU SÜREÇTE hâlâ canlı olup olmadığı.
//
// Veritabanındaki durum tek başına yetmez: iptal asenkron. `Cancel` hemen
// döner ama container'ın silinmesi ve `status='cancelled'` yazımı arka planda
// sürer. Yalnızca DB'ye bakılsaydı iptalden hemen sonraki bir silme, hâlâ
// koşan goroutine ile yarışırdı — o goroutine silinmiş kayda `UPDATE` atar
// (sessizce 0 satır), olay yazarken de FK ihlaline düşerdi.
func (m *Manager) Running(id uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.cancels[id]
	return ok
}

// Delete, bitmiş bir çalıştırma kaydını siler.
//
// Silme kapısı Manager'da: "çalışıyor mu" sorusunun iki kaynağı var (DB durumu
// ve bellekteki canlı iş listesi) ve ikisine birden yalnızca burası bakabilir.
func (m *Manager) Delete(ctx context.Context, id uuid.UUID) error {
	if m.Running(id) {
		return ErrActive
	}
	if err := m.store.Delete(ctx, id); err != nil {
		return err
	}
	// Açık SSE bağlantıları kapansın: kayıt gittiği için artık hiçbir olay
	// gelmeyecek ve abone 25 sn'lik ping'lerle sonsuza kadar beklerdi.
	//
	// Veritabanına YAZILMAZ — yazılacak satır kalmadı; bu yalnızca o an bağlı
	// olanlara "bitti, kapat" demek için.
	m.bus.Publish(id, events.Event{
		Level: "info", Message: "çalıştırma kaydı silindi",
		TS:       time.Now().UTC().Format(time.RFC3339),
		Terminal: true,
	})
	return nil
}

// Active, o an çalışan iş sayısı.
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// tryAcquire, boş slot varsa alır.
//
// Sabit boyutlu kanal semaforu yerine sayaç kullanılıyor: sınır ayarlardan
// geliyor ve çalışma anında değişebiliyor. Kanal boyutu değiştirilemezdi.
func (m *Manager) tryAcquire() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	limit := m.limits.MaxConcurrent()
	if m.active >= limit {
		return false
	}
	m.active++
	return true
}

func (m *Manager) release() {
	m.mu.Lock()
	m.active--
	m.mu.Unlock()
	m.cond.Broadcast()
}

// RecoverInterrupted, açılışta yarım kalmış kayıtları kapatır.
func (m *Manager) RecoverInterrupted(ctx context.Context) (int, error) {
	n, err := m.store.RecoverInterrupted(ctx)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		slog.WarnContext(ctx, "yarım kalmış çalıştırmalar kapatıldı", "adet", n)
	}
	return n, nil
}

// Shutdown, çalışan işleri iptal eder ve temizlenmelerini bekler.
//
// Beklemek şart: container'lar runner'ın defer'ları içinde siliniyor,
// süreç erken ölürse sahipsiz container kalır.
func (m *Manager) Shutdown() {
	m.shutdown()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(45 * time.Second):
		slog.Error("çalıştırmalar zamanında temizlenmedi — sahipsiz container kalmış olabilir")
	}
}
