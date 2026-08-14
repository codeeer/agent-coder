"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { CACertStatus as Status } from "@/lib/types";
import { Badge, Notice } from "@/components/ui/primitives";

/**
 * Kurumsal sertifikanın durumu.
 *
 * NEDEN VAR: ölçüldü ki yönetici ayarlarda "sertifika" arayınca hiçbir şey
 * bulamıyordu (spec 017, Problem 1). Adres ve kimliği doğru girip her şeyi
 * tamamladığını sanabiliyor, kurulumun üçüncü ayağı hakkında ekran sessiz
 * kalıyordu.
 *
 * BURADAKİ HER DEĞER SERTİFİKADAN OKUNUR. Sahibi, imzalayanı ve bitiş tarihi
 * sunucuda ayrıştırılıp geliyor; hiçbiri kullanıcıya sordurulmuyor veya
 * tahmin edilmiyor.
 */
export function CACertStatus() {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["network", "ca"],
    queryFn: api.network.ca,
  });

  if (isPending) return <Notice>Sertifika durumu okunuyor…</Notice>;
  if (isError) return <Notice tone="error">{describeError(error).message}</Notice>;

  if (data.source === "none") {
    return (
      <Notice>
        Kurumsal kök sertifika <strong>tanımlı değil</strong>. SSL denetimi yapan
        bir ağda değilseniz gerekmez.
      </Notice>
    );
  }

  return (
    <div className="space-y-2.5">
      <p className="flex flex-wrap items-center gap-2 text-sm">
        <Badge tone="success">tanımlı</Badge>
        <span className="text-ink-2">{kaynakMetni(data.source)}</span>
      </p>

      {/* Kart değil, ayraçlı satırlar: zincirde birden çok sertifika olabilir
          ve her birini ayrı kutuya koymak listeyi kutu yığınına çevirirdi. */}
      <div className="divide-y divide-line rounded-lg border border-line">
        {data.certificates.map((c, i) => (
          <div key={`${c.subject}-${i}`} className="px-3 py-2.5">
            <p className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{c.subject}</span>
              {c.selfSigned ? (
                <Badge>kök</Badge>
              ) : (
                <Badge tone="info">ara</Badge>
              )}
              {/* Süresi dolmuş sertifika REDDEDİLMEZ — kurum onu hâlâ
                  kullanıyor olabilir. Yapılacak şey durumu söylemek. */}
              {c.expired && <Badge tone="danger">süresi dolmuş</Badge>}
            </p>
            <p className="mt-0.5 text-2xs text-ink-3">
              İmzalayan: {c.issuer} · Geçerlilik sonu: {tarih(c.notAfter)}
            </p>
          </div>
        ))}
      </div>

      {data.certificates.length === 0 && (
        <Notice tone="warning">
          Sertifika tanımlı ama okunamadı. Değeri yeniden girmeyi deneyin.
        </Notice>
      )}
    </div>
  );
}

/**
 * Kaynağın ne anlama geldiği.
 *
 * "Tanımlı" demek yetmiyor: iki kaynak birden mümkün ve hangisinin geçerli
 * olduğu, kullanıcının doğru yeri düzenlemesi için gerekli.
 */
function kaynakMetni(source: Status["source"]): string {
  switch (source) {
    case "settings":
      return "Aşağıdaki alandan tanımlanmış.";
    case "env":
      return "Sunucudaki RUNNER_EXTRA_CA_CERT dosyasından geliyor. Aşağıya bir sertifika girerseniz o geçerli olur.";
    default:
      return "";
  }
}

/** Tarihi yerelleştirilmiş ve kısa gösterir. */
function tarih(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleDateString("tr-TR", {
    day: "numeric",
    month: "long",
    year: "numeric",
  });
}
