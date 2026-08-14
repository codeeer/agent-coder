# Spec: Sandbox çıkış denetimi

- **Spec no:** 020
- **Tarih:** 2026-08-14
- **Durum:** Taslak

---

## Problem

Agent'ın çalıştığı sandbox bugün **internetteki her adrese çıkabiliyor** ve bunu
kimse sınırlayamıyor.

Bu bir tahmin değil, ölçüm: [veri sızıntısı analizi](../../docs/veri-sizintisi-analizi.md)
sırasında agent, sandbox'ta zaten kurulu olan JDK ve Maven yerine internetten
kendi kopyalarını indirip çalıştırdı, ayrıca alakasız bir siteye bağlandı. Görev
bunların hiçbirini istememişti. Aynı analiz şu sonuca varıyor: kaynak kodu dışarı
taşımak isteyen bir talimat — kötü niyetli bir görev metni, repository'ye gömülmüş
bir yönerge, ele geçirilmiş bir bağımlılık — bunu **yapabilir**; hiçbir şey
engellemiyor.

Kurumsal kurulumlarda ikinci bir sorun var: çıkışın kurumun proxy'sinden geçmesi
gerekiyor. Bugün bunu söylemenin ürün içinde bir yolu yok. Sunucu ayarıyla denenen
yol ise ölçümde **yetersiz çıktı**: trafiğin bir kısmı proxy'yi yok sayıp doğrudan
dışarı çıktı. Yani "proxy tanımladık" demek, çıkışın oradan geçtiği anlamına
gelmiyor.

Bu özellik olmazsa ürün, çıkışı denetlenen hiçbir kuruma güvenle kurulamaz ve
sandbox'ın serbest çıkışı açık bir risk olarak kalır.

## Amaç

Yönetici, ayarlar ekranından bir proxy adresi ve bir whitelist tanımlayabilecek.
Tanımlandığında agent'ın çalıştığı ortam internete **yalnızca** o proxy üzerinden
ve **yalnızca** whitelist'teki domain'lere çıkabilecek; whitelist'te olmayan bir
adrese ulaşma girişimi engellenecek ve çalıştırmada görünecek. Proxy tanımlanmazsa
bugünkü davranış aynen sürecek.

## Kapsam dışı

- **İçerik denetimi.** Ürün, izin verilen bir adrese giden trafiğin içine bakmaz;
  yalnızca hangi adrese çıkıldığına karar verir. TLS açılmaz.
- **İzin verilen bir domain üzerinden sızdırma.** Bir domain'e izin verildiyse, o
  domain'in sunduğu her imkân da açılmış olur (örneğin `github.com` açıkken gist).
  Whitelist bunu kapatmaz; bu sınır kullanıcıya açıkça söylenir.
- **Ürünün kendi çıkışı.** Bu spec agent'ın çalıştığı ortamı kapsar; backend'in
  model kataloğu çekmesi gibi kendi çağrıları kapsam dışıdır.
- **Kimlik doğrulaması isteyen proxy.** İlk sürümde proxy adresi kimlik bilgisi
  içermez; kullanıcı adı/parola isteyen proxy'ler desteklenmez.
- **IP'ye göre izin.** Whitelist yalnızca domain kabul eder.

### Sonraya bırakılan: geniş açılan domain'ler

opencode her çalıştırmada GitHub'dan bir yardımcı program indiriyor (ölçüm:
[veri sızıntısı analizi](../../docs/veri-sizintisi-analizi.md) bulgu 4). Bu yüzden
`github.com` her zaman izinli olmak zorunda. Kapı TLS'i açmadığı için yalnızca
domain'i görüyor, yolu göremiyor — dolayısıyla **`github.com`'u bu tek indirme için
açmak, GitHub'ın tamamını açıyor.**

Bu sürümde böyle kabul edildi. Daha iyi çözüm, indirilen programı runner imajına
gömmek ve `github.com`'u otomatik izinliler listesinden çıkarmaktır; ama opencode'un
gömülü kopyayı gerçekten kullandığı **ölçülmeden** söz verilemez. Aynı soru
opencode'un çalışma anında dışarıya bağımlı olduğu diğer adresler için de geçerli.
Ayrı bir iş olarak değerlendirilecek.

---

## Kullanıcı Hikâyeleri

### H1 — Çıkışı proxy'ye mecbur etmek

**Yönetici** olarak, agent ortamının internete yalnızca kurumun proxy'sinden
çıkmasını istiyorum, çünkü kurum politikası denetlenmemiş çıkışa izin vermiyor ve
"ayarı girdim" demek yetmiyor — ölçüm, ayarın atlanabildiğini gösterdi.

Kabul kriterleri:

