"use client";

import { Button } from "@/components/ui/primitives";

/**
 * Liste sayfalama denetimi.
 *
 * Her liste ekranında AYNI bileşen kullanılır. Ayrı ayrı yazıldığında biri
 * "Sayfa 2 / 5" derken diğeri "51–100" der, biri son sayfada düğmeyi kapatır
 * diğeri kapatmaz — kullanıcı her listede yeniden öğrenmek zorunda kalır.
 *
 * Aralık HER ZAMAN yazılır, düğmeler yalnızca birden fazla sayfa varken çıkar:
 * "12 kayıt" bilgidir ve listenin tamamını gördüğünü söyler; tek sayfalık bir
 * listede kapalı iki ok ise yalnızca gürültüdür.
 */
export function Pagination({
  total,
  limit,
  offset,
  onChange,
  /** Kayıt türünün adı — "3 akış", "12 çalıştırma". */
  unit = "kayıt",
}: {
  total: number;
  limit: number;
  offset: number;
  onChange: (offset: number) => void;
  unit?: string;
}) {
  if (total === 0) return null;

  const first = offset + 1;
  const last = Math.min(offset + limit, total);
  const page = Math.floor(offset / limit) + 1;
  const pageCount = Math.max(1, Math.ceil(total / limit));
  const many = pageCount > 1;

  return (
    <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
      <span className="text-xs text-ink-2">
        {many ? (
          <>
            <strong className="font-medium text-ink">
              {first}–{last}
            </strong>{" "}
            / {total} {unit}
          </>
        ) : (
          <>
            {total} {unit}
          </>
        )}
      </span>

      {many && (
        <div className="flex items-center gap-2">
          <Button
            size="sm"
            onClick={() => onChange(Math.max(0, offset - limit))}
            disabled={offset === 0}
            aria-label="Önceki sayfa"
          >
            ← Önceki
          </Button>
          <span className="text-xs text-ink-2 tabular-nums">
            {page} / {pageCount}
          </span>
          <Button
            size="sm"
            onClick={() => onChange(offset + limit)}
            disabled={last >= total}
            aria-label="Sonraki sayfa"
          >
            Sonraki →
          </Button>
        </div>
      )}
    </div>
  );
}
