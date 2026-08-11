"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { api } from "@/lib/api";
import { Pagination } from "@/components/ui/Pagination";
import { describeError } from "@/lib/errors";
import type { Agent, LLMProvider, Model } from "@/lib/types";
import { ModelPicker } from "@/components/models/ModelPicker";
import { StartRunForm } from "@/components/runs/StartRunForm";
import { IconPlus } from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  List,
  Checkbox,
  Input,
  Notice,
  PageHeader,
  Skeleton,
  Textarea,
  Well,
} from "@/components/ui/primitives";

const PAGE_SIZE = 25;

export default function AgentsPage() {
  const [creating, setCreating] = useState(false);
  const [offset, setOffset] = useState(0);

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

  return (
    /* `max-w-4xl` kaldırıldı: Projeler ve Çalıştırmalar aynı türden listeler
       ama tam genişlikte duruyordu. Aynı iş için iki farklı sayfa genişliği
       keyfi bir farktı. Açıklama metni zaten kendi `max-w-prose` sınırını
       taşıyor, yani okuma satırı uzamıyor. */
    <div>
      <PageHeader
        title="Agent'lar"
        description="Hazır agent'ların talimatını kendi kurallarınıza göre değiştirebilir, sıfırdan kendi agent'ınızı oluşturabilirsiniz."
        actions={
          !creating && (
            <Button
              variant="primary"
              onClick={() => setCreating(true)}
              icon={<IconPlus className="size-4" />}
            >
              Agent oluştur
            </Button>
          )
        }
      />

      <div className="space-y-3">
        {creating && (
          <AgentForm
            providers={providers.data ?? []}
            models={models.data?.items ?? []}
            onDone={() => setCreating(false)}
          />
        )}

        {agents.isPending && <Skeleton rows={3} />}
        {agents.isError && (
          <Notice tone="error">{describeError(agents.error).message}</Notice>
        )}

        {agents.data && agents.data.items.length > 0 && (
          <List>
            {agents.data.items.map((a) => (
              <AgentRow
                key={a.id}
                agent={a}
                providers={providers.data ?? []}
                models={models.data?.items ?? []}
              />
            ))}
          </List>
        )}

        {agents.data && (
          <Pagination
            total={agents.data.total}
            limit={agents.data.limit}
            offset={agents.data.offset}
            onChange={setOffset}
            unit="agent"
          />
        )}
      </div>
    </div>
  );
}

