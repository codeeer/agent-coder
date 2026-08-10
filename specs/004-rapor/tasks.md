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
