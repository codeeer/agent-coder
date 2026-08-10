"use client";

import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { Sidebar } from "@/components/Sidebar";
import { IconClose, IconMenu } from "@/components/ui/icons";

/**
 * Uygulama kabuğu.
 *
 * Kenar çubuğu geniş ekranda sabit durur, dar ekranda çekmeceye dönüşür.
 * Sabit bırakıldığında 390px'lik bir telefonda içeriğe ~200px kalıyordu ve
 * her satır iki kelimeye bölünüyordu — sayfa açılıyordu ama kullanılamıyordu.
 *
 * Çekmece kapanma yolları bilinçli olarak çok: bağlantıya tıklama (yol
 * değişimi), örtüye tıklama, Esc. Kullanıcı bir çıkış arıyorsa ilk denediği
 * şey çalışmalı.
 */
export function AppShell({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = useState(false);
  const pathname = usePathname();

  // Yol değişince çekmece kapanır: menüden bir sayfaya gidip çekmecenin açık
  // kalması, kullanıcının gittiği sayfayı görmemesi demekti.
  useEffect(() => setOpen(false), [pathname]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open]);

  return (
    <div className="flex min-h-screen">
      {/* Geniş ekran: her zaman görünür. */}
      <div className="hidden lg:flex">
        <Sidebar />
      </div>

      {/* Dar ekran: çekmece. `hidden` yerine kaydırma — açılışı ani değil. */}
      <div
        className={`fixed inset-0 z-40 lg:hidden ${open ? "" : "pointer-events-none"}`}
        aria-hidden={!open}
      >
        <button
          type="button"
          tabIndex={open ? 0 : -1}
          aria-label="Menüyü kapat"
          onClick={() => setOpen(false)}
          className={`absolute inset-0 bg-black/40 transition-opacity duration-200 ${
            open ? "opacity-100" : "opacity-0"
          }`}
        />
        <div
          className={`absolute inset-y-0 left-0 flex transition-transform duration-200 ${
            open ? "translate-x-0" : "-translate-x-full"
          }`}
        >
          <Sidebar onNavigate={() => setOpen(false)} />
        </div>
      </div>

      <main className="min-w-0 flex-1">
        {/* Dar ekranda üst çubuk: menüye ulaşmanın tek yolu bu. */}
        <div className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-line bg-surface px-4 lg:hidden">
          <button
            type="button"
            onClick={() => setOpen(true)}
            aria-label="Menüyü aç"
            aria-expanded={open}
            className="-ml-1.5 flex size-9 items-center justify-center rounded-lg text-ink-2 transition-colors hover:bg-raised hover:text-ink"
          >
            {open ? <IconClose className="size-5" /> : <IconMenu className="size-5" />}
          </button>
          <span className="text-[13px] font-semibold tracking-[-0.01em]">Agent Coder</span>
        </div>

        <div className="mx-auto max-w-5xl px-4 py-6 sm:px-6 lg:px-8 lg:py-8">{children}</div>
      </main>
    </div>
  );
}
