# Plan: Toplu çalıştırma

- **Spec:** [spec.md](spec.md) · Onaylandı
- **Tarih:** 2026-08-15

---

## Seçilen yaklaşım

Kalıcı bir **toplu iş** kaydı ve içinde sırayla işlenen **öğeler**; boş slot
oldukça sıradakini başlatan bir zamanlayıcı.

```
run_batches          (akış, görev metni, durum)
  └─ run_batch_items (proje, sıra, durum, workflow_run_id, hata)

zamanlayıcı ──► boş slot var mı? ──► sıradaki bekleyeni başlat
                     ▲
              runs.Manager'ın SINIRI (yeni ayar YOK)
```

Üç karar bu şekli belirliyor:

**Kuyruk kalıcı.** Otuz projelik bir kampanya saatler sürüyor; o sürede bir
yeniden başlatma olağan ve bellekteki kuyruk bekleyen işleri sessizce yok
ederdi.

**Sınır sorulur, kopyalanmaz.** Zamanlayıcı kendi paralellik sayısını
tutmuyor; `runs.Manager`'a "kaç slot boş" diye soruyor. İkinci bir sayı, ayar
değiştiğinde geride kalırdı.

**Öğe bir akış çalışmasına bağlanır.** Toplu iş yeni bir çalıştırma türü
değil; var olan `workflow.Launcher` çağrılıyor ve dönen çalışma kimliği öğeye
yazılıyor.

---

## Elenen alternatifler

