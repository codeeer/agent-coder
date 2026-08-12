"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useState } from "react";
import { api } from "@/lib/api";
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
  IconAlert,
  IconEdit,
  IconFolder,
  IconPlus,
  IconTrash,
  IconWorkflow,
} from "@/components/ui/icons";
import {
  Badge,
  Button,
  EmptyState,
  IconTile,
  Input,
  Metric,
  Notice,
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

const PAGE_SIZE = 24;

/** Etkinlik rakamlarının dönemi. Agent'lar ekranıyla aynı. */
const USAGE_DAYS = 30;

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
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<FilterId>("all");

  const projects = useQuery({
    queryKey: ["projects", offset],
    queryFn: () => api.projects.list({ limit: PAGE_SIZE, offset }),
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
    <div className="flex min-h-0 flex-1 flex-col">
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
              <span className="ml-auto hidden text-2xs text-ink-3 lg:block">
                {items.length === projects.data?.items.length
                  ? `${items.length} proje`
                  : `${items.length} / ${projects.data?.items.length} proje`}
              </span>
            </Toolbar>
          )}

          <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1 pb-1">
            {items.length === 0 ? (
              <Notice>Bu süzgece uyan proje yok.</Notice>
            ) : (
              /* `items-start`: kartlar satır boyunca eşit yüksekliğe
                 ÇEKİLMEZ. Çekilseydi bir kartın silme onayıyla büyümesi
                 komşusunun ortasında boşluk açardı — nitekim açtı. */
              <ul className="grid items-start gap-4 md:grid-cols-2 2xl:grid-cols-3">
                {items.map((p) =>
                  editingId === p.id ? (
                    /* Düzenleme kartın KENDİ HÜCRESİNDE, tam genişlikte
                       açılıyor: ızgaranın altına ya da üstüne taşınsaydı
                       hangi projeyi düzenlediğiniz kaybolurdu. */
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
                    </li>
                  ),
                )}
              </ul>
            )}
          </div>

          <Pagination
            total={total}
            limit={projects.data?.limit ?? PAGE_SIZE}
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

  const save = useMutation({
    mutationFn: () => {
      const body = {
        name: name.trim(),
        repoUrl: repoUrl.trim(),
        defaultBranch: branch.trim() || "main",
        gitProviderId: gitProviderId || undefined,
        clearGitProvider: gitProviderId === "",
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
