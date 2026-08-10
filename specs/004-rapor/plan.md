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

---

## Revizyon — yönetici bakışı (2026-08-11)

### Asıl engel: en somut çıktı yanlış tabloda

Rapor bugün **yalnızca `runs`** üzerinden toplama yapıyor. PR açan düğüm model
çağırmadığı için `runs` kaydı üretmiyor — sonucu `workflow_steps.result_url`'e
yazıyor. Yani:

```
runs                     workflow_steps
├─ maliyet     ✅        ├─ node_kind = 'github.pr'   ← PR burada
├─ token       ✅        ├─ status    = 'succeeded'
├─ diff/files  ✅        └─ result_url = PR adresi
└─ süre        ✅
        ▲                        ▲
        └── rapor buraya bakıyor └── buraya bakmıyor
```

Bu, spec 003'te bilinçli alınmış bir kararın sonucu ("bu düğümler çalıştırma
sayılmaz, maliyeti yoktur") ve o karar doğru. Eksik olan, raporun ikinci
kaynağa da bakması.

### Yeni ölçüler ve nereden geldikleri

`Totals` üç alan kazanır. `Previous` aynı fonksiyondan üretildiği için önceki
döneme göre değişim **kendiliğinden** çalışır.

| Alan | Kaynak | Not |
|---|---|---|
| `prsOpened` | `workflow_steps` ⨝ `workflow_runs` ⨝ `workflows` | `node_kind='github.pr'`, `status='succeeded'` |
| `jiraTasks` | `workflow_processed_issues` ⨝ `workflow_runs` | çalışmaya bağlanmış kayıtlar |
| `runsWithCode` | mevcut `totals()` | `count(*) FILTER (r.diff <> '')` |

`Day` bir alan kazanır: `prsOpened` — kıvılcım grafiği için. Boş günleri Go
tarafında dolduran mevcut kalıp aynen kullanılır.

Kapsam (tarih + proje) `scope()` ile aynı mantıkta kurulur; proje süzgeci
`workflows.project_id` üzerinden uygulanır.

### Ekran düzeni

```
YÖNETİCİ
  12 PR açıldı        ↑ %20        ▁▂▃▅▇█  ← 30 günlük kıvılcım
  ────────────────────────────────────────────────────────────
  8 Jira task'ı    %86 başarı    ort. 74 satır    $0,004 / PR
  otomatik         24/28         PR başına        toplam $0,03
  ────────────────────────────────────────────────────────────
  ⓘ Açılan PR sayısıdır; birleştirilip birleştirilmediğini bu
    sistem takip etmiyor.

DETAY  (değişmiyor)
  Günlük çalıştırma · Günlük maliyet
  Agent · Proje · Model kırılımı · Tekrar eden hatalar
```

Dört destek rakamının seçimi rastgele değil: **1 sonuç, 1 güvenilirlik,
1 risk, 1 birim maliyet**. Hız gösteren her rakamın karşısında onu dengeleyen
bir rakam var (K4).

**Uyarlanan kahraman:** `prsOpened` hem bu hem önceki dönemde sıfırsa kahraman
rakam "tamamlanan iş"e düşer. PR düğümü olmayan kurulumlarda dev bir sıfır,
sistemin çalışmadığı izlenimi verirdi.

### Dokunulacak dosyalar

**Backend**
- `internal/reports/store.go` — üç yeni ölçü, `Day.prsOpened`, `Summary` alanları

**Frontend**
- `lib/types.ts` — `ReportTotals` + `ReportDay` alanları
- `app/reports/page.tsx` — yalnızca `Headline`; `Charts` ve `Breakdowns` aynı kalır
- `components/charts/Sparkline.tsx` (yeni) — eksensiz tek seri, `useWidth` ve
  token renkleriyle; yeni renk sabiti yok
- `components/charts/format.ts` — `formatPerUnit`

Yeniden kullanılanlar: `formatMoney`, `formatCount`, `formatPercent`,
`changeRatio`, `Delta`, `Stat`, `SubStat`.

### Riskler

| Risk | Önlem |
|---|---|
| PR sayısı tek başına yanıltıcı | Yanında batch size ve başarı oranı; merge takibi yapılmadığı ekranda yazılı |
| İki yeni sorgu raporu yavaşlatır | `idx_workflow_runs_created` ve `idx_workflow_steps_run` mevcut; tek istek kuralı korunur |
| PR'sız kurulumda dev sıfır | Kahraman rakam "tamamlanan iş"e düşer |
| Dipnotun sonradan silinmesi | K5: dipnot tasarımın parçası, süs değil |

### Doğrulama

1. `/reports` → kahraman **PR sayısı**, maliyet küçük puntoda ve bölünmüş
2. Ekrandaki PR sayısı `workflow_steps` sayımıyla birebir aynı
3. Proje ve dönem süzgeçleri tüm yeni rakamları birlikte değiştiriyor
4. PR'sız projede kahraman rakam "tamamlanan iş"e düşüyor
5. `make test`, `make test-integration`, `make lint`
6. `node scripts/theme-audit.mjs /reports` — iki temada 0 kalan
7. Dar ekranda sarma çalışıyor, yatay taşma yok
