"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { readableFailure } from "@/lib/failure";
import {
  TRIGGER_TEXT,
  type ReportSummary,
  type Run,
  type WorkflowRun,
} from "@/lib/types";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import { WorkflowRunBadge } from "@/components/workflows/WorkflowStatusBadge";
import { BarList } from "@/components/charts/BarList";
import { Donut } from "@/components/charts/Donut";
import {
  formatCompact,
  formatCount,
  formatMoney,
  formatPercent,
} from "@/components/charts/format";
import { StatCard, type StatCardProps } from "@/components/ui/StatCard";
import { PANO_KPI, kpi } from "@/lib/kpi";
import {
  IconAgent,
  IconCheck,
  IconChip,
  IconFolder,
  IconRefresh,
  IconWorkflow,
} from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  IconTile,
  Metric,
  Panel,
  PageHeader,
  Segmented,
  Select,
  Skeleton,
  Toolbar,
  formatRelative,
  panelLinkClass,
  toneFromKey,
} from "@/components/ui/primitives";

/**
 * Dashboard — sistemin o anki hâli, tek ekranda.
 *
 * ÖNCEKİ HALİ BİR KARŞILAMA EKRANIYDI: dört rakam ve son beş çalışma.
 * Ekranın üçte ikisi boştu ve "şu an ne oluyor?" sorusunun cevabı yoktu —
 * kullanıcı her seferinde Çalıştırmalar'a gidip bakmak zorundaydı.
 *
 * Şimdi bir PANO: üstte dönemin rakamları yönleriyle birlikte, altında
 * altı pano — ne çalışıyor, ne oldu, ne kadarı tuttu, para hangi modele
 * gitti, hangi işler pahalıydı, hangi agent ne kadar iş yaptı.
 *
 * Kurulumu bitmemiş bir sistemde bunların hiçbiri anlamlı değil; o durumda
 * ekran yine kurulum adımlarını gösterir (aşağıdaki `remaining` dalı).
 *
 * UYDURULAN HİÇBİR RAKAM YOK. Bu tür panolarda alışılmış "kazanılan süre",
 * "birleştirilen PR", "aktif agent sayısı" gibi kutular burada bilerek
 * yok: sistem PR'ın sonrasını takip etmiyor, agent'lar sürekli çalışan
 * süreçler değil ve "kazanılan süre" ölçülen değil uydurulan bir sayıdır.
 */

const PERIODS = [
  { id: "7", label: "7 gün" },
  { id: "30", label: "30 gün" },
  { id: "90", label: "90 gün" },
] as const;

