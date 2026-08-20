package sandbox_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	dockerapi "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner/sandbox"
)

/*
 * Bağımlılık önbelleği — GERÇEK Docker üzerinde (spec 027).
 *
 * Buradaki sorular Docker'ın kendi davranışına bağlı ve sahte bir istemci
 * hiçbirini cevaplayamaz: boş bir volume'ün sahipliği nereden geliyor, alt
 * dizine bağlanan volume üst dizindeki dosyayı eziyor mu, ikinci koşu
 * birincinin bıraktığını gerçekten buluyor mu.
 *
 * İKİ AYRI SORU, İKİ AYRI KURULUM:
 *
 *   1. Sandbox bağları DOĞRU KURUYOR MU  → `sandbox.Create` + `ContainerInspect`
 *   2. İMAJ + boş volume ne üretiyor      → doğrudan SDK, `sleep` entrypoint'i
 *
 * İkincisi koşu container'ıyla ölçülemez: runner imajının giriş noktası
 * yapılandırma dosyaları olmadan hemen çıkıyor ve `exec` tutunamıyor. Zaten
 * sorulan şey de sandbox'ın davranışı değil, imajın kendisi — ve bu kurulum
 * planın "yardımcı container" deseninin (runner imajı, entrypoint geçersiz,
 * ağsız) ilk uygulaması oluyor.
 */

const (
	m2Hedef  = "/home/agent/.m2/repository"
	npmHedef = "/home/agent/.npm/_cacache"
)

/*
docker istemcisi TEST İÇİNDE ayrıca kuruluyor.

`sandbox.Manager` kendi istemcisini dışarı vermiyor ve vermemeli. Testin
sorduğu sorular Docker'ın kendi yüzeyine ait; `docker` CLI'ye çıkmak ise
testleri `golang` imajının içinde koşulamaz yapardı — orada CLI yok,
yalnızca soket var.
*/
func istemci(t *testing.T) *client.Client {
	t.Helper()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Skipf("docker yok — atlanıyor: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	return cli
}

// onbellekVolume, testlik bir volume adı üretir ve sonunda siler.
func onbellekVolume(t *testing.T, ek string) string {
	t.Helper()
	ad := "agent-coder-test-cache-" + ek + "-" + uuid.NewString()[:8]
	cli := istemci(t)
	t.Cleanup(func() { _ = cli.VolumeRemove(context.Background(), ad, true) })
	return ad
}

/*
yardimci, runner imajından ağsız ve uzun ömürlü bir container açar.

Plan'daki yardımcı container deseninin ta kendisi: aynı imaj (sahiplik kaysın
diye başka imaj kullanılmaz), `Entrypoint` geçersiz kılınır (imajın kendi
giriş noktası çalışmamalı), ağ yok.
*/
func yardimci(t *testing.T, cli *client.Client, caches []sandbox.CacheMount) string {
	t.Helper()
	ctx, iptal := context.WithTimeout(context.Background(), 60*time.Second)
	defer iptal()

	var mounts []mount.Mount
	for _, c := range caches {
		mounts = append(mounts, mount.Mount{
			Type: mount.TypeVolume, Source: c.Volume, Target: c.Target,
		})
	}

	created, err := cli.ContainerCreate(ctx,
		&dockerapi.Config{Image: imaj(), Entrypoint: []string{"sleep"}, Cmd: []string{"300"}},
		&dockerapi.HostConfig{NetworkMode: "none", Mounts: mounts},
		nil, nil, "agent-coder-test-yardimci-"+uuid.NewString()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cli.ContainerRemove(context.Background(), created.ID,
			dockerapi.RemoveOptions{Force: true})
	})

	require.NoError(t, cli.ContainerStart(ctx, created.ID, dockerapi.StartOptions{}))
	return created.ID
}

