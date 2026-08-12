"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { TRIGGER_TEXT, type WorkflowStep } from "@/lib/types";
import { autoLayout } from "@/lib/flow-layout";
import { graphToFlow } from "@/lib/workflow-graph";
import { formatDuration, formatMoney } from "@/components/charts/format";
import { FlowCanvas } from "@/components/flow/FlowCanvas";
import {
  WorkflowRunBadge,
  WorkflowStepBadge,
  isWorkflowActive,
} from "@/components/workflows/WorkflowStatusBadge";
import {
  Badge,
  Button,
  Card,
  Field,
  Mono,
  Notice,
  PageHeader,
  Section,
  Skeleton,
  formatDate,
} from "@/components/ui/primitives";

export default function WorkflowRunPage() {
  const { id, runId } = useParams<{ id: string; runId: string }>();
  const qc = useQueryClient();

  const run = useQuery({
    queryKey: ["workflow-run", runId],
    queryFn: () => api.workflowRuns.get(runId),
  });

  // Tuval, çalışmanın ÇALIŞTIĞI sürümü gösterir — akış sonradan değişmiş olsa
  // bile. Etkin sürümü çizmek, geçmişi yanlış anlatmak olurdu.
  const version = useQuery({
    queryKey: ["workflow-version", run.data?.versionId],
    queryFn: () => api.workflows.version(run.data!.versionId),
    enabled: !!run.data,
  });

  const active = run.data ? isWorkflowActive(run.data.status) : false;

  // Canlı akış yalnızca "bir şey değişti" sinyali taşır; ekranın kaynağı
  // kaydın kendisi. Böylece sayfa yenilense de ilerleme kaybolmaz —
  // olayları ikinci bir tabloda saklamaya gerek kalmıyor.
  const { messages, connected } = useWorkflowEvents(runId, active, () =>
    qc.invalidateQueries({ queryKey: ["workflow-run", runId] }),
  );

  const cancel = useMutation({
    mutationFn: () => api.workflowRuns.cancel(runId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["workflow-run", runId] }),
  });

  if (run.isPending) return <Skeleton rows={4} />;
  if (run.isError) return <Notice tone="error">{describeError(run.error).message}</Notice>;

  const r = run.data;
  const done = r.steps.filter((s) => s.status === "succeeded").length;

  return (
    <div>
      <PageHeader
        title={r.input || "Akış çalışması"}
        description={
          <span className="flex flex-wrap items-center gap-2">
            <WorkflowRunBadge status={r.status} />
            <Link href={`/workflows/${id}`} className="text-sm hover:text-accent">
              {r.workflowName}
            </Link>
            <Badge>v{r.version}</Badge>
            <span className="text-xs text-ink-3">
              {TRIGGER_TEXT[r.triggerKind].long} ·{" "}
              {formatDate(r.createdAt)}
            </span>
          </span>
        }
        actions={
          active ? (
            <Button variant="danger" disabled={cancel.isPending} onClick={() => cancel.mutate()}>
              {cancel.isPending ? "Durduruluyor…" : "Durdur"}
            </Button>
          ) : (
            <Link href={`/workflows/${id}`}>
              <Button>Akışa dön</Button>
            </Link>
          )
        }
      />

      {r.error && (
        <Notice tone="error" title="Akış durdu">
          {r.error}
        </Notice>
      )}

      <div className="grid gap-5 lg:grid-cols-[1fr_260px]">
        <div className="space-y-5">
          {version.data && (
            <Section
              title="Akış"
              description="Çalışmanın kullandığı tanım; adımlar durumlarına göre renkleniyor."
            >
              <RunCanvas graph={version.data.graph} steps={r.steps} />
            </Section>
          )}

          <Section
            title="Adımlar"
            description={
              active
                ? connected
                  ? `${done}/${r.steps.length} tamamlandı · canlı`
                  : "bağlantı kuruluyor…"
                : `${done}/${r.steps.length} tamamlandı`
            }
          >
            <div className="space-y-2.5">
              {r.steps.map((s, i) => (
                <StepRow key={s.id} step={s} index={i} />
              ))}
            </div>
          </Section>

          {messages.length > 0 && (
            <Section title="İlerleme">
              <Card padded={false}>
                <ul className="max-h-64 overflow-auto p-3 font-mono text-xs">
                  {messages.map((m, i) => (
                    <li
                      key={i}
                      className={
                        m.level === "error"
                          ? "text-danger"
                          : m.level === "warn"
                            ? "text-warn"
                            : "text-ink-2"
                      }
                    >
                      {m.message}
                    </li>
                  ))}
                </ul>
              </Card>
            </Section>
          )}
        </div>

        <aside className="space-y-3">
          <Card>
            <dl className="space-y-3">
              {/* Proje EN ÜSTTE: aynı akış farklı projelerde koşabildiği için
                  "bu çalışma nerede koştu" artık akıştan çıkarılamıyor. */}
              <Field label="Proje" value={r.projectName} />
              <Field label="Toplam maliyet" value={formatMoney(r.costUsd)} />
              <Field label="Token" value={r.tokens.toLocaleString("tr-TR")} />
              <Field
                label="Süre"
                value={
                  r.startedAt && r.finishedAt
                    ? formatDuration(
                        (new Date(r.finishedAt).getTime() - new Date(r.startedAt).getTime()) / 1000,
                      )
                    : "—"
                }
              />
              <Field label="Görev" value={r.input || "—"} />
            </dl>
          </Card>

          {Object.keys(r.triggerPayload).length > 0 && (
            <Card>
              <p className="mb-2 text-2xs tracking-wide text-ink-3 uppercase">
                Tetikleyici verisi
              </p>
              <dl className="space-y-1.5">
                {Object.entries(r.triggerPayload).map(([k, v]) => (
                  <div key={k} className="flex justify-between gap-3 text-xs">
                    <Mono>{k}</Mono>
                    <span className="truncate text-ink-2">{v}</span>
                  </div>
                ))}
              </dl>
            </Card>
          )}
        </aside>
      </div>
    </div>
  );
}

