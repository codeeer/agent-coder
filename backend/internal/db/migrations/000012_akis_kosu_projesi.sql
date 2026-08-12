-- +goose Up

-- Spec 007 (2026-08-12 kararı): akışın projesi artık VARSAYILAN, mühür değil.
--
-- Önce proje akışın kimliğine yazılıydı ve çalışma anında değiştirilemiyordu.
-- Aynı süreci yirmi projede işletmek isteyen kullanıcı yirmi ayrı akış
-- oluşturmak zorunda kalıyordu — tanım tekti, kayıt yirmi taneydi ve biri
-- güncellenip diğerleri unutuluyordu.
--
-- NEDEN KENDİ SÜTUNU: proje bugüne kadar `workflows`'tan JOIN ile okunuyordu.
-- Koşudan koşuya değişebildiği andan itibaren bu yanlış olur: akışın
-- varsayılanı sonradan değiştirilince GEÇMİŞ çalışmaların projesi de geriye
-- dönük değişirdi. Aynı sorun sürüm için zaten `workflow_runs.version_id` ile
-- çözülmüş; proje de aynı yere oturuyor.
ALTER TABLE workflow_runs
    ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;

-- Mevcut çalışmalar bugüne kadar akışın projesinde koştu; kayıt onu yazar.
UPDATE workflow_runs r
   SET project_id = w.project_id
  FROM workflows w
 WHERE w.id = r.workflow_id;

-- Doldurma bittikten SONRA zorunlu olur: sütun baştan NOT NULL eklenseydi
-- mevcut satırlar yüzünden migration hiç uygulanamazdı.
ALTER TABLE workflow_runs ALTER COLUMN project_id SET NOT NULL;

COMMENT ON COLUMN workflow_runs.project_id IS
    'Çalışmanın koştuğu proje. Akışın varsayılanı sonradan değişse bile değişmez.';

-- +goose Down
ALTER TABLE workflow_runs DROP COLUMN project_id;
