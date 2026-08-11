package gitprovider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate_ZorunluAlanlar(t *testing.T) {
	tests := []struct {
		ad        string
		tur       Type
		isim      string
		adres     string
		kullanici string
		hata      error
	}{
		{"github token yeter", TypeGitHub, "GitHub", "", "", nil},
		{"github adres opsiyonel", TypeGitHub, "GH Enterprise", "https://git.sirket.local/api/v3", "", nil},
		{"bitbucket kullanıcı adı ister", TypeBitbucket, "Bitbucket", "", "", ErrMissingUsername},
		{"bitbucket kullanıcı adıyla", TypeBitbucket, "Bitbucket", "", "omer", nil},
		{"genel git adres ister", TypeGeneric, "Kurum Git", "", "omer", ErrMissingBaseURL},
		{"genel git kullanıcı adı ister", TypeGeneric, "Kurum Git", "https://git.sirket.local", "", ErrMissingUsername},
		{"genel git tam", TypeGeneric, "Kurum Git", "https://git.sirket.local", "omer", nil},
		{"ad boş", TypeGitHub, "  ", "", "", ErrEmptyName},
		{"geçersiz tür", Type("uydurma"), "X", "", "", ErrInvalidType},
		{"şemasız adres", TypeGeneric, "X", "git.sirket.local", "omer", ErrInvalidBaseURL},
		{"desteklenmeyen şema", TypeGeneric, "X", "ssh://git.sirket.local", "omer", ErrInvalidBaseURL},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			_, _, _, err := Validate(tt.tur, tt.isim, tt.adres, tt.kullanici)
			if tt.hata == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.hata)
		})
	}
}

func TestValidate_AdresNormalize(t *testing.T) {
	_, adres, _, err := Validate(TypeGeneric, "X", "https://git.sirket.local/path/?a=1#b", "omer")
	require.NoError(t, err)
	require.Equal(t, "https://git.sirket.local/path", adres)
}

func TestAPIURL_VarsayilanlarKullanilir(t *testing.T) {
	require.Equal(t, "https://api.github.com", Provider{Type: TypeGitHub}.APIURL())
	require.Equal(t, "https://api.bitbucket.org/2.0", Provider{Type: TypeBitbucket}.APIURL())

	// Kendi sunucusundaki kurulum varsayılanı ezer.
	p := Provider{Type: TypeGitHub, BaseURL: "https://git.sirket.local/api/v3/"}
	require.Equal(t, "https://git.sirket.local/api/v3", p.APIURL())
}

func testValidator(t *testing.T, handler http.HandlerFunc) (*Validator, string) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Validator{http: srv.Client()}, srv.URL
}

func TestValidate_GitHubBearerKullanir(t *testing.T) {
	var auth, yol string
	v, url := testValidator(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		yol = r.URL.Path
	})

	err := v.Validate(context.Background(),
		Provider{Type: TypeGitHub, BaseURL: url}, "ghp_token123")
	require.NoError(t, err)
	require.Equal(t, "Bearer ghp_token123", auth)
	require.Equal(t, "/user", yol)
}

/*
 * Kendi sunucusundaki Bitbucket (Server / Data Center).
 *
 * httptest adresi api.bitbucket.org olmadığı için bu test SERVER dalını sınar —
 * kurumsal kurulumun gerçek yolu budur.
 *
 * Cloud'un `/user` ucu Bitbucket Server'da YOKTUR; oraya gitmek 404 üretiyordu
 * ve 404 kimlik hatası sayıldığı için kullanıcı doğru token'ını yanlış sanıyordu.
 */
func TestValidate_BitbucketServerRestUcunuCagirir(t *testing.T) {
	var kullanici, parola, yol, sorgu string
	var ok bool
	v, url := testValidator(t, func(w http.ResponseWriter, r *http.Request) {
		kullanici, parola, ok = r.BasicAuth()
		yol = r.URL.Path
		sorgu = r.URL.Query().Get("limit")
	})

	err := v.Validate(context.Background(),
		Provider{Type: TypeBitbucket, BaseURL: url, Username: "omer"}, "server-pat")
	require.NoError(t, err)

	require.Equal(t, "/rest/api/1.0/projects", yol, "Server'da Cloud ucu çağrılmamalı")
	require.Equal(t, "1", sorgu, "doğrulama için tek kayıt yeter")

	// Server PAT'leri de Basic Auth ile çalışır; Bearer'a geçilmedi.
	require.True(t, ok, "basic auth kullanılmalı")
	require.Equal(t, "omer", kullanici)
	require.Equal(t, "server-pat", parola)
}

func TestValidate_BitbucketKullaniciAdiOlmadan(t *testing.T) {
	v := NewValidator()
	err := v.Validate(context.Background(), Provider{Type: TypeBitbucket}, "parola")
	require.ErrorIs(t, err, ErrMissingUsername)
}

func TestValidate_GenelGitDogrulanamaz(t *testing.T) {
	// Doğrulama mümkün değil ama bu bir hata değil: kayıt yine yapılacak,
	// kullanıcı uyarılacak.
	v := NewValidator()
	err := v.Validate(context.Background(),
		Provider{Type: TypeGeneric, BaseURL: "https://git.sirket.local", Username: "omer"}, "token")
	require.ErrorIs(t, err, ErrNotVerifiable)
}

