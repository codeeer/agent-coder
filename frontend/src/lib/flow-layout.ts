import type { FlowEdge, FlowNode } from "@/lib/workflow-graph";

/**
 * Tuval yerleşimi ve döngü tespiti.
 *
 * React'tan bağımsız, saf fonksiyonlar — tuval açılmadan test edilebilsin diye.
 * (Spec 005, Ölçüm 1'in dersi: bileşene gömülen mantığın hatası ancak tarayıcıda
 * görülür.)
 */

/** Sütun aralığı — düğüm genişliği + rahat nefes payı. */
const COLUMN = 280;
/** Satır aralığı. */
const ROW = 130;
const ORIGIN = { x: 40, y: 40 };

/**
 * Düğümleri seviyelere böler.
 *
 * Motorun `Levels()` hesabıyla AYNI mantık: aynı seviyedeki düğümler birbirine
 * bağlı değildir ve aynı anda çalışırlar. Yerleşimin sütunları bu yüzden
 * seviyeden geliyor — tuval, akışın gerçekten nasıl çalıştığını göstermeli.
 *
 * Döngü varsa kalan düğümler son seviyeye konur; yerleşim çökmemeli.
 */
export function levelsOf(nodes: FlowNode[], edges: FlowEdge[]): string[][] {
  const indegree = new Map(nodes.map((n) => [n.id, 0]));
  const next = new Map<string, string[]>();

  for (const e of edges) {
    if (!indegree.has(e.source) || !indegree.has(e.target)) continue;
    next.set(e.source, [...(next.get(e.source) ?? []), e.target]);
    indegree.set(e.target, (indegree.get(e.target) ?? 0) + 1);
  }

  const levels: string[][] = [];
  const placed = new Set<string>();

  while (placed.size < nodes.length) {
    const level = nodes
      .filter((n) => !placed.has(n.id) && (indegree.get(n.id) ?? 0) === 0)
      .map((n) => n.id);

    if (level.length === 0) {
      // Döngü: kalanları tek seviyeye koy ve çık.
      levels.push(nodes.filter((n) => !placed.has(n.id)).map((n) => n.id));
      break;
    }

    for (const id of level) placed.add(id);
    // Kenarlar seviye BİTTİKTEN sonra düşülür; aksi halde aynı seviyedeki bir
    // düğüm diğerini serbest bırakıp yanlış sütuna taşırdı.
    for (const id of level) {
      for (const to of next.get(id) ?? []) {
        indegree.set(to, (indegree.get(to) ?? 1) - 1);
      }
    }
    levels.push(level);
  }
  return levels;
}

/**
 * Konumu olmayan düğüm var mı?
 *
 * "Hepsi 0,0" da konumsuz sayılır: adım listesi editörüyle kurulmuş eski
 * akışlar öyle kaydedilmişti ve tuvalde üst üste yığılırlardı.
 */
export function needsLayout(nodes: FlowNode[]): boolean {
  if (nodes.length === 0) return false;
  return nodes.every((n) => n.position.x === 0 && n.position.y === 0);
}

/**
 * Konumu olmayan düğümleri yerleştirir.
 *
 * Konumu OLAN düğüme dokunmaz: kullanıcının elle taşıdığı bir düğümün her
 * açılışta yerine dönmesi, yaptığı işi geri almak olurdu.
 */
export function autoLayout(nodes: FlowNode[], edges: FlowEdge[]): FlowNode[] {
  if (nodes.length === 0) return nodes;

  const levels = levelsOf(nodes, edges);
  const position = new Map<string, { x: number; y: number }>();

  levels.forEach((level, column) => {
    level.forEach((id, row) => {
      position.set(id, {
        x: ORIGIN.x + column * COLUMN,
        // Seviye içindeki düğümler dikeyde ortalanır: tek düğümlü seviyeler
        // dallanmanın ortasında kalsın, kenara yapışmasın.
        y: ORIGIN.y + (row - (level.length - 1) / 2) * ROW + verticalCenter(levels),
      });
    });
  });

  const layoutAll = needsLayout(nodes);

  return nodes.map((n) => {
    const placed = position.get(n.id);
    if (!placed) return n;
    const hasOwn = !layoutAll && (n.position.x !== 0 || n.position.y !== 0);
    return hasOwn ? n : { ...n, position: placed };
  });
}

/** En kalabalık seviyenin yarısı kadar aşağı kaydırır ki hiçbir satır eksiye düşmesin. */
function verticalCenter(levels: string[][]): number {
  const widest = Math.max(...levels.map((l) => l.length), 1);
  return ((widest - 1) / 2) * ROW;
}

/**
 * Yeni bir bağ döngü yaratır mı?
 *
 * Çizim ANINDA sorulur: kullanıcı bağı çekip kaydetmeye kadar beklerse, hatayı
 * öğrendiğinde yaptığı işi geri almak zorunda kalır.
 */
export function wouldCreateCycle(
  edges: FlowEdge[],
  source: string,
  target: string,
): boolean {
  if (source === target) return true;

  // Hedeften başlayıp kaynağa ulaşabiliyorsak, yeni bağ çemberi kapatır.
  const next = new Map<string, string[]>();
  for (const e of edges) {
    next.set(e.source, [...(next.get(e.source) ?? []), e.target]);
  }

  const seen = new Set<string>();
  const queue = [target];
  while (queue.length > 0) {
    const cur = queue.shift()!;
    if (cur === source) return true;
    if (seen.has(cur)) continue;
    seen.add(cur);
    queue.push(...(next.get(cur) ?? []));
  }
  return false;
}

/** Yeni bir düğümün konumu: en sağdaki düğümün bir sütun sağı. */
export function nextPosition(nodes: FlowNode[]): { x: number; y: number } {
  if (nodes.length === 0) return { ...ORIGIN };

  const rightmost = nodes.reduce((a, b) => (a.position.x >= b.position.x ? a : b));
  return { x: rightmost.position.x + COLUMN, y: rightmost.position.y };
}
