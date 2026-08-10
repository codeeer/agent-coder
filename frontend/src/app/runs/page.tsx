"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import { IconAgent } from "@/components/ui/icons";
import {
  Badge,
  EmptyState,
  Input,
  Mono,
  Notice,
  PageHeader,
  Skeleton,
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

const PAGE_SIZE = 25;

export default function RunsPage() {
  const [filter, setFilter] = useState<(typeof FILTERS)[number]["id"]>("all");
  const [q, setQ] = useState("");
  const [offset, setOffset] = useState(0);

  const { data, isPending, isError, error } = useQuery({
    queryKey: ["runs", offset],
    queryFn: () => api.runs.list({ limit: PAGE_SIZE, offset }),
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
    <div>
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
        <div className="mb-4 flex flex-wrap items-center gap-2">
          <div className="flex rounded-lg border border-line bg-surface p-0.5">
            {FILTERS.map((f) => (
              <button
                key={f.id}
                type="button"
                onClick={() => setFilter(f.id)}
                aria-pressed={filter === f.id}
                className={`rounded-md px-2.5 py-1 text-[12px] transition-colors ${
                  filter === f.id
                    ? "bg-accent-soft font-medium text-accent"
                    : "text-ink-2 hover:text-ink"
                }`}
              >
                {f.label}
              </button>
            ))}
          </div>
          <div className="min-w-[180px] flex-1 sm:max-w-xs">
            <Input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Görev, proje, agent veya model ara"
              aria-label="Çalıştırmalarda ara"
            />
          </div>
        </div>
      )}

      {isPending && <Skeleton rows={3} />}
      {isError && <Notice tone="error">{describeError(error).message}</Notice>}

      {data?.items.length === 0 && (
        <EmptyState
          icon={<IconAgent className="size-5" />}
          title="Henüz çalıştırma yok"
          description="Agent'lar sayfasından bir agent seçip projelerinizden biri üzerinde çalıştırın."
        />
      )}

      {data && data.items.length > 0 && items.length === 0 && (
        <Notice>Bu filtreye uyan çalıştırma yok.</Notice>
      )}

      {items.length > 0 && (
        <div className="overflow-hidden rounded-card border border-line">
          {items.map((run, i) => (
            <Link
              key={run.id}
              href={`/runs/${run.id}`}
              className={`flex items-center gap-4 bg-surface px-4 py-3 transition-colors hover:bg-raised ${
                i > 0 ? "border-t border-line" : ""
              }`}
            >
              <div className="w-32 shrink-0">
                <RunStatusBadge status={run.status} />
              </div>

              <div className="min-w-0 flex-1">
                <div className="truncate text-[13px]">{run.task}</div>
                <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[12px] text-ink-3">
                  {/* Akış adımıysa hangi akışın parçası olduğu görünmeli;
                      aksi halde tek başına çalıştırılmış gibi okunur. */}
                  {run.workflowName && (
                    <Badge tone="accent">
                      {run.workflowName} · {run.stepName}
                    </Badge>
                  )}
                  <span>{run.projectName}</span>
                  <Mono>{run.agentSlug}</Mono>
                  <span className="font-mono">{run.modelId}</span>
                  {run.pushedBranch && (
                    <Badge tone="info">→ {run.pushedBranch}</Badge>
                  )}
                </div>
              </div>

              <div className="shrink-0 text-right">
                <div className="font-mono text-[12px] tabular-nums">
                  {run.costUsd > 0 ? `$${run.costUsd.toFixed(4)}` : "—"}
                </div>
                <div className="mt-1 text-[12px] text-ink-3">
                  {formatRelative(run.createdAt)}
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}

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
          unit="çalıştırma"
        />
      )}
    </div>
  );
}
