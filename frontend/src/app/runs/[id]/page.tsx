"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { useRunEvents } from "@/lib/use-run-events";
import type { Run } from "@/lib/types";
import { Markdown } from "@/components/markdown/Markdown";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import { IconExternal } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  Field,
  Input,
  Mono,
  Notice,
  PageHeader,
  Section,
  Skeleton,
  StatusDot,
  Well,
  formatDate,
} from "@/components/ui/primitives";

export default function RunDetailPage() {
  const { id } = useParams<{ id: string }>();
  const queryClient = useQueryClient();

  const { data: run, isPending, isError, error } = useQuery({
    queryKey: ["run", id],
    queryFn: () => api.runs.get(id),
  });

  const active = run ? isActive(run.status) : false;
  const { events, terminalStatus, connected } = useRunEvents(id, active);

  // Canlı akış "bitti" dediğinde kaydı yeniden çekiyoruz: çıktı, diff ve
  // maliyet ancak o zaman veritabanında hazır olur.
  useEffect(() => {
    if (terminalStatus) {
      void queryClient.invalidateQueries({ queryKey: ["run", id] });
      void queryClient.invalidateQueries({ queryKey: ["runs"] });
    }
  }, [terminalStatus, id, queryClient]);

  if (isPending) return <Skeleton rows={4} />;
  if (isError) return <Notice tone="error">{describeError(error).message}</Notice>;

  return (
    <div>
      <PageHeader
        title={run.task.length > 70 ? run.task.slice(0, 70) + "…" : run.task}
        description={
          <span className="flex flex-wrap items-center gap-2">
            <RunStatusBadge status={run.status} />
            <span className="text-ink-3">·</span>
            <Link href="/runs" className="underline">
              tüm çalıştırmalar
            </Link>
          </span>
        }
        actions={<RunActions run={run} />}
      />

      {run.error && (
        <div className="mb-5">
          <Notice tone="error" title="Çalıştırma başarısız">
            {run.error}
          </Notice>
        </div>
      )}

      <div className="grid gap-5 lg:grid-cols-[1fr_260px]">
        <div className="min-w-0 space-y-5">
          <Section
            title="İlerleme"
            description={
              active
                ? connected
                  ? "Canlı akış bağlı."
                  : "Bağlantı kurulmaya çalışılıyor…"
                : undefined
            }
          >
            <EventLog runId={id} live={events} active={active} />
          </Section>

          {run.output && <AgentOutput output={run.output} />}

          {run.diff && (
            <Section
              title="Değişiklikler"
              description={`${run.files.length} dosya`}
            >
              <DiffView run={run} />
            </Section>
          )}
        </div>

        <aside className="space-y-3">
          <Card>
            <dl className="space-y-3">
              <Field label="Proje" value={run.projectName} />
              <Field label="Branch" value={run.branch} mono />
              <Field label="Agent" value={run.agentSlug} mono />
              <Field label="Model" value={run.modelId} mono />
              <Field
                label="Token"
                value={`${run.promptTokens.toLocaleString("tr")} + ${run.completionTokens.toLocaleString("tr")}`}
                mono
              />
              <Field
                label="Maliyet"
                value={run.costUsd > 0 ? `$${run.costUsd.toFixed(6)}` : "—"}
                mono
              />
              <Field label="Başladı" value={formatDate(run.startedAt)} />
              <Field label="Bitti" value={formatDate(run.finishedAt)} />
            </dl>
          </Card>

          {run.pushedBranch && (
            <Card>
              <Field
                label="Gönderilen branch"
                value={<Mono>{run.pushedBranch}</Mono>}
              />
            </Card>
          )}
        </aside>
      </div>
    </div>
  );
}

function RunActions({ run }: { run: Run }) {
  const queryClient = useQueryClient();
  const [pushing, setPushing] = useState(false);

  const cancel = useMutation({
    mutationFn: () => api.runs.cancel(run.id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["run", run.id] }),
  });

  if (isActive(run.status)) {
    return (
      <Button
        variant="danger"
        onClick={() => cancel.mutate()}
        disabled={cancel.isPending}
      >
        {cancel.isPending ? "İptal ediliyor…" : "İptal et"}
      </Button>
    );
  }

  if (run.diff && !run.pushedBranch) {
    return pushing ? (
      <PushForm run={run} onDone={() => setPushing(false)} />
    ) : (
      <Button variant="primary" onClick={() => setPushing(true)}>
        Branch&apos;e gönder
      </Button>
    );
  }

  return null;
}

