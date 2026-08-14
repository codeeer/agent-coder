# Spec: Kurumsal ağ sertifikası

- **Spec no:** 017
- **Tarih:** 2026-08-14
- **Durum:** Taslak

---

## Problem

SSL denetimi yapan kurumsal ağlarda giden HTTPS trafiği kurumun kendi
sertifikasıyla yeniden imzalanır. O sertifika tanıtılmazsa agent'ın yaptığı her
HTTPS isteği düşer. Bugün bunun bir çözümü var ama üç yerinden aksıyor.

**1. Sertifika arayüzde yok ve arayüz nerede olduğunu da söylemiyor.**

Ayarlar ekranında "sertifika" araması **"Eşleşme yok"** diyor (ölçüldü). Yönetici
paket deposu adresini, kullanıcı adını ve token'ı arayüzden girip her şeyi doğru
yaptığını sanabiliyor; kurulumun üçüncü ayağı hakkında ekran tamamen sessiz.
Hata ancak ilk çalıştırmada, sebebini söylemeyen bir mesajla çıkıyor.

**2. Bugünkü yöntem, ürünün kendi üretim tavsiyesiyle çelişiyor.**

Sertifika, sunucu üzerindeki bir dosya yolu olarak veriliyor ve o dosya her
çalıştırma ortamına bağlanıyor. Ürün belgesi üretim için "uzak bir çalıştırma
sunucusuna taşıyın" diyor — ama uzak sunucuda dosya bağlama çalışmaz: dosya bir
makinede, bağlama başka makinede çözülür. Yani bugünkü mekanizma, önerilen
üretim kurulumunda **kırık**.

Ayrıca sertifikayı değiştirmek sunucuya erişmeyi ve tüm sistemi yeniden
başlatmayı gerektiriyor.

**3. Sertifika, tanıtıldığı hâlde her aracı kapsamıyor.**

Ölçüm (kurumsal Nexus provasında, çalışma ortamının içinde):

| Araç | Sertifikayı tanıyor mu |
| --- | --- |
| Node / npm | evet |
| git | evet |
| **curl** | **hayır** |
| **Ürünün kendi giden çağrıları** (model sağlayıcı, Jira, kod deposu doğrulaması) | **hayır** |

`curl`, agent'ın en sık kullandığı araçlardan biri. Ürünün kendi çağrılarının
kapsanmaması ise daha ağır: kurumsal ağda model sağlayıcıya hiç ulaşılamaz ve
sorunun sertifikayla ilgili olduğu hiçbir yerde yazmaz.

## Amaç

Kurumsal kök sertifikanın **arayüzden** tanımlanabilmesi, tanımlandığının
ekranda görülebilmesi ve tanımlandığında hem agent'ın kullandığı araçları hem de
ürünün kendi giden çağrılarını kapsaması.

## Kapsam dışı

- **TLS doğrulamasını gevşetmek.** Hiçbir koşulda bir "doğrulamayı kapat" ayarı
  eklenmez. Kurumsal ağın çözümü sertifikayı **tanıtmaktır**.
- **Vekil sunucu (proxy) ayarları.** Ayrı bir konu; bu spec yalnızca güven
  zincirini ele alır.
- **Sertifikanın dağıtımı.** Sertifikayı kurumdan edinmek yöneticinin işi; ürün
  onu üretmez, indirmez, keşfetmez.
- **Java / Maven'ın kapsanması.** Çalıştırma ortamında bugün Java yok; sertifika
  oraya spec 018 ile birlikte tanıtılır.
- **Kimlik doğrulama.** Ürün v1'de tek kullanıcılı ve girişsiz; bu spec onu
  değiştirmez.
- **Paket deposu adresi ve kimliği.** Onlar spec 014'te çözüldü.

---

## Kullanıcı Hikâyeleri

### H1 — Sertifikayı arayüzden tanımlamak

**Yönetici** olarak, kurumsal kök sertifikayı **arayüzden** tanımlamak
istiyorum, çünkü bugün sunucuya erişip dosya yolu vermem ve tüm sistemi yeniden
başlatmam gerekiyor.

Kabul kriterleri:

- [ ] Ayarlar ekranında sertifikanın **tamamının** yapıştırılabileceği bir alan
      var
- [ ] Aynı yerde **dosya seçilebilir**; kurumdan gelen sertifika dosyası
      doğrudan verilebilir
- [ ] Metin editöründe okunamayan (ikili) sertifika dosyaları da kabul edilir —
      kullanıcıdan dönüştürme yapması istenmez
- [ ] Zincir taşıyan tek bir dosya verildiğinde kök ve ara sertifikaların
      **hepsi** alınır
