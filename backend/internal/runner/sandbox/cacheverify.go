package sandbox

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

/*
 * Önbellek bütünlük taraması (spec 027 H5).
 *
 * İŞ BÖLÜMÜ: yardımcı container yalnızca VERİ toplar — her artefakt için
 * kayıtlı özet ve hesaplanmış özet. KARARI Go verir.
 *
 * Neden böyle: silme kararı burada ve kabuk betiğinde "hangi biçimler geçerli
 * özet sayılır" mantığı yazılıp test edilemezdi. `.sha1` dosyalarının iki
 * biçimi var ve bozuk bir özet dosyası SİLMEYE YOL AÇMAMALI — o ayrımın
 * testinin ucuz olması gerekiyor.
 */

// VerifyResult, bir önbellek taramasının sonucu.
type VerifyResult struct {
	// Checked, özeti okunabilen ve karşılaştırılan artefakt sayısı.
	Checked int `json:"checked"`
	// Mismatched, özetiyle uyuşmayan — yani silinen — artefakt sayısı.
	Mismatched int `json:"mismatched"`
	/*
	 * Unverifiable, özeti bulunmayan VEYA okunamayan artefakt sayısı.
	 *
	 * Bunlar SİLİNMEZ. Kırpılmış bir özet dosyası yüzünden sağlam bir
	 * artefaktı silmek, doğrulamayı düzeltmesi gereken sorunun kaynağına
	 * çevirirdi (spec 027 H5).
	 */
	Unverifiable int `json:"unverifiable"`
	// Removed, önbellekten gerçekten silinen dosya sayısı.
	Removed int `json:"removed"`
}

/*
verifyScript, yardımcı container'da çalışan tarama komutu.

Her `*.jar` için tek satır üretir:

	<yol>\t<kayıtlı-özet-ham>\t<hesaplanan-özet>

Kayıtlı özet yoksa alan `-` olur. `.sha1` içeriğinin İLK ALANI alınıyor; bu,
"yalnız özet" ve "özet  ad" biçimlerinin ikisini de karşılar. Ham içerik
Go'ya taşınırken sekme kirliliği yaratmasın diye ayrıştırma burada başlıyor,
KARAR ise Go'da veriliyor.
*/
const verifyScript = `find %s -type f -name '*.jar' 2>/dev/null | while IFS= read -r f; do
  if [ -f "$f.sha1" ]; then k=$(awk '{print $1; exit}' "$f.sha1" 2>/dev/null); else k='-'; fi
  [ -z "$k" ] && k='-'
  h=$(sha1sum "$f" 2>/dev/null | awk '{print $1}')
  [ -z "$h" ] && h='-'
  printf '%%s\t%%s\t%%s\n' "$f" "$k" "$h"
done`

// verifyRow, tarama çıktısının tek satırı.
type verifyRow struct {
	Path     string
	Recorded string
	Computed string
}

/*
parseVerifyOutput, tarama çıktısını satırlara ayırır.

Bozuk veya eksik alanlı satırlar ATLANIR, hata sayılmaz: `find` çalışırken bir
koşu dosya yazıyor olabilir ve yarım bir satır yüzünden bütün taramayı
düşürmek, taramayı kullanışsız kılardı.
*/
func parseVerifyOutput(out string) []verifyRow {
	var rows []verifyRow
	for _, satir := range strings.Split(out, "\n") {
		alan := strings.Split(strings.TrimRight(satir, "\r"), "\t")
		if len(alan) != 3 || alan[0] == "" {
			continue
		}
		rows = append(rows, verifyRow{Path: alan[0], Recorded: alan[1], Computed: alan[2]})
	}
	return rows
}

/*
classifyVerifyRow, bir artefaktın silinip silinmeyeceğini söyler.

SİLME YALNIZCA tek bir durumda: kayıtlı özet OKUNABİLİYOR, hesaplanan özet
OKUNABİLİYOR ve ikisi UYUŞMUYOR. Diğer her şey "denetlenemedi".
*/
func classifyVerifyRow(row verifyRow) (sil bool, denetlendi bool) {
	kayitli, kOK := normalizeSHA1(row.Recorded)
	hesap, hOK := normalizeSHA1(row.Computed)
	if !kOK || !hOK {
		return false, false
	}
	return kayitli != hesap, true
}

