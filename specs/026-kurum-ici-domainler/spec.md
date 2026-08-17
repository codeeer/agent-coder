# Spec: Kurum içi domain'ler

- **Spec no:** 026
- **Tarih:** 2026-08-17
- **Durum:** Taslak
- **İyileştirdiği spec:** [020 — Sandbox çıkış denetimi](../020-sandbox-cikis-denetimi/spec.md)

---

## Problem

Çıkış denetimi açıldığında agent ortamının **tüm** trafiği kurumsal proxy'ye
gidiyor. İstisnası yok. Kurumun kendi adreslerine giden trafik de dahil.

Bu üç ayrı sebeple yanlış:

**Proxy dışarı çıkmak için var.** Kurumsal proxy'ler tipik olarak internete
çıkışı denetlemek üzere kurulur; iç adresleri çözmeyi hiç bilmeyebilir ya da
bilinçli olarak reddeder. Kurumun kendi Bitbucket'ına kendi çıkış proxy'si
üzerinden gitmek, kapıdan çıkıp aynı binaya ön kapıdan girmeye çalışmaktır.

**Agent kurumun kendi deposundan kopabiliyor.** Proxy iç adresi çözemez veya
reddederse klonlama düşer ve çalıştırma hiç başlayamaz. Bu ürün tarafında
ölçülmedi — proxy'nin davranışı kuruma göre değişir ve elimizde kurumsal bir
prova yok. Ama olasılık gerçek ve bedeli ağır: çıkış denetimini açmak,
özelliğin hizmet etmek için yazıldığı kurulumu çalışmaz hale getirebilir.

**Sorun izin değil, yönlendirme.** Kurum içi depo adresi çıkış listesine zaten
kendiliğinden ekleniyor (spec 020 H4: kullanıcı adresi ayarlara girdiyse oraya
eriştiğini biliyordur). Yani adrese çıkmak serbest; yanlış olan tek şey oraya
hangi yoldan gidildiği. Bugün bunu söylemenin ürün içinde bir yolu yok.

Bu spec olmazsa kurumlar ya çıkış denetimini hiç açamaz, ya da açıp kendi iç
servislerine erişimi kaybeder. İkisi de spec 020'yi kullanılmaz kılar.

## Amaç

Yönetici, kurumun kendi domain'lerini bir liste olarak tanımlayabilecek. Bu
adreslere çıkış kapısı **kurumsal proxy'ye uğramadan doğrudan** bağlanacak;
diğer her hedef bugünkü gibi proxy üzerinden gidecek. Liste boşken hiçbir şey
değişmeyecek.

Agent'ın çalıştığı ortamın ağ yalıtımı **aynen korunacak**: karar kapıda
veriliyor, agent'a yeni bir yol açılmıyor.

## Kapsam dışı

- **Ağ yalıtımını gevşetmek.** Agent ortamına kurumun iç ağına doğrudan rota
  verilmez. Spec 020 bu kararı ölçerek aldı: ortam değişkeniyle verilen proxy
  atlanabiliyordu, denetim bu yüzden ağdan geliyor. Yeni liste o kararı
  değiştirmez, yalnızca kapının nereye bağlanacağını belirler.
- **İç adrese giden trafiğin denetimi.** Ürün, izin verilen bir adrese giden
  trafiğin içine bakmaz — spec 020 ile aynı sınır. İç adres için de bakmaz.
- **IP aralığıyla iç ağ tanımı.** Liste yalnızca domain kabul eder; çıkış
  listesi de öyle.
- **Proje bazında liste.** Ayar global'dir.
- **Proxy kimlik doğrulaması ve PAC dosyası.** Spec 020'de zaten kapsam dışı.
- **Çıkış denetimi kapalıyken davranış.** Denetim kapalıyken kapı hiç
  kurulmuyor ve agent ortamı zaten doğrudan çıkıyor; bu ayarın uygulanacağı
  bir yer yok.

---

## Kullanıcı Hikâyeleri

### H1 — Kurum içi adrese proxy'siz gidilir

**Yönetici** olarak, kurumun kendi domain'lerini tanımlayıp o adreslere
**kurumsal proxy'ye uğramadan** gidilmesini istiyorum, çünkü proxy dışarı çıkış
için kurulmuş ve iç adresleri çözemeyebilir.

Kabul kriterleri:

- [ ] Listeyle eşleşen bir hedefe bağlantı, kurumsal proxy'ye hiç uğramadan
      kurulur
- [ ] Listeyle eşleşmeyen bir hedef bugünkü gibi kurumsal proxy üzerinden gider
- [ ] Kurumun birden fazla, birbiriyle akraba olmayan domain'i satır satır
      yazılabilir ve hepsi bağımsız olarak eşleşir
- [ ] Agent'ın çalıştığı ortamın ağ yalıtımı değişmez; ona yeni bir çıkış yolu
      açılmaz

### H2 — İzin ve yönlendirme birbirine karışmaz

**Kurum** olarak, "buraya gidilebilir" ile "buraya nasıl gidilir" kararlarının
**ayrı** kalmasını istiyorum, çünkü izin güvenlik kontrolüdür ve başka bir
ayardan sessizce genişlememelidir.

Kabul kriterleri:

