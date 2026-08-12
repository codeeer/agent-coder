"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import {
  WorkflowRunBadge,
  isWorkflowActive,
} from "@/components/workflows/WorkflowStatusBadge";
import { IconFolder, IconPlus, IconTrash, IconWorkflow } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  ConfirmStrip,
  EmptyState,
  IconTile,
  Input,
  Notice,
  PageHeader,
  RowAction,
  SearchField,
  Segmented,
  Select,
  Skeleton,
  Textarea,
  Toolbar,
  formatRelative,
  toneFromKey,
} from "@/components/ui/primitives";
import type { Workflow } from "@/lib/types";

/**
 * Sayfa boyutu.
 *
 * 25 idi ve ekrana sığmıyordu: liste tipik bir masaüstü penceresinde
 * kaydırma gerektiriyor, sayfalama denetimi de ancak sona kadar
 * kaydırılınca görünüyordu — yani sayfalama VARDI ama görünmüyordu.
 * Kaydırmak ile sayfalamak aynı işi iki yoldan yapıyordu.
 *
 * Şimdi liste ekrana sığıyor ve gezinme sayfalamayla oluyor; isteyen
 * sayfa boyutunu denetimin yanından büyütebiliyor.
 */
const PAGE_SIZES = [10, 25, 50] as const;

/**
 * Durum süzgeci.
 *
 * Süzgeç AÇIK SAYFADA çalışır, tüm geçmişte değil — backend'in liste ucu
 * durum parametresi almıyor. Bu, Çalıştırmalar ekranındaki süzgeçle aynı
 * davranış; iki listenin aynı şeyi farklı yapması kullanıcıya her ekranda
 * yeniden öğrenmek düşerdi.
 */
const FILTERS = [
  { id: "all", label: "Tüm durumlar" },
  { id: "active", label: "Etkin" },
  { id: "paused", label: "Duraklatıldı" },
  { id: "draft", label: "Taslak" },
] as const;

type FilterId = (typeof FILTERS)[number]["id"];

function matches(w: Workflow, filter: FilterId): boolean {
  switch (filter) {
    case "active":
      return w.isActive && w.activeVersion !== null;
    case "paused":
      return !w.isActive;
    case "draft":
      return w.activeVersion === null;
    default:
      return true;
  }
}

