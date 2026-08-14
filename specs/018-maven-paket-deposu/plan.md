# Plan: Java / Maven ve kurumsal paket deposu

- **Spec no:** 018 — [spec.md](spec.md)
- **Tarih:** 2026-08-14
- **Durum:** Taslak

---

## Yaklaşım

Dört hikâyenin üçü **mevcut kalıpların ikinci müşterisi**; yalnızca biri yeni
bir mekanizma gerektiriyor.

- **H2 (kurumsal depo)** — npm için spec 014'te kurulan yolun aynısı: adres
  ayardan, kimlik şifreli depodan, yapılandırma dosyası container'a kopyalanır,
  agent'a "bu adresle oynama" denir. `buildNPMRC`'nin yanına `buildSettingsXML`.
- **H3 (sertifika)** — spec 017'nin çözdüğü sertifika Java tarafına da
  tanıtılır. Yeni bir ayar veya yeni bir kaynak yok; var olan sertifikanın
  Java'nın anladığı biçime çevrilmesi.
- **H4 (süre sınırı)** — kayıt defterine bir ayar, iki yapılandırma dosyasına
  birer satır.
- **H1 (Java'nın kendisi)** — asıl yeni iş ve tamamı imajda.

Riskli iki bilinmeyen **plan yazılmadan ölçüldü** (aşağıda), yani planda
tahmine dayanan bir adım yok.

### Ölçülenler

**1. Temurin 17 ve 25, Debian bookworm deposunda var.** Taban imaj
`node:24.13.0-slim` = bookworm; Adoptium'un apt deposu `temurin-17-jdk` ve
`temurin-25-jdk` paketlerini hem arm64 hem amd64 için sunuyor.

**2. Java'ya sertifika tanıtmak root gerektirmiyor.** Ölçüm (uid 10001 ile):

```
sertifika tanıtılmadan : SSLHandshakeException — PKIX path building failed
cacerts kopyalanıp     : "Certificate was added to keystore"
truststore gösterilince: HTTP 401   ← TLS geçti
```

Ve gerçek bir Maven çözümlemesi, HTTPS üzerinden kurumsal aynadan indirdi:

```
[INFO] Downloaded from kurumsal: https://nexus.local/repository/maven-public/...
```

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
| --- | --- | --- | --- |
| İki JDK tek imajda, `MAVEN_OPTS` ile truststore | Etiket ekseni değişmez; ölçüldü, çalışıyor | İmaj büyür | **Seçildi** |
| Java sürümü başına ayrı imaj etiketi | Yalnızca gereken sürüm çekilir | Etiket eksenini ikiye çıkarır; beş yeri (etiket kurgusu, Makefile, CI, doğrulama, arayüz) etkiler. Seçim yüzeyi iki yolda da aynı olduğu için karar geri alınabilir | Elendi |
| `JAVA_TOOL_OPTIONS` ile truststore | Her JVM'i kapsar (`java -jar` dahil) | Her JVM başlangıcında stderr'e `Picked up JAVA_TOOL_OPTIONS…` basar ve bu, **agent'ın okuduğu her çıktıya** düşer. Spec 014'te `always-auth` tam bu sebeple kaldırılmıştı | Elendi |
| Sertifikayı imajın `cacerts`'ine derleme anında gömmek | Koşuda iş yok | Sertifika imaja gömülmez — imaj herkese dağıtılıyor (spec 017) | Elendi |
| Maven'ı apt'tan kurmak | Tek satır | Debian'ın `maven` paketi `default-jdk`'yı sürükler; imaja üçüncü bir JDK girer | Elendi |
| `settings.xml`'in tamamını ayardan almak | Egzotik kurulumlar ifade edilebilir | İçindeki parola düz metin olarak ayarlarda dururdu; spec 014'ün "adres ayarda, sır kimlikte" sınırını delerdi | Elendi (spec: Belirsizlikler) |

---

## Veri Modeli

**Yeni tablo veya migration yok.** İki yeni ayar mevcut `settings` tablosuna
satır olarak girer. Kimlik, spec 014'ün `nexus` kaydını **paylaşır** — yeni bir
kimlik türü eklenmez.

Geri alma: commit'i geri almak yeterli; ayar satırları kalırsa zararsızdır.

## Arayüzler

### Go tipleri

```go
// internal/settings/registry.go — İKİ yeni ayar, mevcut GroupPackages altında
const (
    KeyMavenRegistry = "packages.maven_registry" // KindText, Optional
    KeyPackagesTimeout = "packages.timeout_seconds" // KindInt, 5–600, varsayılan 60
)
```

```go
// internal/runner/runner.go — PackageRegistry genişler
type PackageRegistry struct {
    NPMRegistry   string
    MavenRegistry string   // YENİ
    Username      string
    Token         string
    TimeoutSec    int      // YENİ — npm ve Maven ORTAK
}
func (p PackageRegistry) MavenEnabled() bool { return p.MavenRegistry != "" }
```

```go
// internal/runner/config.go — buildNPMRC'nin yanına
//
// settings.xml YAPI olarak üretilir, metin şablonuyla değil: parola XML'e
// giriyor ve kaçırılmamış bir karakter dosyayı sessizce bozardı.
const settingsXMLPath = "/home/agent/.m2/settings.xml"
func buildSettingsXML(p PackageRegistry) []byte
```

### Container'daki yerleşim

```text
/opt/java/17, /opt/java/25     → mimariden bağımsız sabit bağlar
/opt/maven                     → sürümü sabitlenmiş Maven
JAVA_HOME                      → /opt/java/25 (varsayılan)
/home/agent/.m2/settings.xml   → 0600, kurumsal ayna + kimlik
/home/agent/kurumsal-ca.pem    → spec 017'den, zaten var
/home/agent/kurumsal-truststore.jks → entrypoint üretir (yalnızca sertifika varsa)
MAVEN_OPTS                     → truststore'u gösterir (yalnızca sertifika varsa)
```

### HTTP API

**Yeni uç yok.** İki ayar mevcut ayar uçlarından yönetilir.

### Frontend

**Yeni bileşen yok.** İki ayar da mevcut tiplerden (`text`, `int`), yani
`RuntimeSettings` onları kendiliğinden çizer ve arama kendiliğinden bulur.
Paket deposu bölümünün açıklaması güncellenir.

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
| --- | --- |
| `runner/Dockerfile` | iki Temurin JDK, sabit bağlar, sürümü sabit Maven (H1) |
| `runner/entrypoint.sh` | sertifika varsa truststore üretimi + `MAVEN_OPTS` (H3) |
| `runner/entrypoint-test.sh` | yeni — truststore üretiminin kabuk testi |
| `backend/internal/settings/registry.go` | iki yeni ayar (H2, H4) |
| `backend/internal/runner/runner.go` | `PackageRegistry` iki yeni alan |
| `backend/internal/runner/config.go` | `buildSettingsXML`, npm süre satırları, agent talimatı (H2, H4) |
| `backend/internal/runner/config_test.go` | üretilen dosyaların testleri |
| `backend/internal/runner/paketdeposu_test.go` | uçtan uca yapılandırma testi genişler |
| `backend/cmd/server/main.go` | iki ayar `PackageRegistry`'ye bağlanır |
| `README.md`, `docs/kurumsal-ag.md` | Java/Maven bölümü |

Frontend'de değişen kod dosyası yok.

## Yeniden Kullanılacak Mevcut Kod

- `runner.BuildConfigFiles` + `ConfigFile` — settings.xml aynı yoldan gider;
  yeni bir kopyalama mekanizması yazılmıyor.
- `buildNPMRC` — `buildSettingsXML` onun şeklini birebir izler: adres yoksa
  `nil` döner ve dosya hiç yazılmaz.
- `packageSection` (agent talimatı) — npm için yazılan "adresi değiştirme"
  bölümü Maven cümleleriyle genişler; ikinci bir talimat bloğu açılmaz.
- `credentials.KindNexus` — **aynı kimlik**; yeni tür eklenmiyor.
- `runner.CACertPath` (spec 017) — truststore onu okur.
- `entrypoint.sh`'ın git kimlik kalıbı — koşullu kurulum ve `die` yardımcıları
  hazır.
- `settings.Validate` `KindInt` aralık denetimi — süre sınırı için yeterli.

## Riskler

| Risk | Etki | Önlem |
| --- | --- | --- |
| İmaj ~600 MB büyür | Kapalı ağda transfer ve depolama | Bilinçli takas; JDK'lar ayrı derleme aşamasında, opencode katmanını geçersizleştirmez. Boyut ölçülüp belgelenir |
| Temurin yol adı mimariye göre değişir (`-amd64` / `-arm64`) | Talimat bir mimaride yanlış | `/opt/java/17` ve `/opt/java/25` sabit bağları; talimat ve `JAVA_HOME` yalnızca bunları bilir. İki mimaride de sınanır |
| Maven'ın süre sınırı özellik adı sürüme göre değişiyor (`aether.connector.http.*` / `maven.wagon.http.*`) | Ayar sessizce etkisiz kalır | T ile **ölçülerek** doğrulanır: ulaşılamayan bir adrese karşı süre tutulur. Ölçülmeden "çalışıyor" denmez |
| `MAVEN_OPTS` yalnızca `mvn`/`mvnw`'yi kapsar | Agent `java -jar` çalıştırırsa sertifika tanınmaz | Bilinçli: alternatifi her çıktıya gürültü basmaktı. Sınır **belgelenir**; gerekirse ayrı bir spec |
| Truststore, JDK 17'nin `cacerts`'inden türetilirse 25'te eski CA kümesi taşınır | Genel sertifikalar bayat kalır | Varsayılan JDK'nın (`/opt/java/25`) `cacerts`'i temel alınır |
| `settings.xml` 0600 ama `.m2` dizini yoksa yazma düşer | Container hiç açılmaz | Dizin `ConfigFile` yolundan türetilir; kopyalama tar akışı ara dizinleri oluşturur (ölçülür) |
| npm süre sınırı, yavaş hatta büyük paketi keser | Çalışan kurulum kırılır | Ayar; varsayılan 60 sn ama 600'e kadar yükseltilebilir. Kurumsal adres tanımlı değilken hiç yazılmaz |

## Test Stratejisi

- **Birim (Go):**
  - `buildSettingsXML`: adres yokken `nil`; adres varken `mirrorOf=*`; kimlik
    yokken `<server>` **yok**; parolada XML özel karakteri kaçırılıyor
  - `buildNPMRC`: süre satırları yalnızca adres tanımlıyken
  - `PackageRegistry.MavenEnabled`
  - `packageSection`: Maven tanımlıyken talimatta adres ve "değiştirme" cümlesi
- **Kabuk:** `entrypoint` truststore üretimi — sertifika varken üretir, yokken
  hiç dokunmaz, `MAVEN_OPTS` yalnızca üretildiğinde tanımlanır
- **Elle doğrulama (kurumsal Nexus provasına karşı):**
  1. `java -version` ve `mvn -version` çalışır; iki sürüm de `/opt/java/*`'de
  2. Talimatla ikinci JDK'ya geçilir ve `java -version` onu gösterir
  3. Sertifika tanımlıyken `mvn` kurumsal aynadan indirir (bugün ölçüldü)
  4. Sertifika tanımsızken PKIX hatası alınır — karşıtlık gösterilir
  5. **Gerçek agent koşusu**: `mybatis-spring-boot-starter` derlenir
  6. Ulaşılamayan adreste koşu, süre sınırı kadar bekler — **ölçülür**
  7. Adres tanımsızken davranış bugünküyle aynı
- **Statik:** `gofmt`, `go vet`, `shellcheck`, `go test`.

## Uygulama Sırası

En riskli parça imaj: hem en uzun geri bildirim döngüsü hem diğer her şeyin
önkoşulu.

1. **İmaj: iki JDK + Maven + sabit bağlar** — `make runner`, sonra container'da
   `java -version` / `mvn -version` (H1)
2. **Entrypoint: truststore + `MAVEN_OPTS`** — sertifikalı ve sertifikasız iki
   koşulda kabuk testi (H3)
3. **Ayarlar: Maven adresi + süre sınırı** — arayüz kendiliğinden çizer (H2, H4)
4. **`buildSettingsXML` + npm süre satırları + agent talimatı** (H2, H4)
5. **Uçtan uca**: prova Nexus'una karşı gerçek agent koşusu (H1–H4)
6. **Belgeler ve boyut ölçümü**
