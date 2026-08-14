/**
 * Paylaşılan görsel parçalar.
 *
 * Amaç tam bir tasarım sistemi değil; aynı görünümün her sayfada yeniden
 * yazılmasını önlemek ve hiyerarşiyi tutarlı tutmak.
 */

import type React from "react";
import { IconAlert, IconSearch } from "@/components/ui/icons";

// ─── Yüzey ──────────────────────────────────────────────────────────────────

export function Card({
  children,
  className = "",
  padded = true,
}: {
  children: React.ReactNode;
  className?: string;
  padded?: boolean;
}) {
  return (
    <div
      className={`rounded-card border border-line bg-surface shadow-(--shadow-card) ${
        padded ? "p-5" : ""
      } ${className}`}
    >
      {children}
    </div>
  );
}

/**
 * Liste kabı — kayıt listeleri için TEK deyim.
 *
 * Öncesinde uygulamada üç ayrı liste dili vardı: Projeler ve Agent'lar her
 * kaydı ayrı bir `Card` olarak çiziyordu (kayıt başına bir kenarlık, bir
 * gölge, aralarında boşluk), Panel ve Çalıştırmalar tek kap + ayraç
 * kullanıyordu, Modeller ise tablo. Aynı üründe aynı işin üç görünümü.
 *
 * Doğrusu tek kap: bir liste TEK bir şeydir. Her satırı bağımsız bir yüzey
 * yapmak aralarındaki ilişkiyi görsel olarak koparır ve ekranı kenarlıkla
 * doldurur.
 *
 * Ayraçlar `divide-y` ile KAPTAN geliyor, satırdan değil. Öncesinde her
 * liste `i > 0 ? "border-t" : ""` diye kendi indeks mantığını taşıyordu —
 * üç yerde tekrarlanan, ilk satırda yanlışlıkla çizgi bırakmaya açık bir
 * kalıptı. Satırlar artık sırasını bilmek zorunda değil ve `<Link>` de
 * `<div>` de olabilirler.
 */
export function List({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`divide-y divide-line overflow-hidden rounded-card border border-line bg-surface ${className}`}
    >
      {children}
    </div>
  );
}

/**
 * Pano — başlığı KENDİ İÇİNDE taşıyan kart.
 *
 * `Section` başlığı kartın DIŞINA koyar; tek bir kartın üstünde bu doğru
 * çalışır ama yan yana üç pano dizildiğinde çalışmaz: başlıklar kartların
 * dışında havada durur, kartların üst kenarları hizalanmaz ve ızgara
 * dağılır. Referans tasarımın bütün panoları başlığını içeride, ince bir
 * ayraçla ayrılmış bir şeritte taşıyor — bu bileşen onu veriyor.
 *
 * Sağdaki `action` neredeyse her zaman "tümünü gör" bağlantısıdır: pano
 * bir listenin ilk beş satırını gösterir, tamamı başka bir ekrandadır.
 */
export function Panel({
  title,
  description,
  action,
  children,
  /** Gövde kendi dolgusunu yönetsin — tablo ve tam genişlikli listeler için. */
  padded = true,
  className = "",
}: {
  title: React.ReactNode;
  /**
   * Başlığın altındaki açıklama.
   *
   * Panoda genellikle YOKTUR: bir panonun her kutusuna ne işe yaradığını
   * anlatan bir paragraf koymak panoyu belgeye çevirir. Ayarlar ekranında
   * ise gerekli — orada her bölüm bir KARAR istiyor ve kararın sonucunu
   * ancak açıklama söylüyor.
   */
  description?: React.ReactNode;
  action?: React.ReactNode;
  children: React.ReactNode;
  padded?: boolean;
  className?: string;
}) {
  return (
    <section
      className={`flex flex-col overflow-hidden rounded-card border border-line bg-surface shadow-(--shadow-card) ${className}`}
    >
      {/* Açıklama yoksa şerit sabit 48px: yan yana dizilen panoların
          başlıkları aynı hizada dursun. Açıklama varsa yüksekliği
          içeriği belirler. */}
      <header
        className={`flex shrink-0 items-start justify-between gap-3 border-b border-line px-4 ${
          description ? "py-3" : "h-12 items-center"
        }`}
      >
        <div className="min-w-0">
          <h2 className="truncate text-sm font-semibold tracking-[-0.01em]">
            {title}
          </h2>
          {description && (
            <p className="mt-1 max-w-3xl text-xs leading-relaxed text-ink-2">
              {description}
            </p>
          )}
        </div>
        {action && <div className="flex shrink-0 items-center gap-1.5">{action}</div>}
      </header>
      <div className={`min-w-0 flex-1 ${padded ? "p-4" : ""}`}>{children}</div>
    </section>
  );
}

