import { test } from "node:test";
import assert from "node:assert/strict";

import { describeApiError } from "./error-messages.ts";

/*
 * Backend hata kodunun kullanıcı diline çevrilmesi.
 *
 * Bu eşleme ürünün HER hata mesajını belirliyor ve tek işi kullanıcıya ne
 * yapacağını söylemek. Yanlış çeviri sessizdir: kullanıcı ekranda bir cümle
 * görür, o cümlenin yanlış işi önerdiğini anlamasının hiçbir yolu yoktur.
 *
 * `errors.ts` `node --test` altında yüklenemiyor: `api.ts` üzerinden geliyor ve
 * o dosya TypeScript parameter property kullanıyor (tip soyma kipi
 * desteklemiyor). Bu yüzden çeviri saf bir modüle alındı — `error-hints.ts`
 * ile aynı gerekçe, aynı kalıp.
 */

/*
"DEĞER YANLIŞ" İLE "SERVİSE ULAŞILAMADI" AYRI KALIR.

Kullanıcı ikisinde farklı şey yapar: birinde anahtarı düzeltir, diğerinde
tekrar dener. Tek mesaja indirgenselerdi, ağı bir an kopan kullanıcı ÇALIŞAN
anahtarını silip yenisini aramaya başlardı.
*/
test("geçersiz değer ile ulaşılamayan servis farklı şey söyler", () => {
  const deger = describeApiError("invalid_credential", "ham mesaj");
  const servis = describeApiError("service_unreachable", "ham mesaj");

  assert.notEqual(deger.message, servis.message);
  assert.match(deger.message, /doğrulanamadı/i);
  assert.match(servis.message, /ulaşılamadı/i);
  assert.match(servis.hint ?? "", /tekrar deneyin/i, "servis hatası tekrar denemeyi önermeli");
});

/*
ADRES HATASININ İPUCU EKRANA GÖRE DEĞİŞİR — VE BAĞLAM YOKSA HİÇ VERİLMEZ.

Bir zamanlar koşulsuz "sonunda /v1 olmalı" yazıyordu. Bitbucket Server
doğrulaması 404'ü adres hatası olarak raporlamaya başlayınca aynı metin git
ekranında da göründü ve kurumsal kullanıcıya ÇALIŞAN adresini bozmasını
önerir hale geldi.
*/
test("adres hatası ipucunu bağlamdan alır", () => {
  assert.match(describeApiError("invalid_base_url", "m", "llm").hint ?? "", /\/v1/);
  assert.doesNotMatch(describeApiError("invalid_base_url", "m", "git").hint ?? "", /\/v1/);
});

test("bağlam bilinmiyorsa adres ipucu yazılmaz", () => {
  const r = describeApiError("invalid_base_url", "sunucu bu adreste API sunmuyor (404)");
  assert.equal(r.hint, undefined, "yanlış ipucu, ipucu olmamasından kötüdür");
  assert.equal(r.message, "sunucu bu adreste API sunmuyor (404)",
    "adres hatasında backend'in ayrıntılı mesajı korunur");
});

/*
BİLİNMEYEN KOD BACKEND'İN MESAJINI GEÇİRİR.

Yutulup "bir hata oluştu"ya çevrilseydi, backend'in özenle yazdığı her yeni
hata mesajı arayüzde sessizce kaybolurdu — üstelik kod tarafında hiçbir uyarı
çıkmadan.
*/
test("eşlenmemiş kod backend mesajını olduğu gibi gösterir", () => {
  const r = describeApiError("bilinmeyen_kod", "Toplu iş sürüyor — önce iptal edin");
  assert.equal(r.message, "Toplu iş sürüyor — önce iptal edin");
  assert.equal(r.hint, undefined);
});

// Ağ ve zaman aşımı AYNI şeyi söyler: ikisinde de kullanıcının yapacağı iş
// servislerin ayakta olup olmadığına bakmak.
test("ağ hatası ve zaman aşımı aynı yönlendirmeyi verir", () => {
  const ag = describeApiError("network_error", "x");
  const zaman = describeApiError("timeout", "x");

  assert.deepEqual(ag, zaman);
  assert.match(ag.message, /Backend'e ulaşılamadı/);
  assert.match(ag.hint ?? "", /make ps/, "kullanıcıya çalıştırılabilir bir komut verilmeli");
});

// Kullanıcı adı eksikse sebebi yazılır: Bitbucket app password tek başına
// çalışmıyor ve kullanıcı bunu bilmiyor.
test("eksik kullanıcı adı sebebiyle birlikte söylenir", () => {
  const r = describeApiError("missing_username", "x");
  assert.match(r.message, /Kullanıcı adı zorunlu/);
  assert.match(r.hint ?? "", /app password/i);
});

test("bozuk model kataloğu adres düzeltmesi önerir", () => {
  const r = describeApiError("bad_catalog", "x");
  assert.match(r.hint ?? "", /\/v1/);
});
