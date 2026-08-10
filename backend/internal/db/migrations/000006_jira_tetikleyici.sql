-- +goose Up

-- Spec 009 Aşama 2: Jira tetikleyicinin tekrar-işleme koruması.
--
-- İki tetikleme yolu var (tarama ve webhook) ve ikisi aynı anda gelebilir.
-- Koruma UYGULAMA İÇİNDE bir kontrol olsaydı yarışa açık kalırdı: iki yol
-- aynı task'ı aynı anda okur, ikisi de "işlenmemiş" görür, akış iki kez başlar.
-- Bu yüzden koruma veritabanı kısıtıdır.
--
-- Anahtara issue_updated_at DAHİL: task Jira'da güncellenirse yeni bir satır
-- oluşur ve akış yeniden çalışır. Yalnızca (workflow_id, issue_key) olsaydı
-- güncellenen bir task bir daha hiç işlenmezdi.
CREATE TABLE workflow_processed_issues (
    workflow_id      UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    issue_key        TEXT NOT NULL,
    issue_updated_at TEXT NOT NULL,

    workflow_run_id UUID REFERENCES workflow_runs(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (workflow_id, issue_key, issue_updated_at)
);

-- Tarama durumu: kullanıcı "en son ne zaman baktı, ne oldu" görebilmeli.
CREATE TABLE workflow_scan_state (
    workflow_id UUID PRIMARY KEY REFERENCES workflows(id) ON DELETE CASCADE,
    scanned_at  TIMESTAMPTZ,
    found       INTEGER NOT NULL DEFAULT 0,
    started     INTEGER NOT NULL DEFAULT 0,
    error       TEXT
);

-- +goose Down
DROP TABLE workflow_scan_state;
DROP TABLE workflow_processed_issues;
