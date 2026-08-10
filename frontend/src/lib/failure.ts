/**
 * Hata metnini okunur hale getirir.
 *
 * Sağlayıcılar hatayı çoğu zaman JSON gövdesiyle döndürüyor ve rapor sayfasında
 * ham haliyle görünüyordu: yöneticinin ekranında
 * `model çağrısı başarısız: … {"name":"UnknownError","data":{…}}` duruyordu.
 * Gövdedeki `message` alanı zaten insan için yazılmış; onu öne çıkarıp kalanını
 * atıyoruz.
 *
 * Bilgi kaybı olmasın diye tam metin arayüzde `title` olarak duruyor.
 *
 * React'tan ayrı: saf fonksiyon olduğu için testi tarayıcı açmadan yazılabilir.
 */
export function readableFailure(raw: string): string {
  const at = raw.indexOf("{");
  if (at === -1) return raw;

  const prefix = raw.slice(0, at).replace(/[:\s]+$/, "");

  let inner = "";
  try {
    const body: unknown = JSON.parse(raw.slice(at));
    if (body && typeof body === "object") {
      const b = body as Record<string, unknown>;
      const data = b.data as Record<string, unknown> | undefined;
      const candidate = data?.message ?? b.message ?? b.error;
      if (typeof candidate === "string") inner = candidate;
    }
  } catch {
    // Gövde ayrıştırılamıyorsa yalnızca öncesini göstermek yine de daha iyi.
  }

  if (!inner) return prefix || raw;
  return prefix ? `${prefix} — ${inner}` : inner;
}
