# Görevler: Rapor Ekranı — Yönetici Bakışı

- **Spec no:** 012 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Uygulandı

---

## Ölçüler

- [x] T01 `reports.Totals` → `prsOpened` (`workflow_steps` ⨝ `workflow_runs`)
- [x] T02 `reports.Totals` → `jiraTasks` (`workflow_processed_issues`)
- [x] T03 `reports.Totals` → `runsWithCode` (`runs.diff <> ''`)
- [x] T04 `reports.Day` → `prsOpened`; boş günler mevcut kalıpla doldurulur
- [x] T05 Proje süzgeci yeni sorgularda da çalışır (`workflows.project_id`)
- [x] T06 Testler: yeni ölçüler gerçek veriyle doğrulanır

## Arayüz

- [x] T10 `ReportTotals` / `ReportDay` tipleri
- [x] T11 `Sparkline` bileşeni — eksensiz, tek seri, token renkleri
- [x] T12 `formatPerUnit` biçimlendirici
- [x] T13 `Headline` yeniden yazılır: kahraman PR, dört denge rakamı, dipnot
- [x] T14 Uyarlanan kahraman: PR yoksa "tamamlanan iş"
- [x] T15 `Charts` ve `Breakdowns` DEĞİŞMEZ (detay katmanı)

## Doğrulama

- [x] T40 [plan.md](plan.md) doğrulama listesi 1–7
- [x] T41 Ekrandaki PR sayısı veritabanı sayımıyla birebir aynı

## Kapanış

- [x] T90 `AGENTS.md` kuralı; `specs/004-rapor/spec.md`'ye kararın neden
      değiştiği notu (spec silinmez — değişimin kendisi kayıt)

---

### T40/T41 — Doğrulama sonuçları

Gerçek veriyle, ayakta duran kurulumda ölçüldü.

| # | Adım | Sonuç |
|---|------|-------|
| 1 | Kahraman rakam PR, maliyet küçük ve bölünmüş | ✓ `7` · `$0,01 / PR` · not: `toplam $0,09` |
| 2 | Ekrandaki PR sayısı = veritabanı sayımı | ✓ 7 = 7 |
| — | Jira task'ı ve kod üreten çalıştırma | ✓ 1 = 1, 9 = 9 |
| — | Günlük seri toplamı = genel toplam | ✓ 7 = 7 |
| 3 | Dönem ve proje süzgeçleri | ✓ 7/30/90 ve proje değişimi tüm rakamları birlikte değiştiriyor |
| 4 | PR'sız projede uyarlanan kahraman | ✓ Hello World → "TAMAMLANAN İŞ 15" |
| 5 | Testler | ✓ 20 birim + 20 entegrasyon paketi |
| 6 | Tema denetimi | ✓ 74 kontrol, 0 kalan, eşlik hatası yok |
| 7 | Dar ekran | ✓ yatay taşma 0 |

### Ölçüm 1 — en somut çıktı yanlış tablodaydı

Rapor kurulduğundan beri yalnızca `runs` tablosuna bakıyordu. Sistemin ürettiği
en somut şey — açılan pull request — orada **yok**: PR açan düğüm model
çağırmadığı için çalıştırma kaydı üretmiyor (spec 003'te bilinçli alınmış bir
karar, ve o karar doğru).

Sonuç: ekran üç kuruşluk harcamayı 48 punto ile gösterirken, sistemin asıl
çıktısını hiç gösteremiyordu. Rapor artık ikinci kaynağa da bakıyor.

**Ders:** bir ekranın "neyi gösteremediği", gösterdiği şey kadar bilgi taşır.
Veri kaynağı seçimi, farkında olmadan neyin ölçülebileceğini de belirliyor.

### Ölçüm 2 — araştırma ilk içgüdüyü düzeltti

"PR sayısını kahraman yap" ilk kararımdı. Literatür bunu **tek başına**
yanıltıcı sayıyor: bir ölçümde geliştirici başına merge edilen PR %98 artarken
PR başına olay sayısı %242 artmış ve organizasyon düzeyinde teslimat düz
kalmış. Yani PR sayısı ilerleme gibi görünüp gerilemeyi gizleyebiliyor.

Bu yüzden yanına **PR başına değişen satır** kondu: büyük değişiklik kümeleri
riskle en tutarlı ilişkilendirilen ölçü ve elimizde ölçülmüş veri var.

Kural olarak yazıldı (spec 012 K3, `AGENTS.md`): **hız metriği asla yalnız
gösterilmez.**

### Ölçüm 3 — test fikstürü olmayan bir kolona yazıyordu

Yeni testler ilk çalıştırmada `column "hook_token" of relation "workflows" does
not exist` verdi. Fikstürü şemayı okumadan, hatırladığım alan adıyla yazmıştım;
`hook_token` başka bir tabloda.

Küçük bir hata ama sınıfı tanıdık: **hatırlanan şema, okunan şema değildir.**
