# Plan: Projeler, Agent Tanımları ve Agent Çalıştırma

- **Spec no:** 003 — [spec.md](spec.md)
- **Tarih:** 2026-08-09
- **Durum:** İnceleme — onay bekliyor

---

## Yaklaşım

Beş katman:

1. **Ayarlar** — davranış parametreleri veritabanından okunur (H7). Önce gelir çünkü
   sonraki katmanların hepsi parametrelerini buradan alacak.
2. **`Runner` arayüzü ve opencode uygulaması** — sistemin en kritik sınırı ve en riskli
   parçası. Faz 0'da çalıştığını kanıtladığımız akış buraya taşınır, arkasına arayüz konur.
3. **Tanımlar** — `projects` ve `agents` tabloları, CRUD.
4. **Çalıştırma yönetimi** — eşzamanlılık sınırı, iptal, zaman aşımı, olay yayını.
5. **Arayüz** — ayarlar, projeler, agent'lar, çalıştırma ve canlı izleme ekranları.

### Ayarlar: kayıt defteri yaklaşımı

Spec H7 tek tek birkaç alan değil, bir **kural** koyuyor: davranışı belirleyen hiçbir
parametre kodda gömülü kalmaz. Bunu her ayar için ayrı kolon açarak çözmek, her yeni
parametrede migration + backend + frontend değişikliği demek olurdu.

Bunun yerine **kod tarafında tanımlı bir kayıt defteri, veritabanında yalnızca sapmalar**:

```
settings tablosu:  key → value   (yalnızca varsayılandan FARKLI olanlar)
Go registry:       key, grup, etiket, açıklama, tip, varsayılan, min/max, birim
Arayüz:            kayıt defterinden KENDİNİ ÇİZER
```

Böylece yeni bir parametre eklemek tek satırlık bir kayıt defteri girdisi olur — migration
da, frontend değişikliği de gerekmez. Doğrulama ve varsayılan tek yerde durur.

**Ortam değişkeni mi, veritabanı mı?** Sınır net çizilir:

| Nereye | Neden | Örnek |
|---|---|---|
| Ortam değişkeni | Veritabanına *bağlanmak için* gerekenler ve dağıtım topolojisi | `DATABASE_URL`, `SECRET_ENCRYPTION_KEY`, portlar, `RUNNER_IMAGE`, `RUNNER_NETWORK` |
| Veritabanı (ayarlar) | Çalışma davranışını belirleyenler | süre sınırı, eşzamanlılık, CPU/bellek limiti, klonlama derinliği, katalog yenileme aralığı |

