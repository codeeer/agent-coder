import { test } from "node:test";
import assert from "node:assert/strict";

import { edgeAnchor, niceTicks, tickIndexes } from "./scale.ts";

/*
 * Grafik ölçek matematiği.
 *
 * Bu fonksiyonlar bir React bileşen dosyasının içinde durduğu için hiç test
 * edilmemişti; oysa üçü de gözle doğrulanamayan işler yapıyor. Aşağıdaki
 * iddiaların çoğu koddaki yorumların ta kendisi — yorum bir iddiadır, test
 * onun kanıtı.
 */

test("eksen 1/2/5 katlarına yuvarlanır", () => {
  // Ham en büyük değeri tavan yapmak "37" gibi okunmayan bir eksen bırakırdı.
  assert.deepEqual(niceTicks(37), [0, 10, 20, 30, 40]);
  assert.deepEqual(niceTicks(8), [0, 2, 4, 6, 8]);

  /*
   * `count` İSTENEN adım sayısıdır, garanti değil.
   *
   * 100 için kaba adım 25 çıkıyor ama 2,5 katı kuralda yok; bir üste, yani 5
   * katına yuvarlanıyor ve eksen üç etikete iniyor. İstenen dörde yaklaşmak
   * için 25 kullanılsaydı eksen "akılda kalır sayı" kuralını çiğnerdi — bu
   * bilinçli bir öncelik sırası.
   */
  assert.deepEqual(niceTicks(100), [0, 50, 100]);
});

test("eksen her zaman en büyük değeri kapsar", () => {
  for (const max of [1, 3, 7, 37, 99, 101, 1234, 0.004]) {
    const ticks = niceTicks(max);
    assert.ok(ticks[ticks.length - 1]! >= max, `${max} eksenin dışında kaldı`);
    assert.equal(ticks[0], 0, "eksen sıfırdan başlamalı");
  }
});

test("sıfır ve negatif en büyük değerde eksen çökmez", () => {
  // Hiç veri olmayan bir dönemde grafik yine çizilebilmeli.
  assert.deepEqual(niceTicks(0), [0, 1]);
  assert.deepEqual(niceTicks(-5), [0, 1]);
});

test("küçük adımlarda kayan nokta birikmez", () => {
  /*
   * Koddaki yorumun kanıtı: adım toplanarak üretilseydi 0,005 gibi bir adım
   * eksende "0,0100000002" gösterirdi. Çarparak üretmek bunu engelliyor.
   */
  const ticks = niceTicks(0.02);
  for (const t of ticks) {
    assert.equal(
      t,
      Number(t.toFixed(10)),
      `eksen değeri kayan nokta artığı taşıyor: ${t}`,
    );
  }
});

test("az sayıda gün varsa hepsi etiketlenir", () => {
  assert.deepEqual(tickIndexes(5), [0, 1, 2, 3, 4]);
  assert.deepEqual(tickIndexes(7), [0, 1, 2, 3, 4, 5, 6]);
});

test("çok gün varsa etiketler seyreltilir ve uçlar korunur", () => {
  // 90 günün 90 etiketi üst üste biner; eksen yalnızca konumu anlatmalı.
  const idx = tickIndexes(90);
  assert.ok(idx.length <= 7, `çok fazla etiket: ${idx.length}`);
  assert.equal(idx[0], 0, "ilk gün etiketlenmeli");
  assert.equal(idx[idx.length - 1], 89, "son gün etiketlenmeli");
});

test("seyreltilmiş etiketler artan ve yinelemesizdir", () => {
  for (const n of [8, 30, 90, 365]) {
    const idx = tickIndexes(n);
    assert.deepEqual(idx, [...new Set(idx)].sort((a, b) => a - b), `n=${n}`);
    assert.ok(idx.every((i) => i >= 0 && i < n), `n=${n} sınır dışı indeks`);
  }
});

test("uçtaki etiket içeri hizalanır, ortadakiler ortalanır", () => {
  // Ortalandığında yarısı çizim alanının dışına taşıyor ve kırpılıyordu.
  assert.equal(edgeAnchor(0, 30), "start");
  assert.equal(edgeAnchor(29, 30), "end");
  assert.equal(edgeAnchor(15, 30), "middle");
});

test("tek günlük dönemde etiket başa hizalanır", () => {
  // i === 0 önce sınanıyor; tek elemanlı listede ilk kural kazanır.
  assert.equal(edgeAnchor(0, 1), "start");
});
