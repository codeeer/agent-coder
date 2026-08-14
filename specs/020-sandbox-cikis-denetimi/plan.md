# Plan: Sandbox çıkış denetimi

- **Spec no:** 020 — [spec.md](spec.md)
- **Tarih:** 2026-08-14
- **Durum:** Taslak

---

## Yaklaşım

Zorlama **ağdan** gelir, ayardan değil. Proxy tanımlıyken runner container'ı,
internete rotası olmayan ikinci bir Docker network'üne (`internal: true`) alınır.
O network'te NAT yoktur; dışarı çıkmanın tek yolu, aynı network'e bağlı olan
backend'in içindeki **çıkış kapısıdır**. Böylece proxy ayarını okumayan bir
istemcinin (ölçümde JVM) doğrudan çıkışı "no route to host" ile düşer — H1'in
"atlanamaz" şartı buradan gelir.

Kapı, backend sürecinin içinde ikinci bir HTTP dinleyicisidir. `CONNECT` satırındaki
host'a bakar, whitelist'e sorar, izinliyse kurumsal proxy'ye devreder. **TLS
açılmaz** — kapı yalnızca hedefin adını görür, gövdeyi görmez. Ret durumunda hem
`slog` ile loglanır hem de o çalıştırmanın olay akışına uyarı yazılır.

**Kapı backend ile birlikte açılır, ayarla birlikte değil.** Dinleyici her zaman
ayaktadır; ayar boşken sadece kimse ona konuşmaz. Ayarı çalışma anında girilen bir
şeye bağlı başlatıp durdurmak, "ayar kaydedildi ama kapı henüz ayakta değil" gibi
yarış durumları üretirdi. Denetimin açık olup olmadığına **her çalıştırmanın
başında** bakılır: proxy doluysa container kısıtlı network'e alınır, boşsa bugünkü
network'te doğar.

Kapı hangi çalıştırmanın konuştuğunu **kaynak IP** ile bilir: container başladıktan
sonra backend onun kısıtlı network'teki IP'sini öğrenir ve kapıya kaydeder, iş
bitince siler. Bu sayede whitelist çalıştırmaya özel olur (LLM provider ve
repository adresleri o çalıştırmadan gelir) ve ret uyarısı doğru çalıştırmaya
yazılır. İstemci tarafında ek kimlik ayarı gerekmez.

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
|------------|------|------|-------|
| Yalnızca `HTTP_PROXY` ortam değişkenleri | En basit; hiçbir aracı bozmaz | **Ölçüldü: atlanabiliyor** — Koşu B'de 26 bağlantının 5'i proxy'yi atladı | Elendi — "denetim" iddiası atlanabilir mekanizmayla kurulamaz |
| Kısıtlı network + backend içinde kapı | Yeni imaj yok; ayarları canlı okur; reddi doğrudan çalıştırma olayına yazar; TLS açmaz | Backend yeniden başlarsa süren çalıştırmanın ağı kesilir | **Seçildi** |
| Kısıtlı network + ayrı Squid container'ı | Savaşta denenmiş yazılım | Whitelist DB'de ve çalışma anında değişiyor → config üretimi + reload; çalıştırma başına atıf zor; yeni imaj ve yaşam döngüsü | Elendi — işletme yükü kazancından büyük |
| Container içinde `iptables` kuralı | Container'a özel, ağ topolojisi değişmez | `NET_ADMIN` capability gerektirir; sandbox'a ağ yönetme yetkisi vermek güvenlik sınırını **zayıflatır** | Elendi — güvenlik için güvenlik gevşetilmez |
| Çalıştırma başına ayrı network | En sıkı izolasyon | Her çalıştırmada network yarat/sil; Docker'da network sayısı sınırlı; temizlik yeni bir sızıntı kaynağı | Elendi — kazanç, kısıtlı tek network'e göre marjinal |
| Kurumsal proxy'ye bırakmak | Ürün hiç kod yazmaz | Ayarlardaki whitelist hiçbir şey yapmayan bir kutu olurdu | Elendi (spec Belirsizlikler) |
| Whitelist'i `NO_PROXY`'ye yazmak | Kod yazmadan "liste" görüntüsü | **Anlamı ters:** `NO_PROXY` proxy'ye uğramayacakları listeler — izinlileri kapının dışına çıkarırdı | Elendi — hatalı zihin modeli |

---

## Veri Modeli

**Migration yok.** `settings` tablosu anahtar/değer; yeni ayar kayıt defterine bir
satır eklemekle gelir ([000003_calistirma.sql:12](../../backend/internal/db/migrations/000003_calistirma.sql)).

