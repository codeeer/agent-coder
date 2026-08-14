import type { SettingValue } from "@/lib/types";

/**
 * Ayar araması — eşleştirme mantığı.
 *
 * Bileşenin İÇİNDE DEĞİL: Türkçe büyük/küçük harf katlaması ("IŞIK" ↔ "ışık",
 * "İŞ" ↔ "iş") hataya en açık kısım ve bir React bileşeninin içine gömülseydi
 * doğruluğu ancak tarayıcıda, elle, tek tek denenerek görülebilirdi.
 */

/**
 * Katlama `tr` yerelinde yapılır.
 *
 * Varsayılan `toLowerCase()` ile "IŞIK" → "ışik" olur: noktasız I doğru
 * katlanmaz ve kullanıcının yazdığı "ışık" hiçbir zaman eşleşmez. Aynı şekilde
 * "İŞ" → "i̇ş" (birleşik nokta) çıkar ve "iş" aramasını kaçırır.
 */
const fold = (s: string) => s.toLocaleLowerCase("tr");

/** Sorguyu aranacak kelimelere ayırır; boş sorgu boş dizi verir. */
const terms = (q: string) => fold(q).split(/\s+/).filter(Boolean);

/**
 * Bir ayar sorguyla eşleşiyor mu.
 *
 * YALNIZCA EKRANDA GÖRÜNEN ALANLAR aranır: etiket ve açıklama. Ham anahtar
 * (`runner.timeout_minutes`) bilerek dışarıda — orada yakalanan bir eşleşme,
 * kullanıcının ekranda göremediği bir metne dayanır ve sonucun neden geldiği
 * açıklanamaz.
 *
 * Çok kelimeli sorguda kelimelerin HEPSİ eşleşmeli: kullanıcı kelime ekleyerek
 * sonucu daraltmayı bekler, genişletmeyi değil.
 */
export function settingMatches(s: SettingValue, q: string): boolean {
  const words = terms(q);
  if (words.length === 0) return true;
  const haystack = fold(`${s.label} ${s.help}`);
  return words.every((w) => haystack.includes(w));
}

/** Eşleşen ayarlar. Boş sorgu SÜZMEZ — listenin tamamı döner. */
export function filterSettings(items: SettingValue[], q: string): SettingValue[] {
  if (terms(q).length === 0) return items;
  return items.filter((s) => settingMatches(s, q));
}
