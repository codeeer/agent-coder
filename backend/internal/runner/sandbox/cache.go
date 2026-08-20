package sandbox

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
)

/*
 * Koşular arası bağımlılık önbelleği (spec 027).
 *
 * Container'a bağlanan tek şey burasıdır. Sandbox'ın geri kalanında BAĞLAMA
 * YOK kuralı sürüyor (bkz. Create içindeki not): oradaki yasak HOST'TAKİ BİR
 * DOSYAYA dayanıyordu ve uzak Docker host'ta çalışmıyordu. Adlandırılmış
 * volume daemon tarafında yaşar, host'ta bir dosya aramaz — yasağın gerekçesi
 * burada doğmuyor.
 *
 * Bu paket önbelleğin NE OLDUĞUNU bilmez: Maven'ı, npm'i ve yolları `runner`
 * paketi tanımlar. Buraya yalnızca "şu volume şu yola bağlansın" gelir.
 */

// ErrCache, önbellek volume'ü hazırlanamadı.
//
// Ayrı bir sentinel: çağıran bunu ÖLÜMCÜL SAYMAZ. Önbellek bir hızlandırıcıdır,
// önkoşul değil — koşu önbelleksiz sürer (spec 027, Hata Durumları).
var ErrCache = errors.New("bağımlılık önbelleği hazırlanamadı")

// CacheMount, kalıcı bir önbelleğin container içindeki bağlanma noktası.
type CacheMount struct {
	// Volume, Docker'ın yönettiği adlandırılmış volume'ün adı.
	Volume string
	// Target, container içindeki yol.
	//
	// Bu yol İMAJDA VAR OLMALI ve agent'a ait olmalıdır. Docker boş bir
	// volume'ü ancak yol imajda varsa onun sahipliğiyle doldurur; yoksa mount
	// noktasını root'a açar ve agent kendi önbelleğine yazamaz (ölçüldü:
	// spec 027 T04).
	Target string
}

/*
EnsureCaches, önbellek volume'lerinin var olduğundan emin olur.

AYRI BİR ADIM OLMASI BİLİNÇLİ. Docker, bağlanan volume'ü container yaratılırken
kendiliğinden de oluşturur; ona bırakılsaydı önbellek hatası container yaratma
hatasına karışırdı ve çağıran "önbelleksiz devam et" kararını veremezdi — koşu,
hızlandırıcısı yüzünden kaybolurdu.

Idempotent: var olan volume için `VolumeCreate` mevcut olanı döner.
*/
func (m *Manager) EnsureCaches(ctx context.Context, caches []CacheMount) error {
	for _, c := range caches {
		if _, err := m.docker.VolumeCreate(ctx, volume.CreateOptions{
			Name:   c.Volume,
			Labels: map[string]string{LabelManaged: "true"},
		}); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrCache, c.Volume, err)
		}
	}
	return nil
}

/*
cacheMounts, önbellek bağlarını Docker'ın beklediği biçime çevirir.

Önbellek yoksa NİL döner, boş dilim değil: `HostConfig.Mounts` alanı hiç
yazılmamış olur ve önbellek kapalıyken container tanımı bugünküyle birebir
aynı kalır (spec 027 H2).
*/
func cacheMounts(caches []CacheMount) []mount.Mount {
	if len(caches) == 0 {
		return nil
	}

	out := make([]mount.Mount, 0, len(caches))
	for _, c := range caches {
		out = append(out, mount.Mount{
			Type:   mount.TypeVolume,
			Source: c.Volume,
			Target: c.Target,
			// Salt okunur DEĞİL: önbelleğin ısınması koşunun oraya
			// yazabilmesine bağlı. Salt okunur mod spec 027'de kapsam dışı.
			ReadOnly: false,
		})
	}
	return out
}
