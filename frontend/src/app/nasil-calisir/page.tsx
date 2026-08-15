import type { Metadata } from "next";
import {
  Architecture,
  BatchQueue,
  CorporateTLS,
  DataBoundary,
  DataModel,
  EgressControl,
  Parallelism,
  MCPDirections,
  ProductFlow,
  RunnerAnatomy,
  ScriptDeterminism,
  StepLifecycle,
  TriggerPaths,
} from "@/components/docs/diagrams";
import {
  IconBolt,
  IconCost,
  IconEye,
  IconShield,
  IconSparkle,
  IconTrash,
} from "@/components/ui/icons";
import {
  Card,
  IconTile,
  PageHeader,
  type TileTone,
} from "@/components/ui/primitives";

// Marka soneki burada YAZILMAZ: kök düzenin `title.template`'i ekliyor ve o,
// ürün adını `APP_NAME`'den okuyor. Sabit yazılsaydı adı değiştiren kurulumda
// bu tek sekme eski adı göstermeye devam ederdi.
export const metadata: Metadata = {
  title: "Nasıl çalışır",
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
        description="Bir Jira task'ının koda, koddan pull request'e dönüşene kadar izlediği yol — ve kurumsal bir ağda nerede durduğu."
      />

      {/*
        İÇİNDEKİLER — sayfa on iki bölüme çıktı ve sunum sırasında "şu kısma
        atla" demek gerekiyor. Saf çapa bağlantısı: sayfa hâlâ sunucu bileşeni,
        JavaScript taşımıyor.
      */}
      <nav
        aria-label="Sayfa bölümleri"
        className="sticky top-0 z-10 -mx-1 flex flex-wrap gap-1.5 bg-canvas/85 px-1 py-2 backdrop-blur"
      >
        {[
          { id: "urun", label: "Ürün" },
          { id: "mimari", label: "Mimari" },
          { id: "kurumsal", label: "Kurumsal kurulum" },
          { id: "olcek", label: "Ölçek" },
          { id: "baglantilar", label: "Bağlantılar" },
          { id: "kararlar", label: "Kararlar ve sınırlar" },
        ].map((b) => (
          <a
            key={b.id}
            href={`#${b.id}`}
            className="rounded-lg border border-line bg-surface px-2.5 py-1 text-xs text-ink-2 transition-colors hover:border-line-strong hover:text-ink"
          >
            {b.label}
          </a>
        ))}
      </nav>

      <Chapter
        id="urun"
        title="Ürün"
        lead="Ne yaptığı ve neden tek tek çalıştırmaktan hızlı olduğu."
      />


      <Step
        no={1}
        title="Ne yapar"
        lead="Kod yazan agent'lar tek tek, elle çalıştırılıyor. Bir adımın çıktısını diğerine taşımak kopyala-yapıştır. Agent Coder bunu bir akışa çeviriyor."
      >
        <ProductFlow />
        <Note>
          Adımlar birbirine bağlı, her adım <b>kendi modeliyle</b> çalışır:
          analiz için ucuz ve hızlı bir model, kod yazımı için güçlü bir model.
          Aradaki fark hem faturada hem sürede görünür.
        </Note>
      </Step>

      <Step
        no={2}
        title="Neden hızlı"
        lead="Motor adımları düz bir sıraya dizmiyor; grafı seviyelere ayırıyor. Birbirini beklemeyen adımlar aynı anda koşuyor."
      >
        <Parallelism />
        <Note>
          Ölçüldü: tuvalden kurulan iki dallı bir akışta iki adım{" "}
          <b>10 saniye örtüştü</b>. Sıraya dizilseydi toplam süre ikisinin
          toplamı olurdu.
        </Note>
      </Step>

      <Chapter
        id="mimari"
        title="Mimari"
        lead="Üç servis, bir de iş başına açılıp kapanan geçici container'lar."
      />


      {/*
        Beş sıfat şeridi — mimarinin hemen ardında, bilerek.

        Her biri YUKARIDAKİ DİYAGRAMIN bir sonucu, yeni bir iddia değil:
        izole ağ → güvenli, üç servis → ekonomik, seviye seviye paralellik →
        hızlı, adım kayıtları → şeffaf, iş sonunda silme → temiz. Sayfanın
        geri kalanı bunların her birini ayrıntısıyla açıyor; bu şerit
        anlatana bir duraklama noktası veriyor.
      */}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <Trait
          icon={<IconShield className="size-4" />}
          tone="success"
          title="Güvenli"
        >
          İzole ağ, dışarıya port açmaz.
        </Trait>
        <Trait
          icon={<IconCost className="size-4" />}
          tone="info"
          title="Ekonomik"
        >
          Ek altyapı yok, sadece üç servis.
        </Trait>
        <Trait
          icon={<IconBolt className="size-4" />}
          tone="accent"
          title="Hızlı"
        >
          Bağımsız adımlar aynı anda koşar.
        </Trait>
        <Trait
          icon={<IconEye className="size-4" />}
          tone="series"
          title="Şeffaf"
        >
          Girdi, çıktı ve maliyetin yanında motorun ham logu da kayıtlı.
        </Trait>
        <Trait
          icon={<IconTrash className="size-4" />}
          tone="warning"
          title="Temiz"
        >
          İş bitince container ve volume silinir.
        </Trait>
      </div>



      <Step
        no={3}
        title="Parçalar ve sınırlar"
        lead="Üç servis ve bir de iş başına açılıp kapanan geçici container'lar. Kuyruk, mesaj aracısı, ayrı bir işçi servisi yok."
      >
        <Architecture />
        <div className="grid gap-3 sm:grid-cols-3">
          <Mini title="Canvas">
            Akışı çizersiniz. Hangi adım hangi adıma bağlı, hangi model, hangi
            talimat.
          </Mini>
          <Mini title="Engine">
            Grafı seviyelere ayırır, sırayı ve paralelliği belirler, her adımı
            kaydeder.
          </Mini>
          <Mini title="Sandbox">
            Agent&apos;ı kendi container&apos;ında çalıştırır. Kod dışarı
            sızmaz, kimlik bilgisi içeri girmez.
          </Mini>
        </div>
      </Step>

      <Step
        no={4}
        title="Bir adımın ömrü"
        lead="Her agent adımı sıfırdan başlar: temiz bir container, temiz bir klon. Önceki adımdan yalnızca metin geçer."
      >
        <StepLifecycle />
        <Note>
          Son adım pazarlık konusu değil: zaman aşımı, iptal ya da sunucunun
          yeniden başlaması — hangisi olursa olsun container ve volume
          siliniyor. Doğrulaması kolay:
          <code className="mx-1 rounded bg-raised px-1.5 py-0.5 font-mono text-xs">
            docker ps -a
          </code>
          çalışma sonrası boş olmalı.
        </Note>
        <Note>
          <b>Sonuncudan bir önceki adım da pazarlık konusu değil.</b> Container
          silinince motorun teşhis verisi de gider; oysa asıl ihtiyaç tam olarak
          düşen ve zaman aşımına uğrayan koşularda. Bu yüzden loglar{" "}
          <b>silmeden hemen önce</b> toplanıyor: koşu detayındaki{" "}
          <b>Engine logları</b> sekmesinde container&apos;ın çıktısı, motorun
          kendi log dosyaları ve agent&apos;ın tam konuşma geçmişi ayrı ayrı
          duruyor.
        </Note>
        <Note>
          <b>Node sürümü koşudan önce seçilir.</b> Her desteklenen sürüm için
          runner imajı
          <b> build anında</b> hazırlanır; koşu anında hiçbir şey indirilmez.
          Seçilmezse projenin varsayılanı, o da yoksa taban imaj geçerli olur.
          Seçilen sürümün imajı yoksa çalıştırma <b>klonlama başlamadan</b>, ne
          yapılacağını söyleyen bir satırla düşer — yarım iş bırakmaz.
        </Note>
      </Step>

      <Step
        no={5}
        title="Neyi nerede saklıyor"
        lead="Kaydedilen graf değişmez. Geçmiş bir çalışma, o gün hangi tanımla koştuysa onu gösterir."
      >
        <DataModel />
        <Note>
          Maliyet hiçbir yerde ikinci kez tutulmuyor: rapordaki rakamlar
          çalıştırma kayıtlarından toplanıyor. Ayrı bir özet tablosu olsaydı iki
          kaynak olur ve er geç ayrışırlardı.
        </Note>
        <Note>
          <b>Engine logları ayrı yaşar.</b> Çalıştırma kaydı yıllarca değerli
          olabilir; iki megabaytlık ham logu bir haftadan sonra değil. Bu yüzden
          ham log kendi tablosunda, kendi saklama süresiyle duruyor — süresi
          dolunca yalnızca o siliniyor, koşu geçmişi ve maliyet raporu yerinde
          kalıyor. Koşu silinirse logu da onunla gider.
        </Note>
      </Step>

      <Chapter
        id="kurumsal"
        title="Kurumsal kurulum"
        lead="Kapalı ağda, SSL inspection'ın ardında ve verinin kurumdan çıkmasının istenmediği yerde nasıl duruyor."
      />

      <Step
        no={6}
        title="Bir agent adımı nerede çalışıyor"
        lead="Her adım kendi geçici container'ında koşar. İçinde ne var, ne yok ve iş bitince ne kalıyor."
      >
        <RunnerAnatomy />
        <Note>
          Container <b>dışarıya port açmaz</b>, host dosya sistemine bağlanmaz
          ve başka bir çalıştırmanın container&apos;ını görmez. CPU ve bellek
          sınırı ayarlardan gelir; varsayılan iş başına iki çekirdek ve dört GB.
        </Note>
        <Note>
          <b>İş bitince silinir</b> — zaman aşımı, iptal ya da sunucunun yeniden
          başlaması fark etmez. Silmeden hemen önce motorun teşhis verisi
          toplanır: container çıktısı, motorun log dosyaları ve agent&apos;ın
          tam oturum geçmişi. Asıl ihtiyaç düşen koşularda ve o veri
          container&apos;la birlikte gidiyordu.
        </Note>
      </Step>

      <Step
        no={7}
        title="Çıkış denetimi: proxy ve whitelist"
        lead="Denetim ayardan değil AĞDAN geliyor: container internete rotası olmayan bir ağda doğuyor ve dışarıya tek yol çıkış kapısı."
      >
        <EgressControl />
        <Note>
          <b>Neden ortam değişkeni yetmedi:</b> sızıntı ölçümünde{" "}
          <code className="mx-1 rounded bg-raised px-1.5 py-0.5 font-mono text-xs">
            HTTP_PROXY
          </code>
          ile verilen proxy <b>atlanabiliyordu</b> — 26 bağlantının 5&apos;i
          atladı. Bu yüzden proxy değişkeni hâlâ yazılıyor (Java, curl ve npm
          için ayrı ayrı) ama asıl kısıt ağda: rota yoksa atlanacak bir şey de
          yok.
        </Note>
        <Note>
          <b>Kapı TLS açmaz.</b> Yalnızca{" "}
          <code className="font-mono text-xs">CONNECT host:443</code>{" "}
          satırındaki adı görür ve baytları tünneller. Bilinçli bir sınır:
          araya, sağlayıcı anahtarını görebilecek bir nokta koymuyoruz. Bedeli,
          kararın yalnızca <b>host&apos;a</b> dayanması — izinli bir host
          üzerinden sızdırma bu kapının çözebileceği bir şey değil.
        </Note>
        <Note>
          <b>Boş whitelist kısıt yokluğudur.</b> Liste boşken tüm adreslere
          çıkılır. Bu yüzden zorunlu adresler (LLM sağlayıcı, depo, registry,
          MCP) listeye <b>yalnızca liste doluyken</b> ekleniyor: boş bir listeye
          ekleme yapmak, kısıtsız olması gereken bir çalıştırmayı sessizce o dört
          adresle sınırlardı.
        </Note>
      </Step>

      <Step
        no={8}
        title="Veri kurumdan çıkıyor mu"
        lead="Cevap kurulumunuza bağlı ve koşulu net: belirleyici olan modelin nerede çalıştığı."
      >
        <DataBoundary />
        <Note>
          <b>Kurum içi sağlayıcıyla</b> (LiteLLM, vLLM, OpenAI-uyumlu bir servis)
          hiçbir istek kurum ağının dışına çıkmaz: kod, talimat ve model yanıtı
          aynı ağda kalır. OpenRouter zorunlu bir bağımlılık değildir.
        </Note>
        <Note>
          <b>Dış bir sağlayıcıyla</b> model çağrısı kodun bir parçasını dışarı
          taşır. Bunu çıkış kapısı engelleyemez — engellenirse model hiç
          çalışmaz. Dürüst cümle şu: bu bir <b>kurulum kararıdır</b>, bir güvenlik
          ayarı değil. Depoya gönderim ve PR ise her iki kurulumda da{" "}
          <b>akışın</b> işidir; git kimlik bilgisi agent&apos;a hiç ulaşmaz.
        </Note>
      </Step>

      <Step
        no={9}
        title="SSL inspection ve kapalı ağ"
        lead="Kurumsal ağda giden her bağlantı kurumun sertifikasıyla imzalanır. Dört ayrı yerin bunu bilmesi gerekiyor."
      >
        <CorporateTLS />
        <Note>
          Sertifika <b>Ayarlar → Kurumsal ağ</b>&apos;dan girilir; biçimi
          kaydetme anında doğrulanır ve bir sonraki çalıştırmada geçerli olur —
          yeniden başlatma gerekmez. Biri eksik kalırsa o araç sessizce
          &quot;sertifika doğrulanamadı&quot; der; bu yüzden dördü birden
          besleniyor.
        </Note>
        <Note>
          <b>Kapalı ağda kurulum:</b> npm ve Maven kayıt defteri adresi
          ayarlardan verilir, kimlik container&apos;a <b>dosya olarak</b> girer —
          ortam değişkenine değil, çünkü agent{" "}
          <code className="font-mono text-xs">env</code> yazdırabiliyor. Runner
          imajı ve desteklenen Node sürümleri <b>derleme anında</b> hazırlanır;
          koşu sırasında hiçbir şey indirilmez.
        </Note>
      </Step>

      <Chapter
        id="olcek"
        title="Ölçek"
        lead="Bir projede çalışan bir şeyi otuz projede yürütmek — ve otuz projeyi tek tek eklememek."
      />

      <Step
        no={10}
        title="Aynı akış otuz projede"
        lead="Otuz projeyi tek tek tetiklemek otuz tur demek. Hepsini birden başlatmak ise çalışmıyor: sınır dolduğunda çalıştırma reddedilir, sıraya alınmaz."
      >
        <BatchQueue />
        <Note>
          Kuyruk <b>mevcut eşzamanlılık sınırına uyar</b>, kendi paralellik
          ayarını tanımlamaz — &quot;aynı anda kaç iş&quot; sorusunun tek bir
          cevabı olmalı. Bir iş bitince sıradaki kendiliğinden başlar; yoklama
          yok, bitiş olayı kuyruğu uyandırıyor.
        </Note>
        <Note>
          <b>Kuyruk veritabanında yaşar.</b> Otuz projelik bir kampanya saatler
          sürer ve o sürede bir yeniden başlatma olağandır: bekleyenler beklemeye
          devam eder, o an çalışanlar <b>kesildi</b> olarak işaretlenir.
          Kesilenler kendiliğinden tekrarlanmaz — yarım kalmış bir işin yan
          etkisi habersizce tekrarlanmamalı; &quot;kaldığı yerden devam et&quot;
          düğmesi kaç işin sıraya alınacağını üzerinde yazar.
        </Note>
        <Note>
          Bir proje düşerse kuyruk <b>durmaz</b>: sebebi o satırda yazar,
          sıradaki başlar. İptal bekleyenleri düşürür, çalışanlar kendi hâlinde
          sürer ve sonuçları kaydedilir — onay ekranında bu sayılarla yazılıdır.
        </Note>
      </Step>

      <Step
        no={11}
        title="Standart işte standart sonuç"
        lead="Model her seferinde yeniden karar verir. Keşifte bu doğru; prosedürde risk. Betikler bu ikisini ayırıyor."
      >
        <ScriptDeterminism />
        <Note>
          Ayarlar&apos;da bir <b>script kütüphanesi</b> var: bir kez yazarsınız,
          birden fazla agent&apos;a atarsınız. Yeni bir yetki açılmıyor —{" "}
          <b>komut çalıştırma yetkisi zaten açık</b> olan bir agent o betiği
          bugün de kendisi yazıp çalıştırabiliyordu. Değişen tek şey,
          çalıştırdığı metnin sizin gözden geçirdiğiniz metin olması.
        </Note>
        <Note>
          <b>Kampanya klasörleri</b> çok adımlı işler için: yedi adımlık bir
          Node yükseltmesinin adımları bir klasörde toplanır ve agent&apos;a{" "}
          <b>tek seçimle</b> bağlanır. Klasöre sonradan eklediğiniz bir adım o
          agent&apos;ta kendiliğinden geçerli olur — atama tazelenmez, çünkü
          klasörün içeriği çalıştırma anında okunur. Klasör silinirse betikler
          silinmez, klasörsüz kalır.
        </Note>
        <Note>
          Bu ikisi birleşince kampanya olur: <b>bir klasör + otuz proje</b>.
          Gerçek bir ölçüm — beş adımlık bir Spring bakım kampanyası beş projede
          toplu çalıştırıldı; sınır 3 olduğu için üçü koştu, ikisi sırada
          bekledi. Toplam <b>60 saniye</b> ve <b>$0,0152</b>; çok modüllü bir
          depoda betikler on üç alt modülü tek tek gezdi.
        </Note>
      </Step>

      <Step
        no={12}
        title="Otuz projeyi tek tek eklememek"
        lead="Kurumsal Bitbucket'ta bir grubun adresini verirsiniz; altındaki repository'ler listelenir ve seçtikleriniz proje olur."
      >
        <div className="grid gap-3 md:grid-cols-3">
          <Mini title="Önizleme">
            Grup adresi verilir, repository&apos;ler listelenir. Bu adım yan
            etkisizdir: hiçbir şey kaydedilmez.
          </Mini>
          <Mini title="Seçim">
            Zaten kayıtlı olanlar seçilemez ama <b>listede durur</b> —
            gizlenselerdi &quot;gruptaki her şey geldi mi&quot; sorusu
            cevapsız kalırdı. Arşivli olanlar varsayılan seçimin dışındadır.
          </Mini>
          <Mini title="Sonuç">
            Satırlar geldikçe yazılır ve sonunda özet kalır:{" "}
            <b>9 eklendi, 3 atlandı, 1 başarısız</b> — hangisi neden, satırında
            yazar.
          </Mini>
        </div>
      </Step>

      <Chapter
        id="baglantilar"
        title="Bağlantılar"
        lead="Sistemin dış dünyayla konuştuğu iki yol: tetikleyiciler ve MCP."
      />





      <Step
        no={13}
        title="Jira'dan kendiliğinden başlaması"
        lead="İki giriş yolu var: düzenli JQL taraması ve Jira webhook'u. İkisi de aynı kapıdan geçiyor."
      >
        <TriggerPaths />
        <Note>
          Aynı task&apos;ın iki kez işlenmemesi <b>veritabanı kısıtıyla</b>{" "}
          garanti ediliyor. Uygulama içinde &quot;önce sor, sonra yaz&quot;
          biçiminde bir kontrol olsaydı iki yol aynı anda gelince ikisi de
          &quot;işlenmemiş&quot; görür ve akış iki kez başlardı.
        </Note>
      </Step>

      <Step
        no={14}
        title="MCP Server — iki yön"
        lead="Bir agent yalnızca klonlanmış depoyla çalışırsa çok şey bilmez. MCP, standart bir protokolle dış kaynaklara bağlanmayı sağlıyor — ve aynı protokol ters yönde de işliyor."
      >
        <MCPDirections />
        <Note>
          Üç kullanım aynı protokolün üç yüzü. <b>Agent karar verirse</b>{" "}
          esnektir: &quot;bu hatayı düzelt&quot; dediğinizde yığın izini kendisi
          çeker. <b>Akış karar verirse</b> tekrarlanabilir: her çalıştırmada
          aynı araç, aynı argümanlarla. <b>Ters yönde</b> ise Agent Coder başka
          bir agent&apos;ın aracı olur — akışlarınız Claude Desktop&apos;tan
          tetiklenebilir.
        </Note>
      </Step>

      <Chapter
        id="kararlar"
        title="Kararlar ve sınırlar"
        lead="Geri alınması pahalı seçimler ve neyin nereye eriştiği."
      />



      <Step
        no={15}
        title="Üç karar"
        lead="Sistemin bugünkü halini belirleyen, geri alınması pahalı olan seçimler."
      >
        <div className="grid gap-3 md:grid-cols-3">
          <Decision title="Agent motoru değiştirilebilir">
            Bugün opencode kullanılıyor ama kod ona değil, tek bir <b>Runner</b>{" "}
            arayüzüne bakıyor. Motoru değiştirmek bir paketi değiştirmek demek;
            akış motoruna dokunulmuyor.
          </Decision>
          <Decision title="Davranış kodda değil veritabanında">
            Süre sınırı, eşzamanlılık, bellek, tarama aralığı — hiçbiri koda
            gömülü değil. Kod yalnızca <b>tanımı</b> tutuyor (varsayılan ve
            aralık), veritabanı sapmayı. Değişiklik yeniden başlatma
            gerektirmiyor.
          </Decision>
          <Decision title="Adım = çalıştırma">
            Bir akış adımı, elle başlatılan bir çalıştırmayla <b>aynı kaydı</b>{" "}
            yazıyor. Bu yüzden rapor, akışları hiçbir kod değişikliği olmadan
            kapsadı.
          </Decision>
        </div>
      </Step>

      <Step
        no={16}
        title="Güvenlik sınırları"
        lead="Neyin nereye eriştiği bilinçli olarak dar tutuldu."
      >
        <div className="grid gap-3 md:grid-cols-2">
          <Decision title="Anahtarlar şifreli">
            API anahtarları ve token&apos;lar veritabanında AES-256-GCM ile
            saklanıyor. Hiçbir API yanıtında ve hiçbir log satırında
            görünmüyorlar — testlerle doğrulanıyor. Arayüzde yalnızca son dört
            karakter gösteriliyor.
          </Decision>
          <Decision title="Agent'a token geçmez">
            PR açmak ve Jira&apos;ya yazmak <b>akışın</b> işi, agent&apos;ın
            değil; git kimlik bilgisi agent&apos;a hiç ulaşmaz. Dış araçlarda
            (MCP) anahtar container&apos;ın ortam değişkeninde durur ve isteğe
            başlık olarak eklenir — <b>modele hiç gösterilmez</b>.
          </Decision>
          <Decision title="Container'lar izole">
            Runner&apos;lar ayrı bir ağda çalışır, dışarıya port açmaz, CPU ve
            bellek sınırlıdır.
          </Decision>
          <Decision title="Loglar maskelenerek saklanır">
            Engine logları veritabanına <b>yazılmadan önce</b> temizlenir:
            sağlayıcı anahtarı, git token&apos;ı, MCP sırları ve paket deposu
            kimliği — <code className="font-mono text-xs">.npmrc</code>
            &apos;deki base64 hâli dahil — maskelenir. Sonradan temizlemek bir
            kez yazılmış sırrı geri almaz, bu yüzden temizlik yazma anında.
          </Decision>
          <Decision title="Agent onay bekleyemez">
            Çalıştırmalar başsız (headless): soruyu cevaplayacak kimse yok. Onay
            isteyen her izin bir kilitlenmedir — ölçüldü, bir koşu{" "}
            <b>dokuz dakika</b> sıfır token&apos;da asılı kaldı. Bu yüzden soru,
            plan ve ev dizini erişimi <b>sorulmaz, reddedilir</b>.
          </Decision>

          <Decision title="Paketler kurumsal depodan">
            Bir npm kayıt defteri tanımlarsanız agent bağımlılıkları oradan
            çeker; adres ve kimlik container&apos;a dosya olarak girer, ortam
            değişkenine değil — agent{" "}
            <code className="font-mono text-xs">env</code> yazdırdığında token
            görünmez. Tanımlanmazsa npm&apos;in genel deposu kullanılır.
          </Decision>
          <Decision title="MCP Server'lar agent başına açılır">
            Bir MCP sunucusu tanımlamak onu her agent&apos;a açmaz. Hangi
            agent&apos;ın kullanabileceği ayrı seçilir; seçilmeyenlere araçlar{" "}
            <b>hiç sunulmaz</b>. Bağlanamayan bir sunucu sessiz kalmaz, uyarı
            üretir.
          </Decision>
          <Decision title="Script'ler yeni bir kapı açmaz">
            Bir betik yalnızca <b>komut çalıştırma yetkisi zaten açık</b>{" "}
            agent&apos;a kopyalanır; kapalıysa dosya ortama hiç girmez. Yetki
            kuralları bu özellik için değişmedi. &quot;Bash kapalı ama şu betiğe
            izinli&quot; gibi bir ara mod bilinçli olarak <b>yapılmadı</b>: izin
            eşleşmesi ham komut metnine yapıldığı için kapalı bir kapıyı açardı.
          </Decision>
          <Decision title="Dışarıya açılan adres bir anahtardır" tone="warn">
            Agent Coder&apos;ı MCP olarak kullanmanın adresi, bilen herkesin
            akış başlatabileceği anlamına gelir. Ayarlar&apos;dan yenilenebilir.
            Kimlik doğrulama gelene kadar bu adres paylaşılırken dikkat
            edilmeli.
          </Decision>
          <Decision title="Kimlik doğrulama henüz yok" tone="warn">
            v1 tek kullanıcılıktır ve{" "}
            <b>internete açık bir sunucuda çalıştırılmamalıdır</b>. Şema baştan{" "}
            <code className="font-mono text-xs">user_id</code> taşıyor; auth
            sonradan eklenecek.
          </Decision>
        </div>
      </Step>

      {/*
        Kapanış.

        Sunum için yazılmış bir sayfanın son ekranı, anlatanın bıraktığı
        cümle olmalı. Numaralı bir adım DEĞİL: on adımın ardından on birinci
        bir adım gibi durursa özet olmaktan çıkar, sıradan bir bölüm olur.
      */}
      <section className="rounded-card border border-accent/30 bg-accent-soft p-5">
        <div className="flex items-start gap-3">
          {/* Karo DOLU aksan: kartın zemini zaten `accent-soft` ve `IconTile`
              da aynı yumuşak zemini kullanıyor — ikisi üst üste gelince karo
              görünmez oluyordu. */}
          <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-ink">
            <IconSparkle className="size-4" />
          </span>
          <div className="min-w-0">
            <h2 className="text-base font-semibold tracking-[-0.01em]">
              Sonuç
            </h2>
            <p className="mt-1.5 max-w-3xl text-sm leading-relaxed text-ink-2">
              Koddan pull request&apos;e kadar tüm yol otomatik, izlenebilir ve
              tekrarlanabilir. Siz akışı tasarlarsınız; sırayı, paralelliği,
              yeniden denemeyi ve temizliği sistem çalıştırır.
            </p>
          </div>
        </div>
      </section>
    </div>
  );
}

