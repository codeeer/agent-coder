# Spec: Motor logları

- **Spec no:** 015
- **Tarih:** 2026-08-13
- **Durum:** Uygulandı
- **Not:** Geriye dönük yazıldı — bkz. [spec 013 → Ölçüm 6](../013-node-surumlu-runner-imajlari/tasks.md).

---

## Problem

Runner container'ı iş biter bitmez siliniyor ve motorun teşhis bilgisi onunla
birlikte yok oluyor. Bir çalıştırma düştüğünde geriye yalnızca ilerleme
akışının özeti kalıyor; "sağlayıcı sürücüsü yüklenemedi" gibi asıl cümleler
container'ın içindeki dosyalarda kalıp gidiyor.

Ayrıca ilerleme kayıtları canlı olay akışıyla besleniyor
([spec 003](../003-agent-calistirma/spec.md)). O bağlantı koptuğunda
kayıt eksik kalıyor ve agent'ın ne konuştuğu, hangi araçları çağırdığı hiçbir
yerde durmuyor.

## Amaç

Bir çalıştırma nasıl biterse bitsin, motorun ham çıktısı ve agent'ın tam
geçmişi saklansın; koşu detayından okunabilsin.

## Kapsam dışı

- **İlerleme akışının yerini almak.** Akış "ne oldu"yu anlatır, bu "tam
  olarak ne yazıldı"yı. İkisi ayrı katman.
- **Canlı log akışı.** Toplama koşu bitiminde yapılır; süren koşuda arayüz
  periyodik tazeler ama satır satır canlı akış yoktur.
- **Container'ın dosya sistemini arşivlemek.** Yalnızca log ve oturum
  geçmişi alınır.

---

## Kullanıcı hikâyeleri

### H1 — Düşen koşunun sebebini görmek

**Geliştirici** olarak, başarısız bir çalıştırmanın **motor loglarını**
okumak istiyorum, çünkü ilerleme akışı bana yalnızca "başarısız" diyor.

### H2 — Kopan bağlantıdan sonra geçmişi bulmak

**Geliştirici** olarak, canlı akış kopsa bile agent'ın **tam konuşma ve araç
geçmişine** ulaşmak istiyorum.

### H3 — Sırların loga sızmaması

**Yönetici** olarak, saklanan logda API anahtarlarının **görünmemesini**
istiyorum, çünkü bu içerik veritabanında duruyor ve arayüzde gösteriliyor.

---

## Kabul kriterleri

- [x] Üç kaynak toplanıyor: container çıktısı, motorun log dosyaları, oturum
      geçmişi
- [x] Toplama container **silinmeden önce** yapılıyor
- [x] Koşu nasıl biterse bitsin çalışıyor: başarı, hata, iptal, zaman aşımı
- [x] Sırlar **yazılmadan önce** maskeleniyor
- [x] İki megabaytlık bir log koşu detayında açılabiliyor ve indirilebiliyor
- [x] Boyut sınırı aşılırsa **son** korunuyor ve kırpıldığı söyleniyor
- [x] Koşu silinince loglar cascade ile gidiyor
- [x] Saklama süresi ve boyut sınırı ayarlardan; saklama kapatılabiliyor
- [x] Koşu detayında **ayrı sekme**: kaynak seçici, arama, seviye vurgusu,
      indirme
- [x] Oturum geçmişi okunur biçimde gösteriliyor; ham JSON'a geçilebiliyor

---

## Kararlar

### K1 — Loglar sonucun içinden değil, ayrı bir kanaldan taşınır

Bir çalıştırma başarısız olduğunda ortada **sonuç yoktur** — oysa loglara
asıl ihtiyaç tam o andadır. Sonuca bağlanan bir taşıma, en çok gerektiği
durumda hiçbir şey getirmezdi.

### K2 — Kaynak başına tek kayıt, satır satır değil

İçerik sorgulanmıyor, bütün olarak okunuyor. Satır satır saklamak koşu başına
yüzlerce kayıt ve karşılığı olmayan bir yazma maliyeti demekti. Sıkıştırma
ölçüldü: 4.193 kB ham → 407 kB. Ayrıntı: [plan.md → Saklama](plan.md).

### K3 — Kırpma sonu korur

Hata genelde sonda olur; baştaki açılış satırları teşhis için en az değerli
olanlardır. **İstisna:** oturum geçmişi yapılandırılmış bir kayıt olduğu için bayt bazlı
kırpma onu okunamaz hale getirir — orada sınır farklı uygulanır (K7).

### K4 — Maskeleme yazma anında

Sonradan temizlemek bir kez yazılmış sırrı geri almaz. Maskelenen değerler
tek yerde toplanır ki yeni bir sır eklendiğinde listeye girmesi unutulmasın.
Paket deposunun kimliği yapılandırmaya **kodlanmış** olarak yazıldığı için
([spec 014](../014-kurumsal-paket-deposu/spec.md)) listede ham hâli değil,
kodlanmış hâli de bulunuyor — yalnızca ham değer aransaydı kaçardı.

### K5 — Ham log ve koşu kaydı ayrı yaşar

Çalıştırma kaydı yıllarca değerli olabilir; iki megabaytlık ham logu bir
haftadan sonra değil. İki ayrı yaşam süresi bu yüzden var. Süresi dolunca
yalnızca log silinir; koşu geçmişi ve maliyet raporu yerinde kalır.

### K6 — Oturum geçmişi dosyadan değil, motorun kendisinden sorulur

Motorun oturum deposu bu sürümde bir veritabanı dosyası (ölçüldü — istek
"JSON dosyaları" diyordu). Bir veritabanı dosyasını ham kopyalamak metin
üretmez: maskeleme uygulanamaz, arayüzde gösterilemez. Aynı veri motora
sorularak metin biçiminde alınıyor.

**Motor sürümü bu kararı geçersiz kılabilir** — depo biçimi değişirse kaynak
yine motorun kendisidir, dosya değil.

### K7 — Yapılandırılmış içerikte sınır, veriyi bozmadan uygulanır

Bir `npm install && build` koşusunda geçmiş 4,29 MB çıktı ve %96'sı iki
alandı. Bayt bazlı kırpma geriye **okunamayan** bir kayıt bıraktı; okunur
görünüm tam da en çok gerektiği koşuda kayboldu.

Sınır artık kaydın kendi birimine göre uygulanıyor: her mesaj ve her araç
çağrısı yerinde kalır, yalnızca uzun metinlerin **ortası** çıkar ve çıkarılan
miktar yazılır.

---

## Hata durumları

| Durum | Beklenen davranış |
|-------|-------------------|
| Log toplanamaz | Sessizce atlanır; çalıştırmanın sonucu değişmez |
| Motor cevap vermez | Geçmiş 15 sn sonra bırakılır, container silinmesi gecikmez |
| Saklama kapalı | Toplama yapılmaz, sekme "saklanmış log yok" der |
| Geçmiş ayrıştırılamaz | Ham metin olarak saklanır ve gösterilir |
| Süresi dolmuş | Yalnızca log silinir, koşu kaydı kalır |
| Sır çok kısa (8 karakterden az) | **Maskelenmez** — kısa bir dizi metnin her yerinde eşleşir ve logu okunamaz hale getirirdi. Kısa sırlar bu yüzden korunamaz; bunu bilerek kabul ediyoruz |

---

## Karar geçmişi

- **2026-08-13** — K3'e istisna eklendi (K7). Sebep: gerçek bir koşuda
  ölçülen 4,29 MB'lık geçmiş, bayt kırpması yüzünden ayrıştırılamaz hale
  geldi.
