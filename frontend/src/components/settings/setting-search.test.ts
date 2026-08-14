/**
 * Ayar araması testleri.
 *
 * Türkçe katlama gözle doğrulanamaz: "IŞIK" varsayılan `toLowerCase()` ile
 * "ışik" olur ve kullanıcının yazdığı "ışık" hiçbir zaman eşleşmez — ekranda
 * görünen tek şey "eşleşme yok" olduğu için hata da bir arama sonucundan ayırt
 * edilemez. Bu yüzden mantık bileşenin dışında ve burada sınanıyor.
 */

import assert from "node:assert/strict";
import { test } from "node:test";
import { filterSettings, settingMatches } from "./setting-search.ts";
import type { SettingValue } from "@/lib/types";

function setting(over: Partial<SettingValue> = {}): SettingValue {
  return {
    key: "runner.timeout_minutes",
    group: "runner",
    label: "Çalıştırma süre sınırı",
    help: "Bir koşunun tamamlanması için verilen azami süre.",
    kind: "int",
    default: "30",
    value: "30",
    isCustom: false,
    ...over,
  };
}

test("etikette eşleşme", () => {
  assert.equal(settingMatches(setting(), "süre"), true);
});

test("açıklamada eşleşme", () => {
  assert.equal(settingMatches(setting(), "koşunun"), true);
});

test("ne etikette ne açıklamada varsa eşleşmez", () => {
  assert.equal(settingMatches(setting(), "zzz"), false);
});

test("ham anahtar aranmaz — ekranda görünmeyen metin sonuç üretmez", () => {
  // "timeout" yalnızca `key` içinde geçiyor; etiket ve açıklama Türkçe.
  assert.equal(settingMatches(setting(), "timeout"), false);
  assert.equal(settingMatches(setting(), "runner"), false);
});

test("Türkçe katlama: çalışma ↔ ÇALIŞMA", () => {
  assert.equal(settingMatches(setting({ label: "ÇALIŞMA SÜRESİ" }), "çalışma"), true);
  assert.equal(settingMatches(setting({ label: "Çalışma süresi" }), "ÇALIŞMA"), true);
});

test("Türkçe katlama: ışık ↔ IŞIK (noktasız I)", () => {
  assert.equal(settingMatches(setting({ label: "IŞIK" }), "ışık"), true);
  assert.equal(settingMatches(setting({ label: "ışık" }), "IŞIK"), true);
});

test("Türkçe katlama: iş ↔ İŞ (noktalı İ)", () => {
  assert.equal(settingMatches(setting({ label: "İŞ akışı" }), "iş"), true);
  assert.equal(settingMatches(setting({ label: "iş akışı" }), "İŞ"), true);
});

test("baştaki ve sondaki boşluk kırpılır", () => {
  assert.equal(settingMatches(setting(), "  süre  "), true);
});

test("çok kelimeli sorguda kelimelerin hepsi eşleşmeli", () => {
  // İkisi de var: biri etikette, diğeri açıklamada.
  assert.equal(settingMatches(setting(), "süre koşunun"), true);
  // Biri var, diğeri yok → eşleşme yok. Kelime eklemek daraltır.
  assert.equal(settingMatches(setting(), "süre zzz"), false);
});

test("boş sorgu süzmez — liste olduğu gibi döner", () => {
  const items = [setting(), setting({ key: "a", label: "Başka" })];
  assert.deepEqual(filterSettings(items, ""), items);
  assert.deepEqual(filterSettings(items, "   "), items);
});

test("filterSettings yalnızca eşleşenleri döndürür", () => {
  const items = [
    setting({ key: "a", label: "Motor loglarını sakla", help: "Ham loglar." }),
    setting({ key: "b", label: "Rapor dönemi", help: "Varsayılan gün sayısı." }),
  ];
  assert.deepEqual(
    filterSettings(items, "log").map((s) => s.key),
    ["a"],
  );
  assert.deepEqual(filterSettings(items, "zzz"), []);
});