Geri alma: ayarlar boşaltıldığında (veya kayıt defteri satırı kaldırıldığında)
sistem eski davranışına döner — kalıcı şema izi yok.

## Arayüzler

### Go tipleri

```go
// backend/internal/hostlist/hostlist.go
// Whitelist metnini ayrıştırır ve host eşleştirir.
// AYRI PAKET: hem ayar doğrulaması hem kapı aynı kuralı kullanmalı; iki kopya
// er geç ayrışır ve "ayarda kabul edilen satır kapıda tutmuyor" olur.
// certfmt paketiyle aynı gerekçe.

type Pattern struct{ ... }

func Parse(text string) ([]Pattern, error) // boş satır ve # yorumu atlanır
func Match(patterns []Pattern, host string) bool
```

```go
// backend/internal/netgate/gate.go

type Gate struct{ ... }

// New + Serve AÇILIŞTA çağrılır ve port her hâlükârda dinlenir — ayar dolu olsun
// olmasın. Sebebi: proxy ayarı çalışma anında giriliyor; dinleyiciyi ayara bağlı
// başlatıp durdurmak, "ayar girildi ama kapı henüz açılmadı" gibi yarış
// durumları üretirdi. Boş bir port dinlemek bedavadır; kimse konuşmazsa kapı
// hiçbir şey yapmaz.
func New(listen string) *Gate
func (g *Gate) Serve(ctx context.Context) error
func (g *Gate) Address() string // runner'a HTTP_PROXY olarak verilecek URL

// Register, bir çalıştırmayı kaynak IP'sine bağlar. Kayıtsız IP'den gelen istek
// reddedilir — yani ayar boşken kapı fiilen kapalıdır.
func (g *Gate) Register(ip string, r Run)
func (g *Gate) Unregister(ip string)

type Run struct {
    ID       string
    Upstream string             // kurumsal proxy — kayıt anında DONDURULUR
    Allow    []hostlist.Pattern // boşsa tüm host'lar izinli (spec)
    OnDeny   func(host string)  // çalıştırma olay akışına uyarı yazar
}

```

```go
// backend/internal/runner/runner.go — mevcut arayüze eklenir
// Runner paketi netgate'i TİP olarak bilmez; küçük bir arayüz üzerinden konuşur.
type EgressGate interface {
    ProxyURL() string
    Register(ip string, runID string, allow []string, onDeny func(host string))
    Unregister(ip string)
}
```

```go
// backend/internal/runner/sandbox/docker.go — Spec'e eklenir
type Spec struct {
    ...
    Network string // zaten var; artık çalıştırma başına seçiliyor
}
// Container'a eklenir: başlatma sonrası IP
func (c *Container) IPAddress(ctx context.Context, network string) (string, error)
```

### HTTP API

| Metot | Yol | Gövde | Yanıt |
|-------|-----|-------|-------|
| GET | `/api/network/egress` | — | `{proxy:{source,host}, alwaysAllowed:{engine[],providers[],repositories[],registries[]}}` |

Yeni **kaydetme** ucu yok — proxy ve whitelist mevcut `PUT /api/settings/{key}`
ile yazılır. Bu uç yalnızca H4'ün "her zaman izinli adresleri göster" ihtiyacını
karşılar; spec 017'deki `GET /api/network/ca` ile aynı gerekçe: çözülmüş durumu
göstermek.

### Frontend tipleri

```ts
// frontend/src/lib/types.ts
export type SettingKind = "int" | "bool" | "text" | "certificate" | "host_list";

export type EgressInfo = {
  proxy: { source: "setting" | "env" | "none"; host: string };
  alwaysAllowed: {
    engine: string[]; providers: string[];
    repositories: string[]; registries: string[];
  };
};
```

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
|-------|------------|
| `backend/internal/hostlist/` | **yeni** — whitelist ayrıştırma + eşleştirme (+ test) |
| `backend/internal/netgate/` | **yeni** — CONNECT/HTTP kapısı, çalıştırma kaydı, upstream devri (+ test) |
| `backend/internal/settings/registry.go` | `KindHostList`; `network.proxy_url`, `network.allowed_hosts` |
| `backend/internal/settings/service.go` | `Validate`'e `KindHostList` case'i; `validateText`'e proxy URL case'i |
| `backend/internal/runner/runner.go` | `EgressGate` arayüzü; `Request`'e izin listesi ve kapı adresi |
| `backend/internal/runner/opencode/runner.go` | `applyProxy` hedefi kapı olur; opencode'un zorunlu host'ları; kayıt aç/kapa |
| `backend/internal/runner/sandbox/docker.go` | Container IP okuma; network çalıştırma başına |
| `backend/internal/runs/manager.go` | Ayarları `func` olarak taşıma (mevcut `Limits` kalıbı) |
| `backend/internal/httpapi/network.go` | `GET /api/network/egress` |
| `backend/internal/httpapi/router.go` | Rota kaydı |
| `backend/cmd/server/main.go` | Kapıyı başlat; proxy çözücüsü (ayar > env) |
| `backend/internal/config/config.go` | Kısıtlı network adı, kapı portu ve URL'i |
| `deploy/docker-compose.yml` | `restricted` network (`internal: true`); backend iki network'te |
| `frontend/src/lib/types.ts`, `api.ts` | `host_list` kind, `EgressInfo`, uç çağrısı |
| `frontend/src/components/settings/RuntimeSettings.tsx` | Çok satırlı koşulu + `host_list` dalı |
| `frontend/src/components/settings/EgressStatus.tsx` | **yeni** — "her zaman izinli" listesi (`CACertStatus` emsali) |
| `frontend/src/app/settings/page.tsx` | Kurumsal ağ sekmesine yeni bölüm |
| `docs/kurumsal-ag.md`, `README.md`, `.env.example` | Yeni ayarlar, davranış, bilinen sınırlar |

