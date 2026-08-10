"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { ApiError, api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { autoLayout, nextPosition } from "@/lib/flow-layout";
import { TRIGGER_TEXT, type GraphProblem, type NodeKind } from "@/lib/types";
import {
  flowToGraph,
  flowTypeOf,
  graphToFlow,
  makeStepId,
  newTriggerNode,
  type FlowEdge,
  type FlowNode,
} from "@/lib/workflow-graph";
import { Pagination } from "@/components/ui/Pagination";
import { FlowCanvas } from "@/components/flow/FlowCanvas";
import { NodeInspector } from "@/components/flow/NodeInspector";
import {
  WorkflowRunBadge,
  isWorkflowActive,
} from "@/components/workflows/WorkflowStatusBadge";
import { IconAgent, IconComment, IconPullRequest } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  Input,
  Mono,
  Notice,
  PageHeader,
  Section,
  Skeleton,
} from "@/components/ui/primitives";

/** Akış ekranındaki geçmiş listesi; tuvale yer kalsın diye küçük tutulur. */
const RUNS_PAGE_SIZE = 10;

export default function WorkflowEditorPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const qc = useQueryClient();

  const workflow = useQuery({
    queryKey: ["workflow", id],
    queryFn: () => api.workflows.get(id),
  });
  const agents = useQuery({ queryKey: ["agents"], queryFn: () => api.agents.list({ limit: 200 }) });
  const models = useQuery({
    queryKey: ["models", "workflow"],
    queryFn: () => api.models.list({ limit: 500, sort: "provider" }),
  });

  const [nodes, setNodes] = useState<FlowNode[]>([]);
  const [edges, setEdges] = useState<FlowEdge[]>([]);
  const [selected, setSelected] = useState<string | null>(null);
  const [problems, setProblems] = useState<GraphProblem[]>([]);
  const [dirty, setDirty] = useState(false);
  const [input, setInput] = useState("");
  // Adım eklendiğinde tuval yeniden sığdırılır; yeni adım ekran dışında kalmasın.
  const [fitSignal, setFitSignal] = useState(0);
  const [runOffset, setRunOffset] = useState(0);

  // Kayıtlı tanım yüklenince tuvale alınır. Kullanıcının düzenlemesi varsa
  // üzerine yazılmaz — yoksa çizdiği her şey yenilemede uçardı.
  useEffect(() => {
    if (!workflow.data || dirty) return;

    const flow = graphToFlow(workflow.data.graph);

    // Yeni akışın grafı yok: tuval boş açılmaz, başlangıç düğümüyle açılır.
    // Aksi halde kullanıcı adım ekleyip kaydetmeye çalışınca "akışın
    // başlangıcı yok" hatasını alır ve nereden ekleyeceğini bilemez.
    if (flow.nodes.length === 0) {
      setNodes([newTriggerNode()]);
      setEdges([]);
      return;
    }

    // Konumu olmayan eski akışlar (adım listesiyle kurulanlar) üst üste
    // yığılmasın diye yerleştirilir; elle taşınmış düğüme dokunulmaz.
    setNodes(autoLayout(flow.nodes, flow.edges));
    setEdges(flow.edges);
  }, [workflow.data, dirty]);

  const save = useMutation({
    mutationFn: () => api.workflows.saveVersion(id, flowToGraph(nodes, edges)),
    onSuccess: () => {
      setProblems([]);
      setDirty(false);
      void qc.invalidateQueries({ queryKey: ["workflow", id] });
      void qc.invalidateQueries({ queryKey: ["workflows"] });
    },
    onError: (err) => setProblems(err instanceof ApiError ? err.problems : []),
  });

  /*
   * Duraklatma.
   *
   * Pasif akış OTOMATİK tetiklenmez: Jira taraması onu atlar, webhook uçları
   * 409 döner. Elle çalıştırma açık kalır — kullanıcı düğmeye basıyorsa niyeti
   * bellidir. `is_active` alanı en baştan vardı ve liste "pasif" rozetini
   * çizmeye hazırdı, ama onu değiştirecek hiçbir arayüz yoktu.
   */
  const toggleActive = useMutation({
    mutationFn: () =>
      api.workflows.update(id, {
        name: wf.name,
        description: wf.description,
        isActive: !wf.isActive,
      }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["workflow", id] });
      void qc.invalidateQueries({ queryKey: ["workflows"] });
    },
  });

  const start = useMutation({
    mutationFn: () => api.workflows.start(id, input.trim()),
    onSuccess: (run) => router.push(`/workflows/${id}/runs/${run.id}`),
  });

  const runs = useQuery({
    queryKey: ["workflow-runs", id, runOffset],
    queryFn: () =>
      api.workflowRuns.list({ workflow: id, limit: RUNS_PAGE_SIZE, offset: runOffset }),
    refetchInterval: (q) =>
      q.state.data?.items.some((r) => isWorkflowActive(r.status)) ? 3000 : false,
  });

  const problemsByNode = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const p of problems) {
      if (!p.nodeId) continue;
      map.set(p.nodeId, [...(map.get(p.nodeId) ?? []), p.message]);
    }
    return map;
  }, [problems]);

  // Kusurlar ve agent adı düğümlere iliştirilir: kullanıcı hatayı TUVALDE
  // görmeli, bir liste taramak zorunda kalmamalı.
  const viewNodes = useMemo(
    () =>
      nodes.map((n) => ({
        ...n,
        data: {
          ...n.data,
          problems: problemsByNode.get(n.id),
          agentLabel: agents.data?.items.find((a) => a.id === n.data.config.agentId)?.name,
        },
      })),
    [nodes, problemsByNode, agents.data],
  );

  if (workflow.isPending) return <Skeleton rows={4} />;
  if (workflow.isError) {
    return <Notice tone="error">{describeError(workflow.error).message}</Notice>;
  }

  const wf = workflow.data;
  const selectedNode = nodes.find((n) => n.id === selected) ?? null;
  const generalProblems = problems.filter((p) => !p.nodeId);
  const canStart = wf.activeVersionId !== null && !dirty && input.trim() !== "";

  /** Tuvale yeni bir adım ekler. */
  function addStep(kind: NodeKind, label: string) {
    setDirty(true);
    setFitSignal((n) => n + 1);
    setNodes((prev) => {
      const taken = new Set(prev.map((n) => n.id));
      const same = prev.filter((n) => n.data.kind === kind).length;
      const name = same > 0 ? `${label} ${same + 1}` : label;
      const nodeId = makeStepId(name, taken);
      setSelected(nodeId);
      return [
        ...prev,
        {
          id: nodeId,
          type: flowTypeOf(kind),
          position: nextPosition(prev),
          data: { name, kind, config: {} },
        },
      ];
    });
  }

  function updateSelected(patch: Partial<FlowNode["data"]>) {
    if (!selected) return;
    setDirty(true);
    setNodes((prev) =>
      prev.map((n) => (n.id === selected ? { ...n, data: { ...n.data, ...patch } } : n)),
    );
  }

  function deleteSelected() {
    if (!selected) return;
    setDirty(true);
    setNodes((prev) => prev.filter((n) => n.id !== selected));
    setEdges((prev) => prev.filter((e) => e.source !== selected && e.target !== selected));
    setSelected(null);
  }

  return (
    <div className="space-y-7">
      <PageHeader
        title={wf.name}
        description={
          <>
            {wf.projectName}
            {wf.activeVersion
              ? ` · kayıtlı sürüm v${wf.activeVersion}`
              : " · henüz kaydedilmedi"}
            {wf.description && ` · ${wf.description}`}
            {!wf.isActive && " · duraklatıldı"}
          </>
        }
        actions={
          <>
            <Link href="/workflows">
              <Button>Geri</Button>
            </Link>
            {/* Palet: kullanıcı hangi tür adımı ekleyeceğini seçer.
                Tek "Adım ekle" düğmesi, agent olmayan düğümleri gizlerdi. */}
            <Button
              icon={<IconAgent className="size-3.5" />}
              onClick={() => addStep("agent", "Agent adımı")}
            >
              Agent
            </Button>
            <Button
              icon={<IconPullRequest className="size-3.5" />}
              onClick={() => addStep("github.pr", "PR aç")}
            >
              PR aç
            </Button>
            <Button
              icon={<IconComment className="size-3.5" />}
              onClick={() => addStep("jira.comment", "Jira yorumu")}
            >
              Jira yorumu
            </Button>
            <Button
              disabled={toggleActive.isPending}
              title={
                wf.isActive
                  ? "Otomatik tetikleme durur (Jira taraması, dış adres). Elle çalıştırma açık kalır."
                  : "Otomatik tetikleme yeniden başlar."
              }
              onClick={() => toggleActive.mutate()}
            >
              {toggleActive.isPending
                ? "…"
                : wf.isActive
                  ? "Duraklat"
                  : "Etkinleştir"}
            </Button>
            <Button
              variant="primary"
              disabled={save.isPending || nodes.length === 0}
              onClick={() => save.mutate()}
            >
              {save.isPending ? "Kaydediliyor…" : dirty ? "Kaydet" : "Kayıtlı"}
            </Button>
          </>
        }
      />

      {generalProblems.length > 0 && (
        <Notice tone="error" title="Akış kaydedilemedi">
          <ul className="mt-1 list-disc space-y-0.5 pl-4">
            {generalProblems.map((p, i) => (
              <li key={i}>{p.message}</li>
            ))}
          </ul>
        </Notice>
      )}
      {save.isError && problems.length === 0 && (
        <Notice tone="error">{describeError(save.error).message}</Notice>
      )}

      <div className="grid gap-5 xl:grid-cols-[1fr_340px]">
        <FlowCanvas
          nodes={viewNodes}
          edges={edges}
          selectedId={selected}
          fitSignal={fitSignal}
          onSelect={setSelected}
          onNodesChange={(next, meaningful) => {
            if (meaningful) setDirty(true);
            setNodes(next);
          }}
          onEdgesChange={(next, meaningful) => {
            if (meaningful) setDirty(true);
            setEdges(next);
          }}
        />

        <NodeInspector
          node={selectedNode}
          nodes={nodes}
          edges={edges}
          agents={agents.data?.items ?? []}
          models={models.data?.items ?? []}
          problems={selected ? (problemsByNode.get(selected) ?? []) : []}
          workflowId={id}
          hookToken={wf.hookToken}
          workflowActive={wf.isActive}
          onChange={updateSelected}
          onDelete={deleteSelected}
        />
      </div>

      <Section
        title="Çalıştır"
        description="Akışa bir görev metni verin; ilk adımlar bunu görür."
      >
        <Card>
          {dirty && (
            <Notice tone="warning">
              Kaydedilmemiş değişiklikler var. Akış <strong>kayıtlı</strong> sürümle
              çalışır — önce kaydedin.
            </Notice>
          )}
          {!wf.activeVersionId && (
            <Notice tone="warning">
              Akışın kayıtlı bir tanımı yok. Adımları çizip kaydedin.
            </Notice>
          )}

          <div className="mt-3 flex flex-wrap items-end gap-3">
            <label className="block min-w-64 flex-1">
              <span className="text-[11px] tracking-wide text-ink-2 uppercase">Görev</span>
              <Input
                className="mt-1"
                value={input}
                placeholder="Örn: giriş ekranındaki hatayı düzelt"
                onChange={(e) => setInput(e.target.value)}
              />
            </label>
            <Button
              variant="primary"
              disabled={!canStart || start.isPending}
              onClick={() => start.mutate()}
            >
              {start.isPending ? "Başlatılıyor…" : "Akışı çalıştır"}
            </Button>
          </div>

          {start.isError && (
            <Notice tone="error" title={describeError(start.error).message}>
              {describeError(start.error).hint}
            </Notice>
          )}
        </Card>
      </Section>

      <HookCard workflowId={id} token={wf.hookToken} />

      <Section title="Son çalışmalar">
        {runs.data?.total === 0 && (
          <Card>
            <p className="text-[13px] text-ink-3">Bu akış henüz hiç çalışmadı.</p>
          </Card>
        )}
        {runs.data && runs.data.items.length > 0 && (
          <div className="overflow-hidden rounded-card border border-line">
            {runs.data.items.map((r, i) => (
              <Link
                key={r.id}
                href={`/workflows/${id}/runs/${r.id}`}
                className={`flex items-center gap-4 bg-surface px-4 py-3 transition-colors hover:bg-raised ${
                  i > 0 ? "border-t border-line" : ""
                }`}
              >
                <div className="w-28 shrink-0">
                  <WorkflowRunBadge status={r.status} />
                </div>
                <div className="min-w-0 flex-1 truncate text-[13px]">
                  {r.input || <span className="text-ink-3">görev metni yok</span>}
                </div>
                <Badge>v{r.version}</Badge>
                <span className="shrink-0 text-[12px] text-ink-3">
                  {TRIGGER_TEXT[r.triggerKind].short}
                </span>
              </Link>
            ))}
          </div>
        )}

        {runs.data && (
          <Pagination
            total={runs.data.total}
            limit={runs.data.limit}
            offset={runs.data.offset}
            onChange={setRunOffset}
            unit="çalışma"
          />
        )}
      </Section>
    </div>
  );
}

