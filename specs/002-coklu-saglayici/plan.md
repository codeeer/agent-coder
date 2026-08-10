# Plan: Çoklu LLM ve Git Sağlayıcı Desteği

- **Spec no:** 002 — [spec.md](spec.md)
- **Tarih:** 2026-08-09
- **Durum:** İnceleme — onay bekliyor

---

## Yaklaşım

Spec 001'de "tür başına tek kimlik bilgisi" olan yapı, iki ayrı **sağlayıcı tablosuna**
dönüşüyor. Değişimin özü tek cümlede: *kimlik bilgisi artık türün özelliği değil,
sağlayıcının özelliği.*

```
001:  credentials(kind PK) ──► models(id PK)
002:  llm_providers(id) ──1:N──► models(provider_id, id PK)
      git_providers(id)
      credentials(kind PK)  ← yalnızca Jira kalır
```

Üç iş var:

1. **Şema ve veri taşıma** — yeni tablolar, `models`'ın sağlayıcıya bağlanması,
   001'de kaydedilmiş anahtarların kaybolmadan yeni yapıya geçmesi.
2. **Sağlayıcı türü başına adaptör** — her türün doğrulama ve katalog okuma yolu farklı.
   Ortak bir arayüz arkasına alınır.
3. **Bilinmeyen bilginin taşınması** — `context_length` ve `supports_tools` nullable olur
   (`*int`, `*bool`). **Fiyatlar nullable olmaz:** kullanıcı kararıyla bilinmeyen fiyat
   sıfır sayılır, model ücretsiz görünür.

### Doğrulanan teknik gerçekler

| Kaynak | Bulgu | Plana etkisi |
|---|---|---|
| opencode dokümanı | Özel sağlayıcı: `npm: "@ai-sdk/openai-compatible"` + `baseURL` + açık `models` listesi | Runner'ın `opencode.json`'ı üretilecek; **Faz 2'ye taşındı** |
| LiteLLM dokümanı | `/model/info` → `model_info` içinde `max_tokens`, `input_cost_per_token`, `output_cost_per_token` | Katalog buradan okunur |
| LiteLLM dokümanı | Bu alanlar **opsiyonel**, yönetici doldurmadıysa gelmez | "Bilinmiyor" durumu şart |
| OpenRouter (canlı) | `/models/user` zengin meta veri döner | Mevcut adaptör korunur |

**Doğrulanmamış nokta:** Elimizde çalışan bir LiteLLM örneği yok; `/model/info` yanıtının
tam alan yapısı dokümandan okundu, canlı teyit edilmedi. Bu yüzden LiteLLM adaptörü
`/model/info` başarısız olursa OpenAI-uyumlu `/v1/models`'a düşer — o zaman yalnızca model
kimlikleri gelir, diğer alanlar "bilinmiyor" olur. Bazı kurulumlarda yönetici uçları zaten
kapalı olduğu için bu yedek yol her hâlükârda gerekli.

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
|---|---|---|---|
| Ayrı `llm_providers` + `git_providers` tabloları | Her birinin alanları farklı ve net | İki tablo | **Seçildi** |
| Tek `providers` tablosu + `kind` ayrımı | Tek tablo | Bitbucket'ın `username`'i LLM'de anlamsız, yarısı boş kolon | Elendi |
| `models.provider_id` + bileşik birincil anahtar | Aynı model adı iki sağlayıcıda ayrı satır | Sorgular iki kolonla çalışır | **Seçildi** — spec H2 gereği |
| Model kimliğine sağlayıcı ön eki (`sirket/gpt-4o`) | Tek kolon | Ayrıştırma kırılgan, model adında `/` zaten var | Elendi |
| Fiyat bilinmiyorsa sıfır | Kod basit, `*float64` taşımak yok | Model "ücretsiz" görünür ama proxy ücretlendirebilir | **Seçildi** — kullanıcı kararı, riski bilerek kabul etti |
| Bağlam/araç desteği için nullable | Yanlış varsayım yapılmaz | `*int`/`*bool` taşımak | **Seçildi** — araç desteği modelin agent olarak kullanılabilirliğini belirliyor |

---

## Veri Modeli

`backend/internal/db/migrations/000002_coklu_saglayici.sql`

