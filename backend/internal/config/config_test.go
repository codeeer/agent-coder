package config

import (
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

const testDBURL = "postgres://u:p@localhost:5432/db?sslmode=disable"

// setRequired, Load'un zorunlu kıldığı değişkenleri geçerli değerlerle doldurur.
// Testler yalnızca ilgilendikleri değişkeni ezerek okunur kalır.
func setRequired(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)

	t.Setenv("DATABASE_URL", testDBURL)
	t.Setenv("SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(key))
}

func TestLoad_VarsayilanlarUygulanir(t *testing.T) {
	setRequired(t)

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, "development", cfg.Env)
	require.Equal(t, slog.LevelInfo, cfg.LogLevel)
	require.Equal(t, 8080, cfg.HTTP.Port)
	require.Equal(t, ":8080", cfg.Addr())
	require.Equal(t, []string{"http://localhost:3000"}, cfg.HTTP.CORSOrigins)
	require.Equal(t, "agent-coder/opencode-runner:latest", cfg.Runner.Image)
	require.Equal(t, testDBURL, cfg.DB.URL)
	require.False(t, cfg.IsProduction())
}

func TestLoad_OrtamDegiskenleriVarsayilaniEzer(t *testing.T) {
	setRequired(t)
	t.Setenv("APP_ENV", "production")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("BACKEND_PORT", "9090")
	t.Setenv("CORS_ORIGINS", "http://a.test, http://b.test ,")
	t.Setenv("RUNNER_NETWORK", "ozel-ag")

	cfg, err := Load()
	require.NoError(t, err)

	require.Equal(t, "production", cfg.Env)
	require.True(t, cfg.IsProduction())
	require.Equal(t, slog.LevelDebug, cfg.LogLevel)
	require.Equal(t, 9090, cfg.HTTP.Port)
	require.Equal(t, "ozel-ag", cfg.Runner.Network)

	// Boş parçalar atılır, boşluklar kırpılır.
	require.Equal(t, []string{"http://a.test", "http://b.test"}, cfg.HTTP.CORSOrigins)
}

func TestLoad_GecersizPortReddedilir(t *testing.T) {
	setRequired(t)
	t.Setenv("BACKEND_PORT", "70000")

	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "BACKEND_PORT")
}

func TestLoad_BozukSayiVarsayilanaDoner(t *testing.T) {
	setRequired(t)
	// Ayrıştırılamayan değer sunucuyu düşürmez; varsayılana döner.
	t.Setenv("BACKEND_PORT", "abc")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 8080, cfg.HTTP.Port)
}

func TestLoad_DatabaseUrlZorunlu(t *testing.T) {
	setRequired(t)
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "DATABASE_URL")
}

func TestLoad_SifrelemeAnahtariDogrulanir(t *testing.T) {
	tests := []struct {
		ad      string
		anahtar string
		icerir  string
	}{
		{"boş", "", "tanımlı değil"},
		{"base64 değil", "bu-base64-degil!!!", "base64"},
		{"kısa", base64.StdEncoding.EncodeToString(make([]byte, 16)), "32 bayt olmalı"},
		{"uzun", base64.StdEncoding.EncodeToString(make([]byte, 64)), "32 bayt olmalı"},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			setRequired(t)
			t.Setenv("SECRET_ENCRYPTION_KEY", tt.anahtar)

			_, err := Load()
			require.Error(t, err)
			require.Contains(t, err.Error(), "SECRET_ENCRYPTION_KEY")
			require.Contains(t, err.Error(), tt.icerir)
		})
	}
}

func TestLoad_TumSorunlarTekSeferdeRaporlanir(t *testing.T) {
	// Kullanıcı .env dosyasını tek turda düzeltebilsin diye eksiklerin
	// hepsi birden bildirilir, ilkinde durulmaz.
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SECRET_ENCRYPTION_KEY", "")
	t.Setenv("BACKEND_PORT", "70000")

	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "DATABASE_URL")
	require.Contains(t, err.Error(), "SECRET_ENCRYPTION_KEY")
	require.Contains(t, err.Error(), "BACKEND_PORT")
}
