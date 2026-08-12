"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import { readableFailure } from "@/lib/failure";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import {
  formatCompact,
  formatCount,
  formatDuration,
  formatMoney,
} from "@/components/charts/format";
import { IconAgent, IconAlert, IconEdit, IconTrash } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Notice,
  PageHeader,
  SearchField,
  Segmented,
  Select,
  Skeleton,
  StatusDot,
  Toolbar,
  formatRelative,
} from "@/components/ui/primitives";
import type { Run } from "@/lib/types";

/**
 * Çalıştırmalar — geçmiş listesi değil, TEŞHİS ekranı.
 *
 * Kullanıcı buraya dört soruyla geliyor: şu an ne koşuyor, aradığım
 * çalıştırma hangisi, ne battı ve NEDEN battı, bu iş ne kadara mal oldu.
 *
 * ÜÇÜNCÜSÜ HİÇ CEVAPLANMIYORDU. Başarısız bir satır yalnızca "başarısız"
 * yazıyordu; sebebi öğrenmek için kaydı açmak gerekiyordu — oysa hata
 * metni liste yanıtında ZATEN geliyor (`Run.error`). Aynı şey token ve
 * değişen dosya için de geçerliydi: veri geliyor, ekranda yok.
 *
 * SÜZGEÇLERİN KAPSAMI FARKLI ve bu bilerek:
 *
 *   · Proje süzgeci UCA gidiyor (`api.runs.list({ project })`), yani tüm
 *     geçmişte süzüyor ve sayfalama sunucuda kalıyor.
 *   · Durum süzgeci uçta YOK (backend `runs.List` yalnızca proje alıyor).
 *     Açık sayfada süzmek işe yaramıyordu: kırk üç kaydın tek başarısızı
 *     onuncu sayfadaysa "Sorunlu" düğmesi ilk sayfada boş dönüyordu — yani
 *     düğme vardı, işlevi yoktu.
 *
 * Durum süzgeci açıkken bu yüzden PENCERE BÜYÜTÜLÜYOR: tek istekte son 200
 * kayıt çekiliyor ve sayfalama süzülmüş liste üzerinde yapılıyor. 200,
 * backend'in üst sınırı (`store.go`: 200'ü aşan limit 50'ye düşürülür).
 * Daha eski kayıtlar taranmıyor ve bu ekranda AÇIKÇA yazılıyor — sessizce
 * kesilen bir liste, olmayan bir tamlık vaat eder.
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

/** Liste ekrana sığmalı; sayfalama sona kaydırılınca görünüyorsa yok demektir. */
const PAGE_SIZES = [10, 25, 50] as const;

/**
 * Durum süzgeci açıkken taranan kayıt sayısı.
 *
 * Backend'in üst sınırı: bunu aşan bir limit 50'ye DÜŞÜRÜLÜR (`runs.List`),
 * yani "daha çok iste" diye büyütmek tam tersini yapar.
 */
const SCAN_LIMIT = 200;

