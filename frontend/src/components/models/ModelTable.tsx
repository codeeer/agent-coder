"use client";

import type { Model, ModelSort } from "@/lib/types";
import {
  IconChevronDown,
  IconChevronUp,
  IconPlug,
} from "@/components/ui/icons";
import { Badge, IconTile, toneFromKey } from "@/components/ui/primitives";

/** Fiyatı okunabilir biçime çevirir. Milyon token başına USD. */
function formatPrice(perMTok: number): string {
  if (perMTok === 0) return "—";
  if (perMTok < 0.01) return `$${perMTok.toFixed(4)}`;
  return `$${perMTok.toFixed(2)}`;
}

/** Bağlam uzunluğu. null "bilinmiyor" demektir — sıfır değil. */
function formatContext(tokens: number | null): string {
  if (tokens === null) return "—";
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M`;
  if (tokens >= 1_000) return `${Math.round(tokens / 1_000)}K`;
  return String(tokens);
}

const COLUMNS: { key: ModelSort | null; label: string; align?: string }[] = [
  { key: "provider", label: "Sağlayıcı" },
  { key: "name", label: "Model" },
  { key: null, label: "Özellik" },
  { key: "context", label: "Bağlam", align: "text-right" },
  { key: "price", label: "Girdi /M", align: "text-right" },
  { key: null, label: "Çıktı /M", align: "text-right" },
];

export function ModelTable({
  models,
  sort,
  order,
  onSortChange,
}: {
  models: Model[];
  sort: ModelSort;
  order: "asc" | "desc";
  onSortChange: (sort: ModelSort) => void;
}) {
  return (
    // Dar ekranda tablo yatay kayar; sayfanın kendisi kaymaz.
    <div className="overflow-x-auto rounded-card border border-line bg-surface shadow-(--shadow-card)">
      <table className="w-full min-w-215 text-sm">
        {/* Başlık şeridi Çalıştırmalar tablosuyla AYNI: iki tablo aynı
            üründe iki farklı başlık dili konuşmamalı. */}
        <thead>
          <tr className="border-b border-line bg-raised/60 text-left text-2xs tracking-wide text-ink-3 uppercase">
            {COLUMNS.map((col) => (
              <th
                key={col.label}
                className={`px-4 py-2.5 font-medium ${col.align ?? ""}`}
              >
                {col.key ? (
                  /* Sıralama yönü METİN OKUYLA (" ↑") gösteriliyordu; ok
                     karakterinin boyu ve hizası yazı tipine göre değişiyor ve
                     arayüzün ikon kümesiyle ne ölçüsü ne kalınlığı tutuyordu.
                     Bu bir arayüz işareti, tipografi değil — yeri ikon kümesi. */
                  <button
                    onClick={() => onSortChange(col.key as ModelSort)}
                    className={`inline-flex items-center gap-1 uppercase transition-colors hover:text-ink ${
                      col.align === "text-right" ? "flex-row-reverse" : ""
                    }`}
                  >
                    {col.label}
                    {sort === col.key &&
                      (order === "asc" ? (
                        <IconChevronUp className="size-3.5" />
                      ) : (
                        <IconChevronDown className="size-3.5" />
                      ))}
                  </button>
                ) : (
                  col.label
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-line">
          {models.map((m) => (
            <tr
              key={`${m.providerId}:${m.id}`}
              className="align-top transition-colors hover:bg-raised"
            >
              <td className="px-4 py-2.5">
                {/*
                  Sağlayıcı adının yanında renkli bir karo: onlarca satırın
                  arasında hangi satırın hangi sağlayıcıya ait olduğu, adı
                  okumadan görülüyor. Renk sağlayıcının kimliğinden türetilir,
                  yani aynı sağlayıcı her sayfada aynı renkte.
                */}
                <div className="flex items-center gap-2">
                  <IconTile tone={toneFromKey(m.providerId)} size="sm">
                    <IconPlug className="size-3.5" />
                  </IconTile>
                  <span className="truncate text-xs text-ink-2">
                    {m.providerName}
                  </span>
                </div>
              </td>
              <td className="px-4 py-2.5">
                <div className="font-medium">{m.name}</div>
                <div className="mt-0.5 font-mono text-xs text-ink-3">
                  {m.id}
                </div>
              </td>
              <td className="px-4 py-2.5">
                <div className="flex flex-wrap gap-1">
                  <ToolsBadge supportsTools={m.supportsTools} />
                  {m.isFree && <Badge tone="success">ücretsiz</Badge>}
                  {m.isPreview && (
                    <Badge
                      tone="warning"
                      title="Model kimliğinden tahmin edildi; kararlılığı garanti değil"
                    >
                      önizleme
                    </Badge>
                  )}
                </div>
              </td>
              <td className="px-4 py-2.5 text-right font-mono text-xs">
                {formatContext(m.contextLength)}
              </td>
              <td className="px-4 py-2.5 text-right font-mono text-xs">
                {formatPrice(m.promptPricePerMTok)}
              </td>
              <td className="px-4 py-2.5 text-right font-mono text-xs">
                {formatPrice(m.completionPricePerMTok)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/**
 * Araç desteği rozeti — üç durumlu.
 *
 * "Bilinmiyor" ile "desteklemiyor" ayrı gösterilir: agent olarak
 * kullanılabilirlik buna bağlı, yanlış varsayım pahalıya patlar.
 */
function ToolsBadge({ supportsTools }: { supportsTools: boolean | null }) {
  if (supportsTools === true) {
    return (
      <Badge tone="info" title="Agent olarak kullanılabilir">
        araç
      </Badge>
    );
  }
  if (supportsTools === false) {
    return (
      <Badge title="Araç çağıramaz, agent olarak kullanılamaz">araçsız</Badge>
    );
  }
  return (
    <Badge
      tone="warning"
      title="Sağlayıcı bu bilgiyi vermiyor. Agent olarak denenebilir ama çalışacağı garanti değil."
    >
      araç bilinmiyor
    </Badge>
  );
}
