"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { Badge, Notice } from "@/components/ui/primitives";

/**
 * Sandbox çıkış denetiminin durumu (spec 020 H4).
 *
 * NEDEN VAR: ürün, kullanıcı yazmasa da bazı adreslere izin veriyor — LLM
 * sağlayıcı, git repository, paket deposu ve motorun kendi ihtiyaçları.
 * Kullanıcının bilmediği açık bir kapı bırakılmaz; bu bölüm o kapıları
 * gösteriyor.
 *
 * BURADAKİ HER ADRES GERÇEK YAPILANDIRMADAN GELİR. Örnek bir liste
 * gösterilseydi, ekranda görünenle gerçekte açık olan ayrışırdı — ki bu,
 * bölümün var olma sebebini ortadan kaldırırdı.
 */
export function EgressStatus() {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["network", "egress"],
    queryFn: api.network.egress,
  });

  if (isPending) return <Notice>Çıkış denetimi durumu okunuyor…</Notice>;
  if (isError) return <Notice tone="error">{describeError(error).message}</Notice>;

  /*
   * Denetim kapalıyken izinli adres listesi GÖSTERİLMEZ.
   *
   * Gösterilseydi "şu adreslere izinli" diye okunurdu; oysa kapalıyken agent
   * ortamı zaten her adrese çıkabiliyor. Yanlış bir güvence vermek, hiç bilgi
   * vermemekten kötü.
   */
  if (data.proxy.source === "none") {
    return (
      <Notice>
        Çıkış denetimi <strong>kapalı</strong>. Agent ortamı internetteki her
        adrese çıkabilir. Aşağıya bir çıkış proxy&apos;si girerseniz çıkış
        yalnızca o proxy üzerinden yapılır ve izinli domain listesi devreye girer.
      </Notice>
    );
  }

  const gruplar: Array<{ baslik: string; hostlar: string[] | null }> = [
    { baslik: "LLM sağlayıcı", hostlar: data.alwaysAllowed.providers },
    { baslik: "Kod deposu", hostlar: data.alwaysAllowed.repositories },
    { baslik: "Paket deposu", hostlar: data.alwaysAllowed.registries },
    { baslik: "Çalıştırma motoru", hostlar: data.alwaysAllowed.engine },
  ].filter((g) => g.hostlar && g.hostlar.length > 0);

  return (
    <div className="space-y-2.5">
      <p className="flex flex-wrap items-center gap-2 text-sm">
        <Badge tone="success">açık</Badge>
        <span className="text-ink-2">
          {data.proxy.source === "settings"
            ? "Çıkış yalnızca "
            : "Sunucudaki RUNNER_HTTP_PROXY değişkeninden geliyor; çıkış yalnızca "}
          <code className="text-ink-1">{data.proxy.host}</code> üzerinden yapılır.
        </span>
      </p>

      {gruplar.length > 0 && (
        <div className="rounded-lg border border-line">
          <p className="border-b border-line px-3 py-2 text-xs text-ink-3">
            Bu adresler yapılandırmanızdan geliyor ve{" "}
            <strong className="text-ink-2">listeye yazmasanız da izinlidir</strong>.
          </p>
          {/* Ayraçlı satırlar, kart değil: dört kısa liste dört kutuya
              konsaydı bölüm, içeriğinden çok çerçeveden ibaret olurdu. */}
          <div className="divide-y divide-line">
            {gruplar.map((g) => (
              <div
                key={g.baslik}
                className="flex flex-wrap items-baseline gap-x-3 gap-y-1 px-3 py-2"
              >
                <span className="w-36 shrink-0 text-xs text-ink-3">{g.baslik}</span>
                <span className="font-mono text-xs text-ink-1">
                  {g.hostlar!.join(", ")}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* KURUM İÇİ ADRESLER AYRI KUTUDA (spec 026).
          Yukarıdaki listeyle birleştirilseydi "izinli adresler" diye
          okunurdu; oysa bu liste izinle ilgilenmiyor, yalnızca yolu
          söylüyor. Ve asıl söylenmesi gereken şey bu adreslerin kurumsal
          proxy'nin kaydından çıktığı — kullanıcının bilmediği bir kapı
          bırakılmaz. */}
      {data.internalHosts && data.internalHosts.length > 0 && (
        <div className="rounded-lg border border-line">
          <p className="border-b border-line px-3 py-2 text-xs text-ink-3">
            Bu adreslere <strong className="text-ink-2">proxy&apos;ye uğramadan</strong>{" "}
            gidilir; kurumsal proxy&apos;nin kaydına ve denetimine girmezler.
            Liste izin vermez — çıkış izni yukarıdan gelir.
          </p>
          <div className="px-3 py-2">
            <span className="font-mono text-xs text-ink-1">
              {data.internalHosts.join(", ")}
            </span>
          </div>
        </div>
      )}

      {/* ÖLÇÜLEN SINIR, süs değil: kapı TLS açmadığı için yalnızca domain'e
          bakabiliyor. İzinli bir domain'in sunduğu her imkân da açılmış olur.
          Bunu söylememek, listenin verdiğinden fazla güvence vermek olurdu. */}
      <p className="text-xs leading-relaxed text-ink-3">
        İzin verilen bir domain&apos;in tamamı açılır — o adresteki her yol ve
        her port. Örneğin <code>github.com</code> izinliyse oradaki her içerik
        erişilebilir olur.
      </p>
    </div>
  );
}
