# Agent Coder

AI coding agent'larını n8n benzeri bir tuval üzerinde workflow'lara bağlayıp çalıştıran platform.
Örnek akış: Jira'dan task çek → analiz et → kod geliştir → code review → PR aç.

Bu dosya projenin tek kural kaynağıdır. Claude Code ve opencode aynı dosyayı okur
(`CLAUDE.md` bu dosyaya yönlendirir).

---

## Altın Kurallar

1. **Proje dizini dışına hiçbir kalıcı dosya yazılmaz.** Notlar, planlar, geçici çıktılar —
   hepsi `agent-coder/` içinde kalır. Plan dokümanları `plans/NN-konu-YYYY-AA-GG.md` formatında.
2. **Spec-driven.** Kod yazılmadan önce `specs/NNN-ozellik/spec.md` → `plan.md` → `tasks.md`
   sırası tamamlanır. Detay: [.claude/skills/spec-driven/SKILL.md](.claude/skills/spec-driven/SKILL.md)

   **Bir konunun tek spec'i vardır.** Var olan bir ekranın davranışı değişiyorsa yeni klasör
   açılmaz; o spec'e bir karar ve **Karar geçmişi** kaydı eklenir. Aksi halde aynı ekranın iki
   spec'i olur ve hangisinin geçerli olduğu ancak ikisi de okununca anlaşılır — bir kez oldu
   (`004-rapor` / eski `012-rapor-yonetici`). Dizin: [specs/README.md](specs/README.md)
3. **Anlaşılmayan bir şey varsa sor.** Varsayımla ilerlemek yerine soru sor.
4. **opencode bağımlılığı `internal/runner` dışına sızmaz.** Sistemin geri kalanı sadece
   `runner.Runner` arayüzünü bilir — ileride kendi motorumuzla değiştireceğiz.

---

## Mimari

```
┌──────────────┐   HTTP/SSE   ┌──────────────┐   Docker API   ┌────────────────────┐
│  frontend    │ ───────────► │   backend    │ ─────────────► │ opencode-runner    │
│  Next.js/TS  │ ◄─────────── │     Go       │                │ (iş başına, geçici)│
│  React Flow  │              │  DAG motoru  │ ◄───HTTP/SSE── │  opencode serve    │
└──────────────┘              └──────┬───────┘                └─────────┬──────────┘
                                     │                                  │
                              ┌──────▼───────┐            ┌────────▼─────────┐
                              │  PostgreSQL  │            │  LLM sağlayıcı   │
                              └──────────────┘            │ OpenRouter /     │
                                                          │ LiteLLM / uyumlu │
                                                          └──────────────────┘
```

- **frontend** — Next.js 15 App Router, TypeScript strict, Tailwind + shadcn/ui,
  `@xyflow/react` tuvali, TanStack Query, SSE ile canlı run takibi.
- **backend** — Go. Workflow DAG motoru, run/step state, sandbox orkestrasyonu,
  entegrasyonlar (Jira, git sağlayıcılar), çoklu sağlayıcı model kataloğu ve maliyet hesabı.
- **runner** — Servis değil. Backend her agent adımı için bu image'dan geçici bir container
  başlatır, repo'yu clone eder, içinde `opencode serve` çalıştırır, iş bitince siler.
- **postgres** — Tek kalıcı state. Sağlayıcılar, model kataloğu, workflow tanımları,
  run/step geçmişi.

### Dizin haritası

| Dizin | İçerik |
|-------|--------|
| `backend/` | Go servisi — `cmd/` giriş noktaları, `internal/` tüm iş mantığı |
| `frontend/` | Next.js uygulaması |
| `runner/` | opencode runner Docker image'ı ve entrypoint'i |
| `deploy/` | docker-compose dosyaları |
| `plans/` | Numaralı, tarihli plan dokümanları |
| `specs/` | Özellik başına `spec.md` / `plan.md` / `tasks.md` |
| `.claude/` | **Bu projeyi geliştirirken** kullandığımız agent'lar ve skill'ler |
| `.opencode/` | **Ürünün son kullanıcıya sunduğu** agent'lar + opencode config'i |

> `.claude/agents/` ile `.opencode/agents/` karıştırılmamalı. Birincisi bizim geliştirme
> araçlarımız, ikincisi ürünün çalıştırdığı agent'lar. `.claude/skills/` her ikisi tarafından
> da okunur (opencode `.claude/skills/` yolunu doğal olarak destekler).

---

## Komutlar

```bash
make env              # .env oluştur (şifreleme anahtarını kendisi üretir)
make up               # tüm stack'i ayağa kaldır (postgres + backend + frontend)
make dev              # hot-reload ile geliştirme modu
make down             # durdur
make clean            # durdur ve TÜM verileri sil
make logs             # tüm servislerin logları
make migrate          # migration'ları uygula (açılışta zaten uygulanır)
make migrate-down     # son migration'ı geri al
make migrate-status   # migration durumu
make psql             # Postgres kabuğu
make test             # birim testleri (veritabanı gerekmez)
make test-integration # gerçek Postgres'e karşı testler (stack ayakta olmalı)
make lint             # gofmt + go vet + eslint
make runner           # opencode-runner image'ını build et
make ps               # servis durumları
```

`make test` veritabanı olmadan çalışır — entegrasyon testleri `TEST_DATABASE_URL`
tanımlı değilse atlanır. Bunlar `make test-integration` ile ayrı çalıştırılır ve
tek veritabanını paylaştıkları için `-p 1` ile sırayla koşarlar.

Go veya Node host'a kurulu olmak zorunda değil — her şey container içinde derlenir.

---

## Konvansiyonlar

### Go

- Paket düzeni: `cmd/<binary>/main.go` sadece wiring yapar; iş mantığı `internal/` altında.
- Hata sarmalama: `fmt.Errorf("x yapılamadı: %w", err)`. Sentinel hatalar `var ErrFoo = errors.New(...)`.
- Log: `log/slog`, yapılandırılmış alanlarla. **Secret/token asla loglanmaz.**
- DB: `pgx/v5`, sorgular elle yazılır. Tarama `pgx.CollectRows` + `pgx.RowToStructByName`
  ile yapılır. Değer birleştirmesi **her zaman** `$1` parametreleriyle — SQL metni
  kullanıcı girdisiyle birleştirilmez.
