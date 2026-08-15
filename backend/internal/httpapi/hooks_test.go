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
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/config"
	"github.com/agent-coder/backend/internal/testutil"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Tetikleme adresi — ürünün KİMLİK DOĞRULAMASIZ tek kapısı.
 *
 * Adresin kendisi anahtar (spec 007 S3). Buradaki her karar doğrudan güvenlik
 * kararı: yanlış bir cevap ya kimin neye eriştiğini sızdırır ya da dışarıdan
 * gelen bir isteğe olmaması gereken bir iş yaptırır.
 */

// sessizOtobus, akış olaylarını yutan yayıncı.
type sessizOtobus struct{}

func (sessizOtobus) Publish(uuid.UUID, string, string) {}

type hookFixture struct {
	routes http.Handler
	store  *workflow.Store
	proje  uuid.UUID
	agent  uuid.UUID
}

func hookSetup(t *testing.T) hookFixture {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "workflows", "runs", "projects", "agents")

	ctx := context.Background()
	var projectID, agentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, repo_url) VALUES ('Deneme','https://example.com/r.git')
		 RETURNING id`).Scan(&projectID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO agents (slug, name, prompt, source) VALUES ('coder','Coder','p','builtin')
		 RETURNING id`).Scan(&agentID))

	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	cfg, err := config.Load()
	require.NoError(t, err)

	store := workflow.NewStore(pool)
	// Handler kümesi BOŞ: akış başlıyor ama ilk adımda duruyor. Tetikleme yolu
	// gerçek, container açılmıyor.
	exec := workflow.NewExecutor(store, workflow.Handlers{}, sessizOtobus{})

	h := NewHandler(Deps{
		Config: cfg, Workflows: store, WorkflowExec: exec,
		Launcher: workflow.NewLauncher(store, exec),
	})
	t.Cleanup(h.Shutdown)

	return hookFixture{routes: h.Routes(), store: store, proje: projectID, agent: agentID}
}

// akis, istenen durumda kaydedilmiş bir akış üretir ve tetikleme jetonunu döner.
func (f hookFixture) akis(t *testing.T, etkin bool) workflow.Workflow {
	t.Helper()
	ctx := context.Background()

	w, err := f.store.Create(ctx, workflow.CreateInput{ProjectID: f.proje, Name: "Akış"})
	require.NoError(t, err)

	_, err = f.store.SaveVersion(ctx, w.ID, workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "t1", Kind: workflow.KindTriggerManual},
			{ID: "a1", Kind: workflow.KindAgent, Name: "Analiz",
				Config: workflow.NodeConfig{
					AgentID: f.agent.String(), Model: "m1", Prompt: "{{ input }}"}},
		},
		Edges: []workflow.Edge{{From: "t1", To: "a1"}},
	})
	require.NoError(t, err)

	_, err = f.store.Update(ctx, w.ID, workflow.UpdateInput{Name: "Akış", IsActive: etkin})
	require.NoError(t, err)

	w, err = f.store.Get(ctx, w.ID)
	require.NoError(t, err)
	require.NotEmpty(t, w.HookToken, "ön koşul: tetikleme adresi üretilmiş olmalı")
	return w
}

func (f hookFixture) tetikle(t *testing.T, token string, govde any) *httptest.ResponseRecorder {
	t.Helper()

	var okuyucu *bytes.Reader
	switch v := govde.(type) {
	case nil:
		okuyucu = bytes.NewReader(nil)
	case string:
		okuyucu = bytes.NewReader([]byte(v))
	default:
		raw, err := json.Marshal(v)
		require.NoError(t, err)
		okuyucu = bytes.NewReader(raw)
	}

	rec := httptest.NewRecorder()
	f.routes.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/hooks/"+token, okuyucu))
	return rec
}

/*
YANLIŞ ADRES, HANGİ AKIŞLARIN VAR OLDUĞUNU SIZDIRMAZ.

Var olmayan ve yanlış adres AYNI cevabı alıyor. Farklı cevap verilseydi,
dışarıdan biri jeton deneyerek hangi akışların var olduğunu haritalayabilirdi.
*/
func TestTriggerHook_YanlisAdres404(t *testing.T) {
	f := hookSetup(t)
	f.akis(t, true) // gerçek bir akış var ama jetonu kullanılmıyor

	rec := f.tetikle(t, "tamamen-uydurma-jeton", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)

	var body ErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "not_found", body.Error.Code)
	require.NotContains(t, body.Error.Message, "akış",
		"mesaj bir akışın var olup olmadığını ima etmemeli")
}

