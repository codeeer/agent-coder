// Package hostlist, çıkış whitelist'ini ayrıştırır ve host eşleştirir.
//
// NEDEN AYRI PAKET: aynı kuralı iki yer kullanıyor — ayar doğrulaması
// (kullanıcı kaydederken) ve çıkış kapısı (çalıştırma sırasında karar verirken).
// İki kopya yazılsaydı er geç ayrışırlardı ve ortaya en kötü hata çıkardı:
// ayarda kabul edilen bir satırın kapıda tutmaması. Kullanıcı listeye yazdığı
// adresin açık olduğunu sanırken kapalı olurdu — ya da tersi.
//
// certfmt paketiyle aynı gerekçe: doğrulama mantığı, onu çağıran katmanın
// içinde değil kendi paketinde durur.
package hostlist

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

/*
 * gecerliMi, bir satırın domain OLARAK okunabildiğini sınar.
 *
 * Reddedilenlerin ortak noktası: hepsi kullanıcının "bu adresi açtım"
 * sanmasına yol açar ama hiçbir zaman eşleşmez. Kapı yalnızca host'a bakıyor;
 * `https://` şeması, `:443` portu veya `/paket` yolu taşıyan bir satır hiçbir
 * host'la karşılaştırılamaz. Sessizce kabul etmek, listede sanılan ama olmayan
 * bir izin bırakırdı.
 *
 * ASCII dışı karakter de reddediliyor: DNS'te uluslararası adlar punycode
 * (`xn--`) olarak taşınıyor ve kapının gördüğü ad odur. `örnek.com` yazan bir
 * satır telde asla görünmezdi. Kullanıcıya punycode karşılığını yazması
 * söylenir.
 */
func gecerliMi(host string) error {
	if host == "" {
		return errors.New("boş")
	}
	if strings.Contains(host, "://") {
		return errors.New("adres değil domain yazın (şema olmadan)")
	}
	if strings.Contains(host, "/") {
		return errors.New("yol içeremez, yalnızca domain yazın")
	}
	if strings.Contains(host, ":") {
		return errors.New("port içeremez — izinli domain'e tüm portlar açıktır")
	}
	if strings.ContainsAny(host, " \t") {
		return errors.New("boşluk içeremez")
	}
	/*
	 * Yıldız yalnızca `*.ornek.com` önekinde geçerli — o da buraya gelmeden
	 * ayıklanıyor. Geriye kalan her yıldız hatadır.
	 *
	 * Tek başına `*` özellikle reddediliyor: "her şeye izin" demek isteyen
	 * kullanıcı listeyi BOŞ bırakmalı. İki ayrı yazımın aynı anlama gelmesi,
	 * "acaba `*` gerçekten her şeyi mi açıyor" sorusunu doğururdu.
	 */
	if strings.Contains(host, "*") {
		return errors.New("yıldız yalnızca `*.ornek.com` biçiminde kullanılır; " +
			"her adrese izin için listeyi boş bırakın")
	}
	for _, r := range host {
		if r > 127 {
			return errors.New("ASCII dışı karakter — punycode karşılığını yazın (xn--...)")
		}
	}
	/*
	 * NOKTA ŞARTI YOK — kaldırıldı, sebebi ölçüm.
	 *
	 * İlk sürüm "en az bir nokta içermeli" diyordu. Gerçek bir çalıştırmada
	 * depo adresi tek parçalı bir Docker servis adı (`sizinti-depo`) olduğu
	 * için otomatik izinliler listesine hiç giremedi ve klonlama reddedildi.
	 * Kurumsal ağlarda `nexus`, `gitlab` gibi tek parçalı iç adlar yaygın;
	 * şart, çalışması gereken kurulumları kapatıyordu.
	 */
	return nil
}

// Pattern, whitelist'teki tek bir satır.
type Pattern struct {
	// host, karşılaştırılacak ad. Wildcard'da başındaki "*." atılmış hâli.
	host string
	// wildcard true ise yalnızca subdomain'ler eşleşir, apex eşleşmez.
	wildcard bool
}

// Parse, whitelist metnini desenlere çevirir.
//
// Boş satır ve `#` ile başlayan yorum satırı atlanır: kullanıcının listeyi
// gruplayıp açıklama yazabilmesi, uzun listelerde okunabilirliğin tek yolu.
func Parse(text string) ([]Pattern, error) {
	var desenler []Pattern
	for no, satir := range strings.Split(text, "\n") {
		satir = strings.TrimSpace(satir)
		if satir == "" || strings.HasPrefix(satir, "#") {
			continue
		}

		p := Pattern{}
		if strings.HasPrefix(satir, "*.") {
			p.wildcard = true
			satir = strings.TrimPrefix(satir, "*.")
		}
		if err := gecerliMi(satir); err != nil {
			return nil, fmt.Errorf("%d. satır: %w", no+1, err)
		}
		p.host = normalize(satir)
		desenler = append(desenler, p)
	}
	return desenler, nil
}