function AgentRow({
  agent,
  providers,
  models,
}: {
  agent: Agent;
  providers: LLMProvider[];
  models: Model[];
}) {
  const queryClient = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [showPrompt, setShowPrompt] = useState(false);
  const [running, setRunning] = useState(false);

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

  const provider = providers.find((p) => p.id === agent.defaultProviderId);

  return (
    /*
     * `group`: eylemlerin hover'da açılması buna bağlı.
     *
     * Satır önceden üst üste dizilmiş DÖRT metin satırıydı (ad, açıklama,
     * yetkiler, model) ve hepsi sola yaslıydı; aralarındaki tek fark punto
     * ve renkti. Göz nereye bakacağını bilmiyordu.
     *
     * Şimdi üç bölge var: solda çıpa (simge), ortada içerik, sağda eylemler.
     */
    <div className="group px-4 py-3.5">
      <div className="flex items-start gap-3.5">
        {/*
          Görsel çıpa — SİMGE DEĞİL, MONOGRAM.

          Satır çıplak metinle başlıyordu; listede göz her satırın nerede
          başladığını yeniden aramak zorundaydı. Sabit genişlikte bir kutu
          sol kenarda dikey ritim kuruyor.

          İlk hali agent simgesiydi ama altı satırda aynı simge duruyordu:
          hiçbir şeyi ayırt etmiyor, yalnızca yer dolduruyordu — yani
          tanımı gereği dekoratif. Monogram aynı ritmi kuruyor ama ayırt
          edici: satırlar birbirinden bakışla ayrılıyor.

          `toLocaleUpperCase("tr")`: Türkçede "i" harfinin büyüğü "İ".
          Öntanımlı büyütme "I" verir ve bir agent adı "İ" ile başlıyorsa
          monogram yanlış harfi gösterirdi.
        */}
        <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg border border-line bg-raised text-sm font-semibold text-ink-2 select-none">
          {agent.name.charAt(0).toLocaleUpperCase("tr")}
        </div>

        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <h2 className="text-sm font-semibold tracking-[-0.01em]">
              {agent.name}
            </h2>
            <span className="font-mono text-xs text-ink-3">{agent.slug}</span>
            {agent.source === "custom" && <Badge tone="info">özel</Badge>}
            {agent.isModified && <Badge tone="warning">değiştirilmiş</Badge>}
          </div>

          {/* İki satırda kesiliyor: liste taranacak bir şey, okunacak değil.
              Tam metin agent'ı düzenlerken zaten görünüyor. */}
          <p className="mt-1 line-clamp-2 max-w-prose text-sm text-ink-2">
            {agent.description}
          </p>

          <div className="mt-2.5 flex flex-wrap items-center gap-1.5">
            <PermissionLine agent={agent} />
            {agent.defaultModel ? (
              <span className="ml-auto truncate pl-2 font-mono text-2xs text-ink-3">
                {provider ? `${provider.name} · ` : ""}
                {agent.defaultModel}
              </span>
            ) : (
              <span className="ml-auto pl-2 text-2xs text-ink-3">
                model seçilmemiş
              </span>
            )}
          </div>
        </div>

        {!editing && !confirming && !running && (
          /*
           * Satır eylemleri İKİNCİL — ama GÖRÜNÜR.
           *
           * İki uçtan da geçildi. Önce hepsi dolu mor birincil düğmeydi ve
           * liste altı agent gösterdiği için sayfada altı tane duruyordu;
           * aksan her satırda tekrarlanınca vurgu olmaktan çıkıyor ve sağ
           * üstteki gerçek birincil eylem ("Agent oluştur") aralarında
           * kayboluyordu.
           *
           * Sonra tersine kaçıldı: `ghost` yapıldılar ve kenarlıkları da
           * zeminleri de gitti — düğme oldukları anlaşılmıyordu. Bu, projenin
           * kendi `control-line` kuralının ihlaliydi: "bu çizgi olmasaydı
           * kullanıcı orada tıklanabilir bir şey olduğunu anlar mıydı?"
           * Cevap hayırsa kenarlık ZORUNLU.
           *
           * Doğru yer ikisinin ortası: hepsi `secondary` — kenarlıklı, yani
           * düğme oldukları belli; dolu değil, yani birincil eylemle
           * yarışmıyorlar. Hiyerarşi renkle değil DOLGUYLA kuruluyor.
           *
           * İKİNCİ SORUN SAYIYDI: altı satır × üç düğme = ekranda sürekli
           * duran on sekiz düğme. Asıl eylem ("Çalıştır") her satırda
           * kalıyor; geri kalanlar imlecin üstünde olduğu satırda açılıyor.
           * Ekranda aynı anda görünen düğme sayısı üçte birine iniyor.
           *
           * Gizlenmiyorlar, ERTELENİYORLAR — üç yoldan da ulaşılabilir:
           *   · fareyle satırın üstüne gelince (`group-hover`)
           *   · klavyeyle sekmeyle girince (`group-focus-within`)
           *   · dar ekranda hep açık (`sm:` öncesi opaklık 1) — dokunmatik
           *     cihazda hover diye bir şey yok, orada erteleme gizlemek olurdu
           */
          <div className="flex shrink-0 items-start gap-2">
            <div className="flex flex-wrap justify-end gap-2 transition-opacity duration-150 sm:opacity-0 sm:group-focus-within:opacity-100 sm:group-hover:opacity-100">
              <Button onClick={() => setShowPrompt((v) => !v)}>
                {showPrompt ? "Talimatı gizle" : "Talimatı gör"}
              </Button>
              <Button onClick={() => setEditing(true)}>Düzenle</Button>
              {agent.isModified && (
                <Button onClick={() => reset.mutate()} disabled={reset.isPending}>
                  {reset.isPending ? "…" : "Sıfırla"}
                </Button>
              )}
              {agent.source === "custom" && (
                <Button variant="danger" onClick={() => setConfirming(true)}>
                  Sil
                </Button>
              )}
            </div>
            <Button onClick={() => setRunning(true)}>Çalıştır</Button>
          </div>
        )}

        {confirming && (
          <div className="flex shrink-0 items-center gap-2">
            <span className="text-xs text-ink-2">Emin misiniz?</span>
            <Button
              variant="danger"
              onClick={() => remove.mutate()}
              disabled={remove.isPending}
            >
              {remove.isPending ? "Siliniyor…" : "Evet, sil"}
            </Button>
            <Button onClick={() => setConfirming(false)}>Vazgeç</Button>
          </div>
        )}
      </div>

      {remove.isError && (
        <p className="mt-2 text-sm text-danger">
          {describeError(remove.error).message}
        </p>
      )}

      {showPrompt && !editing && (
        <Well className="mt-4 max-h-80 overflow-auto p-3.5">
          <pre className="font-mono text-xs leading-relaxed whitespace-pre-wrap">
            {agent.prompt}
          </pre>
        </Well>
      )}

      {running && (
        <div className="mt-4">
          <StartRunForm agent={agent} onDone={() => setRunning(false)} />
        </div>
      )}

      {editing && (
        <div className="mt-4">
          <AgentForm
            agent={agent}
            providers={providers}
            models={models}
            onDone={() => setEditing(false)}
            inline
          />
        </div>
      )}
    </div>
  );
}

