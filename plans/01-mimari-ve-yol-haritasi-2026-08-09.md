# 01 — Mimari ve Yol Haritası

- **Tarih:** 2026-08-09
- **Durum:** Onaylandı — **2026-08-09'da revize edildi** (aşağıya bkz.)
- **Kapsam:** Tüm sistem (v1)

## Revizyon — 2026-08-09: sağlayıcılar çoğullaştı

Bu dokümanın ilk hali iki noktada gereğinden dar yazılmıştı ve düzeltildi.
Ayrıntı ve gerekçe: [specs/002-coklu-saglayici/](../specs/002-coklu-saglayici/spec.md)

| İlk hali | Şimdiki hali |
|---|---|
| Model erişimi yalnızca **OpenRouter** | **Birden fazla LLM sağlayıcı** aynı anda: OpenRouter, LiteLLM proxy, genel OpenAI-uyumlu |
| Kod deposu yalnızca **GitHub**, kimlik doğrulama token | **GitHub** (token), **Bitbucket** (kullanıcı adı + parola), **genel Git** |
| `credentials(kind)` — tür başına tek kayıt | `llm_providers` ve `git_providers` tabloları, çoklu kayıt; `credentials` yalnızca Jira |
| Runner'da sabit `opencode.json` | Yapılandırma **çalışma anında** seçilen sağlayıcıdan üretilir |

**Neden:** Kurumların çoğu kendi LLM proxy'sini işletiyor (veri dışarı çıkmasın, merkezi
bütçe/kota, kurum içi modeller) ve hepsi GitHub kullanmıyor. Bitbucket ayrıca tek token
yerine kullanıcı adı + parola çifti ister — "token" varsayımı baştan yanlıştı.

Aşağıdaki metinde `openrouter` geçen yerler, o sağlayıcının **örnek** olduğu anlamına gelir;
mimari artık sağlayıcıya bağlı değildir.

## Context

`/Users/omer/workspaces/ai/agent-coder` şu an tamamen boş. Sıfırdan kurulacak sistem: kullanıcının n8n benzeri bir tuval üzerinde **AI coding agent workflow'ları** tasarlayıp çalıştırabildiği bir platform. Örnek akış: Jira'dan task çek → analiz et → kod geliştirme agent'ına ver → code review agent'ına ver → PR aç.

Çözülen problem: bugün kod geliştirme agent'ları tek tek, elle, terminalden çalıştırılıyor. Her adımın farklı modelle çalışması, adımların birbirine bağlanması, sonucun izlenmesi ve maliyetinin ölçülmesi mümkün değil.

Sistem başlangıçta **opencode**'u headless çalıştırma motoru olarak kullanacak; ileride kendi motorumuzla değiştirilebilmesi için arkasına bir `Runner` arayüzü konacak.

### Doğrulanmış teknik dayanaklar

opencode dokümantasyonundan teyit edildi:

| Konu | Gerçek |
|---|---|
| Headless mod | `opencode serve --port 4096 --hostname 0.0.0.0`; `OPENCODE_SERVER_PASSWORD` ile basic auth |
| Session aç | `POST /session` → `{ id }` |
| Prompt gönder | `POST /session/:id/message` body: `{ model:{providerID,modelID}, agent, parts }` — **senkron** |
| Diff / iptal | `GET /session/:id/diff` · `POST /session/:id/abort` |
| Canlı olay akışı | `GET /event` (SSE), ilk olay `server.connected` |
| Sağlık | `GET /global/health` → `{ healthy, version }` |
| Provider listesi | `GET /config/providers` |
| OpenRouter | `provider.openrouter.options.apiKey = "{env:OPENROUTER_API_KEY}"` |
| Agent (md) | `.opencode/agents/<isim>.md`, frontmatter: `description, mode, model, temperature, permission` |
| Skill (md) | `.opencode/skills/<ad>/SKILL.md` **ve `.claude/skills/<ad>/SKILL.md`** — Claude formatı doğrudan destekleniyor |

**Kritik sonuç:** `model` ve `agent` her mesajda parametre olduğu için "her node farklı modelle çalışsın" isteği opencode'un doğal davranışı — özel bir şey yazmamıza gerek yok. Ve opencode `.claude/skills/` okuduğu için "Claude mimarisine sadık kal" isteği tek kaynaktan karşılanıyor.

### Faz 0'da canlı doğrulanan düzeltmeler (2026-08-09, opencode 1.18.15)

Yukarıdaki tablo dokümantasyondan çıkarılmıştı; çalışan sisteme karşı test edilince dört madde değişti:

1. **Agent dizini çoğul:** `.opencode/agents/`. Tekil `.opencode/agent/` runtime loader tarafından okunmuyor, oraya konan agent'lar sessizce yok sayılıyor (opencode#14410).
2. **`model` bir nesne:** `{ providerID: "openrouter", modelID: "anthropic/claude-haiku-4.5" }`. Düz `"openrouter/anthropic/..."` metni yalnızca config ve agent frontmatter'ında geçerli.
3. **`/api/...` namespace'i kullanılamaz:** `POST /api/session/:id/prompt` asenkron ve tamamlanmayı beklemek için gereken `POST /api/session/:id/wait` bu sürümde uygulanmamış (`ServiceUnavailableError`). Prompt kabul ediliyor ama asistan yanıtı üretilmiyor. Senkron `POST /session/:id/message` kullanılacak.
4. **Maliyeti opencode hesaplıyor:** Yanıtta `info.cost` (USD) ve `info.tokens` geliyor. `models_cache` fiyatlarından yeniden hesaplama gereksiz — katalog yalnızca model seçimi ve arayüzde fiyat gösterimi için tutulacak.

Ayrıca kullanışlı bulunan uç noktalar: `GET /session/:id/diff` (git diff'e gerek yok), `POST /session/:id/abort` (iptal), `GET /vcs/status`, ve `POST /session/:id/message` gövdesindeki `format` alanı (JSON Schema ile yapılandırılmış çıktı — `reviewer` bulgu listesi için).

### Onaylanan kararlar

- **Sandbox:** İş başına geçici container. Backend her agent adımı için Docker API ile bir `opencode-runner` container'ı ayağa kaldırır, repo'yu clone eder, iş bitince siler.
- **Workflow motoru:** Kendi Go DAG motorumuz (Postgres state + worker goroutine'ler). Ek altyapı servisi yok.
- **Auth:** v1'de yok, tek kullanıcı. Şema yine de `user_id` taşır, sonradan eklenir.
- **v1 kapsamı (tamamı):** manuel + webhook trigger, Jira entegrasyonu, git sağlayıcıya PR çıktısı, model kataloğu + maliyet takibi.
- **Sağlayıcılar çoğuldur (revizyon):** LLM tarafında OpenRouter / LiteLLM proxy / genel OpenAI-uyumlu, git tarafında GitHub / Bitbucket / genel Git. Aynı anda birden fazla tanımlı olabilir.

---

## Metodoloji: Spec-Driven

Her özellik kod yazılmadan önce `specs/NNN-ozellik-adi/` altında üç dosyayla tanımlanır:

```
specs/001-workflow-engine/
  spec.md    # NE ve NEDEN — kullanıcı hikâyeleri, kabul kriterleri. Teknoloji adı geçmez.
  plan.md    # NASIL — teknik tasarım, veri modeli, arayüzler, riskler
  tasks.md   # Sıralı, işaretlenebilir görev listesi ([ ] / [x]), her biri test edilebilir
```

Kural: `spec.md` onaylanmadan `plan.md`, `plan.md` onaylanmadan `tasks.md`, `tasks.md` onaylanmadan kod yazılmaz. `specs/000-template/` bu üç dosyanın boş şablonunu tutar.

---

## Repo İskeleti

```
agent-coder/
├── AGENTS.md                     # projenin ana agent kural dosyası (mimari, komutlar, konvansiyonlar)
├── CLAUDE.md                     # tek satır: "Bkz. AGENTS.md" (iki araç da aynı kaynağı okur)
├── README.md
├── Makefile                      # up, down, migrate, test, lint, seed
├── .env.example
├── plans/
│   └── 01-mimari-ve-yol-haritasi-2026-08-09.md
├── specs/
│   ├── 000-template/{spec.md,plan.md,tasks.md}
│   └── 001-.../
├── .claude/
│   ├── agents/                   # geliştirme sırasında BİZİM kullandığımız alt-agent'lar
│   │   ├── go-backend-dev.md
│   │   ├── next-frontend-dev.md
│   │   └── spec-reviewer.md
│   └── skills/                   # opencode bunları da okur → tek kaynak
│       ├── spec-driven/SKILL.md
│       ├── go-conventions/SKILL.md
│       └── db-migrations/SKILL.md
├── .opencode/
│   ├── opencode.json             # provider: openrouter, permission ayarları
│   └── agents/                   # ÜRÜNÜN sunduğu agent'lar (runner image'a kopyalanır)
│       ├── analyst.md
│       ├── coder.md
│       ├── reviewer.md
│       ├── tester.md
│       └── upgrader.md
├── backend/                      # Go
├── frontend/                     # Next.js + TypeScript
├── runner/                       # opencode runner Docker image
└── deploy/
    ├── docker-compose.yml
    └── docker-compose.dev.yml
```

**İki ayrı agent kümesi var, karıştırılmamalı:**
- `.claude/agents/*` → *bizim* bu projeyi geliştirirken kullandığımız agent'lar.
- `.opencode/agents/*` → *ürünün son kullanıcıya sunduğu* agent'lar; runner image'ına gömülür ve DB'ye senkronlanır.

---

## Backend (Go)

```
backend/
├── cmd/server/main.go
├── cmd/migrate/main.go
├── internal/
│   ├── config/            env tabanlı config (envconfig)
│   ├── db/
│   │   ├── migrations/    goose .sql dosyaları
│   │   └── sqlc/          sqlc üretimi (pgx/v5)
│   ├── httpapi/           chi router, handler'lar, SSE endpoint
│   ├── workflow/
│   │   ├── graph.go       node/edge modeli, doğrulama, topolojik sıra, döngü tespiti
│   │   ├── executor.go    run döngüsü, step state makinesi, retry
│   │   ├── context.go     run context (jsonb) + `{{ steps.analyst.output }}` şablonlama
│   │   └── nodes/         node kind'leri: trigger, agent, git, jira, condition, http
│   ├── runner/
│   │   ├── runner.go      *** Runner arayüzü — opencode'u ileride değiştirme noktası ***
│   │   ├── opencode/      opencode HTTP istemcisi (session, message, SSE)
│   │   └── sandbox/       Docker container yaşam döngüsü (create/start/wait/logs/rm)
│   ├── agentreg/          .opencode/agents/*.md → DB senkronu, CRUD
│   ├── llm/               sağlayıcı adaptörleri (openrouter, litellm, openai-uyumlu)
│   ├── gitprovider/       git erişimi (github, bitbucket, genel)
│   ├── catalog/           sağlayıcı bazlı model kataloğu + fiyat
│   ├── secrets/           AES-GCM ile credential şifreleme
│   ├── integrations/
│   │   ├── jira/          REST v3 istemci, JQL polling, webhook alıcı, yorum yazma
│   │   └── github/        branch push, PR aç, PR yorumu
│   └── events/            in-process pub/sub → frontend'e SSE
└── go.mod
```

Bağımlılıklar: `chi` (router), `pgx/v5` + `sqlc` (DB), `goose` (migration), `docker/docker` (sandbox), `slog` (log), `testify` (test).

### Runner arayüzü — sistemin en önemli sınırı

opencode'u ileride kendi motorumuzla değiştirebilmemizi sağlayan tek nokta:

```go
// backend/internal/runner/runner.go
type Runner interface {
    Run(ctx context.Context, req RunRequest) (*RunResult, error)
}

type RunRequest struct {
    AgentSlug string            // "reviewer" — .opencode/agents/reviewer.md
    Model     string            // "openrouter/anthropic/claude-sonnet-4.5"
    Prompt    string            // şablondan render edilmiş
    Repo      RepoSpec          // URL, branch, çalışma dizini, git kimlik bilgisi
    Env       map[string]string
    Timeout   time.Duration
    OnEvent   func(Event)       // canlı akış → SSE → UI
}

type RunResult struct {
    Text     string             // agent'ın nihai metin çıktısı
    Diff     string             // unified diff
    Branch   string             // push edildiyse branch adı
    Usage    Usage              // PromptTokens, CompletionTokens
    CostUSD  float64            // models_cache fiyatından hesaplanır
}
```

v1'de tek uygulama: `OpencodeRunner` = `sandbox.Docker` + `opencode.Client`.

### Bir agent adımının çalışma akışı

```
1. Backend: docker create opencode-runner:latest
             --network agent-coder_internal
             -v run-<uuid>:/work
             -e OPENROUTER_API_KEY=... -e GIT_TOKEN=...
2. Container entrypoint: git clone <repo> -b <branch> /work
                         opencode serve --hostname 0.0.0.0 --port 4096
3. Backend: GET  /global/health   (hazır olana kadar bekle, timeout'lu)
4. Backend: POST /session
5. Backend: GET  /event           (SSE dinle → events pubsub → UI)
6. Backend: POST /session/:id/message  { agent, model, parts:[{type:"text", text:prompt}] }
7. Backend: git diff + (gerekiyorsa) branch push, token/cost'u workflow_steps'e yaz
8. Backend: docker rm -f + volume rm
```

Backend container'ına `/var/run/docker.sock` mount edilir (sibling-container deseni). Runner container'ları izole bir Docker network'ünde çalışır, dışarıya port açmaz.

---

## Veritabanı (PostgreSQL)

```sql
users            (id, email, created_at)                    -- v1: tek seed satır
projects         (id, user_id, name, repo_url, default_branch, git_credential_id)
llm_providers    (id, type, name, slug, base_url, secret_enc, hint, is_default)
                 -- type: 'openrouter' | 'litellm' | 'openai_compatible'
git_providers    (id, type, name, base_url, username, secret_enc, hint)
                 -- type: 'github' | 'bitbucket' | 'generic'
credentials      (kind, secret_enc, hint, metadata)
                 -- yalnızca 'jira'; secret_enc AES-GCM
models_cache     (model_id PK, name, context_length, prompt_price, completion_price,
                  supports_tools, fetched_at)
agents           (id, slug, name, description, system_prompt, default_model,
                  tools jsonb, source, updated_at)          -- source: 'file' | 'db'
workflows        (id, project_id, name, description, active_version, is_active)
workflow_versions(id, workflow_id, version, graph jsonb, created_at)
workflow_runs    (id, workflow_id, version, status, trigger_kind, trigger_payload jsonb,
                  context jsonb, error, started_at, finished_at)
workflow_steps   (id, run_id, node_id, node_kind, agent_id, model, status, attempt,
                  input jsonb, output jsonb, diff, branch,
                  prompt_tokens, completion_tokens, cost_usd,
                  error, started_at, finished_at)
step_logs        (id, step_id, ts, level, message)
webhooks         (id, workflow_id, token UNIQUE, secret, created_at)
jira_watches     (id, project_id, jql, last_seen_updated, poll_interval_sec, is_active)
```

`status` enum: `pending | running | succeeded | failed | cancelled | skipped`.

Maliyet: opencode yanıtındaki `info.cost` doğrudan `workflow_steps.cost_usd`'ye yazılır (Faz 0'da doğrulandı — yeniden hesaplamaya gerek yok). Run toplamı `workflow_steps` üzerinden agregasyonla bulunur. `models_cache` fiyatları yalnızca model seçim ekranında tahmini maliyet göstermek için kullanılır.

---

## Workflow Graph Modeli

`workflow_versions.graph` içinde saklanan JSON — React Flow ile birebir uyumlu:

```jsonc
{
  "nodes": [
    { "id": "t1", "kind": "trigger.jira",
      "config": { "jql": "project = ABC AND status = 'To Do'" },
      "position": { "x": 0, "y": 0 } },

    { "id": "a1", "kind": "agent",
      "config": {
        "agent": "analyst",
        "model": "openrouter/anthropic/claude-sonnet-4.5",
        "prompt": "Şu Jira task'ını analiz et:\n{{ trigger.issue.summary }}\n\n{{ trigger.issue.description }}"
      } },

    { "id": "a2", "kind": "agent",
      "config": { "agent": "coder",
                  "model": "openrouter/openai/gpt-5",
                  "prompt": "Analiz:\n{{ steps.a1.output }}\n\nUygula." } },

    { "id": "a3", "kind": "agent",
      "config": { "agent": "reviewer",
                  "model": "openrouter/google/gemini-3-pro",
                  "prompt": "Şu diff'i incele:\n{{ steps.a2.diff }}" } },

    { "id": "g1", "kind": "github.pr",
      "config": { "title": "{{ trigger.issue.key }}: {{ trigger.issue.summary }}",
                  "body": "{{ steps.a3.output }}" } }
  ],
  "edges": [
    { "from": "t1", "to": "a1" }, { "from": "a1", "to": "a2" },
    { "from": "a2", "to": "a3" }, { "from": "a3", "to": "g1" }
  ]
}
```

**v1 node kind'leri:** `trigger.manual`, `trigger.webhook`, `trigger.jira`, `agent`, `github.pr`, `jira.comment`, `condition`, `http.request`.

Şablonlama: Go `text/template` üzerine ince bir sarmalayıcı; `{{ trigger.* }}` ve `{{ steps.<nodeId>.(output|diff|branch) }}` erişimi run context'ten gelir.

Motor davranışı: graph doğrulanır (döngü yok, tek trigger, erişilemez node yok) → topolojik sıra → aynı seviyedeki bağımsız node'lar paralel (goroutine + errgroup) → her step DB'ye yazılır ve `events` üzerinden SSE'ye düşer. Retry: node başına `maxAttempts` (varsayılan 1), üstel geri çekilme.

---

## Frontend (Next.js + TypeScript)

Next.js 15 App Router, TypeScript strict, Tailwind + shadcn/ui, **@xyflow/react** (n8n benzeri tuval), TanStack Query, Zustand (tuval state'i), SSE ile canlı run takibi.

```
frontend/src/
├── app/
│   ├── page.tsx                    # dashboard: son run'lar, maliyet özeti
│   ├── workflows/page.tsx          # liste
│   ├── workflows/[id]/page.tsx     # *** tuval editörü ***
│   ├── runs/[id]/page.tsx          # canlı run detayı
│   ├── agents/page.tsx             # agent CRUD + varsayılan model
│   ├── models/page.tsx             # OpenRouter kataloğu, fiyatlar
│   └── settings/page.tsx           # LLM sağlayıcılar, git erişimleri, Jira
├── components/
│   ├── flow/                       # NodeCanvas, AgentNode, TriggerNode, NodeInspector
│   ├── run/                        # RunTimeline, StepDetail, DiffViewer, LogStream
│   └── ui/                         # shadcn
└── lib/
    ├── api.ts                      # tip güvenli backend istemcisi
    ├── sse.ts                      # EventSource sarmalayıcı + yeniden bağlanma
    └── types.ts                    # backend şemasından üretilen tipler
```

Tuval deneyimi: soldan node paletini sürükle-bırak → node'a tıkla → sağ panelde **agent seçimi + model seçimi (OpenRouter kataloğundan arama) + prompt editörü**. Kaydet → yeni `workflow_version`. "Çalıştır" → `/runs/[id]`'ye yönlendir, node'lar canlı renklenir (bekliyor/çalışıyor/başarılı/hatalı), her node'un altında token ve $ maliyeti görünür.

---

## Docker

`deploy/docker-compose.yml`:

| Servis | Not |
|---|---|
| `postgres` | 16-alpine, named volume, healthcheck |
| `backend` | Go, `/var/run/docker.sock` mount, `:8080` |
| `frontend` | Next.js, `:3000` |
| `migrate` | tek seferlik goose job, backend'den önce çalışır |

`runner/Dockerfile` — servis değil, backend'in `docker run` ettiği image: alpine + `git`, `curl`, `bun`, `opencode` (npm), `.opencode/` (opencode.json + agent md'leri) kopyalanır, entrypoint clone eder ve `opencode serve` başlatır.

Geliştirme: `docker-compose.dev.yml` ile hot reload (backend `air`, frontend `next dev`).

`.env.example`: `OPENROUTER_API_KEY`, `POSTGRES_*`, `SECRET_ENCRYPTION_KEY`, `GITHUB_TOKEN`, `JIRA_BASE_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN`, `OPENCODE_SERVER_PASSWORD`.

---

## Uygulama Fazları

Her faz kendi `specs/NNN-*/` klasörüyle başlar ve çalışan+doğrulanabilir bir çıktıyla biter.

**Faz 0 — İskelet ve altyapı**
Repo yapısı, `AGENTS.md`, `CLAUDE.md`, `.claude/skills/*`, `specs/000-template/`, `plans/01-*.md`, docker-compose, Makefile, backend `/health` + frontend boş sayfa. Çıktı: `make up` üç servisi ayağa kaldırıyor.

**Faz 1 — Veri katmanı ve sağlayıcılar** ✅ *(spec 001 + 002)*
goose migration'ları, AES-GCM secret store, LLM ve git sağlayıcı CRUD API + ayarlar sayfası, sağlayıcı bazlı model kataloğu senkronu + `/models` sayfası. Çıktı: UI'dan birden fazla sağlayıcı tanımlanıp modelleri görülebiliyor.

**Faz 2 — Runner (en riskli faz, erken doğrulanmalı)**
`runner/Dockerfile`, `sandbox` paketi (Docker SDK), `opencode` HTTP istemcisi, `Runner` arayüzü + `OpencodeRunner`. `POST /api/agents/:slug/run` ile tek bir agent'ı elle çalıştır, SSE ile logları izle. Çıktı: UI'dan "reviewer" agent'ı seçilen modelle bir repo üzerinde çalışıp diff ve token/maliyet döndürüyor.

**Faz 3 — Workflow motoru**
Graph modeli + doğrulama, executor, run/step state, şablonlama, `condition` + `http.request` node'ları, manuel ve webhook trigger, run/step API + SSE. Çıktı: 3 node'lu zincir (analyst → coder → reviewer) uçtan uca çalışıyor, her adım farklı modelle.

**Faz 4 — Tuval ve run izleme**
React Flow editörü, node inspector, versiyonlama, canlı run sayfası, diff viewer, maliyet gösterimi. Çıktı: workflow tamamen UI'dan kurulup çalıştırılabiliyor.

**Faz 5 — Jira ve GitHub**
Jira JQL polling + webhook alıcı + yorum yazma, GitHub branch push + PR açma + PR yorumu, `trigger.jira` / `github.pr` / `jira.comment` node'ları. Çıktı: Jira task'ı → otomatik PR, Jira'ya link yorumu.

**Faz 6 — Agent kütüphanesi ve sertleştirme**
`.opencode/agents/*.md` beşlisinin promptlarının olgunlaştırılması, DB senkronu, agent'a özel `permission` kısıtları (reviewer/tester `edit: deny`), timeout, iptal, retry, eşzamanlılık limiti, hata halinde container temizliği.

---

## Ürün Agent'ları (`.opencode/agents/*.md`)

| Agent | Amaç | permission |
|---|---|---|
| `analyst` | Task'ı analiz eder, etkilenen dosyaları ve uygulama planını çıkarır | `edit: deny`, `bash: allow` |
| `coder` | Planı uygular, kod yazar/değiştirir | tam yetki |
| `reviewer` | Diff'i inceler, yapılandırılmış bulgu listesi döner | `edit: deny` |
| `tester` | Değişen kod için unit test yazar ve çalıştırır | `edit: allow` (sadece test dosyaları), `bash: allow` |
| `upgrader` | Bağımlılık/framework yükseltmesi yapar, breaking change'leri düzeltir | tam yetki |

Her biri Claude formatıyla yazılır; `.claude/skills/` altındaki ortak skill'leri (kod konvansiyonları, commit mesajı formatı) opencode zaten okur.

---

## Doğrulama

**Faz 0:** `make up` → `curl localhost:8080/health` → `{"status":"ok"}`; `localhost:3000` açılıyor; `docker compose ps` üçü de healthy.

**Faz 1:** `make migrate` hatasız; ayarlardan bir LLM sağlayıcı tanımla → `/models` sayfasında modelleri ve fiyatları listeleniyor; ikinci bir sağlayıcı eklenince modeller ayrı ayrı görünüyor; DB'de `secret_enc` düz metin **değil**.

**Faz 2 (en kritik):** `/agents` sayfasından `reviewer` agent'ını seç, model olarak bir OpenRouter modeli ver, bir test repo URL'i gir, çalıştır. Beklenen: SSE ile canlı log akışı; sonuçta metin çıktı + token sayıları + $ maliyet; `docker ps -a` çalışma sonrası artık container bırakmamış; `docker volume ls` artık volume bırakmamış.

**Faz 3:** `curl -X POST /api/workflows/:id/runs` → `workflow_steps` tablosunda her node için bir satır, doğru sırayla, her biri farklı `model` değeriyle; kasıtlı hatalı node ekle → run `failed`, sonraki node'lar `skipped`.

**Faz 4:** Tarayıcıda tuvalden 3 node'lu workflow kur, kaydet, çalıştır, node'ların canlı renk değiştirdiğini ve diff'in görüntülendiğini gör.

**Faz 5:** Test Jira projesinde task oluştur → 60 sn içinde run tetikleniyor → GitHub'da PR açılıyor → Jira issue'ya PR linki yorum olarak düşüyor.

**Birim testler:** `workflow/graph` (döngü tespiti, topolojik sıra, doğrulama), `workflow/context` (şablon çözümleme), `models` (maliyet hesabı), `secrets` (şifrele/çöz round-trip), `runner/opencode` (httptest ile sahte opencode sunucusu). `make test` hepsini çalıştırır.

---

## Riskler

| Risk | Önlem |
|---|---|
| opencode API'si sürümler arası değişebilir | Runner image'ında opencode sürümü pinlenir; `opencode` istemcisi tek pakette izole, `Runner` arayüzü değişmez |
| Docker socket erişimi = host'ta root eşdeğeri | Runner'lar izole network, kaynak limitleri (cpu/mem), read-only rootfs dışında sadece `/work`; ileride uzak Docker host'a taşınabilir |
| Uzun süren agent'lar kaynak tüketir | Adım başına timeout + global eşzamanlılık limiti + iptal endpoint'i |
| Model maliyeti kontrolsüz artar | Run başına maliyet tavanı; aşılırsa run durdurulur |
| Git kimlik bilgisi container'a sızar | Token yalnızca env ile geçer, log'lardan maskelenir, container iş bitince silinir |

---

## Revizyon — Rapor sayfası eklendi (2026-08-09)

Bu plan yazıldığında raporlama Faz 4'ün ("maliyet gösterimi") bir parçası sayılmıştı.
Kullanıcı isteği üzerine **kendi sayfasına** çıkarıldı ve Faz 2'de teslim edildi
([spec 004](../specs/004-rapor/spec.md)).

Gerekçe: "maliyeti node'un altında göster" ile "bu ay ne harcadık, karşılığında ne
üretildi" farklı sorulardır. İkincisi tek bir çalıştırmaya değil, **tüm geçmişe** bakar
ve workflow'lar gelmeden de anlamlıdır.

Tasarım notları:

- Rapor **türetilmiş veridir**. Özet tablo, sayaç veya materialized view yok; `runs`
  üzerinde agregasyon yapılır. İkinci bir gerçek kaynağı yaratmamak bilinçli bir karar.
- Agent hangi yolla çalıştırılırsa çalıştırılsın (arayüz, API, ileride workflow) kayıt
  `runs` tablosuna düştüğü için rapor **eksiksizdir**. Faz 3 geldiğinde workflow
  kırılımı eklenecek; toplamlar zaten kapsıyor olacak.
- `workflow_steps` tablosu geldiğinde rapor sorguları `runs` ∪ `workflow_steps` yerine
  **tek tablo** üzerinde kalmalı: adımlar da `runs` kaydı üretirse rapor değişmeden
  doğru kalır. Faz 3 tasarımında bu gözetilecek.

## Revizyon — davranış parametreleri ortam değişkenlerinden çıkarıldı (2026-08-09)

Bu plandaki `.env.example` listesi `RUNNER_TIMEOUT_SEC`, `RUNNER_MAX_CONCURRENCY`,
`RUNNER_CPU_LIMIT` ve `RUNNER_MEMORY_LIMIT` içeriyordu. Spec 003 H7 ile bu parametreler
veritabanına (ayarlar kayıt defteri) taşındı ama ortam değişkenleri **okunmaya devam
ediyordu ve hiçbir yerde kullanılmıyordu** — `.env`'i değiştiren kullanıcı hiçbir etki
görmeyecekti.

Dördü de `config.RunnerConfig`, `.env.example` ve `docker-compose.yml` içinden kaldırıldı.
Kalan sınır nettir:

- **Ortam değişkeni:** veritabanına bağlanmak için gerekenler + dağıtım topolojisi
  (`RUNNER_IMAGE`, `RUNNER_NETWORK`, `OPENCODE_SERVER_PASSWORD`, portlar, anahtarlar).
- **Veritabanı:** çalışma davranışını belirleyen her şey.

## Revizyon — Faz 3 tamamlandı (2026-08-09)

Planın Faz 3 tanımı `condition` ve `http.request` düğümlerini de içeriyordu.
Kullanıcı kararıyla **kapsam dışına alındı** (spec 007 K3): koşul ifadelerinin
kendi dili, doğrulaması ve hata modeli var; motor önce en basit haliyle
çalıştırılıp gerçek ihtiyaç görülecek. `kind` alanı açık uçlu olduğu için
eklenmeleri migration gerektirmiyor.

Planda öngörülmeyen üç karar:

1. **Adım = çalıştırma.** Akış adımı ayrı bir kavram değil; `runs` tablosuna
   normal bir kayıt yazıyor. Bu sayede rapor, çalıştırma listesi, detay, iptal
   ve gönderme **hiç değişmeden** akışları kapsadı. Plandaki `workflow_steps`
   tablosu duruyor ama yalnızca düğüm durumu için (`skipped` hali).
2. **Şablon doğrulaması kaydetme anına çekildi.** Bir adımın referans verdiği
   düğüm ATASI olmak zorunda. En sinsi durum paralel kardeşe referans: bazen
   çalışır bazen çalışmaz.
3. **`internal/runbuild` paketi çıktı.** Çalıştırma girdisini kuran mantık HTTP
   handler'ının içindeydi; motor da aynı çözümlemeye ihtiyaç duyunca ortak bir
   pakete taşındı.

Ayrıca `runs.Manager` bloklayan bir giriş noktası (`Execute`) kazandı; gövde
`Start` ile ortak olduğu için akış adımları da genel eşzamanlılık sınırından,
zaman aşımından ve iptalden aynı yerden geçiyor.

## Revizyon — Faz 4 tamamlandı (2026-08-10)

Tuval editörü planlandığı gibi React Flow (`@xyflow/react`) ile yapıldı ve
**backend'e hiç dokunulmadı** — düğüm konumlarının ve tetikleyici düğümün Faz
3'te şimdiden saklanması tam da bunun içindi.

Planda öngörülmeyen iki karar:

1. **Adım listesi editörü kaldırıldı.** Aynı veriyi düzenleyen iki ekran ikisinin
   de bakımını gerektirir ve er geç ayrışır. Tuval doğrusal akışı da rahat kuruyor.
2. **İzleme, düzenlemeyle aynı bileşen.** `FlowCanvas` salt okunur modda
   çalışıyor; ikinci bir çizim bileşeni aynı görselin iki yerde tutulması olurdu.

Otomatik yerleşim (`lib/flow-layout.ts`) motorun seviye hesabıyla aynı mantığı
kullanıyor: sütun = seviye. Tuval, akışın gerçekten nasıl çalıştığını gösteriyor
— aynı sütundaki adımlar aynı anda koşanlar.

## Revizyon — Faz 5 tamamlandı (2026-08-10)

Jira ve GitHub entegrasyonu planlandığı gibi tamamlandı; uçtan uca hedef
(`Jira task'ı → otomatik PR → Jira'ya link yorumu`) gerçek bir Jira sitesi ve
gerçek bir depo üzerinde ölçüldü.

Planda öngörülmeyen üç şey:

1. **Jira tetikleyici tuvale "eklenmiyor", başlangıç düğümü ona çevriliyor.**
   Bir akışın tam olarak bir girişi var; "Jira tetikleyici ekle" düğmesi ikinci
   bir başlangıç üretir ve kaydetme her seferinde reddedilirdi.

2. **Akış kendi kendini tetikliyordu.** Jira'ya yazılan yorum task'ın
   güncellenme zamanını değiştiriyor, tekrar-işleme koruması da bu zamanı
   anahtar alıyor. Beş dakikalık tarayıcıyla bu, kimse bir şey yapmadan her
   turda yeni bir PR ve yeni bir model maliyeti demekti. Yorum adımı artık
   ürettiği güncellemeyi kendi adına işaretliyor.

   Genel ders: **bir sistem dış dünyaya yazıyorsa, kendi yazdığını okumadığından
   emin olmak ayrı bir iştir.** Bunu birim testi değil, gerçek Jira'ya karşı
   ikinci bir tetikleme denemesi gösterdi.

3. **`condition` ve `http.request` düğümleri hâlâ yok.** Faz 3 planında
   sayılmışlardı ama hiçbir gerçek akış onları istemedi; kayıt defteri (`kinds.go`
   + `handler.go`) sayesinde eklenmeleri motoru değiştirmiyor. İhtiyaç doğunca
   eklenecekler — şimdiden yazılsalardı kullanılmayan yüzey olurdu.

Jira tarafında planın "REST v3 istemci, JQL polling" satırı bir düzeltme
gerektirdi: eski `/rest/api/3/search` ucu Ağustos 2025'ten beri 410 dönüyor,
arama `POST /rest/api/3/search/jql` ile `nextPageToken` sayfalaması üzerinden
yapılıyor.

## Revizyon — Arayüz denetimi (2026-08-10)

Beş faz boyunca arayüz özellik özellik büyüdü ama hiç **bütün olarak**
kullanılmadı. Baştan sona bir kullanıcı gibi gezildiğinde çıkanlar aşağıda.

Planda hiç öngörülmemiş iki şey:

1. **Responsive hiç düşünülmemişti.** Kenar çubuğu sabit 216px'ti ve kırılma
   noktası yoktu; telefonda içeriğe ~175px kalıyor, akış ekranında "Kaydet"
   düğmesi ekranın dışında kalıyordu. Plandaki frontend bölümü sayfaları ve
   bileşenleri sayıyor, hiçbir yerde ekran genişliğinden söz etmiyor.

2. **Renk doğrulaması bir araç gerektiriyordu.** İki kez elle düzeltildi
   (spec 006 ve sonrası) ama hiç ölçülmedi. Ölçünce on dört bileşenin koyu
   temada, birinin açık temada eşiğin altında olduğu görüldü — hepsi iki token
   satırıydı. Göz bu hatayı bulamaz: iki tema aynı anda görülemiyor.

   `scripts/theme-audit.mjs` bu yüzden var. Aracın kendisi de bir ders verdi:
   ilk sürümü "0 hata" dedi çünkü Tailwind v4'ün `oklab(... / 0.35)` çıktısını
   tanımayıp elemanları sessizce atlıyordu.

Ayrıca token modeline tek bir rol eklendi: `--color-control-line`. Bir düğmenin
sınırı ile bir kartın ayracı aynı şey değil — ilki erişilebilirlik gereği 3:1
olmak zorunda, ikincisi değil. Bu ayrım yokken bileşenler en yakın süsleme
token'ını ödünç alıyordu.

## Revizyon — MCP desteği, Aşama 1 (2026-08-10)

Planda hiç yoktu: agent'ların dış araçlara **standart bir protokolle** erişmesi.
Faz 5'e kadar her kaynak için ayrı istemci yazdık (Jira, GitHub); üçüncü ve
dördüncü kaynak aynı işi tekrar yazmak olurdu.

Beklenmedik şekilde ucuz çıktı ve sebebi Faz 2'de verilmiş bir karardı:
`opencode.json` imaja gömülmüyor, **her çalıştırmada Go tarafında üretilip**
container'a kopyalanıyor. MCP eklemek üretilen haritaya bir anahtar eklemek
oldu — imaj, entrypoint ve motor istemcisi hiç değişmedi.

Pahalı olan kısım beklediğim yerde değildi: **MCP'nin kendisi değil, etrafındaki
tesisat**. Şifreli, çok kayıtlı, doğrulamalı bir bağlantı yönetimi + arayüz
bölümü + agent ataması.

İki not:

1. **Sessiz başarısızlık.** Motor, bağlanamayan bir MCP sunucusunu uyarmadan yok
   sayıyor. Bu, hata ayıklaması en zor sınıftan bir arıza: agent araçsız kalıyor
   ama kimse sebebini görmüyor. Mesaj göndermeden önce durum sorgulaması eklendi.

2. **Doğrulanmamış davranışa yaslanan güvenlik kuralı yazmadım.** Yetki
   sıralamasının semantiğini bilmediğim için toptan bir "geri kalan yasak"
   kuralını kaldırdım. Ölçülmeden konsaydı, güvenlik sağlamak yerine yanlış bir
   güven duygusu verirdi.