## Yeniden Kullanılacak Mevcut Kod

- **`applyProxy`** ([runner/opencode/runner.go:439](../../backend/internal/runner/opencode/runner.go)) —
  büyük/küçük harfli `HTTP_PROXY` ailesi, `NODE_USE_ENV_PROXY`, JVM `-D` özellikleri
  ve `NO_PROXY=localhost` zaten yazılı ve **testli**. Yeniden yazılmaz; yalnızca
  hedefi kurumsal proxy yerine kapı olur.
- **`cacert.Resolver`** ([main.go:114](../../backend/cmd/server/main.go)) — "ayar
  kazanır, ortam değişkeni yedekte kalır" çözümü. Proxy adresi için aynı kalıp.
- **`certfmt` paketi** — ayar doğrulamasının ayrı pakete çıkarılma emsali;
  `hostlist` onu örnek alır.
- **`runner.Event` + `emit`** — ret uyarıları çalıştırma olay akışına buradan
  yazılır; yeni bir olay kanalı açılmaz.
- **`settings` `Limits`/`Packages` closure kalıbı** ([runs/manager.go:29](../../backend/internal/runs/manager.go)) —
  ayarların çalışma anında okunması; yeniden başlatma gerekmemesi buradan geliyor.
- **`sandbox` etiketleri** (`agent-coder.managed`, `agent-coder.run-id`) ve
  `CleanupOrphans` — kapı kayıtlarının temizliği aynı yaşam döngüsüne bağlanır.
- **`Textarea`** ([primitives.tsx:567](../../frontend/src/components/ui/primitives.tsx))
  ve `CertificateField`'ın tam genişlik yerleşimi — whitelist alanı için.
- **`CACertStatus.tsx`** — "çözülmüş durumu göster" bileşeninin emsali.
- **`scripts/sizinti-analizi/`** — doğrulamanın ölçüm düzeneği; yeniden yazılmaz.

---

## Riskler

