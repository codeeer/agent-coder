"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { SCRIPT_DIR, type ScriptFolder } from "@/lib/types";
import { IconPlus } from "@/components/ui/icons";
import {
  Badge,
  Button,
  ConfirmStrip,
  Input,
  Notice,
  PanelCard,
} from "@/components/ui/primitives";

/**
 * Kampanya klasörleri (spec 022).
 *
 * Bir klasör, standart bir yükseltmenin adımlarını bir arada tutar ve
 * agent'a tek seçimle bağlanmasını sağlar. Klasöre sonradan eklenen bir betik,
 * o klasörü kullanan agent'larda kendiliğinden geçerli olur — atama
 * tazelenmez.
 */
export function ScriptFolders() {
  const [adding, setAdding] = useState(false);

  const folders = useQuery({
    queryKey: ["script-folders"],
    queryFn: api.scriptFolders.list,
  });

  const liste = folders.data?.items ?? [];

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="text-sm font-medium">Kampanya klasörleri</p>
          <p className="mt-0.5 text-xs text-ink-2">
            Standart bir yükseltmenin adımları bir klasörde toplanır ve
            agent&apos;a tek seçimle bağlanır.
          </p>
        </div>
        {!adding && (
          <Button
            icon={<IconPlus className="size-4" />}
            onClick={() => setAdding(true)}
          >
            Klasör aç
          </Button>
        )}
      </div>

      {adding && <FolderForm onDone={() => setAdding(false)} />}

      {folders.isError && (
        <Notice tone="error">{describeError(folders.error).message}</Notice>
      )}

      {liste.length === 0 && !adding && (
        <p className="text-xs text-ink-3">
          Henüz klasör yok. Yedi adımlık bir Node yükseltmesi gibi çok adımlı
          bir iş varsa adımları bir klasörde toplayın; agent&apos;a tek seçimle
          bağlanır ve sonradan eklediğiniz adım kendiliğinden geçerli olur.
        </p>
      )}

      {liste.map((f) => (
        <FolderCard key={f.id} folder={f} />
      ))}
    </div>
  );
}

function FolderCard({ folder }: { folder: ScriptFolder }) {
  const qc = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.scriptFolders.remove(folder.id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["script-folders"] });
      void qc.invalidateQueries({ queryKey: ["scripts"] });
      void qc.invalidateQueries({ queryKey: ["agents"] });
    },
  });

  if (editing) {
    return <FolderForm folder={folder} onDone={() => setEditing(false)} />;
  }

  return (
    <PanelCard>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{folder.name}</span>
            <Badge tone="neutral">{folder.scriptCount} betik</Badge>
            {folder.agentCount > 0 && (
              <Badge tone="info">{folder.agentCount} agent</Badge>
            )}
          </div>
          {folder.description && (
            <p className="mt-1 text-sm text-ink-2">{folder.description}</p>
          )}
          {/* Dizin yolu: kullanıcı agent talimatında ona atıfta bulunmak
              isterse aynı metni kullanabilmeli. */}
          <p className="mt-1 font-mono text-xs break-all text-ink-3">
            {SCRIPT_DIR}/{folder.name}
          </p>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <Button onClick={() => setEditing(true)}>Düzenle</Button>
          <Button variant="danger" onClick={() => setConfirming(true)}>
            Sil
          </Button>
        </div>
      </div>

      {/*
        Silmenin SONUCU yazılıyor, "emin misiniz?" değil: betikler silinmiyor,
        klasörsüz kalıyor. Kullanıcı neye razı olduğunu bilmeli.
      */}
      {confirming && (
        <ConfirmStrip
          question={`"${folder.name}" klasörü silinsin mi?`}
          consequence={
            folder.scriptCount > 0
              ? `${folder.scriptCount} betik SİLİNMEZ, klasörsüz kalır.` +
                (folder.agentCount > 0
                  ? ` ${folder.agentCount} agent bu klasörü kullanıyor ve bu adımları kaybeder.`
                  : "")
              : "Bu klasör boş."
          }
          confirmLabel="Klasörü sil"
          busy={remove.isPending}
          error={remove.isError ? describeError(remove.error).message : undefined}
          onConfirm={() => remove.mutate()}
          onCancel={() => setConfirming(false)}
        />
      )}

    </PanelCard>
  );
}

function FolderForm({
  folder,
  onDone,
}: {
  folder?: ScriptFolder;
  onDone: () => void;
}) {
  const qc = useQueryClient();
  const editing = folder !== undefined;

  const [name, setName] = useState(folder?.name ?? "");
  const [description, setDescription] = useState(folder?.description ?? "");

  const save = useMutation({
    mutationFn: () => {
      const body = { name: name.trim(), description: description.trim() };
      return editing
        ? api.scriptFolders.update(folder.id, body)
        : api.scriptFolders.create(body);
    },
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["script-folders"] });
      void qc.invalidateQueries({ queryKey: ["scripts"] });
      onDone();
    },
  });

  return (
    <PanelCard>
      <p className="text-sm font-medium">
        {editing ? "Klasörü düzenle" : "Yeni klasör"}
      </p>

      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">Ad</span>
          <Input
            className="mt-1"
            value={name}
            placeholder="node-24-upgrade"
            onChange={(e) => setName(e.target.value)}
          />
          <span className="mt-1 block text-2xs text-ink-3">
            Dizin adı olur:{" "}
            <span className="font-mono break-all">
              {SCRIPT_DIR}/{name.trim() || "node-24-upgrade"}
            </span>
            . Küçük harf, rakam ve - kullanılabilir.
          </span>
        </label>

        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">
            Kampanya ne?
          </span>
          <Input
            className="mt-1"
            value={description}
            placeholder="Node 18'den 24'e standart yükseltme adımları"
            onChange={(e) => setDescription(e.target.value)}
          />
          {/* Açıklama süs değil: agent'ın talimatına yazılıyor ve kampanyanın
              ne olduğunu modele anlatan tek şey bu. Tek tek betik
              açıklamaları işin bütününü anlatmaz. */}
          <span className="mt-1 block text-2xs text-ink-3">
            Agent&apos;ın talimatına yazılır — işin <strong>bütününü</strong>{" "}
            buradan anlar.
          </span>
        </label>
      </div>

      <p className="mt-3 text-2xs text-ink-3">
        Adımların sırası <strong>adlarından</strong> gelir. Sırayı korumak
        için <span className="font-mono">01-</span>,{" "}
        <span className="font-mono">02-</span> gibi önekler kullanın; agent
        talimatında da bu sırayla görür.
      </p>

      {save.isError && (
        <Notice tone="error">{describeError(save.error).message}</Notice>
      )}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Button
          variant="primary"
          onClick={() => save.mutate()}
          disabled={save.isPending || !name.trim()}
        >
          {save.isPending ? "Kaydediliyor…" : "Kaydet"}
        </Button>
        <Button onClick={onDone} disabled={save.isPending}>
          Vazgeç
        </Button>
      </div>
    </PanelCard>
  );
}
