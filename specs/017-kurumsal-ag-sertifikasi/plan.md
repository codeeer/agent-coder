# Plan: Kurumsal ağ sertifikası

- **Spec no:** 017 — [spec.md](spec.md)
- **Tarih:** 2026-08-14
- **Durum:** Taslak

---

## Yaklaşım

Sertifika **bir ayar** olur ve çalışma ortamına **bir dosya olarak kopyalanır**.
İki cümlenin ikisi de mevcut kalıpların üstüne oturuyor:

- Spec 016'da "denetim ayarın **tipinden** gelir" kuralı kuruldu. Kayıt defterine
  yeni bir tip (`KindPEM`) ve tek bir tanım eklemek, arayüzde çok satırlı alanı,
  kaydetme akışını, sıfırlamayı ve **aramayı** bedava getiriyor. Ekran kodu bu
  ayarın adını bilmiyor.
- `sandbox.copyFiles` zaten tar akışıyla container'a `Uid/Gid 10001` sahipliğinde
  dosya yazıyor (`.npmrc`, `opencode.json`, agent tanımları oradan gidiyor).
  Sertifika **bir `ConfigFile` daha**. Ölçüldü: `$HOME` altına konan PEM'e üç env
  değişkeni gösterildiğinde node, git ve curl'ün üçü de kurumsal adrese TLS
  hatası almadan ulaşıyor.

Bu, **bind mount'u tamamen ortadan kaldırıyor** — `RUNNER_EXTRA_CA_CERT` ile
verilen dosya da artık okunup aynı yoldan kopyalanıyor. Böylece eski ayar
uzak Docker host'ta da çalışır hâle geliyor; bugün çalışmıyor.

