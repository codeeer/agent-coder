"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { readableFailure } from "@/lib/failure";
import { reportPeriods } from "@/lib/report-periods";
import { describeError } from "@/lib/errors";
import type {
  ReportGroup,
  ReportSummary,
  ReportTotals,
  Run,
} from "@/lib/types";
import { BarList } from "@/components/charts/BarList";
import { Donut } from "@/components/charts/Donut";
import { DailyTrendChart } from "@/components/charts/DailyTrendChart";
import { CostTrendChart } from "@/components/charts/CostTrendChart";
import { RunsByDayTable } from "@/components/charts/RunsByDayChart";
import { StatCard, type StatCardProps } from "@/components/ui/StatCard";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import {
  formatCompact,
  formatCount,
  formatDuration,
  formatMoney,
  formatPercent,
  formatPerUnit,
} from "@/components/charts/format";
import {
  IconAgent,
  IconAlert,
  IconCheck,
  IconChip,
  IconComment,
  IconCost,
  IconEdit,
  IconFolder,
  IconPlay,
  IconPullRequest,
  IconRefresh,
} from "@/components/ui/icons";
import {
  Badge,
  Button,
  EmptyState,
  Notice,
  Metric,
  PageHeader,
  Panel,
  Segmented,
  Select,
  Skeleton,
  Toolbar,
  formatRelative,
  panelLinkClass,
  type TileTone,
} from "@/components/ui/primitives";
import { RAPOR_KPI, kpi, type KpiKey } from "@/lib/kpi";

/**
 * Rapor — yönetici özeti.
 *
 * Tek soruya cevap verir: "para nereye gitti, karşılığında ne üretildi, ne
 * kadarı tuttu?" Agent'ın nasıl çalıştırıldığı (arayüz, API, akış) fark
 * etmez; hepsi aynı çalıştırma geçmişine düştüğü için rapor eksiksizdir.
 *
 * SERBEST TARİH ARALIĞI YOK ve bu bilinçli. Uç yalnızca "kaç gün geriye"
 * parametresi alıyor; "geçen ay" bugünden geriye sayan bir pencereyle ifade
 * edilemez. Eklenseydi seçilen aralık ile gösterilen aralık birbirini
 * tutmazdı. Ekranda yazan tarihler seçimden türetilmiyor, yanıtın kendi
 * `from`/`to` alanlarından okunuyor.
 *
 * AÇILIŞ DÖNEMİ SAYFANIN KARARI DEĞİL. Sayfa 30'u sabit tutuyor ve her
 * istekte açıkça gönderiyordu; backend "Varsayılan rapor dönemi" ayarını
 * yalnızca parametre GELMEDİĞİNDE uyguladığı için ayarı 7 yapan kullanıcı
 * yine 30 gün görüyordu. Artık ilk istek dönemi hiç göndermiyor ve etkin
 * dönem yanıttan okunuyor — ayarın tek sahibi backend.
 */

/** Son çalıştırmalar panosunun penceresi. */
const RECENT_LIMIT = 8;

