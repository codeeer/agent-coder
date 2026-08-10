# Görevler: Rapor — Yönetici Özeti

- **Spec no:** 004 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı

---

## Ayarlar

- [x] T01 `settings/registry.go` — `reports.default_days` (30) ve `reports.timezone`
      (Europe/Istanbul); yeni `reports` grubu → `GET /api/settings` ikisini listeliyor

## Backend

- [x] T02 `internal/reports/store.go` — `Summary`, `Totals`, `Day`, `Group`, `Failure`
      tipleri; takvim günü sınırları, saat dilimi çözümü → derlenir
- [x] T03 Toplam sorgusu — durum kırılımı, maliyet, token, `files` jsonb'sinden dosya ve
      +/− satır, gönderilen branch, ortalama süre
- [x] T04 Günlük kırılım — boş günler dolduruluyor, tarih rapor saat diliminde
- [x] T05 Agent / model / proje kırılımları ve tekrar eden hatalar
- [x] T06 `httpapi/reports.go` + route + `main.go` wiring → `GET /api/reports/summary`
- [x] T07 Entegrasyon testleri: toplamlar, dönem dışı kaydın sızmaması, boş gün doldurma,
      günlük toplam = genel toplam, önceki dönem, boş dönem (nil değil boş dilim),
      varsayılan dönem ayarı, hata kırılımı, geçersiz saat diliminin UTC'ye düşmesi
      → `make test-integration` yeşil

## Arayüz

- [x] T10 `lib/types.ts` + `lib/api.ts` — rapor tipleri ve `api.reports.summary`
- [x] T11 `components/charts/format.ts` — para, sayı, süre, yüzde, gün etiketi;
      eksen için tek basamak seçimi
- [x] T12 `components/charts/chrome.tsx` — gösterge, genişlik ölçümü, yuvarlak eksen
      değerleri, etiket seyreltme, ipucu kutusu
- [x] T13 `RunsByDayChart` + tablo görünümü — yığılmış sütun, ipucunda tam kırılım
- [x] T14 `CostTrendChart` — alan + çizgi, imleç çizgisi, uç etiket hizalaması
- [x] T15 `BarList` — yatay çubuk, tek renk (büyüklüğü uzunluk taşır)
- [x] T16 `app/reports/page.tsx` — dönem/proje filtresi, kahraman rakam, kutucuklar,
      iki grafik, agent/proje çubukları, model tablosu, hata listesi
- [x] T17 Sidebar'a "Rapor" + `IconReport`
- [x] T18 Grafik renk jetonları `globals.css` — durum paleti temaya göre değişmez
- [x] T19 Doğrulama: `npm run typecheck` ve `npm run lint` temiz; 1280 ve 1440 genişlikte
      ekran görüntüsü alındı, başlık taşması ve eksen etiketi kırpılması düzeltildi

## Yan iş: ölü ortam değişkenleri temizlendi

Rapor sayfası "parametre nerede duruyor" sorusunu tekrar gündeme getirdi ve şu kusur
görüldü: `RUNNER_TIMEOUT_SEC`, `RUNNER_MAX_CONCURRENCY`, `RUNNER_CPU_LIMIT`,
`RUNNER_MEMORY_LIMIT` hâlâ `config.go`, `.env.example` ve compose içinde duruyordu ama
**hiçbir yerde okunmuyordu** — çalıştırma sınırları spec 003 H7 ile ayarlara taşınmıştı.
`.env` içindeki bu satırları değiştiren kullanıcı hiçbir etki görmeyecekti.

- [x] T20 Dördü `config.RunnerConfig`, `.env.example` ve `docker-compose.yml` içinden
      kaldırıldı; `RunnerConfig` yalnızca dağıtım topolojisini (imaj, ağ, parola) tutuyor
- [x] T21 `config_test.go` bu alanlar yerine imaj/ağ üzerinden doğrulama yapacak
      şekilde güncellendi → `make test` yeşil

---

## Notlar

### Ölçüm 1 — durum paleti tek başına yetmiyordu

Dört durum rengi (başarılı / iptal / zaman aşımı / başarısız) yığın olarak
doğrulayıcıdan geçirildiğinde "zaman aşımı" ile "iptal" çifti normal görüşte ΔE 13,6
ölçtü — eşik 15. Yani tam renk gören biri bile ikisini ayırmakta zorlanırdı.

Çözüm rengi değiştirmek değil, **grafiği sadeleştirmek** oldu: yığın üç parçaya indi
(başarılı / diğer / başarısız, ΔE 27,6) ve beş durumun tam kırılımı ipucunda ve tabloda
verildi. Grafik anlatır, tablo kanıtlar.

### Ölçüm 2 — `w-full` dışarıdan verilen genişliği yeniyordu

