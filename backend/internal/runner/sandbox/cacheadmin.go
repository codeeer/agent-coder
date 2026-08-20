package sandbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
)

/*
 * Önbellek bakımı: boyut ve temizleme (spec 027 H3).
 *
 * HEPSİ YARDIMCI CONTAINER İÇİNDE. Backend volume'ün içini göremez: Docker
 * host uzaktaysa `/var/lib/docker` backend'in dosya sisteminde yoktur.
 * `os.Stat` ile bakmak tek makineli kurulumda çalışıp uzak host'ta sessizce
 * yanlış cevap veren türden bir hata olurdu.
 *
 * Yardımcı container'ın dört kuralı — üçü güvenlik veya doğruluk gerekçeli:
 *
 *   1. HER ZAMAN RUNNER İMAJI. Volume'e ilk dokunan container boş volume'ün
 *      sahipliğini belirler; başka bir imaj ilk mount'u yaparsa sahiplik
 *      `root`a kayar ve agent bir daha kendi önbelleğine yazamaz.
 *   2. ENTRYPOINT GEÇERSİZ KILINIR. İmajın kendi giriş noktası motoru
 *      başlatır; yardımcının işi tek bir komut.
 *   3. AĞSIZ. İşi yalnızca yerel dosyaları okumak. Çıkış denetiminin
 *      (spec 020) etrafından dolaşan bir yol açılmamalı.
 *   4. KISA ÖMÜRLÜ. Komut biter, çıktı okunur, container silinir.
 */

// helperTimeout, yardımcı container'a tanınan süre.
//
// Büyük bir önbellekte `du` dakikalar sürebilir; ama sonsuz beklemek, tek bir
// asılı bakım işleminin bakım ucunu kalıcı olarak kilitlemesi demek olurdu.
const helperTimeout = 5 * time.Minute

// ErrCacheAdmin, önbellek bakım işlemi yapılamadı.
var ErrCacheAdmin = errors.New("önbellek bakımı yapılamadı")

/*
ErrCacheInUse, önbellek şu anda bir container tarafından kullanılıyor.

ÖLÇÜLDÜ: Docker, bağlı bir volume'ü silmeyi 409 ile reddediyor. Çağıranın bunu
ayırt etmesi gerekiyor çünkü kullanıcıya söylenecek şey farklı — "bakım
başarısız" değil, "şu an yapılamaz, koşular bitince tekrar deneyin".

Çalışan koşu kapısı (`Active()`) bunu çoğu zaman önler ama YETMEZ: kapı ile
silme arasında bir koşu başlayabilir ve sahipsiz kalmış bir container da
volume'ü tutuyor olabilir. Sentinel, o yarışın sonucunu anlaşılır kılıyor.
*/
var ErrCacheInUse = errors.New("önbellek şu anda kullanılıyor")

/*
CacheSize, önbelleğin bayt cinsinden boyutunu döner.

Volume hiç oluşturulmamışsa (-1, nil) döner: "henüz kullanılmadı" ile "boş"
AYRI ŞEYLER (spec 027 H3). Sıfır dönseydi arayüz hiç çalıştırılmamış bir
önbelleği "0 B" diye gösterir ve kullanıcı onu boşaltılmış sanırdı.
*/
func (m *Manager) CacheSize(ctx context.Context, image string, c CacheMount) (int64, error) {
	var (
		exists bool
		err    error
	)
	if exists, err = m.volumeExists(ctx, c.Volume); err != nil {
		return 0, err
	}
	if !exists {
		return -1, nil
	}

	// `du -sb`: makinece okunur, bayt cinsinden. İnsan için biçimlenmiş çıktı
	// ("1,2G") yerelleştirmeye ve yuvarlamaya tabi olurdu ve temizleme
	// onayındaki "ne kadar yer boşalacak" sayısı ondan besleniyor.
	out, err := m.runHelper(ctx, image, []CacheMount{c},
		"du -sb "+shellQuote(c.Target)+" 2>/dev/null || true")
	if err != nil {
		return 0, err
	}
	return parseDuBytes(out)
}

