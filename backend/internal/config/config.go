// Package config, ortam değişkenlerinden uygulama yapılandırmasını okur.
//
// Yapılandırma yalnızca burada, tek noktada okunur. Diğer paketler os.Getenv
// çağırmaz; ihtiyaç duydukları değeri Config üzerinden alır.
package config

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agent-coder/backend/internal/secrets"
)

// Config uygulamanın tüm çalışma zamanı ayarlarını tutar.
type Config struct {
	Env      string // development | production
	LogLevel slog.Level

	HTTP   HTTPConfig
	DB     DBConfig
	Runner RunnerConfig

	// OpenRouterAPIKey, .env üzerinden gelen varsayılan anahtar. Faz 1'den itibaren
	// kullanıcı bunu arayüzden girip veritabanında şifreli saklayabilecek.
	OpenRouterAPIKey string
	// SecretEncryptionKey, credential'ları AES-GCM ile şifreler. base64, 32 bayt.
	SecretEncryptionKey string

	// GitHubAPIURL, GitHub Enterprise kurulumları için değiştirilebilir.
	GitHubAPIURL string
}

// HTTPConfig sunucunun dinleme ve zaman aşımı ayarları.
type HTTPConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	// CORSOrigins, tarayıcıdan erişecek frontend adresleri.
	CORSOrigins []string
}

// DBConfig PostgreSQL bağlantı ayarları.
type DBConfig struct {
	URL         string
	MaxConns    int32
	MaxConnIdle time.Duration
}

// RunnerConfig agent adımlarını çalıştıran geçici container'ların ayarları.
//
// Burada YALNIZCA dağıtım topolojisi durur: hangi imaj, hangi ağ, hangi parola.
// Süre sınırı, eşzamanlılık, CPU ve bellek gibi DAVRANIŞ parametreleri
// `internal/settings` kayıt defterindedir ve veritabanından okunur (spec 003 H7);
// burada da tutulsalardı iki kaynak olur ve .env'i değiştiren kullanıcı hiçbir
// etki görmezdi.
type RunnerConfig struct {
	Image   string
	Network string
	// ServerPassword, backend ile runner içindeki opencode arasındaki basic auth parolası.
	ServerPassword string
	/*
	 * ExtraCACert, HOST üzerindeki kurumsal kök sertifikanın (PEM) yolu.
	 *
	 * SSL denetimi yapan kurumsal ağlar TLS'i kendi sertifikalarıyla açıp
	 * kapatıyor; bu ağlarda runner'ın yaptığı her HTTPS isteği (depo
	 * klonlama, model kataloğu, paket deposu) "unable to get local issuer
	 * certificate" ile düşer. Bu değer doluysa dosya container'a SALT
	 * OKUNUR bağlanır ve NODE_EXTRA_CA_CERTS ile gösterilir.
	 *
	 * Boş bırakmak varsayılandır ve hiçbir şeyi değiştirmez.
	 */
	ExtraCACert string
	/*
	 * HTTPProxy, runner container'ının çıkış trafiğini yönlendireceği vekil.
	 *
	 * DENETİM ARACI. Doluysa container'a HTTP_PROXY ailesi (büyük ve küçük
	 * harfli), NODE_USE_ENV_PROXY ve Maven'ın -D vekil özellikleri geçilir.
	 * Sandbox'ın dışarıya ne gönderdiği ancak trafik açılabilir bir vekilden
	 * geçirilerek okunabiliyor; ölçümün nasıl yapıldığı ve ne bulduğu
	 * `docs/veri-sizintisi-analizi.md` dosyasındadır.
	 *
	 * Boş bırakmak varsayılandır ve hiçbir şeyi değiştirmez.
	 *
	 * DİKKAT — bu ayar açıkken runner'ın TÜM dış trafiği üçüncü bir sürece
	 * yönlendirilir ve o süreç TLS'i açabiliyorsa sağlayıcı anahtarı dahil
	 * her şeyi görür. Bu yüzden açılışta uyarı loglanır (cmd/server/main.go);
	 * sessizce açık kalmamalı.
	 *
	 * Vekil trafiği YÖNLENDİRİR, MECBUR ETMEZ: vekili yok sayan bir istemci
	 * doğrudan çıkabilir. Ölçüldü — Maven'ın JVM tarafı bunu yaptı. Bu ayarı
	 * "tüm çıkış denetleniyor" garantisi sanmayın.
	 */
	HTTPProxy string
}

