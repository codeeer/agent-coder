"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";
import type { Model } from "@/lib/types";
import { IconSearch } from "@/components/ui/icons";
import { Badge } from "@/components/ui/primitives";

/**
 * Aranabilir model seçici.
 *
 * Katalogda yüzlerce model olabildiği için düz açılır liste kullanılamaz:
 * kullanıcı adın birkaç harfini yazıp seçebilmeli.
 *
 * Seçim (sağlayıcı, model) ÇİFTİDİR, tek başına model kimliği değil: aynı
 * model adı iki sağlayıcıda bulunabilir ve hangisinin kastedildiği belirsiz
 * kalırsa yanlış sağlayıcıya istek gider.
 */
export function ModelPicker({
  models,
  providerId,
  modelId,
  onChange,
  emptyLabel = "Seçin…",
  disabled,
}: {
  models: Model[];
  providerId: string | null;
  modelId: string;
  onChange: (selection: { providerId: string; modelId: string } | null) => void;
  /** Hiçbir şey seçili değilken gösterilen metin. */
  emptyLabel?: string;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [highlight, setHighlight] = useState(0);

  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const listId = useId();

  const selected = useMemo(
    () =>
      models.find(
        (m) => m.id === modelId && (providerId ? m.providerId === providerId : true),
      ),
    [models, modelId, providerId],
  );

  const matches = useMemo(() => {
    const q = query.trim().toLowerCase();

    /*
     * Arama boşken SEÇİLİ model listenin başında durur.
     *
     * Önceden liste alfabetik ilk 60 modeli gösteriyordu ve seçili model —
     * örneğin `anthropic/claude-haiku-4.5` — çoğu zaman o 60'ın dışında
     * kalıyordu: kullanıcı listeyi açtığında kendi seçimini göremiyor, hangi
     * modelde olduğunu doğrulamak için adını yazmak zorunda kalıyordu.
     */
    if (!q) {
      const rest = models.filter((m) => m !== selected).slice(0, 59);
      return selected ? [selected, ...rest] : models.slice(0, 60);
    }

    // Kimlik, ad ve sağlayıcı adında arar. Kimlikte geçenler önce gelsin:
    // kullanıcı genelde model kimliğini yazar.
    const scored = models
      .map((m) => {
        const id = m.id.toLowerCase();
        const name = m.name.toLowerCase();
        const provider = m.providerName.toLowerCase();

        if (id.startsWith(q)) return { m, score: 0 };
        if (id.includes(q)) return { m, score: 1 };
        if (name.toLowerCase().includes(q)) return { m, score: 2 };
        if (provider.includes(q)) return { m, score: 3 };
        return null;
      })
      .filter((x): x is { m: Model; score: number } => x !== null);

    scored.sort((a, b) => a.score - b.score || a.m.id.localeCompare(b.m.id));
    return scored.slice(0, 60).map((x) => x.m);
  }, [models, query, selected]);

  // Liste değişince vurgu başa döner; aksi halde var olmayan satır seçili kalır.
  useEffect(() => setHighlight(0), [query]);

  /*
   * Liste açılınca SEÇİLİ modele vurgu koyulur.
   *
   * Önceden liste her açılışta alfabetik başa düşüyordu: kullanıcı 400 model
   * arasından birini seçmiş olsa bile listeyi açtığında `ai21/...` görüyor ve
   * hangisinin seçili olduğunu anlamıyordu. Aşağıdaki "vurguyu görünür tut"
   * etkisi bu vurguyu ekrana kaydırıyor, yani seçili model kendiliğinden
   * görünüyor.
   *
   * Yalnızca açılış anında: kullanıcı yazmaya başlayınca vurgu arama sonucuna
   * ait olmalı.
   */
  useEffect(() => {
    if (!open) return;
    const i = matches.findIndex((m) => m.id === modelId && m.providerId === providerId);
    setHighlight(i >= 0 ? i : 0);
    // `matches` bilerek dışarıda: liste yazdıkça değişiyor ve o durumda vurgu
    // yukarıdaki etkiye ait.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  // Vurgulanan satırı görünür tut.
  useEffect(() => {
    if (!open) return;
    listRef.current
      ?.querySelector(`[data-index="${highlight}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }, [highlight, open]);

  // Dışarı tıklama kapatır.
  useEffect(() => {
    if (!open) return;

    const onPointerDown = (e: PointerEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) {
        setOpen(false);
        setQuery("");
      }
    };
    document.addEventListener("pointerdown", onPointerDown);
    return () => document.removeEventListener("pointerdown", onPointerDown);
  }, [open]);

  function choose(m: Model) {
    onChange({ providerId: m.providerId, modelId: m.id });
    setOpen(false);
    setQuery("");
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!open) {
        setOpen(true);
        return;
      }
      setHighlight((h) => {
        const next = e.key === "ArrowDown" ? h + 1 : h - 1;
        if (next < 0) return matches.length - 1;
        if (next >= matches.length) return 0;
        return next;
      });
      return;
    }

    if (e.key === "Enter" && open) {
      e.preventDefault();
      const m = matches[highlight];
      if (m) choose(m);
      return;
    }

    if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
      setQuery("");
      inputRef.current?.blur();
    }
  }

  return (
    <div ref={rootRef} className="relative">
      <div className="relative">
        <IconSearch className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-ink-3" />
        <input
          ref={inputRef}
          role="combobox"
          aria-expanded={open}
          aria-controls={listId}
          aria-autocomplete="list"
          autoComplete="off"
          disabled={disabled}
          /* Kenar ve hover, ortak girdi stiliyle (`fieldBase`) AYNI token'ları
            kullanır. Elle kopyalandığı için `control-line` düzeltmesinden payını
            almamıştı: aynı görünmesi gereken iki kutu iki farklı kenar rengi
            taşıyordu. */
          className="w-full rounded-lg border border-control-line bg-canvas py-1.5 pr-16 pl-8 font-mono text-[12px] outline-none transition-colors duration-150 placeholder:font-sans placeholder:text-ink-3 hover:border-ink-2 focus:border-accent disabled:opacity-50"
          value={open ? query : (selected?.id ?? "")}
          placeholder={selected ? selected.id : emptyLabel}
          onFocus={() => setOpen(true)}
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
          }}
          onKeyDown={onKeyDown}
        />

        {selected && !open && (
          <button
            type="button"
            onClick={() => onChange(null)}
            className="absolute top-1/2 right-2 -translate-y-1/2 rounded px-1.5 py-0.5 text-[11px] text-ink-3 transition-colors hover:bg-raised hover:text-ink"
          >
            temizle
          </button>
        )}
      </div>

      {/* Seçili modelin sağlayıcısı ve uyarısı kapalıyken de görünür. */}
      {selected && !open && (
        <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
          <span className="text-[11px] text-ink-3">{selected.providerName}</span>
          <ToolsHint model={selected} />
        </div>
      )}

      {open && (
        <ul
          ref={listRef}
          id={listId}
          role="listbox"
          className="absolute z-20 mt-1 max-h-72 w-full overflow-auto rounded-lg border border-control-line bg-overlay py-1 shadow-(--shadow-pop)"
        >
          {matches.length === 0 && (
            <li className="px-3 py-2.5 text-[13px] text-ink-3">
              &quot;{query}&quot; için model bulunamadı
            </li>
          )}

          {matches.map((m, i) => {
            const active = i === highlight;
            const isSelected =
              m.id === modelId && m.providerId === providerId;

            return (
              <li
                key={`${m.providerId}:${m.id}`}
                role="option"
                aria-selected={isSelected}
                data-index={i}
                onPointerEnter={() => setHighlight(i)}
                onPointerDown={(e) => {
                  // Girdi odağı kaybedip listenin kapanmasını önler.
                  e.preventDefault();
                  choose(m);
                }}
                className={`cursor-pointer px-3 py-1.5 ${active ? "bg-raised" : ""}`}
              >
                <div className="flex items-center justify-between gap-3">
                  <span className="truncate font-mono text-[12px]">
                    <Highlight text={m.id} query={query} />
                  </span>
                  <span className="shrink-0 text-[11px] text-ink-3">
                    {m.providerName}
                  </span>
                </div>
                <div className="mt-0.5 flex items-center gap-1.5">
                  <ToolsHint model={m} />
                  {m.isFree && <Badge tone="success">ücretsiz</Badge>}
                  {m.contextLength && (
                    <span className="text-[11px] text-ink-3">
                      {formatContext(m.contextLength)} bağlam
                    </span>
                  )}
                  {m.promptPricePerMTok > 0 && (
                    <span className="text-[11px] text-ink-3">
                      ${m.promptPricePerMTok.toFixed(2)}/M
                    </span>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}

/** Eşleşen harfleri kalınlaştırır — kullanıcı neden bu sonucun geldiğini görür. */
function Highlight({ text, query }: { text: string; query: string }) {
  const q = query.trim().toLowerCase();
  if (!q) return <>{text}</>;

  const at = text.toLowerCase().indexOf(q);
  if (at < 0) return <>{text}</>;

  return (
    <>
      {text.slice(0, at)}
      <span className="font-semibold text-accent">
        {text.slice(at, at + q.length)}
      </span>
      {text.slice(at + q.length)}
    </>
  );
}

/**
 * Araç desteği ipucu.
 *
 * Yalnızca sorunlu durumlar gösterilir: destekleyen modeller çoğunluk olduğu
 * için her satıra rozet koymak gürültü yapar.
 */
function ToolsHint({ model }: { model: Model }) {
  if (model.supportsTools === true) return null;

  if (model.supportsTools === false) {
    return (
      <Badge tone="danger" title="Araç çağıramaz, agent olarak kullanılamaz">
        araçsız
      </Badge>
    );
  }
  return (
    <Badge tone="warning" title="Sağlayıcı bu bilgiyi vermiyor">
      araç bilinmiyor
    </Badge>
  );
}

function formatContext(tokens: number): string {
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M`;
  if (tokens >= 1_000) return `${Math.round(tokens / 1_000)}K`;
  return String(tokens);
}