/**
 * Agent'ın neye yetkisi olduğu.
 *
 * Önceki hali üç yetkiyi de rozetle gösteriyordu: açık olanlar YEŞİL, kapalılar
 * gri. İki sorun vardı. Yeşil "iyi" demek; oysa burada "bu agent dosyalarınızı
 * değiştirebilir" demek — riskli olan durumu güvenli renkle boyuyordu. Ve beş
 * agent yan yana on beş rozet ediyordu.
 *
 * Şimdi yalnızca AÇIK olanlar yazılıyor; hiçbiri yoksa tek cümle. Kapalı bir
 * yetkinin ekranda yer kaplaması gerekmiyor — yokluğu zaten cevap.
 */
function PermissionLine({ agent }: { agent: Agent }) {
  /* Çip olarak kısaldılar: "dosya değiştirebilir" bir cümlenin parçasıydı,
     "dosya" ise bir etiket. Tam anlam `title` ile duruyor. */
  const izinli = [
    agent.allowEdit && "dosya yazar",
    agent.allowBash && "komut çalıştırır",
    agent.allowWebfetch && "ağa çıkar",
  ].filter(Boolean) as string[];

  const mcp = agent.mcpServerIds.length;
  // Betikler yalnızca komut yetkisi açıkken çalıştırılabiliyor; kapalıyken
  // sayıyı yazmak, olmayan bir yeteneği varmış gibi göstermek olurdu.
  const betik = agent.allowBash ? agent.scriptIds.length : 0;

  /*
   * Yetkiler ÇİP, cümle değil.
   *
   * Önceden "Yetkileri: dosya değiştirebilir · komut çalıştırabilir · ağa
   * çıkabilir" diye tek satır düz metindi. İki sorunu vardı: satırdaki diğer
   * üç metin bloğuyla aynı görünüyordu, ve bir agent'ın neye yetkili olduğu
   * ancak cümle okunarak anlaşılıyordu — oysa bu, listede TARANAN bir bilgi.
   *
   * "Yalnızca okur" hâli bilerek ayrı tutuldu ve `ok` tonunda: bu bir eksiklik
   * değil, güvenlik özelliği. Yetkisiz bir agent'ın deponuza yazamayacağını
   * söylüyor ve o cümlenin kaybolmaması gerekiyordu.
   */
  return (
    <>
      {izinli.length === 0 ? (
        <Badge tone="success" title="Bu agent hiçbir şeyi değiştiremez">
          yalnızca okur
        </Badge>
      ) : (
        izinli.map((y) => <Badge key={y}>{y}</Badge>)
      )}
      {mcp > 0 && (
        <Badge tone="info" title="Dış araç sunucusu (MCP)">
          {mcp} dış araç
        </Badge>
      )}
      {betik > 0 && <Badge title="Hazır betik">{betik} betik</Badge>}
    </>
  );
}