Backend'in kendi çağrıları (H5) ayrı bir problem: kodda **yedi ayrı**
`http.Client` var, ortak bir yerden gelmiyorlar. Hepsine tek bir taşıyıcı
enjekte edilir; taşıyıcı geçerli sertifikayı ayardan okur ve **değiştiğinde
kendini yeniler**, böylece yeniden başlatma gerekmez.

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
| --- | --- | --- | --- |
| Sertifikayı `ConfigFile` olarak kopyalamak | Uzak Docker host'ta çalışır, yeniden başlatma yok, mevcut kopyalama yolu kullanılır | — | **Seçildi** |
| Bugünkü bind mount'u korumak | Değişiklik yok | Uzak Docker host'ta çalışmıyor (README'nin kendi üretim tavsiyesi); değiştirmek `make restart` istiyor | Elendi |
| Ayarı `KindText` olarak eklemek | Yeni tip yok | PEM çok satırlı; tek satırlık girdi kutusuna sığmaz, doğrulanmaz | Elendi |
| Sertifikayı şifreli kimlik deposunda tutmak | Mevcut kart kalıbı | Kök sertifika sır değil; maskelenir, görüntülenemez ve H2'nin "sahibini göster" kriteri imkânsızlaşır | Elendi |
| Sertifika için ayrı tablo | Şema temiz | Ayarlar tablosu zaten metin tutuyor; yeni tablo, kayıt defteri kalıbını ve aramayı boşa çıkarırdı | Elendi |
| `http.DefaultTransport`'u global olarak değiştirmek | Yedi çağrı yerine dokunulmaz | Global durum değiştirme; ayar değişince güvenli yenileme yapılamaz; testlerde sızıntı | Elendi |
| Yalnızca PEM kabul edip kullanıcıya `openssl` komutu göstermek | `certfmt` hiç yazılmaz | Kurumsal ekipler ikili ve zincirli dosyaları aynı uzantıyla veriyor; kullanıcı elindekinin hangisi olduğunu bilmiyor | Elendi |
| PKCS#7 için hazır bir kütüphane eklemek | Yazılacak kod yok | Standart kütüphane dışı bir bağımlılık; ihtiyaç duyulan şey SignedData içinden sertifika kümesini almak — dar ve sabit bir parça | Elendi (bkz. Riskler) |
| Dosyayı tarayıcıda çevirmek | Sunucuya uç eklenmez | Aynı çevirme mantığının ikinci kopyası; tarayıcıda ASN.1 ayrıştırmak ve Go tarafıyla aynı sonucu vermesini garanti etmek pahalı | Elendi |
| Backend imajının sistem güven deposuna yazmak (`update-ca-certificates`) | Her Go çağrısı kendiliğinden kapsanır | Açılışta root gerekir, imaj değişir ve **çalışma anında güncellenemez** (H5 kriteri) | Elendi |

---

## Veri Modeli

**Yeni tablo veya migration yok.** Sertifika mevcut `settings` tablosunda bir
satır olarak durur (`network.corporate_ca`). Değer metindir ve varsayılandan
sapan değerler zaten orada tutuluyor.

PEM ~2KB; kolon metin olduğu için sınır sorunu yok. Sertifika **sır değildir**,
şifrelenmez.

Geri alma stratejisi: commit'i geri almak yeterli. Ayar satırı kalırsa
zararsızdır — bilinmeyen anahtar okunmaz.

## Arayüzler

### Go tipleri

```go
// internal/settings/registry.go — YENİ tip
//
// Tipin adı BİÇİM DEĞİL, ŞEY: PEM bir yazılış biçimi, sertifika ise ayarın
// ne olduğu. Saklanan değer her zaman normalleştirilmiş PEM'dir ama tip adı
// buna bağlanmaz — yarın başka bir biçim kabul edilirse tip adı yalan olmaz.
const KindCertificate Kind = "certificate"

// Kayıt defterine tek tanım:
//   Key: "network.corporate_ca", Group: GroupNetwork, Kind: KindCertificate, Optional: true
const GroupNetwork = "network"   // GroupLabels: "Kurumsal ağ"
```

```go
// internal/settings/service.go — Validate'e KindCertificate dalı
// PEM blokları çözülür, her biri x509 ile ayrıştırılır.
// Hiç sertifika yoksa veya ayrıştırma düşerse ErrInvalidValue.
```

```go
// internal/certfmt — YENİ (saf)
//
// NE GELİRSE PEM'E ÇEVİRİR. Tüketicilerin hepsi (NODE_EXTRA_CA_CERTS,
// GIT_SSL_CAINFO, CURL_CA_BUNDLE, Go güven havuzu) YALNIZCA PEM okuyor;
// bu yüzden normalleştirme bir kolaylık değil zorunluluk.
//
// Sırayla denenir: PEM → çıplak base64 → DER → PKCS#7.
// Sertifika DIŞINDAKİ bloklar (özel anahtar dahil) atılır.
func ToPEM(raw []byte) (string, error)
```

```go
// internal/tlstrust — YENİ paket
//
// Geçerli sertifikayı ayardan okur, değiştiğinde taşıyıcıyı yeniler.
// Pool sağlayıcıyı çağrı başına okur; ayar bellekte tutulduğu için ucuz.
type Trust struct{ ... }

func New(pem func() string) *Trust
// RoundTripper, backend'in TÜM giden çağrılarında kullanılacak taşıyıcı.
func (t *Trust) RoundTripper() http.RoundTripper
// Client, verilen zaman aşımıyla hazır istemci.
func (t *Trust) Client(timeout time.Duration) *http.Client
```

```go
// internal/certinfo — YENİ (küçük, saf)
//
// Ekranda gösterilecek bilgi SERTİFİKANIN KENDİSİNDEN okunur (spec: ölçülmeyen
// gösterilmez).
type Info struct {
    Subject   string `json:"subject"`
    Issuer    string `json:"issuer"`
    NotAfter  time.Time `json:"notAfter"`
    Expired   bool   `json:"expired"`
}
func Parse(pem string) ([]Info, error)
```

```go
// internal/runner — sertifika çalışma ortamına dosya olarak gider
type Request struct {
    ...
    // CACert, kurumsal kök sertifikanın PEM içeriği. Boşsa hiçbir şey yazılmaz.
    CACert string
}
const caCertPath = "/home/agent/kurumsal-ca.pem"
```

### HTTP API

```text
GET  /api/network/ca            → { source: "settings"|"env"|"none",
                                    certificates: [ {subject, issuer, notAfter, expired} ] }

POST /api/network/ca/normalize  → { "data": "<dosya baytlarının base64'ü>" }
                                → { pem: "...", certificates: [ ... ] }
```

**Sertifikayı kaydeden yeni bir uç yok.** Kaydetme mevcut ayar ucundan olur
(`PUT /api/settings/network.corporate_ca`).

`normalize` ucu **saklamaz, çevirir**: kullanıcı dosya seçince arayüz baytları
buraya gönderir, dönen PEM metin alanına düşer, kullanıcı onu **görür** ve
normal "Kaydet" akışıyla kaydeder (spec H1: dosya seçmek tek başına
kaydetmez). Böylece ikili dosya için ayrı bir yazma yolu, ayrı doğrulama ve
ayrı hata biçimi çıkmıyor.

Boyut sınırı: 64KB. Sertifika zinciri bunun çok altında; sınır, ayarlar
tablosuna büyük gövde yazılmasını da engelliyor.

### Frontend tipleri

```ts
// lib/types.ts
export type SettingKind = "int" | "bool" | "text" | "certificate";   // genişletme

export interface CACertInfo {
  source: "settings" | "env" | "none";
  certificates: { subject: string; issuer: string; notAfter: string; expired: boolean }[];
}
```

```tsx
// components/ui/primitives.tsx — mevcut Textarea kullanılır, yeni kalıp YOK.
// components/settings/CACertStatus.tsx — YENİ: kaynak + çözülen bilgi şeridi.
```

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
| --- | --- |
| `backend/internal/settings/registry.go` | `KindCertificate`, `GroupNetwork`, `GroupLabels` girdisi, tek ayar tanımı |
| `backend/internal/settings/service.go` | `Validate`'e `KindCertificate` dalı |
| `backend/internal/certfmt/certfmt.go` | yeni — PEM / base64 / DER / PKCS#7 → PEM (saf) |
| `backend/internal/certfmt/certfmt_test.go` | yeni — dört biçim, özel anahtarın atılması, çöp girdi |
| `backend/internal/certinfo/certinfo.go` | yeni — PEM → `Info` (saf) |
| `backend/internal/certinfo/certinfo_test.go` | yeni — geçerli, bozuk, çoklu, süresi dolmuş |
| `backend/internal/tlstrust/tlstrust.go` | yeni — yenilenebilir taşıyıcı |
| `backend/internal/tlstrust/tlstrust_test.go` | yeni — ayar değişince yeni havuz |
| `backend/internal/llm/client.go` | istemci taşıyıcıdan gelir |
| `backend/internal/gitprovider/validator.go` | aynı |
| `backend/internal/credentials/validator.go` | aynı |
| `backend/internal/integrations/jira/comment.go` | aynı |
| `backend/internal/integrations/github/pr.go` | aynı |
| `backend/internal/mcp/client.go` | `authTransport.base` → taşıyıcı |
| `backend/internal/runner/opencode/client.go` | aynı |
| `backend/internal/runner/config.go` | `CACert` doluysa `ConfigFile` üretilir |
| `backend/internal/runner/opencode/runner.go` | üç env değişkeni (`CURL_CA_BUNDLE` **yeni**), bind mount yerine dosya |
| `backend/internal/runner/sandbox/docker.go` | `caBind` ve `ExtraCACert` kaldırılır |
| `backend/internal/httpapi/router.go` | `GET /api/network/ca`, `POST /api/network/ca/normalize` |
| `backend/internal/httpapi/network.go` | yeni handler |
| `backend/cmd/server/main.go` | sertifika çözümleyici (ayar → env dosyası), taşıyıcı kurulumu |
| `deploy/docker-compose.yml` | runner bind'i kaldırılır; backend'in okuduğu bind kalır |
| `frontend/src/lib/types.ts` | `"pem"` ve `CACertInfo` |
| `frontend/src/lib/api.ts` | `network.ca()` |
| `frontend/src/components/settings/RuntimeSettings.tsx` | `certificate` dalı → `Textarea` + dosya seçme, tam genişlik |
| `frontend/src/components/settings/CACertStatus.tsx` | yeni |
| `frontend/src/app/settings/page.tsx` | yeni **Kurumsal ağ** bölümü |
| `README.md`, `docs/kurumsal-ag.md`, `.env.example` | yöntem değişikliği |

## Yeniden Kullanılacak Mevcut Kod

Bu işin büyük kısmı **zaten yazılmış**:

- `sandbox.copyFiles` — tar akışıyla dosya kopyalama, `Uid/Gid 10001`. Sertifika
  için yeni bir mekanizma **yazılmıyor**; `ConfigFile` listesine bir eleman
  ekleniyor.
- `runner.BuildConfigFiles` — `.npmrc` için kurulan dallanma kalıbının aynısı
  (`buildNPMRC` → `ConfigFile`). Sertifika `buildCACert` ile aynı şekli izler.
- `settings.Validate` + `Definition.Optional` — "boş = kapalı" davranışı
  `KeyNPMRegistry`'de zaten var, aynısı kullanılır.
- `RuntimeSettings` + spec 016'nın tipten gelen denetimi — `bool` için açılan
  dal, `pem` için ikinci kez açılıyor. Arama (`setting-search.ts`) ve
  kaydetme akışı **hiç değişmiyor**; "sertifika" araması kendiliğinden çalışır
  (H3).
- `Textarea` (`primitives.tsx`) — hazır; yeni kalıp eklenmiyor.
- `Panel`, `Badge`, `Notice` — durum şeridi için.
- `paketdeposu_test.go` — kurumsal yapılandırmanın uçtan uca sınandığı test
  kalıbı; sertifika testi aynı kalıbı izler.

## Riskler

| Risk | Etki | Önlem |
| --- | --- | --- |
| Bind mount kaldırılınca mevcut kurulumların sertifikası devre dışı kalır | Kurumsal kurulumlar güncellemede kırılır | `RUNNER_EXTRA_CA_CERT` **okunmaya devam eder**: dosya okunup aynı yoldan kopyalanır. Compose'da backend'in okuduğu bind korunur. Testle sınanır |
| Yedi `http.Client`'tan biri atlanır | O yol kurumsal ağda sessizce düşer | Değişiklikten sonra `grep -rn "http.Client{"` ile sayılır; taşıyıcısız kalan olmamalı |
| Ayar değişince taşıyıcı yenilenmez | H5'in "yeniden başlatma yok" kriteri karşılanmaz | Taşıyıcı geçerli PEM'i çağrı başına okur, değiştiyse havuzu yeniden kurar; birim testle sınanır |
| Çok satırlı alan ayar satırının denetim hizasını bozar | Spec 016'da ölçülen hiza kırılır | `pem` dalı satırı **tam genişliğe** geçirir (denetim sütununa sıkıştırılmaz); dokuz sayı alanının hizası `getBoundingClientRect` ile yeniden ölçülür |
| PEM'in `TrimSpace`'i içeriği bozar | Sertifika geçersiz olur | Yalnızca baş/son boşluk kırpılıyor; blok içi satır sonları korunur. Testle sınanır |
| Süresi dolmuş sertifika reddedilir | Kurum hâlâ o sertifikayı kullanıyor olabilir | Kabul edilir, ekranda süresinin dolduğu yazılır (spec: Hata durumları) |
| Taşıyıcı testlerde gerçek ağa çıkar | Testler kırılgan olur | `tlstrust` testi yalnızca havuz kurulumunu sınar; ağ çağrısı yapmaz |
| **PKCS#7 için standart kütüphanede ayrıştırıcı yok** | Zincirli dosya kabul edilemez ya da yeni bağımlılık girer | `encoding/asn1` ile **yalnızca** SignedData içindeki sertifika kümesi çıkarılır — imza doğrulanmaz, içerik yorumlanmaz, dar bir okuma. Sınama uydurma veriyle değil, `openssl crl2pkcs7` ile üretilmiş **gerçek** dosyalarla yapılır. Ayrıştırma beklenenden kırılgan çıkarsa küçük bir bağımlılığa dönülür ve **gerekçesi `tasks.md` Notlar'a yazılır** |
| Biçim tanıma yanlış dalı seçer | Geçerli sertifika reddedilir | Sıra denenir ve **ilk başarılı** olan kabul edilir; hiçbiri tutmazsa hata. Dört biçimin dördü de testte |
| Seçilen dosyada özel anahtar da var | Anahtar veritabanına yazılır | `certfmt` yalnızca sertifika bloklarını alır; diğer her blok atılır. Testle sınanır (spec: Hata durumları) |

## Test Stratejisi

- **Birim (Go):**
  - `certfmt`: PEM, çıplak base64, DER ve PKCS#7 girdileri aynı PEM'i verir;
    özel anahtar içeren dosyadan yalnızca sertifika alınır; çöp girdi hata
    verir. Test verileri **openssl ile üretilir**, elle yazılmaz
  - `certinfo`: geçerli tek sertifika, kök+ara zincir, bozuk metin, süresi
    dolmuş sertifika
  - `settings.Validate`: `KindCertificate` geçerli/geçersiz; boş değer
    `Optional` ile geçerli
  - `tlstrust`: PEM değişince havuzun yenilendiği; boş PEM'de sistem havuzunun
    korunduğu
  - `runner.BuildConfigFiles`: `CACert` doluysa dosya üretilir (doğru yol, mod),
    boşsa hiç üretilmez
  - `runner.buildEnv`: üç env değişkeni yalnızca sertifika varken
- **Entegrasyon:** `GET /api/network/ca` — ayardan, env'den ve hiçbirinden
  gelen üç durum; `POST /api/network/ca/normalize` — dört biçim ve boyut sınırı
- **Elle doğrulama (kurumsal Nexus provasıyla, iki temada):**
  1. Sertifika **arayüzden** yapıştırılır, kaydedilir
  2. Ayarlarda "sertifika" araması ayarı bulur
  3. Sahibi/imzalayanı/bitiş tarihi ekranda görünür ve gerçek sertifikayla
     eşleşir
  4. Bozuk metin reddedilir, önceki değer korunur
  5. Çalışma ortamında node, git ve **curl** kurumsal adrese ulaşır
  6. **Yeniden başlatmadan** sertifika değiştirilir ve yeni değer geçerli olur
  7. Ayar boşaltılır, `RUNNER_EXTRA_CA_CERT` doluyken hâlâ çalışır ve ekran
     kaynağın env olduğunu söyler
  8. Denetim hizası ve iki tema yeniden ölçülür
- **Statik:** `go vet`, `npx tsc --noEmit`, `npx eslint .` temiz.

## Uygulama Sırası

Riskli parça **arayüz değil, mevcut kurulumları bozmadan mekanizmayı
değiştirmek**. Önce o doğrulanır.

1. **`certfmt` + `certinfo`** — saf, testleriyle yeşile alınır. Biçim
   dönüştürme en belirsiz parça (PKCS#7); ekrana hiç bağlanmadan bitirilir
2. **`KindCertificate` + kayıt defteri tanımı + `Validate`** — backend'de ayar
   var, arayüz henüz yok; `curl` ile kaydedilip okunur
3. **Sertifika çözümleyici + `ConfigFile` + üç env değişkeni** — bind mount
   kaldırılır. Buraya kadar arayüz hiç değişmedi, yani kurumsal ağ davranışı
   tek başına doğrulanabilir (H4)
4. **`tlstrust` + yedi istemci** — backend'in kendi çağrıları (H5)
5. **`GET /api/network/ca` + `POST /api/network/ca/normalize`** (H2, H3)
6. **Arayüz: Kurumsal ağ bölümü, `certificate` dalı, dosya seçme, durum
   şeridi** (H1, H2, H3)
7. **Belgeler** — README, `docs/kurumsal-ag.md`, `.env.example`
8. **Tam doğrulama** — Nexus provası, iki tema, üç genişlik, statik denetimler
