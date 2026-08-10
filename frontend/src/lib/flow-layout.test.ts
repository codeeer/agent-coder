/**
 * Tuval yerleşimi ve döngü tespiti testleri.
 *
 * Tuval açılmadan çalışırlar: yerleşim ve döngü mantığı React'tan bağımsız
 * tutuldu, çünkü bileşene gömülen mantığın hatası ancak tarayıcıda görülür
 * (spec 005, Ölçüm 1).
 */

import assert from "node:assert/strict";
import { test } from "node:test";
import {
  autoLayout,
  levelsOf,
  needsLayout,
  nextPosition,
  wouldCreateCycle,
} from "./flow-layout.ts";
import type { FlowEdge, FlowNode } from "./workflow-graph.ts";

function node(id: string, x = 0, y = 0): FlowNode {
  return {
    id,
    type: id === "t" ? "trigger" : "agent",
    position: { x, y },
    data: { name: id, kind: id === "t" ? "trigger.manual" : "agent", config: {} },
  };
}

function edge(source: string, target: string): FlowEdge {
  return { id: `${source}->${target}`, source, target };
}

/*
 *      t ──┬── a ──┐
 *          └── b ──┴── c
 */
const nodes = [node("t"), node("a"), node("b"), node("c")];
const edges = [edge("t", "a"), edge("t", "b"), edge("a", "c"), edge("b", "c")];

test("seviyeler motorun hesabıyla aynı: paralel düğümler aynı seviyede", () => {
  assert.deepEqual(levelsOf(nodes, edges), [["t"], ["a", "b"], ["c"]]);
});

test("döngüde yerleşim çökmez, kalanlar son seviyeye konur", () => {
  const dongulu = [node("a"), node("b")];
  const halka = [edge("a", "b"), edge("b", "a")];
  const levels = levelsOf(dongulu, halka);
  assert.equal(levels.length, 1);
  assert.deepEqual(levels[0]?.sort(), ["a", "b"]);
});

test("konumsuz akış üst üste binmeden yerleşir", () => {
  const placed = autoLayout(nodes, edges);
  const seen = new Set(placed.map((n) => `${n.position.x},${n.position.y}`));
  assert.equal(seen.size, nodes.length, "her düğüm ayrı bir noktada olmalı");

  const byID = new Map(placed.map((n) => [n.id, n.position]));
  // Sütun = seviye: sonraki seviye daha sağda.
  assert.ok(byID.get("a")!.x > byID.get("t")!.x);
  assert.ok(byID.get("c")!.x > byID.get("a")!.x);
  // Paralel kardeşler aynı sütunda, farklı satırda.
  assert.equal(byID.get("a")!.x, byID.get("b")!.x);
  assert.notEqual(byID.get("a")!.y, byID.get("b")!.y);
  // Hiçbir düğüm eksi koordinata düşmemeli — tuvalin dışında kalırdı.
  for (const n of placed) {
    assert.ok(n.position.x >= 0 && n.position.y >= 0, `${n.id} eksi koordinatta`);
  }
});

test("kullanıcının taşıdığı düğüm yerinde kalır", () => {
  const karisik = [node("t", 10, 20), node("a"), node("b")];
  const zincir = [edge("t", "a"), edge("a", "b")];

  const placed = autoLayout(karisik, zincir);
  const t = placed.find((n) => n.id === "t")!;
  assert.deepEqual(t.position, { x: 10, y: 20 }, "elle konulan düğüm oynatılmamalı");

  const a = placed.find((n) => n.id === "a")!;
  assert.notDeepEqual(a.position, { x: 0, y: 0 }, "konumsuz düğüm yerleştirilmeli");
});

test("needsLayout yalnızca hepsi sıfırdayken doğru", () => {
  assert.equal(needsLayout(nodes), true);
  assert.equal(needsLayout([node("t", 5, 5), node("a")]), false);
  assert.equal(needsLayout([]), false);
});

test("döngü yaratacak bağ yakalanır", () => {
  const zincir = [edge("a", "b"), edge("b", "c")];

  assert.equal(wouldCreateCycle(zincir, "c", "a"), true, "c→a çemberi kapatır");
  assert.equal(wouldCreateCycle(zincir, "a", "a"), true, "kendine bağ");
  assert.equal(wouldCreateCycle(zincir, "b", "a"), true, "kısa çember");

  assert.equal(wouldCreateCycle(zincir, "a", "c"), false, "kısayol döngü değildir");
  assert.equal(wouldCreateCycle(zincir, "c", "d"), false, "yeni dal");
});

test("yeni düğüm en sağdakinin sağına konur", () => {
  const p = nextPosition([node("a", 100, 50), node("b", 300, 50)]);
  assert.ok(p.x > 300, "yeni düğüm sağda olmalı");
  assert.equal(nextPosition([]).x, 40, "boş tuvalde başlangıç noktası");
});
