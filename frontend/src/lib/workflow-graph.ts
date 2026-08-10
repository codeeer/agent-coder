import type { NodeKind, WorkflowGraph, WorkflowNodeConfig } from "@/lib/types";

/**
 * Graf ↔ tuval dönüşümü.
 *
 * React'tan ayrı duruyor: saf fonksiyon olduğu için test edilebilir ve ekran
 * kodu okunur kalıyor. Doğrusal akış varsayan eski adım listesi yardımcıları
 * tuvalle birlikte kaldırıldı (spec 008 K2) — grafa artık dallanma da girebilir.
 */

/**
 * Tetikleyici düğümün sabit kimliği.
 *
 * Adım kimlikleri bununla çakışamaz: çakışsaydı kullanıcı "başlangıç" adında
 * bir adım ekleyince akışın girişi belirsizleşirdi.
 */
export const TRIGGER_ID = "baslangic";

/** Boş bir tuval için başlangıç düğümü. */
export function newTriggerNode(): FlowNode {
  return {
    id: TRIGGER_ID,
    type: "trigger",
    position: { x: 40, y: 40 },
    data: { name: "", kind: "trigger.manual", config: {} },
  };
}

/** Türkçe harfleri katlayarak kimlik üretir. */
const FOLD: Record<string, string> = {
  ç: "c", ğ: "g", ı: "i", ö: "o", ş: "s", ü: "u",
  Ç: "c", Ğ: "g", İ: "i", Ö: "o", Ş: "s", Ü: "u",
};

/** Addan okunur, benzersiz bir düğüm kimliği türetir. */
export function makeStepId(name: string, taken: Set<string>): string {
  const base =
    name
      .split("")
      .map((c) => FOLD[c] ?? c)
      .join("")
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-+|-+$/g, "")
      .slice(0, 24) || "adim";

  let id = base;
  let n = 2;
  while (taken.has(id) || id === TRIGGER_ID) {
    id = `${base}-${n++}`;
  }
  return id;
}

/* ── Tuval dönüşümü (spec 008) ───────────────────────────────────────────── */

/** Tuvaldeki bir düğümün taşıdığı veri. */
export interface FlowNodeData {
  name: string;
  kind: NodeKind;
  config: WorkflowNodeConfig;
}

/** React Flow'un beklediği düğüm biçimi. */
export interface FlowNode {
  id: string;
  type: string;
  position: { x: number; y: number };
  data: FlowNodeData;
}

/** React Flow'un beklediği bağ biçimi. */
export interface FlowEdge {
  id: string;
  source: string;
  target: string;
}

/**
 * Düğüm türünden tuval bileşenine.
 *
 * Tetikleyici, agent ve "eylem" (PR, Jira) farklı çizilir — kullanıcı akışın
 * neresinin model çağırdığını, neresinin dış servise dokunduğunu bir bakışta
 * ayırt edebilmeli.
 */
export function flowTypeOf(kind: NodeKind): string {
  if (kind.startsWith("trigger.")) return "trigger";
  return kind === "agent" ? "agent" : "action";
}

/** Bağ kimliği kaynak-hedef çiftinden türetilir; ayrıca saklanmaz. */
export function edgeId(source: string, target: string): string {
  return `${source}->${target}`;
}

/** Kayıtlı grafı tuval düğümlerine çevirir. */
export function graphToFlow(graph: WorkflowGraph | null): {
  nodes: FlowNode[];
  edges: FlowEdge[];
} {
  if (!graph) return { nodes: [], edges: [] };

  return {
    nodes: graph.nodes.map((n) => ({
      id: n.id,
      type: flowTypeOf(n.kind),
      position: n.position ?? { x: 0, y: 0 },
      data: { name: n.name ?? "", kind: n.kind, config: n.config ?? {} },
    })),
    edges: graph.edges.map((e) => ({
      id: edgeId(e.from, e.to),
      source: e.from,
      target: e.to,
    })),
  };
}

/**
 * Tuvali kayıt edilecek grafa çevirir.
 *
 * Konumlar YUVARLANIR: React Flow ondalıklı piksel üretiyor ve kayıtta
 * `123.45678` gibi değerler tutmak, iki kaydetme arasında anlamsız farklar
 * doğurup "değişiklik var" uyarısını yanlış tetiklerdi.
 */
export function flowToGraph(nodes: FlowNode[], edges: FlowEdge[]): WorkflowGraph {
  return {
    nodes: nodes.map((n) => {
      const name = n.data.name.trim();
      return {
        id: n.id,
        kind: n.data.kind,
        // Boş ad ALANI HİÇ yazılmaz — backend de `omitempty` ile öyle üretiyor.
        // Yazılsaydı kaydedilen graf okunanla birebir aynı olmaz ve
        // "değişiklik var" uyarısı hiç değişiklik yokken tetiklenirdi.
        ...(name ? { name } : {}),
        config: n.data.config,
        position: { x: Math.round(n.position.x), y: Math.round(n.position.y) },
      };
    }),
    edges: edges.map((e) => ({ from: e.source, to: e.target })),
  };
}

/**
 * Bir düğümden ÖNCE çalışan tüm düğümler.
 *
 * Şablon referansı yalnızca ataya verilebilir: paralel bir kardeşin çıktısı bu
 * adım çalışırken hazır olmayabilir ve backend zaten reddeder. Panelde yalnızca
 * geçerli seçenekleri göstermek, kullanıcıyı hataya davet etmemek demek.
 */
export function ancestorsOf(edges: FlowEdge[], nodeId: string): Set<string> {
  const parents = new Map<string, string[]>();
  for (const e of edges) {
    parents.set(e.target, [...(parents.get(e.target) ?? []), e.source]);
  }

  const seen = new Set<string>();
  const queue = [...(parents.get(nodeId) ?? [])];
  while (queue.length > 0) {
    const cur = queue.shift()!;
    if (seen.has(cur)) continue;
    seen.add(cur);
    queue.push(...(parents.get(cur) ?? []));
  }
  return seen;
}
