import {
  formatCompact,
  formatCount,
  formatDuration,
  formatMoney,
  formatPercent,
} from "../components/charts/format.ts";
import type { ReportSummary, ReportTotals } from "@/lib/types";

/**
 * Dönem rakamlarının TANIMI — pano ve rapor ekranlarının ortak kaynağı.
 *
 * `StatCard` kartın GÖRÜNÜŞÜNÜ ortaklıyor ve kendi yorumunda sebebini de
 * söylüyor: iki kopya kaçınılmaz olarak ayrışır. Ama rakamın KENDİSİ —
 * hangi alandan türediği, hangi biçimle yazıldığı, artışının iyi mi kötü mü
 * sayıldığı — iki ekranda ayrı ayrı yazılıydı.
 *
 * Ayrışması daha pahalı olan katman burasıydı: yanlış hizalanmış bir kart
 * gözle görülür, aynı dönemin başarı oranını iki ekranda farklı hesaplamak
 * görülmez. Pano şeridinin yorumu bu değişmezi zaten söylüyordu ("aynı
 * rakamın iki ekranda farklı biçimlenmesi, iki farklı sayı olduğu izlenimini
 * verirdi") ama yapı onu zorlamıyordu.
 *
 * GÖRÜNÜM BURADA DEĞİL: simge ve ton ekrana özgü kararlar — rapor şeridinde
 * on kart var ve göz aradığını simgeden buluyor, panodaki sekizlik şeritte
 * aynı simgeler gürültü olurdu.
 */

export type KpiKey =
  | "runs"
  | "succeeded"
  | "success"
  | "prsOpened"
  | "jiraTasks"
  | "tokens"
  | "cost"
  | "avgDuration"
  | "filesChanged"
  | "linesChanged"
  | "pushedBranches";

export interface Kpi {
  label: string;
  value: string;
  /** Değişim oranı bu ikisinden hesaplanır. */
  current: number;
  previous: number;
  /** true: artış iyi · false: artış kötü · null: yön yorumlanmaz. */
  upIsGood: boolean | null;
  /** Rakamın ALTINDAKİ kırılım — yalnızca gerekli olduğu yerde. */
  detail?: string;
}

/**
 * Biten çalıştırma sayısı.
 *
 * Başarı oranının PAYDASI bu, toplam çalıştırma değil: süren işler henüz
 * başarılı ya da başarısız değil ve paydaya konurlarsa oran, iş bittikçe
 * kendiliğinden yükselen yanlış bir sayı olur.
 */
function biten(t: ReportTotals): number {
  return t.runs - t.active;
}

/** Sıfır paydada oran 0 — bölme hatası değil, "henüz ölçülmedi". */
function oran(pay: number, payda: number): number {
  return payda > 0 ? pay / payda : 0;
}

export function kpi(key: KpiKey, data: ReportSummary): Kpi {
  const t = data.totals;
  const p = data.previous;

  switch (key) {
    case "runs":
      return {
        label: "Çalıştırma",
        value: formatCount(t.runs),
        current: t.runs,
        previous: p.runs,
        upIsGood: true,
      };

    case "succeeded":
      return {
        label: "Tamamlanan",
        value: formatCount(t.succeeded),
        current: t.succeeded,
        previous: p.succeeded,
        upIsGood: true,
      };

    case "success":
      return {
        label: "Başarı",
        value: formatPercent(t.succeeded, biten(t)),
        current: oran(t.succeeded, biten(t)),
        previous: oran(p.succeeded, biten(p)),
        upIsGood: true,
      };

    case "prsOpened":
      return {
        label: "Açılan PR",
        value: formatCount(t.prsOpened),
        current: t.prsOpened,
        previous: p.prsOpened,
        upIsGood: true,
      };

    case "jiraTasks":
      return {
        label: "Jira'dan",
        value: formatCount(t.jiraTasks),
        current: t.jiraTasks,
        previous: p.jiraTasks,
        upIsGood: true,
      };

    case "tokens":
      return {
        label: "Token",
        value: formatCompact(t.promptTokens + t.completionTokens),
        current: t.promptTokens + t.completionTokens,
        previous: p.promptTokens + p.completionTokens,
        // Yön nötr: çok token harcamak ne iyi ne kötü, işin boyutuna bağlı.
        upIsGood: null,
      };

    case "cost":
      return {
        label: "Maliyet",
        value: formatMoney(t.costUsd),
        current: t.costUsd,
        previous: p.costUsd,
        // Şeridin TEK "aşağısı iyi" rakamı: ölçek büyürken maliyetin artması
        // normaldir, yönetilebilir olan birim maliyettir.
        upIsGood: false,
      };

    case "avgDuration":
      return {
        label: "Ort. süre",
        value: formatDuration(t.avgDurationSec),
        current: t.avgDurationSec,
        previous: p.avgDurationSec,
        upIsGood: false,
      };

    case "filesChanged":
      return {
        label: "Değişen dosya",
        value: formatCompact(t.filesChanged),
        current: t.filesChanged,
        previous: p.filesChanged,
        // Yön nötr: çok dosya değiştirmek ne iyi ne kötü, bağlama bağlı.
        upIsGood: null,
      };

    case "linesChanged":
      return {
        label: "Değişen kod satırı",
        value: formatCompact(t.additions + t.deletions),
        current: t.additions + t.deletions,
        previous: p.additions + p.deletions,
        upIsGood: null,
        /*
         * Kırılım, komşu "değişen dosya" rakamından ayırt edilmesini sağlıyor:
         * ikisi tesadüfen aynı sayıya düştüğünde (her çalıştırma bir dosyada
         * bir satır değiştirdiğinde) aynı şeyi sayıyorlar sanılıyordu.
         */
        detail: `+${formatCompact(t.additions)} −${formatCompact(t.deletions)}`,
      };

    case "pushedBranches":
      return {
        label: "Gönderilen branch",
        value: formatCount(t.pushedBranches),
        current: t.pushedBranches,
        previous: p.pushedBranches,
        upIsGood: true,
      };
  }
}

/*
 * İKİ EKRANIN RAKAM SIRASI — burada, ekranların içinde değil.
 *
 * Sıralar farklı ve bu bilinçli: pano sekiz, rapor on rakam gösteriyor. Ama
 * KESİŞİMDEKİ rakamların aynı tanımdan gelmesi zorunlu, yoksa kullanıcı aynı
 * dönemi iki ekranda iki türlü okur.
 *
 * Listeler ekran dosyalarında dururken test onları GÖREMİYORDU: `page.tsx` ve
 * `reports/page.tsx` React bileşeni olduğu için `node --test` altında
 * yüklenemiyor, bu yüzden testin içine elle kopyalanmışlardı. Kopya, korumaya
 * çalıştığı ayrışmanın kendisiydi — ekran değişse test fark etmezdi.
 *
 * `as const`: eleman türü on literal anahtar oluyor ve rapor ekranındaki
 * simge tablosu eksik anahtar bırakırsa TypeScript derlemede duruyor.
 */
export const PANO_KPI = [
  "runs",
  "succeeded",
  "success",
  "prsOpened",
  "tokens",
  "cost",
  "avgDuration",
  "filesChanged",
] as const;

export const RAPOR_KPI = [
  "runs",
  "prsOpened",
  "jiraTasks",
  "tokens",
  "cost",
  "success",
  "avgDuration",
  "filesChanged",
  "linesChanged",
  "pushedBranches",
] as const;
