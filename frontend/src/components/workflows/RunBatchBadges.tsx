import { Badge, StatusDot } from "@/components/ui/primitives";
import {
  RUN_BATCH_ITEM_STATUS_TEXT,
  RUN_BATCH_STATUS_TEXT,
  type RunBatchItemStatus,
  type RunBatchStatus,
} from "@/lib/types";

/**
 * Toplu iş ve öğe durum rozetleri (spec 023).
 *
 * Renk tek başına anlam taşımaz: her rozet metnini de gösterir. Renk körlüğünde
 * ve siyah-beyaz çıktıda okunabilen tek kanal etikettir.
 */

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
      return "warning";
    default:
      return "neutral";
  }
}

/** Toplu iş sürüyor mu — ekranın kendiliğinden tazelenip tazelenmeyeceği buna bağlı. */
export function isBatchActive(status: RunBatchStatus): boolean {
  return status === "queued" || status === "running";
}

export function RunBatchBadge({ status }: { status: RunBatchStatus }) {
  // `done` başarı DEĞİL, "kuyrukta iş kalmadı" demek; bu yüzden yeşil değil
  // nötr. Başarı sayılarda okunur.
  const tone: Tone = status === "cancelled" ? "warning" : toneOf(status);
  return (
    <Badge tone={tone}>
      <span className="flex items-center gap-1">
        {status === "running" && <StatusDot tone="accent" pulse />}
        {RUN_BATCH_STATUS_TEXT[status]}
      </span>
    </Badge>
  );
}

export function RunBatchItemBadge({ status }: { status: RunBatchItemStatus }) {
  return (
    <Badge tone={toneOf(status)}>
      <span className="flex items-center gap-1">
        {status === "running" && <StatusDot tone="accent" pulse />}
        {RUN_BATCH_ITEM_STATUS_TEXT[status]}
      </span>
    </Badge>
  );
}