export default function DashboardPage() {
  const [days, setDays] = useState<(typeof PERIODS)[number]["id"]>("7");
  const [project, setProject] = useState("");

  const providers = useQuery({
    queryKey: ["llm-providers"],
    queryFn: api.llmProviders.list,
  });
  const projects = useQuery({
    queryKey: ["projects"],
    queryFn: () => api.projects.list({ limit: 200 }),
  });
  const workflows = useQuery({
    queryKey: ["workflows"],
    queryFn: () => api.workflows.list(),
  });

  const report = useQuery({
    queryKey: ["report", "dashboard", days, project],
    queryFn: () =>
      api.reports.summary({
        days: Number(days),
        project: project || undefined,
      }),
  });

  const runs = useQuery({
    queryKey: ["runs", "dashboard", project],
    queryFn: () => api.runs.list({ limit: 30, project: project || undefined }),
    // Çalışan iş varsa pano kendini tazelesin — "canlı etkinlik" panosu ancak
    // gerçekten canlıysa o adı hak eder.
    refetchInterval: (q) =>
      q.state.data?.items.some((r) => isActive(r.status)) ? 5000 : false,
  });

  const workflowRuns = useQuery({
    queryKey: ["workflow-runs", "dashboard"],
    queryFn: () => api.workflowRuns.list({ limit: 8 }),
  });

  const setupLoading =
    providers.isPending || projects.isPending || workflows.isPending;

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

  if (setupLoading || remaining.length > 0) {
    return (
      <div>
        <PageHeader
          title="Dashboard"
          description="Kod yazan agent'ları birbirine bağlayıp çalıştırın: Jira task'ından PR'a kadar."
        />
        {setupLoading ? (
          <Skeleton rows={3} />
        ) : (
          <div className="max-w-3xl space-y-2.5">
            {steps.map((s) => (
              <SetupStep key={s.title} {...s} first={s === remaining[0]} />
            ))}
          </div>
        )}
      </div>
    );
  }

  const data = report.data;

  return (
    /* Pano da kabuğun tam yükseklik düzenini kullanıyor: panolar ızgarası
       kayan bölgede, başlık ve süzgeç şeridi üstte sabit. */
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader
        title="Dashboard"
        description={
          data
            ? `Son ${data.days} gün · ${data.timezone} saatiyle`
            : "Agent'ların o anki durumu ve dönemin özeti."
        }
      />

      {/*
        Süzgeç şeridi — referansın her ekranında başlığın hemen altında.
        Dönem ve proje seçimi PANONUN TAMAMINA uygulanır, tek bir kutuya
        değil; bu yüzden panoların içinde değil, üstünde tek bir yerde
        duruyor.
      */}
      <Toolbar>
        <div className="w-48 shrink-0">
          <Select
            className="h-8 text-xs"
            value={project}
            onChange={(e) => setProject(e.target.value)}
            aria-label="Proje süzgeci"
          >
            <option value="">Tüm projeler</option>
            {projects.data?.items.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </Select>
        </div>

        <Segmented
          label="Dönem"
          options={PERIODS}
          value={days}
          onChange={setDays}
        />

        <div className="ml-auto flex items-center gap-2">
          {report.isFetching && (
            <span className="text-2xs text-ink-3">tazeleniyor…</span>
          )}
          <Button
            size="sm"
            icon={<IconRefresh className="size-4" />}
            onClick={() => {
              void report.refetch();
              void runs.refetch();
              void workflowRuns.refetch();
            }}
          >
            Yenile
          </Button>
        </div>
      </Toolbar>

      <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1 pb-1">
        {report.isPending || !data ? (
          <Skeleton rows={4} />
        ) : (
          <div className="space-y-4">
            <KpiStrip data={data} />

            <div className="grid items-start gap-4 xl:grid-cols-3">
              <LiveActivity
                runs={runs.data?.items ?? []}
                loading={runs.isPending}
              />
              <Timeline runs={workflowRuns.data?.items ?? []} />
              <Outcomes data={data} />
            </div>

            <div className="grid items-start gap-4 xl:grid-cols-3">
              <ModelUsage data={data} />
              <TopRuns runs={runs.data?.items ?? []} />
              <AgentBreakdown data={data} />
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

/* ── Rakam şeridi ────────────────────────────────────────────────────────── */

/** Panonun şerit sırası — rapor ekranınınkinden farklı ve bu bilinçli. */
/**
 * Dönemin sekiz rakamı — her biri yönüyle birlikte.
 *
 * Rapor ekranındaki şeritle AYNI bileşeni kullanıyor (`StatCard`). Aynı
 * rakamın iki ekranda farklı biçimlenmesi, iki farklı sayı olduğu
 * izlenimini verirdi.
 */
function KpiStrip({ data }: { data: ReportSummary }) {
  /*
   * Sekiz rakam SİMGESİZ: dört-sekiz kartlık bir şeritte simge ayırt etmeye
   * yardım etmiyor, gürültü ekliyor. Rapor ekranındaki on kartlık şeritte
   * durum tersi ve orada simge var.
   */
  const cards: StatCardProps[] = PANO_KPI.map((key) => ({
    ...kpi(key, data),
    periodNote: "öncekine göre",
  }));

  return (
    /* Dar ekranda 2, orta ekranda 4, geniş ekranda sekizi tek sıra. Sabit
       sekizli bırakmak 1280px'de etiketleri kesiyordu. */
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4 2xl:grid-cols-8">
      {cards.map((c) => (
        <StatCard key={c.label} {...c} />
      ))}
    </div>
  );
}

/* ── Canlı etkinlik ──────────────────────────────────────────────────────── */

/**
 * Şu an çalışan işler.
 *
 * Boşken "hiçbir şey çalışmıyor" demek yetmez — o cümle bir sonraki adımı
 * söylemiyor. Bu yüzden boş durumda en son biten işler gösteriliyor: pano
 * hiçbir zaman boş bir kutu olmuyor.
 */
function LiveActivity({ runs, loading }: { runs: Run[]; loading: boolean }) {
  const active = runs.filter((r) => isActive(r.status));
  const shown = active.length > 0 ? active.slice(0, 5) : runs.slice(0, 5);

  return (
    <Panel
      title={
        <span className="flex items-center gap-2">
          Canlı etkinlik
          {active.length > 0 && (
            <Badge tone="accent">{active.length} çalışıyor</Badge>
          )}
        </span>
      }
      action={
        <Link href="/runs" className={panelLinkClass}>
          Tümü
        </Link>
      }
      padded={false}
    >
      {loading ? (
        <p className="p-4 text-sm text-ink-3">Yükleniyor…</p>
      ) : shown.length === 0 ? (
        <p className="p-4 text-sm text-ink-3">
          Bu projede henüz çalıştırma yok.
        </p>
      ) : (
        <>
          {active.length === 0 && (
            <p className="border-b border-line px-4 py-2 text-2xs text-ink-3">
              Şu an çalışan iş yok — en son bitenler:
            </p>
          )}
          <ul className="divide-y divide-line">
            {shown.map((r) => (
              <li key={r.id}>
                <Link
                  href={`/runs/${r.id}`}
                  className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-raised"
                >
                  <IconTile tone={toneFromKey(r.agentSlug)}>
                    <IconAgent className="size-4" />
                  </IconTile>

                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-sm font-medium">
                        {r.agentSlug}
                      </span>
                      <span className="shrink-0 font-mono text-2xs text-ink-3">
                        {r.projectName}
                      </span>
                    </div>
                    <p className="mt-0.5 truncate text-xs text-ink-2">
                      {r.task}
                    </p>
                  </div>

                  <div className="flex shrink-0 flex-col items-end gap-1">
                    <RunStatusBadge status={r.status} />
                    <span className="text-2xs text-ink-3">
                      {formatRelative(r.startedAt ?? r.createdAt)}
                    </span>
                  </div>
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}
    </Panel>
  );
}

/* ── Zaman çizelgesi ─────────────────────────────────────────────────────── */

/**
 * Son akış çalışmaları — saatiyle birlikte.
 *
 * Saat SOLDA, sabit genişlikte bir sütunda: referansın zaman çizelgesini
 * taranabilir kılan şey bu hizalama. Göreli zaman ("2 saat önce") burada
 * kullanılmıyor; çizelgenin işi olayları BİRBİRİNE GÖRE sıralamak ve bunun
 * için mutlak saat gerekiyor.
 */
function Timeline({ runs }: { runs: WorkflowRun[] }) {
  return (
    <Panel
      title="Son akış çalışmaları"
      action={
        <Link href="/runs" className={panelLinkClass}>
          Tümü
        </Link>
      }
      padded={false}
    >
      {runs.length === 0 ? (
        <p className="p-4 text-sm text-ink-3">Henüz akış çalışması yok.</p>
      ) : (
        <ul className="divide-y divide-line">
          {runs.slice(0, 6).map((r) => (
            <li key={r.id}>
              <Link
                href={`/workflows/${r.workflowId}/runs/${r.id}`}
                className="flex items-start gap-3 px-4 py-2.5 transition-colors hover:bg-raised"
              >
                <span className="w-10 shrink-0 pt-0.5 font-mono text-2xs tabular-nums text-ink-3">
                  {clockOf(r.finishedAt ?? r.startedAt ?? r.createdAt)}
                </span>

                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm">{r.workflowName}</p>
                  {/* Adım sayısı BİLEREK yok: liste ucu adımları döndürmüyor
                      (`steps` boş geliyor) ve "0 adım" yazmak, olmayan bir
                      bilgiyi sıfırmış gibi göstermek olurdu. */}
                  <p className="mt-0.5 truncate text-2xs text-ink-3">
                    {TRIGGER_TEXT[r.triggerKind].long} ·{" "}
                    {formatMoney(r.costUsd)} · {formatCompact(r.tokens)} token
                  </p>
                </div>

                <span className="shrink-0">
                  <WorkflowRunBadge status={r.status} />
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}

/** ISO zamandan "14:32". Tarih değil saat: çizelge zaten bugünü anlatıyor. */
function clockOf(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleTimeString("tr-TR", { hour: "2-digit", minute: "2-digit" });
}

/* ── Sonuç dağılımı ──────────────────────────────────────────────────────── */

function Outcomes({ data }: { data: ReportSummary }) {
  const t = data.totals;

  /*
   * Dilim renkleri DURUM paletinden değil GRAFİK paletinden geliyor
   * (`--color-chart-*`). Aradaki fark bilinçli: durum renkleri temaya göre
   * değişir, grafik renkleri değişmez — iki ekran görüntüsü
   * karşılaştırıldığında "başarılı yeşil" aynı renk olmalı.
   */
  const slices = [
    {
      key: "ok",
      label: "Tamamlandı",
      value: t.succeeded,
      color: "var(--color-chart-good)",
    },
    {
      key: "active",
      label: "Sürüyor",
      value: t.active,
      color: "var(--color-series)",
    },
    {
      key: "other",
      label: "İptal / zaman aşımı",
      value: t.cancelled + t.timeout + t.interrupted,
      color: "var(--color-chart-other)",
    },
    {
      key: "bad",
      label: "Başarısız",
      value: t.failed,
      color: "var(--color-chart-bad)",
    },
  ].filter((s) => s.value > 0);

  return (
    <Panel
      title="Sonuç dağılımı"
      action={
        <Link href="/reports" className={panelLinkClass}>
          Rapor
        </Link>
      }
    >
      <div className="py-2">
        <Donut
          slices={slices}
          centerValue={formatCount(t.runs)}
          centerNote="çalıştırma"
          formatValue={formatCount}
        />
      </div>

      <dl className="mt-4 grid grid-cols-3 gap-3 border-t border-line pt-4">
        {/* Etiketler KISA: üç kutu panonun üçte birini paylaşıyor ve
            "Gönderilen branch" orada "GÖNDERİLEN BRAN…" diye kırpılıyordu. */}
        <Metric
          size="sm"
          label="Kod üreten"
          value={formatCount(t.runsWithCode)}
        />
        <Metric
          size="sm"
          label="Branch"
          value={formatCount(t.pushedBranches)}
        />
        <Metric size="sm" label="Jira'dan" value={formatCount(t.jiraTasks)} />
      </dl>
    </Panel>
  );
}

/* ── Model kullanımı ─────────────────────────────────────────────────────── */

function ModelUsage({ data }: { data: ReportSummary }) {
  const t = data.totals;

  return (
    <Panel
      title="Model kullanımı"
      action={
        <Link href="/models" className={panelLinkClass}>
          Modeller
        </Link>
      }
    >
      <dl className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <Metric
          size="sm"
          label="Toplam token"
          value={formatCompact(t.promptTokens + t.completionTokens)}
        />
        <Metric size="sm" label="Girdi" value={formatCompact(t.promptTokens)} />
        <Metric
          size="sm"
          label="Çıktı"
          value={formatCompact(t.completionTokens)}
        />
        <Metric size="sm" label="Maliyet" value={formatMoney(t.costUsd)} />
      </dl>

      <div className="mt-4 border-t border-line pt-4">
        <BarList
          rows={data.byModel.slice(0, 4).map((g) => ({
            key: g.key,
            label: g.label,
            value: g.costUsd,
            valueLabel: formatMoney(g.costUsd),
            note: `${formatCount(g.runs)} iş · ${formatCompact(g.tokens)} token`,
          }))}
        />
      </div>
    </Panel>
  );
}

/* ── Öne çıkan çalıştırmalar ─────────────────────────────────────────────── */

/**
 * Dönemin en pahalı işleri.
 *
 * "En yeni" değil "en pahalı" sıralanıyor: en yeniler zaten yandaki iki
 * panoda duruyor ve bir panonun aynı listeyi üçüncü kez göstermesi yer
 * israfı olurdu. Para nereye gitti sorusunun satır bazındaki cevabı bu.
 */
function TopRuns({ runs }: { runs: Run[] }) {
  const top = [...runs].sort((a, b) => b.costUsd - a.costUsd).slice(0, 6);

  return (
    <Panel
      title="En pahalı çalıştırmalar"
      action={
        <Link href="/runs" className={panelLinkClass}>
          Tümü
        </Link>
      }
      padded={false}
    >
      {top.length === 0 ? (
        <p className="p-4 text-sm text-ink-3">Kayıt yok.</p>
      ) : (
        <ul className="divide-y divide-line">
          {top.map((r) => (
            <li key={r.id}>
              <Link
                href={`/runs/${r.id}`}
                className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-raised"
              >
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm">{r.task}</p>
                  <p className="mt-0.5 truncate font-mono text-2xs text-ink-3">
                    {r.agentSlug} · {r.modelId}
                  </p>
                </div>
                <div className="shrink-0 text-right">
                  <div className="font-mono text-xs tabular-nums">
                    {r.costUsd > 0 ? formatMoney(r.costUsd) : "—"}
                  </div>
                  <div className="mt-0.5 text-2xs text-ink-3">
                    {formatRelative(r.createdAt)}
                  </div>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}

/* ── Agent kırılımı ──────────────────────────────────────────────────────── */

function AgentBreakdown({ data }: { data: ReportSummary }) {
  const failures = data.failures.slice(0, 3);

  return (
    <Panel
      title="Agent bazında"
      action={
        <Link href="/agents" className={panelLinkClass}>
          Agent&apos;lar
        </Link>
      }
    >
      <BarList
        rows={data.byAgent.slice(0, 4).map((g) => ({
          key: g.key,
          label: g.label,
          value: g.runs,
          valueLabel: `${formatCount(g.runs)} iş`,
          note: `${formatPercent(g.succeeded, g.runs)} başarı · ${formatMoney(g.costUsd)}`,
        }))}
      />

      {/* Hatalar ancak VARSA görünür. Boş bir "tekrar eden hatalar" başlığı,
          olmayan bir sorunu ima ederdi. */}
      {failures.length > 0 && (
        <div className="mt-4 border-t border-line pt-3">
          <p className="mb-2 text-2xs font-medium tracking-wide text-ink-3 uppercase">
            Tekrar eden hatalar
          </p>
          <ul className="space-y-1.5">
            {failures.map((f) => (
              <li
                key={f.message}
                className="flex items-start justify-between gap-3"
              >
                <span
                  className="min-w-0 flex-1 truncate text-xs text-ink-2"
                  title={f.message}
                >
                  {readableFailure(f.message)}
                </span>
                <Badge tone="danger">{formatCount(f.count)}×</Badge>
              </li>
            ))}
          </ul>
        </div>
      )}
    </Panel>
  );
}

/* ── Kurulum ─────────────────────────────────────────────────────────────── */

/**
 * Kurulum adımı.
 *
 * Yalnızca SIRADAKİ adımın düğmesi vurgulu: üçünü birden vurgulamak,
 * hangisinden başlanacağını yine kullanıcıya bırakmak olurdu.
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
  const Icon =
    href === "/settings"
      ? IconChip
      : href === "/projects"
        ? IconFolder
        : IconWorkflow;

  return (
    <Card className={done ? "opacity-60" : undefined}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          {done ? (
            <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-ok-soft text-ok">
              <IconCheck className="size-4" />
            </span>
          ) : (
            <IconTile tone={first ? "accent" : "info"}>
              <Icon className="size-4" />
            </IconTile>
          )}
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
