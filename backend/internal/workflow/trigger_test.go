package workflow_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Tekrar-işleme koruması.
 *
 * Korumanın veritabanı kısıtı olmasının sebebi budur: iki tetikleme yolu
 * (tarama ve webhook) aynı anda gelebilir. Uygulama içi bir kontrol (önce sor,
 * sonra yaz) yarışa açık olurdu.
 */

func TestMarkProcessed_AyniTaskIkinciKezIslenmez(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	ilk, err := f.store.MarkProcessed(ctx, w.ID, "SCRUM-1", "2026-08-10T00:00:00")
	require.NoError(t, err)
	require.True(t, ilk, "ilk görüşte akış başlatılmalı")

	ikinci, err := f.store.MarkProcessed(ctx, w.ID, "SCRUM-1", "2026-08-10T00:00:00")
	require.NoError(t, err)
	require.False(t, ikinci, "aynı task ikinci kez başlatılmamalı")
}

// TestMarkProcessed_GuncellenenTaskYenidenIslenir — anahtara güncellenme zamanı
// dahil; yalnızca (akış, task) olsaydı güncellenen task bir daha hiç işlenmezdi.
func TestMarkProcessed_GuncellenenTaskYenidenIslenir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	_, err := f.store.MarkProcessed(ctx, w.ID, "SCRUM-1", "2026-08-10T00:00:00")
	require.NoError(t, err)

	sonra, err := f.store.MarkProcessed(ctx, w.ID, "SCRUM-1", "2026-08-10T09:30:00")
	require.NoError(t, err)
	require.True(t, sonra, "task güncellenince yeniden işlenmeli")
}

// TestUnmarkProcessed_BaslatilamayanTaskTekrarDenenir — gerçek bir hatanın
// testi: işaret akış başlatılmadan ÖNCE konuyor (yarış için gerekli), o yüzden
// başlatma hata verirse işaret geri alınmalı. Alınmazsa task hiç çalışmadan
// "işlendi" sayılır ve bir daha hiç denenmez.
func TestUnmarkProcessed_BaslatilamayanTaskTekrarDenenir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	_, err := f.store.MarkProcessed(ctx, w.ID, "SCRUM-9", "2026-08-10T00:00:00")
	require.NoError(t, err)

	require.NoError(t, f.store.UnmarkProcessed(ctx, w.ID, "SCRUM-9", "2026-08-10T00:00:00"))

	tekrar, err := f.store.MarkProcessed(ctx, w.ID, "SCRUM-9", "2026-08-10T00:00:00")
	require.NoError(t, err)
	require.True(t, tekrar, "başlatılamayan task sonraki taramada yeniden denenmeli")
}

// TestUnmarkProcessed_CalismayaBagliTaskSilinmez — geri alma yalnızca boşta
// kalan işareti temizler. Akışı gerçekten başlatmış bir kaydı silmek, o task'ı
// ikinci kez çalıştırmak demekti.
func TestUnmarkProcessed_CalismayaBagliTaskSilinmez(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	_, err := f.store.MarkProcessed(ctx, w.ID, "SCRUM-9", "2026-08-10T00:00:00")
	require.NoError(t, err)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)
	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerJira, Input: "x",
	})
	require.NoError(t, err)
	require.NoError(t, f.store.LinkProcessed(ctx, w.ID, "SCRUM-9", "2026-08-10T00:00:00", run.ID))

	require.NoError(t, f.store.UnmarkProcessed(ctx, w.ID, "SCRUM-9", "2026-08-10T00:00:00"))

	tekrar, err := f.store.MarkProcessed(ctx, w.ID, "SCRUM-9", "2026-08-10T00:00:00")
	require.NoError(t, err)
	require.False(t, tekrar, "çalışması başlamış task yeniden işlenmemeli")
}

func TestMarkProcessed_FarkliAkislarBirbirindenBagimsiz(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	a := f.newWorkflow(t)
	b := f.newWorkflow(t)

	ilk, err := f.store.MarkProcessed(ctx, a.ID, "SCRUM-1", "t")
	require.NoError(t, err)
	require.True(t, ilk)

	digeri, err := f.store.MarkProcessed(ctx, b.ID, "SCRUM-1", "t")
	require.NoError(t, err)
	require.True(t, digeri, "aynı task farklı akışta ayrıca işlenebilir")
}

// TestMarkProcessed_EszamanliCagriTekKazanir — iki tetikleme yolunun aynı anda
// gelmesi tam olarak bu; korumanın veritabanında olmasının sebebi.
func TestMarkProcessed_EszamanliCagriTekKazanir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	const n = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		kazanan int
	)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := f.store.MarkProcessed(ctx, w.ID, "SCRUM-9", "aynı-an")
			if err == nil && ok {
				mu.Lock()
				kazanan++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	require.Equal(t, 1, kazanan, "eşzamanlı %d çağrıdan yalnızca biri akışı başlatmalı", n)
}

func TestScanState_KaydedilirVeOkunur(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := f.newWorkflow(t)

	// Hiç taranmamış akış hata vermemeli, boş durum dönmeli.
	state, err := f.store.ScanState(ctx, w.ID)
	require.NoError(t, err)
	require.Nil(t, state.ScannedAt)

	msg := "JQL geçersiz"
	require.NoError(t, f.store.SaveScanState(ctx, workflow.ScanState{
		WorkflowID: w.ID, Found: 3, Started: 1, Error: &msg,
	}))

	state, err = f.store.ScanState(ctx, w.ID)
	require.NoError(t, err)
	require.NotNil(t, state.ScannedAt)
	require.Equal(t, 3, state.Found)
	require.Equal(t, 1, state.Started)
	require.Equal(t, "JQL geçersiz", *state.Error)
}
