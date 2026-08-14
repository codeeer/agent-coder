# Görevler: Kurumsal ağ sertifikası

- **Spec no:** 017 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Uygulandı (2026-08-14)

Sıra riske göre: önce mevcut kurulumları bozmadan mekanizmayı değiştirmek,
sonra backend'in kendi çağrıları, en son arayüz. İlk beş görev bitene kadar
arayüz hiç değişmez — kurumsal ağ davranışı tek başına doğrulanabilsin.

Doğrulamalar bu oturumda kurulan kurumsal Nexus provasına karşı koşulur
(bkz. [docs/kurumsal-ag.md](../../docs/kurumsal-ag.md)).

---

## Temel — sertifikanın okunması

- [x] T00 Test verileri openssl ile üretilir: PEM, çıplak base64, DER, PKCS#7
      zinciri, sertifika+özel anahtar içeren dosya → beş dosya elde edilir ve
      `certfmt` testine gömülür (elle uydurulmuş veri kullanılmaz)
- [x] T01 `certfmt.ToPEM`: dört biçim de aynı PEM'i verir → `go test` yeşil
- [x] T02 [P] `certfmt` sertifika dışındaki blokları atar → özel anahtar içeren
      dosyada çıktıda anahtar **yok**; çöp girdi hata döndürür
- [x] T03 `certinfo` paketi: PEM → sahip / imzalayan / bitiş tarihi →
      `go test` yeşil
- [x] T04 [P] `certinfo` testleri: tek sertifika, kök+ara zincir, bozuk metin,
      süresi dolmuş sertifika → dördü de beklenen sonucu verir
- [x] T05 `KindCertificate` tipi ve `network.corporate_ca` ayarı kayıt defterine
      girer (grup `network`, `Optional`) → `GET /api/settings` yanıtında görünür
- [x] T06 `Validate` `KindCertificate` dalını alır → geçerli PEM 200, bozuk metin
      422, boş değer kabul edilir
- [x] T07 Baş/son boşluk kırpması PEM'i bozmuyor → başında ve sonunda boşlukla
      yapıştırılan sertifika geçerli sayılır ve okunabilir kalır

## H4 — Çalışma ortamının kapsanması

- [x] T10 Sertifika kaynağı çözümlenir: ayar doluysa ayar, boşsa
      `RUNNER_EXTRA_CA_CERT` dosyası → iki kaynak da aynı içeriği üretir
- [x] T11 `BuildConfigFiles` sertifika doluyken dosya üretir → doğru yol ve mod;
      sertifika boşken **hiç** dosya üretilmez
- [x] T12 Üç ortam değişkeni verilir (`CURL_CA_BUNDLE` **yeni**) → yalnızca
      sertifika varken; yokken bugünküyle aynı
