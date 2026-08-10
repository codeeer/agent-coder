# Spec: Tema Seçimi — Sistem / Açık / Koyu

- **Spec no:** 006
- **Tarih:** 2026-08-09
- **Durum:** Uygulandı
- **Faz:** 2 — [plans/01](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Arayüz yalnızca **işletim sisteminin** tercihini takip ediyor. Açık tema jetonları
en baştan tanımlı ve her ekran onlarla çalışacak şekilde yazıldı, ama kullanıcının
bunu seçmesinin bir yolu yok: makinesi koyu temadaysa uygulama koyu, açıksa açık.

Bu iki durumda sorun oluyor:

- Sistemi koyu tema kullanan ama **bu uygulamayı açık temada** görmek isteyen
  kullanıcı (sunum yapıyor, projeksiyona veriyor, ekran görüntüsü alıyor) bunu
  yapamıyor.
- Tersi de geçerli: sistemi açık, uygulamayı koyu isteyen.

## Amaç

Kullanıcı temayı kendisi seçebilsin; seçmezse sistemi takip etmeye devam etsin.

## Kullanıcı hikâyeleri

1. Kullanıcı olarak **açık** temayı seçebilmeliyim, sistemim koyu olsa bile.
2. Kullanıcı olarak **koyu** temayı seçebilmeliyim, sistemim açık olsa bile.
3. Kullanıcı olarak **sisteme geri dönebilmeliyim** — bir kez seçim yapmak beni
   sonsuza kadar sabit bir temaya mahkûm etmemeli.
4. Kullanıcı olarak seçimim **sayfa yenilendiğinde ve sekmeler arasında** korunmalı.
5. Kullanıcı olarak sayfa açılırken **yanlış temanın bir an görünüp sıçramasını**
   yaşamamalıyım.

## Kabul kriterleri

- [x] Üç durum vardır: sistem (varsayılan), açık, koyu.
- [x] "Sistem" seçiliyken işletim sistemi teması değişirse arayüz **anında** takip eder.
- [x] Seçim yenilemede korunur.
- [x] İlk boyamada doğru tema uygulanır; beyaz/siyah çakma olmaz.
- [x] Tema anahtarı her sayfadan erişilebilir.
- [x] Grafiklerin **durum renkleri** temaya göre değişmez (başarılı yeşili iki temada
      da aynı renktir — iki ekran görüntüsü karşılaştırılabilir kalsın).

## Kapsam dışı

- **Ek temalar** (yüksek kontrast, sepya vb.). İki tema yeterli.
- **Sunucuda saklama.** Gerekçe aşağıda.
- **Tema başına ayrı tasarım.** Jetonlar zaten iki temayı da karşılıyor; renk
  değerleri değişmiyor, yalnızca seçilebilir hale geliyor.

## Karar: seçim veritabanında DEĞİL, tarayıcıda

Projenin kuralı "davranış parametreleri koda gömülmez, veritabanında durur" der
(spec 003 H7). Tema bu kuralın kapsamında **değildir**: bir davranış parametresi
değil, **bakan kişinin** tercihidir.

Sunucuda tutulsaydı aynı kurulumu kullanan iki kişiden biri temayı değiştirince
diğerinin ekranı da değişirdi. Kimlik doğrulama geldiğinde (v1'de yok) kullanıcı
başına saklamak istenirse o zaman yeniden değerlendirilir.
