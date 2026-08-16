/**
 * Tema seçimi.
 *
 * Üç durum: `system` (varsayılan), `light`, `dark`. Seçim `<html data-theme>`
 * özniteliğine yazılır; CSS geri kalanı halleder.
 *
 * NEDEN VERİTABANINDA DEĞİL: projenin kuralı "davranış parametreleri koda
 * gömülmez, veritabanında durur" der ama bu bir davranış parametresi değil,
 * BAKAN KİŞİNİN tercihidir. Aynı kurulumu iki kişi iki farklı temayla
 * kullanabilmeli; sunucuda tutulsaydı biri diğerinin ekranını değiştirirdi.
 * Bu yüzden yeri tarayıcının kendi belleği.
 */

export type ThemeMode = "system" | "light" | "dark";

export const THEME_STORAGE_KEY = "agent-coder-theme";

/** Kaydedilmiş seçim; yoksa veya bozuksa sisteme düşer. */
export function readMode(): ThemeMode {
  try {
    const raw = localStorage.getItem(THEME_STORAGE_KEY);
    if (raw === "light" || raw === "dark" || raw === "system") return raw;
  } catch {
    // Gizli sekmede localStorage erişimi hata verebilir; tema yüzünden
    // uygulama düşmemeli.
  }
  return "system";
}

/** Seçimi kaydeder ve hemen uygular. */
export function writeMode(mode: ThemeMode): void {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, mode);
  } catch {
    // Kaydedilemese de seçim bu oturum boyunca geçerli olsun.
  }
  applyMode(mode);
}

/** Seçimi `<html data-theme>` özniteliğine yazar. */
export function applyMode(mode: ThemeMode): void {
  const root = document.documentElement;
  const dark =
    mode === "dark" ||
    (mode === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);

  root.dataset.theme = dark ? "dark" : "light";
}

/**
 * İlk boyamadan ÖNCE çalışan betik.
 *
 * `<head>` içine engelleyici olarak konur. Tema React yüklendikten sonra
 * uygulansaydı sayfa önce yanlış temayla boyanır, sonra sıçrardı — koyu tema
 * bekleyen kullanıcının gözüne beyaz bir çakma olarak çarpar.
 *
 * Bilerek küçük ve bağımsız: bir modülü beklemesi, beklemenin kendisi olurdu.
 */
export const themeBootstrapScript = `
(function(){
  try{
    var m = localStorage.getItem(${JSON.stringify(THEME_STORAGE_KEY)});
    // Doğrulama \`readMode\` ile AYNI olmak zorunda. Önceden yalnızca
    // \`|| "system"\` vardı ve o sadece BOŞ değeri yakalıyordu: depoda tanınmayan
    // bir değer varken betik "açık" boyuyor, React ise \`readMode\` sayesinde
    // sisteme düşüp koyu uyguluyordu. Yani betik, tam olarak önlemek için
    // yazıldığı sıçramayı kendisi üretiyordu.
    if (m !== "light" && m !== "dark") m = "system";
    var dark = m === "dark" || (m === "system" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);
    document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  }catch(e){}
})();
`.trim();
