/**
 * Tema denetimi — açık ve koyu temayı ÖLÇEREK karşılaştırır.
 *
 * NEDEN VAR: bu projede renk hataları iki kez yalnızca ekrana bakılarak
 * yakalandı (spec 006). Ama göz, 4.4:1 ile 4.6:1 arasını ayırt edemez ve iki
 * temayı yan yana tutamaz. Bu betik hesaplanmış renkleri okur, WCAG kontrast
 * oranını hesaplar ve iki temanın sonucunu karşılaştırır.
 *
 * Ölçülenler:
 *   - Metin kontrastı (AA: normal 4.5, iri 3.0)
 *   - Metin dışı kontrast (WCAG 1.4.11): kenarlık ve tutamaklar 3.0
 *   - Düğme durumları: normal / hover / focus / disabled
 *   - Tema eşliği: bir bileşen bir temada geçip diğerinde kalıyor mu
 *
 * Kullanım:
 *   node scripts/theme-audit.mjs            # tüm sayfalar, iki tema
 *   node scripts/theme-audit.mjs /settings  # tek sayfa
 */

import { chromium } from "playwright";

const BASE = process.env.UI_URL ?? "http://localhost:3002";
const PAGES = process.argv.slice(2).length
  ? process.argv.slice(2)
  : ["/", "/workflows", "/projects", "/agents", "/runs", "/reports", "/models", "/settings"];

/* Sayfa içinde çalışan ölçüm. Tarayıcı bağlamına serileştirildiği için
   bağımlılıksız ve tek parça olmak zorunda. */
function probe() {
  const srgb = (c) => {
    const v = c / 255;
    return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
  };
  const lum = ([r, g, b]) => 0.2126 * srgb(r) + 0.7152 * srgb(g) + 0.0722 * srgb(b);
  const ratio = (a, b) => {
    const [x, y] = [lum(a), lum(b)].sort((m, n) => n - m);
    return (x + 0.05) / (y + 0.05);
  };
  /*
   * Rengi TARAYICIYA çözdürür.
   *
   * Düzenli ifadeyle `rgb(...)` ayrıştırmak yetmiyor: Tailwind v4, `/35` gibi
   * saydamlık ekini `oklab(... / 0.35)` olarak üretiyor. Elle yazılmış
   * ayrıştırıcı bunu tanımayıp `null` dönüyordu ve ölçüm o elemanı SESSİZCE
   * atlıyordu — yani rozet kenarlarının ve "Sil" düğmesinin hiçbiri hiç
   * ölçülmemişti. Tuval, tarayıcının kendi renk motorunu kullanır; hangi
   * sözdizimi gelirse gelsin doğru cevabı verir.
   */
  const cvs = document.createElement("canvas");
  cvs.width = cvs.height = 1;
  const cx = cvs.getContext("2d", { willReadFrequently: true });
  const parse = (str) => {
    if (!str) return null;
    cx.globalCompositeOperation = "copy";
    cx.fillStyle = "rgba(0,0,0,0)";
    cx.fillRect(0, 0, 1, 1);
    cx.fillStyle = str;
    cx.fillRect(0, 0, 1, 1);
    const d = cx.getImageData(0, 0, 1, 1).data;
    return { rgb: [d[0], d[1], d[2]], a: d[3] / 255 };
  };
  /** Saydam rengi arkasındakiyle harmanlar. */
  const over = (fg, bg) => fg.rgb.map((c, i) => Math.round(c * fg.a + bg[i] * (1 - fg.a)));

  /** Elemanın GERÇEK arka planı: saydam geçerek yukarı yürür. */
  const bgOf = (el) => {
    let cur = el;
    let acc = null;
    while (cur) {
      const c = parse(getComputedStyle(cur).backgroundColor);
      if (c && c.a > 0) {
        acc = acc === null ? c : { rgb: over(acc, c.rgb), a: 1 };
        if (acc.a === 1 && c.a === 1) return acc.rgb;
      }
      cur = cur.parentElement;
    }
    return acc ? acc.rgb : [255, 255, 255];
  };

  const visible = (el) => {
    const r = el.getBoundingClientRect();
    const s = getComputedStyle(el);
    return r.width > 0 && r.height > 0 && s.visibility !== "hidden" && s.display !== "none";
  };

  /** Elemanın kendi metni var mı (çocuklarınki değil). */
  const ownText = (el) =>
    [...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim().length > 1);

  /** Bileşeni tanımlayan kısa imza — aynı bileşenin tekrarları tek satır olsun. */
  const sig = (el) => {
    const cls = (el.getAttribute("class") || "")
      .split(/\s+/)
      .filter((c) => /^(bg|text|border)-/.test(c))
      .sort()
      .join(" ");
    return `${el.tagName.toLowerCase()}${cls ? " ." + cls : ""}`;
  };

  const out = [];
  const seen = new Set();

  const record = (el, kind, fgStr, need, label) => {
    const bg = bgOf(el);
    const fgP = parse(fgStr);
    if (!fgP) return;
    const fg = fgP.a < 1 ? over(fgP, bg) : fgP.rgb;
    const r = ratio(fg, bg);
    const key = `${kind}|${sig(el)}|${label}`;
    if (seen.has(key)) return;
    seen.add(key);
    out.push({
      kind,
      label,
      sig: sig(el),
      text: (el.textContent || "").trim().slice(0, 28),
      fg: `rgb(${fg.join(",")})`,
      bg: `rgb(${bg.join(",")})`,
      ratio: Math.round(r * 100) / 100,
      need,
      pass: r >= need,
    });
  };

  // 1) Metin kontrastı
  document.querySelectorAll("body *").forEach((el) => {
    if (!visible(el) || !ownText(el)) return;
    const s = getComputedStyle(el);
    const px = parseFloat(s.fontSize);
    const bold = parseInt(s.fontWeight, 10) >= 700;
    const large = px >= 24 || (bold && px >= 18.66);
    record(el, "metin", s.color, large ? 3 : 4.5, `${Math.round(px)}px`);
  });

  // 2) Metin dışı: kenarlıklar (WCAG 1.4.11)
  document.querySelectorAll("button, input, select, textarea, [role=button]").forEach((el) => {
    if (!visible(el)) return;
    const s = getComputedStyle(el);
    if (parseFloat(s.borderTopWidth) < 0.5) return;
    // Saydam kenar bir sınır değildir: bu tür düğmeler DOLGUSUYLA tanınır
    // (birincil düğme gibi). Ölçmek yanlış alarm üretiyordu.
    const b = parse(s.borderTopColor);
    if (!b || b.a === 0) return;
    if (b.a === 1) {
      const own = parse(s.backgroundColor);
      if (own && own.a === 1 && own.rgb.join() === b.rgb.join()) return;
    }
    record(el, "kenarlık", s.borderTopColor, 3, "border");
  });

  return out;
}

