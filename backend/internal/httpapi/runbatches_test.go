package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/config"
	"github.com/agent-coder/backend/internal/runbatch"
	"github.com/agent-coder/backend/internal/testutil"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Toplu çalıştırma uçları (spec 023 Blok 4).
 *
 * Uçların işi kuyruğu DOLDURMAK: başlatma sıraya koyar ve döner. Bu yüzden
 * burada zamanlayıcı yok — testler uçların sözleşmesini ölçüyor, kuyruğun
 * işleyişini değil (o `runbatch` paketinde ölçülüyor).
 */

type batchFixture struct {
	h          *Handler
	pool       *pgxpool.Pool
	store      *runbatch.Store
	workflowID uuid.UUID
	projects   []uuid.UUID
}

func batchHandler(t *testing.T, projeAdlari ...string) batchFixture {
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
	require.NotEmpty(t, ids, "en az bir proje gerekli")

	wfStore := workflow.NewStore(pool)
	wf, wfErr := wfStore.Create(ctx, workflow.CreateInput{
		ProjectID: ids[0], Name: "Node yükseltme"})
	require.NoError(t, wfErr)

	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	cfg, err := config.Load()
	require.NoError(t, err)

	store := runbatch.NewStore(pool)
	h := NewHandler(Deps{Config: cfg, RunBatches: store, Workflows: wfStore})
	t.Cleanup(h.Shutdown)

	return batchFixture{h: h, pool: pool, store: store, workflowID: wf.ID, projects: ids}
}

// surumKaydet, akışa etkin bir sürüm yazar — akışın "kayıtlı tanımı" olur.
func (f batchFixture) surumKaydet(t *testing.T) {
	t.Helper()

	var agentID uuid.UUID
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`INSERT INTO agents (slug, name, prompt, source) VALUES ('coder','Coder','p','builtin')
		 RETURNING id`).Scan(&agentID))

	_, err := workflow.NewStore(f.pool).SaveVersion(context.Background(), f.workflowID,
		workflow.Graph{
			Nodes: []workflow.Node{
				{ID: "t1", Kind: workflow.KindTriggerManual},
				{ID: "a1", Kind: workflow.KindAgent, Name: "Uygula",
					Config: workflow.NodeConfig{AgentID: agentID.String(), Model: "m1",
						Prompt: "{{ input }}"}},
			},
			Edges: []workflow.Edge{{From: "t1", To: "a1"}},
		})
	require.NoError(t, err)
}

func (f batchFixture) istek(t *testing.T, method, yol string, govde any) *httptest.ResponseRecorder {
	t.Helper()

	var body *bytes.Reader
	if govde != nil {
		b, err := json.Marshal(govde)
		require.NoError(t, err)
		body = bytes.NewReader(b)
	} else {
		body = bytes.NewReader(nil)
	}

	rec := httptest.NewRecorder()
	f.h.Routes().ServeHTTP(rec, httptest.NewRequest(method, yol, body))
	return rec
}

