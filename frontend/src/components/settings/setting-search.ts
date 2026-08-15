import { fold } from "../../lib/search.ts";
import type { SettingValue } from "@/lib/types";

/**
 * Ayar araması — eşleştirme mantığı.
 *
 * Bileşenin İÇİNDE DEĞİL: doğruluğu ancak tarayıcıda, elle, tek tek denenerek
 * görülebilirdi.
 *
 * Katlama kuralı `lib/search`'ten geliyor ve gerekçesi orada. Buradaki fark
 * EŞLEŞTİRME BİÇİMİ: ayar araması çok kelimeli ve VE'li, ürünün geri kalanı
 * ise tek parça alt dize arıyor. İkisi bilinçli olarak ayrı (aşağıya bakın),
 * bu yüzden `matchesAny` kullanılmıyor.
 */

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
