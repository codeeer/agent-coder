# Plan: <Özellik Adı>

- **Spec no:** NNN — [spec.md](spec.md)
- **Tarih:** YYYY-AA-GG
- **Durum:** Taslak | İnceleme | Onaylandı

> **Bu dosyanın kuralı:** **NASIL** yapılacağı. `spec.md` onaylanmadan bu dosya yazılmaz.
> Buradaki her başlık `spec.md`'deki bir hikâyeye veya kurala dayanmalı — dayanmıyorsa
> ya spec eksik ya da kapsam kayması var.

---

## Yaklaşım

Seçilen çözümün 3-5 cümlelik özeti. Neden bu yol?

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
|------------|------|------|-------|
| ... | ... | ... | Seçildi / Elendi — gerekçe |

---

## Veri Modeli

Yeni tablolar, değişen kolonlar, migration'lar.

```sql
-- backend/internal/db/migrations/NNNNNN_ad.sql
```

Geri alma (rollback) stratejisi: ...

## Arayüzler

### Go tipleri

```go
// backend/internal/<paket>/<dosya>.go
```

### HTTP API

| Metot | Yol | Gövde | Yanıt |
|-------|-----|-------|-------|
| ... | ... | ... | ... |

### Frontend tipleri

```ts
// frontend/src/lib/types.ts
```

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
|-------|------------|
| `backend/internal/...` | yeni / düzenleme — ne yapıyor |

## Yeniden Kullanılacak Mevcut Kod

Sıfırdan yazmak yerine kullanılacak mevcut fonksiyon ve paketler:

- `backend/internal/...` — ...

---

## Riskler

| Risk | Etki | Önlem |
|------|------|-------|
| ... | ... | ... |

## Test Stratejisi

- **Birim:** hangi paketler, hangi kenar durumlar
- **Entegrasyon:** hangi akış uçtan uca
- **Elle doğrulama:** adım adım, gözlenebilir sonucuyla

## Uygulama Sırası

Neyin neden önce geldiği — özellikle riskli parçaların erken doğrulanması.

1. ...