export default function RunsPage() {
  const [filter, setFilter] = useState<(typeof FILTERS)[number]["id"]>("all");
  const [q, setQ] = useState("");
  const [project, setProject] = useState("");
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState<number>(PAGE_SIZES[0]);

  const projects = useQuery({
    queryKey: ["projects"],
    queryFn: () => api.projects.list({ limit: 200 }),
  });

  // Durum süzgeci açıkken tek istekte geniş pencere; kapalıyken sunucu
  // sayfalaması. İkisi aynı uç, yalnızca pencere farklı.
  const filtering = filter !== "all";
  const scanLimit = filtering ? SCAN_LIMIT : limit;
  const scanOffset = filtering ? 0 : offset;

  const { data, isPending, isError, error } = useQuery({
    queryKey: ["runs", scanOffset, scanLimit, project],
    queryFn: () =>
      api.runs.list({
        limit: scanLimit,
        offset: scanOffset,
        project: project || undefined,
      }),
    // Çalışan iş varsa liste kendini tazelesin. Süren bir işin süresi de
    // bu tazelemeyle ilerliyor.
    refetchInterval: (query) =>
      query.state.data?.items.some((r) => isActive(r.status)) ? 3000 : false,
  });

  const matched = useMemo(() => {
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

  // Süzgeç açıkken sayfalama SÜZÜLMÜŞ liste üzerinde; kapalıyken sunucu
  // zaten sayfalamış durumda.
  const items = filtering ? matched.slice(offset, offset + limit) : matched;

  /** Taranan pencere gerçekten kesiliyor mu — yazılacak mı? */
  const truncated = filtering && (data?.total ?? 0) > SCAN_LIMIT;

  return (
    /* Üç bölge: başlık + araç çubuğu üstte, liste ortada kayar, sayfalama
       altta sabit — bkz. `AppShell`'deki kabuk kararı. */
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader
        title="Çalıştırmalar"
        description="Agent çalıştırma geçmişi. Süren işler kendiliğinden tazelenir."
      />

      {data && (data.items.length > 0 || project !== "") && (
        <Toolbar>
          <Segmented
            label="Durum süzgeci"
            options={FILTERS}
            value={filter}
            /* Süzgeç değişince başa dönülür: üçüncü sayfadayken süzmek,
               süzülmüş listenin var olmayan üçüncü sayfasına bakmak olurdu. */
            onChange={(next) => {
              setFilter(next);
              setOffset(0);
            }}
          />

          {/* Proje süzgeci UCA gidiyor — tüm geçmişte süzüyor. Diğer iki
              denetim yalnızca açık sayfada çalışıyor; fark aşağıdaki
              satırda yazılı. */}
          <div className="w-44 shrink-0">
            <Select
              className="h-8 text-xs"
              value={project}
              onChange={(e) => {
                setProject(e.target.value);
                setOffset(0);
                setQ("");
              }}
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

          <SearchField
            className="min-w-45 flex-1 sm:max-w-xs"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Görev, agent veya model ara…"
            aria-label="Çalıştırmalarda ara"
          />

          <div className="ml-auto">
            <Capacity active={data.active} limit={data.concurrencyLimit} />
          </div>
        </Toolbar>
      )}

      {/* Kayan bölge. `min-h-0` olmadan flex öğesi içeriğinden küçülemez ve
          sayfalama ekranın dışına düşerdi. */}
      <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">
        {isPending && <Skeleton rows={3} />}
        {isError && <Notice tone="error">{describeError(error).message}</Notice>}

        {data?.items.length === 0 && project === "" && (
          <EmptyState
            icon={<IconAgent className="size-4" />}
            title="Henüz çalıştırma yok"
            description="Agent'lar sayfasından bir agent seçip projelerinizden biri üzerinde çalıştırın."
          />
        )}

        {data?.items.length === 0 && project !== "" && (
          <Notice>Bu projede çalıştırma yok.</Notice>
        )}

        {data && data.items.length > 0 && matched.length === 0 && (
          <Notice>
            {filtering
              ? `Son ${formatCount(data.items.length)} çalıştırma arasında bu süzgece uyan yok.`
              : "Bu sayfada aramaya uyan çalıştırma yok — sonraki sayfalarda olabilir."}
          </Notice>
        )}

        {items.length > 0 && <RunTable items={items} />}

        {/* Pencerenin kesildiği SÖYLENİYOR. Sessizce kesilen bir liste,
            olmayan bir tamlık vaat eder. */}
        {truncated && (
          <p className="mt-2 px-1 text-2xs text-ink-3">
            Süzgeç son {formatCount(SCAN_LIMIT)} çalıştırma üzerinde çalıştı;
            toplam {formatCount(data!.total)} kayıt var. Daha eskisi için proje
            süzgecini kullanın.
          </p>
        )}
      </div>

      {data && (
        <Pagination
          /* Süzgeç açıkken sayaç SÜZÜLMÜŞ listeyi anlatır: "43 çalıştırma"
             yazarken ekranda bir tane göstermek, sayacı yalancı yapardı. */
          total={filtering ? matched.length : data.total}
          limit={limit}
          offset={offset}
          onChange={(next) => {
            setOffset(next);
            // Sayfa değişince süzgeç DURUR ama arama temizlenir: süzgeç bir
            // niyet ("sorunluları görüyorum"), arama ise o sayfaya ait bir
            // daraltma. Aramayı taşımak, kullanıcıya boş bir sayfa gösterirdi.
            setQ("");
          }}
          pageSize={{
            value: limit,
            options: PAGE_SIZES,
            onChange: (size) => {
              setLimit(size);
              setOffset(0);
            },
          }}
          unit="çalıştırma"
        />
      )}
    </div>
  );
}

/* ── Kapasite ────────────────────────────────────────────────────────────── */

/**
 * Eşzamanlılık göstergesi.
 *
 * Öncesinde yalnızca doluyken bir rozet olarak çıkıyordu ve "0/3" boşken
 * gizleniyordu — gerekçe "boş bir oran kullanıcıya bir şey söylemez"di.
 * Ama bu ekranın asıl sorusu "şu an ne koşuyor"; sıfır da bir cevaptır ve
 * göstergenin YERİNİN sabit olması, doluluğun fark edilmesini sağlıyor.
 *
 * Sınır dolduğunda ton uyarıya dönüyor: yeni başlatılan işler o anda
 * reddediliyor ve bunu bilmek gerekiyor.
 */
function Capacity({ active, limit }: { active: number; limit: number }) {
  const full = limit > 0 && active >= limit;

  return (
    <span
      className={`flex items-center gap-1.5 rounded-lg border px-2 py-1 text-2xs whitespace-nowrap ${
        full
          ? "border-warn/30 bg-warn-soft text-warn"
          : active > 0
            ? "border-accent/25 bg-accent-soft text-accent"
            : "border-line text-ink-3"
      }`}
      title={
        full
          ? "Eşzamanlılık sınırı dolu — yeni işler reddedilir"
          : "Şu an çalışan iş / eşzamanlılık sınırı"
      }
    >
      <StatusDot tone={full ? "warning" : active > 0 ? "accent" : "neutral"} pulse={active > 0} />
      <span className="tabular-nums">
        {active} / {limit}
      </span>
      <span className="hidden sm:inline">{full ? "sınır dolu" : "çalışıyor"}</span>
    </span>
  );
}

/* ── Tablo ───────────────────────────────────────────────────────────────── */

/**
 * Çalıştırma tablosu.
 *
 * Aynı türden onlarca kaydın karşılaştırıldığı bir ekranda satırlar değil
 * SÜTUNLAR okunur; sütunun ne olduğunu söyleyen tek şey de başlığıdır.
 *
 * Dar ekranda tablo yatay kaydırılır, satırlara BÖLÜNMEZ: bölünmüş bir
 * tablo hizayı kaybeder ve hizasını kaybeden bir tablo listeden kötüdür.
 */
function RunTable({ items }: { items: Run[] }) {
  const totals = items.reduce(
    (acc, r) => ({
      cost: acc.cost + r.costUsd,
      tokens: acc.tokens + r.promptTokens + r.completionTokens,
    }),
    { cost: 0, tokens: 0 },
  );

  return (
    <Card padded={false} className="overflow-hidden">
      <div className="overflow-x-auto">
        <table className="w-full min-w-240 text-sm">
          <thead>
            <tr className="border-b border-line bg-raised/60 text-left text-2xs tracking-wide text-ink-3 uppercase">
              <th className="w-36 py-2.5 pl-4 font-medium">Durum</th>
              <th className="py-2.5 font-medium">Görev / Proje</th>
              <th className="w-56 py-2.5 font-medium">Agent / Model</th>
              <th className="w-20 py-2.5 text-right font-medium">Süre</th>
              <th className="w-20 py-2.5 text-right font-medium">Token</th>
              <th className="w-24 py-2.5 text-right font-medium">Maliyet</th>
              <th className="w-28 py-2.5 text-right font-medium">Başlatıldı</th>
              {/* Başlıksız sütun: "Eylem" yazmak her satırda tek bir ikon
                  duran bir sütuna gereksiz bir kademe eklerdi. */}
              <th className="w-14 py-2.5 pr-4">
                <span className="sr-only">Eylemler</span>
              </th>
            </tr>
          </thead>

          <tbody className="divide-y divide-line">
            {items.map((run) => {
              const live = isActive(run.status);
              const tokens = run.promptTokens + run.completionTokens;

              return (
                <tr
                  key={run.id}
                  /* Süren iş, kronolojik listede kaybolmasın diye kendi
                     zeminini taşıyor: bu ekranın en zamana duyarlı satırı. */
                  className={`group transition-colors ${
                    live ? "bg-accent-soft/40 hover:bg-accent-soft" : "hover:bg-raised"
                  }`}
                >
                  <td className="py-2.5 pl-4 align-top">
                    <RunStatusBadge status={run.status} />
                  </td>

                  <td className="max-w-0 py-2.5 pr-4 align-top">
                    {/*
                      Bağlantı SATIRIN TAMAMI değil ilk hücrede: `<tr>` bir
                      bağlantı olamaz ve her hücreye ayrı `<a>` koymak ekran
                      okuyucuya aynı hedefi yedi kez okutur.
                    */}
                    <Link
                      href={`/runs/${run.id}`}
                      className="block truncate font-medium group-hover:text-accent"
                      title={run.task}
                    >
                      {run.task}
                    </Link>

                    <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-2xs text-ink-3">
                      {/* Akış adımıysa hangi akışın parçası olduğu görünmeli;
                          aksi halde tek başına çalıştırılmış gibi okunur. */}
                      {run.workflowName && (
                        <Badge tone="accent">
                          {run.workflowName} · {run.stepName}
                        </Badge>
                      )}
                      <span className="truncate">{run.projectName}</span>
                      <FileSummary run={run} />
                      {run.pushedBranch && (
                        <Badge tone="info">→ {run.pushedBranch}</Badge>
                      )}
                    </div>

                    {/*
                      HATA SATIRDA.

                      Liste yanıtı hata metnini zaten taşıyordu ve ekran onu
                      hiç göstermiyordu: "başarısız" yazan bir satırın sebebini
                      öğrenmek için kaydı açmak gerekiyordu. Bu ekranın işi
                      teşhis; teşhisin ilk adımı burada olmalı.
                    */}
                    {run.error && (
                      <p
                        className="mt-1 flex items-start gap-1.5 text-2xs text-danger"
                        title={run.error}
                      >
                        <IconAlert className="mt-px size-3.5 shrink-0" />
                        <span className="truncate">{readableFailure(run.error)}</span>
                      </p>
                    )}
                  </td>

                  <td className="max-w-0 py-2.5 pr-4 align-top">
                    <div className="truncate font-mono text-xs">{run.agentSlug}</div>
                    <div className="mt-0.5 truncate font-mono text-2xs text-ink-3">
                      {run.modelId}
                    </div>
                  </td>

                  <td className="py-2.5 pr-4 text-right align-top text-xs tabular-nums text-ink-2">
                    {durationOf(run)}
                  </td>

                  <td className="py-2.5 pr-4 text-right align-top text-xs tabular-nums text-ink-2">
                    {tokens > 0 ? formatCompact(tokens) : "—"}
                  </td>

                  <td className="py-2.5 pr-4 text-right align-top font-mono text-xs tabular-nums">
                    {run.costUsd > 0 ? formatMoney(run.costUsd) : "—"}
                  </td>

                  <td className="py-2.5 text-right align-top text-xs text-ink-3">
                    {formatRelative(run.createdAt)}
                  </td>

                  <td className="py-2.5 pr-4 align-top">
                    <DeleteRunButton run={run} />
                  </td>
                </tr>
              );
            })}
          </tbody>

          {/*
            Toplam satırı SÜTUNLARIN ALTINDA.

            Sayfada on iş görünüyor ama neye mal olduğu görünmüyordu.
            Toplamı kenara bir cümle olarak yazmak da olurdu; sütunun altına
            koymak hangi rakamın neyin toplamı olduğunu ayrıca söylemeyi
            gereksiz kılıyor.
          */}
          {/*
            Şerit YAPIŞKAN: tablo kayan bölgenin içinde ve toplam satırı
            listenin sonunda kalsaydı ancak sona kadar kaydıranlar görürdü —
            sayfalamayı listenin ardına koymakla aynı hata.

            Zemin YARI SAYDAM DEĞİL (`bg-raised`, `/60` değil): yapışkan bir
            şeridin altından satırların geçtiği görünürse rakamlar okunmaz.
          */}
          {/*
            Toplam satırı SÜTUNLARIN ALTINDA.

            Sayfada on iş görünüyor ama neye mal olduğu görünmüyordu.
            Toplamı kenara bir cümle olarak yazmak da olurdu; sütunun altına
            koymak hangi rakamın neyin toplamı olduğunu ayrıca söylemeyi
            gereksiz kılıyor.

            YAPIŞKAN YAPILMADI ve bu ölçülerek karara bağlandı: `sticky`'nin
            kapsayıcısı en yakın kaydırma kabıdır, burada da tablonun
            `overflow-x-auto` sarmalayıcısı. O kabın yüksekliği içeriği
            kadar olduğu için `bottom-0`ın tutunacağı bir dip yok — şerit
            hem `<tfoot>`a hem hücrelere verildiğinde denendi, ikisi de
            kıpırdamadı. Tablonun sonundaki toplam satırı zaten alışılmış
            yerinde duruyor.
          */}
          <tfoot>
            <tr className="border-t border-line bg-raised/60 text-2xs">
              <td className="py-2 pl-4 text-ink-3 uppercase" colSpan={4}>
                Bu sayfada {formatCount(items.length)} iş
              </td>
              <td className="py-2 pr-4 text-right font-medium tabular-nums">
                {totals.tokens > 0 ? formatCompact(totals.tokens) : "—"}
              </td>
              <td className="py-2 pr-4 text-right font-mono font-medium tabular-nums">
                {totals.cost > 0 ? formatMoney(totals.cost) : "—"}
              </td>
              <td className="py-2" />
              <td className="py-2 pr-4" />
            </tr>
          </tfoot>
        </table>
      </div>
    </Card>
  );
}

/**
 * Satır içi silme.
 *
 * Düğme, silinemeyen satırlarda GİZLENMİYOR — pasifleşiyor ve sebebi
 * `title`'da yazıyor. Gizlemek kullanıcıya "böyle bir eylem yok" derdi;
 * oysa eylem var, o satırda geçerli değil.
 *
 * Onay satırın kendisinde açılamaz (bir `<tr>` içine şerit sığmaz), bu yüzden
 * düğme iki adımlı: birinci tık soruyu, ikinci tık silmeyi yapar. Aradaki
 * "Vazgeç" için satırın dışına tıklamak yeterli değil, bu yüzden soru
 * durumunda düğme metni açıkça "Silinsin mi?" diyor.
 */
function DeleteRunButton({ run }: { run: Run }) {
  const qc = useQueryClient();
  const [asking, setAsking] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.runs.remove(run.id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["runs"] });
      void qc.invalidateQueries({ queryKey: ["projects"] });
      void qc.invalidateQueries({ queryKey: ["report"] });
    },
    onSettled: () => setAsking(false),
  });

  const engel = isActive(run.status)
    ? "Çalıştırma sürüyor — önce iptal edin"
    : run.workflowRunId !== null
      ? "Bu çalıştırma bir akışın adımı — akış çalışmasını silin"
      : null;

  if (engel) {
    return (
      <Button size="sm" variant="danger" disabled title={engel} aria-label={engel}>
        <IconTrash className="size-4" />
      </Button>
    );
  }

  if (asking) {
    return (
      <div className="flex items-center justify-end gap-1">
        <Button
          size="sm"
          variant="danger"
          disabled={remove.isPending}
          onClick={() => remove.mutate()}
        >
          {remove.isPending ? "…" : "Sil"}
        </Button>
        <Button size="sm" onClick={() => setAsking(false)}>
          Vazgeç
        </Button>
      </div>
    );
  }

  return (
    <Button
      size="sm"
      variant="danger"
      onClick={() => setAsking(true)}
      aria-label="Bu çalıştırmayı sil"
      title={`Sil — ${formatMoney(run.costUsd)} raporlardan düşecek`}
      className="opacity-100 transition-opacity duration-150 sm:opacity-0 sm:group-focus-within:opacity-100 sm:group-hover:opacity-100"
    >
      <IconTrash className="size-4" />
    </Button>
  );
}

