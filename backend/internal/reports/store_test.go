package reports_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/reports"
	"github.com/agent-coder/backend/internal/settings"
	"github.com/agent-coder/backend/internal/testutil"
)

// setup, boş bir rapor ortamı hazırlar: proje + agent + ayar servisi.
func setup(t *testing.T) (*pgxpool.Pool, *reports.Store, uuid.UUID, uuid.UUID) {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "runs", "projects", "agents", "settings")

	ctx := context.Background()

	var projectID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (name, repo_url) VALUES ('Deneme', 'https://example.com/r.git')
		 RETURNING id`).Scan(&projectID); err != nil {
		t.Fatalf("proje eklenemedi: %v", err)
	}

	var agentID uuid.UUID
	if err := pool.QueryRow(ctx,
		`INSERT INTO agents (slug, name, prompt, source) VALUES ('coder', 'Coder', 'p', 'builtin')
		 RETURNING id`).Scan(&agentID); err != nil {
		t.Fatalf("agent eklenemedi: %v", err)
	}

	svc := settings.NewService(pool)
	if err := svc.Load(ctx); err != nil {
		t.Fatalf("ayarlar yüklenemedi: %v", err)
	}

	return pool, reports.NewStore(pool, svc), projectID, agentID
}

// runSpec, teste özel kısa çalıştırma tanımı.
type runSpec struct {
	status    string
	model     string
	agentSlug string
	cost      float64
	prompt    int
	completed int
	files     string
	pushed    *string
	// ageDays, kaydın kaç gün önce oluşturulduğu.
	ageDays  int
	duration time.Duration
}

func insertRun(t *testing.T, pool *pgxpool.Pool, projectID, agentID uuid.UUID, s runSpec) {
	t.Helper()

	if s.agentSlug == "" {
		s.agentSlug = "coder"
	}
	if s.model == "" {
		s.model = "anthropic/claude-sonnet-4.5"
	}
	if s.files == "" {
		s.files = "[]"
	}

	// Zaman damgası ŞU ANDAN geriye sayılarak üretilir, UTC gün ortasına
	// sabitlenerek değil.
	//
	// Sabitlenmiş hali kırıldı: rapor günleri RAPOR SAAT DİLİMİNE göre bölüyor
	// (Europe/Istanbul) ve UTC ile İstanbul farklı takvim günlerine düştüğünde
	// "bugün eklenen kayıt" raporun dünü oluyordu. Test yerelde geçip gece
	// yarısından sonra kırılıyordu — ürün doğruydu, test kırılgandı.
	created := time.Now().AddDate(0, 0, -s.ageDays)

	_, err := pool.Exec(context.Background(), `
		INSERT INTO runs (project_id, agent_id, agent_slug, agent_prompt,
			provider_slug, model_id, branch, task, status,
			cost_usd, prompt_tokens, completion_tokens, files, pushed_branch,
			created_at, started_at, finished_at)
		VALUES ($1,$2,$3,'prompt','openrouter',$4,'main','iş',$5,
			$6,$7,$8,$9::jsonb,$10,$11,$11,$12)`,
		projectID, agentID, s.agentSlug, s.model, s.status,
		s.cost, s.prompt, s.completed, s.files, s.pushed,
		created, created.Add(s.duration))
	if err != nil {
		t.Fatalf("çalıştırma eklenemedi: %v", err)
	}
}

func TestSummaryTotals(t *testing.T) {
	pool, store, projectID, agentID := setup(t)

	branch := "agent/deneme"
	insertRun(t, pool, projectID, agentID, runSpec{
		status: "succeeded", cost: 0.25, prompt: 1000, completed: 200,
		files:    `[{"file":"a.go","additions":10,"deletions":2,"status":"modified"}]`,
		pushed:   &branch,
		duration: 60 * time.Second,
	})
	insertRun(t, pool, projectID, agentID, runSpec{
		status: "succeeded", cost: 0.25, prompt: 500, completed: 100,
		files:    `[{"file":"b.go","additions":5,"deletions":1,"status":"added"},{"file":"c.go","additions":1,"deletions":0,"status":"added"}]`,
		duration: 120 * time.Second,
	})
	insertRun(t, pool, projectID, agentID, runSpec{status: "failed", cost: 0.1, ageDays: 2})
	insertRun(t, pool, projectID, agentID, runSpec{status: "timeout", ageDays: 3})
	// Dönemin DIŞINDA: 40 gün önceki kayıt 30 günlük rapora girmemeli.
	insertRun(t, pool, projectID, agentID, runSpec{status: "succeeded", cost: 9.99, ageDays: 40})

	sum, err := store.Summary(context.Background(), reports.Query{Days: 30})
	if err != nil {
		t.Fatalf("özet üretilemedi: %v", err)
	}

	if sum.Totals.Runs != 4 {
		t.Errorf("çalıştırma sayısı = %d, beklenen 4", sum.Totals.Runs)
	}
	if sum.Totals.Succeeded != 2 || sum.Totals.Failed != 1 || sum.Totals.Timeout != 1 {
		t.Errorf("durum kırılımı yanlış: %+v", sum.Totals)
	}
	if got := sum.Totals.CostUSD; got < 0.5999 || got > 0.6001 {
		t.Errorf("maliyet = %v, beklenen 0.60 (dönem dışı kayıt sızmış olabilir)", got)
	}
	if sum.Totals.PromptTokens != 1500 || sum.Totals.CompletionTokens != 300 {
		t.Errorf("token toplamı yanlış: %+v", sum.Totals)
	}
	if sum.Totals.FilesChanged != 3 {
		t.Errorf("değişen dosya = %d, beklenen 3", sum.Totals.FilesChanged)
	}
	if sum.Totals.Additions != 16 || sum.Totals.Deletions != 3 {
		t.Errorf("satır toplamı yanlış: +%d −%d, beklenen +16 −3",
			sum.Totals.Additions, sum.Totals.Deletions)
	}
	if sum.Totals.PushedBranches != 1 {
		t.Errorf("gönderilen branch = %d, beklenen 1", sum.Totals.PushedBranches)
	}
	if sum.Totals.AvgDurationSec < 44.9 || sum.Totals.AvgDurationSec > 45.1 {
		t.Errorf("ortalama süre = %v, beklenen 45 sn", sum.Totals.AvgDurationSec)
	}
}

// TestSummaryDailyFillsGaps, kaydı olmayan günlerin de dizide durduğunu doğrular:
// eksik gün grafikte atlanırsa zaman ekseni sıkışır ve trend yanlış okunur.
func TestSummaryDailyFillsGaps(t *testing.T) {
	pool, store, projectID, agentID := setup(t)

	insertRun(t, pool, projectID, agentID, runSpec{status: "succeeded", cost: 1, ageDays: 0})
	insertRun(t, pool, projectID, agentID, runSpec{status: "failed", ageDays: 3})

	sum, err := store.Summary(context.Background(), reports.Query{Days: 7})
	if err != nil {
		t.Fatalf("özet üretilemedi: %v", err)
	}

	if len(sum.Daily) != 7 {
		t.Fatalf("gün sayısı = %d, beklenen 7", len(sum.Daily))
	}
	// Diziye tarih sırasıyla girmeli: son eleman bugün.
	last := sum.Daily[len(sum.Daily)-1]
	if last.Succeeded != 1 || last.Runs != 1 {
		t.Errorf("bugünün satırı yanlış: %+v", last)
	}
	if sum.Daily[3].Failed != 1 {
		t.Errorf("3 gün önceki satır yanlış: %+v", sum.Daily[3])
	}

	total := 0
	for _, d := range sum.Daily {
		total += d.Runs
	}
	if total != sum.Totals.Runs {
		t.Errorf("günlük toplam %d, genel toplam %d — ikisi tutmalı",
			total, sum.Totals.Runs)
	}
}

func TestSummaryBreakdowns(t *testing.T) {
	pool, store, projectID, agentID := setup(t)

	insertRun(t, pool, projectID, agentID, runSpec{status: "succeeded", agentSlug: "coder", model: "m1", cost: 0.3})
	insertRun(t, pool, projectID, agentID, runSpec{status: "failed", agentSlug: "coder", model: "m1", cost: 0.1})
	insertRun(t, pool, projectID, agentID, runSpec{status: "succeeded", agentSlug: "reviewer", model: "m2", cost: 0.2})

	sum, err := store.Summary(context.Background(), reports.Query{Days: 30})
	if err != nil {
		t.Fatalf("özet üretilemedi: %v", err)
	}

	if len(sum.ByAgent) != 2 {
		t.Fatalf("agent kırılımı = %d satır, beklenen 2", len(sum.ByAgent))
	}
	// En çok çalışan başa gelmeli.
	if sum.ByAgent[0].Label != "coder" || sum.ByAgent[0].Runs != 2 {
		t.Errorf("agent sıralaması yanlış: %+v", sum.ByAgent[0])
	}
	if sum.ByAgent[0].Succeeded != 1 || sum.ByAgent[0].Failed != 1 {
		t.Errorf("agent durum kırılımı yanlış: %+v", sum.ByAgent[0])
	}

	if len(sum.ByModel) != 2 {
		t.Errorf("model kırılımı = %d satır, beklenen 2", len(sum.ByModel))
	}
	if len(sum.ByProject) != 1 || sum.ByProject[0].Label != "Deneme" {
		t.Errorf("proje kırılımı yanlış: %+v", sum.ByProject)
	}
}

func TestSummaryPreviousPeriod(t *testing.T) {
	pool, store, projectID, agentID := setup(t)

	insertRun(t, pool, projectID, agentID, runSpec{status: "succeeded", cost: 1, ageDays: 1})
	// 10 gün önce = 7 günlük dönemin bir öncekine düşer.
	insertRun(t, pool, projectID, agentID, runSpec{status: "succeeded", cost: 4, ageDays: 10})

	sum, err := store.Summary(context.Background(), reports.Query{Days: 7})
	if err != nil {
		t.Fatalf("özet üretilemedi: %v", err)
	}

	if sum.Totals.Runs != 1 {
		t.Errorf("dönem içi = %d, beklenen 1", sum.Totals.Runs)
	}
	if sum.Previous.Runs != 1 || sum.Previous.CostUSD < 3.99 {
		t.Errorf("önceki dönem yanlış: %+v", sum.Previous)
	}
}

// TestSummaryEmpty, hiç kayıt yokken sayfanın çizilebildiğini doğrular:
// nil dilim JSON'da null olur ve arayüzde çökme üretir.
func TestSummaryEmpty(t *testing.T) {
	_, store, _, _ := setup(t)

	sum, err := store.Summary(context.Background(), reports.Query{Days: 30})
	if err != nil {
		t.Fatalf("özet üretilemedi: %v", err)
	}

	if sum.Totals.Runs != 0 || sum.Totals.CostUSD != 0 {
		t.Errorf("boş dönem sıfır olmalı: %+v", sum.Totals)
	}
	if sum.ByAgent == nil || sum.ByModel == nil || sum.ByProject == nil || sum.Failures == nil {
		t.Error("kırılımlar boş dilim olmalı, nil değil — JSON'da null dönerdi")
	}
	if len(sum.Daily) != 30 {
		t.Errorf("boş dönemde de 30 gün dönmeli, dönen %d", len(sum.Daily))
	}
}

// TestSummaryDefaultDays, dönem verilmediğinde ayardaki varsayılanın
// kullanıldığını doğrular — parametre kodda gömülü kalmamalı (H7).
func TestSummaryDefaultDays(t *testing.T) {
	pool, _, _, _ := setup(t)

	ctx := context.Background()
	svc := settings.NewService(pool)
	if err := svc.Load(ctx); err != nil {
		t.Fatalf("ayarlar yüklenemedi: %v", err)
	}
	if err := svc.Set(ctx, settings.KeyReportDefaultDays, "14"); err != nil {
		t.Fatalf("ayar değiştirilemedi: %v", err)
	}

	sum, err := reports.NewStore(pool, svc).Summary(ctx, reports.Query{})
	if err != nil {
		t.Fatalf("özet üretilemedi: %v", err)
	}
	if sum.Days != 14 || len(sum.Daily) != 14 {
		t.Errorf("varsayılan dönem uygulanmadı: days=%d, gün=%d", sum.Days, len(sum.Daily))
	}
}

func TestSummaryFailures(t *testing.T) {
	pool, store, projectID, agentID := setup(t)

	ctx := context.Background()
	insertRun(t, pool, projectID, agentID, runSpec{status: "failed"})
	insertRun(t, pool, projectID, agentID, runSpec{status: "failed"})
	if _, err := pool.Exec(ctx,
		`UPDATE runs SET error = 'depo bulunamadı'`); err != nil {
		t.Fatalf("hata metni yazılamadı: %v", err)
	}

	sum, err := store.Summary(ctx, reports.Query{Days: 30})
	if err != nil {
		t.Fatalf("özet üretilemedi: %v", err)
	}

	if len(sum.Failures) != 1 || sum.Failures[0].Count != 2 {
		t.Fatalf("hata kırılımı yanlış: %+v", sum.Failures)
	}
	if sum.Failures[0].Message != "depo bulunamadı" {
		t.Errorf("hata metni = %q", sum.Failures[0].Message)
	}
}

// TestLocationFallsBackToUTC, tanınmayan saat dilimi raporu bozmamalı.
func TestLocationFallsBackToUTC(t *testing.T) {
	pool, _, _, _ := setup(t)

	ctx := context.Background()
	svc := settings.NewService(pool)
	if err := svc.Load(ctx); err != nil {
		t.Fatalf("ayarlar yüklenemedi: %v", err)
	}
	// Doğrulama yalnızca "boş olamaz" der; geçersiz bir ad buraya girebilir.
	if err := svc.Set(ctx, settings.KeyReportTimezone, "Mars/Olympus"); err != nil {
		t.Fatalf("ayar değiştirilemedi: %v", err)
	}

	store := reports.NewStore(pool, svc)
	if loc := store.Location(); loc != time.UTC {
		t.Errorf("saat dilimi = %v, beklenen UTC", loc)
	}
	if _, err := store.Summary(ctx, reports.Query{Days: 7}); err != nil {
		t.Fatalf("geçersiz saat diliminde rapor üretilemedi: %v", err)
	}
}

/* ── Akış tarafındaki ölçüler (spec 012) ─────────────────────────────────── */

// insertPRStep, bir akış çalışması ve içinde bir PR adımı ekler.
//
// PR adımının `runs` kaydı YOKTUR (spec 003): PR açan düğüm model çağırmıyor.
// Test bu yüzden doğrudan akış tablolarına yazıyor — raporun ikinci kaynağa
// baktığını doğrulamanın tek yolu bu.
func insertPRStep(t *testing.T, pool *pgxpool.Pool, projectID uuid.UUID,
	status string, ageDays int,
) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	created := time.Now().AddDate(0, 0, -ageDays)

	var workflowID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO workflows (project_id, name)
		VALUES ($1, 'Akış') RETURNING id`,
		projectID).Scan(&workflowID); err != nil {
		t.Fatalf("akış eklenemedi: %v", err)
	}

	var versionID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO workflow_versions (workflow_id, version, graph)
		VALUES ($1, 1, '{"nodes":[],"edges":[]}'::jsonb) RETURNING id`,
		workflowID).Scan(&versionID); err != nil {
		t.Fatalf("sürüm eklenemedi: %v", err)
	}

	// project_id çalışmanın kendi sütunu (migration 000012): akış artık farklı
	// projelerde koşabildiği için proje akıştan türetilmiyor.
	var runID uuid.UUID
	if err := pool.QueryRow(ctx, `
		INSERT INTO workflow_runs (workflow_id, project_id, version_id, version,
			status, trigger_kind, created_at)
		VALUES ($1, $2, $3, 1, 'succeeded', 'manual', $4) RETURNING id`,
		workflowID, projectID, versionID, created).Scan(&runID); err != nil {
		t.Fatalf("akış çalışması eklenemedi: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO workflow_steps (workflow_run_id, node_id, node_kind, level, status)
		VALUES ($1, 'pr', 'github.pr', 0, $2)`, runID, status); err != nil {
		t.Fatalf("adım eklenemedi: %v", err)
	}
	return runID
}

