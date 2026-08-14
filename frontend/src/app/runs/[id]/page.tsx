"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { useRunEvents } from "@/lib/use-run-events";
import type { Run } from "@/lib/types";
import { Markdown } from "@/components/markdown/Markdown";
import { EngineLogs } from "@/components/runs/EngineLogs";
import { RunStatusBadge, isActive } from "@/components/runs/RunStatusBadge";
import {
  formatCompact,
  formatCount,
  formatDuration,
  formatMoney,
} from "@/components/charts/format";
import {
  IconAgent,
  IconAlert,
  IconBolt,
  IconEdit,
  IconExternal,
  IconTerminal,
  IconTrash,
} from "@/components/ui/icons";
import {
  Badge,
  Button,
  Card,
  ConfirmStrip,
  IconTile,
  Input,
  Notice,
  Panel,
  Skeleton,
  StatusDot,
  Well,
  formatDate,
  formatRelative,
} from "@/components/ui/primitives";

export default function RunDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [confirmingDelete, setConfirmingDelete] = useState(false);
  const [sekme, setSekme] = useState<SekmeID>("sonuc");

  const {
    data: run,
    isPending,
    isError,
    error,
  } = useQuery({
    queryKey: ["run", id],
    queryFn: () => api.runs.get(id),
  });

  const active = run ? isActive(run.status) : false;

  /*
   * SÜREN KOŞUDA VARSAYILAN SEKME İLERLEME.
   *
   * Sonuç henüz yok; kullanıcıyı boş bir sekmeye düşürüp "İlerleme'ye bakın"
   * demek, bakması gereken yere kendisinin gitmesini istemek olurdu.
   *
   * BİR KEZ: `useRef` bekçisi olmadan koşu bitince (status değişince) sekme
   * geri zıplar ve kullanıcının o an baktığı yeri elinden alırdı. Seçim
   * kullanıcıya geçtikten sonra kod bir daha karışmıyor.
   */
  const sekmeAyarlandi = useRef(false);
  useEffect(() => {
    if (sekmeAyarlandi.current || !run) return;
    sekmeAyarlandi.current = true;
    if (isActive(run.status)) setSekme("ilerleme");
  }, [run]);
  const { events, terminalStatus, connected } = useRunEvents(id, active);

  // Canlı akış "bitti" dediğinde kaydı yeniden çekiyoruz: çıktı, diff ve
  // maliyet ancak o zaman veritabanında hazır olur.
  useEffect(() => {
    if (terminalStatus) {
      void queryClient.invalidateQueries({ queryKey: ["run", id] });
      void queryClient.invalidateQueries({ queryKey: ["runs"] });
    }
  }, [terminalStatus, id, queryClient]);

  /*
   * Silme.
   *
   * Mutasyon yükleme kontrolünün ÜSTÜNDE tanımlı: hook'lar koşullu return'den
   * sonra çağrılamaz. Tetiklenmesi yalnızca kayıt geldikten sonra mümkün.
   *
   * Rapor anahtarı da tazelenir — maliyet ve token doğrudan bu satırdan
   * toplanıyor, kayıt gidince geçmiş rakamlar da değişiyor.
   */
  const remove = useMutation({
    mutationFn: () => api.runs.remove(id),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["runs"] });
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
      void queryClient.invalidateQueries({ queryKey: ["report"] });
      router.push("/runs");
    },
  });

  if (isPending) return <Skeleton rows={4} />;
  if (isError)
    return <Notice tone="error">{describeError(error).message}</Notice>;

  const tokens = run.promptTokens + run.completionTokens;
  const adds = run.files.reduce((sum, f) => sum + f.additions, 0);
  const dels = run.files.reduce((sum, f) => sum + f.deletions, 0);

  return (
    /* Tam yükseklik: künye üstte sabit, içerik ortada kayar — arayüzün
       geri kalanıyla aynı kabuk düzeni. */
    <div className="flex min-h-0 flex-1 flex-col">
      {/*
        KÜNYE KARTI.

        Öncesinde başlık `PageHeader` idi ve görev metninin ilk 70 karakteri
        H1 olarak basılıyordu — bir cümlenin ortasından kesilmiş hâli sayfa
        başlığı olmuyordu. Şimdi görev iki satıra kadar açılıyor, kimlik
        bilgisi (agent, model, proje, branch) altına iniyor ve ölçüler
        kartın kendi şeridinde duruyor.
      */}
      <Card className="mb-4 shrink-0">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex min-w-0 items-start gap-3">
            <IconTile tone={toneOf(run)}>
              <IconAgent className="size-4" />
            </IconTile>

            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <RunStatusBadge status={run.status} />
                {run.workflowName && (
                  <Badge tone="accent">
                    {run.workflowName} · {run.stepName}
                  </Badge>
                )}
                {run.pushedBranch && (
                  <Badge tone="info">→ {run.pushedBranch}</Badge>
                )}
              </div>

              {/* Görev metni KIRPILMIYOR, iki satıra sarılıyor. Bu ekranın
                  konusu bu cümle. */}
              <h1 className="mt-2 line-clamp-2 text-lg font-semibold tracking-[-0.02em]">
                {run.task}
              </h1>

              <p className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-ink-3">
                <span className="font-mono text-ink-2">{run.agentSlug}</span>
                <span aria-hidden="true">·</span>
                <span className="font-mono">{run.modelId}</span>
                <span aria-hidden="true">·</span>
                <span>{run.projectName}</span>
                <span aria-hidden="true">·</span>
                <span className="font-mono">{run.branch}</span>
                {/* Node sürümü yalnızca SEÇİLMİŞSE yazılır: seçmeyen koşularda
                    runner imajının kendi sürümü geçerli ve onu burada iddia
                    etmek, ölçmediğimiz bir şeyi göstermek olurdu. */}
                {run.nodeVersion && (
                  <>
                    <span aria-hidden="true">·</span>
                    <span className="font-mono">node {run.nodeVersion}</span>
                  </>
                )}
                <span aria-hidden="true">·</span>
                <span>{formatRelative(run.createdAt)}</span>
              </p>
            </div>
          </div>

          <div className="flex shrink-0 items-center gap-2">
            <RunActions
              run={run}
              onConfirmDelete={() => setConfirmingDelete(true)}
            />
            <Link
              href="/runs"
              className="rounded text-xs text-ink-3 transition-colors hover:text-accent"
            >
              Tüm çalıştırmalar
            </Link>

          </div>
        </div>

        {/*
          ÖLÇÜLER TEK SATIR.

          Öncesinde dört eşit blok vardı (Süre · Token · Maliyet · Değişiklik),
          her biri 308px — dört sayı için 1232px yatay alan. `ui.md`'nin adıyla
          saydığı kalıp: "az bilgiyi büyük kartlara yayan generic SaaS
          dashboard düzeni". Sayılar aynı, kapladıkları yer bir satır.

          HER SAYI BİRİMİYLE yazılıyor: "25,4 B" tek başına neyin sayısı
          olduğunu söylemiyordu, blok başlığı söylüyordu. Başlık kalkınca
          birim satırın içine girdi.
        */}
        <dl className="mt-3 flex flex-wrap items-baseline gap-x-3 gap-y-1 border-t border-line pt-3 text-xs">
          <Olcu
            etiket="süre"
            deger={durationOf(run)}
            baslik={run.finishedAt ? `bitti: ${formatDate(run.finishedAt)}` : "sürüyor"}
          />
          <Olcu
            etiket="token"
            deger={tokens > 0 ? formatCompact(tokens) : "—"}
            baslik={
              tokens > 0
                ? `${formatCount(run.promptTokens)} girdi · ${formatCount(run.completionTokens)} çıktı`
                : undefined
            }
          />
          <Olcu
            etiket="maliyet"
            deger={run.costUsd > 0 ? formatMoney(run.costUsd) : "—"}
            baslik={run.providerSlug || undefined}
            mono
          />
          <Olcu
            etiket="değişiklik"
            deger={
              run.files.length > 0
                ? `${formatCount(run.files.length)} dosya`
                : "kod değişmedi"
            }
            baslik={
              run.files.length > 0
                ? `+${formatCount(adds)} −${formatCount(dels)} satır`
                : undefined
            }
          />
          {run.files.length > 0 && (
            <span className="text-2xs tabular-nums">
              <span className="text-ok">+{formatCompact(adds)}</span>{" "}
              <span className="text-danger">−{formatCompact(dels)}</span>
            </span>
          )}
        </dl>

        {/* Hata künyenin İÇİNDE: ayrı bir kutuya alınsaydı çalıştırmanın
            kimliğinden kopardı ve sayfanın en önemli cümlesi bağlamsız
            kalırdı. */}
        {run.error && (
          <div className="mt-4 rounded-lg border border-danger/30 bg-danger-soft px-3.5 py-2.5">
            <p className="flex items-start gap-2 text-xs">
              <IconAlert className="mt-px size-4 shrink-0 text-danger" />
              <span className="min-w-0">
                <span className="font-medium text-danger">
                  Çalıştırma başarısız
                </span>
                <span className="mt-0.5 block break-words text-ink-2">
                  {run.error}
                </span>
              </span>
            </p>
          </div>
        )}

        {/*
          Silme onayı künyenin içinde, tam genişlikte.

          Sonucu SAYIYLA yazıyor: maliyet ve token doğrudan bu satırın
          sütunlarında duruyor, ayrı bir özet tablo yok — kayıt gidince rapor
          rakamları da o kadar azalıyor. "Emin misiniz?" bunu söylemezdi.
        */}
        {confirmingDelete && (
          <ConfirmStrip
            className="mt-4 rounded-lg border"
            question="Bu çalıştırma kaydı silinsin mi?"
            consequence={
              <>
                <strong>
                  {formatMoney(run.costUsd)}
                  {tokens > 0 && <> ve {formatCompact(tokens)} token</>}
                </strong>{" "}
                raporlardan düşecek; çıktı, diff ve olay geçmişi de gider. Bu
                geri alınamaz.
              </>
            }
            busy={remove.isPending}
            error={
              remove.isError ? describeError(remove.error).message : undefined
            }
            onConfirm={() => remove.mutate()}
            onCancel={() => setConfirmingDelete(false)}
          />
        )}
      </Card>

      {/* Motor logları AYRI SEKMEDE: teşhis katmanı, günlük kullanımın
          parçası değil. Aynı sütuna eklenseydi her koşuda gözün önünden
          geçerdi; oysa oraya yalnızca bir şey ters gittiğinde bakılır. */}
      <RunTabs active={sekme} onSelect={setSekme} />

      {/*
        Kayan bölge: künye ve sekmeler sabit kalırken içerik kayıyor.

        İLERLEME SEKMESİ AYRI DAVRANIYOR. Diğerlerinde (sonuç, diff, engine
        logları) içerik uzun ve sayfanın kayması doğru: metin baştan sona
        okunuyor. İlerleme ise CANLI bir akış — panonun başlığı ve "canlı"
        göstergesi gözden kaybolmamalı, akan olaylar kendi kutusunda kaymalı.
        Terminal ve CI log kalıbının aynısı.

        Bu yüzden o sekmede dış kaydırma KAPALI ve yerleşim `flex` ile tam
        alanı dolduruyor. Öncesi `calc(100vh-18rem)` ile başlık yüksekliğini
        TAHMİN ediyordu; tahmin tutmayınca 620px pencerede iki kaydırma
        çubuğu birden çıkıyordu (ölçüldü). Flex ile tahmin yok.
      */}
      <div
        className={`-mx-1 min-h-0 flex-1 px-1 pb-1 ${
          sekme === "ilerleme"
            ? "flex flex-col overflow-hidden"
            : "space-y-4 overflow-y-auto"
        }`}
      >
        {sekme === "motor" && <EngineLogs runId={id} live={active} />}

        {sekme === "ilerleme" && (
          <Panel
            className="flex min-h-0 flex-1 flex-col"
            title="İlerleme"
            action={
              active ? (
                <span className="flex items-center gap-1.5 text-2xs text-ink-3">
                  <StatusDot tone={connected ? "accent" : "neutral"} pulse />
                  {connected ? "canlı" : "bağlanıyor…"}
                </span>
              ) : undefined
            }
            padded={false}
          >
            <EventLog runId={id} live={events} active={active} />
          </Panel>
        )}

        {sekme === "sonuc" &&
          (run.output ? (
            <AgentOutput output={run.output} />
          ) : (
            /* Boşluk BİR CEVAPTIR: süren bir koşuda henüz çıktı yok ve
               nereye bakılacağını söylemek, boş bir kutu göstermekten
               iyidir. */
            <Notice>
              {active
                ? "Çalıştırma sürüyor — sonuç bittiğinde burada görünecek. İlerleme sekmesinden canlı takip edebilirsiniz."
                : "Bu çalıştırma bir çıktı üretmedi."}
            </Notice>
          ))}

        {sekme === "degisiklik" &&
          (run.diff ? (
            <Changes run={run} />
          ) : (
            <Notice>
              {active
                ? "Çalıştırma sürüyor — değişen dosyalar bittiğinde burada görünecek."
                : "Bu çalıştırma hiçbir dosyayı değiştirmedi."}
            </Notice>
          ))}
      </div>
    </div>
  );
}

