# Plan: Çalışma dizini yerleşimi

- **Spec no:** 025 — [spec.md](spec.md)
- **Tarih:** 2026-08-17
- **Durum:** Taslak

---

## Yaklaşım

Bugün `runner.ProjectDir` bir **sabit**. Onu, kök sabiti (`WorkRoot`) ile
kökten ve depo adresinden yol üreten saf bir fonksiyona ayırıyoruz. Yol,
çalıştırma başına **bir kez** `runs.Manager.execute` içinde hesaplanıp
`runner.Request`'e yazılıyor; hem ortam değişkeni hem de agent'a verilen
talimat metni aynı alandan okuyor. Böylece iki gerçek üretme ihtimali
ortadan kalkıyor (spec, Davranış Kuralları).

`execute` bilinçli bir seçim: `Egress`, `CACert`, `Packages`, `Timeout` ve
`Limits` zaten orada, `runs.Limits` içindeki closure'lardan okunuyor. Yeni
ayar aynı kalıba giriyor — yeni bir mekanizma icat edilmiyor, mevcut olan
bir alan daha kazanıyor.

Ad türetme saf bir fonksiyon olduğu için tablo testiyle sınanıyor; path
traversal koruması (spec H3) burada, tek bir yerde.

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
| ---------- | ---- | ---- | ----- |
| Yolu `runner.Request`'e alan olarak koy, `execute`'ta hesapla | Mevcut `Egress`/`CACert` kalıbıyla birebir aynı; tek kaynak | `Request` bir alan daha büyür | **Seçildi** |
| Yolu `opencode.Runner` içinde hesapla | `Request` büyümez | Talimat metni `runner` paketinde üretiliyor; ikinci bir hesap noktası doğar — spec'in yasakladığı ayrışma | Elendi |
| Ayarı `entrypoint.sh`'ta çöz | Backend hiç değişmez | Betik `PROJECT_DIR`'i dayatmama sözleşmesini bozar; talimat metni yine yanlış kalır | Elendi |
| Ayar türü: yeni `KindChoice` (root/repo) | Adlandırılmış, üçüncü yerleşim eklenebilir | Registry'ye `Options` alanı, `RuntimeSettings.tsx`'e yeni render dalı, servis tarafında doğrulama — ayar çerçevesi değişir | Elendi — bkz. aşağıdaki not |
| Ayar türü: `KindBool` | Sıfır çerçeve değişikliği; jenerik render zaten var | İleride üçüncü bir yerleşim gerekirse tür değişir | **Seçildi** |

> **Ayar türü notu — onayınıza sunulan tek sapma.** Brief `WorkdirLayout
> (root \| repo)` diye iki değerli bir ayar istiyordu. Ayar çerçevesinde
> bugün seçenek listesi taşıyan bir tür **yok**; eklemek registry, servis
> doğrulaması ve arayüz render'ını birlikte değiştirmek demek. İki değerli
> bir tercih için bu, taşınan değere göre pahalı.
>
> Bu yüzden **ayar** `KindBool` olarak giriyor, ama **Go tarafındaki tip**
> yine adlandırılmış kalıyor (`WorkdirLayout`). Üçüncü bir yerleşim
> gerekirse değişen tek şey ayarın türü olur; `ProjectDir()` imzası ve
> çağıranlar aynı kalır. Enum'u ayar katmanına kadar taşımayı tercih
> ederseniz söyleyin — plan buna göre güncellenir.

---

## Veri Modeli

**Migration yok.** Ayar, mevcut `settings` mekanizmasına yeni bir `Key`
olarak giriyor; kayıt defteri kod sabiti, tablo şeması değişmiyor.

Geri alma: ayar silinirse veya okunamazsa varsayılan (`root`) uygulanır —
spec H2 gereği çalıştırma bu yüzden düşmez.

## Arayüzler

### Go tipleri

```go
// backend/internal/runner/runner.go

// WorkRoot, çalışma ortamındaki sabit kök.
const WorkRoot = "/work"

// WorkdirLayout, projenin kökün neresine açılacağı.
type WorkdirLayout string

const (
    LayoutRoot WorkdirLayout = "root" // /work
    LayoutRepo WorkdirLayout = "repo" // /work/<repo-adı>
)

// ProjectDir, verilen yerleşim ve depo adresi için proje kökünü üretir.
// Ad türetilemezse veya güvenli değilse WorkRoot döner.
func ProjectDir(layout WorkdirLayout, repoURL string) string
```

`runner.Request` yeni alan:

```go
// ProjectDir, projenin container içindeki kökü. Çalıştırma başına BİR KEZ
// hesaplanır; env ve talimat metni aynı değeri kullanır. Boşsa WorkRoot.
ProjectDir string
```

