-- +goose Up

-- Motorun ham logları — koşu bitince yok olmasın.
--
-- Runner container'ı geçicidir: iş biter bitmez silinir ve opencode'un asıl
-- teşhis bilgisi (kendi log dosyaları + container stdout/stderr) onunla
-- birlikte gider. Bugüne kadar kök neden analizi ancak koşu SIRASINDA
-- `docker exec` ile mümkündü; kullanıcıdan bu beklenemez.
--
-- SATIR SATIR DEĞİL, kaynak başına TEK BLOB. Milyonlarca satırlık bir tablo
-- kurmanın karşılığı yok: bu içerik sorgulanmıyor, bütün olarak okunuyor.
-- Satır bazlı saklamak hem yazma maliyetini hem tablo boyutunu koşu başına
-- yüzlerce katına çıkarırdı.
CREATE TABLE run_engine_logs (
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Koşu silinince logu da gider: yetim kayıt kalmaz (koşu silme, spec 003
    -- 2026-08-12 kararı).
    run_id  UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,

    -- 'stdout'  → container'ın stdout/stderr'i
    -- 'file'    → opencode'un kendi log dosyaları
    -- 'session' → agent'ın tam konuşma ve araç geçmişi
    source  TEXT NOT NULL,

    -- İçerik GZIP'li. Metin olarak tutulsaydı tipik bir koşu için tablo
    -- birkaç megabayt büyürdü; bu veri okunmaktan çok saklanıyor.
    content BYTEA NOT NULL,

    -- Sıkıştırılmamış boyut: arayüz "2,1 MB" diyebilsin diye. `length(content)`
    -- sıkıştırılmış boyutu verir ve kullanıcıya yanlış gelir.
    raw_size INTEGER NOT NULL,

    -- Baştan kırpıldıysa true. Hata genelde SONDA olduğu için son kısım
    -- korunur; kullanıcı eksik bir metne baktığını bilmeli.
    truncated BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Bir koşunun aynı kaynaktan iki logu olamaz.
    UNIQUE (run_id, source)
);

-- Temizlik işi yaşa göre siliyor; koşu detayı da run_id ile okuyor.
CREATE INDEX idx_run_engine_logs_created ON run_engine_logs (created_at);

COMMENT ON COLUMN run_engine_logs.content IS
    'gzip''li ham log. Gizli değerler yazılmadan önce maskelenir.';

-- +goose Down
DROP TABLE run_engine_logs;
