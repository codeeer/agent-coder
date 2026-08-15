package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/config"
	"github.com/agent-coder/backend/internal/credentials"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * Kimlik bilgisi uçları.
 *
 * Buradan geçen değerler sırdır ve iki yönlü risk taşır: yanıtta sızabilir,
 * ya da doğrulanmadan kaydedilip ilk gerçek kullanımda patlayabilir.
 */

const gizliDeger = "sk-COK-GIZLI-JIRA-TOKENI-7f21"

type credFixture struct {
	routes http.Handler
	store  *credentials.Store
	// istek, sahte Jira'ya kaç çağrı gittiği.
	istek *int
}

func credSetup(t *testing.T, jiraDurum int) credFixture {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "credentials")

	sayac := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sayac++
		w.WriteHeader(jiraDurum)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("TEST_JIRA_URL", srv.URL)

	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	cfg, err := config.Load()
	require.NoError(t, err)

	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(make([]byte, secrets.KeySize)))
	require.NoError(t, err)
	store := credentials.NewStore(pool, cipher)

	h := NewHandler(Deps{
		Config: cfg, Credentials: store,
		JiraValidator: credentials.NewValidator(nil),
	})
	t.Cleanup(h.Shutdown)

	return credFixture{routes: h.Routes(), store: store, istek: &sayac}
}

func (f credFixture) kaydet(t *testing.T, kind string, govde any) *httptest.ResponseRecorder {
	t.Helper()

	raw, err := json.Marshal(govde)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	f.routes.ServeHTTP(rec,
		httptest.NewRequest(http.MethodPut, "/api/credentials/"+kind, bytes.NewReader(raw)))
	return rec
}

/*
DOĞRULAMA DÜŞERSE KAYIT YAPILMAZ.

Geçersiz bir anahtarın veritabanına girip ilk gerçek kullanımda patlaması,
hatanın kaynağını çok daha zor bulunur kılardı: kullanıcı ayarları "tanımlı"
görür, arıza dakikalar sonra bambaşka bir yerden gelir.
*/
func TestPutCredential_DogrulamaDusersKaydetmez(t *testing.T) {
	f := credSetup(t, http.StatusUnauthorized)

	rec := f.kaydet(t, "jira", map[string]any{
		"secret":   gizliDeger,
		"metadata": map[string]string{"base_url": envJira(t), "email": "u@e.com"},
	})
	require.GreaterOrEqual(t, rec.Code, 400, "reddedilmeliydi")

	_, err := f.store.Get(context.Background(), credentials.KindJira)
	require.ErrorIs(t, err, credentials.ErrNotConfigured,
		"doğrulanamayan anahtar veritabanına girmemeli")
}

// Doğrulama geçerse kaydediliyor ve yanıt kaydın kendisini dönüyor.
func TestPutCredential_DogrulamaGecersKaydeder(t *testing.T) {
	f := credSetup(t, http.StatusOK)

	rec := f.kaydet(t, "jira", map[string]any{
		"secret":   gizliDeger,
		"metadata": map[string]string{"base_url": envJira(t), "email": "u@e.com"},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	secret, _, err := f.store.Reveal(context.Background(), credentials.KindJira)
	require.NoError(t, err)
	require.Equal(t, gizliDeger, secret)
}

/*
YANIT SIRRI GERİ VERMEZ.

Kaydetme yanıtı kaydın kendisini dönüyor ve o kayıt tarayıcıya gidiyor. Sır
yanıta konsaydı, kullanıcının kendi ekranında görünmesi bir yana, tarayıcı
geçmişine ve ara sunucu loglarına da düşerdi.
*/
func TestPutCredential_YanitSirriGeriVermez(t *testing.T) {
	f := credSetup(t, http.StatusOK)

	rec := f.kaydet(t, "jira", map[string]any{
		"secret":   gizliDeger,
		"metadata": map[string]string{"base_url": envJira(t), "email": "u@e.com"},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), gizliDeger, "sır yanıta konmamalı")
}

// Listeleme de aynı kurala tabi.
func TestListCredentials_SirriGeriVermez(t *testing.T) {
	f := credSetup(t, http.StatusOK)
	require.NoError(t, f.store.Put(context.Background(), credentials.KindJira, gizliDeger, nil))

	rec := httptest.NewRecorder()
	f.routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/credentials", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), gizliDeger)
}

/*
BOŞ SIR AĞA ÇIKMADAN REDDEDİLİR.

Boş bir değeri doğrulatmak için servise gitmek, karşı tarafı boşuna meşgul
etmek ve kullanıcıyı gereksiz beklettikten sonra aynı şeyi söylemek olurdu.
*/
func TestPutCredential_BosSirAgaCikmadanReddedilir(t *testing.T) {
	f := credSetup(t, http.StatusOK)

	rec := f.kaydet(t, "jira", map[string]any{"secret": ""})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body ErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "missing_secret", body.Error.Code)
	require.Zero(t, *f.istek, "boş sır için doğrulama servisine gidilmemeli")
}

func TestPutCredential_GecersizTurReddedilir(t *testing.T) {
	f := credSetup(t, http.StatusOK)

	rec := f.kaydet(t, "olmayan", map[string]any{"secret": "x"})
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var body ErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "invalid_kind", body.Error.Code)
	require.Zero(t, *f.istek)
}

// Hiç kayıt yokken liste null değil boş dizi döner: arayüz doğrudan üzerinde
// döngü kuruyor.
func TestListCredentials_BosKurulumdaBosDizi(t *testing.T) {
	f := credSetup(t, http.StatusOK)

	rec := httptest.NewRecorder()
	f.routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/credentials", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "[]", trimJSON(rec.Body.String()))
}

func envJira(t *testing.T) string {
	t.Helper()
	v := os.Getenv("TEST_JIRA_URL")
	require.NotEmpty(t, v)
	return v
}

func trimJSON(s string) string {
	return strings.TrimSpace(s)
}
