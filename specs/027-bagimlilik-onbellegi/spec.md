# Spec: Bağımlılık önbelleği — koşular arası paylaşılan artefakt deposu

- **Spec no:** 027
- **Tarih:** 2026-08-19
- **Durum:** Taslak
- **İlgili plan:** [plans/03 — Bağımlılık Önbelleği: Ölçüldü, Çözülmedi](../../plans/03-bagimlilik-onbellegi-2026-08-14.md)

---

## Problem

**Her koşu bağımlılıkları sıfırdan indiriyor.** Çalışma ortamı her koşuda
doğuyor ve siliniyor; indirilen artefaktlar da onunla gidiyor. Bir sonraki
koşu, bir dakika önce inen aynı ağacı baştan indiriyor.

Bedeli tahmin değil, **ölçüldü** ([plans/03](../../plans/03-bagimlilik-onbellegi-2026-08-14.md)):
gerçek bir Java projesinde soğuk bir başlangıç **5 dakika 49 saniye ve 569 MB**,
797 artefakt — üstelik kurumsal paket deposu iç ağdayken.

Bu bedel üç yerde çarpılıyor:

1. **Kampanya ölçeğinde.** Devam eden Spring Boot 3.1 → 4.1 / Spring 7
   kampanyası 50'den fazla uygulamayı kapsıyor, uygulama başına yaklaşık 190
   artefakt. Uygulamaların büyük kısmı **aynı** çekirdek ağacı çekiyor; o ağaç
   bugün 50 kez iniyor.
2. **Toplu çalıştırmada.** Bir akış onlarca projede sıraya konduğunda
   ([spec 023](../023-toplu-calistirma/spec.md)) indirme süresi koşu sayısıyla
   çarpılıyor.
3. **Aynı koşunun içinde.** Agent bir hedef sürümü deneyip geri aldığında
   ikinci bir tam indirme daha yapılıyor.

Sonuç: koşu süresinin büyük kısmı model çalışmıyor, ağ bekliyor. Kullanıcı
kampanyanın ilerlemesini değil, indirme çubuğunu izliyor.

`plans/03` bu sorunu ölçtü ama **çözümü açık bıraktı.** Denenen ilk yol —
sık kullanılan bağımlılıkları çalışma ortamı imajına önceden gömmek — aynı
ölçümle çürüdü: 569 MB'ın yalnızca %3'ü uygulama bağımlılıklarıydı, geri kalanı
projenin kendi derleme araç zinciriydi ve o zincir projeden projeye tamamen
değişiyordu. Tahmine dayalı bir liste tutmak, isabeti savrulan ve çalışma
ortamını yaklaşık 600 MB büyüten bir yatırım demekti.

## Amaç

Koşuların indirdiği artefaktlar **koşu bitince silinmesin**; bir sonraki koşu
onları hazır bulsun. Önbellek kendi kendine ısınsın — hangi sürümlerin
tutulacağına dair bir liste tutulmasın, indirilen her şey sonraki koşuya
sunulsun. Kampanyanın ilk koşusu bedeli öder, kalan 50 koşu ödemez.

## Kapsam dışı

- **Bağımlılıkları çalışma ortamına önceden gömmek.** `plans/03` bunu ölçümle
  eledi; bu spec o yolu yeniden açmıyor.
- **Hangi sürümlerin tutulacağını seçmek.** Önbellek kullanıldıkça dolar;
  seçici, beyaz liste veya sürüm tahmini yok.
- **Otomatik boyut sınırı veya yaşa göre temizleme.** Bu sürümde temizlik
  kullanıcının elle verdiği bir karar. Saklama politikası ihtiyacı doğarsa
  ayrıca ele alınır.
- **Salt okunur önbellek modu.** Koşuların önbelleği yalnızca okuduğu, doldurma
  işini ayrı ve denetimli bir sürecin yaptığı düzen bu sürümde yok; güven
  varsayımı değişirse (bkz. Davranış Kuralları) değerlendirilecek ilk seçenek
  budur.
- **Kaynak kodun veya çalışma dizininin koşular arasında saklanması.** Yalnızca
  indirilen bağımlılıklar kalıcı olur; çalışma dizini her koşuda temiz doğmaya
  devam eder ([spec 025](../025-calisma-dizini-yerlesimi/spec.md)).
