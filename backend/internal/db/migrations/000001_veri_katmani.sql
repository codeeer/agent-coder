-- +goose Up

-- Kimlik bilgisi türleri. Enum kullanılıyor ki geçersiz bir tür veritabanına
-- hiç giremesin. Yeni tür eklemek: ALTER TYPE credential_kind ADD VALUE '...'
CREATE TYPE credential_kind AS ENUM ('openrouter', 'github', 'jira');

-- Tür başına tek kayıt (spec 001 kararı) — bu yüzden kind birincil anahtar.
CREATE TABLE credentials (
    kind       credential_kind PRIMARY KEY,
    secret_enc BYTEA          NOT NULL,  -- AES-256-GCM; asla düz metin
    hint       TEXT           NOT NULL,  -- arayüzde gösterilen son 4 karakter
    metadata   JSONB          NOT NULL DEFAULT '{}'::jsonb,  -- jira: base_url, email
    created_at TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ    NOT NULL DEFAULT now()
);

COMMENT ON COLUMN credentials.secret_enc IS
    'AES-256-GCM ile şifreli. internal/secrets dışında çözülmez, loglanmaz.';

-- OpenRouter model kataloğunun yerel kopyası.
CREATE TABLE models (
    id                TEXT           PRIMARY KEY,  -- "anthropic/claude-sonnet-4.5"
    provider          TEXT           NOT NULL,     -- id'nin ilk parçası
    name              TEXT           NOT NULL,
    description       TEXT           NOT NULL DEFAULT '',
    context_length    INTEGER        NOT NULL,
    max_output_tokens INTEGER,                     -- kataloğun bir kısmında boş
    prompt_price      NUMERIC(20,12) NOT NULL,     -- token başına USD
    completion_price  NUMERIC(20,12) NOT NULL,     -- token başına USD
    supports_tools    BOOLEAN        NOT NULL,
    is_free           BOOLEAN        NOT NULL,
    is_preview        BOOLEAN        NOT NULL,
    modality          TEXT           NOT NULL DEFAULT '',
    raw               JSONB          NOT NULL,     -- ileride gerekecek alanlar
    synced_at         TIMESTAMPTZ    NOT NULL DEFAULT now()
);

-- Agent olarak yalnızca araç destekleyen modeller kullanılabilir; bu filtre
-- her model seçim ekranında çalışacağı için kısmi index.
CREATE INDEX idx_models_tools ON models (supports_tools) WHERE supports_tools;
CREATE INDEX idx_models_provider ON models (provider);
CREATE INDEX idx_models_search ON models
    USING gin (to_tsvector('simple', id || ' ' || name));

-- Katalog senkron durumu. Tek satır tutulur; CHECK kısıtı ikinci satırı engeller.
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
