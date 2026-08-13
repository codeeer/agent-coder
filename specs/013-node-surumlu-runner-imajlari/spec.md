# Spec: Node sürümlü runner imajları ve koşu öncesi sürüm seçimi

- **Spec no:** 013
- **Tarih:** 2026-08-12
- **Durum:** Uygulandı
- **Not:** Bu spec **geriye dönük** yazıldı — özellik önce uygulandı, spec sonra.
  Metodolojiden sapma; sapmanın kendisi [tasks.md → Ölçüm 6](tasks.md) olarak
  kayda geçti.

---

## Problem

Agent'ın koşturacağı `npm install` ve `npm run build` komutları belirli bir
Node sürümü istiyor. Runner imajında tek bir Node var ve o sürüm neyse proje
ona uymak zorunda. Uymuyorsa çalıştırma, agent'ın hiçbir şekilde
düzeltemeyeceği bir hatayla düşüyor.

Sürümü koşu anında indirmek (nvm, fnm, volta) akla geliyor ama bu üründe
yapılamaz: runner container'ı **kapalı ağda da çalışmak zorunda** ve koşu
anında ağa çıkan her adım, kapalı ağda çalışmayan bir adımdır.

## Amaç

Kullanıcı, çalıştırmayı başlatmadan önce Node sürümünü seçebilsin. Seçilen
sürümün imajı **derleme anında** hazırlanmış olsun; koşu anında hiçbir şey
indirilmesin.

## Kapsam dışı

- **Java, Python, Go sürümleri.** Runner imajında bunların hiçbiri yok; ayar
  koymak karşılığı olmayan bir söz olurdu. Etiket şeması (`node-<sürüm>`)
  araç adını taşıyor, yani ikincisine yer bırakıyor — ama bu spec onu
  kapsamıyor.
- **Sürümü koşu anında indirmek.** Bilerek yapılmadı (bkz. Problem).
- **Kullanıcının kendi imajını yüklemesi.** Global imaj ayarıyla bugün de
  mümkün; bu spec onu değiştirmiyor.

---

## Kullanıcı hikâyeleri

### H1 — Koşu başlatırken sürüm seçmek

**Geliştirici** olarak, çalıştırmayı başlatmadan önce **Node sürümünü
seçmek** istiyorum, çünkü projemin build'i o sürümü gerektiriyor.

### H2 — Proje varsayılanı

**Geliştirici** olarak, bir projeye **varsayılan Node sürümü** atamak
istiyorum, çünkü aynı depo için her seferinde aynı seçimi yapmak istemiyorum.

### H3 — Eksik imajı erken öğrenmek

**Geliştirici** olarak, seçtiğim sürümün imajı yoksa bunu **klonlama
başlamadan** öğrenmek istiyorum, çünkü iki dakika bekleyip anlamsız bir
hatayla karşılaşmak istemiyorum.

---

## Kabul kriterleri

- [x] Koşu başlatma formunda Node sürümü seçilebiliyor; boş bırakılabiliyor
- [x] Boş bırakılınca proje varsayılanı, o da yoksa taban imaj kullanılıyor
- [x] Öncelik sırası: **koşu > proje > global imaj ayarı**
- [x] Koşu formundaki sürüm listesi, imajı **gerçekten yayınlanmış**
      sürümlerle birebir aynı — listede olmayan bir sürüm seçilemiyor
- [x] Geçersiz sürüm reddediliyor; koşu hiç başlamıyor
- [x] Seçilen sürümün imajı yoksa çalıştırma **klonlamadan önce**, ne
      yapılacağını söyleyen bir mesajla düşüyor
- [x] Koşu detayında sürüm yalnızca **seçilmişse** yazılıyor

---

## Kararlar

### K1 — Sürüm listesi kodda değil, düz metin dosyasında

Listeyi üç ayrı yer okuyor: uygulama, imajları yayınlayan işlem hattı ve
yerel derleme komutu. Üçü farklı dillerde yazılmış; hangisinin diline
yazılsaydı diğer ikisi okuyamazdı.

Düz metin, üçünün de ayrıştırabildiği tek biçim. Ayrıntı:
[plan.md → Tek kaynak](plan.md).

### K2 — Etiket şeması araç adını taşır: `node-24.13.0`

`:24.13.0` değil `:node-24.13.0`. Bugün tek araç var ama etiket alanı
paylaşılan bir isim alanı; ileride `java-21` gelirse `21` ile `24.13.0`
arasında hangisinin ne olduğu ancak biçimden anlaşılırdı.

### K3 — Eksik imaj indirilmez, bildirilir

Sistem eksik bir imajı kendiliğinden çekmeye **çalışmaz**. Sebep kapalı ağ:
indirme denemesi orada zaten başarısız olur ve hata "ağ yok" diye görünür,
oysa gerçek sebep "imaj hazırlanmamış"tır. Yanlış sebebi gösteren bir hata,
hiç hata vermemekten kötüdür.

Bunun yerine açık bir hata veriliyor ve mesaj, kurulumun biçimine göre ne
yapılacağını söylüyor (yerelde derlemek mi, yayından çekmek mi).

### K4 — Kontrol klonlamadan ÖNCE

Önceden imaj varlığı yalnızca açılışta bir kez sınanıyordu ve hatası log'a
düşüp yutuluyordu; eksik imaj ancak kullanıcı iki dakika bekledikten sonra
görünürdü. Sürümlü imajlarda bu daha da kötü: taban imaj yerinde ama seçilen
varyant hazırlanmamış olabilir.

---

## Hata durumları

| Durum | Beklenen davranış |
|-------|-------------------|
| Listede olmayan sürüm gönderilir | İstek reddedilir, koşu hiç başlamaz |
| Seçilen sürümün imajı hazır değil | Çalıştırma klonlamadan önce düşer; mesaj imajın nasıl hazırlanacağını söyler |
| Sürüm seçilmemiş | Proje varsayılanı, o da yoksa taban imaj — hata değil |

---

## Karar geçmişi

- **2026-08-13** — K3'ün bedeli ölçüldü: sürümlü imajlar taban imaj
  değiştiğinde kendiliğinden yenilenmiyor ve bayat kalabiliyor. Bir kez
  gerçekleşti (bkz. [tasks.md → Ölçüm 5](tasks.md)). Karar değişmedi; imaj
  hazırlamanın **tüm varyantları** kapsaması gerektiği belgelendi.
