package runs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runs"
)

/*
 * Silme kapısı — Manager'daki İKİNCİ savunma hattı.
 *
 * "Bu çalıştırma sürüyor mu" sorusunun iki kaynağı var ve ikisi AYNI ŞEYİ
 * SÖYLEMEK ZORUNDA DEĞİL:
 *   · veritabanındaki durum
 *   · bellekteki canlı iş listesi (`m.cancels`)
 *
 * `store.Delete` yalnızca birinciye bakıyor ve o kontrol test edilmiş
 * durumda. Manager'ın kendi kontrolü ise korumasızdı: mutasyon taraması
 * `if m.Running(id) { return ErrActive }` satırını kaldırdığında hiçbir test
 * kırmızıya dönmedi.
 *
 * Aradaki fark gerçek bir pencere: kayıt veritabanında bitmiş görünürken
 * goroutine hâlâ canlı olabiliyor. O anda silme geçseydi kayıt giderdi ve
 * arkada çalışmaya devam eden iş, artık var olmayan bir çalıştırmaya olay
 * yazmayı sürdürürdü.
 */

/*
VERİTABANI "BİTTİ" DERKEN CANLI İŞ SİLİNEMEZ.

Test bu pencereyi BİLEREK kuruyor: motor hâlâ bloke edilmiş durumdayken kayıt
doğrudan bitmiş olarak işaretleniyor. Böylece `store.Delete`'in kontrolü
geçerdi — geriye yalnızca Manager'ın kontrolü kalıyor ve ölçülen tam olarak o.
*/
func TestManagerDelete_VeritabaniBitmisDeseDeCanliIsSilinemez(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	stub := &stubRunner{block: make(chan struct{})}
	m, _ := newManager(f, stub, func() int { return 1 })
	defer m.Shutdown()

	run, err := m.Start(ctx, startInput(f, "süren iş"))
	require.NoError(t, err)
	waitFor(t, "iş motora ulaşmalı", func() bool { return stub.started.Load() == 1 })

	// Kaydı bitmiş göster; goroutine hâlâ `block` kanalında bekliyor.
	require.NoError(t, f.store.Finish(ctx, run.ID, runs.StatusSucceeded, nil, nil))

	// ÖN KOŞUL: mağaza katmanının kontrolü artık geçerdi. Bu satır olmadan
	// test, korumak istediği Manager kontrolünü değil mağazanınkini ölçerdi.
	okunan, err := f.store.Get(ctx, run.ID)
	require.NoError(t, err)
	require.True(t, okunan.Status.Terminal(),
		"ön koşul: kayıt bitmiş görünmeli, yoksa mağaza zaten reddederdi")
	require.True(t, m.Running(run.ID), "ön koşul: goroutine hâlâ canlı olmalı")

	require.ErrorIs(t, m.Delete(ctx, run.ID), runs.ErrActive,
		"canlı goroutine varken silme geçmemeli")

	// Reddedilen silme kaydı bozmamalı.
	_, err = f.store.Get(ctx, run.ID)
	require.NoError(t, err)
}

/*
BİTMİŞ VE ARTIK CANLI OLMAYAN İŞ SİLİNEBİLİR.

Kapının yalnızca reddettiğini sınamak yetmez: her şeyi reddeden bir kontrol de
o testi geçerdi. Karşı yön de yazılı olmalı.
*/
func TestManagerDelete_BitmisIsSilinebilir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	stub := &stubRunner{block: make(chan struct{})}
	m, _ := newManager(f, stub, func() int { return 1 })
	defer m.Shutdown()

	run, err := m.Start(ctx, startInput(f, "bitecek iş"))
	require.NoError(t, err)
	waitFor(t, "iş motora ulaşmalı", func() bool { return stub.started.Load() == 1 })

	close(stub.block) // motor dönsün
	waitFor(t, "iş bitmeli", func() bool { return !m.Running(run.ID) })

	require.NoError(t, m.Delete(ctx, run.ID))

	_, err = f.store.Get(ctx, run.ID)
	require.ErrorIs(t, err, runs.ErrNotFound)
}
