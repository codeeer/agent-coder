import { Badge, StatusDot } from "@/components/ui/primitives";
import type { WorkflowRunStatus, WorkflowStepStatus } from "@/lib/types";

/**
 * Akış ve adım durum rozetleri.
 *
 * Renk tek başına anlam taşımaz: her rozet metnini de gösterir. Renk körlüğünde
 * ve siyah-beyaz çıktıda okunabilen tek kanal etikettir.
 */

const RUN_LABELS: Record<WorkflowRunStatus, string> = {
  pending: "sırada",
  running: "çalışıyor",
  succeeded: "başarılı",
  failed: "başarısız",
  cancelled: "durduruldu",
  interrupted: "kesildi",
};

const STEP_LABELS: Record<WorkflowStepStatus, string> = {
  pending: "sırada",
  running: "çalışıyor",
  succeeded: "başarılı",
  failed: "başarısız",
  skipped: "atlandı",
  cancelled: "durduruldu",
};

type Tone = "neutral" | "accent" | "success" | "warning" | "danger";


function toneOf(status: string): Tone {
  switch (status) {
    case "running":
      return "accent";
    case "succeeded":
      return "success";
    case "failed":
      return "danger";
    case "cancelled":
    case "interrupted":
    case "skipped":
      return "warning";
    default:
      return "neutral";
  }
}

export function isWorkflowActive(status: WorkflowRunStatus): boolean {
  return status === "pending" || status === "running";
}

export function WorkflowRunBadge({ status }: { status: WorkflowRunStatus }) {
  return (
    <Badge tone={toneOf(status)}>
      <span className="flex items-center gap-1">
        {status === "running" && <StatusDot tone="accent" pulse />}
        {RUN_LABELS[status]}
      </span>
    </Badge>
  );
}

export function WorkflowStepBadge({ status }: { status: WorkflowStepStatus }) {
  return (
    <Badge tone={toneOf(status)}>
      <span className="flex items-center gap-1">
        {status === "running" && <StatusDot tone="accent" pulse />}
        {STEP_LABELS[status]}
      </span>
    </Badge>
  );
}
