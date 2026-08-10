"use client";

import { useQuery } from "@tanstack/react-query";

import { api } from "@/lib/api";
import type { Agent, JiraScanState, Model } from "@/lib/types";
import { ancestorsOf, type FlowEdge, type FlowNode } from "@/lib/workflow-graph";
import { ModelPicker } from "@/components/models/ModelPicker";
import { IconTrash } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  Input,
  Mono,
  Notice,
  Select,
  Textarea,
  Well,
  formatDate,
} from "@/components/ui/primitives";

/**
 * Seçili düğümün ayar paneli.
 *
 * Tuval "ne neye bağlı" sorusunu, panel "bu adım ne yapıyor" sorusunu cevaplar.
 * İkisini aynı karta sıkıştırmak, düğümleri okunmaz derecede büyütürdü.
 */
export function NodeInspector({
  node,
  nodes,
  edges,
  agents,
  models,
  problems,
  workflowId,
  hookToken,
  workflowActive,
  onChange,
  onDelete,
}: {
  node: FlowNode | null;
  nodes: FlowNode[];
  edges: FlowEdge[];
  agents: Agent[];
  models: Model[];
  problems: string[];
  workflowId: string;
  hookToken: string;
  workflowActive: boolean;
  onChange: (patch: Partial<FlowNode["data"]>) => void;
  onDelete: () => void;
}) {
  if (!node) {
    return (
      <Card>
        <p className="text-[13px] text-ink-2">
          Düzenlemek için tuvalden bir adım seçin. Yeni adım eklemek için{" "}
          <strong>Adım ekle</strong>, bağ kurmak için bir adımın sağ ucundan
          diğerinin sol ucuna sürükleyin.
        </p>
      </Card>
    );
  }

  if (node.data.kind === "github.pr" || node.data.kind === "jira.comment") {
    return (
      <ActionFields
        node={node}
        nodes={nodes}
        edges={edges}
        problems={problems}
        onChange={onChange}
        onDelete={onDelete}
      />
    );
  }

  if (node.data.kind !== "agent") {
    return (
      <TriggerFields
        node={node}
        workflowId={workflowId}
        hookToken={hookToken}
        workflowActive={workflowActive}
        problems={problems}
        onChange={onChange}
      />
    );
  }

  // Referans YALNIZCA atalara verilebilir: paralel bir kardeşin çıktısı bu adım
  // çalışırken hazır olmayabilir ve backend zaten reddeder.
  const ancestors = ancestorsOf(edges, node.id);
  const refs = nodes.filter((n) => ancestors.has(n.id) && n.data.kind === "agent");

  const insert = (text: string) =>
    onChange({ config: { ...node.data.config, prompt: (node.data.config.prompt ?? "") + text } });

  return (
    <Card>
      <div className="mb-3 flex items-center justify-between gap-2">
        <Badge title="Şablon referanslarında bu kimlik kullanılır">
          <span className="font-mono">{node.id}</span>
        </Badge>
        <Button size="sm" variant="danger" icon={<IconTrash className="size-3.5" />} onClick={onDelete}>
          Adımı sil
        </Button>
      </div>

      {problems.length > 0 && (
        <Notice tone="error">
          <ul className="list-disc space-y-0.5 pl-4">
            {problems.map((p, i) => (
              <li key={i}>{p}</li>
            ))}
          </ul>
        </Notice>
      )}

      <label className="mt-3 block">
        <span className="text-[11px] tracking-wide text-ink-2 uppercase">Ad</span>
        <Input
          className="mt-1"
          value={node.data.name}
          placeholder="Adım adı"
          onChange={(e) => onChange({ name: e.target.value })}
        />
      </label>

      <label className="mt-3 block">
        <span className="text-[11px] tracking-wide text-ink-2 uppercase">Agent</span>
        <Select
          className="mt-1"
          value={node.data.config.agentId ?? ""}
          onChange={(e) =>
            onChange({ config: { ...node.data.config, agentId: e.target.value } })
          }
        >
          <option value="">Seçin…</option>
          {agents.map((a) => (
            <option key={a.id} value={a.id}>
              {a.name}
            </option>
          ))}
        </Select>
      </label>

      <div className="mt-3">
        <span className="text-[11px] tracking-wide text-ink-2 uppercase">Model</span>
        <div className="mt-1">
          <ModelPicker
            models={models}
            providerId={node.data.config.providerId ?? null}
            modelId={node.data.config.model ?? ""}
            onChange={(sel) =>
              onChange({
                config: {
                  ...node.data.config,
                  providerId: sel?.providerId,
                  model: sel?.modelId ?? "",
                },
              })
            }
            emptyLabel="Agent varsayılanı"
          />
        </div>
      </div>

      {/* Gönderim AÇIKÇA seçilir: bir akışın habersizce depoya yazması
          sürpriz olurdu. PR açan akışta bu gerekli. */}
      <label className="mt-3 flex items-start gap-2">
        <input
          type="checkbox"
          className="mt-0.5 size-3.5 accent-accent"
          checked={node.data.config.autoPush ?? false}
          onChange={(e) =>
            onChange({ config: { ...node.data.config, autoPush: e.target.checked } })
          }
        />
        <span className="text-[12px]">
          Değişikliği branch&apos;e gönder
          <span className="mt-0.5 block text-[11px] text-ink-3">
            Bu adım kod değiştirirse yeni bir branch açılır. PR düğümü bu
            branch&apos;i kullanır.
          </span>
        </span>
      </label>

      <label className="mt-3 block">
        <span className="text-[11px] tracking-wide text-ink-2 uppercase">Talimat</span>
        <Textarea
          className="mt-1 h-36"
          value={node.data.config.prompt ?? ""}
          placeholder="Bu adımda agent ne yapsın?"
          onChange={(e) =>
            onChange({ config: { ...node.data.config, prompt: e.target.value } })
          }
        />
      </label>

      <Well className="mt-2 p-2.5">
        <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-ink-2">
          <span>Ekle:</span>
          <RefButton label="{{ input }}" onClick={() => insert("{{ input }}")} />
          {refs.map((r) => (
            <RefButton
              key={r.id}
              label={`{{ steps.${r.id}.output }}`}
              title={`${r.data.name || r.id} adımının çıktısı`}
              onClick={() => insert(`{{ steps.${r.id}.output }}`)}
            />
          ))}
          {refs.length === 0 && (
            <span className="text-ink-3">— bu adımdan önce çalışan adım yok</span>
          )}
        </div>
      </Well>
    </Card>
  );
}