| Alternatif | Neden elendi |
| --- | --- |
| Yoklamalı zamanlayıcı (2 sn'de bir bak) | Slot boşalma sinyali ZATEN var (`cond.Broadcast`); var olan bilgiyi yok sayıp aynı soruyu tekrar sormak olurdu. |
| Bellekte kuyruk (goroutine + kanal) | Yeniden başlatma bekleyenleri yok ederdi; kullanıcı bunu "neden hiç başlamadı" diye fark ederdi. |
| Hepsini birden başlatıp `ErrTooManyRuns` alanları yeniden denemek | Reddedilen çalıştırmanın **kaydı oluşmuyor**; kullanıcı otuz işten yirmi yedisini hiç göremezdi. Ayrıca yeniden deneme aralığı yeni bir zamanlayıcı demek — yani aynı işi daha kötü yapmak. |
| Eşzamanlılık sınırını yükseltmek | İş başına 2 çekirdek + 4 GB; otuz iş 60 çekirdek + 120 GB. Sınır keyfi değil. |
| Kuyruğa kendi paralellik ayarı vermek | "Aynı anda kaç iş" iki yerden yönetilir ve er geç çelişirdi. |
| `runs.Manager`'ı kuyruklu hale getirmek | Tek çalıştırmanın davranışını da değiştirirdi: bugün "sınır dolu" diyen uç, sessizce beklemeye başlardı. Toplu iş bunu isterken tekil çağrı istemiyor. |

---

## Yeniden kullanılacak mevcut kod

| Ne | Nerede | Nasıl |
| --- | --- | --- |
| Akış başlatma | `workflow.Launcher.Launch` (proje seçimiyle) | Öğe başlatma bunu çağırır; yeni bir başlatma yolu yazılmaz. |
| **Slot boşalma sinyali** | `runs.Manager.release()` → `cond.Broadcast()` | Sinyal zaten üretiliyor; zamanlayıcı ona bağlanır, yoklama yazılmaz. |
| Eşzamanlılık sınırı | `runs.Manager.Active()` + `limits.MaxConcurrent()` | Zamanlayıcı buradan okur. |
| Sıraya alma kalıbı | `scripts.SetAgentFolders` (sil+ekle, tek transaction) | Öğelerin toplu yazımı. |
| Kısmi başarı dili | spec 021 içe aktarma özeti | "9 tamamlandı, 3 başarısız" biçimi aynı. |
| Onay şeridi | `ConfirmStrip` (soru + SONUÇ) | İptal onayı. |
| Seçim mantığı | `components/projects/import-selection.ts` kalıbı | Proje seçimi saf modülde, testli. |

---

## Veri modeli

`000017_toplu_calistirma.sql`

```sql
CREATE TABLE run_batches (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    -- Görev metni toplu işin kendisinde: otuz öğe aynı işi yapıyor.
    task        TEXT NOT NULL DEFAULT '',
    -- queued | running | done | cancelled
    status      TEXT NOT NULL DEFAULT 'queued',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE run_batch_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id    UUID NOT NULL REFERENCES run_batches(id) ON DELETE CASCADE,
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    -- position: sıra EKLENME sırasıdır (spec). Öncelik yok.
    position    INT  NOT NULL,
    -- pending | running | succeeded | failed | interrupted | cancelled
    status      TEXT NOT NULL DEFAULT 'pending',
    -- Akış çalışması başlatıldıktan SONRA dolar; öncesinde NULL.
    workflow_run_id UUID REFERENCES workflow_runs(id) ON DELETE SET NULL,
    error       TEXT NOT NULL DEFAULT '',
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,

    -- Aynı proje aynı toplu işte iki kez yer almaz (spec → davranış kuralları).
    UNIQUE (batch_id, project_id)
);

CREATE INDEX idx_batch_items_bekleyen
    ON run_batch_items (batch_id, position) WHERE status = 'pending';
```

---

## Zamanlayıcı

`internal/runbatch` (yeni paket).

```go
func (s *Scheduler) Run(ctx context.Context)   // main.go'dan goroutine
func (s *Scheduler) Reconcile(ctx) error       // açılışta bir kez
```

**Olay güdümlü.** Slot boşaldığında zamanlayıcı uyandırılır; iki saniyede bir
"acaba boşaldı mı" diye sorulmaz.

Sinyal ZATEN var: `runs.Manager.release()` slot'u bıraktıktan sonra
`cond.Broadcast()` çağırıyor. Yoklama, var olan bir bilgiyi yok sayıp aynı
soruyu saniyede bir tekrar sormak olurdu.

Bağımlılık yönü ters çevrilmiyor — `runs` paketi kuyruğu TANIMIYOR. Manager'a
isteğe bağlı bir uyandırma kancası veriliyor ve bağ `main.go`'da kuruluyor:

```go
// runs.Limits
OnSlotFree func()   // nil olabilir; tek çalıştırma yolu bunu kullanmıyor

// main.go
limits.OnSlotFree = scheduler.Wake
```

`Wake`, tamponu 1 olan bir kanala **bloklamayan** gönderim yapar: zamanlayıcı
o an meşgulse sinyal kaybolmaz, birikmez de — "bak" demek bir kez yeter.

Zamanlayıcı ayrıca toplu iş oluşturulduğunda ve "kaldığı yerden devam"
denildiğinde uyandırılır.

**İKİ SİNYAL GEREKİYOR, biri yetmiyor** (Blok 6'da ölçülerek eklendi).
`OnSlotFree` "kapasite açıldı" der ama "iş bitti" DEMEZ: slotu bırakan
`defer finish()`, motorun `FinishRun`'ından ÖNCE koşuyor. O sinyalle uyanan
kuyruk çalışmayı hâlâ `running` görür, öğeyi kapatamaz — ve bitişi yalnızca
dakikalık emniyet turunda fark ederdi. Yani sigorta, mekanizmanın yerine
geçerdi: biten iş bir dakikaya kadar kuyrukta yer tutar, sıradaki o kadar geç
başlardı.

Bu yüzden `workflow.Executor`'a ikinci bir kanca eklendi — `OnRunFinished`,
son durum YAZILDIKTAN sonra çağrılır (üç kapanış yolunun hepsinden: başarı,
hata, iptal). Bağımlılık yönü yine korunuyor; bağ `main.go`'da.

**Bir de emniyet turu var (varsayılan bir dakika).** Bu MEKANİZMA DEĞİL,
sigorta: bir uyandırma her nasılsa kaçarsa kuyruk sonsuza kadar durmasın diye.
Dakikada bir tur, sistemin fark etmeyeceği bir maliyet; sessizce donmuş bir
kuyruk ise kullanıcının ancak "neden hiç başlamadı" diye sorarak fark edeceği
bir arıza.

Süre **ayardan** okunur (`runner.batch_safety_interval_minutes`), koddan değil —
kodda gömülü davranış parametresi bırakılmaz. Her turda yeniden okunduğu için
değişiklik yeniden başlatma istemez; süren beklemeyi kesmez ama her uyandırma
zaten yeni turu yeni değerle kurar.

### Tuzak: `ErrTooManyRuns` alan öğe BAŞARISIZ SAYILMAZ

**Bu bölüm Blok 2'ye başlarken düzeltildi — planın ilk hâli yanlış bir olguya
dayanıyordu.** Kod okundu:

- `workflow.Launcher.Launch` sınıra **hiç bakmıyor**: kaydı oluşturup motoru
  goroutine olarak başlatıyor ve `nil` dönüyor. `ErrTooManyRuns` dönmesi mümkün
  değil.
- Sınır **adım seviyesinde**: `runs.Manager.begin` → `tryAcquire`. Dolu
  olduğunda `runbuild.StepRunner` hatayı yukarı veriyor ve motor akış
  çalışmasının TAMAMINI `failed` yapıyor (`executor.go` → `e.fail`).
- Yani bir öğe sınır yüzünden düştüğünde bu, senkron bir dönüş değil,
  **dakikalar sonra veritabanına düşen bir sonuç.**
- Ayrıca **bir öğe = bir slot değil**: paralel dallı bir akış aynı anda birden
  fazla slot tutar.

Tuzak yok olmadı, **yer değiştirdi.** Karşılığı iki parçalı:

1. **Zamanlayıcı kendi öğe sayısını sınırlar.** Yeni öğe yalnızca *çalışan öğe
   sayısı* sınırın altındayken VE manager'da gerçekten boş slot varken başlar.
   İkinci koşul olmasa, slotları başka çalıştırmalar tutarken başlatılan öğe
   anında düşer ve kuyruk kendini yerdi.
2. **Sınır hatasıyla düşen öğe `pending`'e döner**, `failed` olmaz. Sonuç
   okunurken hata `runs.ErrTooManyRuns`'un metniyle karşılaştırılır — sınır
   hatası öğenin başarısızlığı değil, zamanlamanın sonucudur.

Elenen alternatif: *adım sınır dolduğunda beklesin* (`StepRunner` yeniden
denesin). Sorunu kaynağında çözerdi ama spec 023'ün kapsamı dışında BÜTÜN
akışların davranışını değiştirirdi — bugün hızla düşen bir adım yarın sessizce
beklemeye başlardı. Aynı gerekçeyle "`runs.Manager`'ı kuyruklu hale getirmek"
de elenmişti.

### Öğe önce SAHİPLENİLİR, sonra başlatılır

`Claim` (pending → running) akış çalışması başlatılmadan ÖNCE yazılır; dönen
çalışma kimliği `Attach` ile eklenir. Ters sırada, iki çağrı arasında düşen bir
süreç öğeyi `pending` bırakır ve açılışta aynı iş İKİNCİ KEZ başlatılırdı — yan
etkisi (branch'e gönderilmiş bir değişiklik) habersizce tekrarlanmış olurdu.

Bu sırayla en kötü hâl, çalışma kimliği olmayan `running` bir öğedir; açılışta
`Reconcile` onu `interrupted` yapar ve kullanıcı "kaldığı yerden devam" der.

### Açılışta uzlaştırma (`Reconcile`)

Backend kapandığında `running` kalan öğeler var. Container'lar gitti; o
çalışmalar tamamlanamaz.

Açılışta bu öğeler **`interrupted`** olarak işaretlenir. Kendiliğinden
denenmez (spec kararı): yarım kalmış bir işin yan etkisi — branch'e
gönderilmiş bir değişiklik — habersizce tekrarlanmamalı. Kullanıcı
**"Kaldığı yerden devam et"** düğmesiyle onları yeniden sıraya alır.

---

## Go arayüzleri

```go
type Batch struct {
    ID, WorkflowID uuid.UUID
    Task           string
    Status         string
    Counts         Counts        // bekleyen/çalışan/biten/başarısız/kesilen
}

type Item struct {
    ID, ProjectID   uuid.UUID
    ProjectName     string      // JOIN'den; ekran isim gösteriyor
    Position        int
    Status          string
    WorkflowRunID   *uuid.UUID
    Error           string
}

func (s *Store) Create(ctx, workflowID uuid.UUID, task string, projectIDs []uuid.UUID) (Batch, error)
func (s *Store) List(ctx, limit, offset int) ([]Batch, int, error)
func (s *Store) Get(ctx, id uuid.UUID) (Batch, []Item, error)
func (s *Store) Cancel(ctx, id uuid.UUID) (int, error)   // yalnızca bekleyenleri düşürür
func (s *Store) Resume(ctx, id uuid.UUID) (int, error)   // kesilenleri pending yapar

// Zamanlayıcının kullandıkları
func (s *Store) NextPending(ctx) (Item, bool, error)
func (s *Store) Claim(ctx, itemID uuid.UUID) error        // pending → running
func (s *Store) Attach(ctx, itemID, workflowRunID uuid.UUID) error
func (s *Store) Requeue(ctx, itemID uuid.UUID) error      // sınır hatası → pending
func (s *Store) RunningItems(ctx) ([]Item, error)         // sayım + sonuç toplama
func (s *Store) MarkFinished(ctx, itemID uuid.UUID, status, errMsg string) error
func (s *Store) InterruptRunning(ctx) (int, error)       // açılışta
```

---

## HTTP uçları

```
POST   /api/run-batches            { workflowId, task, projectIds[] }
GET    /api/run-batches            → liste + sayılar
GET    /api/run-batches/{id}       → toplu iş + öğeler
POST   /api/run-batches/{id}/cancel → kaç bekleyen düştü
POST   /api/run-batches/{id}/resume → kaç kesilen sıraya alındı
```

---

## Arayüz

**Akış detayında** "Çok projede çalıştır": proje seçim listesi (arama +
tümünü seç), seçim sayacı, tek birincil eylem.

**Toplu iş ekranı**: üstte sayılar (bekleyen · çalışan · tamamlandı ·
başarısız), altta öğe listesi. Çalışan/biten satır kendi akış çalışmasına
bağlanır. İş sürerken tazelenir (mevcut `refetchInterval` kalıbı).

Eylemler: **İptal** (bekleyenler düşer, çalışanlar sürer — onayda yazılı) ve
**Kaldığı yerden devam et** (yalnızca kesilenler; üzerinde sayı yazar).

---

## Riskler

| Risk | Karşılık |
| --- | --- |
| **`ErrTooManyRuns` öğeyi düşürür** → kuyruk kendini yer | İki katman: zamanlayıcı çalışan öğe sayısını sınırın altında tutar; buna rağmen sınır hatasıyla düşen öğe `failed` değil `pending` olur. Testle kilitli |
| Zamanlayıcı iptal edilmiş toplu işin öğesini başlatır | `NextPending` yalnızca `queued`/`running` toplu işlerden okur |
| Uyandırma sinyali kaçarsa kuyruk donar | Dakikalık emniyet turu; ayrıca `Wake` tamponlu kanala bloklamadan yazdığı için sinyal kaybolmaz |
| Açılışta uzlaştırma çalışmazsa öğeler sonsuza kadar `running` | `Reconcile` açılışta koşar ve kaç öğeyi düşürdüğünü loglar |
| **Kullanıcı runner container'ını elle siler** | Sinyal KAYBOLMAZ: slot'u bırakan şey container değil, goroutine'in `defer finish()`'i. Bağlantı kopar, goroutine çözülür, slot bırakılır. Tek etkisi gecikme — en kötü hâlde çalıştırma süre sınırı kadar. Emniyet turu burada devreye GİRMEZ ve girmemeli: slot gerçekten dolu, boş saymak sınırı aşardı. |
| Proje silinmiş | `ON DELETE CASCADE` öğeyi düşürür; toplu iş sayıları buna göre |
| Aynı projeyi iki kez seçme | `UNIQUE (batch_id, project_id)` |

---

## Test stratejisi

**Birim / veritabanı:**

- Sıra: `NextPending` en küçük `position`'ı verir
- `ErrTooManyRuns` alan öğe **pending kalır** — kuyruğun kendini yemediğinin
  kanıtı
- İptal: bekleyenler düşer, çalışan öğe DOKUNULMAZ
- `Resume`: yalnızca `interrupted` olanlar `pending` olur; `succeeded` ve
  `failed` dokunulmaz
- `InterruptRunning`: açılışta `running` olanlar `interrupted` olur
- Aynı proje iki kez → hata
- Bir öğe düşünce kuyruk devam eder

**Zamanlayıcı:** sahte bir başlatıcıyla, sınır 2 iken 5 öğenin **ikişer ikişer**
işlendiği ölçülür — aynı anda çalışan sayısı sınırı aşmaz.

**Arayüz:** seçim mantığı saf modülde; ekran `ui.md`'ye göre iki temada.

---

## Uygulama sırası

1. Şema + depo (sıra, iptal, resume, uzlaştırma).
2. **Zamanlayıcı** — riskli parça: sınıra uyma ve `ErrTooManyRuns` tuzağı.
3. Açılışta uzlaştırma + `main.go` bağlantısı.
4. Uçlar.
5. Arayüz: seçim ve toplu iş ekranı.
6. Belgeler, kapanış.
