# Spec: Güvenlik sertleştirme

- **Spec no:** 024
- **Tarih:** 2026-08-16
- **Güncellendi:** 2026-08-17 — belirsizlikler karara bağlandı
- **Durum:** Taslak
- **Kaynak:** [Güvenlik raporu analizi](../../security-reports/analiz.md) —
  bağımsız kaynak kod değerlendirmesi, 10 bulgu

---

## Problem

Ürün bugün **güvenilmeyen bir ağda dağıtılamaz.** Bu bir tahmin değil: bağımsız
bir kaynak kod değerlendirmesi 10 bulgu çıkardı ve bulguların tamamı, taramadan
sonraki 103 commit'e rağmen hâlâ açık.

Bulguların ortak teması, **tek kullanıcılı ve özel ağ varsayımının kod
tarafından hiçbir yerde zorunlu kılınmaması.** Ürün bugüne kadar bu varsayımla
yazıldı ve varsayım belgelendi. Ama hedef dağıtım artık kurumsal bir IP
bloğunda, yük dengeleyici arkasında yayınlanan çok kullanıcılı bir kurulum.
Varsayım geçerliliğini yitirdi; onu taşıyan kod ise yerinde duruyor.

En ağır zincir şu: kaydedilen bir git erişim bilgisi, çalıştırılabilir bir
betiğin içine olduğu gibi yerleştiriliyor. İçinde doğru karakter dizisi geçen
bir erişim bilgisi, doğrulama sırasında **komut çalıştırabiliyor.** Sunucu
ayrıca çalıştırma ortamlarını başlatabilmek için makinenin tamamına yetkili
olduğundan, etki sunucunun ele geçirilmesine kadar uzanıyor. Ve bu işlemi
başlatmak için kimlik doğrulaması gerekmiyor.

İkinci tema, gizli değerlerin kaydedildikleri sınırın dışına taşınabilmesi:
kaydedilmiş bir erişim bilgisi kullanıcı tarafından başka bir adrese
yönlendirilebiliyor, çalıştırma tetikleme anahtarları loglara ham olarak
düşüyor, dış adres tanımları iç ağa yönlendirilebiliyor.

Üçüncüsü ve en sessiz olanı: **ürün kimin ne yaptığını bilmiyor.** Kullanıcı
kavramı olmadığı için "bu depoya kim push etti", "bu erişim bilgisini kim
değiştirdi" sorularının cevabı yok. Çok kullanıcılı bir kurumsal kurulumda
bu, teknik bir eksik değil kurulum engelidir.

Bu spec olmazsa ürün, kurumsal kuruluma teknik olarak hazır olmadan
dağıtılmış olur.

## Amaç

Ürün, kendisine ulaşabilen herkesin yönetici olmadığı bir dünyaya göre
davranacak. Yönetim işlemleri kimlik doğrulaması isteyecek ve kimin yaptığı
kayda geçecek. Kaydedilen hiçbir gizli değer komut çalıştıramayacak,
kaydedildiği adresten başkasına gitmeyecek ve loglara düşmeyecek. Varsayılan
kurulum tahmin edilebilir bir parolayla açılmayacak, çalıştırmalar birbirine
bağlanamayacak, dış adres tanımları iç ağa yönlendirilemeyecek.

## Kapsam dışı

- **Rol ve yetki yönetimi.** Bu spec kimlik getirir: kim olduğun bilinir ve
  yaptığın kayda geçer. Ama "kim neyi yapabilir" ayrımı **gelmez** — kimliği
  doğrulanmış herkes aynı yetkiye sahiptir. Yetki ayrımı ayrı bir iştir.
- **Kurumsal kimlik sağlayıcı (SSO/OIDC) entegrasyonu.** Hedef budur ama bu
  spec'in konusu değildir. Bu spec'in getirdiği doğrulama katmanı, ilerde
  anahtarın nasıl verildiğini değiştirmeye izin verecek şekilde tasarlanır.
