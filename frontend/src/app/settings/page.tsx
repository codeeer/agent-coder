"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import {
  CredentialCard,
  type CredentialSpec,
} from "@/components/settings/CredentialCard";
import { GitProviderSection } from "@/components/settings/GitProviderSection";
import { McpAccessSection } from "@/components/settings/McpAccessSection";
import { McpServerSection } from "@/components/settings/McpServerSection";
import { LLMProviderSection } from "@/components/settings/LLMProviderSection";
import { CACertStatus } from "@/components/settings/CACertStatus";
import { EgressStatus } from "@/components/settings/EgressStatus";
import { RuntimeSettings } from "@/components/settings/RuntimeSettings";
import { ScriptSection } from "@/components/settings/ScriptSection";
import {
  IconChip,
  IconComment,
  IconFolder,
  IconPackage,
  IconPlay,
  IconPlug,
  IconReport,
  IconShield,
  IconTerminal,
} from "@/components/ui/icons";
import {
  Notice,
  PageHeader,
  Panel,
  SearchField,
  Toolbar,
} from "@/components/ui/primitives";

/** Jira, spec 002'den sonra credentials ucunun ilgilendiği tek tür. */
const JIRA_SPEC: CredentialSpec = {
  kind: "jira",
  title: "Jira",
  description:
    "Task'ları çekmek ve sonuçları issue'ya yorum olarak yazmak için kullanılacak.",
  secretLabel: "API token",
  placeholder: "ATATT…",
  helpUrl: "https://id.atlassian.com/manage-profile/security/api-tokens",
  extraFields: [
    {
      name: "base_url",
      label: "Jira adresi",
      placeholder: "https://sirketiniz.atlassian.net",
    },
    { name: "email", label: "E-posta", placeholder: "ad@sirket.com" },
  ],
  // Token `myself` ucuna karşı gerçekten sınanıyor (credentials.Validator).
  verifies: true,
};

/*
 * Ayarlar bölümleri.
 *
 * İKİ KURAL belirledi bu listeyi:
 *
 * 1. Her ayar, AİT OLDUĞU şeyin yanında durur. "Jira tarama aralığı" Jira
 *    erişiminin altında, "MCP süre sınırı" MCP sunucularının altında. Önceden
 *    bütün davranış parametreleri tek bir "Çalışma ayarları" yığınındaydı ve
 *    kullanıcı tek bir şeyi ayarlamak için sayfanın iki ayrı yerine bakıyordu.
 *
 * 2. Bölümler alt alta değil, YAN MENÜDE. Dizildiklerinde sayfa her yeni
 *    bölümle uzuyordu; en alttaki (Jira) neredeyse görünmüyordu. Yeni bölüm
 *    eklemek artık listeye bir satır — kimsenin kaydırma mesafesi artmıyor.
 */
/*
 * Kurumsal paket deposu kimliği.
 *
 * Adres ve kullanıcı adı BURADA DEĞİL, ayarlarda: onlar sır değil ve kayıt
 * defteri kalıbı (etiket, açıklama, doğrulama) zaten onları çiziyor. Burada
 * yalnızca token durur — bu kartın tamamı "bir sır sakla" için var.
 */
const NEXUS_SPEC: CredentialSpec = {
  kind: "nexus",
  title: "Paket deposu kimliği",
  description:
    "Kayıt defteri kimlik doğrulama istiyorsa parola veya erişim token'ı. Anonim okumaya açık depolarda gerekmez.",
  secretLabel: "Parola / token",
  placeholder: "NpmToken.abc…",
  /*
   * `verifies` YOK — ve bu bilinçli.
   *
   * Nexus kurulumları farklı kimlik doğrulama uçları sunuyor; yanlış tahmin
   * edilen bir uç ÇALIŞAN bir token'ı reddederdi. Backend de bu yüzden
   * doğrulamıyor (`credentials.Validator`: nexus → nil).
   *
   * Arayüz bunu söylemek zorunda: eskiden düğme "Doğrula ve kaydet" diyor ve
   * yanına "kaydetmeden önce değerin gerçekten çalıştığı sınanır" yazıyordu.
   * Hiçbiri olmuyordu — geçersiz bir token "tanımlı" rozetiyle kaydediliyor,
   * kullanıcı doğrulanmış sanıyor, hata ancak koşuda çıkıyordu.
   */
};

