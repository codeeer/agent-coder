"use client";

import { useState } from "react";
import type { ReportDay } from "@/lib/types";
import {
  Legend,
  Tooltip,
  niceTicks,
  tickIndexes,
  useWidth,
  type Series,
} from "@/components/charts/chrome";
import { formatCount, formatDayLabel, formatDayLong } from "@/components/charts/format";

/**
 * Gün gün seyir — dört sayım serisi tek eksende.
 *
 * Başlık "Günlük özet" idi ve yanlış okunuyordu: otuz günlük bir pencerede
 * "günlük", dönemin gün gün kırılımını değil BUGÜNÜ çağrıştırıyor. Başlık
 * artık granülerliği söylüyor, dönemi ise panonun sağ ucundaki "30 gün"
 * yazısı.
 *
 * Öncesinde günlük kırılım iki ayrı panoya bölünmüştü: sonuçlar (yığılmış
 * sütun) ve maliyet (alan). Açılan PR'ın günlük seyri ise `ReportDay`
 * içinde VARDI ama hiçbir yerde çizilmiyordu.
 *
 * Dördü aynı eksende çizilebilir çünkü hepsi SAYIM: çalıştırma, tamamlanan,
 * başarısız, açılan PR. Maliyet bilerek dışarıda — para birimi ayrı bir
 * ölçek ve aynı eksene konsaydı iki büyüklük birbirini yanlış anlatırdı;
 * kendi panosunda duruyor.
 *
 * Çizgiler arasındaki AÇIKLIK bilgi taşıyor: "çalıştırma" ile "tamamlanan"
 * çizgileri ne kadar ayrılıyorsa o gün o kadar iş tutmamış demektir.
 *
 * Renk tek kanal değil: her seri göstergede adıyla, ipucunda ise adı ve
 * sayısıyla duruyor. Grafik hiç çizilmese bilgi eksilmez.
 */

const SERIES: Series[] = [
  { key: "runs", label: "Çalıştırma", color: "var(--color-series)" },
  { key: "succeeded", label: "Tamamlanan", color: "var(--color-chart-good)" },
  { key: "failed", label: "Başarısız", color: "var(--color-chart-bad)" },
  { key: "prsOpened", label: "Açılan PR", color: "var(--color-chart-other)" },
];

type SeriesKey = "runs" | "succeeded" | "failed" | "prsOpened";

const PLOT_HEIGHT = 200;
const AXIS_WIDTH = 34;
const TOP_PAD = 8;

