"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api";
import {
  PAGE_SIZE,
  readView,
  writeView,
  type ProjectView,
} from "@/lib/project-view";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import type {
  GitProvider,
  Project,
  ReportGroup,
  Run,
  Workflow,
} from "@/lib/types";
import { repoLabel } from "@/components/projects/repo-url";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import {
  formatCount,
  formatMoney,
  formatPercent,
} from "@/components/charts/format";
import {
  IconEdit,
  IconFolder,
  IconAlert,
  IconGrid,
  IconPlus,
  IconRows,
  IconWorkflow,
  IconTrash,
} from "@/components/ui/icons";
import {
  Badge,
  Button,
  EmptyState,
  IconTile,
  Input,
  ConfirmStrip,
  Metric,
  Notice,
  RowAction,
  PageHeader,
  Panel,
  SearchField,
  Segmented,
  Select,
  Skeleton,
  Toolbar,
  formatRelative,
  toneFromKey,
} from "@/components/ui/primitives";

/**
 * Projeler — depo kaydı değil, BAĞLANTI SAĞLIĞI ekranı.
 *
 * Kullanıcı buraya iki nedenle geliyor: yeni bir depo bağlamak (riskli an —
 * adres ve erişim kaydetmeden önce gerçekten sınanıyor) ve "bu depo doğru
 * bağlı mı, kullanılıyor mu?" sorusuna bakmak.
 *
 * ÖNCEKİ HALİ İKİNCİ SORUYU HİÇ CEVAPLAMIYORDU. Her satır tek puntoda dört
 * kavramlık bir metin zinciriydi (`adres · branch · "3 gün önce eklendi" ·
 * "12 çalıştırma"`); taranamıyordu. `runCount` vardı ama zaman bağlamı
 * yoktu — 12 çalıştırma bu hafta mı, altı ay önce mi, proje ölü mü canlı
 * mı, ekrandan anlaşılmıyordu. Maliyet ve başarı oranı `reports.byProject`
 * içinde zaten duruyordu ve hiç gösterilmiyordu.
 *
 * Şimdi kart ızgarası: proje sayısı azdır ve her kart depo kimliğini,
 * bağlantı biçimini ve son 30 günün etkinliğini birlikte taşır. Düzenleme
 * kartın kendi hücresinde açılır — ızgara yerinden oynamaz.
 */


/** Etkinlik rakamlarının dönemi. Agent'lar ekranıyla aynı. */
const USAGE_DAYS = 30;

/*
 * Görünüm seçenekleri.
 *
 * Etiketler METİN DEĞİL İKON: araç çubuğunda arama ve süzgeç zaten yer
 * kaplıyor, "Liste" ve "Kart" kelimeleri satırı taşırıyordu. Ekran okuyucu
 * ve fare kullanıcısı için `title` metni duruyor; etkin görünüm ayrıca
 * zeminle işaretli, yani renk tek kanal değil.
 */
const VIEWS = [
  { id: "list", label: <IconRows className="size-4" />, title: "Liste görünümü" },
  { id: "card", label: <IconGrid className="size-4" />, title: "Kart görünümü" },
] as const;

const FILTERS = [
  { id: "all", label: "Tümü" },
  { id: "auth", label: "Kimlikli" },
  { id: "public", label: "Açık depo" },
] as const;

type FilterId = (typeof FILTERS)[number]["id"];