- **Kurumsal paket deposu yapılandırması.** Hangi depodan indirileceği ve kimlik
  bilgileri bugünkü gibi koşu başına yazılmaya devam eder
  ([spec 014](../014-kurumsal-paket-deposu/spec.md),
  [spec 018](../018-maven-paket-deposu/spec.md)); bu spec yalnızca **indirilenin
  nerede saklandığını** değiştirir.
- **Java ve Node dışındaki ekosistemler.** Bu sürümde Maven ve npm.

---

## Kullanıcı Hikâyeleri

### H1 — İkinci koşu indirmez

**Kampanya yürüten kullanıcı** olarak, aynı projede arka arkaya iki koşu
yaptığımda **ikincisinin bağımlılıkları yeniden indirmemesini** istiyorum,
çünkü indirme süresi kampanyanın en büyük kalemi.

Kabul kriterleri:

- [ ] Önbellek açıkken aynı proje üzerinde iki koşu arka arkaya yapıldığında,
      **ikinci koşunun derleme çıktısında hiçbir artefakt için indirme satırı
      görünmez.**
- [ ] İkinci koşunun bağımlılık çözme aşaması birincisinden **belirgin biçimde
      kısa** sürer; fark koşu detayında görünen süreye yansır.
- [ ] Farklı bir projede yapılan koşu, ortak artefaktları **indirmeden** bulur;
      yalnızca o projeye özgü olanları indirir.

### H2 — Kapalıyken hiçbir şey değişmez

**Yönetici** olarak, önbelleği **açıp kapatabilmek** istiyorum ve kapalıyken
sistemin bugünküyle **birebir aynı** davranmasını istiyorum, çünkü çalışan bir
kurulumu yeni bir özellik yüzünden riske atmak istemiyorum.

Kabul kriterleri:

- [ ] Önbellek **varsayılan olarak kapalıdır**; hiçbir ayar değiştirilmeden
      yapılan kurulum bugünkü davranışı gösterir.
- [ ] Ayar kapatıldığında sonraki koşular önbelleğe **ne yazar ne okur**;
      artefaktlar bugünkü gibi her koşuda yeniden iner.
- [ ] Ayarın değiştirilmesi **sunucunun yeniden başlatılmasını gerektirmez**;
      değişiklik bir sonraki koşuda geçerlidir.
- [ ] Ayar kapatıldığında biriken önbellek **silinmez**; yeniden açıldığında
      önceki birikim kullanılmaya devam eder.

### H3 — Ne kadar yer tuttuğunu görürüm ve temizleyebilirim

**Yönetici** olarak, önbelleğin **ne kadar yer tuttuğunu görmek** ve gerektiğinde
**temizlemek** istiyorum, çünkü kendi kendine dolan ve hiç küçülmeyen bir alan,
sunucunun diskini sessizce bitirir.

Kabul kriterleri:

- [ ] Ayarın yanında **her ekosistemin boyutu ayrı ayrı** okunabilir biçimde
      görünür — tek bir toplam, diski hangisinin doldurduğunu söylemezdi.
- [ ] Hiç koşu yapılmamışsa boyut **"0" değil, "henüz kullanılmadı"** anlamında
      gösterilir — sıfır, çalışmış ama boş kalmış bir önbellekle karışır.
- [ ] "Temizle" eylemi **her ekosistem için ayrı** çalışır; biri temizlenirken
      diğerinin birikimi durur.
- [ ] "Temizle" onay ister ve onaydan sonra ilgili önbellek boşalır; boyut
      göstergesi bunu yansıtır.
- [ ] **Süren bir koşu varken temizleme yapılmaz**; kullanıcıya sebebi açıkça
      söylenir.
- [ ] Temizlemeden sonraki ilk koşu artefaktları yeniden indirir ve önbelleği
      yeniden doldurur.

### H4 — Eşzamanlı koşular birbirini bozmaz

**Kampanya yürüten kullanıcı** olarak, aynı anda koşan işlerin **birbirinin
bağımlılıklarını bozmamasını** istiyorum, çünkü bozuk bir artefakt, sebebi
günlerce anlaşılmayan derleme hatalarına yol açar.

Kabul kriterleri:

- [ ] Aynı artefaktı aynı anda çözen iki koşu, **ikisi de başarıyla** tamamlanır.
- [ ] Eşzamanlı koşulardan sonra önbellekteki artefaktlar **eksiksiz ve
      geçerlidir**; yarım inmiş veya bozuk dosya kalmaz **ve iki koşu da
      başarıyla biter.**
- [ ] Toplu çalıştırmanın eşzamanlılık sınırına kadar açılan koşularda bu
      davranış korunur.

