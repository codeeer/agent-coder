"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import type { ModelSort, ToolsFilter } from "@/lib/types";
import { ModelTable } from "@/components/models/ModelTable";
import {
  IconAlert,
  IconCheck,
  IconChip,
  IconRefresh,
  IconSearch,
} from "@/components/ui/icons";
import {
  Button,
  Checkbox,
  EmptyState,
  Input,
  Notice,
  PageHeader,
  Select,
  Skeleton,
  formatRelative,
} from "@/components/ui/primitives";

const PAGE_SIZE = 50;

const TOOLS_OPTIONS: { value: ToolsFilter; label: string }[] = [
  { value: "yes", label: "Yalnızca araç destekleyenler" },
  { value: "", label: "Hepsi" },
  { value: "unknown", label: "Araç desteği bilinmeyenler" },
];

export default function ModelsPage() {
  const queryClient = useQueryClient();

  const [rawQuery, setRawQuery] = useState("");
  const [query, setQuery] = useState("");
  const [providerId, setProviderId] = useState("");
  const [tools, setTools] = useState<ToolsFilter>("yes");
  const [freeOnly, setFreeOnly] = useState(false);
  const [sort, setSort] = useState<ModelSort>("name");
  const [order, setOrder] = useState<"asc" | "desc">("asc");
  const [page, setPage] = useState(0);

  // Her tuş vuruşunda istek atmamak için arama geciktirilir.
  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(rawQuery);
      setPage(0);
    }, 250);
    return () => clearTimeout(timer);
  }, [rawQuery]);

  const { data, isPending, isError, error } = useQuery({
    queryKey: ["models", { query, providerId, tools, freeOnly, sort, order, page }],
    queryFn: () =>
      api.models.list({
        provider: providerId || undefined,
        q: query || undefined,
        tools,
        free: freeOnly,
        sort,
        order,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
  });

  const refresh = useMutation({
    mutationFn: api.models.refresh,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["models"] }),
  });

  function toggleSort(next: ModelSort) {
    if (next === sort) {
      setOrder((o) => (o === "asc" ? "desc" : "asc"));
    } else {
      setSort(next);
      setOrder(next === "name" || next === "provider" ? "asc" : "desc");
    }
    setPage(0);
  }
  const providers = data?.providers ?? [];
  const staleProviders = providers.filter((p) => p.lastError);

  return (
    <div>
      <PageHeader
        title="Modeller"
        description={
          <>
            Tanımlı tüm sağlayıcıların modelleri. Agent olarak yalnızca{" "}
            <span className="font-medium text-ink">araç</span> rozetli modeller
            güvenle kullanılabilir.
          </>
        }
        actions={
          <Button
            onClick={() => refresh.mutate()}
            disabled={refresh.isPending}
            icon={<IconRefresh className="size-4" />}
          >
            {refresh.isPending ? "Yenileniyor…" : "Tümünü yenile"}
          </Button>
        }
      />

      {/* Hiç sağlayıcı yoksa liste zaten boş olacak; önce bunu söylüyoruz. */}
      {data && !data.configured && (
        <div className="mt-4">
          <Notice tone="warning">
            Henüz LLM sağlayıcı tanımlı değil.{" "}
            <Link href="/settings" className="underline">
              Ayarlardan bir sağlayıcı ekleyin
            </Link>
            .
          </Notice>
        </div>
      )}

      {/* Bir sağlayıcının hatası diğerlerini etkilemez; hangisi olduğu yazılır. */}
      {staleProviders.length > 0 && (
        <div className="mt-4">
          <Notice tone="warning">
            <p className="font-medium">Bazı sağlayıcıların kataloğu güncellenemedi</p>
            <ul className="mt-1 space-y-0.5">
              {staleProviders.map((p) => (
                <li key={p.providerId}>
                  {p.providerName}: {p.lastError} — en son{" "}
                  {formatRelative(p.lastSuccessAt)} tarihli liste gösteriliyor
                </li>
              ))}
            </ul>
          </Notice>
        </div>
      )}

      {refresh.data && (
        <div className="mt-4">
          <Notice>
            <ul className="space-y-0.5">
              {/* `✓` / `✗` metin karakterleriydi: yazı tipine göre boyu ve
                  hizası değişiyor, arayüzün geri kalanındaki ikonlarla ne
                  ölçüsü ne kalınlığı tutuyordu. Artık aynı ikon kümesinden. */}
              {refresh.data.results.map((r) => (
                <li key={r.providerId} className="flex items-start gap-1.5">
                  {r.ok ? (
                    <IconCheck className="mt-0.5 size-4 shrink-0 text-ok" />
                  ) : (
                    <IconAlert className="mt-0.5 size-4 shrink-0 text-danger" />
                  )}
                  <span>
                    {r.providerName}: {r.ok ? `${r.count} model` : r.error}
                  </span>
                </li>
              ))}
            </ul>
          </Notice>
        </div>
      )}

      {refresh.isError && (
        <div className="mt-4">
          <Notice tone="error">{describeError(refresh.error).message}</Notice>
        </div>
      )}

      {/* Filtreler */}
      <div className="mt-5 flex flex-wrap items-center gap-2.5 rounded-card border border-line bg-surface p-3">
        <div className="relative w-full sm:w-64">
          <IconSearch className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-ink-3" />
          <Input
            className="pl-8"
            aria-label="Model veya sağlayıcı ara"
            placeholder="Model veya sağlayıcı ara…"
            value={rawQuery}
            onChange={(e) => setRawQuery(e.target.value)}
          />
        </div>

        {/*
          Genişlik SARMALAYICIDAN geliyor, `Select`'in kendi sınıfından değil.
          Öncesinde `className="w-auto"` yazılıydı ama işe yaramıyordu:
          `fieldBase` zaten `w-full` taşıyor ve iki utility de aynı özelliği
          (`width`) veriyor. Hangisinin kazandığına sınıf dizgisindeki sıra
          değil, üretilen CSS'teki sıra karar veriyor — `w-full` kazanıyordu.
          Sonuç: 256px'lik arama kutusunun altında tam genişlikte iki açılır
          liste, süzgeç çubuğu üç satıra yayılmış ve bozuk duruyordu.

          Arama kutusu zaten bu deseni kullanıyordu (`<div className="w-64">`);
          üçü artık aynı yolla ölçülüyor ve tek satırda hizalanıyor.
        */}
        <div className="w-full sm:w-52">
          <Select
            aria-label="Sağlayıcıya göre süz"
            value={providerId}
            onChange={(e) => {
              setProviderId(e.target.value);
              setPage(0);
            }}
          >
            <option value="">Tüm sağlayıcılar</option>
            {providers.map((p) => (
              <option key={p.providerId} value={p.providerId}>
                {p.providerName} ({p.modelCount})
              </option>
            ))}
          </Select>
        </div>

        <div className="w-full sm:w-64">
          <Select
            aria-label="Araç desteğine göre süz"
            value={tools}
            onChange={(e) => {
              setTools(e.target.value as ToolsFilter);
              setPage(0);
            }}
          >
            {TOOLS_OPTIONS.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </Select>
        </div>

        <Checkbox
          label="Yalnızca ücretsizler"
          checked={freeOnly}
          onChange={(v) => {
            setFreeOnly(v);
            setPage(0);
          }}
        />

      </div>

      {/* Üç durum: yükleniyor, hata, boş */}
      <div className="mt-4">
        {isPending && <Skeleton rows={4} />}

        {isError && (
          <Notice tone="error">{describeError(error).message}</Notice>
        )}

        {data && data.items.length === 0 && (
          <EmptyState
            icon={<IconChip className="size-4" />}
            title={query ? `"${query}" için sonuç yok` : "Model bulunamadı"}
            description={
              query
                ? "Aramayı daraltmayı veya filtreleri gevşetmeyi deneyin."
                : data.configured
                  ? "Bu filtrelerle eşleşen model yok."
                  : "Sağlayıcı eklendikten sonra katalog otomatik indirilecek."
            }
          />
        )}

        {data && data.items.length > 0 && (
          <>
            <ModelTable
              models={data.items}
              sort={sort}
              order={order}
              onSortChange={toggleSort}
            />

            <Pagination
              total={data.total}
              limit={PAGE_SIZE}
              offset={page * PAGE_SIZE}
              onChange={(next) => setPage(Math.floor(next / PAGE_SIZE))}
              unit="model"
            />
          </>
        )}
      </div>
    </div>
  );
}