```sql
-- +goose Up
CREATE TYPE llm_provider_type AS ENUM ('openrouter', 'litellm', 'openai_compatible');
CREATE TYPE git_provider_type AS ENUM ('github', 'bitbucket', 'generic');

CREATE TABLE llm_providers (
    id         UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    type       llm_provider_type NOT NULL,
    name       TEXT              NOT NULL,          -- kullanıcının verdiği ad
    slug       TEXT              NOT NULL UNIQUE,   -- opencode provider kimliği
    base_url   TEXT              NOT NULL,
    secret_enc BYTEA             NOT NULL,
    hint       TEXT              NOT NULL,
    is_default BOOLEAN           NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ       NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ       NOT NULL DEFAULT now()
);

-- En fazla bir varsayılan olabilir; kural veritabanında dayatılır ki
-- eşzamanlı iki güncelleme onu bozamasın.
CREATE UNIQUE INDEX idx_llm_providers_one_default
    ON llm_providers (is_default) WHERE is_default;

CREATE TABLE git_providers (
    id         UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    type       git_provider_type NOT NULL,
    name       TEXT              NOT NULL,
    base_url   TEXT              NOT NULL DEFAULT '',  -- kendi sunucusundakiler için
    username   TEXT              NOT NULL DEFAULT '',  -- bitbucket ve genel git
    secret_enc BYTEA             NOT NULL,
    hint       TEXT              NOT NULL,
    created_at TIMESTAMPTZ       NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ       NOT NULL DEFAULT now()
);

-- models artık bir sağlayıcıya ait. Aynı model adı iki sağlayıcıda
-- ayrı satırdır (spec H2).
ALTER TABLE models ADD COLUMN provider_id UUID
    REFERENCES llm_providers(id) ON DELETE CASCADE;

-- Bağlam ve araç desteği bilinmeyebilir. Fiyatlar NOT NULL kalır:
-- bilinmeyen fiyat sıfır sayılır (kullanıcı kararı), model ücretsiz görünür.
ALTER TABLE models
    ALTER COLUMN context_length DROP NOT NULL,
    ALTER COLUMN supports_tools DROP NOT NULL;

-- Katalog durumu sağlayıcı başına tutulur (spec H2: biri düşerse
-- diğerleri etkilenmez).
CREATE TABLE provider_sync (
    provider_id     UUID PRIMARY KEY REFERENCES llm_providers(id) ON DELETE CASCADE,
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    model_count     INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT
);

-- ─── 001'den veri taşıma ────────────────────────────────────────────────
-- Kaydedilmiş OpenRouter anahtarı sağlayıcıya dönüşür (spec H6).
INSERT INTO llm_providers (type, name, slug, base_url, secret_enc, hint, is_default)
SELECT 'openrouter', 'OpenRouter', 'openrouter',
       'https://openrouter.ai/api/v1', secret_enc, hint, true
FROM credentials WHERE kind = 'openrouter';

-- Kaydedilmiş GitHub token'ı git sağlayıcısına dönüşür.
INSERT INTO git_providers (type, name, base_url, username, secret_enc, hint)
SELECT 'github', 'GitHub', '', '', secret_enc, hint
FROM credentials WHERE kind = 'github';

DELETE FROM credentials WHERE kind IN ('openrouter', 'github');
-- Jira credentials tablosunda kalır; git sağlayıcısı değildir.

-- Mevcut modeller taşınan sağlayıcıya bağlanır. Sağlayıcı yoksa
-- (anahtar yalnızca .env'deydi) katalog boşaltılır; açılışta yeniden indirilir.
UPDATE models SET provider_id = (SELECT id FROM llm_providers LIMIT 1);
DELETE FROM models WHERE provider_id IS NULL;

ALTER TABLE models ALTER COLUMN provider_id SET NOT NULL;
ALTER TABLE models DROP CONSTRAINT models_pkey;
ALTER TABLE models ADD PRIMARY KEY (provider_id, id);

INSERT INTO provider_sync (provider_id, model_count, last_success_at)
SELECT p.id, count(m.id), now()
FROM llm_providers p LEFT JOIN models m ON m.provider_id = p.id
GROUP BY p.id;

DROP TABLE catalog_sync;

-- +goose Down
CREATE TABLE catalog_sync (
    id              BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    model_count     INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT
);
INSERT INTO catalog_sync (id) VALUES (true);

DELETE FROM models;   -- bileşik anahtardan tekile dönüşte çakışma olmasın
ALTER TABLE models DROP CONSTRAINT models_pkey;
ALTER TABLE models ADD PRIMARY KEY (id);
ALTER TABLE models DROP COLUMN provider_id;
ALTER TABLE models
    ALTER COLUMN context_length SET NOT NULL,
    ALTER COLUMN supports_tools SET NOT NULL;

DROP TABLE provider_sync;
DROP TABLE git_providers;
DROP TABLE llm_providers;
DROP TYPE git_provider_type;
DROP TYPE llm_provider_type;
```

