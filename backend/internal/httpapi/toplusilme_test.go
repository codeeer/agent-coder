package httpapi

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/config"
	"github.com/agent-coder/backend/internal/runbatch"
	"github.com/agent-coder/backend/internal/testutil"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Toplu çalıştırma ve akış çalışması silme.
 *
 * Geçmiş birikiyordu ve arayüz bunu ÜSTELİK YANLIŞ ANLATIYORDU: çalıştırmalar
 * ekranında toplu işin adımına ait satırda çöp kutusu görünüyor ama tıklanmıyor,
 * ipucu "akış çalışmasını silin" diyordu — oysa ne akış çalışması ekranında ne
 * de toplu iş ekranında silme vardı. Kullanıcı yapılması imkânsız bir eyleme
 * yönlendiriliyordu.
 *
 * Buradaki testler UÇLARIN VARLIĞINI ölçüyor; kaskadın kendisi `workflow` ve
 * `runbatch` paketlerinde ayrıca sınanıyor.
 */

func silmeSetup(t *testing.T) (http.Handler, *workflow.Store, *runbatch.Store, uuid.UUID, uuid.UUID) {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "run_batch_items", "run_batches",
		"workflows", "runs", "projects", "agents")

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

	flows := workflow.NewStore(pool)
	batches := runbatch.NewStore(pool)

	wf, err := flows.Create(ctx, workflow.CreateInput{ProjectID: projectID, Name: "Akış"})
	require.NoError(t, err)
	_, err = flows.SaveVersion(ctx, wf.ID, workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "t1", Kind: workflow.KindTriggerManual},
			{ID: "a1", Kind: workflow.KindAgent, Name: "Analiz",
				Config: workflow.NodeConfig{AgentID: agentID.String(), Model: "m1", Prompt: "{{ input }}"}},
		},
		Edges: []workflow.Edge{{From: "t1", To: "a1"}},
	})
	require.NoError(t, err)

	h := NewHandler(Deps{Config: cfg, Workflows: flows, RunBatches: batches})
	t.Cleanup(h.Shutdown)

	return h.Routes(), flows, batches, wf.ID, projectID
}

/*
BİTMİŞ TOPLU İŞ SİLİNEBİLMELİ.

Otuz projelik bir kampanya bittikten sonra kayıt listede kalıyor ve
kaldırılamıyor. Kullanıcının biriken kuyruğu temizlemesinin hiçbir yolu yok.
*/
func TestSilme_BitmisTopluIsSilinebilir(t *testing.T) {
	routes, _, batches, wfID, projectID := silmeSetup(t)
	ctx := context.Background()

	b, err := batches.Create(ctx, wfID, "Node 24'e yükselt", []uuid.UUID{projectID})
	require.NoError(t, err)
	_, err = batches.Cancel(ctx, b.ID) // bitmiş sayılsın
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/run-batches/"+b.ID.String(), nil))

	require.Less(t, rec.Code, 300, "toplu iş silinebilmeli, gelen: %d %s", rec.Code, rec.Body.String())

	_, _, err = batches.Get(ctx, b.ID)
	require.Error(t, err, "kayıt gerçekten gitmeli")
}

/*
SÜREN TOPLU İŞ SİLİNEMEZ.

Kuyruk hâlâ iş başlatıyor; kaydı silmek çalışan container'ları sahipsiz
bırakırdı. Önce iptal edilmeli.
*/
func TestSilme_SurenTopluIsSilinemez(t *testing.T) {
	routes, _, batches, wfID, projectID := silmeSetup(t)
	ctx := context.Background()

	b, err := batches.Create(ctx, wfID, "iş", []uuid.UUID{projectID})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/run-batches/"+b.ID.String(), nil))

	require.Equal(t, http.StatusConflict, rec.Code,
		"süren kuyruk silinememeli — önce iptal edilmeli")
}

/*
AKIŞ ÇALIŞMASI SİLİNEBİLMELİ.

Çalıştırmalar ekranı, akış adımına ait satırda kullanıcıyı buraya yönlendiriyor
("akış çalışmasını silin"). Bu uç yoksa o ipucu yalan söylüyor demektir.
*/
func TestSilme_AkisCalismasiSilinebilir(t *testing.T) {
	routes, flows, _, wfID, _ := silmeSetup(t)
	ctx := context.Background()

	// Gerçek başlatma yolundan geçiliyor; handler kümesi boş olduğu için
	// çalışma ilk adımda duruyor ve container açılmıyor.
	exec := workflow.NewExecutor(flows, workflow.Handlers{}, sessizOtobus{})
	run, err := workflow.NewLauncher(flows, exec).Launch(ctx, workflow.LaunchInput{
		WorkflowID: wfID, Trigger: workflow.TriggerManual, Input: "iş",
	})
	require.NoError(t, err)
	require.NoError(t, flows.FinishRun(ctx, run.ID, workflow.RunSucceeded, nil))

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/workflow-runs/"+run.ID.String(), nil))

	require.Less(t, rec.Code, 300, "akış çalışması silinebilmeli, gelen: %d", rec.Code)

	_, err = flows.GetRun(ctx, run.ID)
	require.ErrorIs(t, err, workflow.ErrNotFound, "silinen çalışma hâlâ okunuyor")
}

// Süren akış çalışması silinemez: canlı bir container'ı sahipsiz bırakırdı.
func TestSilme_SurenAkisCalismasiSilinemez(t *testing.T) {
	routes, flows, _, wfID, _ := silmeSetup(t)
	ctx := context.Background()

	exec := workflow.NewExecutor(flows, workflow.Handlers{}, sessizOtobus{})
	run, err := workflow.NewLauncher(flows, exec).Launch(ctx, workflow.LaunchInput{
		WorkflowID: wfID, Trigger: workflow.TriggerManual, Input: "iş",
	})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	routes.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/workflow-runs/"+run.ID.String(), nil))

	require.Equal(t, http.StatusConflict, rec.Code, "süren çalışma silinememeli")
}
