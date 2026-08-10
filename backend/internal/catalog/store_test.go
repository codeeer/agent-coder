package catalog_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/catalog"
	"github.com/agent-coder/backend/internal/llm"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
)

func newCipher(t *testing.T) *secrets.Cipher {
	t.Helper()
	key := make([]byte, secrets.KeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)
	c, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(key))
	require.NoError(t, err)
	return c
}

// setup, temiz bir katalog ve sağlayıcı deposu döner.
func setup(t *testing.T) (*catalog.Store, *llm.Store, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.TestDB(t)
	// models, llm_providers'a CASCADE bağlı; sağlayıcıları silmek yeter.
	testutil.Truncate(t, pool, "llm_providers")
	return catalog.NewStore(pool), llm.NewStore(pool, newCipher(t)), pool
}

// addProvider, test için bir sağlayıcı oluşturur.
func addProvider(t *testing.T, store *llm.Store, name string, typ llm.Type, baseURL string) llm.Provider {
	t.Helper()
	p, err := store.Create(context.Background(), llm.CreateInput{
		Type: typ, Name: name, BaseURL: baseURL, Secret: "gizli-anahtar-" + name,
	})
	require.NoError(t, err)
	return p
}

// seedModel, doğrudan veritabanına model yazar.
func seedModel(t *testing.T, pool *pgxpool.Pool, providerID uuid.UUID, id, name string,
	ctxLen *int, promptPrice float64, tools *bool) {
	t.Helper()

	_, err := pool.Exec(context.Background(), `
		INSERT INTO models (provider_id, id, provider, name, description, context_length,
			max_output_tokens, prompt_price, completion_price, supports_tools,
			is_free, is_preview, modality, raw)
		VALUES ($1,$2,'test',$3,'',$4,NULL,$5,$5,$6,$7,false,'','{}'::jsonb)`,
		providerID, id, name, ctxLen, promptPrice, tools, promptPrice == 0)
	require.NoError(t, err)
}

func ptr[T any](v T) *T { return &v }

func TestList_SaglayiciBazindaAyrilir(t *testing.T) {
	store, providers, pool := setup(t)
	ctx := context.Background()

	// İki sağlayıcıda AYNI isimli model — spec 002 H2: karışmamalı.
	p1 := addProvider(t, providers, "Şirket LiteLLM", llm.TypeLiteLLM, "https://llm.local/v1")
	p2 := addProvider(t, providers, "OpenRouter", llm.TypeOpenRouter, "")

	seedModel(t, pool, p1.ID, "gpt-4o-mini", "GPT-4o mini", ptr(128000), 0.0000001, ptr(true))
	seedModel(t, pool, p2.ID, "gpt-4o-mini", "GPT-4o mini", ptr(128000), 0.0000005, ptr(true))

	all, total, err := store.List(ctx, catalog.ListFilter{})
	require.NoError(t, err)
	require.Equal(t, 2, total, "aynı isimli model iki ayrı satır olmalı")
	require.Len(t, all, 2)

	// Sağlayıcı filtresi ayırıyor.
	only1, total1, err := store.List(ctx, catalog.ListFilter{ProviderID: &p1.ID})
	require.NoError(t, err)
	require.Equal(t, 1, total1)
	require.Equal(t, "Şirket LiteLLM", only1[0].ProviderName)
	require.Equal(t, p1.ID, only1[0].ProviderID)
}

func TestList_SaglayiciSilinincModelleriGider(t *testing.T) {
	store, providers, pool := setup(t)
	ctx := context.Background()

	p1 := addProvider(t, providers, "Bir", llm.TypeLiteLLM, "https://a.local/v1")
	p2 := addProvider(t, providers, "İki", llm.TypeLiteLLM, "https://b.local/v1")
	seedModel(t, pool, p1.ID, "m1", "M1", ptr(1000), 0, ptr(true))
	seedModel(t, pool, p2.ID, "m2", "M2", ptr(1000), 0, ptr(true))

	require.NoError(t, providers.Delete(ctx, p1.ID))

	_, total, err := store.List(ctx, catalog.ListFilter{})
	require.NoError(t, err)
	require.Equal(t, 1, total, "silinen sağlayıcının modelleri de gitmeli")

	// Diğer sağlayıcınınki duruyor.
	remaining, _, err := store.List(ctx, catalog.ListFilter{})
	require.NoError(t, err)
	require.Equal(t, "m2", remaining[0].ID)
}

