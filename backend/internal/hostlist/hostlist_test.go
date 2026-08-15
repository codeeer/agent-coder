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

/*
 * Tek parçalı ad (nokta içermeyen) GEÇERLİDİR.
 *
 * ÖLÇÜLEREK BULUNDU: ilk sürüm "en az bir nokta içermeli" diyordu ve gerçek
 * bir çalıştırmada depo adresi `sizinti-depo` olan bir kurulumda klonlama
 * reddedildi (backend logu: "sandbox çıkışı engellendi host=sizinti-depo").
 * Kurumsal ağlarda `nexus`, `gitlab`, `artifactory` gibi tek parçalı iç adlar
 * yaygın; Docker ağındaki servis adları da öyle. Nokta şartı hepsini
 * kapatıyordu.
 */
func TestParse_TekParcaliAdGecerlidir(t *testing.T) {
	desenler, err := Parse("nexus")
	require.NoError(t, err)
	require.True(t, Match(desenler, "nexus"))
	require.False(t, Match(desenler, "nexus.sirket.local"))
}

func TestParse_TekParcaliWildcard(t *testing.T) {
	desenler, err := Parse("*.nexus")
	require.NoError(t, err)
	require.True(t, Match(desenler, "alt.nexus"))
	require.False(t, Match(desenler, "nexus"))
}

/*
 * Host / Hosts — "kullanıcı yazmasa da izinli" listesinin ilkeli.
 *
 * Testler burada, çağıranların içinde değil: aynı ilkeli hem kapının izin
 * listesi (`runs`) hem de kullanıcıya "hangi kapılar açık" diyen ekran
 * (`httpapi`) kullanıyor. Çağıranlardan birinin içinde dursaydı, ilkel
 * değiştiğinde yalnızca o taraf korunurdu.
 */

// Adresten YALNIZCA host kalır: şema, port ve yol düşer. Port özellikle
// düşüyor — izinli bir domain'e tüm portlar açıktır (kurumsal Nexus 8081'de).
func TestHosts_AdresteYalnizcaHostKalir(t *testing.T) {
	hostlar := Hosts([]string{
		"https://openrouter.ai/api/v1",
		"https://nexus.sirket.local:8081/repository/npm/",
	})

	require.Equal(t, []string{"openrouter.ai", "nexus.sirket.local"}, hostlar)
}

// Yinelenen, boş ve ayrıştırılamayan değerler sessizce atlanır: bunlar
// kullanıcının doldurmamış olabileceği ayar alanlarından geliyor.
func TestHosts_YinelenenVeBosAtlanir(t *testing.T) {
	hostlar := Hosts([]string{
		"https://ayni.local/a", "https://ayni.local/b", "", "bu adres değil",
	})

	require.Equal(t, []string{"ayni.local"}, hostlar)
}

// SIRA KORUNUR: liste kullanıcıya gösteriliyor ve her okunuşta farklı
// sıralanan bir liste değişmiş gibi okunurdu.
func TestHosts_SiraKorunur(t *testing.T) {
	hostlar := Hosts([]string{
		"https://ucuncu.local", "https://birinci.local", "https://ikinci.local",
	})

	require.Equal(t, []string{"ucuncu.local", "birinci.local", "ikinci.local"}, hostlar)
}

/*
Host'un ürettiği ad Parse'a VERİLEBİLİR olmalı.

İkisi bu pakette yan yana duruyor ama zinciri kuran çağıran: `runs` adresten
host çıkarıyor, `runner/opencode` o host'u Parse'a veriyor. Araya şema, port
ya da yol sızsaydı Parse reddederdi ve zorunlu adres — kullanıcının hiç
yazmadığı ama ürünün ihtiyaç duyduğu adres — listeye giremezdi.
*/
func TestHost_UrettigiAdParseEdilebilir(t *testing.T) {
	for _, adres := range []string{
		"https://nexus.sirket.local:8081/repository/npm/",
		"https://git.sirket.local/takim/proje.git",
		"http://sizinti-depo/proje.git", // tek parçalı iç ad
	} {
		h := Host(adres)
		require.NotEmpty(t, h, adres)

		desenler, err := Parse(h)
		require.NoError(t, err, "çıkarılan host whitelist satırı olarak geçerli olmalı: %s", adres)
		require.True(t, Match(desenler, h))
	}
}
