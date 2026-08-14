import { test } from "node:test";
import assert from "node:assert/strict";

import { reportPeriods } from "./report-periods.ts";

/*
 * GERÇEK BİR HATANIN KAYDI: "Varsayılan rapor dönemi" ayarı 7 yapıldığı hâlde
 * Raporlar sayfası 30 gün açılıyordu. Sayfa dönemi kendi içinde sabit tutuyor
 * ve her istekte AÇIKÇA gönderiyordu; backend ayarı yalnızca parametre
 * gelmediğinde uyguladığı için ayar hiç devreye girmiyordu.
 *
 * Dönem artık yanıttan okunuyor. Bu da yeni bir soru doğuruyor: ayar 7/30/90
 * dışında bir değer olabilir (1–365 arası serbest). O zaman hiçbir segment
 * seçili görünmezdi — kullanıcı 14 günlük veriye bakarken denetim boş dururdu.
 */

test("varsayılan seçenekler 7/30/90", () => {
  assert.deepEqual(
    reportPeriods(null).map((s) => s.id),
    ["7", "30", "90"],
  );
});

test("etkin dönem listedeyse yeni seçenek eklenmez", () => {
  assert.deepEqual(
    reportPeriods(7).map((s) => s.id),
    ["7", "30", "90"],
  );
});

test("listede olmayan etkin dönem SIRASINA eklenir", () => {
  assert.deepEqual(
    reportPeriods(14).map((s) => s.id),
    ["7", "14", "30", "90"],
  );
});

test("etkin dönem en büyükse sona eklenir", () => {
  assert.deepEqual(
    reportPeriods(365).map((s) => s.id),
    ["7", "30", "90", "365"],
  );
});

test("etiket birim taşır", () => {
  assert.equal(reportPeriods(null)[0]?.label, "7 gün");
});

// Anlamsız değerler listeyi kirletmez: 0 "dönem yok" demek, negatif zaten
// geçersiz. İkisi de yalnızca varsayılan listeyi döndürür.
test("sıfır ve negatif yok sayılır", () => {
  assert.deepEqual(
    reportPeriods(0).map((s) => s.id),
    ["7", "30", "90"],
  );
  assert.deepEqual(
    reportPeriods(-5).map((s) => s.id),
    ["7", "30", "90"],
  );
});
