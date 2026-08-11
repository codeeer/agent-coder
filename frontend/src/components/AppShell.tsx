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
    /*
      Kabuk EKRAN YÜKSEKLİĞİNDE ve belgenin kendisi kaymaz.

      Öncesinde `min-h-screen` idi ve kaydırma belgeye aitti: uzun bir liste
      sayfayı uzatıyor, sayfalama denetimi de listenin ARDINDAN geliyordu —
      yani sayfalar arasında gezinmek için önce listenin sonuna kadar
      kaydırmak gerekiyordu. Kaydırmak ile sayfalamak aynı işi iki yoldan
      yapınca ikisi de yarım kalıyor.

      Şimdi kaydırma içerik bölgesine ait. Liste ekranları bunun üstüne
      kendi düzenini kuruyor: başlık ve araç çubuğu üstte sabit, liste
      ortada kayan bölge, sayfalama altta — her zaman görünür, sağ alt
      köşede (referans tasarımdaki yeri).
    */
    <div className="flex h-screen overflow-hidden">
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

      <main className="flex min-w-0 flex-1 flex-col overflow-hidden">
        {/* Dar ekranda üst çubuk: menüye ulaşmanın tek yolu bu. */}
        <div className="sticky top-0 z-30 flex h-14 items-center gap-3 border-b border-line bg-surface px-4 lg:hidden">
          <button
            type="button"
            onClick={() => setOpen(true)}
            aria-label="Menüyü aç"
            aria-expanded={open}
            className="-ml-1.5 flex size-9 items-center justify-center rounded-lg text-ink-2 transition-colors hover:bg-raised hover:text-ink"
          >
            {open ? <IconClose className="size-4" /> : <IconMenu className="size-4" />}
          </button>
          <span className="text-sm font-semibold tracking-[-0.01em]">Agent Coder</span>
        </div>

        {/*
          İçerik ORTALANMIYOR — sola yaslı.

          Önce `mx-auto max-w-5xl`, sonra `mx-auto max-w-7xl` idi. İkisinde de
          asıl sorun `max-w` değil `mx-auto`ydu: içerik ortalanınca kenar
          çubuğu ile içeriğin başı arasında koca bir oluk kalıyordu. 1920px'lik
          bir ekranda bu oluk 240px'e çıkıyor ve sayfa, kenar çubuğuna bağlı
          bir uygulama gibi değil, ortada yüzen bir kutu gibi duruyor.

          Kenar çubuğu zaten sol kenarı tutuyor; içerik onun hemen devamında
          başlamalı. Üst sınır yalnızca çok geniş ekranlarda satırların
          okunamayacak kadar uzamasını engellemek için var, ortalamak için
          değil — bu yüzden `mx-auto` YOK.
        */}
        {/*
          Kaydıran bölge BURASI, belge değil.

          İçteki sarmalayıcı `h-full`: kendi içeriği ekrana sığan sayfalar
          (liste ekranları) böylece KESİN bir yüksekliğe sahip oluyor ve
          "üstte başlık, ortada kayan liste, altta sayfalama" düzenini
          kurabiliyor. Sığmayan sayfalarda (ayarlar, çalıştırma detayı)
          içerik taşar ve dıştaki bölge kayar — yani hiçbir şey kırpılmaz.
        */}
        {/*
          Dolgu KAYAN KABIN kendisinde, içindeki sarmalayıcıda değil.

          Sarmalayıcıda olsaydı: kabuk artık tam yükseklikte ve sarmalayıcı
          `h-full`, yani alt dolgusu 100% yüksekliğin dibinde durur —
          içeriği taşan uzun sayfalarda (belgeler, ayarlar) taşan kısım o
          dolgunun ALTINDA kalır ve sayfanın sonu ekranın kenarına yapışır.
          Kaydırma kabının kendi alt dolgusu ise taşmaya dahil edilir.
        */}
        <div className="flex-1 overflow-y-auto px-5 py-6 sm:px-6 lg:px-8 lg:py-7">
          <div className="flex h-full max-w-[1680px] flex-col">{children}</div>
        </div>
      </main>
    </div>
  );
}
