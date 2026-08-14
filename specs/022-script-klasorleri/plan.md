# Plan: Script klasörleri

- **Spec:** [spec.md](spec.md) · Onaylandı
- **Tarih:** 2026-08-14

---

## Seçilen yaklaşım

Script'e **isteğe bağlı bir klasör** eklenir; agent'a hem klasör hem tekil
script atanabilir. Çalıştırma anında ikisi birleştirilip bugünkü yola —
container'daki dosyalar ve talimat metni — akar.

```
scripts                       agent
  ├─ folder_id? ──► script_folders ◄── agent_script_folders ──► agents
  └────────────────────────────────────  agent_scripts  ──────►
```

Üç karar bu şekli belirliyor:

**Klasör script'in ÜZERİNDE bir alan, ayrı bir bağ değil.** Bir script en
fazla bir klasörde olacağı için (kullanıcı kararı) çoktan-çoğa bir tablo
gereksiz; `scripts.folder_id` hem kuralı veritabanı seviyesinde garanti eder
hem de "klasör silinince script klasörsüz kalır" davranışını `ON DELETE SET
NULL` ile bedavaya verir (H5).

**Atama iki ayrı kümedir.** `agent_scripts` (tekil) korunur, yanına
`agent_script_folders` eklenir. Klasörü "içindeki script'lere atama" olarak
çözmek daha az tablo demekti ama H3'ü bozardı: klasöre sonradan eklenen script
o agent'ta geçerli olmazdı.

**Çözüm çalıştırma anında yapılır.** `ForAgent` iki kümenin birleşimini döner.
Böylece klasör içeriği değiştiğinde hiçbir atama tazelenmez.

---

## Elenen alternatifler

| Alternatif | Neden elendi |
| --- | --- |
| Klasör yerine ad öneki kuralı (`node24-01-…`) | Bugün de yapılabiliyor ve sorunu çözmüyor: liste yine düz, atama yine tek tek, kampanya yine isimsiz. |
| Çoktan-çoğa `script_folders` bağı | Kullanıcı kararı "bir script en fazla bir klasörde". Çoktan-çoğa, kuralı uygulama katmanına taşır ve veritabanı garanti etmez. |
| Klasörü atarken içindeki script'leri `agent_scripts`'e yazmak | Tek tablo ile idare edilirdi ama klasöre sonradan eklenen script o agent'a geçmezdi — H3'ün tam tersi. |
| Ürünün ayrı bir sıra alanı tutması | Spec kararı: sıra addan gelir. İki doğruluk kaynağı (kayıttaki sıra ve dosya adı) container'a bakan kişiye yanlış sırayı gösterebilirdi. |
| Klasörü akış adımına bağlamak | Kullanıcı kararı: bağlanma noktası agent. |

---

## Yeniden kullanılacak mevcut kod

| Ne | Nerede | Nasıl |
| --- | --- | --- |
| Ad doğrulaması | `scripts.validName` | Klasör adı da dizin adına dönüşüyor; **aynı** kural, kopyalanmadan kullanılacak. |
| Yol üretimi | `Script.Path()`, `Dir` | Klasörü hesaba katacak şekilde genişletilecek — ikinci bir yol üreticisi YAZILMAYACAK. |
| Talimat bloğu | `runner/config.go` → `scriptSection` | Yapısı korunur, klasör başlığı eklenir. |
| Yetki kapısı | `scriptsFor(a)` | Tek kapı; klasörlü script'ler de buradan geçecek. Bu fonksiyon "yeni yetenek açmıyor" iddiasının tek kanıtı. |
| Atama kalıbı | `SetAgentScripts` (sil + ekle, tek transaction) | Klasör ataması aynı kalıpla. |
| Ayarlar ekranı | `components/settings/ScriptSection.tsx` | Gruplu listeye dönüşür; yeni ekran açılmaz. |
| Agent formu | `app/agents/page.tsx` | Script seçimi klasör + tekil olacak şekilde genişler. |

