# Plan: Bitbucket grubundan toplu proje ekleme

- **Spec:** [spec.md](spec.md) · Onaylandı
- **Tarih:** 2026-08-14

---

## Seçilen yaklaşım

İki fazlı, iki uçlu bir akış:

```
1. ÖNİZLEME   grup adresi + erişim  →  Bitbucket REST listesi  →  durum etiketli repo listesi
                                        (hızlı: yalnızca sayfalı liste çağrısı)

2. İÇE AKTARMA  seçilenler  →  paralel `git ls-remote --symref`  →  kayıt  →  satır satır sonuç akışı
                                (doğrulama + varsayılan branch AYNI işlemde)
```

### Planın merkezindeki karar: varsayılan branch git'ten okunur, Bitbucket'tan değil

Bitbucket'ın `…/repos/{slug}/branches/default` ucu yerine `git ls-remote --symref <url> HEAD`
kullanılacak. Üç gerekçe:

1. **Zaten koşacak.** Spec erişim sınamasını zorunlu kıldı ve sınama `ls-remote`.
   `--symref` eklemek ek maliyet getirmiyor; branch bilgisi bedavaya geliyor.
2. **Ölçemediğimiz en riskli bağımlılığı ortadan kaldırıyor.** Varsayılan branch
   ucu Data Center sürümleri arasında değişen uçların başında geliyor ve elimizde
   test edilecek bir sunucu yok. `ls-remote` ise git protokolü — sunucu sürümünden
   bağımsız.
3. **Doğru cevabı o veriyor.** Klonlama anında geçerli olan şey git'in gördüğü
   HEAD'dir; API'nin söylediği değil.

Böylece Bitbucket REST'e olan bağımlılık **tek bir uca** iniyor: grubun repository
listesi. O uç 4.x'ten beri aynı ve yalnızca `slug`, `name`, `links.clone[]`
alanlarına dokunacağız.

---

## Elenen alternatifler

| Alternatif | Neden elendi |
| --- | --- |
| Varsayılan branch'i Bitbucket'ın kendi ucundan okumak | Sürüm farkı riski en yüksek uç ve ölçemiyoruz. `ls-remote` zaten koşuyor ve cevabı veriyor. |
| Bitbucket çağrılarını tarayıcıdan yapmak | Erişim anahtarı (PAT) tarayıcıya inerdi. Anahtar backend'de şifreli duruyor, orada kalmalı. |
| Tek uçta hem listeleyip hem eklemek | Spec seçim aşaması istiyor (H2). Kullanıcı görmeden onaylayamaz. |
| Mevcut `POST /api/projects`'i grup adresi de kabul edecek şekilde genişletmek | Kısmi başarı semantiği farklı: bugünkü uç ya 201 ya 4xx dönüyor. "51'i eklendi, 3'ü olmadı" bu modele sığmıyor ve tek repo ekleyen kullanıcının hata mesajlarını bulanıklaştırırdı. |
| Grubu veritabanına varlık olarak yazıp senkron etmek | Spec kapsam dışı: kendiliğinden senkron yok, grup kalıcı katman değil. |
| İlerlemeyi SSE ile yayınlamak | `EventSource` yalnızca GET yapabiliyor; içe aktarma seçim listesini gövdede taşıyor. İş kaydı + ayrı SSE ucu çifti, tek istek ömrü kadar yaşayacak bir iş için gereksiz durum yaratırdı. |
| İlerleme yerine belirsiz bir bekleme göstergesi | Yüz repository'de kullanıcı ne kadar kaldığını göremezdi. Kabul kriteri "işin ilerlediğini görür" diyor. |
| `repo_url` üzerine unique index | Mevcut tek repository ekleme akışının davranışını değiştirirdi (bugün mükerrer adres kabul ediliyor) — spec kapsamı dışında bir karar. Tekilleştirme içe aktarma sırasında yapılır. |

---

## Yeniden kullanılacak mevcut kod

Bu bölüm doldurulmadan koda geçilmiyor (AGENTS.md). Aranan ve bulunanlar:

