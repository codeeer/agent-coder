# Spec: Toplu çalıştırma — bir akışı çok projede sıraya koyma

- **Spec no:** 023
- **Tarih:** 2026-08-15
- **Durum:** Uygulandı (2026-08-15)

---

## Problem

**Aynı akışı otuz projede işletmenin yolu, otuz kez elle tetiklemek.**

Ürün bunun yarısını çözmüş durumda: bir akış çalıştırılırken proje seçilebiliyor
ve aynı süreç farklı projelerde koşabiliyor. Eksik olan **çokluk**: kullanıcı
akış sayfasına gidip proje seçiyor, başlatıyor, bitmesini bekliyor, tekrar
seçiyor. Otuz proje otuz tur demek.

Ve doğrudan hepsini birden başlatmak **çalışmıyor** — ölçüldü:

Eşzamanlı çalıştırma sınırı dolduğunda çalıştırma yöneticisi **sıraya
koymuyor, anında reddediyor.** Bu red kod tabanında tek bir yerde ele
alınıyor: bir hata mesajına çevriliyor. Hiçbir şey yeniden denemiyor, hiçbir
şey beklemiyor.

Somut sonucu: sınır 3 iken otuz iş başlatılırsa **üçü çalışır, yirmi yedisi
anında düşer** ve o yirmi yedisinin kaydı bile oluşmaz.

> **Ölçüm düzeltmesi (Blok 2, 2026-08-15).** Yukarıdaki cümle tek çalıştırma
> ucu için doğru; **akış çalışmaları için değil.** Kod okundu: `Launcher.Launch`
> sınıra hiç bakmıyor, kaydı oluşturuyor. Sınır adım seviyesinde uygulanıyor ve
> dolu olduğunda akış çalışmasının tamamı `failed` oluyor. Yani otuz akış
> başlatılırsa otuz KAYIT oluşur, yirmi yedisi "sınır dolu" diye başarısız
> görünür — kullanıcı için sonuç aynı derecede kötü, ama arızanın biçimi farklı
> ve kuyruğun karşılığı da farklı (bkz. [plan.md](plan.md) → Tuzak).

Sınırı yükseltmek çözüm değil: iş başına iki çekirdek ve dört GB ayrılıyor,
otuz eşzamanlı iş altmış çekirdek ve yüz yirmi GB demek. Sınır keyfi değil,
makinenin gerçeği.

**Yani bu bir liste ekranı değil, bir kuyruk.**

## Amaç

Kullanıcının bir akışı **birden fazla projede tek hamlede sıraya koyması**;
sistemin eşzamanlılık sınırına uyarak sırayla işletmesi ve ilerlemeyi tek
ekranda göstermesi.

## Kapsam dışı

- **Eşzamanlılık sınırının değiştirilmesi.** Kuyruk mevcut sınıra *uyar*, kendi
  paralelliğini tanımlamaz. "Aynı anda kaç iş" sorusunun tek bir cevabı olmalı.
- **Tek çalıştırma (agent) toplu koşumu.** Sıraya konan şey akış çalışmasıdır.
- **Zamanlanmış toplu iş.** Kuyruk kullanıcının tetiklediği bir eylemle
  doluyor; takvimle değil.
- **Projeler arası bağımlılık.** Sıradaki işler birbirinden bağımsız; biri
  diğerinin çıktısını beklemez.
- **Kuyruğun önceliklendirilmesi.** Sıra, eklenme sırasıdır.

---

## Kullanıcı Hikâyeleri

### H1 — Akışı çok projede sıraya koyma

**Standart bir kampanyayı yürüten kullanıcı** olarak, **bir akışı seçtiğim
projelerin hepsinde çalıştırmak** istiyorum, çünkü **otuz projede aynı işi
otuz kez elle tetiklemek uygulanabilir değil.**

Kabul kriterleri:

- [x] Kullanıcı bir akış için birden çok proje seçip tek hamlede başlatabilir
- [x] Seçim yapılırken kaç proje seçildiği görünür
- [x] Başlatıldığında kaç işin sıraya alındığı yazılır
- [x] Hiç proje seçilmeden başlatılamaz

