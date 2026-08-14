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

const STRIP: Record<Tone, string> = {
  accent: "border-accent",
  success: "border-ok",
  danger: "border-danger",
  warning: "border-warn",
  info: "border-info",
  neutral: "border-line",
};

/**
 * Satır başındaki durum şeridinin rengi.
 *
 * Liste ekranında durum, kendi sütununda duran bir rozet DEĞİL: satırın sol
 * kenarındaki ince bir şerit ve yanındaki kısa etiket. Rozet 144px'lik bir
 * sütun tutuyordu ve o genişliğin tek işi tek bir kelimeyi ortalamaktı.
 *
 * ŞERİT TEK KANAL DEĞİL: yanında her zaman durum metni duruyor. Renk körlüğü
 * ve siyah-beyaz çıktıda okunan tek kanal etikettir (ui.md → Anlamlı renk).
 */
export function statusStrip(status: RunStatus): string {
  return STRIP[STATUS[status].tone];
}

const TEXT: Record<Tone, string> = {
  accent: "text-accent",
  success: "text-ok",
  danger: "text-danger",
  warning: "text-warn",
  info: "text-info",
  neutral: "text-ink-3",
};

/** Durum etiketinin rengi — şeritle aynı ton, üstveri satırında kullanılıyor. */
export function statusText(status: RunStatus): string {
  return TEXT[STATUS[status].tone];
}

/** Durumun kısa etiketi — şeridin yanında yazılan metin. */
export function statusLabel(status: RunStatus): string {
  return STATUS[status].label;
}
