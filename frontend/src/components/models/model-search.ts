import { fold, needle } from "../../lib/search.ts";
import type { Model } from "@/lib/types";

/**
 * Model arama — sıralama ve vurgulama mantığı.
 *
 * Bileşenin İÇİNDE DEĞİL: burada iki şey gözle doğrulanamıyor. Sıralama
 * puanları ancak yüzlerce modelin arasından hangisinin öne çıktığına bakarak
 * anlaşılır; vurgulama ise katlanmış metinde bulunan bir KONUMU ham metne
 * uyguluyor ve kayma yalnızca belirli harflerde ortaya çıkıyor.
 *
 * Bu modül, katlamanın düz `toLowerCase()` olduğu bir dönemin ardından
 * ayrıldı: ürünün geri kalanı `tr` yerelinde katlarken burası katlamıyordu ve
 * fark kimsenin yazılı bir kanıtı olmadığı için görülmemişti.
 */

/**
 * Listede gösterilen en fazla model.
 *
 * Katalogda yüzlerce model olabiliyor; hepsini çizmek açılır listeyi
 * kullanılamaz hâle getirirdi.
 */
export const MODEL_LIMIT = 60;

/**
 * Sorguya uyan modeller — en iyi eşleşme başta.
 *
 * SORGU BOŞKEN seçili model listenin başında durur: liste alfabetik ilk
 * MODEL_LIMIT kaydı gösterdiğinde kullanıcının kendi seçimi çoğu zaman o
 * kümenin dışında kalıyor ve hangi modelde olduğunu doğrulamak için adını
 * yazmak zorunda kalıyordu.
 *
 * SIRALAMA kimliği önceler: kullanıcı genelde model kimliğini yazar. Eşit
 * puanlarda kimliğe göre alfabetik — sıra her çizimde aynı olmalı.
 */
export function modelAra(
  models: Model[],
  sorgu: string,
  secili?: Model,
): Model[] {
  const q = needle(sorgu);

  if (!q) {
    const digerleri = models
      .filter((m) => m !== secili)
      .slice(0, MODEL_LIMIT - 1);
    return secili ? [secili, ...digerleri] : models.slice(0, MODEL_LIMIT);
  }

  const puanli = models
    .map((m) => {
      const id = fold(m.id);
      const ad = fold(m.name);
      const saglayici = fold(m.providerName);

      if (id.startsWith(q)) return { m, puan: 0 };
      if (id.includes(q)) return { m, puan: 1 };
      if (ad.includes(q)) return { m, puan: 2 };
      if (saglayici.includes(q)) return { m, puan: 3 };
      return null;
    })
    .filter((x): x is { m: Model; puan: number } => x !== null);

  puanli.sort((a, b) => a.puan - b.puan || a.m.id.localeCompare(b.m.id));
  return puanli.slice(0, MODEL_LIMIT).map((x) => x.m);
}

/**
 * Vurgulanacak aralık — eşleşme yoksa null.
 *
 * Konum KATLANMIŞ metinde bulunup HAM metne uygulanıyor, dolayısıyla katlama
 * uzunluğu korumak zorunda. Düz `toLowerCase()` bunu garanti etmiyor: "İ" için
 * `i` artı ayrı bir birleşik nokta üretiyor, yani bir karakter iki oluyor ve o
 * noktadan sonraki her dilim kayıyor — kullanıcı yanlış harflerin
 * kalınlaştığını görüyordu. `fold` (tr) tek karakter verir.
 */
export function vurguAraligi(
  metin: string,
  sorgu: string,
): [number, number] | null {
  const q = needle(sorgu);
  if (!q) return null;

  const konum = fold(metin).indexOf(q);
  if (konum < 0) return null;
  return [konum, konum + q.length];
}