- Migration: `goose`, `backend/internal/db/migrations/NNNNNN_ad.sql`. Migration'lar geri alınabilir olmalı.
- Context her zaman ilk parametre: `func Do(ctx context.Context, ...)`.
- Test: `testify/require`. Harici servisler `httptest` ile taklit edilir.

### TypeScript / Next.js

- `strict: true`, `any` yasak — bilinmeyen için `unknown` + daraltma.
- Server Component varsayılan; `"use client"` sadece gerektiğinde.
- API tipleri `frontend/src/lib/types.ts` içinde tek kaynaktan; elle `fetch` yerine `lib/api.ts`.
- Bileşen dosyaları `PascalCase.tsx`, yardımcılar `camelCase.ts`.
- **Saf mantık React bileşeninin içine gömülmez.** Ayrıştırma, hesaplama, biçimlendirme
  kendi modülünde durur ve `node --test` ile test edilir. Bileşenin içindeki mantık test
  edilemez; edilemeyen mantığın hatası ancak tarayıcıda görülür (spec 005, Ölçüm 1:
  gömülü bir ayrıştırıcıdaki sonsuz döngü 32 GB bellek tüketip makineyi kilitledi).
- **`g` bayraklı bir düzenli ifade özyinelemeli kodda PAYLAŞILMAZ.** `RegExp` nesnesi
  `lastIndex` durumunu kendi içinde taşır; iç çağrı onu sıfırlarsa dış döngü aynı
  eşleşmeyi sonsuza kadar bulur. Her çağrıda yeni nesne üretin.
- **Güvenilmeyen metni işleyen kod sınırlıdır.** Agent çıktısı, model yanıtı ve depo
  içeriği güvenilmez: özyineleme derinliğine ve üretilen eleman sayısına üst sınır
  koyun. Sınıra gelindiğinde biçimlenmemiş metin gösterilir — donmak seçenek değil.
- **Kullanıcı metni asla HTML'e çevrilmez.** `dangerouslySetInnerHTML` kullanılmaz;
  render doğrudan React elemanı üretir. Bağlantı şemaları beyaz listeyle sınırlıdır
  (`http`, `https`, `mailto`, göreli yollar).

### Tema

Üç durum: sistem (varsayılan), açık, koyu. Seçim `<html data-theme="light|dark">`
özniteliğinde durur; öznitelik yoksa işletim sistemi tercihi geçerlidir.

- Yeni bir renk jetonu eklerken **koyu karşılığı da yazılır** — hem
  `@media (prefers-color-scheme: dark) { :root:not([data-theme]) }` hem de
  `:root[data-theme="dark"]` bloğuna. Liste bilerek iki yerdedir; biri seçim
  yapılmadan önceki, diğeri açık seçim durumunu karşılar, tek seçiciye indirilemez.
- Jetonla birlikte `color-scheme` de verilir; tarayıcının kendi çizdikleri
  (kaydırma çubuğu, form denetimleri) ancak onunla doğru tarafa geçer.
- Tema **veritabanına yazılmaz.** Davranış parametresi değil, bakan kişinin
  tercihidir; sunucuda tutulsaydı bir kullanıcı diğerinin ekranını değiştirirdi.
- Renk sabiti CSS'e gömülmez, jetondan gelir — bir veri URI'sinin içine bile
  (`--select-arrow` örneği).
- **`globals.css`'e yazılan öğe kuralları `@layer base` içinde durur.** CSS'te
  katmansız bir kural, katmanlı olanların hepsini yener; Tailwind utility'leri
  `@layer utilities` içindedir. Katman dışına yazılan bir bildirim aynı özelliği
  veren her utility'yi sessizce ezer. (Spec 006, Ölçüm 3: katmansız bir
  `button { color: inherit }` düğmelerdeki bütün metin renklerini eziyordu.)
  Tek bilinçli istisna odak halkasıdır — o her zaman kazanmalı.
- **Tailwind'in preflight'ı zaten yaptığı şeyi tekrar yazmayın.** Aynı sıfırlama
  ikinci kez yazıldığında faydası yok, katman dışına düşme riski var.

**Açık tema, koyu temanın tersi DEĞİLDİR.**

Koyu tema referans olarak iyi görünüyor diye açık temayı onun renklerini ters
çevirerek üretmeyin. Her bileşenin açık temadaki **kontrastı, görsel hiyerarşisi
ve anlamı** bağımsız olarak değerlendirilir.

Sebep basit: aynı renk çifti iki zeminde aynı işi yapmaz.

- Koyu zeminde ayırt edilen soluk bir gri, beyaz kart üzerinde kaybolur —
  `ink-3` koyu temada geçerken açık temada sayfa zemininde 3,98:1 kalıyordu.
- Ters yön de olur: `info` rengi rozet zemininde koyu temada 6,87:1 ile rahat
  geçerken açık temada 4,11:1 kalıyordu.
- Hiyerarşi de taşınmaz. Koyu zeminde "daha parlak = daha önemli"dir; açık
  zeminde "daha koyu = daha önemli". Değerleri ters çevirmek vurguyu ters çevirir.
- Anlam da taşınmaz. Koyu zeminde yumuşak duran bir uyarı sarısı, beyaz üzerinde
  hem okunmaz hem de uyarı gibi durmaz.

Uyulacak sıra: her tema için değeri **ayrı seç**, sonra `scripts/theme-audit.mjs`
ile **ölç**. Aracın "tema eşliği" bölümü tam olarak bu hatayı arar — bir
bileşenin bir temada geçip diğerinde kalması. Göz bu hatayı bulamaz: iki tema
aynı anda görülemiyor ve 4,1 ile 4,6 arası bakışla ayırt edilmiyor.

### Görsel doğrulama

Tip kontrolü, linter ve birim testler **rengin ve yerleşimin doğru olduğunu
söyleyemez.** Bu projede iki hata yalnızca ekrana bakılarak yakalandı: kilitlenen
bir sayfa (spec 005) ve düğme yazılarının yanlış renkte çıkması (spec 006).

- Playwright MCP `.mcp.json` içinde tanımlı.
- Betikle: `node scripts/screenshot.mjs <yol> <light|dark> <çıktı.png>`
- Kurulum kök `package.json`'da (`npm install`). Orada **yalnızca geliştirme
  araçları** durur; uygulama bağımlılıkları `frontend/package.json` ve
  `backend/go.mod` içindedir. Araç elle kurulmak zorunda olsaydı bir sonraki
  oturumda yine yok olurdu — bir kez oldu.
