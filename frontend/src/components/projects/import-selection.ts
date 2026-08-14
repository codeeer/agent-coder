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
