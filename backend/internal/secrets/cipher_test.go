package secrets

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// testKey, geçerli bir 32 baytlık anahtar üretir.
func testKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, KeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(key)
}

func TestNewCipher_GecerliAnahtar(t *testing.T) {
	c, err := NewCipher(testKey(t))
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestNewCipher_GecersizAnahtarlarReddedilir(t *testing.T) {
	tests := []struct {
		ad      string
		anahtar string
		hata    error
	}{
		{"boş", "", ErrKeyMissing},
		{"base64 değil", "bu-base64-degil!!!", nil},
		{"çok kısa", base64.StdEncoding.EncodeToString([]byte("kisa")), ErrKeySize},
		{"çok uzun", base64.StdEncoding.EncodeToString(make([]byte, 64)), ErrKeySize},
		{"31 bayt", base64.StdEncoding.EncodeToString(make([]byte, 31)), ErrKeySize},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			c, err := NewCipher(tt.anahtar)
			require.Error(t, err)
			require.Nil(t, c)
			if tt.hata != nil {
				require.ErrorIs(t, err, tt.hata)
			}
		})
	}
}

func TestSifreleCoz_TurTesti(t *testing.T) {
	c, err := NewCipher(testKey(t))
	require.NoError(t, err)

	tests := []struct {
		ad    string
		girdi string
	}{
		{"tipik api anahtarı", "sk-or-v1-abc123def456"},
		{"boş metin", ""},
		{"tek karakter", "x"},
		{"türkçe karakterler", "şifreli-değer-ĞÜİÖÇ"},
		{"uzun değer", strings.Repeat("uzun-anahtar-", 500)},
		{"satır sonu içeren", "satir1\nsatir2\r\nsatir3"},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			blob, err := c.EncryptString(tt.girdi)
			require.NoError(t, err)

			// Şifreli çıktı düz metni içermemeli.
			//
			// Kısa girdilerde bu kontrol ANLAMSIZ ve KARARSIZDIR: 30 baytlık
			// rastgele çıktının içinde tek bir "x" baytının bulunma ihtimali
			// ~%11'dir ve test bazen kendiliğinden kırmızıya döner. Sızıntı
			// kontrolü ancak rastgele denk gelmeyecek uzunlukta anlamlıdır.
			if len(tt.girdi) >= 8 {
				require.NotContains(t, string(blob), tt.girdi)
			}

			cozulmus, err := c.DecryptString(blob)
			require.NoError(t, err)
			require.Equal(t, tt.girdi, cozulmus)
		})
	}
}

func TestSifrele_AyniGirdiFarkliCiktiUretir(t *testing.T) {
	c, err := NewCipher(testKey(t))
	require.NoError(t, err)

	const gizli = "sk-or-v1-ayni-deger"

	ilk, err := c.EncryptString(gizli)
	require.NoError(t, err)
	ikinci, err := c.EncryptString(gizli)
	require.NoError(t, err)

	// Rastgele nonce sayesinde iki çıktı farklı olmalı — aksi halde
	// aynı anahtarın kullanıldığı veritabanından anlaşılabilirdi.
	require.NotEqual(t, ilk, ikinci)

	// Yine de ikisi de aynı değere çözülür.
	a, err := c.DecryptString(ilk)
	require.NoError(t, err)
	b, err := c.DecryptString(ikinci)
	require.NoError(t, err)
	require.Equal(t, gizli, a)
	require.Equal(t, gizli, b)
}

func TestCoz_YanlisAnahtarBasarisiz(t *testing.T) {
	dogru, err := NewCipher(testKey(t))
	require.NoError(t, err)
	yanlis, err := NewCipher(testKey(t))
	require.NoError(t, err)

	blob, err := dogru.EncryptString("sk-or-v1-gizli")
	require.NoError(t, err)

	cozulmus, err := yanlis.DecryptString(blob)
	require.ErrorIs(t, err, ErrDecrypt)
	require.Empty(t, cozulmus)
}

func TestCoz_TekBitBozulmaYakalanir(t *testing.T) {
	c, err := NewCipher(testKey(t))
	require.NoError(t, err)

	orijinal, err := c.EncryptString("sk-or-v1-dokunulmamis")
	require.NoError(t, err)

	// Blob'un her baytını tek tek bozup hepsinin yakalandığını doğruluyoruz.
	// GCM kimlik doğrulaması sayesinde hiçbiri sessizce geçmemeli.
	for i := range orijinal {
		bozuk := make([]byte, len(orijinal))
		copy(bozuk, orijinal)
		bozuk[i] ^= 0x01

		_, err := c.Decrypt(bozuk)
		require.Error(t, err, "bayt %d bozulduğunda hata beklenirdi", i)
	}
}

func TestCoz_BozukGirdilerReddedilir(t *testing.T) {
	c, err := NewCipher(testKey(t))
	require.NoError(t, err)

	tests := []struct {
		ad   string
		blob []byte
		hata error
	}{
		{"nil", nil, ErrMalformed},
		{"boş", []byte{}, ErrMalformed},
		{"çok kısa", []byte{0x01, 0x02, 0x03}, ErrMalformed},
		{"sadece sürüm baytı", []byte{currentVersion}, ErrMalformed},
		{"bilinmeyen sürüm", append([]byte{0xFF}, make([]byte, 40)...), ErrVersion},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			_, err := c.Decrypt(tt.blob)
			require.ErrorIs(t, err, tt.hata)
		})
	}
}

func TestSifrele_SurumBaytiYazilir(t *testing.T) {
	c, err := NewCipher(testKey(t))
	require.NoError(t, err)

	blob, err := c.EncryptString("deger")
	require.NoError(t, err)
	require.Equal(t, currentVersion, blob[0])
}

func TestMask(t *testing.T) {
	tests := []struct {
		girdi    string
		beklenen string
	}{
		{"sk-or-v1-abcdef1234fd36", "fd36"},
		{"12345", "2345"},
		{"1234", ""}, // çok kısa: hiçbir şey açığa çıkarma
		{"abc", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.girdi, func(t *testing.T) {
			require.Equal(t, tt.beklenen, Mask(tt.girdi))
		})
	}
}