export default function ProjectsPage() {
  const [adding, setAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [offset, setOffset] = useState(0);

  /*
   * Görünüm tercihi.
   *
   * İlk çizim SUNUCUDA yapılıyor ve orada `localStorage` yok; bu yüzden
   * varsayılanla başlanır ve kaydedilmiş tercih ilk efektle uygulanır. Tema
   * anahtarındaki kalıbın aynısı — aksi halde sunucu ve istemci çıktısı
   * uyuşmaz ve React uyarı basar.
   */
  const [view, setView] = useState<ProjectView>("list");
  useEffect(() => setView(readView()), []);
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<FilterId>("all");

  const projects = useQuery({
    queryKey: ["projects", offset, view],
    queryFn: () => api.projects.list({ limit: PAGE_SIZE[view], offset }),
  });
  const gitProviders = useQuery({
    queryKey: ["git-providers"],
    queryFn: api.gitProviders.list,
  });

  /*
   * Etkinlik rakamları raporun kendi ucundan — yeni bir uç YOK.
   * `byProject` satırlarının anahtarı proje kimliği (backend:
   * `r.project_id::text`), yani listeyle doğrudan eşleşiyor.
   */
  const report = useQuery({
    queryKey: ["report", "projects", USAGE_DAYS],
    queryFn: () => api.reports.summary({ days: USAGE_DAYS }),
  });

  // Son çalıştırma penceresi: her projenin en son işi bundan çıkarılıyor.
  const runs = useQuery({
    queryKey: ["runs", "projects-panel"],
    queryFn: () => api.runs.list({ limit: 100 }),
    refetchInterval: (query) =>
      query.state.data?.items.some((r) => isActive(r.status)) ? 5000 : false,
  });

  // Akış sayısı için tek istek; kart başına sorgu atmak N istek ederdi.
  const workflows = useQuery({
    queryKey: ["workflows", "projects-panel"],
    queryFn: () => api.workflows.list({ limit: 200 }),
  });

  const items = useMemo(() => {
    const rows = projects.data?.items ?? [];
    const needle = q.trim().toLocaleLowerCase("tr");
    return rows.filter((p) => {
      const authed = p.gitProviderId !== null;
      if (filter === "auth" && !authed) return false;
      if (filter === "public" && authed) return false;
      return (
        needle === "" ||
        [p.name, p.repoUrl, p.defaultBranch].some((v) =>
          v.toLocaleLowerCase("tr").includes(needle),
        )
      );
    });
  }, [projects.data, filter, q]);

  /** Projeye ait son çalıştırma — liste zamana göre sıralı geldiği için ilki. */
  const lastRunOf = (projectId: string): Run | undefined =>
    runs.data?.items.find((r) => r.projectId === projectId);

  const total = projects.data?.total ?? 0;

  return (
    /* `min-w-0`: bu kap, kenar çubuğunun yanındaki esnek satırın çocuğu.
       Varsayılan `min-width: auto` ile içeriğinden dar olmayı reddediyor ve
       tablonun `min-w-240`'ı SAYFA GÖVDESİNİ yatay kaydırıyordu. Kayma
       yalnızca tablo kabına ait olmalı (spec 019). */
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <PageHeader
        title="Projeler"
        description="Agent'ların üzerinde çalışacağı kod depoları. Bir kez tanımlanır, her çalıştırmada listeden seçilir."
        actions={
          !adding && (
            <Button
              variant="primary"
              onClick={() => {
                setAdding(true);
                setEditingId(null);
              }}
              icon={<IconPlus className="size-4" />}
            >
              Proje ekle
            </Button>
          )
        }
      />

      {projects.isPending && <Skeleton rows={2} />}
      {projects.isError && (
        <Notice tone="error">{describeError(projects.error).message}</Notice>
      )}

      {/* Ekleme formu ızgaranın ÜSTÜNDE: yeni bir kaydın ızgarada yeri
          yok ve bir hücreye sıkıştırılınca doğrulama hataları için yer
          kalmıyordu. */}
      {adding && (
        <div className="mb-4">
          <ProjectForm
            gitProviders={gitProviders.data ?? []}
            onDone={() => setAdding(false)}
          />
        </div>
      )}

      {total === 0 && !adding && projects.data && (
        <EmptyState
          icon={<IconFolder className="size-4" />}
          title="Henüz proje yok"
          description="Bir agent çalıştırabilmek için önce üzerinde çalışacağı depoyu tanımlayın. Kaydetmeden önce depoya erişilebildiği sınanır."
          action={
            <Button variant="primary" onClick={() => setAdding(true)}>
              İlk projeyi ekle
            </Button>
          }
        />
      )}

      {total > 0 && (
        <>
          {/* Araç çubuğu yalnızca aranacak kadar kayıt varken: iki projeli
              bir kurulumda arama kutusu, olmayan bir yığını ima ederdi. */}
          {total > 3 && (
            <Toolbar>
              <SearchField
                className="min-w-50 flex-1 sm:max-w-xs"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="Proje, depo veya branch ara…"
                aria-label="Projelerde ara"
              />
              <Segmented
                label="Erişim süzgeci"
                options={FILTERS}
                value={filter}
                onChange={setFilter}
              />
              {/* Görünüm anahtarı arama ve süzgeçle AYNI YERDE: ui.md
                  araç çubuğunu "arama, süzgeç, görünüm anahtarı tek yerde"
                  diye tanımlıyor. Etkin görünüm hem zeminle hem ikonla
                  belli — renk tek kanal değil. */}
              <Segmented
                label="Görünüm"
                options={VIEWS}
                value={view}
                onChange={(v) => {
                  setView(v);
                  writeView(v);
                  // Sayfa boyutu görünüme göre değişiyor; eski offset yeni
                  // boyutta anlamsız (hatta toplamın ötesinde) olabilir.
                  setOffset(0);
                }}
              />
              <span className="ml-auto hidden text-2xs text-ink-3 lg:block">
                {items.length === projects.data?.items.length
                  ? `${items.length} proje`
                  : `${items.length} / ${projects.data?.items.length} proje`}
              </span>
            </Toolbar>
          )}

          {/* `min-w-0`: esnek kutu çocuğu varsayılan olarak içeriğinden dar
              olmayı reddeder. Onsuz tablonun `min-w-240`'ı SAYFA GÖVDESİNİ
              yatay kaydırıyordu; kaymanın yalnızca tablo kabına ait olması
              gerekiyor (spec 019). */}
          <div className="-mx-1 min-h-0 min-w-0 flex-1 overflow-y-auto px-1 pb-1">
            {items.length === 0 ? (
              /*
               * Boş sonuç NEDEN boş olduğunu söylemeli.
               *
               * Önceki metin her durumda "Bu süzgece uyan proje yok" diyordu —
               * kullanıcı bir kelime ARADIYSA yanlış yeri gösteriyordu:
               * süzgeç "Tümü"de dururken sorun aramadaydı. Hangi ölçütün
               * sonuç vermediği ve nasıl geri dönüleceği yazılıyor.
               */
              <Notice>
                {q.trim() && filter !== "all"
                  ? `“${q.trim()}” aramasına ve seçili erişim süzgecine birlikte uyan proje yok. Aramayı temizleyin veya süzgeci “Tümü” yapın.`
                  : q.trim()
                    ? `“${q.trim()}” aramasına uyan proje yok. Başka bir kelime deneyin veya aramayı temizleyin.`
                    : "Seçili erişim süzgecine uyan proje yok. Süzgeci “Tümü” yaparak hepsini görebilirsiniz."}
              </Notice>
            ) : view === "card" ? (
              /* `items-start`: kartlar satır boyunca eşit yüksekliğe
                 ÇEKİLMEZ. Çekilseydi bir kartın silme onayıyla büyümesi
                 komşusunun ortasında boşluk açardı — nitekim açtı. */
              <ul className="grid items-start gap-4 md:grid-cols-2 2xl:grid-cols-3">
                {items.map((p) =>
                  editingId === p.id ? (
                    <li key={p.id} className="col-span-full">
                      <ProjectForm
                        project={p}
                        gitProviders={gitProviders.data ?? []}
                        onDone={() => setEditingId(null)}
                      />
                    </li>
                  ) : (
                    <li key={p.id}>
                      <ProjectCard
                        project={p}
                        provider={gitProviders.data?.find((g) => g.id === p.gitProviderId)}
                        usage={report.data?.byProject.find((g) => g.key === p.id)}
                        lastRun={lastRunOf(p.id)}
                        workflows={
                          workflows.data?.items.filter((w) => w.projectId === p.id) ?? []
                        }
                        onEdit={() => {
                          setEditingId(p.id);
                          setAdding(false);
                        }}
                      />
                    </li>
                  ),
                )}
              </ul>
            ) : (
              /*
               * IZGARA DEĞİL TABLO — ve bu ölçülmüş bir karar.
               *
               * Kart düzeninde bir proje 227 piksel yer kaplıyordu; 665
               * piksellik alanda altı proje görünüyor, sayfa boyutu ise 24
               * idi. Yani bir sayfa dolduğunda sayfalama denetimine varmak
               * için dört ekran boyu kaydırmak gerekiyordu: kaydırma,
               * sayfalamanın yerine geçmişti (spec 019).
               *
               * Asıl kayıp yer değil KARŞILAŞTIRMA: kartlarda aynı bilgi her
               * kartın farklı yerinde duruyor, iki projenin branch'ini yan
               * yana görmek mümkün olmuyordu. Sütun hizası bunu çözüyor.
               *
               * Kalıp ÜRÜNÜN KENDİSİNDEN geliyor (Çalıştırmalar ekranı):
               * yatay kaydırılabilir gerçek tablo, `divide-y` satırlar,
               * sağa hizalı üstveri sütunları, başlıksız eylem sütunu.
               */
              <div className="overflow-hidden rounded-card border border-line bg-surface shadow-(--shadow-card)">
                <div className="overflow-x-auto">
                  {/* `table-fixed`: sütun genişlikleri BAĞLAYICI olsun diye. Otomatik
                      düzende hücre içeriğe göre genişliyor ve `truncate` hiç
                      devreye girmiyordu — uzun bir depo yolu komşu sütunun
                      üstüne taşıyordu. */}
                  <table className="w-full min-w-240 table-fixed text-sm">
                    <thead>
                      <tr className="border-b border-line bg-raised/60 text-left text-2xs tracking-wide text-ink-3 uppercase">
                        <th className="py-2.5 pl-4 font-medium">Proje</th>
                        <th className="w-64 py-2.5 font-medium">Erişim</th>
                        <th className="w-40 py-2.5 font-medium">Akışlar</th>
                        <th className="w-52 py-2.5 font-medium">Kullanım (30g)</th>
                        <th className="w-44 py-2.5 font-medium">Son çalışma</th>
                        {/* Başlıksız: her satırda iki ikon duran bir sütuna
                            "Eylemler" yazmak gereksiz bir kademe eklerdi. */}
                        <th className="w-20 py-2.5 pr-4">
                          <span className="sr-only">Eylemler</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-line">
                {items.map((p) =>
                  editingId === p.id ? (
                    /* Düzenleme SATIRIN YERİNDE, tüm sütunları kaplayarak
                       açılıyor: listenin altına ya da üstüne taşınsaydı hangi
                       projeyi düzenlediğiniz kaybolurdu. */
                    <tr key={p.id}>
                      <td colSpan={6} className="p-4">
                        <ProjectForm
                          project={p}
                          gitProviders={gitProviders.data ?? []}
                          onDone={() => setEditingId(null)}
                        />
                      </td>
                    </tr>
                  ) : (
                    <ProjectRow
                      key={p.id}
                        project={p}
                        provider={gitProviders.data?.find(
                          (g) => g.id === p.gitProviderId,
                        )}
                        usage={report.data?.byProject.find(
                          (g) => g.key === p.id,
                        )}
                        lastRun={lastRunOf(p.id)}
                        workflows={
                          workflows.data?.items.filter(
                            (w) => w.projectId === p.id,
                          ) ?? []
                        }
                        onEdit={() => {
                          setEditingId(p.id);
                          setAdding(false);
                        }}
                      />
                  ),
                )}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </div>

          <Pagination
            total={total}
            limit={projects.data?.limit ?? PAGE_SIZE[view]}
            offset={projects.data?.offset ?? 0}
            onChange={(next) => {
              setOffset(next);
              setQ("");
              setEditingId(null);
            }}
            unit="proje"
          />
        </>
      )}
    </div>
  );
}

/* ── Kart ────────────────────────────────────────────────────────────────── */

function ProjectCard({
  project,
  provider,
  usage,
  lastRun,
  workflows,
  onEdit,
}: {
  project: Project;
  provider?: GitProvider;
  usage?: ReportGroup;
  lastRun?: Run;
  workflows: Workflow[];
  onEdit: () => void;
}) {
  const queryClient = useQueryClient();
  const [confirming, setConfirming] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.projects.remove(project.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
      setConfirming(false);
    },
  });

  const repo = repoLabel(project.repoUrl);

  return (
    /* `group`: ikincil eylemler hover ve klavye odağında açılıyor. Kart
       başına iki düğme × yirmi dört kart, ekranda sürekli duran kırk sekiz
       düğme ederdi. Gizlenmiyorlar, ERTELENİYORLAR — dokunmatik cihazda
       (hover yok) `sm` altında hep açıklar. */
    <div className="group flex h-full flex-col rounded-card border border-line bg-surface shadow-(--shadow-card) transition-colors hover:border-line-strong">
      <div className="flex items-start gap-3 p-4">
        <IconTile tone={toneFromKey(project.id)}>
          <IconFolder className="size-4" />
        </IconTile>

        <div className="min-w-0 flex-1">
          <h2 className="truncate text-sm font-semibold tracking-[-0.01em]">
            {project.name}
          </h2>

          {/*
            Depo kimliği `kullanici/depo`, sunucu adı onun altında.
            Öncesinde ham URL yazılıyordu ve dar bir sütunda kırpılınca en
            çok yeri `https://` kaplıyordu — yani en az bilgi taşıyan parça.
          */}
          <p
            className="mt-0.5 truncate font-mono text-xs text-ink-2"
            title={project.repoUrl}
          >
            {repo.path}
          </p>
        </div>

        <div className="flex shrink-0 items-center gap-1 opacity-100 transition-opacity duration-150 sm:opacity-0 sm:group-focus-within:opacity-100 sm:group-hover:opacity-100">
          <Button
            size="sm"
            onClick={onEdit}
            icon={<IconEdit className="size-4" />}
            aria-label={`${project.name} projesini düzenle`}
          >
            <span className="sr-only sm:not-sr-only">Düzenle</span>
          </Button>
          <Button
            size="sm"
            variant="danger"
            onClick={() => setConfirming(true)}
            aria-label={`${project.name} projesini sil`}
          >
            <IconTrash className="size-4" />
          </Button>
        </div>
      </div>

      {/* ── Bağlantı ── */}
      <div className="flex flex-wrap items-center gap-1.5 px-4 pb-3">
        <Badge title={`Varsayılan branch: ${project.defaultBranch}`}>
          {project.defaultBranch}
        </Badge>
        {repo.host && <Badge>{repo.host}</Badge>}
        {provider ? (
          <Badge
            tone={provider.verified ? "info" : "warning"}
            title={
              provider.verified
                ? `${provider.name} kimliğiyle klonlanır`
                : `${provider.name} — erişim doğrulanamadan kaydedilmiş`
            }
          >
            {provider.name}
          </Badge>
        ) : (
          <Badge title="Kimlik doğrulaması olmadan klonlanır">açık depo</Badge>
        )}
      </div>

      {/*
        Akışlar SAYIYLA değil ADLARIYLA.

        "3 akış" bir sayaçtır ve kullanıcının sorusuna cevap vermez: bu
        depoda hangi otomasyonlar var? Adlar hem cevabı verir hem de
        doğrudan tıklanabilir. Üçten fazlası sığmaz; kalanı sayıyla
        belirtilip listeye bırakılıyor.
      */}
      {workflows.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 px-4 pb-3">
          <IconWorkflow className="size-3.5 shrink-0 text-ink-3" />
          {workflows.slice(0, 3).map((w) => (
            <Link
              key={w.id}
              href={`/workflows/${w.id}`}
              className="max-w-40 truncate rounded-md border border-line bg-raised px-1.5 py-px text-2xs text-ink-2 transition-colors hover:border-accent/40 hover:text-accent"
              title={w.name}
            >
              {w.name}
            </Link>
          ))}
          {workflows.length > 3 && (
            <Link
              href="/workflows"
              className="rounded text-2xs text-ink-3 transition-colors hover:text-accent"
            >
              +{workflows.length - 3}
            </Link>
          )}
        </div>
      )}

      {/* ── Etkinlik ── */}
      <div className="mt-auto border-t border-line px-4 py-3">
        {usage ? (
          <dl className="grid grid-cols-3 gap-3">
            <Metric
              label={`Çalıştırma (${USAGE_DAYS}g)`}
              value={formatCount(usage.runs)}
            />
            <Metric
              label="Başarı"
              value={formatPercent(usage.succeeded, usage.runs)}
              tone={usage.succeeded / usage.runs >= 0.8 ? "ok" : "warn"}
            />
            <Metric label="Maliyet" value={formatMoney(usage.costUsd)} />
          </dl>
        ) : (
          /* Son 30 günde kayıt yoksa TOPLAM sayı gösteriliyor: proje eski
             ama kullanılmış olabilir. "0" yazmak, hiç çalıştırılmamış bir
             projeyi başarısız gibi gösterirdi. */
          <p className="text-xs text-ink-3">
            Son {USAGE_DAYS} günde çalıştırma yok
            {project.runCount > 0 &&
              ` · toplam ${formatCount(project.runCount)} çalıştırma`}
          </p>
        )}

        <div className="mt-2.5 flex items-center gap-2 text-2xs text-ink-3">
          {lastRun ? (
            <>
              <RunStatusBadge status={lastRun.status} />
              <span className="truncate">
                son çalışma {formatRelative(lastRun.createdAt)}
              </span>
            </>
          ) : (
            <span>{formatRelative(project.createdAt)} eklendi</span>
          )}
        </div>
      </div>

      {/* ── Silme onayı ── */}
      {confirming && (
        <div className="border-t border-danger/30 bg-danger-soft px-4 py-3">
          <p className="flex items-start gap-2 text-xs text-ink">
            <IconAlert className="mt-px size-4 shrink-0 text-danger" />
            <span>
              {project.runCount > 0 ? (
                <>
                  <strong>
                    {formatCount(project.runCount)} çalıştırma geçmişi
                  </strong>{" "}
                  de silinecek. Bu geri alınamaz.
                </>
              ) : (
                <>Bu proje silinsin mi?</>
              )}
            </span>
          </p>
          <div className="mt-2.5 flex gap-2">
            <Button
              size="sm"
              variant="danger"
              onClick={() => remove.mutate()}
              disabled={remove.isPending}
            >
              {remove.isPending ? "Siliniyor…" : "Evet, sil"}
            </Button>
            <Button size="sm" onClick={() => setConfirming(false)}>
              Vazgeç
            </Button>
          </div>
          {remove.isError && (
            <p className="mt-2 text-xs text-danger">
              {describeError(remove.error).message}
            </p>
          )}
        </div>
      )}
    </div>
  );
}