func (f batchFixture) olustur(t *testing.T, projeler ...uuid.UUID) batchResponse {
	t.Helper()

	ids := make([]string, 0, len(projeler))
	for _, p := range projeler {
		ids = append(ids, p.String())
	}
	rec := f.istek(t, http.MethodPost, "/api/run-batches", map[string]any{
		"workflowId": f.workflowID.String(),
		"task":       "Node 24'e yükselt",
		"projectIds": ids,
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var out batchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

// T30 — POST /api/run-batches → 201 ve KAÇ ÖĞE sıraya alındığı.
func TestCreateRunBatch_SirayaAlinanSayisiylaDoner(t *testing.T) {
	f := batchHandler(t, "alfa", "beta", "gama")
	f.surumKaydet(t)

	out := f.olustur(t, f.projects...)

	require.Equal(t, 3, out.Counts.Total, "kaç işin sıraya alındığı yanıtta yazar")
	require.Equal(t, 3, out.Counts.Pending)
	require.Equal(t, runbatch.StatusQueued, out.Status)
	require.Equal(t, "Node yükseltme", out.WorkflowName)
}

// T31 — GET /api/run-batches ve /{id}.
func TestGetRunBatch_SayilarVeOgeler(t *testing.T) {
	f := batchHandler(t, "alfa", "beta")
	f.surumKaydet(t)
	created := f.olustur(t, f.projects...)

	rec := f.istek(t, http.MethodGet, "/api/run-batches/"+created.ID.String(), nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var detay batchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &detay))
	require.Len(t, detay.Items, 2, "detayda öğeler döner")
	require.Equal(t, "alfa", detay.Items[0].ProjectName, "satır hangi projeye ait olduğunu gösterir")
	require.Equal(t, 2, detay.Counts.Pending)

	rec = f.istek(t, http.MethodGet, "/api/run-batches", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var liste struct {
		Items []runbatch.Batch `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &liste))
	require.Equal(t, 1, liste.Total)
	require.Equal(t, 2, liste.Items[0].Counts.Total, "liste sayıları taşır")
}

func TestGetRunBatch_OlmayanKimlik(t *testing.T) {
	f := batchHandler(t, "alfa")

	rec := f.istek(t, http.MethodGet, "/api/run-batches/"+uuid.NewString(), nil)
	require.Equal(t, http.StatusNotFound, rec.Code)

	rec = f.istek(t, http.MethodGet, "/api/run-batches/abc", nil)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

/*
T32 — iptal ve devam: KAÇ ÖĞENİN ETKİLENDİĞİ yanıtta.

"İptal edildi" yetmez; kullanıcı ne olduğunu bilmeli. Yanıt eylemden SONRAKİ
durumu da taşır ki ekran kendi tahminiyle değil sistemin söylediğiyle tazelensin.
*/
func TestCancelResumeRunBatch_EtkilenenSayisiDoner(t *testing.T) {
	f := batchHandler(t, "alfa", "beta", "gama")
	f.surumKaydet(t)
	created := f.olustur(t, f.projects...)
	ctx := context.Background()

	// İlk öğe çalışıyor; iptal ona dokunmamalı.
	_, items, err := f.store.Get(ctx, created.ID)
	require.NoError(t, err)
	require.NoError(t, f.store.Claim(ctx, items[0].ID))

	rec := f.istek(t, http.MethodPost, "/api/run-batches/"+created.ID.String()+"/cancel", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var iptal batchActionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &iptal))
	require.Equal(t, 2, iptal.Affected, "yalnızca bekleyenler düşer")
	require.Equal(t, runbatch.StatusCancelled, iptal.Status)
	require.Equal(t, 1, iptal.Counts.Running, "çalışan iş sürüyor")

	// Çalışan öğe kesildi (sunucu yeniden başladı) → devam onu sıraya alır.
	require.NoError(t, f.store.MarkFinished(ctx, items[0].ID, runbatch.ItemInterrupted, "kesildi"))

	rec = f.istek(t, http.MethodPost, "/api/run-batches/"+created.ID.String()+"/resume", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var devam batchActionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &devam))
	require.Equal(t, 1, devam.Affected, "yalnızca kesilen öğe sıraya alınır")
	require.Equal(t, runbatch.StatusQueued, devam.Status)
}

// T23'ün uç karşılığı: bitmiş bir toplu işi iptal HATA DEĞİL, durumu söylenir.
func TestCancelRunBatch_BitmisIsHataDegil(t *testing.T) {
	f := batchHandler(t, "alfa")
	f.surumKaydet(t)
	created := f.olustur(t, f.projects[0])
	ctx := context.Background()

	_, items, err := f.store.Get(ctx, created.ID)
	require.NoError(t, err)
	require.NoError(t, f.store.Claim(ctx, items[0].ID))
	require.NoError(t, f.store.MarkFinished(ctx, items[0].ID, runbatch.ItemSucceeded, ""))

	rec := f.istek(t, http.MethodPost, "/api/run-batches/"+created.ID.String()+"/cancel", nil)
	require.Equal(t, http.StatusOK, rec.Code)

	var out batchActionResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Zero(t, out.Affected)
	require.Equal(t, runbatch.StatusDone, out.Status, "durumu söylenir")
}

/*
T33 — hata durumları spec tablosuna uyar.

Hepsinin ortak kuralı: YAPILANDIRMA EKSİĞİ 5xx DÖNMEZ. Yeni kurulum yapan bir
kullanıcı "işlem tamamlanamadı" görürse uygulamanın bozuk olduğunu sanır; oysa
eksik olan bir ayar.
*/
func TestCreateRunBatch_HataDurumlari(t *testing.T) {
	f := batchHandler(t, "alfa", "beta")
	f.surumKaydet(t)

	durumlar := []struct {
		ad   string
		body map[string]any
		kod  int
		code string
	}{
		{
			ad:   "hiç proje seçilmedi",
			body: map[string]any{"workflowId": f.workflowID.String(), "projectIds": []string{}},
			kod:  http.StatusBadRequest, code: "no_projects",
		},
		{
			ad: "aynı proje iki kez",
			body: map[string]any{"workflowId": f.workflowID.String(),
				"projectIds": []string{f.projects[0].String(), f.projects[0].String()}},
			kod: http.StatusBadRequest, code: "duplicate_project",
		},
		{
			ad: "akış silinmiş",
			body: map[string]any{"workflowId": uuid.NewString(),
				"projectIds": []string{f.projects[0].String()}},
			kod: http.StatusNotFound, code: "not_found",
		},
		{
			ad: "proje silinmiş",
			body: map[string]any{"workflowId": f.workflowID.String(),
				"projectIds": []string{uuid.NewString()}},
			kod: http.StatusNotFound, code: "project_not_found",
		},
		{
			ad: "geçersiz proje kimliği",
			body: map[string]any{"workflowId": f.workflowID.String(),
				"projectIds": []string{"abc"}},
			kod: http.StatusBadRequest, code: "invalid_project",
		},
	}

	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			rec := f.istek(t, http.MethodPost, "/api/run-batches", d.body)
			require.Equal(t, d.kod, rec.Code, rec.Body.String())

			var body ErrorBody
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, d.code, body.Error.Code)
			require.NotEmpty(t, body.Error.Message, "sebebi YAZILIR")
		})
	}
}

/*
Akışın kayıtlı tanımı yoksa toplu iş HİÇ BAŞLAMAZ.

Sıraya konsaydı otuz öğe tek tek başlatılıp tek tek düşerdi: kullanıcı otuz
satırlık bir başarısızlık listesi görür, sebebi ancak satırların içinde okurdu.
Ve bu 5xx değil, ne yapılacağını söyleyen bir 4xx.
*/
func TestCreateRunBatch_TanimsizAkisSirayaKonmaz(t *testing.T) {
	f := batchHandler(t, "alfa")

	rec := f.istek(t, http.MethodPost, "/api/run-batches", map[string]any{
		"workflowId": f.workflowID.String(),
		"projectIds": []string{f.projects[0].String()},
	})
	require.Equal(t, http.StatusConflict, rec.Code)

	var body ErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "no_version", body.Error.Code)
	require.Contains(t, body.Error.Message, "adımları kaydedin",
		"mesaj ne yapılacağını söylemeli")

	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM run_batches`).Scan(&n))
	require.Zero(t, n, "başlatılamayan toplu iş kayıt bırakmamalı")
}
