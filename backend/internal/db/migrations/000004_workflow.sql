-- +goose Up

-- Spec 007: akışlar — agent adımlarını birbirine bağlayan workflow motoru.

CREATE TYPE workflow_run_status AS ENUM (
    'pending', 'running', 'succeeded', 'failed', 'cancelled', 'interrupted');

-- Adımın 'skipped' hali AKIŞA ÖZELDİR: bir önceki adım hata verince sonrakiler
-- hiç çalışmaz. Bu yüzden runs.status ile aynı liste değil.
CREATE TYPE workflow_step_status AS ENUM (
    'pending', 'running', 'succeeded', 'failed', 'skipped', 'cancelled');

CREATE TYPE workflow_trigger_kind AS ENUM ('manual', 'webhook');

CREATE TABLE workflows (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT true,

    -- Çalıştırılacak sürüm. Akış yeni oluşturulduğunda henüz sürümü yoktur.
    active_version_id UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflows_project ON workflows (project_id, created_at DESC);

-- Sürümler DEĞİŞMEZ: kaydedilen graf bir daha güncellenmez, yenisi eklenir.
-- Geçmiş bir çalışmanın hangi tanımla çalıştığı ancak böyle doğru kalır.
CREATE TABLE workflow_versions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    version     INTEGER NOT NULL,
    graph       JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workflow_id, version)
);

-- Sürüm silinemez: ona bağlı bir çalışma varsa geçmiş okunamaz hale gelirdi.
ALTER TABLE workflows
    ADD CONSTRAINT fk_workflows_active_version
    FOREIGN KEY (active_version_id) REFERENCES workflow_versions(id) ON DELETE SET NULL;

CREATE TABLE workflow_runs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    version_id  UUID NOT NULL REFERENCES workflow_versions(id) ON DELETE RESTRICT,

    -- ANLIK KOPYA: sürüm numarası burada da durur ki listeleme için birleştirme
    -- gerekmesin ve tanım değişse bile geçmiş doğru görünsün (spec 003 kuralı).
    version INTEGER NOT NULL,

    status          workflow_run_status NOT NULL DEFAULT 'pending',
    trigger_kind    workflow_trigger_kind NOT NULL,
    trigger_payload JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Kullanıcının akışa verdiği görev metni; {{ input }} bunu görür.
    input TEXT NOT NULL DEFAULT '',
    error TEXT,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE INDEX idx_workflow_runs_workflow ON workflow_runs (workflow_id, created_at DESC);
CREATE INDEX idx_workflow_runs_created ON workflow_runs (created_at DESC);
-- Açılışta yarım kalanları bulmak için kısmi index.
CREATE INDEX idx_workflow_runs_active ON workflow_runs (status)
    WHERE status IN ('pending', 'running');

-- Adım durumu.
--
-- Bir adım GERÇEKTEN çalıştıysa karşılığı bir runs kaydıdır (run_id). Hiç
-- çalışmadıysa (skipped) runs kaydı YOKTUR — rapor runs üzerinden toplandığı
-- için oraya sahte "atlandı" satırları yazmak rakamları kirletirdi.
CREATE TABLE workflow_steps (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_run_id UUID NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,

    node_id   TEXT NOT NULL,
    node_kind TEXT NOT NULL,
    node_name TEXT NOT NULL DEFAULT '',
    -- Seviye, paralel çalışan adımları gruplar; arayüz sıralamayı buradan yapar.
    level     INTEGER NOT NULL DEFAULT 0,

    status workflow_step_status NOT NULL DEFAULT 'pending',
    error  TEXT,

    -- Çalıştırma silinirse adım kaydı düşmez, yalnızca bağı kopar.
    run_id UUID REFERENCES runs(id) ON DELETE SET NULL,

    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,

    UNIQUE (workflow_run_id, node_id)
);

CREATE INDEX idx_workflow_steps_run ON workflow_steps (workflow_run_id, level);
-- Çalıştırma listesinde "bu kayıt hangi akışın adımı" sorusunu cevaplar.
CREATE INDEX idx_workflow_steps_run_id ON workflow_steps (run_id) WHERE run_id IS NOT NULL;

-- Dışarıdan tetikleme adresi. Kimlik doğrulama v1'de yok; ADRESİN KENDİSİ
-- anahtardır, bu yüzden tahmin edilemez olmalı ve yenilenebilmelidir.
CREATE TABLE workflow_hooks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL UNIQUE REFERENCES workflows(id) ON DELETE CASCADE,
    token       TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON COLUMN workflow_runs.version IS
    'Çalışma anındaki sürüm numarasının kopyası. Akış sonradan değişse bile değişmez.';
COMMENT ON TABLE workflow_versions IS
    'Değişmez sürümler: kaydedilen graf güncellenmez, yenisi eklenir.';

-- +goose Down
DROP TABLE workflow_hooks;
DROP TABLE workflow_steps;
DROP TABLE workflow_runs;
ALTER TABLE workflows DROP CONSTRAINT fk_workflows_active_version;
DROP TABLE workflow_versions;
DROP TABLE workflows;
DROP TYPE workflow_trigger_kind;
DROP TYPE workflow_step_status;
DROP TYPE workflow_run_status;
