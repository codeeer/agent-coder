"use client";

import { useState } from "react";
import { formatCompact, formatMoney } from "@/components/charts/format";
import {
  IconAgent,
  IconChevronDown,
  IconChevronRight,
  IconComment,
  IconTerminal,
} from "@/components/ui/icons";
import { Badge, formatDate } from "@/components/ui/primitives";

/**
 * Motorun oturum geçmişi — okunur hâli.
 *
 * Ham JSON teşhis için doğru ama okunacak şey değil: 348 satırın büyük kısmı
 * kimlik, damga ve anlık görüntü karması. Kullanıcının aradığı şey ise dört
 * cümlelik bir konuşma ve iki araç çağrısı.
 *
 * Ham metin KAYBOLMUYOR — "Ham JSON" anahtarı ve indirme düğmesi yerinde.
 * Burada değişen, varsayılan olarak neyin gösterildiği.
 */

type Parca = {
  type: string;
  text?: string;
  tool?: string;
  state?: {
    status?: string;
    title?: string;
    input?: unknown;
    output?: string;
  };
  cost?: number;
  tokens?: {
    input?: number;
    output?: number;
    reasoning?: number;
    cache?: { read?: number; write?: number };
  };
};

type Mesaj = {
  info?: {
    role?: string;
    agent?: string;
    modelID?: string;
    providerID?: string;
    cost?: number;
    time?: { created?: number };
    model?: { modelID?: string };
  };
  parts?: Parca[];
};

/**
 * parse, ham metni mesaj dizisine çevirir.
 *
 * Başarısızlık `null` döner ve çağıran ham görünüme düşer: biçimi değişmiş
 * bir motor sürümünde boş ekran göstermek, teşhis verisini yok saymak olurdu.
 */
export function parseTranscript(raw: string): Mesaj[] | null {
  try {
    const v: unknown = JSON.parse(raw);
    if (!Array.isArray(v) || v.length === 0) return null;
    return v as Mesaj[];
  } catch {
    return null;
  }
}

/** Bir mesajın toplam maliyeti ve token'ı — parçalardan toplanır. */
function olcu(m: Mesaj) {
  let cost = m.info?.cost ?? 0;
  let girdi = 0;
  let cikti = 0;
  for (const p of m.parts ?? []) {
    if (p.type !== "step-finish") continue;
    cost = cost || (p.cost ?? 0);
    const t = p.tokens;
    if (!t) continue;
    girdi += (t.input ?? 0) + (t.cache?.read ?? 0) + (t.cache?.write ?? 0);
    cikti += (t.output ?? 0) + (t.reasoning ?? 0);
  }
  return { cost, girdi, cikti };
}

export function SessionTranscript({ mesajlar }: { mesajlar: Mesaj[] }) {
  return (
    <div className="divide-y divide-line">
      {mesajlar.map((m, i) => (
        <MesajBloku key={i} mesaj={m} />
      ))}
    </div>
  );
}

function MesajBloku({ mesaj }: { mesaj: Mesaj }) {
  const kullanici = mesaj.info?.role === "user";
  const { cost, girdi, cikti } = olcu(mesaj);
  const zaman = mesaj.info?.time?.created;

  // Adım işaretleri gösterilmez: `step-start` ve `step-finish` motorun iç
  // muhasebesi. Taşıdıkları ölçüler başlığa çıkıyor, kendileri gürültü.
  const parcalar = (mesaj.parts ?? []).filter(
    (p) => p.type !== "step-start" && p.type !== "step-finish",
  );

  return (
    <div className="px-4 py-3">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-2xs text-ink-3">
        {kullanici ? (
          <IconComment className="size-3.5 shrink-0 text-ink-3" />
        ) : (
          <IconAgent className="size-3.5 shrink-0 text-accent" />
        )}
        <span className="font-medium text-ink-2">
          {kullanici ? "Görev" : (mesaj.info?.agent ?? "agent")}
        </span>
        {mesaj.info?.modelID && (
          <span className="font-mono">{mesaj.info.modelID}</span>
        )}
        {/* Ölçüler yalnızca VARSA yazılır: sıfır maliyet, ölçülmemiş bir
            adımı bedavaymış gibi gösterirdi. */}
        {cost > 0 && <span>{formatMoney(cost)}</span>}
        {girdi + cikti > 0 && (
          <span title={`${girdi} girdi · ${cikti} çıktı`}>
            {formatCompact(girdi + cikti)} token
          </span>
        )}
        {/* Motor damgayı epoch MİLİSANİYE olarak yazıyor; formatDate ISO
            bekliyor. Çevrim burada yapılıyor ki biçim tek yerde kalsın. */}
        {zaman && (
          <span className="ml-auto">
            {formatDate(new Date(zaman).toISOString())}
          </span>
        )}
      </div>

      <div className="mt-2 space-y-2">
        {parcalar.map((p, i) => (
          <ParcaGovdesi key={i} parca={p} />
        ))}
        {parcalar.length === 0 && (
          <p className="text-xs text-ink-3">Bu adımda içerik yok.</p>
        )}
      </div>
    </div>
  );
}

