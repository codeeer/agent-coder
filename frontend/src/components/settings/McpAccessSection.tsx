"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import {
  Button,
  Card,
  Mono,
  Notice,
  Section,
  Skeleton,
  Well,
} from "@/components/ui/primitives";

/**
 * Agent Coder'ı dışarıya MCP sunucusu olarak açan adres.
 *
 * Ters yön: yukarıdaki bölüm BİZİM dış araçlara bağlanmamızı sağlıyor, bu
 * bölüm başkalarının BİZE bağlanmasını.
 *
 * Adres anahtar niteliğindedir — dış tetikleme adresindeki desenin aynısı.
 */
export function McpAccessSection() {
  const qc = useQueryClient();
  const [copied, setCopied] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const access = useQuery({ queryKey: ["mcp-access"], queryFn: api.mcpAccess.get });

  const rotate = useMutation({
    mutationFn: api.mcpAccess.rotate,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["mcp-access"] });
      setConfirming(false);
    },
  });

  const url = access.data?.url ?? "";

  return (
    <Section
      title="Agent Coder'ı dışarıya aç"
      description="Claude Desktop, Cursor veya başka bir MCP istemcisi buradaki akışları listeleyip başlatabilir."
    >
      <Card>
        {access.isPending && <Skeleton rows={1} />}
        {access.isError && (
          <Notice tone="error">{describeError(access.error).message}</Notice>
        )}

        {access.data && (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <Mono className="min-w-0 flex-1 break-all">{url}</Mono>
              <Button
                size="sm"
                onClick={() => {
                  void navigator.clipboard.writeText(url);
                  setCopied(true);
                  setTimeout(() => setCopied(false), 1500);
                }}
              >
                {copied ? "Kopyalandı" : "Kopyala"}
              </Button>
              {!confirming ? (
                <Button size="sm" variant="danger" onClick={() => setConfirming(true)}>
                  Yenile
                </Button>
              ) : (
                <>
                  <span className="text-xs text-ink-2">
                    Eski adres anında geçersiz olur.
                  </span>
                  <Button
                    size="sm"
                    variant="danger"
                    onClick={() => rotate.mutate()}
                    disabled={rotate.isPending}
                  >
                    {rotate.isPending ? "Yenileniyor…" : "Evet, yenile"}
                  </Button>
                  <Button size="sm" onClick={() => setConfirming(false)}>
                    Vazgeç
                  </Button>
                </>
              )}
            </div>

            <p className="mt-2 text-xs text-ink-3">
              <strong>Bu adres bir anahtardır</strong> — bilen herkes akışlarınızı
              başlatabilir. Paylaşırken dikkat edin; sızdıysa yenileyin.
            </p>

            {/* Kurulumun kendisi belge okumayı gerektirmemeli: yapıştırılacak
                metin doğrudan burada dursun. */}
            <Well className="mt-4 p-3">
              <p className="text-xs font-medium">Claude Desktop kurulumu</p>
              <p className="mt-1 text-2xs text-ink-2">
                Ayarlar → Developer → Edit Config, sonra şunu ekleyin:
              </p>
              <pre className="mt-2 overflow-x-auto font-mono text-2xs leading-relaxed text-ink-2">
{`{
  "mcpServers": {
    "agent-coder": {
      "type": "http",
      "url": "${url}"
    }
  }
}`}
              </pre>
            </Well>

            <p className="mt-3 text-xs text-ink-3">
              Sunulan araçlar: <Mono>akislari_listele</Mono>,{" "}
              <Mono>akis_calistir</Mono>, <Mono>calisma_durumu</Mono>.
              Başlatılan çalışmalar burada da görünür — tetikleyicisi{" "}
              <strong>MCP</strong> yazar.
            </p>
          </>
        )}

        {rotate.isError && (
          <Notice tone="error">{describeError(rotate.error).message}</Notice>
        )}
      </Card>
    </Section>
  );
}
