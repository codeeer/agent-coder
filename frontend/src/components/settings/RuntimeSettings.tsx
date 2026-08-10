"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { SettingValue } from "@/lib/types";
import { Badge, Button, Card, Input, Notice } from "@/components/ui/primitives";

/**
 * Çalışma ayarları bölümü.
 *
 * Bu bileşen hangi ayarların var olduğunu BİLMEZ — listeyi backend'deki kayıt
 * defterinden alır ve etiket, açıklama, birim, aralık bilgisiyle kendini çizer.
 * Yeni bir parametre eklendiğinde buraya dokunmak gerekmez.
 */
export function RuntimeSettings({
  /**
   * Yalnızca bu grupları çiz. Verilmezse hepsi.
   *
   * Ayarlar ekranı her grubu AİT OLDUĞU bölümde gösteriyor: "Jira tarama
   * aralığı" Jira erişiminin yanında, "MCP süre sınırı" MCP sunucularının
   * yanında. Hepsini tek bir "Çalışma ayarları" yığınına koymak, kullanıcının
   * bir şeyi ayarlamak için iki ayrı yere bakması demekti.
   */
  groups: only,
  /** Grup başlıkları — tek gruplu bölümde başlık tekrar olur. */
  showHeadings = true,
}: {
  groups?: string[];
  showHeadings?: boolean;
} = {}) {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["settings"],
    queryFn: api.settings.list,
  });

  if (isPending) return <Notice>Ayarlar yükleniyor…</Notice>;
  if (isError) return <Notice tone="error">{describeError(error).message}</Notice>;

  // Ayarları gruplarına göre böl; sıra kayıt defterinden gelir.
  const groups = new Map<string, SettingValue[]>();
  for (const item of data.items) {
    if (only && !only.includes(item.group)) continue;
    const list = groups.get(item.group) ?? [];
    list.push(item);
    groups.set(item.group, list);
  }

  if (groups.size === 0) return null;

  return (
    <div className="space-y-6">
      {[...groups.entries()].map(([group, items]) => (
        <div key={group}>
          {showHeadings && (
            <h3 className="text-sm font-medium text-ink-2">
              {data.groups[group] ?? group}
            </h3>
          )}
          <div className={showHeadings ? "mt-2 space-y-2" : "space-y-2"}>
            {items.map((item) => (
              <SettingRow key={item.key} setting={item} />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

function SettingRow({ setting }: { setting: SettingValue }) {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(setting.value);

  // Sunucudan gelen değer değişirse (kaydetme veya sıfırlama sonrası) taslağı eşitle.
  useEffect(() => setDraft(setting.value), [setting.value]);

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ["settings"] });
  };

  const save = useMutation({
    mutationFn: () => api.settings.set(setting.key, draft.trim()),
    onSuccess: invalidate,
  });

  const reset = useMutation({
    mutationFn: () => api.settings.reset(setting.key),
    onSuccess: invalidate,
  });

  const changed = draft.trim() !== setting.value;
  const busy = save.isPending || reset.isPending;

  return (
    <Card>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="font-medium">{setting.label}</span>
            {setting.isCustom && <Badge tone="info">değiştirilmiş</Badge>}
          </div>
          {/* Ham anahtar (`runner.timeout_minutes`) ekrandan kaldırıldı:
              kullanıcının işine yaramıyor, on ayarın yanında tekrar edince
              sayfayı geliştirici ekranına çeviriyordu. Destek için başlıkta
              duruyor. */}
          <p className="mt-1 max-w-prose text-sm text-ink-2" title={setting.key}>
            {setting.help}
          </p>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          <div className="w-28">
            <Input
              type={setting.kind === "int" ? "number" : "text"}
              /* Etiket ayrı bir kutuda duruyor; ekran okuyucu kutuyu tek
                 başına okuduğunda "düzenle, 30" diyordu. */
              aria-label={setting.unit ? `${setting.label} (${setting.unit})` : setting.label}
              value={draft}
              min={setting.min}
              max={setting.max}
              onChange={(e) => setDraft(e.target.value)}
              disabled={busy}
            />
          </div>
          {setting.unit && (
            <span className="w-16 text-sm text-ink-2">{setting.unit}</span>
          )}

          {/* Düğmeler yalnızca yapılacak bir iş varken çizilir. On ayarın
              yanında sürekli sönük duran on "Kaydet", sayfayı hem kalabalık
              gösteriyor hem de hangisinin tıklanabilir olduğunu gizliyordu. */}
          {changed && (
            <Button variant="primary" onClick={() => save.mutate()} disabled={busy}>
              {save.isPending ? "Kaydediliyor…" : "Kaydet"}
            </Button>
          )}

          {!changed && save.isSuccess && (
            <span className="text-sm text-ok">Kaydedildi</span>
          )}

          {!changed && !save.isSuccess && setting.isCustom && (
            <Button onClick={() => reset.mutate()} disabled={busy}>
              {reset.isPending ? "…" : "Sıfırla"}
            </Button>
          )}
        </div>
      </div>

      <p className="mt-2 text-xs text-ink-3">
        Varsayılan: {setting.default}
        {setting.min !== undefined && setting.max !== undefined && (
          <> · İzin verilen aralık: {setting.min}–{setting.max}</>
        )}
      </p>

      {(save.isError || reset.isError) && (
        <p className="mt-2 text-sm text-danger">
          {describeError(save.error ?? reset.error).message}
        </p>
      )}
    </Card>
  );
}
