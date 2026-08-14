# Spec: Script klasörleri — standart upgrade kampanyaları

- **Spec no:** 022
- **Tarih:** 2026-08-14
- **Durum:** Onaylandı

---

## Problem

**Aynı standart değişikliği onlarca projede aynı şekilde uygulamanın yolu yok.**

Somut durum: kurumsal bir framework üzerinde geliştirilen otuz proje Node 18'de
ve Node 24'e yükseltilecek. Framework ekibi hangi dosyalarda ne değişeceğini
yedi adım olarak belirledi. Adımların çoğu mekanik ve her projede aynı; ama
aralarında karar gerekiyor — bir adımdan sonra derleme kırılabiliyor, agent onu
düzeltip devam etmeli.

Ürün bu iş için gerekli parçanın çoğuna sahip: yazılmış, gözden geçirilmiş
script'ler agent'a atanabiliyor, container'a kopyalanıyor ve agent'ın
talimatına yolu ve açıklamasıyla yazılıyor. Eksik olan **düzen**:

- Kütüphane **düz**. Yedi Node 24 script'i, beş Spring script'i ve üç genel
  script aynı listede yan yana duruyor.
- Atama **tek tek**. Bir kampanyayı agent'a bağlamak yedi kutucuk işaretlemek
  demek; yedincisi unutulursa hata çalıştırma sırasında ortaya çıkıyor.
- **Kampanya diye bir şey yok.** "Node 24 yükseltmesi" bir isim taşımıyor;
  hangi script'lerin ona ait olduğu yalnızca adlandırma disiplininde yaşıyor.

Kütüphane büyüdükçe bu üçü birlikte bozuluyor: ikinci kampanya eklendiğinde
liste karışıyor, üçüncüde hangi script'in neye ait olduğu ancak adına bakarak
tahmin ediliyor.

## Amaç

Bir kampanyanın script'lerinin **tek bir klasör altında toplanması**, agent'a
**tek hamlede** bağlanabilmesi ve agent'ın o klasörü — dizin yolu ve içindeki
adımlarla birlikte — talimatında görmesi.

## Kapsam dışı

- **Toplu çalıştırma.** Bir akışı otuz projede birden koşturmak ayrı bir iş
  (ayrı spec). Bu spec bittiğinde otuz proje hâlâ teker teker tetiklenir.
- **Sıranın ürün tarafından zorlanması.** Adımlar arasında agent karar
  veriyor; klasör sırayı **anlatır**, işletmez. Sıra mekanik olsaydı doğru
  çözüm yedi script değil, yedisini çağıran tek script olurdu.
- **Yetki modeli.** Script'ler bugün olduğu gibi yalnızca bash yetkisi açık
  agent'lara kopyalanır. Klasör bu kuralı ne genişletir ne daraltır.
- **Script içeriğinin sürümlenmesi.** Bir kampanyanın geçmiş sürümlerini
  saklamak ayrı bir ihtiyaç.
- **Klasörün projeye veya akış adımına bağlanması.** Bağlanma noktası bugünkü
  gibi **agent**.
- **Script'lerin ürün tarafından üretilmesi.** Klasörün içindekileri kullanıcı
  yazar; ürün onları çalıştırılabilir hale getirir ve anlatır.

---

## Kullanıcı Hikâyeleri

### H1 — Kampanyayı klasör olarak toplama

**Standart bir yükseltmeyi yürüten kullanıcı** olarak, **bir kampanyanın
script'lerini tek bir klasörde toplamak** istiyorum, çünkü **kütüphane
büyüdükçe hangi script'in hangi işe ait olduğunu adından tahmin etmek
istemiyorum.**

Kabul kriterleri:

- [ ] Kullanıcı ad ve açıklama vererek klasör oluşturabilir
- [ ] Yeni bir script oluştururken bir klasöre konabilir ya da klasörsüz
      bırakılabilir
- [ ] Var olan bir script'in klasörü sonradan değiştirilebilir; klasörden
      çıkarılabilir
- [ ] Bir script **en fazla bir** klasörde bulunur
- [ ] Klasör listesi, her klasörün kaç script taşıdığını gösterir
- [ ] Klasörsüz script'ler kaybolmaz; kendi başlıkları altında görünür

### H2 — Ortak script'lerin klasör dışında kalması

**Kullanıcı** olarak, **birden fazla kampanyada işe yarayan script'leri
klasörsüz bırakmak** istiyorum, çünkü **onları her klasöre kopyalamak, zamanla
birbirinden ayrışan kopyalar üretir.**

Kabul kriterleri:

- [ ] Klasörsüz script'ler kütüphanede ayrı bir grup olarak durur
- [ ] Bir agent'a **klasör** ve **klasörsüz script** aynı anda atanabilir
- [ ] Klasörsüz bir script birden fazla agent'a atanabilir

### H3 — Klasörü agent'a tek hamlede bağlama

**Kullanıcı** olarak, **bir kampanyayı agent'a tek seçimle bağlamak**
istiyorum, çünkü **yedi kutucuktan birini unutmak, hatayı çalıştırma anına
erteler.**

Kabul kriterleri:

- [ ] Agent'a klasör atandığında klasörün **tüm** script'leri o agent'ta
      geçerli olur
- [ ] Klasöre sonradan eklenen bir script, o klasörü atamış agent'larda
      **sonraki çalıştırmada** kendiliğinden geçerli olur; atama tekrar
      yapılmaz
