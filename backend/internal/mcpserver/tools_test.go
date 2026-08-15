package mcpserver

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/testutil"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Dışarıya açılan üç araç.
 *
 * Bu paketin çağıranı bir insan değil, BAŞKA BİR AGENT: Claude Desktop, Cursor
 * ya da bir betik. Yanlış bir cevap kullanıcıya sorulmadan uygulanıyor — bu
 * yüzden aracın söylediği şeyle yaptığı şey aynı olmak zorunda.
 */

// sessizYayin, akış olaylarını yutan yayıncı.
type sessizYayin struct{}

func (sessizYayin) Publish(uuid.UUID, string, string) {}

type araçFixture struct {
	s     *Server
	store *workflow.Store
	proje uuid.UUID
	agent uuid.UUID
}

func araçSetup(t *testing.T) araçFixture {
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

	store := workflow.NewStore(pool)
	// Handler kümesi BOŞ: başlatılan akış ilk adımda "bu türü çalıştıramıyorum"
	// deyip duruyor. Başlatma yolu gerçek, container açılmıyor.
	launcher := workflow.NewLauncher(store,
		workflow.NewExecutor(store, workflow.Handlers{}, sessizYayin{}))

	return araçFixture{s: New(store, launcher, "test"), store: store,
		proje: projectID, agent: agentID}
}

/*
akis, istenen durumda bir akış kurar.

surumVar false ise akışın kaydedilmiş tanımı yoktur; etkin false ise
duraklatılmıştır. İkisi listeleme aracının ayırt etmesi gereken iki ayrı
sebep.
*/
func (f araçFixture) akis(t *testing.T, ad string, surumVar, etkin bool) workflow.Workflow {
	t.Helper()
	ctx := context.Background()

	w, err := f.store.Create(ctx, workflow.CreateInput{
		ProjectID: f.proje, Name: ad, Description: ad + " açıklaması",
	})
	require.NoError(t, err)

	if surumVar {
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
	}

	_, err = f.store.Update(ctx, w.ID, workflow.UpdateInput{Name: ad, IsActive: etkin})
	require.NoError(t, err)

	w, err = f.store.Get(ctx, w.ID)
	require.NoError(t, err)
	return w
}

// ── akislari_listele ────────────────────────────────────────────────────────

/*
ÇALIŞTIRILAMAYAN AKIŞIN SEBEBİ YAZILIR.

Çağıran bir agent. "Runnable: false" tek başına ona ne yapacağını söylemez;
sebep yazılmazsa kullanıcıya "akış çalışmadı" diye döner ve neden çalışmadığı
kimsenin göremediği bir yerde kalır. İki sebep AYRI: biri kullanıcının akışı
hiç kaydetmemiş olması, diğeri bilerek duraklatmış olması.
*/
func TestListWorkflows_CalistirilamamaSebebiAyirtEdilir(t *testing.T) {
	f := araçSetup(t)
	f.akis(t, "Tanımsız", false, true)
	f.akis(t, "Duraklatılmış", true, false)
	f.akis(t, "Hazır", true, true)

	_, out, err := f.s.listWorkflows(context.Background(), nil, listInput{})
	require.NoError(t, err)
	require.Len(t, out.Workflows, 3)

	sebep := map[string]workflowInfo{}
	for _, w := range out.Workflows {
		sebep[w.Name] = w
	}

	require.False(t, sebep["Tanımsız"].Runnable)
	require.Contains(t, sebep["Tanımsız"].Reason, "tanımı yok")

	require.False(t, sebep["Duraklatılmış"].Runnable)
	require.Contains(t, sebep["Duraklatılmış"].Reason, "duraklat")

	require.True(t, sebep["Hazır"].Runnable)
	require.Empty(t, sebep["Hazır"].Reason, "çalıştırılabilir akışta sebep alanı boş kalmalı")
}

