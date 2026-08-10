// Package runner, bir agent'ı izole bir sandbox içinde çalıştırır.
//
// KRİTİK SINIR: opencode'a dair hiçbir tip, sabit veya varsayım bu dosyada geçmez.
// Sistemin geri kalanı yalnızca Runner arayüzünü bilir. Bu sınır, ileride opencode'u
// kendi motorumuzla değiştirebilmemizin tek garantisidir.
package runner

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Runner, bir agent çalıştırmasını yürütür.
type Runner interface {
	// Run, çalıştırmayı yürütür ve ilerlemeyi emit ile bildirir.
	//
	// ctx iptal edilirse çalıştırma durdurulur ve ErrCancelled döner.
	// Uygulama, hangi yolla çıkarsa çıksın (başarı, hata, panik, iptal, zaman aşımı)
	// oluşturduğu tüm kaynakları temizlemekle yükümlüdür.
	Run(ctx context.Context, req Request, emit EventFunc) (*Result, error)
}

// EventFunc, çalıştırma sırasındaki ilerlemeyi alır.
// Uygulama bunu birden fazla goroutine'den ÇAĞIRMAZ; sıralı çağrılır.
type EventFunc func(Event)

// Level, olay önem düzeyi.
type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Event, çalıştırma sırasındaki bir ilerleme bildirimi.
type Event struct {
	Level   Level
	Message string
}

// Request, bir çalıştırmanın tüm girdisi.
type Request struct {
	RunID    uuid.UUID
	Repo     RepoSpec
	Agent    AgentSpec
	Provider ProviderSpec
	Model    string
	Task     string

	// Timeout, çalıştırmanın azami süresi. Ayarlardan gelir.
	Timeout time.Duration

	// Limits, container kaynak sınırları. Ayarlardan gelir.
	Limits Limits
}

// RepoSpec, üzerinde çalışılacak depo.
type RepoSpec struct {
	URL    string
	Branch string

	// Username boşsa kimlik doğrulamasız klonlama yapılır (açık depolar).
	Username string
	Secret   string

	// CloneDepth, kaç commit'lik geçmiş klonlanacağı.
	CloneDepth int
}

// HasCredentials, depo erişimi için kimlik bilgisi verilip verilmediği.
func (r RepoSpec) HasCredentials() bool { return r.Secret != "" }

// AgentSpec, çalıştırılacak agent'ın tanımı.
//
// Prompt burada TAM METİN olarak taşınır, referans olarak değil: çalıştırma
// kaydı hangi talimatla çalıştığını kalıcı olarak bilmeli.
type AgentSpec struct {
	Slug        string
	Description string
	Prompt      string

	AllowEdit     bool
	AllowBash     bool
	AllowWebfetch bool
}

// ProviderSpec, modele erişim için gereken sağlayıcı bilgisi.
type ProviderSpec struct {
	// Slug, çalıştırma motorunun yapılandırmasındaki sağlayıcı kimliği.
	Slug string
	// Kind, sağlayıcı türü: openrouter | litellm | openai_compatible
	Kind    string
	BaseURL string
	APIKey  string
}

// Limits, container kaynak sınırları.
type Limits struct {
	CPUCores int
	MemoryGB int
}

// FileChange, değişen bir dosyanın özeti.
type FileChange struct {
	File      string `json:"file"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"` // modified | added | deleted
}

// Result, tamamlanmış bir çalıştırmanın çıktısı.
type Result struct {
	Output string
	Diff   string
	// Files, değişen dosyaların özeti. Arayüz "3 dosya, +42 −8" için kullanır.
	Files []FileChange

	PromptTokens     int
	CompletionTokens int
	CostUSD          float64
}

// HasChanges, agent'ın kodda değişiklik üretip üretmediği.
func (r Result) HasChanges() bool { return r.Diff != "" }

var (
	// ErrTimeout, çalıştırma süre sınırını aştı.
	ErrTimeout = errors.New("çalışma süre sınırını aştı")

	// ErrCancelled, çalıştırma iptal edildi.
	ErrCancelled = errors.New("çalışma iptal edildi")

	// ErrRepoAccess, depo klonlanamadı (yanlış adres, branch yok, yetki yok).
	ErrRepoAccess = errors.New("depoya erişilemedi")

	// ErrProviderAuth, sağlayıcı anahtarı reddedildi.
	ErrProviderAuth = errors.New("sağlayıcı anahtarı geçersiz")

	// ErrModel, model çağrısı başarısız oldu.
	ErrModel = errors.New("model çağrısı başarısız")

	// ErrSandbox, sandbox kurulamadı veya beklenmedik şekilde öldü.
	ErrSandbox = errors.New("çalışma ortamı hazırlanamadı")
)