/**
 * Sıfat kutusu — mimarinin ardındaki beşli şerit.
 *
 * Renk BURADA anlamlı: her sıfat sistemin ayrı bir niteliğini işaret
 * ediyor ve karo o niteliği ayırt edilebilir kılıyor. Durum rengi değil —
 * "güvenli" yeşili bir başarı bildirimi değil, bir kimlik.
 */
function Trait({
  icon,
  tone,
  title,
  children,
}: {
  icon: React.ReactNode;
  tone: TileTone;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-card border border-line bg-surface p-3.5 shadow-(--shadow-card)">
      <div className="flex items-center gap-2.5">
        <IconTile tone={tone} size="sm">
          {icon}
        </IconTile>
        <p className="text-sm font-medium">{title}</p>
      </div>
      <p className="mt-2 text-xs leading-relaxed text-ink-2">{children}</p>
    </div>
  );
}

/* ── Sayfa parçaları ─────────────────────────────────────────────────────── */

/**
 * Bölüm başlığı — sayfa on altı adıma çıkınca gerekti.
 *
 * Adımlar numaralı bir dizi olarak akıyor; bölüm başlığı o diziyi konularına
 * ayırıyor ve içindekiler bağlantısının hedefi oluyor. Numaralar SÜREKLİ
 * kalıyor (her bölümde 1'den başlamıyor): sunumda "yedinci adım" demek,
 * "kurumsal bölümünün ikinci adımı" demekten kısa.
 */
