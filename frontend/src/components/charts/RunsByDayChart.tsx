"use client";

import { useState } from "react";
import type { ReportDay } from "@/lib/types";
import {
  Legend,
  Tooltip,
  useWidth,
  type Series,
} from "@/components/charts/chrome";
import { niceTicks, tickIndexes } from "@/components/charts/scale";
import { formatCount, formatDayLabel, formatDayLong } from "@/components/charts/format";

/**
 * Günlük çalıştırma — sonuca göre yığılmış sütunlar.
 *
 * Yığın SONUÇ gösterir, seri değil: bu yüzden renkler durum paletinden gelir
 * (yeşil/sarı/kırmızı) ve her biri göstergede ve tabloda ADIYLA da durur.
 * Sarı, açık temada 3:1 kontrastın altındadır — etiketler ve tablo görünümü
 * bu yüzden isteğe bağlı değil, zorunlu.
 *
 * Üç yığın çizilir; iptal/zaman aşımı/kesildi/devam eden ayrımı ipucunda ve
 * tabloda tam haliyle verilir. Yedi ayrı renk yığmak grafiği okunmaz yapardı.
 */

const SERIES: Series[] = [
  { key: "succeeded", label: "Başarılı", color: "var(--color-chart-good)" },
  { key: "other", label: "Diğer", color: "var(--color-chart-other)" },
  { key: "failed", label: "Başarısız", color: "var(--color-chart-bad)" },
];

const PLOT_HEIGHT = 200;

export function RunsByDayChart({ days }: { days: ReportDay[] }) {
  const [hover, setHover] = useState<number | null>(null);
  const { ref, width } = useWidth<HTMLDivElement>();

  const max = Math.max(...days.map((d) => d.runs), 0);
  const ticks = niceTicks(max);
  const top = ticks.at(-1) ?? 1;
  const labelled = new Set(tickIndexes(days.length));

  if (max === 0) {
    return (
      <ChartShell series={SERIES}>
        <EmptyPlot />
      </ChartShell>
    );
  }

  const active = hover === null ? null : days[hover];
  const bandWidth = days.length > 0 ? width / days.length : 0;

  // Sütunlar arasındaki boşluk yüzey rengiyle ayırır; 2px kuraldır ama 90 günde
  // bant zaten ~5px kalır ve 2px boşluk sütunun yarısını yer.
  const gap = days.length > 45 ? "gap-px" : "gap-[2px]";

  return (
    <ChartShell series={SERIES}>
      <div ref={ref} className="relative">
        {/* Değer eksenini okumak için ince, kesiksiz kılavuz çizgileri. */}
        <div className="relative" style={{ height: PLOT_HEIGHT }}>
          {ticks.map((t) => (
            <div
              key={t}
              className="absolute right-0 left-0 border-t border-line"
              style={{ bottom: `${(t / top) * 100}%` }}
            >
              <span className="absolute -top-2 -left-0.5 -translate-x-full bg-surface pr-1 text-2xs tabular-nums text-ink-3">
                {formatCount(t)}
              </span>
            </div>
          ))}

          <div
            className={`absolute inset-0 flex items-end ${gap}`}
            onPointerLeave={() => setHover(null)}
          >
            {days.map((d, i) => (
              // Sütunlar odaklanılabilir DEĞİL: 90 gün 90 sekme durağı demek
              // olurdu. Klavye ve ekran okuyucu için tablo görünümü var;
              // aria-label kırılımı yine de her sütuna iliştirir.
              <div
                key={d.date}
                role="img"
                aria-label={`${formatDayLong(d.date)}: ${d.runs} çalıştırma, ${d.succeeded} başarılı, ${d.failed} başarısız`}
                onPointerEnter={() => setHover(i)}
                className="flex h-full flex-1 cursor-default items-end justify-center"
              >
                <Column day={d} top={top} dim={hover !== null && hover !== i} />
              </div>
            ))}
          </div>
        </div>

        {/*
          Zaman ekseni — yalnızca konumu anlatacak kadar etiket.

          Etiketler bandın İÇİNE değil, üstüne konumlanır: 90 günde bir bant
          ~5px kalır ve "9 Ağu" o genişliğe sığmadığı için kırpılıp kaybolurdu.
        */}
        <div className="relative mt-2 h-4">
          {days.map((d, i) =>
            labelled.has(i) ? (
              // Uç etiketler ortalanırsa yarısı kabın dışına taşar; uçlarda
              // hizalama içeri döner.
              <span
                key={d.date}
                className={`absolute text-2xs whitespace-nowrap text-ink-3 ${
                  i === 0
                    ? "left-0"
                    : i === days.length - 1
                      ? "right-0"
                      : "-translate-x-1/2"
                }`}
                style={
                  i === 0 || i === days.length - 1
                    ? undefined
                    : { left: `${((i + 0.5) / days.length) * 100}%` }
                }
              >
                {formatDayLabel(d.date)}
              </span>
            ) : null,
          )}
        </div>

        {active && (
          <Tooltip x={(hover! + 0.5) * bandWidth} width={width}>
            <DayDetail day={active} />
          </Tooltip>
        )}
      </div>
    </ChartShell>
  );
}

