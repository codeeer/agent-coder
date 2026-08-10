# Plan: Workflow Motoru

- **Spec no:** 007 — [spec.md](spec.md)
- **Durum:** Uygulandı

---

## En önemli karar: adım = çalıştırma

Bir akış adımı **zaten bir agent çalıştırmasıdır.** Bu yüzden adım için ayrı bir
"çalıştırma" kavramı üretilmiyor: her agent adımı `runs` tablosuna normal bir kayıt
yazar.

Sonuçları:

- **Rapor kodu değişmiyor.** Rapor `runs` üzerinden toplandığı için akış adımları
  kendiliğinden dahil olur. Ayrı bir tablo tutulsaydı rapor iki kaynağı birleştirmek
  zorunda kalır ve toplamların tutması bir bakım yüküne dönüşürdü.
- **Çalıştırmalar listesi bölünmüyor.** Geçmiş tek yerde durur.
- **Çalıştırma detayı, iptal, gönderme, canlı akış** olduğu gibi çalışır.

Ayrıca `workflow_steps` tablosu tutulur ama **yalnızca düğüm durumu** için: bir adım
hiç çalışmadıysa (`skipped`) ortada bir `runs` kaydı olmaz. `runs` tablosuna sahte
"atlandı" satırları yazmak raporu kirletirdi.

```
workflow_runs 1 ──< workflow_steps  ──0..1  runs
                    (node_id, status)        (gerçekten çalışan adım)
```

## Veri modeli

Migration `000004_workflow.sql`:

```sql
workflows          (id, project_id, name, description, is_active,
                    active_version_id, created_at, updated_at)
workflow_versions  (id, workflow_id, version, graph jsonb, created_at)
                   -- (workflow_id, version) UNIQUE
workflow_runs      (id, workflow_id, version_id, version,
                    status workflow_run_status, trigger_kind, trigger_payload jsonb,
                    input text, error, created_at, started_at, finished_at)
workflow_steps     (id, workflow_run_id, node_id, node_kind, status,
                    run_id UUID NULL REFERENCES runs(id) ON DELETE SET NULL,
                    error, started_at, finished_at)
                   -- (workflow_run_id, node_id) UNIQUE
workflow_hooks     (id, workflow_id UNIQUE, token TEXT UNIQUE, created_at)
```

