package opencode

import (
	"context"
	"errors"
	"fmt"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runner/sandbox"
)

/*
 * Koşular arası bağımlılık önbelleği (spec 027).
 *
 * BURADA OLMASININ SEBEBİ: bu dosya container'ın nasıl kurulduğunu bilen
 * yerdir. `runs` paketi `sandbox`'ı tanımıyor ve tanımamalı — o katmana giden
 * tek bilgi `Request.DependencyCache` boolean'ı.
 *
 * SÖZLEŞMENİN ÖTEKİ YARISI `runner/Dockerfile`'DA. Buradaki yolların imajda
 * ÖNCEDEN OLUŞTURULMUŞ ve `agent`'a ait olması gerekiyor; aksi halde boş
 * volume `root:root` bağlanır ve agent kendi önbelleğine yazamaz. İki taraf
 * ayrışırsa hiçbir şey hata vermez, önbellek sessizce çalışmaz.
 */

const (
	// mavenCacheVolume / npmCacheVolume, önbelleklerin volume adları.
	//
	// SABİT olmaları özelliğin kendisi: ad koşuya veya projeye göre değişseydi
	// her koşu kendi boş önbelleğini açar ve hiçbir şey hızlanmazdı.
	//
	// AYRI olmaları spec 027 H3'ten: kullanıcı ekosistem başına boyut görüyor
	// ve ayrı ayrı temizleyebiliyor.
	mavenCacheVolume = "agent-coder-cache-maven"
	npmCacheVolume   = "agent-coder-cache-npm"

	mavenCachePath = "/home/agent/.m2/repository"
	npmCachePath   = "/home/agent/.npm/_cacache"
)

/*
dependencyCaches, bağlanacak önbellekleri döner.

Kapalıyken NİL döner (boş dilim değil): `HostConfig.Mounts` alanı hiç yazılmaz
ve container tanımı özellik eklenmeden önceki hâliyle birebir aynı kalır
(spec 027 H2).

Yalnızca `repository` ve `_cacache` bağlanıyor; `~/.m2/settings.xml` ve
`~/.npmrc` koşu başına yazılmaya devam ediyor (spec 014/018) ve alt dizine
bağlanan volume onları etkilemiyor — ölçüldü.
*/
func dependencyCaches(enabled bool) []sandbox.CacheMount {
	if !enabled {
		return nil
	}
	return []sandbox.CacheMount{
		{Volume: mavenCacheVolume, Target: mavenCachePath},
		{Volume: npmCacheVolume, Target: npmCachePath},
	}
}

/*
cachesOrNone, önbellek hazırlığının sonucuna göre ne bağlanacağını söyler.

AYRI BİR FONKSİYON OLMASI BİLİNÇLİ. Buradaki karar — "hata olursa koşuyu
düşürme, önbelleksiz devam et, ama sebebi duyur" — spec 027'nin hata
kuralının tamamı. `Run` içine gömülü kalsaydı, sınanması gerçek bir Docker
hatası üretmeyi gerektirirdi ve pratikte hiç sınanmazdı.

Uyarı ASIL SEBEBİ taşır: "önbellek kullanılamıyor" tek başına kullanıcıyı
sebebi aramaya bırakır.
*/
func cachesOrNone(caches []sandbox.CacheMount, err error, emit runner.EventFunc) []sandbox.CacheMount {
	if err == nil {
		return caches
	}
	emit(runner.Event{
		Level: runner.LevelWarn,
		Message: "bağımlılık önbelleği kullanılamıyor, koşu önbelleksiz sürüyor: " +
			err.Error(),
	})
	return nil
}

// cacheByID, kimlikten önbelleğe.
func cacheByID(id runner.CacheID) (sandbox.CacheMount, string, bool) {
	switch id {
	case runner.CacheMaven:
		return sandbox.CacheMount{Volume: mavenCacheVolume, Target: mavenCachePath}, "Maven", true
	case runner.CacheNPM:
		return sandbox.CacheMount{Volume: npmCacheVolume, Target: npmCachePath}, "npm", true
	default:
		return sandbox.CacheMount{}, "", false
	}
}