---

## Veri modeli

Yeni migration: `000016_script_klasorleri.sql`

```sql
CREATE TABLE script_folders (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,   -- dizin adına dönüşür
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Klasör silinince script'ler SİLİNMEZ, klasörsüz kalır (spec H5).
ALTER TABLE scripts
    ADD COLUMN folder_id UUID REFERENCES script_folders(id) ON DELETE SET NULL;

CREATE TABLE agent_script_folders (
    agent_id  UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    folder_id UUID NOT NULL REFERENCES script_folders(id) ON DELETE CASCADE,
    PRIMARY KEY (agent_id, folder_id)
);
```

### Ad benzersizliği — buradaki tuzak veritabanının kendisinde

Bugün `scripts.name` **global** UNIQUE. Yeni kural: aynı klasörde aynı ad
olamaz, farklı klasörlerde olabilir.

Saf `UNIQUE (folder_id, name)` **YETMEZ**: Postgres iki NULL'ı birbirinden
farklı sayar, yani iki klasörsüz script aynı adı alabilir — ve o ikisi
container'da **aynı dosyaya** yazılırdı. Postgres 16 kullanıldığı için doğru
karşılık:

```sql
ALTER TABLE scripts DROP CONSTRAINT scripts_name_key;
CREATE UNIQUE INDEX scripts_klasor_ad ON scripts (folder_id, name) NULLS NOT DISTINCT;
```

`NULLS NOT DISTINCT` klasörsüzleri de tek bir kümede benzersiz tutar.

---

## Go arayüzleri

```go
// scripts paketi
type Folder struct {
    ID          uuid.UUID
    Name        string
    Description string
    ScriptCount int   // listede gösterilir (H1); ayrı sorgu değil, JOIN ile
}

// Script'e eklenen alan
type Script struct {
    // …
    FolderID   *uuid.UUID
    FolderName string   // yolu üretmek için; ayrı sorgu yapılmasın diye JOIN'den gelir
}

// Yol artık klasörü hesaba katıyor. TEK üretici.
func (s Script) Path() string   // /home/agent/scripts[/<klasör>]/<ad>.sh
func (f Folder) Path() string   // /home/agent/scripts/<klasör>

func (s *Store) ListFolders(ctx) ([]Folder, error)
func (s *Store) CreateFolder(ctx, in FolderInput) (Folder, error)
func (s *Store) UpdateFolder(ctx, id, in FolderInput) (Folder, error)
func (s *Store) DeleteFolder(ctx, id) error
func (s *Store) FolderUsage(ctx, id) (scripts int, agents int, err error)  // silme onayı için

func (s *Store) SetAgentFolders(ctx, agentID, folderIDs) error

// ForAgent DEĞİŞİR: tekil atamalar + atanmış klasörlerin TÜM script'leri.
// Sıra: önce klasörsüzler, sonra klasör adı, sonra script adı.
func (s *Store) ForAgent(ctx, agentID) ([]Script, error)
```

### Runner tarafı

```go
type ScriptSpec struct {
    Name        string
    Description string
    Content     string
    Folder      string // boşsa kök
}

type FolderSpec struct {   // talimatta klasörü anlatmak için
    Name        string
    Description string
}
```

`AgentSpec` içine `ScriptFolders []FolderSpec` eklenir — klasörün **açıklaması**
script'lerde yok ve model kampanyanın ne olduğunu ondan öğrenecek (H4).

---

## HTTP uçları

```
GET    /api/script-folders            → klasörler + script sayısı
POST   /api/script-folders
PUT    /api/script-folders/{id}
DELETE /api/script-folders/{id}       → 409 + kullanım sayıları (onay istenmişse)
GET    /api/script-folders/{id}/usage → kaç script, kaç agent
```

Script uçlarına `folderId` alanı eklenir (oluşturma ve güncelleme).
Agent uçlarına `scriptFolderIds` eklenir; `scriptIds` korunur.

---

## Proje dizini — script'in çapası

