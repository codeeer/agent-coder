/**
 * Projeler ekranının görünüm tercihi.
 *
 * İki görünümün farklı işleri var: **liste** karşılaştırma için (aynı bilgi
 * her satırda aynı sütunda, iki projenin branch'i yan yana okunur), **kart**
 * az sayıda projeyi rahat okumak için.
 *
 * NEDEN VERİTABANINDA DEĞİL: bu bir davranış parametresi değil, BAKAN KİŞİNİN
 * tercihi. Aynı kurulumu iki kişi iki farklı görünümle kullanabilmeli;
 * sunucuda tutulsaydı biri diğerinin ekranını değiştirirdi. Tema seçiminde
 * verilen kararın aynısı (bkz. `lib/theme.ts`).
 */

export type ProjectView = "list" | "card";

export const PROJECT_VIEW_STORAGE_KEY = "agent-coder-project-view";

/**
 * Görünüme göre sayfa boyutu.
 *
 * ÖLÇÜLDÜ (1440×900, 665 piksellik içerik alanı):
 *   · liste: satır 48px → 13 satır sığıyor
 *   · kart : kart 227px, iki sütun → 6 kart sığıyor
 *
 * Tek bir sayfa boyutu ikisine birden uymuyor: 12 kart görünümünde iki ekran
 * boyu kaydırma, 6 ise liste görünümünde ekranın yarısını boş bırakırdı.
 * Sayfalamanın gerçekten sayfalama olması her iki görünümde de isteniyor
 * (spec 019 H3, H6).
 */
export const PAGE_SIZE: Record<ProjectView, number> = {
  list: 12,
  card: 6,
};

/** Kaydedilmiş tercih; yoksa veya bozuksa listeye düşer. */
export function readView(): ProjectView {
  try {
    const raw = localStorage.getItem(PROJECT_VIEW_STORAGE_KEY);
    if (raw === "list" || raw === "card") return raw;
  } catch {
    // Gizli sekmede localStorage erişimi hata verebilir; görünüm tercihi
    // yüzünden ekran düşmemeli.
  }
  return "list";
}

/** Tercihi kaydeder. Kaydedilemese de seçim bu oturum boyunca geçerlidir. */
export function writeView(view: ProjectView): void {
  try {
    localStorage.setItem(PROJECT_VIEW_STORAGE_KEY, view);
  } catch {
    // Yoksay.
  }
}