// cacheOrder, arayüzde ve yanıtta kullanılan sabit sıra.
//
// SABİT: sıra map üzerinden gelseydi her istekte değişir ve iki satır
// birbiriyle yer değiştirdiği için kullanıcı yanlış kutuya tıklardı.
var cacheOrder = []runner.CacheID{runner.CacheMaven, runner.CacheNPM}

/*
CacheStatus, tanımlı bütün önbelleklerin durumunu döner (runner.CacheAdmin).

Boyut, volume'ü bağlayan kısa ömürlü bir yardımcı container içinde ölçülür:
backend volume'ün içini göremez (Docker host uzak olabilir).
*/
func (r *Runner) CacheStatus(ctx context.Context) ([]runner.CacheInfo, error) {
	out := make([]runner.CacheInfo, 0, len(cacheOrder))
	for _, id := range cacheOrder {
		mount, label, _ := cacheByID(id)

		boyut, err := r.sandbox.CacheSize(ctx, r.image, mount)
		if err != nil {
			return nil, err
		}

		// -1: hiç oluşturulmamış. "Boş" ile karıştırılmasın diye Used=false.
		out = append(out, runner.CacheInfo{
			ID: id, Label: label,
			SizeBytes: max(boyut, 0),
			Used:      boyut >= 0,
		})
	}
	return out, nil
}

// ClearCache, önbelleği boşaltır (runner.CacheAdmin).
func (r *Runner) ClearCache(ctx context.Context, id runner.CacheID) (int64, error) {
	mount, _, ok := cacheByID(id)
	if !ok {
		return 0, fmt.Errorf("%w: %q", runner.ErrUnknownCache, id)
	}

	bosalan, err := r.sandbox.ClearCache(ctx, r.image, mount)
	if errors.Is(err, sandbox.ErrCacheInUse) {
		/*
		 * Sandbox'ın sentinel'i paket sınırında runner'ınkine çevriliyor:
		 * HTTP katmanı `sandbox`'ı tanımıyor ve tanımamalı.
		 *
		 * KULLANICIYA DÖNÜK İPUCU DA BURADA üretiliyor: volume adı çalışma
		 * ortamının iç bilgisi ve yalnızca bu katman biliyor. Uç katmanına
		 * taşınsaydı adlandırma iki yerde yaşardı.
		 */
		return 0, fmt.Errorf("%w: hangi çalışma ortamının tuttuğunu görmek için: "+
			"docker ps -a --filter volume=%s", runner.ErrCacheBusy, mount.Volume)
	}
	return bosalan, err
}

// VerifyCache, önbelleği tarar ve bozuk artefaktları siler (runner.CacheAdmin).
func (r *Runner) VerifyCache(ctx context.Context, id runner.CacheID) (runner.VerifyResult, error) {
	mount, _, ok := cacheByID(id)
	if !ok {
		return runner.VerifyResult{}, fmt.Errorf("%w: %q", runner.ErrUnknownCache, id)
	}

	// npm'in kendi bütünlük denetimi var; `_cacache` biçimini ürünün bilmesi
	// gerekmiyor. Maven tarafında ise özet karşılaştırması bize ait.
	verify := r.sandbox.VerifyCache
	if id == runner.CacheNPM {
		verify = r.sandbox.VerifyNPMCache
	}

	sonuc, err := verify(ctx, r.image, mount)
	if err != nil {
		return runner.VerifyResult{}, err
	}
	return runner.VerifyResult{
		Checked:      sonuc.Checked,
		Mismatched:   sonuc.Mismatched,
		Unverifiable: sonuc.Unverifiable,
		Removed:      sonuc.Removed,
	}, nil
}
