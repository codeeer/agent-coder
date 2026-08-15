package scripts_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/agentreg"
	"github.com/agent-coder/backend/internal/scripts"
	"github.com/agent-coder/backend/internal/testutil"
)

func newStore(t *testing.T) (*scripts.Store, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "agent_scripts", "scripts", "runs", "agents")
	return scripts.NewStore(pool), pool
}

// seedAgent, gerçek agent deposunu kullanarak bir agent yaratır.
//
// Elle INSERT yazılmıyor: ilk hali `source` sütununu atlamıştı ve test
// NOT NULL ihlaliyle düştü. Hatırlanan şema, okunan şema değildir — ve şema
// değişince kırılmayacak tek yol, üretimde kullanılan yoldan geçmek.
func seedAgent(t *testing.T, pool *pgxpool.Pool, slug string) uuid.UUID {
	t.Helper()
	store := agentreg.NewStore(pool, func() int { return 100_000 })
	a, err := store.Create(context.Background(), agentreg.CreateInput{
		Slug: slug, Name: slug, Prompt: "talimat",
	})
	require.NoError(t, err)
	return a.ID
}

func TestStore_CRUD(t *testing.T) {
	store, _ := newStore(t)
	ctx := context.Background()

	created, err := store.Create(ctx, scripts.CreateInput{
		Name: "upgrade-deps", Description: "Bağımlılıkları yükseltir",
		Content: "#!/bin/bash\r\nnpm update",
	})
	require.NoError(t, err)
	require.Equal(t, "#!/bin/bash\nnpm update\n", created.Content,
		"satır sonları kayıt anında temizlenmeli")

	t.Run("aynı ad ikinci kez kabul edilmez", func(t *testing.T) {
		_, err := store.Create(ctx, scripts.CreateInput{Name: "upgrade-deps", Content: "x"})
		require.ErrorIs(t, err, scripts.ErrDuplicateName)
	})

	t.Run("güncelleme kısmî olabilir", func(t *testing.T) {
		desc := "Yeni açıklama"
		updated, err := store.Update(ctx, created.ID, scripts.UpdateInput{Description: &desc}, false)
		require.NoError(t, err)
		require.Equal(t, desc, updated.Description)
		require.Equal(t, created.Content, updated.Content, "gönderilmeyen alan korunur")
	})

	t.Run("listede görünür", func(t *testing.T) {
		items, total, err := store.List(ctx, scripts.Filter{Limit: 25})
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.Len(t, items, 1)
	})

	t.Run("silinir", func(t *testing.T) {
		require.NoError(t, store.Delete(ctx, created.ID))
		_, err := store.Get(ctx, created.ID)
		require.ErrorIs(t, err, scripts.ErrNotFound)
		require.ErrorIs(t, store.Delete(ctx, created.ID), scripts.ErrNotFound)
	})
}

