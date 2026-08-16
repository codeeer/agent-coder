package llm_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/llm"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * LLM sağlayıcı mağazası.
 *
 * Bu paketin mağazasında HİÇ veritabanı testi yoktu; mutasyon taraması beş
 * ayrı `ErrNotFound` çevirisinin korumasız olduğunu gösterdi.
 *
 * Hata TÜRÜ ürünün cevabıdır: HTTP katmanı `ErrNotFound` görünce 404 döner,
 * ham `pgx.ErrNoRows` görünce 500 "işlem tamamlanamadı". İkincisinde,
 * silinmiş bir sağlayıcının ayar sayfasını açan kullanıcı sistemin
 * bozulduğunu sanar.
 *
 * Burada ayrıca anahtarın şifreli tutulduğu ve okunabildiği de sınanıyor —
 * mağazanın asıl işi bu.
 */

func newStore(t *testing.T) *llm.Store {
	t.Helper()
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "runs", "llm_providers")

	cipher, err := secrets.NewCipher(
		base64.StdEncoding.EncodeToString(make([]byte, secrets.KeySize)))
	require.NoError(t, err)
	return llm.NewStore(pool, cipher)
}

func saglayici(t *testing.T, s *llm.Store, ad, anahtar string) llm.Provider {
	t.Helper()
	p, err := s.Create(context.Background(), llm.CreateInput{
		Type: llm.TypeOpenRouter, Name: ad, Secret: anahtar,
	})
	require.NoError(t, err)
	return p
}

/* --------------------------------------------------- olmayan kayıt ------ */

func TestGet_OlmayanSaglayici(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, llm.ErrNotFound)
}

func TestReveal_OlmayanSaglayici(t *testing.T) {
	s := newStore(t)
	_, err := s.Reveal(context.Background(), uuid.New())
	require.ErrorIs(t, err, llm.ErrNotFound)
}

func TestUpdate_OlmayanSaglayici(t *testing.T) {
	s := newStore(t)
	ad := "yeni"
	_, err := s.Update(context.Background(), uuid.New(), llm.UpdateInput{Name: &ad})
	require.ErrorIs(t, err, llm.ErrNotFound)
}

func TestDelete_OlmayanSaglayici(t *testing.T) {
	s := newStore(t)
	require.ErrorIs(t, s.Delete(context.Background(), uuid.New()), llm.ErrNotFound)
}

/*
HİÇ SAĞLAYICI YOKKEN VARSAYILAN SORULURSA `ErrNotFound`.

Bu yol kurulumun İLK anındaki hali ve en sık geçilen yer: çalıştırma
başlatılırken varsayılan sağlayıcı okunuyor. Ham hata yukarı çıksaydı,
sağlayıcı eklemeyi unutan kullanıcı "işlem tamamlanamadı" görürdü — oysa
yapması gereken şey belli.
*/
func TestDefault_HicSaglayiciYok(t *testing.T) {
	s := newStore(t)
	_, err := s.Default(context.Background())
	require.ErrorIs(t, err, llm.ErrNotFound)
}

/* ------------------------------------------------------- anahtar -------- */

/*
ANAHTAR ŞİFRELİ YAZILIR, AÇIK OKUNUR.

Sağlayıcı kaydı listede ve ayar ekranında sürekli görünüyor; anahtarın orada
düz metin durması sızıntının en ucuz yolu olurdu. Yazarken şifrelenir,
yalnızca `Reveal` ile çözülür.
*/
func TestReveal_AnahtarSifreliSaklanirVeGeriOkunur(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	p := saglayici(t, s, "OpenRouter", "sk-gizli-anahtar")

	acik, err := s.Reveal(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "sk-gizli-anahtar", acik)

	// Listede anahtar YOK, yalnızca ipucu var.
	liste, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, liste, 1)
	require.NotContains(t, liste[0].Hint, "sk-gizli-anahtar",
		"ipucu anahtarın tamamını taşımamalı")
}

/*
BOŞ ANAHTAR MEVCUDU KORUR.

Kullanıcı yalnızca adı değiştirmek istediğinde anahtarı yeniden yazmak zorunda
kalmamalı. Boş dize "sil" sayılsaydı, ad düzenlemesi sessizce erişimi koparır
ve sonraki çalıştırma kimlik doğrulama hatasıyla düşerdi.
*/
func TestUpdate_BosAnahtarMevcuduKorur(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	p := saglayici(t, s, "OpenRouter", "sk-ilk")

	yeniAd, bos := "Yeni ad", ""
	_, err := s.Update(ctx, p.ID, llm.UpdateInput{Name: &yeniAd, Secret: &bos})
	require.NoError(t, err)

	acik, err := s.Reveal(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "sk-ilk", acik, "boş anahtar mevcudu silmemeli")
}

// İlk sağlayıcı kendiliğinden varsayılan olur (spec 002 H4): tek sağlayıcı
// varken kullanıcıdan ayrıca "varsayılan yap" beklemek anlamsız olurdu.
func TestCreate_IlkSaglayiciVarsayilanOlur(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	p := saglayici(t, s, "OpenRouter", "sk-1")

	varsayilan, err := s.Default(ctx)
	require.NoError(t, err)
	require.Equal(t, p.ID, varsayilan.ID)
}

func TestCreate_GecersizTurReddedilir(t *testing.T) {
	s := newStore(t)
	_, err := s.Create(context.Background(), llm.CreateInput{
		Type: llm.Type("uydurma"), Name: "x", Secret: "k"})
	require.ErrorIs(t, err, llm.ErrInvalidType)
}
