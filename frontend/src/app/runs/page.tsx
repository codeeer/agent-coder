"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import { formatDuration, formatMoney } from "@/components/charts/format";
import { IconAgent } from "@/components/ui/icons";
import {
  Badge,
  Card,
  EmptyState,
  Notice,
  PageHeader,
  SearchField,
  Segmented,
  Skeleton,
  Toolbar,
  formatRelative,
} from "@/components/ui/primitives";
import type { Run } from "@/lib/types";

/**
 * Liste filtresi.
 *
 * Sayfa elli kaydı tek listede gösteriyordu ve hepsi birbirine benziyordu:
 * "dün başarısız olan hangisiydi" sorusunun cevabı gözle taramaktı. Filtre
 * yüklenmiş kayıtlar ÜZERİNDE çalışır — başlık bunu açıkça söylüyor, aksi
 * halde kullanıcı tüm geçmişte arama yaptığını sanırdı.
 */
const FILTERS = [
  { id: "all", label: "Tümü", match: () => true },
  { id: "running", label: "Çalışıyor", match: (r: Run) => isActive(r.status) },
  { id: "ok", label: "Başarılı", match: (r: Run) => r.status === "succeeded" },
  {
    id: "bad",
    label: "Sorunlu",
    match: (r: Run) => ["failed", "timeout", "interrupted"].includes(r.status),
  },
] as const;

/**
 * Sayfa boyutu — bkz. Akışlar ekranındaki aynı karar.
 *
 * Liste ekrana sığmalı; sayfalama denetimi ancak sona kaydırılınca
 * görünüyorsa sayfalama yok demektir.
 */
const PAGE_SIZES = [10, 25, 50] as const;

export default function RunsPage() {
  const [filter, setFilter] = useState<(typeof FILTERS)[number]["id"]>("all");
  const [q, setQ] = useState("");
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState<number>(PAGE_SIZES[0]);

  const { data, isPending, isError, error } = useQuery({
    queryKey: ["runs", offset, limit],
    queryFn: () => api.runs.list({ limit, offset }),
    // Çalışan iş varsa liste kendini tazelesin.
    refetchInterval: (q) =>
      q.state.data?.items.some((r) => isActive(r.status)) ? 3000 : false,
  });

  const items = useMemo(() => {
    const rows = data?.items ?? [];
    const test = FILTERS.find((f) => f.id === filter)!.match;
    const needle = q.trim().toLocaleLowerCase("tr");
    return rows.filter(
      (r) =>
        test(r) &&
        (needle === "" ||
          [r.task, r.projectName, r.agentSlug, r.modelId, r.workflowName]
            .filter(Boolean)
            .some((v) => v!.toLocaleLowerCase("tr").includes(needle))),
    );
  }, [data, filter, q]);

  return (
    /* Üç bölge: başlık + araç çubuğu üstte, liste ortada kayar, sayfalama
       altta sabit — bkz. `AppShell`'deki kabuk kararı. */
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader
        title="Çalıştırmalar"
        description="Agent çalıştırma geçmişi. Süzgeç ve arama açık sayfada çalışır. Süren işler kendiliğinden tazelenir."
        actions={
          /* Sınır yalnızca doluyken ilgi çekici; boştayken "0 / 3" gibi bir
             oran kullanıcıya hiçbir şey söylemiyordu. */
          data && data.active > 0 && (
            <Badge tone={data.active >= data.concurrencyLimit ? "warning" : "accent"}>
              {data.active >= data.concurrencyLimit
                ? `sınır dolu — ${data.active}/${data.concurrencyLimit} çalışıyor`
                : `${data.active} iş çalışıyor`}
            </Badge>
          )
        }
      />

      {data && data.items.length > 0 && (
        <Toolbar>
          <Segmented
            label="Durum süzgeci"
            options={FILTERS}
            value={filter}
            onChange={setFilter}
          />
          <SearchField
            className="min-w-45 flex-1 sm:max-w-sm"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Görev, proje, agent veya model ara…"
            aria-label="Çalıştırmalarda ara"
          />
          <span className="ml-auto hidden text-2xs text-ink-3 lg:block">
            süzgeç ve arama açık sayfada çalışır
          </span>
        </Toolbar>
      )}

      {/* Kayan bölge. `min-h-0` olmadan flex öğesi içeriğinden küçülemez ve
          sayfalama ekranın dışına düşerdi. */}
      <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">
        {isPending && <Skeleton rows={3} />}
        {isError && <Notice tone="error">{describeError(error).message}</Notice>}

        {data?.items.length === 0 && (
          <EmptyState
            icon={<IconAgent className="size-4" />}
            title="Henüz çalıştırma yok"
            description="Agent'lar sayfasından bir agent seçip projelerinizden biri üzerinde çalıştırın."
          />
        )}

        {data && data.items.length > 0 && items.length === 0 && (
          <Notice>Bu filtreye uyan çalıştırma yok.</Notice>
        )}

        {items.length > 0 && <RunTable items={items} />}
      </div>

      {data && (
        <Pagination
          total={data.total}
          limit={data.limit}
          offset={offset}
          onChange={(next) => {
            setOffset(next);
            // Sayfa değişince süzgeç DURUR ama arama temizlenir: süzgeç bir
            // niyet ("sorunluları görüyorum"), arama ise o sayfaya ait bir
            // daraltma. Aramayı taşımak, kullanıcıya boş bir sayfa gösterirdi.
            setQ("");
          }}
          pageSize={{
            value: limit,
            options: PAGE_SIZES,
            onChange: (size) => {
              setLimit(size);
              setOffset(0);
            },
          }}
          unit="çalıştırma"
        />
      )}
    </div>
  );
}