| Risk | Etki | Önlem |
|------|------|-------|
| Kapı, tüm çıkış trafiğinin darboğazı olur (Koşu B'de 190 MB JDK indirildi) | Bellek/CPU | Gövde tamponlanmaz; `io.Copy` ile akıtılır. Ölçülür |
| Backend yeniden başlarsa süren çalıştırmanın ağı kesilir | Çalıştırma düşer | Kabul edildi (spec). Belgelenir |
| Kaynak IP'nin yeniden kullanılması yanlış çalıştırmaya atıf yapar | Yanlış whitelist / yanlış uyarı | Kayıt container silinirken **her yolda** kapatılır (mevcut `defer ct.Remove` kalıbı) |
| Kısıtlı network'te backend↔runner DNS çözümlemesi bozulur | Çalıştırma hiç başlamaz | Backend de aynı network'te; `waitReady` zaten sağlık kontrolü yapıyor — erken kırılır, sessiz kalmaz |
| `java -jar` doğrudan çalıştırıldığında proxy almaz | O çağrı düşer | Bilinen ve **kabul edilmiş** sınır (sertifikada da aynı boşluk var). Belgeye ve olay akışına yazılır |
| Kurumsal proxy ayakta değil | Her çalıştırma düşer | Kapı, upstream'e bağlanamazsa anlaşılır hata döner; "bilinmeyen hata" yok |
| Whitelist ayarı loglara düşer ve okunmaz olur | Log gürültüsü | Ayar log'u özetlenir — sertifikada ölçülmüş aynı sorun ([settings.go:105](../../backend/internal/httpapi/settings.go)) |
| Kapı yalnızca host'a bakıyor; izinli host üzerinden sızdırma mümkün | Yanlış güven | Spec'te kapsam dışı; arayüzde ve belgede açıkça yazılır |
| `github.com` otomatik izinli — opencode oradan yardımcı program indiriyor, ama bu GitHub'ın tamamını açıyor | Whitelist sanılandan geniş | Bu sürümde kabul edildi ve **yazılı** (spec → "Sonraya bırakılan"). Çözümü ayrı iş: programı imaja gömüp bu satırı kaldırmak |
| Kapının portu açılamaz (port meşgul) | Denetim hiç çalışamaz | Dinleyici **açılışta** kurulur ve bind hatası backend'i başlatmaz. Ayar sonradan girildiğinde kapının hazır olmadığı bir an olmaz |
| Ayar çalıştırma sürerken değiştirilir | Süren çalıştırma tutarsız davranır | Proxy adresi ve whitelist, çalıştırma **kayıt anında dondurulur**; süren iş başladığı kurallarla biter |

## Test Stratejisi

- **Birim — `hostlist`:** tam domain, `*.` subdomain, apex'in wildcard'a girmemesi,
  büyük/küçük harf, boş satır ve `#` yorumu, geçersiz satırlar (URL, port, boşluk,
  ASCII dışı), boş liste.
- **Birim — `netgate`:** izinli CONNECT geçer, izinsiz reddedilir ve `OnDeny`
  çağrılır; kayıtsız IP reddedilir; boş `Allow` her host'u geçirir; upstream'e
  devir; upstream ölüyken anlaşılır hata. Sahte upstream proxy ile, gerçek ağ yok.
- **Birim — `settings`:** proxy URL doğrulaması (şema, host, kimlik gömülü →
  reddedilir ve hata mesajı secret'ı **tekrarlamaz**); `KindHostList` doğrulaması.
- **Birim — `applyProxy`:** kapı adresi verildiğinde değişkenlerin kapıyı
  göstermesi; proxy boşken hiçbir değişken yazılmaması.
- **Entegrasyon:** kapı + sahte upstream ayağa kaldırılıp gerçek `http.Client` ile
  uçtan uca izin/ret.
- **Elle doğrulama — H1'in kanıtı (ölçüm):** proxy tanımlıyken Koşu B tekrarlanır;
  `scripts/sizinti-analizi/yakala.sh` ile köprüde tcpdump alınır. **Runner IP'sinden
  kapı dışında hiçbir hedefe SYN olmamalı** — önceki ölçümde bu sayı 5'ti
  (`repo.maven.apache.org`), beklenen yeni değer **0**.
- **Elle doğrulama — H3:** whitelist'ten `archive.apache.org` çıkarılıp aynı görev
  koşulur; çalıştırma ekranında ret uyarısı görünmeli, backend 5xx dönmemeli.
- **Elle doğrulama — kapalı davranış:** proxy boşken runner eski network'te,
  `applyProxy` hiçbir değişken yazmıyor, davranış bugünküyle aynı.
- **Elle doğrulama — H2/H4 arayüz:** iki tema; geçersiz satırın reddi; "her zaman
  izinli" listesinin gerçek yapılandırmadan geldiği; proxy boşken whitelist'in
  etkisiz olduğunun söylenmesi.
- **Statik:** `make test`, `make lint-backend`, `npx tsc --noEmit`, `npx eslint .`

## Uygulama Sırası

Riskli ve belirsiz olan başta: kapının kendisi ağ ve arayüz olmadan test edilebilir.

1. **`hostlist`** — saf mantık, bağımlılıksız, tam test.
2. **`netgate`** — kapı + sahte upstream ile testler. Ürüne henüz bağlanmaz.
3. **Ayarlar** — kayıt defteri, doğrulama, testler. (1'i kullanır.)
4. **Network ve container yolu** — compose'da kısıtlı network, `Spec` network
   seçimi, container IP okuma, `applyProxy` hedefi, kapı kaydı. **İlk uçtan uca
   çalıştırma burada alınır.**
5. **Ret olayları** — olay akışı + `slog`; tekrarlı ret'in akışı boğmaması.
6. **`GET /api/network/egress`** ve arayüz: `host_list` dalı + "her zaman izinli"
   bileşeni.
7. **Ölçüm** — tcpdump ile bypass sayısının 0 olduğunun kanıtlanması ve rapora
   yazılması. Bu adım atlanırsa H1 doğrulanmamış sayılır.
8. **Belgeler** — `docs/kurumsal-ag.md`, `README.md`, `.env.example`.
