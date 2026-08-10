"use client";

import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import {
  CredentialCard,
  type CredentialSpec,
} from "@/components/settings/CredentialCard";
import { GitProviderSection } from "@/components/settings/GitProviderSection";
import { McpServerSection } from "@/components/settings/McpServerSection";
import { LLMProviderSection } from "@/components/settings/LLMProviderSection";
import { RuntimeSettings } from "@/components/settings/RuntimeSettings";
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

export default function SettingsPage() {
  const { data, isPending, isError, error } = useQuery({
    queryKey: ["credentials"],
    queryFn: api.credentials.list,
  });

  const jira = data?.find((c) => c.kind === "jira");

  return (
    <div>
      <PageHeader
        title="Ayarlar"
        description="Kaydedilen gizli değerler veritabanında şifreli saklanır ve bir daha tam haliyle gösterilmez."
      />

      <div className="space-y-10">
        <LLMProviderSection />

        <GitProviderSection />

        <McpServerSection />

        <Section
          title="Çalışma ayarları"
          description="Süre sınırı, eşzamanlılık ve kaynak limitleri. Değişiklik sunucu yeniden başlatılmadan geçerli olur."
        >
          <RuntimeSettings />
        </Section>

        <Section
          title="Jira"
          description="Akışları Jira task'larıyla tetiklemek için."
        >
          <div>
            {isPending && <Notice>Yükleniyor…</Notice>}
            {isError && (
              <Notice tone="error">{describeError(error).message}</Notice>
            )}
            {!isPending && !isError && (
              <CredentialCard spec={JIRA_SPEC} credential={jira} />
            )}
          </div>
        </Section>
      </div>

      <p className="mt-10 text-[12px] leading-relaxed text-ink-3">
        Not: <code>.env</code> dosyasında <code>OPENROUTER_API_KEY</code> tanımlıysa
        ve hiç LLM sağlayıcı yoksa, açılışta otomatik olarak bir OpenRouter
        sağlayıcısı oluşturulur. İstemiyorsanız o değişkeni boşaltın.
      </p>
    </div>
  );
}