/**
 * Pano başlığındaki "tümü" bağlantısı.
 *
 * `Button` DEĞİL: pano başlığında altı düğme yan yana durunca şerit bir
 * araç çubuğuna dönüşüyor ve panonun asıl işi olan başlık okunmuyor.
 * Referansta da bunlar düğme değil, sessiz birer bağlantı.
 */
export const panelLinkClass =
  "rounded text-xs font-medium text-ink-3 transition-colors hover:text-accent";

/**
 * Pano içindeki kayıt kutusu.
 *
 * KART İÇİNDE KART OLMASIN diye var. Ayarlar ekranında her bölüm bir pano
 * (`bg-surface`) ve içindeki her kayıt — sağlayıcı, MCP sunucusu, betik —
 * bir `Card`'dı, yani AYNI zeminden ikinci bir kutu. İki yüzey aynı renkte
 * olunca aralarındaki ilişkiyi yalnızca ince bir kenarlık taşıyordu ve
 * ekran, iç içe geçmiş kutulardan oluşan bir yığın gibi okunuyordu.
 *
 * Bir kademe yukarı (`raised`) çıkınca hiyerarşi görünür oluyor: pano
 * kaptır, içindekiler onun elemanları.
 */
export function PanelCard({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-lg border border-line bg-raised p-4 ${className}`}>
      {children}
    </div>
  );
}

/** Kart içinde ikinci düzey yüzey — kod, diff, ayar satırı gibi bloklar. */
export function Well({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-lg border border-line bg-raised ${className}`}>
      {children}
    </div>
  );
}

// ─── Sayfa iskeleti ─────────────────────────────────────────────────────────

/**
 * Sayfa başlığı — arayüzün TEK üst şeridi.
 *
 * Referans tasarımın en belirgin yapısal kararı bu: ekranın en üstünde tek
 * bir satır var ve o satır hem sayfanın kim olduğunu (başlık + alt satır)
 * hem de genel denetimleri (tema) taşıyor. İkinci bir uygulama çubuğu yok.
 *
 * Bu yüzden tema anahtarı buraya taşındı; kenar çubuğunun dibinde
 * duruyordu. Oradaki yeri referansta SİSTEM DURUMUNA ait (bkz. Sidebar) ve
 * daha önemlisi: tema, sayfanın değil arayüzün ayarı — sayfa başlığının
 * sağ ucu, her ekranda aynı yerde duran tek noktadır.
 *
 * Alttaki kenarlık kaldırıldı. Başlığı içerikten ayıran şey artık boşluk;
 * altındaki araç çubuğu ve panolar zaten kendi kenarlıklarını taşıyor ve
 * üst üste üç yatay çizgi ekranı katlara bölüyordu.
 */
