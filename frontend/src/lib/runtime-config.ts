/**
 * Çalışma anı yapılandırması — API adresi ve ürün adı.
 *
 * NEDEN VAR: `NEXT_PUBLIC_*` değişkenleri **derleme anında** bundle'a metin
 * olarak gömülür. Bu, hazır bir Docker imajı yayınlamayı imkânsız kılıyordu:
 * imaj hangi adrese derlendiyse ona mühürlü kalır, portunu değiştiren ya da
 * sunucuya kuran herkes "sunucuya ulaşılamıyor" görürdü.
 *
 * Çözüm tema önyüklemesindeki desenin aynısı: sunucu, HTML'i üretirken adresi
 * bir betikle sayfaya yazar. Böylece TEK bir imaj her kuruluma uyar.
 *
 * Tarayıcı yine doğrudan backend'e bağlanır — araya vekil girmediği için canlı
 * olay akışı (SSE) hiç etkilenmez.
 */

/** Adresin sayfaya yazıldığı genel değişken. */
const GLOBAL = "__AGENT_CODER_API_URL__";

/**
 * apiBase, isteklerin gideceği kök adres.
 *
 * Sıra önemli:
 *   1. Sayfaya çalışma anında yazılan değer — hazır imajda tek doğru kaynak
 *   2. Derleme anındaki değer — `next dev` ve testler için
 *
 * Modül seviyesinde sabit olarak DEĞİL, çağrı başına çözülür: sabit olsaydı
 * modül ilk yüklendiği anda okunur ve enjekte edilen betikten önce çalışma
 * ihtimali doğardı.
 */
export function apiBase(): string {
  if (typeof window !== "undefined") {
    const injected = (window as unknown as Record<string, unknown>)[GLOBAL];
    if (typeof injected === "string" && injected !== "") {
      return injected;
    }
  }
  return process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
}

/**
 * apiConfigScript, sunucunun sayfaya gömdüğü betik.
 *
 * `JSON.stringify` zorunlu: değer ortam değişkeninden geliyor ve tırnak ya da
 * `</script>` içerseydi sayfayı bozardı.
 */
export function apiConfigScript(url: string): string {
  return `window.${GLOBAL}=${JSON.stringify(url)};`;
}

/**
 * serverApiUrl, sunucu tarafında çalışma anında okunan adres.
 *
 * `API_URL` bilinçli olarak `NEXT_PUBLIC_` ÖNEKSİZ: Next, `NEXT_PUBLIC_` ile
 * başlayan her okumayı derleme anında metin olarak değiştiriyor — sunucu
 * bileşeninde bile. Bu değişken o adı taşısaydı, çalışma anında okuduğumuzu
 * sanırken yine derleme anındaki değeri gömerdik.
 */
export function serverApiUrl(): string {
  return (
    process.env.API_URL ??
    process.env.NEXT_PUBLIC_API_URL ??
    "http://localhost:8080"
  );
}

/** Hiçbir şey verilmediğinde arayüzde görünen ürün adı. */
export const DEFAULT_APP_NAME = "Agent Coder";

/**
 * serverAppName, arayüzde görünen ürün adı.
 *
 * Kurulumu yapan `APP_NAME` verirse kelime markası ve sekme başlığı onu
 * gösterir; vermezse varsayılan kalır. Aynı imaj her markayla çalışır — değer
 * imaja gömülmez.
 *
 * `serverApiUrl` ile aynı gerekçeyle `NEXT_PUBLIC_` ÖNEKSİZ (yukarıdaki nota
 * bkz.); o önek değeri derleme anında gömer ve her marka için ayrı imaj
 * gerekirdi.
 *
 * Boş dize de varsayılana düşer: compose değişkeni tanımlanmadığında
 * container'a `APP_NAME=""` olarak ulaşıyor. Varsayılan YALNIZCA burada
 * yazılı; compose'a ikinci bir varsayılan konsaydı ikisi er geç ayrışırdı.
 */
export function serverAppName(): string {
  const verilen = process.env.APP_NAME?.trim();
  return verilen ? verilen : DEFAULT_APP_NAME;
}
