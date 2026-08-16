package workflow_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
//
// ADIMA GERÇEK BİR ÇALIŞTIRMA BAĞLANIR. Bağlanmazsa "adımların çalıştırmaları
// da gitti" iddiası boşa döner: silinecek bir çalıştırma olmadığı için sayım
// silme kaldırılsa bile sıfır çıkar. Ölçüldü — bu yardımcının ilk hâli tam
// olarak bu yüzden hiçbir şeyi korumuyordu.
func (f fixture) calisma(t *testing.T, durum workflow.RunStatus) workflow.Run {
	run, _ := f.calismaVeAdimlari(t, durum)
	return run
}

// calismaVeAdimlari, çalışmayı ve adımlarına bağlanan ÇALIŞTIRMA KİMLİKLERİNİ
// birlikte döner.
//
// Kimlikler silmeden ÖNCE alınmak zorunda: `workflow_steps` kaskadla gidiyor,
// yani silme sonrası "hangi çalıştırmalar bu akışa aitti" diye sorulacak bir
// yer kalmıyor. Bu ayrımı kaçırmak, bu dosyadaki iddiayı iki kez boşa
// düşürdü — bir kere adıma çalıştırma bağlanmadığı için, bir kere de sayım
// silinmiş satırlar üzerinden yapıldığı için.
func (f fixture) calismaVeAdimlari(
	t *testing.T, durum workflow.RunStatus,
) (workflow.Run, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	w := f.newWorkflow(t)
	v, err := f.store.SaveVersion(ctx, w.ID, f.graph())
	require.NoError(t, err)

	run, err := f.store.CreateRun(ctx, workflow.CreateRunInput{
		Workflow: w, Version: v, Trigger: workflow.TriggerManual, Input: "iş",
	})
	require.NoError(t, err)
	calistirmalar := f.adimaCalistirmaBagla(t, run.ID)

	if durum != workflow.RunPending {
		require.NoError(t, f.store.FinishRun(ctx, run.ID, durum, nil))
	}
	return run, calistirmalar
}

// kalanCalistirma, verilen kimliklerden kaçının hâlâ durduğunu sayar.
func (f fixture) kalanCalistirma(t *testing.T, ids []uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM runs WHERE id = ANY($1)`, ids).Scan(&n))
	return n
}

// adimaCalistirmaBagla, HER agent adımına kendi `runs` kaydını bağlar ve
// üretilen çalıştırmaların kimliklerini döner.
//
// Motor bunu çalışma sırasında yapıyor; test motoru döndürmüyor (container
// açılırdı), o yüzden aynı bağ elle kuruluyor. Her adımın AYRI çalıştırması
// olması önemli: tek bir kayıt paylaşılsaydı "hepsi gitti" iddiası, tek satır
// silen bir uygulamada da geçerdi.
func (f fixture) adimaCalistirmaBagla(t *testing.T, workflowRunID uuid.UUID) []uuid.UUID {
	t.Helper()
	ctx := context.Background()

	rows, err := f.pool.Query(ctx,
		`SELECT id FROM workflow_steps
		  WHERE workflow_run_id = $1 AND node_kind = 'agent' ORDER BY level`, workflowRunID)
	require.NoError(t, err)
	adimlar, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	require.NoError(t, err)
	require.NotEmpty(t, adimlar, "ön koşul: bağlanacak agent adımı olmalı")

	var runIDs []uuid.UUID
	for _, adimID := range adimlar {
		var runID uuid.UUID
		require.NoError(t, f.pool.QueryRow(ctx, `
			INSERT INTO runs (project_id, agent_id, agent_slug, agent_prompt,
			                  provider_slug, model_id, branch, task, status, node_version)
			VALUES ($1, $2, 'coder', 'p', 'openrouter', 'm1', 'main', 'adım işi',
			        'succeeded', '24')
			RETURNING id`, f.projectID, f.agentID).Scan(&runID))

		_, err := f.pool.Exec(ctx,
			`UPDATE workflow_steps SET run_id = $1 WHERE id = $2`, runID, adimID)
		require.NoError(t, err)
		runIDs = append(runIDs, runID)
	}
	return runIDs
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

	run, calistirmalar := f.calismaVeAdimlari(t, workflow.RunSucceeded)
	require.Len(t, calistirmalar, 2, "ön koşul: iki agent adımının çalıştırması var")
	require.Equal(t, 2, f.kalanCalistirma(t, calistirmalar), "ön koşul: ikisi de duruyor")

	require.NoError(t, f.store.DeleteRun(ctx, run.ID))

	_, err := f.store.GetRun(ctx, run.ID)
	require.ErrorIs(t, err, workflow.ErrNotFound)

	var adim int
	require.NoError(t, f.pool.QueryRow(ctx,
		`SELECT count(*) FROM workflow_steps WHERE workflow_run_id = $1`, run.ID).Scan(&adim))
	require.Zero(t, adim, "adım satırları kaskadla gitmeliydi")

	require.Zero(t, f.kalanCalistirma(t, calistirmalar),
		"adımların çalıştırmaları yetim kaldı")
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

/*
AKIŞ SİLİNİNCE ADIM ÇALIŞTIRMALARI DA GİDER.

`workflow_runs.workflow_id` şemada CASCADE: akış tanımı silinince çalışmaları
ve adım satırları kendiliğinden gidiyor. Ama `workflow_steps.run_id` SET NULL
olduğu için adımların ÇALIŞTIRMA kayıtları yerinde kalıyordu — bağları kopmuş
hâlde. Kaybolmuyorlardı; Çalıştırmalar ekranında sıradan çalıştırma gibi
görünmeye başlıyorlardı.

Bu, `DeleteRun` ile TUTARSIZDI: aynı veriyi bir yoldan silmek temizliyor,
diğerinden silmek bırakıyordu. Kullanıcı hangi kapıdan girdiğine göre farklı
sonuç alıyordu.
*/
func TestDelete_AkisSilininceAdimCalistirmalariDaGider(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run, calistirmalar := f.calismaVeAdimlari(t, workflow.RunSucceeded)
	w, err := f.store.GetRun(ctx, run.ID)
	require.NoError(t, err)
	require.Equal(t, 2, f.kalanCalistirma(t, calistirmalar), "ön koşul: ikisi de duruyor")

	require.NoError(t, f.store.Delete(ctx, w.WorkflowID))

	require.Zero(t, f.kalanCalistirma(t, calistirmalar),
		"akış silindi ama adım çalıştırmaları kaldı")
}

/*
SÜREN ÇALIŞMASI OLAN AKIŞ SİLİNEMEZ — ve hiçbir şeye dokunmaz.

Koruma zaten vardı; buradaki yeni iddia, REDDEDİLEN silmenin geçmişi
bozmaması. Çalıştırma silme adımı korumadan önce çalışsaydı, "silinemez"
cevabını alan kullanıcı verisinin bir kısmını çoktan kaybetmiş olurdu.
*/
func TestDelete_ReddedilenSilmeCalistirmalaraDokunmaz(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run, calistirmalar := f.calismaVeAdimlari(t, workflow.RunPending)
	w, err := f.store.GetRun(ctx, run.ID)
	require.NoError(t, err)

	require.ErrorIs(t, f.store.Delete(ctx, w.WorkflowID), workflow.ErrRunning)

	require.Equal(t, 2, f.kalanCalistirma(t, calistirmalar),
		"reddedilen silme çalıştırmalara dokunmamalı")
}
