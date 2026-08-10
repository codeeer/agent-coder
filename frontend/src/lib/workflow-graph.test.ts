/** Graf ↔ tuval dönüşümü testleri. */

import assert from "node:assert/strict";
import { test } from "node:test";
import {
  ancestorsOf,
  flowToGraph,
  graphToFlow,
  makeStepId,
} from "./workflow-graph.ts";
import type { WorkflowGraph } from "./types.ts";

const graph: WorkflowGraph = {
  nodes: [
    { id: "t", kind: "trigger.manual", config: {}, position: { x: 40, y: 40 } },
    {
      id: "analiz",
      kind: "agent",
      name: "Analiz",
      config: { agentId: "a1", model: "m", prompt: "{{ input }}" },
      position: { x: 320, y: 40 },
    },
  ],
  edges: [{ from: "t", to: "analiz" }],
};

test("gidiş-dönüş dönüşüm grafı bozmaz", () => {
  const flow = graphToFlow(graph);
  const back = flowToGraph(flow.nodes, flow.edges);
  assert.deepEqual(back, graph);
});

test("düğüm türü tuval bileşenini seçer", () => {
  const flow = graphToFlow(graph);
  assert.equal(flow.nodes[0]?.type, "trigger");
  assert.equal(flow.nodes[1]?.type, "agent");
  assert.equal(flow.edges[0]?.id, "t->analiz");
});

test("boş graf boş tuval verir", () => {
  assert.deepEqual(graphToFlow(null), { nodes: [], edges: [] });
});

// React Flow ondalıklı piksel üretir; kayıtta tutulursa iki kaydetme arasında
// anlamsız fark doğar ve "değişiklik var" uyarısı yanlış tetiklenir.
test("konumlar yuvarlanır", () => {
  const back = flowToGraph(
    [
      {
        id: "a",
        type: "agent",
        position: { x: 123.456, y: 78.9 },
        data: { name: " Ad ", kind: "agent", config: {} },
      },
    ],
    [],
  );
  assert.deepEqual(back.nodes[0]?.position, { x: 123, y: 79 });
  assert.equal(back.nodes[0]?.name, "Ad", "addaki boşluklar kırpılır");
});

/*
 *   t ──┬── a ──┐
 *       └── b ──┴── c
 */
test("atalar yalnızca ÖNCE çalışanları içerir", () => {
  const edges = [
    { id: "1", source: "t", target: "a" },
    { id: "2", source: "t", target: "b" },
    { id: "3", source: "a", target: "c" },
    { id: "4", source: "b", target: "c" },
  ];

  assert.deepEqual([...ancestorsOf(edges, "c")].sort(), ["a", "b", "t"]);
  // a ile b paralel kardeş: biri diğerinin atası DEĞİL.
  assert.deepEqual([...ancestorsOf(edges, "a")], ["t"]);
  assert.deepEqual([...ancestorsOf(edges, "t")], []);
});

test("kimlik addan türetilir ve çakışmaz", () => {
  const taken = new Set<string>();
  assert.equal(makeStepId("Şirket İncelemesi", taken), "sirket-incelemesi");
  taken.add("analiz");
  assert.equal(makeStepId("Analiz", taken), "analiz-2");
  assert.equal(makeStepId("", new Set()), "adim");
});
