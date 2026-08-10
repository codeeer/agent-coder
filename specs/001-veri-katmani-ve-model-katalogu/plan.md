# Plan: Veri Katmanı ve Model Kataloğu

- **Spec no:** 001 — [spec.md](spec.md)
- **Tarih:** 2026-08-09
- **Durum:** İnceleme — onay bekliyor

---

## Yaklaşım

Üç bağımsız parça, riskli olandan başlayarak:

1. **Şifreleme** (`internal/secrets`) — dışa bağımlılığı olmayan, saf ve test edilebilir bir
   paket. Yanlış yapılırsa en pahalı hata burada olur, o yüzden önce ve tek başına yazılır.
2. **Kalıcılık** (`internal/db`) — bağlantı havuzu, migration'lar, tip güvenli sorgular.
   Üzerine `credentials` ve `models` depoları oturur.
3. **OpenRouter** (`internal/openrouter`) — kimlik doğrulama ve katalog indirme.
   Katalog senkronu açılışta bir kez, sonra günlük olarak arka planda çalışır.

Bunların üzerine ince bir HTTP katmanı ve iki ekran gelir.

### Faz 0'da doğrulanan gerçekler

Bu plan tahminlere değil, çalışan sisteme karşı yapılan ölçümlere dayanıyor:

| Ölçüm | Sonuç | Plana etkisi |
|---|---|---|
| `GET /api/v1/models/user` | 400 model, geçersiz anahtarla **401** | Katalog kaynağı bu olacak — hem kullanıcıya özel hem de anahtar doğrulaması işini görüyor |
| `GET /api/v1/models` (anahtarsız) | Aynı 400 model | Genel katalog; spec "kullanıcının erişebildiği" dediği için kullanılmayacak |
| Fiyatı sıfır olan modeller | 17 | "Ücretsiz" rozeti **fiyattan** türetilecek |
| `id` sonu `:free` olanlar | 14 | Sonek yetersiz — 3 ücretsiz model bu sonekle bitmiyor |
| `supported_parameters` içinde `tools` | 333 | "Araç destekli" rozeti buradan |
| `max_completion_tokens` boş olanlar | 49 | Kolon `NULL` kabul etmeli |

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
|---|---|---|---|
| Katalog kaynağı: `/models/user` | Kullanıcıya özel, anahtar doğrulaması yerleşik | Anahtar zorunlu | **Seçildi** — spec'in "erişebildiği modeller" kuralı bunu gerektiriyor |
| Katalog kaynağı: `/models` | Anahtarsız çalışır | Kullanıcıya özel değil | Elendi |
| Migration: açılışta gömülü çalıştırma | Ek servis yok, sıralama sorunu yok | Çok örnekli kurulumda yarış riski | **Seçildi** — sistem tek örnekli |
| Migration: ayrı tek seferlik compose servisi | Açılış ile migration ayrışır | Ek servis, `depends_on` zinciri uzar | Elendi (ana plandan sapma, aşağıda gerekçesi) |
| Şifreleme: uygulama tarafında AES-256-GCM | Anahtar veritabanından bağımsız, DB dökümünde secret yok | Anahtar kaybı = veri kaybı | **Seçildi** |
| Şifreleme: Postgres `pgcrypto` | Daha az kod | Anahtar SQL'e ve dolayısıyla loglara girer | Elendi |

### Ana plandan iki bilinçli sapma

1. **`users` ve `projects` tabloları bu fazda oluşturulmuyor.** Ana plan "şema yine de
   `user_id` taşır" diyordu. Bu fazın hiçbir hikâyesi kullanıcı kavramına ihtiyaç duymuyor;
   şimdi eklemek her sorguya kullanılmayan bir join getirir. Tek kullanıcılı ve verisi az bir
   sistemde `user_id` kolonunu sonradan eklemek önemsiz bir migration. `projects` Faz 3'te,
   workflow'larla birlikte gelecek.
2. **Migration ayrı servis değil, açılışta gömülü çalışıyor.** Backend tek örnek olduğu için
   yarış koşulu yok; buna karşılık `depends_on` zinciri kısalıyor ve "migration servisi
   bitmeden backend başladı" sınıfı hatalar tamamen ortadan kalkıyor. Elle kontrol için
   ayrı bir `cmd/migrate` ikilisi yine bulunacak (`make migrate`, `make migrate-down`).

---

## Veri Modeli

`backend/internal/db/migrations/000001_veri_katmani.sql`