| Ne | Nerede | Nasıl kullanılacak |
| --- | --- | --- |
| Cloud / Server ayrımı | [validator.go:100](../../backend/internal/gitprovider/validator.go#L100) `bitbucketCloud` | H4'ün tamamı. Testli ve yaralı bir kod — yeniden yazılmayacak, çağrılacak. |
| Taban adres çözümü | [provider.go:97](../../backend/internal/gitprovider/provider.go#L97) `Provider.APIURL` | Grup adresinden çıkan taban ile karşılaştırma. |
| Erişim bilgisi çözümü | [projects.go:190](../../backend/internal/httpapi/projects.go#L190) `verifyRepoAccess` | Kimlik çözme kalıbı aynen; farkı, içe aktarmada **bir kez** çözülüp N repository'de kullanılması. |
| `ls-remote` sarmalayıcısı | [verify.go:46](../../backend/internal/projects/verify.go#L46) `Verifier.Verify` | Üzerine `--symref` okuyan kardeş bir metot eklenecek; askpass ve hata sınıflandırma paylaşılacak. |
| Parola sızdırmayan git çağrısı | `WriteAskpass`, `InjectUsername`, `classifyGitError` | Olduğu gibi. |
| Proje kaydı ve doğrulaması | `projects.Input.Normalize`, `Store.Create` | Olduğu gibi. |
| Adres okunurlaştırma | [repo-url.ts:24](../../frontend/src/components/projects/repo-url.ts#L24) `repoLabel` | Önizleme listesinde repository'yi göstermek için. |
| Ekran bileşenleri | `components/ui/primitives.tsx` | Yeni bileşen kitaplığı eklenmeyecek. |

---

## Veri modeli

**Migration yok.** Yeni tablo, yeni sütun, yeni enum değeri gerekmiyor. İçe
aktarılan her repository bugünkü `projects` satırının aynısı; onu elle
eklenmişten ayıran bir alan **bilinçli olarak** eklenmiyor (spec: "ayırt
edilmeye gerek kalmadan görünür").

---

## Yeni kod

### `internal/bitbucket` (yeni paket)

Bitbucket Data Center'ın okuma tarafı. `gitprovider`'a konmuyor: orası erişim
**tanımını** yönetiyor, burası o erişimle **veri okuyor**; ikisini aynı pakete
koymak sağlayıcı doğrulamasını depo listelemeye bağlardı.

```go
// GroupRef, bir grup adresinin çözülmüş hali.
type GroupRef struct {
    BaseURL string // https://sirket.com/bitbucket   (context path DAHİL)
    Key     string // ODEME  ·  kişiselde ~AHMET
}

// ParseGroupURL, tarayıcıdan kopyalanan grup adresini çözer.
func ParseGroupURL(raw string) (GroupRef, error)

type Repo struct {
    Slug      string
    Name      string
    CloneURL  string // links.clone içinden http olan, kullanıcı adı AYIKLANMIŞ
    Archived  bool   // kaynak bildirmiyorsa false
}

// ListRepos, grubun TÜM repository'lerini döner — sayfalama tükenene kadar.
func (c *Client) ListRepos(ctx context.Context, g GroupRef) ([]Repo, error)
```

Hatalar sınıflandırılır: `ErrNotGroupURL`, `ErrCloudAddress`, `ErrGroupNotFound`,
`ErrForbidden`, `ErrUnreachable`.

### `projects` paketine ekleme

```go
// DefaultBranch, deponun HEAD'inin gösterdiği branch'i döner ve aynı
// çağrıda erişimi sınar. Boş dönmez: okunamazsa hata döner.
func (v *Verifier) DefaultBranch(ctx context.Context, repoURL string, creds *Credentials) (string, error)

// ExistingRepoURLs, kayıtlı tüm depo adreslerini normalize edilmiş
// anahtarlarla döner — mükerrer denetimi için.
func (s *Store) ExistingRepoURLs(ctx context.Context) (map[string]uuid.UUID, error)
```

### HTTP uçları

```
POST /api/projects/import/preview
     { "groupUrl": "...", "gitProviderId": "..." }
  →  { "group": {"baseUrl","key"},
       "repos": [ {"slug","name","cloneUrl","archived","status"} ] }
     status: "new" | "already_registered"

POST /api/projects/import
     { "gitProviderId": "...", "repos": [ {"slug","name","cloneUrl"} ] }
  →  application/x-ndjson — her repository bitince BİR satır:
     {"slug":"odeme-api","result":"created","projectId":"..."}
     {"slug":"odeme-web","result":"failed","reason":"depo erişimi reddedildi"}
     {"summary":{"created":51,"skipped":9,"failed":3}}
```

Akış biçimi NDJSON: istek POST gövdesi taşıdığı için `EventSource` kullanılamıyor;
`fetch` + akış okuma hem POST'u hem ilerlemeyi karşılıyor.

---

## Tuzaklar

Kod okunurken çıkanlar — hepsi gerçek ve hepsi sessizce bozan cinsten:

- **`Input.Normalize()` boş branch'i sessizce `main` yapıyor**
  ([store.go](../../backend/internal/projects/store.go)). Spec ise "varsayılan
  branch uydurulmaz" diyor. İçe aktarma **asla boş branch geçirmemeli**; HEAD'i
  okunamayan repository eklenmez. Normalize'ın varsayılanına güvenmek, spec'in
  en sert kuralını sessizce çiğnerdi.

- **`Normalize()` adreste gömülü kullanıcı adını REDDEDİYOR.** Bitbucket'ın
  verdiği klonlama adresi çoğu kurulumda `https://ahmet@sunucu/scm/KEY/slug.git`
  biçiminde. Ayıklanmazsa **her içe aktarma tamamen başarısız olur** — üstelik
  sebebi "adres kullanıcı adı içermemeli" diyen, kullanıcının anlamayacağı bir
  mesajla.

- **Sayfalama atlanırsa hata başarı gibi görünür.** Varsayılan sayfa boyutu 25;
  tek çağrı yapan bir uygulama 60 repository'lik gruptan 25'ini alır ve "25 proje
  eklendi" der. Kimse hata görmez. `isLastPage` yanlış olduğu sürece
  `nextPageStart` ile devam edilecek.

- **Context path.** Kurumsal kurulumların çoğu kökte değil. Taban adres,
  `/projects/{KEY}` parçasından **öncesi**dir; sabit host varsaymak bu
  kurulumlarda çalışmaz.

- **Cloud denetimi grup ayrıştırmasından ÖNCE.** Bulut adresi de
  `/projects/…` içerebiliyor; sıra ters olursa kullanıcı H4'teki açık mesaj
  yerine anlamsız bir 404 alır.

- **Kişisel repository'ler** `/users/ahmet` yolundan gelir ve grup anahtarı
  `~AHMET`'tir.

---

## Riskler

| Risk | Karşılık |
| --- | --- |
| **Gerçek sunucuda ölçüm yok.** Varsayımlar belgeye dayanıyor. | Yalnızca sürümler arası değişmeyen alanlara (`slug`, `name`, `links.clone`) bağlanılacak. Hata mesajları kaynağın ham yanıtını **saklamayacak** — issue açan kişinin elinde iz kalmalı. |
| Sürüm farkı yüzünden liste hiç ayrıştırılamaz | Ayrıştırma hatası, "sunucu beklenmedik bir yanıt verdi" + ham gövdenin kısaltılmış hali ile bildirilir; sessiz boş liste dönmez. |
| Yüz repository'nin sınaması uzun sürer | Sınırlı eşzamanlılık (8) ve repository başına kendi süre sınırı. Bir repository'nin takılması diğerlerini durdurmaz. |
| İstemci zaman aşımı | NDJSON akışı ilk satırdan itibaren veri gönderdiği için bağlantı sessiz kalmıyor; istemci tarafında bu çağrının süresi ayrıca yükseltilecek. |
| Eşzamanlı iki içe aktarma aynı repository'yi ekler | Mükerrer denetimi kayıt anında da tekrarlanır; yarış hâlinde ikinci kayıt atlanır. |

---

## Test stratejisi

**Birim (Go):**

- `ParseGroupURL`: düz adres · sondaki eğik çizgi · context path · `/repos/x/browse`
  ekli adres · `/users/ahmet` · grup olmayan adres · bulut adresi
- `ListRepos`: `httptest` ile **iki sayfalık** yanıt — tek sayfa alan bir kod bu
  testte düşer
- Klonlama adresi seçimi: http/ssh karışık listeden http'nin seçilmesi, gömülü
  kullanıcı adının ayıklanması
- `DefaultBranch`: `ls-remote --symref` çıktısının ayrıştırılması, HEAD okunamama hâli
- Mükerrer normalizasyonu: büyük/küçük harf, sondaki `.git` ve `/`
- İçe aktarma: kısmi başarıda başarılıların kalması

**Sahte sunucu:** belgelenmiş yanıt biçimlerini veren bir `httptest` sunucusu.
**Bu sunucu kendi kodumuzu doğrular, Atlassian'ın gerçek yanıtını değil** —
sonuçlar "gerçek sunucuda doğrulandı" diye sunulmayacak (spec kararı).

**Arayüz:** saf mantık (durum etiketleri, seçim sayacı) kendi modülünde test
edilir; ekran `ui.md`'ye göre tarayıcıda iki temada doğrulanır.

**Kapı:** `make test` · `npx tsc --noEmit` · `npx eslint .` · `make lint-backend`.

---

## Uygulama sırası

Riskli parça başa (AGENTS.md):

1. `internal/bitbucket`: adres ayrıştırma + sayfalı listeleme, `httptest` ile.
   Arayüz ve veritabanı olmadan test edilebilir; en çok bilinmezlik burada.
2. `Verifier.DefaultBranch` — `--symref` okuma ve hata sınıflandırma.
3. `Store.ExistingRepoURLs` + normalizasyon.
4. Önizleme ucu.
5. İçe aktarma ucu: eşzamanlılık sınırı, NDJSON akışı, kısmi başarı.
6. Arayüz: "Gruptan içe aktar" akışı, seçim listesi, ilerleme, sonuç özeti.
7. Belgeler (`README`, `docs/`), `tasks.md` kapanışı.

---

## Sonraya bırakılan

- **Tek repository formuna grup adresi yapıştırılırsa yönlendirme.** Kullanıcının
  refleksi mevcut alana yapıştırmak olabilir; "bu bir grup adresi, içe aktarmayı
  kullanın" demek küçük ama spec'te olmayan bir ek. Ayrı iş.
- **GitHub organization ve Bitbucket Cloud workspace.** Aynı akış, farklı
  listeleme şeması. `bitbucket` paketinin sınırları buna göre dar tutuluyor ki
  sonradan kardeş bir paket eklenebilsin.
- **`repo_url` üzerine unique index.** Mevcut veri temiz (8 kayıt, 8 farklı
  adres), ama kısıt tek repository ekleme akışının davranışını da değiştirir.
