package workflow_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Motor testleri sahte bir StepRunner ile çalışır: gerçek container açmadan
 * sıra, paralellik, hata ve iptal davranışı sınanabilsin diye. Motorun en çok
 * hata barındıran kısmı bu mantık, en ucuz doğrulama yeri de burası.
 */

type fakeRunner struct {
	mu sync.Mutex

	// makeRun, gerçek bir çalıştırma kaydı üretir. Sahte bir UUID döndürmek
	// yabancı anahtar kısıtına takılıyordu — kısıt haklıydı, testin kendisi
	// gerçekçi değildi.
	makeRun func() uuid.UUID

	// prompts, her düğüme giden çözümlenmiş talimat.
	prompts map[string]string
	// order, adımların başlama sırası.
	order []string
	// overlap, aynı anda kaç adımın çalıştığının en yüksek değeri.
	active, overlap int

	// failOn, bu düğümde hata döndürülür.
	failOn string
	// block, doluysa her adım bu kanalı bekler (iptal testleri için).
	block chan struct{}
	// delay, adım başına yapay gecikme (paralellik ölçümü için).
	delay time.Duration
}

func newFake(makeRun func() uuid.UUID) *fakeRunner {
	return &fakeRunner{prompts: map[string]string{}, makeRun: makeRun}
}

func (f *fakeRunner) RunStep(ctx context.Context, req workflow.StepRequest) (workflow.StepOutcome, error) {
	f.mu.Lock()
	f.prompts[req.Node.ID] = req.Prompt
	f.order = append(f.order, req.Node.ID)
	f.active++
	if f.active > f.overlap {
		f.overlap = f.active
	}
	fail := f.failOn == req.Node.ID
	block, delay := f.block, f.delay
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()

	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return workflow.StepOutcome{RunID: f.makeRun()}, ctx.Err()
		}
	}
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return workflow.StepOutcome{RunID: f.makeRun()}, ctx.Err()
		}
	}
	if fail {
		return workflow.StepOutcome{RunID: f.makeRun()}, errors.New("motor patladı")
	}

	return workflow.StepOutcome{
		RunID:  f.makeRun(),
		Output: req.Node.ID + " çıktısı",
		Diff:   req.Node.ID + " diff",
	}, nil
}

// waitRun, çalışma bitene kadar bekler.
func waitRun(t *testing.T, f fixture, id uuid.UUID) workflow.Run {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		run, err := f.store.GetRun(context.Background(), id)
		require.NoError(t, err)
		if run.Status.Terminal() {
			return run
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("akış zamanında bitmedi")
	return workflow.Run{}
}

// start, akışı kurar ve motoru çalıştırır.
func (f fixture) start(t *testing.T, g workflow.Graph, fake *fakeRunner, input string) (
	*workflow.Executor, workflow.Run,
) {
	t.Helper()
	ctx := context.Background()

	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, g)
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: input,
	})
	require.NoError(t, err)

	ex := workflow.NewExecutor(f.store, workflow.Handlers{
		workflow.KindAgent: workflow.NewAgentHandler(fake),
	}, nil)
	ex.Start(run, v.Graph)
	return ex, run
}

func TestExecutor_SiraliAkisVeSablonAktarimi(t *testing.T) {
	f := setup(t)
	fake := newFake(func() uuid.UUID { return f.insertRun(t, 0.01, 10, 5) })

	ex, run := f.start(t, f.graph(), fake, "hata düzelt")
	defer ex.Shutdown()

	after := waitRun(t, f, run.ID)
	require.Equal(t, workflow.RunSucceeded, after.Status)
	require.Equal(t, []string{"a1", "a2"}, fake.order, "adımlar sırayla çalışmalı")

	// {{ input }} ve {{ steps.a1.output }} gerçekten dolu gelmeli.
	require.Contains(t, fake.prompts["a1"], "hata düzelt")
	require.Contains(t, fake.prompts["a2"], "a1 çıktısı")

	for _, st := range after.Steps {
		require.Equal(t, workflow.StepSucceeded, st.Status)
		require.NotNil(t, st.RunID, "her adım bir çalıştırma kaydına bağlanmalı")
	}
}

// TestExecutor_ParalelAdimlarAyniAnda — kabul kriteri: bağımsız adımlar beklemez.
func TestExecutor_ParalelAdimlarAyniAnda(t *testing.T) {
	f := setup(t)
	fake := newFake(func() uuid.UUID { return f.insertRun(t, 0.01, 10, 5) })
	fake.delay = 80 * time.Millisecond

	g := workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "t1", Kind: workflow.KindTriggerManual},
			{ID: "a1", Kind: workflow.KindAgent,
				Config: workflow.NodeConfig{AgentID: f.agentID.String(), Model: "m", Prompt: "x"}},
			{ID: "a2", Kind: workflow.KindAgent,
				Config: workflow.NodeConfig{AgentID: f.agentID.String(), Model: "m", Prompt: "y"}},
		},
		Edges: []workflow.Edge{{From: "t1", To: "a1"}, {From: "t1", To: "a2"}},
	}

	ex, run := f.start(t, g, fake, "x")
	defer ex.Shutdown()

	after := waitRun(t, f, run.ID)
	require.Equal(t, workflow.RunSucceeded, after.Status)
	require.Equal(t, 2, fake.overlap, "aynı seviyedeki iki adım aynı anda çalışmalı")
}

