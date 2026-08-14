-- +goose Up

-- Toplu çalıştırma — bir akışı çok projede sıraya koyma (spec 023).
--
-- Bu bir liste ekranı değil, bir KUYRUK. Eşzamanlılık sınırı dolduğunda
-- çalıştırma yöneticisi sıraya koymuyor, anında reddediyor: sınır 3 iken otuz
-- iş başlatılırsa üçü çalışır, yirmi yedisinin KAYDI BİLE OLUŞMAZ. Kuyruğun
-- veritabanında durmasının sebebi de bu — otuz projelik bir kampanya saatler
-- sürer ve o sürede bir yeniden başlatma olağandır; bellekteki kuyruk bekleyen
-- işleri sessizce yok ederdi.

-- Durumlar serbest metin değil ENUM (proje konvansiyonu). Zamanlayıcı öğe
-- durumunu koddan yazıyor; yazım hatası veritabanında sessizce durmak yerine
-- burada reddedilir.
CREATE TYPE run_batch_status AS ENUM ('queued', 'running', 'done', 'cancelled');

-- Öğe durumları akış çalışmasının durumlarıyla AYNI listeye benziyor ama ayrı
-- bir tip: `workflow_run_status`'a yeni bir değer eklendiğinde o değer kuyruk
-- için de geçerli sayılırdı. İki alanın bugün aynı görünmesi, yarın da aynı
-- kalacağı anlamına gelmez.
CREATE TYPE run_batch_item_status AS ENUM (
    'pending', 'running', 'succeeded', 'failed', 'interrupted', 'cancelled');

CREATE TABLE run_batches (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,

    -- Görev metni toplu işin KENDİSİNDE: otuz öğe aynı işi yapıyor. Öğe başına
    -- kopyalansaydı aynı metin otuz kez saklanır ve ekran hangisinin geçerli
    -- olduğunu sormak zorunda kalırdı.
    task        TEXT NOT NULL DEFAULT '',

    status      run_batch_status NOT NULL DEFAULT 'queued',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_run_batches_olusturma ON run_batches (created_at DESC);

CREATE TABLE run_batch_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id    UUID NOT NULL REFERENCES run_batches(id) ON DELETE CASCADE,
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,

    -- position: sıra EKLENME sırasıdır (spec → davranış kuralları). Öncelik
    -- yok; kullanıcı sırayı seçim yaparken kurar.
    position    INT NOT NULL,

    status      run_batch_item_status NOT NULL DEFAULT 'pending',

    -- Akış çalışması başlatıldıktan SONRA dolar; öncesinde NULL. Toplu iş bir
    -- çalıştırma değil — her öğe kendi akış çalışmasına bağlanır.
    workflow_run_id UUID REFERENCES workflow_runs(id) ON DELETE SET NULL,

    error       TEXT NOT NULL DEFAULT '',
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,

    -- Aynı proje aynı toplu işte iki kez yer almaz (spec → davranış kuralları).
    -- Kural burada, uygulama katmanında değil: seçim listesi bir gün başka bir
    -- ekrandan da beslenebilir.
    UNIQUE (batch_id, project_id)
);

-- Zamanlayıcının TEK sık sorgusu "sıradaki bekleyen hangisi". Kısmi index
-- yalnızca bekleyenleri tutar: tamamlanmış otuz bin öğe biriktiğinde de sorgu
-- bekleyenlerin sayısı kadar iş yapar.
CREATE INDEX idx_run_batch_items_bekleyen
    ON run_batch_items (batch_id, position) WHERE status = 'pending';

CREATE INDEX idx_run_batch_items_batch ON run_batch_items (batch_id, position);
CREATE INDEX idx_run_batch_items_project ON run_batch_items (project_id);

-- +goose Down
DROP TABLE run_batch_items;
DROP TABLE run_batches;
DROP TYPE run_batch_item_status;
DROP TYPE run_batch_status;
