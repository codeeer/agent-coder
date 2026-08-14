package certfmt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Test verileri UYDURULMAZ, openssl ile üretilir (specs/017 → T00).
 *
 * Elle yazılmış bir "PKCS#7 gibi görünen" bayt dizisi, ayrıştırıcının gerçek
 * dosyalarda çalıştığını göstermez — asıl risk tam olarak gerçek üreticilerin
 * ürettiği yapıyı okuyabilmek.
 */

func oku(t *testing.T, ad string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", ad))
	require.NoError(t, err)
	return b
}

// konular, PEM metnindeki sertifikaların CN'lerini döndürür.
func konular(t *testing.T, pemMetin string) []string {
	t.Helper()
	var out []string
	rest := []byte(pemMetin)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return out
		}
		require.Equal(t, "CERTIFICATE", block.Type, "çıktıda sertifika dışında blok olmamalı")
		c, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		out = append(out, c.Subject.CommonName)
	}
}

func TestToPEM_PEMOlduguGibiKabulEdilir(t *testing.T) {
	got, err := ToPEM(oku(t, "kok.pem"))
	require.NoError(t, err)
	require.Equal(t, []string{"Ornek Kurum Kok CA"}, konular(t, got))
}

func TestToPEM_DERPEMeCevrilir(t *testing.T) {
	got, err := ToPEM(oku(t, "kok.der"))
	require.NoError(t, err)
	require.Equal(t, []string{"Ornek Kurum Kok CA"}, konular(t, got))
}

// Kullanıcı bir portaldan gövdeyi başlıksız kopyaladığında böyle geliyor.
func TestToPEM_CiplakBase64Kabul(t *testing.T) {
	got, err := ToPEM(oku(t, "kok.base64"))
	require.NoError(t, err)
	require.Equal(t, []string{"Ornek Kurum Kok CA"}, konular(t, got))
}

func TestToPEM_DortBicimAyniSonucuVerir(t *testing.T) {
	pemDen, err := ToPEM(oku(t, "kok.pem"))
	require.NoError(t, err)
	derDen, err := ToPEM(oku(t, "kok.der"))
	require.NoError(t, err)
	b64ten, err := ToPEM(oku(t, "kok.base64"))
	require.NoError(t, err)

	require.Equal(t, pemDen, derDen, "DER ve PEM aynı sertifikayı vermeli")
	require.Equal(t, pemDen, b64ten, "çıplak base64 ve PEM aynı sertifikayı vermeli")
}

// Windows CA'ları zinciri sık sık PKCS#7 olarak verir; kök ve ara birlikte gelir.
func TestToPEM_PKCS7ZincirinTamaminiGetirir(t *testing.T) {
	for _, ad := range []string{"zincir.p7b", "zincir.p7b.der"} {
		t.Run(ad, func(t *testing.T) {
			got, err := ToPEM(oku(t, ad))
			require.NoError(t, err)
			require.Equal(t,
				[]string{"Ornek Kurum Kok CA", "Ornek Kurum Ara CA"},
				konular(t, got),
				"zincirdeki iki sertifika da alınmalı")
		})
	}
}

func TestToPEM_CoklaPEMZinciriKorunur(t *testing.T) {
	got, err := ToPEM(oku(t, "zincir.pem"))
	require.NoError(t, err)
	require.Len(t, konular(t, got), 2)
}

/*
 * Özel anahtar ATILIR.
 *
 * Fixture depoya konmuyor: gerçek bir özel anahtarı depoya yazmak sır
 * tarayıcılarını tetikler ve incelemede alarm verir. Anahtar burada, testin
 * içinde üretiliyor.
 */
func TestToPEM_OzelAnahtarAtilir(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	girdi := append(keyPEM, oku(t, "kok.pem")...)

	got, err := ToPEM(girdi)
	require.NoError(t, err)
	require.Equal(t, []string{"Ornek Kurum Kok CA"}, konular(t, got))
	require.NotContains(t, got, "PRIVATE KEY", "özel anahtar çıktıya sızmamalı")
}

func TestToPEM_SertifikaOlmayanIcerikReddedilir(t *testing.T) {
	_, err := ToPEM(oku(t, "cop.txt"))
	require.ErrorIs(t, err, ErrNoCertificate)
}

func TestToPEM_BosIcerikReddedilir(t *testing.T) {
	_, err := ToPEM(nil)
	require.ErrorIs(t, err, ErrNoCertificate)
}

// Base64 olarak çözülebilen ama sertifika olmayan içerik de reddedilmeli.
func TestToPEM_GecerliBase64AmaSertifikaDegil(t *testing.T) {
	_, err := ToPEM([]byte("aGVsbG8gd29ybGQgYnUgc2VydGlmaWthIGRlZ2ls"))
	require.ErrorIs(t, err, ErrNoCertificate)
}
