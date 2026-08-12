package workflow_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/workflow"
)

// ikinciProje, aynı akışın koşabileceği başka bir proje üretir.
func (f fixture) ikinciProje(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`INSERT INTO projects (name, repo_url)
		 VALUES ('İkinci', 'https://example.com/iki.git') RETURNING id`).Scan(&id))
	return id
}

// TestCreateRun_ProjeVerilmezseAkisinVarsayilani — tetikleyicilerin yolu bu.
func TestCreateRun_ProjeVerilmezseAkisinVarsayilani(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)
	require.Equal(t, f.projectID, run.ProjectID)
	require.Equal(t, "Deneme", run.ProjectName)
}

// TestCreateRun_ProjeKosudaDegistirilebilir — bir akış, yirmi projede
// koşabilsin diye (spec 007, 2026-08-12 kararı).
func TestCreateRun_ProjeKosudaDegistirilebilir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)
	digeri := f.ikinciProje(t)

	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
		ProjectID: digeri,
	})
	require.NoError(t, err)
	require.Equal(t, digeri, run.ProjectID)
	require.Equal(t, "İkinci", run.ProjectName)

	// Akışın kendi varsayılanı değişmedi.
	tekrar, err := f.store.Get(ctx, w.ID)
	require.NoError(t, err)
	require.Equal(t, f.projectID, tekrar.ProjectID)
}

// TestCreateRun_ProjeAnlikKopyadir — asıl sebep bu.
//
// Proje akıştan JOIN ile okunsaydı, akışın varsayılanı sonradan
// değiştirildiğinde GEÇMİŞ çalışmaların projesi de geriye dönük değişirdi;
// "hangi projede koştu?" sorusunun cevabı bozulurdu.
func TestCreateRun_ProjeAnlikKopyadir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)
	digeri := f.ikinciProje(t)

	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)

	// Akışın varsayılan projesi doğrudan değiştirilir (UpdateInput projeye
	// dokunmuyor — burada sınanan şey geçmişin sabitliği).
	_, err = f.pool.Exec(ctx,
		`UPDATE workflows SET project_id = $1 WHERE id = $2`, digeri, w.ID)
	require.NoError(t, err)

	sonra, err := f.store.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, f.projectID, sonra.ProjectID,
		"geçmiş çalışmanın projesi geriye dönük değişmemeli")
	require.Equal(t, "Deneme", sonra.ProjectName)
}

// TestListRuns_ProjeAdiTasinir — aynı akışın hangi çalışması nerede koştu,
// listede görünmeli.
func TestListRuns_ProjeAdiTasinir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)
	digeri := f.ikinciProje(t)

	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	for _, p := range []uuid.UUID{uuid.Nil, digeri} {
		_, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
			Workflow: w, Version: v, Trigger: workflow.TriggerManual,
			Input: "x", ProjectID: p,
		})
		require.NoError(t, err)
	}

	list, total, err := f.store.ListRuns(ctx, workflow.ListRunsFilter{WorkflowID: &w.ID})
	require.NoError(t, err)
	require.Equal(t, 2, total)

	adlar := []string{list[0].ProjectName, list[1].ProjectName}
	require.ElementsMatch(t, []string{"Deneme", "İkinci"}, adlar)
}
