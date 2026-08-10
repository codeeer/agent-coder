package workflow_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/testutil"
	"github.com/agent-coder/backend/internal/workflow"

	"github.com/agent-coder/backend/internal/paging"
)

type fixture struct {
	pool      *pgxpool.Pool
	store     *workflow.Store
	projectID uuid.UUID
	agentID   uuid.UUID
}

func setup(t *testing.T) fixture {
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

	return fixture{pool: pool, store: workflow.NewStore(pool), projectID: projectID, agentID: agentID}
}

// graph, iki adımlı geçerli bir akış üretir.
func (f fixture) graph() workflow.Graph {
	return workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "t1", Kind: workflow.KindTriggerManual},
			{ID: "a1", Kind: workflow.KindAgent, Name: "Analiz",
				Config: workflow.NodeConfig{AgentID: f.agentID.String(), Model: "m1",
					Prompt: "Görevi analiz et: {{ input }}"}},
			{ID: "a2", Kind: workflow.KindAgent, Name: "Uygula",
				Config: workflow.NodeConfig{AgentID: f.agentID.String(), Model: "m2",
					Prompt: "Analize göre uygula:\n{{ steps.a1.output }}"}},
		},
		Edges: []workflow.Edge{{From: "t1", To: "a1"}, {From: "a1", To: "a2"}},
	}
}

func (f fixture) newWorkflow(t *testing.T) workflow.Workflow {
	t.Helper()
	w, err := f.store.Create(context.Background(), workflow.CreateInput{
		ProjectID: f.projectID, Name: "Kod inceleme", Description: "deneme",
	})
	require.NoError(t, err)
	return w
}

func TestCreate_TetiklemeAdresiBirlikteUretilir(t *testing.T) {
	f := setup(t)
	w := f.newWorkflow(t)

	require.NotEmpty(t, w.HookToken, "akışla birlikte tetikleme adresi üretilmeli")
	require.Nil(t, w.ActiveVersionID, "yeni akışın henüz sürümü yok")
	require.Equal(t, "Deneme", w.ProjectName)
}

func TestSaveVersion_NumaraArtarVeEtkinOlur(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	v1, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)
	require.Equal(t, 1, v1.Version)

	v2, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)
	require.Equal(t, 2, v2.Version)

	after, err := f.store.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, v2.ID, *after.ActiveVersionID)
	require.Equal(t, 2, *after.ActiveVersion)
}

// TestSaveVersion_GecersizGrafKaydedilmez — hata en ucuz yerde, kaydetme anında.
func TestSaveVersion_GecersizGrafKaydedilmez(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	bozuk := workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "t1", Kind: workflow.KindTriggerManual},
			{ID: "a1", Kind: workflow.KindAgent, Config: workflow.NodeConfig{Prompt: "x"}},
		},
		Edges: []workflow.Edge{{From: "t1", To: "a1"}},
	}

	_, err := f.store.SaveVersion(ctx, w.ID, bozuk)
	require.Error(t, err)

	var ve *workflow.ValidationError
	require.ErrorAs(t, err, &ve)

	after, err := f.store.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Nil(t, after.ActiveVersionID, "geçersiz graf veritabanına hiç girmemeli")
}

// TestCreateRun_SurumAnlikKopyadir — akış sonradan değişse de geçmiş doğru kalmalı.
func TestCreateRun_SurumAnlikKopyadir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	v1, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v1, Trigger: workflow.TriggerManual, Input: "hata düzelt",
	})
	require.NoError(t, err)
	require.Equal(t, 1, run.Version)

	// Akış değişiyor: yeni sürüm kaydediliyor.
	_, err = f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	after, err := f.store.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, 1, after.Version, "geçmiş çalışma hâlâ 1. sürümle çalıştığını göstermeli")
	require.Equal(t, v1.ID, after.VersionID)
}

func TestCreateRun_AdimlarBastanYazilir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)

	// Tetikleyici bir adım DEĞİLDİR; yalnızca iki agent adımı olmalı.
	require.Len(t, run.Steps, 2)
	require.Equal(t, "a1", run.Steps[0].NodeID)
	require.Equal(t, "Analiz", run.Steps[0].Name)
	require.Equal(t, workflow.StepPending, run.Steps[0].Status)
	require.Less(t, run.Steps[0].Level, run.Steps[1].Level, "adımlar seviyeye göre sıralı")
}