`workflow_runs.version` anlık kopyadır (spec 003'teki kurala uygun): akış sonradan
değişse de geçmiş çalışma hangi sürümle çalıştığını gösterir. Ayrıca `version_id`
ile o sürümün grafına doğrudan erişilir.

`workflow_steps.status`: `pending | running | succeeded | failed | skipped | cancelled`.

Çalıştırmalar listesinde "bu kayıt X akışının Y adımı" bilgisi `runs`'a kolon
eklenerek DEĞİL, `workflow_steps` üzerinden LEFT JOIN ile gösterilir. Aynı bilgiyi iki
yerde tutmak, ikisinin ayrışması demektir.

## Graf modeli

`workflow_versions.graph`:

```jsonc
{
  "nodes": [
    { "id": "t1", "kind": "trigger.manual", "position": { "x": 0, "y": 0 } },
    { "id": "a1", "kind": "agent",
      "config": { "agentId": "…", "providerId": "…", "model": "…",
                  "prompt": "Şu görevi analiz et:\n{{ input }}" },
      "position": { "x": 0, "y": 120 } }
  ],
  "edges": [{ "from": "t1", "to": "a1" }]
}
```

`position` bu fazda kullanılmıyor ama **şimdiden saklanıyor**: Faz 4'te tuval
geldiğinde graf yapısı değişmesin. Tetikleyici de düğüm olarak modelleniyor — aynı
sebeple.

Düğüm türleri: `trigger.manual`, `trigger.webhook`, `agent`. Koşul ve HTTP düğümleri
kapsam dışı (K3), ama `kind` alanı açık uçlu olduğu için eklenmeleri migration
gerektirmez.

## Doğrulama — `internal/workflow/graph.go`

Kaydetme anında, kaydetmeden **önce**:

| Kural | Neden |
|---|---|
| Tam bir tetikleyici düğüm | Akışın nereden başladığı belirsiz kalmamalı |
| Tetikleyiciye gelen kenar yok | Tetikleyici giriş noktasıdır |
| Kenarlar var olan düğümlere işaret eder | Yazım hatası sessizce kalmasın |
| Döngü yok | Motor sonsuza kadar dönerdi |
| Her düğüm tetikleyiciden erişilebilir | Erişilemeyen adım hiç çalışmaz, kullanıcı bunu bilmeli |
| Agent düğümünde agent var, talimat boş değil | Çalıştırma anında değil, kaydetme anında söylenmeli |
| Şablon referansları ATA düğümlere işaret eder | `{{ steps.a2.output }}` daha çalışmamış bir adımı gösteriyorsa değer boş olurdu |

Son kural özellikle önemli: spec "sessizce boş metinle çalışmaz" diyor. En ucuz yeri
kaydetme anıdır.

## Şablon — `internal/workflow/template.go`

`{{ input }}`, `{{ trigger.<alan> }}`, `{{ steps.<düğüm>.output|diff|branch }}`.

Go `text/template` **kullanılmıyor**: düğüm kimlikleri tire içerebiliyor ve onun
söz dizimine uymuyor, hata mesajları da kullanıcıya gösterilecek kadar anlaşılır
değil. Yerine ~60 satırlık bir çözümleyici: `{{ ... }}` yakala, haritadan bak,
**bulunamazsa açık hata döndür.** Bilinmeyen referansı boş metne çevirmek, spec'in
yasakladığı sessiz yanlışlık olurdu.

## Motor — `internal/workflow/executor.go`

1. Graf topolojik sıraya konur, **seviyelere** bölünür.
2. Aynı seviyedeki düğümler `errgroup` ile paralel çalışır (kabul kriteri).
3. Her agent düğümü için talimat çözümlenir → `runs.Manager` üzerinden çalıştırılır.
4. Çıktı, diff ve branch akış bağlamına yazılır; sonraki seviye onu görür.
5. Bir adım başarısız olursa (K2) grup iptal edilir, kalan düğümler `skipped`.

**`runs.Manager`'a bloklayan bir giriş noktası ekleniyor.** Bugünkü `Start` kaydı
oluşturup arka planda çalıştırıp hemen dönüyor (HTTP isteği beklemesin diye). Motorun
ise adımın bitmesini beklemesi gerekiyor. Mevcut gövde ortak kalacak şekilde
`Execute(ctx, in) (Run, error)` ekleniyor — eşzamanlılık sayacı, timeout ve iptal
aynı yerden geçmeye devam eder. Böylece akış adımları da genel sınıra tabi olur
(kabul kriteri).

**İptal:** akış çalışması kendi context'ini taşır; iptal edilince süren adımın
context'i de düşer ve `runner` container'ı temizler (spec 003'teki garanti).

**Yeniden başlatma:** sunucu ölürse `RecoverInterrupted` akış çalışmalarını da
kapatır — yoksa arayüzde sonsuza kadar dönen akış kalır.

## HTTP

| Uç | İş |
|---|---|
| `GET/POST /api/workflows`, `GET/PATCH/DELETE /api/workflows/{id}` | CRUD |
| `POST /api/workflows/{id}/versions` | Yeni sürüm kaydet (doğrulamadan geçerse) |
| `POST /api/workflows/{id}/runs` | Elle başlat (giriş metniyle) |
| `GET /api/workflow-runs`, `GET /api/workflow-runs/{id}` | Liste ve detay (adımlarıyla) |
| `GET /api/workflow-runs/{id}/events` | Canlı akış (SSE) |
| `POST /api/workflow-runs/{id}/cancel` | Durdur |
| `POST /api/workflows/{id}/hook/rotate` | Tetikleme adresini yenile |
| `POST /hooks/{token}` | Dışarıdan tetikleme (S3: adres anahtardır) |

Webhook ucu `/api/` altında **değil**: dışarıya açılan tek uç orası olduğu için ayrı
durması, ileride farklı bir güvenlik politikası uygulanmasını kolaylaştırır.

Canlı akış mevcut `events.Bus` üzerinden gider; konu olarak akış çalışması kimliği
kullanılır. Adım olayları hem kendi çalıştırma kanalına hem akış kanalına düşer.

## Arayüz (tuval yok — K1)

| Sayfa | İçerik |
|---|---|
| `/workflows` | Liste, oluştur, sil; her akışın son çalışması |
| `/workflows/[id]` | **Adım listesi editörü**: adım ekle/sil/sırala, her adımda agent + model + talimat; şablon değişkenleri için yardım; kaydet → yeni sürüm |
| `/workflows/[id]/runs/[runId]` | Canlı ilerleme: adım adım durum, süre, maliyet; adımdan çalıştırma detayına bağlantı |
| `/runs` | Satırda "X akışının Y adımı" rozeti |

Adım listesi editörü doğrusal bir akış üretir (her adım bir öncekine bağlı). Graf
modeli paralelliği destekliyor ama bu fazda arayüzden kurulamıyor — tuvalin işi.

## Riskler

| Risk | Önlem |
|---|---|
| Motor bir adımı beklerken tıkanır | Adım timeout'u zaten var; akışın kendi context'i de iptal edilebilir |
| Akış tek başına bütün kapasiteyi yutar | Adımlar genel eşzamanlılık sayacından geçer |
| Şablon referansı sessizce boşa düşer | Kaydetme anında doğrulanır, çalışma anında hata verir |
| Paralel adımlar aynı depoya yazar | Her adım kendi container'ında kendi kopyasında çalışır; çakışma ancak gönderme anında olur, o da bu fazda elle |
| Rapor akışları saymaz | Adımlar `runs`'a yazdığı için otomatik dahil; entegrasyon testi bunu sınar |

## Doğrulama

1. Üç adımlı akış (analiz → kod → inceleme), her adım farklı modelle → uçtan uca çalışır
2. İkinci adımın talimatında `{{ steps.a1.output }}` gerçekten dolu gelir
3. Paralel iki adım aynı anda çalışır (zaman damgaları örtüşür)
4. Döngülü / erişilemez düğümlü / var olmayan agent'lı graf **kaydedilemez**
5. Kasıtlı hatalı adım → akış `failed`, sonrakiler `skipped`
6. Çalışan akış durdurulur → süren adım kesilir, container kalmaz
7. Webhook adresi çağrılınca akış başlar; yanlış adres 404
8. Rapor toplamları akış adımlarını kapsar ve çalıştırma listesiyle tutar
9. Sunucu yeniden başlatılınca yarım akış `interrupted` olur
