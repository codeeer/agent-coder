package credentials

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Kimlik bilgisi doğrulaması.
 *
 * Ayrımın kendisi ürün kararı: "değer yanlış" ile "şu an denenemedi" farklı
 * şeyler. Birinde kullanıcı anahtarı düzeltmeli, diğerinde tekrar denemeli.
 * İkisi tek hataya düşseydi, ağı bir an kopan kullanıcı çalışan anahtarını
 * silip yenisini aramaya başlardı.
 */

// sahteJira, `myself` ucuna sabit bir durum döndüren sunucu.
func sahteJira(t *testing.T, durum int) *httptest.Server {
	t.Helper()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/api/3/myself", r.URL.Path, "doğrulama bu ucu çağırmalı")
		w.WriteHeader(durum)
	}))
	t.Cleanup(s.Close)
	return s
}

func jiraMeta(baseURL string) map[string]string {
	return map[string]string{"base_url": baseURL, "email": "u@e.com"}
}

func TestValidate_CalisanAnahtarKabulEdilir(t *testing.T) {
	srv := sahteJira(t, http.StatusOK)

	err := NewValidator(nil).Validate(context.Background(), KindJira,
		"gecerli-token", jiraMeta(srv.URL))
	require.NoError(t, err)
}

/*
REDDEDİLEN ANAHTAR İLE ULAŞILAMAYAN SERVİS AYRI HATALAR.

401/403 anahtarın yanlış olduğunu söylüyor; 5xx ve bağlantı hatası yalnızca
"şu an denenemedi" demek. Aynı hataya düşselerdi kullanıcı, ağı bir an kopunca
çalışan anahtarını yanlış sanardı.
*/
func TestValidate_YanlisAnahtarVeUlasilamayanServisAyrilir(t *testing.T) {
	for _, durum := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := sahteJira(t, durum)
		err := NewValidator(nil).Validate(context.Background(), KindJira, "kotu", jiraMeta(srv.URL))
		require.ErrorIs(t, err, ErrInvalidSecret, "durum %d anahtarı suçlamalı", durum)
	}

	srv := sahteJira(t, http.StatusInternalServerError)
	err := NewValidator(nil).Validate(context.Background(), KindJira, "iyi", jiraMeta(srv.URL))
	require.ErrorIs(t, err, ErrServiceUnreachable, "5xx anahtarı suçlamamalı")
}

// Hiç ulaşılamayan adres de "denenemedi" sayılır, "yanlış" değil.
func TestValidate_KapaliAdresUlasilamadiSayilir(t *testing.T) {
	err := NewValidator(nil).Validate(context.Background(), KindJira,
		"token", jiraMeta("http://127.0.0.1:1"))

	require.ErrorIs(t, err, ErrServiceUnreachable)
}

/*
404 ANAHTARI SUÇLAR — ve bu bilinçli.

Yanlış yazılmış bir base_url ya da bu ucu sunmayan bir sunucu 404 veriyor.
"Servise ulaşılamıyor" denseydi kullanıcı tekrar denemeyi beklerdi; oysa
düzeltmesi gereken bir şey var.
*/
func TestValidate_404DuzeltilecekBirSeyOldugunuSoyler(t *testing.T) {
	srv := sahteJira(t, http.StatusNotFound)

	err := NewValidator(nil).Validate(context.Background(), KindJira, "token", jiraMeta(srv.URL))
	require.ErrorIs(t, err, ErrInvalidSecret)
	require.Contains(t, err.Error(), "404")
}

/*
ZORUNLU ALANLAR AĞA ÇIKMADAN SINANIR.

base_url ya da email eksikken istek atmak, anlamsız bir adrese gidip
"ulaşılamıyor" demek olurdu — kullanıcı eksik alanı değil ağını kontrol
etmeye başlardı.
*/
func TestValidate_EksikAlanAgaCikmadanYakalanir(t *testing.T) {
	v := NewValidator(nil)
	ctx := context.Background()

	err := v.Validate(ctx, KindJira, "token", map[string]string{"email": "u@e.com"})
	require.ErrorIs(t, err, ErrMissingMetadata)
	require.Contains(t, err.Error(), "base_url")

	err = v.Validate(ctx, KindJira, "token", map[string]string{"base_url": "https://x.local"})
	require.ErrorIs(t, err, ErrMissingMetadata)
	require.Contains(t, err.Error(), "email")
}

/*
DOĞRULANABİLİR UCU OLMAYAN TÜR SESSİZCE KABUL EDİLİR.

Nexus kurulumları farklı yollar sunuyor; yanlış tahmin edilen bir uç, ÇALIŞAN
bir token'ı reddederdi. Doğrulama uydurmak yerine kabul ediliyor ve hata ilk
koşuda npm'in kendi mesajıyla görünüyor.
*/
func TestValidate_DogrulanamayanTurKabulEdilir(t *testing.T) {
	err := NewValidator(nil).Validate(context.Background(), KindNexus, "npm-token", nil)
	require.NoError(t, err)
}

func TestValidate_BilinmeyenTurReddedilir(t *testing.T) {
	err := NewValidator(nil).Validate(context.Background(), Kind("olmayan"), "x", nil)
	require.ErrorIs(t, err, ErrInvalidKind)
}

// Adresin sonundaki eğik çizgi sorun çıkarmaz: kullanıcı Jira adresini
// çoğunlukla tarayıcıdan kopyalıyor ve o hâliyle sonda `/` oluyor.
func TestValidate_AdresSonundakiEgikCizgiTemizlenir(t *testing.T) {
	srv := sahteJira(t, http.StatusOK)

	err := NewValidator(nil).Validate(context.Background(), KindJira,
		"token", jiraMeta(srv.URL+"/"))
	require.NoError(t, err)
}