- [ ] Alan boş bırakılabilir; boş olması bugünkü davranışın aynısıdır
- [ ] Kaydetme, diğer ayarlarla aynı açık "Kaydet" akışını kullanır
- [ ] Dosya seçmek tek başına kaydetmez: seçilen dosyanın içeriği alana gelir,
      kullanıcı görür ve **sonra** kaydeder
- [ ] Kaydedilen sertifika sonraki çalıştırmalarda **yeniden başlatmadan**
      geçerli olur
- [ ] Kök sertifikanın yanında ara sertifikalar da varsa hepsi birden
      yapıştırılabilir
- [ ] Sertifika gizli bir değer değildir; kaydedildikten sonra ekranda
      maskelenmeden görüntülenebilir

### H2 — Geçersiz sertifikanın kaydedilmemesi

**Yönetici** olarak, yanlış bir metin yapıştırdığımda bunu **kaydederken**
öğrenmek istiyorum, çünkü bugün hata ancak bir çalıştırma yarıda düştüğünde
ortaya çıkıyor.

Kabul kriterleri:

- [ ] Sertifika olmayan bir metin reddedilir ve neden reddedildiği yazılır
- [ ] Reddedilen değer kaydedilmez; önceki geçerli değer korunur
- [ ] Kaydedilen sertifikanın **kime ait olduğu, kimin imzaladığı ve ne zaman
      geçerliliğini yitireceği** ekranda görünür
- [ ] Geçerliliği dolmuş bir sertifika bu durumu açıkça belirtir
- [ ] Bu bilgilerin hepsi sertifikanın kendisinden okunur; hiçbiri tahmin
      edilmez

### H3 — Sertifikanın durumunun görünür olması

**Yönetici** olarak, sertifikanın tanımlı olup olmadığını **aramayla
bulabilmek** istiyorum, çünkü bugün ekranda hiçbir izi yok.

Kabul kriterleri:

- [ ] Ayarlarda "sertifika" araması sonuç döndürür
- [ ] Sertifika sunucu tarafında dosya yoluyla tanımlanmışsa, arayüz bunu
      **tanımlı** olarak gösterir ve nereden geldiğini söyler
- [ ] Hiçbir yerde tanımlı değilse bu da açıkça görünür

### H4 — Agent'ın tüm araçlarının kapsanması

**Geliştirici** olarak, sertifika tanımlıyken agent'ın çalıştırdığı **her**
aracın kurumsal adreslere ulaşabilmesini istiyorum, çünkü bugün bazıları
çalışıyor bazıları sessizce düşüyor.

Kabul kriterleri:

- [ ] Sertifika tanımlıyken çalışma ortamında Node/npm kurumsal adrese
      ulaşabilir
- [ ] Aynı koşulda git kurumsal adrese ulaşabilir
- [ ] Aynı koşulda **`curl`** kurumsal adrese ulaşabilir
- [ ] Sertifika tanımlı değilken bu araçların davranışı bugünküyle aynıdır
- [ ] Sertifika güvenilen kök listesine **eklenir**; genel sertifikalar geçerli
      kalır

### H5 — Ürünün kendi çağrılarının kapsanması

**Yönetici** olarak, sertifika tanımlıyken ürünün **kendi** giden çağrılarının
da çalışmasını istiyorum, çünkü kurumsal ağda bugün model sağlayıcıya hiç
ulaşılamıyor.

Kabul kriterleri:

- [ ] Sertifika tanımlıyken model sağlayıcı doğrulaması kurumsal ağda başarılı
      olur
- [ ] Aynı durum Jira ve kod deposu erişimi doğrulaması için de geçerlidir
- [ ] Sertifika tanımlı değilken davranış bugünküyle aynıdır
- [ ] Sertifika değiştirildiğinde bu çağrılar **yeniden başlatmadan** yeni
      sertifikayı kullanır

---

## Davranış Kuralları

- **Biçim kullanıcının derdi değildir.** Sertifika dosyaları farklı biçimlerde
  dağıtılır ve kurumdan gelenin hangisi olduğu çoğu zaman dosya adından
  anlaşılmaz. Ürün ne verilirse onu kabul eder, içeride **tek bir biçime**
  çevirir ve o biçimde saklar. Kullanıcıdan dönüştürme komutu çalıştırması
  istenmez.
- **Arayüz kazanır, sunucu ayarı yedekte kalır.** Sertifika arayüzden
  tanımlanmışsa o kullanılır; tanımlanmamışsa sunucudaki mevcut dosya yolu
  ayarına düşülür. Böylece bugünkü kurulumlar güncellemede bozulmaz ve yeni
  kurulumlar hiç sunucuya inmez.
- **Hangi kaynaktan geldiği kullanıcıya söylenir.** İki kaynak da mümkünken
  "tanımlı" demek yetmez; hangisinin geçerli olduğu yazılır.
