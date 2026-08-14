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
		items, total, err := store.List(ctx, 25, 0)
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
