import { test } from "node:test";
import assert from "node:assert/strict";

import { MODEL_LIMIT, modelAra, vurguAraligi } from "./model-search.ts";
import type { Model } from "@/lib/types";

function model(id: string, ek: Partial<Model> = {}): Model {
  return {
    id,
    name: id,
    providerId: "p1",
    providerName: "OpenRouter",
    supportsTools: true,
    ...ek,
  } as Model;
}

/*
 * Model arama.
 *
 * İki iddia gözle doğrulanamıyordu ve bu yüzden bir hata uzun süre görülmedi:
 * bu modül düz `toLowerCase()` kullanırken ürünün geri kalanı Türkçe katlama
 * yapıyordu.
 */

test("sıralama kimliği önceler", () => {
  const models = [
    model("z/claude-sonnet", { name: "claude" }),
    model("anthropic/claude-opus"),
  ];

  // "claude" hem kimlikte hem adda geçiyor; kimlikte geçen önce gelmeli.
  const sonuc = modelAra(models, "claude");
  assert.deepEqual(
    sonuc.map((m) => m.id),
    ["anthropic/claude-opus", "z/claude-sonnet"],
  );
});

test("sorgu boşken seçili model başta durur", () => {
  const models = [model("a/bir"), model("b/iki"), model("c/uc")];
  const secili = models[2];

  const sonuc = modelAra(models, "", secili);
  assert.equal(sonuc[0], secili, "kullanıcı kendi seçimini ilk bakışta görmeli");
  assert.equal(sonuc.length, 3);
  assert.equal(new Set(sonuc).size, 3, "seçili model iki kez listelenmemeli");
});

test("liste MODEL_LIMIT ile sınırlanır", () => {
  const models = Array.from({ length: 200 }, (_, i) =>
    model(`p/model-${String(i).padStart(3, "0")}`),
  );

  assert.equal(modelAra(models, "").length, MODEL_LIMIT);
  assert.equal(modelAra(models, "model").length, MODEL_LIMIT);
});

/*
 * ASIL REGRESYON: Türkçe katlama.
 *
 * Bu modül düz `toLowerCase()` kullanırken "IŞIK" → "ışik" oluyordu ve
 * kullanıcının yazdığı "ışık" hiçbir zaman eşleşmiyordu.
 */
test("Türkçe büyük harfle yazılmış model adı küçük harfle bulunur", () => {
  const models = [model("kurum/ISIK-v2", { name: "IŞIK Modeli" })];

  assert.equal(modelAra(models, "ışık").length, 1, "noktasız I katlanmalı");
  assert.equal(modelAra(models, "IŞIK").length, 1);
});

test("sağlayıcı adı da Türkçe kurala göre aranır", () => {
  const models = [model("x/y", { providerName: "İŞ Bankası AI" })];

  assert.equal(modelAra(models, "iş").length, 1);
});

/*
 * Vurgulama, katlanmış metinde bulduğu konumu HAM metne uyguluyor. Katlama
 * uzunluğu değiştirirse dilimler kayar ve yanlış harfler kalınlaşır.
 */
test("vurgu aralığı ham metindeki doğru harfleri gösterir", () => {
  const metin = "İSTANBUL Modeli";
  const aralik = vurguAraligi(metin, "istanbul");

  assert.notEqual(aralik, null);
  const [bas, son] = aralik!;
  assert.equal(metin.slice(bas, son), "İSTANBUL",
    "düz toLowerCase ile 'İ' iki karaktere açılır ve dilim kayardı");
});

test("vurgu aralığı metnin ortasında da doğru", () => {
  const metin = "kurum/IŞIK-v2";
  const aralik = vurguAraligi(metin, "ışık");

  assert.notEqual(aralik, null);
  const [bas, son] = aralik!;
  assert.equal(metin.slice(bas, son), "IŞIK");
});

test("eşleşme yoksa ve sorgu boşken aralık yok", () => {
  assert.equal(vurguAraligi("claude-opus", "gemini"), null);
  assert.equal(vurguAraligi("claude-opus", "   "), null);
});
