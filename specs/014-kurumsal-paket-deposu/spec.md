# Spec: Kurumsal paket deposu (npm kayıt defteri)

- **Spec no:** 014
- **Tarih:** 2026-08-12
- **Durum:** Uygulandı
- **Not:** Geriye dönük yazıldı — bkz. [spec 013 → Ölçüm 6](../013-node-surumlu-runner-imajlari/tasks.md).

---

## Problem

Kurumsal ağlarda container'lar npm'in genel deposuna çıkamaz; paketler iç bir
depodan (Nexus, Artifactory) çekilir. Bugün böyle bir adres tanımlanamıyor ve
agent'ın paket kurduğu her kurumsal kurulumda iş burada duruyor.

Ayrıca bu iş başlarken **daha temel bir hata ölçüldü**: agent'ın npm'i
yalnızca kurumsal ağda değil, **hiçbir yerde** çalışmıyordu (bkz. K1).

## Amaç

Bir npm kayıt defteri adresi tanımlanınca container içindeki tüm paket
kurulumları oraya gitsin, agent bunu bilsin ve adresle oynamasın.

## Kapsam dışı

- **maven / pypi / go proxy.** Runner imajında ne Java ne Python var; ayar
  koymak karşılığı olmayan bir söz olurdu. Anahtar adları (`packages.npm_*`)
  ikincisine yer bırakıyor.
- **Docker registry vekili.** Agent container içinde imaj çekmiyor.
- **TLS doğrulamasını gevşetmek.** Kurumsal ağın çözümü CA'yı **tanıtmaktır**;
  `strict-ssl=false` hiçbir koşulda kabul edilmez (bkz. K5).

---

## Kullanıcı hikâyeleri

### H1 — Kurumsal depo tanımlamak

**Yönetici** olarak, ayarlardan bir npm kayıt defteri adresi **tanımlamak**
istiyorum, çünkü kurumsal ağda paketler yalnızca oradan çekilebiliyor.

### H2 — Kimlik bilgisi zorunlu olmasın

**Yönetici** olarak, deponun **anonim okumaya açık** olduğu kurulumlarda
kullanıcı adı ve token girmek zorunda kalmak istemiyorum.

### H3 — Agent'ın adresle oynamaması

**Geliştirici** olarak, agent'ın `--registry` bayrağı vererek genel depoya
kaçmamasını istiyorum, çünkü kapalı ağda o komut zaman aşımıyla düşer ve
agent bunu "paket yok" diye yorumlar.

---

## Kabul kriterleri

- [x] Ayarlar → **Paket deposu** sekmesinde adres ve kullanıcı adı alanları
- [x] Alanlar **boş bırakılabiliyor**; boş = özellik kapalı
- [x] Token ayrı bir kimlik kaydı; **hiç yoksa** kimlik doğrulama yapılmıyor
- [x] Adres tanımlıyken agent'ın kurduğu paketler **o adresten** geliyor
- [x] Token hiçbir çalıştırmanın ortamında, çıktısında veya motor logunda
      görünmüyor
- [x] Kimlik gömülü adres (`https://user:pass@…`) reddediliyor
- [x] Adres tanımlıysa agent talimatına paket deposu bölümü ekleniyor;
      tanımlı değilse **hiç eklenmiyor**
- [x] Motorun kapalı ağ davranışı değişmiyor: koşu anında npm'e çıkmıyor

---

## Kararlar

### K1 — `offline` bayrağı motora özel, imaja değil

Kapalı ağ kararının ([spec 003](../003-agent-calistirma/spec.md)) ilk
uygulaması kısıtı **imaj geneli** bir ortam değişkeniyle veriyordu. Değişken
container'daki her süreci bağladığı için agent'ın kabuğuna da geçti ve
agent'ın paket kurulumu her yerde düştü.

Kısıt artık **yalnızca motoru** kapsıyor; agent'ın çalışma alanı serbest.
İkisini ayıran mekanizma ölçülerek seçildi — bkz. [plan.md → Kapsam ayrımı](plan.md).

### K2 — Kimlik ortam değişkeninde durmaz

Agent ortam değişkenlerini yazdırabiliyor ve çıktısı hem modele hem loglara
düşüyor. Kimlik bu yüzden agent'ın çalışma alanının **dışındaki** bir
yapılandırma dosyasında tutuluyor: kullanıcının diff'ine de girmiyor.
Git token'ı için verilen kararın aynısı.

### K3 — Adres ayarda, token kimlik deposunda

Kimlik deposunun değişmezi şu: **orada duran her kayıt bir sırdır.**
Opsiyonellik, boş bir sır kabul ederek değil, **kaydın hiç olmamasıyla**
sağlanır — değişmez bozulmaz. Adres ve kullanıcı adı sır değildir, ayarlarda
durur.

### K4 — "Boş = kapalı" ayrıca işaretlenir

Ayarlar boş metin değerini reddediyordu ve haklıydı: zorunlu bir ayarın boş
bırakılması bir yapılandırma hatasıdır. Ama "boş = bu özellik kapalı" ondan
ayrı bir şey. Bu yüzden ayar tanımına, boşluğun geçerli olduğunu söyleyen
açık bir işaret eklendi — kural gevşetilmedi, ikinci bir hâl tanındı.

### K5 — Kimlik gömülü adres reddedilir

`https://user:pass@nexus…` biçimi doğrulamada reddediliyor. İki sebep:
kimlik URL'e gömülmemeli **ve** ayar yazma yolu değeri log'a yazıyor.

---

## Hata durumları

| Durum | Beklenen davranış |
|-------|-------------------|
| Adres boş | Yapılandırma hiç yazılmaz, agent talimatına bölüm eklenmez |
| Adres var, token yok | Depoya kimliksiz gidilir |
| Adreste kimlik gömülü | Ayar kaydedilmez, açıklayıcı hata |
| Depoda paket yok | Agent'a "depoda gerçekten yok demektir" denir; genel depoya kaçması istenmez |
