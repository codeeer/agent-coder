"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import { WorkflowRunBadge } from "@/components/workflows/WorkflowStatusBadge";
import { IconCheck } from "@/components/ui/icons";
import {
  Button,
  Card,
  List,
  PageHeader,
  Section,
  Skeleton,
  formatRelative,
} from "@/components/ui/primitives";

/**
 * Açılış ekranı.
 *
 * Buradaki tek soru şu: **kullanıcı şimdi ne yapmalı?**
 *
 * Önceki hali sistemin durumunu (backend sağlığı, sürüm, ortam) ve geliştirme
 * yol haritasını gösteriyordu — ikisi de geliştiricinin sorusuydu, kullanıcının
 * değil. Kurulumu bitmemiş biri ne yapacağını, bitmiş biri de dün ne olduğunu
 * öğrenemiyordu.
 *
 * Şimdi ekran duruma göre iki şeyden birini gösteriyor: eksik kurulum adımları,
 * ya da son çalışmalar.
 */
export default function DashboardPage() {
  const providers = useQuery({ queryKey: ["llm-providers"], queryFn: api.llmProviders.list });
  const projects = useQuery({ queryKey: ["projects"], queryFn: () => api.projects.list({ limit: 200 }) });
  const workflows = useQuery({ queryKey: ["workflows"], queryFn: () => api.workflows.list() });
  const runs = useQuery({
    queryKey: ["workflow-runs", "dashboard"],
    queryFn: () => api.workflowRuns.list({ limit: 5 }),
  });
  const report = useQuery({
    queryKey: ["report", "dashboard"],
    queryFn: () => api.reports.summary({ days: 7 }),
  });

  const loading = providers.isPending || projects.isPending || workflows.isPending;

  const steps = [
    {
      done: (providers.data?.length ?? 0) > 0,
      title: "Bir model sağlayıcı tanımlayın",
      help: "Agent'ların hangi modelle konuşacağı buradan gelir.",
      href: "/settings" as const,
      cta: "Ayarlar'a git",
    },
    {
      done: (projects.data?.total ?? 0) > 0,
      title: "Bir proje ekleyin",
      help: "Agent'ların üzerinde çalışacağı kod deposu.",
      href: "/projects" as const,
      cta: "Proje ekle",
    },
    {
      done: (workflows.data?.total ?? 0) > 0,
      title: "İlk akışınızı kurun",
      help: "Adımları birbirine bağlayın: kod yaz → incele → PR aç.",
      href: "/workflows" as const,
      cta: "Akış oluştur",
    },
  ];
  const remaining = steps.filter((s) => !s.done);

  return (
    <div className="space-y-7">
      <PageHeader
        title="Agent Coder"
        description="Kod yazan agent'ları birbirine bağlayıp çalıştırın: Jira task'ından PR'a kadar."
        actions={
          <Link href="/workflows">
            {/* "+" simgesi yoktu: düğme yeni bir şey OLUŞTURMUYOR, listeye
                götürüyor. Simge ile etiket aynı şeyi vaat etmeli. */}
            <Button variant="primary">Akışlara git</Button>
          </Link>
        }
      />

      {loading ? (
        <Skeleton rows={3} />
      ) : remaining.length > 0 ? (
        <Section
          title="Başlarken"
          description="Üç adım; sırayla yapılması gerekiyor."
        >
          <div className="space-y-2.5">
            {steps.map((s) => (
              <SetupStep key={s.title} {...s} first={s === remaining[0]} />
            ))}
          </div>
        </Section>
      ) : (
        <>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <Stat
              label="Son 7 gün"
              value={String(report.data?.totals.runs ?? "—")}
              hint="çalıştırma"
            />
            <Stat
              label="Maliyet"
              value={report.data ? formatUsd(report.data.totals.costUsd) : "—"}
              hint="son 7 gün"
            />
            <Stat
              label="Başarı"
              value={successRate(report.data?.totals)}
              hint="tamamlanan işlerde"
            />
            <Stat
              label="Akış"
              value={String(workflows.data?.total ?? "—")}
              hint="tanımlı"
            />
          </div>

          <Section
            title="Son çalışmalar"
            description="Akışların en son ne yaptığı."
            actions={
              <Link href="/runs">
                <Button size="sm">Tümü</Button>
              </Link>
            }
          >
            {runs.isPending ? (
              <Skeleton rows={3} />
            ) : (runs.data?.items.length ?? 0) === 0 ? (
              <Card>
                <p className="text-sm text-ink-2">
                  Henüz çalışma yok. Bir akış açıp <strong>Akışı çalıştır</strong>{" "}
                  deyin.
                </p>
              </Card>
            ) : (
              /*
               * Tek kap, ayraçlı satırlar — beş ayrı kart değil.
               *
               * Her satır kendi kenarlığı ve gölgesi olan bağımsız bir kart
               * olarak duruyordu: listede beş kenarlık, beş gölge ve
               * aralarında beş boşluk vardı. Bir liste tek bir şeydir; her
               * satırı ayrı bir yüzey yapmak, aralarındaki ilişkiyi görsel
               * olarak koparıyordu. Akış ekranındaki "Son çalışmalar" listesi
               * zaten bu düzeni kullanıyordu — ikisi artık aynı.
               */
              <List>
                {runs.data?.items.map((r) => (
                  <Link
                    key={r.id}
                    href={`/workflows/${r.workflowId}/runs/${r.id}`}
                    className="flex flex-wrap items-center gap-x-3 gap-y-1.5 px-4 py-2.5 transition-colors hover:bg-raised"
                  >
                    <WorkflowRunBadge status={r.status} />
                    <span className="min-w-0 flex-1 truncate text-sm font-medium">
                      {r.workflowName}
                    </span>
                    <span className="text-xs text-ink-3">
                      {formatRelative(r.createdAt)}
                    </span>
                  </Link>
                ))}
              </List>
            )}
          </Section>
        </>
      )}
    </div>
  );
}

