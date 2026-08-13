/**
 * "Nasıl çalışır" sayfasının diyagramları.
 *
 * Elle yazılmış SVG — grafik kütüphanesi yok, rapor grafikleriyle aynı yaklaşım.
 * Renkler token'dan (`var(--color-*)`) okunur; böylece diyagramlar iki temada da
 * kendiliğinden doğru görünür ve tema denetiminden geçer.
 *
 * Her diyagram `viewBox` ile ölçeklenir ve `width: 100%` ile kabına oturur:
 * sunumda tam ekran yansıtıldığında da telefonda da okunur kalması gerekiyor.
 *
 * Metin diyagramın İÇİNDE: sunumu yapan kişi ekranı gösterip anlatacak, bir
 * yandan da yandaki paragrafı okumak zorunda kalmamalı.
 */

/* ── Ortak parçalar ──────────────────────────────────────────────────────── */

/** Kutu — akıştaki bir adım ya da mimarideki bir servis. */
function Box({
  x,
  y,
  w,
  h,
  title,
  subtitle,
  tone = "surface",
}: {
  x: number;
  y: number;
  w: number;
  h: number;
  title: string;
  subtitle?: string;
  tone?: "surface" | "accent" | "muted";
}) {
  const fill =
    tone === "accent"
      ? "var(--color-accent-soft)"
      : tone === "muted"
        ? "var(--color-raised)"
        : "var(--color-surface)";
  const stroke =
    tone === "accent" ? "var(--color-accent)" : "var(--color-line-strong)";
  const titleFill =
    tone === "accent" ? "var(--color-accent)" : "var(--color-ink)";

  return (
    <g>
      <rect
        x={x}
        y={y}
        width={w}
        height={h}
        rx={10}
        fill={fill}
        stroke={stroke}
        strokeWidth={1.5}
      />
      <text
        x={x + w / 2}
        y={subtitle ? y + h / 2 - 5 : y + h / 2 + 4}
        textAnchor="middle"
        fontSize={13}
        fontWeight={500}
        fill={titleFill}
      >
        {title}
      </text>
      {subtitle && (
        <text
          x={x + w / 2}
          y={y + h / 2 + 13}
          textAnchor="middle"
          fontSize={11}
          fill="var(--color-ink-3)"
        >
          {subtitle}
        </text>
      )}
    </g>
  );
}

/** Ok — iki kutu arasındaki yön. */
function Arrow({
  x1,
  y1,
  x2,
  y2,
  label,
  dashed = false,
}: {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  label?: string;
  dashed?: boolean;
}) {
  return (
    <g>
      <line
        x1={x1}
        y1={y1}
        x2={x2}
        y2={y2}
        stroke="var(--color-line-strong)"
        strokeWidth={1.5}
        strokeDasharray={dashed ? "4 4" : undefined}
        markerEnd="url(#ac-ok)"
      />
      {label && (
        <text
          x={(x1 + x2) / 2}
          y={y1 === y2 ? y1 - 7 : (y1 + y2) / 2 - 5}
          textAnchor="middle"
          fontSize={10.5}
          fill="var(--color-ink-3)"
        >
          {label}
        </text>
      )}
    </g>
  );
}

/** Ok ucu tanımı — her SVG'de bir kez. */
function Defs() {
  return (
    <defs>
      <marker
        id="ac-ok"
        viewBox="0 0 10 10"
        refX="9"
        refY="5"
        markerWidth="6"
        markerHeight="6"
        orient="auto"
      >
        <path d="M0 0 L10 5 L0 10 z" fill="var(--color-line-strong)" />
      </marker>
    </defs>
  );
}

