"use client";

import { useEffect, useRef, useState } from "react";

/**
 * Grafik iskeleti — her grafiğin ortak parçaları.
 *
 * Kural: iki veya daha fazla seri varsa gösterge (legend) HER ZAMAN durur.
 * Renk tek başına kimlik taşımaz; renk körlüğünde veya siyah-beyaz çıktıda
 * okunabilen tek kanal etikettir.
 */

/** Bir grafik serisinin kimliği. */
export interface Series {
  key: string;
  label: string;
  /** CSS renk değeri — işaretin rengi, metnin değil. */
  color: string;
}

/**
 * Gösterge.
 *
 * Metin ASLA seri rengini giymez: açık renkler yazı olarak okunmaz. Kimlik,
 * metnin yanındaki renkli kareden gelir.
 */
export function Legend({ series }: { series: Series[] }) {
  return (
    <ul className="flex flex-wrap items-center gap-x-4 gap-y-1.5">
      {series.map((s) => (
        <li key={s.key} className="flex items-center gap-1.5 text-[12px] text-ink-2">
          <span
            aria-hidden="true"
            className="size-2.5 shrink-0 rounded-[3px]"
            style={{ background: s.color }}
          />
          {s.label}
        </li>
      ))}
    </ul>
  );
}

/**
 * Grafiğin genişliğini ölçer.
 *
 * SVG gerçek piksel boyutunda çizilir; viewBox'ı esnetmek 2px'lik çizgiyi ve
 * daire yarıçaplarını da esneterek işaret ölçülerini bozardı.
 */
export function useWidth<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  const [width, setWidth] = useState(0);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const ro = new ResizeObserver((entries) => {
      const w = entries[0]?.contentRect.width ?? 0;
      setWidth(Math.round(w));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  return { ref, width };
}

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
 * Grafik ipucu kutusu.
 *
 * Konum, kabın kenarlarına göre kırpılır: son sütunun ipucu ortalanırsa
 * kartın dışına taşar ve okunamaz.
 */
export function Tooltip({
  x,
  width,
  children,
}: {
  /** İpucunun bağlı olduğu noktanın kap içindeki yatay konumu (px). */
  x: number;
  /** Kabın genişliği (px). */
  width: number;
  children: React.ReactNode;
}) {
  const BOX = 190;
  const left = Math.min(Math.max(x - BOX / 2, 0), Math.max(width - BOX, 0));

  return (
    <div
      className="pointer-events-none absolute top-0 z-10 rounded-lg border border-line bg-overlay px-3 py-2 shadow-(--shadow-pop)"
      style={{ left, width: BOX }}
    >
      {children}
    </div>
  );
}
