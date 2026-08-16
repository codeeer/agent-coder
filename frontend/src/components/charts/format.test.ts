import { test } from "node:test";
import assert from "node:assert/strict";

/*
 * Rapor sayılarının biçimlendirilmesi.
 *
 * Bu modül raporun TAMAMINI biçimlendiriyor: grafikteki, ipucundaki ve
 * tablodaki her rakam buradan geçiyor. Buradaki bir hata veriyi bozmuyor —
 * daha kötüsünü yapıyor, doğru veriyi yanlış okutuyor. Sessiz arıza sınıfı:
 * hiçbir şey patlamıyor, kullanıcı yalnızca yanlış rakama bakıyor.
 *
 * SAAT DİLİMİ BİLEREK UTC DEĞİL. Gün etiketi iddiası ancak testin saat dilimi
 * UTC'den farklıysa bir şey ölçer: UTC'de çalışan bir test, `timeZone: "UTC"`
 * kaldırılsa bile geçerdi. New York (UTC−4/−5) seçildi çünkü gece yarısı UTC
 * damgası orada bir ÖNCEKİ güne düşüyor — kayma olursa görünür.
 *
 * Bu yüzden modül DİNAMİK yükleniyor: `format.ts` gün biçimlendiricisini modül
 * seviyesinde bir kez kuruyor ve statik `import` hoist edildiği için TZ
 * atamasından önce çalışırdı.
 */
process.env.TZ = "America/New_York";

const {
  changeRatio,
  formatCompact,
  formatCount,
  formatDayLabel,
  formatDayLong,
  formatDuration,
  formatMoney,
  formatPercent,
  formatPerUnit,
  moneyAxis,
} = await import("./format.ts");

/* ---------------------------------------------------------------- para --- */

/*
KÜÇÜK TUTAR "BEDAVA" GÖRÜNMEZ.

Model çağrıları kuruşun binde birleriyle ölçülüyor. İki basamağa yuvarlanınca
0,0009 $ ekranda "0,00 $" oluyor ve kullanıcı o çağrının hiçbir şeye mal
olmadığını sanıyor — oysa binlerce kez tekrarlanan bir adım.
*/
test("bir kuruşun altındaki tutar dört basamakla yazılır", () => {
  assert.match(formatMoney(0.0009), /0,0009/);
  assert.doesNotMatch(formatMoney(0.0009), /^0,00\s/);
});

test("bir kuruş ve üstü iki basamakla yazılır", () => {
  assert.match(formatMoney(0.01), /0,01/);
  assert.match(formatMoney(12.5), /12,50/);
});

// Sıfır da biçimlendiriciden geçer: elle yazılmış bir "0,00 $" dizisi, aynı
// eksende biçimlendiricinin ürettiğinden farklı görünürdü.
test("sıfır da para gibi biçimlenir", () => {
  assert.equal(formatMoney(0), formatMoney(0));
  assert.match(formatMoney(0), /0,00/);
  assert.ok(formatMoney(0).includes("$"), "para birimi düşmemeli");
});

test("eksi tutar işaretini korur", () => {
  assert.match(formatMoney(-3.5), /-|−/);
});

/*
EKSENİN BASAMAK SAYISI TEK SEFERDE SEÇİLİR.

Etiket başına karar verilseydi aynı eksende "0,0050 $" ile "0,01 $" yan yana
dururdu ve okuyucu iki farklı birim gördüğünü sanardı. Karar EN BÜYÜK değere
göre bir kez verilir, tüm etiketlere aynı biçim uygulanır.
*/
test("eksen etiketleri kendi aralarında aynı basamağı kullanır", () => {
  const basamak = (s: string) => (s.split(",")[1] ?? "").replace(/\D/g, "").length;

  const kucuk = moneyAxis(0.05);
  assert.equal(basamak(kucuk(0.005)), basamak(kucuk(0.05)));
  assert.equal(basamak(kucuk(0.05)), 4, "küçük eksen dört basamak ister");

  const buyuk = moneyAxis(12);
  assert.equal(basamak(buyuk(0.005)), basamak(buyuk(12)));
  assert.equal(basamak(buyuk(12)), 2);
});

/* --------------------------------------------------------------- sayı ---- */

test("tam sayı binlik ayraç alır ve yuvarlanır", () => {
  assert.equal(formatCount(1284), "1.284");
  assert.equal(formatCount(1284.6), "1.285");
});

