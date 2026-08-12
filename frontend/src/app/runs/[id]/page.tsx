"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { useRunEvents } from "@/lib/use-run-events";
import type { Run } from "@/lib/types";
import { Markdown } from "@/components/markdown/Markdown";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import {
  formatCompact,
  formatCount,
  formatDuration,
  formatMoney,
} from "@/components/charts/format";
import { IconAgent, IconAlert, IconExternal, IconTrash } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  ConfirmStrip,
  IconTile,
  Input,
  Metric,
  Notice,
  Panel,
  Skeleton,
  StatusDot,
  Well,
  formatDate,
  formatRelative,
} from "@/components/ui/primitives";

export default function RunDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const {
    data: run,
    isPending,
    isError,
    error,
  } = useQuery({
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

  /*
   * Silme.
   *
   * Mutasyon yükleme kontrolünün ÜSTÜNDE tanımlı: hook'lar koşullu return'den
   * sonra çağrılamaz. Tetiklenmesi yalnızca kayıt geldikten sonra mümkün.
   *
   * Rapor anahtarı da tazelenir — maliyet ve token doğrudan bu satırdan
   * toplanıyor, kayıt gidince geçmiş rakamlar da değişiyor.
   */
  const remove = useMutation({
    mutationFn: () => api.runs.remove(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["runs"] });
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
      void queryClient.invalidateQueries({ queryKey: ["report"] });
      router.push("/runs");
    },
  });

  if (isPending) return <Skeleton rows={4} />;
  if (isError)
    return <Notice tone="error">{describeError(error).message}</Notice>;

  const tokens = run.promptTokens + run.completionTokens;
  const adds = run.files.reduce((sum, f) => sum + f.additions, 0);
  const dels = run.files.reduce((sum, f) => sum + f.deletions, 0);

  return (
    /* Tam yükseklik: künye üstte sabit, içerik ortada kayar — arayüzün
       geri kalanıyla aynı kabuk düzeni. */
    <div className="flex min-h-0 flex-1 flex-col">
      {/*
        KÜNYE KARTI.

        Öncesinde başlık `PageHeader` idi ve görev metninin ilk 70 karakteri
        H1 olarak basılıyordu — bir cümlenin ortasından kesilmiş hâli sayfa
        başlığı olmuyordu. Şimdi görev iki satıra kadar açılıyor, kimlik
        bilgisi (agent, model, proje, branch) altına iniyor ve ölçüler
        kartın kendi şeridinde duruyor.
      */}
      <Card className="mb-4 shrink-0">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex min-w-0 items-start gap-3">
            <IconTile tone={toneOf(run)}>
              <IconAgent className="size-4" />
            </IconTile>

            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <RunStatusBadge status={run.status} />
                {run.workflowName && (
                  <Badge tone="accent">
                    {run.workflowName} · {run.stepName}
                  </Badge>
                )}
                {run.pushedBranch && (
                  <Badge tone="info">→ {run.pushedBranch}</Badge>
                )}
              </div>

              {/* Görev metni KIRPILMIYOR, iki satıra sarılıyor. Bu ekranın
                  konusu bu cümle. */}
              <h1 className="mt-2 line-clamp-2 text-lg font-semibold tracking-[-0.02em]">
                {run.task}
              </h1>

              <p className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-ink-3">
                <span className="font-mono text-ink-2">{run.agentSlug}</span>
                <span aria-hidden="true">·</span>
                <span className="font-mono">{run.modelId}</span>
                <span aria-hidden="true">·</span>
                <span>{run.projectName}</span>
                <span aria-hidden="true">·</span>
                <span className="font-mono">{run.branch}</span>
                <span aria-hidden="true">·</span>
                <span>{formatRelative(run.createdAt)}</span>
              </p>
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            <RunActions run={run} onConfirmDelete={() => setConfirmingDelete(true)} />
            <Link
              href="/runs"
              className="rounded text-xs text-ink-3 transition-colors hover:text-accent"
            >
              Tüm çalıştırmalar
            </Link>
          </div>
        </div>

        {/*
          ÖLÇÜLER.

          Öncesinde sağda 260px'lik bir sütunda sekiz eşit ağırlıklı `Field`
          satırı vardı ve o sütun yüzünden bu ekranın asıl içeriği — diff ve
          olay akışı — dar kalıyordu. Dördü buraya çıktı, kalanı üstteki
          üstveri satırına indi.

          SÜRE İLK KEZ GÖRÜNÜYOR: eskiden yalnızca "başladı" ve "bitti"
          damgaları vardı ve farkı kullanıcının kendisi hesaplıyordu.
        */}
        <dl className="mt-4 grid grid-cols-2 gap-4 border-t border-line pt-4 sm:grid-cols-4">
          <Metric
            label="Süre"
            value={durationOf(run)}
            note={run.finishedAt ? formatDate(run.finishedAt) : "sürüyor"}
          />
          <Metric
            label="Token"
            value={tokens > 0 ? formatCompact(tokens) : "—"}
            note={
              tokens > 0
                ? `${formatCount(run.promptTokens)} girdi · ${formatCount(run.completionTokens)} çıktı`
                : undefined
            }
          />
          <Metric
            label="Maliyet"
            value={run.costUsd > 0 ? formatMoney(run.costUsd) : "—"}
            note={run.providerSlug || undefined}
          />
          <Metric
            label="Değişiklik"
            value={
              run.files.length > 0
                ? `${formatCount(run.files.length)} dosya`
                : "—"
            }
            note={
              run.files.length > 0
                ? `+${formatCount(adds)} −${formatCount(dels)} satır`
                : "kod değişmedi"
            }
          />
        </dl>

        {/* Hata künyenin İÇİNDE: ayrı bir kutuya alınsaydı çalıştırmanın
            kimliğinden kopardı ve sayfanın en önemli cümlesi bağlamsız
            kalırdı. */}
        {run.error && (
          <div className="mt-4 rounded-lg border border-danger/30 bg-danger-soft px-3.5 py-2.5">
            <p className="flex items-start gap-2 text-xs">
              <IconAlert className="mt-px size-4 shrink-0 text-danger" />
              <span className="min-w-0">
                <span className="font-medium text-danger">
                  Çalıştırma başarısız
                </span>
                <span className="mt-0.5 block break-words text-ink-2">
                  {run.error}
                </span>
              </span>
            </p>
          </div>
        )}

        {/*
          Silme onayı künyenin içinde, tam genişlikte.

          Sonucu SAYIYLA yazıyor: maliyet ve token doğrudan bu satırın
          sütunlarında duruyor, ayrı bir özet tablo yok — kayıt gidince rapor
          rakamları da o kadar azalıyor. "Emin misiniz?" bunu söylemezdi.
        */}
        {confirmingDelete && (
          <ConfirmStrip
            className="mt-4 rounded-lg border"
            question="Bu çalıştırma kaydı silinsin mi?"
            consequence={
              <>
                <strong>
                  {formatMoney(run.costUsd)}
                  {tokens > 0 && <> ve {formatCompact(tokens)} token</>}
                </strong>{" "}
                raporlardan düşecek; çıktı, diff ve olay geçmişi de gider. Bu
                geri alınamaz.
              </>
            }
            busy={remove.isPending}
            error={remove.isError ? describeError(remove.error).message : undefined}
            onConfirm={() => remove.mutate()}
            onCancel={() => setConfirmingDelete(false)}
          />
        )}
      </Card>

      {/* Kayan bölge: künye sabit kalırken içerik kayıyor. */}
      <div className="-mx-1 min-h-0 flex-1 space-y-4 overflow-y-auto px-1 pb-1">
        <Panel
          title="İlerleme"
          action={
            active ? (
              <span className="flex items-center gap-1.5 text-2xs text-ink-3">
                <StatusDot tone={connected ? "accent" : "neutral"} pulse />
                {connected ? "canlı" : "bağlanıyor…"}
              </span>
            ) : undefined
          }
          padded={false}
        >
          <EventLog runId={id} live={events} active={active} />
        </Panel>

        {run.output && <AgentOutput output={run.output} />}

        {run.diff && <Changes run={run} />}
      </div>
    </div>
  );
}

