"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useMemo, useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import { matchesAny, needle } from "@/lib/search";
import type { Agent, LLMProvider, Model, ReportGroup, Run } from "@/lib/types";
import { ModelPicker } from "@/components/models/ModelPicker";
import { StartRunForm } from "@/components/runs/StartRunForm";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import {
  formatCompact,
  formatCount,
  formatDuration,
  formatMoney,
  formatPercent,
} from "@/components/charts/format";
import {
  IconAgent,
  IconEdit,
  IconGlobe,
  IconPlay,
  IconPlug,
  IconPlus,
  IconTerminal,
  IconTrash,
  IconUndo,
} from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  Checkbox,
  EmptyState,
  IconTile,
  Input,
  Metric,
  Notice,
  PageHeader,
  Panel,
  SearchField,
  Segmented,
  Skeleton,
  Textarea,
  Toolbar,
  Well,
  formatRelative,
  panelLinkClass,
  ConfirmInline,
} from "@/components/ui/primitives";

/**
 * Agent'lar — tanım ekranı değil, SEÇİM ekranı.
 *
 * ÖNCEKİ HALİ DÜZ BİR LİSTEYDİ ve iki soruya da cevap vermiyordu:
 *
 *   1. "Bu iş için hangi agent'ı çalıştırayım?" Altı satır aynı görsel
 *      ağırlıktaydı; hangisinin gerçekten kullanıldığı, ne kadara mal
 *      olduğu, ne kadarının tuttuğu ekranda YOKTU — oysa bu veri
 *      `reports.byAgent` içinde zaten duruyor ve rapor ekranı onu
 *      gösteriyordu. Agent'ı seçen kişi ise burada.
 *
 *   2. "Bu agent'ı nasıl ayarlarım?" Talimat, yetkiler ve araçlar ancak
 *      satırın içinde açılan devasa bir formda görünüyordu; form açılınca
 *      liste aşağı kayıyor ve karşılaştırma imkânı kayboluyordu.
 *
 * Şimdi master–detay: solda taranan liste, sağda seçilenin künyesi.
 * Formlar da detay sütununda açılıyor — liste hiç oynamıyor, yani
 * "şunu mu bunu mu" karşılaştırması form açıkken de sürüyor.
 */

const PAGE_SIZE = 25;

/** Kullanım rakamlarının dönemi. Rapor ekranının varsayılanıyla aynı. */
const USAGE_DAYS = 30;

const FILTERS = [
  { id: "all", label: "Tümü" },
  { id: "builtin", label: "Hazır" },
  { id: "custom", label: "Özel" },
  { id: "modified", label: "Değiştirilmiş" },
] as const;

type FilterId = (typeof FILTERS)[number]["id"];

function matches(a: Agent, filter: FilterId): boolean {
  switch (filter) {
    case "builtin":
      return a.source === "builtin";
    case "custom":
      return a.source === "custom";
    case "modified":
      return a.isModified;
    default:
      return true;
  }
}

/** Detay sütununun ne gösterdiği. Aynı anda yalnızca biri açık olabilir. */
type Mode = "view" | "edit" | "run" | "create";