/*
KISALTMA EŞİĞİ 10.000.

Altında kısaltma YAPILMAZ: "1.284" bir token sayısı olarak okunur, "1,3 B"
okunmaz. Üstünde tam sayı yazmak sütunu taşırır.
*/
test("on binin altı kısaltılmaz, üstü B olur", () => {
  assert.equal(formatCompact(9999), "9.999");
  assert.equal(formatCompact(12_900), "12,9 B");
  assert.equal(formatCompact(4_200_000), "4,2 Mn");
});

test("kısaltma eksi sayıda da çalışır", () => {
  assert.equal(formatCompact(-12_900), "-12,9 B");
});

/* --------------------------------------------------------------- süre ---- */

/*
SÜRESİZ İŞ "0 sn" DEĞİL "—".

Sıfır, çalışmamış bir işi "anında bitti" gibi gösterir. Ürünün kuralı bu:
ölçülmemiş şey rakamla değil tireyle yazılır.
*/
test("sıfır ve eksi süre tire olur", () => {
  assert.equal(formatDuration(0), "—");
  assert.equal(formatDuration(-5), "—");
});

test("bir dakikanın altı saniye yazar", () => {
  assert.equal(formatDuration(45), "45 sn");
});

test("dakika ve saniye birlikte yazılır, tam dakikada saniye düşer", () => {
  assert.equal(formatDuration(125), "2 dk 5 sn");
  assert.equal(formatDuration(120), "2 dk");
});

test("bir saatin üstü saat ve dakika yazar", () => {
  assert.equal(formatDuration(3661), "1 sa 1 dk");
});

/* ------------------------------------------------------------- yüzde ----- */

// Bölen sıfırken oran UYDURULMAZ: 0/0 için "%0" yazmak, hiç çalıştırılmamış
// bir şeyi başarısız gibi gösterirdi.
test("bölen sıfırsa yüzde tire olur", () => {
  assert.equal(formatPercent(0, 0), "—");
  assert.equal(formatPercent(5, 0), "—");
});

test("yüzde tam sayıya yuvarlanır", () => {
  assert.equal(formatPercent(1, 3), "%33");
  assert.equal(formatPercent(2, 3), "%67");
});

/* -------------------------------------------------------------- tarih ---- */

/*
GÜN ETİKETİ KAYMAZ.

Gün etiketini backend rapor saat diliminde üretiyor. Tarayıcı onu kendi yerel
saatiyle yeniden yorumlarsa gün bir kayar: UTC−5'te "2026-08-09" gece yarısı
damgası 8 Ağustos akşamına düşer ve grafikteki sütun bir gün geriye kayar.
Rapor o zaman yanlış günü anlatır — üstelik hiçbir uyarı vermeden.

Bu test New York saatinde koşuyor (dosya başındaki nota bkz.); UTC'de koşsaydı
`timeZone: "UTC"` kaldırılsa bile geçerdi.
*/
test("gün etiketi yerel saat diliminde geriye kaymaz", () => {
  assert.equal(formatDayLabel("2026-08-09"), "9 Ağu");
  assert.equal(formatDayLabel("2026-01-01"), "1 Oca");
});

test("uzun gün etiketi de kaymaz ve hafta gününü yazar", () => {
  const uzun = formatDayLong("2026-08-09");
  assert.match(uzun, /9 Ağustos/);
  assert.match(uzun, /Paz/, "9 Ağustos 2026 pazar günü");
});

/* ------------------------------------------------------------- değişim --- */

/*
ÖNCEKİ DÖNEM YOKSA ORAN YOK.

Sıfırdan artışı yüzdeyle anlatmanın matematiksel bir karşılığı yok; "%∞" ya da
"%100" yazmak uydurma olurdu. `null` dönmesi, kartın "önceki dönem yok"
yazmasını sağlıyor.
*/
test("önceki dönem sıfır ya da eksiyse oran null", () => {
  assert.equal(changeRatio(10, 0), null);
  assert.equal(changeRatio(10, -1), null);
});

test("oran işaretiyle birlikte hesaplanır", () => {
  assert.equal(changeRatio(150, 100), 0.5);
  assert.equal(changeRatio(50, 100), -0.5);
  assert.equal(changeRatio(100, 100), 0);
});

/* ---------------------------------------------------- birim maliyet ------ */

// Payda sıfırken oran uydurulmaz — sıfıra bölmek `Infinity` verir ve ekranda
// "∞ $ / PR" belirirdi.
test("birim sayısı sıfırsa birim maliyet tire olur", () => {
  assert.equal(formatPerUnit(12, 0, "PR"), "—");
});

test("birim maliyet para biçimini ve birimi taşır", () => {
  const s = formatPerUnit(1, 250, "PR");
  assert.match(s, /\/ PR$/);
  assert.match(s, /0,0040/, "kuruş altı tutar dört basamak kalmalı");
});