- [ ] Kurum içi listesine bir adres yazmak, o adrese çıkış izni **vermez**
- [ ] Çıkış izni olmayan bir adres, kurum içi listesinde olsa bile reddedilir
- [ ] Karar sırası her zaman aynıdır: önce izin, sonra yönlendirme

### H3 — Liste boşken hiçbir şey değişmez

**Mevcut kullanıcı** olarak, bu özellik eklendiğinde **hiçbir şeyin
değişmemesini** istiyorum.

Kabul kriterleri:

- [ ] Liste boşken her hedef bugünkü gibi kurumsal proxy üzerinden gider
- [ ] Boş liste "hepsi doğrudan gitsin" anlamına **gelmez** — çıkış listesindeki
      boş-liste kuralının tersi geçerlidir
- [ ] Liste okunamaz veya bozuksa da davranış bugünküyle aynı kalır

### H4 — Doğrudan bağlantı başarısızsa proxy denenmez

**Kurum** olarak, iç adrese doğrudan bağlantı kurulamadığında ürünün
**sessizce kurumsal proxy'yi denememesini** istiyorum, çünkü o adres için
"proxy'den geçme" demiştim; geri düşmek tam da kaçınmak istediğim yoldan
kimlik bilgisi geçirmek olur.

Kabul kriterleri:

- [ ] Doğrudan bağlantı başarısız olursa kurumsal proxy denenmez
- [ ] Başarısızlık çalıştırmanın olay akışında görünür ve hangi adrese
      ulaşılamadığı söylenir

### H5 — Geniş desenin bedeli görünür

**Yönetici** olarak, listeyi geniş tutmanın ne anlama geldiğini **ayarı
yazarken** görmek istiyorum, çünkü buradaki risk çıkış listesindekiyle aynı
yönde değil.

Kabul kriterleri:

- [ ] Ayarın açıklaması, listenin geniş tutulmasının trafiği kurumsal
      denetimin dışına çıkardığını açıkça söyler
- [ ] Açıklama, listenin izin vermediğini de söyler

---

## Davranış Kuralları

- **Karar kapıda verilir, agent ortamında değil.** Yalıtım korunur; agent'a
  hangi adresin nasıl gideceği bildirilmez ve onun tarafından değiştirilemez.
- **Önce izin, sonra yönlendirme.** Sıra tersine dönemez.
- **Boş liste hiçbir şeyle eşleşmez.** Bu, çıkış listesindeki "boş liste kısıt
  değil kısıtsızlıktır" kuralının **tersidir** ve bilinçlidir: aynı davranış
  burada kurumsal proxy'yi tamamen devre dışı bırakırdı.
- **Söz dizimi çıkış listesiyle aynıdır.** Aynı ekrandaki iki liste farklı
  kural işletmez.
- **Sessiz geri düşme yoktur.** Ne doğrudandan proxy'ye, ne tersi.
- **Ayar, çalıştırma başlarken dondurulur.** Süren bir iş, başladığı
  kurallarla biter — spec 020 ile aynı ilke.

## Hata Durumları

| Durum | Beklenen davranış |
| ----- | ----------------- |
| Liste boş veya tanımsız | Her hedef kurumsal proxy üzerinden gider |
| Liste bozuk/okunamıyor | Bugünkü davranış sürer; çalıştırma bu yüzden düşmez |
| Hedef listede ama çıkış izni yok | Reddedilir; izin verilmediği söylenir |
| Doğrudan bağlantı kurulamıyor | Çalıştırma hata alır; proxy denenmez, hangi adrese ulaşılamadığı söylenir |
| Çıkış denetimi kapalı | Ayar uygulanmaz; arayüzde etkisiz olduğu söylenir |

---

## Kararlar

Tasarım görüşmesinde bağlanan sorular (2026-08-17):

- **"Proxy uygulanmasın" nasıl gerçekleşecek?** → **Kapı doğrudan bağlanır.**
  Agent ortamının ağına iç ağ rotası vermek elendi: spec 020'nin ölçümle aldığı
  yalıtım kararını bozardı ve kapı tek çıkış olmaktan çıkardı.
- **Liste izin de versin mi?** → **Hayır, yalnızca yönlendirme.** İzin tek
  yerde, çıkış listesinde kalır; güvenlik kontrolü başka bir ayardan sessizce
  genişlemez.
- **Alt alan adları nasıl yazılacak?** → **Çıkış listesiyle aynı kural.**
  Aynı ekranda yan yana duran iki listenin farklı söz dizimi işletmesi
  kullanıcı için tuzak olurdu.

## Belirsizlikler

- [ ] **Doğrudan gidilen hedefler olay akışına yazılsın mı?** Öneri: **hayır** —
      her istekte satır üretip akışı boğar; çıkış durumu ucunda listenin
      görünmesi yeterli. Görüşmede soruldu ama cevaplanmadı; `plan.md`'i
      bu öneriyle yazmak mümkün, ama aksi kararlaştırılırsa H5'e bir kabul
      kriteri eklenmesi gerekir. → **Cevap:**

## Bağımlılıklar

- [Spec 020](../020-sandbox-cikis-denetimi/spec.md) uygulanmış olmalı; bu spec
  onun kurduğu çıkış kapısının üzerine biniyor.
