"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import type { Agent, LLMProvider, Model } from "@/lib/types";
import { ModelPicker } from "@/components/models/ModelPicker";
import { StartRunForm } from "@/components/runs/StartRunForm";
import { IconPlus } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  Checkbox,
  Input,
  Notice,
  PageHeader,
  Skeleton,
  Textarea,
  Well,
} from "@/components/ui/primitives";

const PAGE_SIZE = 25;

export default function AgentsPage() {
  const [creating, setCreating] = useState(false);
  const [offset, setOffset] = useState(0);

  const agents = useQuery({
    queryKey: ["agents", offset],
    queryFn: () => api.agents.list({ limit: PAGE_SIZE, offset }),
  });
  const providers = useQuery({
    queryKey: ["llm-providers"],
    queryFn: api.llmProviders.list,
  });
  // Model seçici için araç destekleyenler ve bilinmeyenler; desteklemeyenler
  // agent olarak kullanılamaz, listelenmeleri kafa karıştırır.
  const models = useQuery({
    queryKey: ["models", "agent-picker"],
    queryFn: () => api.models.list({ limit: 500, sort: "provider" }),
  });

  return (
    <div className="max-w-4xl">
      <PageHeader
        title="Agent'lar"
        description="Hazır agent'ların talimatını kendi kurallarınıza göre değiştirebilir, sıfırdan kendi agent'ınızı oluşturabilirsiniz."
        actions={
          !creating && (
            <Button
              variant="primary"
              onClick={() => setCreating(true)}
              icon={<IconPlus className="size-3.5" />}
            >
              Agent oluştur
            </Button>
          )
        }
      />

      <div className="space-y-3">
        {creating && (
          <AgentForm
            providers={providers.data ?? []}
            models={models.data?.items ?? []}
            onDone={() => setCreating(false)}
          />
        )}

        {agents.isPending && <Skeleton rows={3} />}
        {agents.isError && (
          <Notice tone="error">{describeError(agents.error).message}</Notice>
        )}

        {agents.data?.items.map((a) => (
          <AgentCard
            key={a.id}
            agent={a}
            providers={providers.data ?? []}
            models={models.data?.items ?? []}
          />
        ))}

        {agents.data && (
          <Pagination
            total={agents.data.total}
            limit={agents.data.limit}
            offset={agents.data.offset}
            onChange={setOffset}
            unit="agent"
          />
        )}
      </div>
    </div>
  );
}

