import type { Metadata } from "next";

import {
  Architecture,
  BatchQueue,
  CorporateTLS,
  DataBoundary,
  DataModel,
  EgressControl,
  MCPDirections,
  Parallelism,
  ProductFlow,
  RunnerAnatomy,
  ScriptDeterminism,
  StepLifecycle,
  TriggerPaths,
} from "@/components/docs/diagrams";

export const metadata: Metadata = {
  title: "Nasıl çalışır",
};

/*
 * ─────────────────────────────────────────────────────────────────────────────
 * TASARIM NOTU — bu sayfa bilinçli olarak ürünün geri kalanına benzemiyor.
 *
 * Konu: bu ürünün dünyası bir SINIR üzerine kurulu. Container doğar, işini
 * yapar, silinir; ağın dışına ne çıktığı bir çizgiyle belirlenir. Sayfanın
 * imzası da bu: her blok, etiketi çizginin üzerinde duran kesikli bir MUHAFAZA
 * içinde. Zemin milimetrik kâğıt (açık) ve cyanotype mavi (koyu) — ikisi de
 * mühendislik çiziminin kendi malzemesi ve diyagramların diliyle aynı.
 *
 * Yapı numaralı adım DEĞİL, soru–cevap: bir değerlendirici bu sayfaya sırayla
 * beş soruyla geliyor ve başlıklar o soruların cevabı olarak yazıldı.
 *
 * Diyagramlar `var(--color-*)` jetonlarından besleniyordu; jetonlar burada
 * SAYFA KAPSAMINDA yeniden tanımlanıyor, böylece on üç çizim tek satır kod
 * değişmeden bu paletle yeniden boyanıyor.
 *
 * Sunucu bileşeni olarak kalıyor: kahramandaki satır satır beliriş saf CSS
 * (kademeli animation-delay), JavaScript yok.
 * ─────────────────────────────────────────────────────────────────────────────
 */

/** Kahraman: ürünün kendi olay akışı — metinler koddaki gerçek satırlar. */
const KOSU = [
  ["00:00", "çalışma ortamı hazırlanıyor (node 24)"],
  ["00:03", "çıkış denetimi açık — dışarıya yalnızca izinli adreslere, proxy üzerinden"],
  ["00:09", "depo hazır, agent başlatılıyor"],
  ["00:11", "01-bakim-notu çalıştı · docs/BAKIM-NOTU.md yazıldı"],
  ["00:18", "02-spring-boot-surumu çalıştı · pom.xml: parent 3.2.0"],
  ["00:24", "03-mvnw-cmd-sil çalıştı · mvnw.cmd silindi"],
  ["00:39", "çalışma tamamlandı · 4 dosya değişti · $0,0030"],
  ["00:41", "engine logları toplandı"],
  ["00:42", "container + volume silindi"],
] as const;

/** Ray ve şerit aynı listeden çiziliyor: iki yerde tutulan bir menü ayrışır. */
const BOLUMLER = [
  { id: "ne-yapiyor", kisa: "Ne yapıyor" },
  { id: "nerede", kisa: "Nerede çalışıyor" },
  { id: "agdan-ne-cikiyor", kisa: "Ağdan ne çıkıyor" },
  { id: "otuz-proje", kisa: "Otuz projede" },
  { id: "bozulunca", kisa: "Bozulunca" },
  { id: "sinirlar", kisa: "Sınırlar" },
] as const;

