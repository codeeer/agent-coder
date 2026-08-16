import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";

import {
  THEME_STORAGE_KEY,
  applyMode,
  readMode,
  themeBootstrapScript,
  writeMode,
  type ThemeMode,
} from "./theme.ts";

/*
 * Tema seçimi (spec 006).
 *
 * Üç durum var ve üçüncüsü —`system`— sürekli unutulanı: "koyu değil" ile
 * "işletim sistemini takip et" aynı şey değil. Karıştırıldığında arıza
 * sessizdir: kullanıcı temayı seçer, çalışır görünür, sonraki açılışta işletim
 * sistemi tercihi onunkini ezer.
 *
 * TARAYICI YOK: `localStorage`, `matchMedia` ve `document` burada elle
 * kuruluyor. Ölçülen şey tarayıcı değil KARAR — hangi girdide hangi temanın
 * seçildiği.
 */

type Depo = { veri: Map<string, string>; patla: boolean };
let depo: Depo;

/** kur, tarayıcı yüzeylerini sahteleriyle değiştirir. */
function kur(kayitli?: string, sistemKoyu = false, patla = false): void {
  depo = { veri: new Map(), patla };
  if (kayitli !== undefined) depo.veri.set(THEME_STORAGE_KEY, kayitli);

  const g = globalThis as Record<string, unknown>;
  g.localStorage = {
    getItem(k: string) {
      if (depo.patla) throw new Error("gizli sekmede erişim yok");
      return depo.veri.get(k) ?? null;
    },
    setItem(k: string, v: string) {
      if (depo.patla) throw new Error("gizli sekmede yazılamaz");
      depo.veri.set(k, v);
    },
  };
  g.window = {
    matchMedia: (q: string) => ({ matches: sistemKoyu && q.includes("dark") }),
    localStorage: g.localStorage,
  };
  g.document = { documentElement: { dataset: {} as Record<string, string> } };
}

function ekrandakiTema(): string | undefined {
  return (globalThis as unknown as {
    document: { documentElement: { dataset: Record<string, string> } };
  }).document.documentElement.dataset.theme;
}

beforeEach(() => kur());

/* ------------------------------------------------------------ okuma ----- */

test("kaydedilmiş üç değer olduğu gibi okunur", () => {
  for (const m of ["light", "dark", "system"] as ThemeMode[]) {
    kur(m);
    assert.equal(readMode(), m);
  }
});

/*
BOZUK DEĞER SİSTEME DÜŞER.

Depodaki değer elle düzenlenmiş, eski bir sürümden kalmış ya da başka bir
uygulamanın anahtarıyla çakışmış olabilir. Doğrulanmadan `data-theme`'e
yazılsaydı CSS eşleşmeyen bir değer görür ve sayfa jetonsuz — yani biçimsiz —
boyanırdı.
*/
test("tanınmayan değer sisteme düşer", () => {
  kur("mor");
  assert.equal(readMode(), "system");

  kur("");
  assert.equal(readMode(), "system");
});

test("hiç kayıt yoksa sistem seçilir", () => {
  kur(undefined);
  assert.equal(readMode(), "system");
});

// Gizli sekmede `localStorage` erişimi hata veriyor. Tema yüzünden uygulamanın
// tamamının düşmesi kabul edilemez — okunamayan tercih varsayılana düşer.
test("depo erişilemezse uygulama düşmez", () => {
  kur("dark", false, true);
  assert.equal(readMode(), "system");
});

/* ----------------------------------------------------------- yazma ------ */

/*
YAZMA HEMEN UYGULANIR.

Yalnızca kaydedilseydi seçim bir sonraki sayfa yüklemesine kadar görünmezdi:
kullanıcı "Koyu"ya basar, hiçbir şey olmaz, düğmenin bozuk olduğunu sanardı.
*/
test("seçim hem kaydedilir hem anında uygulanır", () => {
  writeMode("dark");

  assert.equal(depo.veri.get(THEME_STORAGE_KEY), "dark");
  assert.equal(ekrandakiTema(), "dark");
});