/*
 * Sekme sırası KULLANICININ SORU SIRASI.
 *
 * Öncesinde tek bir "Çalıştırma" sekmesi vardı ve içinde sırayla ilerleme,
 * çıktı ve diff yığılıydı: kullanıcının ilk sorusu "sonuç ne" ama önce olay
 * akışı geliyordu, kod değişikliği 995px aşağıdaydı (ölçüldü). Artık her biri
 * kendi sekmesinde ve sıra sonuç → değişiklikler → ilerleme → loglar.
 *
 * İlerleme ARKADA ama kaybolmadı: süren bir koşuda varsayılan sekme o oluyor,
 * çünkü henüz sonuç yok.
 */
const SEKMELER = [
  { id: "sonuc", label: "Sonuç", Icon: IconAgent },
  { id: "degisiklik", label: "Değişiklikler", Icon: IconEdit },
  { id: "ilerleme", label: "İlerleme", Icon: IconBolt },
  { id: "motor", label: "Engine logları", Icon: IconTerminal },
] as const;

type SekmeID = (typeof SEKMELER)[number]["id"];

/**
 * Koşu detayının iki görünümü.
 *
 * Ayarlar sayfasındaki gezinme kalıbının yatay hâli: etkin sekme hem renkle
 * hem ALT ŞERİTLE işaretlenir — renk körlüğünde tek başına renk yetmez.
 */
