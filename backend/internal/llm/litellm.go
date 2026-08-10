package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

// liteLLMClient, LiteLLM proxy'si.
//
// Katalog önce yönetici ucu /model/info'dan okunur; orada fiyat, bağlam uzunluğu
// ve araç desteği bulunabilir. Ancak bu alanlar LiteLLM yapılandırmasında
// OPSİYONELDİR — yönetici doldurmadıysa gelmezler. Ayrıca bazı kurulumlarda
// yönetici uçları tamamen kapalıdır.
//
// Bu yüzden /model/info başarısız olursa OpenAI-uyumlu /models ucuna düşülür;
// o zaman elimizde yalnızca model kimlikleri kalır ve diğer alanlar "bilinmiyor" olur.
type liteLLMClient struct {
	base string
	http *http.Client
}

func (c *liteLLMClient) Verify(ctx context.Context, key string) error {
	// Önce yönetici ucu denenir; kapalıysa OpenAI-uyumlu uçla doğrulanır.
	var probe struct {
		Data []json.RawMessage `json:"data"`
	}
	err := getJSON(ctx, c.http, c.base+"/model/info", key, &probe)
	if err == nil {
		return nil
	}
	// Geçersiz anahtar her iki uçta da geçersizdir; tekrar denemeye gerek yok.
	if errors.Is(err, ErrUnauthorized) {
		return err
	}
	return getJSON(ctx, c.http, c.base+"/models", key, &probe)
}

// liteLLMModelInfo, /model/info yanıtındaki bir kayıt.
//
// model_info alanlarının tamamı opsiyoneldir; hepsi işaretçi olarak okunur ki
// "verilmemiş" ile "sıfır" birbirine karışmasın.
type liteLLMModelInfo struct {
	ModelName string `json:"model_name"`
	ModelInfo struct {
		MaxTokens               *int     `json:"max_tokens"`
		MaxInputTokens          *int     `json:"max_input_tokens"`
		MaxOutputTokens         *int     `json:"max_output_tokens"`
		InputCostPerToken       *float64 `json:"input_cost_per_token"`
		OutputCostPerToken      *float64 `json:"output_cost_per_token"`
		SupportsFunctionCalling *bool    `json:"supports_function_calling"`
		SupportsToolChoice      *bool    `json:"supports_tool_choice"`
		Mode                    string   `json:"mode"`
	} `json:"model_info"`
}

func (c *liteLLMClient) ListModels(ctx context.Context, key string) ([]Model, error) {
	models, err := c.listFromModelInfo(ctx, key)
	if err == nil {
		return models, nil
	}
	if errors.Is(err, ErrUnauthorized) {
		return nil, err
	}

	slog.WarnContext(ctx, "litellm /model/info okunamadı, /models ucuna düşülüyor",
		"error", err)

	return c.listFromOpenAIEndpoint(ctx, key)
}

func (c *liteLLMClient) listFromModelInfo(ctx context.Context, key string) ([]Model, error) {
	var body struct {
		Data []liteLLMModelInfo `json:"data"`
	}
	if err := getJSON(ctx, c.http, c.base+"/model/info", key, &body); err != nil {
		return nil, err
	}
	if len(body.Data) == 0 {
		return nil, fmt.Errorf("%w: model listesi boş", ErrBadCatalog)
	}

	models := make([]Model, 0, len(body.Data))
	for _, m := range body.Data {
		if m.ModelName == "" {
			// Adı olmayan kayıt kullanılamaz; tüm indirmeyi düşürmek yerine atlanır.
			continue
		}
		raw, _ := json.Marshal(m)
		info := m.ModelInfo

		// Bağlam uzunluğu için önce girdi sınırı, yoksa genel sınır kullanılır.
		ctxLen := info.MaxInputTokens
		if ctxLen == nil {
			ctxLen = info.MaxTokens
		}

		// Araç desteği iki ayrı alandan gelebilir; biri bile true ise destekliyor.
		// İkisi de yoksa nil kalır — "desteklemiyor" DEĞİL, "bilinmiyor".
		var tools *bool
		switch {
		case info.SupportsFunctionCalling != nil:
			tools = info.SupportsFunctionCalling
		case info.SupportsToolChoice != nil:
			tools = info.SupportsToolChoice
		}

		models = append(models, Model{
			ID:              m.ModelName,
			Name:            m.ModelName,
			ContextLength:   ctxLen,
			MaxOutputTokens: info.MaxOutputTokens,
			PromptPrice:     deref(info.InputCostPerToken),
			CompletionPrice: deref(info.OutputCostPerToken),
			SupportsTools:   tools,
			Modality:        info.Mode,
			Raw:             raw,
		})
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("%w: kullanılabilir model bulunamadı", ErrBadCatalog)
	}
	return models, nil
}

// listFromOpenAIEndpoint, yönetici ucu kapalıyken kullanılan yedek yol.
func (c *liteLLMClient) listFromOpenAIEndpoint(ctx context.Context, key string) ([]Model, error) {
	return listOpenAIModels(ctx, c.http, c.base, key)
}

// deref, nil işaretçiyi sıfıra çevirir.
// Fiyatlar için kullanılır: bilinmeyen fiyat sıfır sayılır (spec 002 kullanıcı kararı).
func deref(v *float64) float64 {
	if v == nil {
		return 0
	}
	if *v < 0 {
		return 0
	}
	return *v
}
