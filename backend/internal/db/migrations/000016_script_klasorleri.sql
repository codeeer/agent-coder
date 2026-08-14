-- +goose Up

-- Script klasörleri — standart upgrade kampanyaları (spec 022).
--
-- Bir kampanya (örn. "Node 18'den 24'e yükseltme") yedi adımdan oluşabiliyor
-- ve bu adımlar düz bir kütüphanede diğer kampanyaların script'leriyle
-- karışıyordu. Klasör, kampanyaya bir isim ve tek hamlede atanabilirlik
-- veriyor.
CREATE TABLE script_folders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- name doğrudan DİZİN ADINA dönüşür; script adıyla aynı dar karakter
    -- kümesine tabidir. Kullanıcının gördüğü ad ile agent'ın kullandığı yol
    -- aynı olmalı.
    name        TEXT NOT NULL UNIQUE,
    -- description agent'ın talimatına yazılır: KAMPANYANIN NE OLDUĞUNU model
    -- buradan öğrenir. Tek tek script açıklamaları bunu anlatamaz.
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Bir script EN FAZLA BİR klasörde. Kural burada, uygulama katmanında değil.
--
-- ON DELETE SET NULL bilinçli: klasör silinince içindeki script'ler silinmez,
-- klasörsüz kalır (spec 022 H5). Bir düzenleme kararının veri kaybına
-- dönüşmemesi için.
ALTER TABLE scripts
    ADD COLUMN folder_id UUID REFERENCES script_folders(id) ON DELETE SET NULL;

/*
 * BENZERSİZLİK — buradaki tuzak veritabanının kendisinde.
 *
 * Eski kural: `name` GLOBAL benzersiz. Yeni kural: aynı klasörde aynı ad
 * olamaz, farklı klasörlerde olabilir.
 *
 * Saf `UNIQUE (folder_id, name)` YETMEZ: Postgres iki NULL'ı birbirinden
 * FARKLI sayar. Yani klasörsüz iki script aynı adı alabilirdi — ve ikisi
 * container'da AYNI dosyaya (`/home/agent/scripts/<ad>.sh`) yazılırdı. Biri
 * diğerini sessizce ezerdi; hata ancak agent yanlış script'i çalıştırınca
 * görülürdü.
 *
 * `NULLS NOT DISTINCT` (Postgres 15+) klasörsüzleri de tek bir kümede
 * benzersiz tutar.
 */
ALTER TABLE scripts DROP CONSTRAINT scripts_name_key;

CREATE UNIQUE INDEX scripts_klasor_ad
    ON scripts (folder_id, name) NULLS NOT DISTINCT;

-- Klasör ataması, tekil script atamasından AYRI.
--
-- Klasörü "içindeki script'leri agent_scripts'e yazmak" olarak çözmek daha az
-- tablo demekti ama klasöre SONRADAN eklenen bir script o agent'ta geçerli
-- olmazdı (spec 022 H3). Çözüm çalıştırma anında yapılır.
CREATE TABLE agent_script_folders (
    agent_id  UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    folder_id UUID NOT NULL REFERENCES script_folders(id) ON DELETE CASCADE,
    PRIMARY KEY (agent_id, folder_id)
);

CREATE INDEX idx_agent_script_folders_folder ON agent_script_folders (folder_id);
CREATE INDEX idx_scripts_folder ON scripts (folder_id);

-- +goose Down
DROP TABLE agent_script_folders;
DROP INDEX scripts_klasor_ad;
DROP INDEX idx_scripts_folder;
ALTER TABLE scripts DROP COLUMN folder_id;
ALTER TABLE scripts ADD CONSTRAINT scripts_name_key UNIQUE (name);
DROP TABLE script_folders;