// Load ortam değişkenlerini okur ve doğrular.
// Eksik veya geçersiz zorunlu değerler tek seferde, hepsi birden raporlanır —
// böylece kullanıcı .env dosyasını tek turda düzeltebilir.
func Load() (*Config, error) {
	var problems []string

	cfg := &Config{
		Env:      getString("APP_ENV", "development"),
		LogLevel: parseLogLevel(getString("LOG_LEVEL", "info")),

		HTTP: HTTPConfig{
			Port:            getInt("BACKEND_PORT", 8080),
			ReadTimeout:     getDuration("HTTP_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getDuration("HTTP_WRITE_TIMEOUT", 60*time.Second),
			ShutdownTimeout: getDuration("HTTP_SHUTDOWN_TIMEOUT", 15*time.Second),
			CORSOrigins:     getStringSlice("CORS_ORIGINS", []string{"http://localhost:3000"}),
		},

		DB: DBConfig{
			URL:         getString("DATABASE_URL", ""),
			MaxConns:    int32(getInt("DB_MAX_CONNS", 10)),
			MaxConnIdle: getDuration("DB_MAX_CONN_IDLE", 5*time.Minute),
		},

		Runner: RunnerConfig{
			Image:          getString("RUNNER_IMAGE", "agent-coder/opencode-runner:latest"),
			Network:        getString("RUNNER_NETWORK", "agent-coder_internal"),
			ServerPassword: getString("OPENCODE_SERVER_PASSWORD", ""),
			ExtraCACert:    getString("RUNNER_EXTRA_CA_CERT", ""),
			HTTPProxy:      getString("RUNNER_HTTP_PROXY", ""),
		},

		OpenRouterAPIKey:    getString("OPENROUTER_API_KEY", ""),
		SecretEncryptionKey: getString("SECRET_ENCRYPTION_KEY", ""),
		GitHubAPIURL:        getString("GITHUB_API_URL", "https://api.github.com"),
	}

	if cfg.HTTP.Port < 1 || cfg.HTTP.Port > 65535 {
		problems = append(problems, fmt.Sprintf("BACKEND_PORT geçersiz: %d", cfg.HTTP.Port))
	}
	if cfg.DB.URL == "" {
		problems = append(problems, "DATABASE_URL tanımlı değil")
	}

	// Şifreleme anahtarı burada doğrulanır ki sunucu, kimlik bilgilerini
	// çözemeyeceğini ilk istekte değil, hiç başlamadan bildirsin.
	if err := validateEncryptionKey(cfg.SecretEncryptionKey); err != nil {
		problems = append(problems, "SECRET_ENCRYPTION_KEY "+err.Error()+
			" (üretmek için: openssl rand -base64 32)")
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("yapılandırma geçersiz:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

// IsProduction, üretim ortamında mıyız?
func (c *Config) IsProduction() bool { return c.Env == "production" }

// Addr dinlenecek TCP adresi.
func (c *Config) Addr() string { return fmt.Sprintf(":%d", c.HTTP.Port) }

// validateEncryptionKey, anahtarın base64 çözüldüğünde tam 32 bayt olduğunu doğrular.
func validateEncryptionKey(key string) error {
	if key == "" {
		return fmt.Errorf("tanımlı değil")
	}
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("geçerli base64 değil")
	}
	if len(raw) != secrets.KeySize {
		return fmt.Errorf("%d bayt olmalı, %d bayt verildi", secrets.KeySize, len(raw))
	}
	return nil
}

func getString(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func getStringSlice(key string, def []string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
