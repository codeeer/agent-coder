-- +goose Up

-- Spec 011 Aşama 1: agent'ların erişebileceği MCP sunucuları.
--
-- Yalnızca UZAK sunucular (http/sse). Yerel (stdio) sunucular komutun
-- çalıştırma imajının içinde olmasını gerektirirdi; her yeni araç için imajı
-- değiştirmek, "yeni entegrasyon için kod yazma" sorununu çözmek yerine yer
-- değiştirmesi olurdu (spec 011 K2).
CREATE TYPE mcp_transport AS ENUM ('http', 'sse');

CREATE TABLE mcp_servers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- name araç adlarının ÖNEKİDİR: `sentry` sunucusunun `issue` aracı modele
    -- `sentry_issue` olarak görünür. Bu yüzden benzersiz ve dar karakterli.
    name       TEXT NOT NULL UNIQUE,
    transport  mcp_transport NOT NULL,
    url        TEXT NOT NULL,
    -- Anahtarsız çalışan sunucular var; secret_enc NULL olabilir.
    secret_enc BYTEA,
    hint       TEXT NOT NULL DEFAULT '',
    -- Son doğrulamada sunucunun bildirdiği araç adları.
    --
    -- Saklanıyor çünkü kullanıcı bir agent'a erişim verirken NEYE erişim
    -- verdiğini görmeli. Her ekran açılışında sunucuya gitmek hem yavaş olurdu
    -- hem de sunucu o an kapalıysa liste boş görünürdü.
    tools      JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Hangi agent hangi sunucuya erişebilir.
--
-- Erişim AGENT'A bağlı, adıma değil (spec 011 K1): dosya değiştirme ve komut
-- çalıştırma yetkileri de agent tanımında duruyor ve MCP erişimi aynı türden
-- bir yetenek. Adıma bağlansaydı "reviewer kod değiştiremez" güvencesi adım
-- adım denetlenmek zorunda kalırdı.
CREATE TABLE agent_mcp_servers (
    agent_id      UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    PRIMARY KEY (agent_id, mcp_server_id)
);

CREATE INDEX idx_agent_mcp_servers_server ON agent_mcp_servers (mcp_server_id);

-- +goose Down
DROP TABLE agent_mcp_servers;
DROP TABLE mcp_servers;
DROP TYPE mcp_transport;
