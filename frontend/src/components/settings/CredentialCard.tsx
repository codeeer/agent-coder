"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { ApiError, api } from "@/lib/api";
import type { Credential, CredentialKind } from "@/lib/types";
import { Badge, Button, Input, PanelCard, formatDate } from "@/components/ui/primitives";

/** Bir kimlik bilgisi türünün arayüzde nasıl görüneceği. */
export interface CredentialSpec {
  kind: CredentialKind;
  title: string;
  description: string;
  secretLabel: string;
  placeholder: string;
  helpUrl?: string;
  /** Gizli değere ek olarak istenen alanlar (Jira: base_url, email). */
  extraFields?: { name: string; label: string; placeholder: string }[];
  /** Bu faz içinde henüz kullanılmıyorsa gösterilecek not. */
  unusedNote?: string;
  /*
   * Değer kaydedilmeden önce GERÇEKTEN sınanıyor mu?
   *
   * Her türün doğrulanabilir bir ucu yok: Jira'nın `myself` ucu anahtarın
   * çalıştığını söylüyor, paket deposu token'ı için böyle bir uç yok ve
   * backend onu sessizce kabul ediyor (`credentials.Validator`).
   *
   * Bayrak BİLEREK "doğrulanır" tarafına varsayılan DEĞİL: varsayılan
   * doğruysa, doğrulaması olmayan yeni bir tür eklendiğinde arayüz
   * kendiliğinden yanlış söz vermiş olur. Sözü, sözü tutabilen tür verir.
   */
  verifies?: boolean;
}

export function CredentialCard({
  spec,
  credential,
}: {
  spec: CredentialSpec;
  credential: Credential | undefined;
}) {
  const configured = credential !== undefined;
  const [editing, setEditing] = useState(false);

  return (
    <PanelCard>
      <div className="flex items-start justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <h2 className="font-medium">{spec.title}</h2>
            {configured ? (
              <Badge tone="success">tanımlı</Badge>
            ) : (
              <Badge>tanımlı değil</Badge>
            )}
            {spec.unusedNote && <Badge tone="info">{spec.unusedNote}</Badge>}
          </div>
          <p className="mt-1 max-w-prose text-sm text-ink-2">
            {spec.description}
          </p>
        </div>

        {!editing && (
          <div className="flex shrink-0 gap-2">
            <Button onClick={() => setEditing(true)}>
              {configured ? "Değiştir" : "Ekle"}
            </Button>
            {configured && <DeleteButton kind={spec.kind} />}
          </div>
        )}
      </div>

      {configured && !editing && (
        <dl className="mt-4 flex flex-wrap gap-x-8 gap-y-2 text-sm">
          <div>
            <dt className="text-xs text-ink-2">Değer</dt>
            {/* Gizli değer kaydedildikten sonra bir daha gösterilmez. */}
            <dd className="mt-0.5 font-mono">••••••••{credential.hint}</dd>
          </div>
          <div>
            <dt className="text-xs text-ink-2">Son güncelleme</dt>
            <dd className="mt-0.5">{formatDate(credential.updatedAt)}</dd>
          </div>
          {Object.entries(credential.metadata).map(([k, v]) => (
            <div key={k}>
              <dt className="text-xs text-ink-2">{k}</dt>
              <dd className="mt-0.5 font-mono">{v}</dd>
            </div>
          ))}
        </dl>
      )}

      {editing && (
        <CredentialForm
          spec={spec}
          existing={credential}
          onDone={() => setEditing(false)}
        />
      )}
    </PanelCard>
  );
}

