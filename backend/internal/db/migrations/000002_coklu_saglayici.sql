-- +goose Up

-- Spec 002: kimlik bilgisi artık türün değil, SAĞLAYICININ özelliği.
-- Aynı anda birden fazla LLM ve git sağlayıcı tanımlı olabilir.

CREATE TYPE llm_provider_type AS ENUM ('openrouter', 'litellm', 'openai_compatible');
CREATE TYPE git_provider_type AS ENUM ('github', 'bitbucket', 'generic');

CREATE TABLE llm_providers (
    id         UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
    type       llm_provider_type NOT NULL,
    name       TEXT              NOT NULL,        -- kullanıcının verdiği ad
    slug       TEXT              NOT NULL UNIQUE, -- opencode provider kimliği
    base_url   TEXT              NOT NULL,
    secret_enc BYTEA             NOT NULL,        -- AES-256-GCM; asla düz metin
    hint       TEXT              NOT NULL,
    is_default BOOLEAN           NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ       NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ       NOT NULL DEFAULT now()
);

-- En fazla bir varsayılan olabilir. Kural veritabanında dayatılır ki eşzamanlı
-- iki güncelleme onu bozamasın.
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

-- Modeller artık bir sağlayıcıya ait; aynı model adı iki sağlayıcıda ayrı satırdır.
ALTER TABLE models ADD COLUMN provider_id UUID
    REFERENCES llm_providers(id) ON DELETE CASCADE;

-- Bağlam uzunluğu ve araç desteği bilinmeyebilir (LiteLLM/OpenAI-uyumlu
-- sağlayıcılar bu bilgiyi vermeyebilir). Fiyatlar NOT NULL kalır: bilinmeyen
-- fiyat sıfır sayılır ve model ücretsiz görünür (spec 002, kullanıcı kararı).
ALTER TABLE models
    ALTER COLUMN context_length DROP NOT NULL,
    ALTER COLUMN supports_tools DROP NOT NULL;

COMMENT ON COLUMN models.supports_tools IS
    'NULL = bilinmiyor. false ile karıştırılmamalı: agent olarak kullanılabilirliği belirler.';

-- Katalog durumu sağlayıcı başına tutulur: biri düşerse diğerleri etkilenmez.
CREATE TABLE provider_sync (
    provider_id     UUID PRIMARY KEY REFERENCES llm_providers(id) ON DELETE CASCADE,
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    model_count     INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT
);

-- ─── 001'den veri taşıma (spec 002 H6) ──────────────────────────────────────
-- Kullanıcının girdiği anahtarlar kaybolmadan yeni yapıya geçer.

INSERT INTO llm_providers (type, name, slug, base_url, secret_enc, hint, is_default)
SELECT 'openrouter', 'OpenRouter', 'openrouter',
       'https://openrouter.ai/api/v1', secret_enc, hint, true
FROM credentials WHERE kind = 'openrouter';

INSERT INTO git_providers (type, name, base_url, username, secret_enc, hint)
SELECT 'github', 'GitHub', '', '', secret_enc, hint
FROM credentials WHERE kind = 'github';

-- Jira credentials tablosunda kalır; git sağlayıcısı değildir.
DELETE FROM credentials WHERE kind IN ('openrouter', 'github');

-- Mevcut modeller taşınan sağlayıcıya bağlanır. Sağlayıcı yoksa (anahtar
-- yalnızca .env'deydi) katalog boşaltılır; açılışta bootstrap + senkron doldurur.
UPDATE models SET provider_id = (SELECT id FROM llm_providers ORDER BY created_at LIMIT 1);
DELETE FROM models WHERE provider_id IS NULL;

ALTER TABLE models ALTER COLUMN provider_id SET NOT NULL;
ALTER TABLE models DROP CONSTRAINT models_pkey;
ALTER TABLE models ADD PRIMARY KEY (provider_id, id);

INSERT INTO provider_sync (provider_id, model_count, last_success_at)
SELECT p.id, count(m.id), CASE WHEN count(m.id) > 0 THEN now() ELSE NULL END
FROM llm_providers p LEFT JOIN models m ON m.provider_id = p.id
GROUP BY p.id;

DROP TABLE catalog_sync;

-- +goose Down
-- DİKKAT: Bu geri alma şemayı 001'e döndürür ama TAŞINAN ANAHTARLARI GERİ YAZMAZ
-- ve model kataloğunu siler. Geri alma sonrası kimlik bilgileri yeniden girilmelidir.
-- Çift yönlü tam veri taşıma bilinçli olarak yazılmadı (spec 002 plan.md).

CREATE TABLE catalog_sync (
    id              BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    last_attempt_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    model_count     INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT
);
INSERT INTO catalog_sync (id) VALUES (true);

-- Bileşik anahtardan tekile dönerken çakışma olmasın diye katalog boşaltılır.
DELETE FROM models;
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