- **Agent'ın çalıştığı ortamdaki gizli değerler.** Runner ile agent aynı
  ortamda çalıştığı için, oraya giren her gizli değer agent tarafından
  okunabilir; kabuğu kapatmak bunu çözmez. Tek gerçek çözüm gizli değerin o
  ortama **hiç girmemesidir** ve bu, çalıştırma mimarisinin değişmesi demektir.
  **Ayrı bir spec'e taşındı** (17 Ağustos 2026 kararı).
- **Ağ ve yük dengeleyici yapılandırması.** Sunucu portunun yalnızca yük
  dengeleyiciye açılması iyi bir uygulamadır ve önerilir; ama artık **güvenlik
  sınırı değildir.** Bu spec'ten sonra doğrudan porta bağlanan bir istemci de
  kimlik doğrulamasından geçmek zorundadır. Uygulamanın güvenliği başka bir
  ekibin proxy yapılandırmasına bağlı olmaz.
- **Çalıştırma ortamı yetkisinin mimari olarak daraltılması.** Sunucunun
  makine üzerindeki geniş yetkisi, en ağır bulgunun etkisini büyüten şeydir;
  ama onu kaldırmak çalıştırma mimarisinin yeniden kurulmasıdır ve `plans/`
  altına aittir. Bu spec zinciri **bir önceki halkadan** keser: gizli değer
  zaten komut çalıştıramazsa yetkinin büyüklüğü devreye girmez.
- **Çok örnekli çalışma.** Ürün tek sunucuda çalışır; 16 Ağustos 2026'da
  bilinçli olarak kayıt altına alındı.
- **Bilinen zafiyet (CVE) taraması.** Değerlendirme bağımlılık sürümlerini
  yalnızca envantere aldı. Ayrı bir iştir.

---

## Kullanıcı Hikâyeleri

Sıra önem sırasıdır. H1 ve H2 diğerlerinden önce gelir: biri erişimi,
diğeri en ağır zinciri kapatır.

### H1 — Yönetim işlemleri kimlik ister ve kim yaptığı bilinir

**Kurum** olarak, ürüne ulaşan herkesin yönetici olmamasını ve **yapılan her
ayrıcalıklı işlemin bir kişiye bağlanmasını** istiyorum, çünkü çok kullanıcılı
bir kurulumda "bunu kim yaptı" sorusunun cevapsız kalması kabul edilemez.

Kabul kriterleri:

- [ ] Geçerli bir kişisel erişim anahtarı taşımayan istek hiçbir yönetim
      işlemini çalıştıramaz; açık bir reddedilme yanıtı alır
- [ ] Kural, ürüne nereden ulaşıldığından bağımsız işler; ağ konumu bir
      ayrıcalık sağlamaz
- [ ] Yönetici bir kişi için erişim anahtarı üretebilir ve **tek tek** iptal
      edebilir; iptal edilen anahtarla yapılan istek anında reddedilir
- [ ] Çalıştırma başlatma, push, erişim bilgisi değiştirme ve silme
      işlemlerinde hangi anahtarla yapıldığı sonradan görülebilir
- [ ] Erişim anahtarının kendisi hiçbir logda görünmez
- [ ] Üretim modunda kimlik doğrulaması devre dışı bırakılamaz
- [ ] Sağlık ve hazırlık kontrolleri kimlik istemeye devam etmez — yük
      dengeleyici bunları kullanabilir
- [ ] **Bağımlılık önbelleğini temizleme ve doğrulama, ayar yazma sınıfındadır**
      ([spec 027](../027-bagimlilik-onbellegi/spec.md)): kimlik geldiğinde bu
      iki uç da ayar yazmayla aynı kapıdan geçmeli. Temizleme geri alınamaz ve
      bütün projelerin koşularını yavaşlatır; bugün kimliksiz olması, ayar
      yazmanın da kimliksiz olmasından — ayrı bir karar değil, aynı boşluk

### H2 — Kaydedilen erişim bilgisi komut çalıştıramaz