// TestSteps_MaliyetCalistirmadanOkunur — aynı sayı iki yerde tutulmuyor.
func TestSteps_MaliyetCalistirmadanOkunur(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)

	runID := f.insertRun(t, 0.25, 1000, 200)
	require.NoError(t, f.store.LinkStepRun(ctx, run.Steps[0].ID, runID))
	require.NoError(t, f.store.FinishStep(ctx, run.Steps[0].ID, workflow.StepSucceeded, workflow.StepOutcome{}, nil))

	after, err := f.store.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, 0.25, after.Steps[0].CostUSD)
	require.Equal(t, int64(1200), after.Steps[0].Tokens)
	require.Equal(t, "coder", after.Steps[0].AgentSlug)
	require.Equal(t, 0.25, after.CostUSD, "akış toplamı adımlardan hesaplanır")
}

func TestSkipPending_KalanAdimlariIsaretler(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)

	runID := f.insertRun(t, 0, 0, 0)
	require.NoError(t, f.store.LinkStepRun(ctx, run.Steps[0].ID, runID))
	require.NoError(t, f.store.FinishStep(ctx, run.Steps[0].ID, workflow.StepFailed, workflow.StepOutcome{}, context.DeadlineExceeded))

	n, err := f.store.SkipPending(ctx, run.ID, workflow.StepSkipped)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	after, err := f.store.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, workflow.StepFailed, after.Steps[0].Status)
	require.NotNil(t, after.Steps[0].Error)
	require.Equal(t, workflow.StepSkipped, after.Steps[1].Status,
		"hata sonrası kalan adım 'sırada bekliyor' görünmemeli")
}

func TestRecoverInterrupted(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)
	require.NoError(t, f.store.MarkRunStarted(ctx, run.ID))

	n, err := f.store.RecoverInterrupted(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	after, err := f.store.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, workflow.RunInterrupted, after.Status)
	require.NotNil(t, after.FinishedAt)
	for _, st := range after.Steps {
		require.Equal(t, workflow.StepCancelled, st.Status, "yarım adım da kapanmalı")
	}

	// İkinci çağrı bir şey bulmamalı.
	n, err = f.store.RecoverInterrupted(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestHook_YenilenenAdresEskisiniGecersizKilar(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	found, err := f.store.ByHookToken(ctx, w.HookToken)
	require.NoError(t, err)
	require.Equal(t, w.ID, found.ID)

	yeni, err := f.store.RotateHook(ctx, w.ID)
	require.NoError(t, err)
	require.NotEqual(t, w.HookToken, yeni)

	_, err = f.store.ByHookToken(ctx, w.HookToken)
	require.ErrorIs(t, err, workflow.ErrNotFound, "eski adres artık çalışmamalı")

	found, err = f.store.ByHookToken(ctx, yeni)
	require.NoError(t, err)
	require.Equal(t, w.ID, found.ID)
}

func TestList_SonCalismaGosterilir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	list, _, err := f.store.List(ctx, nil, paging.Page{Limit: 100})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Nil(t, list[0].LastRun, "hiç çalışmamış akışın son çalışması yok")

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)
	require.NoError(t, f.store.FinishRun(ctx, run.ID, workflow.RunSucceeded, nil))

	list, _, err = f.store.List(ctx, nil, paging.Page{Limit: 100})
	require.NoError(t, err)
	require.NotNil(t, list[0].LastRun)
	require.Equal(t, workflow.RunSucceeded, list[0].LastRun.Status)
}

func TestActiveVersion_SurumYoksaHata(t *testing.T) {
	f := setup(t)
	w := f.newWorkflow(t)

	_, err := f.store.ActiveVersion(context.Background(), w.ID)
	require.ErrorIs(t, err, workflow.ErrNoVersion)
}

// insertRun, adıma bağlanacak gerçek bir çalıştırma kaydı ekler.
func (f fixture) insertRun(t *testing.T, cost float64, prompt, completion int) uuid.UUID {
	t.Helper()

	var id uuid.UUID
	err := f.pool.QueryRow(context.Background(), `
		INSERT INTO runs (project_id, agent_id, agent_slug, agent_prompt, provider_slug,
			model_id, branch, task, status, cost_usd, prompt_tokens, completion_tokens)
		VALUES ($1,$2,'coder','p','openrouter','m','main','iş','succeeded',$3,$4,$5)
		RETURNING id`,
		f.projectID, f.agentID, cost, prompt, completion).Scan(&id)
	require.NoError(t, err)
	return id
}
