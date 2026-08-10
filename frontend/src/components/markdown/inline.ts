/**
 * Satır içi Markdown ayrıştırıcısı.
 *
 * React'tan AYRI ve saf tutuluyor — çünkü buradaki bir hata tarayıcıyı
 * kilitleyebiliyor ve bunun testi DOM gerektirmeden yazılabilmeli.
 *
 * ── Neden düzenli ifade her çağrıda YENİDEN üretiliyor ────────────────────
 * `g` bayraklı bir RegExp nesnesi `lastIndex` durumunu KENDİ İÇİNDE taşır.
 * Modül düzeyinde tek bir nesne paylaşılıp fonksiyon kendini çağırdığında iç
 * çağrı `lastIndex`'i sıfırlar; dış döngü aynı eşleşmeyi sonsuza kadar
 * yeniden bulur ve her turda diziye bir eleman ekler. Sonuç: sınırsız bellek
 * tüketimi ve donan bir sekme. Bu tam olarak yaşandı (bkz. spec 005, Ölçüm 1).
 *
 * Kural: özyinelemeli bir ayrıştırıcıda `g` bayraklı düzenli ifade PAYLAŞILMAZ.
 */

/** Satır içi bir parça. */
export type InlineToken =
  | { kind: "text"; text: string }
  | { kind: "code"; text: string }
  | { kind: "strong"; children: InlineToken[] }
  | { kind: "em"; children: InlineToken[] }
  | { kind: "strike"; children: InlineToken[] }
  | { kind: "link"; href: string | null; raw: string; children: InlineToken[] };

/*
 * Sıra ÖNEMLİ: kod önce gelir, çünkü kodun içindeki yıldız ve köşeli parantez
 * işaretleme değil, metindir.
 *
 * Hiçbir alternatif iç içe niceleyici taşımıyor — güvenilmeyen metinde
 * felaket geri izlemeye (catastrophic backtracking) yol açmasınlar diye.
 */
const PATTERN =
  "(`+)([\\s\\S]*?)\\1" + // `kod`
  "|\\*\\*([\\s\\S]+?)\\*\\*" + // **kalın**
  "|__([\\s\\S]+?)__" + // __kalın__
  "|~~([\\s\\S]+?)~~" + // ~~üstü çizili~~
  "|\\[([^\\]]*)\\]\\(([^)\\s]+)\\)" + // [metin](adres)
  "|\\*([^*\\n]+?)\\*" + // *italik*
  "|_([^_\\n]+?)_"; // _italik_

/**
 * İç içe geçme sınırı.
 *
 * Girdi güvenilmeyen olduğu için derinlik sınırsız bırakılmaz: `**` gibi bir
 * işaretin binlerce kez tekrarı çağrı yığınını taşırabilir. Sınıra gelindiğinde
 * metin ham haliyle döner — hiçbir şey kaybolmaz, yalnızca biçimlenmez.
 */
const MAX_DEPTH = 12;

/**
 * Tek bir çağrının üretebileceği azami parça sayısı.
 *
 * Doğru çalışan ayrıştırmada bu sınıra ASLA gelinmez — tek bir paragrafta
 * 20.000 satır içi işaret olmaz. Sınır, olası bir ilerleme hatasının belleği
 * tüketmesini engelleyen emniyet kemeridir: kilitlenmek yerine biçimlenmemiş
 * metin göstermek her zaman daha iyidir.
 */
const MAX_TOKENS = 20_000;

/** Metni satır içi parçalara ayırır. */
export function tokenizeInline(text: string, depth = 0): InlineToken[] {
  if (text === "") return [];
  if (depth >= MAX_DEPTH) return [{ kind: "text", text }];

  // HER ÇAĞRIDA YENİ nesne: `lastIndex` durumu çağrılar arasında paylaşılmaz.
  const re = new RegExp(PATTERN, "g");

  const out: InlineToken[] = [];
  let last = 0;
  let m: RegExpExecArray | null;

  while ((m = re.exec(text)) !== null) {
    // Sıfır uzunlukta eşleşme döngüyü ilerletmez; olmaması gerekir ama
    // olursa sonsuz döngü yerine bir karakter ilerleyip devam edilir.
    if (m[0] === "") {
      re.lastIndex++;
      continue;
    }
    if (out.length >= MAX_TOKENS) break;

    if (m.index > last) out.push({ kind: "text", text: text.slice(last, m.index) });
    last = m.index + m[0].length;

    if (m[2] !== undefined) {
      out.push({ kind: "code", text: m[2] });
      continue;
    }

    const strong = m[3] ?? m[4];
    if (strong !== undefined) {
      out.push({ kind: "strong", children: tokenizeInline(strong, depth + 1) });
      continue;
    }

    if (m[5] !== undefined) {
      out.push({ kind: "strike", children: tokenizeInline(m[5], depth + 1) });
      continue;
    }

    if (m[7] !== undefined) {
      const label = m[6] || m[7];
      out.push({
        kind: "link",
        href: safeHref(m[7]),
        raw: m[0],
        children: tokenizeInline(label, depth + 1),
      });
      continue;
    }

    const em = m[8] ?? m[9];
    if (em !== undefined) {
      out.push({ kind: "em", children: tokenizeInline(em, depth + 1) });
    }
  }

  if (last < text.length) out.push({ kind: "text", text: text.slice(last) });
  return out;
}

/**
 * Bağlantı adresini güvenli mi diye süzer.
 *
 * Agent çıktısı GÜVENİLMEYEN metindir. Yalnızca beyaz listedeki şemalar
 * bağlantıya çevrilir; `javascript:` bu yolla etkisiz kalır ve düz metin
 * olarak görünür — sessizce silinmez, kullanıcı ne yazdığını görsün.
 */
export function safeHref(raw: string): string | null {
  const href = raw.trim();
  if (href === "") return null;

  if (/^(https?:\/\/|mailto:)/i.test(href)) return href;
  // Göreli ve çapa bağlantılar; şema içermedikleri için zararsız.
  if (/^(\/|\.\/|\.\.\/|#)/.test(href)) return href;
  return null;
}