Script yazarı projesinin İÇİNDEKİ yolu biliyor; kökün nereye açıldığını
bilmiyor. Bugün kök `/work` ama bu değer Go tarafında **hiç geçmiyor**:
yalnızca `entrypoint.sh` (`readonly WORKDIR=/work`) ve Dockerfile biliyor.

Karar: **değeri ürün belirler, betik okur.**

```go
// runner paketi — TEK tanım
const ProjectDir = "/work"
```

```bash
# entrypoint.sh — ürünün verdiğini kullanır, kendi değerini dayatmaz
readonly WORKDIR="${PROJECT_DIR:-/work}"
```

Container'a `PROJECT_DIR` olarak geçer. Böylece:

- script `"$PROJECT_DIR/config/webpack.config.js"` yazar ve çalışma dizininden
  BAĞIMSIZ olur — bash aracının hangi dizinde koştuğu bizim denetimimizde değil
- değer tek yerde tanımlı; `entrypoint.sh`'daki varsayılan yalnızca eski
  çağrılar için güvenlik ağı

Değişken üç yerde görünür olur: agent talimatı, script düzenleme ekranı,
README.

## Container yerleşimi

```
/home/agent/scripts/
  ├─ ortak-temizlik.sh              ← klasörsüz (bugünkü davranış, değişmez)
  └─ node-24-upgrade/
       ├─ 01-engine-alanini-guncelle.sh
       ├─ 02-bagimliliklari-yukselt.sh
       └─ …
```

### Tuzak: alt dizin tar akışında AÇIKÇA oluşturulmalı

