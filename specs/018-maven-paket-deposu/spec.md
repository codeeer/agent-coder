# Spec: Java / Maven ve kurumsal paket deposu

- **Spec no:** 018
- **Tarih:** 2026-08-14
- **Durum:** Uygulandı (2026-08-14)

---

## Problem

Agent bugün Java projelerinde çalışamıyor. Çalışma ortamında ne Java ne de
Maven var; agent bir Java deposunu klonlayabiliyor ama derleyemiyor,
çalıştıramıyor, test edemiyor.

Java varsayılsa bile ikinci bir engel çıkardı: kurumsal ağda Maven bağımlılıkları
yalnızca iç depodan çekilebiliyor. npm için bu spec 014'te çözüldü — adres
ayarlardan veriliyor, kimlik şifreli saklanıyor ve agent'a "bu adresle oynama"
deniyor. Maven'ın böyle bir karşılığı yok.

Maven'da kaçış yolu npm'dekinden daha geniş: proje dosyasının kendisi bağımlılık
deposu ilan edebiliyor. Yani yalnızca "varsayılan depoyu değiştir" demek yetmez;
projenin ilan ettiği depoların da kuruma yönlendirilmesi gerekir, aksi halde
kapalı ağda derleme, sebebi anlaşılmayan bir zaman aşımıyla düşer.

Üçüncü bir engel daha var ama bu spec onu **çözmüyor**: çalışma ortamı her
koşuda sıfırdan doğup silindiği için indirilen bağımlılıklar da siliniyor.
Ölçüldü — gerçek bir projede 5 dakika 49 saniye ve 569 MB. Sorun gerçek;
çözümü ise elimizdeki veriyle henüz tasarlanamıyor (bkz. Kapsam dışı).

## Amaç

Agent'ın Java projelerinde derleme ve test çalıştırabilmesi ve bunu yaparken
tüm bağımlılıkların kurumsal depodan gelmesi.

## Kapsam dışı

- **Java sürümünün arayüzden seçilmesi.** Çalışma ortamında iki sürüm birden
  bulunur ve agent'a hangisini kullanacağı **talimatla** söylenir. Proje ayarı,
  koşu seçicisi veya sürüm listesi eklenmez.
- **Diğer paket yöneticileri** (Python, Go, .NET).
- **Gradle.** Yalnızca Maven ele alınır.
- **Kurumsal sertifika mekanizmasının kendisi** — spec 017. Bu spec yalnızca o
  sertifikanın Java tarafında da geçerli olmasını ister.
- **Bağımlılık önbelleği — hiçbir biçimiyle.** Ne imaja gömülü hazır liste, ne
  paylaşılan volume. Ölçüldü ve konuşulan çözüm çürütüldü: gerçek bir projede
  569 MB'ın yalnızca **%3'ü** Spring'e aitti; ağırlık, projenin ana POM'unun
  sürüklediği derleme araç zincirindeydi (Kotlin, OpenRewrite, RocksDB,
  BouncyCastle). Tahmine dayalı bir hazır liste, imajı ~600 MB büyütüp
  ağırlığın onda birine bile dokunmazdı. Sorun gerçek ve **unutulmadı**:
  ölçümlerle ve denenmemiş yollarla birlikte
  [plans/03-bagimlilik-onbellegi](../../plans/03-bagimlilik-onbellegi-2026-08-14.md)
  altına yazıldı; doğru çözüm veri toplandıktan sonra tasarlanacak.
- **Çalışma ortamı imajının kapalı ağda derlenmesi.** İmaj bugün derlenirken
  genel depolara çıkıyor; bu ayrı bir sorun.

---

## Kullanıcı Hikâyeleri

### H1 — Java projesinde çalışabilmek

**Geliştirici** olarak, agent'ın bir Java projesini **derleyip test
edebilmesini** istiyorum, çünkü bugün depoyu klonlayabiliyor ama tek bir komut
çalıştıramıyor.

Kabul kriterleri:

- [x] Agent çalışma ortamında Maven komutları çalıştırabilir
- [x] Çalışma ortamında **iki** Java sürümü birden bulunur
- [x] Sürümlerden biri varsayılandır ve ne olduğu bellidir
- [x] Agent, diğer sürüme geçebilmek için gereken bilgiyi **talimatında** bulur;
      bu bilgiyi tahmin etmesi veya araması gerekmez
