"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { McpServer, McpTransport } from "@/lib/types";
import { IconPlus } from "@/components/ui/icons";
import {
  Badge,
  Button,
  PanelCard,
  Input,
  Notice,
  Panel,
  Select,
  Skeleton,
  formatDate,
} from "@/components/ui/primitives";

/**
 * MCP sunucuları — agent'ların erişebileceği dış araçlar.
 *
 * Kaydetme sunucuya BAĞLANMADAN tamamlanmaz: adres veya anahtar yanlışsa
 * kullanıcı bunu burada öğrenir, bir agent araçsız kaldığında değil.
 *
 * Araç listesi gösterilir çünkü kullanıcı bir agent'a erişim verirken NEYE
 * erişim verdiğini görmeden karar veremez.
 */
export function McpServerSection() {
  const [adding, setAdding] = useState(false);

  const servers = useQuery({ queryKey: ["mcp-servers"], queryFn: api.mcpServers.list });

  return (
    <Panel
      title="Dış araçlar (MCP)"
      description="Agent'ların erişebileceği dış araç sunucuları. Bir sunucu tanımladıktan sonra hangi agent'ların kullanabileceğini Agent'lar ekranından seçersiniz."
      action={
        !adding && (
          <Button
            variant="primary"
            icon={<IconPlus className="size-4" />}
            onClick={() => setAdding(true)}
          >
            Sunucu ekle
          </Button>
        )
      }
    >
      <div className="space-y-3">
        {adding && <ServerForm onDone={() => setAdding(false)} />}

        {servers.isPending && <Skeleton rows={2} />}
        {servers.isError && (
          <Notice tone="error">{describeError(servers.error).message}</Notice>
        )}

        {servers.data?.length === 0 && !adding && (
          <PanelCard>
            <p className="text-sm text-ink-2">
              Henüz sunucu yok. MCP, bir agent&apos;ın hata takip sistemi, dokümantasyon
              veya veritabanı şeması gibi dış kaynaklara <strong>standart bir
              protokolle</strong> erişmesini sağlar — her kaynak için ayrı kod
              yazmadan.
            </p>
          </PanelCard>
        )}

        {servers.data?.map((s) => (
          <ServerCard key={s.id} server={s} />
        ))}

        {/* Sınırın kendisi bir bilgi: kullanıcı yerel bir sunucu adresi girip
            neden kabul edilmediğini aramasın. */}
        <p className="text-xs text-ink-3">
          Yalnızca uzak sunucular (HTTP/SSE) desteklenir. Bilgisayarda komut olarak
          çalışan (stdio) sunucular için henüz destek yok.
        </p>
      </div>
    </Panel>
  );
}

function ServerCard({ server }: { server: McpServer }) {
  const qc = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.mcpServers.remove(server.id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      void qc.invalidateQueries({ queryKey: ["agents"] });
    },
  });

  if (editing) {
    return <ServerForm server={server} onDone={() => setEditing(false)} />;
  }

  return (
    <PanelCard>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{server.name}</span>
            <Badge>{server.transport.toUpperCase()}</Badge>
            {server.hasSecret ? (
              <Badge tone="info">anahtar ••••{server.hint}</Badge>
            ) : (
              <Badge>anahtarsız</Badge>
            )}
          </div>
          <p className="mt-1 font-mono text-xs break-all text-ink-2">{server.url}</p>
          <p className="mt-1 text-2xs text-ink-3">
            Son doğrulama: {formatDate(server.updatedAt)}
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
            <span className="text-xs text-ink-2">Agent&apos;lardan da kaldırılacak.</span>
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

      <ToolList tools={server.tools} />
    </PanelCard>
  );
}

/** Sunucunun sunduğu araçlar — agent'a ne verdiğinizin listesi. */
function ToolList({ tools }: { tools: string[] }) {
  if (tools.length === 0) {
    return (
      <p className="mt-3 text-xs text-ink-3">
        Bu sunucu hiç araç bildirmedi.
      </p>
    );
  }

  return (
    <div className="mt-3 flex flex-wrap items-center gap-1.5">
      <span className="text-2xs tracking-wide text-ink-3 uppercase">
        {tools.length} araç
      </span>
      {tools.map((t) => (
        <span
          key={t}
          className="rounded border border-line bg-raised px-1.5 py-0.5 font-mono text-2xs text-ink-2"
        >
          {t}
        </span>
      ))}
    </div>
  );
}

