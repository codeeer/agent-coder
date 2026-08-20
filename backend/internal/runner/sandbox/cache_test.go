package sandbox

import (
	"testing"

	"github.com/docker/docker/api/types/mount"
	"github.com/stretchr/testify/require"
)

/*
 * Bağımlılık önbelleği bağları (spec 027).
 *
 * Bu testler Docker'sız: bağların DOĞRU KURULDUĞU, Docker'ın onları nasıl
 * uyguladığından ayrı bir sorudur ve en ucuz burada cevaplanır.
 */

// Önbellek yokken HİÇ mount üretilmemeli.
//
// Boş bir dilim yerine nil dönmek önemli: `HostConfig.Mounts` alanı hiç
// yazılmamış olur ve önbellek kapalıyken container tanımı bugünküyle BİREBİR
// aynı kalır (spec 027 H2).
func TestCacheMounts_OnbellekYokkenHicMountUretmez(t *testing.T) {
	require.Nil(t, cacheMounts(nil))
	require.Nil(t, cacheMounts([]CacheMount{}))
}

func TestCacheMounts_HerOnbellekIcinBirBagUretir(t *testing.T) {
	got := cacheMounts([]CacheMount{
		{Volume: "onbellek-maven", Target: "/home/agent/.m2/repository"},
		{Volume: "onbellek-npm", Target: "/home/agent/.npm/_cacache"},
	})

	require.Len(t, got, 2)
	require.Equal(t, mount.TypeVolume, got[0].Type)
	require.Equal(t, "onbellek-maven", got[0].Source)
	require.Equal(t, "/home/agent/.m2/repository", got[0].Target)
	require.Equal(t, "onbellek-npm", got[1].Source)
}

/*
YAZILABİLİR OLMALI.

Önbelleğin kendi kendine ısınması, koşunun oraya YAZABİLMESİNE bağlı. Salt
okunur bağlanırsa Maven ilk artefaktı yazamadan düşer ve önbellek sonsuza
kadar boş kalır — üstelik hata, sebebiyle ilgisiz bir yerden gelir.

Salt okunur mod spec 027'de bilerek kapsam dışı; açılacaksa ayrı bir karar.
*/
func TestCacheMounts_SaltOkunurDegildir(t *testing.T) {
	got := cacheMounts([]CacheMount{{Volume: "v", Target: "/t"}})

	require.Len(t, got, 1)
	require.False(t, got[0].ReadOnly, "önbellek yazılabilir olmalı")
}
