package runbatch_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runbatch"
)

/*
 * Öğe kapısı — zamanlayıcının en sık geçtiği yol.
 *
 * `Claim`, `Requeue` ve `MarkFinished` aynı ortak gövdeyi (`withBatch`)
 * kullanıyor ve o gövdedeki "öğe yok" kontrolü korumasızdı: mutasyon taraması
 * onu kaldırdığında hiçbir test kırmızıya dönmedi.
 *
 * Sessizce yutulması ZAMANLAYICIDA arıza demek: silinmiş bir toplu işin öğesi
 * için gelen çağrı hiçbir şey yapmadan başarılı dönerdi ve kuyruk, var olmayan
 * bir işi bitmiş sanarak ilerlerdi. `ErrNotFound` bunu görünür kılıyor.
 */

func TestOgeKapisi_OlmayanOgeSessizceGecmez(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	yok := uuid.New()

	require.ErrorIs(t, f.store.Claim(ctx, yok), runbatch.ErrNotFound)
	require.ErrorIs(t, f.store.Requeue(ctx, yok), runbatch.ErrNotFound)
	require.ErrorIs(t, f.store.MarkFinished(ctx, yok, runbatch.ItemSucceeded, ""),
		runbatch.ErrNotFound)
}

/*
OLMAYAN PROJE İLE TOPLU İŞ BAŞLATILAMAZ.

Toplu iş ekranı projeleri listeden seçtiriyor; ama seçim yapılırken başka bir
sekmede silinmiş bir proje listede kalmış olabilir. Yabancı anahtar ihlali ham
hâliyle dönseydi kullanıcı 500 görürdü — oysa yapması gereken belli: listeyi
tazeleyip yeniden seçmek.

Bu bekçi mutasyon taramasında GÖRÜNMÜYORDU: desen, üstündeki `ErrDuplicateProject`
bekçisiyle birlikte tek eşleşmeye düşüyor ve bu satır hiç denenmiyordu.
*/
func TestCreate_OlmayanProjeReddedilir(t *testing.T) {
	f := setup(t, "alfa")

	_, err := f.store.Create(context.Background(), f.workflowID, "iş",
		[]uuid.UUID{uuid.New()})
	require.ErrorIs(t, err, runbatch.ErrProjectNotFound)
}
