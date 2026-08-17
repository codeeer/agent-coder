"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { truthy, type SettingValue } from "@/lib/types";
import { filterSettings } from "@/components/settings/setting-search";
import {
  Badge,
  Button,
  EmptyState,
  Input,
  Notice,
  Switch,
  Textarea,
} from "@/components/ui/primitives";
import { IconSearch } from "@/components/ui/icons";

// truthy `@/lib/types`'a taşındı: ayarı okuyan ikinci ekran çıkınca kural
// kopyalanmıştı ve kopya daha katıydı (spec 025).

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
  /**
   * Verilirse yalnızca eşleşen ayarlar çizilir.
   *
   * OPSİYONEL, ve öyle kalmalı: bileşen altı ayrı bölümden çağrılıyor ve
   * sorgu verilmediğinde kod yolu bugünküyle birebir aynı olmalı.
   */
  query = "",
  /**
   * Verilirse yalnızca bu anahtarlar çizilir.
   *
   * Grup filtresi yetmiyor: "Kurumsal ağ" grubunda hem sertifika hem çıkış
   * denetimi var ve ikisi ayrı panolara ait — biri SSL denetimi yapan ağlar
   * için, diğeri sandbox'ın nereye çıkabileceği için. Tek panoda toplamak,
   * pano başlığını ikisini birden anlatamaz hâle getiriyordu.
   */
  keys,
}: {
  groups?: string[];
  showHeadings?: boolean;
  query?: string;
  keys?: string[];
} = {}) {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["settings"],
    queryFn: api.settings.list,
  });

  if (isPending) return <Notice>Ayarlar yükleniyor…</Notice>;
  if (isError) return <Notice tone="error">{describeError(error).message}</Notice>;

  // Ayarları gruplarına göre böl; sıra kayıt defterinden gelir.
  const groups = new Map<string, SettingValue[]>();
  for (const item of filterSettings(data.items, query)) {
    if (only && !only.includes(item.group)) continue;
    if (keys && !keys.includes(item.key)) continue;
    const list = groups.get(item.group) ?? [];
    list.push(item);
    groups.set(item.group, list);
  }

  if (groups.size === 0) {
    /* Süzgeç yokken boş sonuç "burada gösterilecek bir ayar yok" demektir ve
       çağıran bölüm kendi başlığını zaten çizmiştir — araya bir kutu koymak
       gürültü olurdu. Süzgeç varken boşluk BİR CEVAPTIR: kullanıcı bir şey
       aradı, sonucu ne olduğu yazılmalı. */
    if (!query.trim()) return null;
    return (
      <EmptyState
        icon={<IconSearch className="size-5" />}
        title="Eşleşme yok"
        description={
          <>
            “{query.trim()}” hiçbir ayarın adında veya açıklamasında geçmiyor.
            Başka bir kelime deneyin.
          </>
        }
      />
    );
  }

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
  const rows = (
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

  /*
   * Süzgeç görünümü KENDİ YÜZEYİNİ taşır.
   *
   * Bölümlerden çağrıldığında satırlar zaten bir `Panel`in içinde duruyor ve
   * yüzey oradan geliyor. Arama sonucu ise bir bölüme ait değil — sonuçların
   * kapsayıcısı olabilecek tek bir bölüm başlığı yok. Yüzeyi çağıran tarafa
   * bıraksaydık "eşleşme yok" durumu, kenarlıklı bir kartın içindeki kesik
   * kenarlıklı ikinci bir kutu olarak çıkardı.
   */
  if (!query.trim()) return rows;
  return (
    <div className="overflow-hidden rounded-card border border-line bg-surface shadow-(--shadow-card)">
      {rows}
    </div>
  );
}

/*
 * HostListField, izinli domain listesi.
 *
 * Sertifika alanının aksine dosya seçme yok — liste elle yazılan bir şey.
 * Buna karşılık YAZIM BİÇİMİ görünür olmalı: kullanıcı `*.` ile alt alan
 * adlarını açabildiğini ve şema/port yazmaması gerektiğini yalnızca yardım
 * metninden öğrenirse ilk denemede hata alır. Örnek, alanın hemen altında
 * duruyor.
 */
