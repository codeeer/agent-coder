-- +goose Up

-- Spec 009: agent olmayan adımların sonucu.
--
-- Agent adımının çıktısı `runs` kaydında duruyor. PR ve Jira düğümlerinin ise
-- `runs` kaydı YOK (model çağırmıyorlar, rapor rakamlarını bozmasınlar diye).
-- Sonuçları saklanmayınca kullanıcı "PR aç ✓" görüyor ama PR'ın adresini
-- göremiyordu — en işe yarar bilgi kayboluyordu.
ALTER TABLE workflow_steps
    ADD COLUMN result_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN result_url  TEXT NOT NULL DEFAULT '';

COMMENT ON COLUMN workflow_steps.result_url IS
    'Adımın ürettiği adres (açılan PR, yazılan yorum). Agent adımlarında boş.';

-- +goose Down
ALTER TABLE workflow_steps DROP COLUMN result_url, DROP COLUMN result_text;