// calistir, container içinde kabuk komutu çalıştırıp çıktısını döner.
func calistir(t *testing.T, cli *client.Client, id, komut string) string {
	t.Helper()
	ctx, iptal := context.WithTimeout(context.Background(), 30*time.Second)
	defer iptal()

	ex, err := cli.ContainerExecCreate(ctx, id, dockerapi.ExecOptions{
		Cmd: []string{"sh", "-c", komut},
		// Tty: çıktı çoğullanmaz, düz okunur — testin ihtiyacı bu kadarı.
		Tty: true, AttachStdout: true, AttachStderr: true,
	})
	require.NoError(t, err)

	ek, err := cli.ContainerExecAttach(ctx, ex.ID, dockerapi.ExecAttachOptions{Tty: true})
	require.NoError(t, err)
	defer ek.Close()

	cikti, err := io.ReadAll(ek.Reader)
	require.NoError(t, err)

	incele, err := cli.ContainerExecInspect(ctx, ex.ID)
	require.NoError(t, err)
	require.Zero(t, incele.ExitCode, "komut başarılı olmalı: %s\nçıktı: %s", komut, cikti)

	return string(cikti)
}

func TestEnsureCaches_AyniVolumeIkiKezOlusturulabilir(t *testing.T) {
	m := yonetici(t)
	ctx, iptal := context.WithTimeout(context.Background(), 30*time.Second)
	defer iptal()

	caches := []sandbox.CacheMount{{Volume: onbellekVolume(t, "idem"), Target: m2Hedef}}

	require.NoError(t, m.EnsureCaches(ctx, caches))
	require.NoError(t, m.EnsureCaches(ctx, caches),
		"idempotent olmalı: var olan volume hata vermemeli")
}

// Sandbox, verilen önbellekleri container'a gerçekten bağlar.
func TestCreate_OnbellekMountlariKurulur(t *testing.T) {
	m := yonetici(t)
	cli := istemci(t)
	ctx, iptal := context.WithTimeout(context.Background(), 120*time.Second)
	defer iptal()

	if err := m.EnsureImage(ctx, imaj()); err != nil {
		t.Skipf("runner imajı yok — atlanıyor: %v", err)
	}

	caches := []sandbox.CacheMount{
		{Volume: onbellekVolume(t, "m2"), Target: m2Hedef},
		{Volume: onbellekVolume(t, "npm"), Target: npmHedef},
	}
	require.NoError(t, m.EnsureCaches(ctx, caches))

	c := onbellekliContainer(t, m, caches)

	bilgi, err := cli.ContainerInspect(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, bilgi.Mounts, 2)

	hedefler := map[string]string{}
	for _, mt := range bilgi.Mounts {
		hedefler[mt.Destination] = mt.Name
		require.True(t, mt.RW, "önbellek yazılabilir bağlanmalı: %s", mt.Destination)
	}
	require.Equal(t, caches[0].Volume, hedefler[m2Hedef])
	require.Equal(t, caches[1].Volume, hedefler[npmHedef])
}

/*
ÖNBELLEK KAPALIYKEN CONTAINER TANIMI DEĞİŞMEZ.

Spec 027 H2'nin "birebir aynı" iddiasının Docker tarafındaki kanıtı.
*/
func TestCreate_OnbellekKapaliykenHicMountYok(t *testing.T) {
	m := yonetici(t)
	cli := istemci(t)
	ctx, iptal := context.WithTimeout(context.Background(), 120*time.Second)
	defer iptal()

	if err := m.EnsureImage(ctx, imaj()); err != nil {
		t.Skipf("runner imajı yok — atlanıyor: %v", err)
	}

	c := onbellekliContainer(t, m, nil)

	bilgi, err := cli.ContainerInspect(ctx, c.ID)
	require.NoError(t, err)
	require.Empty(t, bilgi.Mounts, "önbellek kapalıyken hiç mount olmamalı")
}

// onbellekliContainer, verilen önbelleklerle bir koşu container'ı açar.
func onbellekliContainer(t *testing.T, m *sandbox.Manager, caches []sandbox.CacheMount) *sandbox.Container {
	t.Helper()
	ctx, iptal := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(iptal)

	network := os.Getenv("RUNNER_NETWORK")
	if network == "" {
		network = "agent-coder_internal"
	}

	c, err := m.Create(ctx, sandbox.Spec{
		RunID: uuid.NewString(), Image: imaj(), Network: network,
		CPUCores: 1, MemoryGB: 1, Caches: caches,
	})
	require.NoError(t, err, "container ayağa kalkmalı")
	t.Cleanup(func() { c.Remove(context.Background()) })
	return c
}

