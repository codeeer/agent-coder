"use client";

import { IconChevronLeft, IconChevronRight } from "@/components/ui/icons";

/**
 * Liste sayfalama denetimi.
 *
 * Her liste ekranında AYNI bileşen kullanılır. Ayrı ayrı yazıldığında biri
 * "Sayfa 2 / 5" derken diğeri "51–100" der, biri son sayfada düğmeyi kapatır
 * diğeri kapatmaz — kullanıcı her listede yeniden öğrenmek zorunda kalır.
 *
 * DÜZEN: solda toplam, sağda sayfa numaraları. Öncesinde ikisi de "← Önceki
 * / 2 / 5 / Sonraki →" biçiminde metin düğmelerdi ve sayfalar arasında
 * gezinmenin tek yolu tek tek ilerlemekti; beşinci sayfaya gitmek dört
 * tıklama ediyordu. Numaralı düğmeler hedefe DOĞRUDAN götürüyor.
 *
 * SAYFA BOYUTU da burada. Listenin ekrana sığması, kaç kaydın gösterildiğine
 * bağlı ve bu bir tercih: kimi kullanıcı kaydırmak yerine sayfalamak ister,
 * kimi tersini. Varsayılan, tipik bir masaüstü ekranında sayfanın DİKEY
 * KAYDIRMA GEREKTİRMEYECEĞİ değerde tutuluyor.
 */
export function Pagination({
  total,
  limit,
  offset,
  onChange,
  /** Kayıt türünün adı — "3 akış", "12 çalıştırma". */
  unit = "kayıt",
  /** Sayfa boyutu seçilebiliyorsa verilir; verilmezse seçici çıkmaz. */
  pageSize,
}: {
  total: number;
  limit: number;
  offset: number;
  onChange: (offset: number) => void;
  unit?: string;
  pageSize?: {
    value: number;
    options: readonly number[];
    onChange: (size: number) => void;
  };
}) {
  if (total === 0) return null;

  const page = Math.floor(offset / limit) + 1;
  const pageCount = Math.max(1, Math.ceil(total / limit));
  const many = pageCount > 1;

  return (
    <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
      <span className="text-xs text-ink-3">
        Toplam{" "}
        <strong className="font-medium text-ink-2 tabular-nums">{total}</strong>{" "}
        {unit}
      </span>

      <div className="flex items-center gap-2">
        {pageSize && (
          <label className="flex items-center gap-1.5 text-xs text-ink-3">
            <select
              value={pageSize.value}
              onChange={(e) => pageSize.onChange(Number(e.target.value))}
              aria-label="Sayfa başına kayıt"
              className="h-8 rounded-lg border border-control-line bg-surface pr-7 pl-2.5 text-xs text-ink-2 transition-colors hover:border-ink-2"
            >
              {pageSize.options.map((n) => (
                <option key={n} value={n}>
                  {n} / sayfa
                </option>
              ))}
            </select>
          </label>
        )}

        {many && (
          <nav className="flex items-center gap-1" aria-label="Sayfalar">
            <Arrow
              dir="prev"
              disabled={page === 1}
              onClick={() => onChange(Math.max(0, offset - limit))}
            />

            {pageNumbers(page, pageCount).map((p, i) =>
              p === null ? (
                <span
                  key={`gap-${i}`}
                  className="px-1 text-xs text-ink-3"
                  aria-hidden="true"
                >
                  …
                </span>
              ) : (
                <button
                  key={p}
                  type="button"
                  onClick={() => onChange((p - 1) * limit)}
                  aria-current={p === page ? "page" : undefined}
                  aria-label={`Sayfa ${p}`}
                  className={`h-8 min-w-8 rounded-lg px-2 text-xs tabular-nums transition-colors ${
                    p === page
                      ? "bg-accent font-medium text-accent-ink"
                      : "border border-line text-ink-2 hover:bg-raised hover:text-ink"
                  }`}
                >
                  {p}
                </button>
              ),
            )}

            <Arrow
              dir="next"
              disabled={page >= pageCount}
              onClick={() => onChange(offset + limit)}
            />
          </nav>
        )}
      </div>
    </div>
  );
}

function Arrow({
  dir,
  disabled,
  onClick,
}: {
  dir: "prev" | "next";
  disabled: boolean;
  onClick: () => void;
}) {
  const Icon = dir === "prev" ? IconChevronLeft : IconChevronRight;
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={dir === "prev" ? "Önceki sayfa" : "Sonraki sayfa"}
      className="flex size-8 items-center justify-center rounded-lg border border-line text-ink-2 transition-colors hover:bg-raised hover:text-ink disabled:pointer-events-none disabled:opacity-40"
    >
      <Icon className="size-4" />
    </button>
  );
}

/**
 * Gösterilecek sayfa numaraları; `null` bir boşluk (…) demektir.
 *
 * Hepsini yazmak 13 sayfada 13 düğme eder ve şerit satırı taşar. Kural:
 * ilk sayfa, son sayfa ve içinde bulunulanın komşuları her zaman görünür —
 * kullanıcının gitmek isteyeceği yerler bunlar.
 */
export function pageNumbers(page: number, pageCount: number): (number | null)[] {
  if (pageCount <= 7) {
    return Array.from({ length: pageCount }, (_, i) => i + 1);
  }

  const out = new Set<number>([1, pageCount, page]);
  if (page - 1 > 1) out.add(page - 1);
  if (page + 1 < pageCount) out.add(page + 1);

  const sorted = [...out].sort((a, b) => a - b);
  const withGaps: (number | null)[] = [];
  sorted.forEach((n, i) => {
    const prev = sorted[i - 1];
    if (prev !== undefined && n - prev > 1) withGaps.push(null);
    withGaps.push(n);
  });
  return withGaps;
}
