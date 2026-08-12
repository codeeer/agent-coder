"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { SystemStatus } from "@/components/SystemStatus";
import {
  IconLogo,
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

/**
 * Gezinme — düz liste değil, gruplu.
 *
 * Dokuz öğe tek bir yığın halinde duruyordu ve hepsi aynı düzeydeydi:
 * "Panel" ile "Nasıl çalışır" arasında görsel bir fark yoktu, dolayısıyla
 * menüde bir şey ararken dokuzunu da okumak gerekiyordu.
 *
 * Gruplar kullanıcının işine göre: ÇALIŞMA (akış kur, çalıştır, sonuca bak),
 * TANIMLAR (agent'ların üzerinde çalıştığı şeyler), SİSTEM. "Panel" bilerek
 * grupsuz — giriş noktası, bir kategorinin üyesi değil.
 */
/*
 * `as const` şart: Next'in tipli yolları (`typedRoutes`) `href` için düz
 * `string` kabul etmiyor, literal bekliyor. Bu yüzden grup tipini elle
 * yazmak yerine çıkarıma bırakılıyor — ilk grupta `label: undefined`
 * bilerek duruyor ki her grup aynı alanları taşısın ve `group.label`
 * erişimi birleşim tipinde geçerli olsun.
 */
const NAV = [
  { label: undefined, items: [{ href: "/", label: "Panel", Icon: IconDashboard }] },
  {
    label: "Çalışma",
    items: [
      { href: "/workflows", label: "Akışlar", Icon: IconWorkflow },
      { href: "/runs", label: "Çalıştırmalar", Icon: IconPlay },
      { href: "/reports", label: "Raporlar", Icon: IconReport },
    ],
  },
  {
    label: "Tanımlar",
    items: [
      { href: "/projects", label: "Projeler", Icon: IconFolder },
      { href: "/agents", label: "Agent'lar", Icon: IconAgent },
      { href: "/models", label: "Modeller", Icon: IconChip },
    ],
  },
  {
    label: "Sistem",
    items: [
      { href: "/settings", label: "Ayarlar", Icon: IconSettings },
      { href: "/nasil-calisir", label: "Nasıl çalışır", Icon: IconBook },
    ],
  },
] as const;

export function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  const pathname = usePathname();

  return (
    <aside className="flex h-full w-[216px] shrink-0 flex-col border-r border-line bg-surface">
      {/* "KONTROL PANELİ" alt satırı kaldırıldı: kullanıcıya hiçbir şey
          söylemiyordu ve ürün adının altında ikinci bir metin kademesi
          açıyordu. Kelime markası tek başına duruyor. */}
      <div className="flex h-14 items-center gap-2.5 border-b border-line px-4">
        <Logo />
        <div className="text-sm font-semibold tracking-[-0.01em]">
          Agent Coder
        </div>
      </div>

      <nav className="flex-1 overflow-y-auto p-2.5">
        {NAV.map((group, gi) => (
          <div key={group.label ?? "kok"} className={gi > 0 ? "mt-5" : ""}>
            {group.label && (
              <div className="mb-1 px-2.5 text-2xs font-medium tracking-wide text-ink-3 uppercase">
                {group.label}
              </div>
            )}
            <div className="space-y-0.5">
              {group.items.map(({ href, label, Icon }) => {
                // Kök yol yalnızca tam eşleşmede aktif; diğerleri alt yolları da kapsar.
                const active =
                  href === "/" ? pathname === "/" : pathname.startsWith(href);

                return (
                  <Link
                    key={href}
                    href={href}
                    onClick={onNavigate}
                    aria-current={active ? "page" : undefined}
                    /* py-[7px] (≈34px yükseklik) idi. Kenar çubuğu dar
                       ekranda çekmeceye dönüşüyor, yani bunlar parmakla
                       dokunulan hedefler; 34px rahat dokunma için küçük.
                       py-2 ile 36px'e çıkıyor — masaüstünde de fark
                       edilmeyecek kadar küçük bir artış. */
                    className={`group relative flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm transition-colors duration-150 ${
                      active
                        ? "bg-accent-soft font-medium text-accent"
                        : "text-ink-2 hover:bg-raised hover:text-ink"
                    }`}
                  >
                    {/* Aktif menüyü soldaki şerit de işaretler — renk körlüğünde de okunur. */}
                    {active && (
                      <span className="absolute top-1/2 left-0 h-4 w-0.75 -translate-y-1/2 rounded-r-full bg-accent" />
                    )}
                    <Icon className="size-4 shrink-0" />
                    {label}
                  </Link>
                );
              })}
            </div>
          </div>
        ))}
      </nav>

      {/*
        Dipte tema anahtarı DEĞİL sistem durumu duruyor.

        Tema, arayüzün her ekranda aynı yerde bulunması gereken bir ayarı ve
        yeri sayfa başlığının sağ ucu (bkz. `PageHeader`). Kenar çubuğunun
        dibi ise referans tasarımda sistemin sağlığına ayrılmış — ve orası
        onun doğru yeri: sürekli görünür, hiçbir işi bölmez.
      */}
      <div className="border-t border-line">
        <SystemStatus />
      </div>
    </aside>
  );
}

/**
 * Kelime markası simgesi.
 *
 * Elle yazılmış bir `</>` SVG'siydi (2,2 kalınlıkta — arayüzdeki hiçbir
 * ikonla aynı değil). Özel çizilmiş bir marka işareti olmadığı, aynı glifin
 * lucide'da `CodeXml` olarak zaten bulunduğu için elde tutmanın gerekçesi
 * yoktu.
 */
function Logo() {
  return (
    <div className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-ink">
      <IconLogo className="size-4" />
    </div>
  );
}