/** Diyagram kabı — başlık ve ölçeklenen çizim alanı. */
function Frame({
  viewBox,
  children,
  label,
}: {
  viewBox: string;
  children: React.ReactNode;
  label: string;
}) {
  /*
   * `minWidth`: dar ekranda diyagram KÜÇÜLMEZ, kabı kaydırılır.
   *
   * Yalnızca `w-full` verilseydi 1020 birimlik çizim 390px'e sığdırılır ve
   * 11px'lik etiketler ~4px'e inerdi — diyagram görünür ama okunmaz olurdu.
   * Kart `overflow-x-auto` taşıdığı için yatay kaydırma sayfayı bozmuyor.
   */
  return (
    <div>
      {/* Kaydırma YALNIZCA çizimi sarar. Kart taşısaydı diyagramın altındaki
          açıklama metni de yatay kayardı — okunacak bir paragraf için yanlış. */}
      <div className="overflow-x-auto">
        <svg
          viewBox={viewBox}
          className="w-full"
          style={{ minWidth: 760 }}
          role="img"
          aria-label={label}
        >
          <Defs />
          {children}
        </svg>
      </div>
      <p className="mt-1.5 text-2xs text-ink-3 sm:hidden">
        Diyagramın tamamı için yana kaydırın →
      </p>
    </div>
  );
}

/* ── 1. Ürün akışı ───────────────────────────────────────────────────────── */

/** Jira task'ından PR'a giden zincir — sayfanın açılış görseli. */
export function ProductFlow() {
  const boxes = [
    { title: "Jira task'ı", sub: "tetikleyici", tone: "accent" as const },
    { title: "Analiz", sub: "ucuz model" },
    { title: "Kod yazma", sub: "güçlü model" },
    { title: "PR aç", sub: "GitHub" },
    { title: "Jira yorumu", sub: "sonuç linki" },
  ];
  const W = 172;
  const GAP = 32;

  return (
    <Frame viewBox="0 0 1020 120" label="Jira task'ından PR'a giden akış">
      {boxes.map((b, i) => {
        const x = 10 + i * (W + GAP);
        return (
          <g key={b.title}>
            <Box
              x={x}
              y={25}
              w={W}
              h={64}
              title={b.title}
              subtitle={b.sub}
              tone={b.tone}
            />
            {i < boxes.length - 1 && (
              <Arrow x1={x + W + 6} y1={57} x2={x + W + GAP - 6} y2={57} />
            )}
          </g>
        );
      })}
      <text
        x={510}
        y={110}
        textAnchor="middle"
        fontSize={11}
        fill="var(--color-ink-3)"
      >
        Her kutu tuvalde sürüklenir, birbirine bağlanır ve kendi modelini seçer.
      </text>
    </Frame>
  );
}

/* ── 2. Mimari ───────────────────────────────────────────────────────────── */

