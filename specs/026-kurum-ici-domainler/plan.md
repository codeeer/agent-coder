# Plan: Kurum içi domain'ler

- **Spec no:** 026 — [spec.md](spec.md)
- **Tarih:** 2026-08-17
- **Durum:** Taslak

---

## Yaklaşım

Çıkış kapısı bugün her bağlantıyı koşulsuz kurumsal proxy'ye veriyor. Kapıya
ikinci bir liste veriliyor: hedef bu listeyle eşleşiyorsa kapı **kendisi**
bağlanıyor, eşleşmiyorsa bugünkü yol işliyor.

Karar noktası iki yerde: CONNECT tüneli ve düz HTTP iletimi. İkisi de aynı
eşleştirmeyi kullanıyor, dolayısıyla kural tek yerde tanımlı.

Agent ortamına hiçbir şey söylenmiyor — ne yeni ortam değişkeni, ne ağ
değişikliği. Runner hâlâ yalnızca kapıyı görüyor. Bu, spec 020'nin ölçümle
aldığı kararın korunması demek: "denetim ayardan değil ağdan geliyor".

İzin kontrolü **önce** çalışmaya devam ediyor ve hiç değişmiyor. Yeni liste
yalnızca izin verilmiş bir bağlantının hangi yoldan gideceğini belirliyor.

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
| ---------- | ---- | ---- | ----- |
| Kararı kapıda vermek | Yalıtım korunur; karar atlatılamaz; tek yer | Kapıya bir alan daha | **Seçildi** |
| Runner'a `NO_PROXY` yazmak | Kod değişikliği çok az | **Çalışmaz**: agent ağı `internal: true`, doğrudan bağlantı container'dan çıkamaz. Ayrıca spec 020 ölçtü — ortam değişkeniyle verilen proxy atlanabiliyor | Elendi |
| Agent ağına iç ağ rotası vermek | Sıçrama azalır | Kapı tek çıkış olmaktan çıkar; spec 020'nin tüm güvencesi düşer | Elendi |
| Mevcut `hostlist.Match`'i yeniden kullanmak | Yeni fonksiyon yok | **Ters çalışır**: boş listeye `true` döner, yani liste boşken her hedef doğrudan gider ve kurumsal proxy tamamen devre dışı kalır | Elendi — bkz. `Listed` |

---

## Veri Modeli

**Migration yok.** Ayar, kayıt defterine yeni bir `Key` olarak giriyor; tablo
şeması değişmiyor.

Geri alma: ayar silinirse liste boşalır ve davranış bugünküne döner.

## Arayüzler

### Go tipleri

```go
// backend/internal/hostlist/hostlist.go

/*
Listed, host'un listede olup olmadığını söyler.

BOŞ LİSTE = HAYIR. `Match` ile arasındaki tek fark bu ve bilinçli: `Match`
bir İZİN listesi için yazıldı, orada boş liste "kısıt yok" demek. Bu fonksiyon
ise "bu host şu kümede mi" sorusunu yanıtlıyor; boş kümede hiçbir şey yok.

İkisi ayrı isimde duruyor çünkü aynı imzayı taşıyorlar: yanlış olanı çağırmak
derlenir ve sessizce ters davranır.
*/
func Listed(desenler []Pattern, host string) bool
```

```go
// backend/internal/netgate/netgate.go — Run'a eklenen alan

// Direct, kurumsal proxy'ye UĞRAMADAN gidilecek hedefler (spec 026).
//
// Allow'un TERSİ semantik: boşsa hiçbir hedef doğrudan gitmez, hepsi
// proxy'den geçer. Allow'da boş liste "kısıt yok" demekti.
Direct []hostlist.Pattern
```

```go
// backend/internal/runner/runner.go — EgressSpec'e eklenen alan

/*
 * InternalHosts, kurumun kendi domain'leri — HAM METİN, satır başına bir
 * domain (spec 026).
 *
 * Bu adreslere kapı kurumsal proxy'ye uğramadan bağlanır. İZİN VERMEZ:
 * çıkış izni `AllowedHosts`'un işi ve orada kalır.
 */
InternalHosts string
```

### HTTP API

Yeni uç yok. Ayar mevcut `GET /api/settings` ve `PUT /api/settings/{key}`
üzerinden gelir. `GET /api/network/egress` yanıtı bir alan kazanır:

| Alan | Anlamı |
| ---- | ------ |
| `internalHosts` | Proxy'ye uğramadan gidilecek domain'ler (çözülmüş liste) |

### Frontend tipleri

```ts
// frontend/src/lib/types.ts — EgressResponse'a eklenen alan
/** Kurumsal proxy'ye uğramadan gidilen domain'ler (spec 026). */
internalHosts: string[];
```

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
| ----- | ---------- |
| `backend/internal/hostlist/hostlist.go` | `Listed` eklenir; `Match`'in doc'una boş-liste farkı yazılır |
| `backend/internal/hostlist/hostlist_test.go` | `Listed` için boş liste ve desen testleri |
| `backend/internal/netgate/netgate.go` | `Run.Direct`; `tunel` ve `duzHTTP` yönlendirme dalı |
| `backend/internal/netgate/netgate_test.go` | yönlendirme testleri (aşağıda) |
| `backend/internal/runner/runner.go` | `EgressSpec.InternalHosts` |
| `backend/internal/runner/opencode/egress.go` | listeyi ayrıştırıp `netgate.Run`'a taşır; bozuk liste yutulur ve loglanır |
| `backend/cmd/server/main.go` | ayar closure'a bağlanır |
| `backend/internal/settings/registry.go` | yeni `Key` + tanım |
| `backend/internal/httpapi/egress.go` | durum yanıtına çözülmüş liste |
| `frontend/src/lib/types.ts` | yanıt tipine alan |
| `frontend/src/components/settings/EgressStatus.tsx` | listeyi gösterir |
| `README.md` / `docs/` | çıkış denetimi anlatılan yere yeni ayar |
| `specs/020-.../spec.md` | karar geçmişine tarihli madde |