function PushForm({ run, onDone }: { run: Run; onDone: () => void }) {
  const queryClient = useQueryClient();
  const suggested = `agent-coder/${run.agentSlug}-${run.id.slice(0, 8)}`;
  const [branch, setBranch] = useState(suggested);

  const push = useMutation({
    mutationFn: () => api.runs.push(run.id, branch.trim()),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["run", run.id] });
      onDone();
    },
  });

  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex items-center gap-2">
        <Input
          className="w-64 font-mono text-[12px]"
          value={branch}
          onChange={(e) => setBranch(e.target.value)}
        />
        <Button
          variant="primary"
          onClick={() => push.mutate()}
          disabled={push.isPending || branch.trim() === ""}
          icon={<IconExternal className="size-3.5" />}
        >
          {push.isPending ? "Gönderiliyor…" : "Gönder"}
        </Button>
        <Button onClick={onDone} disabled={push.isPending}>
          Vazgeç
        </Button>
      </div>
      {push.isError && (
        <p className="max-w-md text-right text-[12px] text-danger">
          {describeError(push.error).message}
        </p>
      )}
    </div>
  );
}

/**
 * Olay akışı.
 *
 * Çalışma sürerken canlı olaylar, bittiğinde veritabanındaki kayıt gösterilir.
 * İkisi de aynı `seq` alanını taşıdığı için karışmaz.
 */
function EventLog({
  runId,
  live,
  active,
}: {
  runId: string;
  live: ReturnType<typeof useRunEvents>["events"];
  active: boolean;
}) {
  const boxRef = useRef<HTMLDivElement>(null);

  // Yeni olay geldikçe en alta kaydır — kullanıcı akışı takip etsin.
  useEffect(() => {
    if (active && boxRef.current) {
      boxRef.current.scrollTop = boxRef.current.scrollHeight;
    }
  }, [live.length, active]);

  // Çalışma bittiyse geçmişi veritabanından oku (SSE kapalı).
  const history = useQuery({
    queryKey: ["run-events", runId],
    queryFn: async () => {
      const res = await fetch(api.runs.eventsUrl(runId));
      const text = await res.text();
      return text
        .split("\n")
        .filter((l) => l.startsWith("data: "))
        .map((l) => JSON.parse(l.slice(6)) as { seq: number; level: string; message: string })
        .filter((e) => e.message);
    },
    enabled: !active,
  });

  const items = active ? live : (history.data ?? []);

  if (items.length === 0) {
    return (
      <Well className="p-4">
        <p className="text-[13px] text-ink-3">
          {active ? "Çalışma başlatılıyor…" : "Kayıtlı olay yok."}
        </p>
      </Well>
    );
  }

  return (
    <Well>
      <div ref={boxRef} className="max-h-80 overflow-auto p-3.5">
        <ul className="space-y-1.5">
          {items.map((e) => (
            <li key={e.seq} className="flex items-start gap-2.5 text-[12px]">
              <span className="mt-1.5">
                <StatusDot
                  tone={
                    e.level === "error"
                      ? "danger"
                      : e.level === "warn"
                        ? "warning"
                        : "neutral"
                  }
                />
              </span>
              <span
                className={
                  e.level === "error"
                    ? "text-danger"
                    : e.level === "warn"
                      ? "text-warn"
                      : "text-ink-2"
                }
              >
                {e.message}
              </span>
            </li>
          ))}
        </ul>
      </div>
    </Well>
  );
}

/**
 * Agent çıktısı.
 *
 * Çıktı Markdown'dır ve ham basıldığında okunmaz (spec 005). Biçimli görünüm
 * varsayılandır; ham metin bir tık uzakta durur çünkü kullanıcı çıktıyı çoğu
 * zaman başka bir yere (PR açıklaması, Jira yorumu) yapıştırıyor.
 */
function AgentOutput({ output }: { output: string }) {
  const [raw, setRaw] = useState(false);

  return (
    <Section
      title="Agent çıktısı"
      actions={
        <Button size="sm" onClick={() => setRaw((v) => !v)}>
          {raw ? "Biçimli" : "Ham metin"}
        </Button>
      }
    >
      <Card>
        {raw ? (
          <pre className="overflow-x-auto font-mono text-[12px] leading-relaxed whitespace-pre-wrap">
            {output}
          </pre>
        ) : (
          <Markdown source={output} />
        )}
      </Card>
    </Section>
  );
}

function DiffView({ run }: { run: Run }) {
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-2">
        {run.files.map((f) => (
          <Badge key={f.file} tone={f.status === "added" ? "success" : "neutral"}>
            <span className="font-mono">{f.file}</span>
            <span className="ml-1.5 text-ok">+{f.additions}</span>
            <span className="ml-1 text-danger">−{f.deletions}</span>
          </Badge>
        ))}
      </div>

      <Well>
        <pre className="max-h-[28rem] overflow-auto p-3.5 font-mono text-[12px] leading-relaxed">
          {run.diff.split("\n").map((line, i) => (
            <div
              key={i}
              className={
                line.startsWith("+++") || line.startsWith("---")
                  ? "text-ink-3"
                  : line.startsWith("+")
                    ? "text-ok"
                    : line.startsWith("-")
                      ? "text-danger"
                      : line.startsWith("@@")
                        ? "text-info"
                        : "text-ink-2"
              }
            >
              {line || " "}
            </div>
          ))}
        </pre>
      </Well>
    </div>
  );
}
