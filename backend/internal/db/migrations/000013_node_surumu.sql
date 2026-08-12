-- +goose Up

-- Agent'ın koşturduğu build komutları belirli bir Node sürümü isteyebiliyor.
-- Sürüm artık koşu başlatılmadan önce seçilebiliyor; seçim yapılmazsa projenin
-- varsayılanı, o da boşsa taban runner imajı kullanılır.

-- Her koşuda elle seçmemek için projeye varsayılan.
-- Boş dize = "runner'ın kendi varsayılanı"; NULL kullanılmadı çünkü üç durum
-- değil iki durum var (seçilmiş / seçilmemiş) ve boş dize karşılaştırmayı
-- her yerde NULL kontrolü yazmaktan kurtarıyor.
ALTER TABLE projects ADD COLUMN default_node_version TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN projects.default_node_version IS
    'Bu projede varsayılan Node sürümü. Boşsa runner imajının kendi sürümü.';

-- Koşunun HANGİ sürümle çalıştığı kayda geçer.
--
-- `agent_prompt` ve `model_id` ile aynı gerekçe (spec 003 anlık kopya kararı):
-- projenin varsayılanı sonradan değişse bile geçmiş kayıt neyle koştuğunu
-- doğru göstermeli. Referans tutulsaydı geçmiş, bugünün ayarına göre yeniden
-- yazılmış gibi görünürdü.
ALTER TABLE runs ADD COLUMN node_version TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN runs.node_version IS
    'Çalıştırmanın koştuğu Node sürümü. Boşsa runner imajının kendi sürümü.';

-- +goose Down
ALTER TABLE runs DROP COLUMN node_version;
ALTER TABLE projects DROP COLUMN default_node_version;
