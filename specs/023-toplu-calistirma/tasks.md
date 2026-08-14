# Tasks: Toplu çalıştırma

- **Spec no:** 023 — [spec.md](spec.md) · [plan.md](plan.md)
- **Tarih:** 2026-08-15

Riskli parça başta: zamanlayıcının sınıra uyması ve `ErrTooManyRuns` tuzağı.
Kuyruğun kendini yemesi sessiz bir arıza — otuz projenin çoğu hiç çalışmadan
"başarısız" görünürdü.

---

## Blok 1 — Şema ve depo

- [x] T01 `000017_toplu_calistirma.sql`: `run_batches`, `run_batch_items`,
      bekleyen indeksi → migration uygulanır (test veritabanında sürüm 17;
      geri alma `TestMigration017_GeriAlinabilir` ile kilitli)
- [x] T02 `Create` öğeleri sırayla yazar → `position` 0..n-1
- [x] T03 Aynı proje iki kez REDDEDİLİR → `UNIQUE (batch_id, project_id)`
- [x] T04 `NextPending` en küçük `position`\'ı verir → sıra eklenme sırasıdır
- [x] T05 `NextPending` iptal edilmiş toplu işin öğesini VERMEZ
- [x] T06 `Get` toplu işi öğeleriyle ve proje adlarıyla döner (JOIN)
- [x] T07 Sayılar doğru: bekleyen · çalışan · tamamlanan · başarısız · kesilen

## Blok 2 — Zamanlayıcı (riskli)

- [x] T10 Boş slot varken sıradaki başlar → sahte başlatıcıyla ölçülür
- [x] T11 **Aynı anda çalışan sayısı sınırı AŞMAZ** → sınır 2, beş öğe;
      tepe eşzamanlılık ölçülür (ölçülen tepe: 2)
- [x] T12 **`ErrTooManyRuns` alan öğe `pending` KALIR** → başarısız
      işaretlenmez; kuyruğun kendini yemediğinin kanıtı. İki yol da ölçüldü:
      başlatmada dönen hata ve veritabanından METİN olarak gelen hata
- [x] T12a Slotları kuyruk dışı çalıştırmalar tutuyorsa hiç öğe başlatılmaz —
      planın düzeltmesiyle gelen ikinci koşul
- [x] T13 Bir öğe düşünce kuyruk DEVAM eder → sonraki öğe başlar
- [x] T14 `Wake` bloklamaz ve sinyal kaybolmaz → tamponlu kanal, art arda
      çağrıda birikmez
- [x] T15 Slot boşalınca uyandırma gelir → `runs.Limits.OnSlotFree` bağlandı
      (`main.go`) ve kancanın çağrıldığı `runs` paketinde ölçüldü
- [x] T16 Emniyet turu kuyruğu ilerletir → uyandırma hiç gelmese de
      bekleyen başlar
- [x] T17 Toplu iş bitince durumu `done` olur

## Blok 3 — Uzlaştırma, iptal, devam

- [x] T20 `InterruptRunning` açılışta `running` olanları `interrupted` yapar
      ve kaçını düşürdüğünü loglar (`Scheduler.Reconcile`, `main.go`'da açılışta
      çağrılıyor). İkinci tur sıfır döner: sayı "bu açılışta kesilenler"
- [x] T21 `Cancel` yalnızca BEKLEYENLERİ düşürür → çalışan öğe dokunulmaz ve
      sonucu iptalden sonra da kaydedilir
- [x] T22 `Resume` yalnızca `interrupted` olanları `pending` yapar →
      `succeeded`, `failed` ve `cancelled` dokunulmaz; sıraya alınan öğe
      gerçekten `NextPending`'den gelir
- [x] T23 Bitmiş toplu işi iptal hata DEĞİL → 0 döner, durumu `done` kalır

## Blok 4 — Uçlar

- [x] T30 `POST /api/run-batches` → 201, kaç öğe sıraya alındı
- [x] T31 `GET /api/run-batches` ve `/{id}` → sayılar ve öğeler
- [x] T32 `cancel` ve `resume` → kaç öğenin etkilendiği **ve eylemden sonraki
      durum** yanıtta; ekran kendi tahminiyle tazelenmesin
- [x] T33 Hata durumları spec tablosuna uyar → proje seçilmedi, aynı proje iki
      kez, akış yok, proje yok, geçersiz kimlik
- [x] T34 **Tanımsız akış sıraya KONMAZ** → 409 `no_version`. Sıraya konsaydı
      otuz öğe tek tek başlatılıp tek tek düşerdi; kullanıcı otuz satırlık bir
      başarısızlık listesi görür, sebebi ancak satırların içinde okurdu

## Blok 5 — Arayüz