const TABS = [
  { id: "models", label: "Modeller", Icon: IconChip },
  { id: "repos", label: "Git repository'ler", Icon: IconFolder },
  { id: "jira", label: "Jira", Icon: IconComment },
  { id: "mcp", label: "Dış araçlar", Icon: IconPlug },
  { id: "scripts", label: "Script'ler", Icon: IconTerminal },
  { id: "packages", label: "Package repository", Icon: IconPackage },
  { id: "runner", label: "Çalıştırma", Icon: IconPlay },
  { id: "reports", label: "Rapor", Icon: IconReport },
  { id: "network", label: "Kurumsal ağ", Icon: IconShield },
] as const;

type TabID = (typeof TABS)[number]["id"];

export default function SettingsPage() {
  const [tab, setTab] = useState<TabID>("models");
  /*
   * Arama, bölüm sekmelerinin YERİNE GEÇEN bir görünüm.
   *
   * Sorgu varken içerik sütunu bölüm içeriği yerine süzülmüş ayar listesini
   * gösterir; sorgu temizlenince seçili bölüme döner. `tab` bu sırada
   * korunuyor, yani aramadan çıkış kullanıcıyı bıraktığı yerde buluyor —
   * aksi halde her arama, gezinmeyi de sıfırlardı.
   */
  const [query, setQuery] = useState("");
  const searching = query.trim() !== "";

  return (
    /*
      Üç bölgeli düzen — arayüzün geri kalanıyla aynı: başlık üstte sabit,
      altında YAN MENÜ VE İÇERİK yan yana. Kaydırma yalnızca içerik
      sütununa ait; menü sabit kalıyor.

      Öncesinde sayfanın tamamı kayıyordu ve uzun bir bölümde (örneğin
      Betikler) menü ekranın dışına çıkıyordu: bölüm değiştirmek için önce
      yukarı kaydırmak gerekiyordu.
    */
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader
        title="Ayarlar"
        description="Kaydedilen gizli değerler veritabanında şifreli saklanır ve bir daha tam haliyle gösterilmez."
      />

      <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[212px_1fr]">
        {/* Bölüm seçimi sorguyu TEMİZLER: aramadan çıkışın doğal jesti bir
            bölüme gitmektir. Sorgu kalsaydı sekme seçili görünür ama içeriği
            gösterilmezdi. */}
        {/* Sorgu varken HİÇBİR bölüm seçili görünmez. `tab` korunuyor (arama
            temizlenince oraya dönülecek) ama işaretlenseydi menü, ekranda
            gösterilmeyen bir bölümü "bulunduğunuz yer" diye gösterirdi. */}
        <SettingsNav
          active={searching ? null : tab}
          onSelect={(id) => {
            setTab(id);
            setQuery("");
          }}
        />
        {/* min-w-0: içerideki uzun adresler yan menüyü ezmesin. */}
        <div className="flex min-h-0 min-w-0 flex-col">
          {/* Araç çubuğu kaydırma alanının DIŞINDA: uzun bir bölümde (Betikler)
              aşağı inince arama alanı ekrandan çıkmasın. */}
          <Toolbar className="shrink-0">
            <SearchField
              type="search"
              /* `hint` verilmiyor — bağlı bir kısayol yok, yazılsaydı
                 çalışmayan bir söz verilmiş olurdu (bkz. SearchField). */
              aria-label="Ayarlarda ara"
              placeholder="Ayarlarda ara…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="flex-1"
            />
          </Toolbar>
          <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1 pb-1">
            {searching ? <SearchResults query={query} /> : <TabContent tab={tab} />}
          </div>
        </div>
      </div>
    </div>
  );
}

/**
 * Arama görünümü.
 *
 * Sonuçlar bölüm başlıklarıyla gelir (`showHeadings`), çünkü "Süre sınırı"
 * diye bir sonuç tek başına hangi süreyi anlattığını söylemiyor — ayarın
 * anlamı ait olduğu bölümden geliyor.
 *
 * KAPSAM CÜMLESİ süs değil: bu ekranda ayarların yanında sağlayıcılar, MCP
 * sunucuları, betikler ve kimlik kayıtları da var. Arama onları kapsamıyor;
 * yazılmasaydı "GitHub" arayıp sonuç bulamayan kullanıcı aramayı bozuk
 * sanardı.
 */
function SearchResults({ query }: { query: string }) {
  return (
    <div className="space-y-3">
      <RuntimeSettings query={query} />
      <p className="px-1 text-xs leading-relaxed text-ink-3">
        Arama yalnızca ayarları kapsar. Sağlayıcılar, dış araç sunucuları,
        betikler ve kimlik kayıtları kendi bölümlerinde durur.
      </p>
    </div>
  );
}

