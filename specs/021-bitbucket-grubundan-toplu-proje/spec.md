# Spec: Bitbucket grubundan toplu proje ekleme

- **Spec no:** 021
- **Tarih:** 2026-08-14
- **Durum:** Uygulandı

---

## Problem

**Kurumsal Bitbucket'ta repository'ler gruplar altında duruyor; ürün ise
repository'leri tek tek, elle eklemekten başka yol tanımıyor.**

Kurumsal kurulumda hiyerarşi iki katmanlı: bir grup (ekip, ürün ya da bölüm)
ve altında onlarca repository. Kullanıcının elinde grubun adresi var — tarayıcıda
açtığı, ekip arkadaşına gönderdiği adres o.

Bugün bu adresin ürüne girilebileceği bir yer yok. Kullanıcı grubu açıp
repository'leri tek tek geziyor, her birinin klonlama adresini kopyalayıp forma
yapıştırıyor, adı ve branch'i elle yazıyor. Altmış repository'lik bir grup,
altmış kez tekrarlanan bir form demek.

İşin süresi de yalnızca yazmakla sınırlı değil: her kayıt öncesinde depoya
erişim sınanıyor ve bu sınama repository başına yirmi saniyeye kadar sürebiliyor.
Altmış repository'de bu, tek başına dakikalarla ölçülen bir bekleme.

Sonuç, ölçülebilir bir tıkanıklık değil ama görünür bir tanesi: bu kurulumda
sekiz proje var ve sekizi de tek tek girilmiş. Ekip ölçeğinde bir kurumsal
kurulumda bu yöntem uygulanabilir değil.

## Amaç

Kullanıcının **grup adresini bir kez vermesi** ve o grubun altındaki
repository'leri listeleyip seçtiklerini **tek işlemde** proje olarak
eklemesi.

## Kapsam dışı

- **Bitbucket Cloud.** Bulut sürümünde hiyerarşi ve erişim şeması farklı;
  aynı adımlarla çözülmez. Bu spec yalnızca kurumsal (kendi sunucusunda
  çalışan) Bitbucket'ı kapsar. Bulut için istenirse ayrı iş açılır.
- **GitHub organization ve GitLab group.** Aynı ihtiyaç oralarda da var, ama
  her birinin listeleme şeması ayrı. Bu spec'te yalnızca Bitbucket ele alınır;
  ortaya çıkan akış başkalarına uygulanacaksa ayrı iş.
- **Kendiliğinden senkron.** Gruba sonradan eklenen bir repository arka planda
  kendiliğinden proje olmaz. İçe aktarma **her zaman kullanıcının tetiklediği**
  bir eylemdir; habersiz beliren proje sürprizdir.
- **Veri modeline "grup" kavramı eklemek.** İçe aktarılan repository'lerin her
  biri bugünkü anlamıyla bağımsız bir projedir. Gruplama, ekleme anında
  kullanılan bir kaynaktır; kalıcı bir üst katman değil.
- **Tek repository ekleme akışı.** Bugünkü form ve doğrulama kuralları
  olduğu gibi kalır; bu, onun yerine geçen değil yanına eklenen bir yol.
- **Bitbucket tarafında değişiklik.** Ürün repository oluşturmaz, silmez,
  yetki vermez. Yalnızca okur.

---

## Kullanıcı Hikâyeleri

### H1 — Grup adresinden repository'leri görme

**Kurumsal Bitbucket kullanan bir kullanıcı** olarak, elimdeki **grup adresini
verip altındaki repository'leri listelemek** istiyorum, çünkü **hangilerini
ekleyeceğime ancak listeyi görünce karar verebilirim.**

Kabul kriterleri:

- [x] Verili tanımlı bir kurumsal Bitbucket erişimi, kullanıcı bir grup adresi
      verdiğinde, o grubun altındaki repository'lerin adları listelenir
- [x] Grup yirmi beşten fazla repository içeriyorsa **hepsi** listelenir; liste
      ilk sayfayla sınırlı kalmaz
- [x] Adresin ucundaki eğik çizgi, sondaki ek yol parçaları veya adres
      çubuğundan gelen artıklar listelemeyi engellemez
- [x] Ürünün kök dizinde değil bir alt yolda kurulu olduğu sunucularda da
      liste gelir
- [x] Liste gelirken kullanıcı beklediğini görür; ekran donmuş gibi durmaz

### H2 — Seçtiklerini tek işlemde ekleme

**Kullanıcı** olarak, **listeden seçtiğim repository'lerin hepsini tek seferde
proje olarak eklemek** istiyorum, çünkü **asıl derdim tek tek form doldurmamak.**

Kabul kriterleri:

- [x] Liste geldiğinde repository'ler seçili gelir; kullanıcı istemediklerini
      çıkarabilir
- [x] Kaynak bir repository'nin arşivlenmiş olduğunu bildiriyorsa o
      repository listede **görünür ama seçili gelmez**; kullanıcı isterse
      elle seçebilir
