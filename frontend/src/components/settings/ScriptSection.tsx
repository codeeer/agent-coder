"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { PROJECT_DIR, scriptPath, type Script } from "@/lib/types";
import { IconPlus } from "@/components/ui/icons";
import { Pagination } from "@/components/ui/Pagination";
import { ScriptFolders } from "./ScriptFolders";
import {
  Badge,
  Button,
  PanelCard,
  Input,
  Mono,
  Notice,
  Panel,
  Select,
  Skeleton,
  Textarea,
  Well,
  formatDate,
} from "@/components/ui/primitives";

const PAGE_SIZE = 10;

/**
 * Betik kütüphanesi — agent'ların çalıştırabileceği hazır prosedürler.
 *
 * NEDEN VAR: model bir işi her seferinde yeniden yorumlar. Keşifte doğru olan
 * bu davranış, prosedürde (yükseltme, geçiş, kontrol listesi) risktir. Betik bir
 * kez yazılır ve her çalıştığında aynı şeyi yapar.
 *
 * Sınır kullanıcıya da anlatılır: model betiği NE ZAMAN çağıracağına karar
 * verir, NE YAPACAĞINA betik karar verir.
 */
export function ScriptSection() {
  const [adding, setAdding] = useState(false);
  const [offset, setOffset] = useState(0);

  const scripts = useQuery({
    queryKey: ["scripts", offset],
    queryFn: () => api.scripts.list({ limit: PAGE_SIZE, offset }),
  });

  return (
    <Panel
      title="Script'ler"
      description="Agent'ların çalıştırabileceği hazır kabuk betikleri. Hangi agent'ın hangi betiği kullanabileceğini Agent'lar ekranından seçersiniz."
      action={
        !adding && (
          <Button
            variant="primary"
            icon={<IconPlus className="size-4" />}
            onClick={() => setAdding(true)}
          >
            Betik ekle
          </Button>
        )
      }
    >
      <div className="space-y-3">
        <ScriptFolders />

        <div className="border-t border-line pt-3">
          <p className="text-sm font-medium">Betikler</p>
        </div>

        {adding && <ScriptForm onDone={() => setAdding(false)} />}

        {scripts.isPending && <Skeleton rows={2} />}
        {scripts.isError && (
          <Notice tone="error">{describeError(scripts.error).message}</Notice>
        )}

        {scripts.data?.total === 0 && !adding && (
          <PanelCard>
            <p className="text-sm text-ink-2">
              Henüz betik yok. Bir agent standart bir işi &mdash; bağımlılık
              yükseltme, geçiş uygulama, kontrol listesi &mdash; her seferinde
              biraz farklı yapabilir. Betik bunu sabitler:{" "}
              <strong>model ne zaman çağıracağına karar verir, ne yapacağına
              betik karar verir.</strong>
            </p>
          </PanelCard>
        )}

        {scripts.data?.items.map((s) => (
          <ScriptCard key={s.id} script={s} />
        ))}

        {scripts.data && (
          <Pagination
            total={scripts.data.total}
            limit={scripts.data.limit}
            offset={scripts.data.offset}
            onChange={setOffset}
            unit="betik"
          />
        )}

        {/* Sınırın kendisi bir bilgi: kullanıcı betiğini yazıp neden hiç
            çalışmadığını aramasın. */}
        <p className="text-xs text-ink-3">
          Betikler yalnızca <strong>komut çalıştırma yetkisi açık</strong>{" "}
          agent&apos;lara verilir. Yetkisi kapalı bir agent&apos;ın ortamına
          kopyalanmazlar.
        </p>
      </div>
    </Panel>
  );
}

function ScriptCard({ script }: { script: Script }) {
  const qc = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.scripts.remove(script.id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["scripts"] });
      void qc.invalidateQueries({ queryKey: ["agents"] });
    },
  });

  if (editing) {
    return <ScriptForm script={script} onDone={() => setEditing(false)} />;
  }

  return (
    <PanelCard>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{script.name}</span>
            {script.folderName && <Badge tone="neutral">{script.folderName}</Badge>}
          </div>
          {script.description && (
            <p className="mt-1 text-sm text-ink-2">{script.description}</p>
          )}
          {/* Agent'ın göreceği yol: kullanıcı talimatında ona atıfta bulunmak
              isterse aynı metni kullanabilmeli. */}
          <p className="mt-1 font-mono text-xs break-all text-ink-3">
            {scriptPath(script)}
          </p>
          <p className="mt-1 text-2xs text-ink-3">
            Güncellendi: {formatDate(script.updatedAt)}
          </p>
        </div>

        {!confirming ? (
          <div className="flex shrink-0 gap-2">
            <Button size="sm" onClick={() => setEditing(true)}>
              Düzenle
            </Button>
            <Button size="sm" variant="danger" onClick={() => setConfirming(true)}>
              Sil
            </Button>
          </div>
        ) : (
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-xs text-ink-2">
              Agent&apos;lardan da kaldırılacak.
            </span>
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
        )}
      </div>

      {remove.isError && (
        <Notice tone="error">{describeError(remove.error).message}</Notice>
      )}
    </PanelCard>
  );
}

