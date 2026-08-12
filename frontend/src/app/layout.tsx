import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { AppShell } from "@/components/AppShell";
import { QueryProvider } from "@/lib/query-provider";
import {
  apiConfigScript,
  serverApiUrl,
  serverAppName,
} from "@/lib/runtime-config";
import { themeBootstrapScript } from "@/lib/theme";
import "./globals.css";

/*
 * Arayüz yazı tipi.
 *
 * `latin-ext` alt kümesi ZORUNLU: Türkçenin ğ, ş, ı ve İ harfleri Latin
 * Extended-A bloğunda. Yalnızca `latin` ile bu dört harf başka bir yazı
 * tipine düşer ve satır ortasında kesim değişir.
 *
 * `next/font` yazı tipini DERLEME ANINDA indirip uygulamanın kendi altına
 * koyar; çalışma anında Google'a istek gitmez. Bu yüzden ne yeni bir paket
 * ne de dış bağlantı gerekiyor.
 *
 * `display: swap`: yazı tipi inene kadar metin görünmez kalmaz.
 */
const inter = Inter({
  subsets: ["latin", "latin-ext"],
  display: "swap",
  variable: "--font-inter",
});

/*
 * Başlık sabit DEĞİL: ürün adı `APP_NAME` ile değiştirilebiliyor ve sekmede de
 * onun görünmesi gerekiyor. `metadata` sabiti bunu yapamaz — modül yüklenirken
 * bir kez okunur; `generateMetadata` ise `force-dynamic` sayesinde her istekte
 * çalışır.
 *
 * `template`, alt sayfaların markayı kendilerinin yazmasını gereksiz kılar:
 * "Nasıl çalışır" başlığı sekmede "Nasıl çalışır · <ad>" olarak çıkar.
 */
export function generateMetadata(): Metadata {
  const ad = serverAppName();
  return {
    title: { default: ad, template: `%s · ${ad}` },
    description: "Kod yazan agent'ları akışlara bağlayıp çalıştıran platform",
  };
}

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
    <html lang="tr" className={inter.variable} suppressHydrationWarning>
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
          {/* Ürün adı prop olarak iniyor, sayfaya betikle yazılmıyor.
              Kenar çubuğu bir istemci bileşeni: `window`'dan okusaydı sunucu
              varsayılanı, istemci gerçek adı basar ve metin hydration'da
              uyuşmazdı. */}
          <AppShell appName={serverAppName()}>{children}</AppShell>
        </QueryProvider>
      </body>
    </html>
  );
}
