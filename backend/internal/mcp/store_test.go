package mcp_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/mcp"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * MCP sunucu mağazası.
 *
 * Mağazada hiç veritabanı testi yoktu; mutasyon taraması altı korumasız bekçi
 * gösterdi. İkisi benzersizlik kuralı ve tam da `scripts` paketinde bulunan
 * delikle aynı kalıptaydı: kural oluşturmada sınanıyor, güncellemede
 * sınanmıyor — yani iki adımda deliniyor.
 */

func newStore(t *testing.T) *mcp.Store {
	t.Helper()
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "agent_mcp_servers", "mcp_servers", "runs", "agents")

	cipher, err := secrets.NewCipher(
		base64.StdEncoding.EncodeToString(make([]byte, secrets.KeySize)))
	require.NoError(t, err)
	return mcp.NewStore(pool, cipher)
}

func sunucu(t *testing.T, s *mcp.Store, ad string) mcp.Server {
	t.Helper()
	srv, err := s.Create(context.Background(), mcp.CreateInput{
		Name: ad, Transport: mcp.TransportHTTP, URL: "https://mcp.example.com/x",
	})
	require.NoError(t, err)
	return srv
}

/* --------------------------------------------------- olmayan kayıt ------ */

func TestGet_OlmayanSunucu(t *testing.T) {
	s := newStore(t)
	_, err := s.Get(context.Background(), uuid.New())
	require.ErrorIs(t, err, mcp.ErrNotFound)
}

func TestReveal_OlmayanSunucu(t *testing.T) {
	s := newStore(t)
	_, err := s.Reveal(context.Background(), uuid.New())
	require.ErrorIs(t, err, mcp.ErrNotFound)
}

func TestUpdate_OlmayanSunucu(t *testing.T) {
	s := newStore(t)
	ad := "yeni"
	_, err := s.Update(context.Background(), uuid.New(), mcp.UpdateInput{Name: &ad})
	require.ErrorIs(t, err, mcp.ErrNotFound)
}

/*
OLMAYAN SUNUCU AGENT'A BAĞLANAMAZ.

Yabancı anahtar ihlali ham hâliyle dönseydi kullanıcı 500 görürdü. Oysa olan
şey belli: başka bir sekmede silinmiş bir sunucuyu seçmiş, listeyi tazelemesi
gerekiyor.
*/
func TestSetAgentServers_OlmayanSunucu(t *testing.T) {
	s := newStore(t)
	err := s.SetAgentServers(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	require.ErrorIs(t, err, mcp.ErrNotFound)
}

/* ------------------------------------------------------ benzersizlik --- */

/*
AYNI ADDA İKİ SUNUCU OLAMAZ — OLUŞTURMADA DA, YENİDEN ADLANDIRMADA DA.

Ad, agent yapılandırmasında sunucuyu seçmenin tek yolu. İki kayıt aynı adı
alsaydı kullanıcı hangisini seçtiğini bilemezdi.

İkinci test kritik: kural yalnızca oluşturmada sınansaydı iki adımda
delinirdi — "a" ve "b" kur, sonra "b"yi "a" yap. Sonuç, oluşturmada en baştan
yasaklanan durumun ta kendisi.
*/
func TestCreate_AyniAdReddedilir(t *testing.T) {
	s := newStore(t)
	sunucu(t, s, "github-mcp")

	_, err := s.Create(context.Background(), mcp.CreateInput{
		Name: "github-mcp", Transport: mcp.TransportHTTP, URL: "https://b.example.com/y"})
	require.ErrorIs(t, err, mcp.ErrDuplicateName)
}

func TestUpdate_AyniAdaTasinamaz(t *testing.T) {
	s := newStore(t)
	sunucu(t, s, "github-mcp")
	hedef := sunucu(t, s, "jira-mcp")

	ad := "github-mcp"
	_, err := s.Update(context.Background(), hedef.ID, mcp.UpdateInput{Name: &ad})
	require.ErrorIs(t, err, mcp.ErrDuplicateName)
}

/* ---------------------------------------------------------- anahtar ---- */

/*
ANAHTARI SİLMEK İÇİN AYRI BİR BAYRAK GEREKİYOR.

Boş dize "değiştirme" anlamına geliyor; aynı değer "sil" de olsaydı ikisi
ayırt edilemezdi. Bayrak olmadan, herkese açık bir sunucuya yanlışlıkla
anahtar yazan kullanıcı sunucuyu silip yeniden kurmak — ve agent
atamalarını kaybetmek — zorunda kalırdı.
*/
func TestUpdate_AnahtarYalnizcaBayrakileSilinir(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	srv, err := s.Create(ctx, mcp.CreateInput{
		Name: "anahtarli", Transport: mcp.TransportHTTP,
		URL: "https://mcp.example.com/x", Secret: "gizli-jeton"})
	require.NoError(t, err)

	// Boş dize KORUR.
	bos := ""
	_, err = s.Update(ctx, srv.ID, mcp.UpdateInput{Secret: &bos})
	require.NoError(t, err)
	acik, err := s.Reveal(ctx, srv.ID)
	require.NoError(t, err)
	require.Equal(t, "gizli-jeton", acik, "boş dize anahtarı silmemeli")

	// Bayrak SİLER.
	_, err = s.Update(ctx, srv.ID, mcp.UpdateInput{ClearSecret: true})
	require.NoError(t, err)
	acik, err = s.Reveal(ctx, srv.ID)
	require.NoError(t, err)
	require.Empty(t, acik, "bayrak anahtarı silmeliydi")
}