function HostListField({
  value,
  onChange,
  disabled,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  label: string;
}) {
  const satirSayisi = value ? value.split("\n").filter((s) => s.trim() && !s.startsWith("#")).length : 0;

  return (
    <div className="space-y-2">
      <Textarea
        aria-label={label}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        rows={value ? 8 : 4}
        spellCheck={false}
        placeholder={"ornek.com\n*.npmjs.org\n# yorum satırı"}
        className="resize-y font-mono text-xs"
      />
      <p className="text-xs text-muted">
        {satirSayisi > 0
          ? `${satirSayisi} domain — şema ve port yazılmaz; alt alan adları için *.ornek.com`
          : "Boş bırakılırsa domain kısıtı uygulanmaz; çıkış yine proxy'den geçer."}
      </p>
    </div>
  );
}

/**
 * Sertifika alanı — yapıştırma ve dosya seçme.
 *
 * DOSYA SEÇMEK TEK BAŞINA KAYDETMEZ: seçilen dosya sunucuda PEM'e çevrilir,
 * dönen metin alana düşer, kullanıcı ne kaydedeceğini GÖRÜR ve normal
 * "Kaydet" akışıyla kaydeder (spec 017 H1). Aksi halde kullanıcı, içeriğini
 * hiç görmediği bir dosyayı sisteme güven olarak tanıtmış olurdu.
 *
 * Çevirme sunucuda yapılıyor: ikili biçimleri (DER, PKCS#7) tarayıcıda
 * ayrıştırmak, aynı mantığın ikinci bir kopyasını ve iki kopyanın aynı sonucu
 * verdiğini garanti etme yükünü getirirdi.
 */
function CertificateField({
  value,
  onChange,
  disabled,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled?: boolean;
  label: string;
}) {
  const dosyaRef = useRef<HTMLInputElement>(null);
  const [hata, setHata] = useState<string | null>(null);

  const cevir = useMutation({
    mutationFn: async (file: File) => {
      // Baytlar base64 olarak taşınır: ikili dosyalar JSON'a doğrudan yazılamaz.
      const b64 = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onerror = () => reject(new Error("dosya okunamadı"));
        reader.onload = () => resolve(String(reader.result).split(",")[1] ?? "");
        reader.readAsDataURL(file);
      });
      return api.network.normalizeCA(b64);
    },
    onSuccess: (r) => {
      setHata(null);
      onChange(r.pem);
    },
    // Hata alanı BOZMAZ: kullanıcının yazdığı/yapıştırdığı içerik yerinde kalır.
    onError: (e) => setHata(describeError(e).message),
  });

  return (
    <div className="space-y-2">
      <Textarea
        aria-label={label}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled || cevir.isPending}
        rows={value ? 8 : 4}
        spellCheck={false}
        placeholder="-----BEGIN CERTIFICATE-----&#10;…&#10;-----END CERTIFICATE-----"
        className="resize-y font-mono text-xs"
      />
      <div className="flex flex-wrap items-center gap-2">
        <input
          ref={dosyaRef}
          type="file"
          className="hidden"
          // Uzantı biçimi GARANTİ ETMEZ (aynı .crt hem metin hem ikili
          // olabilir); bu liste yalnızca dosya seçiciyi daraltır.
          accept=".pem,.crt,.cer,.der,.p7b,.p7c,application/x-pem-file,application/pkix-cert"
          onChange={(e) => {
            const f = e.target.files?.[0];
            if (f) cevir.mutate(f);
            // Aynı dosya art arda seçilebilsin.
            e.target.value = "";
          }}
        />
        <Button
          onClick={() => dosyaRef.current?.click()}
          disabled={disabled || cevir.isPending}
        >
          {cevir.isPending ? "Okunuyor…" : "Dosya seç"}
        </Button>
        {value && (
          <Button onClick={() => onChange("")} disabled={disabled || cevir.isPending}>
            Temizle
          </Button>
        )}
      </div>
      {hata && <p className="text-sm text-danger">{hata}</p>}
    </div>
  );
}

