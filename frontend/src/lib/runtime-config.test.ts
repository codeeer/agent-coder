import { test, beforeEach } from "node:test";
import assert from "node:assert/strict";

import {
  DEFAULT_APP_NAME,
  apiBase,
  apiConfigScript,
  serverApiUrl,
  serverAppName,
} from "./runtime-config.ts";

/*
 * Çalışma anı yapılandırması.
 *
 * Buradaki tek amaç TEK bir Docker imajının her kuruluma uyması. `NEXT_PUBLIC_*`
 * değişkenleri derleme anında bundle'a metin olarak gömülüyor; bu modül o
 * gömülmeyi atlatan yolu kuruyor.
 *
 * Arıza sınıfı sinsi: kod doğru görünür, testler geçer, imaj derlendiği adrese
 * mühürlenir ve hata yalnızca BAŞKA bir kurulumda ortaya çıkar — "sunucuya
 * ulaşılamıyor" diye.
 */

const g = globalThis as Record<string, unknown>;
const GLOBAL = "__AGENT_CODER_API_URL__";

beforeEach(() => {
  delete g.window;
  delete process.env.NEXT_PUBLIC_API_URL;
  delete process.env.API_URL;
  delete process.env.APP_NAME;
});

/* ------------------------------------------------------- tarayıcı ------- */

/*
SAYFAYA YAZILAN ADRES, DERLEME ANINDAKİNİ EZER.

Sıra ters olsaydı hazır imaj işe yaramazdı: imaja gömülü adres her zaman
kazanır, sunucuya kuran kullanıcı kendi adresini veremezdi. Bu modülün var
olma sebebi tam olarak bu öncelik.
*/
test("enjekte edilen adres, derleme anındaki değeri ezer", () => {
  process.env.NEXT_PUBLIC_API_URL = "http://derleme-aninda:8080";
  g.window = { [GLOBAL]: "http://kurulumda:9000" };

  assert.equal(apiBase(), "http://kurulumda:9000");
});

test("enjekte edilmemişse derleme anındaki değere düşer", () => {
  process.env.NEXT_PUBLIC_API_URL = "http://derleme-aninda:8080";
  g.window = {};

  assert.equal(apiBase(), "http://derleme-aninda:8080");
});

/*
BOŞ ENJEKSİYON YOK SAYILIR.

Compose değişkeni tanımlanmadığında container'a boş dize olarak ulaşıyor. Boş
değer kabul edilseydi istekler `/api/...` köküne, yani ARAYÜZÜN kendi
sunucusuna giderdi ve backend'e hiç ulaşılmazdı.
*/
test("boş enjeksiyon yok sayılır", () => {
  process.env.NEXT_PUBLIC_API_URL = "http://derleme-aninda:8080";
  g.window = { [GLOBAL]: "" };

  assert.equal(apiBase(), "http://derleme-aninda:8080");
});

test("hiçbir şey verilmezse yerel varsayılan kullanılır", () => {
  g.window = {};
  assert.equal(apiBase(), "http://localhost:8080");
});

// Sunucu tarafında `window` yok; modül bunu varsaymamalı.
test("window olmadan da çalışır", () => {
  assert.doesNotThrow(() => apiBase());
  assert.equal(apiBase(), "http://localhost:8080");
});

/* -------------------------------------------------- gömülen betik ------- */

/*
DEĞER KAÇIŞLANIR.

Adres ortam değişkeninden geliyor ve doğrudan bir `<script>` gövdesine
yazılıyor. Tırnak içerseydi betik sözdizimi bozulur ve sayfa hiç açılmazdı.
*/
test("tırnak içeren adres betiği bozmaz", () => {
  const betik = apiConfigScript('http://a"b/');
  assert.doesNotThrow(() => new Function(betik), "betik ayrıştırılamıyor");
});

test("betik değeri gerçekten global değişkene yazar", () => {
  const kap: Record<string, unknown> = {};
  new Function("window", apiConfigScript("http://x:1"))(kap);
  assert.equal(kap[GLOBAL], "http://x:1");
});

/*
`</script>` HTML AYRIŞTIRICISINI ERKEN KAPATIR.

`JSON.stringify` bunu KAÇIŞLAMAZ — eğik çizgi JSON'da özel karakter değil.
Tarayıcı ise betik gövdesini JavaScript olarak değil, ilk `</script>` dizisine
kadar HAM METİN olarak okur; yani dize içinde olması onu kurtarmaz.

Değer kurulumu yapanın kendi ortam değişkeninden geldiği için dışarıdan
sömürülebilir bir yol değil, ama kazayla girilirse sayfa sessizce bozulur.
Bu test o davranışı KAYIT ALTINA ALIYOR: bugünkü hâli budur ve
`apiConfigScript` değiştirilirken bilinerek değiştirilmelidir.
*/
test("`</script>` içeren adres betik gövdesinden kaçmaz — bugünkü sınır", () => {
  const betik = apiConfigScript("http://a</script><b>");
  assert.ok(
    betik.includes("</script>"),
    "dizi hâlâ ham geçiyor: HTML ayrıştırıcısı betiği burada kapatır",
  );
});

/* --------------------------------------------------- sunucu tarafı ------ */

/*
`API_URL` ÖNEKSİZ OLMAK ZORUNDA — ve önceliği o alır.

Next, `NEXT_PUBLIC_` ile başlayan her okumayı derleme anında metne çeviriyor,
sunucu bileşeninde bile. Öncelik ters olsaydı, çalışma anında okuduğumuzu
sanırken yine derlemedeki değeri gömerdik.
*/
test("sunucu adresinde API_URL öncelikli", () => {
  process.env.API_URL = "http://calisma-aninda:8080";
  process.env.NEXT_PUBLIC_API_URL = "http://derleme-aninda:8080";

  assert.equal(serverApiUrl(), "http://calisma-aninda:8080");
});

test("API_URL yoksa derleme anındaki değere düşer", () => {
  process.env.NEXT_PUBLIC_API_URL = "http://derleme-aninda:8080";
  assert.equal(serverApiUrl(), "http://derleme-aninda:8080");
});

test("hiçbiri yoksa yerel varsayılan", () => {
  assert.equal(serverApiUrl(), "http://localhost:8080");
});

/* ------------------------------------------------------- ürün adı ------- */

test("verilen ürün adı kullanılır", () => {
  process.env.APP_NAME = "Şirket Coder";
  assert.equal(serverAppName(), "Şirket Coder");
});

/*
BOŞ VE BOŞLUKLU AD VARSAYILANA DÜŞER.

Compose değişkeni tanımlanmadığında container'a `APP_NAME=""` olarak ulaşıyor.
Boş dize kabul edilseydi kenar çubuğundaki kelime markası ve sekme başlığı
boş kalırdı — ürünün adı hiçbir yerde görünmezdi.
*/
test("boş ve yalnızca boşluktan oluşan ad varsayılana düşer", () => {
  process.env.APP_NAME = "";
  assert.equal(serverAppName(), DEFAULT_APP_NAME);

  process.env.APP_NAME = "   ";
  assert.equal(serverAppName(), DEFAULT_APP_NAME);
});

test("ad kırpılır", () => {
  process.env.APP_NAME = "  Şirket Coder  ";
  assert.equal(serverAppName(), "Şirket Coder");
});