async function run(page, path, theme) {
  await page.goto(BASE + path, { waitUntil: "networkidle" });
  await page.waitForTimeout(1100);
  const rows = await page.evaluate(probe);
  return rows.map((r) => ({ ...r, path, theme }));
}

/** Düğme durumları: hover / focus / disabled iki temada da ölçülür. */
async function buttonStates(page, path, theme) {
  await page.goto(BASE + path, { waitUntil: "networkidle" });
  await page.waitForTimeout(900);

  const buttons = page.locator("button:visible");
  const n = Math.min(await buttons.count(), 12);
  const rows = [];

  for (let i = 0; i < n; i++) {
    const b = buttons.nth(i);
    const info = await b.evaluate((el) => ({
      text: (el.textContent || "").trim().slice(0, 24),
      disabled: el.disabled,
      cls: (el.getAttribute("class") || "")
        .split(/\s+/)
        .filter((c) => /^(bg|text|border)-/.test(c))
        .join(" "),
    }));

    const read = async () =>
      b.evaluate((el) => {
        const s = getComputedStyle(el);
        return {
          bg: s.backgroundColor,
          color: s.color,
          border: s.borderTopColor,
          opacity: s.opacity,
          outline: `${s.outlineWidth} ${s.outlineStyle}`,
        };
      });

    const rest = await read();
    let hover = null;
    let focus = null;
    if (!info.disabled) {
      await b.hover().catch(() => {});
      await page.waitForTimeout(120);
      hover = await read();
      await b.focus().catch(() => {});
      await page.waitForTimeout(120);
      focus = await read();
      await page.mouse.move(0, 0);
    }
    rows.push({ path, theme, ...info, rest, hover, focus });
  }
  return rows;
}

