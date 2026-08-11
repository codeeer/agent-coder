"use client";

import {
  Background,
  BackgroundVariant,
  Controls,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  type Connection,
  type Edge,
  type EdgeChange,
  type Node,
  type NodeChange,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { useCallback, useEffect, useMemo } from "react";
import { wouldCreateCycle } from "@/lib/flow-layout";
import { edgeId, type FlowEdge, type FlowNode } from "@/lib/workflow-graph";
import { nodeTypes } from "@/components/flow/nodes";

/**
 * Sığdırmada izin verilen en yüksek yakınlaştırma.
 *
 * 1 idi, yani küçük bir akış hiçbir zaman büyütülmüyordu. Pratikte çoğu
 * akışta bağlayıcı kısıt YİNE genişliktir — soldan sağa dizilen bir zincir
 * kabına yatayda sığdığı kadar büyür ve bu sınıra hiç dayanmaz. Fark tek
 * düğümlü ya da çok kısa akışlarda görülür; orada da üst sınır gerekli,
 * yoksa tek bir düğüm ekranı kaplayacak kadar şişerdi.
 */
const FIT_MAX_ZOOM = 1.35;

/**
 * Sinyal değişince görünümü sığdırır.
 *
 * `useReactFlow` yalnızca sağlayıcının İÇİNDE çalışır; bu yüzden ayrı bir
 * bileşen olarak duruyor.
 */
function FitOnSignal({ signal }: { signal?: number }) {
  const flow = useReactFlow();

  useEffect(() => {
    if (signal === undefined) return;
    // Düğüm DOM'a girip ölçülene kadar bekle; hemen çağrılırsa boyutu
    // bilinmeyen düğüm hesaba katılmaz.
    const t = setTimeout(
      () => void flow.fitView({ padding: 0.2, maxZoom: FIT_MAX_ZOOM, duration: 200 }),
      60,
    );
    return () => clearTimeout(t);
  }, [signal, flow]);

  return null;
}

/*
 * React Flow, düğüm `data` alanının `Record<string, unknown>` olmasını şart
 * koşuyor. Kendi tiplerimize indeks imzası eklemek her alan erişimini
 * gevşetirdi; bunun yerine dönüşüm YALNIZCA kütüphane sınırında yapılıyor.
 */
/**
 * Değişiklik kayıtlı veriyi etkiliyor mu?
 *
 * `dimensions` (ölçüm) ve `select` yalnızca görüntüyle ilgili; ikisi de sayfa
 * açılırken kendiliğinden geliyor.
 */
function alters(change: NodeChange | EdgeChange): boolean {
  return change.type !== "dimensions" && change.type !== "select";
}

const toRFNodes = (ns: FlowNode[]) => ns as unknown as Node[];
const fromRFNodes = (ns: Node[]) => ns as unknown as FlowNode[];
const toRFEdges = (es: FlowEdge[]) => es as unknown as Edge[];
const fromRFEdges = (es: Edge[]) => es as unknown as FlowEdge[];

/**
 * Akış tuvali.
 *
 * İki modda çalışır — düzenleme ve izleme (spec 008 K3). İzleme için ayrı bir
 * bileşen yazmak, aynı çizimin iki yerde tutulması ve er geç ayrışması demekti.
 */
export function FlowCanvas({
  nodes,
  edges,
  onNodesChange,
  onEdgesChange,
  onSelect,
  selectedId,
  readOnly = false,
  height = 520,
  fitSignal,
}: {
  nodes: FlowNode[];
  edges: FlowEdge[];
  /**
   * `meaningful`, değişikliğin KAYITLI VERİYİ etkileyip etkilemediğini söyler.
   * React Flow yüklenirken ölçüm ve seçim olayları gönderiyor; onları da
   * değişiklik saymak "kaydedilmemiş değişiklik var" uyarısını hiçbir şey
   * yapılmadan tetikler ve uyarı anlamını yitirir.
   */
  onNodesChange?: (nodes: FlowNode[], meaningful: boolean) => void;
  onEdgesChange?: (edges: FlowEdge[], meaningful: boolean) => void;
  onSelect?: (id: string | null) => void;
  selectedId?: string | null;
  readOnly?: boolean;
  height?: number;
  /**
   * Değeri değişince görünüm yeniden sığdırılır.
   *
   * Yeni eklenen adım görünür alanın dışına düşebiliyor: kullanıcı "Adım ekle"
   * diyor, ekranda hiçbir şey olmuyor sanıyor. Eklemeden sonra tuvali
   * sığdırmak bunu çözer.
   */
  fitSignal?: number;
}) {
  // Seçim tuvalin kendi durumu değil, dışarıdan geliyor: sağ panel ve tuval
  // aynı seçimi göstermeli.
  const viewNodes = useMemo(
    () => nodes.map((n) => ({ ...n, selected: n.id === selectedId })),
    [nodes, selectedId],
  );

  const handleNodes = useCallback(
    (changes: NodeChange[]) => {
      if (!onNodesChange) return;
      onNodesChange(
        fromRFNodes(applyNodeChanges(changes, toRFNodes(nodes))),
        changes.some(alters),
      );
    },
    [nodes, onNodesChange],
  );

  const handleEdges = useCallback(
    (changes: EdgeChange[]) => {
      if (!onEdgesChange) return;
      onEdgesChange(
        fromRFEdges(applyEdgeChanges(changes, toRFEdges(edges))),
        changes.some(alters),
      );
    },
    [edges, onEdgesChange],
  );

  const handleConnect = useCallback(
    (conn: Connection) => {
      if (!onEdgesChange || !conn.source || !conn.target) return;
      onEdgesChange(
        fromRFEdges(
          addEdge({ ...conn, id: edgeId(conn.source, conn.target) }, toRFEdges(edges)),
        ),
        true,
      );
    },
    [edges, onEdgesChange],
  );

  /**
   * Döngü ÇİZİLİRKEN engellenir.
   *
   * Kaydetmeye kadar beklemek, kullanıcıya hatayı ancak işini bitirdikten sonra
   * söylemek ve yaptığını geri aldırmak olurdu. Backend doğrulaması yine son
   * söz: arayüz bir şeyi kaçırırsa kayıt reddedilir.
   */
  const isValidConnection = useCallback(
    (conn: Connection | FlowEdge) => {
      const source = "source" in conn ? conn.source : null;
      const target = "target" in conn ? conn.target : null;
      if (!source || !target) return false;

      // Tetikleyiciye bağ giremez.
      const targetNode = nodes.find((n) => n.id === target);
      if (targetNode?.data.kind.startsWith("trigger.")) return false;

      return !wouldCreateCycle(edges, source, target);
    },
    [edges, nodes],
  );

  return (
    <div
      className="overflow-hidden rounded-card border border-line bg-deck"
      style={{ height }}
    >
      <ReactFlowProvider>
        <FitOnSignal signal={fitSignal} />
        <ReactFlow
          nodes={toRFNodes(viewNodes)}
          edges={toRFEdges(edges)}
          nodeTypes={nodeTypes}
          onNodesChange={readOnly ? undefined : handleNodes}
          onEdgesChange={readOnly ? undefined : handleEdges}
          onConnect={readOnly ? undefined : handleConnect}
          isValidConnection={isValidConnection}
          onNodeClick={(_, node) => onSelect?.(node.id)}
          onPaneClick={() => onSelect?.(null)}
          nodesDraggable={!readOnly}
          nodesConnectable={!readOnly}
          elementsSelectable
          deleteKeyCode={readOnly ? null : ["Backspace", "Delete"]}
          fitView
          fitViewOptions={{ padding: 0.2, maxZoom: FIT_MAX_ZOOM }}
          proOptions={{ hideAttribution: false }}
        >
          <Background variant={BackgroundVariant.Dots} gap={18} size={1} />
          <Controls showInteractive={false} />
        </ReactFlow>
      </ReactFlowProvider>
    </div>
  );
}