/**
 * Kurulum adımı.
 *
 * Yalnızca SIRADAKİ adımın düğmesi var: üçünü birden vurgulamak, hangisinden
 * başlanacağını yine kullanıcıya bırakmak olurdu.
 */
function SetupStep({
  done,
  title,
  help,
  href,
  cta,
  first,
}: {
  done: boolean;
  title: string;
  help: string;
  href: "/settings" | "/projects" | "/workflows";
  cta: string;
  first: boolean;
}) {
  return (
    <Card className={done ? "opacity-60" : undefined}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-2.5">
          <span
            /* Daire 18px'ti; içine giren onay ikonu artık 16px olduğu için
               20px'e çıktı. Tersini yapıp ikonu küçültmek, arayüzdeki tek
               ikon boyutu kuralını tek bir yer için delmek olurdu. */
            className={`mt-px flex size-5 shrink-0 items-center justify-center rounded-full border ${
              done ? "border-ok bg-ok text-white" : "border-line-strong text-ink-3"
            }`}
          >
            {done && <IconCheck className="size-4" />}
          </span>
          <div className="min-w-0">
            <p className={`text-sm font-medium ${done ? "line-through" : ""}`}>
              {title}
            </p>
            <p className="mt-0.5 text-xs text-ink-2">{help}</p>
          </div>
        </div>
        {!done && (
          <Link href={href}>
            <Button size="sm" variant={first ? "primary" : undefined}>
              {cta}
            </Button>
          </Link>
        )}
      </div>
    </Card>
  );
}

function Stat({ label, value, hint }: { label: string; value: string; hint: string }) {
  return (
    <Card>
      <div className="text-2xs font-medium tracking-wide text-ink-3 uppercase">
        {label}
      </div>
      <div className="mt-2 text-xl leading-none font-semibold tabular-nums">
        {value}
      </div>
      <div className="mt-1.5 text-xs text-ink-3">{hint}</div>
    </Card>
  );
}

/** Panelde kuruş hassasiyeti gerekmez; okunur bir büyüklük yeter. */
function formatUsd(v: number): string {
  if (v === 0) return "$0";
  return v < 0.01 ? "<$0,01" : `$${v.toFixed(2).replace(".", ",")}`;
}

function successRate(t?: { runs: number; succeeded: number; active: number }): string {
  if (!t) return "—";
  const finished = t.runs - t.active;
  if (finished <= 0) return "—";
  return `%${Math.round((t.succeeded / finished) * 100)}`;
}
