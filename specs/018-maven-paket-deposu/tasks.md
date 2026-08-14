# Görevler: Java / Maven ve kurumsal paket deposu

- **Spec no:** 018 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Uygulandı (2026-08-14)

Sıra riske göre: önce imaj (en uzun geri bildirim döngüsü ve diğer her şeyin
önkoşulu), sonra sertifika, sonra ayarlar ve dosya üretimi, en sonda gerçek
agent koşusu.

Doğrulamalar bu oturumda ayakta duran kurumsal Nexus provasına karşı koşulur
(bkz. [docs/kurumsal-ag.md](../../docs/kurumsal-ag.md)); Maven tarafı için
Nexus'un hazır gelen `maven-public` grubu kullanılır.

---

## H1 — Java ve Maven çalışma ortamında

- [x] T01 `runner/Dockerfile`: Adoptium deposu eklenir, `temurin-17-jdk` ve
      `temurin-25-jdk` kurulur → `make runner` biter, container'da
      `/opt/java/17` ve `/opt/java/25` **ikisi de** `java -version` veriyor
- [x] T02 Sabit bağlar mimariden bağımsız → `/opt/java/17` ve `/opt/java/25`
      arm64 imajında çözülüyor; yol içinde `amd64`/`arm64` geçmiyor
- [x] T03 Maven sürümü sabitlenmiş tarball'dan kurulur (apt DEĞİL) →
      `mvn -version` çalışır ve `dpkg -l | grep default-jdk` **boş** (üçüncü bir
      JDK girmemiş)
- [x] T04 Varsayılan `JAVA_HOME` = `/opt/java/25` → `java -version` 25 der
- [x] T05 JDK'lar ayrı derleme aşamasında → opencode sürümü değişince JDK
      katmanı yeniden indirilmiyor (katman kimliği aynı kalıyor)
- [x] T06 İmaj boyutu ölçülür ve kaydedilir → öncesi/sonrası MB olarak Notlar'a
- [x] T07 Java kullanmayan koşu bozulmadı → React projesinde `npm install` +
      `npm run build` bugünküyle aynı sonucu veriyor

## H3 — Sertifikanın Java tarafında geçerli olması

- [x] T10 `entrypoint.sh`: sertifika varsa `cacerts` kopyalanıp içine kurumsal
      sertifika eklenir → `/home/agent/kurumsal-truststore.jks` oluşuyor
- [x] T11 `MAVEN_OPTS` yalnızca truststore üretildiğinde tanımlanır →
      sertifika yokken değişken **hiç yok**, davranış bugünküyle aynı
- [x] T12 [P] Kabuk testi: sertifikalı ve sertifikasız iki koşul → truststore
      üretimi ve `MAVEN_OPTS` beklendiği gibi
- [x] T13 Truststore **varsayılan JDK'nın** `cacerts`'inden türetilir → genel
      sertifikalar geçerli kalıyor (genel bir HTTPS adresine erişilebiliyor)
- [x] T14 Sertifikasız karşıtlık gösterilir → `mvn` kurumsal aynaya PKIX hatası
      alıyor; sertifika tanımlanınca aynı komut indiriyor

## H2 — Bağımlılıkların kurumsal depodan gelmesi

- [x] T20 `packages.maven_registry` ayarı kayıt defterine girer (`text`,
      `Optional`) → `GET /api/settings` yanıtında görünür, arayüz kendiliğinden
      çizer
- [x] T21 `buildSettingsXML`: adres yokken **nil** → dosya hiç yazılmaz,
      container bugünküyle birebir aynı
- [x] T22 Adres varken `mirrorOf=*` yazılır → projenin **kendi** ilan ettiği
      depo da kurumsal adrese çevriliyor
- [x] T23 Kimlik kaydı yokken `<server>` bloğu **yazılmaz** → anonim okumaya
      açık depo çalışır
- [x] T24 [P] Parolada XML özel karakteri kaçırılır → `&`, `<`, `"` içeren
      parola geçerli XML üretiyor (testle)