- [x] Sürümlerin yeri, çalışma ortamının hangi işlemci mimarisinde çalıştığından
      bağımsız olarak aynı şekilde ifade edilir
- [x] Java kullanmayan çalıştırmaların davranışı değişmez

### H2 — Bağımlılıkların kurumsal depodan gelmesi

**Yönetici** olarak, Maven bağımlılıklarının **kurumsal depodan** çekilmesini
istiyorum, çünkü kapalı ağda genel depolara erişilemiyor.

Kabul kriterleri:

- [x] Ayarlarda, npm adresinin yanında bir **Maven deposu adresi** alanı var
- [x] Alan boş bırakılabilir; boş = özellik kapalı, bugünkü davranış
- [x] Adres tanımlıyken agent'ın çalıştırdığı Maven komutları bağımlılıkları o
      adresten çeker
- [x] Bağımlılık, **projenin kendi dosyasında** başka bir depo ilan edilmiş olsa
      bile kurumsal adresten çekilir
- [x] Kimlik doğrulama, npm ile **aynı** kayıtlı kimlik bilgisi kullanılarak
      yapılır; ikinci bir kullanıcı adı veya parola istenmez
- [x] Kimlik kaydı hiç yoksa kimlik doğrulama yapılmaz ve anonim okumaya açık
      depolar çalışır
- [x] Parola veya token ortam değişkenine konmaz; agent'ın göremeyeceği bir
      yerde durur
- [x] Agent'ın talimatında "bu depo adresini değiştirme" bilgisi bulunur

### H3 — Kurumsal sertifikanın Java tarafında da geçerli olması

**Yönetici** olarak, tanımladığım kurumsal sertifikanın **Java için de**
geçerli olmasını istiyorum, çünkü Java kendi güven listesini kullanıyor ve
diğer araçlar için yapılan tanıtımı görmüyor.

Kabul kriterleri:

- [x] Sertifika tanımlıyken Maven, kurumsal depoya güvenlik hatası almadan
      ulaşır
- [x] Sertifika tanımlı değilken davranış bugünküyle aynıdır
- [x] Sertifika, Java'nın güvendiği listeye **eklenir**; genel sertifikalar
      geçerli kalır
- [x] Sertifika değiştirildiğinde sonraki çalıştırma yeni sertifikayı kullanır

### H4 — Ulaşılamayan deponun çalıştırmayı yakmaması

**Yönetici** olarak, paket deposuna ulaşılamadığında bunun **hızlıca**
anlaşılmasını istiyorum, çünkü bugün adres yanlış veya erişilemezken
çalıştırma dakikalarca sessizce bekliyor.

Ölçüldü (spec 017 doğrulaması sırasında): kurumsal depoya güvenlik hatası
alınan bir çalıştırmada tek bir paket için **~4 dakika** harcandı. Sebep, paket
yöneticisinin varsayılan bekleme süresinin tek istek için beş dakika olması ve
üstüne birkaç kez yeniden denemesi. Kullanıcı bu süre boyunca ekranda yalnızca
"çalışıyor" görüyor; çalıştırmanın süre bütçesi ise boş yere tükeniyor.

Kabul kriterleri:

- [x] Paket deposu çağrılarının bekleme süresi **ayarlardan** değiştirilebilir
- [x] Ayarın makul bir varsayılanı vardır ve bir aralıkla sınırlıdır
- [x] Aynı ayar **hem npm hem Maven** için geçerlidir; kullanıcı iki ayrı yerde
      aynı kararı vermez
- [x] Ulaşılamayan bir depoda çalıştırma, bugünkünün belirgin biçimde altında
      bir sürede sonuca varır
- [x] Ölü bir adres, sonuç değiştirmeyecek biçimde defalarca denenmez
- [x] Süre sınırı ayarı, kurumsal adres tanımlı **değilken** davranışı
      değiştirmez

---

## Davranış Kuralları

- **Mevcut işlevsellik korunur.** Java kullanmayan çalıştırmalar aynı sonucu
  verir.
- **Kimlik tek yerdedir.** Paket deposu kimliği npm ve Maven için ortaktır;
  aynı sırrın iki kopyası tutulmaz. **Süre sınırı da öyle**: "paket deposu ne
  kadar bekler" sorusunun tek bir cevabı olur.
- **Süre sınırı kodda gömülü değildir.** Bekleme süresi, davranışı belirleyen
  bir parametredir ve bu ürünün kuralı gereği ayarlarda durur. Kuruma göre
  doğru değer değişiyor: iç ağdaki bir depo için kısa olan süre, yavaş bir
  hattın ucundaki depo için çalışan bir kurulumu kırabilir.