function RunTabs({
  active,
  onSelect,
}: {
  active: SekmeID;
  onSelect: (id: SekmeID) => void;
}) {
  return (
    <nav
      aria-label="Koşu görünümü"
      className="-mx-1 flex shrink-0 gap-1 overflow-x-auto border-b border-line px-1"
    >
      {SEKMELER.map(({ id, label, Icon }) => {
        const on = id === active;
        return (
          <button
            key={id}
            type="button"
            onClick={() => onSelect(id)}
            aria-current={on ? "page" : undefined}
            className={`relative flex shrink-0 items-center gap-2 px-2.5 pt-1 pb-2 text-sm whitespace-nowrap transition-colors duration-150 ${
              on ? "font-medium text-ink" : "text-ink-3 hover:text-ink"
            }`}
          >
            <Icon className="size-4 shrink-0" />
            {label}
            {on && (
              <span className="absolute inset-x-1.5 -bottom-px h-0.5 rounded-full bg-accent" />
            )}
          </button>
        );
      })}
    </nav>
  );
}

/**
 * Künye satırındaki tek bir ölçü.
 *
 * Değer ÖNCE, etiket sonra ve sönük: okuyan önce sayıyı görüyor, ne olduğunu
 * hemen yanında. Ters sırada dört etiket üst üste okunuyor, sayılar aralarında
 * kayboluyordu.
 */