- [x] T25 `.m2` dizini yoksa dosya yine yazılıyor → tar akışı ara dizini
      oluşturuyor (container'da doğrulanır)
- [x] T26 Agent talimatına Maven bölümü eklenir → adres ve "bu adresi
      değiştirme, `-s` verme, pom'a depo ekleme" bilgisi talimatta görünüyor
- [x] T27 İki JDK'nın yeri ve varsayılanı talimatta yazılı → agent ikinci
      sürüme geçmek için tahmin etmek zorunda değil

## H4 — Ulaşılamayan deponun çalıştırmayı yakmaması

- [x] T30 `packages.timeout_seconds` ayarı (5–600, varsayılan 60) → arayüzde
      görünür ve kaydedilebilir
- [x] T31 `.npmrc`'ye süre satırları yazılır (`fetch-timeout`, `fetch-retries`)
      → yalnızca kurumsal adres tanımlıyken; tanımsızken dosya bugünküyle aynı
- [x] T32 **Maven süre sınırı özellik adı ÖLÇÜLEREK doğrulanır** → ulaşılamayan
      bir adrese karşı geçen süre tutulur; ayar 10 sn'ye çekildiğinde `mvn`
      belirgin biçimde daha erken vazgeçiyor. Ölçülmeden "çalışıyor" denmez
- [x] T33 npm tarafı ölçülür → ulaşılamayan adreste `npm install`, spec 017
      doğrulamasındaki ~4 dakikanın belirgin altına iniyor
- [x] T34 Süre sınırı npm ve Maven için **aynı** ayardan besleniyor → iki ayrı
      yerde aynı karar verilmiyor

## Uçtan uca ve kapanış

- [x] T40 Gerçek agent koşusu: `mybatis-spring-boot-starter` → agent `mvn`
      çalıştırabiliyor ve bağımlılıklar **kurumsal Nexus'tan** geliyor
      (Nexus erişim logunda `maven-public` istekleri, kullanıcı `admin`)
- [x] T41 Aynı koşuda `java -version` iki sürümü de gösterebiliyor
- [x] T42 Maven adresi tanımsızken davranış bugünküyle aynı → koşu Maven'ı
      genel depoya götürüyor (kapalı ağ dışında sorun değil)
- [x] T43 `README.md` ve `docs/kurumsal-ag.md` Java/Maven bölümüyle güncellenir
      → kapsam tablosunda Java artık ✅; `MAVEN_OPTS` sınırı yazılı
- [x] T44 `gofmt`, `go vet`, `shellcheck`, `go test`, `npx tsc --noEmit`,
      `npx eslint .` temiz

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.

**1. Ayrı derleme aşaması yerine KATMAN SIRASI.**
Plan JDK'ları ayrı bir `FROM` aşamasına koymayı öngörüyordu. Aynı sonuç
(opencode yükseltmesi JDK'ları yeniden indirmesin) katman sırasıyla elde
edildi: JDK bloğu opencode'dan ÖNCE duruyor, yani `OPENCODE_VERSION`
değiştiğinde JDK katmanı geçerli kalıyor. Çok aşamalı build, kazanç
sağlamadan bir kavram daha eklerdi.

**2. `javaSection` KOŞULSUZ yazılıyor.**
Planda belirtilmemişti. Runner projenin dilini bilmiyor — depo, container
başladıktan sonra klonlanıyor. Bölümü "Maven tanımlıysa" koşuluna bağlamak,
kurumsal deposu olmayan bir kurulumda Java projesiyle çalışan agent'ı ikinci
JDK'dan habersiz bırakırdı.

## Gerçek koşuların bulduğu üç hata

Hiçbiri birim testiyle yakalanamazdı; üçü de ancak konteynerde çalışan gerçek
bir agent koşusunda ortaya çıktı.

**1. `cp`, salt okunur `cacerts`'i salt okunur kopyaladı.**
JDK'nın güven listesi `-r--r--r--`. Kopyası da öyle olunca `keytool`
sertifikayı belleğe ekliyor ama dosyaya yazamıyordu ve
"Certificate was added to keystore" satırının HEMEN ARDINDAN
"Permission denied" veriyordu — yanıltıcı bir çift mesaj. Düzeltme:
kopyadan sonra `chmod u+w`.

**2. `JAVA_HOME=/opt/java/17 java` Java 25 çalıştırıyor.**
Agent bunu denedi, 25 aldı ve "dizin yok galiba" diye rapor etti. Sebep:
`java` kabuktan `PATH` ile bulunuyor, `JAVA_HOME` onu etkilemiyor; Maven'ın
başlatıcısı ise `JAVA_HOME` okuyor. Yani `mvn` için doğru, doğrudan `java`
için yanlıştı. Talimat artık ikisini AYIRIYOR ve `JAVA_HOME=... java`nın
çalışmadığını açıkça söylüyor. Regresyon testi kondu.

**3. `.m2` dizini root sahipliğinde yaratılıyordu.**
Docker'ın tar kopyası eksik ara dizinleri root olarak açıyor. `settings.xml`
sorunsuz yazılıyor — yani dosyanın varlığı yanıltıcıydı — ama Maven kendi
yerel deposunu (`~/.m2/repository`) içine açamıyor ve gerçek koşuda
`AccessDeniedException` ile düşüyordu. Düzeltme: dizin imajda önceden
açılıyor (projenin `scripts/` ve `.config/opencode/agents` için zaten
kullandığı kalıp).

## Ölçümler

| Ne | Sonuç |
| --- | --- |
| İmaj boyutu | 832 MB → **1,86 GB** (+1,03 GB; iki JDK + Maven) |
| Java sürümleri | `/opt/java/25` → 25.0.4 LTS · `/opt/java/17` → 17.0.20 |
| Üçüncü JDK | Yok — `default-jdk` kurulmadı, `/usr/lib/jvm` yalnızca iki Temurin |
| Maven | 3.9.16, sha512 doğrulanmış tarball'dan |
| Truststore | 1 kurumsal + **143 genel kök** — genel sertifikalar korundu |
| **Maven süre sınırı özelliği** | `aether.connector.connectTimeout` **çalışıyor**; `maven.wagon.http.*` **hiçbir etki yapmıyor** (Maven 3.9 wagon kullanmıyor) |
| Ulaşılamayan adrese karşı süre | varsayılan **98 sn** → 3 sn sınırla **31 sn** |
| Gerçek agent koşusu | `mvn -pl mybatis-spring-boot-autoconfigure dependency:resolve` → **BUILD SUCCESS**, 76 bağımlılık (Spring Boot 4.1.0, Spring 7.0.8, MyBatis 3.5.19) |
| Kurumsal depodan geldi mi | Nexus erişim logunda **14.997** `Apache-Maven/3.9.16` isteği, kullanıcı `admin`, HTTPS |
| Agent `settings.xml`'i okuyamıyor | Parola dosyada; agent'ın okuma yetkisi `/work` ile sınırlı — token sızmıyor |
| **npm süre sınırı** | ayar öncesi **295 sn** → varsayılan 60 sn ayarla **96 sn** |
| Java kullanmayan koşu | React projesi: `npm install` 4 sn + `npm run build` başarılı — değişmedi |
| Statik | `gofmt`/`go vet`/`shellcheck` temiz, 25 Go paketi yeşil; `tsc`/`eslint` temiz, 66 frontend testi yeşil |