function AgentCard({
  agent,
  providers,
  models,
}: {
  agent: Agent;
  providers: LLMProvider[];
  models: Model[];
}) {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [showPrompt, setShowPrompt] = useState(false);
  const [running, setRunning] = useState(false);

  const invalidate = () =>
    void queryClient.invalidateQueries({ queryKey: ["agents"] });

  const reset = useMutation({
    mutationFn: () => api.agents.reset(agent.id),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: () => api.agents.remove(agent.id),
    onSuccess: () => {
      invalidate();
      setConfirming(false);
    },
  });

  const provider = providers.find((p) => p.id === agent.defaultProviderId);

  return (
    <Card>
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="font-medium">{agent.name}</h2>
            <span className="font-mono text-xs text-ink-2">{agent.slug}</span>
            {agent.source === "builtin" ? (
              <Badge>hazır</Badge>
            ) : (
              <Badge tone="info">özel</Badge>
            )}
            {agent.isModified && <Badge tone="warning">değiştirilmiş</Badge>}
          </div>

          <p className="mt-1 max-w-prose text-sm text-ink-2">
            {agent.description}
          </p>

          <PermissionLine agent={agent} />

          <p className="mt-2 text-xs text-ink-2">
            Varsayılan model:{" "}
            {agent.defaultModel ? (
              <span className="font-mono">
                {provider ? `${provider.name} · ` : ""}
                {agent.defaultModel}
              </span>
            ) : (
              "seçilmemiş"
            )}
          </p>
        </div>

        {!editing && !confirming && !running && (
          <div className="flex shrink-0 flex-wrap justify-end gap-2">
            <Button variant="primary" onClick={() => setRunning(true)}>
              Çalıştır
            </Button>
            <Button onClick={() => setShowPrompt((v) => !v)}>
              {showPrompt ? "Talimatı gizle" : "Talimatı gör"}
            </Button>
            <Button onClick={() => setEditing(true)}>Düzenle</Button>
            {agent.isModified && (
              <Button onClick={() => reset.mutate()} disabled={reset.isPending}>
                {reset.isPending ? "…" : "Sıfırla"}
              </Button>
            )}
            {agent.source === "custom" && (
              <Button variant="danger" onClick={() => setConfirming(true)}>
                Sil
              </Button>
            )}
          </div>
        )}

        {confirming && (
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-xs text-ink-2">Emin misiniz?</span>
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

      {remove.isError && (
        <p className="mt-2 text-sm text-danger">
          {describeError(remove.error).message}
        </p>
      )}

      {showPrompt && !editing && (
        <Well className="mt-4 max-h-80 overflow-auto p-3.5">
          <pre className="font-mono text-[12px] leading-relaxed whitespace-pre-wrap">
            {agent.prompt}
          </pre>
        </Well>
      )}

      {running && (
        <div className="mt-4">
          <StartRunForm agent={agent} onDone={() => setRunning(false)} />
        </div>
      )}

      {editing && (
        <div className="mt-4">
          <AgentForm
            agent={agent}
            providers={providers}
            models={models}
            onDone={() => setEditing(false)}
            inline
          />
        </div>
      )}
    </Card>
  );
}

/**
 * Agent'ın neye yetkisi olduğu.
 *
 * Önceki hali üç yetkiyi de rozetle gösteriyordu: açık olanlar YEŞİL, kapalılar
 * gri. İki sorun vardı. Yeşil "iyi" demek; oysa burada "bu agent dosyalarınızı
 * değiştirebilir" demek — riskli olan durumu güvenli renkle boyuyordu. Ve beş
 * agent yan yana on beş rozet ediyordu.
 *
 * Şimdi yalnızca AÇIK olanlar yazılıyor; hiçbiri yoksa tek cümle. Kapalı bir
 * yetkinin ekranda yer kaplaması gerekmiyor — yokluğu zaten cevap.
 */
function PermissionLine({ agent }: { agent: Agent }) {
  const izinli = [
    agent.allowEdit && "dosya değiştirebilir",
    agent.allowBash && "komut çalıştırabilir",
    agent.allowWebfetch && "ağa çıkabilir",
  ].filter(Boolean) as string[];

  return (
    <p className="mt-2 text-xs text-ink-2">
      {izinli.length === 0 ? (
        <>Yalnızca okur — hiçbir şeyi değiştiremez.</>
      ) : (
        <>Yetkileri: {izinli.join(" · ")}</>
      )}
    </p>
  );
}

function AgentForm({
  agent,
  providers,
  models,
  onDone,
  inline = false,
}: {
  agent?: Agent;
  providers: LLMProvider[];
  models: Model[];
  onDone: () => void;
  inline?: boolean;
}) {
  const queryClient = useQueryClient();
  const editing = agent !== undefined;

  const [name, setName] = useState(agent?.name ?? "");
  const [description, setDescription] = useState(agent?.description ?? "");
  const [prompt, setPrompt] = useState(agent?.prompt ?? "");
  // Varsayılan model (sağlayıcı, model) çiftidir; ikisi birlikte seçilir.
  const [model, setModel] = useState<{ providerId: string; modelId: string } | null>(
    agent?.defaultModel && agent.defaultProviderId
      ? { providerId: agent.defaultProviderId, modelId: agent.defaultModel }
      : null,
  );
  const [allowEdit, setAllowEdit] = useState(agent?.allowEdit ?? true);
  const [allowBash, setAllowBash] = useState(agent?.allowBash ?? true);
  const [allowWebfetch, setAllowWebfetch] = useState(agent?.allowWebfetch ?? false);

  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.agents.update(agent.id, {
            name: name.trim(),
            description: description.trim(),
            prompt,
            defaultProviderId: model?.providerId,
            clearProvider: model === null,
            defaultModel: model?.modelId ?? "",
            allowEdit,
            allowBash,
            allowWebfetch,
          })
        : api.agents.create({
            name: name.trim(),
            description: description.trim(),
            prompt,
            defaultProviderId: model?.providerId,
            defaultModel: model?.modelId ?? "",
            allowEdit,
            allowBash,
            allowWebfetch,
          }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
      onDone();
    },
  });

  const selected = models.find(
    (m) => m.id === model?.modelId && m.providerId === model.providerId,
  );
  const canSubmit = name.trim() !== "" && prompt.trim() !== "";

  const body = (
    <form
      className="space-y-3"
      onSubmit={(e) => {
        e.preventDefault();
        if (canSubmit) save.mutate();
      }}
    >
      <div className="flex gap-3">
        <label className="block flex-1">
          <span className="text-xs text-ink-2">Ad</span>
          <Input
            className="mt-1"
            value={name}
            placeholder="Kod İncelemecisi"
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label className="block flex-2">
          <span className="text-xs text-ink-2">Açıklama</span>
          <Input
            className="mt-1"
            value={description}
            placeholder="Ne yaptığını bir cümleyle anlatın"
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
      </div>

      <label className="block">
        <span className="text-xs text-ink-2">
          Talimat (agent&apos;a verilen sistem yönergesi)
        </span>
        <Textarea
          className="mt-1 h-64 font-mono text-[12px] leading-relaxed"
          value={prompt}
          placeholder="Sen bir kod incelemecisisin. …"
          onChange={(e) => setPrompt(e.target.value)}
        />
      </label>

      <div>
        <span className="text-[11px] tracking-wide text-ink-2 uppercase">
          Varsayılan model
        </span>
        <div className="mt-1">
          <ModelPicker
            models={models}
            providerId={model?.providerId ?? null}
            modelId={model?.modelId ?? ""}
            onChange={setModel}
            emptyLabel="Model ara… (seçilmezse her çalıştırmada seçilir)"
          />
        </div>
      </div>

      {/* Araç desteği agent'ın çalışıp çalışmayacağını belirler; sessiz geçilmez. */}
      {selected && selected.supportsTools === false && (
        <Notice tone="warning">
          Seçilen model araç çağıramıyor. Agent dosya okuyup değiştiremeyeceği
          için büyük olasılıkla işe yaramaz.
        </Notice>
      )}
      {selected && selected.supportsTools === null && (
        <Notice tone="warning">
          Sağlayıcı bu modelin araç desteğini bildirmiyor. Çalışabilir ama garanti
          değil.
        </Notice>
      )}

      <fieldset className="rounded border border-line p-3">
        <legend className="px-1 text-xs text-ink-2">Yetkiler</legend>
        <div className="flex flex-wrap gap-4">
          <Checkbox
            label="Dosya değiştirebilir"
            checked={allowEdit}
            onChange={setAllowEdit}
          />
          <Checkbox
            label="Komut çalıştırabilir"
            checked={allowBash}
            onChange={setAllowBash}
          />
          <Checkbox
            label="Ağa erişebilir"
            checked={allowWebfetch}
            onChange={setAllowWebfetch}
          />
        </div>
      </fieldset>

      {save.isError && (
        <div className="rounded border border-danger/35 bg-danger-soft px-3 py-2 text-sm">
          <p className="font-medium text-danger">
            {describeError(save.error).message}
          </p>
        </div>
      )}

      <div className="flex items-center gap-2 pt-1">
        <Button
          type="submit"
          variant="primary"
          disabled={!canSubmit || save.isPending}
        >
          {save.isPending ? "Kaydediliyor…" : "Kaydet"}
        </Button>
        <Button type="button" onClick={onDone} disabled={save.isPending}>
          Vazgeç
        </Button>
        {providers.length === 0 && (
          <span className="text-xs text-ink-2">
            Model seçebilmek için{" "}
            <Link href="/settings" className="underline">
              önce bir LLM sağlayıcı ekleyin
            </Link>
            .
          </span>
        )}
      </div>
    </form>
  );

  return inline ? body : <Card>{body}</Card>;
}

