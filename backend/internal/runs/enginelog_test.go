package runs_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runs"
)

/*
 * Motor loglarının saklanması.
 *
 * Asıl iddia: koşu bitip container silinse bile ham teşhis verisi duruyor.
 * Bugüne kadar kök neden analizi ancak koşu SIRASINDA mümkündü.
 */

func TestEngineLogs_GidisDonus(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	run := f.create(t, "loglu iş")

	icerik := "timestamp=… level=INFO message=\"llm runtime selected\"\nsatır iki\nüçüncü satır"
	require.NoError(t, f.store.SaveEngineLogs(ctx, run.ID, []runner.EngineLog{
		{Source: runner.EngineLogStdout, Content: icerik},
	}, 1<<20))

	got, err := f.store.EngineLogs(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)

	// Sıkıştırma gidiş-dönüşü içeriği BOZMAMALI.
	require.Equal(t, icerik, got[0].Content)
	require.Equal(t, "stdout", got[0].Source)
	require.Equal(t, len(icerik), got[0].RawSize, "ham boyut sıkıştırılmamış hâl olmalı")
	require.False(t, got[0].Truncated)
}

// TestEngineLogs_SondanKorunur — hata genelde SONDA olur; sınır aşılırsa
// baştaki açılış satırları gider, son kısım kalır.
func TestEngineLogs_SondanKorunur(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	run := f.create(t, "uzun loglu iş")

	icerik := strings.Repeat("A", 5000) + "SON-SATIR-HATA"
	require.NoError(t, f.store.SaveEngineLogs(ctx, run.ID, []runner.EngineLog{
		{Source: runner.EngineLogFile, Content: icerik},
	}, 100))

	got, err := f.store.EngineLogs(ctx, run.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)

	require.True(t, got[0].Truncated, "kırpıldığı işaretlenmeli")
	require.Len(t, got[0].Content, 100)
	require.Contains(t, got[0].Content, "SON-SATIR-HATA", "son kısım korunmalı")
	// Ham boyut KIRPILMAMIŞ hâli anlatır: kullanıcı ne kadarını kaçırdığını
	// görebilmeli.
	require.Equal(t, len(icerik), got[0].RawSize)
}

// TestEngineLogs_KosuSilininceGider — yetim kayıt kalmaz (koşu silme uyumu).
func TestEngineLogs_KosuSilininceGider(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	run := f.create(t, "silinecek iş")

	require.NoError(t, f.store.SaveEngineLogs(ctx, run.ID, []runner.EngineLog{
		{Source: runner.EngineLogStdout, Content: "bir şeyler"},
	}, 1<<20))
	f.bitir(t, run)

	require.NoError(t, f.store.Delete(ctx, run.ID))

	var n int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM run_engine_logs WHERE run_id = $1`, run.ID).Scan(&n))
	require.Zero(t, n, "koşu silinince logu da gitmeliydi")
}

// TestPurgeEngineLogs — süresi dolan gider, yenisi kalır. Çalıştırma kaydının
// kendisine dokunulmaz.
func TestPurgeEngineLogs(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	eski := f.create(t, "eski iş")
	yeni := f.create(t, "yeni iş")
	for _, r := range []runs.Run{eski, yeni} {
		require.NoError(t, f.store.SaveEngineLogs(ctx, r.ID, []runner.EngineLog{
			{Source: runner.EngineLogStdout, Content: "içerik"},
		}, 1<<20))
	}

	_, err := f.pool.Exec(ctx,
		`UPDATE run_engine_logs SET created_at = now() - interval '30 days' WHERE run_id = $1`,
		eski.ID)
	require.NoError(t, err)

	n, err := f.store.PurgeEngineLogs(ctx, 7*24*time.Hour)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	bos, err := f.store.EngineLogs(ctx, eski.ID)
	require.NoError(t, err)
	require.Empty(t, bos, "süresi dolan log silinmeliydi")

	dolu, err := f.store.EngineLogs(ctx, yeni.ID)
	require.NoError(t, err)
	require.Len(t, dolu, 1, "süresi dolmayan log kalmalıydı")

	// Çalıştırma kayıtları YERİNDE: yalnızca ham log siliniyor.
	_, err = f.store.Get(ctx, eski.ID)
	require.NoError(t, err, "temizlik çalıştırma kaydını silmemeli")
}

// TestEngineLogs_BosIcerikYazilmaz — boş bir kaynak satır açmamalı.
func TestEngineLogs_BosIcerikYazilmaz(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	run := f.create(t, "boş loglu iş")

	require.NoError(t, f.store.SaveEngineLogs(ctx, run.ID, []runner.EngineLog{
		{Source: runner.EngineLogStdout, Content: ""},
	}, 1<<20))

	got, err := f.store.EngineLogs(ctx, run.ID)
	require.NoError(t, err)
	require.Empty(t, got)
}
