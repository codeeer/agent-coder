import type { Metadata } from "next";
import { AppShell } from "@/components/AppShell";
import { QueryProvider } from "@/lib/query-provider";
import { apiConfigScript, serverApiUrl } from "@/lib/runtime-config";
import { themeBootstrapScript } from "@/lib/theme";
import "./globals.css";

export const metadata: Metadata = {
  title: "Agent Coder",
  description:
    "Kod yazan agent'ları akışlara bağlayıp çalıştıran platform",
};

/*
 * force-dynamic: API adresi HER İSTEKTE ortamdan okunmalı.
 *
 * Bu satır olmadan Next bu düzeni derleme anında önceden üretir ve o anki adresi
 * HTML'e sabitler — yayınlanan imaj yine tek bir adrese mühürlü kalırdı. Yani
 * burası, hazır imajın taşınabilirliğinin dayandığı satır.
 *
 * Bedeli düşük: ekranların neredeyse tamamı zaten istemci bileşeni ve veriyi
 * çalışma anında çekiyor; önceden üretilecek bir şey yok.
 */
export const dynamic = "force-dynamic";

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    // suppressHydrationWarning: aşağıdaki betik data-theme'i React'tan ÖNCE
    // yazar, dolayısıyla sunucunun ürettiği HTML ile istemcininki bu öznitelikte
    // kasıtlı olarak ayrışır.
    <html lang="tr" suppressHydrationWarning>
      <head>
        {/* Temayı ilk boyamadan önce uygular; yanlış temanın bir an görünüp
            sıçramasını (FOUC) engeller. */}
        <script dangerouslySetInnerHTML={{ __html: themeBootstrapScript }} />
        {/* API adresini çalışma anında yazar. İlk isteği atan kodtan önce
            çalışması gerektiği için burada, <head> içinde. */}
        <script
          dangerouslySetInnerHTML={{ __html: apiConfigScript(serverApiUrl()) }}
        />
      </head>
      <body className="min-h-screen">
        <QueryProvider>
          <AppShell>{children}</AppShell>
        </QueryProvider>
      </body>
    </html>
  );
}