/**
 * Çalıştırma tablosu.
 *
 * ÖNCESİNDE SÜTUN BAŞLIĞI YOKTU: her satır kendi içinde etiketsiz beş
 * değer taşıyordu ve sağdaki iki rakamın hangisinin maliyet, hangisinin
 * süre olduğu ancak biçiminden tahmin ediliyordu. Aynı türden onlarca
 * kaydın karşılaştırıldığı bir ekranda satırlar değil SÜTUNLAR okunur;
 * sütunun ne olduğunu söyleyen tek şey de başlığıdır.
 *
 * Dar ekranda tablo yatay kaydırılır, satırlara BÖLÜNMEZ: bölünmüş bir
 * tablo hizayı kaybeder ve hizasını kaybeden bir tablo, listeden daha
 * kötüdür.
 */
function RunTable({ items }: { items: Run[] }) {
  return (
    <Card padded={false} className="overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full min-w-215 text-sm">
          <thead>
            <tr className="border-b border-line bg-raised/60 text-left text-2xs tracking-wide text-ink-3 uppercase">
              <th className="w-36 py-2.5 pl-4 font-medium">Durum</th>
              <th className="py-2.5 font-medium">Görev / Proje</th>
              <th className="w-64 py-2.5 font-medium">Agent / Model</th>
              <th className="w-20 py-2.5 text-right font-medium">Süre</th>
              <th className="w-24 py-2.5 text-right font-medium">Maliyet</th>
              <th className="w-28 py-2.5 pr-4 text-right font-medium">Başlatıldı</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {items.map((run) => (
              <tr
                key={run.id}
                className="group transition-colors hover:bg-raised"
              >
                <td className="py-2.5 pl-4">
                  <RunStatusBadge status={run.status} />
                </td>

                <td className="max-w-0 py-2.5 pr-4">
                  {/*
                    Bağlantı SATIRIN TAMAMI değil ilk hücrede: `<tr>` bir
                    bağlantı olamaz ve her hücreye ayrı `<a>` koymak ekran
                    okuyucuya aynı hedefi altı kez okutur.
                  */}
                  <Link
                    href={`/runs/${run.id}`}
                    className="block truncate font-medium group-hover:text-accent"
                    title={run.task}
                  >
                    {run.task}
                  </Link>
                  <div className="mt-0.5 flex items-center gap-2 text-2xs text-ink-3">
                    {/* Akış adımıysa hangi akışın parçası olduğu görünmeli;
                        aksi halde tek başına çalıştırılmış gibi okunur. */}
                    {run.workflowName && (
                      <Badge tone="accent">
                        {run.workflowName} · {run.stepName}
                      </Badge>
                    )}
                    <span className="truncate">{run.projectName}</span>
                    {run.pushedBranch && (
                      <Badge tone="info">→ {run.pushedBranch}</Badge>
                    )}
                  </div>
                </td>

                <td className="max-w-0 py-2.5 pr-4">
                  <div className="truncate font-mono text-xs">{run.agentSlug}</div>
                  <div className="mt-0.5 truncate font-mono text-2xs text-ink-3">
                    {run.modelId}
                  </div>
                </td>

                <td className="py-2.5 pr-4 text-right text-xs tabular-nums text-ink-2">
                  {durationOf(run)}
                </td>

                <td className="py-2.5 pr-4 text-right font-mono text-xs tabular-nums">
                  {run.costUsd > 0 ? formatMoney(run.costUsd) : "—"}
                </td>

                <td className="py-2.5 pr-4 text-right text-xs text-ink-3">
                  {formatRelative(run.createdAt)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

/**
 * Çalıştırma süresi.
 *
 * Süren işler için başlangıçtan ŞU ANA kadar; bitenler için gerçek süre.
 * Bitmemiş bir işe tire koymak, ekranda en çok merak edilen sayıyı
 * gizlemek olurdu.
 */
function durationOf(run: Run): string {
  if (!run.startedAt) return "—";
  const end = run.finishedAt ? new Date(run.finishedAt) : new Date();
  const seconds = (end.getTime() - new Date(run.startedAt).getTime()) / 1000;
  return seconds > 0 ? formatDuration(seconds) : "—";
}
