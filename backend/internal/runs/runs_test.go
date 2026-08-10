package runs_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/events"
	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runs"
	"github.com/agent-coder/backend/internal/testutil"
)

// fixture, çalıştırma testleri için proje ve agent hazırlar.
type fixture struct {
	pool      *pgxpool.Pool
	store     *runs.Store
	projectID uuid.UUID
	agentID   uuid.UUID
}

func setup(t *testing.T) fixture {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "runs", "projects", "agents")

	ctx := context.Background()

	var projectID, agentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, repo_url) VALUES ('Deneme', 'https://example.com/r.git')
		 RETURNING id`).Scan(&projectID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO agents (slug, name, prompt, source, builtin_prompt)
		 VALUES ('incelemeci', 'İncelemeci', 'özgün talimat', 'builtin', 'özgün talimat')
		 RETURNING id`).Scan(&agentID))

	return fixture{pool: pool, store: runs.NewStore(pool), projectID: projectID, agentID: agentID}
}

func (f fixture) create(t *testing.T, task string) runs.Run {
	t.Helper()

	run, err := f.store.Create(context.Background(), runs.CreateInput{
		ProjectID:    f.projectID,
		AgentID:      f.agentID,
		AgentSlug:    "incelemeci",
		AgentPrompt:  "özgün talimat",
		ProviderSlug: "openrouter",
		ModelID:      "anthropic/claude-haiku-4.5",
		Branch:       "main",
		Task:         task,
	})
	require.NoError(t, err)
	return run
}

// TestGecmisKopyasiAgentDegisince Sabit Kalir — spec 003'ün anlık kopya kararı.
//
// Agent'ın talimatı sonradan değişirse geçmiş kayıt NEYLE çalıştığını yanlış
// gösterirdi. Kayıt referans değil kopya tuttuğu için değişmez.
func TestGecmisKopyasi_AgentDegisinceSabitKalir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run := f.create(t, "eski görevi incele")

	_, err := f.pool.Exec(ctx,
		`UPDATE agents SET prompt = 'yepyeni talimat' WHERE id = $1`, f.agentID)
	require.NoError(t, err)

	after, err := f.store.Get(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, "özgün talimat", after.AgentPrompt,
		"agent düzenlenince geçmiş kaydın kopyası değişmemeli")
	require.Equal(t, "anthropic/claude-haiku-4.5", after.ModelID)
}

// TestCalistirmasiOlanAgentSilinemez — geçmiş kime ait olduğunu bilmeli.
func TestCalistirmasiOlanAgentSilinemez(t *testing.T) {
	f := setup(t)

	f.create(t, "bir iş")

	_, err := f.pool.Exec(context.Background(),
		`DELETE FROM agents WHERE id = $1`, f.agentID)
	require.Error(t, err, "çalıştırması olan agent silinebilmemeli")
}

// TestOlaylar_SiraliYazilirVeOkunur — SSE geçmişi doğru sırada göndermeli.
func TestOlaylar_SiraliYazilirVeOkunur(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run := f.create(t, "olaylı iş")

	mesajlar := []string{"birinci", "ikinci", "üçüncü", "dördüncü"}
	for i, msg := range mesajlar {
		seq, _, err := f.store.AppendEvent(ctx, run.ID, "info", msg)
		require.NoError(t, err)
		require.Equal(t, i+1, seq, "sıra numarası veritabanında üretilmeli")
	}

	// Başka bir çalıştırmanın olayları karışmamalı: numara çalıştırma başınadır.
	other := f.create(t, "başka iş")
	seq, _, err := f.store.AppendEvent(ctx, other.ID, "info", "yabancı")
	require.NoError(t, err)
	require.Equal(t, 1, seq)

	got, err := f.store.Events(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, got, len(mesajlar))
	for i, e := range got {
		require.Equal(t, i+1, e.Seq)
		require.Equal(t, mesajlar[i], e.Message)
	}
}

/* ── Manager ─────────────────────────────────────────────────────────────── */

// stubRunner, gerçek container açmadan Runner arayüzünü karşılar.
type stubRunner struct {
	// block, Run'ın bekleyeceği kanal; kapanana veya ctx iptal olana kadar sürer.
	block chan struct{}
	// started, kaç çalıştırmanın motora ulaştığı.
	started atomic.Int32
	// err, block kapandığında dönülecek hata.
	err error
}

func (s *stubRunner) Run(ctx context.Context, _ runner.Request, emit runner.EventFunc) (*runner.Result, error) {
	s.started.Add(1)
	emit(runner.Event{Level: runner.LevelInfo, Message: "başladı"})

	select {
	case <-s.block:
		return &runner.Result{Output: "bitti"}, s.err
	case <-ctx.Done():
		return nil, runner.ErrCancelled
	}
}

func newManager(f fixture, r runner.Runner, maxConcurrent func() int) (*runs.Manager, *events.Bus) {
	bus := events.New()
	return runs.NewManager(f.store, r, bus, runs.Limits{
		MaxConcurrent: maxConcurrent,
		Timeout:       func() time.Duration { return time.Minute },
		CPUCores:      func() int { return 1 },
		MemoryGB:      func() int { return 1 },
		CloneDepth:    func() int { return 1 },
	}), bus
}