/** Servisler, sınırlar ve dış dünya. */
export function Architecture() {
  return (
    <Frame viewBox="0 0 1020 470" label="Sistem mimarisi">
      {/* Kullanıcı tarafı */}
      <Box
        x={20}
        y={30}
        w={150}
        h={54}
        title="Tarayıcı"
        subtitle="tuval + izleme"
      />
      <Box x={20} y={120} w={150} h={54} title="Frontend" subtitle="Next.js" />
      <Arrow x1={95} y1={86} x2={95} y2={116} />

      {/* Sunucu */}
      <rect
        x={220}
        y={20}
        width={300}
        height={240}
        rx={12}
        fill="none"
        stroke="var(--color-line)"
        strokeDasharray="5 5"
      />
      <text
        x={370}
        y={42}
        textAnchor="middle"
        fontSize={11}
        fill="var(--color-ink-3)"
      >
        BACKEND (Go) — tek servis, ek altyapı yok
      </text>
      <Box
        x={240}
        y={56}
        w={260}
        h={44}
        title="HTTP API + canlı olay akışı"
        tone="muted"
      />
      <Box
        x={240}
        y={112}
        w={260}
        h={44}
        title="Akış motoru (DAG)"
        subtitle="seviye seviye, paralel"
        tone="accent"
      />
      <Box
        x={240}
        y={168}
        w={260}
        h={44}
        title="Jira tarayıcı"
        subtitle="arka planda, aralıklı"
        tone="muted"
      />

      <Arrow x1={172} y1={147} x2={236} y2={134} label="HTTP / SSE" />

      {/* Veritabanı */}
      <Box
        x={240}
        y={290}
        w={260}
        h={54}
        title="PostgreSQL"
        subtitle="akışlar · çalışmalar · şifreli anahtarlar"
      />
      <Arrow x1={370} y1={264} x2={370} y2={286} />

      {/* Sandbox */}
      <rect
        x={580}
        y={20}
        width={420}
        height={200}
        rx={12}
        fill="none"
        stroke="var(--color-line)"
        strokeDasharray="5 5"
      />
      <text
        x={790}
        y={42}
        textAnchor="middle"
        fontSize={11}
        fill="var(--color-ink-3)"
      >
        İZOLE AĞ — dışarıya port açmaz
      </text>
      <Box
        x={600}
        y={56}
        w={180}
        h={60}
        title="Runner #1"
        subtitle="opencode + repo klonu"
      />
      <Box
        x={800}
        y={56}
        w={180}
        h={60}
        title="Runner #2"
        subtitle="aynı anda çalışır"
      />
      <Box
        x={600}
        y={136}
        w={380}
        h={54}
        title="İş bitince container ve volume silinir"
        tone="muted"
      />

      <Arrow x1={524} y1={110} x2={596} y2={86} label="Docker soketi" />

      {/* Dış servisler */}
      <text
        x={790}
        y={276}
        textAnchor="middle"
        fontSize={11}
        fill="var(--color-ink-3)"
      >
        DIŞ SERVİSLER
      </text>
      <Box x={556} y={290} w={110} h={54} title="LLM" subtitle="OpenRouter…" />
      <Box x={676} y={290} w={110} h={54} title="Git" subtitle="push + PR" />
      <Box
        x={796}
        y={290}
        w={110}
        h={54}
        title="Jira"
        subtitle="task + yorum"
      />
      <Box
        x={916}
        y={290}
        w={84}
        h={54}
        title="MCP"
        subtitle="dış araçlar"
        tone="accent"
      />

      <Arrow x1={680} y1={196} x2={624} y2={284} />
      <text x={578} y={250} fontSize={10.5} fill="var(--color-ink-3)">
        model çağrısı
      </text>
      {/* MCP oku RUNNER'dan çıkar: aracı agent çağırıyor, backend değil. */}
      <Arrow x1={900} y1={196} x2={950} y2={284} />
      <text x={905} y={250} fontSize={10.5} fill="var(--color-ink-3)">
        araç çağrısı
      </text>
      <Arrow x1={520} y1={200} x2={720} y2={286} dashed />
      <Arrow x1={520} y1={215} x2={870} y2={286} dashed />

      <text
        x={510}
        y={385}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Kesikli oklar backend&apos;den çıkar: PR açmak ve Jira&apos;ya yazmak
        agent&apos;ın değil, akışın işidir.
      </text>
      <text
        x={510}
        y={406}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Agent kendi container&apos;ında kodla ve kendisine açılan dış araçlarla
        uğraşır.
      </text>
    </Frame>
  );
}

/* ── 3. Paralellik ───────────────────────────────────────────────────────── */

