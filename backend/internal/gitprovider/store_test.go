package gitprovider_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/gitprovider"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * Git erişimi mağazası.
 *
 * Mağazada hiç veritabanı testi yoktu; mutasyon taraması üç korumasız
 * `ErrNotFound` çevirisi gösterdi. Burada saklanan şey repository'lere yazma
 * yetkisi olan bir jeton, dolayısıyla şifrelemenin gerçekten çalıştığı da
 * yazılı olmalı.
 */

func newStore(t *testing.T) *gitprovider.Store {
	t.Helper()
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "runs", "projects", "git_providers")

	cipher, err := secrets.NewCipher(
		base64.StdEncoding.EncodeToString(make([]byte, secrets.KeySize)))
	require.NoError(t, err)
	return gitprovider.NewStore(pool, cipher)
}

func erisim(t *testing.T, s *gitprovider.Store, jeton string) gitprovider.Provider {
	t.Helper()
	p, err := s.Create(context.Background(), gitprovider.CreateInput{
		Type: gitprovider.TypeGitHub, Name: "GitHub", Secret: jeton,
	})
	require.NoError(t, err)
	return p
}

func TestGet_OlmayanErisim(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, gitprovider.ErrNotFound)
}

func TestReveal_OlmayanErisim(t *testing.T) {
	s := newStore(t)
	_, err := s.Reveal(context.Background(), uuid.New())
	require.ErrorIs(t, err, gitprovider.ErrNotFound)
}

func TestUpdate_OlmayanErisim(t *testing.T) {
	s := newStore(t)
	ad := "yeni"
	_, err := s.Update(context.Background(), uuid.New(), gitprovider.UpdateInput{Name: &ad})
	require.ErrorIs(t, err, gitprovider.ErrNotFound)
}

/*
JETON ŞİFRELİ SAKLANIR VE İPUCUNDA TAMAMI GÖRÜNMEZ.

Bu jeton repository'lere YAZMA yetkisi taşıyor. Ayar ekranında düz metin
durması ya da ipucunun tamamını göstermesi, ekran paylaşımında bile sızıntı
demek olurdu.
*/
func TestReveal_JetonSifreliSaklanir(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	p := erisim(t, s, "ghp_cok-gizli-jeton")

	acik, err := s.Reveal(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "ghp_cok-gizli-jeton", acik)

	require.NotContains(t, p.Hint, "cok-gizli-jeton",
		"ipucu jetonun tamamını taşımamalı")
}

/*
BOŞ JETON MEVCUDU KORUR.

Kullanıcı yalnızca kullanıcı adını düzeltmek istediğinde jetonu yeniden yazmak
zorunda kalmamalı. Boş dize "sil" sayılsaydı, o düzenleme sessizce erişimi
koparır ve sonraki klonlama kimlik doğrulama hatasıyla düşerdi.
*/
func TestUpdate_BosJetonMevcuduKorur(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	p := erisim(t, s, "ghp_ilk-jeton")

	ad, bos := "Yeni ad", ""
	_, err := s.Update(ctx, p.ID, gitprovider.UpdateInput{Name: &ad, Secret: &bos})
	require.NoError(t, err)

	acik, err := s.Reveal(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "ghp_ilk-jeton", acik, "boş jeton mevcudu silmemeli")
}
