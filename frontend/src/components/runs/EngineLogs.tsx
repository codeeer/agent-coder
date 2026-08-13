"use client";

import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import { formatCount } from "@/components/charts/format";
import { IconDownload } from "@/components/ui/icons";
import {
  SessionTranscript,
  parseTranscript,
} from "@/components/runs/SessionTranscript";
import {
  Button,
  Notice,
  SearchField,
  Segmented,
  Skeleton,
} from "@/components/ui/primitives";

/**
 * Motorun ham logları.
 *
 * İlerleme akışı "ne oldu"yu anlatır; bu "tam olarak ne yazıldı"yı. Runner
 * container'ı koşu bitince siliniyor, bu yüzden içerik veritabanından
 * okunuyor — sayfa açıldığında container çoktan yok olmuş oluyor.
 */

/*
 * Kaynaklar, ham olandan anlamlı olana doğru sıralı.
 *
 * "Oturum", ilerleme akışının yedeği: SSE bağlantısı koptuğunda ilerleme
 * kaydı eksik kalır ama motorun kendi oturum deposu tamdır. Bir koşuda ne
 * konuşulduğu sorusunun son başvuru yeri burası.
 */
const KAYNAKLAR = [
  { id: "stdout", label: "Container" },
  { id: "file", label: "Motor" },
  { id: "session", label: "Oturum" },
] as const;

type KaynakID = (typeof KAYNAKLAR)[number]["id"];

/**
 * Ham boyut.
 *
 * "0 KB" YAZILMAZ: 310 baytlık gerçek bir log, sıfıra yuvarlanınca boşmuş
 * gibi görünür — oysa kullanıcı tam da o satırları okuyor.
 */
function boyut(bayt: number): string {
  if (bayt < 1024) return `${bayt} B`;
  if (bayt < 1024 * 1024)
    return `${(bayt / 1024).toFixed(bayt < 10240 ? 1 : 0)} KB`;
  return `${(bayt / 1024 / 1024).toFixed(1)} MB`;
}

/** Bir satırın önem düzeyi — vurgulama için. */
function seviye(satir: string): "error" | "warn" | null {
  if (/\blevel=ERROR\b|\bERROR\b|\berror\b:/.test(satir)) return "error";
  if (/\blevel=WARN\b|\bWARN\b|\bwarning\b:/i.test(satir)) return "warn";
  return null;
}