`.env`'deki `RUNNER_TIMEOUT_SEC`, `RUNNER_MAX_CONCURRENCY`, `RUNNER_CPU_LIMIT`,
`RUNNER_MEMORY_LIMIT` **kaldırılır** — iki kaynak olması, hangisinin geçerli olduğu
belirsizliğini yaratır. (Faz 1'de `.env`/veritabanı ikiliğinin nasıl karıştığını gördük.)
`catalog.SyncInterval` sabiti de ayara dönüşür.

### Bu spec'i şekillendiren ölçümler

Tasarım varsayıma değil, çalışan opencode 1.18.15'e karşı yapılan ölçümlere dayanıyor:

| Ölçüm | Sonuç | Plana etkisi |
|---|---|---|
| Mesajda `system` alanı | **Yok sayılıyor** — model kendi varsayılan prompt'uyla yanıt verdi | Agent prompt'u mesajla gönderilemez |
| Dosyadaki agent tanımı | **Uygulanıyor** | Prompt dosya olarak yazılmalı |
| opencode başladıktan sonra yazılan dosya | **Okunmuyor** | Dosyalar `opencode serve` **öncesinde** yazılmalı |
| `POST /session` `permission` alanı | **Kabul ediliyor** (`{permission, pattern, action}`) | Yetkiler session açılışında gönderilir, dosyaya yazılmaz |
| `GET /session/:id/diff` | Var | Diff için git komutu çalıştırmaya gerek yok |
| Yanıttaki `info.cost` / `info.tokens` | Geliyor | Maliyet doğrudan kaydedilir |

**Sonuç:** Her çalıştırma için container **oluşturulur ama başlatılmaz**, yapılandırma ve
agent tanımı içine kopyalanır, sonra başlatılır. Her iş yeni container aldığı için bu
kısıt bize engel değil.

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
|---|---|---|---|
| Agent prompt'unu mesajın `system` alanıyla göndermek | En temiz | **Çalışmıyor** (ölçüldü) | Elendi |
| Tanımları env değişkeniyle geçip entrypoint'in yazması | Basit | Prompt `docker inspect`'te ve süreç ortamında görünür; büyük prompt env sınırına takılabilir | Elendi |
| Container'ı başlatmadan içine kopyalamak (`CopyToContainer`) | Prompt env'de görünmez, boyut sınırı yok | Bir adım fazla | **Seçildi** |
| Çalıştırma kaydında agent'a referans tutmak | Az veri | Agent sonradan düzenlenirse geçmiş yanlış görünür | Elendi |
| Çalıştırma kaydında prompt ve modeli **kopyalamak** | Geçmiş her zaman doğru | Biraz tekrar veri | **Seçildi** |
| Olayları yalnızca bellekte tutmak | Basit | Sayfa yenilenince ilerleme kaybolur (spec H4 ihlali) | Elendi |
| Olayları veritabanına yazıp SSE ile de yayınlamak | Yenilemeye dayanır | Yazma yükü | **Seçildi** |

### Yapısal değişiklik: ürün agent'ları backend'e taşınıyor

`.opencode/agents/*.md` şu an runner imajına gömülü. Agent tanımları artık **veritabanından**
geldiği için bu dosyaların runner'da olmasının anlamı kalmıyor.

- Beş hazır agent `backend/internal/agentreg/builtin/*.md` altına taşınır ve Go ikilisine
  `embed.FS` ile gömülür. Açılışta veritabanına tohumlanırlar.
- `runner/Dockerfile` artık agent dosyası kopyalamaz.
- `.opencode/opencode.json` **kalır** — bu depoyu opencode ile geliştirirken kullanılıyor.

Böylece "agent tanımı nerede yaşıyor" sorusunun tek cevabı olur: veritabanı; kaynağı ise
gömülü hazır tanımlar.

---

## Veri Modeli

`backend/internal/db/migrations/000003_calistirma.sql`

```sql
-- +goose Up
CREATE TYPE run_status AS ENUM (
    'pending', 'running', 'succeeded', 'failed', 'cancelled', 'timeout', 'interrupted');
CREATE TYPE agent_source AS ENUM ('builtin', 'custom');

CREATE TABLE projects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    repo_url        TEXT NOT NULL,
    default_branch  TEXT NOT NULL DEFAULT 'main',
    -- Açık depolar için erişim gerekmez; silinen erişim projeyi düşürmez.
    git_provider_id UUID REFERENCES git_providers(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE agents (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         TEXT NOT NULL UNIQUE,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    prompt       TEXT NOT NULL,
    source       agent_source NOT NULL,
    -- Hazır agent'ların özgün hali; "sıfırla" bunları geri yükler.
    builtin_prompt      TEXT,
    builtin_description TEXT,
    default_provider_id UUID REFERENCES llm_providers(id) ON DELETE SET NULL,
    default_model       TEXT NOT NULL DEFAULT '',
    -- Kaba yetkiler; opencode'a session açılışında permission kuralı olarak gider.
    allow_edit     BOOLEAN NOT NULL DEFAULT true,
    allow_bash     BOOLEAN NOT NULL DEFAULT true,
    allow_webfetch BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE runs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- Agent silinemez eğer çalıştırması varsa: geçmiş kime ait olduğunu bilmeli.
    agent_id   UUID NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
    provider_id UUID REFERENCES llm_providers(id) ON DELETE SET NULL,

    -- ANLIK KOPYA: agent veya model sonradan değişse bile geçmiş doğru kalır.
    agent_slug    TEXT NOT NULL,
    agent_prompt  TEXT NOT NULL,
    provider_slug TEXT NOT NULL,
    model_id      TEXT NOT NULL,

    branch TEXT NOT NULL,
    task   TEXT NOT NULL,

    status run_status NOT NULL DEFAULT 'pending',
    error  TEXT,
    output TEXT NOT NULL DEFAULT '',
    diff   TEXT NOT NULL DEFAULT '',

    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd          NUMERIC(12,6) NOT NULL DEFAULT 0,

    pushed_branch TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ
);

CREATE INDEX idx_runs_project ON runs (project_id, created_at DESC);
CREATE INDEX idx_runs_status  ON runs (status) WHERE status IN ('pending', 'running');

-- Yalnızca VARSAYILANDAN SAPAN ayarlar tutulur. Varsayılanlar ve geçerli aralıklar
-- Go tarafındaki kayıt defterinde durur; buraya yazılmazlar ki kod güncellendiğinde
-- yeni varsayılanlar kendiliğinden geçerli olsun.
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE run_events (
    run_id  UUID    NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    seq     INTEGER NOT NULL,
    ts      TIMESTAMPTZ NOT NULL DEFAULT now(),
    level   TEXT    NOT NULL,
    message TEXT    NOT NULL,
    PRIMARY KEY (run_id, seq)
);

-- +goose Down
DROP TABLE run_events;
DROP TABLE runs;
DROP TABLE agents;
DROP TABLE projects;
DROP TABLE settings;
DROP TYPE agent_source;
DROP TYPE run_status;
```

**`runs` neden kopya tutuyor:** Kullanıcı bir agent'ın prompt'unu düzenlediğinde geçmişteki
çalıştırmalar "hangi talimatla çalışmıştı" sorusunu doğru cevaplayabilmeli. Referans
tutulsaydı geçmiş sessizce yanlış görünürdü.

**`interrupted` durumu:** Sunucu açılışında `pending` ve `running` kalan kayıtlar bu duruma
çekilir — spec'in "sonsuza kadar çalışıyor görünmez" kuralı.

---

## Arayüzler

### `internal/settings` — parametre kayıt defteri

```go
// Definition, bir ayarın kod tarafındaki tanımı. Arayüz kendini bundan çizer.
type Definition struct {
    Key     string   // "runner.timeout_minutes"
    Group   string   // "runner" | "catalog"
    Label   string   // "Çalışma süre sınırı"
    Help    string   // ne işe yaradığı
    Kind    Kind     // KindInt | KindString | KindBool
    Default string
    Unit    string   // "dakika", "iş", "saat", "GB"
    Min     *int     // KindInt için
    Max     *int
}

// Registry, tanımlı tüm ayarlar. Yeni parametre = buraya tek satır.
var Registry = []Definition{
    {Key: "runner.timeout_minutes", Group: "runner", Kind: KindInt,
     Label: "Çalışma süre sınırı", Unit: "dakika",
     Help: "Bir agent çalıştırması bu süreyi aşarsa durdurulur ve 'zaman aşımı' sayılır.",
     Default: "30", Min: p(1), Max: p(240)},

    {Key: "runner.max_concurrent", Group: "runner", Kind: KindInt,
     Label: "Aynı anda çalışabilecek iş", Unit: "iş",
     Help: "Sınır doluyken başlatılan yeni işler reddedilir.",
     Default: "3", Min: p(1), Max: p(20)},

    {Key: "runner.cpu_limit",    Group: "runner", Kind: KindInt, Unit: "çekirdek", Default: "2",  Min: p(1), Max: p(32)},
    {Key: "runner.memory_limit_gb", Group: "runner", Kind: KindInt, Unit: "GB",   Default: "4",  Min: p(1), Max: p(64)},
    {Key: "runner.clone_depth",  Group: "runner", Kind: KindInt, Unit: "commit",  Default: "1",  Min: p(1), Max: p(1000)},
    {Key: "runner.max_prompt_kb", Group: "runner", Kind: KindInt, Unit: "KB",     Default: "32", Min: p(1), Max: p(256)},

    {Key: "catalog.sync_interval_hours", Group: "catalog", Kind: KindInt, Unit: "saat",
     Label: "Model kataloğu yenileme aralığı",
     Help: "Katalog açılışta bir kez, sonra bu aralıkla kendiliğinden tazelenir.",
     Default: "24", Min: p(1), Max: p(720)},
}

// Service, ayarları okur ve yazar. Okuma sık yapılacağı için bellekte önbelleklenir.
type Service struct{ /* pool + RWMutex korumalı cache */ }

func (s *Service) Load(ctx context.Context) error          // açılışta bir kez
func (s *Service) Int(key string) int                      // önbellekten, kilitsiz hızlı
func (s *Service) Set(ctx context.Context, key, value string) error  // doğrular + önbelleği tazeler
func (s *Service) Reset(ctx context.Context, key string) error       // satırı siler → varsayılana döner
func (s *Service) All() []Value                            // arayüz listesi (değer + varsayılan + sapma bayrağı)

var (
    ErrUnknownKey  = errors.New("bilinmeyen ayar")
    ErrOutOfRange  = errors.New("değer izin verilen aralıkta değil")
    ErrInvalidType = errors.New("değer bu ayarın tipine uymuyor")
)
```

**Değişiklik yeniden başlatma gerektirmez** (spec H7): tüketiciler değeri *kullanım anında*
okur, açılışta bir kez okuyup saklamaz. İki yer buna özel dikkat ister:

- **Eşzamanlılık sınırı:** sabit boyutlu kanal semaforu değişen sınıra uyum sağlamaz.
  Yerine sayaç + `sync.Cond` kullanılır; sınır her kontrolde ayardan okunur.
  Sınır düşürülürse çalışan işler kesilmez, yalnızca yeni işler reddedilir.
- **Katalog yenileme aralığı:** sabit `time.Ticker` yerine her turda ayardan okunan
  `time.After` ile döngü.

### `internal/runner` — sistemin en önemli sınırı

```go
// runner.go — opencode'a dair hiçbir tip bu dosyada geçmez.
type Runner interface {
    Run(ctx context.Context, req Request, emit EventFunc) (*Result, error)
}

type EventFunc func(Event)

type Request struct {
    RunID    uuid.UUID
    Repo     RepoSpec
    Agent    AgentSpec
    Provider ProviderSpec
    Model    string
    Task     string
    Timeout  time.Duration
}

type RepoSpec struct {
    URL      string
    Branch   string
    Username string // boşsa kimlik doğrulamasız klonlama
    Secret   string
}

type AgentSpec struct {
    Slug          string
    Description   string
    Prompt        string
    AllowEdit     bool
    AllowBash     bool
    AllowWebfetch bool
}

type ProviderSpec struct {
    Slug    string // opencode yapılandırmasındaki provider kimliği
    Kind    string // openrouter | litellm | openai_compatible
    BaseURL string
    APIKey  string
}

type Result struct {
    Output          string
    Diff            string
    PromptTokens    int
    CompletionTokens int
    CostUSD         float64
}

type Event struct {
    Level   string // info | warn | error
    Message string
}

var (
    ErrTimeout    = errors.New("çalışma süre sınırını aştı")
    ErrCancelled  = errors.New("çalışma iptal edildi")
    ErrRepoAccess = errors.New("depoya erişilemedi")
    ErrModel      = errors.New("model çağrısı başarısız")
)
```

`internal/runner/opencode` bu arayüzü uygular; `internal/runner/sandbox` Docker yaşam
döngüsünü yönetir. **Bu iki paket dışında hiçbir yerde "opencode" kelimesi geçmez.**

### Bir çalıştırmanın akışı

```
 1. Yapılandırma üretilir:
      opencode.json  ← sağlayıcıdan (provider.<slug>, baseURL, apiKey)
      <slug>.md      ← agent tanımından (description + prompt)
 2. docker create   (BAŞLATILMAZ)  — izole ağ, cpu/mem limiti, adlandırılmış volume
 3. CopyToContainer → /home/agent/.config/opencode/{opencode.json,agents/<slug>.md}
 4. docker start    — entrypoint repo'yu klonlar, opencode serve başlatır
 5. GET  /global/health          (hazır olana kadar bekle, timeout'lu)
 6. GET  /event  (SSE)           → emit(Event) → veritabanı + frontend SSE
 7. POST /session                { permission: agent yetkilerinden üretilir }
 8. POST /session/:id/message    { agent, model, parts }
 9. GET  /session/:id/diff
10. defer: container + volume silinir — HER YOLDA (hata, panik, iptal, zaman aşımı)
```

Adım 10 iptal edilmiş context ile çalışmaz; kendi `context.WithoutCancel` + timeout'unu kullanır.

**Yetki eşlemesi** (agent alanları → opencode permission kuralları):

| Agent alanı | Kural |
|---|---|
| `allow_edit=false` | `{edit, *, deny}`, `{write, *, deny}` |
| `allow_bash=false` | `{bash, *, deny}` |
| `allow_webfetch=false` | `{webfetch, *, deny}` |

Ayrıca her çalıştırmada `{question, *, deny}` ve `{plan_enter, *, deny}` gönderilir:
insan etkileşimi bekleyen bir agent, kimsenin izlemediği bir sandbox'ta sonsuza kadar bekler.

### `internal/runs` — çalıştırma yönetimi

```go
type Manager struct { /* store, runner, semaphore, aktif iptaller */ }

// Start, çalıştırmayı kaydeder ve arka planda başlatır. İstek yanıtını BEKLETMEZ.
func (m *Manager) Start(ctx context.Context, in StartInput) (Run, error)

// Cancel, çalışan bir işi durdurur.
func (m *Manager) Cancel(ctx context.Context, id uuid.UUID) error

// RecoverInterrupted, açılışta yarım kalmış kayıtları 'interrupted' yapar.
func (m *Manager) RecoverInterrupted(ctx context.Context) (int, error)

// Shutdown, çalışan işleri iptal eder ve temizlenmelerini bekler.
func (m *Manager) Shutdown()

var ErrTooManyRuns = errors.New("eşzamanlı çalıştırma sınırı doldu")
```

Eşzamanlılık: `RUNNER_MAX_CONCURRENCY` (varsayılan 3) boyutunda kanal tabanlı semafor.
Slot yoksa **beklemez, reddeder** — kullanıcı kaç iş çalıştığını görür (spec H3).

### `internal/events` — olay yayını

```go
type Bus struct{ /* run_id → aboneler */ }

func (b *Bus) Publish(runID uuid.UUID, e Event)
func (b *Bus) Subscribe(runID uuid.UUID) (<-chan Event, func())
```

Olaylar hem `run_events` tablosuna yazılır hem bus'a yayınlanır. SSE bağlantısı önce
tablodaki geçmişi gönderir, sonra canlı akışa geçer — sayfa yenilense de ilerleme kaybolmaz.

### HTTP API

| Metot | Yol | Not |
|---|---|---|
| GET/POST | `/api/projects` | oluştururken depo erişimi **doğrulanır** |
| PUT/DELETE | `/api/projects/{id}` | silme, bağlı çalıştırma sayısını yanıtta bildirir |
| GET/POST | `/api/agents` | |
| PUT/DELETE | `/api/agents/{id}` | hazır agent silinemez → 409 |
| POST | `/api/agents/{id}/reset` | hazır agent'ı özgün haline döndürür |
| GET | `/api/runs` | `project`, `status`, `limit`, `offset` |
| POST | `/api/runs` | `{projectId, agentId, providerId?, model?, branch?, task}` → 201 · 429 sınır dolu |
| GET | `/api/runs/{id}` | |
| GET | `/api/runs/{id}/events` | **SSE** — önce geçmiş, sonra canlı |
| POST | `/api/runs/{id}/cancel` | 204 · 409 zaten bitmiş |
| POST | `/api/runs/{id}/push` | `{branch?}` → 200 · 409 zaten gönderilmiş · 412 git erişimi yok |
| GET | `/api/settings` | kayıt defteri + mevcut değerler + sapma bayrağı |
| PUT | `/api/settings/{key}` | `{value}` → 200 · 400 aralık dışı · 404 bilinmeyen anahtar |
| DELETE | `/api/settings/{key}` | varsayılana döndürür → 200 |

**Depo erişimi doğrulaması:** `git ls-remote` çalıştırmak yerine runner imajıyla kısa ömürlü
bir container açmak pahalı olurdu. Bunun yerine backend imajına `git` eklenir ve
`git ls-remote --heads <url>` 15 saniyelik timeout'la çalıştırılır. Kimlik bilgisi
`GIT_ASKPASS` ile geçilir, komut satırına **yazılmaz**.

### Frontend

```
/projects            proje listesi + ekle/düzenle/sil
/agents              agent listesi + düzenle/oluştur/sıfırla + "Çalıştır"
/runs                çalıştırma geçmişi
/runs/[id]           canlı izleme: durum, olay akışı, çıktı, diff, maliyet, iptal, push
```

Çalıştırma formu `/agents` üzerinden açılır: proje, branch, model, görev metni.
Model seçicide araç desteği olmayan ve **bilinmeyen** modeller uyarı rozetiyle işaretlenir.

Ayarlar ekranına "Çalışma ayarları" bölümü eklenir. Bu bölüm **kayıt defterinden kendini
çizer**: her ayar için etiket, açıklama, birim ve aralık backend'den gelir. Yeni bir
parametre eklendiğinde frontend'e dokunmak gerekmez — spec H7'nin asıl kazancı bu.

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
|---|---|
| `backend/internal/db/migrations/000003_calistirma.sql` | yeni |
| `backend/internal/settings/{registry.go,service.go}` | yeni — parametre kayıt defteri |
| `backend/internal/runner/{runner.go,config.go}` | yeni — arayüz ve yapılandırma üretimi |
| `backend/internal/runner/sandbox/docker.go` | yeni — create/copy/start/wait/logs/rm |
| `backend/internal/runner/opencode/{client.go,runner.go}` | yeni — HTTP istemcisi ve arayüz uygulaması |
| `backend/internal/agentreg/{store.go,builtin.go,builtin/*.md}` | yeni — CRUD + gömülü hazır agent'lar |
| `backend/internal/projects/{store.go,verify.go}` | yeni |
| `backend/internal/runs/{store.go,manager.go,push.go}` | yeni |
| `backend/internal/events/bus.go` | yeni |
| `backend/internal/httpapi/{projects,agents,runs,sse}.go` | yeni handler'lar |
| `backend/internal/httpapi/router.go` | düzenleme |
| `backend/cmd/server/main.go` | düzenleme — manager, recover, shutdown |
| `backend/internal/config/config.go` | düzenleme — davranış `RUNNER_*` değişkenleri **kaldırılır**, ayarlara taşınır |
| `backend/internal/catalog/syncer.go` | düzenleme — sabit `SyncInterval` yerine ayardan okuma |
| `.env.example` | düzenleme — davranış parametreleri çıkarılır, neden çıkarıldığı yazılır |
| `backend/Dockerfile` | düzenleme — `git` eklenir |
| `runner/{Dockerfile,entrypoint.sh}` | düzenleme — agent dosyaları kopyalanmaz, config dışarıdan gelir |
| `.opencode/agents/*.md` | **taşınır** → `backend/internal/agentreg/builtin/` |
| `frontend/src/app/{projects,agents,runs}/**` | yeni ekranlar |
| `frontend/src/components/settings/RuntimeSettings.tsx` | yeni — kayıt defterinden kendini çizen bölüm |
| `frontend/src/lib/{types.ts,api.ts,sse.ts}` | düzenleme + yeni SSE yardımcısı |

### Yeniden kullanılacak mevcut kod

- `internal/llm.Store.Reveal` — sağlayıcı anahtarı buradan alınır, yeni bir yol açılmaz.
- `internal/gitprovider.Store.Reveal` — depo kimlik bilgisi.
- `internal/catalog.Store.List` — model seçicisi ve "araç desteği bilinmiyor" uyarısı.
- `internal/secrets` — yeni gizli değer eklenmiyor; mevcut şifreleme kullanılır.
- `httpapi.respondJSON/respondError/parseUUIDParam/Deps` — yeni handler'lar aynı desende.
- `runner/entrypoint.sh`'ın klonlama ve kimlik maskeleme mantığı — korunur, yalnızca
  yapılandırma yazma kısmı kaldırılır.
- Frontend `Card/Badge/Button/Input/Notice/formatDate`, `describeError`,
  üç durumlu (yükleniyor/hata/boş) desen.

### Yeni bağımlılık

| Paket | Gerekçe |
|---|---|
| `github.com/docker/docker/client` | Container yaşam döngüsü. Docker CLI'ı kabuktan çağırmak yerine SDK: hata yönetimi ve akış okuma güvenilir. |

---

## Riskler

| Risk | Etki | Önlem |
|---|---|---|
| Container sızıntısı (silinmeyen container/volume) | Disk dolar, sistem durur | Temizlik `defer` + `context.WithoutCancel`; ayrıca açılışta sahipsiz container taraması; entegrasyon testi `docker ps -a` boş olduğunu doğrular |
| Kaçak goroutine (iptal sonrası çalışmaya devam) | Kaynak tüketimi | Her çalıştırma `Manager`'da izlenir, `Shutdown` hepsini bekler |
| Uzun süren iş sunucuyu kilitler | Yeni iş başlamaz | Semafor + adım başına timeout + iptal ucu |
| SSE bağlantıları birikir | Bellek | Abonelik `Subscribe`'ın döndürdüğü fonksiyonla kapanır; handler `defer` ile çağırır |
| Git kimlik bilgisi diff'e veya loga sızar | Sızıntı | Token yalnızca `GIT_ASKPASS`/credential store ile; log maskeleme; sızıntı testi |
| Agent prompt'u çok büyük | Kopyalama başarısız | Prompt uzunluğu sınırlanır (32 KB), aşan reddedilir |
| Model araç desteklemiyor, agent hiçbir şey yapamıyor | Boşa harcanan para | Seçimde uyarı; sonuçta "değişiklik yok" açıkça gösterilir |
| `docker.sock` erişimi = host'ta root | Güvenlik | Zaten bilinen risk (Faz 0'da not edildi); runner izole ağda, kaynak limitli, read-only kök |

