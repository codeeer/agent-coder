/**
 * İkonlar — tek kaynak: lucide-react.
 *
 * Önce hepsi burada elle çizilmiş satır içi SVG'ydi. Otuz ikona ulaşınca
 * bunun iki bedeli ortaya çıktı: her yeni ikon için 24×24 çizim alanında
 * elle yol yazmak gerekiyordu ve çizimler birbirine benzemiyordu — kimi
 * 1,75 kimi 2,2 kalınlıkta, kimi köşeli kimi yuvarlak uçlu.
 *
 * Bu dosya artık bir EŞLEM KATMANI: dışa verilen adlar (`IconAgent`,
 * `IconPlay` …) aynı kaldı, arkalarındaki çizim lucide'dan geliyor.
 * Adları korumanın sebebi, otuz ikonun çağrıldığı elli küsur yeri
 * değiştirmemek — ve "hangi lucide ikonu neyi temsil ediyor" kararının
 * tek bir yerde durması. `IconAgent`'ın `Bot` olduğu buradan okunur;
 * her dosyaya dağılmış `<Bot />` çağrılarından okunmaz.
 *
 * İKİ KURAL, İSTİSNASIZ:
 *   - Boyut 16px (`size-4`). Öncesinde 14, 15, 16 ve 20px karışıktı.
 *   - Kalınlık 2 (lucide varsayılanı).
 *
 * Renk gelmiyor: `currentColor` ile metin renginden miras alınır, yani
 * ikon kendi başına bir renk taşımaz.
 *
 * Diyagramlar ve grafikler BU KURALIN DIŞINDA — `components/docs/diagrams.tsx`
 * ve `components/charts/*` elle yazılmış SVG olarak kalır. Onlar ikon değil;
 * veri ve mimari çizimi, ikon kitaplığında karşılıkları yok.
 */

import {
  BookOpen,
  Bot,
  ChartColumn,
  Check,
  CirclePlay,
  CodeXml,
  Cpu,
  ExternalLink,
  Eye,
  EyeOff,
  Folder,
  GitPullRequest,
  LayoutDashboard,
  Menu,
  MessageSquare,
  Monitor,
  Moon,
  Pencil,
  Plug,
  Plus,
  RefreshCw,
  Search,
  Settings,
  Star,
  Sun,
  Terminal,
  Trash2,
  TriangleAlert,
  Undo2,
  Workflow,
  X,
  type LucideIcon,
} from "lucide-react";

type IconProps = {
  className?: string;
  strokeWidth?: number;
};

/** Arayüzün her yerinde tek ikon boyutu. */
const ICON_SIZE = "size-4";

/** Arayüzün her yerinde tek çizgi kalınlığı (lucide varsayılanı). */
const ICON_STROKE = 2;

/**
 * lucide ikonunu projenin ikon sözleşmesine sarar.
 *
 * `aria-hidden` burada veriliyor, çağrı yerinde değil: bu ikonların hepsi
 * dekoratif ya da yanındaki metnin tekrarı. Anlamı yalnızca ikonla taşınan
 * bir düğme varsa metni `sr-only` ile ayrıca yazılır (bkz. ThemeToggle).
 */
function icon(Glyph: LucideIcon) {
  return function Icon({ className = ICON_SIZE, strokeWidth = ICON_STROKE }: IconProps) {
    return <Glyph className={className} strokeWidth={strokeWidth} aria-hidden="true" />;
  };
}

// ─── Gezinme ────────────────────────────────────────────────────────────────

export const IconDashboard = icon(LayoutDashboard);
export const IconWorkflow = icon(Workflow);
export const IconFolder = icon(Folder);
export const IconAgent = icon(Bot);
export const IconChip = icon(Cpu);
export const IconReport = icon(ChartColumn);
export const IconSettings = icon(Settings);
export const IconBook = icon(BookOpen);

// ─── Eylem ──────────────────────────────────────────────────────────────────

export const IconPlus = icon(Plus);
export const IconTrash = icon(Trash2);
export const IconEdit = icon(Pencil);
export const IconRefresh = icon(RefreshCw);
export const IconUndo = icon(Undo2);
export const IconSearch = icon(Search);
export const IconExternal = icon(ExternalLink);
export const IconPlay = icon(CirclePlay);

// ─── Durum ──────────────────────────────────────────────────────────────────

export const IconCheck = icon(Check);
export const IconAlert = icon(TriangleAlert);
export const IconStar = icon(Star);
export const IconEye = icon(Eye);
export const IconEyeOff = icon(EyeOff);

// ─── Tema ───────────────────────────────────────────────────────────────────

export const IconMonitor = icon(Monitor);
export const IconSun = icon(Sun);
export const IconMoon = icon(Moon);

// ─── Alan adı ───────────────────────────────────────────────────────────────

export const IconPullRequest = icon(GitPullRequest);
export const IconComment = icon(MessageSquare);
export const IconPlug = icon(Plug);
export const IconTerminal = icon(Terminal);

// ─── Kabuk ──────────────────────────────────────────────────────────────────

export const IconMenu = icon(Menu);
export const IconClose = icon(X);

/**
 * Kelime markasının simgesi.
 *
 * Kenar çubuğunda elle yazılmış bir `</>` SVG'siydi. Aynı glif lucide'da
 * `CodeXml` olarak zaten var; özel çizilmiş bir marka işareti değildi,
 * dolayısıyla elde tutmanın bir gerekçesi yoktu.
 */
export const IconLogo = icon(CodeXml);
