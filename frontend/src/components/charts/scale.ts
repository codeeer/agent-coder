/**
 * Grafik ölçek matematiği — saf fonksiyonlar.
 *
 * `chrome.tsx`'ten AYRI dosyada: orası React bileşeni taşıdığı için .tsx, test
 * koşucusu ise yalnızca .ts uzantılı test dosyalarını topluyor. Buradaki üç
 * fonksiyonun hiçbiri gözle doğrulanamaz — eksen yuvarlama kayan nokta
 * biriktirebiliyor, etiket seyreltme uç durumlarda çakışıyor — ve aynı
 * dosyada kaldıkları sürece doğrulukları iddiadan ibaret kalıyordu.
 */

/**
 * Eksen için yuvarlak sayılar üretir.
 *
 * Ham en büyük değeri eksen ucu yapmak "37" gibi okunmayan bir tavan bırakır;
 * 1/2/5 katlarına yuvarlamak eksen etiketlerini akılda kalır hale getirir.
 */
export function niceTicks(max: number, count = 4): number[] {
  if (max <= 0) return [0, 1];

  const rough = max / count;
  const magnitude = 10 ** Math.floor(Math.log10(rough));
  const normalized = rough / magnitude;

  let step: number;
  if (normalized <= 1) step = magnitude;
  else if (normalized <= 2) step = 2 * magnitude;
  else if (normalized <= 5) step = 5 * magnitude;
  else step = 10 * magnitude;

  // Adım TOPLANARAK değil ÇARPILARAK üretilir: 0,005 gibi bir adımı arka arkaya
  // toplamak kayan nokta hatası biriktirir ve eksende "0,0100000002" belirir.
  const steps = Math.ceil(max / step);
  return Array.from({ length: steps + 1 }, (_, i) => i * step);
}

/**
 * Zaman ekseninde gösterilecek etiketleri seyreltir.
 *
 * 90 günün 90 etiketi üst üste biner ve hiçbiri okunmaz; eksen yalnızca
 * konumu anlatacak kadar etiket taşır, geri kalanı ipucunda durur.
 */
export function tickIndexes(length: number, max = 7): number[] {
  if (length <= max) return Array.from({ length }, (_, i) => i);

  const step = (length - 1) / (max - 1);
  const out = new Set<number>();
  for (let i = 0; i < max; i++) out.add(Math.round(i * step));
  return [...out].sort((a, b) => a - b);
}

/**
 * Uçtaki etiket ORTALANMAZ.
 *
 * Ortalandığında yarısı çizim alanının dışına taşıyor ve kırpılıyor — son gün
 * "12 A…" diye görünüyordu. Kural iki grafikte de aynı ve yorumuyla birlikte
 * iki kez yazılıydı.
 */
export function edgeAnchor(i: number, length: number): "start" | "end" | "middle" {
  if (i === 0) return "start";
  if (i === length - 1) return "end";
  return "middle";
}
