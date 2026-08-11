"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { StatusDot } from "@/components/ui/primitives";

/**
 * Kenar çubuğunun dibindeki sistem durumu.
 *
 * Referans tasarımda kenar çubuğunun en altı sistemin sağlığına ve sürüme
 * ayrılmış — ve bu, uygulama kabuğunun en doğru yeri: her ekranda görünür,
 * hiçbir ekranın işini bölmez, göz onu aramaya gitmez ama bozulduğunda
 * kenarda kırmızı bir nokta belirir.
 *
 * Bu bileşen daha önce `BackendStatus` adıyla vardı ve HİÇBİR YERDE
 * kullanılmıyordu: tam genişlikte bir kart olarak yazılmıştı, dolayısıyla
 * bir sayfanın içine konması gerekiyordu ve hiçbir sayfanın konusu
 * "backend'in sürümü" değildi. Kabuğa taşınınca yerini buldu.
 *
 * Veri UYDURULMUYOR: durum, sürüm ve ortam `/health` yanıtından gelir.
 * Yanıt gelmiyorsa bunu da açıkça söyler — sessizce yeşil kalmaz.
 */
export function SystemStatus() {
  const health = useQuery({
    queryKey: ["health"],
    queryFn: api.health,
    // Kabuk her ekranda duruyor; bir kez sorup unutmak, backend saatler önce
    // düşmüşken yeşil bir nokta göstermek demek olurdu.
    refetchInterval: 30_000,
    retry: false,
  });

  const ok = health.isSuccess;
  const failed = health.isError;

  return (
    <div className="px-3 py-3">
      <div className="flex items-center gap-2">
        <StatusDot
          tone={ok ? "success" : failed ? "danger" : "neutral"}
          pulse={health.isPending}
        />
        <span className="truncate text-xs font-medium text-ink-2">
          {ok && "Sistem çalışıyor"}
          {failed && "Backend'e ulaşılamıyor"}
          {health.isPending && "Kontrol ediliyor…"}
        </span>
      </div>

      {/* Sürüm satırı yalnızca bağlantı varken: "Sürüm —" yazmak, bilinmeyen
          bir şeyi biliyormuş gibi göstermenin küçük hâli. */}
      <p className="mt-1 truncate pl-4 text-2xs text-ink-3">
        {ok
          ? `${health.data.version} · ${health.data.env}`
          : failed
            ? "make ps ile servisleri kontrol edin"
            : " "}
      </p>
    </div>
  );
}