/**
 * Çalışmanın tuvali.
 *
 * Düzenleme tuvaliyle AYNI bileşen, salt okunur modda: ikinci bir çizim
 * bileşeni yazmak, aynı görselin iki yerde tutulması ve er geç ayrışması
 * demek olurdu.
 */
function RunCanvas({
  graph,
  steps,
}: {
  graph: import("@/lib/types").WorkflowGraph;
  steps: WorkflowStep[];
}) {
  // Adım kaydından gelen bilgiler düğüme iliştirilir. Agent adı ÇALIŞMA
  // kaydından okunur, graftaki kimlikten değil: agent sonradan silinmiş olsa
  // bile o çalışmada neyin koştuğu doğru görünmeli.
  const byNode = new Map(steps.map((s) => [s.nodeId, s]));
  const flow = graphToFlow(graph);

  const nodes = autoLayout(flow.nodes, flow.edges).map((n) => {
    const step = byNode.get(n.id);
    return {
      ...n,
      data: {
        ...n.data,
        status: step?.status,
        agentLabel: step?.agentSlug || undefined,
        readOnly: true,
      },
    };
  });

  return (
    <FlowCanvas nodes={nodes} edges={flow.edges} readOnly height={360} />
  );
}

/** Tek bir adım satırı. */
function StepRow({ step, index }: { step: WorkflowStep; index: number }) {
  const duration =
    step.startedAt && step.finishedAt
      ? (new Date(step.finishedAt).getTime() - new Date(step.startedAt).getTime()) / 1000
      : 0;

  return (
    <Card className={step.status === "running" ? "border-accent/40" : ""}>
      {/*
        `flex-wrap` KALDIRILDI.

        Aynı listede iki farklı adım düzeni görünüyordu: kısa içerikli adımda
        durum rozeti sağ üstte, uzun çıktılı adımda sol altta. İki ayrı kod
        yolu sanılabilir ama tek bir bileşen var — sebep buydu: sol blok
        uzayınca sağdaki rozet bloğu sarmalanıp alt satıra, yani sola
        düşüyordu. Durumun yeri içeriğin uzunluğuna göre değişiyordu.

        Sarmalama yerine sol blok `min-w-0 flex-1` ile daraltılabilir yapıldı;
        uzun metin artık kendi sütununda sarıyor, rozet her adımda aynı yerde.
      */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 flex-1 items-start gap-2.5">
          <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-raised text-2xs font-medium text-ink-2">
            {index + 1}
          </span>
          <div className="min-w-0">
            <div className="text-sm font-medium">{step.nodeName || step.nodeId}</div>
            <div className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-ink-3">
              {step.agentSlug && <Mono>{step.agentSlug}</Mono>}
              {step.modelId && <Mono>{step.modelId}</Mono>}
              {duration > 0 && <span>{formatDuration(duration)}</span>}
              {step.costUsd > 0 && <span>{formatMoney(step.costUsd)}</span>}
            </div>
            {/* Agent olmayan adımın sonucu: PR adresi en işe yarar bilgi,
                kaybolmamalı. */}
            {step.resultText && (
              <p className="mt-1 text-xs text-ink-2">{step.resultText}</p>
            )}
            {step.resultUrl && (
              <a
                href={step.resultUrl}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-0.5 inline-block truncate text-xs text-accent underline underline-offset-2"
              >
                {step.resultUrl}
              </a>
            )}
            {step.error && <p className="mt-1.5 text-xs text-danger">{step.error}</p>}
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <WorkflowStepBadge status={step.status} />
          {step.runId && (
            <Link href={`/runs/${step.runId}`}>
              <Button size="sm">Detay</Button>
            </Link>
          )}
        </div>
      </div>
    </Card>
  );
}

/* ── Canlı akış ──────────────────────────────────────────────────────────── */

interface Progress {
  level: string;
  message: string;
}

/**
 * Akış ilerlemesini dinler.
 *
 * Her olayda kaydı yeniden çekiyoruz: adım durumları ve maliyetler
 * veritabanında duruyor, olay yalnızca "değişti" diyor. Durumu olay akışından
 * kurmaya çalışmak, iki kaynağın ayrışması demek olurdu.
 */
function useWorkflowEvents(runId: string, enabled: boolean, onChange: () => void) {
  const [messages, setMessages] = useState<Progress[]>([]);
  const [connected, setConnected] = useState(false);
  const changed = useRef(onChange);
  changed.current = onChange;

  useEffect(() => {
    if (!enabled) return;

    const source = new EventSource(api.workflowRuns.eventsUrl(runId));
    source.onopen = () => setConnected(true);
    source.onerror = () => setConnected(false);

    source.onmessage = (msg) => {
      let e: { level?: string; message?: string; terminal?: boolean };
      try {
        e = JSON.parse(msg.data) as typeof e;
      } catch {
        return;
      }

      if (e.message) {
        const text = e.message;
        setMessages((prev) => [...prev, { level: e.level ?? "info", message: text }]);
      }
      changed.current();

      if (e.terminal) {
        source.close();
        setConnected(false);
      }
    };

    return () => {
      source.close();
      setConnected(false);
    };
  }, [runId, enabled]);

  return { messages, connected };
}
