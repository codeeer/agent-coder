# Plan: Rapor Ekranı — Yönetici Bakışı

- **Spec:** [spec.md](spec.md) · **Görevler:** [tasks.md](tasks.md)

---

## Asıl engel: en somut çıktı yanlış tabloda

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

## Yeni ölçüler ve nereden geldikleri

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

## Ekran düzeni

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
bir rakam var (spec 012 K3).

**Uyarlanan kahraman:** `prsOpened` hem bu hem önceki dönemde sıfırsa kahraman
rakam "tamamlanan iş"e düşer. PR düğümü olmayan kurulumlarda dev bir sıfır,
sistemin çalışmadığı izlenimi verirdi.

## Dokunulacak dosyalar

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

## Riskler

| Risk | Önlem |
|---|---|
| PR sayısı tek başına yanıltıcı | Yanında batch size ve başarı oranı; merge takibi yapılmadığı ekranda yazılı |
| İki yeni sorgu raporu yavaşlatır | `idx_workflow_runs_created` ve `idx_workflow_steps_run` mevcut; tek istek kuralı korunur |
| PR'sız kurulumda dev sıfır | Kahraman rakam "tamamlanan iş"e düşer |
| Dipnotun sonradan silinmesi | Spec 012 K4: dipnot tasarımın parçası, süs değil |

## Doğrulama

1. `/reports` → kahraman **PR sayısı**, maliyet küçük puntoda ve bölünmüş
2. Ekrandaki PR sayısı `workflow_steps` sayımıyla birebir aynı
3. Proje ve dönem süzgeçleri tüm yeni rakamları birlikte değiştiriyor
4. PR'sız projede kahraman rakam "tamamlanan iş"e düşüyor
5. `make test`, `make test-integration`, `make lint`
6. `node scripts/theme-audit.mjs /reports` — iki temada 0 kalan
7. Dar ekranda sarma çalışıyor, yatay taşma yok
