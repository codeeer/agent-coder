"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { readableFailure } from "@/lib/failure";
import { describeError } from "@/lib/errors";
import type { ReportGroup, ReportSummary, ReportTotals } from "@/lib/types";
import { BarList } from "@/components/charts/BarList";
import { StatCard, type StatCardProps } from "@/components/ui/StatCard";
import { CostTrendChart } from "@/components/charts/CostTrendChart";
import {
  RunsByDayChart,
  RunsByDayTable,
} from "@/components/charts/RunsByDayChart";
import {
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
  Segmented,
  Select,
  Skeleton,
  Toolbar,
} from "@/components/ui/primitives";

/**
 * Rapor — yönetici özeti.
 *
 * Tek soruya cevap verir: "para nereye gitti, karşılığında ne üretildi, ne
 * kadarı tuttu?" Agent'ın nasıl çalıştırıldığı (arayüz, API, ileride workflow)
 * fark etmez; hepsi aynı çalıştırma geçmişine düştüğü için rapor eksiksizdir.
 */

const PERIODS = [
  { id: "7", label: "7 gün" },
  { id: "30", label: "30 gün" },
  { id: "90", label: "90 gün" },
] as const;

export default function ReportsPage() {
  const [days, setDays] = useState<(typeof PERIODS)[number]["id"]>("30");
  const [project, setProject] = useState("");

  const projects = useQuery({ queryKey: ["projects"], queryFn: () => api.projects.list({ limit: 200 }) });

  const report = useQuery({
    queryKey: ["report", days, project],
    queryFn: () =>
      api.reports.summary({ days: Number(days), project: project || undefined }),
  });

  const data = report.data;

  return (
    <div className="space-y-5">
      <PageHeader
        title="Rapor"
        description={
          data
            ? `${data.days} günlük dönem · ${data.timezone} saatiyle`
            : "Agent'ların ne yaptığını ve neye mal olduğunu özetler."
        }
      />

      {/* Süzgeçler başlığın DEĞİL listenin üstünde: dönem ve proje seçimi
          sayfanın tamamına uygulanır ve her ekranda aynı yerde durur. */}
      <Toolbar>
        {/* Genişlik SARMALAYICIDAN gelir: Select'in kendi sınıfı w-full
            içerir ve dışarıdan verilen bir genişlik onu yenemez. */}
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

        <Segmented label="Dönem" options={PERIODS} value={days} onChange={setDays} />
      </Toolbar>

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
        <>
          <KpiStrip data={data} />
          <Headline data={data} />
          <Charts data={data} />
          <Breakdowns data={data} />
        </>
      )}
    </div>
  );
}

/* ── KPI şeridi ──────────────────────────────────────────────────────────── */

/**
 * Dönemin tamamını tek bakışta veren rakam şeridi.
 *
 * Öncesinde bu bilgiler kahraman kartının altında tek satırlık düz metin
 * olarak duruyordu ("Çalıştırma: 42 · Kod üreten: 9 · …"): sekiz rakam aynı
 * puntoda, aynı renkte, yan yana. Hiçbiri diğerinden ayrılmıyordu ve
 * hiçbirinin YÖNÜ görünmüyordu — 42 çalıştırma iyi mi kötü mü, artmış mı
 * azalmış mı, okunamıyordu.
 *
 * Her kart üç şey söylüyor: ne olduğu, kaç olduğu, hangi yöne gittiği.
 *
 * ÖLÇÜLMEYEN HİÇBİR ŞEY YOK. Bu tür panolarda sık görülen "birleştirilen PR",
 * "reddedilen PR" ya da "kazanılan süre" gibi rakamlar burada bilerek yok:
 * sistem PR'ın sonrasını takip etmiyor ve "kazanılan süre" ölçülen değil
 * uydurulan bir sayıdır. Ekran, bilmediği şeyi ima etmez (spec 012 K4).
 */