`copyFiles` yalnızca **dosya** başlığı yazıyor
([docker.go:210](../../backend/internal/runner/sandbox/docker.go#L210)).
`/home/agent/scripts` imajda `mkdir -p` ile ve doğru sahiplikle var, ama
`node-24-upgrade/` yok. Dizin başlığı yazılmazsa çıkarma sırasında ara dizin
varsayılan sahiplikle oluşur ve agent (uid 10001) onu listeleyemeyebilir.

Bu tam olarak daha önce yaşanmış bir hata; kaydı Dockerfile'da duruyor:

> *"`AccessDeniedException` ile düşüyor. Dosyanın yazılmış olması dizinin
> kullanılabilir olduğu anlamına gelmiyordu."*

Karşılık: `ConfigFile`'a `IsDir` alanı eklenir; her klasör için önce
`tar.TypeDir` başlığı (mode 0o755, uid/gid 10001) yazılır.

---

## Talimat metni

Bugünkü blok korunur, üstüne klasör başlıkları gelir:

```markdown
## Kullanabileceğin betikler

Aşağıdaki betikler önceden yazılmış ve gözden geçirilmiştir. …

- `/home/agent/scripts/ortak-temizlik.sh` — npm önbelleğini temizler

### node-24-upgrade — Node 18'den 24'e standart yükseltme adımları

Dizin: `/home/agent/scripts/node-24-upgrade`
Adımlar NUMARA SIRASINDA yazılmıştır.

- `01-engine-alanini-guncelle.sh` — package.json'daki engines alanını 24'e çeker
- `02-bagimliliklari-yukselt.sh` — …
```

"Adımlar numara sırasında yazılmıştır" cümlesi **sırayı zorlamıyor**, sıranın
var olduğunu söylüyor. Nasıl işleteceği agent'ın kendi talimatında (kullanıcının
yazdığı prompt) anlatılacak — spec'in kapsam dışı bölümüne uygun.

---

## Değişecek dosyalar

| Dosya | Değişiklik |
| --- | --- |
| `db/migrations/000016_script_klasorleri.sql` **(yeni)** | Tablolar + benzersizlik indeksi |
| `internal/scripts/script.go` | `Folder`, `FolderID`, klasörlü `Path()` |
| `internal/scripts/store.go` | Klasör CRUD, `SetAgentFolders`, `ForAgent` birleşimi, `FolderUsage` |
| `internal/runner/runner.go` | `ScriptSpec.Folder`, `AgentSpec.ScriptFolders`, `ConfigFile.IsDir`, `ProjectDir` sabiti |
| `runner/entrypoint.sh` | `WORKDIR` değeri `PROJECT_DIR`'den okunur |
| `internal/runner/opencode/runner.go` | `PROJECT_DIR` ortam değişkeni |
| `internal/runner/config.go` | `scriptPath` klasörlü, `scriptSection` klasör başlıklı, dizin girdileri |
| `internal/runner/sandbox/docker.go` | `copyFiles` dizin başlığı yazar |
| `internal/runbuild/builder.go` | Klasör açıklamalarını `AgentSpec`'e taşır |
| `internal/httpapi/scriptfolders.go` **(yeni)** + `router.go` | Uçlar |
| `internal/httpapi/agents.go` | `scriptFolderIds` |
| `frontend/src/lib/types.ts`, `api.ts` | Yeni tipler ve çağrılar |
| `components/settings/ScriptSection.tsx` | Gruplu liste, klasör oluşturma/silme, script'in klasörü |
| `app/agents/page.tsx` | Klasör + tekil seçim |
| `README.md` | Kampanya kurma anlatımı |

---

## Riskler

| Risk | Karşılık |
| --- | --- |
| **NULL benzersizliği** — iki klasörsüz script aynı adı alırsa container'da aynı dosyaya yazılır | `NULLS NOT DISTINCT` indeksi + birim test |
| **Alt dizin sahipliği** — agent dizini listeleyemez | Tar'da açık dizin başlığı + gerçek container'da doğrulama |
| Mevcut kurulumların bozulması | `folder_id` NULL varsayılan; klasörsüz davranış birebir korunur, testle kilitlenir |
| `ForAgent` birleşiminde mükerrer | Bir script hem tekil hem klasör üzerinden gelebilir → `DISTINCT` ve testi |
| Klasör adı değişince container yolu değişir | Ad değişikliği yalnızca sonraki çalıştırmayı etkiler; süren koşu kendi kopyasıyla devam eder (bugünkü davranışın aynısı) |

---

## Test stratejisi

**Birim:**

- Klasör adı doğrulaması (script kuralının aynısı olduğu testle kilitlenir)
- `Path()`: klasörlü ve klasörsüz
- `scriptSection`: klasörlü + klasörsüz karışık, yalnız klasörsüz, hiç yok
- `scriptsFor`: bash kapalıyken klasörlü script de gelmez

**Veritabanı (`make test-integration`):**

- Aynı klasörde aynı ad reddedilir; **farklı klasörde aynı ad kabul edilir**
- **İki klasörsüz script aynı adı ALAMAZ** — `NULLS NOT DISTINCT` bu testte kanıtlanır
- Klasör silinince script'ler kalır ve `folder_id` NULL olur
- `ForAgent`: tekil + klasör birleşimi, mükerrer yok, sıra doğru
- Klasöre script eklendiğinde atama tazelenmeden `ForAgent`'ta görünür

**Container (gerçek Docker):**

- Klasörlü script gerçekten alt dizine yazılır ve **agent kullanıcısı
  listeleyebilir** — yalnızca dosyanın varlığına bakmak yetmez (Dockerfile'daki
  yara kaydı)

**Arayüz:** `ui.md` — iki tema, gruplu listenin boş ve dolu hâli.

---

## Uygulama sırası

Riskli parça başa:

1. **Migration + benzersizlik indeksi.** NULL tuzağı burada kanıtlanır.
2. `scripts` paketi: klasör CRUD, `Path()`, `ForAgent` birleşimi.
3. **Container yerleşimi**: dizin başlığı + gerçek Docker'da doğrulama.
4. Talimat metni (`scriptSection`).
5. HTTP uçları.
6. Arayüz: ayarlar ve agent formu.
7. Belgeler, `tasks.md` kapanışı.