### H2 — Sınıra uyulması

**Kullanıcı** olarak, **sistemin aynı anda yalnızca sınır kadar iş
çalıştırmasını** istiyorum, çünkü **makine otuz eşzamanlı işi kaldırmaz ve
reddedilen bir iş hiç çalışmamış olur.**

Kabul kriterleri:

- [x] Aynı anda çalışan iş sayısı eşzamanlılık sınırını **aşmaz**
- [x] Sınırı aşan işler **reddedilmez**, kuyrukta bekler
- [x] Bir iş bitince sıradaki kendiliğinden başlar
- [x] Kuyruk, ayardaki sınır değiştirilirse yeni sınıra uyar

### H3 — İlerlemeyi görme

**Kullanıcı** olarak, **toplu işin nerede olduğunu tek ekranda görmek**
istiyorum, çünkü **otuz işin durumunu tek tek aramak, elle tetiklemekten daha
iyi olmaz.**

Kabul kriterleri:

- [x] Toplu iş; bekleyen, çalışan, biten ve başarısız sayılarıyla görünür
- [x] Her satır hangi projeye ait olduğunu ve durumunu gösterir
- [x] Çalışan ve biten işler kendi çalıştırma kaydına bağlanır
- [x] İş sürerken ekran kendiliğinden tazelenir
- [x] Toplu iş bitince sonuç özeti kalır; ekran boşalmaz

### H4 — Hataya rağmen devam

**Kullanıcı** olarak, **bir proje düştüğünde kalanların denenmesini**
istiyorum, çünkü **yirmi yedinci projedeki derleme hatası, yirmi sekizinciyi
denememek için sebep değil.**

Kabul kriterleri:

- [x] Bir iş başarısız olduğunda kuyruk **durmaz**
- [x] Başarısız işler sonuç özetinde adıyla ve sebebiyle listelenir
- [x] Başarılı olanlar korunur

### H5 — Yeniden başlatmaya dayanma

**Kullanıcı** olarak, **backend yeniden başladığında kuyruğun kaldığı yerden
devam etmesini** istiyorum, çünkü **otuz projelik bir kampanya saatler sürer
ve o sürede bir yeniden başlatma olağandır.**

Kabul kriterleri:

- [x] Bekleyen işler yeniden başlatmadan sonra da bekliyor görünür ve sırayla
      başlar
- [x] Yeniden başlatma sırasında çalışan bir iş kesilirse durumu **belirsiz
      bırakılmaz**; ne olduğu yazılır
- [x] Kullanıcı hangi işlerin koştuğunu, hangilerinin beklediğini yeniden
      başlatmadan sonra da görebilir

### H5a — Kaldığı yerden devam

**Kullanıcı** olarak, **kesilen işleri tek düğmeyle kaldığı yerden
sürdürmek** istiyorum, çünkü **yeni bir toplu iş açmak, tamamlanmış yirmi
projeyi de yeniden koşturmak demek olurdu.**

Kabul kriterleri:

- [x] Kesilmiş öğesi olan toplu işte **"Kaldığı yerden devam et"** eylemi
      görünür ve kaç işin sıraya alınacağını üzerinde yazar
- [x] Düğme, kaç işin sıraya alınacağını **önceden** söyler
- [x] Yalnızca **kesilmiş** öğeler sıraya alınır; tamamlananlar tekrar
      koşturulmaz
- [x] Gerçekten başarısız olmuş öğeler (derleme hatası gibi) kendiliğinden
      sıraya alınmaz — onlar çalıştı ve bir sonuç üretti
- [x] Kesilmiş öğe yoksa düğme çıkmaz

### H6 — Vazgeçme

**Kullanıcı** olarak, **başlattığım toplu işi durdurabilmek** istiyorum, çünkü
**yanlış akışı seçtiğimi üçüncü projede fark edebilirim.**

Kabul kriterleri:

