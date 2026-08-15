/**
 * Arama — katlama kuralı ve alan eşleştirme.
 *
 * NEDEN TEK YERDE: bu kural sekiz ekranda geçiyordu ve her biri kendi
 * kopyasını taşıyordu. Kopyalardan biri (model seçici) düz `toLowerCase()`
 * kullanıyordu; yani kural yazıldıktan sonra gerçekten ayrıştı ve ürünün bir
 * köşesinde Türkçe arama sessizce bozuk kaldı. Kopya sayısı arttıkça bunun
 * tekrarlanmaması için bir sebep de kalmıyordu.
 */

/**
 * Türkçe harf katlaması.
 *
 * Varsayılan `toLowerCase()` ile "IŞIK" → "ışik" olur: noktasız I doğru
 * katlanmaz ve kullanıcının yazdığı "ışık" hiçbir zaman eşleşmez.
 *
 * "İ" farkı daha da sinsi: varsayılan katlama "i̇" üretiyor — `i` artı ayrı bir
 * birleşik nokta, yani İKİ karakter. Yalnızca eşleşmeyi kaçırmakla kalmaz,
 * katlanmış metinde bulunan bir konumu ham metne uygulayan her kod (vurgulama
 * gibi) kayar. `tr` yerelinde katlama tek karakter verir ve konumlar korunur.
 */
export function fold(s: string): string {
  return s.toLocaleLowerCase("tr");
}

/**
 * Kullanıcının yazdığını aranabilir iğneye çevirir: kırpılmış ve katlanmış.
 *
 * Boş dize "süzme yok" demektir; çağıranlar bunu böyle okur.
 */
export function needle(q: string): string {
  return fold(q.trim());
}

/**
 * Alanlardan herhangi biri iğneyi içeriyor mu.
 *
 * BOŞ İĞNE HER KAYDI GEÇİRİR: arama kutusu boşken liste süzülmez. Ters
 * yorumlansaydı kutuya dokunmayan kullanıcı boş bir ekran görürdü.
 *
 * Dolmamış alanlar (null, undefined, boş dize) atlanır — çağıranların ayrıca
 * `.filter(Boolean)` yazmasına gerek kalmasın diye.
 */
export function matchesAny(
  alanlar: readonly (string | null | undefined)[],
  n: string,
): boolean {
  if (n === "") return true;
  return alanlar.some((v) => !!v && fold(v).includes(n));
}
