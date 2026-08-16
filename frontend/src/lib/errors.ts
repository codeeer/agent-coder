import { ApiError } from "./api";
import { type ErrorContext } from "./error-hints";
import { describeApiError } from "./error-messages";

export type { ErrorContext };

/**
 * Backend hatasını kullanıcının yapabileceği bir eyleme çevirir.
 *
 * İNCE BİR KATMAN: tek işi `ApiError` olup olmadığına bakmak. Çevirinin
 * kendisi `error-messages.ts`'te ve orada test ediliyor — bu dosya `api.ts`
 * üzerinden `node --test`'in yükleyemediği koda bağlandığı için buradaki her
 * satır test dışında kalıyor. Ne kadar azsa o kadar iyi.
 */
export function describeError(
  error: unknown,
  context?: ErrorContext,
): {
  message: string;
  hint?: string;
} {
  if (!(error instanceof ApiError)) {
    return { message: "Beklenmeyen bir hata oluştu" };
  }
  return describeApiError(error.code, error.message, context);
}