```sql
-- +goose Up
CREATE TYPE credential_kind AS ENUM ('openrouter', 'github', 'jira');

-- Tür başına tek kayıt (spec kararı): kind birincil anahtar.
CREATE TABLE credentials (
    kind        credential_kind PRIMARY KEY,
    secret_enc  BYTEA       NOT NULL,          -- AES-256-GCM, asla düz metin
    hint        TEXT        NOT NULL,          -- maskeli gösterim, örn. "fd36"
    metadata    JSONB       NOT NULL DEFAULT '{}'::jsonb,  -- jira: base_url, email
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE models (
    id                TEXT           PRIMARY KEY,   -- "anthropic/claude-sonnet-4.5"
    provider          TEXT           NOT NULL,      -- id'nin ilk parçası
    name              TEXT           NOT NULL,
    description       TEXT           NOT NULL DEFAULT '',
    context_length    INTEGER        NOT NULL,
    max_output_tokens INTEGER,                      -- 49 modelde boş
    prompt_price      NUMERIC(20,12) NOT NULL,      -- token başına USD
    completion_price  NUMERIC(20,12) NOT NULL,
    supports_tools    BOOLEAN        NOT NULL,
    is_free           BOOLEAN        NOT NULL,
    is_preview        BOOLEAN        NOT NULL,
    modality          TEXT           NOT NULL DEFAULT '',
    raw               JSONB          NOT NULL,      -- ileride gerekecek alanlar için
    synced_at         TIMESTAMPTZ    NOT NULL DEFAULT now()
);

CREATE INDEX idx_models_tools    ON models (supports_tools) WHERE supports_tools;
CREATE INDEX idx_models_provider ON models (provider);
CREATE INDEX idx_models_search   ON models USING gin (to_tsvector('simple', id || ' ' || name));

-- Katalog senkron durumu — tek satırlık tablo.
CREATE TABLE catalog_sync (
    id              BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    model_count     INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT
);
INSERT INTO catalog_sync (id) VALUES (true);

-- +goose Down
DROP TABLE catalog_sync;
DROP TABLE models;
DROP TABLE credentials;
DROP TYPE credential_kind;
```

**Fiyat birimi:** OpenRouter token başına USD veriyor (`"0.000001"`). Veritabanına aynen
`NUMERIC(20,12)` olarak yazılır; milyon token başına dönüşüm yalnızca gösterim anında yapılır.
`float` kullanılmaz — para.

**Rozet türetme kuralları** (indirme anında hesaplanıp kolona yazılır):

- `is_free` = `prompt_price = 0 AND completion_price = 0` — `:free` soneki değil,
  çünkü 3 ücretsiz model o sonekle bitmiyor.
- `supports_tools` = `supported_parameters` dizisi `"tools"` içeriyor.
- `is_preview` = `id`'de `preview`, `-exp`, `experimental`, `alpha` veya `beta` geçiyor.
  **Bu bir sezgisel kural** — OpenRouter'ın böyle bir alanı yok. Yanlış pozitif riski var,
  kabul ediliyor: rozet bilgilendiricidir, hiçbir davranışı engellemez.

**Geri alma:** `Down` bloğu tabloları ve enum'u düşürür. Veri kaybı olur ama bu tablolarda
yalnızca yeniden üretilebilir katalog ve yeniden girilebilir kimlik bilgisi var.

---

## Arayüzler

### `internal/secrets`

```go
// Cipher, kimlik bilgilerini AES-256-GCM ile şifreler.
// Şifreli blob düzeni: [sürüm:1][nonce:12][şifreli metin + etiket]
// Sürüm baytı ileride anahtar döndürmeyi mümkün kılar.
type Cipher struct{ aead cipher.AEAD }

// NewCipher base64 kodlu 32 baytlık anahtardan cipher üretir.
// Anahtar eksik veya yanlış uzunluktaysa hata döner — sessizce zayıf moda düşmez.
func NewCipher(base64Key string) (*Cipher, error)

func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error)
func (c *Cipher) Decrypt(blob []byte) ([]byte, error)   // bozulmuş blob'da hata
```

### `internal/credentials`

```go
type Kind string   // "openrouter" | "github" | "jira"

// Credential, gizli değeri İÇERMEZ — listeleme ve API yanıtları bunu kullanır.
type Credential struct {
    Kind      Kind              `json:"kind"`
    Hint      string            `json:"hint"`       // "fd36"
    Metadata  map[string]string `json:"metadata"`   // jira: base_url, email
    UpdatedAt time.Time         `json:"updatedAt"`
}

type Store struct{ /* db + cipher */ }

func (s *Store) List(ctx context.Context) ([]Credential, error)
func (s *Store) Reveal(ctx context.Context, k Kind) (secret string, meta map[string]string, err error)
func (s *Store) Put(ctx context.Context, k Kind, secret string, meta map[string]string) error  // upsert
func (s *Store) Delete(ctx context.Context, k Kind) error

var ErrNotConfigured = errors.New("kimlik bilgisi tanımlı değil")
```