**Yönetici** olarak, bir git erişim bilgisi kaydettiğimde **o değerin yalnızca
parola olarak kullanılmasını** istiyorum, çünkü içine gömülmüş bir yönergenin
sunucuda çalışması ürünün en ağır riskidir.

Kabul kriterleri:

- [ ] İçinde satır sonu, betik sınırlayıcısı veya kabuk metakarakterleri geçen
      bir erişim bilgisi kullanıldığında, değer **aynen** parola olarak iletilir
      ve hiçbir komut çalışmaz
- [ ] Aynı değer bir depo doğrulamasında, bir toplu içe aktarmada ve bir push
      işleminde kullanıldığında da aynı şekilde davranır
- [ ] Satır sonu veya boş bayt içeren bir erişim bilgisi, kaydedilmeden önce
      açık bir hata ile reddedilir (ikincil savunma katmanı; birincil koruma
      değerin çalıştırılabilir metne hiç girmemesidir)

### H3 — Erişim bilgisi kaydedildiği adresten başkasına gitmez

**Yönetici** olarak, kayıtlı bir erişim bilgisinin **yalnızca tanımlandığı
adrese** gönderilmesini istiyorum, çünkü adresi değiştirip mevcut anahtarı
kendi sunucusuna yönlendirebilen biri o anahtarı çalar.

Kabul kriterleri:

- [ ] Bir model sağlayıcısının veya MCP sunucusunun adresi değiştirilip gizli
      değer alanı boş bırakıldığında, eski gizli değer yeni adrese
      **gönderilmez**; kullanıcıdan yeniden girmesi istenir
- [ ] Adresin şeması, sunucu adı veya etkin portu değiştiğinde bu kural işler;
      yalnızca yol değiştiğinde işlemez
- [ ] Kayıtlı bir git erişim bilgisi, tanımlandığı adresten farklı bir depo
      adresi için seçilmek istendiğinde işlem reddedilir
- [ ] Yönlendirme (redirect) sonrasında da hedefin aynı kökene ait olduğu
      yeniden doğrulanır

### H4 — Çalıştırmalar birbirine bağlanamaz

**Kurum** olarak, aynı anda çalışan iki işin **birbirini görememesini**
istiyorum, çünkü biri ele geçirilirse diğerinin deposu ve anahtarları da
gitmiş olur.

Kabul kriterleri:

- [ ] Bir çalıştırma ortamı, başka bir çalıştırma ortamına ağ üzerinden
      bağlanamaz veya adını çözemez
- [ ] Bir çalıştırmanın denetim arayüzü, doğru kimlik bilgisi olmadan yapılan
      isteği reddeder
- [ ] Bu kimlik bilgisi her çalıştırma için ayrıdır
- [ ] Kurulumda üretilen kimlik bilgisi gerçekten kullanılır; üretilip hiçbir
      yere iletilmeyen bir ayar kalmaz

### H5 — Varsayılan kurulum tahmin edilebilir parolayla açılmaz

**Kurulumu yapan kişi** olarak, standart kurulum adımlarını izlediğimde
**benzersiz bir veritabanı parolası** üretilmesini istiyorum, çünkü depoda
yazılı olan bir parola parola değildir.

Kabul kriterleri:

- [ ] Kurulum hazırlığı her çalıştırıldığında benzersiz bir veritabanı parolası
      üretir ve bağlantı bilgisini onunla tutarlı şekilde günceller
- [ ] Depoda yazılı olan bilinen varsayılan parolalarla üretim modunda sistem
      başlatılamaz; açık bir hata verir
- [ ] Üretim kurulumunda veritabanı dışarıya port yayınlamaz

### H6 — Dış adres tanımları iç ağa çıkamaz

**Kurum** olarak, ürüne girilen dış servis adreslerinin **iç ağa
yönlendirilememesini** istiyorum, çünkü ürün iç ağın içinde çalışıyor ve
oradan ulaşılabilecek şeyler dışarıdan ulaşılamayacak şeylerdir.

Kabul kriterleri:

- [ ] Yerel adresler, özel ağ aralıkları, bağlantı-yerel adresler ve bulut
      üstveri adresleri hedef olarak reddedilir; hem IPv4 hem IPv6 için
