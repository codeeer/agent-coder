"use client";

import { Handle, Position, type NodeProps } from "@xyflow/react";
import { memo } from "react";
import type { NodeKind, WorkflowStepStatus } from "@/lib/types";
import type { FlowNodeData } from "@/lib/workflow-graph";
import {
  IconAgent,
  IconComment,
  IconPlay,
  IconPullRequest,
} from "@/components/ui/icons";

/**
 * Tuval düğümleri.
 *
 * `memo` ile sarılı: büyük bir akışta her sürükleme adımında tüm düğümlerin
 * yeniden çizilmesi tuvali tutuklaştırır.
 */

/** Düğümün taşıdığı ek gösterim bilgisi. */
export interface NodeExtras extends FlowNodeData {
  /** Agent'ın okunur adı — kimlik değil, kullanıcı bunu görmeli. */
  agentLabel?: string;
  /** Doğrulama kusurları; varsa düğüm kırmızı çerçeve alır. */
  problems?: string[];
  /** İzleme modunda adımın durumu. */
  status?: WorkflowStepStatus;
  /** Salt okunur mod (çalışma izleme). */
  readOnly?: boolean;
}

const STATUS_LABEL: Record<WorkflowStepStatus, string> = {
  pending: "sırada",
  running: "çalışıyor",
  succeeded: "başarılı",
  failed: "başarısız",
  skipped: "atlandı",
  cancelled: "durduruldu",
};

/**
 * Durum kenarlığı.
 *
 * Durum ASLA yalnızca renkle anlatılmaz — her düğümde metin etiketi de var
 * (projenin genel kuralı; renk körlüğünde ve siyah-beyaz çıktıda tek okunur kanal).
 */
const STATUS_RING: Record<WorkflowStepStatus, string> = {
  pending: "border-line",
  running: "border-accent",
  succeeded: "border-ok",
  failed: "border-danger",
  skipped: "border-line opacity-55",
  cancelled: "border-warn opacity-75",
};

function AgentNodeBase({ data, selected }: NodeProps) {
  const d = data as unknown as NodeExtras;
  const hasProblem = (d.problems?.length ?? 0) > 0;

  const ring = hasProblem
    ? "border-danger"
    : d.status
      ? STATUS_RING[d.status]
      : selected
        ? "border-accent"
        : "border-line";

  return (
    <div
      className={`w-56 rounded-card border bg-surface shadow-(--shadow-card) transition-colors ${ring} ${
        selected ? "ring-2 ring-accent/30" : ""
      }`}
    >
      {/* Gelen bağ solda, giden sağda: akış soldan sağa okunur. */}
      <Handle type="target" position={Position.Left} className="!size-2 !bg-line-strong" />

      <div className="flex items-start gap-2 px-3 py-2.5">
        <IconAgent className="mt-0.5 size-4 shrink-0 text-ink-3" />
        <div className="min-w-0 flex-1">
          <div className="truncate text-[13px] font-medium">
            {d.name || <span className="text-ink-3">adsız adım</span>}
          </div>
          {/* Düzenlemede eksik agent uyarı, izlemede sessizlik: çalışmış bir
              adımda "seçilmedi" yazmak yanlış olurdu. */}
          {(d.agentLabel || !d.readOnly) && (
            <div className="mt-0.5 truncate text-[11px] text-ink-3">
              {d.agentLabel ?? "agent seçilmedi"}
            </div>
          )}
          {d.config.model && (
            <div className="mt-0.5 truncate font-mono text-[10px] text-ink-3">
              {d.config.model}
            </div>
          )}
        </div>
      </div>

      {(d.status || hasProblem) && (
        <div className="border-t border-line px-3 py-1.5 text-[11px]">
          {hasProblem ? (
            <span className="text-danger">{d.problems?.[0]}</span>
          ) : (
            <span className="text-ink-2">{STATUS_LABEL[d.status!]}</span>
          )}
        </div>
      )}

      <Handle type="source" position={Position.Right} className="!size-2 !bg-line-strong" />
    </div>
  );
}

/**
 * Eylem düğümü — PR açma, Jira yorumu.
 *
 * Agent düğümünden AYRI çiziliyor: kullanıcı akışın neresinin model çağırdığını
 * (para harcadığını), neresinin dış servise dokunduğunu bir bakışta ayırmalı.
 */
function ActionNodeBase({ data, selected }: NodeProps) {
  const d = data as unknown as NodeExtras;
  const hasProblem = (d.problems?.length ?? 0) > 0;

  const ring = hasProblem
    ? "border-danger"
    : d.status
      ? STATUS_RING[d.status]
      : selected
        ? "border-accent"
        : "border-line";

  const isPR = d.kind === "github.pr";
  const Icon = isPR ? IconPullRequest : IconComment;

  return (
    <div
      className={`w-56 rounded-card border bg-raised shadow-(--shadow-card) transition-colors ${ring} ${
        selected ? "ring-2 ring-accent/30" : ""
      }`}
    >
      <Handle type="target" position={Position.Left} className="!size-2 !bg-line-strong" />

      <div className="flex items-start gap-2 px-3 py-2.5">
        <Icon className="mt-0.5 size-4 shrink-0 text-ink-3" />
        <div className="min-w-0 flex-1">
          <div className="truncate text-[13px] font-medium">
            {d.name || (isPR ? "PR aç" : "Jira'ya yorum yaz")}
          </div>
          <div className="mt-0.5 truncate text-[11px] text-ink-3">
            {isPR
              ? d.config.title || "başlık yok"
              : d.config.issueKey || "issue seçilmedi"}
          </div>
        </div>
      </div>

      {(d.status || hasProblem) && (
        <div className="border-t border-line px-3 py-1.5 text-[11px]">
          {hasProblem ? (
            <span className="text-danger">{d.problems?.[0]}</span>
          ) : (
            <span className="text-ink-2">{STATUS_LABEL[d.status!]}</span>
          )}
        </div>
      )}

      <Handle type="source" position={Position.Right} className="!size-2 !bg-line-strong" />
    </div>
  );
}

/**
 * Tetikleyici türünün tuvaldeki adı.
 *
 * Düğüm, akışın nasıl başladığını TEK BAKIŞTA söylemeli: "Başlangıç" yazan bir
 * balonun elle mi yoksa Jira'dan mı tetiklendiği ancak paneli açınca anlaşılırdı.
 */
const TRIGGER_LABELS: Partial<Record<NodeKind, string>> = {
  "trigger.manual": "Başlangıç",
  "trigger.webhook": "Dışarıdan",
  "trigger.jira": "Jira task'ı",
};

function TriggerNodeBase({ data, selected }: NodeProps) {
  const d = data as unknown as NodeExtras;

  return (
    <div
      className={`rounded-full border bg-raised px-4 py-2 shadow-(--shadow-card) ${
        selected ? "border-accent ring-2 ring-accent/30" : "border-line-strong"
      }`}
    >
      <div className="flex items-center gap-2">
        <IconPlay className="size-3.5 text-ink-3" />
        <span className="text-[12px] font-medium">{TRIGGER_LABELS[d.kind] ?? "Başlangıç"}</span>
      </div>
      {/* Tetikleyiciye bağ GİRMEZ: akışın başıdır, giriş tutamağı yok. */}
      <Handle type="source" position={Position.Right} className="!size-2 !bg-line-strong" />
    </div>
  );
}

export const AgentNode = memo(AgentNodeBase);
export const ActionNode = memo(ActionNodeBase);
export const TriggerNode = memo(TriggerNodeBase);

export const nodeTypes = {
  agent: AgentNode,
  action: ActionNode,
  trigger: TriggerNode,
};
