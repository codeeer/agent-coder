# Görevler: Veri Katmanı ve Model Kataloğu

- **Spec no:** 001 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı (2026-08-09)

---

## Hazırlık

- [x] T01 `Makefile: env` hedefi `openssl rand -base64 32` ile gerçek şifreleme anahtarı üretsin → `make env` sonrası `.env` içindeki `SECRET_ENCRYPTION_KEY` örnek değer değil
- [x] T02 Go bağımlılıkları eklensin (`pgx/v5`, `goose/v3`) → `make tidy` hatasız, `make test` yeşil kalır

## Şifreleme

- [x] T10 `internal/secrets/cipher.go` — AES-256-GCM, `[sürüm][nonce][şifreli]` düzeni → derlenir
- [x] T11 `internal/secrets/cipher_test.go` — tur testi, yanlış anahtar, tek bit bozulma, geçersiz anahtar uzunluğu, boş girdi → `make test` yeşil

## Yapılandırma

- [x] T20 `config`: `DATABASE_URL` ve `SECRET_ENCRYPTION_KEY` zorunlu olsun, anahtar 32 bayta çözülmeli → eksikken sunucu net mesajla başlamaz
- [x] T21 `config` testleri güncellensin → `make test` yeşil

## Veri katmanı

- [x] T30 `internal/db/pool.go` — pgx havuzu, açılışta ping, yeniden deneme → derlenir
- [x] T31 `internal/db/migrations/000001_veri_katmani.sql` → `make migrate` hatasız
- [x] T32 `internal/db/migrate.go` — `embed.FS` + goose, açılışta çalışır → `make up` sonrası `\dt` üç tabloyu gösterir
- [x] T33 `cmd/migrate` ikilisi + `make migrate` / `make migrate-down` → up→down→up hatasız
- [x] T34 `/readyz` ucu (veritabanı ping) → DB kapalıyken 503, açıkken 200
- [x] T35 ~~sqlc yapılandırması~~ → **iptal, bkz. Sapma 2.** Yerine sorgular elle yazıldı ve gerçek Postgres'e karşı entegrasyon testleriyle doğrulandı (`make test-integration`)

## Kimlik bilgileri

- [x] T40 `internal/credentials/store.go` — `List` / `Reveal` / `Put` (upsert) / `Delete` → derlenir
- [x] T41 [P] `internal/openrouter/client.go` — `VerifyKey`, `ListModels`, `ErrUnauthorized` / `ErrUnreachable` → derlenir
- [x] T42 [P] `internal/credentials/validator.go` — openrouter, github, jira doğrulayıcıları → derlenir
- [x] T43 `internal/credentials/resolver.go` — veritabanı, yoksa `.env` sırası → birim testi geçer
- [x] T44 `httpapi/credentials.go` — `GET` / `PUT` / `DELETE` uçları → curl ile 200/422/503/204 doğrulanır
- [x] T45 Sızıntı testi: yanıt gövdelerinde gizli değer geçmiyor → `make test` yeşil
- [x] T46 `openrouter` istemci testleri (httptest: 200, 401, 500, bozuk JSON, zaman aşımı) → `make test` yeşil
- [x] T47 `credentials` testleri (maskeleme, ikinci `Put` değiştirir) → `make test` yeşil

## Model kataloğu

- [x] T50 `internal/catalog/syncer.go` — indir, rozet türet, tek transaction'da upsert + eskileri sil → derlenir
- [x] T51 Rozet türetme testleri (`is_free`, `supports_tools`, `is_preview`, boş `max_output_tokens`) → `make test` yeşil
- [x] T52 `Run` — açılışta bir kez + 24 saatte bir; hata açılışı engellemez → başarısız senkronla sunucu ayakta kalır
- [x] T53 `httpapi/models.go` — arama, filtre, sıralama, sayfalama + `stale` bayrağı → curl ile doğrulanır
- [x] T54 `POST /api/models/refresh` → 200 ve model sayısı döner

## Arayüz