export function PageHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: React.ReactNode;
  actions?: React.ReactNode;
}) {
  return (
    <header className="mb-5 flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
      <div className="min-w-0">
        <h1 className="text-xl font-semibold">{title}</h1>
        {description && (
          <p className="mt-1 max-w-3xl text-sm leading-relaxed text-ink-2">
            {description}
          </p>
        )}
      </div>

      {/* `flex-wrap`: dar ekranda beş düğmelik bir sıra tek satıra sığmıyor ve
          `shrink-0` yüzünden sondakiler ekranın dışına taşıyordu — akış
          ekranında "Kaydet" düğmesi telefonda hiç görünmüyordu. */}
      {/* Yalnızca sayfanın KENDİ eylemleri. Tema anahtarı buradan alındı ve
          kenar çubuğunun dibine taşındı: genel bir tercih, sayfanın birincil
          eylemiyle aynı yerde durunca onunla ağırlık yarışına giriyordu. */}
      <div className="flex flex-wrap items-center gap-2">{actions}</div>
    </header>
  );
}

export function Section({
  title,
  description,
  actions,
  children,
}: {
  title: string;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section>
      <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold tracking-[-0.01em]">{title}</h2>
          {description && (
            <p className="mt-1 max-w-2xl text-sm leading-relaxed text-ink-2">
              {description}
            </p>
          )}
        </div>
        {actions && <div className="flex flex-wrap gap-2">{actions}</div>}
      </div>
      {children}
    </section>
  );
}

/**
 * Araç çubuğu — listenin üstündeki arama + süzgeç şeridi.
 *
 * Referansın her liste ekranında var ve hep aynı yerde: başlığın altı,
 * listenin üstü. Kendi yüzeyi VAR (kart gibi) çünkü listeyi süzen
 * denetimler listenin bir parçası değil, ona uygulanan bir katman —
 * zeminde serbest duran denetimler sayfanın neresine ait olduğunu
 * söylemiyordu.
 */
export function Toolbar({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`mb-4 flex flex-wrap items-center gap-2 rounded-card border border-line bg-surface p-2 ${className}`}
    >
      {children}
    </div>
  );
}

/**
 * Arama alanı — solda büyüteç, sağda kısayol ipucu.
 *
 * İpucu (`⌘K` gibi) SÜS DEĞİL: kısayol gerçekten bağlıysa verilir. Bağlı
 * olmayan bir kısayolu yazmak, çalışmayan bir söz vermek olur.
 */