function TabContent({ tab }: { tab: TabID }) {
  switch (tab) {
    case "models":
      return (
        <div className="space-y-4">
          <LLMProviderSection />
          <Panel
            title="Model kataloğu"
            description="Model listesinin ve fiyatların ne sıklıkla tazeleneceği."
            padded={false}
          >
            <RuntimeSettings groups={["catalog"]} showHeadings={false} />
          </Panel>
          <p className="text-xs leading-relaxed text-ink-3">
            Not: <code>.env</code> dosyasında <code>OPENROUTER_API_KEY</code>{" "}
            tanımlıysa ve hiç LLM sağlayıcı yoksa, açılışta otomatik olarak bir
            OpenRouter sağlayıcısı oluşturulur. İstemiyorsanız o değişkeni boşaltın.
          </p>
        </div>
      );

    case "repos":
      return <GitProviderSection />;

    case "jira":
      return <JiraTab />;

    case "mcp":
      return (
        <div className="space-y-4">
          <McpServerSection />
          <Panel
            title="Süre sınırı"
            description="Dış araç sunucularına bağlanma ve araç çağırma süresi."
            padded={false}
          >
            <RuntimeSettings groups={["mcp"]} showHeadings={false} />
          </Panel>
          <McpAccessSection />
        </div>
      );

    case "scripts":
      return <ScriptSection />;

    case "packages":
      return <PackagesTab />;

    case "runner":
      return (
        <Panel
          title="Çalıştırma"
          description="Süre sınırı, eşzamanlılık ve kaynak limitleri. Değişiklik sunucu yeniden başlatılmadan geçerli olur."
          padded={false}
        >
          <RuntimeSettings groups={["runner"]} showHeadings={false} />
        </Panel>
      );

    case "network":
      return (
        <div className="space-y-4">
          {/* TEK PANO. Durum ve alan aynı şeyin iki yüzü; ayrı panolara
              konduklarında "Kurumsal kök sertifika" başlığı üç kez tekrar
              ediyordu (iki pano başlığı + satır etiketi). */}
          <Panel
            title="Kurumsal kök sertifika"
            description="SSL denetimi yapan ağlarda gerekir. Tanımlanırsa hem agent'ın çalıştırdığı araçlar hem de Agent Coder'ın kendi giden çağrıları bu sertifikaya güvenir; genel sertifikalar geçerli kalmaya devam eder."
            padded={false}
          >
            {/* Durum ÖNCE: kullanıcının ilk sorusu "tanımlı mı ve hangisi",
                "ne yazayım" değil. */}
            <div className="border-b border-line px-4 py-3.5">
              <CACertStatus />
            </div>
            <RuntimeSettings keys={["network.corporate_ca"]} showHeadings={false} />
          </Panel>

          {/* AYRI PANO. Sertifika "SSL denetimi yapan ağda çalışabilmek" için,
              çıkış denetimi ise "sandbox nereye çıkabilsin" için. Tek panoda
              toplandıklarında pano başlığı ikisini birden anlatamıyordu. */}
          <Panel
            title="Sandbox çıkış denetimi"
            description="Agent ortamının internete nasıl ve nereye çıkabileceği. Proxy tanımlanmazsa hiçbir kısıt uygulanmaz."
            padded={false}
          >
            <div className="border-b border-line px-4 py-3.5">
              <EgressStatus />
            </div>
            <RuntimeSettings
              keys={["network.proxy_url", "network.allowed_hosts"]}
              showHeadings={false}
            />
          </Panel>
          <p className="text-xs leading-relaxed text-ink-3">
            TLS doğrulamasını kapatan bir ayar yoktur ve eklenmeyecektir. Kurumsal
            ağın çözümü kök sertifikayı tanıtmaktır.
          </p>
        </div>
      );

    case "reports":
      return (
        <Panel
          title="Rapor"
          description="Rapor ekranının varsayılan dönemi ve günlük kırılımın saat dilimi."
          padded={false}
        >
          <RuntimeSettings groups={["reports"]} showHeadings={false} />
        </Panel>
      );
  }
}