func TestList_AracFiltresiUcDurumlu(t *testing.T) {
	store, providers, pool := setup(t)
	ctx := context.Background()
	p := addProvider(t, providers, "Test", llm.TypeLiteLLM, "https://x.local/v1")

	seedModel(t, pool, p.ID, "destekliyor", "A", ptr(1000), 0, ptr(true))
	seedModel(t, pool, p.ID, "desteklemiyor", "B", ptr(1000), 0, ptr(false))
	seedModel(t, pool, p.ID, "bilinmiyor", "C", ptr(1000), 0, nil)

	t.Run("filtre yok", func(t *testing.T) {
		_, total, err := store.List(ctx, catalog.ListFilter{})
		require.NoError(t, err)
		require.Equal(t, 3, total)
	})

	t.Run("yalnızca destekleyenler", func(t *testing.T) {
		models, total, err := store.List(ctx, catalog.ListFilter{Tools: catalog.ToolsOnly})
		require.NoError(t, err)
		require.Equal(t, 1, total, "bilinmeyen model destekleyenlere DAHİL EDİLMEMELİ")
		require.Equal(t, "destekliyor", models[0].ID)
	})

	t.Run("yalnızca bilinmeyenler", func(t *testing.T) {
		models, total, err := store.List(ctx, catalog.ListFilter{Tools: catalog.ToolsUnknown})
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Equal(t, "bilinmiyor", models[0].ID)
		require.Nil(t, models[0].SupportsTools, "bilinmiyor nil olarak taşınmalı")
	})
}

func TestList_BilinmeyenBaglamNilTasinir(t *testing.T) {
	store, providers, pool := setup(t)
	ctx := context.Background()
	p := addProvider(t, providers, "Test", llm.TypeLiteLLM, "https://x.local/v1")

	seedModel(t, pool, p.ID, "bilinmeyen-baglam", "A", nil, 0, nil)

	models, _, err := store.List(ctx, catalog.ListFilter{})
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Nil(t, models[0].ContextLength, "bilinmeyen bağlam sıfır değil nil olmalı")
	require.True(t, models[0].IsFree, "fiyat gelmezse ücretsiz sayılır (kullanıcı kararı)")
}

func TestList_FiyatMilyonTokenaCevrilir(t *testing.T) {
	store, providers, pool := setup(t)
	ctx := context.Background()
	p := addProvider(t, providers, "Test", llm.TypeLiteLLM, "https://x.local/v1")

	seedModel(t, pool, p.ID, "m", "M", ptr(1000), 0.000001, ptr(true))

	models, _, err := store.List(ctx, catalog.ListFilter{})
	require.NoError(t, err)
	require.InDelta(t, 1.0, models[0].PromptPricePerMTok, 0.0001)
}

func TestList_AramaVeSiralama(t *testing.T) {
	store, providers, pool := setup(t)
	ctx := context.Background()
	p := addProvider(t, providers, "Test", llm.TypeLiteLLM, "https://x.local/v1")

	seedModel(t, pool, p.ID, "anthropic/claude", "Claude", ptr(1000), 0.000003, ptr(true))
	seedModel(t, pool, p.ID, "openai/gpt", "GPT", ptr(9000), 0.000001, ptr(true))

	t.Run("arama", func(t *testing.T) {
		_, total, err := store.List(ctx, catalog.ListFilter{Query: "CLAUDE"})
		require.NoError(t, err)
		require.Equal(t, 1, total, "arama büyük/küçük harf duyarsız olmalı")
	})

	t.Run("fiyata göre azalan", func(t *testing.T) {
		models, _, err := store.List(ctx, catalog.ListFilter{Sort: catalog.SortPrice, Desc: true})
		require.NoError(t, err)
		require.Equal(t, "anthropic/claude", models[0].ID)
	})

	t.Run("geçersiz sıralama alanı isme düşer", func(t *testing.T) {
		// Kolon adı yalnızca izinli haritadan gelir; uydurma değer SQL'e sızmaz.
		models, _, err := store.List(ctx,
			catalog.ListFilter{Sort: catalog.SortField("m.id; DROP TABLE models")})
		require.NoError(t, err)
		require.Len(t, models, 2, "tablo hâlâ yerinde olmalı")
	})
}

func TestList_LimitSinirlanir(t *testing.T) {
	f := catalog.ListFilter{Limit: 100_000, Offset: -5}
	f.Normalize()
	require.Equal(t, 500, f.Limit, "üst sınır dayatılmalı")
	require.Equal(t, 0, f.Offset)

	f = catalog.ListFilter{}
	f.Normalize()
	require.Equal(t, 50, f.Limit)
	require.Equal(t, catalog.SortName, f.Sort)
	require.Equal(t, catalog.ToolsAny, f.Tools)
}

