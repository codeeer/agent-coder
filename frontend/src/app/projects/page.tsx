"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import type { GitProvider, Project } from "@/lib/types";
import { IconFolder, IconPlus } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  List,
  Notice,
  PageHeader,
  Select,
  Skeleton,
  formatRelative,
} from "@/components/ui/primitives";

const PAGE_SIZE = 25;

export default function ProjectsPage() {
  const [adding, setAdding] = useState(false);
  const [offset, setOffset] = useState(0);

  const projects = useQuery({
    queryKey: ["projects", offset],
    queryFn: () => api.projects.list({ limit: PAGE_SIZE, offset }),
  });
  const gitProviders = useQuery({
    queryKey: ["git-providers"],
    queryFn: api.gitProviders.list,
  });

  return (
    <div>
      <PageHeader
        title="Projeler"
        description="Agent'ların üzerinde çalışacağı kod depoları. Bir kez tanımlanır, her çalıştırmada listeden seçilir."
        actions={
          !adding && (
            <Button
              variant="primary"
              onClick={() => setAdding(true)}
              icon={<IconPlus className="size-4" />}
            >
              Proje ekle
            </Button>
          )
        }
      />

      <div className="space-y-3">
        {adding && (
          <ProjectForm
            gitProviders={gitProviders.data ?? []}
            onDone={() => setAdding(false)}
          />
        )}

        {projects.isPending && <Skeleton rows={2} />}

        {projects.isError && (
          <Notice tone="error">{describeError(projects.error).message}</Notice>
        )}

        {projects.data?.total === 0 && !adding && (
          <EmptyState
            icon={<IconFolder className="size-4" />}
            title="Henüz proje yok"
            description="Bir agent çalıştırabilmek için önce üzerinde çalışacağı depoyu tanımlayın."
            action={
              <Button variant="primary" onClick={() => setAdding(true)}>
                İlk projeyi ekle
              </Button>
            }
          />
        )}

        {projects.data && projects.data.items.length > 0 && (
          <List>
            {projects.data.items.map((p) => (
              <ProjectRow
                key={p.id}
                project={p}
                gitProviders={gitProviders.data ?? []}
              />
            ))}
          </List>
        )}

        {projects.data && (
          <Pagination
            total={projects.data.total}
            limit={projects.data.limit}
            offset={projects.data.offset}
            onChange={setOffset}
            unit="proje"
          />
        )}
      </div>
    </div>
  );
}

function ProjectRow({
  project,
  gitProviders,
}: {
  project: Project;
  gitProviders: GitProvider[];
}) {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const provider = gitProviders.find((g) => g.id === project.gitProviderId);

  const remove = useMutation({
    mutationFn: () => api.projects.remove(project.id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
      setConfirming(false);
    },
  });

  return (
    <div className="px-4 py-3">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="font-medium">{project.name}</h2>
            {/* Rozet YALNIZCA sağlayıcı için: bu bir sınıflandırma, iki
                değerden biri. Çalıştırma sayısı ise metadata — aşağıdaki
                satırda düz metin olarak duruyor. Sayıyı hap içine koymak,
                rozetin "durum/tür" anlamını sulandırıyordu. */}
            {provider ? (
              <Badge tone="info">{provider.name}</Badge>
            ) : (
              <Badge title="Kimlik doğrulamasız klonlanır">açık depo</Badge>
            )}
          </div>

          {/*
            Öncesinde `DEPO` / `BRANCH` / `EKLENDİ` büyük harf etiketleriyle
            bir `<dl>` idi. O düzen bir DETAY ekranının dili: her değerin
            üstünde adı yazar. Liste satırında değerlerin ne olduğu zaten
            biçimlerinden belli (URL, branch adı, göreli zaman) ve etiketler
            satır başına üç fazladan metin kademesi ekliyordu.
          */}
          <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-ink-2">
            <span className="truncate font-mono">{project.repoUrl}</span>
            <span className="text-ink-3">·</span>
            <span className="font-mono">{project.defaultBranch}</span>
            <span className="text-ink-3">·</span>
            <span>{formatRelative(project.createdAt)} eklendi</span>
            {project.runCount > 0 && (
              <>
                <span className="text-ink-3">·</span>
                <span>{project.runCount} çalıştırma</span>
              </>
            )}
          </div>
        </div>

        {!editing && !confirming && (
          <div className="flex shrink-0 gap-2">
            <Button onClick={() => setEditing(true)}>Düzenle</Button>
            <Button variant="danger" onClick={() => setConfirming(true)}>
              Sil
            </Button>
          </div>
        )}

        {confirming && (
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-xs text-ink-2">
              {project.runCount > 0
                ? `${project.runCount} çalıştırma geçmişi de silinecek.`
                : "Emin misiniz?"}
            </span>
            <Button
              variant="danger"
              onClick={() => remove.mutate()}
              disabled={remove.isPending}
            >
              {remove.isPending ? "Siliniyor…" : "Evet, sil"}
            </Button>
            <Button onClick={() => setConfirming(false)}>Vazgeç</Button>
          </div>
        )}
      </div>

      {editing && (
        <div className="mt-4">
          <ProjectForm
            project={project}
            gitProviders={gitProviders}
            onDone={() => setEditing(false)}
            inline
          />
        </div>
      )}
    </div>
  );
}

