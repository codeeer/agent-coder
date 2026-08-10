"use client";

import { useWidth } from "@/components/charts/chrome";

/**
 * Kıvılcım grafiği — eksensiz, etiketsiz, tek seri.
 *
 * Kahraman rakamın yanında durur ve tek bir soruya cevap verir: **artıyor mu?**
 * Eksen ve etiket bilinçli olarak yok; onlar sayfanın altındaki tam grafiklerin
 * işi. Buraya eklenseydi hem yer kaplar hem de kahraman rakamla yarışırdı.
 *
 * Erişilebilirlik: grafik tek başına bilgi taşımaz — yanındaki değişim oranı
 * aynı bilgiyi metin olarak veriyor. `aria-label` yine de dönemi ve toplamı
 * söyler ki ekran okuyucu boş bir grafik duyurmasın.
 */
export function Sparkline({
  values,
  label,
  height = 34,
}: {
  values: number[];
  /** Ekran okuyucu için: neyin serisi olduğu. */
  label: string;
  height?: number;
}) {
  const { ref, width } = useWidth<HTMLDivElement>();

  const max = Math.max(...values, 0);
  const total = values.reduce((sum, v) => sum + v, 0);

  return (
    <div ref={ref} className="w-full">
      {width > 0 && (
        <svg
          width={width}
          height={height}
          role="img"
          aria-label={`${label}: ${values.length} günlük seyir, toplam ${total}`}
        >
          {/* Hiç veri yoksa düz bir taban çizgisi: boş bir kutu, grafiğin
              yüklenmediği izlenimi verirdi. */}
          {max === 0 ? (
            <line
              x1={0}
              y1={height - 1}
              x2={width}
              y2={height - 1}
              stroke="var(--color-line)"
              strokeWidth={1.5}
            />
          ) : (
            <path
              d={areaPath(values, width, height, max)}
              fill="var(--color-accent-soft)"
              stroke="var(--color-accent)"
              strokeWidth={1.5}
              strokeLinejoin="round"
            />
          )}
        </svg>
      )}
    </div>
  );
}

/**
 * Kapalı alan yolu üretir.
 *
 * Tek nokta olduğunda `step` sıfıra bölünürdü; o durumda çizgi tüm genişliğe
 * yayılır — tek günlük bir dönem de bir şey göstermeli.
 */
function areaPath(values: number[], width: number, height: number, max: number): string {
  const top = 2;
  const usable = height - top - 1;
  const step = values.length > 1 ? width / (values.length - 1) : width;

  const point = (v: number, i: number) => {
    const x = values.length > 1 ? i * step : width / 2;
    const y = top + usable * (1 - v / max);
    return `${Math.round(x * 10) / 10},${Math.round(y * 10) / 10}`;
  };

  const line = values.map(point).join(" L ");
  return `M ${line} L ${width},${height} L 0,${height} Z`;
}
