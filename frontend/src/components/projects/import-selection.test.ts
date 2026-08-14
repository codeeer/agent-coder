import { test } from "node:test";
import assert from "node:assert/strict";

import {
  secilebilir,
  varsayilanSecim,
  secimAcKapa,
  tumunuSec,
} from "./import-selection.ts";
import type { ImportRepo } from "@/lib/types";

/*
 * Seçim kuralları (spec 021 H2).
 *
 * Bileşenin içine gömülmüyor: "hangi kayıt seçili gelir" sorusunun cevabı
 * ekran görüntüsüyle doğrulanamaz — kırk repository'lik bir listede gözle
 * sayılamaz.
 */

function repo(slug: string, ek: Partial<ImportRepo> = {}): ImportRepo {
  return {
    slug,
    name: slug,
    cloneUrl: `https://bb/scm/O/${slug}.git`,
    archived: false,
    status: "new",
    ...ek,
  };
}

test("zaten kayıtlı olan seçilemez", () => {
  assert.equal(secilebilir(repo("api")), true);
  assert.equal(secilebilir(repo("web", { status: "already_registered" })), false);
});

test("varsayılan seçim: yeni olanlar", () => {
  const liste = [repo("api"), repo("web", { status: "already_registered" })];

  assert.deepEqual(varsayilanSecim(liste), ["api"]);
});

/*
 * ARŞİVLİ OLANLAR SEÇİLİ GELMEZ ama listede durur ve elle seçilebilir
 * (spec 021 → kullanıcı kararı). Gizlenselerdi kullanıcı arşivlenmiş bir
 * repository'yi hiç ekleyemezdi.
 */
test("arşivli olan seçili gelmez", () => {
  const liste = [repo("api"), repo("eski", { archived: true })];

  assert.deepEqual(varsayilanSecim(liste), ["api"]);
  assert.equal(secilebilir(repo("eski", { archived: true })), true);
});

test("seçim açılıp kapanır", () => {
  assert.deepEqual([...secimAcKapa(new Set(["a"]), "b")].sort(), ["a", "b"]);
  assert.deepEqual([...secimAcKapa(new Set(["a", "b"]), "b")], ["a"]);
});

// "Tümünü seç" ZATEN KAYITLI OLANLARI ALMAZ: alsaydı kullanıcı seçtiğini
// sanır, sonuç ekranında hepsinin atlandığını görürdü.
test("tümünü seç yalnızca seçilebilirleri alır", () => {
  const liste = [
    repo("api"),
    repo("eski", { archived: true }),
    repo("web", { status: "already_registered" }),
  ];

  assert.deepEqual(tumunuSec(liste).sort(), ["api", "eski"]);
});