- [x] T13 Bind mount kaldırılır (`caBind`, `sandbox.Spec.ExtraCACert`, compose
      runner bind'i) → çalışma ortamı yine sertifikayı görür
- [x] T14 Prova Nexus'una karşı: node, git ve **curl** üçü de TLS hatası almadan
      ulaşır → üçünün de HTTP durumu döner, sertifika hatası yok
- [x] T15 `RUNNER_EXTRA_CA_CERT` ile kurulmuş mevcut kurulum bozulmaz → ayar
      boşken, yalnızca env doluyken T14 aynı sonucu verir

## H5 — Ürünün kendi çağrılarının kapsanması

- [x] T20 `tlstrust` paketi: geçerli PEM'den havuz kurar, PEM değişince
      **yeniler** → birim testte iki farklı PEM iki farklı havuz verir
- [x] T21 [P] Boş PEM'de sistem güven havuzu korunur → genel adresler çalışmaya
      devam eder
- [x] T22 Yedi `http.Client` taşıyıcıyı kullanır → `grep -rn "http.Client{"`
      sonucundaki her yer taşıyıcıdan besleniyor, taşıyıcısız kalan yok
- [x] T23 Kurumsal sertifikayla imzalanmış bir adrese backend'den ulaşılır →
      prova Nexus'una karşı sağlayıcı doğrulaması tipinde bir çağrı başarılı
- [x] T24 Sertifika değişince **yeniden başlatmadan** yeni değer geçerli olur →
      ayar değiştirilir, sonraki çağrı yeni havuzu kullanır

## H2, H3 — Durumun görünmesi

- [x] T30 `GET /api/network/ca` eklenir → kaynak (`settings` / `env` / `none`)
      ve çözülen sertifika bilgisi döner
- [x] T31 Üç kaynak durumu da doğrulanır → yalnızca ayar, yalnızca env, hiçbiri
- [x] T32 Frontend `certificate` tipini çizer: satır tam genişliğe geçer, çok
      satırlı alan gelir → içine PEM yapıştırılıp kaydedilebilir
- [x] T32a `POST /api/network/ca/normalize` eklenir → dört biçim de PEM döner;
      64KB üstü gövde reddedilir
- [x] T32b Arayüzde dosya seçme → seçilen **ikili** (DER) dosyanın içeriği PEM
      olarak alana düşer; kaydetmeden önce kullanıcı görür
- [x] T32c Zincirli dosya (PKCS#7) seçilir → kök **ve** ara sertifikaların ikisi
      birden alana gelir, durum şeridinde ikisi de listelenir
- [x] T32d Sertifika olmayan dosya seçilir → alan doldurulmaz, dosyanın
      sertifika içermediği söylenir, mevcut içerik bozulmaz
- [x] T33 Ayar satırı denetim hizası bozulmaz → sayı alanlarının sol kenarı
      `getBoundingClientRect` ile yeniden ölçülür, spec 016'daki değerle aynı
- [x] T34 **Kurumsal ağ** bölümü ayarlar menüsüne eklenir → bölüm açılır ve
      sertifika alanı görünür
- [x] T35 Durum şeridi: kaynak + sahip + imzalayan + bitiş tarihi → gerçek
      sertifikanın değerleriyle eşleşir
- [x] T36 Süresi dolmuş sertifika kaydedilir ama durumu yazılır → "süresi
      dolmuş" bilgisi görünür
- [x] T37 Bozuk metin reddedilir → hata satırın yanında görünür, önceki değer
      korunur
- [x] T38 "sertifika" araması ayarı bulur → spec 016'da "Eşleşme yok" diyen
      arama artık sonuç döndürür

## Belgeler ve kapanış

- [x] T40 `README.md`, `docs/kurumsal-ag.md` ve `.env.example` yeni yöntemi
      anlatır → sertifikanın arayüzden girildiği, `.env`'in yedek kaldığı yazılı
- [x] T41 `docs/kurumsal-ag.md` kapsam tablosu güncellenir → `curl` artık
      kapsanıyor, backend artık kapsanıyor
- [x] T42 İki tema ve üç genişlik → çok satırlı alan taşmıyor, durum şeridi iki
      temada okunuyor
- [x] T43 `go vet`, `npx tsc --noEmit`, `npx eslint .` temiz; `go test` ve
      `npm run test` yeşil

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.

**1. `certfmt` planda yoktu — tip adı düzeltmesiyle birlikte geldi.**
Plan `KindPEM` diyordu; PEM bir yazılış biçimi, ayarın kendisi ise sertifika.
Ad düzeltilince (`KindCertificate`) kabul edilen biçimlerin de genişlemesi
gerekti: kurumsal ekipler DER ve PKCS#7 dosyalarını aynı `.crt`/`.cer`
uzantısıyla dağıtıyor, yani kullanıcı elindekinin hangisi olduğunu bilmiyor.

**2. PKCS#7 için bağımlılık eklenmedi.**
Standart kütüphanede ayrıştırıcı yok. `encoding/asn1` ile yalnızca SignedData
içindeki sertifika kümesi okunuyor — imza doğrulanmıyor, içerik
yorumlanmıyor. Riskli görünüyordu; `openssl crl2pkcs7` ile üretilmiş gerçek
dosyalara karşı ilk denemede çalıştı (hem PEM zırhlı hem ikili).

**3. `BuildConfigFiles`'ın imzası değiştirilmedi.**
Plan sertifikayı oraya parametre olarak eklemeyi öngörüyordu. Fonksiyon
yirmiden fazla yerden çağrılıyor ve sertifika, sağlayıcı/agent/model üçlüsüyle
aynı cümlenin parçası değil — bağımsız bir ortam gerçeği. Ayrı bir
`CACertFile` fonksiyonu ikisini de olduğu yerde bıraktı.

**4. Yedi değil ALTI istemci bağlandı.**
`runner/opencode/client.go` bilerek dışarıda: hedefi internet değil, izole
Docker ağındaki container ve adres `http://` — doğrulanacak TLS zinciri yok.
Gerekçe koda yazıldı ki sonradan "eksik" sanılmasın.

**5. HTTP/2'yi bozan bir hata testle yakalandı (planda öngörülmemişti).**
Kurumsal sertifika eklenirken `TLSClientConfig` komple değiştiriliyordu.
`Transport.Clone()` o alanı HTTP/2 için zaten dolduruyor (`NextProtos`);
değiştirmek backend'in **tüm** giden çağrılarını sessizce HTTP/1.1'e
düşürüyordu — sertifikayla hiç ilgisi olmayan bir yan etki. Havuz artık
mevcut yapılandırmanın üstüne yazılıyor, regresyon testi kondu.

**6. Ayar log'u sertifikada özetleniyor (planda yoktu).**
Doğrulama sırasında görüldü: ayar değişikliği değerin tamamını logluyor ve
sertifikada bu, her kaydetmede ~2KB base64 demek. Sır olduğu için değil,
ÖLÇÜSÜ yüzünden: o satırdan sonra logu okumak imkânsızlaşıyordu. Sertifika
tipinde artık "sertifika: <sahip adları>" yazılıyor.

**7. Arayüzde iki pano yerine tek pano.**
Plan durum şeridi ile alanı ayrı panolara koyuyordu; tarayıcıda görüldü ki
"Kurumsal kök sertifika" başlığı üç kez tekrar ediyor (iki pano başlığı + satır
etiketi). Tek panoda birleştirildi, durum önce.

**8. Süresi dolmuş sertifika için fixture eklendi.**
`certinfo/testdata/suresi-dolmus.pem` — openssl ile geçmiş tarihli üretildi.
Özel anahtar içeren fixture ise depoya KONMADI: gerçek bir anahtarı depoya
yazmak sır tarayıcılarını tetikler. O test verisi testin içinde üretiliyor.

## Ölçümler

| Ne | Sonuç |
| --- | --- |
| Denetim hizası (T33) | Dokuz denetim, sol kenar **1196px** — spec 016'daki değerin aynısı |
| Çalışma ortamı: node / git / curl (T14) | Üçü de TLS hatası almadan Nexus'a ulaştı; `npm install` nexus.local'den geldi |
| Backend'in kendi çağrısı, sertifikasız (T23) | "adrese ulaşılamadı" — TLS el sıkışması düşüyor |
| Backend'in kendi çağrısı, sertifikalı (T23) | "anahtar doğrulanamadı" — TLS geçti, Nexus yanıt verdi |
| Yeniden başlatma (T24) | Yok: iki ölçüm arasında yalnızca ayar değişti |
| Ortam değişkeni yedeği (T15) | Ayar boşken kaynak `env`, backend çağrısı yine başarılı |
| İkili DER dosyası (T32b) | Arayüzde seçildi, 1233 baytlık PEM olarak alana düştü |
| PKCS#7 zinciri (T32c) | Tek dosyadan **iki** sertifika alandı; durum şeridinde "kök" ve "ara" |
| Sertifika olmayan dosya (T32d) | "dosya sertifika içermiyor"; mevcut iki sertifika bozulmadı |
| Geçersiz metin (T37) | Reddedildi; sunucudaki değer korundu |
| Süresi dolmuş (T36) | Kaydedildi ve "süresi dolmuş" **yazıyla** işaretlendi |
| Telefon genişliği (T42) | 390px'te yatay taşma yok |
| Statik (T43) | `gofmt`/`go vet` temiz, 25 Go paketi yeşil; `tsc`/`eslint` temiz, 66 frontend testi yeşil |