function Olcu({
  etiket,
  deger,
  baslik,
  mono,
}: {
  etiket: string;
  deger: string;
  baslik?: string;
  mono?: boolean;
}) {
  return (
    /*
     * `div` içinde `dt`+`dd`: HTML5'te geçerli ve gruplamayı korur. Etiket
     * kaynakta ÖNCE (dt, dd sırası zorunlu) ama ekranda SONRA (`order`) —
     * böylece ekran okuyucu "süre, 55 sn" derken göz "55 sn süre" görüyor.
     */
    <div className="flex items-baseline gap-1" title={baslik}>
      <dt className="order-2 text-2xs text-ink-3">{etiket}</dt>
      <dd className={`order-1 font-medium tabular-nums ${mono ? "font-mono" : ""}`}>
        {deger}
      </dd>
    </div>
  );
}

/** Künye karosunun tonu — durumdan geliyor, dekoratif değil. */
function toneOf(run: Run) {
  if (isActive(run.status)) return "accent" as const;
  if (run.status === "succeeded") return "success" as const;
  if (run.status === "failed") return "danger" as const;
  return "warning" as const;
}

/**
 * Çalıştırma süresi.
 *
 * Süren işler için başlangıçtan ŞU ANA kadar; bitenler için gerçek süre.
 * Liste ekranındaki hesapla aynı.
 */
