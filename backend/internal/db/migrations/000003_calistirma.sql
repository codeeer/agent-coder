-- +goose Up

-- Spec 003: projeler, agent tanımları, çalıştırma ve çalışma ayarları.

CREATE TYPE run_status AS ENUM (
    'pending', 'running', 'succeeded', 'failed', 'cancelled', 'timeout', 'interrupted');
CREATE TYPE agent_source AS ENUM ('builtin', 'custom');

-- Yalnızca VARSAYILANDAN SAPAN ayarlar tutulur. Varsayılanlar ve geçerli aralıklar
-- Go tarafındaki kayıt defterinde durur; buraya yazılmazlar ki kod güncellendiğinde
-- yeni varsayılanlar kullanıcının dokunmadığı her ayarda kendiliğinden geçerli olsun.
CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    repo_url        TEXT NOT NULL,
    default_branch  TEXT NOT NULL DEFAULT 'main',
    -- Açık depolar için erişim gerekmez; erişim silinirse proje düşmez.
    git_provider_id UUID REFERENCES git_providers(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE agents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    prompt      TEXT NOT NULL,
    source      agent_source NOT NULL,

    -- Hazır agent'ların özgün hali; "sıfırla" bunları geri yükler.
    -- Kullanıcının oluşturduklarında NULL.
    builtin_prompt      TEXT,
    builtin_description TEXT,

    default_provider_id UUID REFERENCES llm_providers(id) ON DELETE SET NULL,
    default_model       TEXT NOT NULL DEFAULT '',

    -- Kaba yetkiler; çalıştırma motoruna session açılışında kural olarak gider.
    allow_edit     BOOLEAN NOT NULL DEFAULT true,
    allow_bash     BOOLEAN NOT NULL DEFAULT true,
    allow_webfetch BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE runs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- Çalıştırması olan agent silinemez: geçmiş kime ait olduğunu bilmeli.
    agent_id    UUID NOT NULL REFERENCES agents(id) ON DELETE RESTRICT,
    provider_id UUID REFERENCES llm_providers(id) ON DELETE SET NULL,

    -- ANLIK KOPYA: agent veya model sonradan değişse bile geçmiş doğru kalır.
    -- Referans tutulsaydı geçmiş sessizce yanlış görünürdü.
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
    -- Değişen dosyaların özeti: [{file, additions, deletions, status}]
    files  JSONB NOT NULL DEFAULT '[]'::jsonb,

    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd          NUMERIC(12,6) NOT NULL DEFAULT 0,

    pushed_branch TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ
);

CREATE INDEX idx_runs_project ON runs (project_id, created_at DESC);
CREATE INDEX idx_runs_created ON runs (created_at DESC);
-- Açılışta yarım kalan kayıtları bulmak için kısmi index.
CREATE INDEX idx_runs_active ON runs (status) WHERE status IN ('pending', 'running');

CREATE TABLE run_events (
    run_id  UUID    NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    seq     INTEGER NOT NULL,
    ts      TIMESTAMPTZ NOT NULL DEFAULT now(),
    level   TEXT    NOT NULL,
    message TEXT    NOT NULL,
    PRIMARY KEY (run_id, seq)
);

COMMENT ON COLUMN runs.agent_prompt IS
    'Çalıştırma anındaki agent talimatının kopyası. Agent sonradan düzenlense bile değişmez.';

-- +goose Down
DROP TABLE run_events;
DROP TABLE runs;
DROP TABLE agents;
DROP TABLE projects;
DROP TABLE settings;
DROP TYPE agent_source;
DROP TYPE run_status;
