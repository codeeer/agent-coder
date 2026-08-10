/**
 * Ayrıştırıcı testleri.
 *
 * Node'un yerleşik test koşucusuyla çalışır (`node --test`); ek bir test
 * kütüphanesi eklenmedi. Ayrıştırma saf bir fonksiyon olduğu için DOM'a da
 * gerek yok — React'tan ayrı tutulmasının sebebi tam olarak bu.
 */

import assert from "node:assert/strict";
import { test } from "node:test";
import { parseMarkdown } from "./parse.ts";

test("başlık seviyesi ve metni okunur", () => {
  const blocks = parseMarkdown("## Özet\n### Bulgular ###");
  assert.deepEqual(blocks, [
    { kind: "heading", level: 2, text: "Özet" },
    { kind: "heading", level: 3, text: "Bulgular" },
  ]);
});

test("paragraf satırları birleştirilir, boş satır böler", () => {
  const blocks = parseMarkdown("bir\niki\n\nüç");
  assert.deepEqual(blocks, [
    { kind: "paragraph", text: "bir iki" },
    { kind: "paragraph", text: "üç" },
  ]);
});

test("çitli kod bloğu dilini ve gövdesini korur", () => {
  const blocks = parseMarkdown("```go\nfmt.Println()\n\n// yorum\n```");
  assert.deepEqual(blocks, [
    { kind: "code", lang: "go", code: "fmt.Println()\n\n// yorum" },
  ]);
});

test("kapanmayan kod bloğu içeriği kaybetmez", () => {
  const blocks = parseMarkdown("```\nyarım kalan çıktı");
  assert.deepEqual(blocks, [
    { kind: "code", lang: "", code: "yarım kalan çıktı" },
  ]);
});

test("kod bloğunun içindeki başlık işareti başlık sayılmaz", () => {
  const blocks = parseMarkdown("```\n# bu bir yorum satırı\n```");
  assert.equal(blocks.length, 1);
  assert.equal(blocks[0]?.kind, "code");
});

test("tablo başlık, hizalama ve satırlarıyla okunur", () => {
  const blocks = parseMarkdown(
    "| Öğe | Değer |\n|---|---:|\n| Branch | `master` |\n| Commit | 1 |",
  );
  assert.deepEqual(blocks, [
    {
      kind: "table",
      header: ["Öğe", "Değer"],
      align: ["left", "right"],
      rows: [
        ["Branch", "`master`"],
        ["Commit", "1"],
      ],
    },
  ]);
});

test("ayıraç satırı olmayan boru işaretli metin tablo değildir", () => {
  const blocks = parseMarkdown("a | b | c");
  assert.deepEqual(blocks, [{ kind: "paragraph", text: "a | b | c" }]);
});

test("eksik hücreli tablo satırı tamamlanır", () => {
  const blocks = parseMarkdown("| a | b |\n|---|---|\n| yalnız |");
  const table = blocks[0];
  assert.equal(table?.kind, "table");
  assert.deepEqual(table.rows, [["yalnız", ""]]);
});

test("madde ve numaralı listeler ayrı bloklardır", () => {
  const blocks = parseMarkdown("- bir\n- iki\n1. üç");
  assert.equal(blocks.length, 2);
  assert.deepEqual(blocks[0], {
    kind: "list",
    ordered: false,
    items: [
      { text: "bir", depth: 0 },
      { text: "iki", depth: 0 },
    ],
  });
  assert.equal(blocks[1]?.kind, "list");
  assert.equal((blocks[1] as { ordered: boolean }).ordered, true);
});

test("girintili madde ikinci seviyeye düşer", () => {
  const blocks = parseMarkdown("- üst\n  - alt");
  const list = blocks[0];
  assert.equal(list?.kind, "list");
  assert.deepEqual(list.items.map((i) => i.depth), [0, 1]);
});

test("alıntı ve yatay çizgi tanınır", () => {
  const blocks = parseMarkdown("> not\n> devamı\n\n---");
  assert.deepEqual(blocks, [
    { kind: "quote", text: "not devamı" },
    { kind: "hr" },
  ]);
});

test("paragraf, sonrasındaki blok başlangıcında biter", () => {
  const blocks = parseMarkdown("metin\n## Başlık");
  assert.deepEqual(blocks, [
    { kind: "paragraph", text: "metin" },
    { kind: "heading", level: 2, text: "Başlık" },
  ]);
});

test("tanınmayan girdi kaybolmaz", () => {
  const blocks = parseMarkdown("<script>alert(1)</script>");
  assert.deepEqual(blocks, [
    { kind: "paragraph", text: "<script>alert(1)</script>" },
  ]);
});

test("CRLF satır sonları bozulmaya yol açmaz", () => {
  const blocks = parseMarkdown("# Başlık\r\n\r\nmetin");
  assert.deepEqual(blocks, [
    { kind: "heading", level: 1, text: "Başlık" },
    { kind: "paragraph", text: "metin" },
  ]);
});
