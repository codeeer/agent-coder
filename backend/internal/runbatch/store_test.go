package runbatch_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runbatch"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * Toplu çalıştırma deposu (spec 023 Blok 1).
 *
 * Buradaki testlerin çoğu SIRA ve SÜZGEÇ üzerine: kuyruğun doğru öğeyi doğru
 * anda vermesi, zamanlayıcının tek bilgi kaynağı. Yanlış öğeyi veren bir sorgu
 * ancak "neden bu proje hiç çalışmadı" sorusuyla, saatler sonra fark edilirdi.
 */

type fixture struct {
	pool       *pgxpool.Pool
	store      *runbatch.Store
	workflowID uuid.UUID
	projects   []uuid.UUID
}

func setup(t *testing.T, projeAdlari ...string) fixture {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "run_batch_items", "run_batches",
		"workflows", "runs", "projects", "agents")

	ctx := context.Background()

	ids := make([]uuid.UUID, 0, len(projeAdlari))
	for _, ad := range projeAdlari {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO projects (name, repo_url) VALUES ($1, $2) RETURNING id`,
			ad, "https://example.com/"+ad+".git").Scan(&id))
		ids = append(ids, id)
	}

	var projectID uuid.UUID
	if len(ids) > 0 {
		projectID = ids[0]
	} else {
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO projects (name, repo_url) VALUES ('Varsayılan','https://example.com/v.git')
			 RETURNING id`).Scan(&projectID))
	}

	var workflowID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO workflows (project_id, name) VALUES ($1, 'Node yükseltme')
		RETURNING id`, projectID).Scan(&workflowID))

	return fixture{pool: pool, store: runbatch.NewStore(pool),
		workflowID: workflowID, projects: ids}
}

func (f fixture) create(t *testing.T, projeler ...uuid.UUID) runbatch.Batch {
	t.Helper()
	b, err := f.store.Create(context.Background(), f.workflowID, "Node 24'e yükselt", projeler)
	require.NoError(t, err)
	return b
}

// T02 — sıra EKLENME sırasıdır: gönderilen dizi position 0..n-1 olur.
func TestCreate_OgelerSirayla(t *testing.T) {
	f := setup(t, "alfa", "beta", "gama")
	b := f.create(t, f.projects[2], f.projects[0], f.projects[1])

	_, items, err := f.store.Get(context.Background(), b.ID)
	require.NoError(t, err)
	require.Len(t, items, 3)

	for i, it := range items {
		require.Equal(t, i, it.Position, "position eklenme sırasını izlemeli")
		require.Equal(t, runbatch.ItemPending, it.Status)
	}
	require.Equal(t, []string{"gama", "alfa", "beta"},
		[]string{items[0].ProjectName, items[1].ProjectName, items[2].ProjectName},
		"sıra kullanıcının seçtiği sıradır, alfabetik değil")
}

// T03 — aynı proje aynı toplu işte iki kez yer alamaz. Kural veritabanında:
// seçim listesi bir gün başka bir ekrandan da beslenebilir.
func TestCreate_AyniProjeIkiKezReddedilir(t *testing.T) {
	f := setup(t, "alfa", "beta")

	_, err := f.store.Create(context.Background(), f.workflowID, "görev",
		[]uuid.UUID{f.projects[0], f.projects[1], f.projects[0]})

	require.ErrorIs(t, err, runbatch.ErrDuplicateProject)

	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM run_batches`).Scan(&n))
	require.Zero(t, n, "reddedilen toplu iş yarım kaydedilmemeli")
}

func TestCreate_HicProjeSecilmedi(t *testing.T) {
	f := setup(t)
	_, err := f.store.Create(context.Background(), f.workflowID, "görev", nil)
	require.ErrorIs(t, err, runbatch.ErrNoProjects)
}

func TestCreate_AkisYoksaBaslatilamaz(t *testing.T) {
	f := setup(t, "alfa")
	_, err := f.store.Create(context.Background(), uuid.New(), "görev",
		[]uuid.UUID{f.projects[0]})
	require.ErrorIs(t, err, runbatch.ErrWorkflowNotFound)
}

// T04 — sıradaki bekleyen EN KÜÇÜK position'lı olandır.
func TestNextPending_EnKucukPosition(t *testing.T) {
	f := setup(t, "alfa", "beta", "gama")
	f.create(t, f.projects[1], f.projects[2], f.projects[0])

	it, ok, err := f.store.NextPending(context.Background())
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 0, it.Position)
	require.Equal(t, "beta", it.ProjectName)
}

// Bir öğe çalışmaya geçince sıradaki BİR SONRAKİ bekleyendir — aynı öğe iki kez
// verilmez, yoksa sınır kadar öğe aynı projeyi çalıştırırdı.
func TestNextPending_CalisanOgeyiTekrarVermez(t *testing.T) {
	f := setup(t, "alfa", "beta")
	f.create(t, f.projects[0], f.projects[1])
	ctx := context.Background()

	ilk, ok, err := f.store.NextPending(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, f.store.MarkRunning(ctx, ilk.ID, f.workflowRun(t, ilk.ProjectID)))

	ikinci, ok, err := f.store.NextPending(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, ilk.ID, ikinci.ID)
	require.Equal(t, 1, ikinci.Position)
}

