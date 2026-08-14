package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testProvider, sahte bir sunucuya bağlı sağlayıcı üretir.
func testProvider(t *testing.T, typ Type, handler http.HandlerFunc) Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := NewClient(Provider{Type: typ, BaseURL: srv.URL}, nil)
	require.NoError(t, err)
	return c
}

func TestNewClient_GecersizTur(t *testing.T) {
	_, err := NewClient(Provider{Type: Type("uydurma")}, nil)
	require.ErrorIs(t, err, ErrInvalidType)
}

func TestNewClient_OpenRouterAdresiEzilemez(t *testing.T) {
	// Kullanıcı başka adres girse bile OpenRouter'ın sabit adresi kullanılır.
	c, err := NewClient(Provider{Type: TypeOpenRouter, BaseURL: "https://kotu.test"}, nil)
	require.NoError(t, err)

	or, ok := c.(*openRouterClient)
	require.True(t, ok)
	require.Equal(t, OpenRouterBaseURL, or.base)
}

// ─── Ortak hata eşlemesi ────────────────────────────────────────────────────

func TestDurumKodlariDogruHatayaEslenir(t *testing.T) {
	tests := []struct {
		ad   string
		kod  int
		hata error
	}{
		{"401", http.StatusUnauthorized, ErrUnauthorized},
		{"403", http.StatusForbidden, ErrUnauthorized},
		{"500", http.StatusInternalServerError, ErrUnreachable},
		{"503", http.StatusServiceUnavailable, ErrUnreachable},
		{"429", http.StatusTooManyRequests, ErrBadCatalog},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			c := testProvider(t, TypeOpenAICompatible, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.kod)
			})
			require.ErrorIs(t, c.Verify(context.Background(), "anahtar"), tt.hata)
		})
	}
}

func TestUlasilamazSunucu(t *testing.T) {
	c, err := NewClient(Provider{Type: TypeOpenAICompatible, BaseURL: "http://127.0.0.1:1"}, nil)
	require.NoError(t, err)
	require.ErrorIs(t, c.Verify(context.Background(), "anahtar"), ErrUnreachable)
}

func TestIptalEdilenContext(t *testing.T) {
	c := testProvider(t, TypeOpenAICompatible, func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	require.ErrorIs(t, c.Verify(ctx, "anahtar"), ErrUnreachable)
}

// ─── OpenRouter ─────────────────────────────────────────────────────────────

func TestOpenRouter_KullaniciyaOzelUcCagrilir(t *testing.T) {
	// OpenRouter'ın adresi sabit olduğu için NewClient'ı test sunucusuna
	// yönlendiremeyiz; adaptör doğrudan kuruluyor.
	var yol string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		yol = r.URL.Path
		w.Write([]byte(`{"data":[{"id":"anthropic/claude-haiku-4.5","name":"Haiku"}]}`))
	}))
	t.Cleanup(srv.Close)

	c := &openRouterClient{base: srv.URL, http: srv.Client()}
	models, err := c.ListModels(context.Background(), "anahtar")
	require.NoError(t, err)
	require.Len(t, models, 1)

	// Genel /models değil: erişimi kısıtlı anahtarlarda liste daralmalı ve
	// geçersiz anahtar hemen 401 vermeli.
	require.Equal(t, "/models/user", yol)
}

func TestOpenRouter_TamKayitAyristirilir(t *testing.T) {
	const govde = `{"data":[{
		"id": "anthropic/claude-haiku-4.5",
		"name": "Anthropic: Claude Haiku 4.5",
		"context_length": 200000,
		"architecture": {"modality": "text+image->text"},
		"pricing": {"prompt": "0.000001", "completion": "0.000005"},
		"top_provider": {"max_completion_tokens": 64000},
		"supported_parameters": ["tools", "temperature"]
	}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(govde))
	}))
	t.Cleanup(srv.Close)

	c := &openRouterClient{base: srv.URL, http: srv.Client()}
	models, err := c.ListModels(context.Background(), "anahtar")
	require.NoError(t, err)
	require.Len(t, models, 1)

	m := models[0]
	require.Equal(t, "anthropic/claude-haiku-4.5", m.ID)
	require.NotNil(t, m.ContextLength)
	require.Equal(t, 200000, *m.ContextLength)
	require.Equal(t, 0.000001, m.PromptPrice)
	require.Equal(t, 0.000005, m.CompletionPrice)
	require.NotNil(t, m.SupportsTools)
	require.True(t, *m.SupportsTools)
	require.False(t, m.IsFree())
}

func TestOpenRouter_AracDesteklemeyenModelNilDegilFalse(t *testing.T) {
	// OpenRouter bu bilgiyi her zaman verir; "bilinmiyor" durumu oluşmamalı.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"data":[{"id":"a/b","name":"B","supported_parameters":["temperature"]}]}`))
	}))
	t.Cleanup(srv.Close)

	c := &openRouterClient{base: srv.URL, http: srv.Client()}
	models, err := c.ListModels(context.Background(), "anahtar")
	require.NoError(t, err)
	require.NotNil(t, models[0].SupportsTools)
	require.False(t, *models[0].SupportsTools)
}

// ─── LiteLLM ────────────────────────────────────────────────────────────────

