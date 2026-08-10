"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ThemeToggle } from "@/components/ThemeToggle";
import {
  IconAgent,
  IconBook,
  IconPlay,
  IconReport,
  IconChip,
  IconDashboard,
  IconFolder,
  IconSettings,
  IconWorkflow,
} from "@/components/ui/icons";

const NAV = [
  { href: "/", label: "Panel", Icon: IconDashboard },
  { href: "/workflows", label: "Akışlar", Icon: IconWorkflow },
  { href: "/projects", label: "Projeler", Icon: IconFolder },
  { href: "/agents", label: "Agent'lar", Icon: IconAgent },
  { href: "/runs", label: "Çalıştırmalar", Icon: IconPlay },
  { href: "/reports", label: "Rapor", Icon: IconReport },
  { href: "/models", label: "Modeller", Icon: IconChip },
  { href: "/settings", label: "Ayarlar", Icon: IconSettings },
  { href: "/nasil-calisir", label: "Nasıl çalışır", Icon: IconBook },
] as const;

export function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname();

  return (
    <aside className="flex h-full w-[216px] shrink-0 flex-col border-r border-line bg-surface">
      <div className="flex h-14 items-center gap-2.5 border-b border-line px-4">
        <Logo />
        <div className="leading-none">
          <div className="text-[13px] font-semibold tracking-[-0.01em]">
            Agent Coder
          </div>
          <div className="mt-1 text-[10px] tracking-wide text-ink-3 uppercase">
            kontrol paneli
          </div>
        </div>
      </div>

      <nav className="flex-1 space-y-0.5 p-2.5">
        {NAV.map(({ href, label, Icon }) => {
          // Kök yol yalnızca tam eşleşmede aktif; diğerleri alt yolları da kapsar.
          const active =
            href === "/" ? pathname === "/" : pathname.startsWith(href);

          return (
            <Link
              key={href}
              href={href}
              onClick={onNavigate}
              aria-current={active ? "page" : undefined}
              className={`group relative flex items-center gap-2.5 rounded-lg px-2.5 py-[7px] text-[13px] transition-colors duration-150 ${
                active
                  ? "bg-accent-soft font-medium text-accent"
                  : "text-ink-2 hover:bg-raised hover:text-ink"
              }`}
            >
              {/* Aktif menüyü soldaki şerit de işaretler — renk körlüğünde de okunur. */}
              {active && (
                <span className="absolute top-1/2 left-0 h-4 w-[3px] -translate-y-1/2 rounded-r-full bg-accent" />
              )}
              <Icon className="size-[15px] shrink-0" />
              {label}
            </Link>
          );
        })}
      </nav>

      <div className="border-t border-line p-3">
        <ThemeToggle />
      </div>
    </aside>
  );
}

function Logo() {
  return (
    <div className="flex size-7 shrink-0 items-center justify-center rounded-[7px] bg-accent text-accent-ink">
      <svg
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="size-[15px]"
        aria-hidden="true"
      >
        <path d="m8 8-4 4 4 4" />
        <path d="m16 8 4 4-4 4" />
      </svg>
    </div>
  );
}
