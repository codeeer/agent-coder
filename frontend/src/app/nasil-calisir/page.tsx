import type { Metadata } from "next";
import {
  Architecture,
  DataModel,
  Parallelism,
  MCPDirections,
  ProductFlow,
  ScriptDeterminism,
  StepLifecycle,
  TriggerPaths,
} from "@/components/docs/diagrams";
import { Card, PageHeader } from "@/components/ui/primitives";

export const metadata: Metadata = {
  title: "Nasıl çalışır · Agent Coder",
};

/**
 * "Nasıl çalışır" — sistemi anlatan tek sayfa.
 *
 * Bu sayfa SUNUM İÇİN: birine ekranı gösterip anlatırken kullanılacak. Bu yüzden
 * her bölüm bir diyagramla açılır ve metin diyagramı tekrar etmez, tamamlar.
 * Uzun paragraf yok; anlatan kişi konuşacak, ekran ona eşlik edecek.
 *
 * İstemci tarafı durumu yok: veri çekmiyor, etkileşim taşımıyor. Sunucu bileşeni
 * olarak kalması hem daha hızlı açılıyor hem de yansıtırken bir yükleme anı
 * göstermiyor.
 */
export default function HowItWorksPage() {
  return (
    <div className="space-y-10">
      <PageHeader
        title="Nasıl çalışır?"
        description="Bir Jira task'ının koda, koddan pull request'e dönüşene kadar izlediği yol."
      />

      <Step
        no={1}
        title="Ne yapar"
        lead="Kod yazan agent'lar tek tek, elle çalıştırılıyor. Bir adımın çıktısını diğerine taşımak kopyala-yapıştır. Agent Coder bunu bir akışa çeviriyor."
      >
        <ProductFlow />
        <Note>
          Adımlar birbirine bağlı, her adım <b>kendi modeliyle</b> çalışır: analiz için ucuz ve hızlı
          bir model, kod yazımı için güçlü bir model. Aradaki fark hem faturada hem sürede görünür.
        </Note>
      </Step>

      <Step
        no={2}
        title="Parçalar ve sınırlar"
        lead="Üç servis ve bir de iş başına açılıp kapanan geçici container'lar. Kuyruk, mesaj aracısı, ayrı bir işçi servisi yok."
      >
        <Architecture />
        <div className="grid gap-3 sm:grid-cols-3">
          <Mini title="Tuval">
            Akışı çizersiniz. Hangi adım hangi adıma bağlı, hangi model, hangi talimat.
          </Mini>
          <Mini title="Motor">
            Grafı seviyelere ayırır, sırayı ve paralelliği belirler, her adımı kaydeder.
          </Mini>
          <Mini title="Sandbox">
            Agent&apos;ı kendi container&apos;ında çalıştırır. Kod dışarı sızmaz, kimlik bilgisi içeri girmez.
          </Mini>
        </div>
      </Step>

      <Step
        no={3}
        title="Bir adımın ömrü"
        lead="Her agent adımı sıfırdan başlar: temiz bir container, temiz bir klon. Önceki adımdan yalnızca metin geçer."
      >
        <StepLifecycle />
        <Note>
          Son adım pazarlık konusu değil: zaman aşımı, iptal ya da sunucunun yeniden başlaması —
          hangisi olursa olsun container ve volume siliniyor. Doğrulaması kolay:
          <code className="mx-1 rounded bg-raised px-1.5 py-0.5 font-mono text-[12px]">docker ps -a</code>
          çalışma sonrası boş olmalı.
        </Note>
      </Step>

      <Step
        no={4}
        title="Neden hızlı"
        lead="Motor adımları düz bir sıraya dizmiyor; grafı seviyelere ayırıyor. Birbirini beklemeyen adımlar aynı anda koşuyor."
      >
        <Parallelism />
        <Note>
          Ölçüldü: tuvalden kurulan iki dallı bir akışta iki adım <b>10 saniye örtüştü</b>. Sıraya
          dizilseydi toplam süre ikisinin toplamı olurdu.
        </Note>
      </Step>

      <Step
        no={5}
        title="Jira'dan kendiliğinden başlaması"
        lead="İki giriş yolu var: düzenli JQL taraması ve Jira webhook'u. İkisi de aynı kapıdan geçiyor."
      >
        <TriggerPaths />
        <Note>
          Aynı task&apos;ın iki kez işlenmemesi <b>veritabanı kısıtıyla</b> garanti ediliyor. Uygulama
          içinde &quot;önce sor, sonra yaz&quot; biçiminde bir kontrol olsaydı iki yol aynı anda gelince
          ikisi de &quot;işlenmemiş&quot; görür ve akış iki kez başlardı.
        </Note>
      </Step>

      <Step
        no={6}
        title="Dış araçlar — iki yön"
        lead="Bir agent yalnızca klonlanmış depoyla çalışırsa çok şey bilmez. MCP, standart bir protokolle dış kaynaklara bağlanmayı sağlıyor — ve aynı protokol ters yönde de işliyor."
      >
        <MCPDirections />
        <Note>
          Üç kullanım aynı protokolün üç yüzü. <b>Agent karar verirse</b> esnektir:
          &quot;bu hatayı düzelt&quot; dediğinizde yığın izini kendisi çeker.{" "}
          <b>Akış karar verirse</b> tekrarlanabilir: her çalıştırmada aynı araç, aynı
          argümanlarla. <b>Ters yönde</b> ise Agent Coder başka bir agent&apos;ın aracı
          olur — akışlarınız Claude Desktop&apos;tan tetiklenebilir.
        </Note>
      </Step>

      <Step
        no={7}
        title="Standart işte standart sonuç"
        lead="Model her seferinde yeniden karar verir. Keşifte bu doğru; prosedürde risk. Betikler bu ikisini ayırıyor."
      >
        <ScriptDeterminism />
        <Note>
          Ayarlar&apos;da bir <b>betik kütüphanesi</b> var: bir kez yazarsınız, birden
          fazla agent&apos;a atarsınız. Yeni bir yetki açılmıyor —{" "}
          <b>komut çalıştırma yetkisi zaten açık</b> olan bir agent o betiği bugün de
          kendisi yazıp çalıştırabiliyordu. Değişen tek şey, çalıştırdığı metnin sizin
          gözden geçirdiğiniz metin olması.
        </Note>
      </Step>

      <Step
        no={8}
        title="Neyi nerede saklıyor"
        lead="Kaydedilen graf değişmez. Geçmiş bir çalışma, o gün hangi tanımla koştuysa onu gösterir."
      >
        <DataModel />
        <Note>
          Maliyet hiçbir yerde ikinci kez tutulmuyor: rapordaki rakamlar çalıştırma kayıtlarından
          toplanıyor. Ayrı bir özet tablosu olsaydı iki kaynak olur ve er geç ayrışırlardı.
        </Note>
      </Step>

      <Step
        no={9}
        title="Üç karar"
        lead="Sistemin bugünkü halini belirleyen, geri alınması pahalı olan seçimler."
      >
        <div className="grid gap-3 md:grid-cols-3">
          <Decision title="Agent motoru değiştirilebilir">
            Bugün opencode kullanılıyor ama kod ona değil, tek bir <b>Runner</b> arayüzüne bakıyor.
            Motoru değiştirmek bir paketi değiştirmek demek; akış motoruna dokunulmuyor.
          </Decision>
          <Decision title="Davranış kodda değil veritabanında">
            Süre sınırı, eşzamanlılık, bellek, tarama aralığı — hiçbiri koda gömülü değil. Kod
            yalnızca <b>tanımı</b> tutuyor (varsayılan ve aralık), veritabanı sapmayı. Değişiklik
            yeniden başlatma gerektirmiyor.
          </Decision>
          <Decision title="Adım = çalıştırma">
            Bir akış adımı, elle başlatılan bir çalıştırmayla <b>aynı kaydı</b> yazıyor. Bu yüzden
            rapor, akışları hiçbir kod değişikliği olmadan kapsadı.
          </Decision>
        </div>
      </Step>

      <Step
        no={10}
        title="Güvenlik sınırları"
        lead="Neyin nereye eriştiği bilinçli olarak dar tutuldu."
      >
        <div className="grid gap-3 md:grid-cols-2">
          <Decision title="Anahtarlar şifreli">
            API anahtarları ve token&apos;lar veritabanında AES-256-GCM ile saklanıyor. Hiçbir API
            yanıtında ve hiçbir log satırında görünmüyorlar — testlerle doğrulanıyor. Arayüzde
            yalnızca son dört karakter gösteriliyor.
          </Decision>
          <Decision title="Agent'a token geçmez">
            PR açmak ve Jira&apos;ya yazmak <b>akışın</b> işi, agent&apos;ın değil; git kimlik bilgisi
            agent&apos;a hiç ulaşmaz. Dış araçlarda (MCP) anahtar container&apos;ın ortam
            değişkeninde durur ve isteğe başlık olarak eklenir — <b>modele hiç gösterilmez</b>.
          </Decision>
          <Decision title="Container'lar izole">
            Runner&apos;lar ayrı bir ağda çalışır, dışarıya port açmaz, CPU ve bellek sınırlıdır.
          </Decision>
          <Decision title="Dış araçlar agent başına açılır">
            Bir MCP sunucusu tanımlamak onu her agent&apos;a açmaz. Hangi agent&apos;ın
            kullanabileceği ayrı seçilir; seçilmeyenlere araçlar <b>hiç sunulmaz</b>.
            Bağlanamayan bir sunucu sessiz kalmaz, uyarı üretir.
          </Decision>
          <Decision title="Betikler yeni bir kapı açmaz">
            Bir betik yalnızca <b>komut çalıştırma yetkisi zaten açık</b> agent&apos;a
            kopyalanır; kapalıysa dosya ortama hiç girmez. Yetki kuralları bu özellik
            için değişmedi. &quot;Bash kapalı ama şu betiğe izinli&quot; gibi bir ara mod
            bilinçli olarak <b>yapılmadı</b>: izin eşleşmesi ham komut metnine yapıldığı
            için kapalı bir kapıyı açardı.
          </Decision>
          <Decision title="Dışarıya açılan adres bir anahtardır" tone="warn">
            Agent Coder&apos;ı MCP olarak kullanmanın adresi, bilen herkesin akış
            başlatabileceği anlamına gelir. Ayarlar&apos;dan yenilenebilir. Kimlik
            doğrulama gelene kadar bu adres paylaşılırken dikkat edilmeli.
          </Decision>
          <Decision title="Kimlik doğrulama henüz yok" tone="warn">
            v1 tek kullanıcılıktır ve <b>internete açık bir sunucuda çalıştırılmamalıdır</b>. Şema
            baştan <code className="font-mono text-[12px]">user_id</code> taşıyor; auth sonradan eklenecek.
          </Decision>
        </div>
      </Step>
    </div>
  );
}

