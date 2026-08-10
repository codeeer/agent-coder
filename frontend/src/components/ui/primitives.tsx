/**
 * Paylaşılan görsel parçalar.
 *
 * Amaç tam bir tasarım sistemi değil; aynı görünümün her sayfada yeniden
 * yazılmasını önlemek ve hiyerarşiyi tutarlı tutmak.
 */

import type React from "react";

// ─── Yüzey ──────────────────────────────────────────────────────────────────

export function Card({
  children,
  className = "",
  padded = true,
}: {
  children: React.ReactNode;
  className?: string;
  padded?: boolean;
}) {
  return (
    <div
      className={`rounded-card border border-line bg-surface shadow-(--shadow-card) ${
        padded ? "p-5" : ""
      } ${className}`}
    >
      {children}
    </div>
  );
}

/** Kart içinde ikinci düzey yüzey — kod, diff, ayar satırı gibi bloklar. */
export function Well({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-lg border border-line bg-raised ${className}`}>
      {children}
    </div>
  );
}

// ─── Sayfa iskeleti ─────────────────────────────────────────────────────────

export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <header className="mb-7 flex flex-wrap items-start justify-between gap-4 border-b border-line pb-5">
      <div className="min-w-0">
        <h1 className="text-[22px] leading-tight font-semibold tracking-[-0.01em]">
          {title}
        </h1>
        {description && (
          <p className="mt-1.5 max-w-2xl text-[13px] leading-relaxed text-ink-2">
            {description}
          </p>
        )}
      </div>
      {/* `flex-wrap`: dar ekranda beş düğmelik bir sıra tek satıra sığmıyor ve
          `shrink-0` yüzünden sondakiler ekranın dışına taşıyordu — akış
          ekranında "Kaydet" düğmesi telefonda hiç görünmüyordu. */}
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
    </header>
  );
}

export function Section({
  title,
  description,
  actions,
  children,
}: {
  title: string;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section>
      <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-[15px] font-semibold tracking-[-0.01em]">{title}</h2>
          {description && (
            <p className="mt-1 max-w-2xl text-[13px] leading-relaxed text-ink-2">
              {description}
            </p>
          )}
        </div>
        {/* `flex-wrap`: dar ekranda beş düğmelik bir sıra tek satıra sığmıyor ve
          `shrink-0` yüzünden sondakiler ekranın dışına taşıyordu — akış
          ekranında "Kaydet" düğmesi telefonda hiç görünmüyordu. */}
      {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
      </div>
      {children}
    </section>
  );
}

// ─── Düğme ──────────────────────────────────────────────────────────────────

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
type ButtonSize = "sm" | "md";

const buttonVariants: Record<ButtonVariant, string> = {
  primary:
    "bg-accent text-accent-ink hover:bg-accent-hover border border-transparent",
  // Sınır SÜSLEME token'ı değil `control-line`: düğmenin nerede başlayıp
  // bittiği yalnızca bu çizgiden anlaşılıyor, dolayısıyla 3:1 olmalı.
  // `line-strong` ile ölçüm 1,8:1 (açık) / 1,63:1 (koyu) veriyordu.
  secondary:
    "bg-surface text-ink border border-control-line hover:border-ink-2 hover:bg-raised",
  ghost:
    "bg-transparent text-ink-2 border border-transparent hover:bg-raised hover:text-ink",
  // /35 idi: kenar açık temada 1,76:1, koyu temada 1,85:1 ölçüldü — yani "Sil"
  // düğmesinin sınırı iki temada da görünmüyordu. Bu ölçüm uzun süre yapılmadı
  // çünkü denetim aracı saydam renkleri sessizce atlıyordu (bkz. spec 010).
  // /70: açık 3,32:1, koyu 3,90:1 — eşiği geçen en hafif değer.
  danger:
    "bg-transparent text-danger border border-danger/70 hover:bg-danger-soft",
};

const buttonSizes: Record<ButtonSize, string> = {
  sm: "h-7 px-2.5 text-[12px] gap-1.5",
  md: "h-8 px-3 text-[13px] gap-1.5",
};

export function Button({
  children,
  variant = "secondary",
  size = "md",
  icon,
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: React.ReactNode;
}) {
  return (
    <button
      {...props}
      className={`inline-flex items-center justify-center rounded-lg font-medium whitespace-nowrap transition-colors duration-150 disabled:pointer-events-none disabled:opacity-60 ${buttonVariants[variant]} ${buttonSizes[size]} ${className}`}
    >
      {icon}
      {children}
    </button>
  );
}

// ─── Rozet ve durum ─────────────────────────────────────────────────────────

type Tone = "neutral" | "info" | "success" | "warning" | "danger" | "accent";

const badgeTones: Record<Tone, string> = {
  neutral: "bg-raised text-ink-2 border-line",
  info: "bg-info-soft text-info border-info/25",
  success: "bg-ok-soft text-ok border-ok/25",
  warning: "bg-warn-soft text-warn border-warn/25",
  danger: "bg-danger-soft text-danger border-danger/25",
  accent: "bg-accent-soft text-accent border-accent/25",
};

export function Badge({
  children,
  tone = "neutral",
  title,
}: {
  children: React.ReactNode;
  tone?: Tone;
  title?: string;
}) {
  return (
    <span
      title={title}
      className={`inline-flex items-center rounded-md border px-1.5 py-[1px] text-[11px] font-medium whitespace-nowrap ${badgeTones[tone]}`}
    >
      {children}
    </span>
  );
}

const dotTones: Record<Tone, string> = {
  neutral: "bg-ink-3",
  info: "bg-info",
  success: "bg-ok",
  warning: "bg-warn",
  danger: "bg-danger",
  accent: "bg-accent",
};

/** Durum noktası. pulse, sürmekte olan bir iş için. */
export function StatusDot({
  tone = "neutral",
  pulse = false,
}: {
  tone?: Tone;
  pulse?: boolean;
}) {
  return (
    <span
      className={`inline-block size-1.5 shrink-0 rounded-full ${dotTones[tone]} ${
        pulse ? "animate-pulse-dot" : ""
      }`}
    />
  );
}

// ─── Form ───────────────────────────────────────────────────────────────────

// Girdi kenarı da bir DENETİM sınırıdır: `border-line` ile beyaz kart üzerinde
// 1,31:1 ölçülüyordu — kutunun nerede olduğu görünmüyordu.
const fieldBase =
  "w-full rounded-lg border border-control-line bg-canvas px-2.5 py-1.5 text-[13px] outline-none transition-colors duration-150 placeholder:text-ink-3 hover:border-ink-2 focus:border-accent disabled:opacity-50";

export function Input({
  className = "",
  ...props
}: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`${fieldBase} ${className}`} />;
}

export function Textarea({
  className = "",
  ...props
}: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`${fieldBase} ${className}`} />;
}

export function Select({
  className = "",
  children,
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select {...props} className={`${fieldBase} cursor-pointer ${className}`}>
      {children}
    </select>
  );
}

/** Etiketli form alanı. */
export function Label({
  children,
  hint,
  className = "",
}: {
  children: React.ReactNode;
  hint?: string;
  className?: string;
}) {
  return (
    <label className={`block ${className}`}>
      <span className="mb-1 block text-[11px] font-medium tracking-wide text-ink-2 uppercase">
        {children}
        {hint && (
          <span className="ml-1.5 font-normal normal-case opacity-70">{hint}</span>
        )}
      </span>
    </label>
  );
}

export function Checkbox({
  label,
  checked,
  onChange,
  disabled,
}: {
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <label
      className={`inline-flex items-center gap-2 text-[13px] ${
        disabled ? "opacity-50" : "cursor-pointer"
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        className="size-3.5 accent-accent"
      />
      {label}
    </label>
  );
}

// ─── Bilgi kutuları ─────────────────────────────────────────────────────────

const noticeTones: Record<"neutral" | "info" | "warning" | "error", string> = {
  neutral: "border-line bg-raised text-ink-2",
  info: "border-info/25 bg-info-soft text-ink",
  warning: "border-warn/30 bg-warn-soft text-ink",
  error: "border-danger/30 bg-danger-soft text-ink",
};

export function Notice({
  tone = "neutral",
  title,
  children,
}: {
  tone?: "neutral" | "info" | "warning" | "error";
  title?: string;
  children?: React.ReactNode;
}) {
  return (
    <div
      className={`rounded-lg border px-3.5 py-2.5 text-[13px] leading-relaxed ${noticeTones[tone]}`}
    >
      {title && <p className="font-medium">{title}</p>}
      {children && <div className={title ? "mt-0.5" : ""}>{children}</div>}
    </div>
  );
}

/** Liste boşken gösterilen, ne yapılacağını söyleyen blok. */
export function EmptyState({
  icon,
  title,
  description,
  action,
}: {
  icon?: React.ReactNode;
  title: string;
  description?: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <div className="rounded-card border border-dashed border-line-strong bg-surface/40 px-6 py-12 text-center">
      {icon && (
        <div className="mx-auto mb-3 flex size-10 items-center justify-center rounded-full bg-raised text-ink-3">
          {icon}
        </div>
      )}
      <p className="text-[14px] font-medium">{title}</p>
      {description && (
        <p className="mx-auto mt-1.5 max-w-md text-[13px] leading-relaxed text-ink-2">
          {description}
        </p>
      )}
      {action && <div className="mt-4 flex justify-center">{action}</div>}
    </div>
  );
}

/** Yüklenirken gösterilen iskelet satırlar. */
export function Skeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div className="space-y-2.5">
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          className="h-[72px] animate-pulse rounded-card border border-line bg-surface"
        />
      ))}
    </div>
  );
}

