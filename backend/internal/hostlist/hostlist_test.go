package hostlist

import (
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Bu paketin testleri iki tüketiciyi birden koruyor: ayar doğrulaması ve çıkış
 * kapısı. İkisi aynı kuralı kullanmak ZORUNDA — ayrıştırma ayarda kabul edip
 * kapıda tutmazsa, kullanıcı listeye yazdığı satırın çalıştığını sanır.
 */

func TestParse_BosSatirVeYorumAtlanir(t *testing.T) {
	desenler, err := Parse("ornek.com\n\n# yorum satırı\nbaska.com\n")
	require.NoError(t, err)
	require.Len(t, desenler, 2)
}

// Tam domain YALNIZCA kendisini açar. Subdomain'in de açılması beklenseydi
// `*.` yazmanın anlamı kalmazdı ve kullanıcı sandığından geniş bir kapı açardı.
func TestMatch_TamDomainSubdomainiAcmaz(t *testing.T) {
	desenler, err := Parse("ornek.com")
	require.NoError(t, err)

	require.True(t, Match(desenler, "ornek.com"))
	require.False(t, Match(desenler, "alt.ornek.com"))
}

// Wildcard subdomain'leri açar ama APEX'İ AÇMAZ. Apex de açılsaydı `*.ornek.com`
// ile `ornek.com` arasında fark kalmaz ve kullanıcı iki satırdan hangisini
// yazdığının bir önemi olmadığını sanırdı.
func TestMatch_WildcardSubdomainiAcarApexiAcmaz(t *testing.T) {
	desenler, err := Parse("*.ornek.com")
	require.NoError(t, err)

	require.True(t, Match(desenler, "alt.ornek.com"))
	require.True(t, Match(desenler, "derin.alt.ornek.com"))
	require.False(t, Match(desenler, "ornek.com"))
	require.False(t, Match(desenler, "baskaornek.com"))
}

// DNS adları büyük/küçük harf duyarsız ve sondaki nokta köke işaret eder.
// Normalleştirilmezse kullanıcının kopyala-yapıştır yaptığı satır sessizce
// tutmazdı — ve neden tutmadığı ekranda görünmezdi.
func TestMatch_BuyukKucukHarfVeSondakiNokta(t *testing.T) {
	desenler, err := Parse("ORNEK.com.")
	require.NoError(t, err)

	require.True(t, Match(desenler, "ornek.com"))
	require.True(t, Match(desenler, "OrNeK.CoM."))
}

// Boş liste KISIT DEĞİL, kısıtsızlıktır (spec 020). Aksi yorumlansaydı ayarı
// ilk açan herkesin ürünü kilitlenirdi.
func TestMatch_BosListeHerHostuGecirir(t *testing.T) {
	desenler, err := Parse("")
	require.NoError(t, err)
	require.Empty(t, desenler)

	require.True(t, Match(desenler, "herhangi.com"))
}

/*
 * Geçersiz satırlar KAYDEDİLMEDEN reddedilir.
 *
 * Sebebi kullanıcı deneyimi değil, güvenlik: `https://ornek.com` yazan biri o
 * adresi açtığını sanır. Sessizce kabul edilip hiçbir zaman eşleşmeyen bir
 * satır, kullanıcının listede sandığı ama olmayan bir izin demektir — ve bunu
 * ancak çalıştırma düştüğünde fark eder.
 */
func TestParse_GecersizSatirlarReddedilir(t *testing.T) {
	durumlar := map[string]string{
		"şema":              "https://ornek.com",
		"port":              "ornek.com:443",
		"yol":               "ornek.com/paket",
		"ic boşluk":         "ornek com",
		"ASCII dışı":        "örnek.com",
		"yıldız tek başına": "*",
		"wildcard boş":      "*.",
	}

	for ad, satir := range durumlar {
		t.Run(ad, func(t *testing.T) {
			_, err := Parse(satir)
			require.Error(t, err)
		})
	}
}

// Hata KAÇINCI SATIRDA olduğunu söylemeli: 40 satırlık bir listede "geçersiz
// satır" demek, kullanıcıyı satır satır aramaya bırakır.
func TestParse_HataSatirNumarasiniSoyler(t *testing.T) {
	_, err := Parse("ornek.com\nbaska.com\nhttps://bozuk.com")
	require.Error(t, err)
	require.Contains(t, err.Error(), "3")
}
