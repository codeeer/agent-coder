import type { Metadata } from "next";
import { AppShell } from "@/components/AppShell";
import { QueryProvider } from "@/lib/query-provider";
import { themeBootstrapScript } from "@/lib/theme";
import "./globals.css";

export const metadata: Metadata = {
  title: "Agent Coder",
  description:
    "Kod yazan agent'ları akışlara bağlayıp çalıştıran platform",
};

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
      </head>
      <body className="min-h-screen">
        <QueryProvider>
          <AppShell>{children}</AppShell>
        </QueryProvider>
      </body>
    </html>
  );
}