> **Geri alma dürüstlüğü:** `Down` bloğu şemayı 001'e döndürür ama **taşınan anahtarları
> geri yazmaz** ve model kataloğunu siler. Anahtarlar geri alma sonrası yeniden girilmeli.
> Bunu gizlemek yerine burada ve migration dosyasında yorumla belirtiyorum; alternatifi
> (çift yönlü tam veri taşıma) bu aşamada emeğe değmez.

**`slug` neden var:** opencode yapılandırmasında her sağlayıcının bir kimliği olmak zorunda
(`provider.<slug>`). Kullanıcının verdiği addan türetilir (`"Şirket LiteLLM"` → `sirket-litellm`),
çakışırsa sonuna sayı eklenir. Faz 2'de runner config'i bu slug ile üretilecek.

---

## Arayüzler

### `internal/llm` — sağlayıcı adaptörleri

```go
type Type string
const (
    TypeOpenRouter       Type = "openrouter"
    TypeLiteLLM          Type = "litellm"
    TypeOpenAICompatible Type = "openai_compatible"
)

// Provider, bir LLM sağlayıcının yapılandırması.
type Provider struct {
    ID        uuid.UUID
    Type      Type
    Name      string
    Slug      string
    BaseURL   string
    Hint      string
    IsDefault bool
    UpdatedAt time.Time
}

// Model, katalogdaki bir model.
type Model struct {
    ID              string
    Name            string
    ContextLength   *int       // nil = bilinmiyor, "—" gösterilir
    MaxOutputTokens *int       // nil = bilinmiyor
    PromptPrice     float64    // token başına USD; bilinmiyorsa 0 (= ücretsiz)
    CompletionPrice float64
    SupportsTools   *bool      // nil = bilinmiyor — "desteklemiyor" DEĞİL
    Modality        string
    Raw             json.RawMessage
}

// Client, tür başına uygulanır.
type Client interface {
    // Verify, anahtarın ve adresin çalıştığını doğrular.
    Verify(ctx context.Context, key string) error
    // ListModels, sağlayıcının modellerini döner.
    ListModels(ctx context.Context, key string) ([]Model, error)
}

func NewClient(p Provider) (Client, error)

var (
    ErrUnauthorized  = errors.New("anahtar geçersiz")
    ErrUnreachable   = errors.New("adrese ulaşılamıyor")
    ErrBadCatalog    = errors.New("model listesi okunamadı")
    ErrInvalidBaseURL = errors.New("servis adresi geçersiz")
)
```

Tür başına davranış:

| Tür | Doğrulama | Katalog | Meta veri |
|---|---|---|---|
| `openrouter` | `GET {base}/key` | `GET {base}/models/user` | Tam: fiyat, bağlam, araç |
| `litellm` | `GET {base}/model/info` | `GET {base}/model/info` | `model_info` doluysa tam, değilse kısmi |
| `litellm` (yedek) | — | `GET {base}/models` | Yalnızca model kimlikleri |
| `openai_compatible` | `GET {base}/models` | `GET {base}/models` | Yalnızca model kimlikleri |

`base_url` doğrulaması: `https://` veya `http://` şemasıyla başlamalı, ayrıştırılabilir
olmalı, sonundaki `/` kırpılır. OpenRouter türünde adres alanı gizlenir ve sabit değer kullanılır.

### `internal/gitprovider` — git erişimi

