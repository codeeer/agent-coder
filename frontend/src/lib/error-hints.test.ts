/**
 * Adres hatası ipucu testleri.
 *
 * Bu metin kullanıcının NE DEĞİŞTİRECEĞİNİ belirliyor. Yanlış ipucu, kullanıcıyı
 * doğru olan şeyi bozmaya iter: kurumsal Bitbucket adresinin sonuna `/v1`
 * eklemek gibi.
 *
 * `describeError`'ın kendisi buradan çağrılamıyor: `errors.ts` API istemcisini
 * içeri alıyor ve o zincir `node --test` altında çözülemiyor. Bu yüzden değişen
 * mantık `error-hints.ts` içine ayrıldı ve test edilen o.
 */

import assert from "node:assert/strict";
import { test } from "node:test";
import { baseUrlHint } from "./error-hints.ts";

/*
 * Korunan regresyon: ipucu bir zamanlar koşulsuz "/v1" örneği veriyordu.
 * Bitbucket Server doğrulaması 404'ü adres hatası olarak raporlamaya başlayınca
 * o metin git ekranında da görünür oldu.
 */
test("git ekranında /v1 önerilmez", () => {
  const ipucu = baseUrlHint("git") ?? "";
  assert.doesNotMatch(ipucu, /\/v1/);
  assert.match(ipucu, /bitbucket/i);
});

test("LLM ekranında /v1 örneği korunur", () => {
  // Kullanıcıların en sık atladığı ayrıntı; bu ipucu değerini koruyor.
  assert.match(baseUrlHint("llm") ?? "", /\/v1/);
});

test("bağlam bilinmiyorsa ipucu hiç yazılmaz", () => {
  // Yanlış ipucu, ipucu olmamasından kötüdür.
  assert.equal(baseUrlHint(), undefined);
});

test("iki ekranın ipuçları birbirinden farklı", () => {
  assert.notEqual(baseUrlHint("llm"), baseUrlHint("git"));
});