function AgentForm({
  agent,
  providers,
  models,
  onDone,
  inline = false,
}: {
  agent?: Agent;
  providers: LLMProvider[];
  models: Model[];
  onDone: () => void;
  inline?: boolean;
}) {
  const queryClient = useQueryClient();
  const editing = agent !== undefined;

  const [name, setName] = useState(agent?.name ?? "");
  const [description, setDescription] = useState(agent?.description ?? "");
  const [prompt, setPrompt] = useState(agent?.prompt ?? "");
  // Varsayılan model (sağlayıcı, model) çiftidir; ikisi birlikte seçilir.
  const [model, setModel] = useState<{ providerId: string; modelId: string } | null>(
    agent?.defaultModel && agent.defaultProviderId
      ? { providerId: agent.defaultProviderId, modelId: agent.defaultModel }
      : null,
  );
  const [allowEdit, setAllowEdit] = useState(agent?.allowEdit ?? true);
  const [allowBash, setAllowBash] = useState(agent?.allowBash ?? true);
  const [allowWebfetch, setAllowWebfetch] = useState(agent?.allowWebfetch ?? false);
  const [mcpIds, setMcpIds] = useState<string[]>(agent?.mcpServerIds ?? []);
  const [scriptIds, setScriptIds] = useState<string[]>(agent?.scriptIds ?? []);

  const mcpServers = useQuery({ queryKey: ["mcp-servers"], queryFn: api.mcpServers.list });
  // Seçim listesi olduğu için tek sayfa yetmez; sınır yüksek tutuluyor.
  const scripts = useQuery({
    queryKey: ["scripts", "all"],
    queryFn: () => api.scripts.list({ limit: 200 }),
  });

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
      if (!editing && (mcpIds.length > 0 || scriptIds.length > 0)) {
        await api.agents.update(saved.id, { mcpServerIds: mcpIds, scriptIds });
      }
      void queryClient.invalidateQueries({ queryKey: ["agents"] });
      onDone();
    },
  });

  const selected = models.find(
    (m) => m.id === model?.modelId && m.providerId === model.providerId,
  );
  const canSubmit = name.trim() !== "" && prompt.trim() !== "";

  const body = (
    <form
      className="space-y-3"
      onSubmit={(e) => {
        e.preventDefault();
        if (canSubmit) save.mutate();
      }}
    >
      <div className="flex gap-3">
        <label className="block flex-1">
          <span className="text-xs text-ink-2">Ad</span>
          <Input
            className="mt-1"
            value={name}
            placeholder="Kod İncelemecisi"
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label className="block flex-2">
          <span className="text-xs text-ink-2">Açıklama</span>
          <Input
            className="mt-1"
            value={description}
            placeholder="Ne yaptığını bir cümleyle anlatın"
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
      </div>

      <label className="block">
        <span className="text-xs text-ink-2">
          Talimat (agent&apos;a verilen sistem yönergesi)
        </span>
        <Textarea
          className="mt-1 h-64 font-mono text-xs leading-relaxed"
          value={prompt}
          placeholder="Sen bir kod incelemecisisin. …"
          onChange={(e) => setPrompt(e.target.value)}
        />
      </label>

      <div>
        <span className="text-2xs tracking-wide text-ink-2 uppercase">
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
        <Notice tone="warning">
          Seçilen model araç çağıramıyor. Agent dosya okuyup değiştiremeyeceği
          için büyük olasılıkla işe yaramaz.
        </Notice>
      )}
      {selected && selected.supportsTools === null && (
        <Notice tone="warning">
          Sağlayıcı bu modelin araç desteğini bildirmiyor. Çalışabilir ama garanti
          değil.
        </Notice>
      )}

      <fieldset className="rounded border border-line p-3">
        <legend className="px-1 text-xs text-ink-2">Yetkiler</legend>
        <div className="flex flex-wrap gap-4">
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
      </fieldset>

      {/* Dış araçlar da bir YETKİDİR: bu agent'ın neye erişebildiğinin parçası.
          Ayrı bir kutuda çünkü listesi değişken ve her kurulumda farklı. */}
      <fieldset className="mt-3 rounded border border-line p-3">
        <legend className="px-1 text-xs text-ink-2">Dış araçlar (MCP)</legend>
        {mcpServers.data === undefined ? (
          <p className="text-xs text-ink-3">Yükleniyor…</p>
        ) : mcpServers.data.length === 0 ? (
          <p className="text-xs text-ink-3">
            Tanımlı sunucu yok. Ayarlar → Dış araçlar bölümünden ekleyebilirsiniz.
          </p>
        ) : (
          <div>
            <PickerList>
              {mcpServers.data.map((srv) => (
                <PickerRow
                  key={srv.id}
                  title={srv.name}
                  note={`${srv.tools.length} araç`}
                  checked={mcpIds.includes(srv.id)}
                  onChange={(on) =>
                    setMcpIds((prev) =>
                      on ? [...prev, srv.id] : prev.filter((id) => id !== srv.id),
                    )
                  }
                />
              ))}
            </PickerList>
            <p className="mt-2 text-2xs text-ink-3">
              Seçilmeyen sunucuların araçları bu agent&apos;a hiç sunulmaz.
            </p>
          </div>
        )}
      </fieldset>

      {/* Betikler: model NE ZAMAN çağıracağına karar verir, NE YAPACAĞINA betik
          karar verir. Prosedür işlerinde doğaçlamayı kesen tek şey bu. */}
      <fieldset className="mt-3 rounded border border-line p-3">
        <legend className="px-1 text-xs text-ink-2">Betikler</legend>
        {scripts.data === undefined ? (
          <p className="text-xs text-ink-3">Yükleniyor…</p>
        ) : scripts.data.items.length === 0 ? (
          <p className="text-xs text-ink-3">
            Tanımlı betik yok. Ayarlar → Betikler bölümünden ekleyebilirsiniz.
          </p>
        ) : (
          <div>
            <PickerList>
              {scripts.data.items.map((s) => (
                <PickerRow
                  key={s.id}
                  title={s.name}
                  note={s.description}
                  mono
                  checked={scriptIds.includes(s.id)}
                  onChange={(on) =>
                    setScriptIds((prev) =>
                      on ? [...prev, s.id] : prev.filter((id) => id !== s.id),
                    )
                  }
                />
              ))}
            </PickerList>

            {/*
             * Sessiz bir kural görünür kılınıyor.
             *
             * Komut yetkisi kapalıyken betik container'a hiç kopyalanmıyor.
             * Yazılmasaydı kullanıcı betiği seçer, kaydeder, çalıştırır ve
             * agent'ın onu neden hiç çağırmadığını arardı — hiçbir hata mesajı
             * çıkmadan.
             */}
            {!allowBash && scriptIds.length > 0 ? (
              <div className="mt-2">
                <Notice tone="warning">
                  Bu agent&apos;ın <strong>komut çalıştırma</strong> yetkisi kapalı.
                  Seçilen betikler kaydedilir ama çalıştırma ortamına
                  kopyalanmaz — yetkiyi açana kadar kullanılamazlar.
                </Notice>
              </div>
            ) : (
              <p className="mt-2 text-2xs text-ink-3">
                Seçilen betikler agent&apos;ın ortamına konur ve talimatına
                yazılır. Ne zaman çağıracağına agent karar verir.
              </p>
            )}
          </div>
        )}
      </fieldset>

      {save.isError && (
        <div className="rounded border border-danger/35 bg-danger-soft px-3 py-2 text-sm">
          <p className="font-medium text-danger">
            {describeError(save.error).message}
          </p>
        </div>
      )}

      <div className="flex items-center gap-2 pt-1">
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

  return inline ? body : <Card>{body}</Card>;
}