export default function WorkflowsPage() {
  const [creating, setCreating] = useState(false);
  const [offset, setOffset] = useState(0);
  const [limit, setLimit] = useState<number>(PAGE_SIZES[0]);
  const [filter, setFilter] = useState<FilterId>("all");
  const [q, setQ] = useState("");

  const workflows = useQuery({
    queryKey: ["workflows", offset, limit],
    queryFn: () => api.workflows.list({ limit, offset }),
    // Çalışan akış varsa liste kendini tazelesin.
    refetchInterval: (q) =>
      q.state.data?.items.some((w) => w.lastRun && isWorkflowActive(w.lastRun.status))
        ? 3000
        : false,
  });

  const items = useMemo(() => {
    const rows = workflows.data?.items ?? [];
    const needle = q.trim().toLocaleLowerCase("tr");
    return rows.filter(
      (w) =>
        matches(w, filter) &&
        (needle === "" ||
          [w.name, w.description, w.projectName]
            .filter(Boolean)
            .some((v) => v.toLocaleLowerCase("tr").includes(needle))),
    );
  }, [workflows.data, filter, q]);

  return (
    /*
      Üç bölgeli düzen: üstte başlık ve araç çubuğu, ortada KAYAN liste,
      altta sayfalama. Sayfalama listenin ardından gelen sıradan bir blok
      olsaydı ona ulaşmak için önce listenin sonuna kadar kaydırmak
      gerekirdi — yani sayfalama, ancak kaydırmayı bitirenlere görünürdü.
    */
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader
        title="Akışlar"
        description="Adımları birbirine bağlı, kaydedilebilir akışlar. Her adım kendi agent'ı ve kendi modeliyle çalışır."
        actions={
          <Button
            variant="primary"
            icon={<IconPlus className="size-4" />}
            onClick={() => setCreating(true)}
          >
            Akış oluştur
          </Button>
        }
      />

      {creating && <CreateForm onDone={() => setCreating(false)} />}

      {/* Araç çubuğu yalnızca gerçekten liste varken: tek akışı olan bir
          kurulumda arama kutusu, aramayı gerektiren bir yığın olduğunu ima
          ederdi. */}
      {(workflows.data?.total ?? 0) > 0 && (
        <Toolbar>
          <SearchField
            className="min-w-50 flex-1 sm:max-w-sm"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Akış, açıklama veya proje ara…"
            aria-label="Akışlarda ara"
          />
          <Segmented
            label="Durum süzgeci"
            options={FILTERS}
            value={filter}
            onChange={setFilter}
          />
        </Toolbar>
      )}

      {/* Kayan bölge. `min-h-0` şart: bir flex öğesi varsayılan olarak
          içeriğinden küçülemez ve bu kural olmadan liste kabı taşıp
          sayfalamayı ekranın dışına iterdi. */}
      <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">
        {workflows.isPending && <Skeleton rows={3} />}
        {workflows.isError && (
          <Notice tone="error">{describeError(workflows.error).message}</Notice>
        )}

        {workflows.data?.total === 0 && !creating && (
        <EmptyState
          icon={<IconWorkflow className="size-4" />}
          title="Henüz akış yok"
          description="Bir akış, agent'ları sırayla çalıştırır ve her adımın çıktısını bir sonrakine geçirir — kopyala-yapıştır olmadan."
          action={
            <Button variant="primary" onClick={() => setCreating(true)}>
              İlk akışı oluştur
            </Button>
          }
        />
        )}

        {workflows.data && workflows.data.items.length > 0 && items.length === 0 && (
          <Notice>Bu süzgece uyan akış yok.</Notice>
        )}

        {items.length > 0 && (
          <div className="space-y-2.5">
            {items.map((w) => (
              <WorkflowRow key={w.id} workflow={w} />
            ))}
          </div>
        )}
      </div>

      {/* Sayfalama KAYAN BÖLGENİN DIŞINDA: her zaman görünür, sağ alt köşede. */}
      {workflows.data && (
        <Pagination
          total={workflows.data.total}
          limit={workflows.data.limit}
          offset={workflows.data.offset}
          onChange={(next) => {
            setOffset(next);
            // Arama sayfayla birlikte sıfırlanır: taşınsaydı kullanıcıya boş
            // bir sayfa gösterirdi. Süzgeç ise bir NİYET, o kalıyor.
            setQ("");
          }}
          pageSize={{
            value: limit,
            options: PAGE_SIZES,
            onChange: (size) => {
              setLimit(size);
              // Boyut değişince ilk sayfaya dönülür: 50'lik listenin 3.
              // sayfasındayken boyutu 10'a çekmek, var olmayan bir kayıt
              // aralığına bakmak demek.
              setOffset(0);
            },
          }}
          unit="akış"
        />
      )}
    </div>
  );
}

/**
 * Liste satırı.
 *
 * Bağlantı satırın TAMAMINI değil içeriğini sarar; silme düğmesi kardeş
 * hücrede durur. Bir `<a>` içine `<button>` konamaz (geçersiz HTML) ve satırı
 * bütünüyle bağlantı bırakmak silmeyi imkânsız kılardı. Aynı ayrım
 * Çalıştırmalar tablosunda da var.
 *
 * Düğme durgunken görünmez (`sm:opacity-0`): her satırda duran bir silme
 * düğmesi, listenin asıl işi olan "aç"ın önüne geçerdi. Odakla da beliriyor —
 * klavyeyle gezen kullanıcı için görünmezlik erişilemezlik olmamalı. Dar
 * ekranda hover yok, bu yüzden orada hep görünür.
 */