// TestSummaryPRsOpened — raporun en somut çıktısı `runs` tablosunda YOK;
// ikinci kaynaktan gelmesi gerekiyor (spec 012).
func TestSummaryPRsOpened(t *testing.T) {
	pool, store, projectID, _ := setup(t)
	testutil.Truncate(t, pool, "workflow_steps", "workflow_runs", "workflow_versions", "workflows")

	insertPRStep(t, pool, projectID, "succeeded", 1)
	insertPRStep(t, pool, projectID, "succeeded", 2)
	// Başarısız adım PR AÇMAMIŞTIR; sayılmamalı.
	insertPRStep(t, pool, projectID, "failed", 1)

	s, err := store.Summary(context.Background(), reports.Query{Days: 30})
	if err != nil {
		t.Fatalf("özet alınamadı: %v", err)
	}

	if s.Totals.PRsOpened != 2 {
		t.Fatalf("açılan PR sayısı 2 olmalı, %d geldi", s.Totals.PRsOpened)
	}

	// Günlük seri toplamı genel toplamla tutmalı: iki farklı sorgudan geliyorlar
	// ve ayrışırlarsa kullanıcı grafikle rakamın çeliştiğini görür.
	var günlük int
	for _, d := range s.Daily {
		günlük += d.PRsOpened
	}
	if günlük != s.Totals.PRsOpened {
		t.Fatalf("günlük PR toplamı (%d) genel toplamla (%d) tutmuyor", günlük, s.Totals.PRsOpened)
	}
}