const browser = await chromium.launch();
const all = [];
const btns = [];

for (const theme of ["light", "dark"]) {
  const page = await browser.newPage({
    viewport: { width: 1440, height: 950 },
    colorScheme: theme,
  });
  await page.addInitScript((t) => localStorage.setItem("agent-coder-theme", t), theme);
  for (const path of PAGES) {
    all.push(...(await run(page, path, theme)));
    btns.push(...(await buttonStates(page, path, theme)));
  }
  await page.close();
}
await browser.close();

/* ── Rapor ────────────────────────────────────────────────────────────────── */

const fails = all.filter((r) => !r.pass);
console.log(`\nÖLÇÜM: ${all.length} kontrol, ${fails.length} kalan\n`);

if (fails.length) {
  console.log("KONTRAST KALANLARI");
  const groups = new Map();
  for (const f of fails) {
    const k = `${f.theme}|${f.kind}|${f.sig}|${f.label}`;
    if (!groups.has(k)) groups.set(k, { ...f, paths: new Set() });
    groups.get(k).paths.add(f.path);
  }
  for (const g of [...groups.values()].sort((a, b) => a.ratio - b.ratio)) {
    console.log(
      `  [${g.theme.padEnd(5)}] ${String(g.ratio).padStart(5)}:1 (gerek ${g.need}) ` +
        `${g.kind}/${g.label}  ${g.fg} üzerinde ${g.bg}\n` +
        `           ${g.sig}\n` +
        `           "${g.text}"  →  ${[...g.paths].join(" ")}`,
    );
  }
}

/* Tema eşliği: aynı imza bir temada geçip diğerinde kalıyor mu. */
console.log("\nTEMA EŞLİĞİ (bir temada geçip diğerinde kalanlar)");
const byKey = new Map();
for (const r of all) {
  const k = `${r.kind}|${r.sig}|${r.label}`;
  if (!byKey.has(k)) byKey.set(k, {});
  const slot = byKey.get(k);
  // En kötü örneği tut: bir bileşen bir yerde kalıyorsa sorun vardır.
  if (!slot[r.theme] || r.ratio < slot[r.theme].ratio) slot[r.theme] = r;
}
let parity = 0;
for (const [k, v] of byKey) {
  if (!v.light || !v.dark) continue;
  if (v.light.pass !== v.dark.pass) {
    parity++;
    const bad = v.light.pass ? v.dark : v.light;
    console.log(
      `  ${bad.theme.toUpperCase()} kalıyor: ${bad.ratio}:1 vs diğer tema ` +
        `${(v.light.pass ? v.light : v.dark).ratio}:1 — ${k}\n` +
        `      "${bad.text}" (${bad.path})`,
    );
  }
}
if (!parity) console.log("  yok — iki tema aynı sonucu veriyor");

/* Düğme durumları. */
console.log("\nDÜĞME DURUMLARI");
const bkey = (b) => `${b.cls || "(sınıfsız)"}|${b.disabled ? "disabled" : "normal"}`;
const bmap = new Map();
for (const b of btns) {
  const k = bkey(b);
  if (!bmap.has(k)) bmap.set(k, {});
  bmap.get(k)[b.theme] ??= b;
}
for (const [k, v] of bmap) {
  const l = v.light;
  const d = v.dark;
  if (!l || !d) continue;
  const hoverChanges = (b) =>
    b.hover && (b.hover.bg !== b.rest.bg || b.hover.border !== b.rest.border || b.hover.color !== b.rest.color);
  const focusRing = (b) => b.focus && b.focus.outline !== "0px none";
  console.log(`  ${k.split("|")[0].slice(0, 70)} [${k.split("|")[1]}]`);
  console.log(`     light: bg ${l.rest.bg} · yazı ${l.rest.color} · opacity ${l.rest.opacity}`);
  console.log(`     dark : bg ${d.rest.bg} · yazı ${d.rest.color} · opacity ${d.rest.opacity}`);
  if (!l.disabled) {
    console.log(
      `     hover light:${hoverChanges(l) ? "var" : "YOK"} dark:${hoverChanges(d) ? "var" : "YOK"} · ` +
        `focus halkası light:${focusRing(l) ? "var" : "YOK"} dark:${focusRing(d) ? "var" : "YOK"}`,
    );
  }
}
