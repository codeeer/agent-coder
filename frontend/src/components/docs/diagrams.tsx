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

  /*
   * DİK KÖŞE ve SAÇ TELİ ÇİZGİ — bu bir kart değil, bir çizim öğesi.
   *
   * Yuvarlatılmış köşe ve 1,5px kenarlık, kutuyu arayüzdeki bir bileşene
   * benzetiyordu. Teknik çizimde kenar kalınlığı bilgi taşır: vurgulu olan
   * kalın, sıradan olan ince. Köşedeki tik işareti de aynı dilden — çizim
   * sayfalarındaki hizalama işareti.
   */
  const kalinlik = tone === "accent" ? 1.4 : 0.9;

  return (
    <g className="cizim-kutu">
      <rect
        x={x}
        y={y}
        width={w}
        height={h}
        rx={1}
        fill={fill}
        stroke={stroke}
        strokeWidth={kalinlik}
      />
      {tone === "accent" && (
        /* Sol üstte tik: vurgulu kutuyu renk DIŞINDA bir kanalla da işaretler. */
        <path
          d={`M${x} ${y + 9} L${x} ${y} L${x + 9} ${y}`}
          fill="none"
          stroke={stroke}
          strokeWidth={2.4}
        />
      )}
      <text
        x={x + w / 2}
        y={subtitle ? y + h / 2 - 4 : y + h / 2 + 4}
        textAnchor="middle"
        fontSize={12.5}
        fontWeight={600}
        letterSpacing="-0.01em"
        fill={titleFill}
      >
        {title}
      </text>
      {subtitle && (
        <text
          x={x + w / 2}
          y={y + h / 2 + 13}
          textAnchor="middle"
          fontSize={9.5}
          letterSpacing="0.04em"
          fill="var(--color-ink-3)"
          className="cizim-mono"
        >
          {/*
            BÜYÜK HARFE ÇEVRİLMİYOR — iki sebeple. Altyazılarda araç ve dosya
            adları geçiyor (`akislari_listele`) ve Türkçe büyük harf onları
            `AKİSLARİ_LİSTELE` yapıyordu: tanımlayıcı artık yanlış yazılmış
            oluyor. Ayrıca büyük harf metni genişletip iki kutudan taşırıyordu
            (ölçüldü: 317px yazı, 280px kutu). Teknik his mono ve harf
            aralığından geliyor, versaldan değil.
          */}
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
        strokeWidth={1}
        strokeDasharray={dashed ? "3 3" : undefined}
        markerEnd="url(#ac-ok)"
      />
      {label && (
        <text
          x={(x1 + x2) / 2}
          y={y1 === y2 ? y1 - 8 : (y1 + y2) / 2 - 6}
          textAnchor="middle"
          fontSize={9.5}
          letterSpacing="0.06em"
          fill="var(--color-ink-3)"
          className="cizim-mono cizim-etiket"
        >
          {label}
        </text>
      )}
    </g>
  );
}

/**
 * Bölge — bir sınırı çevreleyen kesikli kutu (container, ağ, kurum).
 *
 * Mimari diyagramında elle tekrarlanıyordu; kurumsal diyagramlarda beş kez
 * daha gerekince kendi parçasına çıkarıldı.
 */
function Zone({
  x,
  y,
  w,
  h,
  label,
  tone = "line",
}: {
  x: number;
  y: number;
  w: number;
  h: number;
  label: string;
  tone?: "line" | "danger" | "ok";
}) {
  const stroke =
    tone === "danger"
      ? "var(--color-danger)"
      : tone === "ok"
        ? "var(--color-ok)"
        : "var(--color-line-strong)";

  /*
   * ETİKET ÇİZGİYLE AYNI RENK DEĞİL — ölçümle düzeltildi.
   *
   * Önce ikisi de `stroke` kullanıyordu; nötr bölgede bu `line-strong` demek
   * ve koyu temada 11px'lik etiket kart zemininde **1,77** kontrastla
   * çıkıyordu (eşik 4,5). Çizgi için doğru olan renk, metin için yanlış:
   * kenarlık sessiz olmalı, etiket okunmalı.
   */
  const labelFill = tone === "line" ? "var(--color-ink-2)" : stroke;

  /* Köşe tikleri: çizim sayfalarındaki hizalama işaretleri. Kesikli çerçeveye
     sınır olduğunu tek başına söyletmiyor — köşeler de söylüyor. */
  const tik = 10;

  return (
    <g>
      <rect
        x={x}
        y={y}
        width={w}
        height={h}
        rx={1}
        fill="none"
        stroke={stroke}
        strokeWidth={0.9}
        strokeDasharray="4 4"
      />
      <g fill="none" stroke={stroke} strokeWidth={1.6}>
        <path d={`M${x} ${y + tik} L${x} ${y} L${x + tik} ${y}`} />
        <path d={`M${x + w - tik} ${y} L${x + w} ${y} L${x + w} ${y + tik}`} />
        <path d={`M${x + w} ${y + h - tik} L${x + w} ${y + h} L${x + w - tik} ${y + h}`} />
        <path d={`M${x + tik} ${y + h} L${x} ${y + h} L${x} ${y + h - tik}`} />
      </g>
      <text
        x={x + 16}
        y={y + 17}
        fontSize={9.5}
        fontWeight={500}
        letterSpacing="0.14em"
        fill={labelFill}
        className="cizim-mono"
      >
        {label.toLocaleUpperCase("tr")}
      </text>
    </g>
  );
}

/**
 * REDDEDİLEN yol — kırmızı kesikli çizgi ve ucunda çarpı.
 *
 * Diyagramda "olmayan yol"u çizmek bilinçli: kurumsal okuyucunun ilk sorusu
 * "peki şuraya çıkabilir mi" oluyor ve cevabı ancak çizilmiş bir engel
 * gösterebiliyor. Renk tek kanal değil — yanında "reddedilir" yazıyor.
 */
function Blocked({
  x1,
  y1,
  x2,
  y2,
  label,
}: {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  label?: string;
}) {
  const mx = (x1 + x2) / 2;
  const my = (y1 + y2) / 2;
  return (
    <g>
      <line
        x1={x1}
        y1={y1}
        x2={x2}
        y2={y2}
        stroke="var(--color-danger)"
        strokeWidth={1.5}
        strokeDasharray="4 4"
      />
      {/* Çarpı yerine BARİYER: çizginin üzerine dik duran çift çizgi, teknik
          çizimde "buradan geçilmez" işaretidir ve renkten bağımsız okunur. */}
      <g stroke="var(--color-danger)" strokeWidth={2} strokeLinecap="square">
        <line x1={mx - 3} y1={my - 8} x2={mx - 3} y2={my + 8} />
        <line x1={mx + 3} y1={my - 8} x2={mx + 3} y2={my + 8} />
      </g>
      {label && (
        <text
          x={mx}
          y={my - 12}
          textAnchor="middle"
          fontSize={10.5}
          fill="var(--color-danger)"
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
        markerWidth="5"
        markerHeight="5"
        orient="auto"
      >
        {/* Dolu üçgen değil açık uç: teknik çizimin ok başı incedir. */}
        <path
          d="M0 1 L10 5 L0 9"
          fill="none"
          stroke="var(--color-line-strong)"
          strokeWidth="1.6"
        />
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
          className="cizim-svg w-full"
          style={{ minWidth: 760 }}
          role="img"
          aria-label={label}
        >
          <Defs />
          {children}
        </svg>
      </div>
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
        fontSize={9.5}
        letterSpacing="0.1em"
        fill="var(--color-ink-3)"
        className="cizim-mono"
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
        subtitle="canvas + izleme"
      />
      <Box x={20} y={120} w={150} h={54} title="Frontend" subtitle="Next.js" />
      <Arrow x1={95} y1={86} x2={95} y2={116} />

      {/* Sunucu */}
      <Zone x={220} y={20} w={300} h={240} label="Backend (Go) — tek servis" />
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
        subtitle="akışlar · çalışmalar · anahtarlar"
      />
      <Arrow x1={370} y1={264} x2={370} y2={286} />

      {/* Sandbox */}
      <Zone x={580} y={20} w={420} h={200} label="İzole ağ — dışarıya port açmaz" />
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
        fontSize={9.5}
        letterSpacing="0.14em"
        fill="var(--color-ink-3)"
        className="cizim-mono"
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
  /*
   * SEVİYE ÇİZGİLERİ SÜTUNLARIN ARASINDA, İÇİNDEN DEĞİL.
   *
   * Önce çizgiler sabit aralıkla (140, 430, 720) konuyordu ve birincisi
   * "Başlangıç" kutusunun tam ortasından geçiyordu — kutu ikiye bölünmüş
   * görünüyordu. Ayrıca başlangıç bir SEVİYE DEĞİL, tetikleyici: etiketler
   * artık sütunların üstünde, ayraçlar boşluklarda.
   */
  const sutunlar = [
    { x: 30, w: 150, etiket: "başlangıç" },
    { x: 250, w: 180, etiket: "1. seviye" },
    { x: 540, w: 180, etiket: "2. seviye" },
    { x: 830, w: 160, etiket: "3. seviye" },
  ];
  const ayraclar = [215, 485, 775];

  return (
    <Frame viewBox="0 0 1020 268" label="Paralel çalıştırma">
      {ayraclar.map((x) => (
        <line
          key={x}
          x1={x}
          y1={34}
          x2={x}
          y2={214}
          stroke="var(--color-line)"
          strokeWidth={0.9}
          strokeDasharray="3 5"
        />
      ))}

      {sutunlar.map((s) => (
        <text
          key={s.etiket}
          x={s.x + s.w / 2}
          y={24}
          textAnchor="middle"
          fontSize={9.5}
          letterSpacing="0.1em"
          fill="var(--color-ink-3)"
          className="cizim-mono"
        >
          {s.etiket}
        </text>
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

      {/*
        ÖLÇÜ ÇİZGİSİ — teknik çizimin kendi aracı ve buradaki asıl bilgi:
        iki adım örtüştüğü için seviye 18 saniyede bitiyor, 30'da değil.
      */}
      <g stroke="var(--color-accent)" strokeWidth={0.9} fill="none">
        <line x1={250} y1={216} x2={250} y2={230} />
        <line x1={430} y1={216} x2={430} y2={230} />
        <line x1={250} y1={223} x2={430} y2={223} />
      </g>
      <text
        x={340}
        y={246}
        textAnchor="middle"
        fontSize={9.5}
        letterSpacing="0.04em"
        fill="var(--color-accent)"
        className="cizim-mono"
      >
        örnek: aynı anda 18 sn — sırayla olsaydı 30 sn
      </text>

      <text
        x={730}
        y={246}
        textAnchor="middle"
        fontSize={11}
        fill="var(--color-ink-2)"
      >
        Sonraki seviye, öncekinin TAMAMI bitmeden başlamaz.
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
    ["6", "Engine logları alınır", "maskelenip saklanır"],
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
        strokeWidth={0.9}
        strokeDasharray="3 5"
      />

      <text
        x={256}
        y={30}
        textAnchor="middle"
        fontSize={9.5}
        letterSpacing="0.12em"
        fill="var(--color-ink-2)"
        className="cizim-mono"
      >
        DIŞARIYA — biz araç kullanırız
      </text>
      <text
        x={768}
        y={30}
        textAnchor="middle"
        fontSize={9.5}
        letterSpacing="0.12em"
        fill="var(--color-ink-2)"
        className="cizim-mono"
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
        subtitle="listele · çalıştır · durum"
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
    <Frame viewBox="0 0 1020 340" label="Script'lerin getirdiği belirlilik">
      <text
        x={30}
        y={30}
        fontSize={9.5}
        letterSpacing="0.12em"
        fill="var(--color-ink-2)"
        className="cizim-mono"
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
        strokeWidth={0.9}
        strokeDasharray="3 5"
      />

      <text
        x={30}
        y={182}
        fontSize={9.5}
        letterSpacing="0.12em"
        fill="var(--color-ink-2)"
        className="cizim-mono"
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

/* ── Kurumsal: runner container'ının anatomisi ───────────────────────────── */

/**
 * Bir agent adımının içinde ne var, ne yok.
 *
 * Kurumsal değerlendirmede ilk sorulan bu: "kodum nerede çalışıyor, orada
 * başka ne var, iş bitince ne kalıyor". Cevabın tamamı tek çizimde.
 */
export function RunnerAnatomy() {
  return (
    <Frame viewBox="0 0 1020 392" label="Runner container'ının anatomisi">
      <Box x={20} y={150} w={160} h={64} title="Backend" subtitle="Go, tek servis" />
      <Arrow x1={186} y1={182} x2={246} y2={182} label="Docker API" />

      <Zone x={250} y={30} w={470} h={330} label="GEÇİCİ CONTAINER — iş başına açılır" />

      <Box x={272} y={62} w={200} h={54} title="/work" subtitle="deponun klonu" tone="accent" />
      <Box x={490} y={62} w={208} h={54} title="opencode motoru" subtitle="başsız (headless)" />
      <Box
        x={272}
        y={132}
        w={200}
        h={54}
        title="/home/agent/scripts"
        subtitle="yalnızca yetkiliyse"
      />
      <Box x={490} y={132} w={208} h={54} title="ortam değişkenleri" subtitle="token'lar, proxy" />
      <Box
        x={272}
        y={202}
        w={426}
        h={48}
        title="CPU ve bellek sınırı — ayardan"
        subtitle="varsayılan 2 çekirdek / 4 GB"
        tone="muted"
      />

      <text x={272} y={286} fontSize={11.5} fill="var(--color-ink-2)">
        Dışarıya PORT AÇMAZ. Host dosya sistemine bağlanmaz.
      </text>
      <text x={272} y={306} fontSize={11.5} fill="var(--color-ink-2)">
        Başka bir çalıştırmanın container&apos;ını görmez.
      </text>
      <text x={272} y={334} fontSize={11} fill="var(--color-ink-3)">
        Çıkış denetimi açıkken internete rotası olmayan ayrı bir ağda doğar.
      </text>

      <Arrow x1={724} y1={120} x2={806} y2={120} label="silmeden önce" />
      <Box
        x={812}
        y={92}
        w={184}
        h={56}
        title="Engine logları"
        subtitle="container çıktısı + oturum"
      />

      <Arrow x1={724} y1={230} x2={790} y2={230} label="iş bitince" />
      <Box
        x={796}
        y={202}
        w={200}
        h={56}
        title="container + volume"
        subtitle="SİLİNİR"
        tone="muted"
      />
      <text x={796} y={286} fontSize={11} fill="var(--color-ink-3)">
        Zaman aşımı, iptal ya da sunucunun
      </text>
      <text x={796} y={302} fontSize={11} fill="var(--color-ink-3)">
        yeniden başlaması — hepsinde silinir.
      </text>
    </Frame>
  );
}

/* ── Kurumsal: çıkış denetimi ────────────────────────────────────────────── */

/**
 * Proxy, whitelist ve reddedilen yol.
 *
 * DENETİM AYARDAN DEĞİL AĞDAN GELİYOR — ölçülmüş bir karar: ortam
 * değişkeniyle verilen proxy ATLANABİLİYOR (26 bağlantının 5'i atladı).
 * Bu yüzden container internete rotası olmayan bir ağda doğuyor ve dışarıya
 * tek yol kapı.
 */
export function EgressControl() {
  return (
    <Frame viewBox="0 0 1020 470" label="Çıkış denetimi: kapı, whitelist ve kurumsal proxy">
      <Zone x={20} y={40} w={250} h={190} label="İNTERNETE ROTASI OLMAYAN AĞ" />
      <Box
        x={42}
        y={80}
        w={206}
        h={64}
        title="Runner container"
        subtitle="HTTP_PROXY = kapı"
        tone="accent"
      />
      <text x={42} y={176} fontSize={11} fill="var(--color-ink-3)">
        Java, curl ve npm için de
      </text>
      <text x={42} y={192} fontSize={11} fill="var(--color-ink-3)">
        ayrı ayrı yazılır — atlanamasın.
      </text>

      {/* Etiket okun ÜSTÜNDE değil altında: 66px'lik boşluğa 95px'lik metin
          sığmıyordu ve iki kutunun üzerine biniyordu. */}
      <Arrow x1={274} y1={112} x2={340} y2={112} />
      <text x={307} y={136} textAnchor="middle" fontSize={10.5} fill="var(--color-ink-3)">
        CONNECT
      </text>

      <Box
        x={346}
        y={70}
        w={240}
        h={84}
        title="Çıkış kapısı"
        subtitle="çalıştırma başına ayrı dinleyici"
        tone="accent"
      />
      <text x={346} y={176} fontSize={11} fill="var(--color-ink-3)">
        TLS AÇMAZ: yalnızca CONNECT
      </text>
      <text x={346} y={192} fontSize={11} fill="var(--color-ink-3)">
        satırındaki adı görür, baytları tünneller.
      </text>

      {/* İzinli yol */}
      <Arrow x1={590} y1={96} x2={660} y2={96} label="izinliyse" />
      <Box
        x={666}
        y={68}
        w={150}
        h={56}
        title="Kurumsal proxy"
        subtitle="tanımlıysa"
        tone="muted"
      />
      <Arrow x1={820} y1={96} x2={880} y2={96} />
      <Box x={886} y={68} w={110} h={56} title="Hedef" subtitle="izinli host" />

      {/* Reddedilen yol */}
      <Blocked x1={590} y1={166} x2={790} y2={196} />
      <text x={690} y={208} textAnchor="middle" fontSize={10.5} fill="var(--color-danger)">
        403 — bu adrese çıkış izinli değil
      </text>
      <Box x={796} y={170} w={200} h={52} title="Listede olmayan adres" tone="muted" />

      {/* Liste */}
      <Zone x={20} y={258} w={976} h={186} label="KAPININ BAKTIĞI LİSTE" />

      <Box
        x={42}
        y={292}
        w={300}
        h={62}
        title="Sizin whitelist'iniz"
        subtitle="satır başına bir domain"
      />
      <text x={42} y={378} fontSize={11.5} fill="var(--color-ink-2)">
        BOŞ BIRAKMAK KISIT YOKLUĞUDUR: liste boşken
      </text>
      <text x={42} y={396} fontSize={11.5} fill="var(--color-ink-2)">
        tüm adreslere çıkılır, denetim yalnızca proxy&apos;dir.
      </text>

      <text x={372} y={318} fontSize={16} fill="var(--color-ink-3)">
        +
      </text>

      <Box
        x={400}
        y={292}
        w={330}
        h={62}
        title="Zorunlu adresler — kendiliğinden"
        subtitle="LLM sağlayıcı · depo · registry · MCP"
        tone="accent"
      />
      <text x={400} y={378} fontSize={11.5} fill="var(--color-ink-2)">
        Ayarlara zaten girdiğiniz adresler listeye kendiliğinden
      </text>
      <text x={400} y={396} fontSize={11.5} fill="var(--color-ink-2)">
        eklenir — ama YALNIZCA whitelist doluyken.
      </text>

      <text x={766} y={310} fontSize={11.5} fontWeight={600} fill="var(--color-ink-2)">
        Karar host&apos;a bakar,
      </text>
      <text x={766} y={328} fontSize={11.5} fontWeight={600} fill="var(--color-ink-2)">
        porta bakmaz.
      </text>
      <text x={766} y={352} fontSize={11} fill="var(--color-ink-3)">
        İzinli bir host üzerinden
      </text>
      <text x={766} y={368} fontSize={11} fill="var(--color-ink-3)">
        sızdırma bu kapının
      </text>
      <text x={766} y={384} fontSize={11} fill="var(--color-ink-3)">
        çözebileceği bir şey değil.
      </text>
    </Frame>
  );
}

/* ── Kurumsal: verinin sınırı ────────────────────────────────────────────── */

/**
 * Kodun kurumdan çıkıp çıkmadığı — koşullu bir cevap, ve koşulu çizili.
 *
 * "Veri dışarı çıkmaz" tek başına YANLIŞ olurdu: model çağrısı kodun bir
 * parçasını sağlayıcıya taşır. Belirleyici olan sağlayıcının nerede
 * çalıştığı; diyagram iki kurulumu yan yana koyuyor.
 */
export function DataBoundary() {
  return (
    <Frame viewBox="0 0 1020 430" label="Verinin sınırı: iki kurulum">
      <Zone x={20} y={30} w={470} h={370} label="A · KURUM İÇİ SAĞLAYICI" tone="ok" />
      <Box x={44} y={70} w={190} h={54} title="Depo" subtitle="kurum içi git" />
      <Box x={44} y={144} w={190} h={54} title="Runner" subtitle="klon + agent" tone="accent" />
      <Arrow x1={139} y1={130} x2={139} y2={140} />
      <Box
        x={266}
        y={144}
        w={200}
        h={54}
        title="LLM sağlayıcı"
        subtitle="LiteLLM / vLLM — kurum içi"
        tone="accent"
      />
      <Arrow x1={240} y1={171} x2={260} y2={171} />
      <Box x={44} y={222} w={422} h={48} title="Agent Coder veritabanı" subtitle="çıktı, diff, maliyet, loglar" tone="muted" />

      <text x={44} y={306} fontSize={11.5} fontWeight={600} fill="var(--color-ok)">
        Hiçbir istek kurum ağının dışına çıkmaz.
      </text>
      <text x={44} y={326} fontSize={11} fill="var(--color-ink-2)">
        Kod, prompt ve model yanıtı aynı ağın içinde kalır;
      </text>
      <text x={44} y={342} fontSize={11} fill="var(--color-ink-2)">
        çıkış kapısı yalnızca iç adreslere izinlidir.
      </text>
      <text x={44} y={370} fontSize={11} fill="var(--color-ink-3)">
        Kapalı ağ kurulumu: runner imajı ve Node sürümleri
      </text>
      <text x={44} y={386} fontSize={11} fill="var(--color-ink-3)">
        derleme anında hazırlanır, koşuda indirme yapılmaz.
      </text>

      <Zone x={520} y={30} w={476} h={370} label="B · DIŞ SAĞLAYICI (OpenRouter vb.)" />
      <Box x={544} y={70} w={190} h={54} title="Depo" subtitle="kurum içi git" />
      <Box x={544} y={144} w={190} h={54} title="Runner" subtitle="klon + agent" tone="accent" />
      <Arrow x1={639} y1={130} x2={639} y2={140} />
      <Arrow x1={740} y1={171} x2={790} y2={171} label="izinli host" />
      <Box x={796} y={144} w={180} h={54} title="Dış LLM API" subtitle="internet" />
      <Box x={544} y={222} w={432} h={48} title="Agent Coder veritabanı" subtitle="çıktı, diff, maliyet, loglar" tone="muted" />

      <text x={544} y={306} fontSize={11.5} fontWeight={600} fill="var(--color-warn)">
        Model çağrısı kodun bir parçasını dışarı taşır.
      </text>
      <text x={544} y={326} fontSize={11} fill="var(--color-ink-2)">
        Talimat ve ilgili dosyalar sağlayıcıya gider — bu, çıkış
      </text>
      <text x={544} y={342} fontSize={11} fill="var(--color-ink-2)">
        kapısının engelleyebileceği bir şey değil, kurulum kararıdır.
      </text>
      <text x={544} y={370} fontSize={11} fill="var(--color-ink-3)">
        Depoya gönderim ve PR yine akışın işidir; git kimlik
      </text>
      <text x={544} y={386} fontSize={11} fill="var(--color-ink-3)">
        bilgisi agent&apos;a hiç ulaşmaz.
      </text>
    </Frame>
  );
}

/* ── Kurumsal: SSL inspection ────────────────────────────────────────────── */

/** Kurumsal kök sertifikanın gittiği dört yer. */
export function CorporateTLS() {
  return (
    <Frame viewBox="0 0 1020 300" label="Kurumsal kök sertifikanın dağıtımı">
      <Box
        x={20}
        y={110}
        w={220}
        h={70}
        title="Kök sertifika (PEM)"
        subtitle="Ayarlar → Kurumsal ağ"
        tone="accent"
      />

      <Arrow x1={246} y1={100} x2={330} y2={62} />
      <Arrow x1={246} y1={132} x2={330} y2={124} />
      <Arrow x1={246} y1={158} x2={330} y2={186} />
      <Arrow x1={246} y1={186} x2={330} y2={248} />

      <Box x={336} y={36} w={300} h={52} title="Backend'in giden çağrıları" subtitle="Jira, git API, katalog, MCP" />
      <Box x={336} y={98} w={300} h={52} title="Container'ın sistem deposu" subtitle="curl, git, opencode" />
      <Box x={336} y={160} w={300} h={52} title="Java truststore" subtitle="Maven ve JVM ayrı bakar" />
      <Box x={336} y={222} w={300} h={52} title="Node / npm" subtitle="NODE_EXTRA_CA_CERTS" />

      <text x={676} y={70} fontSize={11.5} fill="var(--color-ink-2)">
        SSL inspection yapan bir ağda dördü de
      </text>
      <text x={676} y={88} fontSize={11.5} fill="var(--color-ink-2)">
        gerekiyor: biri eksikse o araç sessizce
      </text>
      <text x={676} y={106} fontSize={11.5} fill="var(--color-ink-2)">
        &quot;sertifika doğrulanamadı&quot; der.
      </text>

      <text x={676} y={150} fontSize={11} fill="var(--color-ink-3)">
        Sertifika arayüzden değiştirilir ve
      </text>
      <text x={676} y={166} fontSize={11} fill="var(--color-ink-3)">
        bir sonraki çalıştırmada geçerli olur —
      </text>
      <text x={676} y={182} fontSize={11} fill="var(--color-ink-3)">
        yeniden başlatma gerekmez.
      </text>

      <text x={676} y={222} fontSize={11} fill="var(--color-ink-3)">
        Bozuk bir PEM kaydedilmez: dosya
      </text>
      <text x={676} y={238} fontSize={11} fill="var(--color-ink-3)">
        biçimi kaydetme anında doğrulanır.
      </text>
    </Frame>
  );
}

/* ── Ölçek: toplu çalıştırma kuyruğu ─────────────────────────────────────── */

/** Otuz projede aynı akış — sınır kadar paralel, gerisi sırada. */
export function BatchQueue() {
  // Aralık 44'ten 38'e indi: son kutu bölge çerçevesinin ve altındaki notun
  // üzerine biniyordu (ölçüldü, ekran görüntüsünde görüldü).
  const bekleyen = ["proje-4", "proje-5", "proje-6", "…proje-30"].map((ad, i) => ({
    ad,
    y: 226 + i * 38,
  }));

  return (
    <Frame viewBox="0 0 1020 420" label="Toplu çalıştırma kuyruğu">
      <Box
        x={20}
        y={120}
        w={190}
        h={64}
        title="Akış + 30 proje"
        subtitle="tek hamlede sıraya"
        tone="accent"
      />
      <Arrow x1={216} y1={152} x2={276} y2={152} />

      <Zone x={280} y={30} w={300} h={356} label="KUYRUK — kalıcı, veritabanında" />
      <Box x={302} y={70} w={256} h={44} title="proje-1" subtitle="çalışıyor" tone="accent" />
      <Box x={302} y={122} w={256} h={44} title="proje-2" subtitle="çalışıyor" tone="accent" />
      <Box x={302} y={174} w={256} h={44} title="proje-3" subtitle="çalışıyor" tone="accent" />
      {bekleyen.map((b) => (
        <Box key={b.ad} x={302} y={b.y} w={256} h={32} title={b.ad} tone="muted" />
      ))}

      <text x={302} y={406} fontSize={11} fill="var(--color-ink-3)">
        Sıra eklenme sırasıdır; öncelik yok.
      </text>

      <Arrow x1={584} y1={92} x2={650} y2={92} label="sınır kadar" />
      <Box
        x={656}
        y={62}
        w={200}
        h={60}
        title="Eşzamanlılık sınırı"
        subtitle="Ayarlar → Çalıştırma"
        tone="accent"
      />
      <text x={866} y={88} fontSize={11.5} fill="var(--color-ink-2)">
        Kuyruğun kendi
      </text>
      <text x={866} y={104} fontSize={11.5} fill="var(--color-ink-2)">
        paralellik ayarı YOK.
      </text>

      <Arrow x1={584} y1={196} x2={650} y2={196} label="biri bitince" />
      <Box x={656} y={166} w={340} h={60} title="sıradaki kendiliğinden başlar" subtitle="olay güdümlü — yoklama yok" />

      <Box
        x={656}
        y={252}
        w={340}
        h={64}
        title="Bir proje düşerse kuyruk DURMAZ"
        subtitle="sebebi satırında yazar, sıradaki başlar"
        tone="muted"
      />
      <Box
        x={656}
        y={330}
        w={340}
        h={64}
        title="Sunucu yeniden başlarsa"
        subtitle="bekleyenler bekler, kesilenler düğmeyle sürer"
        tone="muted"
      />
    </Frame>
  );
}