`Reveal` bilinçli olarak böyle adlandırıldı: çağrı yerleri okurken gizli değere
eriştikleri belli olsun. Yalnızca sunucu içinden çağrılır, asla HTTP yanıtına gitmez.

**Anahtar çözümleme sırası** — Faz 0 kurulumunun bozulmaması için:

```go
// OpenRouterKey önce veritabanına, yoksa .env değerine bakar.
func (r *Resolver) OpenRouterKey(ctx context.Context) (string, error)
```

### `internal/openrouter`

```go
type Client struct{ http *http.Client; baseURL string }

type KeyInfo struct {
    Usage      float64 `json:"usage"`
    Limit      *float64 `json:"limit"`
    IsFreeTier bool    `json:"is_free_tier"`
}

func (c *Client) VerifyKey(ctx context.Context, key string) (*KeyInfo, error)
func (c *Client) ListModels(ctx context.Context, key string) ([]Model, error)

var (
    ErrUnauthorized = errors.New("anahtar geçersiz")     // 401
    ErrUnreachable  = errors.New("servise ulaşılamadı")  // ağ / 5xx
)
```

Bu iki sentinel hata, spec'in hata tablosundaki "geçersiz anahtar" ile "ağ sorunu"
ayrımını taşır — kullanıcı ikisini farklı mesajla görür.

### `internal/credentials` doğrulayıcılar

```go
// Validator, kaydetmeden önce kimlik bilgisinin gerçekten çalıştığını sınar.
type Validator interface {
    Validate(ctx context.Context, secret string, meta map[string]string) error
}
```

| Tür | Doğrulama çağrısı |
|---|---|
| openrouter | `GET https://openrouter.ai/api/v1/key` |
| github | `GET {GITHUB_API_URL}/user` |
| jira | `GET {metadata.base_url}/rest/api/3/myself` (basic auth: email + token) |

GitHub ve Jira bu fazda kullanılmasa da doğrulanıyor: spec H2 "aynı doğrulamadan geçer"
diyor ve geçersiz bir token'ı Faz 5'te keşfetmek çok daha pahalı.

### `internal/catalog`

```go
type Syncer struct{ /* store, client, resolver, db */ }

// Sync katalogu indirir ve veritabanını günceller. Kaydedilen model sayısını döner.
func (s *Syncer) Sync(ctx context.Context) (int, error)

// Run açılışta bir kez, sonra 24 saatte bir Sync çağırır. ctx iptal edilene kadar çalışır.
func (s *Syncer) Run(ctx context.Context)
```

Senkron bir transaction içinde: yeni liste upsert edilir, listede olmayan eski satırlar
silinir, `catalog_sync` güncellenir. Başarısızlıkta eski katalog **olduğu gibi kalır** ve
`last_error` yazılır — spec'in "eski liste + uyarı" davranışı bunu gerektiriyor.

### HTTP API

| Metot | Yol | Gövde | Yanıt |
|---|---|---|---|
| GET | `/api/credentials` | — | `[{kind, hint, metadata, updatedAt}]` — gizli değer **yok** |
| PUT | `/api/credentials/{kind}` | `{secret, metadata?}` | 200 `{kind, hint, ...}` · 422 doğrulanamadı · 503 ulaşılamadı |
| DELETE | `/api/credentials/{kind}` | — | 204 · 404 |
| GET | `/api/models` | — | `{items, total, syncedAt, stale, lastError}` |
| POST | `/api/models/refresh` | — | 200 `{count, syncedAt}` · 503 |
| GET | `/readyz` | — | 200 · 503 (veritabanı erişilemezse) |

`/api/models` sorgu parametreleri: `q`, `tools=1`, `free=1`, `sort=name\|price\|context`,
`order=asc\|desc`, `limit` (varsayılan 50, azami 200), `offset`.

`PUT` yanıtı gönderilen gizli değeri **geri yansıtmaz** — yalnızca maskeli `hint`.

### Frontend tipleri