func TestLiteLLM_DoluModelInfo(t *testing.T) {
	const govde = `{"data":[{
		"model_name": "gpt-4o-mini",
		"model_info": {
			"max_input_tokens": 128000,
			"max_output_tokens": 16384,
			"input_cost_per_token": 0.00000015,
			"output_cost_per_token": 0.0000006,
			"supports_function_calling": true,
			"mode": "chat"
		}
	}]}`

	c := testProvider(t, TypeLiteLLM, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/model/info", r.URL.Path)
		w.Write([]byte(govde))
	})

	models, err := c.ListModels(context.Background(), "anahtar")
	require.NoError(t, err)
	require.Len(t, models, 1)

	m := models[0]
	require.Equal(t, "gpt-4o-mini", m.ID)
	require.Equal(t, 128000, *m.ContextLength)
	require.Equal(t, 16384, *m.MaxOutputTokens)
	require.Equal(t, 0.00000015, m.PromptPrice)
	require.True(t, *m.SupportsTools)
	require.False(t, m.IsFree())
}

func TestLiteLLM_BosModelInfoBilinmiyorBirakir(t *testing.T) {
	// LiteLLM yöneticisi model_info alanlarını doldurmamışsa fiyat sıfır sayılır
	// (model ücretsiz görünür) ama bağlam ve araç desteği BİLİNMİYOR kalır.
	c := testProvider(t, TypeLiteLLM, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"data":[{"model_name":"kurum-modeli","model_info":{}}]}`))
	})

	models, err := c.ListModels(context.Background(), "anahtar")
	require.NoError(t, err)
	require.Len(t, models, 1)

	m := models[0]
	require.Equal(t, "kurum-modeli", m.ID)
	require.Nil(t, m.ContextLength, "bağlam bilinmiyor olmalı, sıfır değil")
	require.Nil(t, m.SupportsTools, "araç desteği bilinmiyor olmalı, false değil")
	require.True(t, m.IsFree(), "fiyat gelmezse ücretsiz sayılır (kullanıcı kararı)")
}

func TestLiteLLM_ModelInfoKapaliysaModelsUcunaDuser(t *testing.T) {
	var gorulenYollar []string
	c := testProvider(t, TypeLiteLLM, func(w http.ResponseWriter, r *http.Request) {
		gorulenYollar = append(gorulenYollar, r.URL.Path)
		if r.URL.Path == "/model/info" {
			// Bazı kurulumlarda yönetici uçları kapalıdır.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"data":[{"id":"kurum-modeli","object":"model"}]}`))
	})

	models, err := c.ListModels(context.Background(), "anahtar")
	require.NoError(t, err)
	require.Equal(t, []string{"/model/info", "/models"}, gorulenYollar)
	require.Len(t, models, 1)
	require.Equal(t, "kurum-modeli", models[0].ID)
	require.Nil(t, models[0].SupportsTools)
}

func TestLiteLLM_GecersizAnahtarYedegeDusmez(t *testing.T) {
	var cagriSayisi int
	c := testProvider(t, TypeLiteLLM, func(w http.ResponseWriter, _ *http.Request) {
		cagriSayisi++
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := c.ListModels(context.Background(), "anahtar")
	require.ErrorIs(t, err, ErrUnauthorized)

	// Anahtar geçersizse ikinci uç da geçersiz olacağı için tekrar denenmez.
	require.Equal(t, 1, cagriSayisi)
}

func TestLiteLLM_AdsizKayitAtlanir(t *testing.T) {
	c := testProvider(t, TypeLiteLLM, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"data":[
			{"model_name":"","model_info":{}},
			{"model_name":"gecerli","model_info":{}}
		]}`))
	})

	models, err := c.ListModels(context.Background(), "anahtar")
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "gecerli", models[0].ID)
}

func TestLiteLLM_ToolChoiceAlaniDaKabulEdilir(t *testing.T) {
	c := testProvider(t, TypeLiteLLM, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"data":[{"model_name":"m","model_info":{"supports_tool_choice":true}}]}`))
	})

	models, err := c.ListModels(context.Background(), "anahtar")
	require.NoError(t, err)
	require.NotNil(t, models[0].SupportsTools)
	require.True(t, *models[0].SupportsTools)
}

// ─── OpenAI-uyumlu ──────────────────────────────────────────────────────────

func TestOpenAICompat_YalnizcaKimlikGelir(t *testing.T) {
	c := testProvider(t, TypeOpenAICompatible, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/models", r.URL.Path)
		w.Write([]byte(`{"data":[
			{"id":"llama-3.3-70b","object":"model","owned_by":"vllm"},
			{"id":"qwen-2.5-coder","object":"model","owned_by":"vllm"}
		]}`))
	})

	models, err := c.ListModels(context.Background(), "anahtar")
	require.NoError(t, err)
	require.Len(t, models, 2)

	for _, m := range models {
		require.NotEmpty(t, m.ID)
		require.Nil(t, m.ContextLength)
		require.Nil(t, m.MaxOutputTokens)
		require.Nil(t, m.SupportsTools)
		require.True(t, m.IsFree())
	}
}

func TestOpenAICompat_BosListeHataSayilir(t *testing.T) {
	c := testProvider(t, TypeOpenAICompatible, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"data":[]}`))
	})

	_, err := c.ListModels(context.Background(), "anahtar")
	require.ErrorIs(t, err, ErrBadCatalog)
}

func TestOpenAICompat_BozukJSON(t *testing.T) {
	c := testProvider(t, TypeOpenAICompatible, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"data": bu json değil`))
	})

	_, err := c.ListModels(context.Background(), "anahtar")
	require.ErrorIs(t, err, ErrBadCatalog)
}

func TestParsePrice(t *testing.T) {
	require.Equal(t, 0.000001, parsePrice("0.000001"))
	require.Equal(t, float64(0), parsePrice(""))
	require.Equal(t, float64(0), parsePrice("fiyat-değil"))
	require.Equal(t, float64(0), parsePrice("-5"), "negatif fiyat kabul edilmemeli")
}