/*
Match, host'un İZİN LİSTESİNDEN geçip geçmediğini söyler.

BOŞ LİSTE HER HOST'U GEÇİRİR. Bu bir kolaylık değil, spec 020'nin kuralı:
boş whitelist kısıt değil kısıtsızlıktır. Ters yorumlansaydı ayarı ilk açan
herkesin ürünü kilitlenirdi.

`Listed` ile KARIŞTIRMAYIN — ikisi aynı imzayı taşıyor, yani yanlış olanı
çağırmak derlenir ve sessizce ters davranır. Bu fonksiyon "geçebilir mi",
diğeri "listede mi" sorusunu yanıtlar. Bir kümede üyelik arıyorsanız
`Listed` istiyorsunuz.
*/
func Match(desenler []Pattern, host string) bool {
	if len(desenler) == 0 {
		return true
	}
	return eslesenVar(desenler, host)
}

/*
Listed, host'un listede olup olmadığını söyler.

BOŞ LİSTE HİÇBİR HOST'U İÇERMEZ — `Match`'ten tek farkı bu ve bilinçli.
`Match` bir izin kapısı; orada boş liste "kapı yok" demek. Burada ise soru
üyelik: boş kümede hiçbir şey yoktur.

NEDEN AYRI FONKSİYON: fark yalnızca boş listede ortaya çıkıyor, yani yanlış
olanı çağıran kod normal ayarlarla ÇALIŞIYOR görünür ve ancak liste
boşaldığında ters davranır. Spec 026'da bunun bedeli ölçülebilir: kurum içi
listesi boşken `Match` kullanılsaydı her hedef doğrudan gider ve kurumsal
proxy sessizce devre dışı kalırdı.
*/
func Listed(desenler []Pattern, host string) bool {
	if len(desenler) == 0 {
		return false
	}
	return eslesenVar(desenler, host)
}

// eslesenVar, ortak eşleştirme. Boş liste kararı ÇAĞIRANIN işi — iki
// fonksiyonun ayrıldığı tek nokta orası.
func eslesenVar(desenler []Pattern, host string) bool {
	host = normalize(host)
	for _, p := range desenler {
		if p.eslesir(host) {
			return true
		}
	}
	return false
}

/*
Host, bir YAPILANDIRMA ADRESİNDEN host adını çıkarır.

Boş, ayrıştırılamayan veya host taşımayan değer için boş döner — hata değil.
Bu adreslerin hepsi kullanıcının doldurmamış olabileceği ayar alanlarından
geliyor (LLM sağlayıcı, registry, MCP sunucusu). Yapılandırılmamış bir alan
yüzünden çalıştırmayı reddetmek ya da ekranda "bozuk adres" göstermek,
kullanıcının hiç dokunmadığı bir yer için gürültü çıkarmak olurdu.

`normalize` UYGULANMAZ: dönen ad `Parse`'a veriliyor ve normalleştirme orada
zaten yapılıyor. Burada da yapmak, aynı işin iki yerde durması olurdu.

Kapının istek üzerinden gördüğü hedef host BAŞKA bir şeydir ve netgate'in
kendi işidir: orada kaynak bir URL değil `host:port` biçiminde bir istek
hedefidir.
*/
func Host(adres string) string {
	if adres == "" {
		return ""
	}
	u, err := url.Parse(adres)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

/*
Hosts, adres listesinden SIRAYI KORUYAN, yinelemesiz host listesi üretir.

İKİ ÇAĞIRANI VAR ve ikisi de aynı soruya cevap veriyor: "kullanıcı yazmasa da
hangi adresler izinli". Biri kapının gerçekten izin verdiği listeyi kuruyor
(`runs`), diğeri kullanıcıya gösterilen listeyi (`httpapi`). Ayrı kopyalarla
hesaplandıklarında biri değişip diğeri kalabilir ve ekran, açık olan kapıları
yanlış gösterir — oysa o ekranın var olma sebebi tam da kullanıcının bilmediği
bir kapı bırakmamak. Bu yüzden ilkel burada, iki çağıranın da altında duruyor.

Sıra korunuyor: liste kullanıcıya gösteriliyor ve her okunuşta farklı sıralanan
bir liste, değişmiş gibi okunurdu.
*/
func Hosts(adresler []string) []string {
	gorulen := make(map[string]bool, len(adresler))
	var hostlar []string
	for _, a := range adresler {
		h := Host(a)
		if h == "" || gorulen[h] {
			continue
		}
		gorulen[h] = true
		hostlar = append(hostlar, h)
	}
	return hostlar
}

// normalize, DNS adını karşılaştırılabilir hâle getirir: küçük harf ve sondaki
// kök noktası atılmış. İkisi de DNS'te anlamsız fark; normalleştirilmezse
// kullanıcının kopyaladığı satır sessizce tutmazdı.
func normalize(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

func (p Pattern) eslesir(host string) bool {
	if p.wildcard {
		return strings.HasSuffix(host, "."+p.host)
	}
	return host == p.host
}