export default function AgentsPage() {
  const [offset, setOffset] = useState(0);
  const [q, setQ] = useState("");
  const [filter, setFilter] = useState<FilterId>("all");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [mode, setMode] = useState<Mode>("view");

  const agents = useQuery({
    queryKey: ["agents", offset],
    queryFn: () => api.agents.list({ limit: PAGE_SIZE, offset }),
  });
  const providers = useQuery({
    queryKey: ["llm-providers"],
    queryFn: api.llmProviders.list,
  });
  // Model seçici için araç destekleyenler ve bilinmeyenler; desteklemeyenler
  // agent olarak kullanılamaz, listelenmeleri kafa karıştırır.
  const models = useQuery({
    queryKey: ["models", "agent-picker"],
    queryFn: () => api.models.list({ limit: 500, sort: "provider" }),
  });

  /*
   * Kullanım rakamları raporun kendi ucundan geliyor — yeni bir uç YOK.
   * `byAgent` satırlarının anahtarı çalıştırma kaydındaki agent slug'ı,
   * yani agent listesiyle doğrudan eşleşiyor.
   */
  const report = useQuery({
    queryKey: ["report", "agents", USAGE_DAYS],
    queryFn: () => api.reports.summary({ days: USAGE_DAYS }),
  });

  // Son çalıştırmalar penceresi: seçili agent'ın son işleri bundan süzülüyor.
  const runs = useQuery({
    queryKey: ["runs", "agents-panel"],
    queryFn: () => api.runs.list({ limit: 100 }),
    refetchInterval: (query) =>
      query.state.data?.items.some((r) => isActive(r.status)) ? 5000 : false,
  });

  const mcpServers = useQuery({
    queryKey: ["mcp-servers"],
    queryFn: api.mcpServers.list,
  });
  const scripts = useQuery({
    queryKey: ["scripts", "all"],
    queryFn: () => api.scripts.list({ limit: 200 }),
  });
  const items = useMemo(() => {
    const rows = agents.data?.items ?? [];
    const n = needle(q);
    return rows.filter(
      (a) =>
        matches(a, filter) &&
        matchesAny([a.name, a.slug, a.description, a.defaultModel], n),
    );
  }, [agents.data, filter, q]);

  // Seçim listeden düşerse (süzgeç, arama, silme) ilk sıradakine kayar:
  // detay sütununun boş kalması, ekranın yarısını boşa harcamak olurdu.
  const selected = items.find((a) => a.id === selectedId) ?? items[0] ?? null;

  function select(agent: Agent) {
    setSelectedId(agent.id);
    setMode("view");
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <PageHeader
        title="Agent'lar"
        description="Her agent bir talimat, bir varsayılan model ve bir yetki kümesidir."
        actions={
          <Button
            variant="primary"
            onClick={() => setMode("create")}
            icon={<IconPlus className="size-4" />}
          >
            Agent oluştur
          </Button>
        }
      />

      {agents.isPending && <Skeleton rows={4} />}
      {agents.isError && (
        <Notice tone="error">{describeError(agents.error).message}</Notice>
      )}

      {agents.data && agents.data.total === 0 && mode !== "create" && (
        <EmptyState
          icon={<IconAgent className="size-4" />}
          title="Henüz agent yok"
          description="Hazır agent'lar kurulumda gelir. Hiçbiri görünmüyorsa backend ilk açılışını tamamlamamış olabilir."
          action={
            <Button variant="primary" onClick={() => setMode("create")}>
              İlk agent&apos;ı oluştur
            </Button>
          }
        />
      )}

      {agents.data && agents.data.total > 0 && (
        <>
          <Toolbar>
            <SearchField
              className="min-w-50 flex-1 sm:max-w-xs"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Agent, slug veya model ara…"
              aria-label="Agent'larda ara"
            />
            <Segmented
              label="Kaynak süzgeci"
              options={FILTERS}
              value={filter}
              onChange={setFilter}
            />
            <span className="ml-auto hidden text-2xs text-ink-3 lg:block">
              {items.length === agents.data.items.length
                ? `${items.length} agent`
                : `${items.length} / ${agents.data.items.length} agent`}
            </span>
          </Toolbar>

          {/*
            İki sütun: solda seçim, sağda karar.

            Dar ekranda alt alta geçiyor ve liste kendi yüksekliğiyle
            sınırlanıyor (`max-h`): telefonda tam boy bir liste, detayın
            hiç görünmemesi demek olurdu.
          */}
          <div className="grid min-h-0 flex-1 gap-4 lg:grid-cols-[340px_1fr]">
            <AgentList
              agents={items}
              /* Yeni agent formu açıkken listede seçili satır YOK: detay
                 sütunu artık o agent'ı göstermiyor ve vurgulu satır
                 kullanıcıya yanlış şeyi işaret ederdi. */
              selectedId={mode === "create" ? null : (selected?.id ?? null)}
              onSelect={select}
              usage={report.data?.byAgent ?? []}
            />

            <div className="-mx-1 min-h-0 overflow-y-auto px-1 pb-1">
              {mode === "create" ? (
                <AgentForm
                  providers={providers.data ?? []}
                  models={models.data?.items ?? []}
                  onDone={() => setMode("view")}
                />
              ) : selected === null ? (
                <Card>
                  <p className="text-sm text-ink-3">
                    Bu süzgece uyan agent yok.
                  </p>
                </Card>
              ) : mode === "edit" ? (
                <AgentForm
                  agent={selected}
                  providers={providers.data ?? []}
                  models={models.data?.items ?? []}
                  onDone={() => setMode("view")}
                />
              ) : mode === "run" ? (
                <StartRunForm agent={selected} onDone={() => setMode("view")} />
              ) : (
                <AgentDetail
                  agent={selected}
                  provider={providers.data?.find(
                    (p) => p.id === selected.defaultProviderId,
                  )}
                  model={models.data?.items.find(
                    (m) =>
                      m.id === selected.defaultModel &&
                      m.providerId === selected.defaultProviderId,
                  )}
                  usage={report.data?.byAgent.find(
                    (g) => g.key === selected.slug,
                  )}
                  runs={
                    runs.data?.items.filter(
                      (r) => r.agentSlug === selected.slug,
                    ) ?? []
                  }
                  mcpNames={(mcpServers.data ?? [])
                    .filter((s) => selected.mcpServerIds.includes(s.id))
                    .map((s) => s.name)}
                  scriptNames={(scripts.data?.items ?? [])
                    .filter((s) => selected.scriptIds.includes(s.id))
                    .map((s) => s.name)}
                  onRun={() => setMode("run")}
                  onEdit={() => setMode("edit")}
                />
              )}
            </div>
          </div>

          <Pagination
            total={agents.data.total}
            limit={agents.data.limit}
            offset={agents.data.offset}
            onChange={(next) => {
              setOffset(next);
              setQ("");
              setSelectedId(null);
            }}
            unit="agent"
          />
        </>
      )}
    </div>
  );
}

/* ── Liste sütunu ────────────────────────────────────────────────────────── */