// TestExecutor_HataSonrasiSonrakilerAtlanir — spec 007 K2.
func TestExecutor_HataSonrasiSonrakilerAtlanir(t *testing.T) {
	f := setup(t)
	fake := newFake(func() uuid.UUID { return f.insertRun(t, 0.01, 10, 5) })
	fake.failOn = "a1"

	ex, run := f.start(t, f.graph(), fake, "x")
	defer ex.Shutdown()

	after := waitRun(t, f, run.ID)
	require.Equal(t, workflow.RunFailed, after.Status)
	require.NotNil(t, after.Error)
	require.Contains(t, *after.Error, "motor patladı")

	require.Equal(t, workflow.StepFailed, after.Steps[0].Status)
	require.Equal(t, workflow.StepSkipped, after.Steps[1].Status,
		"hatadan sonraki adım 'sırada bekliyor' kalmamalı")
	require.NotContains(t, fake.order, "a2", "atlanan adım hiç çalıştırılmamalı")
}

// TestExecutor_BasarisizAdimDaCalistirmayaBaglanir — kullanıcı hatalı adımın
// çıktısını ve maliyetini görebilmeli.
func TestExecutor_BasarisizAdimCalistirmayaBaglanir(t *testing.T) {
	f := setup(t)
	fake := newFake(func() uuid.UUID { return f.insertRun(t, 0.01, 10, 5) })
	fake.failOn = "a1"

	ex, run := f.start(t, f.graph(), fake, "x")
	defer ex.Shutdown()

	after := waitRun(t, f, run.ID)
	require.NotNil(t, after.Steps[0].RunID)
	require.Nil(t, after.Steps[1].RunID, "hiç çalışmayan adımın çalıştırması olmaz")
}

func TestExecutor_Iptal(t *testing.T) {
	f := setup(t)
	fake := newFake(func() uuid.UUID { return f.insertRun(t, 0.01, 10, 5) })
	fake.block = make(chan struct{})

	ex, run := f.start(t, f.graph(), fake, "x")
	defer ex.Shutdown()

	// İlk adım motora ulaşana kadar bekle.
	require.Eventually(t, func() bool {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(fake.order) == 1
	}, 5*time.Second, 10*time.Millisecond)

	require.NoError(t, ex.Cancel(run.ID))

	after := waitRun(t, f, run.ID)
	// İptal BAŞARISIZLIK DEĞİLDİR: kullanıcı bilerek durdurdu.
	// Sunucu kapanması ayrı bir durumdur (interrupted) — karışmamalı.
	require.Equal(t, workflow.RunCancelled, after.Status)
	require.Equal(t, workflow.StepCancelled, after.Steps[0].Status)
	require.Equal(t, workflow.StepCancelled, after.Steps[1].Status)

	close(fake.block)

	// Bitmiş akış ikinci kez iptal edilemez.
	require.Error(t, ex.Cancel(run.ID))
}

func TestExecutor_UcAdimliZincir(t *testing.T) {
	f := setup(t)
	fake := newFake(func() uuid.UUID { return f.insertRun(t, 0.01, 10, 5) })

	g := workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "t1", Kind: workflow.KindTriggerManual},
			{ID: "analiz", Kind: workflow.KindAgent, Name: "Analiz",
				Config: workflow.NodeConfig{AgentID: f.agentID.String(), Model: "ucuz",
					Prompt: "Analiz et: {{ input }}"}},
			{ID: "kod", Kind: workflow.KindAgent, Name: "Kod",
				Config: workflow.NodeConfig{AgentID: f.agentID.String(), Model: "guclu",
					Prompt: "Uygula:\n{{ steps.analiz.output }}"}},
			{ID: "inceleme", Kind: workflow.KindAgent, Name: "İnceleme",
				Config: workflow.NodeConfig{AgentID: f.agentID.String(), Model: "orta",
					Prompt: "İncele:\n{{ steps.kod.diff }}"}},
		},
		Edges: []workflow.Edge{
			{From: "t1", To: "analiz"}, {From: "analiz", To: "kod"}, {From: "kod", To: "inceleme"},
		},
	}

	ex, run := f.start(t, g, fake, "şu hatayı düzelt")
	defer ex.Shutdown()

	after := waitRun(t, f, run.ID)
	require.Equal(t, workflow.RunSucceeded, after.Status)
	require.Equal(t, []string{"analiz", "kod", "inceleme"}, fake.order)

	// Zincirin her halkası bir öncekinin çıktısını görmeli.
	require.Contains(t, fake.prompts["analiz"], "şu hatayı düzelt")
	require.Contains(t, fake.prompts["kod"], "analiz çıktısı")
	require.Contains(t, fake.prompts["inceleme"], "kod diff")
}