// TestSummaryPRsOpenedProjeSuzgeci — PR sayımı proje süzgecine uymalı. Proje
// akışın varsayılanından değil, ÇALIŞMANIN kendi kaydından okunur
// (`workflow_runs.project_id`): aynı akış farklı projelerde koşabiliyor.
func TestSummaryPRsOpenedProjeSuzgeci(t *testing.T) {
	pool, store, projectID, _ := setup(t)
	testutil.Truncate(t, pool, "workflow_steps", "workflow_runs", "workflow_versions", "workflows")

	insertPRStep(t, pool, projectID, "succeeded", 1)

	var digerProje uuid.UUID
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO projects (name, repo_url) VALUES ('Diğer', 'https://example.com/o.git')
		RETURNING id`).Scan(&digerProje); err != nil {
		t.Fatalf("ikinci proje eklenemedi: %v", err)
	}
	insertPRStep(t, pool, digerProje, "succeeded", 1)

	s, err := store.Summary(context.Background(), reports.Query{Days: 30, ProjectID: &projectID})
	if err != nil {
		t.Fatalf("özet alınamadı: %v", err)
	}
	if s.Totals.PRsOpened != 1 {
		t.Fatalf("süzgeçli PR sayısı 1 olmalı, %d geldi", s.Totals.PRsOpened)
	}
}

// TestSummaryRunsWithCode — "çalıştı" ile "bir şey üretti" aynı şey değil.
func TestSummaryRunsWithCode(t *testing.T) {
	pool, store, projectID, agentID := setup(t)

	insertRun(t, pool, projectID, agentID, runSpec{status: "succeeded", ageDays: 1})
	insertRun(t, pool, projectID, agentID, runSpec{status: "succeeded", ageDays: 1})
	if _, err := pool.Exec(context.Background(),
		`UPDATE runs SET diff = 'diff --git a/x b/x' WHERE id = (SELECT id FROM runs LIMIT 1)`); err != nil {
		t.Fatalf("diff yazılamadı: %v", err)
	}

	s, err := store.Summary(context.Background(), reports.Query{Days: 30})
	if err != nil {
		t.Fatalf("özet alınamadı: %v", err)
	}
	if s.Totals.Runs != 2 {
		t.Fatalf("çalıştırma sayısı 2 olmalı, %d", s.Totals.Runs)
	}
	if s.Totals.RunsWithCode != 1 {
		t.Fatalf("kod üreten çalıştırma 1 olmalı, %d geldi", s.Totals.RunsWithCode)
	}
}
