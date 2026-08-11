"use client";

import { useEffect, useState } from "react";
import { applyMode, readMode, writeMode, type ThemeMode } from "@/lib/theme";
import { IconMoon, IconMonitor, IconSun } from "@/components/ui/icons";

/**
 * Tema anahtarı — sistem / açık / koyu.
 *
 * "Sistem" ayrı bir seçenek olarak duruyor, yalnızca açık-koyu ikilisi değil:
 * kullanıcıların çoğu işletim sistemiyle birlikte değişmesini ister ve bu
 * seçenek olmazsa bir kez seçtikleri tema sonsuza kadar sabit kalır.
 */
const OPTIONS: Array<{
  mode: ThemeMode;
  label: string;
  Icon: (p: { className?: string }) => React.ReactElement;
}> = [
  { mode: "system", label: "Sistem", Icon: IconMonitor },
  { mode: "light", label: "Açık", Icon: IconSun },
  { mode: "dark", label: "Koyu", Icon: IconMoon },
];

export function ThemeToggle() {
  // Sunucuda localStorage yok; ilk çizim her zaman "system" ile yapılır ve
  // bağlanmadan sonra düzeltilir. Sayfanın RENKLERİ bundan etkilenmez —
  // onları `<head>` içindeki betik zaten boyamadan önce ayarladı.
  const [mode, setMode] = useState<ThemeMode>("system");

  useEffect(() => setMode(readMode()), []);

  // "Sistem" seçiliyken işletim sistemi teması değişirse arayüz takip etmeli.
  useEffect(() => {
    if (mode !== "system") return;

    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyMode("system");
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [mode]);

  function choose(next: ThemeMode) {
    setMode(next);
    writeMode(next);
  }

  return (
    /*
     * Kompakt: `flex` + `flex-1` ile kenar çubuğunun tam genişliğini
     * kaplıyordu. Nadiren dokunulan bir tercih, en sık kullanılan menü
     * öğesiyle aynı görsel ağırlıktaydı. Artık içeriği kadar yer tutuyor.
     */
    <div
      role="group"
      aria-label="Tema"
      className="inline-flex overflow-hidden rounded-lg border border-line"
    >
      {OPTIONS.map(({ mode: m, label, Icon }, i) => {
        const active = mode === m;
        return (
          <button
            key={m}
            type="button"
            onClick={() => choose(m)}
            aria-pressed={active}
            title={label}
            className={`flex size-8 items-center justify-center transition-colors duration-150 ${
              i > 0 ? "border-l border-line" : ""
            } ${
              active
                ? "bg-accent-soft text-accent"
                : "text-ink-3 hover:bg-raised hover:text-ink"
            }`}
          >
            <Icon className="size-4" />
            <span className="sr-only">{label}</span>
          </button>
        );
      })}
    </div>
  );
}
