# Görevler: MCP Desteği

- **Spec no:** 011 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Uygulandı (Aşama 1–3)

---

## Aşama 1 — Agent'lara uzak MCP sunucuları

### Bağlantı yönetimi

- [x] T01 `internal/mcp/server.go` — `Server` tipi (gizli değer TAŞIMAZ),
      `Transport`, doğrulama, sentinel hatalar
- [x] T02 Migration `000008_mcp.sql` — `mcp_servers` + `agent_mcp_servers`
- [x] T03 `internal/mcp/store.go` — CRUD + `Reveal`; boş secret mevcut değeri korur
- [x] T04 `internal/mcp/client.go` — resmi Go SDK ile bağlan, `tools/list`
- [x] ~~T05 ayrı `validator.go`~~ — yazılmadı: `Client.ListTools`'un kendisi doğrulama;
      ayrı bir `Validator` tipi yalnızca tören olurdu
- [x] T06 Testler: `httptest` ile sahte MCP sunucusu + **sızıntı testi**

### Çalıştırmaya bağlama

- [x] T10 `runner.AgentSpec`'e MCP sunucuları
- [x] T11 `BuildConfigFiles` → `"mcp"` bloğu; her sunucuda açık `timeout`
- [x] T12 `BuildPermissions` → atanmamış sunucular için `deny`
- [x] T13 `buildEnv` → gizli değerler ortam değişkeniyle; config `{env:}` ile referans
- [x] T14 Motor hazır olduktan sonra **durum kontrolü**; bağlanamayan sunucu uyarı olur
- [x] T15 `runbuild.Builder` agent'ın sunucularını çözüp aktarır
- [x] T16 Ayar: `mcp.timeout_seconds`

### Arayüz

- [x] T20 `httpapi/mcpservers.go` + router + wiring
- [x] T21 Ayarlar'a "MCP sunucuları" bölümü; doğrulama sonucu ve araç listesi görünür
- [x] T22 Agent formunda MCP sunucusu seçimi
- [x] T23 Yerel (stdio) sunucuların desteklenmediği arayüzde yazar

### Doğrulama

- [x] T40 [plan.md](plan.md) doğrulama listesi 1–7
- [x] T41 **Gerçek bir MCP sunucusuyla uçtan uca**

## Aşama 2 — `mcp.call` düğümü

- [x] T50 `KindMCPCall` + `NodeConfig` alanları + `KindSpec`
- [x] T51 `MCPHandler`
- [x] T52 Arayüz: palet, düğüm görseli, alan paneli (araç **listeden** seçilir)
- [x] T53 Doğrulama: çıktı sonraki adıma geçer; yanlış araç kaydetme anında reddedilir

## Aşama 3 — Agent Coder MCP sunucusu

- [x] T60 `internal/mcpserver/` — akışları araç olarak açar
- [x] T61 `/mcp` ucu; adres anahtar niteliğinde
- [x] T62 README'ye istemci kurulumu
- [x] T63 Doğrulama: dış bir istemciden akış başlatılır

## Kapanış

- [x] T90 `AGENTS.md`, `plans/01`, `/nasil-calisir` diyagramı güncellenir

---

## Sıra ve gerekçesi

Bağlantı yönetimi (T01–T06) önce: çalıştırmaya bağlama adımlarının hepsi bir
sunucu tanımının var olmasına dayanıyor.

T04 (Go MCP istemcisi) bu aşamada **doğrulama için** yazılıyor, ama asıl
kazancı Aşama 2'de: `mcp.call` düğümü aynı istemciyi kullanacak. Yani Aşama 1
bitince Aşama 2'nin en pahalı parçası zaten ödenmiş oluyor.

T14 listenin en kolay atlanacak ama en değerli maddesi. Motor, bağlanamayan bir
MCP sunucusunu sessizce yok sayıyor; bu kontrol olmazsa arıza ancak "agent neden
aptallaştı" sorusuyla, günler sonra fark edilir.

---

### T40/T41 — Aşama 1 doğrulama sonuçları

Gerçek, herkese açık bir MCP sunucusuyla (`mcp.deepwiki.com`) ölçüldü.

