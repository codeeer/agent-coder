"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { GitProvider, GitProviderType } from "@/lib/types";
import { FormError } from "./LLMProviderSection";
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

/**
 * Tür başına arayüz davranışı.
 *
 * GitHub tek token ile çalışır; Bitbucket kullanıcı adı + app password ister.
 * Bu fark alan zorunluluklarına yansır.
 */
const TYPES: Record<
  GitProviderType,
  {
    label: string;
    description: string;
    needsUsername: boolean;
    needsBaseURL: boolean;
    secretLabel: string;
    placeholder: string;
    verifiable: boolean;
  }
> = {
  github: {
    label: "GitHub",
    description: "github.com veya GitHub Enterprise.",
    needsUsername: false,
    needsBaseURL: false,
    secretLabel: "Personal access token (repo yetkisi)",
    placeholder: "ghp_… veya github_pat_…",
    verifiable: true,
  },
  bitbucket: {
    label: "Bitbucket",
    description: "Bitbucket Cloud app password veya Bitbucket Server.",
    needsUsername: true,
    needsBaseURL: false,
    secretLabel: "App password / parola",
    placeholder: "app password",
    verifiable: true,
  },
  generic: {
    label: "Genel Git",
    description:
      "Sağlayıcısı bilinmeyen HTTPS Git deposu. Klonlama ve push çalışır; PR açma gibi sağlayıcıya özel özellikler olmaz.",
    needsUsername: true,
    needsBaseURL: true,
    secretLabel: "Parola veya token",
    placeholder: "token",
    verifiable: false,
  },
};

export function GitProviderSection() {
  const [adding, setAdding] = useState(false);
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["git-providers"],
    queryFn: api.gitProviders.list,
  });

  return (
    <Panel
      title="Git erişimleri"
      description="Agent'ın ürettiği kodu branch'e göndermek ve PR açmak için kullanılır."
      action={
        !adding && (
          <Button variant="primary" onClick={() => setAdding(true)}>
            Git erişimi ekle
          </Button>
        )
      }
    >
      <div className="space-y-3">
        {adding && <GitProviderForm onDone={() => setAdding(false)} />}

        {isPending && <Notice>Git erişimleri yükleniyor…</Notice>}
        {isError && <Notice tone="error">{describeError(error).message}</Notice>}

        {data?.length === 0 && !adding && (
          <Notice>Henüz git erişimi tanımlı değil.</Notice>
        )}

        {data?.map((p) => <GitProviderCard key={p.id} provider={p} />)}
      </div>
    </Panel>
  );
}

function GitProviderCard({ provider }: { provider: GitProvider }) {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const spec = TYPES[provider.type];

  const invalidate = () =>
    void queryClient.invalidateQueries({ queryKey: ["git-providers"] });

  const remove = useMutation({
    mutationFn: () => api.gitProviders.remove(provider.id),
    onSuccess: () => {
      invalidate();
      setConfirming(false);
    },
  });

  return (
    <PanelCard>
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="font-medium">{provider.name}</h3>
            <Badge>{spec.label}</Badge>
            {!provider.verified && (
              <Badge
                tone="warning"
                title="Bu tür için doğrulama yapılamıyor; bilgiler ilk gerçek kullanımda sınanacak"
              >
                doğrulanamadı
              </Badge>
            )}
          </div>

          <dl className="mt-2 flex flex-wrap gap-x-6 gap-y-1 text-sm">
            {provider.username && (
              <Field label="Kullanıcı adı" value={provider.username} />
            )}
            <Field label="Değer" value={`••••••••${provider.hint}`} mono />
            {provider.baseUrl && (
              <Field label="Adres" value={provider.baseUrl} mono />
            )}
            <Field
              label="Son güncelleme"
              value={formatDate(provider.updatedAt)}
            />
          </dl>
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
          <div className="shrink-0">
            {/* "Emin misiniz?" idi: neyin silineceğini de sonucunu da
                söylemiyordu. Kullanıcı neye razı olduğunu bilmeli. */}
            <ConfirmInline
              question={
                <>
                  <strong>{provider.name}</strong> erişimi silinsin mi?
                </>
              }
              consequence="Bu erişimi kullanan projeler kimlik doğrulamasız kalır."
              busy={remove.isPending}
              onConfirm={() => remove.mutate()}
              onCancel={() => setConfirming(false)}
            />
          </div>
        )}
      </div>

      {editing && (
        <GitProviderForm provider={provider} onDone={() => setEditing(false)} />
      )}
    </PanelCard>
  );
}

