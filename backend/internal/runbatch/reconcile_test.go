package runbatch_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runbatch"
)

/*
 * Uzlaştırma, iptal ve devam (spec 023 Blok 3).
 *
 * Üçünün ortak sorusu: BİR EYLEM NEYE DOKUNMAZ. Fazla dokunan bir iptal süren
 * bir işi yarıda keser, fazla dokunan bir "devam" tamamlanmış yirmi projeyi
 * yeniden koşturur. İkisi de veri kaybı değil ama ikisi de kullanıcının
 * beklemediği bir maliyet üretir.
 */

// T20 — açılışta `running` kalan öğeler `interrupted` olur.
//
// Backend kapandığında container'lar gitti; o çalışmalar tamamlanamaz. Kayıt
// `running` bırakılsaydı arayüzde sonsuza kadar dönen bir iş görünürdü.
func TestReconcile_YarimKalanOgelerKesilirIsaretlenir(t *testing.T) {
	f := setup(t, "alfa", "beta", "gama")
	ctx := context.Background()
	b := f.create(t, f.projects...)

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)

	f.baslat(t, items[0].ID, items[0].ProjectID)
	f.baslat(t, items[1].ID, items[1].ProjectID)
	require.NoError(t, f.store.MarkFinished(ctx, items[1].ID, runbatch.ItemSucceeded, ""))

	n, err := f.store.InterruptRunning(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n, "yalnızca çalışan öğe kesilmiş sayılır")

	_, items, err = f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.ItemInterrupted, items[0].Status)
	require.NotEmpty(t, items[0].Error, "ne olduğu YAZILIR — durum belirsiz bırakılmaz")
	require.Equal(t, runbatch.ItemSucceeded, items[1].Status, "biten iş dokunulmaz")
	require.Equal(t, runbatch.ItemPending, items[2].Status, "bekleyen beklemeye devam eder")
}

// İkinci bir uzlaştırma turu HİÇBİR ŞEY düşürmez: sayı "bu açılışta kesilenler"
// demektir, "kesilmiş olanlar" değil. Aksi halde açılış logu her seferinde
// büyüyen bir rakam yazardı.
func TestReconcile_IkinciTurBosDoner(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	f.baslat(t, items[0].ID, items[0].ProjectID)

	first, err := f.store.InterruptRunning(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, first)

	second, err := f.store.InterruptRunning(ctx)
	require.NoError(t, err)
	require.Zero(t, second)
}

// Bekleyeni kalmayan bir toplu iş, uzlaştırmadan sonra "çalışıyor" görünmez.
func TestReconcile_TopluIsinDurumuDaTazelenir(t *testing.T) {
	f := setup(t, "alfa", "beta")
	ctx := context.Background()
	b := f.create(t, f.projects[0], f.projects[1])

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	f.baslat(t, items[0].ID, items[0].ProjectID)
	f.baslat(t, items[1].ID, items[1].ProjectID)

	_, err = f.store.InterruptRunning(ctx)
	require.NoError(t, err)

	batch, _, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.StatusDone, batch.Status,
		"çalışanı ve bekleyeni kalmayan toplu iş 'çalışıyor' görünmemeli")
	require.Equal(t, 2, batch.Counts.Interrupted)
}

/*
T21 — İPTAL YALNIZCA BEKLEYENLERİ DÜŞÜRÜR.

Çalışan öğe dokunulmaz ve kendi hâlinde devam eder (spec 023 H6): süren bir
container'ı yarıda kesmek, yan etkisi yarım kalmış bir iş bırakırdı.
*/
func TestCancel_YalnizcaBekleyenlerDuser(t *testing.T) {
	f := setup(t, "alfa", "beta", "gama")
	ctx := context.Background()
	b := f.create(t, f.projects...)

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	f.baslat(t, items[0].ID, items[0].ProjectID)

	dusen, err := f.store.Cancel(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, 2, dusen, "iptalin SONUCU sayıyla söylenir")

	batch, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.ItemRunning, items[0].Status, "çalışan öğe dokunulmaz")
	require.Equal(t, runbatch.ItemCancelled, items[1].Status)
	require.Equal(t, runbatch.ItemCancelled, items[2].Status)
	require.Equal(t, runbatch.StatusCancelled, batch.Status)
}

// Çalışan öğenin sonucu iptalden SONRA da kaydedilir — ve toplu işi 'done'a
// döndürmez. Kullanıcı onu iptal etti; sonradan "bitti" demek kararı silerdi.
func TestCancel_SurenIsinSonucuYineKaydedilir(t *testing.T) {
	f := setup(t, "alfa", "beta")
	ctx := context.Background()
	b := f.create(t, f.projects[0], f.projects[1])

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	f.baslat(t, items[0].ID, items[0].ProjectID)

	_, err = f.store.Cancel(ctx, b.ID)
	require.NoError(t, err)
	require.NoError(t, f.store.MarkFinished(ctx, items[0].ID, runbatch.ItemSucceeded, ""))

	batch, _, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Counts.Succeeded, "süren işin sonucu korunur")
	require.Equal(t, runbatch.StatusCancelled, batch.Status)
}

