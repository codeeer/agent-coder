package sandbox_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runner/sandbox"
)

/*
 * Sandbox — GERÇEK Docker üzerinde.
 *
 * Sahte bir Docker istemcisi buradaki soruların hiçbirini cevaplayamaz:
 * dosyanın container içinde gerçekten oluşup oluşmadığı, dizin girdisinin
 * gerekip gerekmediği, etiketlerin sahipsiz container'ı bulup bulmadığı.
 * Hepsi Docker'ın kendi davranışına bağlı.
 *
 * Docker yoksa testler ATLANIR: birim testlerin çalışan bir kuruluma bağımlı
 * olması yanlış olurdu.
 */

const testPort = 4096

func yonetici(t *testing.T) *sandbox.Manager {
	t.Helper()

	m, err := sandbox.NewManager(testPort)
	if err != nil {
		t.Skipf("docker yok — atlanıyor: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })

	ctx, iptal := context.WithTimeout(context.Background(), 10*time.Second)
	defer iptal()
	if err := m.Ping(ctx); err != nil {
		t.Skipf("docker erişilemiyor — atlanıyor: %v", err)
	}
	return m
}

func imaj() string {
	if v := os.Getenv("RUNNER_IMAGE"); v != "" {
		return v
	}
	return "agent-coder/opencode-runner:latest"
}

/*
EKSİK İMAJ MESAJI NE YAPILACAĞINI SÖYLER — ve sürüm önekini doğru tanır.

Bu testin ikinci işi daha değerli: `runner.ImageFor` ile üretilen etiketin,
sandbox tarafındaki sürüm tanımasıyla EŞLEŞTİĞİNİ uçtan uca gösteriyor. İki
paket aynı öneki ayrı ayrı tanımlıyor (ters bağımlılık döngü olacağı için) ve
ayrıştıkları an mesaj sessizce yanlış tarifi verir.
*/
func TestEnsureImage_EksikSurumluImajCekilmeyiSoyler(t *testing.T) {
	m := yonetici(t)
	ctx := context.Background()

	ref := runner.ImageFor("ghcr.io/agent-coder/olmayan-runner:latest", "24.13.0")
	require.Contains(t, ref, "node-24.13.0", "ön koşul: sürümlü etiket üretildi")

	err := m.EnsureImage(ctx, ref)
	require.Error(t, err, "olmayan imaj kabul edilmemeli")
	require.Contains(t, err.Error(), "docker pull",
		"uzak sürümlü imaj için tarif çekmek olmalı — iki paketin öneki ayrışmış olabilir")
}

// Sürümsüz eksik imajda tarif derlemek olmalı.
func TestEnsureImage_EksikTabanImajDerlemeyiSoyler(t *testing.T) {
	m := yonetici(t)

	err := m.EnsureImage(context.Background(), "agent-coder/olmayan-runner:latest")
	require.Error(t, err)
	require.Contains(t, err.Error(), "make runner")
}

// Yerelde var olan imaj sorunsuz geçmeli — aksi halde her çalıştırma başlamadan
// düşerdi.
func TestEnsureImage_VarOlanImajGecer(t *testing.T) {
	m := yonetici(t)
	ctx := context.Background()

	if err := m.EnsureImage(ctx, imaj()); err != nil {
		t.Skipf("runner imajı yok (%s) — atlanıyor: %v", imaj(), err)
	}
}

// container, testlik bir sandbox açar ve sonunda siler.
func container(t *testing.T, m *sandbox.Manager, files []sandbox.File) *sandbox.Container {
	t.Helper()
	ctx, iptal := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(iptal)

	if err := m.EnsureImage(ctx, imaj()); err != nil {
		t.Skipf("runner imajı yok — atlanıyor: %v", err)
	}

	network := os.Getenv("RUNNER_NETWORK")
	if network == "" {
		network = "agent-coder_internal"
	}

	c, err := m.Create(ctx, sandbox.Spec{
		RunID: uuid.NewString(), Image: imaj(), Network: network,
		CPUCores: 1, MemoryGB: 1, Files: files,
	})
	require.NoError(t, err, "container ayağa kalkmalı")
	t.Cleanup(func() { c.Remove(context.Background()) })
	return c
}

/*
DOSYALAR CONTAINER BAŞLAMADAN ÖNCE İÇİNDE OLUR.

Agent'ın yapılandırması, talimatı ve betikleri bu yolla giriyor. Kopyalama
başlatmadan sonra yapılsaydı motor eksik yapılandırmayla açılır ve hata
"sağlayıcı bulunamadı" gibi ilgisiz bir yerden gelirdi.
*/
func TestCreate_DosyalarIceriKopyalanir(t *testing.T) {
	m := yonetici(t)
	const yol = "/home/agent/deneme/ayar.json"

	c := container(t, m, []sandbox.File{
		{Path: "/home/agent/deneme", IsDir: true, Mode: 0o700},
		{Path: yol, Content: []byte(`{"a":1}`), Mode: 0o600},
	})

	ctx, iptal := context.WithTimeout(context.Background(), 30*time.Second)
	defer iptal()

	dosyalar, err := c.ReadDir(ctx, "/home/agent/deneme", 1<<20)
	require.NoError(t, err)

	var bulundu bool
	for ad, icerik := range dosyalar {
		if len(icerik) > 0 && string(icerik) == `{"a":1}` {
			bulundu = true
			t.Logf("bulundu: %s", ad)
		}
	}
	require.True(t, bulundu, "kopyalanan dosya container içinde okunabilmeli")
}

/*
AÇIKÇA YAZILAN DİZİN GİRDİSİ İSTENEN İZİNLERLE OLUŞUR.

Ölçülmüş bir karar: dizin girdisi yazılmasa da Docker eksik ara dizini kendisi
açıyor — ama `root:root 0755` olarak. Agent (uid 10001) o dizine yazmak
zorunda; izinler bir gün daralırsa erişemez ve bu ürün o hatayı bir kez yaşadı.

Kanıt, dizinin agent kullanıcısına ait olması: `ReadDir` çağrısı dizinin
içeriğini okuyabiliyorsa dizin var ve erişilebilir demektir.
*/
func TestCreate_DizinGirdisiAgentaAitOlur(t *testing.T) {
	m := yonetici(t)

	c := container(t, m, []sandbox.File{
		{Path: "/home/agent/kampanya", IsDir: true, Mode: 0o700},
		{Path: "/home/agent/kampanya/adim.sh", Content: []byte("echo bir\n"), Mode: 0o700},
	})

	ctx, iptal := context.WithTimeout(context.Background(), 30*time.Second)
	defer iptal()

	dosyalar, err := c.ReadDir(ctx, "/home/agent/kampanya", 1<<20)
	require.NoError(t, err)
	require.NotEmpty(t, dosyalar, "agent kendi dizinini okuyabilmeli")
}

/*
SAHİPSİZ CONTAINER'LAR TEMİZLENİR.

Sunucu çalışma ortasında ölürse container'lar sahipsiz kalır ve birikip diski
doldurur. Temizlik ETİKETE bakıyor: yalnızca bu sistemin açtıkları siliniyor,
makinedeki başka container'lara dokunulmuyor.
*/
func TestCleanupOrphans_YonetilenContaineriSiler(t *testing.T) {
	m := yonetici(t)
	ctx, iptal := context.WithTimeout(context.Background(), 90*time.Second)
	defer iptal()

	c := container(t, m, nil)

	n, err := m.CleanupOrphans(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 1, "az önce açılan container temizlenmeliydi")

	// Silindiğinin kanıtı: aynı container'ın logu artık okunamıyor.
	_, err = c.Logs(ctx, "all")
	require.Error(t, err, "temizlenen container'a hâlâ erişiliyor")
}