### H5 — Önbelleğin bozulmadığını doğrularım

**Yönetici** olarak, önbellekteki artefaktların **indirildikleri hâlde durup
durmadığını denetleyebilmek** istiyorum, çünkü önbellek koşular arası
yazılabilir bir alan ve bir koşunun oraya bıraktığı bozuk veya değiştirilmiş
bir artefakt, sonraki koşulara sessizce bulaşır.

Kabul kriterleri:

- [ ] "Doğrula" eylemi, önbellekteki artefaktları indirildikleri sırada
      kaydedilen özetleriyle karşılaştırır ve **kaç artefaktın denetlendiğini,
      kaçının uyuşmadığını** söyler.
- [ ] Uyuşmayan artefakt **önbellekten silinir**; sonraki koşu onu kaynağından
      yeniden indirir ve koşu başarıyla tamamlanır.
- [ ] Hiçbir uyumsuzluk yoksa sonuç bunu açıkça söyler — sessiz bir bitiş,
      "çalıştı mı çalışmadı mı" sorusunu bırakır.
- [ ] Doğrulama **süren koşuları etkilemez**; koşular sürerken çalıştırılamıyorsa
      sebebi açıkça söylenir.
- [ ] Özeti kaydedilmemiş bir artefakt "bozuk" sayılmaz; denetlenemeyenlerin
      sayısı ayrıca bildirilir.
- [ ] **Okunamayan veya bozuk bir özet dosyası asla silmeye yol açmaz**;
      artefakt "denetlenemedi" sayılır. Silme yalnızca özeti okunabilen ve
      özetiyle **uyuşmayan** artefakta uygulanır — aksi hâlde kırpılmış bir
      özet dosyası yüzünden sağlam bir artefakt silinir ve doğrulama,
      düzeltmesi gereken sorunun kaynağına dönüşür.

---

## Davranış Kuralları

- **Önbellek bütün projeler arasında paylaşılır.** Kampanyanın kazancı zaten
  burada: 50 uygulama aynı çekirdek ağacı kullanıyor. Proje başına ayrı bir
  önbellek, ortak ağacı 50 kez indirmek demek olurdu ve özelliğin varlık
  sebebini ortadan kaldırırdı.
- **Paylaşımın güven sınırı bilinerek kabul ediliyor.** Önbellek koşular arası
  **yazılabilir** bir kanaldır: bir koşunun indirdiği artefaktı sonraki bir
  koşu kullanır. Bu kabul edilebilir çünkü bu kurulumdaki projelerin tamamı
  aynı kuruma ait, artefaktların tamamı aynı kurumsal paket deposundan geliyor
  ve koşular zaten o depoya ve kod deposuna erişebiliyor — paylaşılan önbellek
  var olmayan bir güven sınırı açmıyor. **Bu varsayım değişirse** — birbirine
  güvenmeyen ekiplerin aynı kurulumu paylaşması gibi — bu karar yeniden
  değerlendirilmelidir.

  Riski tamamen ortadan kaldırmayan ama küçülten **iki hafifletici** var:

  1. **Önbellek doğrulanabilir.** Kullanıcı, önbellekteki artefaktların
     indirildikleri sırada kaydedilen özetleriyle uyuşup uyuşmadığını
     denetleyebilir; uyuşmayan artefakt silinir ve bir sonraki koşuda kaynağından
     yeniden iner. Bu, zehirlenmeyi **önlemez** ama fark edilebilir ve geri
     alınabilir kılar — bugün öyle bir imkân hiç yok.
  2. **Salt okunur mod bu sürümde yok.** Koşuların önbelleği yalnızca okuduğu,
     doldurmayı ayrı ve denetimli bir işlemin yaptığı düzen kapsam dışı
     bırakıldı; güven varsayımı değişirse değerlendirilecek ilk seçenek budur.
- **Koşu başına yazılan yapılandırma dosyalarına dokunulmaz.** Hangi depodan
  indirileceğini ve kimlik bilgilerini taşıyan dosyalar bugünkü gibi her koşuda
  yeniden yazılır; kalıcı olan yalnızca **indirilen artefaktların durduğu
  dizindir.** Kimlik bilgisi önbellekte saklanmaz.
- **Önbellek bir hızlandırıcıdır, önkoşul değildir.** İçindeki bir artefakt
  silinirse koşu başarısız olmaz, yeniden indirir. Önbellek hiç kullanılamaz
  hale gelirse de koşu **durmaz**: önbelleksiz sürer ve sebebi olay akışında
  görünür. Bir kampanya koşusu, hızlandırıcısı çalışmadığı için kaybolmamalıdır.