// T23 — bitmiş bir toplu işi iptal etmek HATA DEĞİLDİR.
func TestCancel_BitmisTopluIsHataDegil(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	f.baslat(t, items[0].ID, items[0].ProjectID)
	require.NoError(t, f.store.MarkFinished(ctx, items[0].ID, runbatch.ItemSucceeded, ""))

	dusen, err := f.store.Cancel(ctx, b.ID)
	require.NoError(t, err, "bitmiş işi iptal hata değil")
	require.Zero(t, dusen, "düşecek bekleyen yok")

	batch, _, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.StatusDone, batch.Status,
		"bitmiş iş iptalle geriye alınmaz; durumu olduğu gibi söylenir")
}

/*
T22 — DEVAM YALNIZCA KESİLENLERİ SIRAYA ALIR.

`succeeded` tamamlandı, `failed` çalıştı ve bir sonuç üretti (derleme hatası
gibi). İkisini de tekrar koşturmak, kullanıcının kaçındığı şeyi — tamamlanmış
yirmi projenin yeniden çalışmasını — geri getirirdi.
*/
func TestResume_YalnizcaKesilenlerSirayaAlinir(t *testing.T) {
	f := setup(t, "alfa", "beta", "gama", "delta")
	ctx := context.Background()
	b := f.create(t, f.projects...)

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)

	// 0: kesildi · 1: tamamlandı · 2: başarısız · 3: iptal edildi
	f.baslat(t, items[0].ID, items[0].ProjectID)
	require.NoError(t, f.store.MarkFinished(ctx, items[0].ID, runbatch.ItemInterrupted, "kesildi"))
	f.baslat(t, items[1].ID, items[1].ProjectID)
	require.NoError(t, f.store.MarkFinished(ctx, items[1].ID, runbatch.ItemSucceeded, ""))
	f.baslat(t, items[2].ID, items[2].ProjectID)
	require.NoError(t, f.store.MarkFinished(ctx, items[2].ID, runbatch.ItemFailed, "derleme hatası"))
	_, err = f.store.Cancel(ctx, b.ID)
	require.NoError(t, err)

	sirayaAlinan, err := f.store.Resume(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, 1, sirayaAlinan, "düğme kaç işin sıraya alınacağını ÖNCEDEN söyler")

	batch, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.ItemPending, items[0].Status)
	require.Empty(t, items[0].Error, "yeniden denenecek öğenin eski hatası silinir")
	require.Equal(t, runbatch.ItemSucceeded, items[1].Status, "tamamlanan tekrar koşmaz")
	require.Equal(t, runbatch.ItemFailed, items[2].Status,
		"gerçekten başarısız olan kendiliğinden sıraya alınmaz — o çalıştı ve sonuç üretti")
	require.Equal(t, runbatch.ItemCancelled, items[3].Status, "iptal edilen de dokunulmaz")

	require.Equal(t, runbatch.StatusQueued, batch.Status,
		"toplu iş yeniden kuyruğa girer; 'cancelled' kalsaydı NextPending öğeyi hiç görmezdi")
}

// Sıraya alınan öğe gerçekten KUYRUKTAN gelir. Durum diriltilmezse öğe
// `pending` görünür ama zamanlayıcı onu hiç almaz — sessizce donmuş bir kuyruk.
func TestResume_SirayaAlinanOgeKuyruktanGelir(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	f.baslat(t, items[0].ID, items[0].ProjectID)
	require.NoError(t, f.store.MarkFinished(ctx, items[0].ID, runbatch.ItemInterrupted, "kesildi"))

	_, ok, err := f.store.NextPending(ctx)
	require.NoError(t, err)
	require.False(t, ok, "kesilmiş öğe kendiliğinden sıraya girmez")

	n, err := f.store.Resume(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	p, ok, err := f.store.NextPending(ctx)
	require.NoError(t, err)
	require.True(t, ok, "devam denince öğe kuyruğa girer")
	require.Equal(t, items[0].ID, p.ID)
}

// Kesilmiş öğesi olmayan bir toplu işte "devam" hiçbir şey yapmaz — düğme de
// zaten çıkmaz (spec 023 H5a).
func TestResume_KesilmisOgeYoksaSifir(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	n, err := f.store.Resume(ctx, b.ID)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestCancelResume_OlmayanTopluIs(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	_, err := f.store.Cancel(ctx, f.workflowID) // var olmayan bir toplu iş kimliği
	require.ErrorIs(t, err, runbatch.ErrNotFound)

	_, err = f.store.Resume(ctx, f.workflowID)
	require.ErrorIs(t, err, runbatch.ErrNotFound)
}