/** Tek bir günün sütunu. */
function Column({ day, top, dim }: { day: ReportDay; top: number; dim: boolean }) {
  const other = day.runs - day.succeeded - day.failed;

  // Üstten alta: başarısız, diğer, başarılı. Başarılı en altta durur ki
  // günden güne karşılaştırılan taban aynı yerden başlasın.
  const parts = [
    { key: "failed", value: day.failed, color: "var(--color-chart-bad)" },
    { key: "other", value: other, color: "var(--color-chart-other)" },
    { key: "succeeded", value: day.succeeded, color: "var(--color-chart-good)" },
  ].filter((p) => p.value > 0);

  return (
    <div
      className="flex w-full max-w-[24px] flex-col gap-[2px] transition-opacity duration-150"
      style={{
        height: `${(day.runs / top) * 100}%`,
        opacity: dim ? 0.45 : 1,
      }}
    >
      {parts.map((p, i) => (
        <div
          key={p.key}
          // Yalnızca en üstteki parçanın ucu yuvarlanır; taban kare kalır.
          className={i === 0 ? "rounded-t-[4px]" : ""}
          style={{
            height: `${(p.value / day.runs) * 100}%`,
            background: p.color,
          }}
        />
      ))}
    </div>
  );
}

/** İpucu içeriği — kırılımın TAMAMI burada, renk hiçbir şeyi tek başına taşımaz. */
function DayDetail({ day }: { day: ReportDay }) {
  const rows: Array<[string, number]> = [
    ["Başarılı", day.succeeded],
    ["Başarısız", day.failed],
    ["Zaman aşımı", day.timeout],
    ["İptal", day.cancelled],
    ["Kesildi", day.interrupted],
    ["Sürüyor", day.active],
  ];

  return (
    <>
      <div className="text-xs font-medium">{formatDayLong(day.date)}</div>
      <div className="mt-1.5 space-y-0.5">
        {rows
          .filter(([, v]) => v > 0)
          .map(([label, v]) => (
            <div key={label} className="flex justify-between gap-3 text-2xs">
              <span className="text-ink-2">{label}</span>
              <span className="tabular-nums">{formatCount(v)}</span>
            </div>
          ))}
        <div className="flex justify-between gap-3 border-t border-line pt-0.5 text-2xs font-medium">
          <span>Toplam</span>
          <span className="tabular-nums">{formatCount(day.runs)}</span>
        </div>
      </div>
    </>
  );
}

function ChartShell({
  series,
  children,
}: {
  series: Series[];
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-3">
      <Legend series={series} />
      <div className="pl-7">{children}</div>
    </div>
  );
}

function EmptyPlot() {
  return (
    <div
      className="flex items-center justify-center rounded-lg border border-dashed border-line text-xs text-ink-3"
      style={{ height: PLOT_HEIGHT }}
    >
      Bu dönemde çalıştırma yok
    </div>
  );
}

/**
 * Grafiğin tablo karşılığı.
 *
 * İsteğe bağlı bir ekstra değil: yığındaki sarı açık temada 3:1 kontrastın
 * altında kaldığı için sayıya renkten bağımsız bir yol açık kalmak zorunda.
 */
export function RunsByDayTable({ days }: { days: ReportDay[] }) {
  const rows = days.filter((d) => d.runs > 0);

  if (rows.length === 0) {
    return <p className="text-sm text-ink-3">Bu dönemde çalıştırma yok.</p>;
  }

  return (
    <div className="max-h-[260px] overflow-auto">
      <table className="w-full text-xs">
        <thead className="sticky top-0 bg-surface">
          <tr className="border-b border-line text-left text-2xs tracking-wide text-ink-3 uppercase">
            <th className="py-1.5 pr-3 font-medium">Gün</th>
            <th className="py-1.5 pr-3 text-right font-medium">Toplam</th>
            <th className="py-1.5 pr-3 text-right font-medium">Başarılı</th>
            <th className="py-1.5 pr-3 text-right font-medium">Başarısız</th>
            <th className="py-1.5 text-right font-medium">Diğer</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((d) => (
            <tr key={d.date} className="border-b border-line/60 last:border-0">
              <td className="py-1.5 pr-3">{formatDayLong(d.date)}</td>
              <td className="py-1.5 pr-3 text-right tabular-nums">
                {formatCount(d.runs)}
              </td>
              <td className="py-1.5 pr-3 text-right tabular-nums">
                {formatCount(d.succeeded)}
              </td>
              <td className="py-1.5 pr-3 text-right tabular-nums">
                {formatCount(d.failed)}
              </td>
              <td className="py-1.5 text-right tabular-nums">
                {formatCount(d.runs - d.succeeded - d.failed)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