```go
type Type string
const (
    TypeGitHub    Type = "github"
    TypeBitbucket Type = "bitbucket"
    TypeGeneric   Type = "generic"
)

type Provider struct {
    ID       uuid.UUID
    Type     Type
    Name     string
    BaseURL  string   // kendi sunucusunda barındırılanlar için
    Username string   // bitbucket ve genel git
    Hint     string
}

// Validator, erişim bilgisinin çalıştığını sınar.
// ErrNotVerifiable dönerse kayıt yine de yapılır (spec H5).
type Validator interface {
    Validate(ctx context.Context, p Provider, secret string) error
}

var ErrNotVerifiable = errors.New("bu tür için doğrulama yapılamıyor")
```

| Tür | Kimlik doğrulama | Doğrulama çağrısı |
|---|---|---|
| `github` | Bearer token | `GET {base\|api.github.com}/user` |
| `bitbucket` | Basic: kullanıcı adı + app password | `GET {base\|api.bitbucket.org/2.0}/user` |
| `generic` | Basic: kullanıcı adı + token | yok → `ErrNotVerifiable`, uyarıyla kaydedilir |

### HTTP API

| Metot | Yol | Not |
|---|---|---|
| GET | `/api/llm-providers` | gizli değer yok, `hint` var |
| POST | `/api/llm-providers` | `{type,name,baseUrl,secret,isDefault}` — doğrular, kaydeder, katalogu indirir |
| PUT | `/api/llm-providers/{id}` | `secret` boş bırakılırsa mevcut anahtar korunur |
| DELETE | `/api/llm-providers/{id}` | modelleri de siler (CASCADE) |
| POST | `/api/llm-providers/{id}/sync` | tek sağlayıcının katalogunu tazeler |
| GET | `/api/git-providers` | |
| POST/PUT/DELETE | `/api/git-providers[/{id}]` | |
| GET | `/api/models` | `provider`, `q`, `tools`, `free`, `unknown`, `sort`, `order`, `limit`, `offset` |
| POST | `/api/models/refresh` | tüm sağlayıcılar; **kısmi başarı** döner |
| GET | `/api/credentials` | yalnızca Jira kalır |

`POST /api/models/refresh` yanıtı, spec'in "biri düşerse diğerleri etkilenmez" kuralını
görünür kılar:

```json
{ "results": [
    { "providerId": "…", "name": "Şirket LiteLLM", "ok": true,  "count": 12 },
    { "providerId": "…", "name": "OpenRouter",     "ok": false, "error": "adrese ulaşılamadı" }
] }
```

### Frontend tipleri

```ts
export type LLMProviderType = "openrouter" | "litellm" | "openai_compatible";
export type GitProviderType = "github" | "bitbucket" | "generic";

export interface LLMProvider {
  id: string; type: LLMProviderType; name: string; slug: string;
  baseUrl: string; hint: string; isDefault: boolean; updatedAt: string;
  sync: { lastSuccessAt: string | null; modelCount: number; lastError: string | null };
}

export interface GitProvider {
  id: string; type: GitProviderType; name: string;
  baseUrl: string; username: string; hint: string; updatedAt: string;
}

export interface Model {
  providerId: string; providerName: string;
  id: string; name: string;
  contextLength: number | null;          // null = bilinmiyor → "—"
  maxOutputTokens: number | null;
  promptPricePerMTok: number;            // bilinmiyorsa 0
  completionPricePerMTok: number;
  supportsTools: boolean | null;         // null = bilinmiyor
  isFree: boolean;
  isPreview: boolean;
}
```

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
|---|---|
| `backend/internal/db/migrations/000002_coklu_saglayici.sql` | yeni |
| `backend/internal/llm/{provider.go,client.go,openrouter.go,litellm.go,openaicompat.go}` | yeni — `internal/openrouter` buraya taşınır |
| `backend/internal/llm/store.go` | yeni — sağlayıcı CRUD, varsayılan yönetimi |
| `backend/internal/gitprovider/{store.go,validator.go}` | yeni |
| `backend/internal/catalog/{store.go,syncer.go}` | düzenleme — sağlayıcı bazlı, nullable alanlar |
| `backend/internal/credentials/*` | düzenleme — yalnızca Jira; OpenRouter/GitHub kodu kaldırılır |
| `backend/internal/httpapi/{llmproviders.go,gitproviders.go}` | yeni |
| `backend/internal/httpapi/{models.go,router.go,credentials.go}` | düzenleme |
| `backend/cmd/server/main.go` | düzenleme — bootstrap, yeni bağımlılıklar |
| `frontend/src/lib/{types.ts,api.ts}` | düzenleme |
| `frontend/src/components/settings/{LLMProviderCard,LLMProviderForm,GitProviderCard,GitProviderForm}.tsx` | yeni |
| `frontend/src/components/models/ModelTable.tsx` | düzenleme — sağlayıcı sütunu, "bilinmiyor" gösterimi |
| `frontend/src/app/{settings,models}/page.tsx` | düzenleme |

