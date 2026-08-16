"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { describeError, type ErrorContext } from "@/lib/errors";
import type { LLMProvider, LLMProviderType } from "@/lib/types";
import {
  Badge,
  Button,
  PanelCard,
  Field,
  Input,
  Notice,
  Select,
  Panel,
  formatDate,
  ConfirmInline,
} from "@/components/ui/primitives";

/** Tür başına arayüz davranışı. */
const TYPES: Record<
  LLMProviderType,
  { label: string; description: string; needsBaseURL: boolean; placeholder: string }
> = {
  openrouter: {
    label: "OpenRouter",
    description:
      "Yüzlerce modele tek anahtarla erişim. Fiyat, bağlam ve araç desteği bilgisi katalogla birlikte gelir.",
    needsBaseURL: false,
    placeholder: "sk-or-v1-…",
  },
  litellm: {
    label: "LiteLLM proxy",
    description:
      "Kurum içi LiteLLM sunucusu. Fiyat ve bağlam bilgisi, LiteLLM yöneticisi model_info alanlarını doldurduysa gelir.",
    needsBaseURL: true,
    placeholder: "sk-…",
  },
  openai_compatible: {
    label: "OpenAI-uyumlu servis",
    description:
      "vLLM, Azure OpenAI, Ollama gibi /v1/models sunan her servis. Katalogda yalnızca model adları görünür.",
    needsBaseURL: true,
    placeholder: "anahtar",
  },
};

export function LLMProviderSection() {
  const [adding, setAdding] = useState(false);
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["llm-providers"],
    queryFn: api.llmProviders.list,
  });

  return (
    <Panel
      title="LLM sağlayıcılar"
      description="Birden fazla sağlayıcı aynı anda tanımlı olabilir. Her agent adımı hangi sağlayıcının hangi modelini kullanacağını ayrı seçer."
      action={
        !adding && (
          <Button variant="primary" onClick={() => setAdding(true)}>
            Sağlayıcı ekle
          </Button>
        )
      }
    >
      <div className="space-y-3">
        {adding && <LLMProviderForm onDone={() => setAdding(false)} />}

        {isPending && <Notice>Sağlayıcılar yükleniyor…</Notice>}

        {isError && (
          <Notice tone="error">{describeError(error).message}</Notice>
        )}

        {data?.length === 0 && !adding && (
          <Notice>
            Henüz sağlayıcı yok. Model kataloğunu görebilmek için en az bir
            sağlayıcı ekleyin.
          </Notice>
        )}

        {data?.map((p) => <LLMProviderCard key={p.id} provider={p} />)}
      </div>
    </Panel>
  );
}

function LLMProviderCard({ provider }: { provider: LLMProvider }) {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  /*
   * Onay durumu KARTTA tutuluyor, silme düğmesinde değil.
   *
   * Düğmenin içinde dursaydı onay şeridi, diğer eylemlerin yanına sıkışırdı:
   * ölçüldü, kart 209px taşıyor ve "Evet, sil" ekranın dışında kalıyordu.
   * Onay açıkken bütün eylem sırası yerini bırakıyor — git erişimi ve MCP
   * bölümlerindeki kalıbın aynısı.
   */
  const [confirming, setConfirming] = useState(false);
  const spec = TYPES[provider.type];

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ["llm-providers"] });
    void queryClient.invalidateQueries({ queryKey: ["models"] });
  };

  const sync = useMutation({
    mutationFn: () => api.llmProviders.sync(provider.id),
    onSuccess: invalidate,
  });

  const remove = useMutation({
    mutationFn: () => api.llmProviders.remove(provider.id),
    onSuccess: () => {
      invalidate();
      setConfirming(false);
    },
  });

  const setDefault = useMutation({
    mutationFn: () =>
      api.llmProviders.update(provider.id, { isDefault: true }),
    onSuccess: invalidate,
  });

  return (
    <PanelCard>
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="font-medium">{provider.name}</h3>
            <Badge>{spec.label}</Badge>
            {provider.isDefault && <Badge tone="success">varsayılan</Badge>}
          </div>

          <dl className="mt-2 flex flex-wrap gap-x-6 gap-y-1 text-sm">
            <Field label="Anahtar" value={`••••••••${provider.hint}`} mono />
            {spec.needsBaseURL && (
              <Field label="Adres" value={provider.baseUrl} mono />
            )}
            <Field
              label="Model"
              value={String(provider.sync?.modelCount ?? 0)}
            />
            <Field
              label="Son güncelleme"
              value={formatDate(provider.sync?.lastSuccessAt ?? null)}
            />
          </dl>

          {provider.sync?.lastError && (
            <p className="mt-2 text-sm text-warn">
              Katalog güncellenemedi: {provider.sync.lastError}
            </p>
          )}
        </div>

        {!editing && !confirming && (
          <div className="flex shrink-0 flex-wrap justify-end gap-2">
            <Button onClick={() => sync.mutate()} disabled={sync.isPending}>
              {sync.isPending ? "Yenileniyor…" : "Katalogu yenile"}
            </Button>
            {!provider.isDefault && (
              <Button
                onClick={() => setDefault.mutate()}
                disabled={setDefault.isPending}
              >
                Varsayılan yap
              </Button>
            )}
            <Button onClick={() => setEditing(true)}>Düzenle</Button>
            <Button variant="danger" onClick={() => setConfirming(true)}>
              Sil
            </Button>
          </div>
        )}

        {confirming && (
          <div className="min-w-0">
            <ConfirmInline
              question={
                <>
                  <strong>{provider.name}</strong> silinsin mi?
                </>
              }
              consequence={`${provider.sync?.modelCount ?? 0} model de silinecek.`}
              busy={remove.isPending}
              onConfirm={() => remove.mutate()}
              onCancel={() => setConfirming(false)}
            />
          </div>
        )}
      </div>

      {sync.isError && (
        <p className="mt-2 text-sm text-danger">
          {describeError(sync.error).message}
        </p>
      )}

      {editing && (
        <LLMProviderForm provider={provider} onDone={() => setEditing(false)} />
      )}
    </PanelCard>
  );
}

