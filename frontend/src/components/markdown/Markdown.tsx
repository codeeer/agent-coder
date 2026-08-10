import { Fragment, useMemo, type ReactNode } from "react";
import { tokenizeInline, type InlineToken } from "./inline";
import { parseMarkdown, type Block } from "./parse";

/**
 * Markdown gösterimi.
 *
 * GÜVENLİK: render YALNIZCA React elemanı üretir; `dangerouslySetInnerHTML`
 * kullanılmaz. Agent çıktısı güvenilmeyen metindir — içindeki `<script>` React
 * tarafından kaçırılır ve ekranda metin olarak görünür. Bu yüzden ayrıca bir
 * arındırıcıya (sanitizer) gerek yoktur.
 *
 * Bu dosya YALNIZCA çizim yapar. Ayrıştırma `parse.ts` ve `inline.ts` içinde,
 * React'tan bağımsız ve test edilebilir halde durur — çizim koduna gömülü bir
 * ayrıştırıcı test edilemediği için hatası ancak tarayıcıda görülüyordu.
 */
export function Markdown({ source }: { source: string }) {
  const blocks = useMemo(() => parseMarkdown(source), [source]);

  return (
    <div className="space-y-3 text-[13px] leading-relaxed">
      {blocks.map((b, i) => (
        <BlockView key={i} block={b} />
      ))}
    </div>
  );
}

function BlockView({ block }: { block: Block }) {
  switch (block.kind) {
    case "heading":
      return <Heading level={block.level} text={block.text} />;

    case "paragraph":
      return <p>{inline(block.text)}</p>;

    case "code":
      return (
        // Uzun satır sayfayı değil, kendi kabını kaydırır.
        <pre className="overflow-x-auto rounded-lg border border-line bg-raised p-3">
          <code className="font-mono text-[12px] leading-relaxed">
            {block.code}
          </code>
        </pre>
      );

    case "list":
      return <List block={block} />;

    case "table":
      return <Table block={block} />;

    case "quote":
      return (
        <blockquote className="border-l-2 border-line-strong pl-3 text-ink-2 italic">
          {inline(block.text)}
        </blockquote>
      );

    case "hr":
      return <hr className="border-line" />;
  }
}

function Heading({ level, text }: { level: number; text: string }) {
  // Boyut farkı hiyerarşiyi taşır; çıktı içindeki en büyük başlık bile sayfa
  // başlığından küçüktür, onunla yarışmasın diye.
  const size =
    level <= 1
      ? "text-[16px] font-semibold"
      : level === 2
        ? "text-[15px] font-semibold"
        : level === 3
          ? "text-[13px] font-semibold"
          : "text-[13px] font-medium text-ink-2";

  const Tag = `h${Math.min(level + 1, 6)}` as "h2";

  return (
    <Tag className={`mt-4 first:mt-0 tracking-[-0.01em] ${size}`}>
      {inline(text)}
    </Tag>
  );
}

function List({ block }: { block: Extract<Block, { kind: "list" }> }) {
  const Tag = block.ordered ? "ol" : "ul";

  return (
    <Tag
      className={`space-y-1 pl-5 ${block.ordered ? "list-decimal" : "list-disc"}`}
    >
      {block.items.map((item, i) => (
        <li key={i} className={item.depth > 0 ? "ml-4" : ""}>
          {inline(item.text)}
        </li>
      ))}
    </Tag>
  );
}

function Table({ block }: { block: Extract<Block, { kind: "table" }> }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-line">
      <table className="w-full text-[12px]">
        <thead>
          <tr className="border-b border-line bg-raised text-left text-[11px] tracking-wide text-ink-3 uppercase">
            {block.header.map((cell, i) => (
              <th
                key={i}
                className={`px-3 py-2 font-medium ${alignClass(block.align[i])}`}
              >
                {inline(cell)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {block.rows.map((row, r) => (
            <tr key={r} className="border-b border-line last:border-0">
              {row.map((cell, c) => (
                <td
                  key={c}
                  className={`px-3 py-1.5 align-top ${alignClass(block.align[c])}`}
                >
                  {inline(cell)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function alignClass(align: string | undefined): string {
  if (align === "center") return "text-center";
  if (align === "right") return "text-right";
  return "text-left";
}

/** Metni ayrıştırır ve parçaları çizer. */
function inline(text: string): ReactNode {
  return renderTokens(tokenizeInline(text));
}

function renderTokens(tokens: InlineToken[]): ReactNode {
  return tokens.map((t, i) => {
    switch (t.kind) {
      case "text":
        return <Fragment key={i}>{t.text}</Fragment>;

      case "code":
        return (
          <code
            key={i}
            className="rounded border border-line bg-raised px-1 py-0.5 font-mono text-[12px]"
          >
            {t.text}
          </code>
        );

      case "strong":
        return (
          <strong key={i} className="font-semibold">
            {renderTokens(t.children)}
          </strong>
        );

      case "em":
        return (
          <em key={i} className="italic">
            {renderTokens(t.children)}
          </em>
        );

      case "strike":
        return (
          <s key={i} className="text-ink-3">
            {renderTokens(t.children)}
          </s>
        );

      case "link":
        // Güvenli olmayan şema bağlantıya çevrilmez; yazıldığı gibi metin
        // olarak görünür ki kullanıcı agent'ın ne ürettiğini görsün.
        return t.href ? (
          <a
            key={i}
            href={t.href}
            target="_blank"
            rel="noopener noreferrer"
            className="text-accent underline underline-offset-2"
          >
            {renderTokens(t.children)}
          </a>
        ) : (
          <Fragment key={i}>{t.raw}</Fragment>
        );
    }
  });
}