### Yeniden kullanılacak mevcut kod

- `internal/secrets` — olduğu gibi; iki yeni tablo da aynı `Cipher` ile şifrelenir.
- `internal/openrouter/client.go` — silinmez, `internal/llm/openrouter.go` olarak taşınır;
  `Model` metotları (`IsFree`, `SupportsTools`, `IsPreview`, `parsePrice`) korunur, yalnızca
  dönüş tipleri nullable hale gelir.
- `internal/credentials` içindeki `checkAuthResponse` — `gitprovider` doğrulayıcıları
  aynı durum kodu yorumlamasını kullanır, ortak bir yardımcıya taşınır.
- `internal/catalog/store.go`'daki `ListFilter`, `Normalize`, `sortColumns` deseni —
  sağlayıcı filtresi eklenerek korunur.
- `httpapi.respondJSON` / `respondError` / `Deps` yapısı — yeni handler'lar aynı desende.
- Frontend `Card`, `Badge`, `Button`, `Input`, `Notice`, `formatDate` — yeni ekranlar
  bunları kullanır, yeni görsel bileşen icat edilmez.
- Frontend `describeError` deseni — yeni hata kodları buraya eklenir.

### Ortam değişkeni yedeğinin yeni hali

001'de `.env`'deki anahtar bir "yedek çözümleme" idi. Sağlayıcılar artık satır olduğu için
bu yaklaşım anlamını yitiriyor. Yerine **bootstrap**: sunucu açılışında `llm_providers`
tablosu **tamamen boşsa** ve `OPENROUTER_API_KEY` tanımlıysa, otomatik olarak bir OpenRouter
sağlayıcısı oluşturulur.

Böylece mevcut kurulum hiçbir şey yapmadan çalışmaya devam eder ve anahtar arayüzde
görünür hale gelir. Kural bilinçli olarak "tablo boşsa" — kullanıcı sağlayıcıyı silip
yeniden başlatırsa geri gelir; bunu istemeyen `.env`'deki değişkeni boşaltır.
Davranış `AGENTS.md`'ye yazılır.

---

## Riskler

| Risk | Etki | Önlem |
|---|---|---|
| LiteLLM `/model/info` yanıtı beklediğimizden farklı | Katalog boş gelir | `/models`'a düşen yedek yol; ayrıştırma hataları modeli atlar, senkronu düşürmez |
| Migration mevcut anahtarı kaybeder | Kullanıcı yeniden girer | Taşıma SQL'i migration içinde; entegrasyon testi 001 verisiyle taşımayı doğrular |
| Araç desteği bilinmeyen model "desteklemiyor" sayılır | LiteLLM modelleri agent olarak hiç seçilemez — kurum için sistem işe yaramaz | `*bool` derleyici tarafından zorlanır; "nil araç desteği false değildir" testi |
| Bileşik birincil anahtar sorguları bozar | Yanlış model listelenir | `models` sorgularının tamamı entegrasyon testinde |
| İki varsayılan sağlayıcı oluşur | Belirsiz davranış | Kısmi UNIQUE index veritabanı seviyesinde engeller |
| Genel Git doğrulanamadan kaydedilir | Yanlış bilgi Faz 5'te patlar | Kayıt sırasında ve listede açık uyarı |

---

## Test Stratejisi

**Birim**

- `llm/litellm` — `httptest` ile: dolu `model_info`, boş `model_info`, `/model/info` 404
  → `/models` yedeğine düşme, bozuk JSON, 401.