function Chapter({
  id,
  title,
  lead,
}: {
  id: string;
  title: string;
  lead: string;
}) {
  return (
    <section id={id} className="scroll-mt-14 border-t border-line pt-6">
      <h2 className="text-lg font-semibold tracking-[-0.01em]">{title}</h2>
      <p className="mt-1 max-w-3xl text-sm leading-relaxed text-ink-2">{lead}</p>
    </section>
  );
}

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
        <span className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full bg-accent text-sm font-semibold text-accent-ink">
          {no}
        </span>
        <div className="min-w-0">
          {/* Bölüm h2 olduğu için adım h3: başlık sırası atlanmamalı. */}
          <h3 className="text-base font-semibold tracking-[-0.01em]">{title}</h3>
          <p className="mt-1 max-w-3xl text-sm leading-relaxed text-ink-2">
            {lead}
          </p>
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
    <p className="mt-4 border-t border-line pt-3 text-sm leading-relaxed text-ink-2">
      {children}
    </p>
  );
}

function Mini({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <div className="rounded-lg border border-line bg-raised p-3">
      <p className="text-sm font-medium">{title}</p>
      <p className="mt-1 text-xs leading-relaxed text-ink-2">{children}</p>
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
        tone === "warn"
          ? "border-warn/40 bg-warn-soft"
          : "border-line bg-raised"
      }`}
    >
      <p className="text-sm font-medium">{title}</p>
      <p className="mt-1.5 text-xs leading-relaxed text-ink-2">{children}</p>
    </div>
  );
}