- [ ] Klasörden çıkarılan bir script o agent'larda artık geçerli olmaz
- [ ] Agent ekranında hangi klasörlerin ve hangi tekil script'lerin atandığı
      görünür

### H4 — Agent'ın klasörü görmesi

**Kullanıcı** olarak, **agent'ın klasörü ve içindeki adımları talimatında
görmesini** istiyorum, çünkü **model, varlığını bilmediği bir dizini
kullanmaz.**

Kabul kriterleri:

- [ ] Agent'ın talimatına klasörün **dizin yolu** yazılır
- [ ] Klasörün açıklaması yazılır — kampanyanın ne olduğunu model buradan
      anlar
- [ ] Klasördeki script'ler **adlarının sırasıyla** listelenir; her birinin
      yolu ve açıklaması yazılır
- [ ] Klasörsüz script'ler bugünkü gibi ayrı listelenir
- [ ] Klasör de tekil script de yoksa bu bölüm hiç yazılmaz
- [ ] Container içinde klasörün script'leri gerçekten o dizin altında durur

### H5 — Klasörün silinmesi

**Kullanıcı** olarak, **bir kampanya bittiğinde klasörü kaldırmak** istiyorum,
çünkü **biten kampanyaların listede durması, yaşayanları gizler.**

Kabul kriterleri:

- [ ] Klasör silinmeden önce kaç script taşıdığı ve kaç agent'a atandığı
      söylenir
- [ ] Klasör silindiğinde içindeki script'ler **silinmez**, klasörsüz kalır
- [ ] Silme, o klasörü kullanan agent'ların diğer atamalarını etkilemez

---

## Davranış Kuralları

- **Klasör bir kampanyadır, bir dizin değildir — ama dizine dönüşür.**
  Kullanıcı için "Node 24 yükseltmesi"; container içinde gerçek bir alt dizin.
  İkisinin adı aynıdır ki kullanıcının gördüğü ile agent'ın kullandığı yol
  ayrışmasın.

- **Sıra addan gelir.** Klasör içindeki script'ler adlarına göre sıralanır.
  Kullanıcı sırayı `01-`, `02-` gibi öneklerle kurar; ürün ayrı bir sıra
  bilgisi tutmaz. Böylece sıra hem arayüzde hem `ls` çıktısında aynı görünür ve
  iki yerde ayrışamaz.

- **Klasör yetki açmaz.** Bugünkü kural aynen geçerli: bash yetkisi kapalı bir
  agent'a hiçbir script — klasörlü ya da klasörsüz — kopyalanmaz ve talimatında
  anlatılmaz.

- **Atama klasöre yapılır, içeriğine değil.** Klasöre eklenen script otomatik
  geçerli olur. Bu bilinçli: kampanya büyürken her agent'ı tekrar düzenlemek,
  "tek yerden güncelle" vaadini bozardı.

- **Ad çakışması klasör içinde denetlenir.** İki farklı klasörde aynı adda
  script olabilir; aynı klasörde olamaz. Dosya yolları farklı olduğu için
  çakışma da yoktur.

- **Klasörsüz script'lerin bugünkü davranışı değişmez.** Bu spec var olan hiçbir
  kurulumu bozmaz: mevcut script'ler klasörsüz olarak kalır ve aynı yerde,
  aynı yolda çalışmaya devam eder.

---

## Hata Durumları

| Durum | Beklenen davranış |
| --- | --- |
| Klasör adı boş | Ad zorunlu denir |
| Klasör adı geçersiz karakter içeriyor | Hangi karakterlerin geçerli olduğu yazılır (dizin adına dönüştüğü için dar bir küme) |
| Aynı adda klasör zaten var | Var olduğu söylenir; sessizce ikinci kayıt açılmaz |
| Aynı klasörde aynı adda script | Çakışma söylenir; hangi klasörde olduğu yazılır |
| Klasör silinirken içinde script var | Kaç script'in klasörsüz kalacağı söylenir, sonra onaylatılır |
| Bash yetkisi kapalı agent'a klasör atanmış | Atama korunur ama çalıştırmada hiçbir script kopyalanmaz; agent ekranında bu durum **görünür** olur |

---

## Belirsizlikler

- [x] **Sıra ad önekiyle mi kurulmalı, yoksa ürün ayrı bir sıra bilgisi mi
      tutmalı?** Önek (`01-`, `02-`) sıfır makine gerektiriyor ve sıra hem
      arayüzde hem container'daki `ls` çıktısında aynı görünüyor. Buna karşılık
      kullanıcı öneki koymayı unutursa sıra alfabetik olur ve sessizce yanlış
      çalışır. Ayrı bir sıra bilgisi bu tuzağı kapatır ama iki doğruluk kaynağı
      üretir (kayıttaki sıra ile dosya adı) ve container'a bakan biri yanlış
      sırayı görebilir.
      → **Cevap: ad öneki.** Arayüz sıralamanın ada göre olduğunu açıkça
      yazar; ürün ayrı bir sıra bilgisi tutmaz.

- [x] **Bash yetkisi kapalı bir agent'a klasör atanabilmeli mi?** Engellemek
      "sonra açarım" diyen kullanıcıyı tıkar; izin verip sessiz kalmak ise
      çalışmayan bir yapılandırma üretir. → **Cevap: izin verilir**, agent
      ekranında uyarı görünür.

## Bağımlılıklar

- Script kütüphanesi ve agent'a atama (spec 012). Bu spec onun üzerine
  kuruluyor; kurallarını değiştirmiyor.