function durationOf(run: Run): string {
  if (!run.startedAt) return "—";
  const end = run.finishedAt ? new Date(run.finishedAt) : new Date();
  const seconds = (end.getTime() - new Date(run.startedAt).getTime()) / 1000;
  return seconds > 0 ? formatDuration(seconds) : "—";
}

/**
 * Çalıştırmanın kendi eylemleri.
 *
 * Süren işte tek eylem iptal; bitmiş işte gönderme ve silme birlikte
 * durabilir. Silme İPTALİN YERİNE GEÇMEZ — süren bir kaydı silmek, kaydı
 * olmayan bir container bırakırdı; o yüzden yalnızca bitmiş işte görünür.
 */
function RunActions({
  run,
  onConfirmDelete,
}: {
  run: Run;
  onConfirmDelete: () => void;
}) {
  const queryClient = useQueryClient();
  const [pushing, setPushing] = useState(false);

  const cancel = useMutation({
    mutationFn: () => api.runs.cancel(run.id),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ["run", run.id] }),
  });

  if (isActive(run.status)) {
    return (
      <Button
        variant="danger"
        onClick={() => cancel.mutate()}
        disabled={cancel.isPending}
      >
        {cancel.isPending ? "İptal ediliyor…" : "İptal et"}
      </Button>
    );
  }

  if (pushing) {
    return <PushForm run={run} onDone={() => setPushing(false)} />;
  }

  return (
    <>
      {run.diff && !run.pushedBranch && (
        <Button variant="primary" onClick={() => setPushing(true)}>
          Branch&apos;e gönder
        </Button>
      )}
      {/* Akış adımları silinemez: kayıt gitse de akış geçmişinde maliyeti ve
          agent'ı boşalmış bir adım kalırdı. Düğme gizlenmiyor, SEBEBİ
          yazılıyor — gizlenen bir eylem kullanıcıya "yok" der, oysa var. */}
      <Button
        variant="danger"
        icon={<IconTrash className="size-4" />}
        disabled={run.workflowRunId !== null}
        title={
          run.workflowRunId !== null
            ? "Bu çalıştırma bir akışın adımı — akış çalışmasını silin"
            : undefined
        }
        onClick={onConfirmDelete}
      >
        Sil
      </Button>
    </>
  );
}