---

## Test Stratejisi

**Birim**

- `runner/config` — üretilen `opencode.json` ve agent `.md` içeriği; üç sağlayıcı türü için;
  **anahtarın dosyaya düz metin yazılmadığı** (env referansı kullanıldığı).
- `runner` — yetki eşlemesi: `allow_*` kombinasyonları → beklenen permission kuralları.
- `runner/opencode` — `httptest` ile sahte opencode: başarılı akış, session hatası,
  mesaj hatası, SSE kopması, diff boş.
- `settings` — aralık dışı değer reddi, bilinmeyen anahtar, tip uyuşmazlığı, sıfırlama
  varsayılana döndürür, **kayıt defterindeki her varsayılanın kendi min/max aralığında
  olduğu** (tanım hatasını yakalar).
- `runs/manager` — sınır (`ErrTooManyRuns`), iptal, timeout, `RecoverInterrupted`,
  **sınır arttırılınca yeniden başlatmadan geçerli oluyor**.
- `events/bus` — çok aboneli yayın, abonelik kapatma, kapalı kanala yazmama.
- `agentreg` — hazır agent tohumlama, düzenleme sonrası "değiştirilmiş" işareti, sıfırlama.
- `projects/verify` — geçersiz URL, ulaşılamayan depo, kimlik doğrulama hatası.

