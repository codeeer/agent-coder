package opencode

import (
	"testing"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/stretchr/testify/require"
)

/*
 * Vekil ölçüm düzeneğinin testleri (scripts/sizinti-analizi/).
 *
 * Bu testler ürün davranışını değil, ÖLÇÜMÜN KENDİSİNİ koruyor. Sızıntı
 * analizinin tüm sonucu "trafik gerçekten vekilden geçti mi" varsayımına
 * dayanıyor; değişkenlerden biri sessizce düşerse ölçüm "hiçbir yere istek
 * yok" gibi görünür ve yanlış bir rapor üretirdi.
 */

func TestApplyProxy_BosDegerHicbirSeyEklemez(t *testing.T) {
	env := map[string]string{}
	applyProxy(env, "")
	require.Empty(t, env)
}

// Küçük ve büyük harfli biçim İKİSİ BİRDEN gerekiyor: curl yalnızca küçük
// harfliyi, Node büyük harfliyi okuyor. Yalnız biri konsaydı isteklerin bir
// kısmı vekile hiç uğramazdı.
func TestApplyProxy_HerIkiHarfBiciminiDeTanimlar(t *testing.T) {
	env := map[string]string{}
	applyProxy(env, "http://sizinti-mitm:8080")

	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		require.Equal(t, "http://sizinti-mitm:8080", env[k], "%s tanımlı olmalı", k)
	}
	require.Equal(t, "1", env["NODE_USE_ENV_PROXY"])
}

// Sağlık kontrolü 127.0.0.1'e curl atıyor; vekile yönlenseydi container hiç
// hazır sayılmaz ve ölçüm hiç başlamazdı.
func TestApplyProxy_LocalhostVekilDisinda(t *testing.T) {
	env := map[string]string{}
	applyProxy(env, "http://sizinti-mitm:8080")

	require.Contains(t, env["NO_PROXY"], "127.0.0.1")
	require.Contains(t, env["no_proxy"], "127.0.0.1")
}

// JVM hiçbir *_PROXY değişkenini okumaz — Maven'lı koşu ancak sistem
// özellikleriyle ölçülebiliyor.
func TestApplyProxy_MavenSistemOzellikleri(t *testing.T) {
	env := map[string]string{}
	applyProxy(env, "http://sizinti-mitm:8080")

	require.Contains(t, env["MAVEN_OPTS"], "-Dhttps.proxyHost=sizinti-mitm")
	require.Contains(t, env["MAVEN_OPTS"], "-Dhttps.proxyPort=8080")
}

// Süre sınırı ile vekil aynı değişkeni paylaşıyor; ikincisi birincisini
// EZMEMELİ, yoksa Maven ölçümü ulaşılamayan adreste dakikalarca asılı kalır.
func TestApplyProxy_MevcutMavenOptsKorunur(t *testing.T) {
	env := buildEnv(runner.Request{Packages: runner.PackageRegistry{
		MavenRegistry: "https://m.local/", TimeoutSec: 60,
	}}, "http://sizinti-mitm:8080")

	require.Contains(t, env["MAVEN_OPTS"], "aether.connector.connectTimeout=60000")
	require.Contains(t, env["MAVEN_OPTS"], "-Dhttps.proxyHost=sizinti-mitm")
}
