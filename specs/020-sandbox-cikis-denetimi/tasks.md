# Tasks: Sandbox çıkış denetimi

- **Spec no:** 020 — [spec.md](spec.md) · [plan.md](plan.md)
- **Tarih:** 2026-08-14

Sıra plandaki gerekçeye uyar: riskli ve belirsiz olan (kapının kendisi) başta,
ağ ve arayüz sonra. İlk üç blok ağ olmadan test edilebilir.

---

## Blok 1 — Whitelist dilbilgisi

- [x] T01 `internal/hostlist` paketi: `Parse` satırları ayrıştırır, boş satır ve
      `#` yorumunu atlar → birim testi: 3 satırlık metinden 2 desen çıkar
- [x] T02 `Match` tam domain eşleştirir → `ornek.com` deseni `ornek.com`'u geçirir,
      `alt.ornek.com`'u geçirmez
- [x] T03 `*.ornek.com` deseni subdomain'leri geçirir, apex'i geçirmez →
      `alt.ornek.com` true, `ornek.com` false
- [x] T04 Büyük/küçük harf ve sondaki nokta normalleştirilir → `ORNEK.com.`
      deseniyle `ornek.com` eşleşir
- [x] T05 Geçersiz satırlar reddedilir, hata satır numarasını söyler → URL
      (`https://a.com`), port (`a.com:443`), boşluklu metin, ASCII dışı karakter
      için ayrı ayrı hata
- [x] T06 Boş liste `Match`'te her host'u geçirir → spec'teki "boş whitelist
      kısıtsızlıktır" kuralı testle kilitlenir

## Blok 2 — Çıkış kapısı

- [x] T10 `internal/netgate`: `New` + `Serve` dinleyiciyi açar, `Address()` runner'a
      verilecek URL'i döner → test rastgele portta ayağa kaldırır ve bağlanır
- [x] T11 `CONNECT` izinli host için upstream'e devredilir → sahte upstream proxy
      ile test: gövde uçtan uca akar
- [x] T12 `CONNECT` izinsiz host için reddedilir ve `OnDeny` çağrılır → istemci
      hata alır, `OnDeny` bir kez host adıyla çağrılır
- [x] T13 Kayıtsız IP'den gelen istek reddedilir → ayar boşken kapının fiilen
      kapalı olduğu testle gösterilir
