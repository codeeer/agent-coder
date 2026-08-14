# Görevler: Kurumsal paket deposu

- **Spec no:** 014 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı

---

## Yapılanlar

- [x] T01 `ENV NPM_CONFIG_OFFLINE` kaldırıldı, motora kapsamlı `.npmrc`
- [x] T02 Hata yeniden üretildi ve düzeltme aynı komutla doğrulandı
- [x] T10 Migration `000014` — `nexus` kimlik türü
- [x] T20 `settings.Definition.Optional` + `packages` grubu
- [x] T21 Adres doğrulaması (kimlik gömülü URL reddi)
- [x] T22 `buildNPMRC` + `packageSection` (+ birim testleri)
- [x] T23 `credentials.Validator` `nexus` türünü kabul eder
- [x] T30 Ayarlar → Paket deposu sekmesi
- [x] T40 `runner/offline_test.sh` yeşil kalır
- [x] T90 Gerçek koşuda agent `npm install` çalıştırır
- [x] T91 Sahte kurumsal depoya karşı uçtan uca doğrulama (bkz. Ölçüm 6);
      `always-auth` kaldırıldı — npm 9'da ölmüş ayar, her komutta uyarı basıyordu
- [x] T93 Ayarlar ekranı tarayıcıda denendi (bkz. Ölçüm 7); kimlik kartı
      doğrulamadığı hâlde "Doğrula ve kaydet" diyordu — `verifies` bayrağı eklendi
- [x] T92 `AGENTS.md` Durum kaydı; spec 003 Karar geçmişine kapsam düzeltmesi

---

## Ölçüm notları

### Ölçüm 1 — Kapalı ağ düzeltmesi agent'ın npm'ini kırmıştı

Bu iş "kurumsal depo ekleyelim" diye başladı; ilk komut hatayı gösterdi:

```
$ docker run --rm …runner:latest npm install is-odd
npm error code ENOTCACHED
npm error request to https://registry.npmjs.org/is-odd failed:
        cache mode is 'only-if-cached' but no cached response is available.
```

Yani agent hiçbir kurulumda bağımlılık kuramıyordu — kurumsal ağda değil,
**hiçbir yerde**. Sebep bir önceki spec'in kendi düzeltmesiydi: motorun koşu
anında npm'e çıkmasını engellemek için konan `ENV NPM_CONFIG_OFFLINE=true`
imaj geneliydi.

**Ders:** bir kısıt "doğru şeye" uygulanıyor diye doğru kapsamda değildir.
`ENV` bir container'daki **her** süreci bağlar; niyet yalnızca bir süreçti.
Kapsam, niyetin bir parçasıdır ve ayrıca ölçülmelidir.

### Ölçüm 2 — Kapsamı ölçmeden çözüm seçilmedi

`.npmrc` kapsamının motoru agent'tan ayırabildiği **varsayılmadı**, ölçüldü:

```
motor dizininde (~/.config/opencode)   npm config get offline → true
agent'ın dizininde (/work)             npm config get offline → false
kullanıcı ~/.npmrc registry=…          her ikisinde de geçerli
```

Üçüncü satır aynı zamanda kurumsal depo çözümünün de dayanağı: adres
kullanıcı kapsamında verilince ikisini de bağlıyor, `offline` ise yalnızca
motoru. İki kısıt, iki farklı kapsam, tek dosya biçimi.

### Ölçüm 3 — Ayarı log'a yazan kod, doğrulamayı zorunlu kılar

Adres doğrulamasına "kimlik gömülü URL reddedilir" kuralı estetik gerekçeyle
değil, **ayar yazma yolunun değeri log'a yazdığı** görüldüğü için kondu.
`https://kullanici:parola@nexus…` kaydedilseydi parola düz metin olarak
sunucu logunda dururdu.

**Ders:** bir alanın nereye aktığını bilmeden doğrulama kuralı yazılmaz.

### Ölçüm 4 — Kendi testim yanlış yerde arıyordu

`.npmrc` sızıntı testi şöyle yazılmıştı:

```go
require.False(t, strings.Contains(got, "https:"))
```

Test geçmiyordu — çünkü dosyanın **meşru** `registry=https://…` satırını da
tarıyordu. İddia "kimlik satırlarında adres olmasın" iken kontrol "dosyanın
hiçbir yerinde `https` olmasın" diyordu.

**Ders:** bir test kırmızıysa önce iddianın kendisi okunur. Kodu iddiaya
uydurmadan önce, iddianın yazdığım şey olduğundan emin olunmalı.

### Ölçüm 5 — Düzeltme "doğrulandı" ama üretimde çalışmıyordu

Düzeltme `latest` imajında doğrulandı. Bir gün sonra gerçek koşuda aynı
`ENOTCACHED` hatası görüldü: koşular `latest` değil `node-24.13.0` kullanıyor
ve o varyant yeniden derlenmemişti.

Ayrıntı ve ders: [spec 013 → Ölçüm 5](../013-node-surumlu-runner-imajlari/tasks.md).

Buraya düşen kısmı şu: **bir düzeltmenin doğrulaması, gerçekte kullanılan
artefakt üzerinde yapılmalıdır.** "İmajı derledim, çalışıyor" cümlesi hangi
imaj olduğu söylenmeden bir kanıt değildir.

