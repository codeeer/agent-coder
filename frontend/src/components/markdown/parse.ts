/**
 * Markdown blok ayrıştırıcısı.
 *
 * Agent çıktısı Markdown'dır; ham basıldığında okunmaz. Buradaki ayrıştırma
 * SAF bir fonksiyondur — React'tan bağımsız olduğu için tek başına test edilir.
 *
 * Kapsam bilinçli olarak dardır: agent çıktısında gerçekten görülen sözdizimi.
 * Genişletme (mermaid, formül, dipnot) gerekirse o zaman eklenir.
 *
 * TASARIM KURALI: tanınmayan hiçbir satır ATILMAZ, paragraf olur. Bir çıktının
 * sessizce kaybolması, biçimlenmemesinden çok daha kötü olurdu.
 */

export type Align = "left" | "center" | "right";

/** Liste maddesi; depth yalnızca iki seviye taşır (0 ve 1). */
export interface ListItem {
  text: string;
  depth: number;
}

export type Block =
  | { kind: "heading"; level: number; text: string }
  | { kind: "paragraph"; text: string }
  | { kind: "code"; lang: string; code: string }
  | { kind: "list"; ordered: boolean; items: ListItem[] }
  | { kind: "table"; header: string[]; align: Align[]; rows: string[][] }
  | { kind: "quote"; text: string }
  | { kind: "hr" };

const HEADING = /^(#{1,6})\s+(.*)$/;
const FENCE = /^\s*(```|~~~)(.*)$/;
const HR = /^(-{3,}|\*{3,}|_{3,})$/;
const LIST = /^(\s*)([-*+]|\d+[.)])\s+(.*)$/;
const QUOTE = /^\s*>\s?(.*)$/;
// Ayıraç satırı: |---|:--:|---| — hizalama iki nokta ile verilir.
const TABLE_SEP = /^\|?\s*:?-{1,}:?\s*(\|\s*:?-{1,}:?\s*)*\|?$/;

/** Markdown metnini blok listesine çevirir. */
export function parseMarkdown(source: string): Block[] {
  const lines = source.replace(/\r\n?/g, "\n").split("\n");
  const blocks: Block[] = [];

  let i = 0;
  while (i < lines.length) {
    const line = lines[i] ?? "";

    if (line.trim() === "") {
      i++;
      continue;
    }

    const fence = FENCE.exec(line);
    if (fence) {
      const marker = fence[1] ?? "```";
      const body: string[] = [];
      i++;
      // Kapanmayan blok dosya sonunda biter: yarım kalmış çıktıda da içerik görünür.
      while (i < lines.length && !(lines[i] ?? "").trim().startsWith(marker)) {
        body.push(lines[i] ?? "");
        i++;
      }
      i++; // kapanış çiti
      blocks.push({
        kind: "code",
        lang: (fence[2] ?? "").trim(),
        code: body.join("\n"),
      });
      continue;
    }

    const heading = HEADING.exec(line);
    if (heading) {
      blocks.push({
        kind: "heading",
        level: (heading[1] ?? "#").length,
        // Kapanış diyezleri (### Başlık ###) atılır.
        text: (heading[2] ?? "").replace(/\s+#+\s*$/, "").trim(),
      });
      i++;
      continue;
    }

    if (HR.test(line.trim())) {
      blocks.push({ kind: "hr" });
      i++;
      continue;
    }

    // Tablo, ancak İKİNCİ satırı ayıraçsa tablodur; tek başına boru işaretli
    // bir satır sıradan metin olabilir.
    if (
      line.trim().includes("|") &&
      TABLE_SEP.test((lines[i + 1] ?? "").trim())
    ) {
      const header = splitRow(line);
      const align = parseAlign(lines[i + 1] ?? "", header.length);
      i += 2;

      const rows: string[][] = [];
      while (i < lines.length && (lines[i] ?? "").trim().includes("|")) {
        rows.push(padRow(splitRow(lines[i] ?? ""), header.length));
        i++;
      }
      blocks.push({ kind: "table", header, align, rows });
      continue;
    }

    if (LIST.test(line)) {
      const first = LIST.exec(line);
      const ordered = /\d/.test(first?.[2] ?? "");
      const items: ListItem[] = [];

      while (i < lines.length) {
        const m = LIST.exec(lines[i] ?? "");
        if (!m) break;
        // Numaralıdan maddeliye geçiş yeni bir listedir.
        if (/\d/.test(m[2] ?? "") !== ordered) break;
        items.push({
          text: (m[3] ?? "").trim(),
          depth: Math.min(Math.floor((m[1] ?? "").length / 2), 1),
        });
        i++;
      }
      blocks.push({ kind: "list", ordered, items });
      continue;
    }

    if (QUOTE.test(line)) {
      const body: string[] = [];
      while (i < lines.length) {
        const m = QUOTE.exec(lines[i] ?? "");
        if (!m) break;
        body.push(m[1] ?? "");
        i++;
      }
      blocks.push({ kind: "quote", text: body.join(" ").trim() });
      continue;
    }

    // Paragraf: bir sonraki boş satıra veya yeni blok başlangıcına kadar.
    const para: string[] = [];
    while (i < lines.length) {
      const l = lines[i] ?? "";
      if (l.trim() === "" || startsBlock(l, lines[i + 1] ?? "")) break;
      para.push(l.trim());
      i++;
    }
    if (para.length > 0) {
      blocks.push({ kind: "paragraph", text: para.join(" ") });
    }
  }

  return blocks;
}

/** Satır yeni bir blok başlatıyor mu? Paragrafın nerede biteceğini belirler. */
function startsBlock(line: string, next: string): boolean {
  return (
    HEADING.test(line) ||
    FENCE.test(line) ||
    HR.test(line.trim()) ||
    LIST.test(line) ||
    QUOTE.test(line) ||
    (line.includes("|") && TABLE_SEP.test(next.trim()))
  );
}

/** Tablo satırını hücrelere böler. */
function splitRow(line: string): string[] {
  return line
    .trim()
    .replace(/^\|/, "")
    .replace(/\|$/, "")
    .split("|")
    .map((c) => c.trim());
}

/** Ayıraç satırından kolon hizalamalarını okur. */
function parseAlign(line: string, count: number): Align[] {
  const cells = splitRow(line);
  const out: Align[] = [];
  for (let i = 0; i < count; i++) {
    const c = cells[i] ?? "";
    const left = c.startsWith(":");
    const right = c.endsWith(":");
    out.push(left && right ? "center" : right ? "right" : "left");
  }
  return out;
}

/** Eksik hücreleri tamamlar; bozuk tablo satırı render'ı düşürmemeli. */
function padRow(cells: string[], count: number): string[] {
  const out = cells.slice(0, count);
  while (out.length < count) out.push("");
  return out;
}