export function EngineLogs({ runId, live }: { runId: string; live: boolean }) {
  const [kaynak, setKaynak] = useState<KaynakID>("stdout");
  const [q, setQ] = useState("");
  const [ham, setHam] = useState(false);

  const logs = useQuery({
    queryKey: ["engine-logs", runId],
    queryFn: () => api.runs.engineLogs(runId),
    // Koşu sürerken loglar henüz yazılmamış olabilir: toplama container
    // silinmeden HEMEN ÖNCE yapılıyor. Süren koşuda tazeleyip bekliyoruz.
    refetchInterval: live ? 5000 : false,
  });

  const secili = logs.data?.items.find((l) => l.source === kaynak);

  /*
   * Oturum geçmişi JSON; ayrıştırılabiliyorsa konuşma olarak gösterilir.
   *
   * Ayrıştırılamıyorsa ham tabloya düşer — motorun biçimi değiştiğinde boş
   * ekran göstermek, elimizdeki tek kaydı yok saymak olurdu.
   */
  const konusma = useMemo(
    () =>
      kaynak === "session" && secili ? parseTranscript(secili.content) : null,
    [kaynak, secili],
  );
  const konusmaGoster = konusma !== null && !ham;

  const satirlar = useMemo(() => {
    const hepsi = (secili?.content ?? "").split("\n");
    const needle = q.trim().toLocaleLowerCase("tr");
    if (needle === "") return hepsi.map((metin, i) => ({ no: i + 1, metin }));
    // Satır numarası ARAMADAN ÖNCEKİ numaradır: kullanıcı süzülmüş listede
    // gördüğü satırı ham metinde bulabilmeli.
    return hepsi
      .map((metin, i) => ({ no: i + 1, metin }))
      .filter((s) => s.metin.toLocaleLowerCase("tr").includes(needle));
  }, [secili, q]);

  function indir() {
    if (!secili) return;
    const url = URL.createObjectURL(
      new Blob([secili.content], { type: "text/plain;charset=utf-8" }),
    );
    const a = document.createElement("a");
    a.href = url;
    a.download = `run-${runId}-${kaynak}.log`;
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    /*
     * BAŞLIK ŞERİDİ YOK.
     *
     * Sekmenin adı zaten "Motor logları"; panelin de aynı başlığı taşıması
     * ekranın en üstünde aynı kelimeyi iki kez yazmak olurdu. Bunun yerine
     * tek bir araç çubuğu var: kaynak, arama, ölçü ve indirme aynı satırda.
     */
    <section className="flex flex-col overflow-hidden rounded-card border border-line bg-surface shadow-(--shadow-card)">
      {logs.isPending && (
        <div className="px-4 py-3.5">
          <Skeleton rows={3} />
        </div>
      )}
      {logs.isError && (
        <div className="px-4 py-3.5">
          <Notice tone="error">{describeError(logs.error).message}</Notice>
        </div>
      )}

      {logs.data && logs.data.items.length === 0 && (
        <p className="px-4 py-3.5 text-sm text-ink-3">
          {live
            ? "Loglar çalışma bitince toplanacak — container silinmeden hemen önce."
            : "Bu çalıştırma için saklanmış log yok. Ayarlardan saklama kapatılmış veya saklama süresi dolmuş olabilir."}
        </p>
      )}

      {logs.data && logs.data.items.length > 0 && (
        <>
          <div className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-line px-4 py-2.5">
            <Segmented
              label="Log kaynağı"
              options={KAYNAKLAR.filter((k) =>
                logs.data.items.some((l) => l.source === k.id),
              )}
              value={kaynak}
              onChange={setKaynak}
            />
            {/* Arama telefonda ALT SATIRA iner (`order-last`): denetimler tek
                sıraya sığmayınca en geniş olanı aşağı almak, üç satıra
                kırılmaktan iyi. Geniş ekranda sıra bozulmaz.

                Konuşma görünümünde arama YOK: metin katlanmış bloklar içinde
                duruyor ve bulunamayan bir eşleşme, aramanın çalışmadığını
                düşündürürdü. Ham görünüme geçildiğinde geri geliyor. */}
            {!konusmaGoster && (
              <SearchField
                className="order-last w-full sm:order-0 sm:w-auto sm:max-w-xs sm:min-w-45 sm:flex-1"
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="Log içinde ara…"
                aria-label="Motor loglarında ara"
              />
            )}
            <span className="ml-auto text-2xs text-ink-3">
              {konusmaGoster
                ? `${formatCount(konusma.length)} mesaj`
                : q.trim() === ""
                  ? `${formatCount(satirlar.length)} satır`
                  : `${formatCount(satirlar.length)} eşleşme`}
              {secili && ` · ${boyut(secili.rawSize)}`}
            </span>
            {/* Ham JSON anahtarı yalnızca ayrıştırılabilen geçmişte anlamlı;
                ayrıştırılamayan içerik zaten ham gösteriliyor. */}
            {konusma !== null && (
              <Segmented
                label="Oturum görünümü"
                options={[
                  { id: "konusma", label: "Konuşma" },
                  { id: "ham", label: "Ham JSON" },
                ]}
                value={ham ? "ham" : "konusma"}
                onChange={(v) => setHam(v === "ham")}
              />
            )}
            <Button
              size="sm"
              icon={<IconDownload className="size-4" />}
              onClick={indir}
            >
              İndir
            </Button>
          </div>

          {/* Kırpma SÖYLENİR: eksik bir metne baktığını bilmeyen kullanıcı,
              olmayan bir satırı arar durur. */}
          {secili?.truncated && (
            <p className="border-b border-warn/30 bg-warn-soft px-4 py-2 text-2xs text-ink">
              Bu log boyut sınırını aştı; <strong>başı kırpıldı</strong>, son
              kısmı duruyor. Sınır: Ayarlar → Çalıştırma.
            </p>
          )}

          {/* Yatay kaydırma KENDİ KABINDA: uzun bir log satırı sayfayı
              yatayda kaydırmamalı. */}
          <div className="max-h-140 overflow-auto">
            {konusmaGoster && <SessionTranscript mesajlar={konusma} />}
            {!konusmaGoster && (
              <table className="w-full border-collapse font-mono text-2xs">
                <tbody>
                  {satirlar.map((s) => {
                    const lvl = seviye(s.metin);
                    return (
                      <tr
                        key={s.no}
                        className={
                          lvl === "error"
                            ? "bg-danger-soft/60"
                            : lvl === "warn"
                              ? "bg-warn-soft/60"
                              : ""
                        }
                      >
                        {/* Satır numarası seçilemez: kullanıcı logu kopyalarken
                          numaraları da almasın. */}
                        <td className="w-12 shrink-0 border-r border-line px-2 py-0.5 text-right align-top text-ink-3 select-none">
                          {s.no}
                        </td>
                        <td
                          className={`px-3 py-0.5 break-all whitespace-pre-wrap ${
                            lvl === "error"
                              ? "text-danger"
                              : lvl === "warn"
                                ? "text-ink"
                                : "text-ink-2"
                          }`}
                        >
                          {s.metin || "\u00a0"}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}

            {!konusmaGoster && satirlar.length === 0 && (
              <p className="px-4 py-3.5 text-sm text-ink-3">
                Aramaya uyan satır yok.
              </p>
            )}
          </div>
        </>
      )}
    </section>
  );
}