function ProjectForm({
  project,
  gitProviders,
  onDone,
  inline = false,
}: {
  project?: Project;
  gitProviders: GitProvider[];
  onDone: () => void;
  inline?: boolean;
}) {
  const queryClient = useQueryClient();
  const editing = project !== undefined;

  const [name, setName] = useState(project?.name ?? "");
  const [repoUrl, setRepoUrl] = useState(project?.repoUrl ?? "");
  const [branch, setBranch] = useState(project?.defaultBranch ?? "main");
  const [gitProviderId, setGitProviderId] = useState(project?.gitProviderId ?? "");

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

  const body = (
    <form
      className="space-y-3"
      onSubmit={(e) => {
        e.preventDefault();
        if (canSubmit) save.mutate();
      }}
    >
      <label className="block">
        <span className="text-xs text-ink-2">Proje adı</span>
        <Input
          className="mt-1"
          value={name}
          placeholder="agent-coder"
          onChange={(e) => setName(e.target.value)}
        />
      </label>

      <label className="block">
        <span className="text-xs text-ink-2">Depo adresi</span>
        <Input
          className="mt-1 font-mono"
          value={repoUrl}
          placeholder="https://github.com/kullanici/depo.git"
          onChange={(e) => setRepoUrl(e.target.value)}
        />
      </label>

      <div className="flex gap-3">
        <label className="block flex-1">
          <span className="text-xs text-ink-2">Varsayılan branch</span>
          <Input
            className="mt-1 font-mono"
            value={branch}
            placeholder="main"
            onChange={(e) => setBranch(e.target.value)}
          />
        </label>

        <label className="block flex-1">
          <span className="text-xs text-ink-2">Git erişimi</span>
          <Select
            className="mt-1"
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
        </label>
      </div>

      {save.isError && <FormError error={save.error} />}

      <div className="flex items-center gap-2 pt-1">
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
        <span className="text-xs text-ink-2">
          Kaydetmeden önce depoya erişilebildiği ve branch&apos;in var olduğu sınanır.
        </span>
      </div>
    </form>
  );

  return inline ? body : <Card>{body}</Card>;
}

function FormError({ error }: { error: unknown }) {
  const { message, hint } = describeError(error);
  return (
    <div className="rounded border border-danger/35 bg-danger-soft px-3 py-2 text-sm">
      <p className="font-medium text-danger">{message}</p>
      {hint && <p className="mt-0.5 text-xs text-ink-2">{hint}</p>}
    </div>
  );
}

