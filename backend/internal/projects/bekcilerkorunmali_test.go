package projects_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/projects"
	"github.com/agent-coder/backend/internal/testutil"
)

func newStore(t *testing.T) *projects.Store {
	t.Helper()
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "runs", "projects")
	return projects.NewStore(pool)
}

/*
 * Proje kapıları.
 *
 * Mutasyon taramasının çıktısı: aşağıdaki iki bekçi kaldırıldığında hiçbir
 * test kırmızıya dönmüyordu.
 */

/*
OLMAYAN PROJE 404 OLMALI, 500 DEĞİL.

`pgx.ErrNoRows` çevrilmezse HTTP katmanı onu tanımaz ve "işlem tamamlanamadı"
der. Kullanıcı silinmiş bir projenin bağlantısına tıkladığında sistemin
bozulduğunu sanar; oysa yapması gereken tek şey listeye dönmek.
*/
func TestGet_OlmayanProje(t *testing.T) {
	s := newStore(t)

	_, err := s.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, projects.ErrNotFound)
}

/*
ADSIZ PROJE OLUŞTURULAMAZ — ve boşluk ad sayılmaz.

Ad, projenin her ekranda göründüğü tek tanıtıcısı. Boş geçseydi listede,
seçicide ve raporda adsız bir satır belirir ve kullanıcı onu diğerlerinden
ayırt edemezdi. Kırpma da bu kuralın parçası: "   " ad değildir.
*/
func TestNormalize_BosAdReddedilir(t *testing.T) {
	for _, ad := range []string{"", "   ", "\t\n"} {
		in := projects.Input{Name: ad, RepoURL: "https://example.com/r.git"}
		require.ErrorIs(t, in.Normalize(), projects.ErrEmptyName,
			"ad %q kabul edilmemeli", ad)
	}
}

// Karşı yön: geçerli ad kırpılarak kabul edilir. Yalnızca reddi sınamak
// yetmez — her şeyi reddeden bir kontrol de o testi geçerdi.
func TestNormalize_GecerliAdKirpilarakKabulEdilir(t *testing.T) {
	in := projects.Input{Name: "  Ödeme servisi  ", RepoURL: "https://example.com/r.git"}
	require.NoError(t, in.Normalize())
	require.Equal(t, "Ödeme servisi", in.Name)
}