- [ ] Proxy adresi tanımlıyken başlatılan bir çalıştırmada, agent ortamının kurduğu
      dış bağlantıların **tamamı** proxy üzerinden geçer; doğrudan kurulan bağlantı
      sayısı sıfırdır
- [ ] Bu, proxy'yi yok sayan bir araç için de geçerlidir: böyle bir araç dışarı
      çıkamaz, bağlantısı başarısız olur
- [ ] Proxy adresi tanımlı değilken hiçbir şey değişmez; çalıştırmalar bugünkü gibi
      çalışır
- [ ] Doğrudan bağlantı sayısının sıfır olduğu ölçülür ve ölçüm belgeye yazılır

### H2 — Çıkılabilecek domain'leri sınırlamak

**Yönetici** olarak, agent ortamının hangi domain'lere çıkabileceğini bir
whitelist ile sınırlamak istiyorum, çünkü serbest çıkış kaynak kodun dışarı
taşınmasına açık kapı bırakıyor.

Kabul kriterleri:

- [ ] Ayarlar ekranında whitelist girilebilir; her satır bir domain
- [ ] Tam domain (`ornek.com`) yalnızca o adresi açar
- [ ] Subdomain'ler için bir yazım biçimi vardır ve yardım metninde gösterilir
- [ ] Whitelist'te olmayan bir domain'e çıkış girişimi engellenir
- [ ] Whitelist boş bırakılırsa domain kısıtı uygulanmaz; proxy zorunluluğu sürer
- [ ] Whitelist yalnızca proxy adresi tanımlıyken anlam taşır; proxy boşken hiçbir
      etkisi yoktur ve bu durum arayüzde söylenir
- [ ] Geçersiz bir satır (domain yerine URL, port, boşluk içeren metin) kaydedilmez
      ve hata mesajı ne yazılması gerektiğini söyler

### H3 — Engellenen çıkışı görmek

**Geliştirici** olarak, bir çalıştırma sırasında engellenen çıkış olduysa bunu
çalıştırmanın kendi ekranında görmek istiyorum, çünkü aksi halde çalıştırmanın
neden yarım kaldığını anlayamam ve whitelist'e ne ekleyeceğimi bilemem.

Kabul kriterleri:

- [ ] Engellenen her domain, çalıştırmanın olay akışında uyarı olarak görünür ve
      hangi domain'in engellendiğini yazar
- [ ] Aynı domain tekrar tekrar denendiğinde olay akışı okunmaz hale gelmez
- [ ] Engellenen bir çıkış, backend'i hataya düşürmez; yalnızca o istek başarısız
      olur
- [ ] Uyarı metni, domain'in whitelist'e eklenmesi gerektiğini söyler

### H4 — Her zaman izinli adresleri görmek

**Yönetici** olarak, ürünün çalışabilmesi için hangi domain'lerin zaten açık
olduğunu görmek istiyorum, çünkü whitelist'e ben yazmadığım halde açık olan bir
adres varsa bunu bilmem gerekir.

Kabul kriterleri:

- [ ] Whitelist alanının yanında "her zaman izinli" domain'ler ayrıca görünür
- [ ] Bu domain'ler gerçek yapılandırmadan gelir: LLM provider'ın adresi, git
      repository adresi, tanımlıysa npm/Maven registry adresi
- [ ] opencode'un kendi çalışması için gereken domain'ler de bu listede görünür
- [ ] Kullanıcı bu domain'leri whitelist'e ayrıca yazmak zorunda kalmaz; boş
      whitelist bırakmak ürünü çalışmaz hale getirmez

---

## Davranış Kuralları

- **Proxy ana anahtardır.** Proxy adresi boşsa bu spec'teki hiçbir davranış devreye
  girmez: ne domain kısıtı, ne yönlendirme. Ürün bugünkü gibi çalışır.
- **Proxy doluysa çıkış mecburdur, tercih değil.** Yönlendirme bir öneri olarak
  değil, başka yol bırakmayarak sağlanır. Sebebi ölçüm: yalnızca ayarla yapılan
  yönlendirme atlandı.
- **Boş whitelist kısıt değil, kısıtsızlıktır.** Proxy dolu ama whitelist boşsa tüm
  domain'ler açıktır; kısıtlama whitelist'e yazılan değerlerden doğar. Boş bir
  whitelist ürünü kilitlemez.
- **Her zaman izinli adresler gizlenmez.** LLM provider, git repository ve registry
  adresleri kullanıcı yazmasa da açıktır — kullanıcı bu adresleri ayarlara zaten
  kendisi girmiş, yani eriştiğini biliyor. Ama arayüzde görünürler; kullanıcının
  bilmediği açık bir kapı bırakılmaz.
- **TLS açılmaz.** Karar yalnızca hedef domain'e bakılarak verilir. Ürün, LLM
  provider'a giden isteğin içini görebilecek bir konuma geçmez.
