import type { RunBatchCounts } from "@/lib/types";

/**
 * Toplu işin sayıları — dört büyük KPI kartı DEĞİL, tek satırlık bir şerit.
 *
 * Beş rakam için ekranın yarısını harcamak, asıl içeriği (otuz projelik öğe
 * listesini) aşağı iterdi (ui.md → kaçınılacaklar).
 *
 * SIFIR OLAN KOVA ÇİZİLMEZ: "0 başarısız" bir bilgi değil, gürültü.
 *
 * Tek istisna SÜREN bir toplu işte bekleyen ve çalışan: kuyruğun boşalışını
 * izlemek kullanıcının tam da beklediği şey, "0 bekliyor" orada bir haber.
 * İş bittiğinde aynı iki sıfır habersizleşir ve kaybolur.
 */
export function CountStrip({
  counts,
  active = false,
  compact = false,
}: {
  counts: RunBatchCounts;
  active?: boolean;
  compact?: boolean;
}) {
  const kovalar: Array<{ etiket: string; deger: number; renk: string; hepDur?: boolean }> = [
    { etiket: "bekliyor", deger: counts.pending, renk: "text-ink-2", hepDur: active },
    { etiket: "çalışıyor", deger: counts.running, renk: "text-accent", hepDur: active },
    { etiket: "tamamlandı", deger: counts.succeeded, renk: "text-ok" },
    { etiket: "başarısız", deger: counts.failed, renk: "text-danger" },
    { etiket: "kesildi", deger: counts.interrupted, renk: "text-warn" },
    { etiket: "iptal", deger: counts.cancelled, renk: "text-warn" },
  ];

  const gorunen = kovalar.filter((k) => k.hepDur || k.deger > 0);

  return (
    <div className={`flex flex-wrap items-center ${compact ? "gap-3" : "gap-5"}`}>
      {gorunen.map((k) => (
        <div key={k.etiket} className="flex items-baseline gap-1.5">
          {/* Renk tek kanal değil: her rakamın yanında etiketi de durur. */}
          <span className={`text-sm font-medium tabular-nums ${k.renk}`}>{k.deger}</span>
          <span className="text-2xs text-ink-3">{k.etiket}</span>
        </div>
      ))}
    </div>
  );
}
