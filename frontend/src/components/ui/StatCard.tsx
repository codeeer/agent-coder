"use client";

import { Sparkline } from "@/components/charts/Sparkline";
import { changeRatio, formatPercent } from "@/components/charts/format";

/**
 * Rakam kartı — panonun ve raporun ortak KPI parçası.
 *
 * Rapor ekranında yazılmıştı ve orada kalmıştı; panoya da aynı şerit
 * gerekince ikinci bir kopya çıkacaktı. İki kopya kaçınılmaz olarak
 * ayrışır: birinde yön oku, diğerinde yüzde işareti farklı durur ve aynı
 * rakam iki ekranda iki türlü okunur.
 *
 * Kart üç şey söyler ve üçü de zorunludur: NE olduğu, KAÇ olduğu, hangi
 * YÖNE gittiği. Yön olmadan bir rakam bilgi değil, süstür.
 */
export interface StatCardProps {
  label: string;
  value: string;
  /** Değişim oranı bu ikisinden hesaplanır. */
  current: number;
  previous: number;
  /** Günlük seri — yalnızca gerçekten varsa verilir. */
  spark?: number[];
  /** true: artış iyi · false: artış kötü · null: yön yorumlanmaz. */
  upIsGood: boolean | null;
  /**
   * Karşılaştırmanın neye göre olduğu.
   *
   * KISA olmak zorunda. Sekiz kart tek sıraya dizildiğinde karta ~125px
   * kalıyor ve bu satır 11px'te ancak yirmi küsur karakter alıyor; "son 7
   * güne göre" yazıldığında ekranda "son 7 güne g…" görünüyordu — kırpılmış
   * bir karşılaştırma, karşılaştırma değildir.
   */
  periodNote: string;
}

export function StatCard({
  label,
  value,
  current,
  previous,
  spark,
  upIsGood,
  periodNote,
}: StatCardProps) {
  const ratio = changeRatio(current, previous);
  const flat = ratio !== null && Math.abs(ratio) < 0.005;
  const good = ratio !== null && ratio > 0 === upIsGood;

  return (
    <div className="rounded-card border border-line bg-surface px-4 py-3.5 shadow-(--shadow-card)">
      <div className="truncate text-2xs font-medium tracking-wide text-ink-3 uppercase">
        {label}
      </div>

      <div className="mt-2.5 flex items-end justify-between gap-3">
        <div className="text-xl leading-none font-semibold tabular-nums">
          {value}
        </div>
        {/* Kıvılcım YALNIZCA günlük serisi gerçekten olan kartlarda. Serisi
            olmayan bir karta düz bir çizgi koymak, olmayan bir veriyi varmış
            gibi göstermek olurdu. */}
        {spark && spark.length > 1 && (
          <div className="hidden w-16 shrink-0 sm:block">
            <Sparkline values={spark} label={`${label} — günlük seyir`} height={28} />
          </div>
        )}
      </div>

      <div className="mt-2.5 truncate text-2xs">
        {ratio === null ? (
          <span className="text-ink-3">önceki dönem yok</span>
        ) : (
          <>
            <span
              className={
                flat || upIsGood === null
                  ? "text-ink-3"
                  : good
                    ? "font-medium text-ok"
                    : "font-medium text-danger"
              }
            >
              {flat
                ? "≈ aynı"
                : `${ratio > 0 ? "↑" : "↓"} ${formatPercent(Math.abs(ratio), 1)}`}
            </span>{" "}
            <span className="text-ink-3">{periodNote}</span>
          </>
        )}
      </div>
    </div>
  );
}
