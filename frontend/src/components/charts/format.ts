/**
 * Rapor sayısı biçimlendirme.
 *
 * Tek yerde durur çünkü aynı rakam grafikte, ipucunda ve tabloda görünür;
 * üç yerde ayrı biçimlendirilirse aynı veri farklı okunur.
 */

const tr = "tr-TR";

/**
 * Para birimi.
 *
 * Model maliyetleri çok küçük olabildiği için basamak sayısı büyüklüğe göre
 * değişir: 0,0009 $ değerini "0,00 $" diye göstermek "bedava" izlenimi verir.
 */
export function formatMoney(value: number): string {
  const abs = Math.abs(value);
  // Sıfır da Intl'den geçer: elle yazılmış bir "0,00 $" dizisi, biçimlendiricinin
  // ürettiğinden farklı görünür ve aynı eksende iki ayrı para biçimi belirir.
  return money(abs > 0 && abs < 0.01 ? 4 : 2).format(value);
}

function money(digits: number): Intl.NumberFormat {
  return new Intl.NumberFormat(tr, {
    style: "currency",
    currency: "USD",
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
}

/**
 * Eksen için para biçimlendirici.
 *
 * Basamak sayısı EN BÜYÜK değere göre bir kez seçilir ve tüm etiketlere
 * uygulanır: aynı eksende "0,0050 $" ile "0,01 $" yan yana durursa okuyucu
 * iki farklı birim görüyor sanır.
 */
export function moneyAxis(max: number): (value: number) => string {
  const fmt = money(max > 0 && max < 0.1 ? 4 : 2);
  return (value) => fmt.format(value);
}

/** Tam sayı — binlik ayraçlı. */
export function formatCount(value: number): string {
  return new Intl.NumberFormat(tr).format(Math.round(value));
}

/** Büyük sayıların kısa hali: 1.284 / 12,9 B / 4,2 Mn. */
export function formatCompact(value: number): string {
  const abs = Math.abs(value);
  if (abs < 10_000) return formatCount(value);
  if (abs < 1_000_000) return `${round1(value / 1_000)} B`;
  return `${round1(value / 1_000_000)} Mn`;
}

function round1(v: number): string {
  return new Intl.NumberFormat(tr, { maximumFractionDigits: 1 }).format(v);
}

/** Süre — saniyeden okunur metne. */
export function formatDuration(seconds: number): string {
  if (seconds <= 0) return "—";
  if (seconds < 60) return `${Math.round(seconds)} sn`;

  const m = Math.floor(seconds / 60);
  const s = Math.round(seconds % 60);
  if (m < 60) return s > 0 ? `${m} dk ${s} sn` : `${m} dk`;

  const h = Math.floor(m / 60);
  return `${h} sa ${m % 60} dk`;
}

/** Yüzde — bölen sıfırsa tire. */
export function formatPercent(part: number, whole: number): string {
  if (whole <= 0) return "—";
  return `%${new Intl.NumberFormat(tr, { maximumFractionDigits: 0 }).format(
    (part / whole) * 100,
  )}`;
}

// timeZone: UTC ZORUNLU — gün etiketi backend'de rapor saat diliminde
// üretildi; tarayıcının yerel saatiyle yeniden yorumlanırsa gün kayar.
const dayFormatter = new Intl.DateTimeFormat(tr, {
  day: "numeric",
  month: "short",
  timeZone: "UTC",
});

/**
 * YYYY-AA-GG → "9 Ağu".
 *
 * Tarih UTC olarak okunur: backend gün etiketini rapor saat diliminde ürettiği
 * için burada ikinci bir dönüşüm uygulanırsa gün bir kayardı.
 */
export function formatDayLabel(iso: string): string {
  return dayFormatter.format(new Date(`${iso}T00:00:00Z`));
}

/** Aynı tarihin uzun hali — ipucunda kullanılır. */
export function formatDayLong(iso: string): string {
  return new Intl.DateTimeFormat(tr, {
    day: "numeric",
    month: "long",
    weekday: "short",
    timeZone: "UTC",
  }).format(new Date(`${iso}T00:00:00Z`));
}

/** İki dönem arasındaki yüzdesel değişim; önceki dönem boşsa null. */
export function changeRatio(current: number, previous: number): number | null {
  if (previous <= 0) return null;
  return (current - previous) / previous;
}