function AgentList({
  agents,
  selectedId,
  onSelect,
  usage,
}: {
  agents: Agent[];
  selectedId: string | null;
  onSelect: (a: Agent) => void;
  usage: ReportGroup[];
}) {
  return (
    <div className="flex max-h-80 min-h-0 flex-col overflow-hidden rounded-card border border-line bg-surface shadow-(--shadow-card) lg:max-h-none">
      <ul className="min-h-0 flex-1 divide-y divide-line overflow-y-auto">
        {agents.map((a) => {
          const on = a.id === selectedId;
          const stats = usage.find((g) => g.key === a.slug);

          return (
            <li key={a.id}>
              <button
                type="button"
                onClick={() => onSelect(a)}
                aria-current={on ? "true" : undefined}
                className={`relative flex w-full items-start gap-3 px-3.5 py-3 text-left transition-colors ${
                  on ? "bg-accent-soft" : "hover:bg-raised"
                }`}
              >
                {/* Seçili satırı soldaki şerit de işaretler — arayüzün geri
                    kalanındaki (kenar çubuğu, ayarlar menüsü) kalıbın aynısı. */}
                {on && (
                  <span className="absolute top-1/2 left-0 h-8 w-0.75 -translate-y-1/2 rounded-r-full bg-accent" />
                )}

                {/*
                  Monogram — simge değil.

                  Altı satırda aynı agent simgesi dursaydı hiçbir şeyi ayırt
                  etmez, yalnızca yer doldururdu. Harf ayırt ediyor.

                  `toLocaleUpperCase("tr")`: Türkçede "i" harfinin büyüğü
                  "İ"; öntanımlı büyütme "I" verir ve monogram yanlış harfi
                  gösterirdi.
                */}
                <span
                  className={`mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-lg border text-xs font-semibold select-none ${
                    on
                      ? "border-accent/30 bg-surface text-accent"
                      : "border-line bg-raised text-ink-2"
                  }`}
                >
                  {a.name.charAt(0).toLocaleUpperCase("tr")}
                </span>

                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-2">
                    <span className="truncate text-sm font-medium">
                      {a.name}
                    </span>
                    {a.source === "custom" && <Badge tone="info">özel</Badge>}
                    {a.isModified && <Badge tone="warning">değişik</Badge>}
                  </span>

                  <span className="mt-0.5 block truncate font-mono text-2xs text-ink-3">
                    {a.slug}
                  </span>

                  <span className="mt-2 flex items-center gap-2">
                    <CapabilityStrip agent={a} />
                    {/* Kullanım rakamı yalnızca VARSA. "0 iş" yazmak,
                        çalıştırılmamış bir agent'ı başarısız gibi gösterir;
                        hazır agent'ların çoğu hiç çalıştırılmamış olabilir. */}
                    {stats && (
                      <span className="ml-auto shrink-0 text-2xs text-ink-3 tabular-nums">
                        {formatCount(stats.runs)} iş
                      </span>
                    )}
                  </span>
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

/* ── Yetki şeridi ────────────────────────────────────────────────────────── */

/**
 * Üç sabit yuva: dosya, komut, ağ.
 *
 * Öncesinde yalnızca AÇIK yetkiler rozet olarak yazılıyordu; liste boyunca
 * her satırda farklı sayıda rozet olunca sütun hizası kayıyordu ve iki
 * agent'ın yetkileri ancak etiketler okunarak karşılaştırılabiliyordu.
 *
 * Sabit üç yuva bunu tabloya çeviriyor: aynı yetki her satırda AYNI yerde
 * duruyor, kapalı olan sönük. Göz artık okumadan karşılaştırıyor.
 *
 * Renk anlamlı: açık bir yazma/çalıştırma yetkisi bir RİSKTİR, yeşil
 * değil amber. "Hiçbir şeyi değiştiremez" ise gerçekten iyi haberdir ve
 * onu detay panelindeki "yalnızca okur" satırı söylüyor.
 */
function CapabilityStrip({ agent }: { agent: Agent }) {
  const caps = [
    {
      on: agent.allowEdit,
      Icon: IconEdit,
      label: "Dosya değiştirebilir",
      tone: "text-warn",
    },
    {
      on: agent.allowBash,
      Icon: IconTerminal,
      label: "Komut çalıştırabilir",
      tone: "text-warn",
    },
    {
      on: agent.allowWebfetch,
      Icon: IconGlobe,
      label: "Ağa erişebilir",
      tone: "text-info",
    },
  ];

  return (
    <span className="flex items-center gap-1.5">
      {caps.map(({ on, Icon, label, tone }) => (
        <span
          key={label}
          title={on ? label : `${label} — kapalı`}
          className={on ? tone : "text-ink-3/40"}
        >
          <Icon className="size-3.5" />
        </span>
      ))}
      {agent.mcpServerIds.length > 0 && (
        <span
          title={`${agent.mcpServerIds.length} dış araç sunucusu (MCP)`}
          className="flex items-center gap-0.5 text-info"
        >
          <IconPlug className="size-3.5" />
          <span className="text-2xs tabular-nums">
            {agent.mcpServerIds.length}
          </span>
        </span>
      )}
    </span>
  );
}

/* ── Detay sütunu ────────────────────────────────────────────────────────── */

function AgentDetail({
  agent,
  provider,
  model,
  usage,
  runs,
  mcpNames,
  scriptNames,
  onRun,
  onEdit,
}: {
  agent: Agent;
  provider?: LLMProvider;
  model?: Model;
  usage?: ReportGroup;
  runs: Run[];
  mcpNames: string[];
  scriptNames: string[];
  onRun: () => void;
  onEdit: () => void;
}) {
  const queryClient = useQueryClient();
  const [confirming, setConfirming] = useState(false);

  const invalidate = () =>
    void queryClient.invalidateQueries({ queryKey: ["agents"] });

  const reset = useMutation({
    mutationFn: () => api.agents.reset(agent.id),
    onSuccess: invalidate,
  });
  const remove = useMutation({
    mutationFn: () => api.agents.remove(agent.id),
    onSuccess: () => {
      invalidate();
      setConfirming(false);
    },
  });

  return (
    <div className="space-y-4">
      {/* ── Künye ── */}
      <Card>
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex min-w-0 items-start gap-3">
            <IconTile tone="accent">
              <IconAgent className="size-4" />
            </IconTile>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="text-lg font-semibold tracking-[-0.02em]">
                  {agent.name}
                </h2>
                {agent.source === "custom" ? (
                  <Badge tone="info">özel</Badge>
                ) : (
                  <Badge>hazır</Badge>
                )}
                {agent.isModified && (
                  <Badge tone="warning">değiştirilmiş</Badge>
                )}
              </div>
              <p className="mt-0.5 flex flex-wrap items-center gap-x-2 text-xs text-ink-3">
                <span className="font-mono">{agent.slug}</span>
                <span aria-hidden="true">·</span>
                {/* Talimatı değiştirilen bir agent'ın NE ZAMAN değiştiği,
                    "değiştirilmiş" rozetinden daha çok şey söyler. */}
                <span>güncellendi {formatRelative(agent.updatedAt)}</span>
              </p>
              {agent.description && (
                <p className="mt-2 max-w-prose text-sm leading-relaxed text-ink-2">
                  {agent.description}
                </p>
              )}
            </div>
          </div>

          {/*
            Eylem hiyerarşisi RENKLE değil DOLGUYLA kuruluyor.

            Tek dolu düğme "Çalıştır" — bu ekranın asıl eylemi. Diğerleri
            kenarlıklı: düğme oldukları belli ama birincil eylemle
            yarışmıyorlar. Öncesinde bunlar liste satırında hover'da
            açılıyordu; detay sütununda yer var, saklamaya gerek yok.
          */}
          {confirming ? (
            <div className="shrink-0">
              {/* Adı yazılıyor: detay sütunu seçili agent'ı gösteriyor ama
                  onay metni tek başına da okunabilir olmalı. */}
              <ConfirmInline
                question={
                  <>
                    <strong>{agent.name}</strong> silinsin mi?
                  </>
                }
                busy={remove.isPending}
                onConfirm={() => remove.mutate()}
                onCancel={() => setConfirming(false)}
              />
            </div>
          ) : (
            <div className="flex shrink-0 flex-wrap items-center gap-2">
              <Button
                variant="primary"
                onClick={onRun}
                icon={<IconPlay className="size-4" />}
              >
                Çalıştır
              </Button>
              <Button onClick={onEdit} icon={<IconEdit className="size-4" />}>
                Düzenle
              </Button>
              {agent.isModified && (
                <Button
                  onClick={() => reset.mutate()}
                  disabled={reset.isPending}
                  icon={<IconUndo className="size-4" />}
                  title="Hazır agent'ı özgün talimatına döndürür"
                >
                  {reset.isPending ? "…" : "Sıfırla"}
                </Button>
              )}
              {agent.source === "custom" && (
                <Button
                  variant="danger"
                  onClick={() => setConfirming(true)}
                  icon={<IconTrash className="size-4" />}
                >
                  Sil
                </Button>
              )}
            </div>
          )}
        </div>

        {remove.isError && (
          <p className="mt-3 text-sm text-danger">
            {describeError(remove.error).message}
          </p>
        )}

        {/* ── Kullanım ── */}
        <div className="mt-4 grid grid-cols-2 gap-4 border-t border-line pt-4 sm:grid-cols-4">
          <Metric
            label={`Çalıştırma (${USAGE_DAYS}g)`}
            value={usage ? formatCount(usage.runs) : "—"}
            note={
              usage ? `${formatCount(usage.succeeded)} tamamlandı` : "kayıt yok"
            }
          />
          <Metric
            label="Başarı"
            value={usage ? formatPercent(usage.succeeded, usage.runs) : "—"}
            note={
              usage && usage.failed > 0
                ? `${usage.failed} başarısız`
                : undefined
            }
            tone={
              usage && usage.runs > 0
                ? usage.succeeded / usage.runs >= 0.8
                  ? "ok"
                  : "warn"
                : undefined
            }
          />
          <Metric
            label="Maliyet"
            value={usage ? formatMoney(usage.costUsd) : "—"}
            note={usage ? `${formatCompact(usage.tokens)} token` : undefined}
          />
          <Metric
            label="Ort. süre"
            value={usage ? formatDuration(usage.avgDurationSec) : "—"}
            note={
              usage
                ? `${formatCompact(usage.filesChanged)} dosya değişti`
                : undefined
            }
          />
        </div>
      </Card>

      {/*
        İki sütun ancak GERÇEKTEN yer varken (2xl).

        `xl`'de denendi ve olmadı: soldaki liste zaten 340px alıyor, 1280px'lik
        bir ekranda detaya kalan ~640px ikiye bölününce talimat sütunu 350px'e
        düşüyor ve tek aralıklı metin her satırda sarıyordu — talimat, bu
        ekranın en çok okunan içeriği.

        `min-w-0` şart: ızgara sütunlarının varsayılan alt sınırı `auto`, yani
        içeriklerinin doğal genişliği. Sağdaki model kimliği (uzun, kırılmayan
        tek aralıklı bir metin) sütunu şişiriyor ve talimat sütununu birkaç
        piksele eziyordu.
      */}
      <div className="grid items-start gap-4 2xl:grid-cols-[1.3fr_1fr]">
        {/* ── Talimat ── */}
        <Panel
          className="min-w-0"
          title="Talimat"
          action={
            <span className="text-2xs text-ink-3 tabular-nums">
              {formatCount(agent.prompt.length)} karakter
            </span>
          }
          padded={false}
        >
          {/*
            Talimat GİZLENMİYOR.

            Öncesinde "Talimatı gör" düğmesinin ardındaydı. Oysa bir agent'ın
            ne olduğu talimatıdır — adı ve açıklaması onun özeti. Ekranın
            asıl içeriğini bir tıklamanın arkasına koymak, ekranı boş
            gösteriyordu.
          */}
          <pre className="max-h-105 overflow-auto px-4 py-3.5 font-mono text-xs leading-relaxed whitespace-pre-wrap">
            {agent.prompt}
          </pre>
        </Panel>

        <div className="min-w-0 space-y-4">
          {/* ── Model ── */}
          <Panel title="Varsayılan model">
            {agent.defaultModel ? (
              <div className="space-y-3">
                <div className="min-w-0">
                  <p className="truncate font-mono text-sm">
                    {agent.defaultModel}
                  </p>
                  <p className="mt-0.5 text-xs text-ink-3">
                    {provider?.name ?? "sağlayıcı bulunamadı"}
                  </p>
                </div>

                {model ? (
                  <dl className="grid grid-cols-3 gap-3 border-t border-line pt-3">
                    <Metric
                      label="Bağlam"
                      value={
                        model.contextLength === null
                          ? "—"
                          : formatCompact(model.contextLength)
                      }
                      size="sm"
                    />
                    <Metric
                      label="Girdi /M"
                      value={formatMoney(model.promptPricePerMTok)}
                      size="sm"
                    />
                    <Metric
                      label="Çıktı /M"
                      value={formatMoney(model.completionPricePerMTok)}
                      size="sm"
                    />
                  </dl>
                ) : (
                  /* Katalogda yoksa sessiz geçilmez: model silinmiş ya da
                     sağlayıcı değişmiş olabilir ve agent çalışmaz. */
                  <Notice tone="warning">
                    Bu model katalogda bulunamadı. Sağlayıcı kaldırılmış veya
                    katalog güncellenmemiş olabilir.
                  </Notice>
                )}

                {model?.supportsTools === false && (
                  <Notice tone="warning">
                    Model araç çağıramıyor — bu agent dosya okuyup değiştiremez.
                  </Notice>
                )}
              </div>
            ) : (
              <p className="text-sm text-ink-3">
                Seçilmemiş — her çalıştırmada model ayrıca seçilir.
              </p>
            )}
          </Panel>

          {/* ── Yetkiler ── */}
          <Panel title="Yetkiler ve araçlar">
            <ul className="divide-y divide-line">
              <PermissionRow
                Icon={IconEdit}
                label="Dosya değiştirebilir"
                on={agent.allowEdit}
                note="Depodaki dosyaları yazabilir."
              />
              <PermissionRow
                Icon={IconTerminal}
                label="Komut çalıştırabilir"
                on={agent.allowBash}
                note="Container içinde kabuk komutu çalıştırır."
              />
              <PermissionRow
                Icon={IconGlobe}
                label="Ağa erişebilir"
                on={agent.allowWebfetch}
                note="Dış adreslerden içerik çekebilir."
              />
            </ul>

            {/* Yetkisiz agent bir EKSİKLİK değil, güvenlik özelliği — ve bu
                cümlenin kaybolmaması gerekiyor. */}
            {!agent.allowEdit && !agent.allowBash && !agent.allowWebfetch && (
              <p className="mt-3 text-xs text-ok">
                Bu agent hiçbir şeyi değiştiremez; yalnızca okur.
              </p>
            )}

            <div className="mt-4 space-y-3 border-t border-line pt-3">
              <ToolList
                label="MCP Server'lar"
                names={mcpNames}
                empty="Hiçbir dış araç sunucusu açılmamış."
              />
              <ToolList
                label="Script'ler"
                names={scriptNames}
                empty="Hazır betik atanmamış."
                mono
                /* Komut yetkisi kapalıyken betik container'a hiç kopyalanmaz;
                   yazılmasaydı kullanıcı agent'ın betiği neden çağırmadığını
                   hiçbir hata mesajı olmadan arardı. */
                warning={
                  !agent.allowBash && scriptNames.length > 0
                    ? "Komut çalıştırma yetkisi kapalı — betikler ortama kopyalanmaz."
                    : undefined
                }
              />
            </div>
          </Panel>

          {/* ── Son çalıştırmalar ── */}
          <Panel
            title="Son çalıştırmalar"
            action={
              <Link href="/runs" className={panelLinkClass}>
                Tümü
              </Link>
            }
            padded={false}
          >
            {runs.length === 0 ? (
              /* "Hiç çalışmadı" DENMİYOR: liste yalnızca son 100 çalıştırmaya
                 bakıyor, daha eskisini görmüyor. Bilmediğimiz şeyi iddia
                 etmiyoruz. */
              <p className="px-4 py-3.5 text-sm text-ink-3">
                Yakın geçmişte bu agent&apos;ın çalıştırması yok.
              </p>
            ) : (
              <ul className="divide-y divide-line">
                {runs.slice(0, 5).map((r) => (
                  <li key={r.id}>
                    <Link
                      href={`/runs/${r.id}`}
                      className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-raised"
                    >
                      <span className="shrink-0">
                        <RunStatusBadge status={r.status} />
                      </span>
                      <span className="min-w-0 flex-1 truncate text-xs">
                        {r.task}
                      </span>
                      <span className="shrink-0 text-2xs text-ink-3">
                        {formatRelative(r.createdAt)}
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            )}
          </Panel>
        </div>
      </div>
    </div>
  );
}

function PermissionRow({
  Icon,
  label,
  on,
  note,
}: {
  Icon: (p: { className?: string }) => React.ReactElement;
  label: string;
  on: boolean;
  note: string;
}) {
  return (
    <li className="flex items-start gap-3 py-2.5 first:pt-0 last:pb-0">
      <span className={`mt-0.5 shrink-0 ${on ? "text-warn" : "text-ink-3/40"}`}>
        <Icon className="size-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className={`block text-sm ${on ? "font-medium" : "text-ink-3"}`}>
          {label}
        </span>
        {on && <span className="mt-0.5 block text-2xs text-ink-3">{note}</span>}
      </span>
      <span className="shrink-0 text-2xs text-ink-3">
        {on ? "açık" : "kapalı"}
      </span>
    </li>
  );
}

function ToolList({
  label,
  names,
  empty,
  mono = false,
  warning,
}: {
  label: string;
  names: string[];
  empty: string;
  mono?: boolean;
  warning?: string;
}) {
  return (
    <div>
      <p className="text-2xs font-medium tracking-wide text-ink-3 uppercase">
        {label}
      </p>
      {names.length === 0 ? (
        <p className="mt-1 text-xs text-ink-3">{empty}</p>
      ) : (
        <div className="mt-1.5 flex flex-wrap gap-1.5">
          {names.map((n) => (
            <span
              key={n}
              className={`rounded-md border border-line bg-raised px-1.5 py-px text-2xs ${
                mono ? "font-mono" : ""
              }`}
            >
              {n}
            </span>
          ))}
        </div>
      )}
      {warning && <p className="mt-1.5 text-2xs text-warn">{warning}</p>}
    </div>
  );
}

/* ── Form ────────────────────────────────────────────────────────────────── */

function AgentForm({
  agent,
  providers,
  models,
  onDone,
}: {
  agent?: Agent;
  providers: LLMProvider[];
  models: Model[];
  onDone: () => void;
}) {
  const queryClient = useQueryClient();
  const editing = agent !== undefined;

  const [name, setName] = useState(agent?.name ?? "");
  const [description, setDescription] = useState(agent?.description ?? "");
  const [prompt, setPrompt] = useState(agent?.prompt ?? "");
  // Varsayılan model (sağlayıcı, model) çiftidir; ikisi birlikte seçilir.
  const [model, setModel] = useState<{
    providerId: string;
    modelId: string;
  } | null>(
    agent?.defaultModel && agent.defaultProviderId
      ? { providerId: agent.defaultProviderId, modelId: agent.defaultModel }
      : null,
  );
  const [allowEdit, setAllowEdit] = useState(agent?.allowEdit ?? true);
  const [allowBash, setAllowBash] = useState(agent?.allowBash ?? true);
  const [allowWebfetch, setAllowWebfetch] = useState(
    agent?.allowWebfetch ?? false,
  );
  const [mcpIds, setMcpIds] = useState<string[]>(agent?.mcpServerIds ?? []);
  const [scriptIds, setScriptIds] = useState<string[]>(agent?.scriptIds ?? []);
  const [scriptFolderIds, setScriptFolderIds] = useState<string[]>(
    agent?.scriptFolderIds ?? [],
  );

  const mcpServers = useQuery({
    queryKey: ["mcp-servers"],
    queryFn: api.mcpServers.list,
  });
  // Seçim listesi olduğu için tek sayfa yetmez; sınır yüksek tutuluyor.
  const scripts = useQuery({
    queryKey: ["scripts", "all"],
    queryFn: () => api.scripts.list({ limit: 200 }),
  });
  const scriptFolders = useQuery({
    queryKey: ["script-folders"],
    queryFn: api.scriptFolders.list,
  });

  // Klasörden gelen betikler ayrıca işaretlenmez: onlar klasörün üyeliğiyle
  // geliyor ve tekil kutucuk olarak gösterilseydi kullanıcı onları
  // "çıkarabileceğini" sanırdı.
  const klasordenGelen = new Set(
    (scripts.data?.items ?? [])
      .filter((s) => s.folderId && scriptFolderIds.includes(s.folderId))
      .map((s) => s.id),
  );

  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.agents.update(agent.id, {
            name: name.trim(),
            description: description.trim(),
            prompt,
            defaultProviderId: model?.providerId,
            clearProvider: model === null,
            defaultModel: model?.modelId ?? "",
            allowEdit,
            allowBash,
            allowWebfetch,
            mcpServerIds: mcpIds,
            scriptIds,
            scriptFolderIds,
          })
        : api.agents.create({
            name: name.trim(),
            description: description.trim(),
            prompt,
            defaultProviderId: model?.providerId,
            defaultModel: model?.modelId ?? "",
            allowEdit,
            allowBash,
            allowWebfetch,
          }),
    onSuccess: async (saved) => {
      // Oluşturma ucu atamaları almıyor; yeni agent kaydedildikten sonra
      // ikinci bir çağrıyla yazılırlar. Aksi halde kullanıcı formda seçim
      // yapar, kaydeder ve seçimi kaybolurdu.
      if (
        !editing &&
        (mcpIds.length > 0 || scriptIds.length > 0 || scriptFolderIds.length > 0)
      ) {
        await api.agents.update(saved.id, {
          mcpServerIds: mcpIds,
          scriptIds,
          scriptFolderIds,
        });
      }
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
      onDone();
    },
  });

  const selected = models.find(
    (m) => m.id === model?.modelId && m.providerId === model.providerId,
  );
  const canSubmit = name.trim() !== "" && prompt.trim() !== "";

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (canSubmit) save.mutate();
      }}
    >
      {/*
        Form ÜÇ PANOYA bölündü: kimlik, talimat, yetki ve araçlar.

        Öncesinde tek bir kartın içinde on bir alan alt alta diziliydi ve
        aralarındaki tek ayrım `fieldset` kenarlıklarıydı. Bir agent'ı
        düzenleyen kişi genellikle TEK bir şeyi değiştirmek ister; hangi
        bölümde olduğunu görmesi gerekiyor.
      */}
      <Panel title={editing ? `${agent.name} — düzenle` : "Yeni agent"}>
        <div className="flex flex-wrap gap-3">
          <label className="block min-w-48 flex-1">
            <span className="text-2xs font-medium tracking-wide text-ink-2 uppercase">
              Ad
            </span>
            <Input
              className="mt-1"
              value={name}
              placeholder="Kod İncelemecisi"
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label className="block min-w-64 flex-2">
            <span className="text-2xs font-medium tracking-wide text-ink-2 uppercase">
              Açıklama
            </span>
            <Input
              className="mt-1"
              value={description}
              placeholder="Ne yaptığını bir cümleyle anlatın"
              onChange={(e) => setDescription(e.target.value)}
            />
          </label>
        </div>

        <div className="mt-3">
          <span className="text-2xs font-medium tracking-wide text-ink-2 uppercase">
            Varsayılan model
          </span>
          <div className="mt-1">
            <ModelPicker
              models={models}
              providerId={model?.providerId ?? null}
              modelId={model?.modelId ?? ""}
              onChange={setModel}
              emptyLabel="Model ara… (seçilmezse her çalıştırmada seçilir)"
            />
          </div>
        </div>

        {/* Araç desteği agent'ın çalışıp çalışmayacağını belirler; sessiz geçilmez. */}
        {selected && selected.supportsTools === false && (
          <div className="mt-3">
            <Notice tone="warning">
              Seçilen model araç çağıramıyor. Agent dosya okuyup
              değiştiremeyeceği için büyük olasılıkla işe yaramaz.
            </Notice>
          </div>
        )}
        {selected && selected.supportsTools === null && (
          <div className="mt-3">
            <Notice tone="warning">
              Sağlayıcı bu modelin araç desteğini bildirmiyor. Çalışabilir ama
              garanti değil.
            </Notice>
          </div>
        )}
      </Panel>

      <Panel
        title="Talimat"
        description="Agent'a verilen sistem yönergesi. Ne yapacağını, neyi yapmayacağını ve çıktıyı nasıl vereceğini burası belirler."
        padded={false}
      >
        <Textarea
          className="h-72 rounded-none border-0 bg-transparent font-mono text-xs leading-relaxed focus:border-0"
          value={prompt}
          placeholder="Sen bir kod incelemecisisin. …"
          onChange={(e) => setPrompt(e.target.value)}
          aria-label="Talimat"
        />
      </Panel>

      <Panel
        title="Yetkiler ve araçlar"
        description="Agent'ın çalıştırma ortamında neye erişebileceği. Kapalı bir yetki, o yeteneğin ortama hiç girmemesi demektir."
      >
        <div className="flex flex-wrap gap-x-6 gap-y-2">
          <Checkbox
            label="Dosya değiştirebilir"
            checked={allowEdit}
            onChange={setAllowEdit}
          />
          <Checkbox
            label="Komut çalıştırabilir"
            checked={allowBash}
            onChange={setAllowBash}
          />
          <Checkbox
            label="Ağa erişebilir"
            checked={allowWebfetch}
            onChange={setAllowWebfetch}
          />
        </div>

        <div className="mt-4 grid gap-4 border-t border-line pt-4 md:grid-cols-2">
          {/* Dış araçlar da bir YETKİDİR: bu agent'ın neye erişebildiğinin
              parçası. Ayrı kutuda çünkü listesi her kurulumda farklı. */}
          <div>
            <p className="text-2xs font-medium tracking-wide text-ink-2 uppercase">
              Dış araçlar (MCP)
            </p>
            {mcpServers.data === undefined ? (
              <p className="mt-1 text-xs text-ink-3">Yükleniyor…</p>
            ) : mcpServers.data.length === 0 ? (
              <p className="mt-1 text-xs text-ink-3">
                Tanımlı sunucu yok. Ayarlar → Dış araçlar bölümünden
                ekleyebilirsiniz.
              </p>
            ) : (
              <>
                <Well className="mt-1.5 px-3">
                  <PickerList>
                    {mcpServers.data.map((srv) => (
                      <PickerRow
                        key={srv.id}
                        title={srv.name}
                        note={`${srv.tools.length} araç`}
                        checked={mcpIds.includes(srv.id)}
                        onChange={(on) =>
                          setMcpIds((prev) =>
                            on
                              ? [...prev, srv.id]
                              : prev.filter((id) => id !== srv.id),
                          )
                        }
                      />
                    ))}
                  </PickerList>
                </Well>
                <p className="mt-1.5 text-2xs text-ink-3">
                  Seçilmeyen sunucuların araçları bu agent&apos;a hiç sunulmaz.
                </p>
              </>
            )}
          </div>

          {/* Kampanya klasörleri: çok adımlı standart bir iş tek seçimle
              bağlanır ve klasöre sonradan eklenen adım kendiliğinden geçerli
              olur — atama tazelenmez (spec 022). */}
          {(scriptFolders.data?.items.length ?? 0) > 0 && (
            <div>
              <p className="text-2xs font-medium tracking-wide text-ink-2 uppercase">
                Kampanya klasörleri
              </p>
              <Well className="mt-1.5 px-3">
                <PickerList>
                  {scriptFolders.data?.items.map((f) => (
                    <PickerRow
                      key={f.id}
                      title={f.name}
                      note={
                        f.description
                          ? `${f.description} · ${f.scriptCount} adım`
                          : `${f.scriptCount} adım`
                      }
                      mono
                      checked={scriptFolderIds.includes(f.id)}
                      onChange={(on) =>
                        setScriptFolderIds((prev) =>
                          on ? [...prev, f.id] : prev.filter((id) => id !== f.id),
                        )
                      }
                    />
                  ))}
                </PickerList>
              </Well>
              <p className="mt-1.5 text-2xs text-ink-3">
                Klasörün <strong>tüm</strong> adımları geçerli olur. Sonradan
                eklenen adım için burayı tekrar düzenlemeniz gerekmez.
              </p>
            </div>
          )}

          {/* Betikler: model NE ZAMAN çağıracağına karar verir, NE YAPACAĞINA
              betik karar verir. Prosedür işlerinde doğaçlamayı kesen tek şey. */}
          <div>
            <p className="text-2xs font-medium tracking-wide text-ink-2 uppercase">
              Betikler
            </p>
            {scripts.data === undefined ? (
              <p className="mt-1 text-xs text-ink-3">Yükleniyor…</p>
            ) : scripts.data.items.length === 0 ? (
              <p className="mt-1 text-xs text-ink-3">
                Tanımlı betik yok. Ayarlar → Betikler bölümünden
                ekleyebilirsiniz.
              </p>
            ) : (
              <>
                <Well className="mt-1.5 px-3">
                  <PickerList>
                    {scripts.data.items.map((s) => (
                      <PickerRow
                        key={s.id}
                        title={s.name}
                        note={
                          klasordenGelen.has(s.id)
                            ? `${s.folderName} klasöründen geliyor`
                            : s.description
                        }
                        mono
                        disabled={klasordenGelen.has(s.id)}
                        checked={
                          scriptIds.includes(s.id) || klasordenGelen.has(s.id)
                        }
                        onChange={(on) =>
                          setScriptIds((prev) =>
                            on
                              ? [...prev, s.id]
                              : prev.filter((id) => id !== s.id),
                          )
                        }
                      />
                    ))}
                  </PickerList>
                </Well>

                {/*
                 * Sessiz bir kural görünür kılınıyor: komut yetkisi kapalıyken
                 * betik container'a hiç kopyalanmıyor. Yazılmasaydı kullanıcı
                 * betiği seçer, kaydeder, çalıştırır ve agent'ın onu neden hiç
                 * çağırmadığını hiçbir hata mesajı olmadan arardı.
                 */}
                {!allowBash &&
                (scriptIds.length > 0 || scriptFolderIds.length > 0) ? (
                  <div className="mt-2">
                    <Notice tone="warning">
                      Bu agent&apos;ın <strong>komut çalıştırma</strong> yetkisi
                      kapalı. Seçilen betikler ve klasörler kaydedilir ama
                      çalıştırma ortamına kopyalanmaz.
                    </Notice>
                  </div>
                ) : (
                  <p className="mt-1.5 text-2xs text-ink-3">
                    Seçilen betikler agent&apos;ın ortamına konur ve talimatına
                    yazılır.
                  </p>
                )}
              </>
            )}
          </div>
        </div>
      </Panel>

      {save.isError && (
        <Notice tone="error" title={describeError(save.error).message}>
          {describeError(save.error).hint}
        </Notice>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <Button
          type="submit"
          variant="primary"
          disabled={!canSubmit || save.isPending}
        >
          {save.isPending ? "Kaydediliyor…" : "Kaydet"}
        </Button>
        <Button type="button" onClick={onDone} disabled={save.isPending}>
          Vazgeç
        </Button>
        {providers.length === 0 && (
          <span className="text-xs text-ink-2">
            Model seçebilmek için{" "}
            <Link href="/settings" className="underline">
              önce bir LLM sağlayıcı ekleyin
            </Link>
            .
          </span>
        )}
      </div>
    </form>
  );
}

/*
 * Seçim listesi — dış araçlar ve betikler için ortak.
 *
 * `Checkbox` bileşeni `inline-flex`; bir kapta yan yana dizilirler. Tek kayıt
 * varken sorun görünmüyordu, ikinci kayıt eklenince iki satır aynı satıra
 * yapıştı. `space-y-*` bunu düzeltmez — dikey boşluk satır içi öğelerde akmaz.
 * Kap açıkça `flex-col` olmak zorunda.
 */
function PickerList({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex max-h-44 flex-col divide-y divide-line overflow-y-auto">
      {children}
    </div>
  );
}

function PickerRow({
  title,
  note,
  mono = false,
  checked,
  disabled = false,
  onChange,
}: {
  title: string;
  note?: string;
  /** Ad bir dosya adına dönüşüyorsa tek aralıklı yazılır. */
  mono?: boolean;
  checked: boolean;
  /**
   * Seçim başka bir yerden geliyorsa kilitlenir — ama satır GİZLENMEZ.
   *
   * Klasörden gelen bir betiği listeden çıkarmak, kullanıcının onu
   * "kaldırabileceğini" sanmasına yol açardı; oysa kaldırmanın yolu klasörü
   * çıkarmak. Gizlemek ise "bu betik agent'ta yok" izlenimi verirdi.
   */
  disabled?: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label
      className={`flex items-start gap-2.5 py-2 ${
        disabled ? "opacity-60" : "cursor-pointer"
      }`}
    >
      {/* mt: kutu, iki satırlık metnin ortasına değil ilk satırın hizasına oturur. */}
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 size-3.5 shrink-0 accent-accent"
      />
      <span className="min-w-0">
        <span className={`block text-sm ${mono ? "font-mono" : "font-medium"}`}>
          {title}
        </span>
        {note && (
          <span className="mt-0.5 block text-xs text-ink-2">{note}</span>
        )}
      </span>
    </label>
  );
}
