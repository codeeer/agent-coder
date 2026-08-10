package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// openAICompatClient, OpenAI-uyumlu herhangi bir servis (vLLM, Azure OpenAI,
// Ollama, TGI, kendi proxy'niz...).
//
// Bu protokolde /models yalnızca model kimliklerini döner: fiyat, bağlam uzunluğu
// ve araç desteği bilgisi YOKTUR. Fiyatlar sıfır (= ücretsiz) sayılır, diğer
// iki alan "bilinmiyor" olarak kalır.
type openAICompatClient struct {
	base string
	http *http.Client
}

func (c *openAICompatClient) Verify(ctx context.Context, key string) error {
	var probe struct {
		Data []json.RawMessage `json:"data"`
	}
	return getJSON(ctx, c.http, c.base+"/models", key, &probe)
}

func (c *openAICompatClient) ListModels(ctx context.Context, key string) ([]Model, error) {
	return listOpenAIModels(ctx, c.http, c.base, key)
}

// openAIModel, OpenAI /models yanıtındaki kayıt.
type openAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

// listOpenAIModels, OpenAI-uyumlu /models ucunu okur.
// LiteLLM'in yedek yolu da buraya düşer.
func listOpenAIModels(ctx context.Context, httpClient *http.Client, base, key string) ([]Model, error) {
	var body struct {
		Data []openAIModel `json:"data"`
	}
	if err := getJSON(ctx, httpClient, base+"/models", key, &body); err != nil {
		return nil, err
	}
	if len(body.Data) == 0 {
		return nil, fmt.Errorf("%w: model listesi boş", ErrBadCatalog)
	}

	models := make([]Model, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ID == "" {
			continue
		}
		raw, _ := json.Marshal(m)

		models = append(models, Model{
			ID:   m.ID,
			Name: m.ID,
			// Bu protokol bu bilgileri vermiyor. ContextLength ve SupportsTools
			// bilinçli olarak nil bırakılır — sıfır ve false DEĞİL.
			ContextLength:   nil,
			MaxOutputTokens: nil,
			SupportsTools:   nil,
			PromptPrice:     0,
			CompletionPrice: 0,
			Raw:             raw,
		})
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("%w: kullanılabilir model bulunamadı", ErrBadCatalog)
	}
	return models, nil
}
