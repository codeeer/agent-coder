package llm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeBaseURL_OpenRouterSabittir(t *testing.T) {
	// Kullanıcı ne girerse girsin OpenRouter'ın adresi değişmez.
	for _, girdi := range []string{"", "https://baska.yer/v1", "saçma"} {
		got, err := NormalizeBaseURL(TypeOpenRouter, girdi)
		require.NoError(t, err)
		require.Equal(t, OpenRouterBaseURL, got)
	}
}

func TestNormalizeBaseURL_GecerliAdresler(t *testing.T) {
	tests := []struct {
		girdi    string
		beklenen string
	}{
		{"https://llm.sirket.local/v1", "https://llm.sirket.local/v1"},
		{"https://llm.sirket.local/v1/", "https://llm.sirket.local/v1"},
		{"https://llm.sirket.local/v1///", "https://llm.sirket.local/v1"},
		{"  https://llm.sirket.local/v1  ", "https://llm.sirket.local/v1"},
		{"http://localhost:4000/v1", "http://localhost:4000/v1"},
		{"https://llm.sirket.local", "https://llm.sirket.local"},
		{"https://llm.sirket.local/v1?x=1", "https://llm.sirket.local/v1"},
		{"https://llm.sirket.local/v1#bolum", "https://llm.sirket.local/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.girdi, func(t *testing.T) {
			got, err := NormalizeBaseURL(TypeLiteLLM, tt.girdi)
			require.NoError(t, err)
			require.Equal(t, tt.beklenen, got)
		})
	}
}

func TestNormalizeBaseURL_GecersizAdresler(t *testing.T) {
	tests := []struct {
		ad    string
		girdi string
	}{
		{"boş", ""},
		{"sadece boşluk", "   "},
		{"şema yok", "llm.sirket.local/v1"},
		{"desteklenmeyen şema", "ftp://llm.sirket.local"},
		{"dosya şeması", "file:///etc/passwd"},
		{"sunucu adı yok", "https://"},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			_, err := NormalizeBaseURL(TypeLiteLLM, tt.girdi)
			require.ErrorIs(t, err, ErrInvalidBaseURL)
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		ad       string
		beklenen string
	}{
		{"OpenRouter", "openrouter"},
		{"Şirket LiteLLM", "sirket-litellm"},
		{"Şirket   LiteLLM", "sirket-litellm"},
		{"  Boşluklu Ad  ", "bosluklu-ad"},
		{"ÇĞİÖŞÜ", "cgiosu"},
		{"çğıöşü", "cgiosu"},
		{"Ar-Ge Proxy", "ar-ge-proxy"},
		{"v2.0 Proxy", "v2-0-proxy"},
		{"---kenar---", "kenar"},
		{"Türkçe İstanbul", "turkce-istanbul"},
		{"!!!", "saglayici"},
		{"", "saglayici"},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			require.Equal(t, tt.beklenen, Slugify(tt.ad))
		})
	}
}

func TestSlugify_CiktiHepUygunBicimde(t *testing.T) {
	// Slug opencode yapılandırmasında `provider.<slug>` olarak kullanılacak;
	// hangi ad verilirse verilsin sonuç ASCII, küçük harf ve tire olmalı.
	adlar := []string{"Şirket LiteLLM", "ÇOK UZUN BİR SAĞLAYICI ADI " +
		"BURADA DEVAM EDİYOR VE ELLİ KARAKTERİ GEÇİYOR", "a", "🚀 emoji"}

	for _, ad := range adlar {
		s := Slugify(ad)
		require.NotEmpty(t, s)
		require.LessOrEqual(t, len(s), 48)
		require.Regexp(t, `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, s, "ad: %q", ad)
	}
}

func TestType_Valid(t *testing.T) {
	require.True(t, TypeOpenRouter.Valid())
	require.True(t, TypeLiteLLM.Valid())
	require.True(t, TypeOpenAICompatible.Valid())
	require.False(t, Type("uydurma").Valid())
	require.False(t, Type("").Valid())
}

func TestModel_IsFree_BilinmeyenFiyatUcretsizSayilir(t *testing.T) {
	// Spec 002 kullanıcı kararı: fiyat gelmezse sıfır kabul edilir.
	require.True(t, Model{}.IsFree())
	require.True(t, Model{PromptPrice: 0, CompletionPrice: 0}.IsFree())
	require.False(t, Model{PromptPrice: 0.000001}.IsFree())
	require.False(t, Model{CompletionPrice: 0.000005}.IsFree())
}

func TestModel_SupportsTools_NilFalseDegildir(t *testing.T) {
	// Araç desteği bilinmiyorsa "desteklemiyor" DEĞİLDİR: LiteLLM gibi
	// meta veri vermeyen sağlayıcılarda tüm modeller kullanılamaz hale gelirdi.
	bilinmiyor := Model{SupportsTools: nil}
	require.Nil(t, bilinmiyor.SupportsTools)

	no := false
	desteklemiyor := Model{SupportsTools: &no}
	require.NotNil(t, desteklemiyor.SupportsTools)
	require.False(t, *desteklemiyor.SupportsTools)
}

func TestModel_IsPreview(t *testing.T) {
	tests := []struct {
		id       string
		onizleme bool
	}{
		{"qwen/qwen3.6-max-preview", true},
		{"openrouter/auto-beta", true},
		{"google/gemini-2.0-flash-exp", true},
		{"bir/model-ALPHA", true},
		{"anthropic/claude-sonnet-4.5", false},
		{"gpt-4o-mini", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			require.Equal(t, tt.onizleme, Model{ID: tt.id}.IsPreview())
		})
	}
}

func TestValidateName(t *testing.T) {
	got, err := ValidateName("  Şirket LiteLLM  ")
	require.NoError(t, err)
	require.Equal(t, "Şirket LiteLLM", got)

	_, err = ValidateName("   ")
	require.ErrorIs(t, err, ErrEmptyName)

	_, err = ValidateName("")
	require.ErrorIs(t, err, ErrEmptyName)
}
