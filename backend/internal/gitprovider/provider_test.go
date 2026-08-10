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

func TestValidate_BitbucketBasicKullanir(t *testing.T) {
	var kullanici, parola string
	var ok bool
	v, url := testValidator(t, func(w http.ResponseWriter, r *http.Request) {
		kullanici, parola, ok = r.BasicAuth()
	})

	err := v.Validate(context.Background(),
		Provider{Type: TypeBitbucket, BaseURL: url, Username: "omer"}, "app-password")
	require.NoError(t, err)
	require.True(t, ok, "basic auth kullanılmalı")
	require.Equal(t, "omer", kullanici)
	require.Equal(t, "app-password", parola)
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
		{"404 yanlış adres", http.StatusNotFound, ErrInvalidSecret},
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