function ServerForm({ server, onDone }: { server?: McpServer; onDone: () => void }) {
  const qc = useQueryClient();
  const editing = server !== undefined;

  const [name, setName] = useState(server?.name ?? "");
  const [transport, setTransport] = useState<McpTransport>(server?.transport ?? "http");
  const [url, setUrl] = useState(server?.url ?? "");
  const [secret, setSecret] = useState("");
  // Anahtarı silmek ayrı bir niyet: boş bırakmak "değiştirme" demek.
  const [clearSecret, setClearSecret] = useState(false);

  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.mcpServers.update(server.id, {
            name: name.trim(),
            transport,
            url: url.trim(),
            secret: secret.trim() || undefined,
            clearSecret,
          })
        : api.mcpServers.create({
            name: name.trim(),
            transport,
            url: url.trim(),
            secret: secret.trim() || undefined,
          }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["mcp-servers"] });
      onDone();
    },
  });

  return (
    <PanelCard>
      <p className="text-sm font-medium">
        {editing ? "Sunucuyu düzenle" : "Yeni MCP sunucusu"}
      </p>

      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">Ad</span>
          <Input
            className="mt-1"
            value={name}
            placeholder="sentry"
            onChange={(e) => setName(e.target.value)}
          />
          {/* Adın araç adlarına önek olduğunu söylemek gerekiyor: kullanıcı
              neden nokta ve boşluk kabul edilmediğini yoksa anlamaz. */}
          <span className="mt-1 block text-2xs text-ink-3">
            Araç adlarının öneki olur: <span className="font-mono">{name.trim() || "sentry"}_issue</span>.
            Harf, rakam, - ve _ kullanılabilir.
          </span>
        </label>

        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">Taşıma</span>
          <Select
            className="mt-1"
            value={transport}
            onChange={(e) => setTransport(e.target.value as McpTransport)}
          >
            <option value="http">HTTP (güncel sunucular)</option>
            <option value="sse">SSE (eski sunucular)</option>
          </Select>
        </label>
      </div>

      <label className="mt-3 block">
        <span className="text-2xs tracking-wide text-ink-2 uppercase">Adres</span>
        <Input
          className="mt-1 font-mono text-xs"
          value={url}
          placeholder="https://mcp.ornek.com/mcp"
          onChange={(e) => setUrl(e.target.value)}
        />
      </label>

      <label className="mt-3 block">
        <span className="text-2xs tracking-wide text-ink-2 uppercase">
          Erişim anahtarı
        </span>
        <Input
          className="mt-1"
          type="password"
          value={secret}
          placeholder={
            editing
              ? server.hasSecret
                ? "değiştirmek için yazın — boş bırakırsanız korunur"
                : "anahtarsız"
              : "gerekmiyorsa boş bırakın"
          }
          onChange={(e) => setSecret(e.target.value)}
        />
        <span className="mt-1 block text-2xs text-ink-3">
          Şifreli saklanır ve bir daha tam haliyle gösterilmez. Sunucuya
          <span className="font-mono"> Authorization: Bearer</span> başlığıyla gönderilir.
        </span>
      </label>

      {editing && server.hasSecret && (
        <label className="mt-3 flex items-start gap-2">
          <input
            type="checkbox"
            className="mt-0.5 size-3.5 accent-accent"
            checked={clearSecret}
            onChange={(e) => {
              setClearSecret(e.target.checked);
              if (e.target.checked) setSecret("");
            }}
          />
          <span className="text-xs">
            Anahtarı kaldır
            <span className="mt-0.5 block text-2xs text-ink-3">
              Anahtarsız çalışan sunucular var; bazıları anahtar gönderildiğinde
              isteği reddeder.
            </span>
          </span>
        </label>
      )}

      {save.isError && (
        <Notice tone="error">{describeError(save.error).message}</Notice>
      )}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Button
          variant="primary"
          onClick={() => save.mutate()}
          disabled={save.isPending || !name.trim() || !url.trim()}
        >
          {save.isPending ? "Bağlanılıyor…" : "Doğrula ve kaydet"}
        </Button>
        <Button onClick={onDone} disabled={save.isPending}>
          Vazgeç
        </Button>
        <span className="text-xs text-ink-3">
          Kaydetmeden önce sunucuya bağlanılır ve araçları okunur.
        </span>
      </div>
    </PanelCard>
  );
}