/**
 * Tetikleyici düğümünün alanları.
 *
 * Tetikleyici tuvale EKLENMEZ, DEĞİŞTİRİLİR: bir akışın tam olarak bir girişi
 * vardır (backend de öyle doğruluyor). "Jira tetikleyici ekle" düğmesi ikinci
 * bir başlangıç üretir ve kaydetme her seferinde reddedilirdi; bunun yerine
 * var olan başlangıcın türü seçiliyor.
 */
function TriggerFields({
  node,
  workflowId,
  hookToken,
  workflowActive,
  problems,
  onChange,
}: {
  node: FlowNode;
  workflowId: string;
  hookToken: string;
  workflowActive: boolean;
  problems: string[];
  onChange: (patch: Partial<FlowNode["data"]>) => void;
}) {
  const kind = node.data.kind;
  const scan = useQuery({
    queryKey: ["jira-scan", workflowId],
    queryFn: () => api.workflows.jiraScan(workflowId),
    // Tarama arka planda çalışıyor; sayfa açıkken kendiliğinden tazelensin.
    refetchInterval: 30_000,
    enabled: kind === "trigger.jira" && workflowActive,
  });

  return (
    <Card>
      <p className="text-[13px] font-medium">Başlangıç</p>
      <p className="mt-1.5 text-[13px] text-ink-2">
        Akışın giriş noktası. Buraya bağ girmez; buradan çıkan adımlar ilk
        çalışanlardır.
      </p>

      {problems.length > 0 && (
        <Notice tone="error">
          <ul className="list-disc space-y-0.5 pl-4">
            {problems.map((p, i) => (
              <li key={i}>{p}</li>
            ))}
          </ul>
        </Notice>
      )}

      <label className="mt-3 block">
        <span className="text-[11px] tracking-wide text-ink-2 uppercase">Nasıl başlar</span>
        <Select
          className="mt-1"
          value={kind}
          onChange={(e) => onChange({ kind: e.target.value as FlowNode["data"]["kind"] })}
        >
          <option value="trigger.manual">Elle — &quot;Çalıştır&quot; düğmesiyle</option>
          <option value="trigger.jira">Jira task&apos;ı — JQL eşleşince kendiliğinden</option>
        </Select>
      </label>

      {kind === "trigger.jira" ? (
        <>
          <label className="mt-3 block">
            <span className="text-[11px] tracking-wide text-ink-2 uppercase">JQL sorgusu</span>
            <Textarea
              className="mt-1 h-20 font-mono text-[12px]"
              value={node.data.config.jql ?? ""}
              placeholder={`project = SCRUM AND status = "Yapılacaklar"`}
              onChange={(e) => onChange({ config: { ...node.data.config, jql: e.target.value } })}
            />
            <span className="mt-1 block text-[11px] text-ink-3">
              Eşleşen her task akışı bir kez başlatır. Task Jira&apos;da
              güncellenirse yeniden çalışır; akışın kendi yazdığı yorum
              tetikleyici sayılmaz.
            </span>
          </label>

          <div className="mt-3">
            <span className="text-[11px] tracking-wide text-ink-2 uppercase">
              Jira webhook adresi
            </span>
            <Well className="mt-1 p-2">
              <Mono className="break-all">{api.workflowRuns.jiraHookUrl(hookToken)}</Mono>
            </Well>
            <span className="mt-1 block text-[11px] text-ink-3">
              İsteğe bağlı: Jira&apos;ya tanımlanırsa task anında işlenir.
              Tanımlanmazsa tarama yine yakalar — ikisi aynı korumadan geçer.
            </span>
          </div>

          <div className="mt-3">
            <span className="text-[11px] tracking-wide text-ink-2 uppercase">Son tarama</span>
            {/* Duraklatılmış akış taranmaz; "son tarama" satırının eski bir
                zaman göstermesi, tarama sürüyormuş izlenimi verirdi. */}
            {workflowActive ? (
              <ScanSummary state={scan.data} />
            ) : (
              <Notice tone="warning">
                Akış duraklatıldı — tarama yapılmıyor, webhook çağrıları
                reddediliyor. Üstteki <strong>Etkinleştir</strong> ile açılır.
              </Notice>
            )}
          </div>

          <Well className="mt-2 p-2.5">
            <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-ink-2">
              <span>Task alanları:</span>
              {["key", "summary", "description", "status", "issueType", "url"].map((f) => (
                <span key={f} className="rounded border border-line bg-surface px-1.5 py-0.5 font-mono">
                  {`{{ trigger.${f} }}`}
                </span>
              ))}
            </div>
          </Well>
        </>
      ) : (
        <p className="mt-3 text-[13px] text-ink-2">
          Verdiğiniz görev metni <Mono>{"{{ input }}"}</Mono> ile okunur.
        </p>
      )}
    </Card>
  );
}

