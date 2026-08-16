"use client";

import { useEffect, useRef, useState } from "react";

import { edgeAnchor } from "./scale";

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
        <li key={s.key} className="flex items-center gap-1.5 text-xs text-ink-2">
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
 * Yatay ızgara ve y ekseni etiketleri.
 *
 * Koordinatları ÇAĞIRAN verir: iki grafiğin çizim alanı farklı yerde başlıyor
 * (biri dolgu payıyla, diğeri kaydırılmış bir grupla). Ortaklanan şey geometri
 * değil, GÖRÜNÜM — çizgi rengi, etiketin hizası ve puntosu.
 *
 * TEK İSTİSNA, etiketin eksenden uzaklığı (`x1 - 6`). O da görünümün parçası:
 * ortaklamadan önce biri 6, diğeri 8 piksel kullanıyordu ve iki grafiğin
 * etiketleri yan yana konduğunda hizasızlık görünüyordu. Çağırana bırakılsaydı
 * ayrışma geri gelirdi.
 *
 * Ayrı ayrı yazıldıklarında gerçekten ayrıştılar: biri jetonun utility
 * karşılığını (`text-2xs`, 11px) kullanırken diğeri `text-[10px]` sabitliyordu
 * ve aynı sayfadaki iki grafik ekseni farklı puntoda çiziyordu.
 */
export function GridLines({
  ticks,
  y,
  x1,
  x2,
  format,
}: {
  ticks: number[];
  /** Değeri dikey piksel konumuna çeviren ölçek. */
  y: (value: number) => number;
  /** Izgara çizgisinin başladığı ve bittiği yatay konum. */
  x1: number;
  x2: number;
  format: (value: number) => string;
}) {
  return (
    <>
      {ticks.map((t) => (
        <g key={t}>
          <line x1={x1} x2={x2} y1={y(t)} y2={y(t)} className="stroke-line" strokeWidth={1} />
          <text
            x={x1 - 6}
            y={y(t) + 3}
            textAnchor="end"
            className="fill-ink-3 text-2xs tabular-nums"
          >
            {format(t)}
          </text>
        </g>
      ))}
    </>
  );
}

/** Seyreltilmiş gün etiketleri — hangi günlerin yazılacağını `tickIndexes` seçer. */
export function DayLabels({
  days,
  x,
  y,
  indexes,
  format,
}: {
  days: readonly { date: string }[];
  x: (i: number) => number;
  /** Etiket satırının dikey konumu (px). */
  y: number;
  indexes: ReadonlySet<number>;
  format: (iso: string) => string;
}) {
  return (
    <>
      {days.map((d, i) =>
        indexes.has(i) ? (
          <text
            key={d.date}
            x={x(i)}
            y={y}
            textAnchor={edgeAnchor(i, days.length)}
            className="fill-ink-3 text-2xs"
          >
            {format(d.date)}
          </text>
        ) : null,
      )}
    </>
  );
}

/** İmlecin durduğu günü işaretleyen dikey kılavuz. */
export function HoverGuide({ x, y1, y2 }: { x: number; y1: number; y2: number }) {
  return (
    <line x1={x} x2={x} y1={y1} y2={y2} className="stroke-line-strong" strokeWidth={1} />
  );
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
