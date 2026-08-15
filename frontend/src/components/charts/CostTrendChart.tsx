"use client";

import { useState } from "react";
import type { ReportDay } from "@/lib/types";
import {
  DayLabels,
  GridLines,
  HoverGuide,
  Tooltip,
  useWidth,
} from "@/components/charts/chrome";
import { niceTicks, tickIndexes } from "@/components/charts/scale";
import {
  formatCount,
  formatDayLabel,
  formatDayLong,
  formatMoney,
  moneyAxis,
} from "@/components/charts/format";

/**
 * Günlük maliyet — tek seri, alan + çizgi.
 *
 * Çalıştırma sayısıyla AYNI GRAFİĞE konmaz. İki ölçek tek eksene sığmaz ve
 * ikinci bir y ekseni açmak, aralarında olmayan bir ilişki uydurur.
 *
 * Tek seri olduğu için gösterge yoktur: kartın başlığı neyin çizildiğini
 * zaten söylüyor, tek gözlü bir gösterge kutusu yalnızca yer kaplar.
 */

const HEIGHT = 170;
// left, para etiketinin en uzun halini alacak kadar geniş: küçük maliyetler
// dört basamak gösterilir ("0,0025 $") ve dar bir kenarda kırpılırdı.
const PAD = { top: 10, right: 10, bottom: 22, left: 62 };

export function CostTrendChart({ days }: { days: ReportDay[] }) {
  const [hover, setHover] = useState<number | null>(null);
  const { ref, width } = useWidth<HTMLDivElement>();

  const max = Math.max(...days.map((d) => d.costUsd), 0);
  const total = days.reduce((sum, d) => sum + d.costUsd, 0);
  const last = days.at(-1);

  if (!last || total === 0) {
    return (
      <div
        className="flex items-center justify-center rounded-lg border border-dashed border-line text-xs text-ink-3"
        style={{ height: HEIGHT }}
      >
        Bu dönemde maliyet oluşmadı
      </div>
    );
  }

  const ticks = niceTicks(max, 3);
  const top = ticks.at(-1) ?? 1;
  const tickLabel = moneyAxis(top);

  const plotW = Math.max(width - PAD.left - PAD.right, 1);
  const plotH = HEIGHT - PAD.top - PAD.bottom;
  const baseY = PAD.top + plotH;

  const x = (i: number) =>
    PAD.left + (days.length <= 1 ? plotW / 2 : (i / (days.length - 1)) * plotW);
  const y = (v: number) => PAD.top + (1 - v / top) * plotH;

  const line = days
    .map((d, i) => `${i === 0 ? "M" : "L"}${x(i)},${y(d.costUsd)}`)
    .join("");
  // Alan, çizginin altını tabana kapatır — ayrı bir yol hesabı gerekmez.
  const area = `${line}L${x(days.length - 1)},${baseY}L${x(0)},${baseY}Z`;

  const labelled = new Set(tickIndexes(days.length, 6));
  const active = hover === null ? null : days[hover];

  // İmleç konumundan en yakın güne: kullanıcı tam noktanın üstüne gelmek
  // zorunda kalmasın, hedef alanı işaretten büyük olsun.
  function onMove(e: React.PointerEvent<HTMLDivElement>) {
    const rect = e.currentTarget.getBoundingClientRect();
    const rel = e.clientX - rect.left - PAD.left;
    const i = Math.round((rel / plotW) * (days.length - 1));
    setHover(Math.min(Math.max(i, 0), days.length - 1));
  }

  return (
    <div
      ref={ref}
      className="relative"
      onPointerMove={onMove}
      onPointerLeave={() => setHover(null)}
    >
      {width > 0 && (
        <svg
          width={width}
          height={HEIGHT}
          role="img"
          aria-label={`Günlük maliyet grafiği, toplam ${formatMoney(total)}`}
        >
          <GridLines
            ticks={ticks}
            y={y}
            x1={PAD.left}
            x2={width - PAD.right}
            format={tickLabel}
          />

          {/* Alan dolgusu bir yıkama: seriyi bastırmadan gövde kazandırır. */}
          <path d={area} fill="var(--color-series)" opacity={0.1} />
          <path
            d={line}
            fill="none"
            stroke="var(--color-series)"
            strokeWidth={2}
            strokeLinecap="round"
            strokeLinejoin="round"
          />

          {active && hover !== null && (
            <>
              <HoverGuide x={x(hover)} y1={PAD.top} y2={PAD.top + plotH} />
              <circle
                cx={x(hover)}
                cy={y(active.costUsd)}
                r={4}
                fill="var(--color-series)"
                stroke="var(--color-surface)"
                strokeWidth={2}
              />
            </>
          )}

          {/* Son nokta her zaman işaretli: seri nerede bittiğini söylemeli. */}
          <circle
            cx={x(days.length - 1)}
            cy={y(last.costUsd)}
            r={4}
            fill="var(--color-series)"
            stroke="var(--color-surface)"
            strokeWidth={2}
          />

          <DayLabels
            days={days}
            x={x}
            y={HEIGHT - 6}
            indexes={labelled}
            format={formatDayLabel}
          />
        </svg>
      )}

      {active && hover !== null && (
        <Tooltip x={x(hover)} width={width}>
          <div className="text-xs font-medium">{formatDayLong(active.date)}</div>
          <div className="mt-1 flex justify-between gap-3 text-2xs">
            <span className="text-ink-2">Maliyet</span>
            <span className="tabular-nums">{formatMoney(active.costUsd)}</span>
          </div>
          <div className="flex justify-between gap-3 text-2xs">
            <span className="text-ink-2">Çalıştırma</span>
            <span className="tabular-nums">{formatCount(active.runs)}</span>
          </div>
        </Tooltip>
      )}
    </div>
  );
}
