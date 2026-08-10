---
name: go-conventions
description: agent-coder backend'inde Go kodu yazarken, yeni paket veya endpoint eklerken, hata yönetimi/loglama/test yazarken kullanılır. Paket düzenini, hata sarmalama, slog ile loglama, pgx/sqlc kullanımı, HTTP handler ve test kalıplarını tanımlar.
---

# Go Konvansiyonları — agent-coder backend

## Paket düzeni

```
backend/
├── cmd/<binary>/main.go     sadece wiring: config oku, bağımlılıkları kur, sunucuyu başlat
└── internal/                tüm iş mantığı — dışarıdan import edilemez
```

`cmd/` içinde iş mantığı olmaz. `main.go` uzuyorsa mantık `internal/`'a taşınır.

Paketler yeteneğe göre adlandırılır, katmana göre değil: `workflow`, `runner`, `models`
— `utils`, `helpers`, `common`, `base` yasak.

## Hata yönetimi

Sarmalarken her zaman bağlam ekle ve `%w` kullan:

```go
if err != nil {
    return fmt.Errorf("workflow %s çalıştırılamadı: %w", id, err)
}
```

Çağıranın ayırt etmesi gereken hatalar sentinel olur:

```go
var (
    ErrWorkflowNotFound = errors.New("workflow bulunamadı")
    ErrGraphHasCycle    = errors.New("graph döngü içeriyor")
)
```

Hata mesajları küçük harfle başlar, sonunda nokta yoktur. `panic` sadece
programlama hatası için — istek yolunda asla.

## Loglama

`log/slog`, yapılandırılmış alanlarla:

```go
slog.InfoContext(ctx, "adım tamamlandı",
    "run_id", runID, "node_id", nodeID, "model", model,
    "tokens", usage.Total, "cost_usd", cost)
```

**Secret asla loglanmaz.** API key, git token, credential değeri — hiçbiri.
Log'a giden struct'ta secret alanı varsa maskele. Şüphe varsa loglama.

## Context

`ctx context.Context` her zaman ilk parametre. Struct alanı olarak saklanmaz.
Harici çağrılar (Docker, opencode, OpenRouter, Jira, GitHub) mutlaka `ctx`'i taşır
ki iptal ve timeout çalışsın.

## Veritabanı

`pgx/v5` + `sqlc`. Sorgular `backend/internal/db/queries/*.sql` içinde yazılır,
`sqlc generate` ile tip güvenli Go üretilir.

- Elle SQL string'i birleştirme yok — SQL injection ve sürdürülemez kod demek.
- Migration: `goose`, `backend/internal/db/migrations/NNNNNN_ad.sql`,
  `-- +goose Up` / `-- +goose Down` blokları ile. Detay: [db-migrations](../db-migrations/SKILL.md)
- Çok adımlı yazma işlemleri transaction içinde.

## HTTP katmanı

`chi` router. Handler'lar ince olur: parse → doğrula → servis çağır → yanıtla.
İş mantığı handler'da değil, ilgili `internal/` paketinde durur.

```go
func (h *Handler) createWorkflow(w http.ResponseWriter, r *http.Request) {
    var req CreateWorkflowRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "geçersiz gövde")
        return
    }
    wf, err := h.workflows.Create(r.Context(), req)
    if err != nil {
        h.respondErr(w, r, err)   // sentinel hatayı HTTP koduna çevirir
        return
    }
    respondJSON(w, http.StatusCreated, wf)
}
```

Hata yanıtı tek formatta: `{"error": {"code": "...", "message": "..."}}`.

## Eşzamanlılık

- Her goroutine'in bitişi tanımlı olmalı — `errgroup` veya `sync.WaitGroup` ile beklenir.
- Kaçak goroutine yasak; `go func(){}()` yazıyorsan kim bekliyor sorusunun cevabı olmalı.
- Paylaşılan state mutex'le korunur veya kanal üzerinden geçirilir.
- Runner container'ları gibi harici kaynaklar `defer` ile temizlenir; temizlik
  `context.Background()` (iptal edilmiş ctx ile silme çalışmaz) + kendi timeout'uyla yapılır.

## Test

`testify/require` (test akışını durdurur) tercih edilir; `assert` sadece devam
edilebilecek kontrollerde.

- Tablo testleri kenar durumlar için.
- Harici HTTP servisleri `httptest.NewServer` ile taklit edilir — gerçek opencode,
  OpenRouter, Jira veya GitHub'a test içinde çıkılmaz.
- Test adları davranışı anlatır: `TestGraph_DongulüGrafReddedilir`.
- DB testleri gerçek Postgres'e karşı, her test kendi transaction'ında ve sonunda rollback.

## Bağımlılıklar

Yeni bağımlılık eklemek bir karardır — `plan.md`'de gerekçelendirilir.
Standart kütüphane yeterliyse standart kütüphane kullanılır.

## Kritik sınırlar

**`internal/runner/` sızmaz.** `opencode`'a dair hiçbir tip, sabit veya varsayım bu
paketin dışına çıkmaz. Workflow motoru sadece `runner.Runner` arayüzünü bilir. Bu sınır,
ileride opencode'u kendi motorumuzla değiştirebilmemizin tek garantisi.

**Sağlayıcıya özel kod `internal/llm/` içinde kalır.** Sistem birden fazla LLM sağlayıcıyı
destekler (OpenRouter, LiteLLM proxy, OpenAI-uyumlu servisler). Hiçbir yere "OpenRouter"
varsayımı gömülmez; kod `llm.Client` arayüzüyle konuşur, tür ayrımı yalnızca
`llm.NewClient` fabrikasında yapılır. Aynısı git sağlayıcılar için `internal/gitprovider/`.

**"Bilinmiyor" ile "yok" karıştırılmaz.** Sağlayıcılar bazı bilgileri vermez. Bilinmeyen
bağlam uzunluğu ve araç desteği `*int` / `*bool` olarak nil taşınır — sıfır veya false
DEĞİL. Özellikle `SupportsTools == nil` "desteklemiyor" anlamına gelmez; öyle sayılırsa
meta veri vermeyen sağlayıcıların modelleri agent olarak hiç kullanılamaz.
(Fiyatlar bunun istisnası: bilinmeyen fiyat sıfır sayılır — bilinçli kullanıcı kararı.)