// ─── Veri gösterimi ─────────────────────────────────────────────────────────

/** Etiket + değer çifti. */
export function Field({
  label,
  value,
  mono = false,
  className = "",
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
  className?: string;
}) {
  return (
    <div className={`min-w-0 ${className}`}>
      <dt className="text-[11px] tracking-wide text-ink-3 uppercase">{label}</dt>
      <dd
        className={`mt-0.5 truncate text-[13px] ${mono ? "font-mono text-[12px]" : ""}`}
      >
        {value}
      </dd>
    </div>
  );
}

/** Tek satırlık kod/kimlik gösterimi. */
export function Mono({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <code
      className={`rounded bg-raised px-1.5 py-0.5 font-mono text-[12px] text-ink-2 ${className}`}
    >
      {children}
    </code>
  );
}

// ─── Yardımcılar ────────────────────────────────────────────────────────────

/** Tarihi okunabilir yerel biçime çevirir. */
export function formatDate(iso: string | null): string {
  if (!iso) return "hiç";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "bilinmiyor";
  return d.toLocaleString("tr-TR", { dateStyle: "medium", timeStyle: "short" });
}

/** "3 dakika önce" biçiminde göreli zaman. */
export function formatRelative(iso: string | null): string {
  if (!iso) return "hiç";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "bilinmiyor";

  const seconds = Math.round((Date.now() - d.getTime()) / 1000);
  if (seconds < 60) return "az önce";

  const units: [number, Intl.RelativeTimeFormatUnit][] = [
    [60, "minute"],
    [3600, "hour"],
    [86400, "day"],
  ];

  const rtf = new Intl.RelativeTimeFormat("tr", { numeric: "auto" });
  for (let i = units.length - 1; i >= 0; i--) {
    const [size, unit] = units[i]!;
    if (seconds >= size) return rtf.format(-Math.floor(seconds / size), unit);
  }
  return rtf.format(-Math.floor(seconds / 60), "minute");
}
