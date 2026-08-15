package credentials

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * Kimlik bilgisi deposu.
 *
 * Buradaki değerler dışarıya çıkan sırlardır: Jira anahtarı, kurumsal paket
 * deposu token'ı. Bir sızıntı geri alınamaz — token iptal edilene kadar
 * kullanılabilir durumdadır ve nereye sızdığı bilinmez.
 */

// gizliDeger, testlerde aranacak kadar ayırt edici bir sır.
const gizliDeger = "sk-COK-GIZLI-DEGER-9f3a7c21"

func storeSetup(t *testing.T) *Store {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "credentials")

	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(make([]byte, secrets.KeySize)))
	require.NoError(t, err)
	return NewStore(pool, cipher)
}

/*
DIŞARI VERİLEN KAYIT GİZLİ DEĞERİ TAŞIMAZ.

Tipin kendi yorumu bu kuralı koyuyor ("Yeni alan eklerken bu kuralı bozmayın")
ama yapı onu zorlamıyor: birinin `Secret` alanı eklemesi yeter. Bu test o anda
kırmızıya döner.

Kontrol JSON üzerinden yapılıyor çünkü asıl risk orada: kayıt HTTP yanıtına
konuyor ve yeni bir alan sessizce tele çıkardı.
*/
func TestGet_GizliDegerDisariVerilmez(t *testing.T) {
	s := storeSetup(t)
	ctx := context.Background()

	require.NoError(t, s.Put(ctx, KindJira, gizliDeger,
		map[string]string{"base_url": "https://sirket.atlassian.net", "email": "u@e.com"}))

	c, err := s.Get(ctx, KindJira)
	require.NoError(t, err)

	raw, err := json.Marshal(c)
	require.NoError(t, err)
	require.NotContains(t, string(raw), gizliDeger,
		"kayıt HTTP yanıtına konuyor — sır tele çıkmamalı")
}

// Aynı kural LİSTEDE de geçerli; ayrı sınanıyor çünkü ayrı bir sorgu ve ayrı
// bir tarama yolu.
func TestList_GizliDegerDisariVerilmez(t *testing.T) {
	s := storeSetup(t)
	ctx := context.Background()

	require.NoError(t, s.Put(ctx, KindJira, gizliDeger, nil))
	require.NoError(t, s.Put(ctx, KindNexus, "npm-token-BASKA-SIR-1234", nil))

	list, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)

	raw, err := json.Marshal(list)
	require.NoError(t, err)
	require.NotContains(t, string(raw), gizliDeger)
	require.NotContains(t, string(raw), "npm-token-BASKA-SIR-1234")
}

/*
İPUCU SIRRIN KENDİSİ DEĞİL.

Kullanıcı "hangi anahtar tanımlı" sorusunu ipucuyla cevaplıyor. İpucu sırrın
tamamı ya da tahmin edilebilir bir parçası olsaydı, ekranı gören herkes anahtarı
öğrenmiş olurdu.
*/
func TestPut_IpucuSirriAcigaCikarmaz(t *testing.T) {
	s := storeSetup(t)
	ctx := context.Background()

	require.NoError(t, s.Put(ctx, KindJira, gizliDeger, nil))

	c, err := s.Get(ctx, KindJira)
	require.NoError(t, err)
	require.NotEmpty(t, c.Hint, "kullanıcı hangi anahtarın tanımlı olduğunu görebilmeli")
	require.NotEqual(t, gizliDeger, c.Hint)
	require.Less(t, len(c.Hint), len(gizliDeger), "ipucu sırdan kısa olmalı")
}

// Şifreleme gerçekten geri döner: yazılan sır aynen okunabilmeli, yoksa
// çalıştırma anında kimlik doğrulama sessizce düşerdi.
func TestPutReveal_SirAynenGeriGelir(t *testing.T) {
	s := storeSetup(t)
	ctx := context.Background()

	meta := map[string]string{"base_url": "https://sirket.atlassian.net", "email": "u@e.com"}
	require.NoError(t, s.Put(ctx, KindJira, gizliDeger, meta))

	secret, okunanMeta, err := s.Reveal(ctx, KindJira)
	require.NoError(t, err)
	require.Equal(t, gizliDeger, secret)
	require.Equal(t, meta, okunanMeta, "üstveri de birlikte taşınmalı")
}