export default function HowItWorksPage() {
  return (
    <div className="sayfa">
      <style>{CSS}</style>

      {/*
        BÖLÜM RAYI — sayfa dokuz bin piksel; en altta "başa dön" için kaydırmak
        gerekiyordu. Ray yapışkan ve her zaman görünür; çizim sayfalarındaki
        ölçek cetveli gibi kenarda duruyor, dikey bir çizgi ve tikler.

        Saf çapa bağlantısı: JavaScript yok, sayfa sunucu bileşeni kalıyor.
        Dar ekranda ray gizlenir, üstte yatay bir şerit belirir.
      */}
      <nav className="ray" aria-label="Bölümler">
        <a href="#tepe" className="ray-tepe">
          ↑ başa
        </a>
        {BOLUMLER.map((b) => (
          <a key={b.id} href={`#${b.id}`}>
            <span className="ray-tik" aria-hidden="true" />
            {b.kisa}
          </a>
        ))}
      </nav>

      <nav className="serit" aria-label="Bölümler">
        <a href="#tepe">↑ başa</a>
        {BOLUMLER.map((b) => (
          <a key={b.id} href={`#${b.id}`}>
            {b.kisa}
          </a>
        ))}
      </nav>

      <div className="govde">

      {/* ── Kahraman ───────────────────────────────────────────────────── */}
      <header className="kahraman" id="tepe">
        <p className="ustyazi">Agent Coder · nasıl çalışır</p>

        <h1 className="baslik">
          Kod sizin ağınızda kalır.
          <br />
          <span className="baslik-2">İş bitince container kalmaz.</span>
        </h1>

        <p className="giris">
          Bir agent adımı kendi geçici container&apos;ında doğar, işini yapar ve
          silinir. Dışarıya çıkan her bağlantı tek bir kapıdan geçer; o kapının
          neye izin vereceğine siz karar verirsiniz. Aşağıdaki satırlar gerçek
          bir çalışmanın olay akışı.
        </p>

        <div className="muhafaza kosu">
          <span className="muhafaza-etiket">BİR ADIMIN ÖMRÜ · 42 SANİYE</span>
          <ol className="kosu-liste">
            {KOSU.map(([t, satir], i) => (
              <li key={t} style={{ animationDelay: `${140 + i * 110}ms` }}>
                <span className="kosu-saat">{t}</span>
                <span className="kosu-metin">{satir}</span>
              </li>
            ))}
          </ol>

          {/*
            DAMGA — çizim sayfalarındaki kauçuk mühür. Akış bittikten sonra
            basılıyor ve sayfanın tek gösterişli anı burası: container'ın
            silinmesi bu ürünün en karakteristik davranışı.
          */}
          <span className="damga" aria-hidden="true">
            silindi
          </span>
        </div>
      </header>

      {/* ── S1 ─────────────────────────────────────────────────────────── */}
      <Soru
        id="ne-yapiyor"
        soru="Ne yapıyor?"
        cevap="Tek tek çalıştırdığınız agent'ları birbirine bağlar."
        aciklama="Bir adımın çıktısını diğerine kopyala-yapıştır taşımak yerine akış çizersiniz. Her adım kendi modeliyle koşar: analiz için ucuz bir model, kod yazımı için güçlü bir model."
      >
        <Cizim etiket="AKIŞ">
          <ProductFlow />
        </Cizim>
        <Cizim etiket="PARALELLİK — BAĞIMSIZ ADIMLAR AYNI ANDA">
          <Parallelism />
        </Cizim>
        <Olcum
          deger="10 sn"
          etiket="ölçülen örtüşme"
          not="Tuvalden kurulan iki dallı bir akışta iki adım aynı anda koştu. Sıraya dizilseydi süre ikisinin toplamı olurdu."
        />
      </Soru>

      {/* ── S2 ─────────────────────────────────────────────────────────── */}
      <Soru
        id="nerede"
        soru="Kodum nerede çalışıyor?"
        cevap="İş başına açılan, iş bitince silinen bir container'da."
        aciklama="Container dışarıya port açmaz, host dosya sistemine bağlanmaz ve başka bir çalıştırmanın container'ını görmez. CPU ve bellek sınırı ayarlardan gelir."
      >
        <Cizim etiket="CONTAINER'IN ANATOMİSİ">
          <RunnerAnatomy />
        </Cizim>
        <Cizim etiket="BİR ADIMIN ÖMRÜ">
          <StepLifecycle />
        </Cizim>
        <Olcum
          deger="0"
          etiket="çalışma sonrası kalan container"
          not="Zaman aşımı, iptal ya da sunucunun yeniden başlaması fark etmez; silme her yolda çalışır. Doğrulaması: docker ps -a."
        />
        <Cizim etiket="SERVİSLER — KUYRUK YOK, MESAJ ARACISI YOK">
          <Architecture />
        </Cizim>
      </Soru>

      {/* ── S3 — sayfanın ağırlık merkezi ──────────────────────────────── */}
      <Soru
        id="agdan-ne-cikiyor"
        soru="Ağdan ne çıkıyor?"
        cevap="Yalnızca sizin izin verdiğiniz adreslere giden bağlantılar."
        aciklama="Denetim ayardan değil ağdan geliyor: container internete rotası olmayan bir ağda doğuyor ve dışarıya tek yol çıkış kapısı. Ortam değişkeniyle verilen bir proxy atlanabiliyordu — 26 bağlantının 5'i atladı; rota yoksa atlanacak bir şey de yok."
        vurgu
      >
        <Cizim etiket="ÇIKIŞ KAPISI, WHITELIST VE KURUMSAL PROXY">
          <EgressControl />
        </Cizim>

        <div className="ikili">
          <Kural baslik="Kapı TLS açmaz">
            Yalnızca <code>CONNECT host:443</code> satırındaki adı görür ve
            baytları tünneller. Araya, sağlayıcı anahtarını görebilecek bir nokta
            koymuyoruz. Bedeli: karar host&apos;a bakar, porta bakmaz.
          </Kural>
          <Kural baslik="Boş whitelist kısıt yokluğudur">
            Liste boşken tüm adreslere çıkılır. Bu yüzden zorunlu adresler
            (sağlayıcı, depo, registry, MCP) listeye{" "}
            <b>yalnızca liste doluyken</b> eklenir — boş bir listeye ekleme
            yapmak, kısıtsız olması gereken bir çalıştırmayı sessizce dört adrese
            hapsederdi.
          </Kural>
        </div>

        <Cizim etiket="VERİNİN SINIRI — İKİ KURULUM">
          <DataBoundary />
        </Cizim>

        <p className="durust">
          Dürüst cümle: <b>dış bir sağlayıcıda model çağrısı kodun bir parçasını
          dışarı taşır.</b> Bunu çıkış kapısı engelleyemez; engellenirse model hiç
          çalışmaz. Kurum içi bir sağlayıcıyla (LiteLLM, vLLM, OpenAI-uyumlu bir
          servis) hiçbir istek kurum ağının dışına çıkmaz. Bu bir kurulum
          kararıdır, bir güvenlik ayarı değil.
        </p>

        <Cizim etiket="SSL INSPECTION — SERTİFİKANIN GİTTİĞİ DÖRT YER">
          <CorporateTLS />
        </Cizim>
      </Soru>

      {/* ── S4 ─────────────────────────────────────────────────────────── */}
      <Soru
        id="otuz-proje"
        soru="Otuz projede ne oluyor?"
        cevap="Kuyruğa giriyor; sınır kadarı koşuyor, gerisi bekliyor."
        aciklama="Aynı akışı otuz projede tek hamlede sıraya koyarsınız. Kuyruk mevcut eşzamanlılık sınırına uyar, kendi paralellik ayarını tanımlamaz — 'aynı anda kaç iş' sorusunun tek bir cevabı olmalı."
      >
        <Cizim etiket="TOPLU ÇALIŞTIRMA KUYRUĞU">
          <BatchQueue />
        </Cizim>
        <div className="ikili">
          <Kural baslik="Bir proje düşerse kuyruk durmaz">
            Sebebi o satırda yazar, sıradaki başlar. İptal bekleyenleri düşürür;
            çalışanlar kendi hâlinde sürer ve sonuçları kaydedilir.
          </Kural>
          <Kural baslik="Yeniden başlatmaya dayanır">
            Kuyruk veritabanında. Bekleyenler bekler, o an çalışanlar{" "}
            <b>kesildi</b> olarak işaretlenir ve kendiliğinden tekrarlanmaz —
            yarım kalmış bir işin yan etkisi habersizce tekrarlanmamalı.
          </Kural>
        </div>
        <Cizim etiket="BETİKLER — MODEL NE ZAMAN'A, BETİK NE YAPILACAĞINA KARAR VERİR">
          <ScriptDeterminism />
        </Cizim>
        <Olcum
          deger="60 sn · $0,0152"
          etiket="beş projelik gerçek kampanya"
          not="Beş adımlık bir Spring bakım kampanyası beş projede toplu koştu; sınır 3 olduğu için üçü çalıştı, ikisi sırada bekledi. Çok modüllü bir depoda betikler on üç alt modülü tek tek gezdi."
        />
      </Soru>

      {/* ── S5 ─────────────────────────────────────────────────────────── */}
      <Soru
        id="bozulunca"
        soru="Bir şey bozulunca elimde ne kalıyor?"
        cevap="Adımın girdisi, çıktısı, maliyeti ve motorun ham logu."
        aciklama="Kaydedilen graf değişmez: geçmiş bir çalışma, o gün hangi tanımla koştuysa onu gösterir. Maliyet ikinci bir yerde tutulmaz; rapor çalıştırma kayıtlarından toplanır."
      >
        <Cizim etiket="VERİ MODELİ">
          <DataModel />
        </Cizim>
        <div className="ikili">
          <Kural baslik="Loglar container'la gitmez">
            Motorun teşhis verisi silmeden hemen önce toplanır: container
            çıktısı, motorun log dosyaları ve agent&apos;ın tam oturum geçmişi.
            Asıl ihtiyaç düşen koşularda ve o veri container&apos;la birlikte
            gidiyordu.
          </Kural>
          <Kural baslik="Sırlar yazılmadan önce maskelenir">
            Sağlayıcı anahtarı, git token&apos;ı, MCP sırları ve paket deposu
            kimliği — <code>.npmrc</code>&apos;deki base64 hâli dahil. Sonradan
            temizlemek bir kez yazılmış sırrı geri almaz.
          </Kural>
        </div>
        <Cizim etiket="TETİKLEME YOLLARI VE TEKRAR KORUMASI">
          <TriggerPaths />
        </Cizim>
        <Cizim etiket="MCP — İKİ YÖN">
          <MCPDirections />
        </Cizim>
      </Soru>

      {/* ── Sınırlar ───────────────────────────────────────────────────── */}
      <section className="sinirlar" id="sinirlar">
        <p className="ustyazi">Sınırlar</p>
        <h2 className="bolum-baslik">Neyin nereye eriştiği bilinçli olarak dar.</h2>

        <dl className="sinir-liste">
          <Sinir baslik="Anahtarlar şifreli">
            AES-256-GCM ile saklanır; hiçbir API yanıtında ve hiçbir log
            satırında görünmezler. Arayüzde yalnızca son dört karakter.
          </Sinir>
          <Sinir baslik="Agent'a token geçmez">
            PR açmak ve Jira&apos;ya yazmak akışın işi. Git kimlik bilgisi
            agent&apos;a hiç ulaşmaz; MCP anahtarı ortam değişkeninde durur,
            modele gösterilmez.
          </Sinir>
          <Sinir baslik="Agent onay bekleyemez">
            Çalıştırmalar başsız; soruyu cevaplayacak kimse yok. Onay isteyen
            izin bir kilitlenmedir — ölçüldü, bir koşu dokuz dakika sıfır
            token&apos;da asılı kaldı. Soru, plan ve ev dizini erişimi sorulmaz,
            reddedilir.
          </Sinir>
          <Sinir baslik="Betikler yeni bir kapı açmaz">
            Bir betik yalnızca komut çalıştırma yetkisi zaten açık agent&apos;a
            kopyalanır. &quot;Bash kapalı ama şu betiğe izinli&quot; ara modu
            bilinçli olarak yapılmadı: izin eşleşmesi ham komut metnine yapılıyor
            ve kapalı bir kapıyı açardı.
          </Sinir>
          <Sinir baslik="Paketler kurumsal depodan">
            Registry adresi ayarlardan; kimlik container&apos;a dosya olarak
            girer, ortam değişkenine değil — agent <code>env</code> yazdırdığında
            token görünmez.
          </Sinir>
          <Sinir baslik="Kimlik doğrulama henüz yok" uyari>
            v1 tek kullanıcılıktır ve internete açık bir sunucuda
            çalıştırılmamalıdır. Şema baştan <code>user_id</code> taşıyor; auth
            sonradan eklenecek.
          </Sinir>
        </dl>
      </section>

      <footer className="kapanis">
        <p>
          Siz akışı tasarlarsınız; sırayı, paralelliği ve temizliği sistem
          çalıştırır.
        </p>
        <a href="#tepe" className="basa-don">
          ↑ Sayfanın başına dön
        </a>
      </footer>
      </div>
    </div>
  );
}