- [x] T14 Düz HTTP (mutlak URL'li istek) aynı kararı alır → izinli geçer, izinsiz
      reddedilir
- [x] T15 Upstream ulaşılamazken anlaşılır hata döner → test sahte upstream'i
      kapatır, dönen hata "bilinmeyen" değil, sebebi yazar
- [x] T16 `Register`/`Unregister` eşzamanlı çağrılarda güvenli → `-race` ile test
- [x] T17 Gövde tamponlanmaz, akıtılır → 8 MB'lık gövde sabit bellekte geçer
      (ölçüm notu tasks sonuna yazılır)

## Blok 3 — Ayarlar

- [x] T20 `KindHostList` tipi + `network.allowed_hosts` tanımı → `GET /api/settings`
      çıktısında yeni ayar görünür
- [x] T21 `network.proxy_url` tanımı → `GET /api/settings` çıktısında görünür,
      varsayılanı boş
- [x] T22 `Validate` `KindHostList` case'i `hostlist.Parse`'ı çağırır → geçersiz
      satır 400 ile reddedilir, hata hangi satır olduğunu söyler
- [x] T23 Proxy URL doğrulaması: şema ve host zorunlu, kimlik gömülemez →
      `http://kullanici:parola@p:8080` reddedilir **ve hata mesajı parolayı
      tekrarlamaz** (spec 017'deki `TestValidate_KayitDefteriAdresi` kalıbı)
- [x] T24 Kayıt defteri tutarlılık testleri yeşil kalır → `TestRegistry_*` geçer
      (yeni tip `Validate` switch'ine eklenmezse bu test kırmızıya döner)

## Blok 4 — Ağ ve container yolu

- [x] T30 `deploy/docker-compose.yml`: `restricted` network (`internal: true`),
      backend iki network'te → `docker network inspect` ile `Internal: true`
      görünür, backend her iki network'te listelenir
- [x] T31 Kısıtlı network'ün gerçekten kapalı olduğu doğrulanır → o network'e bağlı
      geçici bir container'dan `curl https://example.com` **başarısız** olur
- [x] T32 `sandbox`: container'ın network IP'sini okuma → birim/elle test ile
      başlatılan container'ın IP'si dönüyor
- [x] T33 Çalıştırma başına network seçimi: proxy doluysa kısıtlı, boşsa mevcut →
      iki çalıştırma başlatılır, `docker inspect` ile ikisinin farklı network'te
      doğduğu görülür
- [x] T34 `applyProxy` hedefi kapı olur → birim testi: verilen kapı adresi tüm
      değişkenlerde ve JVM `-D` özelliklerinde görünür
- [x] T35 Çalıştırma kaydı aç/kapa `defer` ile her yola bağlanır → başarı, hata ve
      iptal yollarının üçünde de kayıt kapanır
- [x] T36 **İlk uçtan uca çalıştırma**: proxy tanımlı, whitelist'te repository ve
      `repo1.maven.org` var → çalıştırma başarıyla biter

## Blok 5 — Görünürlük

- [x] T40 Ret, `slog` ile loglanır → backend logunda host ve çalıştırma kimliği
      görünür
- [x] T41 Ret, çalıştırmanın olay akışına uyarı olarak yazılır → arayüzde
      çalıştırma ekranında görünür, metin whitelist'e eklemeyi söyler
- [x] T42 Aynı host tekrar reddedildiğinde akış boğulmaz → 50 denemede olay akışına
      sınırlı sayıda satır düşer, kaç kez denendiği yazar

## Blok 6 — Arayüz

- [ ] T50 `GET /api/network/egress` → proxy kaynağı ve "her zaman izinli" listeler
      dönüyor; `curl` ile doğrulanır
- [ ] T51 `host_list` kind'ı frontend'de çok satırlı alan olarak çizilir →
      ayarlar → Kurumsal ağ'da textarea görünür [P]
- [ ] T52 "Her zaman izinli" bölümü gerçek yapılandırmadan geliyor → ekranda
      görünen adresler tanımlı provider/repository/registry ile aynı [P]
- [ ] T53 Proxy boşken whitelist'in etkisiz olduğu arayüzde söylenir → proxy
      alanı boşaltılınca uyarı beliriyor
- [ ] T54 Açık ve koyu tema ayrı ayrı doğrulanır → DevTools ile ekran görüntüsü,
      hizalama ve taşma kontrolü

## Blok 7 — Ölçüm ve belgeler

- [x] T60 **Zorlamanın kanıtı:** denetim açıkken Maven'lı görev koşulur, köprüde
      tcpdump alınır → runner IP'sinden kapı dışına giden SYN sayısı **0**
      (önceki ölçümde 5'ti). Sayı `## Ölçümler`e yazılır
- [x] T61 **Ret yolu ölçümü:** whitelist'ten `archive.apache.org` çıkarılıp aynı
      görev koşulur → çalıştırma ekranında ret uyarısı, backend 5xx yok
- [ ] T62 **Kapalı davranış:** proxy boşken çalıştırma → runner eski network'te,
      hiçbir proxy değişkeni yazılmamış (`docker inspect` ile env kontrolü)
- [ ] T63 `docs/kurumsal-ag.md`: yeni ayarlar, kapsam tablosu, `java -jar` sınırı
      ve "izinli host üzerinden sızdırma" uyarısı
- [ ] T64 `README.md` ve `.env.example` güncellenir → yeni ortam değişkenleri
      belgelenmiş
- [ ] T65 `docs/veri-sizintisi-analizi.md`'ye "Öneriler" bölümünün 1. maddesinin
      karşılandığı ve yeni ölçüm sonucu eklenir
- [ ] T66 `specs/README.md` tablosuna 016–020 eklenir (tablo 015'te kalmış)
- [ ] T67 `make test` · `make lint-backend` · `npx tsc --noEmit` · `npx eslint .`
      temiz

---

## Ölçümler

| Ne | Değer |
|----|-------|
| Kapı dışına giden bağlantı (önce) | 5 — `repo.maven.apache.org` |
| Kapı dışına giden bağlantı (sonra) | **0** — runner yalnızca kapıya bağlandı |
| 8 MB tünelden akarken heap artışı | < 4 MB (T17, `-race` ile) |
| Kapalı network'ten DNS ile çıkış | `Could not resolve host` |
| Kapalı network'ten **doğrudan IP'ye** çıkış | `Could not connect` — 0 ms, rota yok |
| Kapalı network'te komşu container'a erişim | 200 — kapı erişilebilir |

## Notlar

**Sapma 1 — kimlik kaynak IP ile değil, port ile.** Plan "tek dinleyici +
kaynak IP" diyordu. Ölçüldü: Docker container'ın IP'sini YALNIZCA BAŞLATMADA
atıyor (öncesinde `invalid IP`). Kayıt ancak başlatmadan sonra yapılabilirdi,
oysa container başlar başlamaz klonluyor — kaydın yetişmediği her seferde
klonlama sessizce reddedilirdi. Artık her çalıştırma kendi portunda kendi
oturumunu açıyor; port container yaratılmadan önce biliniyor, yarış yok.
Sonuç ölçümde de görünüyor: iki koşu, iki ayrı kapı portu (44073, 34867).

**Sapma 2 — `internal/hostlist` nokta şartı kaldırıldı.** İlk sürüm "en az bir
nokta içermeli" diyordu. İlk gerçek koşu bu yüzden düştü: depo adresi tek
parçalı bir Docker servis adıydı (`sizinti-depo`), otomatik izinliler
listesine giremedi ve klonlama reddedildi. Kurumsal ağlarda `nexus`, `gitlab`
gibi tek parçalı iç adlar yaygın. Nokta yerine açık bir yıldız kuralı kondu.

**Sapma 3 — ayrıştırılamayan zorunlu adres artık loglanıyor.** Sessiz atlama,
yukarıdaki hatayı gizledi: ekranda yalnızca "klonlama başarısız" yazıyordu ve
sebebi backend logunda aramak gerekti.

**Plana ek — ayar log'unun özetlenmesi.** Plan yalnızca riskler tablosunda
değinmişti; uygulanırken ayrı bir görev haline geldi (`logDegeri` +
`internal/httpapi/settings_test.go`). Gerekçe sertifikadakiyle aynı ve ölçülmüş:
kurumsal bir izin listesi onlarca satır olabiliyor, ham hâliyle loglanınca o
satırdan sonrasını okumak zorlaşıyor. Değer sır değil, sorun ölçü.
