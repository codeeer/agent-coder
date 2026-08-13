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
