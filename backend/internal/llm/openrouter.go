package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// openRouterClient, OpenRouter API'si.
//
// Katalog için genel /models değil kullanıcıya özel /models/user kullanılır:
// erişimi kısıtlı anahtarlarda liste gerçekten daralır ve geçersiz anahtar
// hemen 401 verir.
type openRouterClient struct {
	base string
	http *http.Client
}

func (c *openRouterClient) Verify(ctx context.Context, key string) error {
	var body struct {
		Data struct {
			Usage      float64 `json:"usage"`
			IsFreeTier bool    `json:"is_free_tier"`
		} `json:"data"`
	}
	return getJSON(ctx, c.http, c.base+"/key", key, &body)
}

// openRouterModel, OpenRouter kataloğundaki ham kayıt.
type openRouterModel struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	ContextLength int    `json:"context_length"`

	Architecture struct {
		Modality string `json:"modality"`
	} `json:"architecture"`

	Pricing struct {
		Prompt     string `json:"prompt"`
		Completion string `json:"completion"`
	} `json:"pricing"`

	TopProvider struct {
		MaxCompletionTokens *int `json:"max_completion_tokens"`
	} `json:"top_provider"`

	SupportedParameters []string `json:"supported_parameters"`
}

func (c *openRouterClient) ListModels(ctx context.Context, key string) ([]Model, error) {
	var body struct {
		Data []openRouterModel `json:"data"`
	}
	if err := getJSON(ctx, c.http, c.base+"/models/user", key, &body); err != nil {
		return nil, err
	}
	if len(body.Data) == 0 {
		// Boş katalog gerçek bir durum değil. Sessizce kabul edilirse mevcut
		// katalog silinir; hata sayılıp eski liste korunur.
		return nil, fmt.Errorf("%w: model listesi boş", ErrBadCatalog)
	}

	models := make([]Model, 0, len(body.Data))
	for _, m := range body.Data {
		raw, _ := json.Marshal(m)

		// OpenRouter bu üç bilgiyi her zaman verir; "bilinmiyor" durumu oluşmaz.
		tools := false
		for _, p := range m.SupportedParameters {
			if p == "tools" {
				tools = true
				break
			}
		}

		var ctxLen *int
		if m.ContextLength > 0 {
			ctxLen = ptr(m.ContextLength)
		}

		models = append(models, Model{
			ID:              m.ID,
			Name:            firstNonEmpty(m.Name, m.ID),
			ContextLength:   ctxLen,
			MaxOutputTokens: m.TopProvider.MaxCompletionTokens,
			PromptPrice:     parsePrice(m.Pricing.Prompt),
			CompletionPrice: parsePrice(m.Pricing.Completion),
			SupportsTools:   ptr(tools),
			Modality:        m.Architecture.Modality,
			Raw:             raw,
		})
	}
	return models, nil
}

// ProviderOf, model kimliğinin ilk parçası ("anthropic/claude-x" → "anthropic").
// Katalogda gruplamak için kullanılır.
func ProviderOf(modelID string) string {
	if i := strings.Index(modelID, "/"); i > 0 {
		return modelID[:i]
	}
	return modelID
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