/** Künye karosunun tonu — durumdan geliyor, dekoratif değil. */
function toneOf(run: Run) {
  if (isActive(run.status)) return "accent" as const;
  if (run.status === "succeeded") return "success" as const;
  if (run.status === "failed") return "danger" as const;
  return "warning" as const;
}

/**
 * Çalıştırma süresi.
 *
 * Süren işler için başlangıçtan ŞU ANA kadar; bitenler için gerçek süre.
 * Liste ekranındaki hesapla aynı.
 */
function durationOf(run: Run): string {
  if (!run.startedAt) return "—";
  const end = run.finishedAt ? new Date(run.finishedAt) : new Date();
  const seconds = (end.getTime() - new Date(run.startedAt).getTime()) / 1000;
  return seconds > 0 ? formatDuration(seconds) : "—";
}

/**
 * Çalıştırmanın kendi eylemleri.
 *
 * Süren işte tek eylem iptal; bitmiş işte gönderme ve silme birlikte
 * durabilir. Silme İPTALİN YERİNE GEÇMEZ — süren bir kaydı silmek, kaydı
 * olmayan bir container bırakırdı; o yüzden yalnızca bitmiş işte görünür.
 */
function RunActions({ run, onConfirmDelete }: {
  run: Run;
  onConfirmDelete: () => void;
}) {
  const queryClient = useQueryClient();
  const [pushing, setPushing] = useState(false);

  const cancel = useMutation({
    mutationFn: () => api.runs.cancel(run.id),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ["run", run.id] }),
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

  if (pushing) {
    return <PushForm run={run} onDone={() => setPushing(false)} />;
  }

  return (
    <>
      {run.diff && !run.pushedBranch && (
        <Button variant="primary" onClick={() => setPushing(true)}>
          Branch&apos;e gönder
        </Button>
      )}
      {/* Akış adımları silinemez: kayıt gitse de akış geçmişinde maliyeti ve
          agent'ı boşalmış bir adım kalırdı. Düğme gizlenmiyor, SEBEBİ
          yazılıyor — gizlenen bir eylem kullanıcıya "yok" der, oysa var. */}
      <Button
        variant="danger"
        icon={<IconTrash className="size-4" />}
        disabled={run.workflowRunId !== null}
        title={
          run.workflowRunId !== null
            ? "Bu çalıştırma bir akışın adımı — akış çalışmasını silin"
            : undefined
        }
        onClick={onConfirmDelete}
      >
        Sil
      </Button>
    </>
  );
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
          className="w-64 font-mono text-xs"
          value={branch}
          onChange={(e) => setBranch(e.target.value)}
        />
        <Button
          variant="primary"
          onClick={() => push.mutate()}
          disabled={push.isPending || branch.trim() === ""}
          icon={<IconExternal className="size-4" />}
        >
          {push.isPending ? "Gönderiliyor…" : "Gönder"}
        </Button>
        <Button onClick={onDone} disabled={push.isPending}>
          Vazgeç
        </Button>
      </div>
      {push.isError && (
        <p className="max-w-md text-right text-xs text-danger">
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
        .map(
          (l) =>
            JSON.parse(l.slice(6)) as {
              seq: number;
              level: string;
              message: string;
            },
        )
        .filter((e) => e.message);
    },
    enabled: !active,
  });

  const items = active ? live : (history.data ?? []);

  if (items.length === 0) {
    return (
      <p className="px-4 py-3.5 text-sm text-ink-3">
        {active ? "Çalışma başlatılıyor…" : "Kayıtlı olay yok."}
      </p>
    );
  }

  return (
    /* Kendi kutusu YOK: pano zaten bir kutu ve `Well` ile sarılınca iç içe
       iki çerçeve çıkıyordu. Kayan alan doğrudan panonun gövdesi. */
    <div ref={boxRef} className="max-h-80 overflow-auto px-4 py-3.5">
      <ul className="space-y-1.5">
        {items.map((e) => (
          <li key={e.seq} className="flex items-start gap-2.5 text-xs">
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
    <Panel
      title="Agent çıktısı"
      action={
        <Button size="sm" onClick={() => setRaw((v) => !v)}>
          {raw ? "Biçimli" : "Ham metin"}
        </Button>
      }
    >
      {raw ? (
        <pre className="overflow-x-auto font-mono text-xs leading-relaxed whitespace-pre-wrap">
          {output}
        </pre>
      ) : (
        <Markdown source={output} />
      )}
    </Panel>
  );
}

/**
 * Değişiklikler — dosya çipleri + diff.
 *
 * Diff bu ekranın en geniş içeriği; yan sütun kaldırıldığı için artık tam
 * genişlikte duruyor ve satırlar sarmadan okunuyor.
 */
function Changes({ run }: { run: Run }) {
  const adds = run.files.reduce((sum, f) => sum + f.additions, 0);
  const dels = run.files.reduce((sum, f) => sum + f.deletions, 0);

  return (
    <Panel
      title="Değişiklikler"
      action={
        <span className="text-2xs tabular-nums text-ink-3">
          {formatCount(run.files.length)} dosya · +{formatCount(adds)} −
          {formatCount(dels)}
        </span>
      }
    >
      <div className="flex flex-wrap gap-2">
        {run.files.map((f) => (
          <Badge
            key={f.file}
            tone={f.status === "added" ? "success" : "neutral"}
          >
            <span className="font-mono">{f.file}</span>
            <span className="ml-1.5 text-ok">+{f.additions}</span>
            <span className="ml-1 text-danger">−{f.deletions}</span>
          </Badge>
        ))}
      </div>

      <Well className="mt-3">
        <pre className="max-h-112 overflow-auto p-3.5 font-mono text-xs leading-relaxed">
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
    </Panel>
  );
}