func startInput(f fixture, task string) runs.StartInput {
	return runs.StartInput{
		Create: runs.CreateInput{
			ProjectID:    f.projectID,
			AgentID:      f.agentID,
			AgentSlug:    "incelemeci",
			AgentPrompt:  "özgün talimat",
			ProviderSlug: "openrouter",
			ModelID:      "anthropic/claude-haiku-4.5",
			Branch:       "main",
			Task:         task,
		},
		Repo:     runner.RepoSpec{URL: "https://example.com/r.git", Branch: "main"},
		Agent:    runner.AgentSpec{Slug: "incelemeci", Prompt: "özgün talimat"},
		Provider: runner.ProviderSpec{Slug: "openrouter", Kind: "openrouter", APIKey: "k"},
	}
}

// waitFor, koşul gerçekleşene kadar kısa aralıklarla bekler.
func waitFor(t *testing.T, mesaj string, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("koşul zamanında gerçekleşmedi: %s", mesaj)
}

func TestManager_SinirDolunca_Reddeder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	stub := &stubRunner{block: make(chan struct{})}
	m, _ := newManager(f, stub, func() int { return 1 })
	defer m.Shutdown()

	_, err := m.Start(ctx, startInput(f, "birinci"))
	require.NoError(t, err)
	waitFor(t, "ilk iş motora ulaşmalı", func() bool { return stub.started.Load() == 1 })

	_, err = m.Start(ctx, startInput(f, "ikinci"))
	require.ErrorIs(t, err, runs.ErrTooManyRuns)

	// Reddedilen iş veritabanına HİÇ yazılmamalı: hiç başlamamış "pending"
	// kayıtlar bırakmak geçmişi kirletirdi.
	var n int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM runs WHERE task = 'ikinci'`).Scan(&n))
	require.Zero(t, n)

	close(stub.block)
}

// TestManager_SinirDegisince_YenidenBaslatmadanGecerliOlur — H7'nin can alıcı noktası.
func TestManager_SinirDegisince_YenidenBaslatmadanGecerliOlur(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	// Sınır ayardan okunuyormuş gibi, her çağrıda yeniden hesaplanır.
	var mu sync.Mutex
	limit := 1
	get := func() int {
		mu.Lock()
		defer mu.Unlock()
		return limit
	}

	stub := &stubRunner{block: make(chan struct{})}
	m, _ := newManager(f, stub, get)
	defer m.Shutdown()

	_, err := m.Start(ctx, startInput(f, "birinci"))
	require.NoError(t, err)
	waitFor(t, "ilk iş motora ulaşmalı", func() bool { return stub.started.Load() == 1 })

	_, err = m.Start(ctx, startInput(f, "ikinci"))
	require.ErrorIs(t, err, runs.ErrTooManyRuns)

	mu.Lock()
	limit = 3
	mu.Unlock()

	// Manager yeniden başlatılmadı; yeni sınır hemen geçerli olmalı.
	_, err = m.Start(ctx, startInput(f, "üçüncü"))
	require.NoError(t, err)
	require.Equal(t, 2, m.Active())

	close(stub.block)
}

func TestManager_Iptal_KaydiIptalOlarakKapatir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	stub := &stubRunner{block: make(chan struct{})}
	m, _ := newManager(f, stub, func() int { return 2 })
	defer m.Shutdown()

	run, err := m.Start(ctx, startInput(f, "iptal edilecek"))
	require.NoError(t, err)
	waitFor(t, "iş motora ulaşmalı", func() bool { return stub.started.Load() == 1 })

	require.NoError(t, m.Cancel(run.ID))

	waitFor(t, "kayıt iptal olarak kapanmalı", func() bool {
		got, err := f.store.Get(ctx, run.ID)
		return err == nil && got.Status == runs.StatusCancelled
	})

	// Slot geri verilmeli, yoksa sınır kalıcı olarak eksilirdi.
	waitFor(t, "slot serbest bırakılmalı", func() bool { return m.Active() == 0 })

	// Bitmiş bir iş ikinci kez iptal edilemez.
	require.Error(t, m.Cancel(run.ID))

	close(stub.block)
}

func TestManager_HataDurumu_KayitBasarisizOlur(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	stub := &stubRunner{block: make(chan struct{}), err: errors.New("motor patladı")}
	close(stub.block) // beklemeden bitsin

	m, _ := newManager(f, stub, func() int { return 2 })
	defer m.Shutdown()

	run, err := m.Start(ctx, startInput(f, "patlayacak"))
	require.NoError(t, err)

	waitFor(t, "kayıt başarısız olarak kapanmalı", func() bool {
		got, err := f.store.Get(ctx, run.ID)
		return err == nil && got.Status == runs.StatusFailed
	})

	got, err := f.store.Get(ctx, run.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Error)
	require.Contains(t, *got.Error, "motor patladı")
}

// TestRecoverInterrupted, sunucu ortada ölmüş gibi kalan kayıtları kapatır.
func TestRecoverInterrupted(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run := f.create(t, "yarım kalan")
	require.NoError(t, f.store.MarkRunning(ctx, run.ID))

	n, err := f.store.RecoverInterrupted(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	got, err := f.store.Get(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, runs.StatusInterrupted, got.Status)
	require.NotNil(t, got.FinishedAt, "kapatılan kayıt bitiş zamanı taşımalı")

	// İkinci çağrı bir şey bulmamalı — kapanmış kayıtlara dokunmaz.
	n, err = f.store.RecoverInterrupted(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}
