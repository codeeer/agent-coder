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
- [x] Tema anahtarı her sayfadan erişilebilir. *(2026-08-13'te ölçülünce
      koşu detayında olmadığı görüldü — o ekran `PageHeader` kullanmıyor.
      Anahtar oraya ayrıca eklendi; kriter yeniden doğru.)*
- [x] Grafiklerin **durum renkleri** temaya göre değişmez (başarılı yeşili iki temada
      da aynı renktir — iki ekran görüntüsü karşılaştırılabilir kalsın).

## Kapsam dışı

- **Ek temalar** (yüksek kontrast, sepya vb.). İki tema yeterli.
- **Sunucuda saklama.** Gerekçe aşağıda.
- **Yeni jeton veya yeni renk değeri.** Bu spec temayı *seçilebilir* yapar;
  jetonların değerlerini belirlemez.

  > **Bu madde bir kez yanlış anlaşıldı.** Önceki hâli "renk değerleri
  > değişmiyor" diyordu ve bu, "açık tema koyunun aynısıdır, ayrıca
  > değerlendirilmez" gibi okundu. Yanlıştı: ölçüm beş jetonun açık temada
  > eşiğin altında olduğunu gösterdi ([tasks.md → Ölçüm 2](tasks.md)).
  > Bugünkü kural şudur — **her tema ayrı ayrı değerlendirilir**:
  > [.claude/rules/ui.md → Açık ve koyu tema](../../.claude/rules/ui.md).

## Karar: seçim veritabanında DEĞİL, tarayıcıda

Projenin kuralı "davranış parametreleri koda gömülmez, veritabanında durur" der
(spec 003 H7). Tema bu kuralın kapsamında **değildir**: bir davranış parametresi
değil, **bakan kişinin** tercihidir.

Sunucuda tutulsaydı aynı kurulumu kullanan iki kişiden biri temayı değiştirince
diğerinin ekranı da değişirdi. Kimlik doğrulama geldiğinde (v1'de yok) kullanıcı
başına saklamak istenirse o zaman yeniden değerlendirilir.

---

## Karar geçmişi

### 2026-08-10 — renk değerleri ölçülerek düzeltildi

Bu spec temanın **mekanizmasını** tanımlıyor (üç durum, jeton kapsamları,
`color-scheme`) ve o kısım hâlâ geçerli.

Ama **jetonların değerleri** sonradan yapılan bir arayüz denetiminde
değişti. Sebep: değerler gözle seçilmişti ve ölçülünce bir kısmı eşiğin altında
çıktı — `ink-3` koyu temada, `info` açık temada. Ayrıca denetim sınırı diye ayrı
bir jeton (`control-line`) eklendi.

Kural da oradan geldi: **açık tema, koyu temanın tersi değildir** — her bileşenin
her temadaki kontrastı bağımsız değerlendirilir. Kuralın bugünkü yeri:
[.claude/rules/ui.md](../../.claude/rules/ui.md) → "Açık ve koyu tema".

### 2026-08-12 — anahtar kenar çubuğundan sayfa başlığına taşındı

Arayüz yeniden tasarlanırken kabuk tek bir üst şeride indirildi ve tema
anahtarı sayfa başlığının sağ ucuna geçti; kenar çubuğunun dibi sistem
durumuna ayrıldı.

**Mekanizma değişmedi** — üç durum, `data-theme`, `localStorage` anahtarı ve
ilk boyamadan önce çalışan betik aynı. Değişen yalnızca anahtarın ekrandaki
yeri.

O gün "`PageHeader` her ekranda çiziliyor, dolayısıyla anahtar da her
ekranda" denmişti. **Bu ölçülmemişti ve yanlıştı:** koşu detayı `PageHeader`
kullanmıyor. Bir sonraki maddeye bakın.

### 2026-08-13 — anahtarın "her sayfada" olduğu iddiası ölçüldü ve düzeltildi

Anahtarı sayfa başlığına bağlamak, kriteri **başlık bileşenini kullanan**
sayfalarla sınırlamış. Koşu detayı yeniden tasarlanırken `PageHeader` yerine
kendi künye kartını aldı ve tema anahtarı o ekrandan sessizce kayboldu; on
iki sayfanın on birinde vardı, birinde yoktu.

Ölçüm basitti: `[aria-label="Tema"]` koşu listesinde var, koşu detayında yok.

Anahtar o ekrana ayrıca eklendi. Kalıcı ders: **"her sayfada" bir sayfa
bileşenine bağlanamaz.** Kriter bugün doğru ama kırılgan; `PageHeader`
kullanmayan bir sonraki ekran aynı şeyi tekrar yaşar. Kabuk seviyesine
taşımak açık bir seçenek olarak duruyor.
