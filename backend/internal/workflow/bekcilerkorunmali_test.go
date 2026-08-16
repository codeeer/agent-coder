package workflow_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Akış kapılarının HATA TÜRÜ.
 *
 * Mutasyon taramasının çıktısı: aşağıdaki `ErrNotFound` çevirileri koddan
 * kaldırıldığında hiçbir test kırmızıya dönmüyordu.
 *
 * Çeviri olmadan ham `pgx.ErrNoRows` yukarı çıkıyor; HTTP katmanı onu
 * tanımadığı için 404 yerine 500 "işlem tamamlanamadı" dönüyor. Silinmiş bir
 * akışın bağlantısına tıklayan kullanıcı, sistemin bozulduğunu sanıyor.
 */

func TestGet_OlmayanAkis(t *testing.T) {
	f := setup(t)

	_, err := f.store.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, workflow.ErrNotFound)
}

func TestVersion_OlmayanSurum(t *testing.T) {
	f := setup(t)

	_, err := f.store.Version(context.Background(), uuid.New())
	require.ErrorIs(t, err, workflow.ErrNotFound)
}

/*
OLMAYAN AKIŞA SÜRÜM KAYDEDİLEMEZ.

Tuval ekranı açıkken akış başka bir sekmede silinmiş olabilir. Kaydet'e
basıldığında kullanıcının duyması gereken şey "bu akış artık yok"; 500 hatası
ona çalışmasını kaybettiğini değil, kaydetmenin bozuk olduğunu düşündürür ve
tekrar tekrar denemesine yol açar.
*/
func TestSaveVersion_OlmayanAkis(t *testing.T) {
	f := setup(t)

	_, err := f.store.SaveVersion(context.Background(), uuid.New(), f.graph())
	require.ErrorIs(t, err, workflow.ErrNotFound)
}
