package workflow_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Akış çalışmasının silinmesi.
 *
 * Çalıştırmalar ekranı, akış adımına ait satırda kullanıcıyı buraya
 * yönlendiriyor: "akış çalışmasını silin". Bu yol yoksa o ipucu yapılması
 * imkânsız bir eylemi tarif ediyor demektir.
 */

// calisma, verilen durumda bir akış çalışması ve ona bağlı bir adım üretir.
func (f fixture) calisma(t *testing.T, durum workflow.RunStatus) workflow.Run {
	t.Helper()
	ctx := context.Background()

	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "iş",
	})
	require.NoError(t, err)

	if durum != workflow.RunPending {
		require.NoError(t, f.store.FinishRun(ctx, run.ID, durum, nil))
	}
	return run
}

/*
BİTMİŞ ÇALIŞMA VE ADIMLARININ ÇALIŞTIRMALARI BİRLİKTE GİDER.

Şemada `workflow_steps.run_id` SET NULL: adım satırı kaskadla gitse bile
adımın ÇALIŞTIRMA kaydı yerinde kalıyor. Temizlenmezse otuz projelik bir
kampanyayı silen kullanıcı, Çalıştırmalar ekranında otuz yetim satırla
kalırdı — üstelik onları tek tek silmesi de mümkün olmazdı.
*/
func TestDeleteRun_AdimCalistirmalariDaGider(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run := f.calisma(t, workflow.RunSucceeded)

	require.NoError(t, f.store.DeleteRun(ctx, run.ID))

	_, err := f.store.GetRun(ctx, run.ID)
	require.ErrorIs(t, err, workflow.ErrNotFound)

	var adim, calistirma int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM workflow_steps WHERE workflow_run_id = $1`, run.ID).Scan(&adim))
	require.Zero(t, adim, "adım satırları kaskadla gitmeliydi")

	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM runs r
		  WHERE EXISTS (SELECT 1 FROM workflow_steps s WHERE s.run_id = r.id)`).Scan(&calistirma))
	require.Zero(t, calistirma, "adımların çalıştırmaları yetim kalmamalı")
}

/*
SÜREN ÇALIŞMA SİLİNEMEZ.

Container ve goroutine hâlâ canlı; kaydı silmek onları sahipsiz bırakırdı.
Kullanıcı önce iptal etmeli.
*/
func TestDeleteRun_SurenCalismaSilinemez(t *testing.T) {
	f := setup(t)

	run := f.calisma(t, workflow.RunPending)

	require.ErrorIs(t, f.store.DeleteRun(context.Background(), run.ID), workflow.ErrRunning)
}

func TestDeleteRun_OlmayanCalisma(t *testing.T) {
	f := setup(t)
	require.ErrorIs(t, f.store.DeleteRun(context.Background(), f.projectID), workflow.ErrNotFound)
}
