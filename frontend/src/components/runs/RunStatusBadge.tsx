import type { RunStatus } from "@/lib/types";
import { Badge, StatusDot } from "@/components/ui/primitives";

type Tone = "neutral" | "info" | "success" | "warning" | "danger" | "accent";

const STATUS: Record<RunStatus, { label: string; tone: Tone; live?: boolean }> = {
  pending: { label: "sırada", tone: "accent", live: true },
  running: { label: "çalışıyor", tone: "accent", live: true },
  succeeded: { label: "tamamlandı", tone: "success" },
  failed: { label: "başarısız", tone: "danger" },
  cancelled: { label: "iptal edildi", tone: "neutral" },
  timeout: { label: "zaman aşımı", tone: "warning" },
  interrupted: { label: "kesildi", tone: "warning" },
};

export function RunStatusBadge({ status }: { status: RunStatus }) {
  const s = STATUS[status];
  return (
    <Badge tone={s.tone}>
      <span className="mr-1.5 inline-flex">
        <StatusDot tone={s.tone} pulse={s.live} />
      </span>
      {s.label}
    </Badge>
  );
}

/** Durum sürüyor mu — canlı akış ve iptal düğmesi buna bakar. */
export function isActive(status: RunStatus): boolean {
  return status === "pending" || status === "running";
}
