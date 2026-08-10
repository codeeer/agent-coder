/**
 * Arayüz ekran görüntüsü — görsel doğrulama için.
 *
 * NEDEN VAR: bu projede iki hata yalnızca ekrana bakılarak yakalandı —
 * kilitlenen bir sayfa (spec 005) ve düğme yazılarının yanlış renkte çıkması
 * (spec 006). İkisi de tip kontrolünden, linter'dan ve birim testlerden temiz
 * geçmişti. "Derleniyor" ile "doğru görünüyor" aynı şey değildir.
 *
 * Tema `colorScheme` VE `localStorage` ile birlikte zorlanır: böylece geliştirme
 * makinesi koyu temada olsa bile açık tema görülebilir.
 *
 * Kurulum (bir kez):
 *   npm i -D playwright && npx playwright install chromium
 *
 * Kullanım:
 *   node scripts/screenshot.mjs /reports light rapor.png
 *   node scripts/screenshot.mjs /agents  dark  agent.png 1440 900
 *
 * Ekran görüntüsü yerine hesaplanmış rengi okumak için --probe:
 *   node scripts/screenshot.mjs /agents light --probe
 */

import { chromium } from "playwright";

const [
  ,
  ,
  path = "/",
  theme = "light",
  out = "shot.png",
  width = "1440",
  height = "1000",
] = process.argv;

const BASE = process.env.APP_URL ?? "http://localhost:3002";
const probe = out === "--probe";

const browser = await chromium.launch();
try {
  const ctx = await browser.newContext({
    viewport: { width: +width, height: +height },
    colorScheme: theme === "dark" ? "dark" : "light",
  });
  // Tema seçimi localStorage'da durur; sayfa açılmadan önce yazılır.
  await ctx.addInitScript(
    ([t]) => localStorage.setItem("agent-coder-theme", t),
    [theme],
  );

  const page = await ctx.newPage();
  page.on("pageerror", (e) => console.log("SAYFA HATASI:", e.message));
  page.on(
    "console",
    (m) => m.type() === "error" && console.log("KONSOL:", m.text()),
  );

  await page.goto(BASE + path, { waitUntil: "networkidle", timeout: 30_000 });
  await page.waitForTimeout(600);

  console.log("data-theme =", await page.getAttribute("html", "data-theme"));

  if (probe) {
    // Renk şikâyetlerinde ekran görüntüsünden daha kesin: hesaplanmış değer.
    const rows = await page.evaluate(() =>
      [...document.querySelectorAll("button, a")]
        .map((el) => {
          const t = (el.textContent || "").trim().slice(0, 18);
          if (!t) return null;
          const s = getComputedStyle(el);
          return { t, bg: s.backgroundColor, fg: s.color, bd: s.borderColor };
        })
        .filter(Boolean),
    );
    const seen = new Map();
    for (const r of rows) if (!seen.has(r.t)) seen.set(r.t, r);
    for (const r of seen.values()) {
      console.log(
        `${r.t.padEnd(20)} zemin=${r.bg.padEnd(22)} metin=${r.fg.padEnd(22)} kenar=${r.bd}`,
      );
    }
  } else {
    await page.screenshot({ path: out, fullPage: true });
    console.log("yazıldı:", out);
  }
} finally {
  // Her yolda kapanır — kaçak tarayıcı süreci bırakmaz.
  await browser.close();
}
