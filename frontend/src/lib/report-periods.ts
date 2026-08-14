/** Rapor dönemi seçeneği — `Segmented` denetiminin beklediği biçim. */
export type ReportPeriod = { id: string; label: string };

/**
 * Sabit dönemler.
 *
 * SERBEST TARİH ARALIĞI YOK ve bu bilinçli: uç yalnızca "kaç gün geriye"
 * parametresi alıyor, "geçen ay" bugünden geriye sayan bir pencereyle ifade
 * edilemezdi.
 */
const SABIT = [7, 30, 90];

/**
 * Dönem seçenekleri — etkin dönem listede yoksa ona da yer açılır.
 *
 * NEDEN: "Varsayılan rapor dönemi" ayarı 1–365 arası herhangi bir sayı
 * olabiliyor. Liste sabit kalsaydı 14 gün ayarlayan biri 14 günlük veriye
 * bakarken hiçbir segment seçili görmezdi — denetim, gösterdiği veriyi
 * yalanlardı.
 */
export function reportPeriods(etkin: number | null): ReportPeriod[] {
  const gunler = new Set(SABIT);
  if (etkin !== null && etkin > 0) {
    gunler.add(etkin);
  }
  return [...gunler]
    .sort((a, b) => a - b)
    .map((g) => ({ id: String(g), label: `${g} gün` }));
}
