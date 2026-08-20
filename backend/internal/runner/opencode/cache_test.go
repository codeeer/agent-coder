package opencode

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
)

/*
 * Bağımlılık önbelleği eşlemesi (spec 027).
 *
 * Burada sorulan tek şey: ayar açık/kapalıyken hangi bağlar kuruluyor.
 * Docker'ın bunlarla ne yaptığı `sandbox` paketinin testlerinde ölçülüyor.
 */

/*
KAPALIYKEN HİÇ BAĞ YOK.

Spec 027 H2'nin çekirdeği: varsayılan kapalı ve kapalıyken container tanımı
özellikten önceki hâliyle birebir aynı. Boş dilim değil NİL dönmeli ki
`HostConfig.Mounts` alanı hiç yazılmasın.
*/
func TestDependencyCaches_KapaliykenHicBagYok(t *testing.T) {
	require.Nil(t, dependencyCaches(false))
}

func TestDependencyCaches_AcikkenMavenVeNpmBaglanir(t *testing.T) {
	got := dependencyCaches(true)
	require.Len(t, got, 2)

	hedefler := map[string]string{}
	for _, c := range got {
		hedefler[c.Target] = c.Volume
		require.NotEmpty(t, c.Volume, "volume adı boş olamaz")
	}

	require.Equal(t, mavenCacheVolume, hedefler["/home/agent/.m2/repository"])
	require.Equal(t, npmCacheVolume, hedefler["/home/agent/.npm/_cacache"])
}

/*
VOLUME ADLARI SABİT VE AYIRT EDİCİ.

Önbelleğin değeri koşular arasında AYNI volume'e dönmekten geliyor; ad koşuya,
projeye veya tarihe göre değişseydi her koşu kendi boş önbelleğini açar ve
özellik hiçbir şey hızlandırmazdı.

Ayrıca ikisi AYRI volume: spec 027 H3 ekosistem başına boyut ve temizleme
istiyor.
*/
func TestDependencyCaches_VolumeAdlariSabitVeAyri(t *testing.T) {
	require.NotEqual(t, mavenCacheVolume, npmCacheVolume)
	require.Equal(t, dependencyCaches(true), dependencyCaches(true))
}

/*
HAZIRLIK DÜŞERSE KOŞU ÖNBELLEKSİZ SÜRER — VE SEBEBİ DUYULUR.

Spec 027'nin hata kuralı: önbellek hızlandırıcıdır, önkoşul değil. Ama sessiz
de değil — kullanıcı hızlanma beklerken gördüğü yavaş koşuyu hata sanmamalı.

Bu ikisi BİRLİKTE test ediliyor çünkü tek başına her biri yanlış: sessizce
devam etmek de, koşuyu düşürmek de spec'e aykırı.
*/
func TestCachesOrNone_HataVarsaOnbelleksizDevamVeUyari(t *testing.T) {
	var olaylar []runner.Event
	emit := func(e runner.Event) { olaylar = append(olaylar, e) }

	got := cachesOrNone(dependencyCaches(true), errors.New("volume açılamadı"), emit)

	require.Nil(t, got, "hata hâlinde hiç önbellek bağlanmamalı")
	require.Len(t, olaylar, 1, "sebep olay akışına düşmeli")
	require.Equal(t, runner.LevelWarn, olaylar[0].Level)
	require.Contains(t, olaylar[0].Message, "volume açılamadı",
		"uyarı ASIL SEBEBİ taşımalı; 'önbellek kullanılamıyor' tek başına "+
			"kullanıcıyı sebebi aramaya bırakır")
}

func TestCachesOrNone_HataYoksaOnbellekAynenGecer(t *testing.T) {
	var olaylar []runner.Event
	emit := func(e runner.Event) { olaylar = append(olaylar, e) }

	istenen := dependencyCaches(true)
	got := cachesOrNone(istenen, nil, emit)

	require.Equal(t, istenen, got)
	require.Empty(t, olaylar, "sorun yokken kullanıcıya bir şey söylenmez")
}