- **Secret ayarlara yazılmaz.** Proxy adresi kimlik bilgisi içeremez; ayar değerleri
  loglara düşüyor ve adrese gömülü bir parola düz metne dönüşürdü.
- **Ölçülmeyen gösterilmez.** "Çıkış denetleniyor" iddiası ancak ölçülerek yazılır;
  arayüzde ölçülmemiş bir güvence cümlesi kurulmaz.
- **Ayar arayüzden gelir, ortam değişkeni yedekte kalır.** Daha önce sunucu tarafında
  proxy tanımlamış kurulumlar sessizce bozulmaz; arayüzdeki değer doluysa o kazanır
  ve hangi kaynaktan geldiği söylenir.

## Hata Durumları

| Durum | Beklenen davranış |
|-------|-------------------|
| Proxy adresi geçersiz (şema yok, host yok) | Kaydedilmez; ne yazılması gerektiğini söyleyen hata |
| Proxy adresinde kullanıcı adı/parola var | Kaydedilmez; secret'ın ayara yazılmayacağı söylenir. Hata mesajı secret'ı tekrarlamaz |
| Whitelist'te geçersiz satır | Kaydedilmez; hangi satırın neden geçersiz olduğu söylenir |
| Proxy tanımlı ama ayakta değil | Çalıştırma, ortam hazırlanırken anlaşılır bir hatayla düşer; "bilinmeyen hata" denmez |
| Agent whitelist'te olmayan bir domain'e çıkmaya çalışır | O istek başarısız olur, çalıştırma sürer, olay akışında uyarı görünür |
| Whitelist dolu, proxy boş | Whitelist yok sayılır; arayüz bu durumu söyler |
| Backend yeniden başlarsa süren çalıştırma | Süren çalıştırmanın dış erişimi kesilir; bu bilinen ve belgelenen bir sınırdır |

---

## Belirsizlikler

- [x] Domain kısıtı yalnızca yönlendirme ile mi yapılsın, yoksa başka yol
      bırakılmayacak şekilde mi? → **Cevap:** Başka yol bırakılmadan. Ölçüm,
      yalnızca yönlendirmenin atlanabildiğini gösterdi; "denetim" iddiası
      atlanabilir bir mekanizmayla kurulamaz.
- [x] Kurumun kendi proxy'si varken whitelist'i kim uygulasın? → **Cevap:** Ürün
      uygular, sonra kurumun proxy'sine devreder. Kurumun proxy'sine bırakılsaydı
      ayarlardaki whitelist hiçbir şey yapmayan bir kutu olurdu.
- [x] Whitelist proxy'den bağımsız açılabilsin mi? → **Cevap:** Hayır. Proxy ana
      anahtardır; proxy girilmemişse whitelist devreye girmez.
- [x] Proxy dolu ama whitelist boşken ne olmalı? → **Cevap:** Tüm domain'ler açık,
      proxy zorunlu. Boş whitelist'i "hiçbir şeye izin yok" saymak, ayarı ilk açan
      herkesin ürününü kilitlerdi.
- [x] LLM provider ve git repository adresleri whitelist'e yazılmak zorunda mı? →
      **Cevap:** Hayır, her zaman izinli. Kullanıcı bu adresleri ayarlara zaten
      girmiş; oraya eriştiğinden emin olduğu için girmiş. Ayrıca yazmasını beklemek
      gereksiz tekrar olurdu.
- [x] opencode'un kendi ihtiyaç duyduğu domain'ler kullanıcıya bırakılsın mı? →
      **Cevap:** Hayır, ürün kendiliğinden izin verir ama arayüzde gösterir. Bunlar
      ölçümle bilinen adresler; kullanıcının deneme yanılmayla bulmasını beklemek
      ilk kurulumu kırardı.
- [x] Devreye alırken "önce raporla, sonra engelle" kipi olsun mu? → **Cevap:**
      Hayır. Proxy'nin ana anahtar olması bu ihtiyacı zaten karşılıyor: proxy
      girilmeden hiçbir kısıt yok.
- [x] Whitelist port'a da bakacak mı? → **Cevap:** Hayır, yalnızca domain'e. Kurumsal
      Nexus gibi araçlar standart dışı port kullanıyor (8081); port kısıtı bunları
      kırardı. İzinli bir domain'e her port açıktır.

## Bağımlılıklar

- [017 — Kurumsal ağ sertifikası](../017-kurumsal-ag-sertifikasi/spec.md): "ayar
  kazanır, ortam değişkeni yedekte kalır" kalıbı ve `Kurumsal ağ` ayar grubu oradan
  geliyor.
- [Veri sızıntısı analizi](../../docs/veri-sizintisi-analizi.md): bu spec'in
  gerekçesi ve doğrulama ölçütü o ölçüme dayanıyor.