- `llm/openaicompat` — yalnızca kimlik gelen yanıt; tüm meta veri nil olmalı.
- `llm/openrouter` — mevcut testler nullable dönüşe uyarlanır.
- `llm` — `base_url` doğrulaması (şema yok, boşluk, sondaki `/`, geçersiz URL).
- `llm` — slug türetme: Türkçe karakter, boşluk, çakışma.
- `gitprovider` — üç türün doğrulama çağrısı ve `ErrNotVerifiable`.
- **"Bilinmeyen araç desteği false değildir"** — `SupportsTools` nil olan model
  `tools=1` filtresine düşmez ama "desteklemiyor" olarak da işaretlenmez.
- Fiyatı gelmeyen model 0 fiyatla kaydedilir ve `is_free = true` olur (kullanıcı kararı).

**Entegrasyon** (gerçek Postgres)

- Migration: 001 verisiyle dolu bir veritabanında `up` çalışır, OpenRouter anahtarı
  `llm_providers`'a taşınmış ve **çözülebilir** olur, GitHub token'ı `git_providers`'da,
  Jira `credentials`'da kalır.
- İki sağlayıcı + aynı isimli model → iki ayrı satır, karışmaz.
- Sağlayıcı silinince modelleri ve `provider_sync` satırı da silinir.
- İkinci varsayılan işaretlemek → veritabanı reddeder veya öncekini düşürür.
- Kısmi senkron: bir sağlayıcı hata verir, diğerinin modelleri güncellenir.

**Elle** — aşağıdaki doğrulama listesi.

---

## Uygulama Sırası

1. `internal/llm` iskeleti: tipler, `Client` arayüzü, `base_url` ve slug doğrulaması + testleri.
2. Üç adaptör (`openrouter` taşınır, `litellm`, `openaicompat`) + `httptest` testleri.
3. Migration `000002` + taşıma SQL'i + migration entegrasyon testi.
4. `llm/store.go` ve `gitprovider/store.go` + entegrasyon testleri.
5. `catalog` sağlayıcı bazına geçirilir (nullable alanlar, `provider_sync`, kısmi senkron).
6. HTTP uçları: LLM sağlayıcılar, git sağlayıcılar, güncellenmiş modeller.
7. `main.go` bootstrap ve wiring.
8. Ayarlar ekranı: iki yeni bölüm.
9. Modeller ekranı: sağlayıcı sütunu ve filtresi, "bilinmiyor" gösterimi.
10. Uçtan uca doğrulama, `AGENTS.md` ve spec durum güncellemeleri.

Adım 1-4 bittiğinde sistem hâlâ eski arayüzle ayakta olmalı; kullanıcıya görünen
değişiklik adım 8'de başlar.

---

## Doğrulama

1. **Taşıma:** Güncelleme öncesi OpenRouter anahtarı kayıtlıyken `make up` →
   ayarlarda "OpenRouter" sağlayıcısı `••••fd36` ile görünür, modeller ekranı dolu.
2. `make psql` → `SELECT count(*) FROM credentials;` yalnızca Jira varsa 1, yoksa 0.
3. İkinci sağlayıcı ekle (LiteLLM, uydurma adres) → "adrese ulaşılamadı", kaydedilmez.
4. Genel OpenAI-uyumlu sağlayıcı olarak **kendi backend'imizi** göster
   (`http://backend:8080/api/fake-openai` gibi bir test ucu yok — bunun yerine yerel bir
   sahte sunucu ile denenir) → modeller "bilinmiyor" rozetleriyle listelenir.
5. Modeller ekranında sağlayıcı filtresi çalışır; iki sağlayıcıdaki aynı isimli model
   ayrı satırlarda görünür.
6. Bir sağlayıcıyı sil → modelleri listeden kalkar, diğerininki durur.
7. Bitbucket erişimi ekle, kullanıcı adını boş bırak → kaydedilmez, eksik alan belirtilir.
8. Genel Git erişimi ekle → "doğrulanamadı" uyarısıyla birlikte kaydedilir.
9. `SELECT secret_enc FROM llm_providers;` → okunabilir metin değil.
10. `docker compose logs backend | grep -E 'sk-or|ATATT|ghp_'` → eşleşme yok.
11. `make down && make up` → sağlayıcılar ve katalog yerinde.
12. `make test` ve `make test-integration` yeşil, `make lint` temiz.
