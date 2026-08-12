"use client";

/**
 * Halka grafik — bir bütünün parçaları.
 *
 * Panoda tek bir soruya cevap veriyor: **çalıştırmaların ne kadarı tuttu?**
 * Çubuk grafik de aynı veriyi gösterirdi ama pay/bütün ilişkisini halka
 * kadar doğrudan anlatmazdı; burada okunması gereken şey "kaç tane"
 * değil, "ne kadarı".
 *
 * Halkanın ORTASI boş bırakılmaz: toplam oraya yazılır. Boş bir delik,
 * grafiğin en çok bakılan yerini harcamak olurdu.
 *
 * RENK TEK KANAL DEĞİL: her dilimin karşılığı yanındaki listede adı, sayısı
 * ve payıyla birlikte duruyor. Grafik hiç çizilmese de bilgi eksilmez —
 * projenin grafik kuralı bu (bkz. `charts/chrome.tsx`).
 */
export interface Slice {
  key: string;
  label: string;
  value: number;
  /** CSS renk değeri — dilimin rengi. */
  color: string;
}

export function Donut({
  slices,
  centerValue,
  centerNote,
  size = 132,
  /** Dilim değerlerinin listede nasıl yazılacağı. Verilmezse ham sayı. */
  formatValue = (v) => String(v),
}: {
  slices: Slice[];
  /**
   * Ortada yazan değer — BİÇİMLENDİRİLMİŞ metin, ham sayı değil.
   *
   * Ham sayı alsaydı token gibi büyük değerlerde "3421904" yazardı;
   * biçimlendirme çağıranın işi çünkü birimini yalnızca o biliyor
   * (çalıştırma sayısı mı, token mı, para mı).
   */
  centerValue: string;
  centerNote: string;
  size?: number;
  formatValue?: (value: number) => string;
}) {
  const sum = slices.reduce((acc, s) => acc + s.value, 0);
  const radius = size / 2;
  const stroke = 14;
  // Yarıçap, çizginin ORTASINDAN geçer; dış kenar taşmasın diye yarım
  // kalınlık içeri alınır.
  const r = radius - stroke / 2;
  const circumference = 2 * Math.PI * r;

  let offset = 0;

  return (
    <div className="flex flex-wrap items-center justify-center gap-x-6 gap-y-4">
      <div className="relative shrink-0" style={{ width: size, height: size }}>
        <svg width={size} height={size} role="presentation">
          {/* Taban halka: hiç veri yokken de grafik bir şey gösterir. */}
          <circle
            cx={radius}
            cy={radius}
            r={r}
            fill="none"
            stroke="var(--color-raised)"
            strokeWidth={stroke}
          />
          {sum > 0 &&
            slices.map((s) => {
              const fraction = s.value / sum;
              const dash = fraction * circumference;
              const el = (
                <circle
                  key={s.key}
                  cx={radius}
                  cy={radius}
                  r={r}
                  fill="none"
                  stroke={s.color}
                  strokeWidth={stroke}
                  strokeDasharray={`${dash} ${circumference - dash}`}
                  strokeDashoffset={-offset}
                  // Saat 12'den başlasın: varsayılan başlangıç saat 3 yönü ve
                  // ilk dilim orada başlayınca halka yamuk okunuyor.
                  transform={`rotate(-90 ${radius} ${radius})`}
                />
              );
              offset += dash;
              return el;
            })}
        </svg>

        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-xl leading-none font-semibold tabular-nums">
            {centerValue}
          </span>
          <span className="mt-1 text-2xs text-ink-3">{centerNote}</span>
        </div>
      </div>

      <ul className="min-w-0 flex-1 space-y-2">
        {slices.map((s) => (
          <li key={s.key} className="flex items-center gap-2 text-xs">
            <span
              aria-hidden="true"
              className="size-2.5 shrink-0 rounded-[3px]"
              style={{ background: s.color }}
            />
            <span className="min-w-0 flex-1 truncate text-ink-2">{s.label}</span>
            <span className="shrink-0 tabular-nums">{formatValue(s.value)}</span>
            <span className="w-10 shrink-0 text-right tabular-nums text-ink-3">
              {sum > 0 ? `%${Math.round((s.value / sum) * 100)}` : "—"}
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