/** Seviye kavramı: aynı sütundaki adımlar aynı anda koşar. */
export function Parallelism() {
  return (
    <Frame viewBox="0 0 1020 260" label="Paralel çalıştırma">
      {[0, 1, 2].map((i) => (
        <g key={i}>
          <line
            x1={140 + i * 290}
            y1={30}
            x2={140 + i * 290}
            y2={215}
            stroke="var(--color-line)"
            strokeDasharray="4 4"
          />
          <text
            x={140 + i * 290}
            y={22}
            textAnchor="middle"
            fontSize={11}
            fill="var(--color-ink-3)"
          >
            {i + 1}. seviye
          </text>
        </g>
      ))}

      <Box x={30} y={95} w={150} h={54} title="Başlangıç" tone="accent" />

      <Box x={250} y={45} w={180} h={54} title="Analiz" subtitle="12 sn" />
      <Box x={250} y={145} w={180} h={54} title="Test yaz" subtitle="18 sn" />

      <Box
        x={540}
        y={95}
        w={180}
        h={54}
        title="Kod yazma"
        subtitle="ikisini de bekler"
      />

      <Box x={830} y={95} w={160} h={54} title="PR aç" />

      <Arrow x1={186} y1={110} x2={244} y2={80} />
      <Arrow x1={186} y1={134} x2={244} y2={168} />
      <Arrow x1={436} y1={80} x2={534} y2={112} />
      <Arrow x1={436} y1={168} x2={534} y2={134} />
      <Arrow x1={726} y1={122} x2={824} y2={122} />

      <text
        x={510}
        y={238}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Aynı sütundaki adımlar aynı anda başlar. Sonraki seviye, öncekinin
        TAMAMI bitmeden başlamaz.
      </text>
    </Frame>
  );
}

/* ── 4. Bir adımın ömrü ──────────────────────────────────────────────────── */

/** Tek bir agent adımının container yaşam döngüsü. */
export function StepLifecycle() {
  // Alt yazılar KISA: yedi sütun 143 birim aralıkla diziliyor ve uzun bir alt
  // yazı komşusuna değiyor ("talimatistenirse" diye okunuyordu).
  const steps = [
    ["1", "Container açılır", "geçici, izole ağ"],
    ["2", "Repo klonlanır", "gereken derinlikte"],
    ["3", "Agent çalışır", "seçili model ve talimat"],
    ["4", "Diff alınır", "istenirse branch'e"],
    ["5", "Kayıt yazılır", "çıktı, token, maliyet"],
    // 6. adım SİLMEDEN ÖNCE: teşhis verisi container'ın içinde duruyor ve
    // sıra ters olsaydı hiç var olmamış gibi kaybolurdu.
    ["6", "Motor logları alınır", "maskelenip saklanır"],
    ["7", "Container silinir", "volume ile birlikte"],
  ];

  return (
    <Frame viewBox="0 0 1020 190" label="Bir agent adımının ömrü">
      <line
        x1={80}
        y1={70}
        x2={940}
        y2={70}
        stroke="var(--color-line)"
        strokeWidth={2}
      />
      {steps.map(([n, title, sub], i) => {
        // Ortalanmış sütunlar: yazı sola dayalıyken son adım çerçeveden taşıyordu.
        const x = 80 + i * 143;
        return (
          <g key={n}>
            <circle cx={x} cy={70} r={14} fill="var(--color-accent)" />
            <text
              x={x}
              y={74.5}
              textAnchor="middle"
              fontSize={12}
              fontWeight={600}
              fill="var(--color-accent-ink)"
            >
              {n}
            </text>
            <text
              x={x}
              y={110}
              textAnchor="middle"
              fontSize={12.5}
              fontWeight={500}
              fill="var(--color-ink)"
            >
              {title}
            </text>
            <text
              x={x}
              y={128}
              textAnchor="middle"
              fontSize={10.5}
              fill="var(--color-ink-3)"
            >
              {sub}
            </text>
          </g>
        );
      })}
      <text
        x={510}
        y={32}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Her adım kendi container&apos;ında; biri diğerinin dosyalarını göremez.
      </text>
      <text
        x={510}
        y={168}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Zaman aşımı, iptal veya sunucu yeniden başlaması — hangisi olursa olsun
        6. ve 7. adım çalışır.
      </text>
    </Frame>
  );
}

/* ── 5. Tetikleme ve tekrar koruması ─────────────────────────────────────── */