```ts
// frontend/src/lib/types.ts (mevcut dosyaya eklenir)
export type CredentialKind = "openrouter" | "github" | "jira";

export interface Credential {
  kind: CredentialKind;
  hint: string;
  metadata: Record<string, string>;
  updatedAt: string;
}

export interface Model {
  id: string;
  provider: string;
  name: string;
  contextLength: number;
  maxOutputTokens: number | null;
  promptPricePerMTok: number;      // gösterim birimi: milyon token başına USD
  completionPricePerMTok: number;
  supportsTools: boolean;
  isFree: boolean;
  isPreview: boolean;
}

export interface ModelList {
  items: Model[];
  total: number;
  syncedAt: string | null;
  stale: boolean;          // son senkron başarısızsa true
  lastError: string | null;
}
```

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
|---|---|
| `backend/internal/secrets/cipher.go` | yeni — AES-256-GCM |
| `backend/internal/db/{pool.go,migrate.go}` | yeni — pgx havuzu, gömülü goose |
| `backend/internal/db/migrations/000001_veri_katmani.sql` | yeni |
| `backend/internal/db/queries/{credentials,models}.sql` | yeni — sqlc kaynağı |
| `backend/internal/credentials/{store.go,validator.go,resolver.go}` | yeni |
| `backend/internal/openrouter/client.go` | yeni |
| `backend/internal/catalog/syncer.go` | yeni |
| `backend/internal/httpapi/{credentials.go,models.go,ready.go}` | yeni handler'lar |
| `backend/internal/httpapi/router.go` | düzenleme — yeni rotalar, `Handler`'a bağımlılıklar |
| `backend/internal/config/config.go` | düzenleme — `DATABASE_URL` ve şifreleme anahtarı artık **zorunlu** |
| `backend/cmd/server/main.go` | düzenleme — havuz kur, migration çalıştır, syncer başlat |
| `backend/cmd/migrate/main.go` | yeni — elle `up` / `down` |
| `frontend/src/app/settings/page.tsx` | düzenleme — yer tutucu yerine gerçek ekran |
| `frontend/src/app/models/page.tsx` | düzenleme — yer tutucu yerine gerçek ekran |
| `frontend/src/components/settings/CredentialCard.tsx` | yeni |
| `frontend/src/components/models/{ModelTable,ModelFilters}.tsx` | yeni |
| `frontend/src/lib/{api.ts,types.ts}` | düzenleme — yeni uçlar ve tipler |
| `Makefile` | düzenleme — `sqlc`, `migrate-down`; `env` gerçek anahtar üretsin |
| `deploy/docker-compose.yml` | düzenleme — backend'e `DB_*` değişkenleri |

### Yeniden kullanılacak mevcut kod

- `httpapi.respondJSON` / `respondError` — yeni handler'lar aynı hata gövdesini kullanır,
  yeni bir yanıt biçimi icat edilmez.
- `httpapi.Handler` ve `Routes()` — bağımlılıklar `Handler` alanlarına eklenir,
  ikinci bir router kurulmaz.
- `config.getString/getInt/getDuration` — yeni ayarlar aynı yardımcılarla okunur.
- `config.Load`'un "tüm sorunları tek seferde topla" deseni — yeni zorunlu alanlar
  aynı `problems` listesine eklenir.
- Frontend `apiFetch` ve `ApiError` — yeni çağrılar bunun üzerinden gider.
- Frontend `BackendStatus`'taki yükleniyor/hata/boş üçlü durum deseni — yeni ekranlar
  aynı deseni izler.

### Yeni bağımlılıklar

| Paket | Gerekçe |
|---|---|
| `github.com/jackc/pgx/v5` | Postgres sürücüsü ve havuzu |
| `github.com/pressly/goose/v3` | Migration; gömülü `embed.FS` ile çalışır |
| `sqlc` (araç, bağımlılık değil) | SQL'den tip güvenli Go üretimi |
| `@tanstack/react-query` | İki ekranda da veri çekme, önbellek, yeniden deneme |

---

## Riskler

| Risk | Etki | Önlem |
|---|---|---|
| `SECRET_ENCRYPTION_KEY` kaybolur/değişir | Kayıtlı tüm kimlik bilgileri çözülemez | Açılışta anahtar doğrulanır ve eksikse **başlamadan** açık mesajla durulur; `make env` gerçek anahtar üretir; blob'daki sürüm baytı ileride döndürmeye izin verir |
| Gizli değer log'a veya yanıta sızar | Anahtar sızıntısı | `Credential` tipi gizli değeri hiç taşımaz; erişim yalnızca `Reveal` ile; sızıntıyı yakalayan özel test yazılır |
| Önizleme rozeti sezgisel | Yanlış etiket | Rozet hiçbir davranışı engellemez, yalnızca bilgi; arayüzde ipucu metniyle açıklanır |
| OpenRouter açılışta erişilemez | Katalog boş kalır | Senkron başarısızlığı açılışı **engellemez**; eski katalog + `stale` bayrağı gösterilir |
| Katalog senkronu kısmi yazar | Tutarsız liste | Tek transaction: ya hepsi ya hiçbiri |
| `DATABASE_URL` artık zorunlu | Faz 0 kurulumu bozulabilir | compose zaten sağlıyor; `config.Load` eksikse net mesaj verir |