function PushForm({ run, onDone }: { run: Run; onDone: () => void }) {
  const queryClient = useQueryClient();
  const suggested = `agent-coder/${run.agentSlug}-${run.id.slice(0, 8)}`;
  const [branch, setBranch] = useState(suggested);

  const push = useMutation({
    mutationFn: () => api.runs.push(run.id, branch.trim()),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["run", run.id] });
      onDone();
    },
  });

  return (
    <div className="flex flex-col items-end gap-2">
      <div className="flex items-center gap-2">
        <Input
          className="w-64 font-mono text-xs"
          value={branch}
          onChange={(e) => setBranch(e.target.value)}
        />
        <Button
          variant="primary"
          onClick={() => push.mutate()}
          disabled={push.isPending || branch.trim() === ""}
          icon={<IconExternal className="size-4" />}
        >
          {push.isPending ? "Gönderiliyor…" : "Gönder"}
        </Button>
        <Button onClick={onDone} disabled={push.isPending}>
          Vazgeç
        </Button>
      </div>
      {push.isError && (
        <p className="max-w-md text-right text-xs text-danger">
          {describeError(push.error).message}
        </p>
      )}
    </div>
  );
}

/**
 * Olay akışı.
 *
 * Çalışma sürerken canlı olaylar, bittiğinde veritabanındaki kayıt gösterilir.
 * İkisi de aynı `seq` alanını taşıdığı için karışmaz.
 */
