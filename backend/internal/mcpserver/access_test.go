package mcpserver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * Erişim anahtarı.
 *
 * Kimlik doğrulama v1'de yok: ADRESİN KENDİSİ anahtar. Dolayısıyla bu dosyadaki
 * her davranış doğrudan güvenlik davranışı — anahtarın sızması, dışarıdan
 * herhangi birinin akış başlatabilmesi demek.
 */

func accessSetup(t *testing.T) *Access {
	t.Helper()
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "mcp_access")
	return NewAccess(pool)
}

/*
ANAHTAR İLK İSTEKTE ÜRETİLİR VE SONRA DEĞİŞMEZ.

Kullanıcının "önce anahtar oluştur" diye bir adım atması gerekmiyor. Ama her
okumada yenisi üretilseydi, kullanıcının MCP istemcisine yapıştırdığı adres bir
sonraki sayfa yenilemesinde çalışmaz olurdu.
*/
func TestToken_IlkIstekteUretilirVeKalici(t *testing.T) {
	a := accessSetup(t)
	ctx := context.Background()

	ilk, err := a.Token(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, ilk)

	ikinci, err := a.Token(ctx)
	require.NoError(t, err)
	require.Equal(t, ilk, ikinci, "anahtar her okumada değişmemeli")
}

// Anahtar TAHMİN EDİLEBİLİR OLMAMALI: adres tek savunma olduğu için kısa ya da
// düzenli bir değer, kaba kuvvetle bulunabilir demektir.
func TestToken_YeterinceUzunVeAdresGuvenli(t *testing.T) {
	a := accessSetup(t)

	token, err := a.Token(context.Background())
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(token), 40, "32 baytlık rastgelelik bekleniyor")
	require.NotContains(t, token, "/", "adreste yer alacağı için URL güvenli olmalı")
	require.NotContains(t, token, "+", "adreste yer alacağı için URL güvenli olmalı")
	require.NotContains(t, token, "=", "dolgu karakteri adreste istenmez")
}

func TestValid_DogruAnahtariKabulYanlisiRed(t *testing.T) {
	a := accessSetup(t)
	ctx := context.Background()

	token, err := a.Token(ctx)
	require.NoError(t, err)

	require.True(t, a.Valid(ctx, token))
	require.False(t, a.Valid(ctx, token+"x"))
	require.False(t, a.Valid(ctx, "tamamen-baska-bir-deger"))
}

/*
BOŞ ANAHTAR ASLA GEÇMEZ.

Ayrı olarak sınanıyor çünkü en tehlikeli hâli bu: anahtarın hiç gönderilmediği
bir istek, boş dizeyle boş dizeyi karşılaştırıp geçseydi adres koruması tümden
ortadan kalkardı.
*/
func TestValid_BosAnahtarReddedilir(t *testing.T) {
	a := accessSetup(t)
	require.False(t, a.Valid(context.Background(), ""))
}

/*
YENİLEMEDEN SONRA ESKİ ADRES ANINDA GEÇERSİZ.

Arayüzün kullanıcıya verdiği söz bu ("Eski adres anında geçersiz olur").
Tutulmazsa, anahtarı sızdığı için yenileyen kullanıcı korunduğunu sanır ama
eski adres çalışmaya devam eder.
*/
func TestRotate_EskiAnahtarAnindaGecersiz(t *testing.T) {
	a := accessSetup(t)
	ctx := context.Background()

	eski, err := a.Token(ctx)
	require.NoError(t, err)

	yeni, err := a.Rotate(ctx)
	require.NoError(t, err)
	require.NotEqual(t, eski, yeni, "yenileme gerçekten yeni bir değer üretmeli")

	require.False(t, a.Valid(ctx, eski), "eski adres hâlâ çalışıyor")
	require.True(t, a.Valid(ctx, yeni))
}

// Yenileme İKİNCİ bir kayıt oluşturmaz: tek satırlık tablo, iki geçerli adres
// aynı anda var olamamalı.
func TestRotate_TekAnahtarKalir(t *testing.T) {
	a := accessSetup(t)
	ctx := context.Background()

	_, err := a.Token(ctx)
	require.NoError(t, err)
	yeni, err := a.Rotate(ctx)
	require.NoError(t, err)

	okunan, err := a.Token(ctx)
	require.NoError(t, err)
	require.Equal(t, yeni, okunan, "okuma yenilenmiş anahtarı vermeli")
}