Başlıktaki proje seçicisine `w-44` verildi ama `Select` bileşeninin kendi sınıf dizisi
`w-full` içeriyor; Tailwind'de kazananı sınıf dizisindeki sıra değil üretilen CSS'in
sırası belirler. Seçici tüm satırı kaplayıp dönem düğmelerini ekran dışına itti.

Genişlik sarmalayıcı bir `div`'e taşındı. Bu, ekran görüntüsü alınmadan fark edilmezdi —
tip kontrolü ve linter ikisi de temizdi.

### Ölçüm 3 — sıfırı elle biçimlendirmek eksende iki para biçimi üretti

`formatMoney(0)` sabit `"0,00 $"` dizisi döndürüyordu; `Intl` ise aynı değeri
`"$0,00"` diye yazıyordu. Maliyet ekseninde alt uçta biri, üst uçta diğeri göründü.
Sıfır da biçimlendiriciden geçirildi; ayrıca eksen basamak sayısını **en büyük değere
göre bir kez** seçip tüm etiketlere uyguluyor.

---

## Revizyon — yönetici bakışı (2026-08-11)

### Ölçüler

- [x] R01 `reports.Totals` → `prsOpened` (`workflow_steps` ⨝ `workflow_runs`)
- [x] R02 `reports.Totals` → `jiraTasks` (`workflow_processed_issues`)
- [x] R03 `reports.Totals` → `runsWithCode` (`runs.diff <> ''`)
- [x] R04 `reports.Day` → `prsOpened`; boş günler mevcut kalıpla doldurulur
- [x] R05 Proje süzgeci yeni sorgularda da çalışır (`workflows.project_id`)
- [x] R06 Testler: yeni ölçüler gerçek veriyle doğrulanır

### Arayüz

- [x] R10 `ReportTotals` / `ReportDay` tipleri
- [x] R11 `Sparkline` bileşeni — eksensiz, tek seri, token renkleri
- [x] R12 `formatPerUnit` biçimlendirici
- [x] R13 `Headline` yeniden yazılır: kahraman PR, dört denge rakamı, dipnot
- [x] R14 Uyarlanan kahraman: PR yoksa "tamamlanan iş"
- [x] R15 `Charts` ve `Breakdowns` DEĞİŞMEZ (detay katmanı)

### Doğrulama

- [x] R40 [plan.md](plan.md) doğrulama listesi 1–7
- [x] R41 Ekrandaki PR sayısı veritabanı sayımıyla birebir aynı

### Kapanış

- [x] R90 `AGENTS.md` kuralı; `specs/004-rapor/spec.md`'ye kararın neden
      değiştiği notu (spec silinmez — değişimin kendisi kayıt)

---

### R40/R41 — Doğrulama sonuçları

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

### Ölçüm 4 — en somut çıktı yanlış tablodaydı

Rapor kurulduğundan beri yalnızca `runs` tablosuna bakıyordu. Sistemin ürettiği
en somut şey — açılan pull request — orada **yok**: PR açan düğüm model
çağırmadığı için çalıştırma kaydı üretmiyor (spec 003'te bilinçli alınmış bir
karar, ve o karar doğru).

Sonuç: ekran üç kuruşluk harcamayı 48 punto ile gösterirken, sistemin asıl
çıktısını hiç gösteremiyordu. Rapor artık ikinci kaynağa da bakıyor.

**Ders:** bir ekranın "neyi gösteremediği", gösterdiği şey kadar bilgi taşır.
Veri kaynağı seçimi, farkında olmadan neyin ölçülebileceğini de belirliyor.

### Ölçüm 5 — araştırma ilk içgüdüyü düzeltti

"PR sayısını kahraman yap" ilk kararımdı. Literatür bunu **tek başına**
yanıltıcı sayıyor: bir ölçümde geliştirici başına merge edilen PR %98 artarken
PR başına olay sayısı %242 artmış ve organizasyon düzeyinde teslimat düz
kalmış. Yani PR sayısı ilerleme gibi görünüp gerilemeyi gizleyebiliyor.

Bu yüzden yanına **PR başına değişen satır** kondu: büyük değişiklik kümeleri
riskle en tutarlı ilişkilendirilen ölçü ve elimizde ölçülmüş veri var.

Kural olarak yazıldı (K4, `AGENTS.md`): **hız metriği asla yalnız
gösterilmez.**

### Ölçüm 6 — test fikstürü olmayan bir kolona yazıyordu

Yeni testler ilk çalıştırmada `column "hook_token" of relation "workflows" does
not exist` verdi. Fikstürü şemayı okumadan, hatırladığım alan adıyla yazmıştım;
`hook_token` başka bir tabloda.

Küçük bir hata ama sınıfı tanıdık: **hatırlanan şema, okunan şema değildir.**