/*
TÜR BAŞINA TEK KAYIT.

İkinci kez kaydetmek yeni satır oluşturmaz, mevcudu günceller (spec 001). İki
satır oluşsaydı hangisinin geçerli olduğu belirsiz kalır ve kullanıcı anahtarı
değiştirdiğini sanarken eskisi kullanılmaya devam edebilirdi.
*/
func TestPut_IkinciKayitGuncellerYeniSatirAcmaz(t *testing.T) {
	s := storeSetup(t)
	ctx := context.Background()

	require.NoError(t, s.Put(ctx, KindJira, "eski-anahtar-1111", nil))
	require.NoError(t, s.Put(ctx, KindJira, "yeni-anahtar-2222", nil))

	list, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1, "tür başına tek kayıt")

	secret, _, err := s.Reveal(ctx, KindJira)
	require.NoError(t, err)
	require.Equal(t, "yeni-anahtar-2222", secret, "güncelleme geçerli olmalı")
}

/*
BOŞ SIR KAYDEDİLMEZ.

Kaydedilseydi ortada "tanımlı" görünen ama hiçbir şeyle kimlik doğrulamayan bir
kayıt olurdu; kullanıcı ekranda tanımlı görüp neden çalışmadığını arardı.
*/
func TestPut_BosSirReddedilir(t *testing.T) {
	s := storeSetup(t)

	require.Error(t, s.Put(context.Background(), KindJira, "", nil))
}

// Bilinmeyen tür her kapıda reddedilir: veritabanındaki enum'a düşmeden önce
// ve aynı hatayla.
func TestGecersizTurHerYerdeReddedilir(t *testing.T) {
	s := storeSetup(t)
	ctx := context.Background()

	require.ErrorIs(t, s.Put(ctx, Kind("olmayan"), "x", nil), ErrInvalidKind)
	require.ErrorIs(t, s.Delete(ctx, Kind("olmayan")), ErrInvalidKind)

	_, err := s.Get(ctx, Kind("olmayan"))
	require.ErrorIs(t, err, ErrInvalidKind)

	_, _, err = s.Reveal(ctx, Kind("olmayan"))
	require.ErrorIs(t, err, ErrInvalidKind)
}

/*
TANIMSIZ KAYIT, VERİTABANI HATASINDAN AYRI.

Arayüz "tanımlı değil" ile "okunamadı" arasında farklı davranıyor: birinde
kullanıcıya ekleme formu gösteriliyor, diğerinde hata. Aynı hataya düşselerdi
kullanıcı çalışan bir kurulumda ekleme formu görürdü.
*/
func TestTanimsizKayitAyriHataDoner(t *testing.T) {
	s := storeSetup(t)
	ctx := context.Background()

	_, err := s.Get(ctx, KindJira)
	require.ErrorIs(t, err, ErrNotConfigured)

	_, _, err = s.Reveal(ctx, KindJira)
	require.ErrorIs(t, err, ErrNotConfigured)

	require.ErrorIs(t, s.Delete(ctx, KindJira), ErrNotConfigured,
		"olmayan kaydı silmek sessizce başarılı sayılmamalı")
}

// Üstveri verilmezse boş nesne saklanır; null saklansaydı okuma tarafında
// ayrıştırma düşerdi.
func TestPut_UstveriVerilmezseBosNesne(t *testing.T) {
	s := storeSetup(t)
	ctx := context.Background()

	require.NoError(t, s.Put(ctx, KindNexus, "npm-token-1234", nil))

	c, err := s.Get(ctx, KindNexus)
	require.NoError(t, err)
	require.NotNil(t, c.Metadata)
	require.Empty(t, c.Metadata)
}

// Silinen kayıt gerçekten gider: kalsaydı iptal edilmiş bir anahtar
// kullanılmaya devam ederdi.
func TestDelete_KayitGercektenGider(t *testing.T) {
	s := storeSetup(t)
	ctx := context.Background()

	require.NoError(t, s.Put(ctx, KindJira, gizliDeger, nil))
	require.NoError(t, s.Delete(ctx, KindJira))

	_, err := s.Get(ctx, KindJira)
	require.ErrorIs(t, err, ErrNotConfigured)
}