/**
 * Bir projenin liste satırı.
 *
 * KART DEĞİL SATIR — ve hiçbir bilgi kaybedilmedi. Kartta beş yatay bant
 * hâlinde duran sekiz alan (ad, depo, branch, sunucu, git erişimi, akışlar,
 * otuz günlük ölçüler, son çalışma) burada sütunlara dağılıyor. Kazanç yerden
 * çok KARŞILAŞTIRMADAN geliyor: kartlarda iki projenin branch'ini yan yana
 * görmek mümkün değildi (spec 019).
 */
function ProjectRow({
  project,
  provider,
  usage,
  lastRun,
  workflows,
  onEdit,
}: {
  project: Project;
  provider?: GitProvider;
  usage?: ReportGroup;
  lastRun?: Run;
  workflows: Workflow[];
  onEdit: () => void;
}) {
  const queryClient = useQueryClient();
  const [confirming, setConfirming] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.projects.remove(project.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
      setConfirming(false);
    },
  });

  const repo = repoLabel(project.repoUrl);

  return (
    <>
      <tr className="group transition-colors hover:bg-raised">
        {/* ── Proje: ad + depo yolu ── */}
        <td className="py-2 pl-4">
          <div className="flex items-center gap-2.5">
            {/* `sm`: bileşenin kendi belgesi bu boyutu "liste satırı içi"
                diye tanımlıyor. `md` (36px) satırı 53 pikselde tutuyordu ve
                ekrana sığan satır sayısını belirleyen tek şey oydu. */}
            <IconTile tone={toneFromKey(project.id)} size="sm">
              <IconFolder className="size-3.5" />
            </IconTile>
            <div className="min-w-0">
              <div className="truncate leading-tight font-medium" title={project.name}>
                {project.name}
              </div>
              {/* Depo kimliği `kullanici/depo`; ham URL'de en çok yeri
                  `https://` kaplıyordu — yani en az bilgi taşıyan parça. */}
              <div
                className="truncate font-mono text-2xs leading-tight text-ink-2"
                title={project.repoUrl}
              >
                {repo.path}
              </div>
            </div>
          </div>
        </td>

        {/* ── Erişim: bu depo NASIL klonlanır ── */}
        <td className="py-2">
          <div className="flex items-center gap-1.5 overflow-hidden">
            <Badge title={`Varsayılan branch: ${project.defaultBranch}`}>
              {project.defaultBranch}
            </Badge>
            {repo.host && <Badge>{repo.host}</Badge>}
            {provider ? (
              <Badge
                tone={provider.verified ? "info" : "warning"}
                title={
                  provider.verified
                    ? `${provider.name} kimliğiyle klonlanır`
                    : `${provider.name} — erişim doğrulanamadan kaydedilmiş`
                }
              >
                {/* Renk TEK KANAL DEĞİL: doğrulanmamış erişim rozetin
                    metninde de belli olur. */}
                {provider.verified ? provider.name : `${provider.name} · doğrulanmadı`}
              </Badge>
            ) : (
              <Badge title="Kimlik doğrulaması olmadan klonlanır">açık depo</Badge>
            )}
          </div>
        </td>

        {/* ── Akışlar: SAYIYLA DEĞİL ADLARIYLA ──
            "3 akış" bir sayaçtır ve "bu depoda hangi otomasyonlar var"
            sorusuna cevap vermez. Sütuna ikisi sığar; kalanı listeye bırakılır. */}
        <td className="py-2">
          {workflows.length === 0 ? (
            <span className="text-2xs text-ink-3">—</span>
          ) : (
            <div className="flex flex-wrap items-center gap-1.5">
              {workflows.slice(0, 2).map((w) => (
                <Link
                  key={w.id}
                  href={`/workflows/${w.id}`}
                  className="max-w-32 truncate rounded-md border border-line bg-raised px-1.5 py-px text-2xs text-ink-2 transition-colors hover:border-accent/40 hover:text-accent"
                  title={w.name}
                >
                  {w.name}
                </Link>
              ))}
              {workflows.length > 2 && (
                <Link
                  href="/workflows"
                  className="rounded text-2xs text-ink-3 transition-colors hover:text-accent"
                  title={workflows.slice(2).map((w) => w.name).join(", ")}
                >
                  +{workflows.length - 2}
                </Link>
              )}
            </div>
          )}
        </td>

        {/* ── Kullanım: ÜÇ KUTU DEĞİL TEK SATIR ──
            Bilgi korunuyor (spec 019: hiçbir alan kaldırılmaz), yalnızca
            kartın en büyük dikey payını kaplayan üç ölçü kutusu sessiz bir
            üstveri satırına iniyor. */}
        <td className="py-2">
          {usage ? (
            <span className="text-2xs whitespace-nowrap text-ink-2">
              {formatCount(usage.runs)} çalıştırma
              {" · "}
              <span className={usage.succeeded / usage.runs >= 0.8 ? "text-ok" : "text-warn"}>
                {formatPercent(usage.succeeded, usage.runs)} başarı
              </span>
              {" · "}
              {formatMoney(usage.costUsd)}
            </span>
          ) : (
            /* "0" YAZILMAZ: hiç çalıştırılmamış bir projeyi başarısız gibi
               gösterirdi. Proje eski ama kullanılmış olabilir. */
            <span className="text-2xs text-ink-3">
              kayıt yok
              {project.runCount > 0 && ` · toplam ${formatCount(project.runCount)}`}
            </span>
          )}
        </td>

        {/* ── Son çalışma ── */}
        <td className="py-2">
          {lastRun ? (
            <span className="flex items-center gap-1.5 text-2xs whitespace-nowrap text-ink-3">
              <RunStatusBadge status={lastRun.status} />
              {formatRelative(lastRun.createdAt)}
            </span>
          ) : (
            <span className="text-2xs whitespace-nowrap text-ink-3">
              {formatRelative(project.createdAt)} eklendi
            </span>
          )}
        </td>

        {/* ── Eylemler ──
            `RowAction`: durgunken saydam, hover VE odakta beliriyor. Saydamlık
            düğmeye değil sarmalayıcıya veriliyor — düğmeye verildiğinde
            `disabled:opacity` onu eziyor ve görünürlük tersine dönüyordu. */}
        <td className="py-2 pr-4">
          <RowAction className="justify-end gap-1">
            <Button
              size="sm"
              onClick={onEdit}
              icon={<IconEdit className="size-4" />}
              aria-label={`${project.name} projesini düzenle`}
            />
            <Button
              size="sm"
              variant="danger"
              onClick={() => setConfirming(true)}
              aria-label={`${project.name} projesini sil`}
            >
              <IconTrash className="size-4" />
            </Button>
          </RowAction>
        </td>
      </tr>

      {/* Silme onayı SATIRIN ALTINDA, tüm sütunları kaplayarak: neyi
          onayladığınızı görmeye devam edersiniz. */}
      {confirming && (
        <tr>
          <td colSpan={6} className="p-0">
            <ConfirmStrip
              question={`"${project.name}" projesi silinsin mi?`}
              consequence={
                project.runCount > 0 ? (
                  <>
                    {formatCount(project.runCount)} çalıştırma geçmişi de
                    silinecek. Bu geri alınamaz.
                  </>
                ) : undefined
              }
              busy={remove.isPending}
              error={remove.isError ? describeError(remove.error).message : undefined}
              onConfirm={() => remove.mutate()}
              onCancel={() => setConfirming(false)}
            />
          </td>
        </tr>
      )}
    </>
  );
}

