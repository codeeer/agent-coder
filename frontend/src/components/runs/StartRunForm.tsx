"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { Agent } from "@/lib/types";
import { ModelPicker } from "@/components/models/ModelPicker";
import {
  Button,
  Card,
  Input,
  Notice,
  Select,
  Textarea,
} from "@/components/ui/primitives";

/**
 * Bir agent'ı bir proje üzerinde çalıştırma formu.
 *
 * Model seçilmezse agent'ın varsayılanı kullanılır; o da yoksa backend
 * açık bir hata döner.
 */
export function StartRunForm({
  agent,
  onDone,
}: {
  agent: Agent;
  onDone: () => void;
}) {
  const router = useRouter();

  const projects = useQuery({ queryKey: ["projects"], queryFn: () => api.projects.list({ limit: 200 }) });
  const models = useQuery({
    queryKey: ["models", "run-picker"],
    queryFn: () => api.models.list({ limit: 500, sort: "provider" }),
  });

  const [projectId, setProjectId] = useState("");
  const [branch, setBranch] = useState("");
  // Model seçimi (sağlayıcı, model) çiftidir: aynı model adı iki sağlayıcıda
  // bulunabilir ve tek başına kimlik hangisinin kastedildiğini söylemez.
  const [model, setModel] = useState<{ providerId: string; modelId: string } | null>(
    agent.defaultModel && agent.defaultProviderId
      ? { providerId: agent.defaultProviderId, modelId: agent.defaultModel }
      : null,
  );
  const [task, setTask] = useState("");

  const project = projects.data?.items.find((p) => p.id === projectId);
  const chosenModel = models.data?.items.find(
    (m) => m.id === model?.modelId && m.providerId === model.providerId,
  );

  const start = useMutation({
    mutationFn: () =>
      api.runs.start({
        projectId,
        agentId: agent.id,
        providerId: model?.providerId,
        model: model?.modelId,
        branch: branch.trim() || undefined,
        task: task.trim(),
      }),
    onSuccess: (run) => {
      onDone();
      router.push(`/runs/${run.id}`);
    },
  });

  const canSubmit = projectId !== "" && task.trim() !== "";

  if (projects.data?.total === 0) {
    return (
      <Card>
        <Notice tone="warning">
          Çalıştırmak için önce bir proje tanımlayın.{" "}
          <Link href="/projects" className="underline">
            Projeler
          </Link>
        </Notice>
        <div className="mt-3">
          <Button onClick={onDone}>Kapat</Button>
        </div>
      </Card>
    );
  }

  return (
    <Card>
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (canSubmit) start.mutate();
        }}
      >
        <div className="flex gap-3">
          <label className="block flex-1">
            <span className="text-[11px] tracking-wide text-ink-2 uppercase">
              Proje
            </span>
            <Select
              className="mt-1"
              value={projectId}
              onChange={(e) => {
                setProjectId(e.target.value);
                setBranch("");
              }}
            >
              <option value="">Seçin…</option>
              {projects.data?.items.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </Select>
          </label>

          <label className="block flex-1">
            <span className="text-[11px] tracking-wide text-ink-2 uppercase">
              Branch
            </span>
            <Input
              className="mt-1 font-mono text-[12px]"
              value={branch}
              placeholder={project?.defaultBranch ?? "varsayılan"}
              onChange={(e) => setBranch(e.target.value)}
            />
          </label>
        </div>

        <div>
          <span className="text-[11px] tracking-wide text-ink-2 uppercase">
            Model
          </span>
          <div className="mt-1">
            <ModelPicker
              models={models.data?.items ?? []}
              providerId={model?.providerId ?? null}
              modelId={model?.modelId ?? ""}
              onChange={setModel}
              emptyLabel={
                agent.defaultModel
                  ? `Agent varsayılanı (${agent.defaultModel})`
                  : "Model ara…"
              }
            />
          </div>
        </div>

        {/* Araç desteği agent'ın iş yapıp yapamayacağını belirler. */}
        {chosenModel?.supportsTools === false && (
          <Notice tone="warning">
            Seçilen model araç çağıramıyor; agent dosya okuyup değiştiremez.
          </Notice>
        )}
        {chosenModel?.supportsTools === null && (
          <Notice tone="warning">
            Sağlayıcı bu modelin araç desteğini bildirmiyor — çalışmayabilir.
          </Notice>
        )}

        <label className="block">
          <span className="text-[11px] tracking-wide text-ink-2 uppercase">
            Görev
          </span>
          <Textarea
            className="mt-1 h-28"
            value={task}
            placeholder={`${agent.name} agent'ından ne yapmasını istiyorsunuz?`}
            onChange={(e) => setTask(e.target.value)}
          />
        </label>

        {start.isError && (
          <Notice tone="error" title={describeError(start.error).message}>
            {describeError(start.error).hint}
          </Notice>
        )}

        <div className="flex items-center gap-2 pt-1">
          <Button
            type="submit"
            variant="primary"
            disabled={!canSubmit || start.isPending}
          >
            {start.isPending ? "Başlatılıyor…" : "Çalıştır"}
          </Button>
          <Button type="button" onClick={onDone} disabled={start.isPending}>
            Vazgeç
          </Button>
          {!agent.allowEdit && (
            <span className="text-[12px] text-ink-3">
              Bu agent dosya değiştiremez — yalnızca inceler.
            </span>
          )}
        </div>
      </form>
    </Card>
  );
}
