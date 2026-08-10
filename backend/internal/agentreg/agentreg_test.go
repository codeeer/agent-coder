package agentreg_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/agentreg"
	"github.com/agent-coder/backend/internal/testutil"

	"github.com/agent-coder/backend/internal/paging"
)

// ─── Gömülü tanımlar (veritabanı gerekmez) ──────────────────────────────────

func TestBuiltins_BesAgentGomulu(t *testing.T) {
	builtins := agentreg.Builtins()
	require.Len(t, builtins, 5)

	slugs := make([]string, 0, len(builtins))
	for _, b := range builtins {
		slugs = append(slugs, b.Slug)
	}
	require.ElementsMatch(t,
		[]string{"analyst", "coder", "reviewer", "tester", "upgrader"}, slugs)
}

func TestBuiltins_AlanlarDolu(t *testing.T) {
	for _, b := range agentreg.Builtins() {
		t.Run(b.Slug, func(t *testing.T) {
			require.NotEmpty(t, b.Name)
			require.NotEmpty(t, b.Description)
			require.NotEmpty(t, b.Prompt)
			require.NotContains(t, b.Prompt, "---",
				"talimat gövdesi frontmatter kalıntısı içermemeli")
		})
	}
}

func TestBuiltins_YetkilerFrontmatterdanOkunur(t *testing.T) {
	byslug := map[string]agentreg.Builtin{}
	for _, b := range agentreg.Builtins() {
		byslug[b.Slug] = b
	}

	// reviewer salt okunur olmalı: edit ve write deny.
	require.False(t, byslug["reviewer"].AllowEdit, "reviewer kod değiştirmemeli")
	require.True(t, byslug["reviewer"].AllowBash)
	require.False(t, byslug["reviewer"].AllowWebfetch)

	// analyst da kod değiştirmez ama ağa çıkabilir.
	require.False(t, byslug["analyst"].AllowEdit)
	require.True(t, byslug["analyst"].AllowWebfetch)

	// coder tam yetkili.
	require.True(t, byslug["coder"].AllowEdit)
	require.True(t, byslug["coder"].AllowBash)
}

// ─── Depo (gerçek Postgres) ─────────────────────────────────────────────────

func newStore(t *testing.T) *agentreg.Store {
	t.Helper()
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "runs", "agents")
	return agentreg.NewStore(pool, func() int { return 32 })
}

func TestSeed_HazirAgentlariYazar(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	n, err := store.Seed(ctx)
	require.NoError(t, err)
	require.Equal(t, 5, n)

	list, _, err := store.List(ctx, paging.Page{Limit: 100})
	require.NoError(t, err)
	require.Len(t, list, 5)

	for _, a := range list {
		require.Equal(t, agentreg.SourceBuiltin, a.Source)
		require.False(t, a.IsModified, "yeni tohumlanan agent değiştirilmemiş olmalı")
	}
}

func TestSeed_IkinciCalismaDuzenlemeyiEzmez(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	_, err := store.Seed(ctx)
	require.NoError(t, err)

	list, _, _ := store.List(ctx, paging.Page{Limit: 100})
	target := list[0]

	yeni := "Kullanıcının yazdığı özel talimat."
	_, err = store.Update(ctx, target.ID, agentreg.UpdateInput{Prompt: &yeni})
	require.NoError(t, err)

	// Sunucu yeniden başlarsa tohumlama tekrar çalışır; kullanıcının
	// düzenlemesini EZMEMELİ.
	_, err = store.Seed(ctx)
	require.NoError(t, err)

	after, err := store.Get(ctx, target.ID)
	require.NoError(t, err)
	require.Equal(t, yeni, after.Prompt, "tohumlama kullanıcı düzenlemesini ezmemeli")
	require.True(t, after.IsModified)
}

func TestUpdate_DegistirilmisIsaretiVeSifirlama(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	_, err := store.Seed(ctx)
	require.NoError(t, err)

	list, _, _ := store.List(ctx, paging.Page{Limit: 100})
	a := list[0]
	ozgun := a.Prompt

	yeni := "Tamamen farklı bir talimat."
	updated, err := store.Update(ctx, a.ID, agentreg.UpdateInput{Prompt: &yeni})
	require.NoError(t, err)
	require.True(t, updated.IsModified, "düzenlenen hazır agent işaretlenmeli")

	reset, err := store.Reset(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, ozgun, reset.Prompt, "sıfırlama özgün talimatı geri getirmeli")
	require.False(t, reset.IsModified)
}

func TestReset_YalnizcaHazirAgentIcin(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	custom, err := store.Create(ctx, agentreg.CreateInput{
		Name: "Benim Agent", Prompt: "talimat", AllowEdit: true,
	})
	require.NoError(t, err)

	_, err = store.Reset(ctx, custom.ID)
	require.ErrorIs(t, err, agentreg.ErrNotBuiltin)

	_, err = store.Reset(ctx, uuid.New())
	require.ErrorIs(t, err, agentreg.ErrNotFound)
}

func TestCreate_SlugUretimi(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	a, err := store.Create(ctx, agentreg.CreateInput{
		Name: "Şirket Kod İncelemecisi", Prompt: "talimat",
	})
	require.NoError(t, err)
	// Türkçe harfler ASCII karşılığına katlanır — atılmaz.
	// "Şirket" → "irket" olsaydı kimlik anlamsız kalırdı.
	require.Equal(t, "sirket-kod-incelemecisi", a.Slug)

	// Aynı slug ikinci kez alınamaz.
	_, err = store.Create(ctx, agentreg.CreateInput{
		Slug: a.Slug, Name: "Başka", Prompt: "talimat",
	})
	require.ErrorIs(t, err, agentreg.ErrSlugTaken)
}

func TestCreate_BosTalimatReddedilir(t *testing.T) {
	store := newStore(t)

	_, err := store.Create(context.Background(),
		agentreg.CreateInput{Name: "X", Prompt: "   "})
	require.ErrorIs(t, err, agentreg.ErrEmptyPrompt)
}

func TestCreate_BuyukTalimatReddedilir(t *testing.T) {
	store := newStore(t)

	_, err := store.Create(context.Background(), agentreg.CreateInput{
		Name: "X", Prompt: strings.Repeat("a", 33<<10), // sınır 32 KB
	})
	require.ErrorIs(t, err, agentreg.ErrPromptTooLarge)
}

func TestDelete_HazirAgentSilinemez(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()
	_, err := store.Seed(ctx)
	require.NoError(t, err)

	list, _, _ := store.List(ctx, paging.Page{Limit: 100})
	require.ErrorIs(t, store.Delete(ctx, list[0].ID), agentreg.ErrBuiltinDelete)
}

func TestDelete_KullaniciAgentiSilinir(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	a, err := store.Create(ctx, agentreg.CreateInput{Name: "Gecici", Prompt: "x"})
	require.NoError(t, err)

	require.NoError(t, store.Delete(ctx, a.ID))
	_, err = store.Get(ctx, a.ID)
	require.ErrorIs(t, err, agentreg.ErrNotFound)
}

func TestUpdate_YetkilerDegistirilebilir(t *testing.T) {
	store := newStore(t)
	ctx := context.Background()

	a, err := store.Create(ctx, agentreg.CreateInput{
		Name: "X", Prompt: "t", AllowEdit: true, AllowBash: true,
	})
	require.NoError(t, err)

	no := false
	updated, err := store.Update(ctx, a.ID, agentreg.UpdateInput{AllowEdit: &no})
	require.NoError(t, err)
	require.False(t, updated.AllowEdit)
	require.True(t, updated.AllowBash, "dokunulmayan yetki değişmemeli")
}
