package agentreg_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/agentreg"
	"github.com/agent-coder/backend/internal/testutil"
)

// agentaCalistirmaBagla, agent'a bir çalıştırma kaydı bağlar — yabancı anahtar
// gerçekten kurulsun diye gerçek satır yazılıyor.
func agentaCalistirmaBagla(t *testing.T, agentID uuid.UUID) {
	t.Helper()
	pool := testutil.TestDB(t)
	ctx := context.Background()

	var projectID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, repo_url)
		 VALUES ('Silme denemesi', 'https://example.com/agentreg.git') RETURNING id`).
		Scan(&projectID))

	_, err := pool.Exec(ctx, `
		INSERT INTO runs (project_id, agent_id, agent_slug, agent_prompt, provider_slug,
		                  model_id, branch, task, status, node_version)
		VALUES ($1, $2, 'incelemeci', 'talimat', 'openrouter', 'm1', 'main',
		        'geçmiş', 'succeeded', '24')`, projectID, agentID)
	require.NoError(t, err)
}

/*
 * Agent kapıları.
 *
 * Mutasyon taramasının çıktısı: iki bekçi korumasızdı.
 */

/*
SLUG ÜRETİLEMEYEN AD REDDEDİLİR.

Slug, agent'ın container içindeki dizin adı ve API yolu. Boş ya da geçersiz
bir slug kaydedilseydi agent oluşurdu ama çalıştırılamazdı — hata çok sonra,
ilk koşuda ve anlaşılmaz bir yerde çıkardı.
*/
func TestCreate_SlugUretilemeyenAdReddedilir(t *testing.T) {
	s := newStore(t)

	for _, ad := range []string{"!!!", "---", "   "} {
		_, err := s.Create(context.Background(), agentreg.CreateInput{
			Name: ad, Prompt: "talimat"})
		require.ErrorIs(t, err, agentreg.ErrInvalidSlug, "%q kabul edilmemeli", ad)
	}
}

/*
ÇALIŞTIRMASI OLAN AGENT SİLİNEMEZ.

Yabancı anahtar ihlali ham hâliyle dönseydi kullanıcı 500 görürdü. Oysa
söylenecek şey belli: bu agent'ın geçmişi var, silmek o geçmişi de götürür ya
da götüremez — kullanıcı bunu bilerek karar vermeli.
*/
func TestDelete_CalistirmasiOlanAgentSilinemez(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	a, err := s.Create(ctx, agentreg.CreateInput{
		Slug: "incelemeci", Name: "İncelemeci", Prompt: "talimat"})
	require.NoError(t, err)

	agentaCalistirmaBagla(t, a.ID)

	require.ErrorIs(t, s.Delete(ctx, a.ID), agentreg.ErrInUse)
}