/** Son taramanın özeti — arka planda çalışan taramanın tek görünür izi. */
function ScanSummary({ state }: { state?: JiraScanState }) {
  if (!state?.scannedAt) {
    return (
      <p className="mt-1 text-[12px] text-ink-3">
        Henüz taranmadı. Akış kaydedilip etkinleştirildikten sonra tarama
        kendiliğinden başlar.
      </p>
    );
  }

  return (
    <div className="mt-1 space-y-1">
      <p className="text-[12px] text-ink-2">
        {formatDate(state.scannedAt)} · {state.found} task eşleşti,{" "}
        {state.started} akış başlatıldı
      </p>
      {state.error && <Notice tone="error">{state.error}</Notice>}
    </div>
  );
}

/**
 * PR ve Jira düğümlerinin alanları.
 *
 * Agent panelinden ayrı: bu düğümlerin model ve talimat alanı yok, onun yerine
 * hedef ve içerik alanları var. Tek bir panelde birleştirmek, kullanılmayan
 * alanların sürekli ekranda durması demekti.
 */
function ActionFields({
  node,
  nodes,
  edges,
  problems,
  onChange,
  onDelete,
}: {
  node: FlowNode;
  nodes: FlowNode[];
  edges: FlowEdge[];
  problems: string[];
  onChange: (patch: Partial<FlowNode["data"]>) => void;
  onDelete: () => void;
}) {
  const isPR = node.data.kind === "github.pr";
  const cfg = node.data.config;
  const set = (patch: Partial<typeof cfg>) =>
    onChange({ config: { ...cfg, ...patch } });

  const ancestors = ancestorsOf(edges, node.id);
  const refs = nodes.filter((n) => ancestors.has(n.id) && n.data.kind !== "trigger.manual");

  return (
    <Card>
      <div className="mb-3 flex items-center justify-between gap-2">
        <Badge title="Şablon referanslarında bu kimlik kullanılır">
          <span className="font-mono">{node.id}</span>
        </Badge>
        <Button size="sm" variant="danger" icon={<IconTrash className="size-3.5" />} onClick={onDelete}>
          Adımı sil
        </Button>
      </div>

      {problems.length > 0 && (
        <Notice tone="error">
          <ul className="list-disc space-y-0.5 pl-4">
            {problems.map((p, i) => (
              <li key={i}>{p}</li>
            ))}
          </ul>
        </Notice>
      )}

      <label className="mt-3 block">
        <span className="text-[11px] tracking-wide text-ink-2 uppercase">Ad</span>
        <Input
          className="mt-1"
          value={node.data.name}
          placeholder={isPR ? "PR aç" : "Jira'ya yorum yaz"}
          onChange={(e) => onChange({ name: e.target.value })}
        />
      </label>

      {isPR ? (
        <>
          <label className="mt-3 block">
            <span className="text-[11px] tracking-wide text-ink-2 uppercase">PR başlığı</span>
            <Input
              className="mt-1"
              value={cfg.title ?? ""}
              placeholder="{{ trigger.summary }}"
              onChange={(e) => set({ title: e.target.value })}
            />
          </label>

          <label className="mt-3 block">
            <span className="text-[11px] tracking-wide text-ink-2 uppercase">
              Hedef branch
            </span>
            <Input
              className="mt-1"
              value={cfg.baseBranch ?? ""}
              placeholder="projenin varsayılanı"
              onChange={(e) => set({ baseBranch: e.target.value })}
            />
          </label>

          <label className="mt-3 block">
            <span className="text-[11px] tracking-wide text-ink-2 uppercase">
              Kaynak branch
            </span>
            <Input
              className="mt-1 font-mono text-[12px]"
              value={cfg.headBranch ?? ""}
              placeholder="boş bırakın — gönderim yapan adımdan alınır"
              onChange={(e) => set({ headBranch: e.target.value })}
            />
            <span className="mt-1 block text-[11px] text-ink-3">
              Birden fazla adım gönderim yapıyorsa hangisi olduğu belirtilmeli.
            </span>
          </label>
        </>
      ) : (
        <label className="mt-3 block">
          <span className="text-[11px] tracking-wide text-ink-2 uppercase">
            Issue anahtarı
          </span>
          <Input
            className="mt-1 font-mono text-[12px]"
            value={cfg.issueKey ?? ""}
            placeholder="SCRUM-1 veya {{ trigger.key }}"
            onChange={(e) => set({ issueKey: e.target.value })}
          />
        </label>
      )}

      <label className="mt-3 block">
        <span className="text-[11px] tracking-wide text-ink-2 uppercase">
          {isPR ? "PR açıklaması" : "Yorum metni"}
        </span>
        <Textarea
          className="mt-1 h-32"
          value={cfg.body ?? ""}
          placeholder={isPR ? "Bu PR ne yapıyor?" : "Task'a ne yazılsın?"}
          onChange={(e) => set({ body: e.target.value })}
        />
      </label>

      <Well className="mt-2 p-2.5">
        <div className="flex flex-wrap items-center gap-1.5 text-[11px] text-ink-2">
          <span>Ekle:</span>
          <RefButton label="{{ input }}" onClick={() => set({ body: (cfg.body ?? "") + "{{ input }}" })} />
          {refs.map((r) => (
            <RefButton
              key={r.id}
              label={`{{ steps.${r.id}.${r.data.kind === "agent" ? "output" : "url"} }}`}
              onClick={() =>
                set({
                  body:
                    (cfg.body ?? "") +
                    `{{ steps.${r.id}.${r.data.kind === "agent" ? "output" : "url"} }}`,
                })
              }
            />
          ))}
          {refs.length === 0 && (
            <span className="text-ink-3">— bu adımdan önce çalışan adım yok</span>
          )}
        </div>
      </Well>
    </Card>
  );
}

function RefButton({
  label,
  title,
  onClick,
}: {
  label: string;
  title?: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      title={title}
      onClick={onClick}
      className="rounded border border-control-line bg-surface px-1.5 py-0.5 font-mono transition-colors hover:border-ink-2"
    >
      {label}
    </button>
  );
}
