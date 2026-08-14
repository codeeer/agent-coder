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
	for _, r := range host {
		if r > 127 {
			return errors.New("ASCII dışı karakter — punycode karşılığını yazın (xn--...)")
		}
	}
	if !strings.Contains(host, ".") {
		return errors.New("en az bir nokta içermeli")
	}
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

// Match, host'un desenlerden herhangi birine uyup uymadığını söyler.
//
// BOŞ LİSTE HER HOST'U GEÇİRİR. Bu bir kolaylık değil, spec 020'nin kuralı:
// boş whitelist kısıt değil kısıtsızlıktır. Ters yorumlansaydı ayarı ilk açan
// herkesin ürünü kilitlenirdi.
func Match(desenler []Pattern, host string) bool {
	if len(desenler) == 0 {
		return true
	}
	host = normalize(host)
	for _, p := range desenler {
		if p.eslesir(host) {
			return true
		}
	}
	return false
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