/**
 * Bölüm menüsü.
 *
 * Geniş ekranda dikey, dar ekranda yatay kaydırılabilir şerit. Dar ekranda da
 * dikey bırakılsaydı içeriğe sıra gelmeden yarım ekranı menü kaplardı.
 *
 * KENDİ YÜZEYİ VAR. Öncesinde düğmeler sayfa zemininde serbest duruyordu ve
 * yanlarındaki içerik kartlıydı: menü, sayfanın bir parçası değil de üstüne
 * unutulmuş bir şey gibi görünüyordu. Arayüzün geri kalanında gezinme her
 * zaman bir yüzeyin üstünde duruyor (kenar çubuğu, araç çubuğu) — bu da
 * öyle.
 */
function SettingsNav({
  active,
  onSelect,
}: {
  /** `null` — arama görünümündeyiz, hiçbir bölüm "bulunduğunuz yer" değil. */
  active: TabID | null;
  onSelect: (id: TabID) => void;
}) {
  return (
    <nav
      aria-label="Ayar bölümleri"
      className="-mx-1 flex shrink-0 gap-1 overflow-x-auto px-1 pb-1 lg:mx-0 lg:h-fit lg:flex-col lg:overflow-visible lg:rounded-card lg:border lg:border-line lg:bg-surface lg:p-2 lg:px-2 lg:pb-2 lg:shadow-(--shadow-card)"
    >
      {TABS.map(({ id, label, Icon }) => {
        const on = id === active;
        return (
          <button
            key={id}
            type="button"
            onClick={() => onSelect(id)}
            aria-current={on ? "page" : undefined}
            className={`group relative flex shrink-0 items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm whitespace-nowrap transition-colors duration-150 ${
              on
                ? "bg-accent-soft font-medium text-accent"
                : "text-ink-2 hover:bg-raised hover:text-ink"
            }`}
          >
            {/* Etkin bölümü soldaki şerit de işaretler — kenar çubuğundaki
                kalıbın aynısı, renk körlüğünde de okunur. Yalnızca dikey
                düzende: yatay şeritte "sol kenar" bir sıra değil, bir sınır. */}
            {on && (
              <span className="absolute top-1/2 left-0 hidden h-4 w-0.75 -translate-y-1/2 rounded-r-full bg-accent lg:block" />
            )}
            <Icon className="size-4 shrink-0" />
            {label}
          </button>
        );
      })}
    </nav>
  );
}

/*
 * Paket deposu sekmesi.
 *
 * Jira sekmesiyle aynı şekil: yapılandırma ayarlardan, sır kimlik bilgisi
 * deposundan. İkisi ayrı yerlerde saklanıyor ama kullanıcı için tek bir karar
 * olduğu için tek sekmede duruyorlar.
 */
function PackagesTab() {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["credentials"],
    queryFn: api.credentials.list,
  });

  return (
    <div className="space-y-4">
      <Panel
        title="npm kayıt defteri"
        description="Agent'ın bağımlılıkları nereden çekeceği. Boş bırakılırsa npm'in genel deposu kullanılır — kurumsal ağlarda oraya erişilemeyebilir."
        padded={false}
      >
        <RuntimeSettings groups={["packages"]} showHeadings={false} />
      </Panel>

      <Panel
        title="Kimlik doğrulama"
        description="Yalnızca kayıt defteri kimlik istiyorsa gerekir; anonim okumaya açık depolarda boş bırakın."
      >
        {isPending && <Notice>Yükleniyor…</Notice>}
        {isError && <Notice tone="error">{describeError(error).message}</Notice>}
        {!isPending && !isError && (
          <CredentialCard
            spec={NEXUS_SPEC}
            credential={data?.find((c) => c.kind === "nexus")}
          />
        )}
      </Panel>
    </div>
  );
}

function JiraTab() {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["credentials"],
    queryFn: api.credentials.list,
  });

  return (
    <div className="space-y-4">
      <Panel
        title="Jira erişimi"
        description="Akışları Jira task'larıyla tetiklemek ve sonucu issue'ya yorum olarak yazmak için."
      >
        {isPending && <Notice>Yükleniyor…</Notice>}
        {isError && <Notice tone="error">{describeError(error).message}</Notice>}
        {!isPending && !isError && (
          <CredentialCard
            spec={JIRA_SPEC}
            credential={data?.find((c) => c.kind === "jira")}
          />
        )}
      </Panel>

      <Panel
        title="Tetikleyici"
        description="Jira tetikleyicisi olan akışların ne sıklıkla taranacağı. Tarama, akış ekranındaki başlangıç düğümünden açılır."
        padded={false}
      >
        <RuntimeSettings groups={["jira"]} showHeadings={false} />
      </Panel>
    </div>
  );
}