/*
 * Seçim listesi — dış araçlar ve betikler için ortak.
 *
 * `Checkbox` bileşeni `inline-flex`; bir kapta yan yana dizilirler. Tek kayıt
 * varken sorun görünmüyordu, ikinci kayıt eklenince iki satır aynı satıra
 * yapıştı. `space-y-*` bunu düzeltmez — dikey boşluk satır içi öğelerde akmaz.
 * Kap açıkça `flex-col` olmak zorunda.
 *
 * Ayrıca bu liste iki bilgi taşıyor (ad + ne olduğu). Tek satıra "ad — açıklama"
 * diye sıkıştırıldığında uzun açıklama satırı taşırıyor ve göz adı bulamıyor;
 * ad üstte, açıklama altında duruyor.
 */
function PickerList({ children }: { children: React.ReactNode }) {
  return <div className="flex flex-col divide-y divide-line">{children}</div>;
}

function PickerRow({
  title,
  note,
  mono = false,
  checked,
  onChange,
}: {
  title: string;
  note?: string;
  /** Ad bir dosya adına dönüşüyorsa tek aralıklı yazılır. */
  mono?: boolean;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="flex cursor-pointer items-start gap-2.5 py-2 first:pt-0 last:pb-0">
      {/* mt: kutu, iki satırlık metnin ortasına değil ilk satırın hizasına oturur. */}
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 size-3.5 shrink-0 accent-accent"
      />
      <span className="min-w-0">
        <span className={`block text-sm ${mono ? "font-mono" : "font-medium"}`}>
          {title}
        </span>
        {note && <span className="mt-0.5 block text-xs text-ink-2">{note}</span>}
      </span>
    </label>
  );
}

