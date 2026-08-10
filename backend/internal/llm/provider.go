// Package llm, LLM sağlayıcılarını ve model kataloglarını yönetir.
//
// Sistem birden fazla sağlayıcıyı aynı anda destekler: OpenRouter, kurum içi
// LiteLLM proxy'si veya OpenAI-uyumlu herhangi bir servis. Türe özgü farklar
// Client arayüzünün arkasında kalır; sistemin geri kalanı sağlayıcı türünü bilmez.
package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/slug"
)

// Type, sağlayıcı türü. Veritabanındaki llm_provider_type enum'u ile eşleşir.
type Type string

const (
	TypeOpenRouter       Type = "openrouter"
	TypeLiteLLM          Type = "litellm"
	TypeOpenAICompatible Type = "openai_compatible"
)

// AllTypes, desteklenen tüm türler.
var AllTypes = []Type{TypeOpenRouter, TypeLiteLLM, TypeOpenAICompatible}

// Valid, türün bilinen bir değer olup olmadığını söyler.
func (t Type) Valid() bool {
	for _, v := range AllTypes {
		if t == v {
			return true
		}
	}
	return false
}

// OpenRouterBaseURL, OpenRouter'ın sabit adresi. Bu türde kullanıcıdan adres istenmez.
const OpenRouterBaseURL = "https://openrouter.ai/api/v1"

// FixedBaseURL, türün adresi sabitse onu döner; değilse boş.
func (t Type) FixedBaseURL() string {
	if t == TypeOpenRouter {
		return OpenRouterBaseURL
	}
	return ""
}

var (
	ErrInvalidType    = errors.New("geçersiz sağlayıcı türü")
	ErrInvalidBaseURL = errors.New("servis adresi geçersiz")
	ErrEmptyName      = errors.New("sağlayıcı adı boş olamaz")
	ErrNotFound       = errors.New("sağlayıcı bulunamadı")
)

// Provider, tanımlı bir LLM sağlayıcı. Gizli değeri TAŞIMAZ.
type Provider struct {
	ID        uuid.UUID `json:"id"`
	Type      Type      `json:"type"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	BaseURL   string    `json:"baseUrl"`
	Hint      string    `json:"hint"`
	IsDefault bool      `json:"isDefault"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Model, katalogdaki bir model.
//
// İşaretçi alanlar "bilinmiyor" durumunu taşır. Fiyatlar işaretçi DEĞİLDİR:
// bilinmeyen fiyat sıfır sayılır ve model ücretsiz görünür (spec 002 kullanıcı kararı).
// Araç desteği bu kolaylığın dışındadır — yanlış varsayım modelin agent olarak
// hiç kullanılamaması demek olurdu.
type Model struct {
	ID              string
	Name            string
	ContextLength   *int
	MaxOutputTokens *int
	PromptPrice     float64 // token başına USD
	CompletionPrice float64
	SupportsTools   *bool // nil = bilinmiyor, false = desteklemiyor
	Modality        string
	Raw             json.RawMessage
}

// IsFree, modelin ücretsiz olup olmadığı.
// Fiyatı bilinmeyen model burada ücretsiz görünür — bilinçli kabul edilmiş bir sapma.
func (m Model) IsFree() bool {
	return m.PromptPrice == 0 && m.CompletionPrice == 0
}

// previewMarkers, önizleme/deneysel model tespitinde aranan parçalar.
var previewMarkers = []string{"preview", "-exp", "experimental", "alpha", "beta"}

// IsPreview, modelin önizleme veya deneysel olup olmadığını tahmin eder.
//
// SEZGİSEL: sağlayıcıların böyle bir alanı yok, kimlikteki işaretlere bakılıyor.
// Yanlış pozitif mümkün; bu bilgi yalnızca rozet olarak gösterilir, davranış değiştirmez.
func (m Model) IsPreview() bool {
	id := strings.ToLower(m.ID)
	for _, marker := range previewMarkers {
		if strings.Contains(id, marker) {
			return true
		}
	}
	return false
}

// NormalizeBaseURL, kullanıcının girdiği adresi doğrular ve tekilleştirir.
//
// Tür adresi sabitse (OpenRouter) kullanıcının girdiği yok sayılır.
func NormalizeBaseURL(t Type, raw string) (string, error) {
	if fixed := t.FixedBaseURL(); fixed != "" {
		return fixed, nil
	}

	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: adres boş", ErrInvalidBaseURL)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: ayrıştırılamadı", ErrInvalidBaseURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w: http:// veya https:// ile başlamalı", ErrInvalidBaseURL)
	}
	if u.Host == "" {
		return "", fmt.Errorf("%w: sunucu adı yok", ErrInvalidBaseURL)
	}

	// Sorgu ve parça bilgisi taban adreste anlamsız; sessizce atılır.
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")

	return u.String(), nil
}

// Slugify, kullanıcının verdiği adı sağlayıcı kimliğine çevirir.
//
// Bu kimlik runner içindeki çalıştırma motoru yapılandırmasında
// `provider.<slug>` olarak kullanılır.
func Slugify(name string) string {
	return slug.Make(name, "saglayici")
}

// ValidateName, sağlayıcı adını doğrular.
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrEmptyName
	}
	if len(name) > 100 {
		return "", fmt.Errorf("%w: en fazla 100 karakter", ErrEmptyName)
	}
	return name, nil
}
