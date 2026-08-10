"use client";

import { formatCount } from "@/components/charts/format";

/**
 * Yatay çubuk listesi — kategorilerin büyüklük karşılaştırması.
 *
 * Tek renk kullanılır. Büyüklüğü zaten UZUNLUK taşıyor; her çubuğa ayrı bir
 * renk vermek renge sahte bir anlam yükler ve okuyucu boşuna anlam arar.
 *
 * Etiket çubuğun ÜSTÜNDE durur, içinde değil: agent ve model adları uzundur
 * ve çubuk içine sığmadığında kırpmak kelimenin başını veya sonunu yer.
 */

export interface BarRow {
  key: string;
  label: string;
  value: number;
  /** Etiketin altındaki ikincil bilgi (maliyet, süre gibi). */
  note?: string;
  /** Çubuğun ucundaki metin; verilmezse sayı biçimlendirilerek yazılır. */
  valueLabel?: string;
}

export function BarList({
  rows,
  emptyLabel = "Bu dönemde kayıt yok",
}: {
  rows: BarRow[];
  emptyLabel?: string;
}) {
  if (rows.length === 0) {
    return <p className="text-[13px] text-ink-3">{emptyLabel}</p>;
  }

  const max = Math.max(...rows.map((r) => r.value), 1);

  return (
    <ul className="space-y-3">
      {rows.map((r) => (
        <li key={r.key}>
          <div className="flex items-baseline justify-between gap-3">
            <span className="truncate text-[13px]" title={r.label}>
              {r.label}
            </span>
            <span className="shrink-0 text-[12px] tabular-nums text-ink-2">
              {r.valueLabel ?? formatCount(r.value)}
            </span>
          </div>

          {/* Çubuk tabandan büyür; yalnızca veri ucu yuvarlak. */}
          <div className="mt-1 h-2 w-full overflow-hidden rounded-[2px] bg-raised">
            <div
              className="h-full rounded-r-[4px] bg-accent"
              style={{ width: `${Math.max((r.value / max) * 100, 1.5)}%` }}
            />
          </div>

          {r.note && <p className="mt-1 text-[11px] text-ink-3">{r.note}</p>}
        </li>
      ))}
    </ul>
  );
}