- [ ] Kural, model sağlayıcı, MCP sunucusu, depo ve görev takip entegrasyonu
      tanımlarının hepsinde aynı şekilde işler
- [ ] İzinli bir adresten yasaklı bir adrese yönlendirme reddedilir
- [ ] Adres çözümlemesi istek anında değiştiğinde de kural işler

### H7 — Tetikleme anahtarları loglara düşmez

**Sistemi işleten kişi** olarak, çalıştırma tetikleme anahtarlarının
**loglarda görünmemesini** istiyorum, çünkü bu anahtarlar kimlik doğrulaması
olmadan çalıştırma başlatabiliyor ve logları geniş bir kitle okuyor.

Kabul kriterleri:

- [ ] Bir tetikleme veya MCP anahtarı kullanıldığında, gerçek anahtar değeri
      uygulama loglarında hiçbir biçimde görünmez
- [ ] Log kaydı hangi ucun çağrıldığını yine gösterir; ayırt edilebilirlik
      kaybolmaz

> Daha önce loglanmış anahtarların döndürülmesi bir **işletme adımıdır**;
> ürün bu yeteneği zaten sunuyor.

### H8 — Credential taşıyan adresler şifresiz olamaz

**Kurum** olarak, gizli değer taşıyan bağlantıların **şifreli olmasını**
istiyorum, çünkü aradaki bir gözlemci anahtarı okuyabilir.

Kabul kriterleri:

- [ ] Üretim modunda, gizli değer taşıyan bir entegrasyon adresi şifresiz
      protokolle tanımlanamaz
- [ ] Geliştirme modunda şifresiz adrese izin verilir, ama bu açık bir tercih
      olarak yapılır ve yalnızca yerel hedefleri kapsar

### H9 — Çıkış denetiminin kapalı olduğu görünür

**Kurulumu yapan kişi** olarak, agent'ın çalıştığı ortamın internete serbestçe
çıkabildiği durumu **fark ederek** kabul etmek istiyorum, çünkü bugün bu durum
hiçbir yerde görünmüyor ve sessizce varsayılan.

Kabul kriterleri:

- [ ] Üretim modunda sistem, çıkış denetimi ya yapılandırılmış ya da kapalı
      olduğu açıkça onaylanmış olmadan başlamaz
- [ ] Çıkış denetiminin kapalı olduğu, arayüzde durum olarak görünür
- [ ] Mevcut kurulumların davranışı, onay verildikten sonra değişmez

---

## Davranış Kuralları

- **İki mod vardır: üretim ve geliştirme. Varsayılan üretimdir.** Geliştirme
  modu açıkça seçilir. Mod okunamıyorsa üretim varsayılır — ayarlanmayı
  unutmak güvenli tarafta bırakır.
- **Bu spec ile eklenen her yeni kontrol fail-closed'dır.** Mevcut bir
  kontrolün varsayılanı, kurulumları bozmamak adına korunabilir; ama kapalı
  olduğu kullanıcıya görünür kılınır (bkz. H9).
- **Üretilen her güvenlik ayarı bağlanmış olmalıdır.** Kurulumda üretilen ama
  hiçbir yere iletilmeyen bir değer, güvenlik izlenimi veren ölü koddur.
- **Gizli değer veri kanalında taşınır**, hiçbir koşulda çalıştırılabilir
  metnin içine yerleştirilmez.
- **Genel bir istek gövdesi sınırı vardır**; sınırı aşan gövde biriktirilmeden
  reddedilir. Bu bir sertleştirme maddesidir, ayrı hikâye değildir.
- **Geriye dönük uyumluluk güvenliğin önüne geçmez.** Mevcut kurulumları
  bozacak bir sıkılaştırma gerekiyorsa, sessizce gevşetmek yerine açık bir
  geçiş adımı tanımlanır.

## Hata Durumları

