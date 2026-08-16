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
	"github.com/agent-coder/backend/internal/mcp"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Akış sürümünün kaydedilmesi.
 *
 * Buradaki tek karar şu: HATA KAYDETME ANINDA mı yoksa çalışma anında mı
 * görünsün. Çalışma anına bırakılan bir hata, akışın yarısı koştuktan —
 * yani para harcandıktan — sonra duruyor ve kullanıcı sebebini tuvalde değil
 * çalıştırma kaydında arıyor.
 */

type versiyonFixture struct {
	routes http.Handler
	flows  *workflow.Store
	mcp    *mcp.Store
	proje  uuid.UUID
	agent  uuid.UUID
}

func versiyonSetup(t *testing.T) versiyonFixture {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "workflows", "runs", "projects", "agents", "mcp_servers")

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

	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(make([]byte, secrets.KeySize)))
	require.NoError(t, err)

	flows := workflow.NewStore(pool)
	servers := mcp.NewStore(pool, cipher)

	h := NewHandler(Deps{Config: cfg, Workflows: flows, MCPServers: servers})
	t.Cleanup(h.Shutdown)

	return versiyonFixture{routes: h.Routes(), flows: flows, mcp: servers,
		proje: projectID, agent: agentID}
}

func (f versiyonFixture) akis(t *testing.T) uuid.UUID {
	t.Helper()
	w, err := f.flows.Create(context.Background(),
		workflow.CreateInput{ProjectID: f.proje, Name: "Akış"})
	require.NoError(t, err)
	return w.ID
}

func (f versiyonFixture) kaydet(t *testing.T, id uuid.UUID, g workflow.Graph) *httptest.ResponseRecorder {
	t.Helper()

	raw, err := json.Marshal(map[string]any{"graph": g})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	f.routes.ServeHTTP(rec, httptest.NewRequest(http.MethodPost,
		"/api/workflows/"+id.String()+"/versions", bytes.NewReader(raw)))
	return rec
}

// agentGrafi, tetikleyici + agent adımından oluşan geçerli bir akış.
func (f versiyonFixture) agentGrafi() workflow.Graph {
	return workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "t1", Kind: workflow.KindTriggerManual},
			{ID: "a1", Kind: workflow.KindAgent, Name: "Analiz",
				Config: workflow.NodeConfig{
					AgentID: f.agent.String(), Model: "m1", Prompt: "{{ input }}"}},
		},
		Edges: []workflow.Edge{{From: "t1", To: "a1"}},
	}
}

func TestSaveVersion_GecerliGrafKaydedilir(t *testing.T) {
	f := versiyonSetup(t)
	id := f.akis(t)

	rec := f.kaydet(t, id, f.agentGrafi())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

/*
SİLİNMİŞ MCP SUNUCUSU KAYDETME ANINDA YAKALANIR.

Kayıt defteri veritabanına bakamıyor (bağımlısız olmak zorunda), o yüzden
"seçilen sunucu gerçekten var mı" sorusu burada soruluyor. Çalışma anına
bırakılsaydı akışın yarısı koştuktan sonra duracaktı — ve kullanıcı sebebini
tuvalde değil, çalıştırma kaydında arayacaktı.
*/
func TestSaveVersion_SilinmisMCPSunucusuKayittaYakalanir(t *testing.T) {
	f := versiyonSetup(t)
	id := f.akis(t)

	g := f.agentGrafi()
	g.Nodes = append(g.Nodes, workflow.Node{
		ID: "m1", Kind: workflow.KindMCPCall, Name: "Araç",
		Config: workflow.NodeConfig{MCPServerID: uuid.NewString(), ToolName: "ask"},
	})
	g.Edges = append(g.Edges, workflow.Edge{From: "a1", To: "m1"})

	rec := f.kaydet(t, id, g)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body validationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "invalid_graph", body.Error.Code)
	require.Len(t, body.Error.Problems, 1)
	require.Equal(t, "m1", body.Error.Problems[0].NodeID,
		"sorun HANGİ düğümde olduğunu söylemeli — tuval onu işaretliyor")
	require.Contains(t, body.Error.Problems[0].Message, "bulunamadı")
}

/*
SUNUCUDA OLMAYAN ARAÇ DA KAYITTA YAKALANIR.

Mesaj hem sunucuyu hem aracı adıyla söylüyor: kullanıcı tuvalde açılır listeden
seçiyor ama liste eski olabilir ve "araç yok" tek başına hangisinin yanlış
olduğunu göstermezdi.
*/
func TestSaveVersion_OlmayanAracKayittaYakalanir(t *testing.T) {
	f := versiyonSetup(t)
	id := f.akis(t)

	srv, err := f.mcp.Create(context.Background(), mcp.CreateInput{
		Name: "sentry", Transport: mcp.TransportHTTP, URL: "https://x.dev/mcp",
		Tools: []string{"list_issues"},
	})
	require.NoError(t, err)

	g := f.agentGrafi()
	g.Nodes = append(g.Nodes, workflow.Node{
		ID: "m1", Kind: workflow.KindMCPCall, Name: "Araç",
		Config: workflow.NodeConfig{MCPServerID: srv.ID.String(), ToolName: "olmayan_arac"},
	})
	g.Edges = append(g.Edges, workflow.Edge{From: "a1", To: "m1"})

	rec := f.kaydet(t, id, g)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body validationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Error.Problems, 1)
	require.Contains(t, body.Error.Problems[0].Message, "sentry")
	require.Contains(t, body.Error.Problems[0].Message, "olmayan_arac")
}

// Var olan sunucu ve araç kabul edilir.
func TestSaveVersion_VarOlanAracKabulEdilir(t *testing.T) {
	f := versiyonSetup(t)
	id := f.akis(t)

	srv, err := f.mcp.Create(context.Background(), mcp.CreateInput{
		Name: "sentry", Transport: mcp.TransportHTTP, URL: "https://x.dev/mcp",
		Tools: []string{"list_issues"},
	})
	require.NoError(t, err)

	g := f.agentGrafi()
	g.Nodes = append(g.Nodes, workflow.Node{
		ID: "m1", Kind: workflow.KindMCPCall, Name: "Araç",
		Config: workflow.NodeConfig{MCPServerID: srv.ID.String(), ToolName: "list_issues"},
	})
	g.Edges = append(g.Edges, workflow.Edge{From: "a1", To: "m1"})

	rec := f.kaydet(t, id, g)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

/*
GRAF DOĞRULAMA HATALARI DA AYNI BİÇİMDE DÖNER.

Tuval tek bir biçim okuyor: `error.problems[].nodeId`. Kayıt defterinin
bulduğu sorunlar farklı bir şekle konsaydı, aynı ekran iki ayrı hata biçimi
işlemek zorunda kalırdı.
*/
func TestSaveVersion_GrafHatasiAyniBicimdeDoner(t *testing.T) {
	f := versiyonSetup(t)
	id := f.akis(t)

	// Adımı olmayan akış: kayıt defteri reddediyor.
	rec := f.kaydet(t, id, workflow.Graph{})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body validationResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "invalid_graph", body.Error.Code)
	require.NotEmpty(t, body.Error.Problems, "sorunlar tuvale iletilmeli")
}