/*
VerifyCache, önbelleği tarar; uyuşmayan artefaktları siler.

Tarama ve silme AYNI yardımcı container'da, tek turda yapılmaz: önce veri
toplanır, karar Go'da verilir, sonra yalnızca silinecekler ikinci bir komutla
silinir. Kabuk içinde karar vermek, "hangi biçim geçerli özet sayılır"
mantığını test edilemez bir yere koymak olurdu.
*/
func (m *Manager) VerifyCache(ctx context.Context, image string, c CacheMount) (VerifyResult, error) {
	out, err := m.runHelper(ctx, image, []CacheMount{c},
		fmt.Sprintf(verifyScript, shellQuote(c.Target)))
	if err != nil {
		return VerifyResult{}, err
	}

	var (
		sonuc     VerifyResult
		silinecek []string
	)
	for _, row := range parseVerifyOutput(out) {
		sil, denetlendi := classifyVerifyRow(row)
		if !denetlendi {
			sonuc.Unverifiable++
			continue
		}
		sonuc.Checked++
		if sil {
			sonuc.Mismatched++
			silinecek = append(silinecek, row.Path)
		}
	}

	if len(silinecek) == 0 {
		return sonuc, nil
	}

	// Silme AYRI bir turda: tarama sırasında silmek, `find`'ın altından dosya
	// çekmek olurdu.
	var komut strings.Builder
	for _, yol := range silinecek {
		komut.WriteString("rm -f " + shellQuote(yol) + " " + shellQuote(yol+".sha1") + "; ")
	}
	if _, err := m.runHelper(ctx, image, []CacheMount{c}, komut.String()); err != nil {
		return sonuc, err
	}
	sonuc.Removed = len(silinecek)
	return sonuc, nil
}

/*
npmVerifyScript, npm önbelleğini kendi aracıyla denetler.

`_cacache` biçimini ürünün bilmesi gerekmiyor; npm'in kendi bütünlük denetimi
var ve biçim değişirse onunla birlikte değişir.
*/
const npmVerifyScript = "npm cache verify 2>&1"

/*
parseNPMVerify, `npm cache verify` çıktısını okur.

SEMANTİK UYUŞMAZLIK — BİLEREK KAYDEDİLİYOR. npm, BOZULMA ile REFERANSSIZ
İÇERİĞİN TOPLANMASINI ayırmıyor: ikisini de "garbage-collected" altında
sayıyor. Bu yüzden:

  - "Content verified"          → Checked
  - "Content garbage-collected" → Removed
  - Mismatched                  → HER ZAMAN 0

Maven tarafında `Removed` "bozuktu, silindi" demek; npm tarafında "artık
gerekmiyordu, toplandı" da olabilir. `Mismatched`'i doldurmak, olmayan bir
bozulmayı rapor etmek olurdu — arayüz bu yüzden npm için "bozuk" demiyor.
*/
func parseNPMVerify(out string) VerifyResult {
	var sonuc VerifyResult
	for _, satir := range strings.Split(out, "\n") {
		satir = strings.TrimSpace(satir)
		switch {
		case strings.HasPrefix(satir, "Content verified:"):
			sonuc.Checked = ilkSayi(satir)
		case strings.HasPrefix(satir, "Content garbage-collected:"):
			sonuc.Removed = ilkSayi(satir)
		}
	}
	return sonuc
}

// ilkSayi, satırdaki ilk tam sayıyı döner; yoksa sıfır.
func ilkSayi(satir string) int {
	alan := strings.FieldsFunc(satir, func(r rune) bool { return r < '0' || r > '9' })
	if len(alan) == 0 {
		return 0
	}
	n, err := strconv.Atoi(alan[0])
	if err != nil {
		return 0
	}
	return n
}

// VerifyNPMCache, npm önbelleğini npm'in kendi aracıyla denetler.
func (m *Manager) VerifyNPMCache(ctx context.Context, image string, c CacheMount) (VerifyResult, error) {
	out, err := m.runHelper(ctx, image, []CacheMount{c}, npmVerifyScript)
	if err != nil {
		return VerifyResult{}, err
	}
	return parseNPMVerify(out), nil
}
