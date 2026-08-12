"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { SettingValue } from "@/lib/types";
import { Badge, Button, Input, Notice } from "@/components/ui/primitives";

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

  /*
   * Ayarlar AYRI KUTULAR DEĞİL, AYRAÇLI SATIRLAR.
   *
   * Her ayar kendi kenarlıklı kutusundaydı ve bu kutular zaten kenarlıklı
   * bir panonun içinde duruyordu: beş ayarlık bir bölümde altı kenarlık,
   * altı köşe yarıçapı ve aralarında beş boşluk. Bir ayar listesi TEK bir
   * şeydir; her satırı bağımsız bir yüzey yapmak aralarındaki ilişkiyi
   * koparıyor ve ekranı gereksizce uzatıyordu. Aynı karar liste
   * ekranlarında da verildi (bkz. `List`).
   */
  return (
    <div className="divide-y divide-line">
      {[...groups.entries()].map(([group, items]) => (
        <div key={group} className="divide-y divide-line">
          {showHeadings && (
            <h3 className="px-4 py-2 text-2xs font-medium tracking-wide text-ink-3 uppercase">
              {data.groups[group] ?? group}
            </h3>
          )}
          {items.map((item) => (
            <SettingRow key={item.key} setting={item} />
          ))}
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
    <div className="px-4 py-3.5 transition-colors hover:bg-raised/50">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium">{setting.label}</span>
            {setting.isCustom && <Badge tone="info">değiştirilmiş</Badge>}
          </div>
          {/* Ham anahtar (`runner.timeout_minutes`) ekrandan kaldırıldı:
              kullanıcının işine yaramıyor, on ayarın yanında tekrar edince
              sayfayı geliştirici ekranına çeviriyordu. Destek için başlıkta
              duruyor. */}
          <p className="mt-0.5 max-w-prose text-xs text-ink-2" title={setting.key}>
            {setting.help}
          </p>
        </div>

        <div className="flex shrink-0 items-center gap-2">
          {/* Sayı 112px'e sığar, ADRES SIĞMAZ. Kayıt defteri adresi gibi metin
              ayarlarında kutu geniş: dar bir kutuda kullanıcı yazdığının
              tamamını göremiyor ve yanlışı ancak kaydettikten sonra fark
              ediyor. `max-w-full` dar ekranda taşmayı önlüyor. */}
          <div className={setting.kind === "int" ? "w-28" : "w-full max-w-96 sm:w-96"}>
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

      {/* Opsiyonel ayarların varsayılanı YOK: "Varsayılan:" yazıp arkasını
          boş bırakmak, kullanıcıya okunamayan bir satır göstermek olurdu.
          Onun yerine boş bırakmanın ne demek olduğu yazılıyor. */}
      <p className="mt-1.5 text-2xs text-ink-3">
        {setting.default === "" ? (
          <>Boş bırakılabilir</>
        ) : (
          <>Varsayılan: {setting.default}</>
        )}
        {setting.min !== undefined && setting.max !== undefined && (
          <> · İzin verilen aralık: {setting.min}–{setting.max}</>
        )}
      </p>

      {(save.isError || reset.isError) && (
        <p className="mt-2 text-sm text-danger">
          {describeError(save.error ?? reset.error).message}
        </p>
      )}
    </div>
  );
}