function WorkflowRow({ workflow: w }: { workflow: Workflow }) {
  const qc = useQueryClient();
  const [confirming, setConfirming] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.workflows.remove(w.id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["workflows"] });
      // Akış çalışmaları ve raporlar da bu akışın geçmişini gösteriyordu.
      void qc.invalidateQueries({ queryKey: ["workflow-runs"] });
      void qc.invalidateQueries({ queryKey: ["report"] });
      setConfirming(false);
    },
  });

  return (
    <div className="group overflow-hidden rounded-card border border-line bg-surface shadow-(--shadow-card) transition-colors hover:border-line-strong">
      <div className="flex items-center gap-4 p-3.5">
        <Link
          href={`/workflows/${w.id}`}
          className="flex min-w-0 flex-1 items-center gap-4 rounded transition-colors"
        >
          {/*
            Karo, satırın kimlik işareti. Rengi akışın kimliğinden
            türetiliyor ve sabit kalıyor: kullanıcı ikinci sayfada da
            aradığı akışı aynı renkte buluyor. Karo DURUM ANLATMAZ —
            onu sağdaki rozet söylüyor.
          */}
          <IconTile tone={toneFromKey(w.id)}>
            <IconWorkflow className="size-4" />
          </IconTile>

          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="truncate text-sm font-semibold tracking-[-0.01em]">
                {w.name}
              </span>
              {/* "tanımsız" ne olduğu belirsizdi; "taslak" bir sonraki
                 adımı da söylüyor: henüz kaydedilmemiş bir akış. */}
              {!w.activeVersion && <Badge tone="warning">taslak</Badge>}
              {!w.isActive && <Badge tone="warning">duraklatıldı</Badge>}
            </div>

            {w.description && (
              <p className="mt-0.5 truncate text-xs text-ink-2">{w.description}</p>
            )}

            {/* "varsayılan": akış artık tek bir projeye mühürlü değil,
               çalıştırırken başka bir proje seçilebiliyor. */}
            <p className="mt-1 flex items-center gap-1.5 text-2xs text-ink-3">
              <IconFolder className="size-3.5" />
              varsayılan {w.projectName}
            </p>
          </div>

          {/* Rozet tek başına "akış başarısız" gibi okunuyordu; başına
             "son çalışma" yazınca neyin durumu olduğu belli oluyor. */}
          {w.lastRun && (
            <div className="flex shrink-0 items-center gap-3 text-2xs text-ink-3">
              <span className="hidden sm:inline">
                son çalışma {formatRelative(w.lastRun.createdAt)}
              </span>
              <WorkflowRunBadge status={w.lastRun.status} />
            </div>
          )}
        </Link>

        <RowAction>
          <Button
            size="sm"
            variant="danger"
            onClick={() => setConfirming(true)}
            aria-label={`${w.name} akışını sil`}
          >
            <IconTrash className="size-4" />
          </Button>
        </RowAction>
      </div>

      {confirming && (
        <ConfirmStrip
          className="border-t"
          question="Bu akış silinsin mi?"
          consequence={
            w.runCount > 0 ? (
              <>
                <strong>
                  {w.runCount} çalışma geçmişi
                </strong>{" "}
                de silinecek ve raporlardan düşecek. Bu geri alınamaz.
              </>
            ) : undefined
          }
          busy={remove.isPending}
          error={remove.isError ? describeError(remove.error).message : undefined}
          onConfirm={() => remove.mutate()}
          onCancel={() => setConfirming(false)}
        />
      )}
    </div>
  );
}

/** Yeni akış formu. Akış önce boş oluşur, adımları düzenleme ekranından eklenir. */
function CreateForm({ onDone }: { onDone: () => void }) {
  const qc = useQueryClient();
  const projects = useQuery({ queryKey: ["projects"], queryFn: () => api.projects.list({ limit: 200 }) });

  const [projectId, setProjectId] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  const create = useMutation({
    mutationFn: () =>
      api.workflows.create({ projectId, name: name.trim(), description: description.trim() }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["workflows"] });
      onDone();
    },
  });

  if (projects.data?.total === 0) {
    return (
      <Card className="mb-5">
        <Notice tone="warning">
          Akışın bir varsayılan projesi olmalı. Önce bir proje tanımlayın.{" "}
          <Link href="/projects" className="underline">
            Projeler
          </Link>
        </Notice>
      </Card>
    );
  }

  const canSubmit = projectId !== "" && name.trim() !== "";

  return (
    <Card className="mb-5">
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit) create.mutate();
        }}
      >
        <div className="flex flex-wrap gap-3">
          <label className="block min-w-56 flex-1">
            <span className="text-2xs tracking-wide text-ink-2 uppercase">
              Varsayılan proje
            </span>
            <Select
              className="mt-1"
              value={projectId}
              onChange={(e) => setProjectId(e.target.value)}
            >
              <option value="">Seçin…</option>
              {projects.data?.items.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </label>

          <label className="block min-w-56 flex-1">
            <span className="text-2xs tracking-wide text-ink-2 uppercase">Ad</span>
            <Input
              className="mt-1"
              value={name}
              placeholder="Kod inceleme akışı"
              onChange={(e) => setName(e.target.value)}
            />
          </label>
        </div>

        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">Açıklama</span>
          <Textarea
            className="mt-1 h-16"
            value={description}
            placeholder="Bu akış ne yapar?"
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>

        {create.isError && (
          <Notice tone="error" title={describeError(create.error).message}>
            {describeError(create.error).hint}
          </Notice>
        )}

        <div className="flex gap-2 pt-1">
          <Button type="submit" variant="primary" disabled={!canSubmit || create.isPending}>
            {create.isPending ? "Oluşturuluyor…" : "Oluştur"}
          </Button>
          <Button type="button" onClick={onDone} disabled={create.isPending}>
            Vazgeç
          </Button>
        </div>
      </form>
    </Card>
  );
}
