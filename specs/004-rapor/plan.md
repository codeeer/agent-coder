# Plan: Rapor — Yönetici Özeti

- **Spec no:** 004 — [spec.md](spec.md)
- **Durum:** Uygulandı

---

## Yaklaşım

Rapor **türetilmiş veridir**: yeni tablo, yeni yazma yolu, yeni migration yok.
`runs` tablosu üzerinde agregasyon yapılır. Bu bilinçli bir karar:

- Özet tablo tutmak (materialized view / sayaç tablosu) **ikinci bir gerçek kaynağı**
  yaratırdı; geri dolduramadığımız bir hata olduğunda rakamlar sessizce yanlış kalırdı.
- Veri hacmi buna izin veriyor: çalıştırma sayısı günde onlarca mertebesinde ve
  `idx_runs_created` zaten var. Hacim büyürse önbellek eklemek sonradan mümkün.

## Backend

### `internal/reports`

Tek tip: `Store`. Yalnızca okur.

```go
func (s *Store) Summary(ctx, Query) (Summary, error)

type Query struct { Days int; ProjectID *uuid.UUID }
```

`Summary` altı parçadan oluşur ve **tek istekte** döner:

| Alan | İçerik |
|------|--------|
| `Totals` | dönem toplamları (iş, durum kırılımı, maliyet, token, dosya, +/− satır, branch, ortalama süre) |
| `Previous` | aynı uzunlukta bir önceki dönem — değişim oranı için |
| `Daily` | gün gün kırılım, **boş günler dahil** |
| `ByAgent` / `ByModel` / `ByProject` | kırılım satırları |
| `Failures` | tekrar eden hata metinleri |

Bölümleri ayrı uçlara bölmemenin sebebi tutarlılık: istekler arasında yeni bir
çalıştırma düşerse toplam ile kırılım birbirini tutmazdı.

### Sorgular

- **Dönem sınırı takvim günüdür.** "Son 30 gün" yöneticinin takvimindeki 30 gündür,
  720 saatlik kayan pencere değil. Sınırlar rapor saat diliminde hesaplanır.
- Dosya ve satır sayıları `files` jsonb'sinden `CROSS JOIN LATERAL
  jsonb_array_elements` ile çıkarılır — toplamlar hem satır hem dönem düzeyinde
  gerektiği için alt sorgu yerine lateral.
- Boş günler **Go tarafında doldurulur**: `generate_series` yerine, gün etiketinin
  üretildiği yerle doldurmanın yapıldığı yer aynı olsun diye.
- Ortalama süre yalnızca `started_at` ve `finished_at` dolu kayıtları sayar.

### HTTP

`GET /api/reports/summary?days=<1..365>&project=<uuid>` → `Summary`.
`days` verilmezse `reports.default_days` ayarı kullanılır.

### Ayarlar (yeni)

| Anahtar | Varsayılan | Neden ayar |
|---------|-----------|------------|
| `reports.default_days` | 30 | Kurumun bakış dönemi farklı olabilir |
| `reports.timezone` | Europe/Istanbul | Günlük kırılım hangi takvime göre bölünecek |

Tanınmayan saat dilimi raporu bozmaz, UTC'ye düşer.

## Frontend

`app/reports/page.tsx` + `components/charts/`. Grafik kütüphanesi **eklenmedi**;
ihtiyaç duyulan üç form elle yazılabilecek kadar basit ve paket yükü taşımıyor.

| Bileşen | Form | Neden |
|---------|------|-------|
| Kahraman rakam + kutucuklar | sayı | Toplam maliyet tek bir sayıdır; grafik onu anlatamaz |
| `RunsByDayChart` | yığılmış sütun | Zaman içinde parça-bütün (sonuç dağılımı) |
| `CostTrendChart` | alan + çizgi | Tek serinin zaman içindeki seyri |
| `BarList` | yatay çubuk | Kategori büyüklüğü karşılaştırması, uzun etiketler |
| `GroupTable` | tablo | Çok sütunlu, rakamların kendisi önemli |

Renk kuralları ve doğrulama sonuçları [AGENTS.md § Grafikler](../../AGENTS.md) içinde.
Özet: tek eksen, iki seriden itibaren gösterge zorunlu, durum renkleri temaya göre
değişmez, yığılmış grafiğin tablo görünümü zorunlu.

## Riskler

| Risk | Önlem |
|------|-------|
| Kayıt sayısı büyüyünce sorgu yavaşlar | `idx_runs_created` var; ölçülür, gerekirse önbellek |
| Saat dilimi yanlış girilir | UTC'ye düşer, rapor çalışmaya devam eder |
| Sarı "diğer" rengi açık temada okunmaz | Etiketli gösterge + zorunlu tablo görünümü |
| Rakamlar listeyle tutmaz | Entegrasyon testi günlük toplam = genel toplam eşitliğini sınar |

## Doğrulama

1. `make test-integration` → `internal/reports` yeşil
2. `GET /api/reports/summary?days=7` → dönem dışı kayıt sızmıyor
3. Arayüzde 7/30/90 gün geçişleri ve proje filtresi çalışıyor
4. Boş dönemde sayfa çökmüyor
5. Uç eksen etiketleri kırpılmıyor (1280 ve 1440 genişlikte ekran görüntüsüyle bakıldı)
