# Plan: Bağımlılık önbelleği

- **Spec no:** 027 — [spec.md](spec.md)
- **Durum:** Taslak
- **Kapattığı karar belgesi:** [plans/03](../../plans/03-bagimlilik-onbellegi-2026-08-14.md)

---

## En önemli karar: önbellek backend'in yönettiği adlandırılmış bir alandır

İki ekosistem için **iki adlandırılmış Docker volume**: `agent-coder-cache-maven`
ve `agent-coder-cache-npm`. Koşu container'ına sırasıyla
`/home/agent/.m2/repository` ve `/home/agent/.npm/_cacache` olarak bağlanır.

Bunun neden host bağlama yasağını (spec 017, [docker.go:164-175](../../backend/internal/runner/sandbox/docker.go#L164-L175))
delmediği [spec.md → Davranış Kuralları](spec.md) altında yazılı ve teknik
karşılığı şu: adlandırılmış volume **Docker daemon tarafında** yaşar. Host'ta
bir dosyanın bulunmasını gerektirmez, bu yüzden uzak bir Docker host'ta da
çalışır — yasağın gerekçesi burada doğmuyor.

Bu kararın doğrudan sonucu var ve planın yarısını belirliyor:

> **Backend, volume'ün içini göremez.** Docker host uzaktaysa `/var/lib/docker`
> backend'in dosya sisteminde yoktur. Bu yüzden **boyut ölçümü ve doğrulama,
> volume'ü bağlayan kısa ömürlü bir yardımcı container içinde** çalışır.
> Backend'in `os.Stat` ile volume'e bakması, tek makineli kurulumda çalışıp
> uzak host'ta sessizce yanlış cevap veren türden bir hata olurdu.

## Elenen alternatifler

| Yol | Neden elendi |
| --- | --- |
| **İmaja gömme** | `plans/03` ölçümle eledi: 569 MB'ın %3'ü uygulama bağımlılığı, gerisi projeye özgü araç zinciri. Tahmine dayalı liste, isabeti savrulan bir yatırım |
| **Tek volume + iki subpath mount** | `VolumeOptions.Subpath` Docker API 1.45+ istiyor; ürün API sürümünü müzakere ediyor ([docker.go:87](../../backend/internal/runner/sandbox/docker.go#L87)) ve eski daemon'da sessizce düşerdi. İki volume ayrıca ekosistem başına boyut ve temizleme veriyor |
| **Host dizinini bind mount** | Uzak Docker host'ta çalışmaz — spec 017'nin sertifikayı bağlamayı bırakma gerekçesinin aynısı |
| **Boyutu backend'de `du` ile ölçmek** | Yukarıdaki kutu: uzak host'ta volume backend'in dosya sisteminde yok |
| **Her koşuda bütünlük taraması** | Hızlandırmak için eklenen özelliği yavaşlatıcıya çevirirdi; tarama kullanıcının istediğinde çalıştırdığı bir eylem (spec H5) |

---

## Yeniden kullanılacak mevcut kod

Yazmadan önce arandı; aşağıdakiler **yeniden yazılmayacak**.

| İhtiyaç | Mevcut kod | Nasıl kullanılacak |
| --- | --- | --- |
| Aç/kapat ayarı | [settings/registry.go:54](../../backend/internal/settings/registry.go#L54) `Definition` + `KindBool` | Tek satır kayıt; emsal `KeyEngineLogPersist`. Migration gerekmiyor |
| Ayarın koşuya taşınması | `runs.Manager` içindeki `m.egress()`, `m.caCert()`, `m.projectDir()` kalıbı ([manager.go:505](../../backend/internal/runs/manager.go#L505)) | Aynı kalıpta `m.dependencyCache()` eklenir; `limits` yapısına bir erişimci |
| Docker istemcisi | [sandbox.Manager](../../backend/internal/runner/sandbox/docker.go#L79) `docker *client.Client` | Volume oluşturma/silme/ölçme aynı istemciyi kullanır; ikinci bir bağlantı açılmaz |
| Container yaratma | `sandbox.Manager.Create` + `sandbox.Spec` | `Spec`'e `Caches []CacheMount` eklenir; `HostConfig.Mounts` orada kurulur |
| Uyarıyı olay akışına düşürme | [manager.go:300](../../backend/internal/runs/manager.go#L300) `m.emit(ctx, runID, level, message)` | Önbellek bağlanamadığında `warn` seviyesiyle çağrılır |
| "Süren koşu var mı" | [manager.go:412](../../backend/internal/runs/manager.go#L412) `Manager.Active() int` | Temizleme ve doğrulama kapısı |
| Bakım ucu emsali | `Manager.PurgeEngineLogs` (spec 015) | Aynı biçimde bir bakım işlemi; saklama politikası olan özelliğin emsali |
| Ayar yanında durum kutusu | `components/settings/EgressStatus.tsx`, `CACertStatus.tsx` | `DependencyCacheStatus.tsx` aynı kalıpta yazılır; yeni bileşen kalıbı icat edilmez |
| Koşu ortam değişkenleri | `sandbox.Spec.Env` | Kilit için KULLANILMADI — ölçüm ezilebildiğini gösterdi (bkz. R6 ve "Ölçüm sonuçları") |

**Veri modeli değişmiyor. Migration yok.** Ayar `settings` tablosunda yaşıyor,
önbelleğin durumu Docker'da; ürünün saklaması gereken yeni bir kayıt yok.

---

## Tasarım

### Ayar (H2)

```go
// settings/registry.go — tek kayıt
{
    Key: KeyDependencyCache, Group: GroupRunner, Kind: KindBool,
    Label: "Bağımlılık önbelleği",
    Help:  "İndirilen Maven ve npm artefaktları koşular arasında saklanır…",
    Default: "false",
}
```

Varsayılan `false`; ayar okunamazsa da `false` — kapalı hâl bugünkü davranış
olduğu için güvenli taraf orası. Yeniden başlatma gerekmez: erişimci fonksiyon
her koşuda okunuyor (spec 003 H7 kalıbı).

### Koşuya bağlanma (H1)

```go
// sandbox.Spec'e eklenen alan
type CacheMount struct {
    Volume string // "agent-coder-cache-maven"
    Target string // "/home/agent/.m2/repository"
}
```

`Create` içinde `HostConfig.Mounts` kurulur. **Volume container'dan önce
`VolumeCreate` ile oluşturulur** (idempotent) — Docker'ın otomatik oluşturmasına
bırakılırsa hata yakalanamaz ve H1'in "önbelleksiz devam et" davranışı
kurulamaz.

### Hata hâlinde düşerek devam (spec Hata Durumları)

Volume oluşturulamaz veya bağlanamazsa **koşu durmaz**: `Caches` boş bırakılıp
container önbelleksiz yaratılır ve `m.emit(runID, "warn", …)` ile sebep olay
akışına düşer. Karar noktası tek yerde — `runs.Manager` — olacak; `sandbox`
paketi yalnızca hata döner, düşerek devam etme politikasını bilmez.

### Boyut, temizleme ve doğrulama (H3, H5)

Üçü de **yardımcı container** deseniyle çalışır. Deseni oluşturan dört kural,
üçü de güvenlik veya doğruluk gerekçeli:

1. **Her zaman runner imajı.** Volume'e ilk dokunan container, boş volume'ün
   sahipliğini belirler. Başka bir imaj (`alpine` gibi) ilk mount'u yaparsa
   sahiplik yeniden kayar ve R1 geri gelir. Runner imajı `USER agent` ile
   bitiyor ([Dockerfile:127](../../runner/Dockerfile#L127)), yani bu kural
   konvansiyona değil **imajın kendisine** dayanıyor — yardımcı container
   kendiliğinden uid 10001 olur.
2. **`ENTRYPOINT` geçersiz kılınır.** Runner imajının kendi giriş noktası var
   ([Dockerfile:189](../../runner/Dockerfile#L189)); yardımcı container onu
   çalıştırmamalı, container yaratılırken `Entrypoint` açıkça değiştirilir.
3. **Ağsız.** `NetworkMode: "none"`. Yardımcının işi yalnızca yerel dosyaları
   okumak; dışarıya çıkması için hiçbir sebep yok ve sandbox çıkış denetiminin
   (spec 020) etrafından dolaşan bir yol açılmamalı.
4. **Kısa ömürlü.** Tek komut, çıktı okunur, container silinir; koşu
   container'larıyla aynı temizleme disiplini.

| İşlem | Yardımcı container'da çalışan | Neden |
| --- | --- | --- |
| Boyut | `du -sb <hedef>` | `DiskUsage` API'si bazı depolama sürücülerinde `-1` dönüyor; `du` her sürücüde çalışır |
| Temizle | Volume `VolumeRemove` + yeniden `VolumeCreate` | İçeriği tek tek silmekten hızlı ve atomik; sahiplik imajdan yeniden kurulur |
| Doğrula (Maven) | Her `*.jar` yanındaki `*.jar.sha1` ile karşılaştırma | Maven indirdiği artefaktın özetini yerel depoya zaten yazıyor; ürün ikinci bir defter tutmaz |
| Doğrula (npm) | `npm cache verify` | npm'in kendi bütünlük denetimi; `_cacache` biçimini ürünün bilmesi gerekmez |

Özeti bulunmayan artefakt "bozuk" sayılmaz, **denetlenemedi** olarak ayrı
sayılır (spec H5 son kriteri).

#### Doğrulama inmekte olan artefaktı bozuk sanmamalı

Bu, doğrulamanın en zararlı hata biçimi: koşu sürerken tarama yapılırsa
yarım inmiş bir artefakt "bozuk" görünür ve **silinir** — yani doğrulama,
düzeltmesi gereken şeyi kendisi üretir. İki kat koruma konuyor:

1. **Kapı:** doğrulama yalnızca `Manager.Active() == 0` iken çalışır; süren
   koşu varsa `409` ve kaç koşunun sürdüğü söylenir (spec H5 dördüncü kriteri
   buna izin veriyor).
2. **Emniyet:** kapı bir yarışta aşılsa bile, özeti bulunmayan artefakt
   silinmez — "denetlenemedi" sayılır. Yani en kötü hâlde tarama eksik kalır,
   veri kaybetmez.

Silme yalnızca **özeti olan ve özetiyle uyuşmayan** artefakta uygulanır.

### HTTP uçları

İki ayrı volume olduğu için uçlar da **ekosistem başına**: tek bir "temizle",
kullanıcının yalnızca npm'i temizlemek istediği durumda 569 MB'lık Maven
birikimini de götürürdü.

```text
GET  /api/dependency-cache                  → {enabled, caches:[{id:"maven", sizeBytes|null, used}, {id:"npm", …}]}
POST /api/dependency-cache/{id}/clear       → 200 | 409 (süren koşu var)
POST /api/dependency-cache/{id}/verify      → {checked, mismatched, unverifiable, removed} | 409
```

`{id}` ∈ `maven | npm`; bilinmeyen kimlik `404`. `409` gövdesi kaç koşunun
sürdüğünü söyler — "şu an yapılamaz" tek başına kullanıcıyı ne zaman
deneyeceğini bilmeden bırakır.

### Arayüz

`DependencyCacheStatus.tsx`, ayarın yanında **iki satır** — Maven ve npm — her
biri kendi boyutu, "Temizle" ve "Doğrula" eylemleriyle. Tek bir toplam boyut
göstermek, hangi ekosistemin diski doldurduğu sorusunu cevapsız bırakırdı.

Hiç kullanılmamış önbellek **"0 B" değil "henüz kullanılmadı"** yazar (spec H3).
Temizleme onay ister; onay şeridi hangi ekosistemin ve ne kadar yerin
boşalacağını **sayıyla** yazar (spec 007'nin silme onayı emsali).

---

## Ölçüm sonuçları (uygulama sırasında)

### R1 — sahiplik: doğrulandı, ve gerekçesi de ölçüldü

Boş bir adlandırılmış volume, `runner/Dockerfile`'da oluşturulup `chown`
edilmiş bir yola bağlandığında `agent:agent` oluyor ve agent yazabiliyor.
**Karşı ölçüm:** imajda bulunmayan bir yola (`~/.gradle/caches`) boş volume
bağlanınca sahiplik `root:root` çıkıyor ve yazma düşüyor. Yani `chown` satırı
savunma amaçlı bir hurafe değil, özelliğin taşıyıcısı.

Ayrıca doğrulandı: `.m2/repository`'ye bağlanan volume `.m2/settings.xml`'i
**ezmiyor**, ve içerik ayrı container'lar arasında sahipliğiyle birlikte
kalıyor.

### R6 — kilit ezilebiliyordu: `MAVEN_OPTS` terk edildi

Geçersiz bir JVM bayrağı dedektör olarak kullanıldı. `MAVEN_OPTS` ile verilen
bayrak JVM'e **ulaşıyor** (mvn düşüyor); ama agent `export MAVEN_OPTS=-Xmx1g`
dedikten sonra mvn geçiyor — yani kilit **sessizce kapanıyor.**

Bu yüzden kilit ortam değişkeninden alındı ve `mvn` sarmalayıcısına taşındı:
`/opt/maven/bin/mvn.real` gerçek başlatıcı, `/opt/maven/bin/mvn` sarmalayıcı.
Ölçüldü: agent değişkeni ezdikten sonra bile Maven'ın argv'si kilit bayrağını
taşıyor ve `mvn -v` bozulmadan çalışıyor.

**Sarmalayıcının yeri de ölçümle belirlendi.** Plan `/usr/local/bin/mvn`
demişti; imajın `PATH`'inde `/opt/maven/bin` daha önde olduğu için orası hiç
çağrılmazdı. Bu, yalnızca bozuk bir jar ortaya çıktığında fark edilecek türden
sessiz bir arıza olurdu.

**Sınır açıkça yazılıyor:** sarmalayıcı KAZAYLA kapatmaya karşı korur;
`mvn.real`'i doğrudan çağıran bir agent yine atlar. Tam kapatma sandbox'ın işi
değil ve spec 027'nin güven modeli bunu zaten kabul ediyor.

---

## Riskler

| # | Risk | Neden gerçek | Nasıl kapatılır |
| --- | --- | --- | --- |
| R1 | **Sahiplik.** Boş volume, imajda var olmayan bir yola bağlanırsa `root:root` oluşur; agent (uid 10001, [Dockerfile:97](../../runner/Dockerfile#L97)) yazamaz | Docker boş volume'ü ancak yol imajda **varsa** onun sahipliğiyle doldurur. Sahipliği belirleyen, volume'e **ilk dokunan** container'dır — koşu container'ı olmak zorunda değil | (a) `runner/Dockerfile`'da iki dizin önceden oluşturulup `chown agent:agent` yapılır — **ilk görev bu**. (b) Volume'e dokunan **her** container runner imajından doğar; yardımcı container dahil, başka imaj kullanılmaz |
| R2 | **Maven süreçler arası kilit varsayılan DEĞİL.** Maven Resolver'ın varsayılan kilit fabrikası JVM içi; iki ayrı container iki ayrı JVM demek, yani varsayılan hâlde koruma **yok** | İmajda Maven 3.9.16 var. Paylaşılan bir yerel depoya iki koşunun aynı anda yazması tam olarak bu özelliğin yarattığı durum | Süreçler arası dosya kilidi **baştan** açılır: `MAVEN_OPTS` ile, `settings.xml`'e dokunulmadan. Ölçüm "açılmalı mı"yı değil **"yeterli mi"yi** doğrular: iki eşzamanlı koşu → bozuk jar yok ve ikisi de başarıyla biter |
| R3 | Volume boyutu ölçülemiyor | Depolama sürücüsüne göre değişir | Yardımcı container + `du`; API'ye güvenilmiyor |
| R4 | Temizleme sırasında koşu başlarsa | `Active()` kontrolü ile silme arasında yarış var | Silme, koşu başlatmayla aynı kilidi alır; kapı `runs.Manager`'da, HTTP katmanında değil |
| R5 | `.npm/_cacache` mount'u npm'in diğer durumunu bozar | `_cacache` npm'in tek önbellek dizini değil | Yalnızca `_cacache` bağlanır; `.npmrc` ve global kök dokunulmaz |
| R6 | **Agent kilidi kapatabilir.** Kilit bir ortam değişkeniyle veriliyorsa agent koşu içinde `export MAVEN_OPTS=…` diyerek onu ezebilir | Koşuda çalışan şey modelin yazdığı kod; `-Xmx` ayarlamak için değişkeni ezmek sıradan bir davranış ve **niyet gerektirmiyor**. Kilit kapanınca R2 sessizce geri gelir | T06 ölçer, T07 kapatır. Ezilmeye dayanıklı aday `/usr/local/bin/mvn` sarmalayıcısı; `MAVEN_ARGS` aday değil — o da ortam değişkeni, aynı delik. **T10–T12 bu durumu yakalayamaz**, temiz koşuyu ölçüyorlar |

R1 ve R2 uygulamanın **başına** alındı: ikisi de "çalışmıyor" cevabını en ucuz
veren ve tasarımı değiştirebilecek maddeler.

---

## Test stratejisi

- **Birim (Go):** volume adı üretimi, `Spec.Caches` kurulumu, ayar kapalıyken
  mount eklenmediği, hata hâlinde `Caches`'in boşaltılıp uyarının üretildiği.
  Docker'sız — `sandbox` istemcisi arayüz üzerinden sahte.
- **Elle ölçüm (R1, R2):** gerçek Docker ile; aşağıdaki sıradaki T1 ve T3.
- **Kabul (spec kriterleri):** iki ardışık koşuda ikincisinin derleme çıktısında
  indirme satırı olmaması; eşzamanlı iki koşunun ikisinin de başarıyla bitmesi
  ve bozuk artefakt kalmaması.
- **Gerileme:** ayar kapalıyken mevcut koşu testleri **değişmeden** geçmeli —
  H2'nin "birebir aynı" iddiasının kanıtı budur.

---

## Uygulama sırası

Riskli parçalar başta.

1. **R1 — sahiplik.** `runner/Dockerfile`'a dizinler + `chown`; elle bir koşuda
   agent'ın önbelleğe yazabildiği doğrulanır.
2. **Bağlama.** `sandbox.Spec.Caches` + `Create` içinde `Mounts`; volume önce
   `VolumeCreate`. Süreçler arası kilit `MAVEN_OPTS` ile aynı adımda açılır —
   sonradan eklenecek bir emniyet değil, baştan var olan bir kural.
3. **R2 — eşzamanlılık ölçümü.** İki koşu aynı anda; kilit **yeterli mi**
   ölçülür: bozuk jar yok ve ikisi de başarıyla biter. Sonuç `plan.md`'e yazılır.
   Kilit fabrikası gerekiyorsa `MAVEN_OPTS` burada eklenir.
4. **Ayar + düşerek devam.** Registry kaydı, `m.dependencyCache()`, hata hâlinde
   uyarı.
5. **Boyut + temizleme.** Yardımcı container, iki uç, `Active()` kapısı.
6. **Doğrulama (H5).** Maven `.sha1` karşılaştırması, npm `cache verify`.
7. **Arayüz.** `DependencyCacheStatus.tsx`.
8. **Kapanış.** `plans/03`'e "Çözüldü" bölümü; AGENTS.md'ye yeni komut/konvansiyon
   çıktıysa ekleme.

## Değişecek dosyalar

| Dosya | Ne |
| --- | --- |
| `runner/Dockerfile` | İki dizin + `chown` (R1) |
| `backend/internal/runner/sandbox/docker.go` | `Spec.Caches`, `CacheMount`, `HostConfig.Mounts`, volume oluşturma/silme/ölçme |
| `backend/internal/settings/registry.go` | Tek `Definition` kaydı |
| `backend/internal/runs/manager.go` | `m.dependencyCache()`, düşerek devam + uyarı, temizleme/doğrulama kapısı |
| `backend/internal/httpapi/router.go` + yeni `dependencycache.go` | Üç uç |
| `frontend/src/components/settings/DependencyCacheStatus.tsx` | Yeni |
| `frontend/src/lib/api.ts`, `types.ts` | Üç uç için istemci ve tipler |
| `plans/03-bagimlilik-onbellegi-2026-08-14.md` | Kapanış bölümü |