| # | Adım | Sonuç |
|---|------|-------|
| 1 | Sunucu kaydetme doğrulamadan geçer | ✓ 3 araç okundu: `ask_question`, `read_wiki_contents`, `read_wiki_structure` |
| — | Geçersiz ad reddedilir | ✓ `my.server` → 400 |
| — | Ulaşılamayan sunucu reddedilir | ✓ 422 "Sunucuya bağlanılamadı" |
| 2 | Anahtar veritabanında şifreli | ✓ düz metin eşleşme 0; API yanıtında 0; logda 0 |
| 3 | Atanmış agent aracı kullanır | ✓ `araç çalıştı: deepwiki_read_wiki_structure` |
| 4 | Atanmamış agent kullanamaz | ✓ agent "mevcut araçlarım arasında deepwiki yok" dedi |
| 5 | **Bozuk sunucu sessiz kalmaz** | ✓ `warn: MCP sunucusu "deepwiki" bağlanamadı (failed) — araçları kullanılamayacak` |
| 6 | `make test`, `test-integration`, `lint` | ✓ 19 paket, lint temiz |
| 7 | Tema denetimi | ✓ 108 kontrol, 0 kalan, eşlik hatası yok |

### Ölçüm 1 — kanıt "çalıştı" demek değil

İlk uçtan uca denemede agent doğru cevabı verdi ve bunu kanıt saymaya hazırdım.
Ama model `go-sdk` deposunu **eğitim verisinden de** biliyor olabilirdi: çıktının
doğru olması aracın çağrıldığını göstermez.

Ayırt edici kanıt olay akışında: `araç çalıştı: deepwiki_read_wiki_structure`.
Kontrol deneyi de yapıldı — sunucu atanmamış bir agent aynı görevi alınca
"böyle bir araç yok" dedi.

**Ders:** doğru görünen bir çıktı, mekanizmanın çalıştığının kanıtı değildir.
Ayırt edici gözlem aranmalı.

### Ölçüm 2 — tahmin edilmeyen yetki kuralı

Yetki listesine "atanmamış her MCP aracı yasak" anlamında toptan bir `*_*: deny`
kuralı yazmıştım. Sonra fark ettim: çalıştırma motorunda **ilk eşleşenin mi son
eşleşenin mi kazandığını bilmiyorum**. Yanlış tahmin iki uçtan birine düşerdi —
ya bütün araçlar kapanır ya hiçbiri.

Kural kaldırıldı. Erişim zaten yapılandırmayla sınırlı: o dosyayı biz üretiyoruz
ve içine yalnızca atanmış sunucular giriyor. Sıralama semantiği ölçüldüğünde
beyaz liste kurulabilir; [spec.md](spec.md) açık uçlarda yazıyor.

**Ders:** doğrulanmamış bir davranışa yaslanan güvenlik kuralı, güvenlik
sağlamaz — yanlış bir güven duygusu verir.

---

### T53 — Aşama 2 doğrulama sonuçları

| # | Adım | Sonuç |
|---|------|-------|
| 1 | Yanlış araç adı **kaydetme anında** reddedilir | ✓ `"deepwiki" sunucusunda "olmayan_arac" adında bir araç yok` |
| 2 | Bozuk JSON argümanı reddedilir | ✓ `argümanlar geçerli bir JSON nesnesi değil` |
| 3 | Araç listeden seçilir | ✓ sunucu seçilince 3 araç açılır listede |
| 4 | MCP çıktısı sonraki adıma geçer | ✓ agent'ın talimatında DeepWiki metni göründü |
| 5 | Uçtan uca akış | ✓ `mcp.call → agent`, $0,0027 |
| 6 | Testler + tema | ✓ 19 paket, 100 kontrol 0 kalan |

### Ölçüm 3 — argüman şablonu sırası

Argümanlar bir JSON nesnesi ve içinde `{{ steps.x.output }}` gibi şablonlar var.
Doğal olan sıra "önce şablonu çöz, sonra JSON'u ayrıştır" gibi görünüyor — ama
o sırayla, içinde **tırnak veya satır sonu** olan bir agent çıktısı JSON'u
bozardı. Üstelik her zaman değil: yalnızca belirli bir çıktı geldiğinde.

Sıra tersine kuruldu: önce JSON ayrıştırılır, sonra her string DEĞER şablondan
geçer (`renderDeep`, iç içe nesne ve dizilerde de). Böylece çıktının içeriği
JSON'un yapısını etkileyemiyor.