`runs.Limits` yeni closure — mevcut `Egress`/`CACert` kalıbı:

```go
// WorkdirLayout, proje kökü yerleşimi. Diğerleri gibi FONKSİYON: ayar
// arayüzden değişince yeniden başlatma beklemeden sonraki koşuda geçerli.
WorkdirLayout func() runner.WorkdirLayout
```

`BuildConfigFiles` imzası projeyi kökünü alacak şekilde genişler:

```go
func BuildConfigFiles(p ProviderSpec, a AgentSpec, model string,
    pkg PackageRegistry, projectDir string) ([]ConfigFile, error)
```

### HTTP API

Yeni uç yok. Ayar mevcut `GET /api/settings` ve `PUT /api/settings/{key}`
üzerinden gelir ve gider.

### Frontend tipleri

`frontend/src/lib/types.ts` içindeki `PROJECT_DIR` sabiti **kaldırılır**;
yerine yerleşimden metin üreten bir yardımcı gelir:

```ts
// Betik yazarına gösterilecek proje kökü. Ayar açıkken ad koşuya göre
// değiştiği için somut ad değil kalıp gösterilir.
export const WORK_ROOT = "/work";
export function projectDirLabel(repoKlasoru: boolean): string;
```

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
| ----- | ---------- |
| `backend/internal/runner/runner.go` | `ProjectDir` sabiti → `WorkRoot` + `WorkdirLayout` + `ProjectDir()`; `Request.ProjectDir` alanı |
| `backend/internal/runner/workdir_test.go` | yeni — ad türetme tablo testi |
| `backend/internal/runner/config.go` | `BuildConfigFiles` ve `scriptSection` proje kökünü parametre olarak alır |
| `backend/internal/runner/opencode/runner.go` | `buildEnv` sabit yerine `req.ProjectDir` kullanır (boşsa `WorkRoot`) |
| `backend/internal/runs/manager.go` | `Limits.WorkdirLayout` closure'ı; `execute` içinde bir kez hesap, `Request.ProjectDir`'e yazım |
| `backend/cmd/server/main.go` | closure'ın ayara bağlanması |
| `backend/internal/settings/registry.go` | yeni `Key` + tanım, varsayılan kapalı |
| `frontend/src/lib/types.ts` | `PROJECT_DIR` → `WORK_ROOT` + `projectDirLabel()` |
| `frontend/src/components/settings/ScriptSection.tsx` | proje kökü metni ayara göre değişir |
| `frontend/src/components/docs/diagrams.tsx` | diyagramdaki `/work` kutusu yerleşimden bağımsız ifadeye çevrilir |
| `docs/` (çalışma dizini anlatılan yer) | yeni seçenek + "script'ler `$PROJECT_DIR` kullandığı sürece her iki yerleşimde çalışır" notu |

> **Brief'in kapsamadığı iki yer.** Brief yalnızca backend'de grep istemişti.
> Arayüzde de sabit `/work` var: `types.ts:246` ve onu kullanan
> `ScriptSection.tsx:467` — kullanıcıya *"Proje /work altına klonlanır"*
> diyor. Ayar açıkken bu cümle **yanlış** olur; ürün kullanıcıya yanlış yol
> söyler. `diagrams.tsx:1191`'deki belge diyagramı da aynı durumda.
> İkisi de kapsama alındı.

`runner/entrypoint.sh` **değişmiyor**: `WORKDIR="${PROJECT_DIR:-/work}"`
sözleşmesi zaten doğru ve `git clone` hedef dizini kendisi oluşturuyor
(`/work` imajda mevcut). Doğrulama görevinde sınanacak; gerekirse tek satır
`mkdir -p` eklenecek.

## Yeniden Kullanılacak Mevcut Kod

- `backend/internal/settings/registry.go` — `Definition` + `KindBool`; yeni
  ayar türü yazılmıyor, mevcut kayıt defterine bir madde ekleniyor.
- `frontend/src/components/settings/RuntimeSettings.tsx` — `bool` render
  dalı zaten var; ayar arayüzde **kendiliğinden** görünür, yeni bileşen yok.
- `backend/internal/runs/manager.go` — `Limits` closure kalıbı ve `execute`
  içindeki "koşu başında çöz" noktası; `egress()`/`caCert()` ile aynı biçim.
- `net/url` ve `path` — ad türetmede kendi ayrıştırıcımızı yazmıyoruz.

---

## Riskler

