-- +goose Up

-- Spec 012: agent'ların çalıştırabileceği hazır kabuk betikleri.
--
-- Prosedür işlerinde (yükseltme, geçiş, kontrol listesi) modelin doğaçlaması
-- risktir: aynı akış iki kez çalıştığında iki farklı komut dizisi üretebilir.
-- Betik bir kez yazılır, gözden geçirilir ve her seferinde aynı çalışır.
CREATE TABLE scripts (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- name doğrudan DOSYA ADINA dönüşür (`<name>.sh`), o yüzden benzersiz ve
    -- dar karakterli. Sessiz dönüştürme yapılmaz; uygulama katmanı reddeder.
    name        TEXT NOT NULL UNIQUE,
    -- description agent'ın talimat dosyasına yazılır: betiğin NE ZAMAN
    -- çağrılacağını modele anlatan tek şey budur.
    description TEXT NOT NULL DEFAULT '',
    -- İçerik ŞİFRELENMEZ (spec 012 K5): container içinde zaten düz metin olarak
    -- duruyor ve agent okuyabiliyor. Şifrelemek yanlış bir güvenlik hissi
    -- verirdi. Gizli değerler betiğe değil ortam değişkenine konur.
    content     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Hangi agent hangi betiği çalıştırabilir.
--
-- Erişim AGENT'A bağlı, adıma değil (spec 012 K1): dosya değiştirme ve komut
-- çalıştırma yetkileri de agent tanımında duruyor; betik erişimi aynı türden
-- bir yetenek. MCP erişimindeki kararın aynısı.
CREATE TABLE agent_scripts (
    agent_id  UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    PRIMARY KEY (agent_id, script_id)
);

CREATE INDEX idx_agent_scripts_script ON agent_scripts (script_id);

-- +goose Down
DROP TABLE agent_scripts;
DROP TABLE scripts;