func TestValidate_DurumKodlari(t *testing.T) {
	tests := []struct {
		ad   string
		kod  int
		hata error
	}{
		{"200 geçerli", http.StatusOK, nil},
		{"401 geçersiz", http.StatusUnauthorized, ErrInvalidSecret},
		{"403 yasak", http.StatusForbidden, ErrInvalidSecret},
		// 404 kimlik hatası DEĞİL: sunucu token'a bakmadan "böyle bir uç yok"
		// diyor. Adres hatası olarak raporlanmalı ki kullanıcı token'ını
		// boşuna yenilemesin.
		{"404 yanlış adres", http.StatusNotFound, ErrInvalidBaseURL},
		{"500 sunucu hatası", http.StatusInternalServerError, ErrUnreachable},
		{"418 beklenmedik", http.StatusTeapot, ErrUnreachable},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			v, url := testValidator(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.kod)
			})

			err := v.Validate(context.Background(),
				Provider{Type: TypeGitHub, BaseURL: url}, "token")
			if tt.hata == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.hata)
		})
	}
}

func TestValidate_UlasilamazSunucu(t *testing.T) {
	v := NewValidator()
	err := v.Validate(context.Background(),
		Provider{Type: TypeGitHub, BaseURL: "http://127.0.0.1:1"}, "token")
	require.ErrorIs(t, err, ErrUnreachable)
}

/*
 * Cloud / Server ayrımı — uç seçimi.
 *
 * Saf fonksiyon olarak sınanıyor çünkü Cloud dalı gerçek `api.bitbucket.org`
 * adresine bağlı; sahte sunucuya yönlendirilemez ve gerçek adrese istek atan
 * bir test kabul edilemez. Gövde davranışı Server dalında sınanıyor.
 */
func TestBitbucketProbe_AdreseGoreUcSecer(t *testing.T) {
	tests := []struct {
		ad    string
		adres string
		uc    string
	}{
		{
			ad:    "adres boşken Cloud varsayılanı",
			adres: TypeBitbucket.DefaultAPIURL(),
			uc:    "https://api.bitbucket.org/2.0/user",
		},
		{
			ad:    "Cloud alt alan adı",
			adres: "https://api.bitbucket.org",
			uc:    "https://api.bitbucket.org/user",
		},
		{
			ad:    "kendi sunucusu",
			adres: "https://bitbucket.sirket.local",
			uc:    "https://bitbucket.sirket.local/rest/api/1.0/projects?limit=1",
		},
		{
			ad:    "port taşıyan kendi sunucusu",
			adres: "https://bitbucket.sirket.local:7990",
			uc:    "https://bitbucket.sirket.local:7990/rest/api/1.0/projects?limit=1",
		},
		{
			// Düz metin araması bunu Cloud sanırdı ve kurumsal kurulum yine
			// yanlış uca giderdi. Karar HOST'a bakılarak veriliyor.
			ad:    "yolunda cloud adresi geçen kendi sunucusu",
			adres: "https://bitbucket.sirket.local/api.bitbucket.org",
			uc:    "https://bitbucket.sirket.local/api.bitbucket.org/rest/api/1.0/projects?limit=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			require.Equal(t, tt.uc, bitbucketProbe(tt.adres))
		})
	}
}

// Adres hiç girilmemişken Cloud davranışı korunuyor mu.
func TestBitbucketCloud_AdresBosBirakilirsa(t *testing.T) {
	require.True(t, bitbucketCloud(""), "adres boşken Cloud varsayılanı geçerli")

	p := Provider{Type: TypeBitbucket}
	require.Equal(t, "https://api.bitbucket.org/2.0/user", bitbucketProbe(p.APIURL()))
}

/*
 * Bitbucket dalı durum kodları.
 *
 * Mevcut TestValidate_DurumKodlari yalnızca GitHub ile koşuyordu; Bitbucket'ın
 * kendi dalı sınanmıyordu. Asıl önemli satır 404: kimlik hatası değil adres
 * hatası olmalı.
 */
func TestValidate_BitbucketDurumKodlari(t *testing.T) {
	tests := []struct {
		ad   string
		kod  int
		hata error
	}{
		{"200 geçerli", http.StatusOK, nil},
		{"401 geçersiz anahtar", http.StatusUnauthorized, ErrInvalidSecret},
		{"403 yasak", http.StatusForbidden, ErrInvalidSecret},
		{"404 yanlış adres", http.StatusNotFound, ErrInvalidBaseURL},
		{"500 sunucu hatası", http.StatusInternalServerError, ErrUnreachable},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			v, url := testValidator(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.kod)
			})

			err := v.Validate(context.Background(),
				Provider{Type: TypeBitbucket, BaseURL: url, Username: "omer"}, "pat")
			if tt.hata == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.hata)
			// 404 kimlik hatasına DA sarmalanmamalı: respondGitError'da
			// ErrInvalidSecret dalı önce geliyor, sarmalansaydı yine
			// "invalid_credential" raporlanırdı.
			if tt.kod == http.StatusNotFound {
				require.NotErrorIs(t, err, ErrInvalidSecret)
			}
		})
	}
}

// GitHub ve genel Git davranışı bu değişiklikten etkilenmemeli.
func TestValidate_DigerTurlerEtkilenmedi(t *testing.T) {
	t.Run("github hâlâ /user + Bearer", func(t *testing.T) {
		var auth, yol string
		v, url := testValidator(t, func(w http.ResponseWriter, r *http.Request) {
			auth = r.Header.Get("Authorization")
			yol = r.URL.Path
		})

		require.NoError(t, v.Validate(context.Background(),
			Provider{Type: TypeGitHub, BaseURL: url}, "ghp_x"))
		require.Equal(t, "/user", yol)
		require.Equal(t, "Bearer ghp_x", auth)
	})

	t.Run("genel git doğrulanamaz akışı korunuyor", func(t *testing.T) {
		// spec 002 H5: doğrulanamaması hata değil; kayıt yine yapılır.
		v := NewValidator()
		err := v.Validate(context.Background(),
			Provider{Type: TypeGeneric, BaseURL: "https://git.sirket.local", Username: "omer"}, "t")
		require.ErrorIs(t, err, ErrNotVerifiable)
	})
}
