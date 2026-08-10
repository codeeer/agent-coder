-- +goose Up

-- Dışarıdan bir MCP istemcisiyle başlatılan çalışmalar ayırt edilebilmeli:
-- "bu akışı kim başlattı" sorusunun cevabı çalışma kaydında durmalı.
-- 'webhook' olarak yazılsaydı Claude Desktop'tan gelen iş, genel dış
-- tetiklemeden ayrılamazdı.
ALTER TYPE workflow_trigger_kind ADD VALUE IF NOT EXISTS 'mcp';

-- +goose Down

-- Enum'dan değer silinemez; tür yeniden kurulur.
ALTER TABLE workflow_runs ALTER COLUMN trigger_kind TYPE TEXT;
UPDATE workflow_runs SET trigger_kind = 'webhook' WHERE trigger_kind = 'mcp';
DROP TYPE workflow_trigger_kind;
CREATE TYPE workflow_trigger_kind AS ENUM ('manual', 'webhook', 'jira');
ALTER TABLE workflow_runs ALTER COLUMN trigger_kind TYPE workflow_trigger_kind
    USING trigger_kind::workflow_trigger_kind;