/*
ClearCache, önbelleği boşaltır ve boşalan bayt sayısını döner.

Volume SİLİNİP YENİDEN OLUŞTURULUYOR, içi tek tek silinmiyor: hem daha hızlı,
hem de sahiplik imajdan yeniden kuruluyor. İçeriği silseydik, bir gün kök
dizinin sahipliği bozulursa bunu düzeltecek bir yol kalmazdı.

Dönen sayı çağıranın kayda geçirdiği değerdir; onay şeridi de aynı sayıyı
kullanıcıya gösterir.
*/
func (m *Manager) ClearCache(ctx context.Context, image string, c CacheMount) (int64, error) {
	// Boyut ÖNCE ölçülür: silindikten sonra kaç bayt gittiğini söyleyemezdik.
	boyut, err := m.CacheSize(ctx, image, c)
	if err != nil {
		return 0, err
	}
	if boyut < 0 {
		// Hiç kullanılmamış: silinecek bir şey yok, hata da değil.
		return 0, nil
	}

	if err := m.docker.VolumeRemove(ctx, c.Volume, true); err != nil {
		if errdefs.IsConflict(err) {
			return 0, fmt.Errorf("%w: bir çalıştırma onu bağlamış durumda", ErrCacheInUse)
		}
		return 0, fmt.Errorf("%w: %s silinemedi: %w", ErrCacheAdmin, c.Volume, err)
	}
	if err := m.EnsureCaches(ctx, []CacheMount{c}); err != nil {
		return 0, err
	}
	return boyut, nil
}

// volumeExists, volume'ün oluşturulmuş olup olmadığı.
func (m *Manager) volumeExists(ctx context.Context, name string) (bool, error) {
	if _, err := m.docker.VolumeInspect(ctx, name); err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("%w: %s okunamadı: %w", ErrCacheAdmin, name, err)
	}
	return true, nil
}

/*
runHelper, yardımcı container'da tek bir kabuk komutu çalıştırıp çıktısını
döner.

Container HER YOLDA silinir; bakım işlemi arkasında container bırakmaz.
*/
func (m *Manager) runHelper(ctx context.Context, image string, caches []CacheMount,
	komut string,
) (string, error) {
	ctx, iptal := context.WithTimeout(ctx, helperTimeout)
	defer iptal()

	created, err := m.docker.ContainerCreate(ctx,
		&container.Config{
			Image: image,
			// İmajın kendi giriş noktası motoru başlatır; yardımcının işi bu değil.
			Entrypoint: []string{"sh", "-c"},
			Cmd:        []string{komut},
			Labels:     map[string]string{LabelManaged: "true"},
		},
		&container.HostConfig{
			// Yerel dosya okumaktan başka işi yok.
			NetworkMode:   "none",
			Mounts:        cacheMounts(caches),
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
		},
		nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("%w: yardımcı container oluşturulamadı: %w", ErrCacheAdmin, err)
	}
	defer func() {
		_ = m.docker.ContainerRemove(context.WithoutCancel(ctx), created.ID,
			container.RemoveOptions{Force: true})
	}()

	if err := m.docker.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("%w: yardımcı container başlatılamadı: %w", ErrCacheAdmin, err)
	}

	bitti, hata := m.docker.ContainerWait(ctx, created.ID, container.WaitConditionNotRunning)
	select {
	case err := <-hata:
		if err != nil {
			return "", fmt.Errorf("%w: yardımcı container beklenemedi: %w", ErrCacheAdmin, err)
		}
	case <-bitti:
	case <-ctx.Done():
		return "", fmt.Errorf("%w: yardımcı container zamanında bitmedi", ErrCacheAdmin)
	}

	logs, err := m.docker.ContainerLogs(ctx, created.ID,
		container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", fmt.Errorf("%w: yardımcı container çıktısı okunamadı: %w", ErrCacheAdmin, err)
	}
	defer logs.Close()

	var stdout, stderr strings.Builder
	if _, err := stdcopy.StdCopy(&stdout, &stderr, logs); err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("%w: yardımcı container çıktısı çözülemedi: %w", ErrCacheAdmin, err)
	}
	return stdout.String(), nil
}

/*
shellQuote, bir yolu kabuk için güvenli hâle getirir.

Yollar bugün sabit; yine de kaçırılıyor çünkü bu fonksiyonun girdisi bir gün
ayardan gelirse, o değişikliği yapan kişinin burayı hatırlaması gerekmesin.
*/
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