function CredentialForm({
  spec,
  existing,
  onDone,
}: {
  spec: CredentialSpec;
  existing: Credential | undefined;
  onDone: () => void;
}) {
  const queryClient = useQueryClient();
  const [secret, setSecret] = useState("");
  const [extras, setExtras] = useState<Record<string, string>>(
    () => existing?.metadata ?? {},
  );

  const save = useMutation({
    mutationFn: () =>
      api.credentials.put(spec.kind, {
        secret: secret.trim(),
        metadata: extras,
      }),
    onSuccess: () => {
      // Anahtar değişince model kataloğu da tazelenebilir.
      void queryClient.invalidateQueries({ queryKey: ["credentials"] });
      void queryClient.invalidateQueries({ queryKey: ["models"] });
      onDone();
    },
  });

  const canSubmit =
    secret.trim().length > 0 &&
    (spec.extraFields ?? []).every((f) => (extras[f.name] ?? "").trim() !== "");

  return (
    <form
      className="mt-4 space-y-3"
      onSubmit={(e) => {
        e.preventDefault();
        if (canSubmit) save.mutate();
      }}
    >
      {spec.extraFields?.map((f) => (
        <label key={f.name} className="block">
          <span className="text-xs text-ink-2">{f.label}</span>
          <Input
            className="mt-1"
            value={extras[f.name] ?? ""}
            placeholder={f.placeholder}
            onChange={(e) =>
              setExtras((prev) => ({ ...prev, [f.name]: e.target.value }))
            }
          />
        </label>
      ))}

      <label className="block">
        <span className="text-xs text-ink-2">{spec.secretLabel}</span>
        <Input
          className="mt-1 font-mono"
          type="password"
          autoComplete="off"
          value={secret}
          placeholder={spec.placeholder}
          onChange={(e) => setSecret(e.target.value)}
        />
      </label>

      {spec.helpUrl && (
        <p className="text-xs text-ink-2">
          Anahtarınızı buradan alabilirsiniz:{" "}
          <a
            href={spec.helpUrl}
            target="_blank"
            rel="noreferrer"
            className="underline"
          >
            {spec.helpUrl}
          </a>
        </p>
      )}

      {save.isError && <SaveError error={save.error} />}

      <div className="flex items-center gap-2 pt-1">
        <Button type="submit" variant="primary" disabled={!canSubmit || save.isPending}>
          {save.isPending
            ? spec.verifies
              ? "Doğrulanıyor…"
              : "Kaydediliyor…"
            : spec.verifies
              ? "Doğrula ve kaydet"
              : "Kaydet"}
        </Button>
        <Button type="button" onClick={onDone} disabled={save.isPending}>
          Vazgeç
        </Button>
        <span className="text-xs text-ink-2">
          {spec.verifies
            ? "Kaydetmeden önce değerin gerçekten çalıştığı sınanır."
            : "Değer sınanmadan kaydedilir; yanlışsa ilk çalıştırmada görülür."}
        </span>
      </div>
    </form>
  );
}

/**
 * Kaydetme hatasını kullanıcının yapabileceği eyleme çevirir.
 *
 * "Değer yanlış" ile "servise ulaşılamadı" ayrımı korunur: birinde değeri
 * düzeltmek, diğerinde tekrar denemek gerekir.
 */
function SaveError({ error }: { error: unknown }) {
  const { message, hint } = describeError(error);
  return (
    <div className="rounded border border-danger/35 bg-danger-soft px-3 py-2 text-sm">
      <p className="font-medium text-danger">{message}</p>
      {hint && <p className="mt-0.5 text-xs text-ink-2">{hint}</p>}
    </div>
  );
}

function describeError(error: unknown): { message: string; hint?: string } {
  if (!(error instanceof ApiError)) {
    return { message: "Beklenmeyen bir hata oluştu" };
  }
  switch (error.code) {
    case "invalid_credential":
      return {
        message: "Değer doğrulanamadı",
        hint: "Anahtarı kopyalarken eksik veya fazla karakter kalmış olabilir.",
      };
    case "service_unreachable":
      return {
        message: "Doğrulama servisine ulaşılamadı",
        hint: "İnternet bağlantınızı kontrol edip tekrar deneyin.",
      };
    case "missing_metadata":
      return { message: error.message };
    default:
      return { message: error.message };
  }
}

function DeleteButton({ kind }: { kind: CredentialKind }) {
  const queryClient = useQueryClient();
  const [confirming, setConfirming] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.credentials.remove(kind),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["credentials"] });
      void queryClient.invalidateQueries({ queryKey: ["models"] });
      setConfirming(false);
    },
  });

  if (!confirming) {
    return (
      <Button variant="danger" onClick={() => setConfirming(true)}>
        Sil
      </Button>
    );
  }

  return (
    <div className="flex items-center gap-2">
      <span className="text-xs text-ink-2">Emin misiniz?</span>
      <Button
        variant="danger"
        onClick={() => remove.mutate()}
        disabled={remove.isPending}
      >
        {remove.isPending ? "Siliniyor…" : "Evet, sil"}
      </Button>
      <Button onClick={() => setConfirming(false)} disabled={remove.isPending}>
        Vazgeç
      </Button>
    </div>
  );
}