| Durum | Beklenen davranış |
| ----- | ----------------- |
| İstekte erişim anahtarı yok veya geçersiz | İşlem yapılmaz; kimlik gerektiği söylenir |
| Erişim anahtarı iptal edilmiş | İstek reddedilir; anahtarın geçersiz olduğu söylenir |
| Erişim bilgisinde satır sonu veya boş bayt var | Kayıt reddedilir; hangi karakterin sorun olduğu söylenir |
| Adres değişti, gizli değer alanı boş | Doğrulama yapılmaz; gizli değerin yeniden girilmesi istenir |
| Git erişim bilgisi farklı bir depo kökeni için seçildi | İşlem reddedilir; kökenlerin farklı olduğu söylenir |
| Entegrasyon adresi iç ağa çözümleniyor | Tanım reddedilir; dış bir hedef gerektiği söylenir |
| Yönlendirme yasaklı bir adrese gidiyor | İstek tamamlanmaz; yönlendirmenin reddedildiği söylenir |
| Çalıştırma denetim isteğinde kimlik bilgisi yok veya yanlış | İstek reddedilir |
| Bilinen varsayılan veritabanı parolası, üretim modunda | Sistem başlatılmaz; parolanın değiştirilmesi istenir |
| İstek gövdesi sınırı aşıyor | Gövde okunmadan reddedilir |
| Üretimde şifresiz entegrasyon adresi tanımlandı | Tanım reddedilir |
| Üretimde çıkış denetimi ne yapılandırılmış ne onaylanmış | Sistem başlatılmaz; seçim yapılması istenir |

---

## Kararlar

Taslakta açık bırakılan sorular 17 Ağustos 2026'da karara bağlandı.

- **Kimlik doğrulama nasıl sahiplenilecek?** → **Kişi başına erişim anahtarı,
  uygulamanın kendi yönettiği.** Yük dengeleyiciye gizli başlık enjekte
  ettirme fikri **reddedildi**: uygulamanın güvenliğini başka bir ekibin proxy
  yapılandırmasına bağlıyordu. Uygulama kendi kapısını kendi tutar. Bu karar
  atıfı da kapsama aldı (H1).
- **Agent'ın kabuk erişimi kapatılacak mı?** → **Hayır.** Ürün bir kodlama
  agent'ı; kabuğu kapatmak güvenlik özelliği için ürünü takas etmek olurdu.
  Ayrıca sorunu çözmezdi: agent'ın dosya okuma araçları kalır. Hikâye ayrı
  spec'e taşındı.
- **Üretim modu eklenecek mi?** → **Evet, varsayılan üretim.** H5, H8 ve
  H9'un ön koşulu. Varsayılanın geliştirme olması, üretimde ayarlanmayı
  unutunca tüm kontrollerin sessizce kapanması demekti.
- **Çıkış denetiminin varsayılanı değişecek mi?** → **Hayır, ama görünür
  olacak.** Küratörlü bir allowlist olmadan varsayılanı açmak her mevcut
  kurulumu bozar. Kapalı olduğunun bilinçli kabul edilmesi zorunlu (H9).
- **Gövde sınırı hikâye olarak kalacak mı?** → **Hayır**, davranış kuralına
  indirildi. Kaynak tükenmesi sınıfı ve alan seviyesi sınırlar kısmen zaten
  var.
- **Loglanmış anahtarların döndürülmesi spec'e girecek mi?** → **Hayır**,
  işletme adımı. Ürün rotate yeteneğini zaten sunuyor.

## Belirsizlikler

- Yok. Tüm sorular karara bağlandı; `plan.md`'e geçilebilir.

## Bağımlılıklar

- Yok. Bu spec mevcut ürünün üzerine uygulanır.
- H6'nın ikinci savunma katmanı [spec 020](../020-sandbox-cikis-denetimi/spec.md)
  ile kurulan çıkış denetimine dayanır; o altyapı mevcuttur ve H9 onun
  görünürlüğünü ele alır.
- Agent'ın çalıştığı ortamdaki gizli değerler için **yazılacak ayrı spec** bu
  spec'ten bağımsızdır; ikisi paralel ilerleyebilir.