## Yeniden Kullanılacak Mevcut Kod

- `internal/hostlist` — ayrıştırma, normalize etme ve desen eşleştirme zaten
  var. Yeni bir söz dizimi **yazılmıyor**; yalnızca boş-liste anlamı farklı
  olan ikinci bir sorgu fonksiyonu ekleniyor.
- `internal/settings` — `KindHostList` ve çok satırlı doğrulama mevcut;
  `httpapi/settings.go` zaten yazarken `hostlist.Parse` ile doğruluyor.
- `RuntimeSettings.tsx` — `host_list` render dalı var, ayar arayüzde
  **kendiliğinden** görünür; yeni bileşen yok.
- `netgate.Session` — dinleyici, oturum yaşam döngüsü ve `OnDeny` bildirimi
  olduğu gibi kalıyor.
- `opencode/egress.go` — `egressAllow`'un yanına ikinci bir kurucu ekleniyor,
  mevcut olan değişmiyor.

---

## Riskler

| Risk | Etki | Önlem |
| ---- | ---- | ----- |
| Boş listede `Match` semantiği kullanılır | **Kurumsal proxy tamamen devre dışı kalır** — özelliğin en ağır hatası | Ayrı isimli `Listed`; boş liste testi hem `hostlist` hem `netgate` seviyesinde |
| İzin kontrolü yönlendirmenin arkasına düşer | İzinsiz adres doğrudan gider | Sıra testle kilitlenir: izinsiz + iç domain → reddedilir |
| Doğrudan bağlantı düşünce proxy denenir | Yöneticinin kaçındığı yoldan credential geçer | Geri düşme yok; test upstream'in HİÇ aranmadığını doğrular |
| Geniş desen (`*.com.tr`) yazılır | Trafik kurumsal denetimin dışına çıkar, kimse fark etmez | Yardım metni riski açıkça söyler (spec H5); ürün doğrulama eklemez — izin listesinde de yok, tutarsızlık olurdu |
| Backend'in iç ağa rotası yok sayılır | İç adres kapıdan da çözülemez | Kapı backend'de çalışıyor ve backend host'un rotalarını kullanıyor; iç adrese ulaşamıyorsa hata açık şekilde çalıştırmaya yazılır |
| Bozuk liste çalıştırmayı düşürür | Ayar hatası yüzünden iş kaybı | Ayrıştırma hatası yutulur, loglanır ve liste boş sayılır — bugünkü davranış sürer (spec H3) |

## Test Stratejisi

- **Birim — `hostlist`:** `Listed` boş listede `false`; `Match` boş listede
  `true` (mevcut davranış kilitli kalır). Aynı desen kümesiyle ikisinin
  farklı cevap verdiği bir vaka.
- **Birim — `netgate` yönlendirme:** Sahte bir upstream proxy ve sahte bir
  hedef sunucu ayağa kaldırılır; hangisinin bağlantı aldığı ölçülür.
  - `Direct` boş → upstream alır
  - hedef `Direct` ile eşleşir → **hedef** alır, upstream hiç aranmaz
  - hedef eşleşmez → upstream alır
  - `Allow` reddediyor + hedef `Direct`'te → 403, ikisi de aranmaz
  - doğrudan bağlantı kurulamıyor → 502 ve upstream hiç aranmaz
  - aynı senaryolar hem CONNECT hem düz HTTP için
- **Birim — `opencode/egress.go`:** ham metin desenlere çevrilir; bozuk metin
  çalıştırmayı düşürmez, boş liste üretir.
- **Elle doğrulama:**
  1. Ayar boşken bir çalıştırma → bugünkü davranış, trafik proxy'den geçer
  2. Listeye depo domain'i yazılır → klonlama proxy'ye uğramaz
  3. Aynı çalıştırmada `github.com` → proxy'den geçmeye devam eder
  4. İzin listesi doluyken listede olmayan bir iç domain → reddedilir
  5. Ayarlar ekranında liste görünür, açıklama riski söyler
  6. Çıkış durumu ekranında çözülmüş liste görünür
- **Kapılar:** `go test ./...`, `go vet ./...`, `gofmt`, `npx tsc --noEmit`,
  `npx eslint .`

## Uygulama Sırası

Riskli parça başta: yanlış yapılırsa kurumsal proxy'yi sessizce kapatan şey
eşleştirme semantiği.

1. **`hostlist.Listed` + testleri.** Saf fonksiyon, bağımlılığı yok. Boş
   listenin `Match`'ten farklı davrandığı burada kilitlenir.
2. **`netgate.Run.Direct` + yönlendirme + testleri.** Özelliğin tamamı
   aslında bu adımda çalışır hale gelir; sahte upstream/hedef ile hangi
   tarafın bağlantı aldığı ölçülür. Ayar henüz yok, alan boş geçilir —
   **davranış bu adımda değişmez.**
3. **Ayar zinciri.** `EgressSpec.InternalHosts`, `egress.go` ayrıştırması,
   kayıt defteri maddesi, `main.go` closure'ı. Liste ilk kez gerçekten
   dolabilir hale gelir.
4. **Görünürlük.** Çıkış durumu ucu ve ekranı; ayar açıklamasında geniş
   desen uyarısı.
5. **Belge.** README/docs notu ve spec 020'nin karar geçmişine tarihli madde.
6. **Elle doğrulama.**

Commit bölünmesi: (a) adım 1-2, (b) adım 3-4, (c) adım 5.