- [x] T60 `@tanstack/react-query` kurulumu ve sağlayıcı → `npm run build` başarılı
- [x] T61 `lib/types.ts` + `lib/api.ts` yeni tipler ve uçlar → `npm run typecheck` temiz
- [x] T62 `components/settings/CredentialCard.tsx` — maskeli gösterim, ekle/değiştir/sil, onay → ekranda çalışır
- [x] T63 `app/settings/page.tsx` — üç kart, GitHub/Jira için "henüz kullanılmıyor" notu → ekranda çalışır
- [x] T64 `components/models/ModelTable.tsx` — rozetler, fiyat/bağlam sütunları, tıklanabilir sıralama başlıkları → ekranda çalışır. (Ayrı `ModelFilters.tsx` yazılmadı: filtreler dört satırlık durum tutuyor, ayrı dosyaya taşımak state'i prop olarak gezdirmekten başka bir şey getirmezdi.)
- [x] T65 `app/models/page.tsx` — arama, filtreler, sıralama, "Yenile", son güncelleme zamanı → ekranda çalışır
- [x] T66 Üç durum (yükleniyor / hata / boş) her iki ekranda da var → elle doğrulanır

## Doğrulama ve kapanış

- [x] T90 [plan.md](plan.md) doğrulama listesinin 11 adımı elle yürütülür
- [x] T91 `secret_enc` veritabanında okunabilir değil + backend loglarında `sk-or` geçmiyor
- [x] T92 `make test` yeşil, `make lint` temiz
- [x] T93 `spec.md` durumu "Uygulandı" yapılır
- [x] T94 `AGENTS.md` güncellenir (yeni komutlar, yeni paketler, durum)

---

## Notlar

### Sapma 1 — Go 1.24 → 1.25 (T02)

`pgx/v5` ve `goose/v3`'ün güncel sürümleri Go ≥ 1.25 istiyor. Alternatif, iki kütüphanenin
de eski sürümüne sabitlenmekti; yeni bir projede daha ilk günden geride başlamak anlamsız.
`go.mod`, iki backend Dockerfile'ı ve Makefile'daki araç imajı 1.25'e alındı.

### Sapma 2 — sqlc kullanılmıyor, pgx doğrudan (T35)

**Plan sqlc diyordu; vazgeçtim.** Gerekçe: bu fazın asıl sorgusu olan model listesi
**dinamik** — arama, iki isteğe bağlı filtre, üç sıralama alanı ve iki yön. sqlc'nin bunu
tek sorguda karşılama yolu `ORDER BY CASE WHEN @sort = 'name' THEN ...` kalıbı; hem okunaksız
hem de index kullanımını bozuyor. Alternatifi her kombinasyon için ayrı sorgu yazmak.

Bunun yerine sorgular `internal/credentials` ve `internal/catalog` içinde elle yazılıyor,
tarama `pgx.CollectRows` + `pgx.RowToStructByName` ile yapılıyor. Kod üretme adımı ve ek
araç bağımlılığı ortadan kalkıyor.

**Bedeli:** derleme zamanı SQL–struct doğrulaması kayboluyor. Karşılığında bu sorguları
gerçek Postgres'e karşı çalıştıran entegrasyon testleri yazılıyor (T47, T51) — uyumsuzluk
testte yakalanır.

`make sqlc` hedefi ve AGENTS.md'deki sqlc maddesi kaldırıldı. Söz verilen entegrasyon
testleri yazıldı: `internal/credentials/store_test.go` ve `internal/catalog/store_test.go`,
`make test-integration` ile çalışır.

### Sapma 3 — entegrasyon testleri ayrı hedefte, sırayla

Entegrasyon testleri `make test` içinde değil, ayrı `make test-integration` hedefinde.
İki nedeni var:

1. `make test` veritabanı olmadan da çalışabilmeli. `TEST_DATABASE_URL` tanımlı değilse
   testler `t.Skip` ile atlanır.
2. Test paketleri aynı veritabanını paylaşıyor. İlk denemede `catalog` ve `credentials`
   paketleri şemayı **aynı anda** kurmaya çalışıp çakıştı
   (`duplicate key value violates unique constraint "pg_type_typname_nsp_index"`).
   Çözüm iki katmanlı: `go test -p 1` ile paketler sırayla çalışıyor ve
   `testutil.TestDB` bağlantı + migration'ı süreç başına `sync.Once` ile bir kez yapıyor.

### Ek not — mevcut kurulumlarda `.env`

`make env` artık şifreleme anahtarını kendisi üretiyor, ama daha önce oluşturulmuş
`.env` dosyalarına dokunmuyor. Bu durumdaki kurulumlarda sunucu açık bir mesajla
başlamayı reddediyor. `make check-env` bunu `make up` öncesinde yakalıyor.
