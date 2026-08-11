"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import {
  WorkflowRunBadge,
  isWorkflowActive,
} from "@/components/workflows/WorkflowStatusBadge";
import { IconPlus, IconWorkflow } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  Notice,
  PageHeader,
  Select,
  Skeleton,
  Textarea,
  formatRelative,
} from "@/components/ui/primitives";

const PAGE_SIZE = 25;

export default function WorkflowsPage() {
  const [creating, setCreating] = useState(false);
  const [offset, setOffset] = useState(0);

  const workflows = useQuery({
    queryKey: ["workflows", offset],
    queryFn: () => api.workflows.list({ limit: PAGE_SIZE, offset }),
    // Çalışan akış varsa liste kendini tazelesin.
    refetchInterval: (q) =>
      q.state.data?.items.some((w) => w.lastRun && isWorkflowActive(w.lastRun.status))
        ? 3000
        : false,
  });

  return (
    <div>
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

      {workflows.data && workflows.data.items.length > 0 && (
        <div className="space-y-2.5">
          {workflows.data.items.map((w) => (
            /* Kartın TAMAMI bağlantı: satırın sonundaki "Aç" düğmesi hem
               gereksiz bir tıklama hedefi hem de her satırda tekrar eden
               görsel gürültüydü — kartın kendisi zaten oraya götürüyor. */
            <Link
              key={w.id}
              href={`/workflows/${w.id}`}
              className="block rounded-card border border-line bg-surface p-4 shadow-(--shadow-card) transition-colors hover:border-line-strong hover:bg-raised"
            >
              <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-base font-semibold tracking-[-0.01em]">
                      {w.name}
                    </span>
                    {/* "tanımsız" ne olduğu belirsizdi; "taslak" bir sonraki
                       adımı da söylüyor: henüz kaydedilmemiş bir akış. */}
                    {!w.activeVersion && <Badge tone="warning">taslak</Badge>}
                    {!w.isActive && <Badge tone="warning">duraklatıldı</Badge>}
                  </div>

                  {w.description && (
                    <p className="mt-1 text-sm text-ink-2">{w.description}</p>
                  )}
                  <p className="mt-1.5 text-xs text-ink-3">{w.projectName}</p>
                </div>

                {/* Rozet tek başına "akış başarısız" gibi okunuyordu; başına
                   "son çalışma" yazınca neyin durumu olduğu belli oluyor. */}
                {w.lastRun && (
                  <div className="flex shrink-0 items-center gap-2 text-xs text-ink-3">
                    <span>son çalışma {formatRelative(w.lastRun.createdAt)}</span>
                    <WorkflowRunBadge status={w.lastRun.status} />
                  </div>
                )}
              </div>
            </Link>
          ))}
        </div>
      )}

      {workflows.data && (
        <Pagination
          total={workflows.data.total}
          limit={workflows.data.limit}
          offset={workflows.data.offset}
          onChange={setOffset}
          unit="akış"
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
          Akış bir projeye bağlıdır. Önce bir proje tanımlayın.{" "}
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
            <span className="text-2xs tracking-wide text-ink-2 uppercase">Proje</span>
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
