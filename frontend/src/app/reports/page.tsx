"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { readableFailure } from "@/lib/failure";
import { describeError } from "@/lib/errors";
import type { ReportGroup, ReportSummary, ReportTotals } from "@/lib/types";
import { BarList } from "@/components/charts/BarList";
import { Sparkline } from "@/components/charts/Sparkline";
import { CostTrendChart } from "@/components/charts/CostTrendChart";
import {
  RunsByDayChart,
  RunsByDayTable,
} from "@/components/charts/RunsByDayChart";
import {
  changeRatio,
  formatCompact,
  formatCount,
  formatDuration,
  formatMoney,
  formatPercent,
  formatPerUnit,
} from "@/components/charts/format";
import { IconAgent } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Notice,
  PageHeader,
  Section,
  Select,
  Skeleton,
} from "@/components/ui/primitives";

/**
 * Rapor — yönetici özeti.
 *
 * Tek soruya cevap verir: "para nereye gitti, karşılığında ne üretildi, ne
 * kadarı tuttu?" Agent'ın nasıl çalıştırıldığı (arayüz, API, ileride workflow)
 * fark etmez; hepsi aynı çalıştırma geçmişine düştüğü için rapor eksiksizdir.
 */

const PERIODS = [
  { days: 7, label: "7 gün" },
  { days: 30, label: "30 gün" },
  { days: 90, label: "90 gün" },
] as const;

