---
name: db-migrations
description: agent-coder'da PostgreSQL şeması değiştirilirken kullanılır — tablo/kolon/index ekleme, goose migration yazma, sqlc sorgusu ekleme veya migration hatası giderme. Dosya adlandırma, Up/Down blokları, geri alınabilirlik ve şema konvansiyonlarını tanımlar.
---

# Veritabanı Migration'ları

Araçlar: **goose** (migration), **sqlc** (tip güvenli sorgu üretimi), **pgx/v5** (sürücü).

```
backend/internal/db/
├── migrations/   NNNNNN_ad.sql   — goose
├── queries/      *.sql           — sqlc kaynağı
└── sqlc/                         — ÜRETİLEN kod, elle düzenlenmez
```

## Migration yazma

Dosya adı: `NNNNNN_kisa_ad.sql` — altı haneli artan numara, snake_case.
Örnek: `000004_workflow_runs_ekle.sql`

```sql
-- +goose Up
CREATE TABLE workflow_runs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id  UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    status       run_status NOT NULL DEFAULT 'pending',
    context      JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_workflow_runs_workflow_id ON workflow_runs (workflow_id, created_at DESC);

-- +goose Down
DROP TABLE workflow_runs;
```

**Her migration geri alınabilir olmalı.** `Down` bloğu boş bırakılmaz; gerçekten geri
alınamayan bir durum varsa (veri kaybı) bunu yorumla açıkça yaz ve kullanıcıya sor.

Uygulanmış bir migration **düzenlenmez** — yeni migration yazılır. Sadece henüz
commit edilmemiş ve yalnızca yerelde uygulanmış migration düzeltilebilir.

## Şema konvansiyonları

| Konu | Kural |
|------|-------|
| Tablo adı | çoğul, snake_case: `workflow_runs` |
| Birincil anahtar | `id UUID PRIMARY KEY DEFAULT gen_random_uuid()` |
| Yabancı anahtar | `<tekil>_id`, `REFERENCES` ve açık `ON DELETE` davranışı ile |
| Zaman | her zaman `TIMESTAMPTZ`, asla `TIMESTAMP` |
| Zaman damgaları | `created_at` zorunlu; değişen tablolarda `updated_at` |
| Yapılandırılmamış veri | `JSONB`, `NOT NULL DEFAULT '{}'::jsonb` |
| Para | `NUMERIC(12,6)` — maliyetler için; `float` asla |
| Durum alanları | Postgres `ENUM` tipi (`run_status`), serbest metin değil |
| Boolean | `is_` öneki: `is_active` |

Index kuralı: her yabancı anahtara ve her sık sorgulanan filtre/sıralama kombinasyonuna
index. Sorgu `WHERE workflow_id = $1 ORDER BY created_at DESC` ise index bileşik olmalı.

Enum'a değer eklemek: `ALTER TYPE run_status ADD VALUE 'cancelled';`
Enum'dan değer çıkarmak mümkün değil — tipi yeniden oluşturmak gerekir, önce düşün.

## sqlc sorguları

`backend/internal/db/queries/<konu>.sql` içine yazılır:

```sql
-- name: GetWorkflowRun :one
SELECT * FROM workflow_runs WHERE id = $1;

-- name: ListRunsByWorkflow :many
SELECT * FROM workflow_runs
WHERE workflow_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: MarkRunFailed :exec
UPDATE workflow_runs
SET status = 'failed', error = $2, finished_at = now()
WHERE id = $1;
```

Dönüş türleri: `:one` (tam bir satır), `:many` (liste), `:exec` (dönüş yok),
`:execrows` (etkilenen satır sayısı gerekiyorsa).

Sorgu değiştirdikten sonra: `make sqlc` → üretilen kod derlenmeli.
`internal/db/sqlc/` altındaki dosyalar **elle düzenlenmez**, üretimden gelir.

## Akış

```bash
# 1. Migration yaz          backend/internal/db/migrations/NNNNNN_ad.sql
# 2. Uygula
make migrate
# 3. Doğrula
docker compose -f deploy/docker-compose.yml exec postgres \
  psql -U agentcoder -d agentcoder -c '\d workflow_runs'
# 4. Sorguları yaz          backend/internal/db/queries/*.sql
make sqlc
# 5. Derle
make test
```

Geri alma: `make migrate-down` (son migration'ı geri alır).

## Sık karşılaşılan hatalar

| Belirti | Sebep |
|---------|-------|
| `migration ... already applied` | Uygulanmış migration'ın içeriği değişmiş — yeni migration yaz |
| sqlc `column not found` | Migration uygulanmadan `make sqlc` çalıştırıldı — önce `make migrate` |
| `gen_random_uuid() does not exist` | `pgcrypto` yok — PostgreSQL 13+ ile yerleşik, sürümü kontrol et |
| Enum değeri kabul edilmiyor | Enum'a yeni değer eklenmemiş — `ALTER TYPE ... ADD VALUE` |

## Dikkat

`credentials.secret_enc` şifreli saklanır (AES-GCM, `internal/secrets`).
Bu kolona **asla düz metin yazılmaz** ve içeriği loglanmaz. Şemaya yeni bir secret
alanı eklenecekse aynı şifreleme yolundan geçmeli.
