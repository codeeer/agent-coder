-- +goose Up

-- Jira tetiklemesiyle başlayan çalışmalar `trigger_kind = 'jira'` taşır.
--
-- Ayrı bir değer olmasının sebebi raporlama: "bu akış elle mi tetiklendi,
-- Jira'dan mı geldi" sorusunun cevabı çalışma kaydında durmalı. 'webhook'
-- olarak yazılsaydı Jira'dan gelen iş, genel dış tetiklemeden ayırt edilemezdi.
--
-- ADD VALUE bu geçişte yalnızca TANIMLANIR, kullanılmaz — PostgreSQL yeni
-- değerin aynı işlem içinde kullanılmasına izin vermez.
ALTER TYPE workflow_trigger_kind ADD VALUE IF NOT EXISTS 'jira';

-- +goose Down

-- Enum'dan değer silinemez; tür yeniden kurulur. Bu değeri taşıyan kayıtlar
-- 'webhook'a düşer — bilgi kaybı, ama geri alma yolunun tek seçeneği.
ALTER TABLE workflow_runs ALTER COLUMN trigger_kind TYPE TEXT;
UPDATE workflow_runs SET trigger_kind = 'webhook' WHERE trigger_kind = 'jira';
DROP TYPE workflow_trigger_kind;
CREATE TYPE workflow_trigger_kind AS ENUM ('manual', 'webhook');
ALTER TABLE workflow_runs ALTER COLUMN trigger_kind TYPE workflow_trigger_kind
    USING trigger_kind::workflow_trigger_kind;