- **Adres ayarda, sır kimlik deposunda.** Spec 014'te kurulan sınır korunur;
  yapılandırma dosyasının tamamı kullanıcıya yazdırılmaz.
- **Ölçülmeyen gösterilmez.** Ekrana yeni bir sayaç veya istatistik eklenmez.
- **Çalışma ortamı büyür.** İki Java sürümü ve Maven imajı belirgin biçimde
  büyütür; bu bilinçli bir takastır ve belgelenir.

## Hata Durumları

| Durum | Beklenen davranış |
| --- | --- |
| Maven adresi tanımlı, kimlik yok, depo kimlik istiyor | Maven yetkisizlik hatası alır; hata agent'ın çıktısında görünür ve kimlik eksikliğine işaret eder |
| Maven adresi tanımlı ama adrese ulaşılamıyor | Hata, adresin ulaşılamadığını söyler; agent'ın bunu "bağımlılık yok" diye yorumlamaması için talimatında karşılığı bulunur |
| Adres tanımlı değil, kapalı ağda çalışılıyor | Maven genel depoya çıkmaya çalışır ve düşer; bu bugünkü davranıştır ve bu spec onu değiştirmez |
| Agent kendi depo tanımını eklemeye kalkıyor | Yapılandırma buna izin vermez; bağımlılık yine kurumsal adresten çekilir |
| Sertifika tanımlı değil, kurumsal depo kendi sertifikasını kullanıyor | Maven güven hatası alır; çözümü spec 017'nin tanımladığı sertifikadır |
| Depoya hiç ulaşılamıyor | Süre sınırı dolunca vazgeçilir; çalıştırma dakikalarca beklemez ve hata agent'ın çıktısında görünür |
| Süre sınırı büyük bir paket için yetmiyor | Ayar yükseltilebilir; sınır kodda gömülü değildir |

---

## Belirsizlikler

- [x] Java sürümü değişken mi, sabit mi? → **Cevap:** İki sürüm birden çalışma
      ortamında bulunur (17 ve 25); hangisinin kullanılacağı agent'a talimatla
      söylenir. Sürüm seçici eklenmez: seçim yüzeyi eklemek proje ayarı, koşu
      alanı, doğrulama listesi ve arayüz seçicisi demekti — talimat aynı işi
      hiçbir yeni parça olmadan yapıyor.
- [x] Java sürümleri ayrı imajlar olarak mı yayınlansın? → **Cevap:** Hayır,
      tek imajda birlikte. Ayrı imajlar etiket eksenini ikiye çıkarır ve beş
      ayrı yeri (etiket kurgusu, yerel derleme, sürekli entegrasyon, doğrulama,
      arayüz) etkilerdi. Seçim yüzeyi iki yolda da aynı olduğu için bu karar
      geri alınabilir: ileride boyut sorun olursa yalnızca imaj derleme tarafı
      değişir.
- [x] Maven için ayrı kimlik mi? → **Cevap:** Hayır, npm ile aynı kimlik; yalnızca
      adres ayrı. Kurumsal depoda ikisi aynı sunucunun farklı yollarıdır.
- [x] Yapılandırma dosyasının tamamı ayarlardan mı girilsin? → **Cevap:** Hayır,
      yalnızca adres. Dosyanın tamamı yapıştırılsaydı içindeki parola düz metin
      olarak ayarlarda dururdu; bu ürünün sırları şifreli tuttuğu sınırı delerdi.
      Ayrıca yapıştırılan bir dosya doğrulanamaz.
- [x] Bağımlılık önbelleği bu spec'e girsin mi? → **Cevap:** Hayır, hiçbir
      biçimiyle. Ölçüm, konuşulan çözümü çürüttü (ağırlığın %97'si Spring'de
      değil, projenin araç zincirinde). Sorun gerçek olduğu için unutulmaya
      bırakılmadı; ölçümlerle birlikte `plans/03`'e taşındı ve doğru çözüm
      veriyle tasarlanacak.
## Bağımlılıklar

- **Spec 017** (kurumsal ağ sertifikası) — H3 o spec tamamlanmadan uygulanamaz.
- Spec 014 (kurumsal paket deposu) tamamlanmıştır; H2 onun kurduğu ayar ve
  kimlik yapısını genişletir.