function ScriptForm({ script, onDone }: { script?: Script; onDone: () => void }) {
  const qc = useQueryClient();
  const editing = script !== undefined;

  const [name, setName] = useState(script?.name ?? "");
  const [description, setDescription] = useState(script?.description ?? "");
  const [folderId, setFolderId] = useState(script?.folderId ?? "");
  const [content, setContent] = useState(
    script?.content ?? `#!/usr/bin/env bash\nset -euo pipefail\n\ncd "$PROJECT_DIR"\n`,
  );

  const folders = useQuery({
    queryKey: ["script-folders"],
    queryFn: api.scriptFolders.list,
  });

  const secilenKlasor = folders.data?.items.find((f) => f.id === folderId);

  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.scripts.update(script.id, {
            name: name.trim(),
            description: description.trim(),
            content,
            folderId: folderId || undefined,
            // Klasörden çıkarmak ile "dokunma" JSON'da aynı görünüyor;
            // ayrı bir bayrak olmadan ayırt edilemez.
            clearFolder: folderId === "",
          })
        : api.scripts.create({
            name: name.trim(),
            description: description.trim(),
            content,
            folderId: folderId || undefined,
          }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["scripts"] });
      void qc.invalidateQueries({ queryKey: ["script-folders"] });
      onDone();
    },
  });

  return (
    <PanelCard>
      <p className="text-sm font-medium">
        {editing ? "Betiği düzenle" : "Yeni betik"}
      </p>

      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">Ad</span>
          <Input
            className="mt-1"
            value={name}
            placeholder="upgrade-deps"
            onChange={(e) => setName(e.target.value)}
          />
          {/* Adın dosya adına dönüştüğünü söylemek gerekiyor: kullanıcı neden
              boşluk ve büyük harf kabul edilmediğini yoksa anlamaz. */}
          <span className="mt-1 block text-2xs text-ink-3">
            Dosya adı olur:{" "}
            <span className="font-mono break-all">
              {scriptPath({
                name: name.trim() || "upgrade-deps",
                folderName: secilenKlasor?.name,
              })}
            </span>
            . Küçük harf, rakam ve - kullanılabilir.
          </span>
        </label>

        {/* Klasör seçimi: kampanyaya ait betikler bir arada dursun.
            Klasörsüz bırakmak MEŞRU — birden fazla kampanyada işe yarayan
            ortak betikler oraya kopyalanmamalı. */}
        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">
            Klasör
          </span>
          <Select
            className="mt-1"
            value={folderId}
            onChange={(e) => setFolderId(e.target.value)}
          >
            <option value="">Klasörsüz (ortak betik)</option>
            {folders.data?.items.map((f) => (
              <option key={f.id} value={f.id}>
                {f.name}
              </option>
            ))}
          </Select>
          <span className="mt-1 block text-2xs text-ink-3">
            Bir kampanyanın adımları klasörde toplanır ve agent&apos;a tek
            seçimle bağlanır.
          </span>
        </label>

        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">
            Ne işe yarar
          </span>
          <Input
            className="mt-1"
            value={description}
            placeholder="Bağımlılıkları güvenli sürümlere yükseltir"
            onChange={(e) => setDescription(e.target.value)}
          />
          {/* Açıklama süs değil: agent'ın talimatına yazılıyor ve betiğin ne
              zaman çağrılacağını modele anlatan tek ipucu bu. */}
          <span className="mt-1 block text-2xs text-ink-3">
            Agent&apos;ın talimatına yazılır — betiği <strong>ne zaman</strong>{" "}
            çağıracağını buradan anlar.
          </span>
        </label>
      </div>

      <label className="mt-3 block">
        <span className="text-2xs tracking-wide text-ink-2 uppercase">İçerik</span>
        <Textarea
          className="mt-1 min-h-56 font-mono text-xs leading-relaxed"
          value={content}
          spellCheck={false}
          onChange={(e) => setContent(e.target.value)}
        />
      </label>

      <Well className="mt-3 p-3">
        <p className="text-xs">
          <strong>Betiğe gizli değer yazmayın.</strong> Betikler şifrelenmez ve
          agent onları okuyabilir. Token gerekiyorsa ortam değişkeninden okuyun:{" "}
          <Mono>&quot;$GIT_TOKEN&quot;</Mono>
        </p>
        <p className="mt-2 text-2xs text-ink-2">
          Hata durumunda durması için <Mono>set -euo pipefail</Mono> önerilir.
        </p>
      </Well>

      {/* PROJE DİZİNİ — betik yazarının bilmediği tek şey bu.
          Kullanıcı projesinin İÇİNDEKİ yolu biliyor; kökün nereye açıldığını
          kaynağı okumadan öğrenemezdi. */}
      <Well className="mt-3 p-3">
        <p className="text-xs">
          <strong>Proje <Mono>{PROJECT_DIR}</Mono> altına klonlanır</strong> ve
          bu yol betiğe <Mono>$PROJECT_DIR</Mono> olarak geçer. Çalışma dizinine
          güvenmeyin, bu değişkeni kullanın:{" "}
          <Mono>&quot;$PROJECT_DIR/config/webpack.config.js&quot;</Mono>
        </p>
        <p className="mt-2 text-2xs text-ink-2">
          Kalıcı olması gereken her değişiklik bu dizinin altında olmalı —
          başka bir yere yazılan dosya değişiklik kaydına girmez ve
          çalıştırma bitince kaybolur.
        </p>
      </Well>

      {save.isError && <Notice tone="error">{describeError(save.error).message}</Notice>}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Button
          variant="primary"
          onClick={() => save.mutate()}
          disabled={save.isPending || !name.trim() || !content.trim()}
        >
          {save.isPending ? "Kaydediliyor…" : "Kaydet"}
        </Button>
        <Button onClick={onDone} disabled={save.isPending}>
          Vazgeç
        </Button>
        <span className="text-xs text-ink-3">
          Değişiklik bir sonraki çalıştırmada geçerli olur.
        </span>
      </div>
    </PanelCard>
  );
}
