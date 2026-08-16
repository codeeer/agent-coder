import { test } from "node:test";
import assert from "node:assert/strict";

import {
  baslatEtiketi,
  devamEtiketi,
  gorunenleriBirak,
  iptalSonucu,
  ogeCalismaYolu,
  projeAra,
  secimAcKapa,
  silmeSonucu,
  tumunuSec,
} from "./batch-selection.ts";
import type { Project, RunBatchItem } from "@/lib/types";

/*
 * Toplu çalıştırma seçim kuralları (spec 023).
 *
 * Buradaki soruların hiçbiri ekran görüntüsüyle doğrulanamaz: otuz projelik bir
 * listede "tümünü seç" gerçekten ne seçti, devam düğmesi neden çıktı, iptal
 * onayı ne söz verdi. Hepsi sayıyla ifade ediliyor ve sayılar gözle sayılmaz.
 */

function proje(id: string, ek: Partial<Project> = {}): Project {
  return {
    id,
    name: id,
    // Host'ta "b" YOK: arama alt dizge üzerinde çalışıyor ve
    // "bb.sirket.com" her projeyi "b" sorgusuyla eşleştirirdi.
    repoUrl: `https://git.sirket.com/scm/ODEME/${id}.git`,
    defaultBranch: "main",
    gitProviderId: null,
    defaultNodeVersion: "",
    runCount: 0,
    createdAt: "",
    updatedAt: "",
    ...ek,
  };
}

test("arama ad ve adres üzerinde çalışır", () => {
  const liste = [
    proje("odeme-api", { name: "Ödeme API" }),
    proje("kargo-web", { name: "Kargo Web" }),
  ];

  assert.deepEqual(
    projeAra(liste, "ödeme").map((p) => p.id),
    ["odeme-api"],
  );
  // Adres de aranır: aynı adı taşıyan iki proje ancak adresten ayırt edilir.
  assert.deepEqual(
    projeAra(liste, "scm/ODEME/kargo").map((p) => p.id),
    ["kargo-web"],
  );
  assert.equal(projeAra(liste, "   ").length, 2, "boş sorgu süzmez");
});

test("seçim aç/kapa", () => {
  const bir = secimAcKapa(new Set<string>(), "a");
  assert.deepEqual([...bir], ["a"]);
  assert.deepEqual([...secimAcKapa(bir, "a")], []);
});

/*
 * "TÜMÜNÜ SEÇ" YALNIZCA GÖRÜNENLERİ ALIR.
 *
 * Süzgecin dışındakileri de alsaydı kullanıcı gördüğü iki satırı seçtiğini
 * sanır, otuz proje sıraya girerdi — ve bunu ancak toplu iş ekranında,
 * başlattıktan sonra fark ederdi.
 */
test("tümünü seç: süzgecin dışındakini almaz, önceki seçimi silmez", () => {
  const hepsi = [proje("a"), proje("b"), proje("c")];
  const gorunen = projeAra(hepsi, "b");

  const secili = tumunuSec(new Set(["a"]), gorunen);

  assert.deepEqual([...secili].sort(), ["a", "b"]);
});

test("seçimi temizle: yalnızca görünenleri bırakır", () => {
  const hepsi = [proje("a"), proje("b")];
  const secili = new Set(["a", "b"]);

  const kalan = gorunenleriBirak(secili, projeAra(hepsi, "b"));

  assert.deepEqual([...kalan], ["a"], "süzgecin dışındaki seçim korunur");
});

test("başlatma düğmesi kaç projede çalışacağını yazar", () => {
  assert.equal(baslatEtiketi(0), "Proje seçin");
  assert.equal(baslatEtiketi(1), "1 projede çalıştır");
  assert.equal(baslatEtiketi(30), "30 projede çalıştır");
});

/*
 * DEVAM DÜĞMESİ YALNIZCA KESİLMİŞ ÖĞE VARKEN ÇIKAR (spec 023 H5a).
 *
 * Başarısız olanlar çalıştı ve bir sonuç üretti; onları kendiliğinden
 * tekrarlamak yan etkilerini habersizce tekrarlamak olurdu.
 */
test("devam düğmesi: kesilen yoksa çıkmaz, varsa sayıyı yazar", () => {
  assert.equal(devamEtiketi(0), null);
  assert.equal(devamEtiketi(3), "Kaldığı yerden devam et (3 iş)");
});

/*
 * İPTALİN SONUCU ONAYDAN ÖNCE YAZILIR.
 *
 * "Emin misiniz?" hiçbir şey söylemez. Çalışan işlerin SÜRECEĞİ de yazılır;
 * kullanıcı "iptal" deyince her şeyin duracağını sanmamalı.
 */
test("iptal onayı: ne düşeceğini ve neyin süreceğini yazar", () => {
  assert.equal(
    iptalSonucu(12, 2),
    "12 bekleyen iş düşer; 2 çalışan iş kendi hâlinde sürer ve sonucu kaydedilir.",
  );
  assert.equal(iptalSonucu(5, 0), "5 bekleyen iş düşer.");
  assert.equal(iptalSonucu(0, 1),
    "Düşecek bekleyen iş yok; 1 çalışan iş kendi hâlinde sürer ve sonucu kaydedilir.");
});

test("öğe yalnızca çalışması varken bağlantı olur", () => {
  const oge = (workflowRunId: string | null): RunBatchItem => ({
    id: "i1",
    batchId: "b1",
    projectId: "p1",
    projectName: "Ödeme API",
    position: 0,
    status: workflowRunId ? "running" : "pending",
    workflowRunId,
    error: "",
    startedAt: null,
    finishedAt: null,
  });

  assert.equal(ogeCalismaYolu("w1", oge("r1")), "/workflows/w1/runs/r1");
  assert.equal(ogeCalismaYolu("w1", oge(null)), null,
    "başlatılmamış öğe boş bir sayfaya bağlanmamalı");
});

/*
 * Silmenin sonucu.
 *
 * Silme GERİ ALINAMAZ ve yalnızca toplu işin kaydını değil, altındaki bütün
 * geçmişi götürüyor. Kullanıcının neyi kaybettiğini tıklamadan ÖNCE bilmesi
 * gerek; "emin misiniz?" bunu söylemez.
 */
test("silmeSonucu, gidecek iş sayısını ve geri alınamazlığı yazar", () => {
  const metin = silmeSonucu(30);
  assert.match(metin, /30/, "kaç işin geçmişinin gideceği yazılmalı");
  assert.match(metin, /geri alınamaz/i, "geri alınamazlık söylenmeli");
});

/*
TEK İŞTE "1 iş" DEĞİL.

Sayı çoğul kalıba sokulduğunda ("1 işin geçmişi") Türkçede kulağı tırmalıyor;
daha önemlisi, tek öğeli bir toplu iş yaygın (tek proje seçilerek de
başlatılabiliyor) ve o cümle her seferinde okunuyor.
*/
test("silmeSonucu, tek işte sayı saymaz", () => {
  assert.match(silmeSonucu(1), /geri alınamaz/i);
  assert.doesNotMatch(silmeSonucu(1), /\b1 iş/);
});