- [x] Onaydan sonra seçilen her repository ayrı bir proje olarak eklenir
- [x] Eklenen her proje, o repository'nin **kendi** adını, **kendi** klonlama
      adresini ve **kendi** varsayılan branch'ini taşır; ad kaynaktan gelir,
      ürün tarafından türetilmez
- [x] Eklenen projeler, listelemede kullanılan erişim tanımına bağlanır —
      kullanıcı ayrıca kimlik seçmez
- [x] Ekleme sırasında kullanıcı işin ilerlediğini görür; ekran sessizce
      beklemez
- [x] İşlem bitince kaç projenin eklendiği sayıyla yazılır
- [x] Eklenen projeler proje listesinde, elle eklenmişlerden ayırt edilmeye
      gerek kalmadan görünür

### H2a — Eklemeden önce erişimin sınanması

**Kullanıcı** olarak, **eklenen projelerin gerçekten klonlanabildiğinden emin
olmak** istiyorum, çünkü **çalışmayan bir projeyi ancak bir agent'ı
tetikleyip başarısız olunca fark etmek geç.**

Kabul kriterleri:

- [x] Seçilen her repository, kaydedilmeden önce erişim açısından sınanır
- [x] Sınamayı geçemeyen repository **eklenmez**; adı ve sebebi yazılır
- [x] Sınama, seçim yüz repository'ye kadar çıktığında da makul sürede biter;
      kullanıcı dakikalarca bekletilmez
- [x] Bir repository'nin sınaması uzun sürerse tüm işlem onun yüzünden
      durmaz

### H3 — Aynı grubu yeniden içe aktarma

**Kullanıcı** olarak, **gruba yeni repository eklendiğinde içe aktarmayı
tekrar çalıştırmak** istiyorum, çünkü **grup zamanla büyüyor ve mevcut
projelerimi bozmadan yenilerini almak istiyorum.**

Kabul kriterleri:

- [x] Zaten kayıtlı olan bir repository ikinci kez eklenmez
- [x] Zaten kayıtlı olanlar listede **görünür** ve durumu yazar; gizlenmez
- [x] Bir repository başka bir erişim tanımıyla kayıtlıysa da atlanır;
      mevcut kaydın erişimi değiştirilmez
- [x] Sonuç, eklenen ile atlananı ayrı ayrı söyler: "9 eklendi, 51 zaten
      kayıtlıydı"
- [x] Hiç yeni repository yoksa bu bir hata değildir; öyle söylenir

### H4 — Kurumsal ile bulut karışıklığının önlenmesi

**Kullanıcı** olarak, **yanlışlıkla bulut adresi verdiğimde ne olduğunu
anlamak** istiyorum, çünkü **iki ürünün adı aynı ve adresleri birbirine
benziyor.**

Kabul kriterleri:

- [x] Bulut adresi verildiğinde ürün bunu **fark eder** ve bu yolun yalnızca
      kurumsal kurulumlar için olduğunu söyler
- [x] Mesaj kullanıcıyı suçlamaz; ne yapacağını söyler
- [x] Bulut adresi, kurumsal uca gönderilip anlamsız bir hata üretmez

---

## Davranış Kuralları

- **Grup adresi klonlanabilir bir adres değildir.** Ondan proje üretilmez;
  o yalnızca listeleme kaynağıdır. Her repository kendi projesi olur.

- **Varsayılan branch uydurulmaz.** Kaynak repository'nin varsayılan branch'ini
  söylemiyorsa "main" varsayılmaz. Ürünün en sert kuralı burada da geçerli:
  bilinmeyen bir değer yazılmaz. Böyle bir repository listede görünür, durumu
  yazılır ve nasıl ele alınacağı kullanıcıya bırakılır.

- **Proje adı türetilmez, kaynaktan alınır.** Ürün ada grup öneki eklemez,
  kısaltmaz, biçimlendirmez. İki farklı grupta aynı adda repository olabilir;
  onları ayıran şey ad değil adrestir ve proje listesi zaten adresi gösteriyor.
  Ada dokunmak, kullanıcının Bitbucket'ta gördüğü adla üründe gördüğü adı
  ayırır — bu, çözdüğünden çok karışıklık üretir.

- **Erişimi sınanmamış proje kaydedilmez.** Kaynak listesinden gelmiş olmak
  klonlanabildiğinin kanıtı değildir. Sınamayı geçemeyen repository eklenmez;
  eksik kalması, çalışmayan bir projenin ilk agent çalıştırmasında ortaya
  çıkmasından iyidir.

- **Kimlik bilgisi adrese gömülmez.** Kaynak, klonlama adresini kullanıcı adı
  içinde gömülü olarak verebilir; kaydedilen adres bundan arındırılır. Erişim
  bilgisi zaten ayrı ve şifreli saklanıyor.