// TestExecutor_AgentOlmayanAdiminSonucuSaklanir — PR adresi en işe yarar bilgi;
// kaydedilmezse kullanıcı "PR aç ✓" görür ama nereye gideceğini bilemez.
func TestExecutor_AgentOlmayanAdiminSonucuSaklanir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	w := f.newWorkflow(t)
	g := workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "t1", Kind: workflow.KindTriggerManual},
			{ID: "pr", Kind: workflow.KindGitHubPR, Name: "PR aç",
				Config: workflow.NodeConfig{Title: "Başlık", HeadBranch: "dal"}},
		},
		Edges: []workflow.Edge{{From: "t1", To: "pr"}},
	}
	v, err := f.store.SaveVersion(ctx, w.ID, g)
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)

	ex := workflow.NewExecutor(f.store, workflow.Handlers{
		workflow.KindGitHubPR: fakeAction{url: "https://github.com/a/b/pull/1"},
	}, nil)
	defer ex.Shutdown()
	ex.Start(run, v.Graph)

	after := waitRun(t, f, run.ID)
	require.Equal(t, workflow.RunSucceeded, after.Status)
	require.Equal(t, "https://github.com/a/b/pull/1", after.Steps[0].ResultURL)
	require.Contains(t, after.Steps[0].ResultText, "PR")
	require.Nil(t, after.Steps[0].RunID, "model çağırmayan adımın çalıştırması olmaz")
	require.Zero(t, after.Steps[0].CostUSD, "rapor rakamları bozulmamalı")
}

// fakeAction, model çağırmayan bir düğümü taklit eder.
type fakeAction struct{ url string }

func (f fakeAction) Execute(context.Context, workflow.ExecInput) (workflow.StepOutcome, error) {
	return workflow.StepOutcome{Output: "PR #1 açıldı", URL: f.url}, nil
}

/*
KANCA, SON DURUM YAZILDIKTAN SONRA ÇAĞRILIR.

Toplu çalıştırma kuyruğu (spec 023) bu sinyalle uyanıp öğeyi kapatıyor. Sıra
ters olsaydı kuyruk uyanır, çalışmayı hâlâ `running` görür ve bitişi ancak
dakikalık emniyet turunda fark ederdi — yani biten iş bir dakikaya kadar
kuyrukta yer tutar, sıradaki o kadar geç başlardı. Tam olarak bu oldu ve bu
test onu geri gelmekten korur.

Kanca durumu KENDİ İÇİNDE okuyor: "sonradan bakınca terminal" yetmez, çağrının
YAPILDIĞI ANDA terminal olmalı.
*/
func TestExecutor_KancaSonDurumYazildiktanSonraCagrilir(t *testing.T) {
	f := setup(t)
	fake := newFake(func() uuid.UUID { return f.insertRun(t, 0.01, 10, 5) })

	ctx := context.Background()
	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)

	ex := workflow.NewExecutor(f.store, workflow.Handlers{
		workflow.KindAgent: workflow.NewAgentHandler(fake),
	}, nil)
	defer ex.Shutdown()

	kancaDurumu := make(chan workflow.RunStatus, 1)
	ex.SetOnRunFinished(func(runID uuid.UUID) {
		r, err := f.store.GetRun(context.Background(), runID)
		if err != nil {
			return
		}
		kancaDurumu <- r.Status
	})

	ex.Start(run, v.Graph)

	select {
	case durum := <-kancaDurumu:
		require.Equal(t, workflow.RunSucceeded, durum,
			"kanca çağrıldığı ANDA çalışma bitmiş olmalı — yoksa kuyruk bitişi göremez")
	case <-time.After(10 * time.Second):
		t.Fatal("çalışma bitti ama kanca hiç çağrılmadı")
	}
}

// Hata yolu da kancayı çağırmalı: düşen bir öğe kuyrukta sonsuza kadar
// "çalışıyor" kalmamalı.
func TestExecutor_KancaHataYolundaDaCagrilir(t *testing.T) {
	f := setup(t)
	fake := newFake(func() uuid.UUID { return f.insertRun(t, 0, 0, 0) })
	fake.failOn = "a1"

	ctx := context.Background()
	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)
	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)

	ex := workflow.NewExecutor(f.store, workflow.Handlers{
		workflow.KindAgent: workflow.NewAgentHandler(fake),
	}, nil)
	defer ex.Shutdown()

	kancaDurumu := make(chan workflow.RunStatus, 1)
	ex.SetOnRunFinished(func(runID uuid.UUID) {
		r, err := f.store.GetRun(context.Background(), runID)
		if err != nil {
			return
		}
		kancaDurumu <- r.Status
	})

	ex.Start(run, v.Graph)

	select {
	case durum := <-kancaDurumu:
		require.Equal(t, workflow.RunFailed, durum)
	case <-time.After(10 * time.Second):
		t.Fatal("akış düştü ama kanca hiç çağrılmadı")
	}
}
