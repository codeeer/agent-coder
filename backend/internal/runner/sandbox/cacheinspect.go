package sandbox

import (
	"fmt"
	"strconv"
	"strings"
)

/*
 * Yardımcı container çıktısının ayrıştırılması (spec 027).
 *
 * Boyut ve doğrulama, volume'ü bağlayan kısa ömürlü bir container içinde
 * çalışır — backend volume'ün içini göremez (Docker host uzakta olabilir).
 * Container'dan dönen şey METİN; bu dosya onu sayıya ve karara çevirir.
 *
 * Komutlar makinece okunur çıktı verecek biçimde seçiliyor (`du -sb`): insan
 * için biçimlenmiş bir metin yerelleştirmeye ve yuvarlamaya tabi olurdu ve
 * "ne kadar yer boşalacak" onayı ondan besleniyor.
 */

// parseDuBytes, `du -sb` çıktısındaki bayt sayısını okur.
//
// Anlaşılmayan çıktı SIFIR DEĞİL HATA döner: sıfır "önbellek boş" demektir ve
// kullanıcıya yanlış bilgi verir. Bilinmeyen ile boş ayrı şeylerdir.
func parseDuBytes(out string) (int64, error) {
	alan := strings.FieldsFunc(strings.TrimSpace(out), func(r rune) bool {
		return r == '\t' || r == ' ' || r == '\n'
	})
	if len(alan) == 0 {
		return 0, fmt.Errorf("önbellek boyutu okunamadı: çıktı boş")
	}

	n, err := strconv.ParseInt(alan[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("önbellek boyutu okunamadı: %q", strings.TrimSpace(out))
	}
	return n, nil
}

// sha1Uzunluk, onaltılık gösterimde bir SHA-1 özetinin karakter sayısı.
const sha1Uzunluk = 40

/*
normalizeSHA1, `.sha1` dosyasının içeriğinden özeti çıkarır.

İKİ BİÇİM DE KABUL EDİLİR — Maven'ın yerel deposunda ikisi de bulunuyor:

	2fd4e1c6…                        (yalnız özet)
	2fd4e1c6…  kutuphane-1.0.jar     (sha1sum biçimi)

Yalnız birini kabul etmek diğerini "özeti yok" sayardı; o artefakt sessizce
denetlenemeyenler kutusuna düşer ve tarama eksik çalışırdı.

Geçersiz içerik "uyuşmuyor" DEĞİL, "okunamadı" sayılır (ikinci dönüş değeri).
Fark hayati: uyuşmayan artefakt siliniyor. Kırpılmış bir özet dosyası yüzünden
sağlam bir artefaktı silmek, doğrulamayı sorunun kaynağına çevirirdi.
*/
func normalizeSHA1(raw string) (string, bool) {
	alan := strings.Fields(raw)
	if len(alan) == 0 {
		return "", false
	}

	ozet := strings.ToLower(alan[0])
	if len(ozet) != sha1Uzunluk {
		return "", false
	}
	for _, r := range ozet {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", false
		}
	}
	return ozet, true
}