- **Sertifika gizli değildir.** Kök sertifika tanımı gereği herkese dağıtılır;
  şifreli sır deposuna konmaz, ekranda maskelenmez.
- **Ölçülmeyen gösterilmez.** Sertifikaya dair ekranda görünen her bilgi
  (sahibi, imzalayanı, bitiş tarihi) sertifikanın kendisinden okunur.
- **Değiştirmek yeniden başlatma gerektirmez.**
- **İki tema ayrı ayrı değerlendirilir.**

## Hata Durumları

| Durum | Beklenen davranış |
| --- | --- |
| Sertifika olmayan bir metin yapıştırıldı | Kaydedilmez; neden geçersiz olduğu yazılır, önceki değer korunur |
| Sertifika olmayan bir dosya seçildi | Alan doldurulmaz; dosyanın sertifika içermediği söylenir, mevcut içerik bozulmaz |
| Seçilen dosya sertifikanın yanında özel anahtar da taşıyor | Yalnızca sertifika kısmı alınır; özel anahtar hiçbir yere yazılmaz ve ekranda gösterilmez |
| Seçilen dosya makul bir sertifika boyutundan çok büyük | Reddedilir; boyut sınırı söylenir |
| Sertifika geçerli ama süresi dolmuş | Kaydedilir (kurum onu hâlâ kullanıyor olabilir) ama süresinin dolduğu açıkça belirtilir |
| Sertifika kaydedildi ama çalıştırma yine sertifika hatasıyla düştü | Hata mesajı, sertifikanın tanımlı olduğunu ve sorunun başka yerde olabileceğini söyler; kullanıcı aynı ayarı tekrar tekrar denemez |
| Hem arayüzde hem sunucu ayarında sertifika var | Arayüzdeki kullanılır; ekranda hangisinin geçerli olduğu yazılır |
| Sertifika arayüzden silindi, sunucu ayarı duruyor | Sunucu ayarına düşülür ve bu ekranda görünür |
| Ürünün kendi çağrısı sertifika hatası aldı | Hata, sertifika kaynaklı olduğunu söyler; genel bir "bağlantı hatası" ile geçiştirilmez |

---

## Belirsizlikler

- [x] Sertifika arayüzden mi tanımlansın, sunucu dosyasında mı kalsın? →
      **Cevap:** Arayüzden. Ölçüldü: çalışma ortamına dosya yazma yolu üründe
      zaten var ve kullanılıyor, dosya bağlamaya gerek yok. Bağlama ayrıca uzak
      çalıştırma sunucusunda hiç çalışmıyor — yani bugünkü yöntem, ürünün kendi
      üretim tavsiyesiyle çelişiyor.
- [x] Sunucudaki mevcut ayar kaldırılsın mı? → **Cevap:** Hayır, yedek olarak
      kalsın. Kaldırılsaydı güncelleyen kurulumların sertifikası sessizce devre
      dışı kalır ve bunu ancak ilk başarısız çalıştırmada fark ederlerdi.
- [x] Sertifika sır mıdır? → **Cevap:** Hayır. Kök sertifika herkese dağıtılan
      genel bir belgedir; şifreli sır deposuna konması onu bulunması zor ve
      görüntülenemez yapardı — hiçbir karşılığı olmadan.
- [x] Sertifika hangi biçimlerde girilebilsin? → **Cevap:** Kullanıcının eline ne
      geçtiyse o. Sertifika dosyaları hem metin hem ikili biçimlerde dağıtılıyor
      ve zincir taşıyan kapsayıcı biçimler de yaygın; kurumsal ekipler bunları
      çoğu zaman aynı dosya uzantısıyla veriyor, yani kullanıcı elindekinin
      hangisi olduğunu bilmiyor. "Şu biçime çevirin" demek, ürünün kolayca
      yapabildiği bir işi kullanıcıya yıkmak olurdu. İçeride tek bir biçime
      normalleştirilir.
- [x] Arayüzden sertifika yüklenmesi güvenlik zaafı mı? → **Cevap:** Hayır.
      Aynı arayüz bugün zaten model sağlayıcı anahtarlarını ve kod deposu
      erişimlerini saklıyor ve çalışma ortamında komut çalıştıran agent'lar
      tanımlıyor. Ürün ayrıca girişsiz ve internete açılmaması gerektiği yazılı.
      Sertifika eklemek bu yetkilerin yanında yeni bir kapı açmıyor.

## Bağımlılıklar

Yok.

Spec 018 (Java/Maven) bu spec'in tanımladığı sertifikayı Java tarafına da
tanıtacak; o iş bu spec tamamlanmadan yapılamaz.