// `onlyRunnable` istendiğinde çalıştırılamayanlar HİÇ dönmez: çağıranı
// başlatamayacağı bir kimliğe yönlendirmek olurdu.
func TestListWorkflows_OnlyRunnableSuzer(t *testing.T) {
	f := araçSetup(t)
	f.akis(t, "Tanımsız", false, true)
	f.akis(t, "Duraklatılmış", true, false)
	f.akis(t, "Hazır", true, true)

	_, out, err := f.s.listWorkflows(context.Background(), nil, listInput{OnlyRunnable: true})
	require.NoError(t, err)
	require.Len(t, out.Workflows, 1)
	require.Equal(t, "Hazır", out.Workflows[0].Name)
}

/*
HİÇ AKIŞ YOKKEN BOŞ DİZİ DÖNER, null DEĞİL.

JSON'da `null` ile `[]` aynı şey değil: birçok istemci `null` üzerinde döngü
kurmaya çalışıp hata veriyor. Boş bir kurulumda aracın ilk çağrısı bu.
*/
func TestListWorkflows_HicAkisYokkenBosDizi(t *testing.T) {
	f := araçSetup(t)

	_, out, err := f.s.listWorkflows(context.Background(), nil, listInput{})
	require.NoError(t, err)
	require.NotNil(t, out.Workflows, "null değil boş dizi dönmeli")
	require.Empty(t, out.Workflows)
}

// ── akis_calistir ───────────────────────────────────────────────────────────

/*
GEÇERSİZ KİMLİK NE YAPILACAĞINI SÖYLER.

Çağıran bir agent ve elindeki tek düzeltme yolu hata metni. "geçersiz uuid"
demek onu bir sonraki denemede de aynı yere götürürdü.
*/
func TestRunWorkflow_GecersizKimlikYolGosterir(t *testing.T) {
	f := araçSetup(t)

	_, _, err := f.s.runWorkflow(context.Background(), nil, runInput{WorkflowID: "abc"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "akislari_listele")
}

// Çalıştırılabilir akış başlar ve çağırana NE YAPACAĞI söylenir: akış arka
// planda sürüyor, sonucu ayrı bir araçla sorulacak.
func TestRunWorkflow_BaslatirVeSonrakiAdimiSoyler(t *testing.T) {
	f := araçSetup(t)
	w := f.akis(t, "Hazır", true, true)

	_, out, err := f.s.runWorkflow(context.Background(), nil,
		runInput{WorkflowID: w.ID.String(), Input: "Node 24'e yükselt"})
	require.NoError(t, err)
	require.NotEmpty(t, out.RunID)
	require.Contains(t, out.Note, "calisma_durumu")
}

// Tanımı olmayan akış başlatılamaz — bu kontrol `Launcher`'da.
func TestRunWorkflow_TanimsizAkisBaslatilamaz(t *testing.T) {
	f := araçSetup(t)
	w := f.akis(t, "Tanımsız", false, true)

	_, _, err := f.s.runWorkflow(context.Background(), nil, runInput{WorkflowID: w.ID.String()})
	require.Error(t, err)
}

/*
DURAKLATILMIŞ AKIŞ MCP'DEN DE BAŞLATILAMAMALI.

Aynı sunucu `akislari_listele` çağrısında bu akış için "çalıştırılamaz —
duraklatıldı" diyor. Kimliği doğrudan verilince yine de başlatıyorsa, sunucu
kendi söylediğinin tersini yapıyor demektir.

Dışarıdan gelen diğer yolların HEPSİ bunu engelliyor: webhook ve tetikleme
adresi 409 "akış pasif durumda" döndürüyor, Jira taraması duraklatılmış akışı
hiç okumuyor. Elle başlatma ve toplu çalıştırma engellemiyor ama onlar
kullanıcının o an verdiği bir karar; MCP çağıranı ise bir agent.
*/
func TestRunWorkflow_DuraklatilmisAkisBaslatilamaz(t *testing.T) {
	f := araçSetup(t)
	w := f.akis(t, "Duraklatılmış", true, false)

	_, _, err := f.s.runWorkflow(context.Background(), nil, runInput{WorkflowID: w.ID.String()})
	require.Error(t, err, "duraklatılmış akış dış bir çağıran tarafından başlatılamamalı")
}