/* ── Sayfa parçaları ─────────────────────────────────────────────────────── */

function Step({
  no,
  title,
  lead,
  children,
}: {
  no: number;
  title: string;
  lead: string;
  children: React.ReactNode;
}) {
  return (
    <section className="space-y-4">
      <div className="flex items-start gap-3">
        <span className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full bg-accent text-[13px] font-semibold text-accent-ink">
          {no}
        </span>
        <div className="min-w-0">
          <h2 className="text-[17px] font-semibold tracking-[-0.01em]">{title}</h2>
          <p className="mt-1 max-w-3xl text-[13px] leading-relaxed text-ink-2">{lead}</p>
        </div>
      </div>

      {/* Diyagram kendi kartında: sunumda yansıtılırken çerçevesi olsun.
          Kart kaydırmaz; kaydırma diyagramın kendi kabında (bkz. Frame). */}
      <Card>{children}</Card>
    </section>
  );
}

function Note({ children }: { children: React.ReactNode }) {
  return (
    <p className="mt-4 border-t border-line pt-3 text-[13px] leading-relaxed text-ink-2">
      {children}
    </p>
  );
}

function Mini({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="rounded-lg border border-line bg-raised p-3">
      <p className="text-[13px] font-medium">{title}</p>
      <p className="mt-1 text-[12px] leading-relaxed text-ink-2">{children}</p>
    </div>
  );
}

function Decision({
  title,
  children,
  tone = "neutral",
}: {
  title: string;
  children: React.ReactNode;
  tone?: "neutral" | "warn";
}) {
  return (
    <div
      className={`rounded-lg border p-3.5 ${
        tone === "warn" ? "border-warn/40 bg-warn-soft" : "border-line bg-raised"
      }`}
    >
      <p className="text-[13px] font-medium">{title}</p>
      <p className="mt-1.5 text-[12px] leading-relaxed text-ink-2">{children}</p>
    </div>
  );
}