- [x] Toplu iş iptal edilebilir
- [x] İptal, **bekleyen** işleri düşürür
- [x] Çalışan işler kendi hâlinde devam eder ve sonuçları kaydedilir
- [x] İptalin ne yapacağı, onaydan **önce** yazılır

---

## Davranış Kuralları

- **Kuyruk mevcut sınıra uyar, kendi sınırını tanımlamaz.** "Aynı anda kaç iş
  çalışır" sorusunun tek cevabı olmalı; ikinci bir ayar, iki yerden
  yönetilen ve er geç çelişen bir sistem üretirdi.

- **Sıra eklenme sırasıdır.** Öncelik yok. Kullanıcı sırayı seçim yaparken
  kurar.

- **Kuyruk kalıcıdır.** Bellekte tutulsaydı bir yeniden başlatma bekleyen
  işleri sessizce yok ederdi ve kullanıcı bunu ancak "neden hiç başlamadı"
  diye sorarak fark ederdi.

- **Kısmi başarı geri alınmaz.** Düşen bir iş, tamamlanmışları etkilemez.

- **Aynı akış aynı projede iki kez sıraya konmaz.** Seçim listesinde bir proje
  bir kez yer alır; aynı toplu işte tekrarı anlamsızdır.

- **Toplu iş bir çalıştırma değildir.** Kendi kaydı vardır ve içindeki her
  öğe kendi akış çalışmasına bağlanır. İkisini karıştırmak, "otuz işin
  durumu" ile "bir işin durumu" sorularını aynı ekrana sıkıştırırdı.

---

## Hata Durumları

| Durum | Beklenen davranış |
| --- | --- |
| Hiç proje seçilmedi | Başlatılamaz; sebebi yazılır |
| Seçilen projelerden biri silinmiş | O öğe başarısız işaretlenir, kuyruk devam eder |
| Akış silinmiş | Toplu iş başlatılamaz; sebebi yazılır |
| Bir iş başlatılamadı (yapılandırma eksiği) | O öğe sebebiyle başarısız olur, kuyruk devam eder |
| Backend yeniden başladı | Bekleyenler beklemeye devam eder; kesilen iş belirsiz bırakılmaz |
| Kullanıcı iptal etti | Bekleyenler düşer, çalışanlar sürer |
| Toplu iş zaten bitmiş, tekrar iptal | Hata değil; durumu söylenir |

---

## Belirsizlikler

- [x] **Kuyruk nerede yaşasın?** → **Cevap: veritabanında, kalıcı.** Otuz
      projelik bir kampanya saatler sürer; o sürede bir yeniden başlatma
      olağandır ve kuyruğun kaybolması kullanıcının neyin koştuğunu bilmemesi
      demektir.

- [x] **Bir proje düşerse?** → **Cevap: kuyruk devam eder**, sonunda
      raporlanır.

- [x] **Kaç iş aynı anda?** → **Cevap: mevcut eşzamanlılık sınırı kadar.**
      Hepsi birden başlatılmaz; sınır kadarı koşar, gerisi bekler.

- [x] **Yeniden başlatma sırasında çalışan bir iş ne olur?**
      → **Cevap: kendiliğinden denenmez, "Kaldığı yerden devam et" düğmesiyle.**
      Kesilen iş "kesildi" olarak işaretlenir; kullanıcı düğmeye bastığında
      toplu iş KALDIĞI YERDEN devam eder.

      Kendiliğinden denemek yarım kalmış bir işin yan etkilerini (branch'e
      gönderilmiş bir değişiklik) habersizce tekrarlardı. Kullanıcıyı yeni bir
      toplu iş açmaya zorlamak ise tamamlanmış yirmi işi yeniden koşturma
      riskini doğururdu — düğme ikisinin arasındaki doğru yer.

## Bağımlılıklar

- Akışların proje seçilerek çalıştırılabilmesi (spec 007). Bu spec onun
  üzerine kuruluyor.
- Eşzamanlılık sınırı (spec 003). Kuyruk ona uyar, değiştirmez.
