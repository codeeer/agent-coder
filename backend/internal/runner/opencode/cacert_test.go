package opencode

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
)

/*
 * Kurumsal kök sertifikanın çalışma ortamına ulaşması.
 *
 * Eskiden sertifika host'tan `:ro` bağlanıyordu ve burada bağlamanın salt
 * okunurluğu sınanıyordu. Artık dosya container'a KOPYALANIYOR (spec 017);
 * sınanacak şey de değişti: doğru dosya, doğru yol ve sertifikayı GÖSTEREN
 * ortam değişkenleri.
 */

const ornekPEM = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

func TestBuildEnv_SertifikaYokkenDegiskenTanimlanmaz(t *testing.T) {
	env := buildEnv(runner.Request{})

	require.NotContains(t, env, "NODE_EXTRA_CA_CERTS")
	require.NotContains(t, env, "GIT_SSL_CAINFO")
	require.NotContains(t, env, "CURL_CA_BUNDLE")
}

/*
 * CURL_CA_BUNDLE bu testin ASIL sebebi.
 *
 * Spec 017 provasında ölçüldü: sertifika tanıtılmış bir kurulumda node ve git
 * çalışırken curl `unable to get local issuer certificate` ile düşüyordu.
 * Üçünün birden tanımlandığı burada kilitleniyor ki aynı boşluk geri
 * gelmesin.
 */
func TestBuildEnv_SertifikaVarkenUcDegiskenDeTanimlanir(t *testing.T) {
	env := buildEnv(runner.Request{CACert: ornekPEM})

	require.Equal(t, runner.CACertPath, env["NODE_EXTRA_CA_CERTS"])
	require.Equal(t, runner.CACertPath, env["GIT_SSL_CAINFO"])
	require.Equal(t, runner.CACertPath, env["CURL_CA_BUNDLE"],
		"curl kurumsal sertifikayı tanımalı — ölçülmüş bir boşluğun kaydı")
}

// Değişkenler YOLU gösterir, içeriği değil: sertifika ortam değişkenine
// yazılsaydı agent `env` çıktısında görürdü ve çok satırlı değer kabuk
// tarafından bozulurdu.
func TestBuildEnv_SertifikaIcerigiOrtamDegiskenineYazilmaz(t *testing.T) {
	env := buildEnv(runner.Request{CACert: ornekPEM})

	for k, v := range env {
		require.NotContains(t, v, "BEGIN CERTIFICATE",
			"sertifika içeriği %q değişkenine sızmış", k)
	}
}

func TestCACertFile_BosSertifikaDosyaUretmez(t *testing.T) {
	_, ok := runner.CACertFile("")
	require.False(t, ok)

	_, ok = runner.CACertFile("   \n\t ")
	require.False(t, ok, "yalnızca boşluktan oluşan değer de dosya üretmemeli")
}

func TestCACertFile_DogruYolVeMod(t *testing.T) {
	f, ok := runner.CACertFile(ornekPEM)
	require.True(t, ok)

	require.Equal(t, runner.CACertPath, f.Path)
	require.Equal(t, ornekPEM, string(f.Content))
	// 0644: sır değil ve agent'ın çalıştırdığı her araç okuyabilmeli.
	require.Equal(t, int64(0o644), f.Mode)
}

/*
 * Maven süre sınırı.
 *
 * Özellik adları ÖLÇÜLEREK bulundu: ulaşılamayan bir adrese karşı varsayılanla
 * 98 saniye, `aether.connector.connectTimeout=3000` ile 31 saniye harcandı.
 * `maven.wagon.http.*` hiçbir etki yapmıyor (Maven 3.9 wagon kullanmıyor).
 * Bu test o ölçümün kaydı: ad değişirse ayar sessizce etkisiz kalırdı.
 */
func TestBuildEnv_MavenSureSiniri(t *testing.T) {
	env := buildEnv(runner.Request{Packages: runner.PackageRegistry{
		MavenRegistry: "https://m.local/", TimeoutSec: 60,
	}})

	require.Contains(t, env["MAVEN_OPTS"], "aether.connector.connectTimeout=60000")
	require.Contains(t, env["MAVEN_OPTS"], "aether.connector.requestTimeout=60000")
}

func TestBuildEnv_MavenKapaliykenOptsYok(t *testing.T) {
	// Yalnızca npm tanımlı: Maven'a ait hiçbir şey yazılmamalı.
	env := buildEnv(runner.Request{Packages: runner.PackageRegistry{
		NPMRegistry: "https://n.local/", TimeoutSec: 60,
	}})
	require.NotContains(t, env, "MAVEN_OPTS")
}

func TestBuildEnv_SureSinirsizkenOptsYok(t *testing.T) {
	env := buildEnv(runner.Request{Packages: runner.PackageRegistry{
		MavenRegistry: "https://m.local/",
	}})
	require.NotContains(t, env, "MAVEN_OPTS")
}
