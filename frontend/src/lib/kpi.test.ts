import { test } from "node:test";
import assert from "node:assert/strict";

import { PANO_KPI, RAPOR_KPI, kpi, type KpiKey } from "./kpi.ts";
import type { ReportSummary, ReportTotals } from "@/lib/types";

/*
 * Dönem rakamlarının tanımı.
 *
 * Bu tanımlar pano ve rapor ekranlarında AYRI AYRI yazılıydı. Ayrışması
 * görülmezdi: yanlış hizalanmış bir kart göze çarpar, aynı dönemin başarı
 * oranını iki ekranda farklı hesaplamak çarpmaz.
 */

function totals(ek: Partial<ReportTotals> = {}): ReportTotals {
  return {
    runs: 0,
    succeeded: 0,
    failed: 0,
    cancelled: 0,
    timeout: 0,
    interrupted: 0,
    active: 0,
    costUsd: 0,
    promptTokens: 0,
    completionTokens: 0,
    filesChanged: 0,
    additions: 0,
    deletions: 0,
    pushedBranches: 0,
    runsWithCode: 0,
    prsOpened: 0,
    jiraTasks: 0,
    avgDurationSec: 0,
    ...ek,
  };
}

function ozet(t: Partial<ReportTotals>, p: Partial<ReportTotals> = {}): ReportSummary {
  return {
    from: "2026-01-01",
    to: "2026-01-30",
    days: 30,
    timezone: "Europe/Istanbul",
    totals: totals(t),
    previous: totals(p),
    daily: [],
    byAgent: [],
    byModel: [],
    byProject: [],
    failures: [],
  };
}

/*
 * BAŞARI ORANININ PAYDASI biten çalıştırmadır, toplam değil.
 *
 * Süren işler henüz başarılı ya da başarısız değil; paydaya konurlarsa oran,
 * iş bittikçe kendiliğinden yükselen yanlış bir sayı olur. İki ekranda ayrı
 * yazıldığı sürece birinin bunu kaçırması mümkündü.
 */
test("başarı oranı süren işleri paydaya katmaz", () => {
  // 10 çalıştırma, 4'ü sürüyor, biten 6'nın 3'ü başarılı → %50, %30 değil.
  const k = kpi("success", ozet({ runs: 10, active: 4, succeeded: 3 }));

  assert.equal(k.current, 0.5);
  // Türkçe biçimde yüzde işareti BAŞTA: "%50".
  assert.ok(k.value.includes("50"), `beklenen %50, gelen: ${k.value}`);
  assert.ok(!k.value.includes("30"), `payda toplam çalıştırma olmuş: ${k.value}`);
});

test("başarı oranı önceki dönemde de aynı paydayı kullanır", () => {
  const k = kpi(
    "success",
    ozet({ runs: 10, active: 0, succeeded: 5 }, { runs: 10, active: 5, succeeded: 5 }),
  );

  assert.equal(k.current, 0.5);
  assert.equal(k.previous, 1, "önceki dönemde biten 5 işin 5'i başarılı");
});

test("hiç biten iş yokken oran sıfır — NaN değil", () => {
  // Sıfıra bölme ekrana "NaN%" yazardı.
  const k = kpi("success", ozet({ runs: 3, active: 3, succeeded: 0 }));

  assert.equal(k.current, 0);
  assert.ok(Number.isFinite(k.current));
});

test("token ve satır rakamları toplanarak türer", () => {
  const t = kpi("tokens", ozet({ promptTokens: 1200, completionTokens: 800 }));
  assert.equal(t.current, 2000);

  const s = kpi("linesChanged", ozet({ additions: 90, deletions: 10 }));
  assert.equal(s.current, 100);
});

/*
 * Kırılım, komşu "değişen dosya" rakamından ayırt edilmesini sağlıyor: ikisi
 * tesadüfen aynı sayıya düştüğünde aynı şeyi sayıyorlar sanılıyordu.
 */
test("değişen satır kırılımı ekleme ve silmeyi ayrı gösterir", () => {
  const k = kpi("linesChanged", ozet({ additions: 9, deletions: 0 }));

  assert.ok(k.detail?.includes("9"), `kırılım yok: ${k.detail}`);
  assert.notEqual(k.detail, undefined);
});

test("yalnızca maliyet ve süre için artış kötüdür", () => {
  const kotu: KpiKey[] = ["cost", "avgDuration"];
  const notr: KpiKey[] = ["tokens", "filesChanged", "linesChanged"];
  const iyi: KpiKey[] = [
    "runs",
    "succeeded",
    "success",
    "prsOpened",
    "jiraTasks",
    "pushedBranches",
  ];

  const bos = ozet({});
  for (const k of kotu) assert.equal(kpi(k, bos).upIsGood, false, k);
  for (const k of notr) assert.equal(kpi(k, bos).upIsGood, null, k);
  for (const k of iyi) assert.equal(kpi(k, bos).upIsGood, true, k);
});

/*
 * İKİ EKRANIN ORTAK RAKAMLARI AYNI OLMALI.
 *
 * Pano sekiz, rapor on rakam gösteriyor ve sıraları farklı — bu bilinçli.
 * Ama kesişimdeki yedi rakamın etiketi ve değeri birebir aynı olmak zorunda:
 * kullanıcı iki ekranda aynı dönemi görüyor.
 */
test("pano ve raporun kesişen rakamları birebir aynı", () => {
  // Listeler ARTIK ÜRÜNDEN geliyor. Önceden teste elle kopyalanmışlardı ve
  // ekranlar değişse test fark etmezdi — korumaya çalıştığı ayrışmanın kendisi.
  const kesisim = PANO_KPI.filter((k) => (RAPOR_KPI as readonly string[]).includes(k));
  assert.ok(kesisim.length >= 7, `kesişim beklenenden küçük: ${kesisim.length}`);

  const veri = ozet(
    {
      runs: 42,
      active: 2,
      succeeded: 30,
      prsOpened: 12,
      promptTokens: 5000,
      completionTokens: 2500,
      costUsd: 3.75,
      avgDurationSec: 128,
      filesChanged: 64,
    },
    { runs: 30, active: 0, succeeded: 20, costUsd: 2 },
  );

  /*
    Beklenen etiketler AÇIKÇA yazılı.

    Önceki hali `kpi(k, veri)` ile yine `kpi(k, veri)`'yi karşılaştırıyordu —
    aynı fonksiyonun aynı argümanlarla iki çağrısı, yani yalnızca determinizm
    ölçülüyordu. Ölçüldü: `kpi.ts`'te etiket bozulsa da test geçiyordu.
  */
  const beklenen: Record<string, string> = {
    runs: "Çalıştırma",
    success: "Başarı",
    prsOpened: "Açılan PR",
    tokens: "Token",
    cost: "Maliyet",
    avgDuration: "Ort. süre",
    filesChanged: "Değişen dosya",
  };

  for (const k of kesisim) {
    const kart = kpi(k, veri);
    assert.equal(kart.label, beklenen[k], `${k} etiketi değişmiş`);
    assert.ok(kart.value.length > 0, `${k} değersiz`);
  }

  // Kesişimdeki her anahtarın beklenen tabloda karşılığı olmalı: yeni bir
  // rakam iki ekrana birden eklenirse bu test onu görmeden geçmemeli.
  for (const k of kesisim) {
    assert.ok(k in beklenen, `${k} için beklenen etiket yazılmamış`);
  }
});
