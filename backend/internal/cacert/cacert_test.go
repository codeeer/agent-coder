package cacert

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Kurumsal kök sertifikanın hangi kaynaktan geldiği.
 *
 * İki kaynak var ve sırası önemli: arayüzden girilen ayar kazanır, ortam
 * değişkeniyle verilen dosya yedekte kalır. Yanlış çözülen bir öncelik,
 * kullanıcının arayüzden girdiği sertifikanın hiç kullanılmaması demek — ve
 * bunu ancak SSL denetimi yapan bir ağda, ilk başarısız çağrıda fark eder.
 */

func kokPEM(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "certfmt", "testdata", "kok.pem"))
	require.NoError(t, err)
	return string(b)
}

// dosya, verilen içerikle geçici bir dosya yazar ve yolunu döner.
func dosya(t *testing.T, ad string, icerik []byte) string {
	t.Helper()
	yol := filepath.Join(t.TempDir(), ad)
	require.NoError(t, os.WriteFile(yol, icerik, 0o600))
	return yol
}

func TestResolve_HicKaynakYoksaTanimsiz(t *testing.T) {
	pem, kaynak := NewResolver(nil, "").Resolve()

	require.Empty(t, pem)
	require.Equal(t, SourceNone, kaynak)
}

func TestResolve_YalnizcaAyarVarsaAyardan(t *testing.T) {
	r := NewResolver(func() string { return kokPEM(t) }, "")

	pem, kaynak := r.Resolve()
	require.Equal(t, SourceSettings, kaynak)
	require.Contains(t, pem, "BEGIN CERTIFICATE")
}

func TestResolve_YalnizcaOrtamDegiskeniVarsaYedekten(t *testing.T) {
	r := NewResolver(nil, dosya(t, "kok.pem", []byte(kokPEM(t))))

	pem, kaynak := r.Resolve()
	require.Equal(t, SourceEnv, kaynak)
	require.Contains(t, pem, "BEGIN CERTIFICATE")
}

/*
AYAR YEDEĞİ YENER.

Kullanıcı arayüzden bir sertifika girdiyse kastı açıktır. Ortam değişkeni
kazansaydı, arayüzden girilen değer sessizce hiç kullanılmaz ve kullanıcı
"girdim ama çalışmıyor" derdi — üstelik ekranda tanımlı görünürken.
*/
func TestResolve_AyarYedegiYener(t *testing.T) {
	r := NewResolver(func() string { return kokPEM(t) },
		dosya(t, "kok.pem", []byte(kokPEM(t))))

	_, kaynak := r.Resolve()
	require.Equal(t, SourceSettings, kaynak, "arayüzden girilen değer kazanmalı")
}

/*
AYAR HER ÇAĞRIDA YENİDEN OKUNUR.

Sertifika arayüzden değiştirildiğinde sonraki çağrı yenisini kullanmalı;
açılışta bir kez okunsaydı kullanıcının sunucuyu yeniden başlatması gerekirdi
(spec 017 H1, H5).
*/
func TestResolve_AyarHerCagridaOkunur(t *testing.T) {
	deger := ""
	r := NewResolver(func() string { return deger }, "")

	_, kaynak := r.Resolve()
	require.Equal(t, SourceNone, kaynak, "ön koşul: başlangıçta ayar boş")

	deger = kokPEM(t)
	_, kaynak = r.Resolve()
	require.Equal(t, SourceSettings, kaynak, "ayar değişikliği yeniden başlatma istememeli")

	// Ayar geri alınınca da kaynak düşmeli.
	deger = ""
	_, kaynak = r.Resolve()
	require.Equal(t, SourceNone, kaynak)
}

/*
BOZUK AYAR SUNUCUYU DÜŞÜRMEZ, YEDEĞE DÜŞER.

Ayar kaydedilirken zaten doğrulanıyor; buraya bozuk bir değerin gelmesi bir
tutarsızlıktır. Yine de sunucu çökmemeli: yedeği olan bir kurulum çalışmaya
devam etmeli.
*/
func TestResolve_BozukAyarYedegeDuser(t *testing.T) {
	r := NewResolver(func() string { return "bu bir sertifika değil" },
		dosya(t, "kok.pem", []byte(kokPEM(t))))

	pem, kaynak := r.Resolve()
	require.Equal(t, SourceEnv, kaynak, "bozuk ayar yüzünden yedek devre dışı kalmamalı")
	require.Contains(t, pem, "BEGIN CERTIFICATE")
}

/*
OKUNAMAYAN YEDEK DOSYA SUNUCUYU AÇILMAZ YAPMAZ.

Yanlış yapılandırılmış bir yedek yüzünden sunucunun hiç açılmaması, ayarı
arayüzden düzeltme imkânını da ortadan kaldırırdı — kullanıcı kilitlenirdi.
*/
func TestNewResolver_OkunamayanDosyaCokmez(t *testing.T) {
	r := NewResolver(nil, filepath.Join(t.TempDir(), "olmayan.pem"))

	pem, kaynak := r.Resolve()
	require.Empty(t, pem)
	require.Equal(t, SourceNone, kaynak)
}

// Sertifika İÇERMEYEN bir yedek dosya da aynı şekilde sessizce devre dışı
// kalır; kaynak yokmuş gibi devam edilir.
func TestNewResolver_SertifikasizDosyaSessizceAtlanir(t *testing.T) {
	r := NewResolver(nil, dosya(t, "cop.txt", []byte("merhaba dünya")))

	_, kaynak := r.Resolve()
	require.Equal(t, SourceNone, kaynak)
}

/*
DER BİÇİMİNDEKİ SERTİFİKA DA KABUL EDİLİR.

Kullanıcı Windows'tan dışa aktardığında dosya çoğu zaman DER oluyor.
Çözülen değer HER ZAMAN normalleştirilmiş PEM: sertifikayı tüketen her yer
yalnızca PEM okuyor.
*/
func TestNewResolver_DERBicimiPEMeCevrilir(t *testing.T) {
	der, err := os.ReadFile(filepath.Join("..", "certfmt", "testdata", "kok.der"))
	require.NoError(t, err)

	r := NewResolver(nil, dosya(t, "kok.der", der))

	pem, kaynak := r.Resolve()
	require.Equal(t, SourceEnv, kaynak)
	require.True(t, strings.HasPrefix(strings.TrimSpace(pem), "-----BEGIN CERTIFICATE-----"),
		"tüketiciler yalnızca PEM okuyor")
}

// PEM, kaynağı önemsemeyen çağıranlar için aynı değeri verir.
func TestPEM_ResolveIleAyniDegeri(t *testing.T) {
	r := NewResolver(func() string { return kokPEM(t) }, "")

	pem, _ := r.Resolve()
	require.Equal(t, pem, r.PEM())
}

/*
KAYNAK ADI KULLANICIYA GÖSTERİLİYOR.

İki kaynak birden mümkünken "tanımlı" demek yetmiyor; hangisinin geçerli
olduğu yazılmalı (spec 017: Davranış kuralları).
*/
func TestSourceString_UcDurumunDaKarsiligiVar(t *testing.T) {
	require.Equal(t, "ayarlardan", SourceSettings.String())
	require.Equal(t, "ortam değişkeninden", SourceEnv.String())
	require.Equal(t, "tanımsız", SourceNone.String())
}