export function DailyTrendChart({ days }: { days: ReportDay[] }) {
  const [hover, setHover] = useState<number | null>(null);
  const { ref, width } = useWidth<HTMLDivElement>();

  const max = Math.max(
    ...days.flatMap((d) => [d.runs, d.succeeded, d.failed, d.prsOpened]),
    0,
  );
  const ticks = niceTicks(max);
  const top = ticks[ticks.length - 1] ?? 1;

  const plotWidth = Math.max(width - AXIS_WIDTH, 0);
  // Tek günlük dönemde bölme sıfıra düşer; nokta ortaya konur.
  const step = days.length > 1 ? plotWidth / (days.length - 1) : 0;

  const x = (i: number) => (days.length > 1 ? i * step : plotWidth / 2);
  const y = (v: number) => TOP_PAD + (PLOT_HEIGHT - TOP_PAD) * (1 - v / top);

  const labelIndexes = new Set(tickIndexes(days.length));

  return (
    <div>
      <div className="mb-3">
        <Legend series={SERIES} />
      </div>

      <div ref={ref} className="relative w-full">
        {width > 0 && (
          <svg
            width={width}
            height={PLOT_HEIGHT + 22}
            role="img"
            aria-label={`Gün gün seyir — ${days.length} gün`}
            onPointerLeave={() => setHover(null)}
          >
            {/* Izgara çizgileri ve eksen etiketleri */}
            {ticks.map((t) => (
              <g key={t}>
                <line
                  x1={AXIS_WIDTH}
                  y1={y(t)}
                  x2={width}
                  y2={y(t)}
                  stroke="var(--color-line)"
                  strokeWidth={1}
                />
                <text
                  x={AXIS_WIDTH - 6}
                  y={y(t) + 3}
                  textAnchor="end"
                  className="fill-(--color-ink-3) text-[10px] tabular-nums"
                >
                  {formatCount(t)}
                </text>
              </g>
            ))}

            <g transform={`translate(${AXIS_WIDTH} 0)`}>
              {/* İmlecin bulunduğu günü işaretleyen dikey kılavuz. */}
              {hover !== null && (
                <line
                  x1={x(hover)}
                  y1={TOP_PAD}
                  x2={x(hover)}
                  y2={PLOT_HEIGHT}
                  stroke="var(--color-line-strong)"
                  strokeWidth={1}
                />
              )}

              {SERIES.map((s) => {
                const key = s.key as SeriesKey;
                const points = days.map((d, i) => `${x(i)},${y(d[key])}`);
                return (
                  <g key={s.key}>
                    <polyline
                      points={points.join(" ")}
                      fill="none"
                      stroke={s.color}
                      strokeWidth={1.75}
                      strokeLinejoin="round"
                      strokeLinecap="round"
                    />
                    {/* Nokta yalnızca imlecin durduğu günde: her gün için
                        dört nokta çizmek, otuz günlük bir dönemde grafiği
                        noktalarla doldururdu. */}
                    {hover !== null && (
                      <circle
                        cx={x(hover)}
                        cy={y(days[hover]![key])}
                        r={3}
                        fill="var(--color-surface)"
                        stroke={s.color}
                        strokeWidth={2}
                      />
                    )}
                  </g>
                );
              })}

              {/* Gün etiketleri — seyreltilmiş, üst üste binmesin. */}
              {days.map((d, i) =>
                labelIndexes.has(i) ? (
                  <text
                    key={d.date}
                    x={x(i)}
                    y={PLOT_HEIGHT + 16}
                    /* Uçlardaki etiket ORTALANMAZ: ortalandığında yarısı
                       çizim alanının dışına taşıyor ve son gün "12 A…"
                       diye kırpılıyordu. */
                    textAnchor={
                      i === 0 ? "start" : i === days.length - 1 ? "end" : "middle"
                    }
                    className="fill-(--color-ink-3) text-[10px]"
                  >
                    {formatDayLabel(d.date)}
                  </text>
                ) : null,
              )}

              {/*
                İşaretçi hedefleri EN SONDA ve şeffaf: çizgilerin üstünde
                durmalılar, yoksa ince bir çizginin üzerine gelmek imkânsız
                olurdu. Her gün için tam bir şerit.
              */}
              {days.map((d, i) => (
                <rect
                  key={`hit-${d.date}`}
                  x={days.length > 1 ? x(i) - step / 2 : 0}
                  y={0}
                  width={days.length > 1 ? step : plotWidth}
                  height={PLOT_HEIGHT}
                  fill="transparent"
                  onPointerEnter={() => setHover(i)}
                />
              ))}
            </g>
          </svg>
        )}

        {hover !== null && days[hover] && (
          <Tooltip x={AXIS_WIDTH + x(hover)} width={width}>
            <p className="text-2xs font-medium">{formatDayLong(days[hover]!.date)}</p>
            <ul className="mt-1.5 space-y-1">
              {SERIES.map((s) => (
                <li key={s.key} className="flex items-center gap-1.5 text-2xs">
                  <span
                    aria-hidden="true"
                    className="size-2 shrink-0 rounded-[2px]"
                    style={{ background: s.color }}
                  />
                  <span className="flex-1 text-ink-2">{s.label}</span>
                  <span className="tabular-nums">
                    {formatCount(days[hover]![s.key as SeriesKey])}
                  </span>
                </li>
              ))}
            </ul>
          </Tooltip>
        )}
      </div>
    </div>
  );
}