/*
PASİF AKIŞ DIŞARIDAN BAŞLATILAMAZ.

Jeton doğru olsa bile. Kullanıcı akışı duraklattığında dış tetikleyicilerin de
durması gerekir — yoksa "duraklattım" demek yalnızca arayüzde bir görüntü olur.
*/
func TestTriggerHook_PasifAkis409(t *testing.T) {
	f := hookSetup(t)
	w := f.akis(t, false)

	rec := f.tetikle(t, w.HookToken, nil)
	require.Equal(t, http.StatusConflict, rec.Code)

	var body ErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "inactive", body.Error.Code)
}

func TestTriggerHook_GecerliAdresCalismaBaslatir(t *testing.T) {
	f := hookSetup(t)
	w := f.akis(t, true)

	rec := f.tetikle(t, w.HookToken, map[string]any{"input": "Node 24'e yükselt"})
	require.Equal(t, http.StatusAccepted, rec.Code)

	var body struct {
		WorkflowRunID uuid.UUID `json:"workflowRunId"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotEqual(t, uuid.Nil, body.WorkflowRunID)

	run, err := f.store.GetRun(context.Background(), body.WorkflowRunID)
	require.NoError(t, err)
	require.Equal(t, workflow.TriggerWebhook, run.TriggerKind)
	require.Equal(t, "Node 24'e yükselt", run.Input, "`input` alanı görev metni olur")
}

/*
GÖVDEDEKİ DÜZ METİN ALANLAR ŞABLONA GEÇER — DİĞERLERİ GEÇMEZ.

Alanlar `{{ trigger.<alan> }}` ile okunuyor ve şablon yalnızca metinle
çalışıyor. Sayı ya da nesne olduğu gibi taşınsaydı şablon çözümlemesi
çalıştırma ortasında, agent'a talimat yazılırken patlardı.
*/
func TestTriggerHook_YalnizcaMetinAlanlarPayloadaGirer(t *testing.T) {
	f := hookSetup(t)
	w := f.akis(t, true)

	rec := f.tetikle(t, w.HookToken, map[string]any{
		"key":    "SCRUM-1",
		"sayi":   42,
		"nesne":  map[string]any{"a": 1},
		"dizi":   []any{1, 2},
		"mantik": true,
	})
	require.Equal(t, http.StatusAccepted, rec.Code)

	var body struct {
		WorkflowRunID uuid.UUID `json:"workflowRunId"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))

	run, err := f.store.GetRun(context.Background(), body.WorkflowRunID)
	require.NoError(t, err)
	require.Equal(t, "SCRUM-1", run.TriggerPayload["key"])
	require.NotContains(t, run.TriggerPayload, "sayi")
	require.NotContains(t, run.TriggerPayload, "nesne")
	require.NotContains(t, run.TriggerPayload, "dizi")
	require.NotContains(t, run.TriggerPayload, "mantik")
}

/*
BOZUK GÖVDE İSTEĞİ DÜŞÜRMEZ.

Gövde SERBEST BİÇİMLİ: tetikleyen sistemin ne göndereceği bilinmiyor. Gövde
yüzünden reddetmek, kendi biçimini gönderen her sistemi kapıda durdurmak
olurdu. Ayrıştırılamayan gövdede payload boş kalır ve akış yine başlar.
*/
func TestTriggerHook_BozukGovdeCalismayiEngellemez(t *testing.T) {
	f := hookSetup(t)
	w := f.akis(t, true)

	rec := f.tetikle(t, w.HookToken, "{bu json değil")
	require.Equal(t, http.StatusAccepted, rec.Code, "serbest biçimli gövde isteği düşürmemeli")
}

// Gövdesiz çağrı da geçerli: bazı sistemler yalnızca "oldu" sinyali gönderiyor.
func TestTriggerHook_BosGovdeCalismaBaslatir(t *testing.T) {
	f := hookSetup(t)
	w := f.akis(t, true)

	rec := f.tetikle(t, w.HookToken, nil)
	require.Equal(t, http.StatusAccepted, rec.Code)
}

/*
JETON DEĞİŞİNCE ESKİ ADRES ÇALIŞMAZ.

Jeton yenileme, adresi sızmış bir kullanıcının elindeki tek araç. Eski adres
çalışmaya devam etseydi yenileme hiçbir işe yaramazdı.
*/
func TestTriggerHook_YenilenenJetondaEskiAdresCalismaz(t *testing.T) {
	f := hookSetup(t)
	w := f.akis(t, true)
	eski := w.HookToken

	yeni, err := f.store.RotateHook(context.Background(), w.ID)
	require.NoError(t, err)
	require.NotEqual(t, eski, yeni)

	require.Equal(t, http.StatusNotFound, f.tetikle(t, eski, nil).Code,
		"eski adres hâlâ çalışıyor")
	require.Equal(t, http.StatusAccepted, f.tetikle(t, yeni, nil).Code)
}
