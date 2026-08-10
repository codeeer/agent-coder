-- +goose Up

-- Spec 011 Aşama 3: Agent Coder'ın kendisinin MCP sunucusu olması.
--
-- Dışarıdan bir MCP istemcisi (Claude Desktop, Cursor…) akışları listeleyip
-- başlatabilir. Kimlik doğrulama v1'de yok; ADRESİN KENDİSİ anahtardır —
-- webhook uçlarındaki desenin aynısı (spec 007 S3).
--
-- Tek satırlık tablo: sunucu kurulum başına tek bir erişim adresi var.
-- `only_row` kısıtı ikinci satırın oluşmasını engelliyor; anahtarın iki farklı
-- yerde tutulup birinin unutulması mümkün olmasın.
CREATE TABLE mcp_access (
    only_row   BOOLEAN PRIMARY KEY DEFAULT true CHECK (only_row),
    token      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE mcp_access;