- **Yalıtım kararıyla ilişkisi.** Ürünün koşu ortamına **host makinesindeki bir
  dosyayı bağlama** yasağı bu önbelleği kapsamaz: burada bağlanan şey host'ta
  bulunması gereken bir dosya değil, ürünün kendi oluşturup yönettiği ve koşu
  ortamı uzak bir makinede çalışsa da erişilebilen bir alandır — yasağın
  gerekçesi olan "dosyanın o makinede bulunması" koşulu burada doğmuyor.
- **Önbelleğin varlığı koşu çıktısında görünür olmalıdır.** Kullanıcı bir
  koşunun önbellekten mi beslendiğini yoksa indirme mi yaptığını
  anlayabilmelidir; aksi halde "neden bu koşu yavaştı" sorusu cevapsız kalır.

## Hata Durumları

| Durum | Beklenen davranış |
| --- | --- |
| Önbellek alanı oluşturulamıyor (disk dolu, izin yok) | Koşu **durmaz, önbelleksiz devam eder.** Olay akışına sebebiyle birlikte görünür bir uyarı düşer. Sessizce devam edilmez — kullanıcı hızlanma beklerken yavaş koşuyu hata sanmamalı |
| Önbellek açık ama koşu ortamı ona erişemiyor | Aynı: koşu önbelleksiz sürer, olay akışına sebebiyle uyarı düşer |
| Disk koşu sırasında doluyor | Koşu, ilgili derleme aracının kendi hatasıyla başarısız olur; hata mesajında önbellek diskinin dolduğu belirtilir |
| Temizleme sırasında süren koşu var | Temizleme yapılmaz, kullanıcıya kaç koşunun sürdüğü söylenir |
| Temizleme yarıda kalırsa | Önbellek tutarsız bırakılmaz; sonraki koşu eksik artefaktları yeniden indirerek düzelir |
| Önbellekteki bir artefakt bozuksa | Koşu sırasında derleme aracının kendi doğrulaması devreye girer; ürün koşu başına ayrıca bir bütünlük taraması yapmaz — tarama, kullanıcının istediğinde çalıştırdığı bir eylemdir (H5). Her koşuda taramak, hızlandırmak için eklenen özelliği yavaşlatıcıya çevirirdi |

---

## Belirsizlikler

- [x] Önbellek bütün projeler arasında mı paylaşılsın, proje başına mı ayrılsın?
      → **Cevap: tek paylaşılan önbellek.** Gerekçe ve kabul edilen risk
      "Davranış Kuralları" altında yazılı. Proje başına ayırmak kampanyanın
      kazancının büyük kısmını götürürdü.
- [x] `plans/03` (Durum: Açık) ne olacak? → **Cevap: bu spec onu kapatır.**
      Uygulama tamamlandığında `plans/03`'e hangi yolun seçildiğini, zehirlenme
      itirazına verilen cevabı ve gömme yolunun neden gereksizleştiğini yazan
      bir kapanış bölümü eklenir.
- [x] `plans/03` bir sonraki adım olarak bir ölçüm öneriyordu (koşu sonrası yeni
      inen artefaktların listelenmesi). Bu ölçüm yapılmalı mı?
      → **Cevap: hayır, gereksiz.** O ölçüm **gömme** kararı içindi: neyin
      gömüleceğini belirlemek için. Önbellek yolu sürüm tahmini gerektirmiyor,
      kendi kendine ısınıyor; ölçümün cevaplayacağı soru ortadan kalkıyor.

## Bağımlılıklar

- [spec 003 — agent çalıştırma](../003-agent-calistirma/spec.md): koşu ortamının
  yaşam döngüsü.
- [spec 018 — Maven paket deposu](../018-maven-paket-deposu/spec.md) ve
  [spec 014 — kurumsal paket deposu](../014-kurumsal-paket-deposu/spec.md):
  koşu başına yazılan depo yapılandırması; bu spec onların üstüne biner,
  yerlerini almaz.
- [spec 023 — toplu çalıştırma](../023-toplu-calistirma/spec.md): eşzamanlılık
  davranışının doğrulanacağı yer.
- [spec 016 — ayar denetimi ve arama](../016-ayar-denetimi-ve-arama/spec.md):
  aç/kapat ayarının ve boyut göstergesinin yaşayacağı yer.