### Ölçüm 4 — kaydedilen anahtar geri alınamıyordu

Herkese açık test sunucusuna denemek için bir anahtar yazmıştım. Sonra sunucu
`Authentication is not allowed on the public endpoint` dedi ve anahtarı
**kaldırmanın yolu olmadığını** fark ettim: boş bırakmak "değiştirme" anlamına
geliyor, dolayısıyla kullanıcının tek seçeneği sunucuyu silip yeniden kurmak —
agent atamalarını da kaybederek.

`clearSecret` bayrağı eklendi (projede `clearProvider` ile aynı kalıp).

Not: o sunucu hatayı **normal bir metin sonucu** olarak döndürdü, protokol
düzeyinde hata olarak değil. Yani "araç hata döndürdü" kontrolü (`IsError`) bu
durumu yakalayamaz; akış hata metnini veri sanıp bir sonraki adıma taşır. Bunun
genel bir çözümü yok — araçların hatayı nasıl bildireceği kendilerine kalmış.

---

### T63 — Aşama 3 doğrulama sonuçları

Gerçek bir MCP istemcisiyle (resmi Go SDK, `internal/mcpserver/canli_test.go`)
ayakta duran sunucuya bağlanılarak ölçüldü.

| # | Adım | Sonuç |
|---|------|-------|
| 1 | Dış istemci bağlanıyor | ✓ el sıkışma tamam |
| 2 | Araçlar görünüyor | ✓ `akislari_listele`, `akis_calistir`, `calisma_durumu` |
| 3 | Akışlar listeleniyor | ✓ 6 çalıştırılabilir akış |
| 4 | **Akış gerçekten başlıyor** | ✓ `runId` döndü, durum `pending` |
| 5 | Durum sorgulanıyor | ✓ `running`, 2 adım |
| 6 | Çalışma arayüzde görünüyor | ✓ tetikleyici `mcp`, akış `succeeded` |
| 7 | Yanlış adres | ✓ 404 (var olmayanla aynı cevap) |
| 8 | Testler + tema | ✓ 20 paket, 82 kontrol 0 kalan |

### Ölçüm 5 — "session not found": handler istek başına kuruluyordu

İlk denemede dış istemci bağlanamadı: `sending "notifications/initialized":
session not found`.

Sebep: `Handler()` her çağrıldığında **yeni bir** HTTP handler üretiyordu. MCP
el sıkışması birden fazla isteğe yayılıyor ve oturum durumu handler'da duruyor;
her istekte yeni handler, her istekte yeni (boş) oturum tablosu demekti.

Handler artık `New` içinde bir kez kuruluyor. Oturum başına yeniden kurulan şey
handler değil, MCP *sunucusu* — o durum taşımıyor.

**Ders:** durum tutan bir bileşeni "her seferinde üret" diye yazmak, tek istekli
bir protokolde fark edilmez; çok istekli bir protokolde ilk denemede patlar.

### Not — "Nasıl çalışır" sayfası iki aşama geriden geldi

Aşama 1'de sayfaya MCP kutusu ve güvenlik notu eklenmişti; Aşama 2 ve 3'te
belgeler (`AGENTS.md`, `README`, spec) güncellendi ama **sayfa atlandı** ve
kullanıcı fark etti.

Sayfa artık MCP'ye ayrı bir bölüm ayırıyor: tek diyagramda iki yön —
dışarıya (agent karar verir / akış karar verir) ve içeriye (bizi araç olarak
kullanırlar). Ayrı çizilseydi aradaki simetri, yani aynı protokolün ters yönde
işlemesi, görünmezdi.

**Ders:** "belgeleri güncelle" maddesi kapanış görevi olarak yazıldığında
sayfa gibi ürün içi anlatımlar listeden düşüyor. Bu sayfa da bir belgedir.

### Ölçüm 6 — eksiksiz `Record` yine işini yaptı

`TriggerKind`'a `mcp` eklendiğinde TypeScript hemen uyardı:
`Property 'mcp' is missing in type ... Record<TriggerKind, …>`.

Bu tam olarak daha önce canlı bir hatadan sonra kurulan koruma:
o zaman `jira` eklenmiş ve etiket eksik kaldığı için Jira'nın başlattığı
çalışmalar listede "elle" görünmüştü. Aynı hata bu sefer derleme zamanında
yakalandı.
