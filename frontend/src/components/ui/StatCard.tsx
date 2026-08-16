"use client";

import { changeRatio, formatPercent } from "@/components/charts/format";
import { IconTile, type TileTone } from "@/components/ui/primitives";

/**
 * Rakam kartı — panonun ve raporun ortak KPI parçası.
 *
 * Rapor ekranında yazılmıştı ve orada kalmıştı; panoya da aynı şerit
 * gerekince ikinci bir kopya çıkacaktı. İki kopya kaçınılmaz olarak
 * ayrışır: birinde yön oku, diğerinde yüzde işareti farklı durur ve aynı
 * rakam iki ekranda iki türlü okunur.
 *
 * Kart üç şey söyler ve üçü de zorunludur: NE olduğu, KAÇ olduğu, hangi
 * YÖNE gittiği. Yön olmadan bir rakam bilgi değil, süstür.
 */
export interface StatCardProps {
  label: string;
  value: string;
  /** Değişim oranı bu ikisinden hesaplanır. */
  current: number;
  previous: number;
  /** true: artış iyi · false: artış kötü · null: yön yorumlanmaz. */
  upIsGood: boolean | null;
  /**
   * Karşılaştırmanın neye göre olduğu.
   *
   * KISA olmak zorunda. Sekiz kart tek sıraya dizildiğinde karta ~125px
   * kalıyor ve bu satır 11px'te ancak yirmi küsur karakter alıyor; "son 7
   * güne göre" yazıldığında ekranda "son 7 güne g…" görünüyordu — kırpılmış
   * bir karşılaştırma, karşılaştırma değildir.
   */
  periodNote: string;
  /**
   * Rakamın simgesi.
   *
   * SÜS DEĞİL: on kart yan yana dizildiğinde göz aradığı rakamı etiketi
   * okuyarak değil, simgesinden buluyor. Verilmezse kart simgesiz çizilir —
   * dört kartlık bir şeritte simge kazanç sağlamaz, gürültü olur.
   */
  icon?: React.ReactNode;
  /** Simge karosunun tonu. Rakamı DEĞİL yalnızca karoyu boyar. */
  tone?: TileTone;
  /**
   * Rakamın ALTINDAKİ kırılım — "+9 −0" gibi.
   *
   * Bir rakam tek başına iki farklı şeye benzeyebiliyorsa ayrımı bu satır
   * yapar. Gerçek bir vaka: "değişen dosya 9" ile "değişen kod satırı 9"
   * yan yana duruyordu ve aynı sayı olmaları hata sanıldı — oysa dokuz
   * çalıştırmanın her biri bir dosyada bir satır değiştirmişti. Kırılım
   * yazılınca ikisinin farklı şeyi saydığı okunuyor.
   */
  detail?: string;
}

export function StatCard({
  label,
  value,
  current,
  previous,
  upIsGood,
  periodNote,
  icon,
  tone = "accent",
  detail,
}: StatCardProps) {
  const ratio = changeRatio(current, previous);
  const flat = ratio !== null && Math.abs(ratio) < 0.005;
  const good = ratio !== null && ratio > 0 === upIsGood;

  return (
    <div className="rounded-card border border-line bg-surface px-4 py-3.5 shadow-(--shadow-card)">
      <div className="flex items-center gap-2">
        {icon && (
          <IconTile tone={tone} size="sm">
            {icon}
          </IconTile>
        )}
        <div className="min-w-0 flex-1 truncate text-2xs font-medium tracking-wide text-ink-3 uppercase">
          {label}
        </div>
      </div>

      {/*
        Kıvılcım grafiği KALDIRILDI.

        Karta sığan genişlik ~64px'ti ve otuz günlük bir seri o genişlikte
        okunmuyordu: eğrinin yönü bile seçilemiyordu, yalnızca rakamın
        yanında bir gürültü lekesi duruyordu. Yönü zaten altındaki değişim
        satırı SÖZLE söylüyor ve günlük seyrin okunur hali "Günlük özet"
        panosunda tam boy duruyor.
      */}
      <div className="mt-2.5">
        <span className="text-xl leading-none font-semibold tabular-nums">
          {value}
        </span>
      </div>

      {/*
        Kırılım rakamın YANINDA değil ALTINDA.

        Yanında dururken kırpılıyordu: beş sütunlu şeritte karta ~153px kalıyor,
        rakam ve kırılım yan yana ~158px istiyor. Ölçüldü — ekranda
        "+428,9 B −…" görünüyordu, yani satır tam da var olma sebebini
        ("değişen dosya" ile "değişen kod satırı"nı ayırt etmek) yerine
        getiremiyordu. Kırpılmış bir kırılım, kırılım değildir.

        Kendi satırında tam genişlik buluyor ve yalnızca kırılımı olan kartı
        bir satır uzatıyor.
      */}
      {detail && (
        <div className="mt-1 text-2xs tabular-nums text-ink-3">{detail}</div>
      )}

      <div className="mt-2.5 truncate text-2xs">
        {ratio === null ? (
          <span className="text-ink-3">önceki dönem yok</span>
        ) : (
          <>
            <span
              className={
                flat || upIsGood === null
                  ? "text-ink-3"
                  : good
                    ? "font-medium text-ok"
                    : "font-medium text-danger"
              }
            >
              {flat
                ? "≈ aynı"
                : `${ratio > 0 ? "↑" : "↓"} ${formatPercent(Math.abs(ratio), 1)}`}
            </span>{" "}
            <span className="text-ink-3">{periodNote}</span>
          </>
        )}
      </div>
    </div>
  );
}