export function SearchField({
  className = "",
  hint,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement> & { hint?: string }) {
  return (
    <div className={`relative min-w-0 ${className}`}>
      <span className="pointer-events-none absolute top-1/2 left-3 -translate-y-1/2 text-ink-3">
        <IconSearch className="size-4" />
      </span>
      <input
        {...props}
        className={`${fieldBase} h-9 bg-canvas pl-9 ${hint ? "pr-14" : ""}`}
      />
      {hint && (
        <span className="pointer-events-none absolute top-1/2 right-2.5 -translate-y-1/2 rounded border border-line px-1.5 py-px font-mono text-2xs text-ink-3">
          {hint}
        </span>
      )}
    </div>
  );
}

/**
 * Segment düğmeleri — birbirini dışlayan birkaç seçenek.
 *
 * Aynı kalıp üç ekranda üç ayrı yerde elle yazılmıştı (rapor dönemi,
 * çalıştırma süzgeci, tema anahtarı): biri kutuyu `p-0.5` ile içeriden
 * dolduruyordu, diğeri bölmeleri `border-l` ile ayırıyordu, üçüncüsü
 * yuvarlaklığı farklıydı. Aynı işin üç görünümü.
 */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
  label,
}: {
  options: ReadonlyArray<{ id: T; label: React.ReactNode; title?: string }>;
  value: T;
  onChange: (id: T) => void;
  label: string;
}) {
  return (
    <div
      role="group"
      aria-label={label}
      className="flex shrink-0 rounded-lg border border-line bg-canvas p-0.5"
    >
      {options.map((o) => (
        <button
          key={o.id}
          type="button"
          title={o.title}
          onClick={() => onChange(o.id)}
          aria-pressed={value === o.id}
          className={`flex h-7 items-center gap-1.5 rounded-md px-2.5 text-xs whitespace-nowrap transition-colors duration-150 ${
            value === o.id
              ? "bg-accent-soft font-medium text-accent"
              : "text-ink-2 hover:bg-raised hover:text-ink"
          }`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

// ─── İkon karosu ────────────────────────────────────────────────────────────

/**
 * Renkli ikon karosu — liste satırının solundaki kimlik işareti.
 *
 * Referansın liste satırlarını okunur kılan asıl şey bu: göz, satırı
 * metinden değil karodan buluyor. Karo bir DURUM ANLATMAZ; yalnızca
 * satırları birbirinden ayırır, bu yüzden rengi kaydın kimliğinden
 * türetilir (`toneFromKey`) ve zaman içinde sabit kalır.
 *
 * Durum renkleriyle karışmaması için karo her zaman YUMUŞAK zeminlidir;
 * dolu bir renk kullanılsaydı yanındaki durum rozetiyle yarışırdı.
 */
export type TileTone = "accent" | "info" | "success" | "warning" | "danger" | "series";

const tileTones: Record<TileTone, string> = {
  accent: "bg-accent-soft text-accent",
  info: "bg-info-soft text-info",
  success: "bg-ok-soft text-ok",
  warning: "bg-warn-soft text-warn",
  danger: "bg-danger-soft text-danger",
  series: "bg-series-soft text-series",
};

const TILE_TONES: readonly TileTone[] = [
  "accent",
  "info",
  "success",
  "warning",
  "danger",
  "series",
];

/**
 * Kimlikten sabit bir renk üretir.
 *
 * Sırayla dağıtılsaydı (`i % 6`) sayfa değiştikçe aynı kaydın rengi
 * değişirdi — kullanıcı ikinci sayfada aradığı akışı rengiyle bulamazdı.
 */
export function toneFromKey(key: string): TileTone {
  let hash = 0;
  for (let i = 0; i < key.length; i++) hash = (hash * 31 + key.charCodeAt(i)) >>> 0;
  return TILE_TONES[hash % TILE_TONES.length]!;
}

export function IconTile({
  tone = "accent",
  size = "md",
  children,
}: {
  tone?: TileTone;
  /** sm: liste satırı içi · md: liste satırı başı. */
  size?: "sm" | "md";
  children: React.ReactNode;
}) {
  return (
    <span
      aria-hidden="true"
      className={`flex shrink-0 items-center justify-center rounded-lg ${
        size === "sm" ? "size-7" : "size-9"
      } ${tileTones[tone]}`}
    >
      {children}
    </span>
  );
}

// ─── Düğme ──────────────────────────────────────────────────────────────────

type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
type ButtonSize = "sm" | "md";

const buttonVariants: Record<ButtonVariant, string> = {
  primary:
    "bg-accent text-accent-ink hover:bg-accent-hover border border-transparent",
  // Sınır SÜSLEME token'ı değil `control-line`: düğmenin nerede başlayıp
  // bittiği yalnızca bu çizgiden anlaşılıyor, dolayısıyla 3:1 olmalı.
  // `line-strong` ile ölçüm 1,8:1 (açık) / 1,63:1 (koyu) veriyordu.
  secondary:
    "bg-surface text-ink border border-control-line hover:border-ink-2 hover:bg-raised",
  ghost:
    "bg-transparent text-ink-2 border border-transparent hover:bg-raised hover:text-ink",
  /*
   * Yıkıcı eylem DURGUNKEN sessiz, etkileşimde kırmızı.
   *
   * Önceki hali durgunken de kırmızı metin + kırmızı kenar taşıyordu ve
   * yanındaki "Düzenle" ile aynı boydaydı. Agent listesinde altı satırın
   * her birinde bir kırmızı düğme duruyordu: kırmızı, sayfada en sık
   * görülen renk olunca uyarı olmaktan çıkıyor.
   *
   * Anlam kaybolmuyor, çünkü RENK ZATEN TEK KANAL DEĞİL — düğmenin üstünde
   * "Sil" yazıyor. Projenin durum renkleri için uyguladığı kuralın aynısı.
   *
   * Kenar `control-line`: `secondary` ile aynı jeton, yani aynı ölçülmüş
   * 3:1 denetim sınırı. Bu bilinçli — daha önce kenar `danger/35` iken
   * açık temada 1,76:1, koyu temada 1,85:1 ölçülmüştü ve "Sil" düğmesinin
   * sınırı iki temada da görünmüyordu. Sessizleştirmek o hatayı
   * geri getirmemeli: kırmızıyı hover'a taşıyoruz, sınırı zayıflatmıyoruz.
   */
  danger:
    "bg-transparent text-ink-2 border border-control-line hover:border-danger/70 hover:bg-danger-soft hover:text-danger",
};

const buttonSizes: Record<ButtonSize, string> = {
  sm: "h-7 px-2.5 text-xs gap-1.5",
  md: "h-8 px-3 text-sm gap-1.5",
};

export function Button({
  children,
  variant = "secondary",
  size = "md",
  icon,
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: React.ReactNode;
}) {
  return (
    <button
      {...props}
      className={`inline-flex items-center justify-center rounded-lg font-medium whitespace-nowrap transition-colors duration-150 disabled:pointer-events-none disabled:opacity-60 ${buttonVariants[variant]} ${buttonSizes[size]} ${className}`}
    >
      {icon}
      {children}
    </button>
  );
}

// ─── Rozet ve durum ─────────────────────────────────────────────────────────

type Tone = "neutral" | "info" | "success" | "warning" | "danger" | "accent";

const badgeTones: Record<Tone, string> = {
  neutral: "bg-raised text-ink-2 border-line",
  info: "bg-info-soft text-info border-info/25",
  success: "bg-ok-soft text-ok border-ok/25",
  warning: "bg-warn-soft text-warn border-warn/25",
  danger: "bg-danger-soft text-danger border-danger/25",
  accent: "bg-accent-soft text-accent border-accent/25",
};

export function Badge({
  children,
  tone = "neutral",
  title,
}: {
  children: React.ReactNode;
  tone?: Tone;
  title?: string;
}) {
  return (
    <span
      title={title}
      className={`inline-flex items-center rounded-md border px-1.5 py-px text-2xs font-medium whitespace-nowrap ${badgeTones[tone]}`}
    >
      {children}
    </span>
  );
}

const dotTones: Record<Tone, string> = {
  neutral: "bg-ink-3",
  info: "bg-info",
  success: "bg-ok",
  warning: "bg-warn",
  danger: "bg-danger",
  accent: "bg-accent",
};

/** Durum noktası. pulse, sürmekte olan bir iş için. */
export function StatusDot({
  tone = "neutral",
  pulse = false,
}: {
  tone?: Tone;
  pulse?: boolean;
}) {
  return (
    <span
      className={`inline-block size-1.5 shrink-0 rounded-full ${dotTones[tone]} ${
        pulse ? "animate-pulse-dot" : ""
      }`}
    />
  );
}

// ─── Form ───────────────────────────────────────────────────────────────────

// Girdi kenarı da bir DENETİM sınırıdır: `border-line` ile beyaz kart üzerinde
// 1,31:1 ölçülüyordu — kutunun nerede olduğu görünmüyordu.
const fieldBase =
  "w-full rounded-lg border border-control-line bg-canvas px-2.5 py-1.5 text-sm outline-none transition-colors duration-150 placeholder:text-ink-3 hover:border-ink-2 focus:border-accent disabled:opacity-50";

export function Input({
  className = "",
  ...props
}: React.InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`${fieldBase} ${className}`} />;
}

export function Textarea({
  className = "",
  ...props
}: React.TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea {...props} className={`${fieldBase} ${className}`} />;
}

export function Select({
  className = "",
  children,
  ...props
}: React.SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select {...props} className={`${fieldBase} cursor-pointer ${className}`}>
      {children}
    </select>
  );
}

/** Etiketli form alanı. */
export function Label({
  children,
  hint,
  className = "",
}: {
  children: React.ReactNode;
  hint?: string;
  className?: string;
}) {
  return (
    <label className={`block ${className}`}>
      <span className="mb-1 block text-2xs font-medium tracking-wide text-ink-2 uppercase">
        {children}
        {hint && (
          <span className="ml-1.5 font-normal normal-case opacity-70">{hint}</span>
        )}
      </span>
    </label>
  );
}

export function Checkbox({
  label,
  checked,
  onChange,
  disabled,
}: {
  label: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
}) {
  return (
    <label
      className={`inline-flex items-center gap-2 text-sm ${
        disabled ? "opacity-50" : "cursor-pointer"
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
        className="size-3.5 accent-accent"
      />
      {label}
    </label>
  );
}

/**
 * Açık/kapalı denetimi.
 *
 * `Checkbox` DEĞİL, ve bu bilinçli: onay kutusu kendi etiketini yanına alır ve
 * `size-3.5` ile çizilir. Ayar satırında etiket zaten solda duruyor, denetimler
 * sağda kendi sütununda hizalanıyor — oraya konan bir onay kutusu ikinci bir
 * etiket taşır ve sütunun hizasını kırardı.
 *
 * AÇIK/KAPALI YALNIZCA RENKLE AYRILMAZ: topuz iki durumda farklı yerde durur,
 * yani renk körlüğünde ve siyah-beyaz çıktıda da okunur. Durumu bildiren metin
 * bu bileşenin DIŞINDA, çağıran satırın kendi sütununda durur; burada yalnızca
 * ekran okuyucu için `role="switch"` + `aria-checked` var.
 *
 * Sınır her iki durumda da `control-line`: denetimin nerede olduğu, açık mı
 * kapalı mı olduğundan bağımsız bir bilgi. Dolgu değişince sınırın da
 * değişmesi, kapalı durumda denetimi kaybediyordu.
 */
export function Switch({
  checked,
  onChange,
  disabled,
  label,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  /** Ekran okuyucu için; satırda görünen etiket ayarın adıdır. */
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`inline-flex h-5 w-9 shrink-0 items-center rounded-full border border-control-line transition-colors duration-150 disabled:opacity-50 ${
        checked ? "bg-accent" : "bg-raised"
      } ${disabled ? "" : "cursor-pointer"}`}
    >
      <span
        aria-hidden
        className={`size-3.5 rounded-full transition-transform duration-150 ${
          checked ? "translate-x-4.5 bg-accent-ink" : "translate-x-0.5 bg-ink-2"
        }`}
      />
    </button>
  );
}

// ─── Bilgi kutuları ─────────────────────────────────────────────────────────

const noticeTones: Record<"neutral" | "info" | "warning" | "error", string> = {
  neutral: "border-line bg-raised text-ink-2",
  info: "border-info/25 bg-info-soft text-ink",
  warning: "border-warn/30 bg-warn-soft text-ink",
  error: "border-danger/30 bg-danger-soft text-ink",
};

export function Notice({
  tone = "neutral",
  title,
  children,
}: {
  tone?: "neutral" | "info" | "warning" | "error";
  title?: string;
  children?: React.ReactNode;
}) {
  return (
    <div
      className={`rounded-lg border px-3.5 py-2.5 text-sm leading-relaxed ${noticeTones[tone]}`}
    >
      {title && <p className="font-medium">{title}</p>}
      {children && <div className={title ? "mt-0.5" : ""}>{children}</div>}
    </div>
  );
}

/**
 * Satır eylemi: durgunken görünmez, hover ve odakta belirir.
 *
 * Bir `.group` içinde kullanılır. Her satırda duran bir silme düğmesi,
 * listenin asıl işinin (açmak) önüne geçiyordu; odakla da beliriyor çünkü
 * klavyeyle gezen kullanıcı için görünmezlik erişilemezlik olmamalı. Dar
 * ekranda hover yok, orada hep görünür.
 *
 * SAYDAMLIK DÜĞMEDE DEĞİL SARMALAYICIDA — bu bir hatanın kaydı: `Button`
 * pasifken kendi `disabled:opacity-60`ını uyguluyor ve düğmeye verilen
 * `sm:opacity-0`ı eziyor. Sınıf doğrudan düğmeye konduğunda sonuç tersine
 * dönmüştü: tabloda yalnızca SİLİNEMEYEN satırlarda düğme duruyor,
 * silinebilenlerde görünmüyordu. Sürekli görünen tek eylem, işe yaramayan
 * eylemdi.
 */
export function RowAction({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <span
      className={`inline-flex shrink-0 opacity-100 transition-opacity duration-150 sm:opacity-0 sm:group-focus-within:opacity-100 sm:group-hover:opacity-100 ${className}`}
    >
      {children}
    </span>
  );
}

/**
 * Geri alınamayan bir eylemin yerinde onayı.
 *
 * Modal DEĞİL, şerit: onay silinecek şeyin yanında açılır, kullanıcı neyi
 * onayladığını görmeye devam eder. Projede `window.confirm` ve Dialog
 * bileşeni bilinçli olarak yok.
 *
 * `consequence` en önemli alan: "emin misiniz?" hiçbir şey söylemez.
 * Ne kaybedileceği SAYIYLA yazılır — "12 çalışma geçmişi de silinecek",
 * "$0,004 maliyet raporlardan düşecek". Ölçülemiyorsa yazılmaz (boş bırakılır),
 * uydurulmaz.
 */
export function ConfirmStrip({
  question,
  consequence,
  confirmLabel = "Evet, sil",
  busy = false,
  error,
  onConfirm,
  onCancel,
  className = "",
}: {
  question: string;
  consequence?: React.ReactNode;
  confirmLabel?: string;
  busy?: boolean;
  error?: string;
  onConfirm: () => void;
  onCancel: () => void;
  className?: string;
}) {
  return (
    <div className={`border-danger/30 bg-danger-soft px-4 py-3 ${className}`}>
      <p className="flex items-start gap-2 text-xs text-ink">
        <IconAlert className="mt-px size-4 shrink-0 text-danger" />
        <span>
          {question}
          {consequence && <> {consequence}</>}
        </span>
      </p>
      <div className="mt-2.5 flex gap-2">
        <Button size="sm" variant="danger" onClick={onConfirm} disabled={busy}>
          {busy ? "Siliniyor…" : confirmLabel}
        </Button>
        <Button size="sm" onClick={onCancel} disabled={busy}>
          Vazgeç
        </Button>
      </div>
      {error && <p className="mt-2 text-xs text-danger">{error}</p>}
    </div>
  );
}

/** Liste boşken gösterilen, ne yapılacağını söyleyen blok. */
export function EmptyState({
  icon,
  title,
  description,
  action,
}: {
  icon?: React.ReactNode;
  title: string;
  description?: React.ReactNode;
  action?: React.ReactNode;
}) {
  return (
    <div className="rounded-card border border-dashed border-line-strong bg-surface/40 px-6 py-12 text-center">
      {icon && (
        <div className="mx-auto mb-3 flex size-10 items-center justify-center rounded-full bg-raised text-ink-3">
          {icon}
        </div>
      )}
      <p className="text-base font-medium">{title}</p>
      {description && (
        <p className="mx-auto mt-1.5 max-w-md text-sm leading-relaxed text-ink-2">
          {description}
        </p>
      )}
      {action && <div className="mt-4 flex justify-center">{action}</div>}
    </div>
  );
}

/** Yüklenirken gösterilen iskelet satırlar. */
export function Skeleton({ rows = 3 }: { rows?: number }) {
  return (
    <div className="space-y-2.5">
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          className="h-18 animate-pulse rounded-card border border-line bg-surface"
        />
      ))}
    </div>
  );
}

// ─── Veri gösterimi ─────────────────────────────────────────────────────────

/**
 * Ölçü — etiket, rakam ve isteğe bağlı alt not.
 *
 * ÜÇ KOPYASI VARDI: Agent'lar, Projeler ve Rapor ekranlarının her biri aynı
 * kalıbı kendi içinde yeniden yazmıştı (sonuncusu `MiniStat` adıyla). Üçü
 * de neredeyse aynıydı ama tam değil — biri `mt-1`, diğeri `mt-0.5`
 * kullanıyordu ve yan yana konsalar hizasızlıkları görünürdü. Kalıbın
 * ekranın içine yazılması, ikinci ve üçüncü kopyayı davet ediyor.
 *
 * `tone` YALNIZCA yorumlanabilir bir değer için: başarı oranı iyi ya da
 * kötü olabilir, token sayısı olamaz. Her rakamı renklendirmek renge sahte
 * bir anlam yükler.
 */
export function Metric({
  label,
  value,
  note,
  tone,
  /** md: kart içindeki kahraman rakam · sm: ikincil ölçü. */
  size = "md",
}: {
  label: React.ReactNode;
  value: React.ReactNode;
  note?: React.ReactNode;
  tone?: "ok" | "warn";
  size?: "sm" | "md";
}) {
  return (
    <div className="min-w-0">
      <dt className="truncate text-2xs font-medium tracking-wide text-ink-3 uppercase">
        {label}
      </dt>
      <dd
        className={`truncate font-semibold tabular-nums ${
          size === "sm" ? "mt-0.5 text-sm" : "mt-1 text-lg leading-none"
        } ${tone === "ok" ? "text-ok" : tone === "warn" ? "text-warn" : ""}`}
      >
        {value}
      </dd>
      {note && <p className="mt-1 truncate text-2xs text-ink-3">{note}</p>}
    </div>
  );
}

/** Etiket + değer çifti. */
export function Field({
  label,
  value,
  mono = false,
  className = "",
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
  className?: string;
}) {
  return (
    <div className={`min-w-0 ${className}`}>
      <dt className="text-2xs tracking-wide text-ink-3 uppercase">{label}</dt>
      <dd className={`mt-0.5 truncate text-sm ${mono ? "font-mono text-xs" : ""}`}>
        {value}
      </dd>
    </div>
  );
}

/** Tek satırlık kod/kimlik gösterimi. */
export function Mono({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <code
      className={`rounded bg-raised px-1.5 py-0.5 font-mono text-xs text-ink-2 ${className}`}
    >
      {children}
    </code>
  );
}

// ─── Yardımcılar ────────────────────────────────────────────────────────────

/** Tarihi okunabilir yerel biçime çevirir. */
export function formatDate(iso: string | null): string {
  if (!iso) return "hiç";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "bilinmiyor";
  return d.toLocaleString("tr-TR", { dateStyle: "medium", timeStyle: "short" });
}

/** "3 dakika önce" biçiminde göreli zaman. */
export function formatRelative(iso: string | null): string {
  if (!iso) return "hiç";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "bilinmiyor";

  const seconds = Math.round((Date.now() - d.getTime()) / 1000);
  if (seconds < 60) return "az önce";

  const units: [number, Intl.RelativeTimeFormatUnit][] = [
    [60, "minute"],
    [3600, "hour"],
    [86400, "day"],
  ];

  const rtf = new Intl.RelativeTimeFormat("tr", { numeric: "auto" });
  for (let i = units.length - 1; i >= 0; i--) {
    const [size, unit] = units[i]!;
    if (seconds >= size) return rtf.format(-Math.floor(seconds / size), unit);
  }
  return rtf.format(-Math.floor(seconds / 60), "minute");
}
