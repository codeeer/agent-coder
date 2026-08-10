import { ApiError } from "./api";

/**
 * Backend hatasını kullanıcının yapabileceği bir eyleme çevirir.
 *
 * "Değer yanlış", "adres yanlış" ve "servise ulaşılamadı" ayrımı korunur —
 * kullanıcı üçünde farklı şey yapar.
 */
export function describeError(error: unknown): {
  message: string;
  hint?: string;
} {
  if (!(error instanceof ApiError)) {
    return { message: "Beklenmeyen bir hata oluştu" };
  }

  switch (error.code) {
    case "invalid_credential":
      return {
        message: "Değer doğrulanamadı",
        hint: "Kopyalarken eksik veya fazla karakter kalmış olabilir.",
      };
    case "service_unreachable":
      return {
        message: "Servise ulaşılamadı",
        hint: "Adresi ve bağlantınızı kontrol edip tekrar deneyin.",
      };
    case "invalid_base_url":
      return { message: error.message, hint: "Örnek: https://llm.sirket.local/v1" };
    case "bad_catalog":
      return {
        message: "Servis beklenen biçimde model listesi vermiyor",
        hint: "Adresin sonunda /v1 olması gerekebilir.",
      };
    case "missing_username":
      return {
        message: "Kullanıcı adı zorunlu",
        hint: "Bitbucket app password kullanıcı adıyla birlikte kullanılır.",
      };
    case "network_error":
    case "timeout":
      return {
        message: "Backend'e ulaşılamadı",
        hint: "Servisler ayakta mı? make ps ile kontrol edin.",
      };
    default:
      return { message: error.message };
  }
}
