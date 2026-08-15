import type { ImportRepo } from "@/lib/types";

/**
 * Gruptan içe aktarmada seçim kuralları.
 *
 * Bileşenin içine gömülmüyor: "hangi kayıt seçili gelir" sorusu kırk satırlık
 * bir listede gözle doğrulanamaz; ancak kendi modülünde test edilebilir
 * (AGENTS.md → TypeScript konvansiyonları).
 */

/**
 * Bu repository seçilebilir mi.
 *
 * Zaten kayıtlı olanlar seçilemez — ama LİSTEDE DURUR. Gizlenselerdi kullanıcı
 * "gruptaki her şey geldi mi?" sorusunu cevaplayamazdı.
 */
export function secilebilir(r: ImportRepo): boolean {
  return r.status !== "already_registered";
}

/**
 * Açılışta seçili gelenler: yeni olanlar.
 *
 * Arşivli olanlar DIŞARIDA kalır ama listede görünür ve elle seçilebilir.
 * Arşivlenmiş bir repository'yi kullanıcı gerçekten istiyorsa yolu kapalı
 * olmamalı; istemiyorsa da kırk kutucuğu tek tek boşaltmak zorunda kalmamalı.
 */
export function varsayilanSecim(repos: ImportRepo[]): string[] {
  return repos.filter((r) => secilebilir(r) && !r.archived).map((r) => r.slug);
}

/**
 * Görünen git erişimi: kullanıcının seçimi, yoksa listedeki ilk kayıt.
 *
 * Bileşen durumu YALNIZCA elle yapılan seçimi tutar, görünen değer buradan
 * türetilir. `useState(liste[0]?.id ?? "")` biçiminde tohumlamak yalnızca mount
 * anında çalışır: panel, git erişimleri sorgusu çözülmeden açılırsa liste boş
 * gelir ve durum "" olarak donar. Select sonradan seçenekleri gösterse bile
 * istek boş kimlikle gider, backend'in `*uuid.UUID` çözümlemesi düşer ve
 * kullanıcı sebebini anlatmayan bir 400 görür.
 *
 * Seçim listeden düşerse (erişim silinince) yine ilk kayda dönülür — artık var
 * olmayan bir kimliği göndermektense.
 */
export function etkinProvider(secim: string, liste: { id: string }[]): string {
  if (secim && liste.some((p) => p.id === secim)) return secim;
  return liste[0]?.id ?? "";
}

/** Tek bir kaydın seçimini açar ya da kapatır. */
export function secimAcKapa(secili: Set<string>, slug: string): Set<string> {
  const yeni = new Set(secili);
  if (!yeni.delete(slug)) yeni.add(slug);
  return yeni;
}

/**
 * "Tümünü seç" — yalnızca seçilebilir olanları alır.
 *
 * Zaten kayıtlı olanları da alsaydı kullanıcı seçtiğini sanır, sonuç
 * ekranında hepsinin atlandığını görürdü.
 */
export function tumunuSec(repos: ImportRepo[]): string[] {
  return repos.filter(secilebilir).map((r) => r.slug);
}