function KpiStrip({ data }: { data: ReportSummary }) {
  const t = data.totals;
  const p = data.previous;
  const d = data.daily;

  /** Günlük başarı oranı — biten kayıtlar üzerinden; süren işler payda dışı. */
  const dailyRate = d.map((x) => {
    const finished = x.runs - x.active;
    return finished > 0 ? (x.succeeded / finished) * 100 : 0;
  });

  const finished = t.runs - t.active;
  const prevFinished = p.runs - p.active;

  /*
   * İlk kart SONUÇ rakamıdır ve şeridin en soldaki, yani en çok okunan yeri.
   *
   * PR düğümü olmayan bir kurulumda "Açılan PR: 0" yazmak, en görünür yere
   * sistemin hiç çalışmadığı izlenimini koymak olurdu. Böyle bir kurulumda
   * sonuç rakamı tamamlanan işe düşer — bu ayrım kahraman karttan devralındı.
   */
  const usePRs = t.prsOpened > 0 || p.prsOpened > 0;

  const note = "öncekine göre";

  const cards: StatCardProps[] = [
    usePRs
      ? {
          label: "Açılan PR",
          value: formatCount(t.prsOpened),
          current: t.prsOpened,
          previous: p.prsOpened,
          spark: d.map((x) => x.prsOpened),
          upIsGood: true,
          periodNote: note,
        }
      : {
          label: "Tamamlanan iş",
          value: formatCount(t.succeeded),
          current: t.succeeded,
          previous: p.succeeded,
          spark: d.map((x) => x.succeeded),
          upIsGood: true,
          periodNote: note,
        },
    {
      label: "Çalıştırma",
      value: formatCount(t.runs),
      current: t.runs,
      previous: p.runs,
      spark: d.map((x) => x.runs),
      upIsGood: true,
      periodNote: note,
    },
    {
      label: "Başarı",
      value: formatPercent(t.succeeded, finished),
      current: finished > 0 ? t.succeeded / finished : 0,
      previous: prevFinished > 0 ? p.succeeded / prevFinished : 0,
      spark: dailyRate,
      upIsGood: true,
      periodNote: note,
    },
    {
      label: "Kod üreten",
      value: formatCount(t.runsWithCode),
      current: t.runsWithCode,
      previous: p.runsWithCode,
      upIsGood: true,
      periodNote: note,
    },
    {
      label: "Değişen dosya",
      value: formatCompact(t.filesChanged),
      current: t.filesChanged,
      previous: p.filesChanged,
      /* Yön nötr: çok dosya değiştirmek ne iyi ne kötü, bağlama bağlı.
         Yeşil/kırmızı boyamak olmayan bir yargı üretirdi. */
      upIsGood: null,
      periodNote: note,
    },
    {
      // Etiket KISA: sekiz kart tek sıraya dizildiğinde "GÖNDERİLEN BRAN…"
      // diye kırpılıyordu.
      label: "Branch",
      value: formatCount(t.pushedBranches),
      current: t.pushedBranches,
      previous: p.pushedBranches,
      upIsGood: true,
      periodNote: note,
    },
    {
      label: "Token",
      value: formatCompact(t.promptTokens + t.completionTokens),
      current: t.promptTokens + t.completionTokens,
      previous: p.promptTokens + p.completionTokens,
      upIsGood: null,
      periodNote: note,
    },
    {
      label: "Maliyet",
      value: formatMoney(t.costUsd),
      current: t.costUsd,
      previous: p.costUsd,
      spark: d.map((x) => x.costUsd),
      /* TEK "aşağısı iyi" kart. Maliyetin artması, ölçek büyürken normaldir;
         asıl yönetilebilir olan birim maliyet ve o aşağıdaki kartta duruyor. */
      upIsGood: false,
      periodNote: note,
    },
  ];

  return (
    /*
      Sekiz kart TEK SIRAYA değil, dörtlü iki sıraya diziliyor.
      İçerik sütunu 1280px; sekize bölününce karta ~140px kalıyor ve o
      genişlikte hem etiket ("GÖNDERİLEN BR…") hem değişim metni ("önceki
      dönemde …") kesiliyordu. Kesilen bir rakam şeridi yoğun değil, bozuk
      görünür. Dörtlüde karta ~290px düşüyor ve hiçbir şey kırpılmıyor.
    */
    /* Ekran genişledikçe sıra sayısı düşer: dar ekranda 2, orta ekranda 4,
       geniş ekranda sekizi tek sıra. Sabit dörtlü bırakmak 1920px'de üst
       yarıyı boşa harcıyordu; sabit sekizli bırakmak 1280px'de etiketleri
       kesiyordu. */
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4 2xl:grid-cols-8">
      {cards.map((c) => (
        <StatCard key={c.label} {...c} />
      ))}
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

  // Değişiklik boyutu PR başına hesaplanır; PR yoksa çalıştırma başına.
  const changeUnits = usePRs ? t.prsOpened : t.runsWithCode;
  const linesPerUnit =
    changeUnits > 0 ? Math.round((t.additions + t.deletions) / changeUnits) : 0;

  return (
    <Card>
      {/*
        Kahraman rakam ve kıvılcımı KPI şeridine taşındı — şeridin ilk kartı.
        Burada tekrarlanması aynı sayıyı iki kez göstermek olurdu.
        Geriye kalan, bu kartın asıl işi: DENGE.

        Dört rakam rastgele seçilmedi (spec 012 K3) — biri sonuç, biri
        güvenilirlik, biri RİSK, biri birim maliyet. Hız gösteren bir rakam
        asla yalnız durmaz: PR sayısının arttığı ama değişiklik boyutunun da
        büyüdüğü bir dönem ilerleme değil, biriken risktir. Şerit hızı
        gösteriyor; bu kart onu dengeleyen okumayı veriyor.
      */}
      {/* Açıklama paragrafı kaldırıldı. Panel başlığı ne olduğunu söylüyor;
          altındaki dört rakam kendini anlatıyor. Bir panonun her kutusuna
          ne işe yaradığını anlatan bir paragraf koymak, panoyu belge yapar. */}
      <h2 className="text-sm font-semibold tracking-[-0.01em]">
        Rakamların dengesi
      </h2>

      <dl className="mt-4 grid grid-cols-2 gap-x-8 gap-y-4 sm:grid-cols-4">
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
        <p className="mt-4 border-t border-line pt-3 text-xs text-ink-3">
          Açılan PR sayısıdır — birleştirilip birleştirilmediğini, incelemeden
          geçip geçmediğini bu sistem takip etmiyor.
        </p>
      )}

      <div className="mt-4 flex flex-wrap gap-x-6 gap-y-2 border-t border-line pt-4 text-xs text-ink-2">
        {/* Çalıştırma, kod üreten, gönderilen branch ve token buradan
            kaldırıldı: dördü de artık KPI şeridinde, kendi yönleriyle
            birlikte duruyor. Burada yalnızca şeritte KARŞILIĞI OLMAYANLAR
            kalıyor — süre ve başarısızlık kırılımı. */}
        <SubStat label="Ortalama süre" value={formatDuration(t.avgDurationSec)} />
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
      <dt className="text-2xs tracking-wide text-ink-3 uppercase">{label}</dt>
      {/* Büyük tek sayı orantılı rakamlarla yazılır; tabular-nums bu boyutta gevşek görünür. */}
      <dd className="mt-1 text-xl leading-none font-semibold tracking-[-0.01em]">
        {value}
      </dd>
      {note && <p className="mt-1 text-2xs text-ink-3">{note}</p>}
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

/* ── Grafikler ───────────────────────────────────────────────────────────── */

function Charts({ data }: { data: ReportSummary }) {
  // Yığındaki sarı açık temada 3:1 kontrastın altında; sayıya renkten bağımsız
  // bir yol her zaman açık kalmalı.
  const [asTable, setAsTable] = useState(false);

  return (
    <div className="grid items-start gap-5 xl:grid-cols-2">
      {/*
        Panel başlıkları TEK SATIR: ad solda, eylem/değer sağda.

        Öncesinde her başlığın altında bir açıklama paragrafı vardı ("Sonuca
        göre; kaydı olmayan günler de eksende durur.", "…iki ölçek birbirini
        yanlış anlatır."). Bunlar TASARIM GEREKÇESİYDİ, kullanıcının bilgisi
        değil — yeri kod yorumu, ekran değil. Panonun her kutusuna kendini
        savunan bir paragraf koymak panoyu belgeye çeviriyordu.
      */}
      <Card>
        <div className="mb-4 flex items-center justify-between gap-3">
          <h2 className="text-sm font-semibold tracking-[-0.01em]">
            Günlük çalıştırma
          </h2>
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
        <div className="mb-4 flex items-center justify-between gap-3">
          <h2 className="text-sm font-semibold tracking-[-0.01em]">
            Günlük maliyet
          </h2>
          {/* Dönem toplamı paragrafın içinde bir cümleydi; artık başlığın
              karşısında bir değer. Aynı bilgi, taranabilir yerde. */}
          <span className="font-mono text-xs tabular-nums text-ink-2">
            {formatMoney(data.totals.costUsd)}
          </span>
        </div>
        <CostTrendChart days={data.daily} />
      </Card>
    </div>
  );
}

/* ── Kırılımlar ──────────────────────────────────────────────────────────── */

function Breakdowns({ data }: { data: ReportSummary }) {
  return (
    /*
      Üç sütunlu bant.

      İkili ızgarada "Proje bazında" iki satır içeriyor, komşusu beş; yanında
      ekranın yarısı kadar ölü alan kalıyordu. Hatalar paneli ise en altta tek
      başına tam genişlik kaplıyordu — oysa içeriği birkaç satırlık metin.
      Üçü aynı bantta yan yana durunca hem boşluk kapanıyor hem de "hangi
      agent / hangi proje / neden başarısız" aynı bakışta okunuyor.
    */
    <div className="space-y-5">
      <div className="grid items-start gap-5 xl:grid-cols-3">
        <Card>
          <Section
            title="Agent bazında"
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
          <Section title="Proje bazında">
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

        {data.failures.length > 0 && (
          <Card>
            <Section
              title="Tekrar eden hatalar"
            >
              <ul className="divide-y divide-line">
                {data.failures.map((f) => (
                  <li
                    key={f.message}
                    className="flex items-start justify-between gap-4 py-2.5 first:pt-0 last:pb-0"
                  >
                    <span
                      className="min-w-0 flex-1 text-sm text-ink-2"
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

      <Card padded={false}>
        <div className="flex items-center justify-between gap-3 p-5 pb-3">
          <h2 className="text-sm font-semibold tracking-[-0.01em]">
            Model bazında
          </h2>
          <span className="text-xs text-ink-3">
            {formatCount(data.byModel.length)} model
          </span>
        </div>
        <GroupTable rows={data.byModel} totals={data.totals} />
      </Card>
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
      <p className="px-5 pb-5 text-sm text-ink-3">Bu dönemde kayıt yok.</p>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[720px] text-sm">
        <thead>
          <tr className="border-y border-line text-left text-2xs tracking-wide text-ink-3 uppercase">
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
                <div className="font-mono text-xs">{g.label}</div>
                <div className="mt-0.5 text-2xs text-ink-3">
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