// Kaydedilemese bile seçim bu oturumda geçerli olmalı: gizli sekmede tema
// değiştirmek hiçbir şey yapmasaydı düğme bozuk görünürdü.
test("kaydedilemeyen seçim yine de ekrana uygulanır", () => {
  kur(undefined, false, true);
  writeMode("dark");

  assert.equal(ekrandakiTema(), "dark", "depo patlasa da ekran değişmeli");
});

/* --------------------------------------------------------- uygulama ---- */

test("açık ve koyu doğrudan uygulanır, sistemi dinlemez", () => {
  kur(undefined, true); // işletim sistemi KOYU
  applyMode("light");
  assert.equal(ekrandakiTema(), "light", "açık seçildiyse OS koyu olsa da açık kalır");

  kur(undefined, false); // işletim sistemi AÇIK
  applyMode("dark");
  assert.equal(ekrandakiTema(), "dark");
});

/*
`system` ÜÇÜNCÜ BİR DURUM, "AÇIK"IN EŞ ANLAMLISI DEĞİL.

Aynı seçim, işletim sistemine göre iki farklı sonuç vermek zorunda. `light`'a
eşitlenseydi koyu tema kullanan kullanıcı varsayılan ayarla beyaz bir ekran
görürdü.
*/
test("system, işletim sistemini takip eder", () => {
  kur(undefined, true);
  applyMode("system");
  assert.equal(ekrandakiTema(), "dark");

  kur(undefined, false);
  applyMode("system");
  assert.equal(ekrandakiTema(), "light");
});

/* ------------------------------------------------- önyükleme betiği ---- */

/*
ÖNYÜKLEME BETİĞİ AYNI KARARI VERİR.

Betik bilerek bağımsız: `<head>` içinde, hiçbir modülü beklemeden çalışıyor —
beklemenin kendisi, önlemeye çalıştığı sıçramanın sebebi olurdu. Bedeli, aynı
kuralın İKİ AYRI uygulaması olması. İkisi ayrışırsa arıza tam da en görünür
yerde çıkar: sayfa önce bir temayla boyanır, React yüklenince diğerine atlar.

Bu test iki uygulamayı da çalıştırıp SONUÇLARINI karşılaştırıyor; betiğin
metnine bakmıyor.
*/
test("önyükleme betiği ile applyMode aynı sonucu verir", () => {
  const durumlar: Array<[string | undefined, boolean]> = [
    ["dark", false],
    ["dark", true],
    ["light", false],
    ["light", true],
    ["system", true],
    ["system", false],
    [undefined, true],
    [undefined, false],
    ["mor", true], // bozuk değer
  ];

  for (const [kayitli, sistemKoyu] of durumlar) {
    kur(kayitli, sistemKoyu);
    // Betik `setAttribute` kullanıyor, `applyMode` ise `dataset` — ikisini de
    // aynı yere yazdır ki karşılaştırma anlamlı olsun.
    const kok = (globalThis as unknown as {
      document: { documentElement: Record<string, unknown> };
    }).document.documentElement;
    kok.setAttribute = (ad: string, deger: string) => {
      (kok.dataset as Record<string, string>)[ad.replace("data-", "")] = deger;
    };

    new Function(themeBootstrapScript)();
    const betikSonucu = ekrandakiTema();

    kur(kayitli, sistemKoyu);
    applyMode(readMode());
    const modulSonucu = ekrandakiTema();

    assert.equal(
      betikSonucu,
      modulSonucu,
      `ayrıştılar — kayıtlı=${kayitli} sistemKoyu=${sistemKoyu}`,
    );
  }
});

// Betik kendi hatasını yutar: `<head>` içinde patlayan bir betik sayfanın geri
// kalanının çalışmasını engellerdi.
test("önyükleme betiği depo patlasa da hata fırlatmaz", () => {
  kur("dark", false, true);
  const kok = (globalThis as unknown as {
    document: { documentElement: Record<string, unknown> };
  }).document.documentElement;
  kok.setAttribute = () => {};

  assert.doesNotThrow(() => new Function(themeBootstrapScript)());
});