/** İki tetikleme yolu, tek koruma. */
export function TriggerPaths() {
  return (
    <Frame viewBox="0 0 1020 250" label="Tetikleme yolları ve tekrar koruması">
      <Box
        x={20}
        y={40}
        w={190}
        h={56}
        title="JQL taraması"
        subtitle="aralıklı, yedek yol"
      />
      <Box
        x={20}
        y={130}
        w={190}
        h={56}
        title="Jira webhook"
        subtitle="anında"
      />

      <Arrow x1={216} y1={68} x2={330} y2={100} />
      <Arrow x1={216} y1={158} x2={330} y2={126} />

      <Box
        x={336}
        y={85}
        w={230}
        h={56}
        title="Tek başlatma yolu"
        subtitle="task API'den yeniden okunur"
        tone="accent"
      />

      <Arrow x1={572} y1={113} x2={640} y2={113} />

      <Box
        x={646}
        y={78}
        w={200}
        h={70}
        title="Veritabanı kısıtı"
        subtitle="(akış, task, güncellenme)"
      />

      <Arrow x1={852} y1={98} x2={930} y2={70} label="ilk" />
      <Arrow x1={852} y1={128} x2={930} y2={165} label="tekrar" />

      <Box x={862} y={30} w={140} h={48} title="Akış başlar" />
      <Box x={862} y={148} w={140} h={48} title="Yok sayılır" tone="muted" />

      <text
        x={510}
        y={225}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Koruma uygulamada değil veritabanında: iki yol aynı anda gelse bile
        yalnızca biri kazanır.
      </text>
    </Frame>
  );
}

/* ── 6. Veri modeli ──────────────────────────────────────────────────────── */

/** Ana tablolar ve ilişkileri. */
export function DataModel() {
  const t = (
    x: number,
    y: number,
    name: string,
    cols: string[],
    tone: "surface" | "accent" = "surface",
  ) => (
    <g key={name}>
      <rect
        x={x}
        y={y}
        width={200}
        height={28 + cols.length * 17}
        rx={8}
        fill={
          tone === "accent"
            ? "var(--color-accent-soft)"
            : "var(--color-surface)"
        }
        stroke={
          tone === "accent" ? "var(--color-accent)" : "var(--color-line-strong)"
        }
        strokeWidth={1.5}
      />
      <text
        x={x + 12}
        y={y + 19}
        fontSize={12}
        fontWeight={600}
        fill={tone === "accent" ? "var(--color-accent)" : "var(--color-ink)"}
      >
        {name}
      </text>
      {cols.map((c, i) => (
        <text
          key={c}
          x={x + 12}
          y={y + 36 + i * 17}
          fontSize={10.5}
          fill="var(--color-ink-3)"
        >
          {c}
        </text>
      ))}
    </g>
  );

  return (
    <Frame viewBox="0 0 1020 330" label="Veri modeli">
      {t(20, 30, "projects", ["depo adresi", "varsayılan branch"])}
      {t(20, 160, "agents", ["talimat", "yetkiler", "varsayılan model"])}
      {t(
        280,
        30,
        "workflows",
        ["ad", "etkin mi", "tetikleme adresi"],
        "accent",
      )}
      {t(280, 150, "workflow_versions", ["graf (düğüm + bağ)", "sürüm no"])}
      {t(560, 30, "workflow_runs", ["durum", "tetikleyici", "girdi"], "accent")}
      {t(560, 150, "workflow_steps", ["düğüm", "durum", "sonuç"])}
      {t(820, 150, "runs", ["çıktı · diff", "token · maliyet"])}

      <Arrow x1={222} y1={60} x2={276} y2={60} />
      <Arrow x1={380} y1={110} x2={380} y2={146} />
      <Arrow x1={484} y1={70} x2={556} y2={70} />
      <Arrow x1={660} y1={110} x2={660} y2={146} />
      <Arrow x1={762} y1={200} x2={816} y2={200} />
      {/* agents → runs ilişkisi kutuların İÇİNDEN geçen bir okla çizilmişti;
          okunmuyordu. Alttan dolaşan bir yol daha uzun ama izlenebilir. */}
      <path
        d="M120 232 L120 265 L920 265 L920 224"
        fill="none"
        stroke="var(--color-line-strong)"
        strokeWidth={1.5}
        strokeDasharray="4 4"
        markerEnd="url(#ac-ok)"
      />
      <text
        x={520}
        y={261}
        textAnchor="middle"
        fontSize={10.5}
        fill="var(--color-ink-3)"
      >
        hangi agent koştu
      </text>

      <text
        x={510}
        y={300}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Bir akış adımı normal bir{" "}
        <tspan fontFamily="var(--font-mono, monospace)">runs</tspan> kaydı
        üretir — bu yüzden rapor, elle çalıştırmayı da akış adımını da tek
        kaynaktan sayar.
      </text>
    </Frame>
  );
}