### Ölçüm 6 — Sahte bir kurumsal depo kurulup uçtan uca koşuldu

Birim testleri `.npmrc`'nin **içeriğini** doğruluyordu; hiçbiri o dosyayla
npm'in gerçekten paket indirip indirmediğini söylemiyordu. Nexus'un dört
özelliği birlikte taklit edildi: kimlik doğrulama zorunlu (verdaccio +
htpasswd), depo bir **yol** altında (`/repository/npm-group/`, nginx ile),
HTTPS ve **özel bir kök sertifika**.

`.npmrc` elle yazılmadı — `BuildConfigFiles` ile üretilip container'a
üretimdeki gibi (uid 10001, 0600, `$HOME/.npmrc`) kondu.

| Durum | Sonuç |
| --- | --- |
| `.npmrc` yokken | `E401` — depo gerçekten kimlik istiyor |
| `.npmrc` varken | `added 2 packages`, depo logunda `user: ci-kullanici` |
| CA tanıtılmadan | `UNABLE_TO_VERIFY_LEAF_SIGNATURE` |
| `NODE_EXTRA_CA_CERTS` ile | kurulum başarılı |
| Lockfile npmjs.org'a çözümlü + `npm ci` | istekler **kurumsal depoya** gitti |

Son satır beklenmiyordu ve önemli: klonlanan repoların lockfile'ında
`resolved` adresleri genelde `registry.npmjs.org`'u gösterir. npm 9+ bunları
yapılandırılmış kayıt defterine yeniden yazıyor (`replace-registry-host`),
yani kapalı ağda `npm ci` de çalışıyor. Ölçülmeseydi bu özelliğin en sık
kullanılacak yolunun kırık olduğu varsayılabilirdi.

**Bulunan hata:** `always-auth=true` satırı. npm 9'da **kaldırıldı**, runner
npm 11 taşıyor — yani hiçbir işe yaramıyordu. Zararsız da değildi: dosya
kullanıcı kapsamında olduğu için npm **her** çağrıda

```text
npm warn Unknown user config "always-auth" … This will stop working in the
next major version of npm.
```

basıyordu; agent'ın okuduğu her araç çıktısına ve her derleme loguna. Satır
kaldırıldıktan sonra kimlik doğrulamanın **hem üstveri hem tarball**
isteğinde sürdüğü depo logundan doğrulandı.

**Ders:** Ölçüm 2 kapsamı ölçmüştü, bu ölçüm **davranışı** ölçtü. Bir
yapılandırma dosyasının doğru üretildiğini doğrulamak, onu okuyan aracın o
dosyayla çalıştığını doğrulamak değildir. Aradaki farkı yalnızca dosyayı
gerçek araca verip sonucu okumak kapatır — ve o aracın sürümü de kapsamın
bir parçasıdır: "npm'de böyle yapılır" bilgisi npm 6'dan kalmaydı.

### Ölçüm 7 — "Ayarlara yazınca yeter mi?" — hayır, ve arayüz bunu söylemiyor

Ekran Playwright ile açıldı; adres, kullanıcı adı ve token gerçekten girildi.
Çalışanlar: adres doğrulaması (şemasız adres ve kimlik gömülü URL tarayıcıda
da reddedildi), kaydetme, kalıcılık, silme onayı, iki tema.

Ama üç şey ölçülünce çıktı:

**1. Doğrulamayan düğme "Doğrula ve kaydet" diyordu.** Kasten geçersiz bir
token girildi; "tanımlı" rozetiyle sorunsuz kaydedildi. Yanındaki not
"kaydetmeden önce değerin gerçekten çalıştığı sınanır" diyordu — oysa
`credentials.Validator` nexus için `nil` döner, yani hiçbir sınama yok. Kart
Jira ile ortaktı ve Jira'nın sözünü nexus'a da veriyordu. `CredentialSpec`'e
`verifies` bayrağı eklendi; söz artık yalnızca tutabilen türde görünüyor.

**2. Kurumsal CA arayüzde hiç yok.** Sekizinin sekizi tarandı: hiçbir ayar
bölümünde sertifika alanı yok, "Nasıl çalışır" sayfası paket deposunu
anlatıyor ama sertifikadan söz etmiyor. Oysa HTTPS + özel CA'da `.npmrc`
tek başına YETMİYOR (Ölçüm 6: `UNABLE_TO_VERIFY_LEAF_SIGNATURE`). Gereken
`RUNNER_EXTRA_CA_CERT` + yeniden başlatma yalnızca README ve `.env.example`
içinde yazıyor.

**3. Proxy desteği yok.** `HTTP_PROXY`/`HTTPS_PROXY` runner container'ına
hiç geçmiyor — depoya ancak vekil sunucu üzerinden çıkılan ağlarda özellik
çalışmaz.

2 ve 3 kod hatası değil, **kapsam sınırı** — ama arayüzde karşılığı olmadığı
için kullanıcı bunu ancak koşu düşünce öğreniyor.

**Ders:** "ayar ekranı çalışıyor" ile "kullanıcı bu özelliği kurabiliyor"
aynı şey değil. Bir özelliğin kurulumu ekranın dışına taşıyorsa, ekran bunu
söylemek zorundadır; söylemiyorsa eksik olan kod değil, **yol göstermedir.**