---

## Test Stratejisi

**Birim**

- `secrets` — şifrele/çöz turu; yanlış anahtarla çözme başarısız; **tek bit bozulan blob**
  başarısız; anahtar uzunluğu doğrulaması; boş girdi.
- `openrouter` — `httptest` ile sahte sunucu: normal katalog, 401 → `ErrUnauthorized`,
  500 → `ErrUnreachable`, bozuk JSON, boş gövde, zaman aşımı.
- `catalog` — rozet türetme (`is_free`, `supports_tools`, `is_preview`),
  `max_output_tokens` boş gelen kayıt, fiyat dönüşümü (token → milyon token).
- `credentials` — `hint` maskeleme; aynı türe ikinci `Put` **değiştirir**, ikinci satır
  oluşturmaz.
- `httpapi` — `PUT /api/credentials/{kind}` yanıtında ve `GET /api/credentials`
  gövdesinde gizli değerin **geçmediğini** doğrulayan test; filtre ve sıralama davranışı.

**Entegrasyon** (gerçek Postgres'e karşı, her test kendi transaction'ında, sonunda rollback)

- Migration `up` sonra `down` sonra tekrar `up` — hatasız.
- Kimlik bilgisi kaydet → yeniden oku → çöz → aynı değer.
- Katalog senkronu iki kez çalışır: ikinci çalışmada listeden çıkan model silinir.

**Elle doğrulama** — [aşağıdaki adımlar](#doğrulama).

---

## Uygulama Sırası

Riskli ve alt katmanda olan önce; her adım kendi başına doğrulanabilir.

1. `secrets` paketi + testleri — dışa bağımlılığı yok, en kritik parça.
2. `Makefile: env` gerçek şifreleme anahtarı üretsin; `config` bu anahtarı ve
   `DATABASE_URL`'i zorunlu kılsın.
3. `db` — pgx havuzu, gömülü goose, ilk migration, `/readyz`.
4. sqlc kurulumu + `credentials` ve `models` sorguları.
5. `credentials` deposu ve `Resolver` (veritabanı → `.env` sırası).
6. `openrouter` istemcisi + üç doğrulayıcı.
7. Kimlik bilgisi HTTP uçları.
8. `catalog` senkronu (açılış + günlük) ve model HTTP uçları.
9. Ayarlar ekranı.
10. Modeller ekranı.
11. Uçtan uca doğrulama, `AGENTS.md` ve `spec.md` durum güncellemesi.

Adım 1-3 bittiğinde sistem hâlâ ayakta ve `/readyz` yeşil olmalı; kullanıcıya görünen
değişiklik adım 9'da başlar.

---

## Doğrulama

1. `make clean && make env && make up` → üç servis healthy, `curl :8080/readyz` → 200
2. `make migrate` ve `make migrate-down` → hatasız; `make psql` içinde `\dt` üç tabloyu gösterir
3. Ayarlar ekranında **geçersiz** bir OpenRouter anahtarı gir → kaydedilmez,
   "anahtar doğrulanamadı" mesajı görünür
4. Geçerli anahtarı gir → kaydedilir, ekranda yalnızca `••••fd36` görünür
5. `make psql` → `SELECT secret_enc FROM credentials;` çıktısı **okunabilir metin değil**
6. `docker compose logs backend | grep -i "sk-or"` → **hiç eşleşme yok**
7. Modeller ekranı → 400 model, fiyatlar milyon token başına, 333'ünde "araç" rozeti,
   17'sinde "ücretsiz" rozeti
8. Arama kutusuna `haiku` yaz → liste daralır; "yalnızca araç destekleyenler" filtresi çalışır
9. `make down && make up` → anahtar ve katalog yerinde, yeniden indirme beklenmeden liste dolu
10. Ayarlardan anahtarı sil → modeller ekranı "önce anahtarınızı girin" yönlendirmesi gösterir
11. `make test` yeşil, `make lint` temiz
