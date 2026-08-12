package workflow_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/workflow"
)

// TestDelete_GecmisiBirlikteGider — akış silinince ona bağlı hiçbir kayıt
// arkada kalmamalı.
//
// Bu testin asıl işi CASCADE zincirini gerçek şemaya karşı sınamak:
// `workflow_runs.version_id` ON DELETE RESTRICT taşıyor ve sürüm cascade'i
// önce koşarsa silme FK ihlaliyle düşerdi. Kod okuyarak anlaşılmaz.
func TestDelete_GecmisiBirlikteGider(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)
	require.NoError(t, f.store.FinishRun(ctx, run.ID, workflow.RunSucceeded, nil))

	require.NoError(t, f.store.Delete(ctx, w.ID))

	for _, tablo := range []string{
		"workflows", "workflow_versions", "workflow_runs", "workflow_hooks",
	} {
		var n int
		require.NoError(t, f.pool.QueryRow(ctx,
			`SELECT count(*) FROM `+tablo).Scan(&n), tablo)
		require.Zero(t, n, "%s tablosunda kayıt kaldı", tablo)
	}

	var adim int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM workflow_steps`).Scan(&adim))
	require.Zero(t, adim, "adımlar kaldı")
}

// TestDelete_SurenCalismaVarkenSilinmez — kayıt gitse de motorun goroutine'i
// yaşamaya devam eder; kullanıcı önce durdurmalı.
func TestDelete_SurenCalismaVarkenSilinmez(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	// CreateRun çalışmayı 'pending' olarak bırakır — yani süren bir iş.
	_, err = f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "x",
	})
	require.NoError(t, err)

	require.ErrorIs(t, f.store.Delete(ctx, w.ID), workflow.ErrRunning)

	_, err = f.store.Get(ctx, w.ID)
	require.NoError(t, err, "akış silinmemeliydi")
}

func TestDelete_OlmayanAkis(t *testing.T) {
	f := setup(t)
	require.ErrorIs(t,
		f.store.Delete(context.Background(), uuid.New()), workflow.ErrNotFound)
}

// TestRunCounts_TekSorgudaAkisBasinaSayi — silme onayı "kaç çalışma gidecek"
// diyebilsin diye.
func TestRunCounts_TekSorgudaAkisBasinaSayi(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	bir := f.newWorkflow(t)
	iki, err := f.store.Create(ctx, workflow.CreateInput{
		ProjectID: f.projectID, Name: "İkinci",
	})
	require.NoError(t, err)

	v, err := f.store.SaveVersion(ctx, bir.ID, f.graph())
	require.NoError(t, err)
	for range 3 {
		_, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
			Workflow: bir, Version: v, Trigger: workflow.TriggerManual, Input: "x",
		})
		require.NoError(t, err)
	}

	counts, err := f.store.RunCounts(ctx, []uuid.UUID{bir.ID, iki.ID})
	require.NoError(t, err)
	require.Equal(t, 3, counts[bir.ID])
	// Hiç çalışmamış akış haritada YOK; çağıran sıfır okur.
	require.Zero(t, counts[iki.ID])

	bos, err := f.store.RunCounts(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, bos)
}