- **Kısmi başarı geri alınmaz.** Elli repository'den biri eklenemezse
  diğer kırk dokuz eklenmiş kalır. Başarısız olanlar adıyla ve sebebiyle
  ayrıca yazılır. Bir hata yüzünden tamamlanmış işi geri almak, kullanıcının
  kaybını büyütmekten başka işe yaramaz.

- **Erişim yetkisi kullanıcınındır.** Listede yalnızca verilen erişimin
  görebildiği repository'ler çıkar. Ürün yetki genişletmez.

- **Kurumsal ile bulut ayrımı adresten yapılır, kullanıcıya sorulmaz.**
  "Hangisini kullanıyorum?" sorusu kullanıcının cevaplaması gereken bir soru
  değil; adres zaten cevabı taşıyor. Bu ayrım üründe daha önce bir kez
  yanlış yapıldı ve kendi sunucusunu kullanan herkes doğru erişim bilgisiyle
  reddedildi — aynı hataya ikinci kez düşülmez.

---

## Hata Durumları

| Durum | Beklenen davranış |
| --- | --- |
| Verilen adres bir grup adresi değil | Ne beklendiği örnekle söylenir; liste boş gösterilmez |
| Grup bulunamadı | "Bu grup bulunamadı" — adres veya yetki sorunu olabileceği yazılır |
| Erişim reddedildi | Kimlik sorunu olduğu söylenir; adres yanlış denmez |
| Sunucuya ulaşılamıyor | Ulaşılamadığı söylenir; kimlik bilgisi suçlanmaz |
| Grupta hiç repository yok | Hata değil: "Bu grupta repository yok" |
| Grupta hiç **yeni** repository yok | Hata değil: hepsinin zaten kayıtlı olduğu söylenir |
| Bir repository'nin varsayılan branch'i okunamadı | O repository listede kalır, durumu yazılır; sessizce varsayılan atanmaz |
| Bir repository'nin erişim sınaması başarısız | Eklenmez; adı ve sebebi yazılır. Diğerleri eklenmeye devam eder |
| Bir repository'nin sınaması yanıt vermiyor | Kendi süre sınırında düşer; kalan repository'ler bundan etkilenmez |
| Toplu eklemede bazıları başarısız | Başarılılar kalır; başarısızlar adı ve sebebiyle listelenir |
| Bulut adresi verildi | Bu yolun kurumsal kurulumlar için olduğu söylenir (H4) |
| Erişim tanımı seçilmedi veya yok | Önce erişim tanımlanması gerektiği, nereden yapılacağıyla birlikte söylenir |

---

## Belirsizlikler

- [x] **Gerçek bir kurumsal sunucuda doğrulama yapılabilecek mi?**
      → **Cevap: Hayır.** Deneme lisansı alınmayacak, dolayısıyla gerçek bir
      kurumsal sunucuya karşı ölçüm yapılmayacak.
      **Bunun bedeli açıkça kabul ediliyor:** kaynağın yanıt biçimine dair
      varsayımlar belgeye dayanıyor, ölçüme değil. Sunucu sürümleri arasında
      fark çıkabilir ve bu farklar ancak kullanıcı bildirimiyle ortaya çıkacak.
      Bu yüzden **hata mesajları ham yanıtı gizlememeli**: sorunu bildiren
      kişinin elinde, nedeni gösteren bir iz kalmalı. Sahte bir sunucuyla
      yapılacak sınama yalnızca **kendi kodumuzu** doğrular — varsayımımız
      yanlışsa sahte sunucu aynı yanlışı tekrarlar, dolayısıyla "doğrulandı"
      diye sunulmaz.

- [x] **Proje adı grup önekli mi olsun?**
      → **Cevap: Hayır, önek yok.** Ad kaynaktan geldiği gibi alınır.
      (Bu soru yalnızca toplu içe aktarmada doğuyor; tek repository eklerken
      adı zaten kullanıcı yazıyor.)

- [x] **Arşivlenmiş repository'ler ne olacak?**
      → **Cevap: Listede görünür, seçili gelmez.** Kullanıcı isterse elle
      seçebilir. Kaynak arşiv bilgisini vermiyorsa hepsi seçili gelir.

- [x] **Zaten kayıtlı ama başka bir erişimle bağlanmış bir repository?**
      → **Cevap: Atlanır.** Mevcut kaydın erişimi değiştirilmez; içe aktarma
      var olan bir yapılandırmayı ezmemeli.

- [x] **İçe aktarımda erişim sınaması yapılacak mı?**
      → **Cevap: Evet, yapılacak.** Seçim en fazla yüz repository olacağı
      için maliyet kabul edilebilir sayıldı. Sınama sırası değil paralel
      yürütülmeli; seri yürütmede yüz repository'nin en kötü hâli dakikalarla
      ölçülür ve bu, kabul kriterindeki "makul süre"yi karşılamaz.

## Bağımlılıklar

- Kurumsal Bitbucket erişiminin tanımlanabiliyor ve doğrulanabiliyor olması.
  Bu yetenek üründe **zaten var**; bu spec onun üzerine kuruluyor, onu
  değiştirmiyor.