// T05 — iptal edilmiş toplu işin öğesi kuyruktan GELMEZ.
//
// Süzgeç toplu iş durumunda; olmasaydı iptalden sonra kuyrukta duran bir
// bekleyen, kullanıcının vazgeçtiği işi başlatırdı.
func TestNextPending_IptalEdilenTopluIsinOgesiGelmez(t *testing.T) {
	f := setup(t, "alfa", "beta")
	ctx := context.Background()
	b := f.create(t, f.projects[0], f.projects[1])

	dusen, err := f.store.Cancel(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, 2, dusen)

	_, ok, err := f.store.NextPending(ctx)
	require.NoError(t, err)
	require.False(t, ok, "iptal edilmiş toplu iştan öğe verilmemeli")
}

// T06 — Get toplu işi öğeleriyle ve PROJE ADLARIYLA döner (JOIN).
func TestGet_OgelerVeProjeAdlari(t *testing.T) {
	f := setup(t, "alfa", "beta")
	b := f.create(t, f.projects[0], f.projects[1])

	got, items, err := f.store.Get(context.Background(), b.ID)
	require.NoError(t, err)

	require.Equal(t, b.ID, got.ID)
	require.Equal(t, "Node yükseltme", got.WorkflowName)
	require.Equal(t, "Node 24'e yükselt", got.Task)
	require.Equal(t, runbatch.StatusQueued, got.Status)

	require.Len(t, items, 2)
	require.Equal(t, "alfa", items[0].ProjectName)
	require.Equal(t, "beta", items[1].ProjectName)
	require.Nil(t, items[0].WorkflowRunID, "başlatılmamış öğenin çalışması yok")
}

func TestGet_OlmayanTopluIs(t *testing.T) {
	f := setup(t)
	_, _, err := f.store.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, runbatch.ErrNotFound)
}

// T07 — sayılar doğru: bekleyen · çalışan · tamamlanan · başarısız · kesilen.
//
// Ekranın tek bilgi kaynağı bu sayılar; yanlış olsalar kullanıcı otuz öğeyi tek
// tek saymak zorunda kalırdı.
func TestSayilar_HerDurumKendiKovasinda(t *testing.T) {
	f := setup(t, "alfa", "beta", "gama", "delta", "epsilon")
	ctx := context.Background()
	b := f.create(t, f.projects...)

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)

	// 0: çalışıyor · 1: tamamlandı · 2: başarısız · 3: kesildi · 4: bekliyor
	require.NoError(t, f.store.MarkRunning(ctx, items[0].ID, f.workflowRun(t, items[0].ProjectID)))
	require.NoError(t, f.store.MarkRunning(ctx, items[1].ID, f.workflowRun(t, items[1].ProjectID)))
	require.NoError(t, f.store.MarkFinished(ctx, items[1].ID, runbatch.ItemSucceeded, ""))
	require.NoError(t, f.store.MarkRunning(ctx, items[2].ID, f.workflowRun(t, items[2].ProjectID)))
	require.NoError(t, f.store.MarkFinished(ctx, items[2].ID, runbatch.ItemFailed, "derleme hatası"))
	require.NoError(t, f.store.MarkRunning(ctx, items[3].ID, f.workflowRun(t, items[3].ProjectID)))
	require.NoError(t, f.store.MarkFinished(ctx, items[3].ID, runbatch.ItemInterrupted, "kesildi"))

	got, _, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)

	require.Equal(t, runbatch.Counts{
		Total: 5, Pending: 1, Running: 1, Succeeded: 1, Failed: 1, Interrupted: 1,
	}, got.Counts)
	require.Equal(t, runbatch.StatusRunning, got.Status,
		"çalışan öğesi olan toplu iş 'running' görünür")
}

// Sayılar listede de doğru olmalı: liste ekranı öğe listesini hiç çekmiyor.
func TestList_SayilarlaBirlikte(t *testing.T) {
	f := setup(t, "alfa", "beta")
	ctx := context.Background()
	f.create(t, f.projects[0])
	f.create(t, f.projects[0], f.projects[1])

	list, total, err := f.store.List(ctx, 25, 0)
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, list, 2)

	// En yeni önce.
	require.Equal(t, 2, list[0].Counts.Total)
	require.Equal(t, 2, list[0].Counts.Pending)
	require.Equal(t, 1, list[1].Counts.Total)
}

// workflowRun, öğenin bağlanacağı bir akış çalışması üretir.
//
// Gerçek bir kayıt gerekiyor: `workflow_run_id` yabancı anahtar ve uydurma bir
// UUID yazmak veritabanı tarafından reddedilir — testin de bunu bilmesi iyi.
func (f fixture) workflowRun(t *testing.T, projectID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var versionID uuid.UUID
	err := f.pool.QueryRow(ctx, `
		SELECT id FROM workflow_versions WHERE workflow_id = $1 ORDER BY version DESC LIMIT 1`,
		f.workflowID).Scan(&versionID)
	if err != nil {
		require.NoError(t, f.pool.QueryRow(ctx, `
			INSERT INTO workflow_versions (workflow_id, version, graph)
			VALUES ($1, 1, '{}'::jsonb) RETURNING id`, f.workflowID).Scan(&versionID))
	}

	var runID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx, `
		INSERT INTO workflow_runs (workflow_id, project_id, version_id, version, trigger_kind)
		VALUES ($1, $2, $3, 1, 'manual') RETURNING id`,
		f.workflowID, projectID, versionID).Scan(&runID))
	return runID
}
