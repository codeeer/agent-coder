/**
 * Satır içi ayrıştırıcı testleri.
 *
 * Bu dosyanın var olma sebebi bir kaza: ilk sürümde ayrıştırma çizim kodunun
 * içine gömülüydü ve test edilemiyordu. İçindeki sonsuz döngü ancak tarayıcı
 * 32 GB bellek tüketip sistemi kilitlediğinde görüldü (spec 005, Ölçüm 1).
 *
 * Aşağıdaki "tekrar eden kalın metin" ve "iç içe işaretleme" testleri tam olarak
 * o hatayı yakalar: hata geri gelirse bu testler biter demez.
 */

import assert from "node:assert/strict";
import { test } from "node:test";
import { safeHref, tokenizeInline, type InlineToken } from "./inline.ts";

/** Parçaları okunur bir özete indirger; karşılaştırmayı kısaltır. */
function flat(tokens: InlineToken[]): string {
  return tokens
    .map((t) => {
      switch (t.kind) {
        case "text":
          return t.text;
        case "code":
          return `code(${t.text})`;
        case "strong":
          return `strong(${flat(t.children)})`;
        case "em":
          return `em(${flat(t.children)})`;
        case "strike":
          return `strike(${flat(t.children)})`;
        case "link":
          return `link(${t.href}|${flat(t.children)})`;
      }
    })
    .join("");
}

test("düz metin tek parça döner", () => {
  assert.equal(flat(tokenizeInline("sade metin")), "sade metin");
});

test("kalın, italik, üstü çizili ve kod ayrı ayrı tanınır", () => {
  assert.equal(flat(tokenizeInline("**k**")), "strong(k)");
  assert.equal(flat(tokenizeInline("__k__")), "strong(k)");
  assert.equal(flat(tokenizeInline("*i*")), "em(i)");
  assert.equal(flat(tokenizeInline("_i_")), "em(i)");
  assert.equal(flat(tokenizeInline("~~ç~~")), "strike(ç)");
  assert.equal(flat(tokenizeInline("`kod`")), "code(kod)");
});

/*
 * ── Gerileme testi ────────────────────────────────────────────────────────
 * İlk sürümde `g` bayraklı düzenli ifade modül düzeyinde paylaşılıyordu.
 * Özyinelemeli çağrı `lastIndex`'i sıfırlayınca dış döngü aynı eşleşmeyi
 * sonsuza kadar buluyor ve her turda diziye bir eleman ekliyordu.
 *
 * Aşağıdaki iki durum o döngüyü tetikleyen en küçük girdilerdir.
 */

test("aynı satırda birden fazla kalın metin sonsuz döngüye girmez", () => {
  const tokens = tokenizeInline("**bir** arada **iki** ve **üç**");
  assert.equal(flat(tokens), "strong(bir) arada strong(iki) ve strong(üç)");
  assert.equal(tokens.length, 5, "her işaret bir kez üretilmeli");
});

test("iç içe işaretleme dış döngüyü bozmaz", () => {
  const tokens = tokenizeInline("**kalın `kod` içerir** sonra *italik*");
  assert.equal(
    flat(tokens),
    "strong(kalın code(kod) içerir) sonra em(italik)",
  );
});

test("gerçek agent çıktısındaki satır beklendiği gibi ayrışır", () => {
  // Kilitlenmenin görüldüğü çıktının birebir satırı.
  const tokens = tokenizeInline(
    "Commit, dosyanın sonuna yeni satır ekleyen bir merge PR. **İncelemeye uygun kod yok.**",
  );
  assert.equal(
    flat(tokens),
    "Commit, dosyanın sonuna yeni satır ekleyen bir merge PR. strong(İncelemeye uygun kod yok.)",
  );
});

test("kod içindeki yıldız işaretleme sayılmaz", () => {
  assert.equal(flat(tokenizeInline("`a * b * c`")), "code(a * b * c)");
});

test("bağlantı metni ve adresi ayrılır", () => {
  assert.equal(
    flat(tokenizeInline("[belge](https://ornek.test/a)")),
    "link(https://ornek.test/a|belge)",
  );
});

test("güvenli olmayan bağlantı bağlantı olmaz ama metin kaybolmaz", () => {
  const tokens = tokenizeInline("[tıkla](javascript:alert(1))");
  const first = tokens[0];
  assert.equal(first?.kind, "link");
  assert.equal(first.href, null, "güvensiz şema bağlantıya çevrilmemeli");
  // Metnin tamamı korunur; hiçbir karakter sessizce düşmez.
  assert.equal(flat(tokens), "link(null|tıkla))");
});

test("kapanmayan işaret düz metin kalır", () => {
  assert.equal(flat(tokenizeInline("**yarım kalan")), "**yarım kalan");
});

test("derin iç içe girdi çağrı yığınını taşırmaz", () => {
  // Sınırı aşan derinlikte metin ham haliyle döner; çökme veya donma olmaz.
  const derin = "*".repeat(200) + "x" + "*".repeat(200);
  const tokens = tokenizeInline(derin);
  assert.ok(tokens.length > 0);
  assert.ok(flat(tokens).includes("x"), "içerik kaybolmamalı");
});

test("safeHref yalnızca beyaz listedeki şemalara izin verir", () => {
  assert.equal(safeHref("https://ornek.test/a"), "https://ornek.test/a");
  assert.equal(safeHref("mailto:a@b.test"), "mailto:a@b.test");
  assert.equal(safeHref("/proje/1"), "/proje/1");
  assert.equal(safeHref("#bolum"), "#bolum");

  assert.equal(safeHref("javascript:alert(1)"), null);
  assert.equal(safeHref("JavaScript:alert(1)"), null);
  assert.equal(safeHref("data:text/html;base64,PHN2Zz4="), null);
  assert.equal(safeHref("  "), null);
});
