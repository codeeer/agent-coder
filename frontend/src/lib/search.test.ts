import { test } from "node:test";
import assert from "node:assert/strict";

import { fold, matchesAny, needle } from "./search.ts";

/*
 * Katlama kuralı — ürünün sekiz arama kutusunun ortak zemini.
 *
 * Buradaki testler bir hatanın kaydı: kural yazıldıktan sonra kopyalandı ve
 * kopyalardan biri (model seçici) düz `toLowerCase()` kullanmaya devam etti.
 * Aşağıdaki iki iddia, o kopyanın neden yanlış olduğunu gösteren şeyin ta
 * kendisi.
 */

test("noktasız I küçüğüne doğru katlanır", () => {
  // Düz toLowerCase "IŞIK" → "ışik" verir ve kullanıcının yazdığı "ışık"
  // hiçbir zaman eşleşmez.
  assert.equal(fold("IŞIK"), "ışık");
  assert.ok(fold("IŞIK").includes(fold("ışık")));
});

test("noktalı İ tek karakter olarak katlanır — konumlar kaymaz", () => {
  // Düz toLowerCase "İ" için `i` + birleşik nokta (iki karakter) üretir;
  // katlanmış metinde bulunan bir konum ham metne uygulanınca kayar.
  assert.equal(fold("İŞ"), "iş");
  assert.equal(fold("İŞ").length, "İŞ".length);
  assert.equal(fold("İSTANBUL").length, "İSTANBUL".length);
});

test("needle kırpar ve katlar", () => {
  assert.equal(needle("  IŞIK  "), "ışık");
  assert.equal(needle("   "), "");
  assert.equal(needle(""), "");
});

test("boş iğne her kaydı geçirir", () => {
  // Arama kutusu boşken liste süzülmez; ters yorum boş ekran verirdi.
  assert.equal(matchesAny(["herhangi"], ""), true);
  assert.equal(matchesAny([], ""), true);
});

test("alanlardan herhangi biri eşleşirse yeter", () => {
  assert.equal(matchesAny(["ödeme", "kargo"], "karg"), true);
  assert.equal(matchesAny(["ödeme", "kargo"], "fatura"), false);
});

test("dolmamış alanlar atlanır", () => {
  // Çağıranlar ayrıca .filter(Boolean) yazmak zorunda kalmasın.
  assert.equal(matchesAny([null, undefined, "", "kargo"], "karg"), true);
  assert.equal(matchesAny([null, undefined, ""], "karg"), false);
});

test("eşleştirme büyük/küçük harf farkını Türkçe kurala göre yutar", () => {
  assert.equal(matchesAny(["Ödeme IŞIK Servisi"], needle("ışık")), true);
  assert.equal(matchesAny(["İŞ Takip"], needle("iş")), true);
});
