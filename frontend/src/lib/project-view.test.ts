import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";

import {
  PAGE_SIZE,
  PROJECT_VIEW_STORAGE_KEY,
  readView,
  writeView,
  type ProjectView,
} from "./project-view.ts";

/*
 * Projeler ekranının görünüm tercihi (spec 019).
 *
 * `theme.ts` ile aynı kalıp ve aynı tuzaklar: geçersiz değer, erişilemeyen
 * depo, sessiz varsayılan.
 */

let veri: Map<string, string>;

function kur(kayitli?: string, patla = false): void {
  veri = new Map();
  if (kayitli !== undefined) veri.set(PROJECT_VIEW_STORAGE_KEY, kayitli);

  (globalThis as Record<string, unknown>).localStorage = {
    getItem(k: string) {
      if (patla) throw new Error("gizli sekmede erişim yok");
      return veri.get(k) ?? null;
    },
    setItem(k: string, v: string) {
      if (patla) throw new Error("gizli sekmede yazılamaz");
      veri.set(k, v);
    },
  };
}

beforeEach(() => kur());

test("kaydedilmiş görünüm olduğu gibi okunur", () => {
  for (const v of ["list", "card"] as ProjectView[]) {
    kur(v);
    assert.equal(readView(), v);
  }
});

/*
GEÇERSİZ DEĞER LİSTEYE DÜŞER.

Doğrulanmadan geçseydi `PAGE_SIZE[view]` `undefined` verir ve sayfa boyutu
tanımsız kalırdı: liste ya hiç kayıt göstermez ya da sayfalama çöker.
*/
test("tanınmayan değer listeye düşer", () => {
  kur("tablo");
  assert.equal(readView(), "list");

  kur("");
  assert.equal(readView(), "list");
});

test("hiç kayıt yoksa liste seçilir", () => {
  kur(undefined);
  assert.equal(readView(), "list");
});

// Gizli sekmede depo erişimi hata veriyor; görünüm tercihi yüzünden Projeler
// ekranının tamamı düşmemeli.
test("depo erişilemezse ekran düşmez", () => {
  kur("card", true);
  assert.equal(readView(), "list");
});

test("kaydedilemeyen tercih hata fırlatmaz", () => {
  kur(undefined, true);
  assert.doesNotThrow(() => writeView("card"));
});

test("tercih kaydedilir", () => {
  writeView("card");
  assert.equal(veri.get(PROJECT_VIEW_STORAGE_KEY), "card");
});

/*
SAYFA BOYUTU GÖRÜNÜME GÖRE FARKLI — VE HER İKİSİ İÇİN TANIMLI.

Ölçüldü (1440×900): listede 13 satır, kartta 6 kart sığıyor. Tek bir boyut
ikisine birden uymuyor; 12 kart iki ekran boyu kaydırma, 6 satır ise listede
ekranın yarısını boş bırakırdı.

Buradaki asıl koruma boyutların DEĞERİ değil, ikisinin de tanımlı ve farklı
olması: eksik bir anahtar `undefined` sayfa boyutu demek.
*/
test("her iki görünümün de sayfa boyutu tanımlı", () => {
  for (const v of ["list", "card"] as ProjectView[]) {
    assert.equal(typeof PAGE_SIZE[v], "number", `${v} için sayfa boyutu yok`);
    assert.ok(PAGE_SIZE[v] > 0);
  }
});

test("liste kart görünümünden daha çok kayıt gösterir", () => {
  assert.ok(
    PAGE_SIZE.list > PAGE_SIZE.card,
    "liste karşılaştırma içindir; kart görünümünden az kayıt göstermesi anlamsız",
  );
});