function SettingRow({ setting }: { setting: SettingValue }) {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(setting.value);
  /*
   * Sertifika satırı TAM GENİŞLİĞE geçer.
   *
   * Diğer ayarların denetimi sağdaki dar sütunda hizalanıyor (spec 016'da
   * ölçüldü: dokuz alanın sol kenarı aynı). Çok satırlı bir sertifika o
   * sütuna sıkıştırılamaz — sıkıştırılsaydı ya okunamayacak kadar dar
   * kalırdı ya da sütunu genişletip diğer sekiz satırın hizasını bozardı.
   * Bu yüzden hizaya KATILMAZ, kendi satırına iner.
   */
  /*
   * İzinli domain listesi de çok satırlı: kurumsal bir listede onlarca satır
   * olabiliyor ve tek satırlık bir kutuda ne yazdığı görünmezdi.
   */
  const cokSatirli = setting.kind === "certificate" || setting.kind === "host_list";

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
      <div
        className={`flex flex-wrap items-start justify-between gap-4 ${
          cokSatirli ? "flex-col" : ""
        }`}
      >
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

        <div
          className={`flex items-center gap-2 ${
            cokSatirli ? "w-full flex-wrap items-end" : "shrink-0"
          }`}
        >
          {/* Sayı 112px'e sığar, ADRES SIĞMAZ. Kayıt defteri adresi gibi metin
              ayarlarında kutu geniş: dar bir kutuda kullanıcı yazdığının
              tamamını göremiyor ve yanlışı ancak kaydettikten sonra fark
              ediyor. `max-w-full` dar ekranda taşmayı önlüyor.

              İki durumlu ayar SAYI GENİŞLİĞİNİ paylaşır: anahtar 36px, ama
              kabı sayı alanlarıyla aynı `w-28` olmasa denetim sütununun
              hizası bu tek satırda kırılırdı. */}
          <div
            className={
              cokSatirli
                ? "w-full"
                : setting.kind === "text"
                  ? "w-full max-w-96 sm:w-96"
                  : "w-28"
            }
          >
            {setting.kind === "host_list" ? (
              <HostListField
                value={draft}
                onChange={setDraft}
                disabled={busy}
                label={setting.label}
              />
            ) : cokSatirli ? (
              <CertificateField
                value={draft}
                onChange={setDraft}
                disabled={busy}
                label={setting.label}
              />
            ) : setting.kind === "bool" ? (
              <Switch
                checked={truthy(draft)}
                onChange={(v) => setDraft(String(v))}
                disabled={busy}
                label={setting.label}
              />
            ) : (
              <Input
                type={setting.kind === "int" ? "number" : "text"}
                /* Etiket ayrı bir kutuda duruyor; ekran okuyucu kutuyu tek
                   başına okuduğunda "düzenle, 30" diyordu. */
                aria-label={
                  setting.unit ? `${setting.label} (${setting.unit})` : setting.label
                }
                value={draft}
                min={setting.min}
                max={setting.max}
                onChange={(e) => setDraft(e.target.value)}
                disabled={busy}
              />
            )}
          </div>
          {/* Birim sütunu iki durumlu ayarda DURUM METNİDİR: anahtarın hangi
              tarafta olduğu bir renk ve bir konum, "Açık" ise okunabilir tek
              kanal. İkisi aynı sütunda durur, hiza bozulmaz. */}
          {setting.kind === "bool" && (
            <span className="w-16 text-sm text-ink-2">
              {truthy(draft) ? "Açık" : "Kapalı"}
            </span>
          )}
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
          <>
            Varsayılan:{" "}
            {setting.kind === "bool"
              ? truthy(setting.default)
                ? "Açık"
                : "Kapalı"
              : setting.default}
          </>
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