func TestList_BosKatalogBosDilimDoner(t *testing.T) {
	store, _, _ := setup(t)

	models, total, err := store.List(context.Background(), catalog.ListFilter{})
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.NotNil(t, models, "JSON'da null yerine [] çıkmalı")
	require.Empty(t, models)
}

func TestSyncStatus_HerSaglayiciIcinSatirDoner(t *testing.T) {
	store, providers, _ := setup(t)
	ctx := context.Background()

	addProvider(t, providers, "Bir", llm.TypeLiteLLM, "https://a.local/v1")
	addProvider(t, providers, "İki", llm.TypeLiteLLM, "https://b.local/v1")

	statuses, err := store.SyncStatus(ctx)
	require.NoError(t, err)
	require.Len(t, statuses, 2)

	for _, s := range statuses {
		require.False(t, s.Stale(), "henüz senkron denenmedi, stale olmamalı")
		require.Equal(t, 0, s.ModelCount)
	}
}

func TestProviderStore_IlkSaglayiciVarsayilanOlur(t *testing.T) {
	_, providers, _ := setup(t)
	ctx := context.Background()

	p1 := addProvider(t, providers, "Bir", llm.TypeLiteLLM, "https://a.local/v1")
	require.True(t, p1.IsDefault, "ilk sağlayıcı kendiliğinden varsayılan olmalı")

	p2 := addProvider(t, providers, "İki", llm.TypeLiteLLM, "https://b.local/v1")
	require.False(t, p2.IsDefault)

	// Varsayılanı devretmek öncekini düşürür.
	updated, err := providers.Update(ctx, p2.ID, llm.UpdateInput{IsDefault: ptr(true)})
	require.NoError(t, err)
	require.True(t, updated.IsDefault)

	old, err := providers.Get(ctx, p1.ID)
	require.NoError(t, err)
	require.False(t, old.IsDefault, "eski varsayılan düşürülmeli")
}

func TestProviderStore_VarsayilanSilininceBaskasiVarsayilanOlur(t *testing.T) {
	_, providers, _ := setup(t)
	ctx := context.Background()

	p1 := addProvider(t, providers, "Bir", llm.TypeLiteLLM, "https://a.local/v1")
	p2 := addProvider(t, providers, "İki", llm.TypeLiteLLM, "https://b.local/v1")
	require.True(t, p1.IsDefault)

	require.NoError(t, providers.Delete(ctx, p1.ID))

	remaining, err := providers.Get(ctx, p2.ID)
	require.NoError(t, err)
	require.True(t, remaining.IsDefault, "kalan sağlayıcı varsayılan olmalı")
}

func TestProviderStore_SlugCakismasiCozulur(t *testing.T) {
	_, providers, _ := setup(t)

	p1 := addProvider(t, providers, "Şirket LiteLLM", llm.TypeLiteLLM, "https://a.local/v1")
	p2 := addProvider(t, providers, "Şirket LiteLLM", llm.TypeLiteLLM, "https://b.local/v1")

	require.Equal(t, "sirket-litellm", p1.Slug)
	require.Equal(t, "sirket-litellm-2", p2.Slug, "çakışan slug sayı ekleyerek çözülmeli")
}

func TestProviderStore_AnahtarKorunur(t *testing.T) {
	_, providers, _ := setup(t)
	ctx := context.Background()

	p := addProvider(t, providers, "Test", llm.TypeLiteLLM, "https://x.local/v1")

	// Yalnızca ad değiştirilince anahtar yerinde kalmalı: kullanıcı adı
	// değiştirmek için anahtarını tekrar girmek zorunda olmamalı.
	_, err := providers.Update(ctx, p.ID, llm.UpdateInput{Name: ptr("Yeni Ad")})
	require.NoError(t, err)

	secret, err := providers.Reveal(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "gizli-anahtar-Test", secret)
}

func TestProviderStore_ListeGizliDegerIcermez(t *testing.T) {
	_, providers, _ := setup(t)

	addProvider(t, providers, "Test", llm.TypeLiteLLM, "https://x.local/v1")

	list, err := providers.List(context.Background())
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "Test", list[0].Hint[:0]+list[0].Hint, "hint son 4 karakter olmalı")
	require.Len(t, list[0].Hint, 4)
}