- Renk şikâyetlerinde ekran görüntüsü yerine `--probe`: hesaplanmış
  `background-color` / `color` / `border-color` değerlerini yazdırır, tartışmayı
  bitirir.
- Tema `colorScheme` **ve** `localStorage` ile birlikte zorlanır; geliştirme
  makinesi koyu temada olsa bile açık tema görülebilir.

### Tema denetimi

Renk için **bakmak yetmez, ölçmek gerekir**: göz 4,4:1 ile 4,6:1 arasını ayırt
edemez ve iki temayı aynı anda tutamaz. `node scripts/theme-audit.mjs` her sayfayı
iki temada açıp hesaplanmış renkleri okur, WCAG oranını hesaplar ve **iki temanın
sonucunu karşılaştırır**.

- Eşikler: metin 4,5:1 (iri 3:1), **denetim sınırı 3:1** (WCAG 1.4.11).
- Çıktıdaki "tema eşliği" bölümü asıl değerli kısım: bir bileşenin bir temada
  geçip diğerinde kalması gözle bulunamayan tek hata sınıfıdır.
- Renk **tarayıcıya çözdürülür** (tuval üzerinden). Tailwind v4 `/35` ekini
  `oklab(... / 0.35)` olarak üretiyor; elle yazılmış bir `rgba()` ayrıştırıcısı
  bunları sessizce atlar — bir kez atladı, bütün rozet kenarları ölçüsüz kaldı.
- "0 kalan" sonucunu sorgulayın: aracın gerçekten baktığını doğrulamadan
  temiz raporlamayın.

### Liste uçları ve sayfalama

Tüm liste uçları **aynı zarfı** döner: `{items, total, limit, offset}`. Kimi uç
çıplak dizi dönseydi istemci her uç için ayrı bir okuma yolu yazardı.

- Sınırlar `internal/paging` içinde tek yerde: varsayılan 25, azami 200.
- Azami sınır kullanıcı için değil **veritabanı için**: `?limit=100000` tüm
  tabloyu belleğe çeker.
- Bozuk değer (`?limit=abc`) hata değildir, varsayılana düşer — liste ucu insan
  eliyle de çağrılıyor ve 400 dönmek listeyi hiç göstermemek olurdu.
- `total` **süzgeçten geçen** kayıt sayısıdır, tablodaki toplam değil.
- Açılır liste besleyen sorgular (`projects.list({ limit: 200 })`) pencereyi
  AÇIKÇA geniş verir; varsayılana güvenmek sessizce eksik seçenek gösterirdi.
- Arayüzde tek bileşen: `components/ui/Pagination`. Aralık her zaman yazılır,
  ileri/geri yalnızca birden fazla sayfa varken çıkar.

`limit` adını başka bir anlamda kullanmayın: `/api/runs` bir süre onu
eşzamanlılık sınırı için kullandı ve sayfalama denetimi yanlış aralık çizdi.
Kapasite alanları ayrı: `active`, `concurrencyLimit`.

### Renk token'ları: süsleme mi, denetim mi?

| Token | Sorumluluk |
|---|---|
| `--color-line` | süsleme — kart kenarı, ayraç, düğüm çerçevesi |
| `--color-line-strong` | süsleme (güçlü) — vurgulanmış çerçeve |
| `--color-control-line` | **denetim sınırı** — düğme, girdi, açılır liste |

Ayrımın testi: *bu çizgi olmasaydı kullanıcı orada tıklanabilir bir şey olduğunu
anlar mıydı?* Hayırsa `control-line` ve 3:1 zorunlu. Bu rol tanımlı değilken
bileşenler süsleme token'ını ödünç almıştı; ikincil düğmenin kenarı 1,8:1,
girdininki 1,31:1 ölçüldü — yani düğme ve kutu sınırından tanınamıyordu.

Süsleme çizgilerini koyulaştırmak yanlış cevaptı: arayüz gereksizce ağırlaşırdı.

### Yönetici rakamları

Rapor ekranı bir yöneticinin ekranıdır; iki kural onu yanlış bir şey söylemekten
korur.

**Hız metriği asla yalnız gösterilmez.** Yanında onu dengeleyen bir kalite ya da
risk metriği durur. PR sayısının arttığı ama değişiklik boyutunun da büyüdüğü
bir dönem ilerleme değil, biriken risktir — sektör ölçümlerinde PR sayısı
katlanırken PR başına olay sayısının çok daha hızlı arttığı örnekler var. Tek
başına bir hız rakamı, gerilemeyi ilerleme gibi gösterebilir.

**Ölçmediğimiz şey ekranda ima edilmez.** "Açılan PR" ile "işe yarayan PR" aynı
şey değil; bu sistem PR'ın sonrasını (birleşti mi, incelemeden geçti mi) takip
etmiyor ve ekran bunu açıkça yazıyor. O satır süs değil, tasarımın parçası —
kaldırılırsa ekran yanlış bir iddiada bulunmaya başlar.