function LLMProviderForm({
  provider,
  onDone,
}: {
  provider?: LLMProvider;
  onDone: () => void;
}) {
  const queryClient = useQueryClient();
  const editing = provider !== undefined;

  const [type, setType] = useState<LLMProviderType>(
    provider?.type ?? "openrouter",
  );
  const [name, setName] = useState(provider?.name ?? "");
  const [baseUrl, setBaseUrl] = useState(provider?.baseUrl ?? "");
  const [secret, setSecret] = useState("");

  const spec = TYPES[type];

  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.llmProviders.update(provider.id, {
            name: name.trim(),
            baseUrl: spec.needsBaseURL ? baseUrl.trim() : undefined,
            secret: secret.trim() || undefined,
          })
        : api.llmProviders.create({
            type,
            name: name.trim(),
            baseUrl: spec.needsBaseURL ? baseUrl.trim() : undefined,
            secret: secret.trim(),
          }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["llm-providers"] });
      void queryClient.invalidateQueries({ queryKey: ["models"] });
      onDone();
    },
  });

  const canSubmit =
    name.trim() !== "" &&
    (!spec.needsBaseURL || baseUrl.trim() !== "") &&
    // Düzenlemede anahtar boş bırakılabilir; mevcut olan korunur.
    (editing || secret.trim() !== "");

  const body = (
    <form
      className="space-y-3"
      onSubmit={(e) => {
        e.preventDefault();
        if (canSubmit) save.mutate();
      }}
    >
      {!editing && (
        <label className="block">
          <span className="text-xs text-ink-2">Tür</span>
          <Select
            className="mt-1"
            value={type}
            onChange={(e) => setType(e.target.value as LLMProviderType)}
          >
            {(Object.keys(TYPES) as LLMProviderType[]).map((t) => (
              <option key={t} value={t}>
                {TYPES[t].label}
              </option>
            ))}
          </Select>
          <span className="mt-1 block text-xs text-ink-2">
            {spec.description}
          </span>
        </label>
      )}

      <label className="block">
        <span className="text-xs text-ink-2">Ad</span>
        <Input
          className="mt-1"
          value={name}
          placeholder="Şirket LiteLLM"
          onChange={(e) => setName(e.target.value)}
        />
      </label>

      {spec.needsBaseURL && (
        <label className="block">
          <span className="text-xs text-ink-2">Servis adresi</span>
          <Input
            className="mt-1 font-mono"
            value={baseUrl}
            placeholder="https://llm.sirket.local/v1"
            onChange={(e) => setBaseUrl(e.target.value)}
          />
        </label>
      )}

      <label className="block">
        <span className="text-xs text-ink-2">
          Anahtar{editing && " (boş bırakırsanız mevcut anahtar korunur)"}
        </span>
        <Input
          className="mt-1 font-mono"
          type="password"
          autoComplete="off"
          value={secret}
          placeholder={spec.placeholder}
          onChange={(e) => setSecret(e.target.value)}
        />
      </label>

      {save.isError && <FormError error={save.error} context="llm" />}

      <div className="flex items-center gap-2 pt-1">
        <Button type="submit" variant="primary" disabled={!canSubmit || save.isPending}>
          {save.isPending ? "Doğrulanıyor…" : "Doğrula ve kaydet"}
        </Button>
        <Button type="button" onClick={onDone} disabled={save.isPending}>
          Vazgeç
        </Button>
        <span className="text-xs text-ink-2">
          Kaydetmeden önce adres ve anahtar sınanır.
        </span>
      </div>
    </form>
  );

  return editing ? <div className="mt-4">{body}</div> : <PanelCard>{body}</PanelCard>;
}


/**
 * Form hatası kutusu — LLM ve git erişim formları paylaşır.
 *
 * `context` ipucunun hangi ekrana göre yazılacağını belirler; verilmezse ipucu
 * gösterilmez. Paylaşılan bir bileşenin tek bir ekranın diline göre yazılmış
 * ipucu göstermesi, diğer ekranda yanlış yönlendirme demektir.
 */
export function FormError({
  error,
  context,
}: {
  error: unknown;
  context?: ErrorContext;
}) {
  const { message, hint } = describeError(error, context);
  return (
    <div className="rounded border border-danger/35 bg-danger-soft px-3 py-2 text-sm">
      <p className="font-medium text-danger">{message}</p>
      {hint && <p className="mt-0.5 text-xs text-ink-2">{hint}</p>}
    </div>
  );
}