/**
 * Değişen dosya özeti.
 *
 * `files` liste yanıtında geliyor ama gösterilmiyordu. "Kod üretti mi"
 * sorusunun en ucuz cevabı bu: dosya sayısı ve satır kırılımı.
 */
function FileSummary({ run }: { run: Run }) {
  if (run.files.length === 0) return null;

  const adds = run.files.reduce((sum, f) => sum + f.additions, 0);
  const dels = run.files.reduce((sum, f) => sum + f.deletions, 0);

  return (
    <span
      className="flex items-center gap-1 whitespace-nowrap"
      title={`${run.files.length} dosya · +${adds} −${dels} satır`}
    >
      <IconEdit className="size-3.5" />
      {run.files.length}
      <span className="text-ok">+{formatCompact(adds)}</span>
      <span className="text-danger">−{formatCompact(dels)}</span>
    </span>
  );
}

/**
 * Çalıştırma süresi.
 *
 * Süren işler için başlangıçtan ŞU ANA kadar; bitenler için gerçek süre.
 * Bitmemiş bir işe tire koymak, ekranda en çok merak edilen sayıyı
 * gizlemek olurdu. Süren işlerde liste üç saniyede bir tazelendiği için
 * bu sayı da ilerliyor.
 */
function durationOf(run: Run): string {
  if (!run.startedAt) return "—";
  const end = run.finishedAt ? new Date(run.finishedAt) : new Date();
  const seconds = (end.getTime() - new Date(run.startedAt).getTime()) / 1000;
  return seconds > 0 ? formatDuration(seconds) : "—";
}