/* ── Form ────────────────────────────────────────────────────────────────── */

function ProjectForm({
  project,
  gitProviders,
  onDone,
}: {
  project?: Project;
  gitProviders: GitProvider[];
  onDone: () => void;
}) {
  const queryClient = useQueryClient();
  const editing = project !== undefined;

  const [name, setName] = useState(project?.name ?? "");
  const [repoUrl, setRepoUrl] = useState(project?.repoUrl ?? "");
  const [branch, setBranch] = useState(project?.defaultBranch ?? "main");
  const [gitProviderId, setGitProviderId] = useState(
    project?.gitProviderId ?? "",
  );
  const [nodeVersion, setNodeVersion] = useState(
    project?.defaultNodeVersion ?? "",
  );

  // Yalnızca yeni imaj yayınlanınca değişir; uzun süre taze sayılabilir.
  const nodeVersions = useQuery({
    queryKey: ["node-versions"],
    queryFn: () => api.runner.nodeVersions(),
    staleTime: 60 * 60 * 1000,
  });

  const save = useMutation({
    mutationFn: () => {
      const body = {
        name: name.trim(),
        repoUrl: repoUrl.trim(),
        defaultBranch: branch.trim() || "main",
        gitProviderId: gitProviderId || undefined,
        clearGitProvider: gitProviderId === "",
        defaultNodeVersion: nodeVersion,
      };
      return editing
        ? api.projects.update(project.id, body)
        : api.projects.create(body);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
      onDone();
    },
  });

  const canSubmit = name.trim() !== "" && repoUrl.trim() !== "";

  return (
    <Panel
      title={editing ? `${project.name} — düzenle` : "Yeni proje"}
      description="Kaydetmeden önce depoya erişilebildiği ve branch'in var olduğu sınanır; erişilemiyorsa kayıt oluşmaz."
    >
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit) save.mutate();
        }}
      >
        <div className="flex flex-wrap gap-3">
          <Field label="Proje adı" className="min-w-48 flex-1">
            <Input
              value={name}
              placeholder="agent-coder"
              onChange={(e) => setName(e.target.value)}
            />
          </Field>
          <Field label="Depo adresi" className="min-w-72 flex-2">
            <Input
              className="font-mono text-xs"
              value={repoUrl}
              placeholder="https://github.com/kullanici/depo.git"
              onChange={(e) => setRepoUrl(e.target.value)}
            />
          </Field>
        </div>

        <div className="flex flex-wrap gap-3">
          <Field label="Varsayılan branch" className="min-w-40 flex-1">
            <Input
              className="font-mono text-xs"
              value={branch}
              placeholder="main"
              onChange={(e) => setBranch(e.target.value)}
            />
          </Field>
          <Field label="Git erişimi" className="min-w-48 flex-1">
            <Select
              value={gitProviderId}
              onChange={(e) => setGitProviderId(e.target.value)}
            >
              <option value="">Yok (açık depo)</option>
              {gitProviders.map((g) => (
                <option key={g.id} value={g.id}>
                  {g.name}
                </option>
              ))}
            </Select>
          </Field>

          {/* Her koşuda elle seçmemek için varsayılan; çalıştırırken
              değiştirilebilir. Seçenek yoksa alan hiç gösterilmiyor. */}
          {(nodeVersions.data?.versions.length ?? 0) > 0 && (
            <Field label="Varsayılan Node sürümü" className="min-w-44 flex-1">
              <Select
                value={nodeVersion}
                onChange={(e) => setNodeVersion(e.target.value)}
              >
                <option value="">Runner varsayılanı</option>
                {nodeVersions.data?.versions.map((v) => (
                  <option key={v} value={v}>
                    {v}
                  </option>
                ))}
              </Select>
            </Field>
          )}
        </div>

        {/* Özel depo için kimlik ZORUNLU ve bunu ancak kaydetme anında
            öğrenmek kötü bir sürpriz; erişim seçilmediyse önceden söyleniyor. */}
        {gitProviderId === "" && gitProviders.length > 0 && (
          <Notice>
            Erişim seçilmedi — depo herkese açık değilse klonlama başarısız
            olur.
          </Notice>
        )}
        {gitProviders.length === 0 && (
          <Notice tone="warning">
            Tanımlı git erişimi yok. Özel bir depo bağlayacaksanız önce{" "}
            <Link href="/settings" className="underline">
              Ayarlar → Kod depoları
            </Link>{" "}
            bölümünden bir erişim ekleyin.
          </Notice>
        )}

        {save.isError && (
          <Notice tone="error" title={describeError(save.error).message}>
            {describeError(save.error).hint}
          </Notice>
        )}

        <div className="flex flex-wrap items-center gap-2 pt-1">
          <Button
            type="submit"
            variant="primary"
            disabled={!canSubmit || save.isPending}
          >
            {save.isPending ? "Depo kontrol ediliyor…" : "Doğrula ve kaydet"}
          </Button>
          <Button type="button" onClick={onDone} disabled={save.isPending}>
            Vazgeç
          </Button>
        </div>
      </form>
    </Panel>
  );
}

function Field({
  label,
  className = "",
  children,
}: {
  label: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <label className={`block ${className}`}>
      <span className="mb-1 block text-2xs font-medium tracking-wide text-ink-2 uppercase">
        {label}
      </span>
      {children}
    </label>
  );
}
