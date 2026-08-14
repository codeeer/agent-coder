package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/cacert"
)

func fixture(t *testing.T, ad string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "certfmt", "testdata", ad))
	require.NoError(t, err)
	return b
}

func handlerWith(res *cacert.Resolver) *Handler {
	return &Handler{deps: Deps{CACert: res}}
}

func getCA(t *testing.T, h *Handler) caStatusResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	h.caCert(rec, httptest.NewRequest(http.MethodGet, "/api/network/ca", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var out caStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	return out
}

// ── Kaynak: üç durum ────────────────────────────────────────────────────────

func TestCACert_HicbirKaynakYok(t *testing.T) {
	out := getCA(t, handlerWith(cacert.NewResolver(func() string { return "" }, "")))

	require.Equal(t, cacert.SourceNone, out.Source)
	require.Empty(t, out.Certificates)
}

func TestCACert_AyardanGelir(t *testing.T) {
	pemText := string(fixture(t, "kok.pem"))
	out := getCA(t, handlerWith(cacert.NewResolver(func() string { return pemText }, "")))

	require.Equal(t, cacert.SourceSettings, out.Source)
	require.Len(t, out.Certificates, 1)
	require.Equal(t, "Ornek Kurum Kok CA", out.Certificates[0].Subject)
}

func TestCACert_AyarBossaOrtamDegiskeninenDuser(t *testing.T) {
	yol := filepath.Join("..", "certfmt", "testdata", "kok.pem")
	out := getCA(t, handlerWith(cacert.NewResolver(func() string { return "" }, yol)))

	require.Equal(t, cacert.SourceEnv, out.Source)
	require.Len(t, out.Certificates, 1)
}

// İki kaynak birden doluyken AYAR kazanır (spec 017: Davranış kuralları).
func TestCACert_AyarOrtamDegiskeniniEzer(t *testing.T) {
	ayardaki := string(fixture(t, "ara.pem"))
	yol := filepath.Join("..", "certfmt", "testdata", "kok.pem")

	out := getCA(t, handlerWith(cacert.NewResolver(func() string { return ayardaki }, yol)))

	require.Equal(t, cacert.SourceSettings, out.Source)
	require.Equal(t, "Ornek Kurum Ara CA", out.Certificates[0].Subject)
}

// Çözümleyici hiç kurulmamışsa uç çökmez.
func TestCACert_CozumleyiciYokkenCokmez(t *testing.T) {
	out := getCA(t, handlerWith(nil))
	require.Equal(t, cacert.SourceNone, out.Source)
}

// ── Dosya çevirme ───────────────────────────────────────────────────────────

func normalize(t *testing.T, govde []byte) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(caNormalizeRequest{Data: base64.StdEncoding.EncodeToString(govde)})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	handlerWith(nil).caNormalize(rec,
		httptest.NewRequest(http.MethodPost, "/api/network/ca/normalize", strings.NewReader(string(body))))
	return rec
}

func TestCANormalize_DortBicimDePEMDoner(t *testing.T) {
	for _, ad := range []string{"kok.pem", "kok.der", "kok.base64"} {
		t.Run(ad, func(t *testing.T) {
			rec := normalize(t, fixture(t, ad))
			require.Equal(t, http.StatusOK, rec.Code)

			var out caNormalizeResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
			require.Contains(t, out.PEM, "BEGIN CERTIFICATE")
			require.Len(t, out.Certificates, 1)
			require.Equal(t, "Ornek Kurum Kok CA", out.Certificates[0].Subject)
		})
	}
}

// Zincirli dosya: kök VE ara birlikte gelmeli (spec 017 H1).
func TestCANormalize_PKCS7ZincirinTamamiDoner(t *testing.T) {
	rec := normalize(t, fixture(t, "zincir.p7b"))
	require.Equal(t, http.StatusOK, rec.Code)

	var out caNormalizeResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Certificates, 2)
}

func TestCANormalize_SertifikaOlmayanDosyaReddedilir(t *testing.T) {
	rec := normalize(t, fixture(t, "cop.txt"))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestCANormalize_BuyukGovdeReddedilir(t *testing.T) {
	rec := normalize(t, []byte(strings.Repeat("A", caNormalizeMaxBytes+1)))
	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestCANormalize_BozukGovdeReddedilir(t *testing.T) {
	rec := httptest.NewRecorder()
	handlerWith(nil).caNormalize(rec,
		httptest.NewRequest(http.MethodPost, "/api/network/ca/normalize", strings.NewReader("{bozuk")))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
