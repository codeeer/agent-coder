"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { CacheVerifyResult, DependencyCacheInfo } from "@/lib/types";
import { Badge, Button, ConfirmStrip, Mono, Notice } from "@/components/ui/primitives";

/**
 * Bağımlılık önbelleğinin durumu ve bakımı (spec 027 H3, H5).
 *
 * İKİ AYRI SATIR — Maven ve npm. Tek bir toplam boyut, diski hangisinin
 * doldurduğu sorusunu cevapsız bırakırdı; temizleme de ekosistem başına,
 * çünkü yalnızca npm'i temizlemek isteyen kullanıcı 569 MB'lık Maven
 * birikimini de kaybetmemeli.
 */

/** Bayt sayısını okunur hâle getirir. */
function boyut(bayt: number): string {
  if (bayt < 1024) return `${bayt} B`;
  const birimler = ["KB", "MB", "GB", "TB"];
  let deger = bayt / 1024;
  let i = 0;
  while (deger >= 1024 && i < birimler.length - 1) {
    deger /= 1024;
    i++;
  }
  // Tek ondalık: "1,2 GB" yeterli, "1,23 GB" gürültü.
  return `${deger.toFixed(deger < 10 ? 1 : 0).replace(".", ",")} ${birimler[i]}`;
}

/**
 * Tarama sonucunun cümlesi.
 *
 * npm için "bozuk" DENMEZ: npm bozulmayı referanssız içeriğin toplanmasından
 * ayırmıyor (ölçüldü — spec 027 T42). Maven'da "uyuşmadı" gerçek bir iddia,
 * npm'de olmayan bir bozulmayı rapor etmek olurdu.
 */
function taramaCumlesi(id: DependencyCacheInfo["id"], r: CacheVerifyResult): string {
  if (id === "npm") {
    const temizlenen = r.removed > 0 ? `, ${r.removed} kayıt temizlendi` : "";
    return `${r.checked} kayıt denetlendi${temizlenen}.`;
  }

  const parcalar = [`${r.checked} artefakt denetlendi`];
  parcalar.push(r.mismatched > 0 ? `${r.mismatched} tanesi uyuşmadı ve silindi` : "hiçbiri bozuk değil");
  if (r.unverifiable > 0) {
    // Denetlenemeyen SİLİNMEDİ — cümle bunu açıkça söylemeli, yoksa
    // kullanıcı "silinmiş olabilir mi" diye tereddüt eder.
    parcalar.push(`${r.unverifiable} tanesinin özeti okunamadı (dokunulmadı)`);
  }
  return parcalar.join(", ") + ".";
}

function OnbellekSatiri({
  onbellek,
  onDegisti,
}: {
  onbellek: DependencyCacheInfo;
  onDegisti: () => void;
}) {
  const [onayAcik, setOnayAcik] = useState(false);
  const [tarama, setTarama] = useState<CacheVerifyResult | null>(null);

  const temizle = useMutation({
    mutationFn: () => api.dependencyCache.clear(onbellek.id),
    onSuccess: () => {
      setOnayAcik(false);
      setTarama(null);
      onDegisti();
    },
  });

  const dogrula = useMutation({
    mutationFn: () => api.dependencyCache.verify(onbellek.id),
    onSuccess: (sonuc) => {
      setTarama(sonuc);
      onDegisti();
    },
  });

  const bosMu = !onbellek.used;

  return (
    <div className="border-b border-line last:border-b-0">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 px-4 py-3">
        <span className="w-16 text-sm font-medium text-ink">{onbellek.label}</span>

        {/* HENÜZ KULLANILMADI ≠ 0 B. Sıfır göstermek, çalışmış ama boşaltılmış
            bir önbellekle karıştırır (spec 027 H3). */}
        {bosMu ? (
          <span className="text-sm text-ink-3">henüz kullanılmadı</span>
        ) : (
          <Mono className="text-sm text-ink-2">{boyut(onbellek.sizeBytes)}</Mono>
        )}

        <span className="ml-auto flex items-center gap-2">
          <Button
            size="sm"
            onClick={() => dogrula.mutate()}
            disabled={bosMu || dogrula.isPending || temizle.isPending}
          >
            {dogrula.isPending ? "Denetleniyor…" : "Doğrula"}
          </Button>
          <Button
            size="sm"
            onClick={() => setOnayAcik(true)}
            disabled={bosMu || onayAcik || temizle.isPending}
          >
            Temizle
          </Button>
        </span>
      </div>

      {tarama && !dogrula.isPending && (
        <p className="px-4 pb-3 text-xs text-ink-2">
          {taramaCumlesi(onbellek.id, tarama)}
        </p>
      )}
      {dogrula.isError && (
        <p className="px-4 pb-3 text-xs text-danger">
          {describeError(dogrula.error).message}
        </p>
      )}

      {onayAcik && (
        <ConfirmStrip
          question={
            <>
              <strong>{onbellek.label}</strong> önbelleği temizlensin mi?
            </>
          }
          /* SAYIYLA: "emin misiniz?" hiçbir şey söylemez. Ne kaybedileceği ve
             sonucunun ne olacağı yazılıyor. */
          consequence={
            <>
              {boyut(onbellek.sizeBytes)} yer boşalacak. Sonraki koşular bu
              bağımlılıkları <strong>yeniden indirecek</strong> ve ilk koşu
              belirgin biçimde yavaş olacak.
            </>
          }
          confirmLabel="Evet, temizle"
          busyLabel="Temizleniyor…"
          busy={temizle.isPending}
          error={temizle.isError ? describeError(temizle.error).message : undefined}
          onConfirm={() => temizle.mutate()}
          onCancel={() => setOnayAcik(false)}
        />
      )}
    </div>
  );
}

export function DependencyCacheStatus() {
  const qc = useQueryClient();
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["dependency-cache"],
    queryFn: api.dependencyCache.status,
  });

  if (isPending) return <Notice>Önbellek durumu okunuyor…</Notice>;
  if (isError) return <Notice tone="error">{describeError(error).message}</Notice>;

  const yenile = () => void qc.invalidateQueries({ queryKey: ["dependency-cache"] });

  return (
    <div>
      <div className="flex flex-wrap items-center gap-2 px-4 py-3 text-sm">
        <Badge tone={data.enabled ? "success" : undefined}>
          {data.enabled ? "açık" : "kapalı"}
        </Badge>
        <span className="text-ink-2">
          {data.enabled
            ? "İndirilen bağımlılıklar koşular arasında saklanıyor."
            : /* KAPALIYKEN DE BOYUT GÖSTERİLİR: biriken önbellek duruyor ve
                 kullanıcı onu temizleyebilmeli (spec 027 H2). */
              "Yeni koşular önbelleğe yazmıyor; biriken içerik duruyor."}
        </span>
      </div>

      <div className="border-t border-line">
        {data.caches.map((c) => (
          <OnbellekSatiri key={c.id} onbellek={c} onDegisti={yenile} />
        ))}
      </div>
    </div>
  );
}