**Maliyet bir paydayla gösterilir.** Toplam harcama ölçekle birlikte zaten büyür
ve büyümesi kötü haber değildir; yönetilebilir olan birim maliyettir ("PR başına
$0,004"). Kahraman rakam sonuç olur, maliyet onun altında durur.

Gerekçe ve ölçümler: [spec 004](specs/004-rapor/spec.md).

### Diyagramlar

`/nasil-calisir` sayfasındaki mimari ve akış diyagramları elle yazılmış SVG
(`components/docs/diagrams.tsx`) — grafik kütüphanesi yok, rapor grafikleriyle
aynı yaklaşım. Renkler `var(--color-*)` token'larından okunur, bu yüzden iki
temada da doğru görünürler ve tema denetiminden geçerler.

Dar ekranda diyagram küçülmez, **kaydırılır**: `minWidth` ile taban genişlik
verilir ve yalnızca çizim `overflow-x-auto` bir kaba sarılır. Kartın tamamını
kaydırmak, altındaki açıklama paragrafını da yatay kaydırırdı.

### Ayarlar ekranının düzeni

İki kural:

1. **Her ayar, ait olduğu şeyin yanında durur.** "Jira tarama aralığı" Jira
   erişiminin altında, "MCP süre sınırı" MCP sunucularının altında. Bütün
   davranış parametrelerini tek bir "Çalışma ayarları" yığınına toplamak,
   kullanıcının tek bir şey için sayfanın iki ayrı yerine bakması demekti.
2. **Bölümler alt alta değil, yan menüde.** Dizildiklerinde sayfa her yeni
   bölümle uzuyordu ve en alttaki neredeyse görünmüyordu. Yeni bölüm eklemek
   artık `TABS` listesine bir satır; kimsenin kaydırma mesafesi artmıyor.

`RuntimeSettings` bir `groups` süzgeci alır, böylece kayıt defterindeki bir grup
ait olduğu sekmede çizilir. Yeni bir ayar eklemek yine tek satır — ama grubunu
seçerken hangi sekmede görüneceğini de seçmiş olursunuz.

**Sekme adı değişirse belgelerdeki "Ayarlar → X" yolları da değişir.** Bu
projede bir kez, arayüzde karşılığı olmayan bir yol belgeye yazıldı (spec 010
Ölçüm 6).

### Grafikler

Rapor ekranındaki grafikler elle yazılmış SVG/CSS'tir; grafik kütüphanesi yok.
Uyulan kurallar:

- **Tek eksen.** İki farklı ölçek (çalıştırma sayısı ve dolar) asla aynı grafiğe
  konmaz — ikinci bir y ekseni olmayan bir ilişki uydurur. İki ayrı kart olur.
- **İki veya daha fazla seri varsa gösterge her zaman durur;** kimlik yalnızca
  renkle taşınmaz. Metin seri rengini giymez — renk işaretin, yazı `ink` jetonunun.
- **Durum renkleri** (`--color-chart-good|other|bad`) temaya göre DEĞİŞMEZ ve
  yalnızca sonuç anlatır; seri rengi olarak kullanılmazlar. Doğrulandı: en yakın
  çift, görme engeli benzetimi altında ΔE 11,3 (eşik 8), normal görüşte 27,6 (eşik 15).
- Sarı açık temada 3:1 kontrastın altındadır. Bu yüzden yığılmış grafiğin
  **tablo görünümü zorunludur** — sayıya renkten bağımsız bir yol her zaman açık.
- İşaret ölçüleri sabit: çubuk ≤24px, veri ucu 4px yuvarlak/taban kare, çizgi 2px,
  nokta r≥4 + 2px yüzey halkası, kılavuz çizgileri tek adım gri ve kesiksiz.
- Eksende her noktaya etiket konmaz; kaydı olmayan günler **atlanmaz**, sıfır olarak
  durur (atlanırsa zaman ekseni sıkışır ve trend yanlış okunur).

### Git

- Branch: `faz-N/kisa-aciklama` veya `spec/NNN-ozellik-adi`
- Commit: Conventional Commits — `feat(workflow): DAG topolojik sıralama ekle`
- Bir commit bir mantıksal değişiklik; formatlama ile davranış aynı commit'te karışmaz.

---

## Kritik Teknik Notlar

**opencode HTTP API** (runner container'ının içinde `:4096`) — opencode **1.18.15**'te
canlı doğrulandı:

| İşlem | Endpoint |
|-------|----------|
| Sağlık | `GET /global/health` → `{ healthy, version }` |
| Session aç | `POST /session` → `{ id }` |
| **Prompt gönder (senkron)** | `POST /session/:id/message` — yanıt dönene kadar bekler |
| Diff al | `GET /session/:id/diff` |
| İptal | `POST /session/:id/abort` |
| Olay akışı | `GET /event` (SSE) |
| Provider'lar | `GET /config/providers` |
| Agent listesi | `GET /agent` |
| Çalışma dizini VCS durumu | `GET /vcs/status`, `GET /vcs/diff/raw` |

`POST /session/:id/message` gövdesi:

```json
{
  "agent": "reviewer",
  "model": { "providerID": "openrouter", "modelID": "anthropic/claude-haiku-4.5" },
  "parts": [{ "type": "text", "text": "..." }]
}
```

Yanıtta `info.tokens`, `info.cost` ve `info.finishReason` gelir.

**`model` bir nesnedir, düz metin değil** — `{ providerID, modelID }`. `modelID` içinde
sağlayıcı öneki kalır: `anthropic/claude-haiku-4.5`. Tek parça `"openrouter/anthropic/..."`
metni yalnızca `opencode.json` ve agent frontmatter'ında kullanılır, API'de değil.

`model` ve `agent` **her mesajda** parametre olarak geçer. "Her node farklı modelle çalışsın"
gereksinimi bu sayede ekstra kod olmadan karşılanır.

**Maliyeti opencode hesaplar.** Yanıttaki `info.cost` (USD) ve `info.tokens`
(`input`/`output`/`reasoning`/`cache`) doğrudan `workflow_steps`'e yazılır — model
kataloğu fiyatlarından yeniden hesaplamaya gerek yok. `models` tablosu yalnızca model
seçimi ve arayüzde fiyat gösterimi için tutulur.

**`/api/...` namespace'ini kullanmayın.** opencode'un ikinci nesil API'si
(`POST /api/session/:id/prompt`) asenkrondur ve tamamlanmayı beklemek için gereken
`POST /api/session/:id/wait` 1.18.15'te **uygulanmamıştır**
(`ServiceUnavailableError: Session wait is not available yet`). Prompt kabul edilir ama
asistan yanıtı hiç üretilmez. Senkron `/session/:id/message` yolunu kullanın.

**Yapılandırılmış çıktı mümkün.** `POST /session/:id/message` gövdesi `format` alanıyla
JSON Schema kabul eder — `reviewer` agent'ının bulgu listesini serbest metin yerine
şemalı JSON olarak almak için Faz 6'da değerlendirilecek.

**Agent dizini `agents/` — çoğul.** opencode'un runtime loader'ı `.opencode/agents/`
okur. `opencode agent create` komutu tekil `.opencode/agent/` altına yazar ve oraya konan
dosyalar **sessizce yok sayılır** (opencode#14410). Tekil dizin kullanmayın.

**Runner'da config global yola konur.** `.opencode/opencode.json` ve `.opencode/agents/*`
runner imajında `/home/agent/.config/opencode/` altına kopyalanır — klonlanan repoya
**değil**. Repoya kopyalansaydı bizim dosyalarımız kullanıcının diff'inde görünürdü.
Kullanıcının kendi repo'sundaki `.opencode/` bunun üzerine biner.

**Agent md'lerinde model sabitlenmez.** `.opencode/agents/*.md` dosyalarında `model:`
satırı yoktur: modeli her zaman platform, mesaj başına belirler. Dosyaya sabit model
yazmak hem yanıltıcı olur hem de çoklu sağlayıcı desteğini kırar.

**Runner yapılandırması Faz 2'de üretilecek.** `.opencode/opencode.json` şu an
OpenRouter'a sabit; çoklu sağlayıcı desteğiyle birlikte bu dosya çalışma anında
seçilen sağlayıcıdan üretilecek (`provider.<slug>` + `baseURL` + `models`).

**Jira tetikleyicinin tekrar-işleme koruması VERİTABANINDADIR.** İki tetikleme yolu
(JQL taraması ve webhook) aynı anda gelebilir; "önce sor, sonra yaz" biçiminde bir uygulama
içi kontrol yarışa açıktır. Koruma `workflow_processed_issues` üzerindeki birincil anahtar +
`ON CONFLICT DO NOTHING`'tir; `RowsAffected() == 1` olan çağrı akışı başlatır. Anahtara
task'ın **güncellenme zamanı** dahildir: task değişirse yeniden işlenir.

**İşaret akıştan ÖNCE konur, hata halinde geri alınır.** Sıra tersine çevrilirse yarış geri
gelir; işaret geride bırakılırsa task hiç çalışmadan "işlendi" sayılır ve bir daha denenmez.
`UnmarkProcessed` yalnızca çalışmaya bağlanmamış işareti siler.

**Akışın Jira'ya yazdığı yorum kendisini tetiklemez.** Yorum, task'ın güncellenme zamanını
değiştirir; korumasız bırakılsaydı her tarama turunda yeni bir PR ve yeni bir maliyet
üretilirdi (ölçüldü — spec 009 Ölçüm 4). Yorum adımı, yazdıktan sonra task'ı yeniden okuyup
oluşan güncellemeyi kendi adına işaretler.

**Jira tetikleyicisi olan etkin bir akış sürekli tarar.** `jira.poll_interval_minutes`
(varsayılan 5, aralık 1–1440) her turda JQL'i çalıştırır; eşleşen ama daha önce işlenmiş
task hiçbir şey başlatmaz, yani boşta tarama bedava sayılır. Taramayı durdurmanın yolu
akışı duraklatmaktır (akış ekranında **Duraklat**).

**Duraklatma OTOMATİK tetiklemeyi kapatır, elle çalıştırmayı değil.** Pasif akış tarama
listesine girmez ve webhook uçları `409` döner; `POST /workflows/{id}/runs` çalışmaya devam
eder — kullanıcı düğmeye basıyorsa niyeti bellidir. `jira.scan_limit` (varsayılan 20, aralık 1–200) bir turda en fazla
kaç task'ın işleneceğini sınırlar — geniş bir JQL'in yüzlerce akış başlatmasını engeller.

İlk tarama sunucu açılışında değil, bir aralık sonra yapılır: her yeniden başlatmada
Jira'ya anında yüklenmek istemiyoruz.

**Jira Cloud API v3 hedeflenir.** Arama için `POST /rest/api/3/search/jql` (`nextPageToken`
ile sayfalama) kullanılır; eski `/rest/api/3/search` Ağustos 2025'ten beri 410 döner. Data
Center farklı kimlik doğrulama ve API yüzeyi kullanır, desteklenmiyor.

**MCP yapılandırması çalışma anında üretilir.** `runner.BuildConfigFiles` her çalıştırmada
`opencode.json`'a bir `mcp` bloğu yazıyor; imaja gömülü değil. Erişim anahtarı dosyaya
YAZILMAZ — `{env:AGENT_CODER_MCP_<AD>}` referansı yazılır, değer container ortamından gelir.
Sağlayıcı anahtarındaki desenin aynısı; iki sızıntı testi bunu koruyor.

**MCP sunucusu bağlanamazsa çalıştırma motoru SESSİZ KALIR.** Araçları modele hiç sunmuyor,
hata da vermiyor (ölçüldü — spec 011). Bu yüzden mesaj gönderilmeden önce motorun `GET /mcp`
ucu sorgulanıyor ve bağlanamayan sunucu olay akışına **uyarı** olarak düşüyor. Çalıştırma
başarısız sayılmaz: araç olmadan da iş bitebilir. Bu kontrolü kaldırmayın — arıza aksi halde
"agent neden aptallaştı" sorusuyla, günler sonra fark edilir.

**Araç adı `{sunucu}_{araç}`.** Sunucu adı bu yüzden dar bir karakter kümesiyle sınırlı
(harf, rakam, `-`, `_`): motor izin verilmeyen karakterleri alt çizgiye çeviriyor ve
kullanıcının yazdığı ad ile modelin gördüğü araç adı ayrışırdı.

**MCP sunucu handler'ı BİR KEZ kurulur.** `mcpserver.New` içinde üretilir ve paylaşılır.
İstek başına yeni handler üretmek oturum durumunu kaybettiriyor: MCP el sıkışması birden
fazla isteğe yayılıyor ve ikincisi "session not found" alıyor (ölçüldü — spec 011 Ölçüm 5).
Oturum başına yeniden kurulan şey handler değil, MCP *sunucusu*.

**Dışarıya açılan MCP ucu `/api` altında DEĞİL.** Webhook uçlarıyla aynı yerde durur çünkü
aynı güvenlik modelini paylaşır: kimlik doğrulama yok, adresin kendisi anahtardır. Anahtar
karşılaştırması sabit zamanlıdır (`subtle.ConstantTimeCompare`).

**MCP argümanları önce AYRIŞTIRILIR, sonra şablonlanır.** `mcp.call` düğümünün argümanları
bir JSON nesnesi ve içinde şablon var. Ters sırayla (önce şablon, sonra JSON) yapılsaydı
içinde tırnak olan bir agent çıktısı JSON'u bozardı — hem de yalnızca belirli çıktılarda.
`renderDeep` iç içe nesne ve dizilerde de string değerleri tek tek çözer.

**Yetki kuralı sıralaması DOĞRULANMADI.** Motorun kurallarında ilk mi son eşleşen mi kazanır
bilinmiyor; bu yüzden toptan bir "geri kalan yasak" kuralı yazılmadı. Erişim yapılandırmayla
sınırlanıyor. Ölçmeden beyaz liste kurmayın.

**Yetki desenleri GÜVENLİK SINIRI DEĞİLDİR.** Motorun `bash` yetkisi desen kabul ediyor ama
eşleşme **ham komut metnine** yapılıyor; bash ayrıştırması yok. `betik.sh` için izin veren bir
desen `betik.sh; env` komutunu da geçirir — ve `env` çıktısında `GIT_TOKEN` ile
`AGENT_CODER_PROVIDER_KEY` var. "Bash kapalı ama şu komuta izinli" diye bir mod kurmayın
(spec 012 K2). Yetki ikili kalır: ya bash var ya yok.

**Betikler yalnızca bash yetkisi AÇIK agent'lara kopyalanır** (spec 012 K3). Kapalıyken dosya
container'a hiç girmez — çalıştırılamayacak bir dosyanın orada durması, bir sonraki
geliştiriciyi "madem duruyor, izin de verelim" demeye davet eder. `BuildPermissions` bu
özellik için **hiç değişmedi**; "yeni yetenek açmıyor" iddiasının tek kanıtı bu, bir test de
bunu kilitliyor.

**Betikler `/home/agent/scripts/<ad>.sh`, mod `0o755`.** `/work` altına konamaz: orası
klonlama hedefi ve boş olmak zorunda, ayrıca bizim dosyalarımız kullanıcının diff'ine karışır.
Dizin imajda önceden açılır — tar kopyalaması dizin oluşturmuyor. Betik adı doğrudan dosya
adına dönüştüğü için dar (`[a-z0-9-]`) ve sessizce dönüştürülmez, baştan reddedilir.

**Yapılandırma eksiği 5xx DÖNMEZ.** Sağlayıcı tanımlı değil, git erişimi yok, model
seçilmemiş — bunlar arıza değil eksik ayar. `default` dalına düşüp 500 "internal_error"
verdiklerinde yeni kurulum yapan kullanıcı uygulamanın bozuk olduğunu sanıyor. Her biri
4xx ve **ne yapılacağını söyleyen** bir mesajla döner; bir test bu ayrımı kilitliyor
(`TestRespondRunError_YapilandirmaEksigi500Donmez`).

**Doğrulamada 404, KİMLİK hatası değil ADRES hatasıdır.** Sunucu "böyle bir uç yok"
diyorsa erişim bilgisine hiç bakmamıştır. `ErrInvalidSecret`'e eşlemek kullanıcıya
"anahtarın yanlış" dedirtir; o da doğru anahtarını boşuna yeniler — asıl sorun adres
ya da API şemasıdır. `ErrInvalidBaseURL` ile sarmalanır ve ikisiyle **birden**
sarmalanmaz (`respondGitError`'da `ErrInvalidSecret` dalı önce geliyor, yine kimlik
hatası raporlanırdı).

**Bitbucket Cloud ve Server tek tür, iki uçtur.** API şemaları farklı: Cloud `/2.0/user`,
Server `/rest/api/1.0/...`. Ayrım yeni bir sağlayıcı türüyle değil **adresle** yapılır —
kullanıcıya "hangisini seçmeliyim" sorusu sordurmaz. Karşılaştırma adresin **host'u**
üzerinden; ham metin araması yolunda `api.bitbucket.org` geçen kurumsal bir adresi
yanlışlıkla Cloud sayar.

**Paylaşılan hata bileşenleri tek bir ekranın diline göre yazılmaz.** `describeError`
bir zamanlar `invalid_base_url` için koşulsuz "Örnek: …/v1" ipucu veriyordu; o kod git
erişim formunda da kullanılınca kurumsal Bitbucket kullanıcısına adresinin sonuna `/v1`
eklemesini önerir hale geldi. İpucu artık bağlam alıyor (`error-hints.ts`) ve bağlam
bilinmiyorsa hiç yazılmıyor — yanlış ipucu, ipucu olmamasından kötüdür.

**OpenRouter zorunlu bir bağımlılık değildir.** LiteLLM ve OpenAI-uyumlu servisler
(vLLM, Azure OpenAI, Ollama) eşit desteklenir; kullanıcı arayüzünde ve belgelerde
OpenRouter varsayılan yolmuş gibi sunulmaz. Kurum içi kurulumlarda dışarıya hiç
çıkmadan çalışmak mümkün olmalı.

**Betik içeriği gizli değer değildir.** Şifrelenmez, arayüzde tam metin görünür. Container
içinde zaten düz metin duruyor ve agent okuyabiliyor; şifrelemek yanlış bir güvenlik hissi
verirdi. Gizli değer betiğe değil ortam değişkenine konur.

**compose `--project-directory` zorunlu.** Compose dosyaları `deploy/` altında ama `.env`
proje kökünde. Bu bayrak olmadan compose `.env`'i `deploy/` altında arar, bulamaz ve tüm
port ayarları **sessizce** varsayılana düşer. Makefile bunu zaten geçiriyor; compose'u elle
çağırırsanız siz de geçirin.

**Sandbox güvenliği:** Backend'e `/var/run/docker.sock` mount edilir (sibling-container deseni).
Runner container'ları izole network'te çalışır, dışarıya port açmaz, CPU/RAM limitlidir ve
iş bitince container + volume silinir. Git token'ı sadece env ile geçer, loglardan maskelenir.

---

## Portlar

Bu makinede 3000 / 5432 / 5433 başka projelerce kullanıldığından çakışmayan portlar seçildi.
Değerler `.env` üzerinden değiştirilebilir; container içi portlar sabittir.

| Servis | Host | Container |
|--------|------|-----------|
| frontend | 3002 | 3000 |
| backend | 8080 | 8080 |
| postgres | 5434 | 5432 |
| runner (opencode) | yayınlanmaz | 4096 |

## Ayarlar: ortam değişkeni mi, veritabanı mı?

Sınır nettir ve **kodda gömülü davranış parametresi bırakılmaz**:

| Nerede | Ne |
|--------|-----|
| Ortam değişkeni (`.env`) | Veritabanına bağlanmak için gerekenler ve **dağıtım topolojisi**: `DATABASE_URL`, `SECRET_ENCRYPTION_KEY`, portlar, `RUNNER_IMAGE`, `RUNNER_NETWORK`, `OPENCODE_SERVER_PASSWORD` |
| Veritabanı (`settings` tablosu) | **Davranış**: süre sınırı, eşzamanlılık, CPU/bellek, klonlama derinliği, talimat boyutu, katalog tazeleme aralığı, rapor dönemi ve saat dilimi, **Jira tarama aralığı ve tarama başına task sınırı** |

Mekanizma `internal/settings`:

- **Kod yalnızca TANIMI tutar** — `Registry` içindeki `Definition` satırı: anahtar, grup,
  etiket, açıklama, tip, varsayılan, aralık. **Veritabanı yalnızca SAPMALARI tutar.**
  Kullanıcının elle değiştirmediği bir ayarda yeni sürümün varsayılanı kendiliğinden geçerli olur.
- **Yeni parametre eklemek tek satırdır.** Migration gerekmez, frontend değişikliği gerekmez —
  Ayarlar ekranı listeyi kayıt defterinden çizer.
- Değerler bellekte önbelleklenir ve yazma anında tazelenir; **ayar değişikliği sunucuyu
  yeniden başlatmayı gerektirmez.** Bu yüzden sınırlar `Manager`'a değer olarak değil,
  her kullanımda okunan fonksiyon olarak geçilir.
- Doğrulama yazma anında yapılır; okuma tarafı bozuk değerde varsayılana düşer ve panik etmez.
- `.env` içinde bu parametrelerin karşılığı **yoktur** — olsaydı iki kaynak olur, `.env`'i
  değiştiren kullanıcı hiçbir etki görmezdi.

## Gizli değerler

Kimlik bilgileri üç tabloda tutulur — `llm_providers`, `git_providers`, `credentials` (Jira) —
ve hepsi veritabanında **AES-256-GCM ile şifreli** saklanır. Uyulması gereken kurallar:

- Şifreleme yalnızca `internal/secrets` içinde yapılır. Blob düzeni
  `[sürüm:1][nonce:12][şifreli+etiket]`; sürüm baytı ileride anahtar döndürmek için.
- `llm.Provider`, `gitprovider.Provider` ve `credentials.Credential` tipleri gizli değeri
  **taşımaz**. Erişimin tek yolu her paketteki `Store.Reveal` — adı bilinçli olarak
  dikkat çekicidir.
- Gizli değer log'a, hata mesajına veya HTTP yanıtına **hiçbir koşulda** konmaz.
  Dışarı yalnızca `hint` (son 4 karakter) çıkar.
- `SECRET_ENCRYPTION_KEY` kaybolursa kayıtlı tüm kimlik bilgileri çözülemez.
  `make env` bunu üretir; **yedekleyin**.
- Kimlik bilgileri kaydedilmeden **önce** doğrulanır (ilgili servise gerçek bir istek
  atılarak). Geçersiz değer veritabanına hiç girmez.
- Sağlayıcılar arayüzden tanımlanır. `.env`'deki `OPENROUTER_API_KEY` yalnızca bir
  kolaylıktır: `llm_providers` tablosu **tamamen boşsa** açılışta ondan bir OpenRouter
  sağlayıcısı oluşturulur (bootstrap). Kullanıcı sağlayıcıyı silip yeniden başlatırsa
  geri gelir — istemeyen değişkeni boşaltır.

## Durum

**Betikler tamamlandı** ([spec 012](specs/012-betikler/spec.md)):

- **Prosedür işleri artık doğaçlanmıyor** — Ayarlar'da merkezî bir betik kütüphanesi;
  bir kez yazılan betik birden fazla agent'a atanır. Model **ne zaman** çağıracağına
  karar verir, **ne yapacağına** betik karar verir
- **Tek yerden güncelleme** — içerik çalıştırma anında okunur, imaj yeniden derlenmez
- **Güvenlik deltası sıfır** — betikler yalnızca bash yetkisi zaten açık agent'lara
  gider; o agent betiği bugün de kendisi yazıp çalıştırabiliyordu. `BuildPermissions`
  hiç değişmedi
- **Reddedilen fikir kayıtlı** — "bash kapalı ama şu betiğe izinli" modu gerçek bir açık
  olduğu için düşürüldü (spec 012 K2), ertelenmedi
- İki gerçek çalıştırmayla doğrulandı: yetkili agent betiği çağırdı ve çıktısını verdi;
  yetkisiz agent talimatında betikleri hiç görmedi

**MCP Aşama 1 tamamlandı** ([spec 011](specs/011-mcp/spec.md)):

- **Agent'lar dış araçlara erişiyor** — Ayarlar'dan uzak MCP sunucusu tanımlanır,
  Agent'lar ekranından hangi agent'ın kullanacağı seçilir
- **Kaydetmeden önce doğrulama** — sunucuya bağlanılır, araç listesi çekilip gösterilir
- **Yalıtım ölçüldü** — atanmamış agent aracı göremiyor
- **Sessiz başarısızlık kapatıldı** — bağlanamayan sunucu uyarı üretiyor
- Gerçek bir MCP sunucusuyla uçtan uca doğrulandı (`deepwiki_read_wiki_structure` çağrıldı)

**MCP Aşama 2 tamamlandı:** tuvale `mcp.call` düğümü eklendi. Araç listeden seçilir,
argümanlar şablonlanabilir, çıktı sonraki adıma geçer. Yanlış araç adı ve bozuk JSON
**kaydetme anında** reddedilir.

**MCP Aşama 3 tamamlandı:** Agent Coder'ın kendisi bir MCP sunucusu. Claude Desktop veya
Cursor akışları listeleyip başlatabiliyor; başlatma mevcut `Launcher`'dan geçiyor, dördüncü
bir yol açılmadı. Adres Ayarlar'da, kopyalanabilir kurulum örneğiyle birlikte.

**Arayüz denetimi tamamlandı** ([spec 010](specs/010-arayuz-denetimi/spec.md)):

- **Telefonda kullanılabilir** — kenar çubuğu çekmeceye dönüşüyor; eskiden içeriğe
  ~175px kalıyor ve "Kaydet" ekranın dışında kalıyordu
- **Açılış ekranı kullanıcıya konuşuyor** — kurulum kontrol listesi ya da son
  durum; geliştirme yol haritası kaldırıldı
- **İki tema ölçülerek eşitlendi** — 346 kontrol, 0 kalan, 0 eşlik hatası
- **Denetim sınırı token'ı** (`control-line`) — düğme ve girdi kenarları artık
  sınırlarından tanınıyor
- **Her liste ekranında sayfalama** — ortak zarf ve ortak bileşen
- Etiketsiz alan, adsız düğme kalmadı

**Faz 5 tamamlandı** ([spec 009](specs/009-jira-ve-depo-dugumleri/spec.md)):

- **Jira task'ından akış** — tuvaldeki başlangıç düğümü "Jira task'ı"na çevrilir,
  JQL yazılır; eşleşen her task akışı bir kez başlatır
- **İki tetikleme yolu, tek koruma** — tarama ve webhook aynı başlatma yolundan
  ve aynı "işlendi" kaydından geçer
- **PR açma ve Jira yorumu düğüm olarak** — başlık/gövde önceki adımların
  çıktısından şablonlanır; bu düğümler model çağırmaz, maliyetleri sıfırdır
- Gerçek Jira + gerçek depo ile uçtan uca ölçüldü: `SCRUM-2` → agent → push →
  PR #4 → Jira yorumu, $0.0014

**Faz 4 tamamlandı** ([spec 008](specs/008-tuval-editoru/spec.md)):

- **Tuval editörü** — akış çizilerek kuruluyor: adım ekle, taşı, bağ çek, sil;
  sağ panelde agent + model + talimat
- **Dallanma ve birleşme** arayüzden kurulabiliyor; motor paralel çalıştırıyor
  (ölçüldü: tuvalden kurulan akışta 10 sn örtüşme)
- **Konumlar kaydediliyor**; konumsuz eski akışlar seviyeye göre yerleştiriliyor
- **Döngü çizim anında engelleniyor**; doğrulama kusurları ilgili düğümde görünüyor
- **Canlı izleme tuvalde** — adımlar durumlarına göre renkleniyor (renk + etiket)
- Backend'e HİÇ dokunulmadı: graf modeli Faz 3'te tuval düşünülerek kurulmuştu

**Faz 3 tamamlandı** ([spec 007](specs/007-workflow-motoru/spec.md)):

- **Akışlar** — adımları birbirine bağlı, kaydedilebilir workflow'lar; her adım
  kendi agent'ı ve kendi modeliyle
- **Şablon** — `{{ input }}`, `{{ trigger.<alan> }}`, `{{ steps.<adım>.output|diff|branch }}`;
  bilinmeyen referans sessizce boşa düşmez, **kaydetme anında** reddedilir
- **Paralellik** — bağımsız adımlar aynı anda çalışır (ölçüldü: 5 sn örtüşme)
- **Sürümleme** — kaydedilen graf değişmez; geçmiş çalışma hangi tanımla
  çalıştığını doğru gösterir
- **Tetikleme** — elle veya dışarıdan (adres anahtardır, yenilenebilir)
- Rapor akış adımlarını **kod değişikliği olmadan** kapsıyor: adım = çalıştırma

**Faz 2 çalışıyor** ([spec 003](specs/003-agent-calistirma/spec.md) +
[spec 004](specs/004-rapor/spec.md)):

- **Projeler** — depo tanımı, `git ls-remote` ile erişim doğrulaması
- **Agent tanımları** — beş hazır agent tohumlanıyor, düzenlenebiliyor, sıfırlanabiliyor;
  kullanıcı kendi agent'ını oluşturabiliyor
- **Çalıştırma** — iş başına geçici container, canlı SSE olay akışı, çıktı + diff + dosya
  özeti + token + maliyet, iptal, branch'e gönderme
- **Ayarlar** — yedi çalışma parametresi + iki rapor parametresi arayüzden yönetiliyor,
  değişiklik yeniden başlatmadan geçerli oluyor
- **Rapor** — dönem özeti: kahraman maliyet rakamı, başarı oranı, üretilen değişiklik,
  günlük çalıştırma/maliyet grafikleri, agent/model/proje kırılımı, tekrar eden hatalar
- Doğrulandı: çalıştırma sonrası artık container/volume kalmıyor; eşzamanlılık sınırı
  çalışma anında değişiyor; rapor rakamları çalıştırma listesiyle tutuyor

**Faz 1 tamamlandı** ([spec 001](specs/001-veri-katmani-ve-model-katalogu/spec.md) +
[spec 002](specs/002-coklu-saglayici/spec.md)):

- PostgreSQL şeması (`llm_providers`, `git_providers`, `models`, `provider_sync`,
  `credentials`), migration'lar açılışta uygulanıyor
- **Çoklu LLM sağlayıcı:** OpenRouter, LiteLLM proxy, OpenAI-uyumlu servisler; aynı anda
  birden fazlası tanımlı olabilir, biri düşerse diğerleri etkilenmez
- **Çoklu git sağlayıcı:** GitHub (token), Bitbucket (kullanıcı adı + parola), genel Git
- Kimlik bilgileri şifreli saklanıyor, kaydetmeden önce doğrulanıyor
- Model kataloğu sağlayıcı bazında, açılışta ve günde bir indiriliyor
- Ayarlar ve Modeller ekranları çalışıyor — sağlayıcı filtresi, arama, sıralama, sayfalama
- Doğrulandı: veritabanında düz metin yok, loglarda anahtar yok, yeniden başlatmada veri
  duruyor, 001 verisi 002 migration'ıyla kaybolmadan taşınıyor

**Faz 0 tamamlandı** — iskelet ayakta ve doğrulandı:

- `make up` → postgres, backend, frontend üçü de healthy
- `GET :8080/health` → `{"status":"ok","version":"dev","env":"development"}`
- Arayüz `:3002` üzerinden backend'e bağlanıyor
- `make runner` → opencode 1.18.15 headless çalışıyor, repo klonlama doğrulandı
- Beş ürün agent'ı (analyst, coder, reviewer, tester, upgrader) opencode tarafından tanınıyor
- `make test` → 10 Go testi geçiyor; `make lint` temiz
- **Uçtan uca gerçek model çağrısı doğrulandı:** runner → clone → `opencode serve` →
  session → `reviewer` agent'ı + `claude-haiku-4.5` → model repo'yu okudu, özet döndü,
  6.333 token / $0,00085 raporlandı. Faz 2'nin çekirdeği çalışıyor.

Sıradaki: **Faz 5 — Jira ve kod deposu entegrasyonları.** Jira'dan task çekme,
PR açma, issue'ya yorum yazma.
Faz listesi ve doğrulama adımları:
[plans/01-mimari-ve-yol-haritasi-2026-08-09.md](plans/01-mimari-ve-yol-haritasi-2026-08-09.md)
