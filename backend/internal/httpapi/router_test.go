package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/config"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()

	// config.Load bu iki değişkeni zorunlu kılar; testler gerçek bir
	// veritabanına bağlanmaz, yalnızca geçerli biçimde değer verir.
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))

	cfg, err := config.Load()
	require.NoError(t, err)

	// Bağımlılıksız handler: veritabanına dokunan uçlar 503 döner, geri kalanı
	// (sağlık, yönlendirme, CORS, hata biçimi) buradan doğrulanabilir.
	h := NewHandler(Deps{Config: cfg})
	t.Cleanup(h.Shutdown)
	return h.Routes()
}

func TestHealth_OkDoner(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "application/json")

	var body HealthResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "ok", body.Status)
	require.Equal(t, "development", body.Env)
	require.NotEmpty(t, body.Uptime)
}

func TestBilinmeyenYol_TekTipHataDoner(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/yok", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)

	var body ErrorBody
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "not_found", body.Error.Code)
	require.NotEmpty(t, body.Error.Message)
}

func TestDesteklenmeyenMetot_405Doner(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/health", nil))

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestCORS_SadeceIzinliOriginIcinBaslikEklenir(t *testing.T) {
	t.Run("izinli origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "http://localhost:3000")

		rec := httptest.NewRecorder()
		testHandler(t).ServeHTTP(rec, req)

		require.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("izinsiz origin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "http://kotu.test")

		rec := httptest.NewRecorder()
		testHandler(t).ServeHTTP(rec, req)

		require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestPreflight_204Doner(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/herhangi", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "http://localhost:3000", rec.Header().Get("Access-Control-Allow-Origin"))
}
