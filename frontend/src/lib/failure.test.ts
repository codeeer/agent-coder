/**
 * Hata metni sadeleştirme testleri.
 *
 * Rapor sayfası yöneticiye okunur bir cümle göstermeli; sağlayıcının JSON
 * gövdesi değil. Metin işleme bileşenin içine gömülseydi doğruluğu ancak
 * tarayıcıda görülebilirdi.
 */

import assert from "node:assert/strict";
import { test } from "node:test";
import { readableFailure } from "./failure.ts";

test("JSON gövdesi atılır, içindeki mesaj öne çıkar", () => {
  const raw =
    'model çağrısı başarısız: mesaj gönderilemedi: durum 500: ' +
    '{"name":"UnknownError","data":{"message":"Unexpected server error.","ref":"err_3f19"}}';
  assert.equal(
    readableFailure(raw),
    "model çağrısı başarısız: mesaj gönderilemedi: durum 500 — Unexpected server error.",
  );
});

test("JSON içermeyen metne dokunulmaz", () => {
  const raw = "zaman aşımı: 30 dakika doldu";
  assert.equal(readableFailure(raw), raw);
});

test("bozuk JSON gövdesinde en azından öncesi gösterilir", () => {
  assert.equal(
    readableFailure("depo klonlanamadı: {bu json değil"),
    "depo klonlanamadı",
  );
});

test("üst düzey message alanı da okunur", () => {
  assert.equal(
    readableFailure('istek reddedildi: {"message":"insufficient credits"}'),
    "istek reddedildi — insufficient credits",
  );
});

// Yalnızca JSON'dan ibaret bir hata metni boş satıra düşmemeli: kullanıcı
// "1 kez" rozetinin yanında bomboş bir alan görürdü.
test("yalnızca JSON gelirse mesaj tek başına gösterilir", () => {
  assert.equal(readableFailure('{"message":"rate limited"}'), "rate limited");
});