function ParcaGovdesi({ parca }: { parca: Parca }) {
  if (parca.type === "text") {
    return (
      <p className="text-sm break-words whitespace-pre-wrap text-ink">
        {parca.text}
      </p>
    );
  }

  if (parca.type === "reasoning") {
    // Akıl yürütme KAPALI başlar: uzun, tekrarlı ve genelde sonucu
    // değiştirmiyor. Arayan bilerek arıyor.
    return (
      <Katlanir baslik="Akıl yürütme" sessiz>
        <p className="text-xs break-words whitespace-pre-wrap text-ink-2">
          {parca.text}
        </p>
      </Katlanir>
    );
  }

  if (parca.type === "tool") {
    const st = parca.state ?? {};
    const basarisiz = st.status !== undefined && st.status !== "completed";
    return (
      <Katlanir
        baslik={
          <span className="flex min-w-0 items-center gap-2">
            <IconTerminal className="size-3.5 shrink-0 text-ink-3" />
            <span className="font-mono text-xs text-ink">{parca.tool}</span>
            {st.title && (
              <span className="truncate text-2xs text-ink-3">{st.title}</span>
            )}
            {/* Durum rozeti yalnızca SORUN varsa: her satırda "tamamlandı"
                yazmak, gerçekten başarısız olanı görünmez yapardı. */}
            {basarisiz && <Badge tone="warning">{st.status}</Badge>}
          </span>
        }
      >
        <div className="space-y-2">
          {st.input !== undefined && (
            <Alan etiket="Girdi" icerik={JSON.stringify(st.input, null, 2)} />
          )}
          {st.output && <Alan etiket="Çıktı" icerik={st.output} />}
        </div>
      </Katlanir>
    );
  }

  // Tanınmayan parça tipi ATLANMAZ, ham gösterilir: motor yeni bir tip
  // eklediğinde bunun sessizce kaybolması, geçmişin eksik olması demek.
  return (
    <Katlanir baslik={`Bilinmeyen parça: ${parca.type}`} sessiz>
      <Alan etiket="" icerik={JSON.stringify(parca, null, 2)} />
    </Katlanir>
  );
}

function Alan({ etiket, icerik }: { etiket: string; icerik: string }) {
  return (
    <div>
      {etiket && (
        <p className="mb-1 text-2xs tracking-wide text-ink-3 uppercase">
          {etiket}
        </p>
      )}
      {/* Uzun çıktı KENDİ KABINDA kayar; sayfayı uzatmaz. */}
      <pre className="max-h-64 overflow-auto rounded-md border border-line bg-canvas px-2.5 py-2 font-mono text-2xs break-words whitespace-pre-wrap text-ink-2">
        {icerik}
      </pre>
    </div>
  );
}

function Katlanir({
  baslik,
  children,
  sessiz = false,
}: {
  baslik: React.ReactNode;
  children: React.ReactNode;
  sessiz?: boolean;
}) {
  const [acik, setAcik] = useState(false);
  return (
    <div className="rounded-md border border-line">
      <button
        type="button"
        onClick={() => setAcik((v) => !v)}
        aria-expanded={acik}
        className={`flex w-full items-center gap-2 px-2.5 py-1.5 text-left transition-colors hover:bg-raised ${
          sessiz ? "text-2xs text-ink-3" : ""
        }`}
      >
        {acik ? (
          <IconChevronDown className="size-3.5 shrink-0 text-ink-3" />
        ) : (
          <IconChevronRight className="size-3.5 shrink-0 text-ink-3" />
        )}
        {baslik}
      </button>
      {acik && (
        <div className="border-t border-line px-2.5 py-2">{children}</div>
      )}
    </div>
  );
}