/* ── Tetikleme adresi ────────────────────────────────────────────────────── */

function HookCard({ workflowId, token }: { workflowId: string; token: string }) {
  const qc = useQueryClient();
  const [copied, setCopied] = useState(false);

  const rotate = useMutation({
    mutationFn: () => api.workflows.rotateHook(workflowId),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ["workflow", workflowId] }),
  });

  const url = api.workflowRuns.hookUrl(token);

  return (
    <Section
      title="Dışarıdan tetikleme"
      description="Bu adrese POST atan herkes akışı başlatabilir — adresin kendisi anahtardır, paylaşırken dikkat edin."
    >
      <Card>
        <div className="flex flex-wrap items-center gap-2">
          <Mono className="min-w-0 flex-1 truncate">{url}</Mono>
          <Button
            size="sm"
            onClick={() => {
              void navigator.clipboard.writeText(url);
              setCopied(true);
              setTimeout(() => setCopied(false), 1500);
            }}
          >
            {copied ? "Kopyalandı" : "Kopyala"}
          </Button>
          <Button size="sm" variant="danger" onClick={() => rotate.mutate()}>
            {rotate.isPending ? "Yenileniyor…" : "Yenile"}
          </Button>
        </div>
        <p className="mt-2 text-[12px] text-ink-3">
          Gövdedeki metin alanları <Mono>{"{{ trigger.<alan> }}"}</Mono> ile,{" "}
          <Mono>input</Mono> alanı ise görev metni olarak kullanılır. Adres yenilenince
          eskisi anında geçersiz olur.
        </p>
      </Card>
    </Section>
  );
}