**Entegrasyon** (gerçek Postgres)

- Çalıştırma kaydı: `pending → running → succeeded`, kopyalanan prompt/model doğru.
- Agent düzenlenince geçmiş çalıştırmanın kopyası **değişmiyor**.
- Çalıştırması olan agent silinemiyor (409).
- `RecoverInterrupted` yarım kayıtları çeviriyor.
- `run_events` sıralı yazılıyor; SSE geçmişi doğru sırada gönderiyor.

**Uçtan uca (gerçek Docker + gerçek model)**

- Küçük bir public repo üzerinde `reviewer` agent'ı çalışır, çıktı ve maliyet kaydedilir.
- Çalışma sonrası `docker ps -a` ve `docker volume ls` **artık bırakmaz**.
- Çalışan bir iş iptal edilir; container 10 saniye içinde kaybolur.

---

## Uygulama Sırası

Riskli olan başta; her adım kendi başına doğrulanabilir.

1. `settings` kayıt defteri + servis + testleri — sonraki adımlar parametreleri buradan okur.
2. `runner` arayüzü + `config` üretimi + testleri (Docker'sız).
3. `sandbox` — Docker SDK ile create/copy/start/rm; sahte bir imajla temizlik testi.
4. `runner/opencode` — Faz 0'da elle yaptığımız akışın kodlanması; `httptest` ile testler.
5. **Uçtan uca duman testi:** koddan tek bir agent çalıştır, çıktı ve maliyet gelsin,
   container temizlensin. *Buraya kadar arayüz yok.*
6. Migration `000003` + `projects`, `agentreg`, `runs` depoları + entegrasyon testleri.
7. `events` bus + `runs.Manager` (semafor, iptal, timeout, recover).
8. HTTP uçları + SSE.
9. `main.go` wiring, açılışta tohumlama ve `RecoverInterrupted`.
10. Ekranlar: projeler → agent'lar → çalıştırma formu → canlı run sayfası.
11. Doğrulama, `AGENTS.md` ve spec durum güncellemeleri.

Adım 5 bu spec'in kalbi: oraya kadar giden yol çalışıyorsa gerisi kabuk.

---

## Doğrulama

1. `make up` → `/readyz` 200; açılış loglarında beş hazır agent'ın tohumlandığı görünür.
2. Ayarlarda git erişimi yokken **açık** bir depo projesi eklenir → kaydedilir.
3. Uydurma bir depo adresiyle proje eklenir → kaydedilmez, "depoya erişilemedi" denir.
4. `/agents` → `reviewer`'ın talimatı düzenlenir → "değiştirilmiş" işareti görünür →
   "sıfırla" ile özgün hali döner.
5. Yeni bir agent oluşturulur, `edit` yetkisi kapatılır.
6. `reviewer` çalıştırılır (küçük public repo, ucuz model). Beklenen:
   canlı olay akışı, sonunda çıktı + token + maliyet.
7. Çalışma sırasında sayfa yenilenir → o ana kadarki ilerleme kaybolmaz.
8. `coder` çalıştırılır ve bir dosya değiştirmesi istenir → diff görünür → "branch'e gönder"
   ile push edilir → depoda branch oluşur.
9. Uzun sürecek bir iş başlatılıp iptal edilir → durum "iptal edildi", container 10 sn içinde yok.
10. Dört iş aynı anda başlatılır (sınır 3) → dördüncüsü 429 ve açıklayıcı mesaj alır.
11. Çalışma sırasında `make down && make up` → iş "kesildi" olarak görünür, "çalışıyor" kalmaz.
12. `docker ps -a` ve `docker volume ls` → runner artığı yok.
13. `make psql` → `runs.agent_prompt` dolu; agent'ı düzenle → eski kaydın kopyası değişmemiş.
14. `docker compose logs backend | grep -E 'sk-or|ghp_|ATATT'` → eşleşme yok.
15. Ayarlarda süre sınırı 1 dakikaya çekilir → uzun bir iş 1 dakikada "zaman aşımı" olur
    (**sunucu yeniden başlatılmadan**).
16. Eşzamanlılık sınırı 1'e çekilir → ikinci iş 429 alır; 5'e çıkarılır → hemen kabul edilir.
17. Bir ayar aralık dışı bir değere çekilmeye çalışılır → reddedilir, aralık söylenir.
18. Ayar sıfırlanır → varsayılana döner, "değiştirilmiş" işareti kalkar.
19. `make test`, `make test-integration` yeşil; `make lint` temiz.
