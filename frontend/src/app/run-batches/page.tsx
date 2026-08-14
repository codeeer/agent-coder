"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";

import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { RunBatch } from "@/lib/types";
import { Pagination } from "@/components/ui/Pagination";
import { IconLayers } from "@/components/ui/icons";
import {
  EmptyState,
  List,
  Notice,
  PageHeader,
  Skeleton,
} from "@/components/ui/primitives";
import { RunBatchBadge, isBatchActive } from "@/components/workflows/RunBatchBadges";
import { CountStrip } from "@/components/workflows/CountStrip";

/**
 * Toplu çalıştırmalar (spec 023).
 *
 * Liste ÖĞELERİ GÖSTERMEZ, sayıları gösterir: otuz öğenin tamamını her satırda
 * açmak listeyi tek bir toplu işe indirirdi. Detay bir tık ötede.
 */
export default function RunBatchesPage() {
  const [offset, setOffset] = useState(0);

  const batches = useQuery({
    queryKey: ["run-batches", offset],
    queryFn: () => api.runBatches.list({ limit: 20, offset }),
    // Süren bir toplu iş varken ekran kendiliğinden tazelenir; bitmişse
    // boşuna istek atmaz.
    refetchInterval: (q) =>
      q.state.data?.items.some((b) => isBatchActive(b.status)) ? 4000 : false,
  });

  if (batches.isPending) return <Skeleton rows={4} />;
  if (batches.isError) {
    return <Notice tone="error">{describeError(batches.error).message}</Notice>;
  }

  return (
    <div className="space-y-6">
      <PageHeader
        title="Toplu çalıştırmalar"
        description="Bir akışın birden çok projede sıraya alınmış koşumları. Yeni bir toplu iş akış ekranından başlatılır."
      />

      {batches.data.total === 0 ? (
        <EmptyState
          icon={<IconLayers />}
          title="Henüz toplu çalıştırma yok"
          description="Bir akışı birden fazla projede çalıştırmak için akış ekranındaki “Çok projede çalıştır” bölümünü kullanın."
        />
      ) : (
        <>
          <List>
            {batches.data.items.map((b) => (
              <BatchRow key={b.id} batch={b} />
            ))}
          </List>
          <Pagination
            total={batches.data.total}
            limit={batches.data.limit}
            offset={batches.data.offset}
            onChange={setOffset}
            unit="toplu iş"
          />
        </>
      )}
    </div>
  );
}

function BatchRow({ batch }: { batch: RunBatch }) {
  const biten = batch.counts.succeeded + batch.counts.failed;

  return (
    <Link
      href={`/run-batches/${batch.id}`}
      className="flex items-center gap-4 px-4 py-3 transition-colors hover:bg-raised"
    >
      <div className="w-24 shrink-0">
        <RunBatchBadge status={batch.status} />
      </div>

      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium">{batch.workflowName}</div>
        <div className="truncate text-2xs text-ink-3">
          {batch.task || "görev metni yok"}
        </div>
      </div>

      {/* İlerleme tek bakışta: "12 / 30". Yüzde yazmak, kaç işin kaldığını
          gizlerdi — kullanıcının sorusu o. */}
      <span className="shrink-0 text-xs text-ink-2 tabular-nums">
        {biten} / {batch.counts.total}
      </span>

      <div className="hidden shrink-0 sm:block">
        <CountStrip counts={batch.counts} active={isBatchActive(batch.status)} compact />
      </div>
    </Link>
  );
}
