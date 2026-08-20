package runs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Bağımlılık önbelleği ayarı (spec 027 H2).
 *
 * Sorulan tek şey: ayar hangi durumda koşuya AÇIK olarak geçiyor. Önbelleğin
 * Docker tarafındaki davranışı `runner/sandbox` testlerinde ölçülüyor.
 */

/*
AYAR TANIMSIZSA KAPALI.

Belirsizlikte kapalı tarafa düşmek bilinçli: kapalı hâl özellikten önceki
davranış. Açık tarafa düşülseydi, erişimciyi bağlamayı unutan bir kurulum
haberi olmadan koşular arası paylaşılan yazılabilir bir alan kazanırdı.
*/
func TestDependencyCache_AyarYoksaKapali(t *testing.T) {
	m := &Manager{}
	require.False(t, m.dependencyCache())
}

func TestDependencyCache_AyarKapaliysaKapali(t *testing.T) {
	m := &Manager{limits: Limits{DependencyCache: func() bool { return false }}}
	require.False(t, m.dependencyCache())
}

func TestDependencyCache_AyarAciksaAcik(t *testing.T) {
	m := &Manager{limits: Limits{DependencyCache: func() bool { return true }}}
	require.True(t, m.dependencyCache())
}

/*
AYAR HER OKUMADA YENİDEN SORULUR.

Yeniden başlatma gerektirmemesinin kanıtı (spec 027 H2 üçüncü kriteri):
erişimci bir fonksiyon ve değeri önbelleğe alınmıyor. Değer bir kez okunup
saklansaydı, ayarı kapatan kullanıcı sunucuyu yeniden başlatana kadar
koşuların hâlâ önbelleğe yazdığını göremezdi.
*/
func TestDependencyCache_DegisiklikYenidenBaslatmaGerektirmez(t *testing.T) {
	acik := false
	m := &Manager{limits: Limits{DependencyCache: func() bool { return acik }}}

	require.False(t, m.dependencyCache())
	acik = true
	require.True(t, m.dependencyCache(), "ayar değişince sonraki okuma yeni değeri vermeli")
}