function EventLog({
  runId,
  live,
  active,
}: {
  runId: string;
  live: ReturnType<typeof useRunEvents>["events"];
  active: boolean;
}) {
  const boxRef = useRef<HTMLDivElement>(null);

  // Yeni olay geldikçe en alta kaydır — kullanıcı akışı takip etsin.
  useEffect(() => {
    if (active && boxRef.current) {
      boxRef.current.scrollTop = boxRef.current.scrollHeight;
    }
  }, [live.length, active]);

  // Çalışma bittiyse geçmişi veritabanından oku (SSE kapalı).
  const history = useQuery({
    queryKey: ["run-events", runId],
    queryFn: async () => {
      const res = await fetch(api.runs.eventsUrl(runId));
      const text = await res.text();
      return text
        .split("\n")
        .filter((l) => l.startsWith("data: "))
        .map(
          (l) =>
            JSON.parse(l.slice(6)) as {
              seq: number;
              level: string;
              message: string;
            },
        )
        .filter((e) => e.message);
    },
    enabled: !active,
  });

  const items = active ? live : (history.data ?? []);

  if (items.length === 0) {
    return (
      <p className="px-4 py-3.5 text-sm text-ink-3">
        {active ? "Çalışma başlatılıyor…" : "Kayıtlı olay yok."}
      </p>
    );
  }

  return (
    /*
     * Kendi kutusu YOK: pano zaten bir kutu ve `Well` ile sarılınca iç içe
     * iki çerçeve çıkıyordu. Kayan alan doğrudan panonun gövdesi.
     *
     * YÜKSEKLİK YERLEŞİMDEN GELİYOR, sayıdan değil. Sabit 320px'ti ve bu,
     * İlerleme ile çıktı ve diff'in ALT ALTA yığıldığı düzenden kalmıştı;
     * her biri kendi sekmesine taşınınca anlamını yitirdi ama kalmıştı —
     * ÖLÇÜLDÜ: olaylar 320px'e sıkışıp içeride kayarken altında 165px boş
     * alan duruyordu. Ardından `calc(100vh-18rem)` denendi, o da başlık
     * yüksekliğini TAHMİN ediyordu ve tahmin tutmayınca iki kaydırma çubuğu
     * çıkıyordu. Artık kutu, `flex-1` ile sekme alanının kalanını alıyor:
     * tahmin yok, her pencere boyunda tam oturuyor.
     *
     * Kaydırma KALIYOR: yeni olay geldikçe dibe kayan takip bu kutunun kendi
     * kaydırmasına bağlı (bkz. yukarıdaki `boxRef`).
     */
    <div
      ref={boxRef}
      className="min-h-0 flex-1 overflow-auto px-4 py-3.5"
    >
      <ul className="space-y-1.5">
        {items.map((e) => (
          <li key={e.seq} className="flex items-start gap-2.5 text-xs">
            <span className="mt-1.5">
              <StatusDot
                tone={
                  e.level === "error"
                    ? "danger"
                    : e.level === "warn"
                      ? "warning"
                      : "neutral"
                }
              />
            </span>
            <span
              className={
                e.level === "error"
                  ? "text-danger"
                  : e.level === "warn"
                    ? "text-warn"
                    : "text-ink-2"
              }
            >
              {e.message}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/**
 * Agent çıktısı.
 *
 * Çıktı Markdown'dır ve ham basıldığında okunmaz (spec 005). Biçimli görünüm
 * varsayılandır; ham metin bir tık uzakta durur çünkü kullanıcı çıktıyı çoğu
 * zaman başka bir yere (PR açıklaması, Jira yorumu) yapıştırıyor.
 */
function AgentOutput({ output }: { output: string }) {
  const [raw, setRaw] = useState(false);

  return (
    <Panel
      title="Agent çıktısı"
      action={
        <Button size="sm" onClick={() => setRaw((v) => !v)}>
          {raw ? "Biçimli" : "Ham metin"}
        </Button>
      }
    >
      {raw ? (
        <pre className="overflow-x-auto font-mono text-xs leading-relaxed whitespace-pre-wrap">
          {output}
        </pre>
      ) : (
        <Markdown source={output} />
      )}
    </Panel>
  );
}

/**
 * Değişiklikler — dosya çipleri + diff.
 *
 * Diff bu ekranın en geniş içeriği; yan sütun kaldırıldığı için artık tam
 * genişlikte duruyor ve satırlar sarmadan okunuyor.
 */
function Changes({ run }: { run: Run }) {
  const adds = run.files.reduce((sum, f) => sum + f.additions, 0);
  const dels = run.files.reduce((sum, f) => sum + f.deletions, 0);

  return (
    <Panel
      title="Değişiklikler"
      action={
        <span className="text-2xs tabular-nums text-ink-3">
          {formatCount(run.files.length)} dosya · +{formatCount(adds)} −
          {formatCount(dels)}
        </span>
      }
    >
      <div className="flex flex-wrap gap-2">
        {run.files.map((f) => (
          <Badge
            key={f.file}
            tone={f.status === "added" ? "success" : "neutral"}
          >
            <span className="font-mono">{f.file}</span>
            <span className="ml-1.5 text-ok">+{f.additions}</span>
            <span className="ml-1 text-danger">−{f.deletions}</span>
          </Badge>
        ))}
      </div>

      <Well className="mt-3">
        <pre className="max-h-[calc(100vh-22rem)] min-h-40 overflow-auto p-3.5 font-mono text-xs leading-relaxed">
          {run.diff.split("\n").map((line, i) => (
            <div
              key={i}
              className={
                line.startsWith("+++") || line.startsWith("---")
                  ? "text-ink-3"
                  : line.startsWith("+")
                    ? "text-ok"
                    : line.startsWith("-")
                      ? "text-danger"
                      : line.startsWith("@@")
                        ? "text-info"
                        : "text-ink-2"
              }
            >
              {line || " "}
            </div>
          ))}
        </pre>
      </Well>
    </Panel>
  );
}
