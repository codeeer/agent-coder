package runs_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runs"
)

// bitir, kaydı terminal duruma taşır — silme yalnızca bitmiş işlerde açık.
func (f fixture) bitir(t *testing.T, run runs.Run) {
	t.Helper()
	require.NoError(t, f.store.Finish(context.Background(), run.ID,
		runs.StatusSucceeded, &runner.Result{Output: "bitti"}, nil))
}

// TestDelete_BitmisKayitVeOlaylariGider — olay geçmişi CASCADE ile gitmeli,
// arkada yetim satır kalmamalı.
func TestDelete_BitmisKayitVeOlaylariGider(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run := f.create(t, "silinecek iş")
	for _, m := range []string{"bir", "iki"} {
		_, _, err := f.store.AppendEvent(ctx, run.ID, "info", m)
		require.NoError(t, err)
	}
	f.bitir(t, run)

	require.NoError(t, f.store.Delete(ctx, run.ID))

	_, err := f.store.Get(ctx, run.ID)
	require.ErrorIs(t, err, runs.ErrNotFound)

	var olay int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM run_events WHERE run_id = $1`, run.ID).Scan(&olay))
	require.Zero(t, olay, "olay geçmişi kaskadla gitmeliydi")
}

// TestDelete_SurenKayitSilinmez — container ve goroutine hâlâ canlı.
func TestDelete_SurenKayitSilinmez(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run := f.create(t, "süren iş") // Create 'pending' bırakır
	require.ErrorIs(t, f.store.Delete(ctx, run.ID), runs.ErrActive)

	_, err := f.store.Get(ctx, run.ID)
	require.NoError(t, err, "kayıt silinmemeliydi")
}

// TestDelete_AkisAdimiSilinmez — silinseydi akış detayında adım "başarılı"
// görünmeye devam eder ama agent'ı, maliyeti ve token'ı boşalırdı.
func TestDelete_AkisAdimiSilinmez(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run := f.create(t, "akışın adımı")
	f.bitir(t, run)
	f.akisAdimiYap(t, run)

	require.ErrorIs(t, f.store.Delete(ctx, run.ID), runs.ErrWorkflowStep)

	_, err := f.store.Get(ctx, run.ID)
	require.NoError(t, err, "kayıt silinmemeliydi")
}

func TestDelete_OlmayanKayit(t *testing.T) {
	f := setup(t)
	require.ErrorIs(t,
		f.store.Delete(context.Background(), uuid.New()), runs.ErrNotFound)
}

// akisAdimiYap, çalıştırmayı bir akış adımına bağlar.
//
// Akış paketini import etmemek için doğrudan SQL: bu test runs paketinin
// davranışını sınıyor, akış motorunun değil.
func (f fixture) akisAdimiYap(t *testing.T, run runs.Run) {
	t.Helper()
	ctx := context.Background()

	var versionID, workflowID, workflowRunID uuid.UUID
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO workflows (project_id, name) VALUES ($1, 'Akış') RETURNING id`,
		f.projectID).Scan(&workflowID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO workflow_versions (workflow_id, version, graph)
		 VALUES ($1, 1, '{"nodes":[],"edges":[]}'::jsonb) RETURNING id`,
		workflowID).Scan(&versionID))
	require.NoError(t, f.pool.QueryRow(ctx,
		`INSERT INTO workflow_runs (workflow_id, project_id, version_id, version,
		     trigger_kind, input)
		 VALUES ($1, $2, $3, 1, 'manual', 'x') RETURNING id`,
		workflowID, f.projectID, versionID).Scan(&workflowRunID))
	_, err := f.pool.Exec(ctx,
		`INSERT INTO workflow_steps (workflow_run_id, node_id, node_kind, level, run_id)
		 VALUES ($1, 'a1', 'agent', 0, $2)`, workflowRunID, run.ID)
	require.NoError(t, err)
}