- [x] T40 Akış detayında "Çok projede çalıştır": arama, tümünü seç, sayaç.
      Tek birincil eylem ve üzerinde kaç projede çalışacağı yazıyor
- [x] T41 Seçim mantığı saf modülde (`batch-selection.ts`), `npm test` 85/85
- [x] T42 Toplu iş ekranı: sayı şeridi + öğe listesi, süren işte 3 sn'de
      tazelenir; liste ekranı 4 sn'de
- [x] T43 Öğe kendi akış çalışmasına bağlanır — başlatılmamış öğe bağlantı
      DEĞİL düz metin (boş sayfaya götürmemeli)
- [x] T44 İptal onayı SONUCU yazar → tarayıcıda ölçüldü: "4 bekleyen iş düşer;
      2 çalışan iş kendi hâlinde sürer ve sonucu kaydedilir."
- [x] T45 **"Kaldığı yerden devam et"** yalnızca kesilmiş öğe varken çıkar ve
      üzerinde sayı yazar ("Kaldığı yerden devam et (2 iş)")
- [x] T46 `ui.md`: iki tema ÖLÇÜLDÜ (açık: 5,16–5,86 · koyu: 5,23–10,25;
      denetim sınırı 3,46 ≥ 3), boş ve dolu hâller, geniş (1440) ve dar (1169)
      masaüstü — yatay taşma yok, konsol temiz

## Blok 6 — Kapanış

- [x] T50 README: "Aynı akışı otuz projede çalıştırın" bölümü + öne çıkanlar
      satırı. `AGENTS.md` → Kritik Teknik Notlar'a sınırın ADIM seviyesinde
      uygulandığı ve slot sinyalinin zaten var olduğu yazıldı
- [x] T51 Kapı temiz: `make test-integration`, `make lint`, `npm run typecheck`,
      `npm run lint`, `npm test` (85/85)
- [x] T52 Kabul kriterleri gerçek sistemde doğrulandı (aşağıda)

---

## Kabul doğrulaması (2026-08-15)

Çalışan sistemde, **hiç container açmadan ve maliyet üretmeden**: kuyruk
kayıtlı sürümü OLMAYAN bir akışa bağlandı, böylece her öğe başlatma anında
"yapılandırma eksiği" ile düşüyor. Ölçülen davranışlar:

| Kriter | Ölçüm |
| --- | --- |
| Kuyruk kendiliğinden ilerler | Sıraya konan 4 öğe, hiç uyandırma gönderilmeden emniyet turunda (~15 sn sonra) işlendi |
| Bir öğe düşünce kuyruk DURMAZ (H4) | Dördü de sırayla denendi, hepsi sebebiyle `failed` oldu |
| Toplu iş bitince `done` (T17) | Bekleyen ve çalışan kalmayınca durum `done` |
| Bitmiş işi iptal hata değil (T23) | `affected: 0`, durum `done` kaldı, HTTP 200 |
| Kesilmiş öğe yokken devam | `affected: 0` — düğme zaten çıkmıyor |
| Devam YALNIZCA kesilenleri alır (H5a) | 2 kesilmiş + 2 başarısız öğede `affected: 2`; başarısızlar dokunulmadı, toplu iş `queued`'a döndü |
| Hiç proje seçilmeden başlatılamaz (H1) | HTTP 400 `no_projects` |
| Akış silinmiş | HTTP 404 `workflow_not_found` |
| Tanımsız akış sıraya konmaz | HTTP 409 `no_version` — "önce adımları kaydedin" |
| Sınır değişince yeni sınıra uyar (H2) | Zamanlayıcı testi: sınır 1 iken tek öğe, 3'e çıkınca sonraki turda üç öğe — yeniden başlatma yok |
| Yeniden başlatmaya dayanır (H5) | Backend GERÇEKTEN yeniden başlatıldı: çalışan öğe `interrupted` + sebebi yazıldı (log: `adet=1`), iki bekleyen öğe hayatta kaldı ve açılıştan sonra sırayla başlatıldı |

Doğrulama kayıtları sonrasında silindi; veritabanında toplu iş kalmadı.

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.

**Plan bir kez düzeltildi (kullanıcı uyarısıyla).** İlk hâli yoklamalı (2 sn)
zamanlayıcı öneriyordu. Kod okununca slot boşalma sinyalinin ZATEN üretildiği
görüldü: `runs.Manager.release()` → `cond.Broadcast()`. Var olan bilgiyi yok
sayıp aynı soruyu saniyede bir sormak yerine olay güdümlü tasarıma geçildi.
Dakikalık tur mekanizma değil, kaçan bir uyandırmaya karşı sigorta olarak
kaldı.