| Risk | Etki | Önlem |
| ---- | ---- | ----- |
| Depo adresinden türetilen ad kökün dışına çıkar | Çalışma ortamında keyfi yola yazma | Ad tek bir saf fonksiyonda üretilir; ayraç/`..` içeren ad reddedilir ve köke düşülür. Tablo testi zorunlu (spec H3) |
| Env ile talimat metni ayrışır | Model ve betikler farklı yola bakar; sessiz bozulma | Tek alan (`Request.ProjectDir`), tek hesap noktası; her iki tüketici aynı alanı okur. İki yerleşim için de test |
| `BuildConfigFiles` imzası ~10 test çağrısını kırar | Derleme hatası | Mekanik; parametre **sona** ekleniyor. Alternatif (girdi struct'ı) daha temiz ama bu değişikliğe göre büyük — ayrı iş |
| Arayüz metni sabit kalırsa kullanıcıya yanlış yol söylenir | Betik yazarı var olmayan yola bakar | `types.ts` sabiti kaldırılıyor; metin ayardan türüyor |
| `/work` altında ilk kez alt dizin açılması | Klonlama düşer | `entrypoint.sh` gerçek bir bare repo ile sınanır (doğrulama görevi) |
| SSH kısa biçimi (`git@host:proj/repo.git`) `url.Parse` ile ayrışmaz | Yanlış ad veya köke düşme | Tabloda ayrı vaka; kısa biçim için ayrı dal |

## Test Stratejisi

- **Birim — `backend/internal/runner`:** `ProjectDir()` tablo testi.
  Vakalar: https + `.git`, https `.git`'siz, sonu `/` ile biten, kısa SSH
  (`git@host:proj/repo.git`), `http`, boş adres, sonu `/.git` olan, adı
  `..` veya `.` çıkan, ad içinde ayraç üreten adres, adı boş bırakan adres.
  Her biri için beklenen yol; güvensiz olanlarda `WorkRoot`.
- **Birim — talimat metni:** `layout=root`'ta metin **birebir bugünkü**
  (mevcut testler değişmeden geçmeli); `layout=repo`'da metinde türetilmiş
  yol geçmeli.
- **Birim — `buildEnv`:** `Request.ProjectDir` boşken `PROJECT_DIR=/work`;
  doluyken verilen değer. Mevcut `cacert_test`/`egress_test` kalıbı.
- **Entegrasyon:** `runs.Manager.execute` yolunda ayar `repo` iken üretilen
  `Request`'te env ve talimat metnindeki yolun **aynı** olduğu.
- **Elle doğrulama:**
  1. `bash -n runner/entrypoint.sh` — söz dizimi temiz.
  2. Yerel bare repo ile yalnızca klonlama bloğu:
     `PROJECT_DIR=/work/x REPO_URL=<bare> …` → klonun `/work/x`'e açıldığı görülür.
  3. Arayüz: Ayarlar → yeni seçenek görünür, varsayılan kapalı.
  4. Ayar kapalıyken bir çalıştırma → `PROJECT_DIR=/work`, betik metni aynı.
  5. Ayar açıkken bir çalıştırma → `PROJECT_DIR=/work/<ad>`, betik metni aynı yol.
  6. Betikler bölümündeki açıklama metni ayara göre doğru yolu gösteriyor.
- **Kapılar:** `go test ./...`, `go vet ./...`, `npx tsc --noEmit`,
  `npx eslint .`

## Uygulama Sırası

Riskli parça **başta**: yol türetme hem güvenlik kriteri taşıyor (H3) hem
de diğer her şey ona bağlı. Ayar ve arayüz, çalışan bir çekirdeğin üstüne
biniyor.

1. **`ProjectDir()` + tablo testleri.** Sabit ikiye ayrılır, saf fonksiyon
   ve testleri yazılır. Ayar henüz yok; çağıranlar `LayoutRoot` geçer, yani
   davranış bu adımda **hiç değişmez** — spec H2 bu noktada zaten sağlanmış olur.
2. **Tek kaynak bağlantısı.** `Request.ProjectDir` alanı, `execute`'ta
   hesap, `buildEnv` ve `BuildConfigFiles` bu alandan okur. Hâlâ sabit
   `LayoutRoot`; mevcut testler değişmeden geçmeli.
3. **Ayar + arayüz.** Registry maddesi, `main.go` closure'ı, `types.ts` ve
   `ScriptSection.tsx` metni. Yerleşim ilk kez gerçekten değişebilir hale gelir.
4. **Belge.** `docs/` notu ve ilgili spec'in karar geçmişine tarihli madde.
5. **Elle doğrulama + `entrypoint.sh` sınaması.**

Commit'ler brief'teki bölünmeye uyar: (a) adım 1-2, (b) adım 3, (c) adım 4.
