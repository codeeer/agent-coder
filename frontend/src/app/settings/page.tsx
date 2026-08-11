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
import { RuntimeSettings } from "@/components/settings/RuntimeSettings";
import { ScriptSection } from "@/components/settings/ScriptSection";
import {
  IconChip,
  IconComment,
  IconFolder,
  IconPlay,
  IconPlug,
  IconReport,
  IconTerminal,
} from "@/components/ui/icons";
import { Notice, PageHeader, Section } from "@/components/ui/primitives";

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
const TABS = [
  { id: "models", label: "Modeller", Icon: IconChip },
  { id: "repos", label: "Kod depoları", Icon: IconFolder },
  { id: "jira", label: "Jira", Icon: IconComment },
  { id: "mcp", label: "Dış araçlar", Icon: IconPlug },
  { id: "scripts", label: "Betikler", Icon: IconTerminal },
  { id: "runner", label: "Çalıştırma", Icon: IconPlay },
  { id: "reports", label: "Rapor", Icon: IconReport },
] as const;

type TabID = (typeof TABS)[number]["id"];

export default function SettingsPage() {
  const [tab, setTab] = useState<TabID>("models");

  return (
    <div>
      <PageHeader
        title="Ayarlar"
        description="Kaydedilen gizli değerler veritabanında şifreli saklanır ve bir daha tam haliyle gösterilmez."
      />

      <div className="grid gap-6 lg:grid-cols-[188px_1fr]">
        <SettingsNav active={tab} onSelect={setTab} />
        {/* min-w-0: içerideki uzun adresler yan menüyü ezmesin. */}
        <div className="min-w-0">
          <TabContent tab={tab} />
        </div>
      </div>
    </div>
  );
}

function TabContent({ tab }: { tab: TabID }) {
  switch (tab) {
    case "models":
      return (
        <div className="space-y-8">
          <LLMProviderSection />
          <Section
            title="Model kataloğu"
            description="Model listesinin ve fiyatların ne sıklıkla tazeleneceği."
          >
            <RuntimeSettings groups={["catalog"]} showHeadings={false} />
          </Section>
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
        <div className="space-y-8">
          <McpServerSection />
          <Section
            title="Süre sınırı"
            description="Dış araç sunucularına bağlanma ve araç çağırma süresi."
          >
            <RuntimeSettings groups={["mcp"]} showHeadings={false} />
          </Section>
          <McpAccessSection />
        </div>
      );

    case "scripts":
      return <ScriptSection />;

    case "runner":
      return (
        <Section
          title="Çalıştırma"
          description="Süre sınırı, eşzamanlılık ve kaynak limitleri. Değişiklik sunucu yeniden başlatılmadan geçerli olur."
        >
          <RuntimeSettings groups={["runner"]} showHeadings={false} />
        </Section>
      );

    case "reports":
      return (
        <Section
          title="Rapor"
          description="Rapor ekranının varsayılan dönemi ve günlük kırılımın saat dilimi."
        >
          <RuntimeSettings groups={["reports"]} showHeadings={false} />
        </Section>
      );
  }
}

/**
 * Bölüm menüsü.
 *
 * Geniş ekranda dikey, dar ekranda yatay kaydırılabilir şerit. Dar ekranda da
 * dikey bırakılsaydı içeriğe sıra gelmeden yarım ekranı menü kaplardı.
 */
function SettingsNav({
  active,
  onSelect,
}: {
  active: TabID;
  onSelect: (id: TabID) => void;
}) {
  return (
    <nav
      aria-label="Ayar bölümleri"
      className="-mx-1 flex gap-1 overflow-x-auto px-1 pb-1 lg:mx-0 lg:flex-col lg:overflow-visible lg:px-0 lg:pb-0"
    >
      {TABS.map(({ id, label, Icon }) => {
        const on = id === active;
        return (
          <button
            key={id}
            type="button"
            onClick={() => onSelect(id)}
            aria-current={on ? "page" : undefined}
            className={`flex shrink-0 items-center gap-2 rounded-lg px-2.5 py-1.75 text-sm whitespace-nowrap transition-colors ${
              on
                ? "bg-accent-soft font-medium text-accent"
                : "text-ink-2 hover:bg-raised hover:text-ink"
            }`}
          >
            <Icon className="size-4 shrink-0" />
            {label}
          </button>
        );
      })}
    </nav>
  );
}

function JiraTab() {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["credentials"],
    queryFn: api.credentials.list,
  });

  return (
    <div className="space-y-8">
      <Section
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
      </Section>

      <Section
        title="Tetikleyici"
        description="Jira tetikleyicisi olan akışların ne sıklıkla taranacağı. Tarama, akış ekranındaki başlangıç düğümünden açılır."
      >
        <RuntimeSettings groups={["jira"]} showHeadings={false} />
      </Section>
    </div>
  );
}