**Planın `ErrTooManyRuns` tuzağı yanlış yerdeydi (Blok 2'de düzeltildi).**
Kod okununca görüldü: `workflow.Launcher.Launch` sınıra hiç bakmıyor, sınır
adım seviyesinde uygulanıyor ve dolduğunda akış çalışmasının TAMAMI `failed`
oluyor — yani hata senkron bir dönüş değil, dakikalar sonra veritabanına düşen
bir sonuç. Ayrıca paralel dallı bir akış aynı anda birden fazla slot tutuyor;
"bir öğe = bir slot" da doğru değildi.

Karşılık iki parçaya bölündü: zamanlayıcı hem *çalışan öğe sayısını* hem de
*yöneticideki boş slotu* kontrol ediyor; buna rağmen sınır hatasıyla düşen öğe
`failed` değil `pending` oluyor. Ayrıntı ve elenen alternatif (adım beklesin)
[plan.md](plan.md) → Tuzak bölümünde; spec'in Problem bölümüne de ölçüm
düzeltmesi eklendi.

**Öğe önce sahiplenilir, sonra başlatılır.** `MarkRunning` ikiye ayrıldı:
`Claim` (pending → running) akış çalışması başlatılmadan önce yazılıyor,
`Attach` çalışma kimliğini sonra ekliyor. Ters sırada, iki çağrı arasında düşen
bir süreç öğeyi `pending` bırakır ve açılışta aynı iş ikinci kez başlatılırdı —
branch'e gönderilmiş bir değişiklik habersizce tekrarlanmış olurdu.

**Durum alanları TEXT değil ENUM (Blok 1'de plandan sapıldı).** Plan iki durum
sütununu da `TEXT` yazıyordu; proje konvansiyonu (`AGENTS.md` → Go, db-migrations
skill'i) durum alanlarının Postgres `ENUM` tipi olmasını söylüyor. Zamanlayıcı
öğe durumunu koddan yazacağı için yazım hatasının veritabanında sessizce
durmasındansa orada reddedilmesi tercih edildi. Bedeli görüldü: `CASE` sonucu
metin olduğu için toplu iş durumunu tazeleyen sorgu açık dönüşüm (`::run_batch_status`)
istedi.

`workflow_run_status` yeniden KULLANILMADI: değer listesi bugün aynı görünüyor
ama ona eklenecek bir değer kuyruk için de geçerli sayılırdı.

**Toplu işin durumu türetilmiş bir değerdir.** Öğe güncellemeleriyle aynı
transaction'da öğelerden yeniden hesaplanır (`refreshStatus`). Ayrıca elle
yönetilseydi arada düşen bir süreç, bekleyeni kalmamış bir toplu işi sonsuza
kadar "çalışıyor" gösterirdi. Tek istisna iptal: iptal edilmiş bir iş, son
çalışan öğesi bitince `done`'a dönmez — kullanıcı onu iptal etti. `Resume` ise
durumu bilerek diriltir; `done` ya da `cancelled` kalsaydı `NextPending` sıraya
alınan öğeleri hiç görmez ve kuyruk sessizce donardı.

**Arayüzde üç şey ölçümle değişti.** (1) Bitmiş bir toplu işte "0 bekliyor ·
0 çalışıyor" yazıyordu; sıfır kovalar artık yalnızca iş SÜRERKEN duruyor —
orada kuyruğun boşalışını izlemek haber, bittikten sonra gürültü. (2) "Backend
yeniden başladığında kesildi" kırmızı yazılıyordu; kesilme bir hata değil
açıklama, kırmızı onu derleme hatasıyla aynı ağırlığa koyup gerçekten başarısız
olan satırı görünmez yapıyordu. (3) `ConfirmStrip` bekleme metnini
"Siliniyor…" olarak sabitlemişti — şerit artık silme dışında da kullanılıyor,
`busyLabel` eklendi.

**Doğrulama gerçek koşum başlatmadan yapıldı.** Ekranları görmek için sıraya iş
koymak, otuz container ve gerçek model maliyeti demekti. Bunun yerine geçici
demo kaydı veritabanına yazıldı — hiçbiri zamanlayıcının alacağı durumda
değildi (bekleyen öğeli olan, kayıtlı sürümü OLMAYAN bir akışa bağlandı: en
kötü hâlde container açmadan "yapılandırma eksiği" ile düşerdi). Kayıtlar
doğrulamadan sonra silindi.

**Container elle silinirse sinyal kaybolmuyor** — bu da kullanıcı sorusuyla
ortaya çıktı ve ölçüldü: slot\'u bırakan şey container değil, goroutine\'in
`defer finish()`\'i. Container silinince bağlantı kopuyor, goroutine çözülüyor,
slot bırakılıyor. Tek etkisi gecikme.