/*
BOŞ VOLUME'ÜN SAHİBİ AGENT OLUR — VE BU KENDİLİĞİNDEN OLMAZ (R1).

Docker, boş bir adlandırılmış volume'ü ancak bağlandığı yol İMAJDA VARSA o
yolun içeriği ve sahipliğiyle doldurur. Yol imajda yoksa mount noktasını
root'a açar; agent kendi önbelleğine yazamaz ve önbellek sessizce hiç
çalışmaz — koşu başarılı görünür ama her seferinde yeniden indirir.

Bu test, `runner/Dockerfile`'daki `mkdir` + `chown` satırı silinirse kırılır.
Ölçüldü: dizin imajda yokken sahiplik `root:root` çıkıyor.
*/
func TestRunnerImaji_OnbellekDizinleriAgentaAitVeYazilabilir(t *testing.T) {
	m := yonetici(t)
	cli := istemci(t)
	ctx, iptal := context.WithTimeout(context.Background(), 120*time.Second)
	defer iptal()

	if err := m.EnsureImage(ctx, imaj()); err != nil {
		t.Skipf("runner imajı yok — atlanıyor: %v", err)
	}

	caches := []sandbox.CacheMount{
		{Volume: onbellekVolume(t, "sahip-m2"), Target: m2Hedef},
		{Volume: onbellekVolume(t, "sahip-npm"), Target: npmHedef},
	}
	id := yardimci(t, cli, caches)

	for _, hedef := range []string{m2Hedef, npmHedef} {
		sahip := strings.TrimSpace(calistir(t, cli, id, "stat -c %U "+hedef))
		require.Equal(t, "agent", sahip, "%s agent'a ait olmalı", hedef)

		calistir(t, cli, id, "touch "+hedef+"/deneme")
	}
}

/*
ALT DİZİNE BAĞLANAN VOLUME `settings.xml`'İ EZMEZ.

Depo yapılandırması `~/.m2/settings.xml`'e koşu başına yazılıyor (spec 014/018)
ve önbellek onun ALT dizinine bağlanıyor. İkisi çakışsaydı önbellek açıldığı an
kurumsal paket deposu yapılandırması kaybolur, koşu dış depoya çıkmaya çalışır
ve sebebi ilgisiz bir yerden gelirdi.
*/
func TestRunnerImaji_OnbellekSettingsDosyasiniEzmez(t *testing.T) {
	m := yonetici(t)
	cli := istemci(t)
	ctx, iptal := context.WithTimeout(context.Background(), 120*time.Second)
	defer iptal()

	if err := m.EnsureImage(ctx, imaj()); err != nil {
		t.Skipf("runner imajı yok — atlanıyor: %v", err)
	}

	id := yardimci(t, cli, []sandbox.CacheMount{
		{Volume: onbellekVolume(t, "settings"), Target: m2Hedef},
	})

	cikti := calistir(t, cli, id,
		`echo "<settings/>" > /home/agent/.m2/settings.xml && cat /home/agent/.m2/settings.xml`)
	require.Contains(t, cikti, "<settings/>",
		"önbellek bağlıyken settings.xml yazılabilmeli")
}

/*
İKİNCİ KOŞU BİRİNCİNİN BIRAKTIĞINI BULUR.

Özelliğin varlık sebebi bu. Ayrı iki container, aynı volume.
*/
func TestRunnerImaji_OnbellekKosularArasindaKalir(t *testing.T) {
	m := yonetici(t)
	cli := istemci(t)
	ctx, iptal := context.WithTimeout(context.Background(), 180*time.Second)
	defer iptal()

	if err := m.EnsureImage(ctx, imaj()); err != nil {
		t.Skipf("runner imajı yok — atlanıyor: %v", err)
	}

	caches := []sandbox.CacheMount{{Volume: onbellekVolume(t, "kalici"), Target: m2Hedef}}

	birinci := yardimci(t, cli, caches)
	calistir(t, cli, birinci, "echo artefakt > "+m2Hedef+"/kutuphane.jar")
	require.NoError(t, cli.ContainerRemove(ctx, birinci, dockerapi.RemoveOptions{Force: true}))

	ikinci := yardimci(t, cli, caches)
	require.Contains(t, calistir(t, cli, ikinci, "cat "+m2Hedef+"/kutuphane.jar"), "artefakt",
		"ikinci koşu birincinin indirdiğini bulmalı")
	require.Equal(t, "agent",
		strings.TrimSpace(calistir(t, cli, ikinci, "stat -c %U "+m2Hedef+"/kutuphane.jar")),
		"kalan artefaktın sahibi de agent olmalı")
}
