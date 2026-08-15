package slug

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Ad → makine kimliği.
 *
 * Kimlik bir kez üretilip kaydediliyor ve sonradan adres, dosya adı ve yetki
 * kuralı olarak kullanılıyor. Bozuk üretilen bir kimlik geriye dönük
 * düzeltilemez: onu referans alan her şey de bozulur.
 */

/*
TÜRKÇE HARFLER ASCII KARŞILIĞINA DÜŞER.

Ölçülmüş arıza: `strings.ToLower` tek başına yetmiyor — "ş" ve "ğ" ASCII'ye hiç
düşmediği için sessizce ATILIYOR ve "Şirket" → "irket" gibi anlamsız kimlikler
oluşuyordu. Kullanıcı adını doğru yazıyor, ürün onu tanınmaz hale getiriyordu.
*/
func TestMake_TurkceHarflerKarsiliginaDuser(t *testing.T) {
	require.Equal(t, "sirket", Make("Şirket", "x"))
	require.Equal(t, "cigdem", Make("Çiğdem", "x"))
	require.Equal(t, "isik-guclu", Make("Işık Güçlü", "x"))
	require.Equal(t, "oder-ustu", Make("Öder Üstü", "x"))
}

/*
NOKTALI İ'NİN AYRIK YAZIMI DA KATLANIR.

"İ" bazı kaynaklarda tek karakter, bazılarında `i` + birleşik nokta olarak
geliyor (kopyala-yapıştırda sık). İkincisi ele alınmasaydı kimlikte görünmez
bir karakter kalır ve eşleşmeler sessizce tutmazdı.
*/
func TestMake_AyrikNoktaliIKatlanir(t *testing.T) {
	require.Equal(t, "istanbul", Make("İstanbul", "x"))
	require.Equal(t, "istanbul", Make("i̇stanbul", "x"))
}

func TestMake_BuyukHarfVeBosluk(t *testing.T) {
	require.Equal(t, "node-24-yukseltme", Make("Node 24 Yükseltme", "x"))
}

// Geçersiz karakterler tireye dönüşür ve UÇLARDA tire kalmaz: `-ad-` biçiminde
// bir kimlik adres olarak da dosya adı olarak da çirkin ve `Valid` onu reddeder.
func TestMake_UclardakiTirelerAtilir(t *testing.T) {
	require.Equal(t, "odeme-servisi", Make("  ödeme / servisi!! ", "x"))
	require.Equal(t, "abc", Make("---abc---", "x"))
}

/*
KULLANILABİLİR HİÇBİR KARAKTER YOKSA YEDEK AD DÖNER.

Kimliksiz kayıt oluşamaz. Boş dönülseydi, veritabanı kısıtı devreye girene
kadar hata görünmez ve kullanıcı "kaydedilemedi" mesajını adıyla
ilişkilendiremezdi.
*/
func TestMake_KullanilabilirKarakterYoksaYedek(t *testing.T) {
	require.Equal(t, "yedek", Make("!!! ???", "yedek"))
	require.Equal(t, "yedek", Make("", "yedek"))
	require.Equal(t, "yedek", Make("   ", "yedek"))
}

// Uzun ad kırpılır ve kırpma SONRASI uçta tire bırakmaz — kırpma tam bir
// tirenin üstüne denk gelebiliyor.
func TestMake_UzunAdKirpilirVeUctaTireBirakmaz(t *testing.T) {
	uzun := Make(strings.Repeat("ab ", 40), "x")

	require.LessOrEqual(t, len(uzun), MaxLength)
	require.False(t, strings.HasSuffix(uzun, "-"), "kırpma sonrası tire kalmamalı")
	require.True(t, Valid(uzun))
}

/*
ÜRETİLEN HER KİMLİK GEÇERLİDİR.

İki fonksiyon aynı kuralın iki yüzü: biri üretiyor, diğeri kabul ediyor.
Ayrıştıkları an kayıt, kendi doğrulayıcısının reddettiği bir kimlik alır —
ve bu ancak ikinci bir işlemde, hiç beklenmeyen bir yerde patlar.
*/
func TestMake_UrettigiKimlikHerZamanGecerli(t *testing.T) {
	adlar := []string{
		"Şirket", "Çiğdem Işık", "Node 24 Yükseltme", "---abc---",
		"a", "1", "ödeme/servisi", "  boşluklu  ad  ",
		strings.Repeat("uzun ad ", 20),
		"ÜST ÜSTE   BOŞLUK", "tire-zaten-var", "sayı123",
		"i̇stanbul", "ÇÖĞİŞÜ",
	}

	for _, ad := range adlar {
		id := Make(ad, "yedek")
		require.True(t, Valid(id), "üretilen kimlik geçersiz: %q → %q", ad, id)
	}
}

// ── Valid ───────────────────────────────────────────────────────────────────

func TestValid_KabulEdilenBicimler(t *testing.T) {
	require.True(t, Valid("a"))
	require.True(t, Valid("abc"))
	require.True(t, Valid("a-b-c"))
	require.True(t, Valid("node-24"))
	require.True(t, Valid("123"))
}

/*
UÇTA TİRE, BÜYÜK HARF VE BOŞLUK REDDEDİLİR.

Kimlik adreste ve dosya adında geçiyor; bu biçimler ya çirkin bir adres üretir
ya da dosya sisteminde beklenmedik davranır.
*/
func TestValid_ReddedilenBicimler(t *testing.T) {
	require.False(t, Valid(""), "boş kimlik olamaz")
	require.False(t, Valid("-abc"))
	require.False(t, Valid("abc-"))
	require.False(t, Valid("ABC"), "büyük harf kimliği iki farklı yazımla eşleştirirdi")
	require.False(t, Valid("a b"))
	require.False(t, Valid("ödeme"), "ASCII dışı karakter adreste taşınamaz")
	require.False(t, Valid("a_b"), "alt çizgi bu kuralın parçası değil")
}

func TestValid_UzunlukSiniri(t *testing.T) {
	require.True(t, Valid(strings.Repeat("a", MaxLength)))
	require.False(t, Valid(strings.Repeat("a", MaxLength+1)))
}
