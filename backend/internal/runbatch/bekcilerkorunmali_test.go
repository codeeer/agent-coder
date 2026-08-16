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