/* ── Parçalar ────────────────────────────────────────────────────────────── */

function Soru({
  id,
  soru,
  cevap,
  aciklama,
  vurgu = false,
  children,
}: {
  id: string;
  soru: string;
  cevap: string;
  aciklama: string;
  vurgu?: boolean;
  children: React.ReactNode;
}) {
  return (
    <section id={id} className={`soru${vurgu ? " soru-vurgu" : ""}`}>
      <div className="soru-bas">
        <p className="ustyazi">{soru}</p>
        <h2 className="bolum-baslik">{cevap}</h2>
        <p className="bolum-aciklama">{aciklama}</p>
      </div>
      <div className="soru-govde">{children}</div>
    </section>
  );
}

/** Diyagram muhafazası — etiketi çerçevenin üzerinde durur. */
function Cizim({
  etiket,
  children,
}: {
  etiket: string;
  children: React.ReactNode;
}) {
  return (
    <figure className="muhafaza cizim">
      <span className="muhafaza-etiket">{etiket}</span>
      {children}
    </figure>
  );
}

/** Tek bir ölçülmüş sayı — iddia değil, kayıt. */
function Olcum({
  deger,
  etiket,
  not,
}: {
  deger: string;
  etiket: string;
  not: string;
}) {
  return (
    <div className="olcum">
      <p className="olcum-deger">{deger}</p>
      <p className="olcum-etiket">{etiket}</p>
      <p className="olcum-not">{not}</p>
    </div>
  );
}