export default function ReportsPage() {
  const [days, setDays] = useState<number>(30);
  const [project, setProject] = useState("");

  const projects = useQuery({ queryKey: ["projects"], queryFn: () => api.projects.list({ limit: 200 }) });

  const report = useQuery({
    queryKey: ["report", days, project],
    queryFn: () => api.reports.summary({ days, project: project || undefined }),
  });

  const data = report.data;

  return (
    <div className="space-y-7">
      <PageHeader
        title="Rapor"
        description={
          data
            ? `${data.days} günlük dönem · ${data.timezone} saatiyle`
            : "Agent'ların ne yaptığını ve neye mal olduğunu özetler."
        }
        actions={
          <div className="flex shrink-0 items-center gap-2">
            {/* Genişlik SARMALAYICIDAN gelir: Select'in kendi sınıfı w-full
                içerir ve dışarıdan verilen bir genişlik onu yenemez. */}
            <div className="w-44 shrink-0">
              <Select
                className="h-8 text-[12px]"
                value={project}
                onChange={(e) => setProject(e.target.value)}
                aria-label="Proje filtresi"
              >
                <option value="">Tüm projeler</option>
                {projects.data?.items.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </Select>
            </div>

            <div className="flex shrink-0 overflow-hidden rounded-lg border border-line">
              {PERIODS.map((p, i) => (
                <button
                  key={p.days}
                  type="button"
                  onClick={() => setDays(p.days)}
                  aria-pressed={days === p.days}
                  className={`h-8 px-3 text-[12px] whitespace-nowrap transition-colors ${
                    i > 0 ? "border-l border-line" : ""
                  } ${
                    days === p.days
                      ? "bg-accent-soft font-medium text-accent"
                      : "bg-surface text-ink-2 hover:bg-raised hover:text-ink"
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>
        }
      />

      {report.isPending && <Skeleton rows={4} />}
      {report.isError && (
        <Notice tone="error" title={describeError(report.error).message}>
          {describeError(report.error).hint}
        </Notice>
      )}

      {data && data.totals.runs === 0 && data.previous.runs === 0 && (
        <EmptyState
          icon={<IconAgent className="size-5" />}
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
        <>
          <Headline data={data} />
          <Charts data={data} />
          <Breakdowns data={data} />
        </>
      )}
    </div>
  );
}

/* ── Başlık rakamları ────────────────────────────────────────────────────── */

/*
 * Kahraman rakam + destekleyici kutucuklar.
 *
 * ÖNCEKİ HALİ TOPLAM MALİYETTİ ve yanlıştı (spec 012). Maliyet bir girdidir;
 * bir yöneticinin ilk sorusu "ne kadar harcadık" değil "karşılığında ne aldık"
 * olur. Üstelik küçük bir maliyet rakamı sistemi değersiz gösteriyordu.
 *
 * Şimdi kahraman rakam ÜRETİLEN İŞ, maliyet ise onun paydası: "PR başına
 * $0,004". Toplam maliyet ekranda duruyor ama küçük puntoda.
 *
 * Dört destek rakamı rastgele seçilmedi — biri sonuç, biri güvenilirlik, biri
 * RİSK, biri birim maliyet. Hız gösteren bir rakam asla yalnız durmaz
 * (spec 012 K3): PR sayısının arttığı ama değişiklik boyutunun da büyüdüğü bir
 * dönem ilerleme değil, biriken risktir.
 */
function Headline({ data }: { data: ReportSummary }) {
  const t = data.totals;
  const p = data.previous;

  // PR açmayan kurulumlarda (akışında PR düğümü olmayan) dev bir sıfır,
  // sistemin çalışmadığı izlenimi verirdi; kahraman rakam tamamlanan işe düşer.
  const usePRs = t.prsOpened > 0 || p.prsOpened > 0;

  const hero = usePRs
    ? {
        label: "Açılan pull request",
        value: formatCount(t.prsOpened),
        current: t.prsOpened,
        previous: p.prsOpened,
        spark: data.daily.map((d) => d.prsOpened),
        sparkLabel: "Günlük açılan PR",
      }
    : {
        label: "Tamamlanan iş",
        value: formatCount(t.succeeded),
        current: t.succeeded,
        previous: p.succeeded,
        spark: data.daily.map((d) => d.succeeded),
        sparkLabel: "Günlük tamamlanan iş",
      };

  // Değişiklik boyutu PR başına hesaplanır; PR yoksa çalıştırma başına.
  const changeUnits = usePRs ? t.prsOpened : t.runsWithCode;
  const linesPerUnit =
    changeUnits > 0 ? Math.round((t.additions + t.deletions) / changeUnits) : 0;

  return (
    <Card>
      <div className="flex flex-wrap items-start justify-between gap-x-8 gap-y-5">
        <div className="min-w-[220px]">
          <div className="text-[12px] tracking-wide text-ink-3 uppercase">
            {hero.label}
          </div>
          <div className="mt-1.5 text-[44px] leading-none font-semibold tracking-[-0.02em]">
            {hero.value}
          </div>
          <Delta
            current={hero.current}
            previous={hero.previous}
            days={data.days}
            upIsGood
          />
        </div>

        {/* Kıvılcım tek soruya cevap verir: artıyor mu? Eksen ve etiket
            bilinçli olarak yok — onlar aşağıdaki tam grafiklerin işi. */}
        {/* Genişlik sınırlı: kıvılcım kartın tamamına yayılınca kahraman
            rakamla yarışıyor ve boş alanı grafik sanılıyor. */}
        <div className="min-w-[160px] flex-1 sm:max-w-[300px]">
          <div className="text-[11px] text-ink-3">{data.days} günlük seyir</div>
          <div className="mt-2">
            <Sparkline values={hero.spark} label={hero.sparkLabel} />
          </div>
        </div>
      </div>

      <dl className="mt-6 grid grid-cols-2 gap-x-8 gap-y-4 border-t border-line pt-5 sm:grid-cols-4">
        <Stat
          label="Jira'dan otomatik"
          value={formatCount(t.jiraTasks)}
          note={t.jiraTasks > 0 ? "task, insan başlatmadan" : "Jira tetikleyici yok"}
        />
        <Stat
          label="Başarı oranı"
          value={formatPercent(t.succeeded, t.runs)}
          note={`${formatCount(t.succeeded)} / ${formatCount(t.runs)} çalıştırma`}
        />
        <Stat
          label={usePRs ? "PR başına değişiklik" : "İş başına değişiklik"}
          value={linesPerUnit > 0 ? `${formatCount(linesPerUnit)} satır` : "—"}
          note={`+${formatCompact(t.additions)} −${formatCompact(t.deletions)} toplam`}
        />
        <Stat
          label={usePRs ? "PR başına maliyet" : "İş başına maliyet"}
          value={formatPerUnit(t.costUsd, usePRs ? t.prsOpened : t.succeeded, usePRs ? "PR" : "iş")}
          note={`toplam ${formatMoney(t.costUsd)}`}
        />
      </dl>

      {/* Bu satır SÜS DEĞİL (spec 012 K4). "Açılan PR" ile "işe yarayan PR"
          aynı şey değil ve sistem aradaki farkı bilmiyor; bilmediğini söylemek
          tasarımın parçası. */}
      {usePRs && (
        <p className="mt-4 border-t border-line pt-3 text-[12px] text-ink-3">
          Açılan PR sayısıdır — birleştirilip birleştirilmediğini, incelemeden
          geçip geçmediğini bu sistem takip etmiyor.
        </p>
      )}

      <div className="mt-4 flex flex-wrap gap-x-6 gap-y-2 border-t border-line pt-4 text-[12px] text-ink-2">
        <SubStat label="Çalıştırma" value={formatCount(t.runs)} />
        <SubStat label="Kod üreten" value={formatCount(t.runsWithCode)} />
        <SubStat label="Gönderilen branch" value={formatCount(t.pushedBranches)} />
        <SubStat label="Ortalama süre" value={formatDuration(t.avgDurationSec)} />
        <SubStat label="Token" value={formatCompact(t.promptTokens + t.completionTokens)} />
        <SubStat label="Başarısız" value={formatCount(t.failed)} />
        {t.timeout > 0 && <SubStat label="Zaman aşımı" value={formatCount(t.timeout)} />}
        {t.cancelled > 0 && <SubStat label="İptal" value={formatCount(t.cancelled)} />}
        {t.interrupted > 0 && <SubStat label="Kesildi" value={formatCount(t.interrupted)} />}
      </div>
    </Card>
  );
}

function Stat({
  label,
  value,
  note,
}: {
  label: string;
  value: string;
  note?: string;
}) {
  return (
    <div>
      <dt className="text-[11px] tracking-wide text-ink-3 uppercase">{label}</dt>
      {/* Büyük tek sayı orantılı rakamlarla yazılır; tabular-nums bu boyutta gevşek görünür. */}
      <dd className="mt-1 text-[22px] leading-none font-semibold tracking-[-0.01em]">
        {value}
      </dd>
      {note && <p className="mt-1 text-[11px] text-ink-3">{note}</p>}
    </div>
  );
}

function SubStat({ label, value }: { label: string; value: string }) {
  return (
    <span>
      {label}: <strong className="font-medium text-ink">{value}</strong>
    </span>
  );
}

/**
 * Önceki dönemle değişim.
 *
 * Yön ile "iyi/kötü" AYNI ŞEY DEĞİLDİR: maliyetin artması kötü, çalıştırmanın
 * artması iyi. Rengi belirleyen bu ikisinin çarpımıdır.
 */
function Delta({
  current,
  previous,
  days,
  upIsGood,
}: {
  current: number;
  previous: number;
  days: number;
  upIsGood: boolean;
}) {
  const ratio = changeRatio(current, previous);
  if (ratio === null) {
    return (
      <p className="mt-2 text-[12px] text-ink-3">
        Önceki {days} günde kayıt yok
      </p>
    );
  }

  const up = ratio > 0;
  const flat = Math.abs(ratio) < 0.005;
  const good = up === upIsGood;

  return (
    <p className="mt-2 text-[12px] text-ink-2">
      <span
        className={
          flat ? "text-ink-3" : good ? "font-medium text-ok" : "font-medium text-danger"
        }
      >
        {flat ? "≈ aynı" : `${up ? "↑" : "↓"} ${formatPercent(Math.abs(ratio), 1)}`}
      </span>{" "}
      önceki {days} güne göre
    </p>
  );
}

/* ── Grafikler ───────────────────────────────────────────────────────────── */

function Charts({ data }: { data: ReportSummary }) {
  // Yığındaki sarı açık temada 3:1 kontrastın altında; sayıya renkten bağımsız
  // bir yol her zaman açık kalmalı.
  const [asTable, setAsTable] = useState(false);

  return (
    <div className="grid gap-5 xl:grid-cols-2">
      <Card>
        <div className="mb-4 flex items-start justify-between gap-3">
          <div>
            <h2 className="text-[15px] font-semibold tracking-[-0.01em]">
              Günlük çalıştırma
            </h2>
            <p className="mt-1 text-[12px] text-ink-2">
              Sonuca göre; kaydı olmayan günler de eksende durur.
            </p>
          </div>
          <Button size="sm" onClick={() => setAsTable((v) => !v)}>
            {asTable ? "Grafik" : "Tablo"}
          </Button>
        </div>

        {asTable ? (
          <RunsByDayTable days={data.daily} />
        ) : (
          <RunsByDayChart days={data.daily} />
        )}
      </Card>

      <Card>
        <div className="mb-4">
          <h2 className="text-[15px] font-semibold tracking-[-0.01em]">
            Günlük maliyet
          </h2>
          <p className="mt-1 text-[12px] text-ink-2">
            Dönem toplamı {formatMoney(data.totals.costUsd)}. Çalıştırma sayısıyla
            aynı eksene konmaz — iki ölçek birbirini yanlış anlatır.
          </p>
        </div>
        <CostTrendChart days={data.daily} />
      </Card>
    </div>
  );
}

/* ── Kırılımlar ──────────────────────────────────────────────────────────── */

function Breakdowns({ data }: { data: ReportSummary }) {
  return (
    <div className="space-y-7">
      <div className="grid gap-5 xl:grid-cols-2">
        <Card>
          <Section
            title="Agent bazında"
            description="Hangi agent ne kadar iş yaptı."
          >
            <BarList
              rows={data.byAgent.map((g) => ({
                key: g.key,
                label: g.label,
                value: g.runs,
                valueLabel: `${formatCount(g.runs)} iş`,
                note: `${formatMoney(g.costUsd)} · ${formatPercent(
                  g.succeeded,
                  g.runs,
                )} başarı · ${formatCompact(g.filesChanged)} dosya`,
              }))}
            />
          </Section>
        </Card>

        <Card>
          <Section title="Proje bazında" description="Emek hangi depoya gitti.">
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
          </Section>
        </Card>
      </div>

      <Card padded={false}>
        <div className="p-5 pb-3">
          <h2 className="text-[15px] font-semibold tracking-[-0.01em]">
            Model bazında
          </h2>
          <p className="mt-1 text-[12px] text-ink-2">
            Hangi model neye mal oldu ve ne kadarı tuttu.
          </p>
        </div>
        <GroupTable rows={data.byModel} totals={data.totals} />
      </Card>

      {data.failures.length > 0 && (
        <Card>
          <Section
            title="Tekrar eden hatalar"
            description="En sık görülen sebepler; ayrıntı çalıştırma kaydında."
          >
            <ul className="divide-y divide-line">
              {data.failures.map((f) => (
                <li
                  key={f.message}
                  className="flex items-start justify-between gap-4 py-2.5 first:pt-0 last:pb-0"
                >
                  <span
                    className="min-w-0 flex-1 text-[13px] text-ink-2"
                    title={f.message}
                  >
                    {readableFailure(f.message)}
                  </span>
                  <Badge tone="danger">{formatCount(f.count)} kez</Badge>
                </li>
              ))}
            </ul>
          </Section>
        </Card>
      )}
    </div>
  );
}

/** Model kırılımı tablosu — sütunlar hizalı okunsun diye tabular rakamlar. */
function GroupTable({
  rows,
  totals,
}: {
  rows: ReportGroup[];
  totals: ReportTotals;
}) {
  if (rows.length === 0) {
    return (
      <p className="px-5 pb-5 text-[13px] text-ink-3">Bu dönemde kayıt yok.</p>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[720px] text-[13px]">
        <thead>
          <tr className="border-y border-line text-left text-[11px] tracking-wide text-ink-3 uppercase">
            <th className="py-2 pl-5 font-medium">Model</th>
            <th className="py-2 pr-4 text-right font-medium">İş</th>
            <th className="py-2 pr-4 text-right font-medium">Başarı</th>
            <th className="py-2 pr-4 text-right font-medium">Maliyet</th>
            <th className="py-2 pr-4 text-right font-medium">Pay</th>
            <th className="py-2 pr-4 text-right font-medium">Token</th>
            <th className="py-2 pr-5 text-right font-medium">Ort. süre</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((g) => (
            <tr key={g.key} className="border-b border-line last:border-0">
              <td className="py-2.5 pl-5">
                <div className="font-mono text-[12px]">{g.label}</div>
                <div className="mt-0.5 text-[11px] text-ink-3">
                  {g.key.split(" / ")[0]}
                </div>
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums">
                {formatCount(g.runs)}
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums">
                {formatPercent(g.succeeded, g.runs)}
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums">
                {formatMoney(g.costUsd)}
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums text-ink-2">
                {formatPercent(g.costUsd, totals.costUsd)}
              </td>
              <td className="py-2.5 pr-4 text-right tabular-nums text-ink-2">
                {formatCompact(g.tokens)}
              </td>
              <td className="py-2.5 pr-5 text-right tabular-nums text-ink-2">
                {formatDuration(g.avgDurationSec)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