func TestStore_AgentAtama(t *testing.T) {
	store, pool := newStore(t)
	ctx := context.Background()

	agentID := seedAgent(t, pool, "coder")
	a, err := store.Create(ctx, scripts.CreateInput{Name: "aaa", Content: "x"})
	require.NoError(t, err)
	b, err := store.Create(ctx, scripts.CreateInput{Name: "bbb", Content: "y"})
	require.NoError(t, err)

	require.NoError(t, store.SetAgentScripts(ctx, agentID, []uuid.UUID{b.ID, a.ID}))

	list, err := store.ForAgent(ctx, agentID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	// Sıra ADA göre sabit: talimat dosyası her çalıştırmada aynı olmalı, yoksa
	// aynı agent farklı çalıştırmalarda farklı bir talimatla koşar.
	require.Equal(t, "aaa", list[0].Name)
	require.Equal(t, "bbb", list[1].Name)

	t.Run("yeniden atama tümünü değiştirir", func(t *testing.T) {
		require.NoError(t, store.SetAgentScripts(ctx, agentID, []uuid.UUID{a.ID}))
		list, err := store.ForAgent(ctx, agentID)
		require.NoError(t, err)
		require.Len(t, list, 1)
	})

	t.Run("boş liste tüm atamaları kaldırır", func(t *testing.T) {
		require.NoError(t, store.SetAgentScripts(ctx, agentID, nil))
		list, err := store.ForAgent(ctx, agentID)
		require.NoError(t, err)
		require.Empty(t, list)
	})

	t.Run("var olmayan betik reddedilir", func(t *testing.T) {
		err := store.SetAgentScripts(ctx, agentID, []uuid.UUID{uuid.New()})
		require.ErrorIs(t, err, scripts.ErrNotFound)
	})

	t.Run("betik silinince atama da düşer", func(t *testing.T) {
		require.NoError(t, store.SetAgentScripts(ctx, agentID, []uuid.UUID{a.ID}))
		require.NoError(t, store.Delete(ctx, a.ID))

		// Çalıştırmanın bozulmaması bu davranışa bağlı: atama kalsaydı
		// `ForAgent` var olmayan bir betiği çözmeye çalışırdı.
		list, err := store.ForAgent(ctx, agentID)
		require.NoError(t, err)
		require.Empty(t, list)
	})
}

/*
 * ARAMA SUNUCUDA YAPILIR (spec 022 kapanışı, ölçümle eklendi).
 *
 * Liste 10'arlı sayfalı. Arama ekranda yapılsaydı yalnızca AÇIK SAYFAYI arardı
 * ve otuz betiklik bir kütüphanede kullanıcı, var olan bir betik için "yok"
 * cevabını alırdı — hem de sessizce, üçüncü sayfada durduğunu bilmeden.
 */
func TestList_AramaAdVeAciklamadaCalisir(t *testing.T) {
	s := newFolderStore(t)
	ctx := context.Background()

	_, err := s.Create(ctx, scripts.CreateInput{
		Name: "node-yukselt", Description: "Node sürümünü 24'e çeker", Content: "echo a"})
	require.NoError(t, err)
	_, err = s.Create(ctx, scripts.CreateInput{
		Name: "pom-duzelt", Description: "Maven parent sürümünü sabitler", Content: "echo b"})
	require.NoError(t, err)

	adla, total, err := s.List(ctx, scripts.Filter{Query: "node", Limit: 25})
	require.NoError(t, err)
	require.Equal(t, 1, total, "total SÜZGECE UYAN toplamdır — sayfalama ona göre çizilir")
	require.Equal(t, "node-yukselt", adla[0].Name)

	// Kullanıcı betiği çoğu zaman ne yaptığıyla hatırlıyor, adıyla değil.
	aciklamayla, _, err := s.List(ctx, scripts.Filter{Query: "maven parent", Limit: 25})
	require.NoError(t, err)
	require.Len(t, aciklamayla, 1)
	require.Equal(t, "pom-duzelt", aciklamayla[0].Name)

	bos, total, err := s.List(ctx, scripts.Filter{Query: "hiçbir şey", Limit: 25})
	require.NoError(t, err)
	require.Empty(t, bos)
	require.Zero(t, total)
}

// Klasör süzgeci: "hepsi", "şu klasör" ve "KLASÖRSÜZ" üç ayrı sorudur.
func TestList_KlasorSuzgeci(t *testing.T) {
	s := newFolderStore(t)
	ctx := context.Background()
	f := klasor(t, s, "node-24")

	_, err := s.Create(ctx, scripts.CreateInput{
		Name: "01-baslat", Content: "echo a", FolderID: &f.ID})
	require.NoError(t, err)
	_, err = s.Create(ctx, scripts.CreateInput{Name: "tekil", Content: "echo b"})
	require.NoError(t, err)

	hepsi, total, err := s.List(ctx, scripts.Filter{Limit: 25})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, hepsi, 2)

	klasorde, _, err := s.List(ctx, scripts.Filter{FolderID: &f.ID, Limit: 25})
	require.NoError(t, err)
	require.Len(t, klasorde, 1)
	require.Equal(t, "01-baslat", klasorde[0].Name)

	// "Klasörsüz" boş bırakmakla aynı şey DEĞİL: kullanıcının "hangi betiğim
	// hiçbir kampanyaya girmemiş" sorusunun tek cevabı bu.
	klasorsuz, total, err := s.List(ctx, scripts.Filter{Unfiled: true, Limit: 25})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, "tekil", klasorsuz[0].Name)
}

// Arama ile klasör süzgeci BİRLİKTE çalışır ve sayfalamayı bozmaz.
func TestList_AramaVeSuzgecSayfalamayiBozmaz(t *testing.T) {
	s := newFolderStore(t)
	ctx := context.Background()
	f := klasor(t, s, "kampanya")

	for _, ad := range []string{"01-adim", "02-adim", "03-adim", "baska"} {
		id := &f.ID
		if ad == "baska" {
			id = nil
		}
		_, err := s.Create(ctx, scripts.CreateInput{Name: ad, Content: "echo x", FolderID: id})
		require.NoError(t, err)
	}

	ilk, total, err := s.List(ctx, scripts.Filter{Query: "adim", FolderID: &f.ID, Limit: 2, Offset: 0})
	require.NoError(t, err)
	require.Equal(t, 3, total, "toplam süzgece uyanları sayar, tabloyu değil")
	require.Len(t, ilk, 2)

	ikinci, _, err := s.List(ctx, scripts.Filter{Query: "adim", FolderID: &f.ID, Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Len(t, ikinci, 1)
	require.NotEqual(t, ilk[0].ID, ikinci[0].ID)
}