export default function ReportsPage() {
  /*
   * `null` = "kullanıcı henüz seçmedi, ayar geçerli". Ayarın değeri buraya
   * kopyalanmıyor: kopyalansaydı ikinci bir doğruluk kaynağı olur ve ayar
   * değiştiğinde açık duran sayfa eskimiş değeri göstermeye devam ederdi.
   */
  const [days, setDays] = useState<string | null>(null);
  const [project, setProject] = useState("");

  const projects = useQuery({
    queryKey: ["projects"],
    queryFn: () => api.projects.list({ limit: 200 }),
  });

  /*
   * Ayar, SEÇENEK LİSTESİ için okunuyor — açılış dönemi için değil, onu
   * yanıtın kendisi söylüyor. Liste yalnızca gösterilen döneme dayansaydı
   * kullanıcı 90'ı seçtiği anda kendi varsayılanı (örn. 14) listeden
   * düşerdi ve geri dönemezdi. Sorgu anahtarı Ayarlar ekranınınkiyle aynı:
   * ikisi tek önbelleği paylaşır, fazladan istek çıkmaz.
   */
  const settings = useQuery({
    queryKey: ["settings"],
    queryFn: api.settings.list,
  });
  const varsayilanGun = Number(
    settings.data?.items.find((s) => s.key === "reports.default_days")?.value,
  );

  const report = useQuery({
    queryKey: ["report", days, project],
    queryFn: () =>
      api.reports.summary({
        days: days ? Number(days) : undefined,
        project: project || undefined,
      }),
  });

  // Son çalıştırmalar rapor ucunda yok; mevcut liste ucundan geliyor.
  const runs = useQuery({
    queryKey: ["runs", "reports-panel", project],
    queryFn: () =>
      api.runs.list({ limit: RECENT_LIMIT, project: project || undefined }),
    refetchInterval: (query) =>
      query.state.data?.items.some((r) => isActive(r.status)) ? 5000 : false,
  });

  const data = report.data;

  /*
   * Seçili dönem: kullanıcının seçimi varsa o, yoksa yanıtın söylediği.
   * Efekt yok — türetiliyor. Efektle kopyalansaydı ilk boyamada yanlış
   * segment seçili görünür, sonra yerine oturur ve zıplardı.
   */
  const seciliDonem = days ?? (data ? String(data.days) : null);
  const donemler = reportPeriods(
    Number.isFinite(varsayilanGun) ? varsayilanGun : null,
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader
        title="Raporlar"
        description="Agent'ların ne yaptığını ve neye mal olduğunu özetler."
      />

      <Toolbar>
        {/* Yanıt gelene kadar hiçbir segment seçili DEĞİL: hangi dönemin
            geçerli olduğunu yalnızca yanıt söylüyor ve tahmin etmek, yarım
            saniye sonra düzelecek yanlış bir vurgu göstermek olurdu. */}
        <Segmented
          label="Dönem"
          options={donemler}
          value={seciliDonem ?? ""}
          onChange={setDays}
        />

        {/* Genişlik SARMALAYICIDAN gelir: Select'in kendi sınıfı `w-full`
            taşır ve dışarıdan verilen bir genişlik onu yenemez. */}
        <div className="w-44 shrink-0">
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

        {/* Dönemin GERÇEK sınırları — gün sayısından hesaplanmıyor, yanıttan
            okunuyor. Backend saat dilimini kendisi uyguluyor ve iki hesap
            birbirini tutmayabilir. */}
        {data && (
          <span className="hidden text-xs text-ink-2 tabular-nums lg:block">
            {formatRange(data.from, data.to)}
          </span>
        )}

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
            }}
          >
            Yenile
          </Button>
        </div>
      </Toolbar>

      <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1 pb-1">
        {report.isPending && <Skeleton rows={4} />}
        {report.isError && (
          <Notice tone="error" title={describeError(report.error).message}>
            {describeError(report.error).hint}
          </Notice>
        )}

        {data && data.totals.runs === 0 && data.previous.runs === 0 && (
          <EmptyState
            icon={<IconAgent className="size-4" />}
            title="Bu dönemde çalıştırma yok"
            description="Bir agent çalıştırdığınızda maliyet, üretilen değişiklik ve başarı oranı burada toplanır."
            action={
              <Link href="/agents">
                <Button variant="primary">Agent çalıştır</Button>
              </Link>
            }
          />
        )}

        {data && (data.totals.runs > 0 || data.previous.runs > 0) && (
          <div className="space-y-4">
            <KpiStrip data={data} />

            <div className="grid items-start gap-4 xl:grid-cols-[1.6fr_1fr]">
              <Panel
                title="Gün gün seyir"
                action={
                  <span className="text-2xs text-ink-3">
                    {data.days} gün · {data.timezone}
                  </span>
                }
              >
                <DailyTrendChart days={data.daily} />
              </Panel>

              <TokenByModel data={data} />
            </div>

            <div className="grid items-start gap-4 xl:grid-cols-2">
              <AgentPerformance rows={data.byAgent} />
              <RecentRuns
                runs={runs.data?.items ?? []}
                loading={runs.isPending}
              />
            </div>

            <div className="grid items-start gap-4 xl:grid-cols-[1.6fr_1fr]">
              <Panel
                title="Gün gün maliyet"
                action={
                  <span className="font-mono text-xs tabular-nums text-ink-2">
                    {formatMoney(data.totals.costUsd)}
                  </span>
                }
              >
                <CostTrendChart days={data.daily} />
              </Panel>

              <Balance data={data} />
            </div>

            <div className="grid items-start gap-4 xl:grid-cols-2">
              <Panel title="Proje bazında">
                <BarList
                  rows={data.byProject.map((g) => ({
                    key: g.key,
                    label: g.label,
                    value: g.runs,
                    valueLabel: `${formatCount(g.runs)} iş`,
                    note: `${formatMoney(g.costUsd)} · +${formatCompact(
                      g.additions,
                    )} −${formatCompact(g.deletions)} satır`,
                  }))}
                />
              </Panel>

              {data.failures.length > 0 ? (
                <Panel
                  title="Tekrar eden hatalar"
                  action={
                    <span className="text-2xs text-ink-3">
                      {formatCount(data.failures.length)} tür
                    </span>
                  }
                  padded={false}
                >
                  <ul className="divide-y divide-line">
                    {data.failures.map((f) => (
                      <li
                        key={f.message}
                        className="flex items-start justify-between gap-4 px-4 py-2.5"
                      >
                        <span
                          className="min-w-0 flex-1 text-xs text-ink-2"
                          title={f.message}
                        >
                          {readableFailure(f.message)}
                        </span>
                        <Badge tone="danger">{formatCount(f.count)}×</Badge>
                      </li>
                    ))}
                  </ul>
                </Panel>
              ) : (
                /* Hata yoksa yeri boş bırakılmıyor: gün gün döküm buraya
                   geliyor. Tablo isteğe bağlı bir süs değil — grafikteki
                   sarı, açık temada 3:1 kontrastın altında kalıyor ve
                   sayıya renkten bağımsız bir yol her zaman açık olmalı. */
                <Panel title="Gün gün döküm" padded={false}>
                  <div className="max-h-80 overflow-y-auto px-4 py-3">
                    <RunsByDayTable days={data.daily} />
                  </div>
                </Panel>
              )}
            </div>

            <Panel
              title="Model bazında"
              action={
                <span className="text-2xs text-ink-3">
                  {formatCount(data.byModel.length)} model
                </span>
              }
              padded={false}
            >
              <GroupTable rows={data.byModel} totals={data.totals} />
            </Panel>

            {/* Verinin tazeliği. Referanstaki "şu tarihte güncellendi"
                satırının karşılığı ve GERÇEK: sorgu yanıtı ne zaman
                aldığını biliyor. */}
            <p className="flex items-center justify-center gap-1.5 pb-2 text-2xs text-ink-3">
              <IconRefresh className="size-3.5" />
              Veriler{" "}
              {formatRelative(new Date(report.dataUpdatedAt).toISOString())}
              &nbsp;güncellendi
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

/* ── Rakam şeridi ────────────────────────────────────────────────────────── */

/**
 * Şeridin sırası ve GÖRÜNÜMÜ — rakamın kendisi değil.
 *
 * Simge süs değil: on kart yan yana dizildiğinde göz aradığı rakamı etiketi
 * okuyarak değil simgesinden buluyor. Panoda sekiz kart var ve orada aynı
 * simgeler kazanç sağlamadığı için hiç konmuyor.
 */
/*
  Simge tablosu ANAHTARLA eşleşiyor, sıra `RAPOR_KPI`'den geliyor.

  Sıra burada elle yazılıyken `lib/kpi.ts` ile ayrışabiliyordu ve test onu
  göremiyordu. `RAPOR_KPI` `as const` olduğu için bu tablo eksik bir anahtar
  bırakırsa TypeScript derlemede durur.
*/
const GORUNUM: Record<
  (typeof RAPOR_KPI)[number],
  { icon: React.ReactNode; tone: TileTone }
> = {
  runs: { icon: <IconPlay className="size-3.5" />, tone: "accent" },
  prsOpened: { icon: <IconPullRequest className="size-3.5" />, tone: "success" },
  jiraTasks: { icon: <IconComment className="size-3.5" />, tone: "info" },
  tokens: { icon: <IconChip className="size-3.5" />, tone: "series" },
  cost: { icon: <IconCost className="size-3.5" />, tone: "warning" },
  success: { icon: <IconCheck className="size-3.5" />, tone: "success" },
  avgDuration: { icon: <IconPlay className="size-3.5" />, tone: "info" },
  filesChanged: { icon: <IconEdit className="size-3.5" />, tone: "series" },
  linesChanged: { icon: <IconEdit className="size-3.5" />, tone: "accent" },
  pushedBranches: { icon: <IconFolder className="size-3.5" />, tone: "info" },
};

const SERIT: { key: KpiKey; icon: React.ReactNode; tone: TileTone }[] =
  RAPOR_KPI.map((key) => ({ key, ...GORUNUM[key] }));

/**
 * Dönemin on rakamı — her biri yönüyle birlikte.
 *
 * ÖLÇÜLMEYEN HİÇBİR ŞEY YOK. Referans tasarımda "Jira yorum" ve "aktif
 * agent" kutuları vardı; ikisinin de bu üründe karşılığı yok — Jira
 * yorumları sayılmıyor ve agent'lar sürekli çalışan süreçler değil, iş
 * başına açılıp kapanan container'lar. Yerlerine gerçekten ölçülen iki
 * rakam kondu: gönderilen branch ve değişen satır.
 */
function KpiStrip({ data }: { data: ReportSummary }) {
  const cards: StatCardProps[] = SERIT.map(({ key, icon, tone }) => ({
    ...kpi(key, data),
    periodNote: "öncekine göre",
    icon,
    tone,
  }));

  return (
    /* İki sıra beşli. Onunu tek sıraya dizmek karta ~130px bırakırdı ve
       "GÖNDERİLEN BRANCH" gibi etiketler kırpılırdı. */
    <div className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5">
      {cards.map((c) => (
        <StatCard key={c.label} {...c} />
      ))}
    </div>
  );
}

/* ── Token dağılımı ──────────────────────────────────────────────────────── */

/**
 * Model başına token — halka.
 *
 * Referanstaki "Token Kullanımı (Modele Göre)" panosunun karşılığı ve
 * verisi gerçekten var: `byModel[].tokens`.
 *
 * Beşten fazla model varsa kalanı "Diğer" altında toplanıyor: on dilimlik
 * bir halkada renkler birbirinden ayırt edilemez ve grafik okunmaz olur.
 */
function TokenByModel({ data }: { data: ReportSummary }) {
  const COLORS = [
    "var(--color-series)",
    "var(--color-chart-good)",
    "var(--color-accent)",
    "var(--color-chart-other)",
    "var(--color-info)",
  ];

  const sorted = [...data.byModel]
    .filter((g) => g.tokens > 0)
    .sort((a, b) => b.tokens - a.tokens);

  const head = sorted.slice(0, 5);
  const rest = sorted.slice(5);
  const restTokens = rest.reduce((sum, g) => sum + g.tokens, 0);

  const slices = [
    ...head.map((g, i) => ({
      key: g.key,
      label: g.label,
      value: g.tokens,
      color: COLORS[i]!,
    })),
    ...(restTokens > 0
      ? [
          {
            key: "diger",
            label: `Diğer (${rest.length} model)`,
            value: restTokens,
            color: "var(--color-line-strong)",
          },
        ]
      : []),
  ];

  const total = data.totals.promptTokens + data.totals.completionTokens;

  return (
    <Panel
      title="Token kullanımı"
      action={
        <Link href="/models" className={panelLinkClass}>
          Modeller
        </Link>
      }
    >
      {slices.length === 0 ? (
        <p className="text-sm text-ink-3">Bu dönemde token harcanmadı.</p>
      ) : (
        <>
          <Donut
            slices={slices}
            centerValue={formatCompact(total)}
            centerNote="toplam token"
            size={140}
            formatValue={formatCompact}
          />
          <dl className="mt-4 grid grid-cols-2 gap-3 border-t border-line pt-3">
            <Metric
              size="sm"
              label="Girdi"
              value={formatCompact(data.totals.promptTokens)}
            />
            <Metric
              size="sm"
              label="Çıktı"
              value={formatCompact(data.totals.completionTokens)}
            />
          </dl>
        </>
      )}
    </Panel>
  );
}

/* ── Agent performansı ───────────────────────────────────────────────────── */

/**
 * Agent başına üretim tablosu.
 *
 * Öncesinde çubuk listesiydi: tek bir büyüklüğü (iş sayısı) çiziyor,
 * kalanını alt satıra düz metin olarak sıkıştırıyordu. Oysa buradaki soru
 * KARŞILAŞTIRMA — hangi agent daha çok iş yapıyor, hangisi daha pahalı,
 * hangisi daha çok kod değiştiriyor. Karşılaştırma sütunlarla okunur.
 */
function AgentPerformance({ rows }: { rows: ReportGroup[] }) {
  const top = rows.slice(0, 6);

  return (
    <Panel
      title="Agent performansı"
      action={
        <Link href="/agents" className={panelLinkClass}>
          Agent&apos;lar
        </Link>
      }
      padded={false}
    >
      {top.length === 0 ? (
        <p className="px-4 py-3.5 text-sm text-ink-3">Bu dönemde kayıt yok.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-125 text-sm">
            <thead>
              <tr className="border-b border-line bg-raised/60 text-left text-2xs tracking-wide text-ink-3 uppercase">
                <th className="py-2 pl-4 font-medium">Agent</th>
                <th className="py-2 pr-3 text-right font-medium">İş</th>
                <th className="py-2 pr-3 text-right font-medium">Token</th>
                <th className="py-2 pr-3 text-right font-medium">Dosya</th>
                {/* Maliyet sütunu yerine DEĞİŞTİRİLEN KOD SATIRI: bir
                    agent'ın ürettiği işin büyüklüğünü dosya sayısından çok
                    bu anlatıyor. Maliyet ekranda kayıp değil — rakam
                    şeridinde, model tablosunda ve proje kırılımında
                    duruyor. */}
                <th
                  className="py-2 pr-3 text-right font-medium"
                  title="Değiştirilen kod satırı (eklenen + silinen)"
                >
                  Kod satırı
                </th>
                <th className="py-2 pr-4 text-right font-medium">Başarı</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {top.map((g) => {
                const rate = g.runs > 0 ? g.succeeded / g.runs : 0;
                return (
                  <tr key={g.key} className="transition-colors hover:bg-raised">
                    <td className="max-w-0 py-2.5 pl-4">
                      <div className="truncate font-mono text-xs">
                        {g.label}
                      </div>
                    </td>
                    <td className="py-2.5 pr-3 text-right tabular-nums">
                      {formatCount(g.runs)}
                    </td>
                    <td className="py-2.5 pr-3 text-right text-xs tabular-nums text-ink-2">
                      {formatCompact(g.tokens)}
                    </td>
                    <td className="py-2.5 pr-3 text-right text-xs tabular-nums text-ink-2">
                      {formatCompact(g.filesChanged)}
                    </td>
                    <td
                      className="py-2.5 pr-3 text-right text-xs tabular-nums text-ink-2"
                      title={`+${formatCount(g.additions)} −${formatCount(g.deletions)}`}
                    >
                      {formatCompact(g.additions + g.deletions)}
                    </td>
                    {/* Başarı oranı TEK renkli sütun: yorumlanabilir olan
                        yalnızca o. Diğerlerini de boyamak renge sahte bir
                        anlam yüklerdi. */}
                    <td
                      className={`py-2.5 pr-4 text-right font-medium tabular-nums ${
                        rate >= 0.8 ? "text-ok" : "text-warn"
                      }`}
                    >
                      {formatPercent(g.succeeded, g.runs)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </Panel>
  );
}

/* ── Son çalıştırmalar ───────────────────────────────────────────────────── */

function RecentRuns({ runs, loading }: { runs: Run[]; loading: boolean }) {
  return (
    <Panel
      title="Son çalıştırmalar"
      action={
        <Link href="/runs" className={panelLinkClass}>
          Tümü
        </Link>
      }
      padded={false}
    >
      {loading ? (
        <p className="px-4 py-3.5 text-sm text-ink-3">Yükleniyor…</p>
      ) : runs.length === 0 ? (
        <p className="px-4 py-3.5 text-sm text-ink-3">Kayıt yok.</p>
      ) : (
        <ul className="divide-y divide-line">
          {runs.map((r) => (
            <li key={r.id}>
              <Link
                href={`/runs/${r.id}`}
                className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-raised"
              >
                <span className="shrink-0">
                  <RunStatusBadge status={r.status} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-xs">{r.task}</span>
                  <span className="mt-0.5 block truncate font-mono text-2xs text-ink-3">
                    {r.agentSlug} · {r.projectName}
                  </span>
                </span>
                <span className="shrink-0 text-right">
                  <span className="block font-mono text-2xs tabular-nums">
                    {r.costUsd > 0 ? formatMoney(r.costUsd) : "—"}
                  </span>
                  <span className="mt-0.5 block text-2xs text-ink-3">
                    {formatRelative(r.createdAt)}
                  </span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}

/* ── Rakamların dengesi ──────────────────────────────────────────────────── */

/**
 * Hız rakamlarını dengeleyen okuma.
 *
 * Bu pano SÜS DEĞİL, projenin kendi kuralı: hız gösteren bir rakam asla
 * yalnız durmaz (AGENTS.md → Yönetici rakamları). PR sayısının arttığı ama
 * değişiklik boyutunun da büyüdüğü bir dönem ilerleme değil, biriken
 * risktir.
 */
function Balance({ data }: { data: ReportSummary }) {
  const t = data.totals;
  const p = data.previous;

  const usePRs = t.prsOpened > 0 || p.prsOpened > 0;
  const changeUnits = usePRs ? t.prsOpened : t.runsWithCode;
  const linesPerUnit =
    changeUnits > 0 ? Math.round((t.additions + t.deletions) / changeUnits) : 0;

  return (
    <Panel title="Rakamların dengesi">
      <dl className="grid grid-cols-2 gap-x-6 gap-y-4">
        <Metric
          size="sm"
          label="Kod üreten iş"
          value={formatCount(t.runsWithCode)}
        />
        <Metric
          size="sm"
          label={usePRs ? "PR başına satır" : "İş başına satır"}
          value={linesPerUnit > 0 ? formatCount(linesPerUnit) : "—"}
        />
        <Metric
          size="sm"
          label={usePRs ? "PR başına maliyet" : "İş başına maliyet"}
          value={formatPerUnit(
            t.costUsd,
            usePRs ? t.prsOpened : t.succeeded,
            usePRs ? "PR" : "iş",
          )}
        />
        <Metric
          size="sm"
          label="Kullanılan agent"
          value={formatCount(data.byAgent.length)}
        />
      </dl>

      <div className="mt-4 flex flex-wrap gap-x-5 gap-y-2 border-t border-line pt-3 text-2xs text-ink-2">
        <span>
          Başarısız:{" "}
          <strong className="font-medium text-ink">
            {formatCount(t.failed)}
          </strong>
        </span>
        {t.timeout > 0 && (
          <span>
            Zaman aşımı:{" "}
            <strong className="font-medium text-ink">
              {formatCount(t.timeout)}
            </strong>
          </span>
        )}
        {t.cancelled > 0 && (
          <span>
            İptal:{" "}
            <strong className="font-medium text-ink">
              {formatCount(t.cancelled)}
            </strong>
          </span>
        )}
        {t.interrupted > 0 && (
          <span>
            Kesildi:{" "}
            <strong className="font-medium text-ink">
              {formatCount(t.interrupted)}
            </strong>
          </span>
        )}
      </div>

      {/* Bu satır SÜS DEĞİL: "açılan PR" ile "işe yarayan PR" aynı şey değil
          ve sistem aradaki farkı bilmiyor. Bilmediğini söylemek tasarımın
          parçası (spec 012 K4). */}
      {usePRs && (
        <p className="mt-3 flex items-start gap-1.5 text-2xs text-ink-3">
          <IconAlert className="mt-px size-3.5 shrink-0" />
          Açılan PR sayısıdır — birleştirilip birleştirilmediğini bu sistem
          takip etmiyor.
        </p>
      )}
    </Panel>
  );
}

/* ── Model tablosu ───────────────────────────────────────────────────────── */

function GroupTable({
  rows,
  totals,
}: {
  rows: ReportGroup[];
  totals: ReportTotals;
}) {
  if (rows.length === 0) {
    return (
      <p className="px-4 py-3.5 text-sm text-ink-3">Bu dönemde kayıt yok.</p>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-180 text-sm">
        <thead>
          <tr className="border-b border-line bg-raised/60 text-left text-2xs tracking-wide text-ink-3 uppercase">
            <th className="py-2 pl-4 font-medium">Model</th>
            <th className="py-2 pr-4 text-right font-medium">İş</th>
            <th className="py-2 pr-4 text-right font-medium">Başarı</th>
            <th className="py-2 pr-4 text-right font-medium">Maliyet</th>
            <th className="py-2 pr-4 text-right font-medium">Pay</th>
            <th className="py-2 pr-4 text-right font-medium">Token</th>
            <th className="py-2 pr-4 text-right font-medium">Ort. süre</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-line">
          {rows.map((g) => (
            <tr key={g.key} className="transition-colors hover:bg-raised">
              <td className="max-w-0 py-2.5 pl-4">
                <div className="truncate font-mono text-xs">{g.label}</div>
                <div className="mt-0.5 truncate text-2xs text-ink-3">
                  {g.key.split(" / ")[0]}
                </div>
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums">
                {formatCount(g.runs)}
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums">
                {formatPercent(g.succeeded, g.runs)}
              </td>
              <td className="py-2.5 pr-4 text-right font-mono text-xs tabular-nums">
                {formatMoney(g.costUsd)}
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums text-ink-2">
                {formatPercent(g.costUsd, totals.costUsd)}
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums text-ink-2">
                {formatCompact(g.tokens)}
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums text-ink-2">
                {formatDuration(g.avgDurationSec)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/* ── Yardımcı ────────────────────────────────────────────────────────────── */

/**
 * Dönemin tarih aralığı — "4 Ağu – 10 Ağu 2025".
 *
 * Yıl yalnızca SONDA yazılıyor: iki tarih neredeyse her zaman aynı yılda ve
 * yılı iki kez yazmak şeridin en dar yerinde gürültü ediyor.
 */
function formatRange(from: string, to: string): string {
  const f = new Date(from);
  const t = new Date(to);
  if (Number.isNaN(f.getTime()) || Number.isNaN(t.getTime())) return "";

  const day = new Intl.DateTimeFormat("tr-TR", {
    day: "numeric",
    month: "short",
  });
  const full = new Intl.DateTimeFormat("tr-TR", {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
  return `${day.format(f)} – ${full.format(t)}`;
}