/* ── 7. MCP: iki yön ─────────────────────────────────────────────────────── */

/**
 * Dış araçlarla iki yönlü ilişki.
 *
 * Tek diyagramda iki yön: dışarıya doğru (biz araç kullanırız) ve içeriye
 * doğru (bizi araç olarak kullanırlar). Ayrı çizilseydi aradaki simetri —
 * aynı protokol, ters yön — görünmezdi.
 */
export function MCPDirections() {
  return (
    <Frame viewBox="0 0 1020 330" label="MCP'nin iki yönü">
      {/* Ortadaki ayraç: iki yön arasındaki sınır. */}
      <line
        x1={512}
        y1={20}
        x2={512}
        y2={300}
        stroke="var(--color-line)"
        strokeDasharray="5 5"
      />

      <text
        x={256}
        y={30}
        textAnchor="middle"
        fontSize={11.5}
        fontWeight={600}
        fill="var(--color-ink-2)"
      >
        DIŞARIYA — biz araç kullanırız
      </text>
      <text
        x={768}
        y={30}
        textAnchor="middle"
        fontSize={11.5}
        fontWeight={600}
        fill="var(--color-ink-2)"
      >
        İÇERİYE — bizi araç olarak kullanırlar
      </text>

      {/* Yön 1a: agent karar verir */}
      <text x={30} y={58} fontSize={11} fill="var(--color-ink-3)">
        1. Agent kendi karar verir
      </text>
      <Box
        x={30}
        y={68}
        w={150}
        h={50}
        title="Agent adımı"
        subtitle="kod yazar"
        tone="accent"
      />
      <Arrow x1={186} y1={80} x2={280} y2={72} />
      <Arrow x1={186} y1={93} x2={280} y2={97} />
      <Arrow x1={186} y1={106} x2={280} y2={122} />
      <Box x={286} y={54} w={190} h={34} title="Sentry" />
      <Box x={286} y={94} w={190} h={34} title="Notion" />
      <Box x={286} y={134} w={190} h={34} title="Veritabanı şeması" />

      {/* Yön 1b: akış karar verir */}
      <text x={30} y={205} fontSize={11} fill="var(--color-ink-3)">
        2. Akış karar verir — her seferinde aynı
      </text>
      <Box
        x={30}
        y={215}
        w={150}
        h={50}
        title="MCP düğümü"
        subtitle="araç seçili"
        tone="accent"
      />
      <Arrow x1={186} y1={240} x2={280} y2={240} label="ask_question" />
      <Box
        x={286}
        y={215}
        w={190}
        h={50}
        title="Belirli araç"
        subtitle="belirli argümanlar"
      />

      {/* Yön 2: dışarıdan bize */}
      <Box
        x={620}
        y={54}
        w={280}
        h={50}
        title="Claude Desktop · Cursor"
        subtitle="başka bir agent"
      />
      <Arrow x1={760} y1={110} x2={760} y2={148} label="MCP" />
      <Box
        x={620}
        y={154}
        w={280}
        h={62}
        title="Agent Coder"
        subtitle="akislari_listele · akis_calistir · calisma_durumu"
        tone="accent"
      />
      <Arrow x1={760} y1={222} x2={760} y2={252} />
      <Box x={620} y={258} w={280} h={44} title="Akış çalışır" tone="muted" />

      <text
        x={256}
        y={300}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Hangi agent&apos;ın hangi sunucuya erişeceği ayrı seçilir.
      </text>
      <text
        x={256}
        y={318}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Seçilmeyenin araçları o agent&apos;a hiç sunulmaz.
      </text>
      <text
        x={768}
        y={318}
        textAnchor="middle"
        fontSize={11.5}
        fill="var(--color-ink-2)"
      >
        Başlatma, elle ve Jira ile aynı kapıdan geçer.
      </text>
    </Frame>
  );
}