function GitProviderForm({
  provider,
  onDone,
}: {
  provider?: GitProvider;
  onDone: () => void;
}) {
  const queryClient = useQueryClient();
  const editing = provider !== undefined;

  const [type, setType] = useState<GitProviderType>(provider?.type ?? "github");
  const [name, setName] = useState(provider?.name ?? "");
  const [baseUrl, setBaseUrl] = useState(provider?.baseUrl ?? "");
  const [username, setUsername] = useState(provider?.username ?? "");
  const [secret, setSecret] = useState("");

  const spec = TYPES[type];

  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.gitProviders.update(provider.id, {
            name: name.trim(),
            baseUrl: baseUrl.trim(),
            username: username.trim(),
            secret: secret.trim() || undefined,
          })
        : api.gitProviders.create({
            type,
            name: name.trim(),
            baseUrl: baseUrl.trim() || undefined,
            username: username.trim() || undefined,
            secret: secret.trim(),
          }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["git-providers"] });
      onDone();
    },
  });

  const canSubmit =
    name.trim() !== "" &&
    (!spec.needsUsername || username.trim() !== "") &&
    (!spec.needsBaseURL || baseUrl.trim() !== "") &&
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
            onChange={(e) => setType(e.target.value as GitProviderType)}
          >
            {(Object.keys(TYPES) as GitProviderType[]).map((t) => (
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
          placeholder={spec.label}
          onChange={(e) => setName(e.target.value)}
        />
      </label>

      <label className="block">
        <span className="text-xs text-ink-2">
          Servis adresi{spec.needsBaseURL ? "" : " (kendi sunucunuzdaysa)"}
        </span>
        <Input
          className="mt-1 font-mono"
          value={baseUrl}
          placeholder={
            type === "github"
              ? "https://git.sirket.local/api/v3"
              : "https://git.sirket.local"
          }
          onChange={(e) => setBaseUrl(e.target.value)}
        />
      </label>

      {spec.needsUsername && (
        <label className="block">
          <span className="text-xs text-ink-2">Kullanıcı adı</span>
          <Input
            className="mt-1"
            value={username}
            placeholder="kullanici-adi"
            onChange={(e) => setUsername(e.target.value)}
          />
        </label>
      )}

      <label className="block">
        <span className="text-xs text-ink-2">
          {spec.secretLabel}
          {editing && " (boş bırakırsanız mevcut değer korunur)"}
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

      {!spec.verifiable && (
        <Notice tone="warning">
          Bu tür için doğrulama yapılamıyor. Bilgiler kaydedilir ama doğru olup
          olmadıkları ancak ilk gerçek kullanımda anlaşılır.
        </Notice>
      )}

      {save.isError && <FormError error={save.error} context="git" />}

      <div className="flex items-center gap-2 pt-1">
        <Button
          type="submit"
          variant="primary"
          disabled={!canSubmit || save.isPending}
        >
          {save.isPending
            ? "Kaydediliyor…"
            : spec.verifiable
              ? "Doğrula ve kaydet"
              : "Kaydet"}
        </Button>
        <Button type="button" onClick={onDone} disabled={save.isPending}>
          Vazgeç
        </Button>
      </div>
    </form>
  );

  return editing ? <div className="mt-4">{body}</div> : <PanelCard>{body}</PanelCard>;
}

