/**
 * Depo adresini okunur parçalara ayırır.
 *
 * Liste ekranında ham URL gösteriliyordu ve dar bir sütunda kırpılınca
 * geriye asıl kimliği taşımayan kısım kalıyordu:
 * "https://github.com/kullanici/dep…" — yani en çok yer kaplayan parça
 * (`https://`) en az bilgi taşıyan parçaydı.
 *
 * Depoyu tanımlayan şey `kullanici/depo`; sunucu adı ise ikincil bir
 * bağlam. İkisi ayrı döndürülüyor ki ekran hangisini öne çıkaracağına
 * kendisi karar versin.
 *
 * Bileşenin içine gömülmüyor: ayrıştırma saf mantıktır ve ancak kendi
 * modülünde test edilebilir (AGENTS.md → TypeScript konvansiyonları).
 */

export interface RepoLabel {
  /** `github.com`, `localhost:3000` … Bilinemiyorsa boş. */
  host: string;
  /** `kullanici/depo`. Ayrıştırılamazsa girdinin kendisi. */
  path: string;
}

export function repoLabel(url: string): RepoLabel {
  const raw = url.trim();
  if (raw === "") return { host: "", path: "" };

  // SSH biçimi: git@github.com:kullanici/depo.git
  // URL ayrıştırıcısı bunu tanımaz; iki nokta üst üste port sanılır.
  const ssh = /^([^@\s]+)@([^:\s]+):(.+)$/.exec(raw);
  if (ssh) {
    return { host: ssh[2]!, path: stripGit(ssh[3]!) };
  }

  try {
    const u = new URL(raw);
    // Kimlik bilgisi adresin içinde olabilir (https://kullanici:token@host/…);
    // `URL.host` onu zaten dışarıda bırakır — ekranda token göstermek olmaz.
    const path = stripGit(u.pathname.replace(/^\/+/, ""));
    return { host: u.host, path: path === "" ? u.host : path };
  } catch {
    // Ayrıştırılamayan bir adres UYDURULMAZ: olduğu gibi gösterilir.
    return { host: "", path: stripGit(raw) };
  }
}

/** Sondaki `.git` ve `/` atılır — kimliğin parçası değil. */
function stripGit(path: string): string {
  return path.replace(/\/+$/, "").replace(/\.git$/i, "");
}