/**
 * Betikler — doğaçlama ile prosedür arasındaki fark.
 *
 * Diyagramın taşıdığı tek fikir: aynı işi iki kez yaptırdığınızda üstteki yol
 * iki farklı sonuç verebilir, alttaki yol veremez. Bu yüzden üst şeritte iki
 * farklı komut, alt şeritte tek bir dosya gösteriliyor.
 */
export function ScriptDeterminism() {
  return (
    <Frame viewBox="0 0 1020 340" label="Betiklerin getirdiği belirlilik">
      <text
        x={30}
        y={30}
        fontSize={11.5}
        fontWeight={600}
        fill="var(--color-ink-2)"
      >
        BETİKSİZ — model her seferinde yeniden karar verir
      </text>

      <Box
        x={30}
        y={44}
        w={170}
        h={46}
        title="Agent adımı"
        subtitle='"yükselt"'
        tone="accent"
      />
      <Arrow x1={206} y1={58} x2={300} y2={48} />
      <Arrow x1={206} y1={78} x2={300} y2={100} />
      <Box x={306} y={30} w={230} h={36} title="npm update" tone="muted" />
      <Box
        x={306}
        y={82}
        w={230}
        h={36}
        title="npm i paket@latest"
        tone="muted"
      />
      <Arrow x1={542} y1={48} x2={620} y2={62} />
      <Arrow x1={542} y1={100} x2={620} y2={78} />
      <Box x={626} y={44} w={200} h={46} title="Farklı sonuçlar" tone="muted" />
      <text x={846} y={72} fontSize={11.5} fill="var(--color-ink-2)">
        1. çalıştırma ≠ 2.
      </text>

      <line
        x1={30}
        y1={148}
        x2={990}
        y2={148}
        stroke="var(--color-line)"
        strokeDasharray="5 5"
      />

      <text
        x={30}
        y={182}
        fontSize={11.5}
        fontWeight={600}
        fill="var(--color-ink-2)"
      >
        BETİKLE — model NE ZAMAN&apos;a, betik NE YAPILACAĞINA karar verir
      </text>

      <Box
        x={30}
        y={196}
        w={170}
        h={46}
        title="Agent adımı"
        subtitle='"yükselt"'
        tone="accent"
      />
      <Arrow x1={206} y1={219} x2={300} y2={219} label="çağırır" />
      <Box
        x={306}
        y={196}
        w={230}
        h={46}
        title="upgrade-deps.sh"
        subtitle="gözden geçirilmiş"
        tone="accent"
      />
      <Arrow x1={542} y1={219} x2={620} y2={219} />
      <Box x={626} y={196} w={200} h={46} title="Aynı sonuç" />
      <text x={846} y={224} fontSize={11.5} fill="var(--color-ink-2)">
        1. çalıştırma = 2.
      </text>

      <text x={30} y={286} fontSize={11.5} fill="var(--color-ink-2)">
        Betik kütüphanede bir kez durur; birden fazla agent&apos;a atanır ve
        çalıştırma anında ortamına kopyalanır.
      </text>
      <text x={30} y={306} fontSize={11.5} fill="var(--color-ink-2)">
        Güncellersiniz — bir sonraki çalıştırma yeni sürümü kullanır, imaj
        yeniden derlenmez.
      </text>
      <text x={30} y={326} fontSize={11.5} fill="var(--color-ink-3)">
        Yalnızca komut çalıştırma yetkisi açık agent&apos;lara verilir;
        kapalıysa dosya ortama hiç girmez.
      </text>
    </Frame>
  );
}