function Kural({
  baslik,
  children,
}: {
  baslik: string;
  children: React.ReactNode;
}) {
  return (
    <div className="kural">
      <p className="kural-baslik">{baslik}</p>
      <p className="kural-metin">{children}</p>
    </div>
  );
}

function Sinir({
  baslik,
  children,
  uyari = false,
}: {
  baslik: string;
  children: React.ReactNode;
  uyari?: boolean;
}) {
  return (
    <div className={`sinir${uyari ? " sinir-uyari" : ""}`}>
      <dt>{baslik}</dt>
      <dd>{children}</dd>
    </div>
  );
}

/* ── Biçim ───────────────────────────────────────────────────────────────── */

const CSS = `
/*
 * FONTLAR REPODA — derleme anında ağa çıkılmıyor.
 *
 * Önce next/font/google kullanılıyordu; o, dosyaları BUILD SIRASINDA Google'dan
 * indiriyor. Kapalı ağda derleme yapan bir kurulumda bu, sayfanın değil
 * DERLEMENİN kırılması demek. Dosyalar public/fonts altında duruyor (sekiz
 * woff2, 156 KB) ve yalnızca bu sayfa yüklenince iniyorlar.
 *
 * Alt kümeler ayrı dosyalarda: latin-ext Türkçe için zorunlu (ş, ğ, İ, ı),
 * latin gerisi. Tarayıcı yalnızca gereken aralığı indirir.
 */
@font-face {
  font-family: 'Martian Mono';
  font-style: normal;
  font-weight: 500 600;
  font-display: swap;
  src: url('/fonts/martian-mono-latin.woff2') format('woff2');
  unicode-range: u+00??,u+0131,u+0152-0153,u+02bb-02bc,u+02c6,u+02da,u+02dc,u+0304,u+0308,u+0329,u+2000-206f,u+20ac,u+2122,u+2191,u+2193,u+2212,u+2215,u+feff,u+fffd;
}
@font-face {
  font-family: 'Martian Mono';
  font-style: normal;
  font-weight: 500 600;
  font-display: swap;
  src: url('/fonts/martian-mono-latin-ext.woff2') format('woff2');
  unicode-range: u+0100-02ba,u+02bd-02c5,u+02c7-02cc,u+02ce-02d7,u+02dd-02ff,u+0304,u+0308,u+0329,u+1d00-1dbf,u+1e00-1e9f,u+1ef2-1eff,u+2020,u+20a0-20ab,u+20ad-20c0,u+2113,u+2c60-2c7f,u+a720-a7ff;
}
@font-face {
  font-family: 'IBM Plex Sans';
  font-style: normal;
  font-weight: 400 600;
  font-display: swap;
  src: url('/fonts/plex-sans-latin.woff2') format('woff2');
  unicode-range: u+00??,u+0131,u+0152-0153,u+02bb-02bc,u+02c6,u+02da,u+02dc,u+0304,u+0308,u+0329,u+2000-206f,u+20ac,u+2122,u+2191,u+2193,u+2212,u+2215,u+feff,u+fffd;
}
@font-face {
  font-family: 'IBM Plex Sans';
  font-style: normal;
  font-weight: 400 600;
  font-display: swap;
  src: url('/fonts/plex-sans-latin-ext.woff2') format('woff2');
  unicode-range: u+0100-02ba,u+02bd-02c5,u+02c7-02cc,u+02ce-02d7,u+02dd-02ff,u+0304,u+0308,u+0329,u+1d00-1dbf,u+1e00-1e9f,u+1ef2-1eff,u+2020,u+20a0-20ab,u+20ad-20c0,u+2113,u+2c60-2c7f,u+a720-a7ff;
}
@font-face {
  font-family: 'IBM Plex Mono';
  font-style: normal;
  font-weight: 400;
  font-display: swap;
  src: url('/fonts/plex-mono-400-latin.woff2') format('woff2');
  unicode-range: u+00??,u+0131,u+0152-0153,u+02bb-02bc,u+02c6,u+02da,u+02dc,u+0304,u+0308,u+0329,u+2000-206f,u+20ac,u+2122,u+2191,u+2193,u+2212,u+2215,u+feff,u+fffd;
}
@font-face {
  font-family: 'IBM Plex Mono';
  font-style: normal;
  font-weight: 400;
  font-display: swap;
  src: url('/fonts/plex-mono-400-latin-ext.woff2') format('woff2');
  unicode-range: u+0100-02ba,u+02bd-02c5,u+02c7-02cc,u+02ce-02d7,u+02dd-02ff,u+0304,u+0308,u+0329,u+1d00-1dbf,u+1e00-1e9f,u+1ef2-1eff,u+2020,u+20a0-20ab,u+20ad-20c0,u+2113,u+2c60-2c7f,u+a720-a7ff;
}
@font-face {
  font-family: 'IBM Plex Mono';
  font-style: normal;
  font-weight: 500;
  font-display: swap;
  src: url('/fonts/plex-mono-500-latin.woff2') format('woff2');
  unicode-range: u+00??,u+0131,u+0152-0153,u+02bb-02bc,u+02c6,u+02da,u+02dc,u+0304,u+0308,u+0329,u+2000-206f,u+20ac,u+2122,u+2191,u+2193,u+2212,u+2215,u+feff,u+fffd;
}
@font-face {
  font-family: 'IBM Plex Mono';
  font-style: normal;
  font-weight: 500;
  font-display: swap;
  src: url('/fonts/plex-mono-500-latin-ext.woff2') format('woff2');
  unicode-range: u+0100-02ba,u+02bd-02c5,u+02c7-02cc,u+02ce-02d7,u+02dd-02ff,u+0304,u+0308,u+0329,u+1d00-1dbf,u+1e00-1e9f,u+1ef2-1eff,u+2020,u+20a0-20ab,u+20ad-20c0,u+2113,u+2c60-2c7f,u+a720-a7ff;
}
.sayfa {
  --yazi-display: 'Martian Mono';
  --yazi-govde: 'IBM Plex Sans';
  --yazi-veri: 'IBM Plex Mono';
  /* Palet: milimetrik kâğıt. Kâğıt soğuk beyaz, mürekkep lacivert, iki aksan —
     çizim mavisi (izinli) ve mühür kırmızısı (reddedilen). */
  --kagit: #e8eef3;
  --sayfa-yuzey: #ffffff;
  --murekkep: #10202e;
  --murekkep-2: #465f75;
  --cizgi: #b7c7d4;
  --cizgi-koyu: #7f97a9;
  --mavi: #17539f;
  --kirmizi: #a52620;
  --kehribar: #8a5a00;
  --izgara: rgba(23, 83, 159, 0.07);

  /* Diyagramlar bu jetonlardan besleniyor: kapsam içinde yeniden tanımlanınca
     on üç çizim tek satır kod değişmeden bu paletle boyanıyor. */
  --color-surface: var(--sayfa-yuzey);
  --color-raised: #f2f6f9;
  --color-line: var(--cizgi);
  --color-line-strong: var(--cizgi-koyu);
  --color-ink: var(--murekkep);
  --color-ink-2: var(--murekkep-2);
  /* Ölçümle koyulaştırıldı: #6b8093 beyaz üzerinde 4,09 veriyordu ve saat
     sütunu ile muhafaza etiketi 10,5–12,5px — eşik 4,5. */
  --color-ink-3: #55697b;
  --color-accent: var(--mavi);
  --color-accent-soft: rgba(23, 83, 159, 0.1);
  /* Aksanın ÜZERİNDEKİ mürekkep. Kâğıt rengine bağlanıyor: koyu temada
     --sayfa-yuzey zaten dönüyor, ayrı bir koyu tanım gerekmiyor. Bu jeton
     eşlenmezse StepLifecycle'ın adım numaraları global paletten gelir ve
     sayfanın mavisiyle yalnızca tesadüfen uyuşur. */
  --color-accent-ink: var(--sayfa-yuzey);
  --color-danger: var(--kirmizi);
  --color-ok: #0f6b4a;
  --color-warn: var(--kehribar);

  /* Kabuğun iç boşluğu (AppShell: px-5 py-6 sm:px-6 lg:px-8 lg:py-7). Kâğıdın
     kenara ulaşması için taşma bu boşluğun tam negatifi olmalı. Sabit
     yazıldığında yalnızca en dar kırılma noktası iptal oluyordu: 1600px'te iki
     yanda 12px uygulama zemini ölçüldü. Şerit de aynı jetonlardan besleniyor. */
  --kabuk-x: 1.25rem;
  --kabuk-y: 1.5rem;

  margin: calc(var(--kabuk-y) * -1) calc(var(--kabuk-x) * -1);
  padding: 0 var(--kabuk-x) 5rem;
  position: relative;
  background-color: var(--kagit);
  /* Milimetrik kâğıt: 8px ince, 40px kalın kare. */
  background-image:
    linear-gradient(var(--izgara) 1px, transparent 1px),
    linear-gradient(90deg, var(--izgara) 1px, transparent 1px),
    linear-gradient(var(--izgara) 1px, transparent 1px),
    linear-gradient(90deg, var(--izgara) 1px, transparent 1px);
  background-size: 40px 40px, 40px 40px, 8px 8px, 8px 8px;
  color: var(--murekkep);
  font-family: var(--yazi-govde), ui-sans-serif, system-ui, sans-serif;
  font-size: 15px;
  line-height: 1.6;
}

/* Tailwind kırılma noktaları: sm 40rem, lg 64rem. Kabuk bu noktalarda iç
   boşluğunu değiştiriyor, kâğıdın taşması da onu takip ediyor. */
@media (min-width: 40rem) {
  .sayfa { --kabuk-x: 1.5rem; }
}
@media (min-width: 64rem) {
  .sayfa { --kabuk-x: 2rem; --kabuk-y: 1.75rem; }
}

/*
 * Koyu: cyanotype. Mavi zeminde açık çizgiler — blueprint'in kendisi.
 *
 * Jeton listesi globals.css'teki kalıbın aynısıyla İKİ KEZ yazılıyor; sebebi
 * ve neden tek seçiciye indirilemediği orada anlatılıyor (üç durum: sistem,
 * açık, koyu). Buradaki kopya sayfanın kendi paletini taşıdığı için gerekli.
 * Medya sorgusu yalnızca "seçim yapılmamış" durumu kapsar — :not([data-theme])
 * ve açık seçim sonda durup gerektiğinde kazanır, globals.css ile aynı sıra.
 */
@media (prefers-color-scheme: dark) {
  :root:not([data-theme]) .sayfa {
    --kagit: #08192b;
    --sayfa-yuzey: #0d2338;
    --murekkep: #e8f1f8;
    --murekkep-2: #a9c2d6;
    --cizgi: #24455f;
    --cizgi-koyu: #4d7796;
    --mavi: #7fb3f2;
    --kirmizi: #ff8b84;
    --kehribar: #e3b25f;
    --izgara: rgba(127, 179, 242, 0.08);
    --color-ink-3: #93aec4;
    --color-raised: #123049;
    --color-accent-soft: rgba(127, 179, 242, 0.2);
    --color-ok: #4cc79a;
  }
}

:root[data-theme="dark"] .sayfa {
  --kagit: #08192b;
  --sayfa-yuzey: #0d2338;
  --murekkep: #e8f1f8;
  --murekkep-2: #a9c2d6;
  --cizgi: #24455f;
  --cizgi-koyu: #4d7796;
  --mavi: #7fb3f2;
  --kirmizi: #ff8b84;
  --kehribar: #e3b25f;
  --izgara: rgba(127, 179, 242, 0.08);
  --color-ink-3: #93aec4;
  --color-raised: #123049;
  --color-accent-soft: rgba(127, 179, 242, 0.2);
  --color-ok: #4cc79a;
}

/* ── Bölüm rayı ─────────────────────────────────────────────────────────── */

.sayfa .govde { min-width: 0; }

.sayfa .ray { display: none; }

/* Ray yalnızca yer olduğunda: dar ekranda içeriği daraltmak, okumayı
   kolaylaştırmak için eklenen bir aracın okumayı zorlaştırması olurdu. */
@media (min-width: 86rem) {
  .sayfa {
    display: grid;
    grid-template-columns: 1fr 12.5rem;
    column-gap: 3rem;
    align-items: start;
  }
  /* Izgaranın tek içerik öğesi .govde; bölümler onun İÇİNDE, dolayısıyla
     ızgara öğesi değiller ve kendilerine grid-column verilemez. */
  .sayfa .govde { grid-column: 1; }

  .sayfa .ray {
    display: flex;
    grid-column: 2;
    grid-row: 1 / -1;
    position: sticky;
    top: 2rem;
    flex-direction: column;
    gap: 0.55rem;
    padding: 0.25rem 0 0.25rem 1rem;
    border-left: 1px solid var(--cizgi);
  }
  .sayfa .ray a {
    position: relative;
    font-family: var(--yazi-veri), monospace;
    font-size: 10.5px;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    color: var(--murekkep-2);
    text-decoration: none;
    line-height: 1.35;
  }
  .sayfa .ray a:hover { color: var(--mavi); }
  .sayfa .ray a:focus-visible { outline: 2px solid var(--mavi); outline-offset: 3px; }
  /* Tik: cetvel çentiği. Bağlantının rayla ilişkisini renk değil biçim kurar. */
  .sayfa .ray-tik {
    position: absolute;
    left: -1rem;
    top: 0.42em;
    width: 0.55rem;
    height: 1px;
    background: var(--cizgi-koyu);
  }
  .sayfa .ray a:hover .ray-tik { background: var(--mavi); width: 0.85rem; }
  .sayfa .ray-tepe { margin-bottom: 0.35rem; color: var(--mavi) !important; }
}

/* Dar ekranda ray yerine üstte şerit. */
.sayfa .serit {
  position: sticky;
  /* Yapışkan ölçü kabuğun İÇ kutusuna göre alınıyor, dolayısıyla top: 0 şeridi
     kabuğun üst boşluğu kadar aşağıda tutuyordu: üstünde kalan bantta içerik
     görünerek kayıyordu (1280px'te 28px ölçüldü). Boşluğun negatifi şeridi
     kâğıdın gerçek üst kenarına oturtuyor. */
  top: calc(var(--kabuk-y) * -1);
  z-index: 5;
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  margin: 0 calc(var(--kabuk-x) * -1);
  padding: 0.6rem var(--kabuk-x);
  background: color-mix(in srgb, var(--kagit) 88%, transparent);
  backdrop-filter: blur(6px);
  border-bottom: 1px solid var(--cizgi);
}
.sayfa .serit a {
  font-family: var(--yazi-veri), monospace;
  font-size: 10.5px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--murekkep-2);
  text-decoration: none;
  padding: 0.15rem 0.45rem;
  border: 1px solid var(--cizgi);
}
.sayfa .serit a:hover { color: var(--mavi); border-color: var(--mavi); }
@media (min-width: 86rem) {
  .sayfa .serit { display: none; }
}

.sayfa .basa-don {
  display: inline-block;
  margin-top: 1.5rem;
  font-family: var(--yazi-veri), monospace;
  font-size: 11px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--mavi);
  text-decoration: none;
  border-bottom: 1px solid currentColor;
}

/* Bölümler şeridin altına gizlenmesin. Şerit tek satır: 820px'e kadar hiçbir
   masaüstü genişliğinde sarmıyor, 43,8px ölçüldü. 3,5rem şeridin altını
   kurtarmıyordu — çapa sıçramasında bölümün üst kenarlığı, S3'ün mavi vurgu
   çizgisi dahil, şeridin arkasında kalıyordu. 4rem hem şeridi hem kenarlığı
   açıkta bırakıyor. */
.sayfa .soru,
.sayfa .sinirlar,
.sayfa .kahraman { scroll-margin-top: 4rem; }

/* ── Çizimler ───────────────────────────────────────────────────────────── */

/* Çizim dar ekranda küçülmez, kabı kaydırılır — 11px etiketler 4px'e inseydi
   diyagram görünür ama okunmaz olurdu. Ölçü TEK yerde: aşağıdaki container
   query aynı sayıya bakıyor. */
.sayfa .cizim-svg { min-width: 760px; }

.sayfa .cizim-kap { container-type: inline-size; }

/* Kaydırma ipucu yalnızca çizim gerçekten sığmadığında. Kap 760'tan darsa
   diyagram kırpılıyordu ve bunu söyleyen hiçbir şey yoktu; "her ihtimale
   karşı" sürekli duran bir ipucu ise sığdığı yerde yalan söylerdi. */
.sayfa .cizim-ipucu { display: none; }

@container (width < 760px) {
  .sayfa .cizim-ipucu {
    display: block;
    margin: 0.5rem 0 0;
    font-family: var(--yazi-veri), monospace;
    font-size: 10.5px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--color-ink-3);
  }
}

/* Çizimin küçük etiketleri mono: teknik çizimde yazı da bir ölçü aracıdır.
   Yalnızca cizim-mono işaretli metinler — kutu başlıkları gövde yazısında
   kalıyor, çünkü mono onları genişletip kutudan taşırıyordu. */
.sayfa .cizim-svg text.cizim-mono {
  font-family: var(--yazi-veri), ui-monospace, monospace;
}

/* Ok etiketi çizginin ve kutunun ÜZERİNE oturur; teknik çizimde etiket
   altındaki çizgiyi keser. Kâğıt rengiyle çekilen ince bir kontur bunu
   yapıyor — ölçümde beş etiketin kutulara 1–3px girdiği görülmüştü. */
.sayfa .cizim-svg text.cizim-etiket {
  paint-order: stroke;
  stroke: var(--sayfa-yuzey);
  stroke-width: 3.5px;
  stroke-linejoin: round;
}

.sayfa code {
  font-family: var(--yazi-veri), ui-monospace, monospace;
  font-size: 0.86em;
  background: var(--color-raised);
  border: 1px solid var(--cizgi);
  border-radius: 3px;
  padding: 0.05em 0.35em;
}

/* Üstyazı: sorunun kendisi — sayfanın tek tekrar eden işareti. */
.sayfa .ustyazi {
  font-family: var(--yazi-veri), monospace;
  font-size: 11px;
  font-weight: 500;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  color: var(--mavi);
  margin: 0 0 0.75rem;
}
.sayfa .ustyazi::before {
  content: "› ";
  opacity: 0.6;
}

/* ── Kahraman ───────────────────────────────────────────────────────────── */

.sayfa .kahraman {
  padding: 3.5rem 0 3rem;
  max-width: 82ch;
}

.sayfa .baslik {
  font-family: var(--yazi-display), monospace;
  font-weight: 600;
  font-size: clamp(1.55rem, 3vw, 2.5rem);
  line-height: 1.16;
  letter-spacing: -0.045em;
  margin: 0;
  text-wrap: balance;
}
.sayfa .baslik-2 {
  color: var(--mavi);
}

.sayfa .giris {
  margin: 1.5rem 0 0;
  max-width: 62ch;
  color: var(--murekkep-2);
}

/* MUHAFAZA — sayfanın imzası: etiketi çerçevenin üzerinde duran kesikli kutu.
   Diyagramların içindeki bölge çerçevesinin aynısı; sayfa ile çizim aynı dili
   konuşuyor. */
.sayfa .muhafaza {
  position: relative;
  margin: 2.25rem 0 0;
  padding: 1.75rem 1.25rem 1.25rem;
  border: 1px dashed var(--cizgi-koyu);
  border-radius: 2px;
  background: var(--sayfa-yuzey);
}
.sayfa .muhafaza-etiket {
  position: absolute;
  top: -0.6rem;
  left: 1rem;
  padding: 0 0.5rem;
  background: var(--sayfa-yuzey);
  font-family: var(--yazi-veri), monospace;
  font-size: 10.5px;
  letter-spacing: 0.1em;
  color: var(--color-ink-3);
}

.sayfa .kosu-liste {
  margin: 0;
  padding: 0;
  list-style: none;
  font-family: var(--yazi-veri), monospace;
  font-size: 12.5px;
  line-height: 1.9;
}
.sayfa .kosu-liste li {
  display: flex;
  gap: 0.9rem;
  opacity: 0;
  animation: belir 420ms ease-out forwards;
}
.sayfa .kosu-saat {
  color: var(--color-ink-3);
  flex-shrink: 0;
}
.sayfa .kosu-metin {
  color: var(--murekkep-2);
}
.sayfa .kosu-liste li:nth-child(2) .kosu-metin {
  color: var(--mavi);
}
.sayfa .kosu-liste li:last-child .kosu-metin {
  color: var(--murekkep);
  font-weight: 500;
}

@keyframes belir {
  from {
    opacity: 0;
    transform: translateY(4px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
/* Son satırın ucunda yanıp sönen blok imleç: akış sürerken makine konuşuyor,
   bitince duruyor. Animasyon bir kez, sonsuz yanıp sönme yok. */
.sayfa .kosu-liste li:last-child .kosu-metin::after {
  content: "";
  display: inline-block;
  width: 0.5em;
  height: 1em;
  margin-left: 0.4em;
  vertical-align: -0.15em;
  background: var(--mavi);
  opacity: 0;
  animation: imlec 1.4s steps(1) 1.25s 3 forwards;
}
@keyframes imlec {
  0%, 49% { opacity: 1; }
  50%, 100% { opacity: 0; }
}

/* DAMGA — mühür. Eğik, kenarlıklı, hafif saydam; akış bitince basılıyor. */
.sayfa .damga {
  position: absolute;
  right: 1.5rem;
  bottom: 1.1rem;
  padding: 0.15rem 0.6rem;
  border: 2px solid var(--kirmizi);
  color: var(--kirmizi);
  font-family: var(--yazi-veri), monospace;
  font-size: 12px;
  font-weight: 500;
  letter-spacing: 0.28em;
  text-transform: uppercase;
  opacity: 0;
  transform: rotate(-9deg) scale(1.6);
  animation: damgala 460ms cubic-bezier(0.2, 1.4, 0.4, 1) 1.55s forwards;
}
@keyframes damgala {
  from { opacity: 0; transform: rotate(-9deg) scale(1.6); }
  60% { opacity: 0.95; transform: rotate(-9deg) scale(0.96); }
  to { opacity: 0.78; transform: rotate(-9deg) scale(1); }
}

/* Başlık satırları da yükselerek gelir — kahraman tek bir hareket olarak
   okunuyor: başlık, akış, damga. */
.sayfa .baslik { animation: yukari 520ms ease-out both; }
.sayfa .baslik-2 { display: inline-block; animation: yukari 520ms ease-out 90ms both; }
.sayfa .giris { animation: yukari 520ms ease-out 180ms both; }
@keyframes yukari {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: none; }
}

/* ÇİZİM ETKİLEŞİMİ — kutuya gelince kenarı kalınlaşır, başlığı aksana döner.
   Fare yoksa hiçbir şey kaybolmuyor: hover süs değil, okuma yardımı. */
.sayfa .cizim-svg .cizim-kutu rect,
.sayfa .cizim-svg .cizim-kutu text {
  transition:
    stroke-width 140ms ease-out,
    fill 140ms ease-out;
}
.sayfa .cizim-svg .cizim-kutu:hover rect { stroke-width: 2.2; }
.sayfa .cizim-svg .cizim-kutu:hover text { fill: var(--mavi); }

/* Bölümler kaydırıldıkça belirir — tarayıcı destekliyorsa, JavaScript'siz. */
@supports (animation-timeline: view()) {
  @media (prefers-reduced-motion: no-preference) {
    .sayfa .cizim,
    .sayfa .olcum,
    .sayfa .ikili,
    .sayfa .durust,
    .sayfa .soru-bas {
      animation: gir linear both;
      animation-timeline: view();
      animation-range: entry 8% cover 26%;
    }
  }
}
@keyframes gir {
  from { opacity: 0; transform: translateY(14px); }
  to { opacity: 1; transform: none; }
}

@media (prefers-reduced-motion: reduce) {
  .sayfa .kosu-liste li {
    animation: none;
    opacity: 1;
  }
  .sayfa .damga,
  .sayfa .baslik,
  .sayfa .baslik-2,
  .sayfa .giris {
    animation: none;
    opacity: 1;
    transform: rotate(-9deg);
  }
  .sayfa .baslik,
  .sayfa .baslik-2,
  .sayfa .giris { transform: none; }
  .sayfa .kosu-liste li:last-child .kosu-metin::after { animation: none; }
}

/* ── Sorular ────────────────────────────────────────────────────────────── */

.sayfa .soru {
  padding: 3rem 0;
  border-top: 1px solid var(--cizgi);
}
.sayfa .soru-vurgu {
  border-top-color: var(--mavi);
}

.sayfa .soru-bas {
  max-width: 74ch;
}

.sayfa .bolum-baslik {
  font-family: var(--yazi-display), monospace;
  font-weight: 500;
  font-size: clamp(1.02rem, 1.6vw, 1.4rem);
  line-height: 1.3;
  letter-spacing: -0.035em;
  margin: 0;
  text-wrap: balance;
}
.sayfa .bolum-aciklama {
  margin: 0.9rem 0 0;
  max-width: 66ch;
  color: var(--murekkep-2);
}

.sayfa .cizim {
  padding-bottom: 0.75rem;
}
.sayfa .cizim svg {
  display: block;
}

.sayfa .ikili {
  display: grid;
  gap: 1rem;
  margin-top: 2rem;
  grid-template-columns: 1fr;
}
@media (min-width: 60rem) {
  .sayfa .ikili {
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
  }
}

.sayfa .kural {
  border-left: 2px solid var(--mavi);
  padding-left: 1rem;
}
.sayfa .kural-baslik {
  margin: 0;
  font-weight: 600;
  font-size: 14px;
  letter-spacing: -0.01em;
}
.sayfa .kural-metin {
  margin: 0.4rem 0 0;
  font-size: 14px;
  color: var(--murekkep-2);
}

/* Ölçüm: iddia değil kayıt — sayı display yazıyla, yanında neyin ölçüldüğü. */
.sayfa .olcum {
  margin-top: 2rem;
  display: grid;
  gap: 0.15rem 1.5rem;
  grid-template-columns: auto 1fr;
  align-items: baseline;
}
.sayfa .olcum-deger {
  grid-row: span 2;
  margin: 0;
  font-family: var(--yazi-display), monospace;
  font-weight: 600;
  font-size: clamp(1.4rem, 2.4vw, 2rem);
  letter-spacing: -0.05em;
  color: var(--mavi);
  white-space: nowrap;
}
.sayfa .olcum-etiket {
  margin: 0;
  font-family: var(--yazi-veri), monospace;
  font-size: 11px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-ink-3);
}
.sayfa .olcum-not {
  margin: 0;
  max-width: 58ch;
  font-size: 14px;
  color: var(--murekkep-2);
}

.sayfa .durust {
  margin: 2rem 0 0;
  padding: 1rem 1.25rem;
  max-width: 76ch;
  border-left: 3px solid var(--kehribar);
  background: var(--color-raised);
  font-size: 14.5px;
}

/* ── Sınırlar ───────────────────────────────────────────────────────────── */

.sayfa .sinirlar {
  padding: 3rem 0;
  border-top: 1px solid var(--cizgi);
}

.sayfa .sinir-liste {
  margin: 2rem 0 0;
  display: grid;
  grid-template-columns: 1fr;
  border-top: 1px solid var(--cizgi);
}
@media (min-width: 60rem) {
  .sayfa .sinir-liste {
    grid-template-columns: 1fr 1fr;
    column-gap: 2.5rem;
  }
}
.sayfa .sinir {
  display: grid;
  grid-template-columns: 1fr;
  gap: 0.25rem;
  padding: 1rem 0;
  border-bottom: 1px solid var(--cizgi);
}
.sayfa .sinir dt {
  font-family: var(--yazi-veri), monospace;
  font-size: 11.5px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--murekkep);
}
.sayfa .sinir dd {
  margin: 0;
  font-size: 14px;
  color: var(--murekkep-2);
}
.sayfa .sinir-uyari dt {
  color: var(--kirmizi);
}
.sayfa .sinir-uyari dt::after {
  content: " · dikkat";
  opacity: 0.75;
}

.sayfa .kapanis {
  padding: 2.5rem 0 0;
  border-top: 1px solid var(--cizgi);
}
.sayfa .kapanis p {
  margin: 0;
  max-width: 60ch;
  font-family: var(--yazi-display), monospace;
  font-weight: 500;
  font-size: clamp(0.95rem, 1.5vw, 1.12rem);
  letter-spacing: -0.03em;
  line-height: 1.45;
}
`;
