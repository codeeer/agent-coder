/**
 * Hata ipuçları — hangi ekranda ne önerileceği.
 *
 * `errors.ts`'ten AYRI bir modül: o dosya `ApiError` sınıfını içeri alıyor ve
 * onunla birlikte tüm API istemcisini sürüklüyor, dolayısıyla `node --test`
 * içinden yüklenemiyor. Buradaki mantık saf ve bağımlılıksız olduğu için test
 * edilebilir kalıyor (`failure.ts` ile aynı kalıp).
 */

/**
 * Hatanın hangi ekrandan geldiği.
 *
 * Aynı hata kodu farklı ekranlarda farklı ipucu ister: `invalid_base_url`,
 * LLM sağlayıcıda "sonunda /v1 olmalı", git erişiminde "kendi sunucunuzun
 * adresi" demektir.
 */
export type ErrorContext = "llm" | "git";

/**
 * baseUrlHint, adres hatasında gösterilecek ipucu.
 *
 * Bir zamanlar `describeError` içinde koşulsuz "Örnek: …/v1" yazıyordu. Bitbucket
 * Server doğrulaması 404'ü adres hatası olarak raporlamaya başlayınca o metin git
 * ekranında da görünür oldu ve kurumsal Bitbucket kullanıcısına adresinin sonuna
 * `/v1` eklemesini önerir hale geldi — kullanıcıyı doğru olanı bozmaya iten bir
 * yönlendirme.
 */
export function baseUrlHint(context?: ErrorContext): string | undefined {
  switch (context) {
    case "llm":
      return "Örnek: https://llm.sirket.local/v1 — çoğu servis /v1 ile biter.";
    case "git":
      return (
        "Kendi sunucunuzun adresi, örn. https://bitbucket.sirket.local. " +
        "Bulut hizmetini kullanıyorsanız boş bırakın."
      );
    default:
      // Bağlam bilinmiyor: yanlış ipucu, ipucu olmamasından kötüdür. Hata
      // mesajının kendisi zaten ayrıntılı geliyor.
      return undefined;
  }
}
