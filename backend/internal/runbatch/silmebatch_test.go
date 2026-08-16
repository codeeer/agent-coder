package runbatch_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runbatch"
)

/*
 * Toplu işin silinmesi.
 *
 * Otuz projelik bir kampanya bittikten sonra listede kalıyor ve
 * kaldırılamıyordu; biriken kuyruğu temizlemenin hiçbir yolu yoktu.
 */

// sahteSilici, akış çalışmalarını silmesi istenenleri kaydeder.
//
// Gerçek silme `workflow` paketinde ve orada test ediliyor; burada ölçülen şey
// toplu işin DOĞRU çalışmaları, doğru sırayla silmeye göndermesi.
type sahteSilici struct {
	silinen []uuid.UUID
	hata    error
}

func (s *sahteSilici) DeleteRun(_ context.Context, id uuid.UUID) error {
	if s.hata != nil {
		return s.hata
	}
	s.silinen = append(s.silinen, id)
	return nil
}

/*
BİTMİŞ TOPLU İŞ, ÖĞELERİYLE BİRLİKTE GİDER.

Toplu işin kaydı silinince öğeleri kaskadla gidiyor; ama öğelerin işaret ettiği
AKIŞ ÇALIŞMALARI şemada SET NULL, yani yerinde kalırdı. Açıkça silinmezlerse
kullanıcı kampanyayı sildiğini sanır, geçmiş başka bir ekranda durmaya devam
ederdi.
*/
func TestDelete_BitmisIsVeCalismalariGider(t *testing.T) {
	f := setup(t, "alfa", "beta")
	ctx := context.Background()
	b := f.create(t, f.projects...)

	// İki öğe de bir akış çalışmasına bağlansın.
	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	var beklenen []uuid.UUID
	for _, it := range items {
		run := f.workflowRun(t, it.ProjectID)
		require.NoError(t, f.store.Claim(ctx, it.ID))
		require.NoError(t, f.store.Attach(ctx, it.ID, run))
		require.NoError(t, f.store.MarkFinished(ctx, it.ID, runbatch.ItemSucceeded, ""))
		beklenen = append(beklenen, run)
	}

	silici := &sahteSilici{}
	require.NoError(t, f.store.Delete(ctx, b.ID, silici))

	require.ElementsMatch(t, beklenen, silici.silinen,
		"öğelerin akış çalışmaları da silinmeye gönderilmeli")

	_, _, err = f.store.Get(ctx, b.ID)
	require.ErrorIs(t, err, runbatch.ErrNotFound)
}

/*
SÜREN TOPLU İŞ SİLİNEMEZ.

Kuyruk hâlâ iş başlatıyor. Kaydı silmek, çalışan container'ları sahipsiz
bırakır ve zamanlayıcı var olmayan bir toplu işin öğelerini işlemeye devam
ederdi. Kullanıcı önce iptal eder.
*/
func TestDelete_SurenIsSilinemez(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	require.ErrorIs(t, f.store.Delete(ctx, b.ID, &sahteSilici{}), runbatch.ErrRunning)

	_, _, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err, "reddedilen silme kaydı bozmamalı")
}

// İptal edilmiş iş silinebilir: kuyruk durmuş, ortada canlı bir şey yok.
func TestDelete_IptalEdilmisIsSilinebilir(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	_, err := f.store.Cancel(ctx, b.ID)
	require.NoError(t, err)

	require.NoError(t, f.store.Delete(ctx, b.ID, &sahteSilici{}))
}

/*
BİR ÇALIŞMA SİLİNEMEZSE TOPLU İŞ DE SİLİNMEZ.

Yarım kalmış bir silme, kullanıcıya "temizlendi" derken geçmişin bir kısmını
bırakırdı — ve hangi kısmın kaldığı hiçbir yerde görünmezdi.
*/
func TestDelete_CalismaSilinemezseIsDeKalir(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.NoError(t, f.store.Claim(ctx, items[0].ID))
	require.NoError(t, f.store.Attach(ctx, items[0].ID, f.workflowRun(t, items[0].ProjectID)))
	require.NoError(t, f.store.MarkFinished(ctx, items[0].ID, runbatch.ItemSucceeded, ""))

	silici := &sahteSilici{hata: context.DeadlineExceeded}
	require.Error(t, f.store.Delete(ctx, b.ID, silici))

	_, _, err = f.store.Get(ctx, b.ID)
	require.NoError(t, err, "çalışma silinemediyse toplu iş de durmalı")
}

/*
İPTAL EDİLMİŞ AMA ÖĞESİ HÂLÂ ÇALIŞAN İŞ DE SİLİNEMEZ.

İptal yalnızca BEKLEYEN öğeleri düşürüyor; o sırada çalışan öğe bitene kadar
devam ediyor. Yani toplu işin durumu 'cancelled' iken bile canlı bir çalışma
kalabiliyor. Yalnızca duruma bakılsaydı bu iş silinebilirdi ve çalışan
container sahipsiz kalırdı.
*/
func TestDelete_IptalEdilseDeCalisanOgeVarkenSilinemez(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.NoError(t, f.store.Claim(ctx, items[0].ID)) // öğe çalışmaya başladı
	_, err = f.store.Cancel(ctx, b.ID)
	require.NoError(t, err)

	require.ErrorIs(t, f.store.Delete(ctx, b.ID, &sahteSilici{}), runbatch.ErrRunning)
}

func TestDelete_OlmayanIs(t *testing.T) {
	f := setup(t)
	require.ErrorIs(t, f.store.Delete(context.Background(), uuid.New(), &sahteSilici{}),
		runbatch.ErrNotFound)
}
