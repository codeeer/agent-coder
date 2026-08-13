# Plan: Motor logları

- **Spec:** [spec.md](spec.md)
- **Durum:** Uygulandı

---

## Toplama

`defer` sırası **kritik**. LIFO çalıştığı için toplama satırı `ct.Remove`'dan
SONRA yazılır; böylece önce toplanır, sonra silinir.

```go
defer ct.Remove(context.WithoutCancel(ctx))          // 2. çalışır
if req.EngineLogs != nil {
    defer func() { req.EngineLogs(collect(...)) }()   // 1. çalışır
}
```

Context `WithoutCancel`: koşu iptal veya zaman aşımıyla bittiğinde `ctx`
kapalıdır; toplama onunla birlikte iptal olsaydı asıl ihtiyaç anında hiçbir
şey toplanmazdı.

## Üç kaynak

| Kaynak | Nereden | Ne anlatır |
|---|---|---|
| `stdout` | `docker logs` (stdcopy ile çözülür) | entrypoint, klonlama |
| `file` | `/home/agent/.local/share/opencode/log/` (tar kopyası) | sürücü çözümleme, izin kararları |
| `session` | `GET /session/{id}/message` | agent'ın konuşma ve araç geçmişi |

## Saklama

`run_engine_logs`: `run_id` (cascade), `source`, `content BYTEA` (gzip),
`raw_size`, `truncated`. `UNIQUE (run_id, source)` + `ON CONFLICT DO UPDATE`.

Süresi dolanlar 6 saatlik bir zamanlayıcıyla siliniyor.

## Ayarlar

| Anahtar | Tür | Varsayılan |
|---|---|---|
| `runner.engine_log_persist` | bool | `true` |
| `runner.engine_log_max_kb` | int | 2048 |
| `runner.engine_log_retention_days` | int | 7 |

## Arayüz

Koşu detayında **iki sekme**: "Çalıştırma" ve "Motor logları". Ham log
kaydırılan sütuna eklenseydi her koşuda gözün önünden geçerdi; oysa oraya
yalnızca bir şey ters gittiğinde bakılır.

Oturum kaynağı ayrıştırılabiliyorsa konuşma olarak gösterilir (rol, model,
maliyet başlıkta; akıl yürütme ve araç çağrıları katlanır bloklarda);
"Ham JSON" anahtarı ham metne döner.

## Değişen dosyalar

| Dosya | Ne |
|---|---|
| `backend/internal/db/migrations/000015_motor_loglari.sql` | tablo |
| `backend/internal/runner/enginelog.go` | tipler, `Redact`, `SecretsOf` |
| `backend/internal/runner/opencode/runner.go` | `collectEngineLogs` |
| `backend/internal/runner/opencode/client.go` | `sessionTranscript`, `trimStrings` |
| `backend/internal/runner/sandbox/docker.go` | `ReadDir`, `Logs` (stdcopy) |
| `backend/internal/runs/enginelog.go` | saklama, okuma, temizlik |
| `backend/internal/runs/manager.go` + `cmd/server/main.go` | log kanalı, purge zamanlayıcı |
| `backend/internal/httpapi/enginelogs.go` | `GET /api/runs/{id}/engine-logs` |
| `frontend/src/app/runs/[id]/page.tsx` | **sekme yapısı** (bkz. tasks.md Ölçüm 8) |
| `frontend/src/components/runs/EngineLogs.tsx` | sekme içeriği |
| `frontend/src/components/runs/SessionTranscript.tsx` | konuşma görünümü |

## Doğrulama

1. Gerçek koşu: container silindikten sonra loglar okunabiliyor
2. Bilerek sızdırılan sırlar saklanan içerikte görünmüyor
3. `Redact` çağrısı düşürülünce test **kırmızıya dönüyor** (mutasyon)
4. Kırpma, cascade, retention, gzip gidiş-dönüşü birim testleriyle
5. Tarayıcı: iki tema, telefon, konsol
