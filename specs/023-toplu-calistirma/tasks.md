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

- [ ] T10 Boş slot varken sıradaki başlar → sahte başlatıcıyla ölçülür
- [ ] T11 **Aynı anda çalışan sayısı sınırı AŞMAZ** → sınır 2, beş öğe;
      tepe eşzamanlılık ölçülür
- [ ] T12 **`ErrTooManyRuns` alan öğe `pending` KALIR** → başarısız
      işaretlenmez; kuyruğun kendini yemediğinin kanıtı
- [ ] T13 Bir öğe düşünce kuyruk DEVAM eder → sonraki öğe başlar
- [ ] T14 `Wake` bloklamaz ve sinyal kaybolmaz → tamponlu kanal, art arda
      çağrıda birikmez
- [ ] T15 Slot boşalınca uyandırma gelir → `runs.Limits.OnSlotFree` bağlanır
- [ ] T16 Emniyet turu kuyruğu ilerletir → uyandırma hiç gelmese de
      bekleyen başlar
- [ ] T17 Toplu iş bitince durumu `done` olur

## Blok 3 — Uzlaştırma, iptal, devam

- [ ] T20 `InterruptRunning` açılışta `running` olanları `interrupted` yapar
      ve kaçını düşürdüğünü loglar
- [ ] T21 `Cancel` yalnızca BEKLEYENLERİ düşürür → çalışan öğe dokunulmaz
- [ ] T22 `Resume` yalnızca `interrupted` olanları `pending` yapar →
      `succeeded` ve `failed` dokunulmaz
- [ ] T23 Bitmiş toplu işi iptal hata DEĞİL → durumu söylenir

## Blok 4 — Uçlar

- [ ] T30 `POST /api/run-batches` → 201, kaç öğe sıraya alındı
- [ ] T31 `GET /api/run-batches` ve `/{id}` → sayılar ve öğeler
- [ ] T32 `cancel` ve `resume` → kaç öğenin etkilendiği yanıtta
- [ ] T33 Hata durumları spec tablosuna uyar → proje seçilmedi, akış yok

## Blok 5 — Arayüz

- [ ] T40 Akış detayında "Çok projede çalıştır": seçim listesi, sayaç
- [ ] T41 Seçim mantığı saf modülde, `npm test` ile
- [ ] T42 Toplu iş ekranı: sayılar + öğe listesi, süren işte tazelenir
- [ ] T43 Öğe kendi akış çalışmasına bağlanır
- [ ] T44 İptal onayı SONUCU yazar: bekleyenler düşer, çalışanlar sürer
- [ ] T45 **"Kaldığı yerden devam et"** yalnızca kesilmiş öğe varken çıkar ve
      üzerinde sayı yazar
- [ ] T46 `ui.md`: iki tema, boş ve dolu hâller

## Blok 6 — Kapanış

- [ ] T50 README: toplu çalıştırma ve kuyruk davranışı
- [ ] T51 Kapı temiz
- [ ] T52 Kabul kriterleri elle doğrulanır; spec "Uygulandı"

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.

**Plan bir kez düzeltildi (kullanıcı uyarısıyla).** İlk hâli yoklamalı (2 sn)
zamanlayıcı öneriyordu. Kod okununca slot boşalma sinyalinin ZATEN üretildiği
görüldü: `runs.Manager.release()` → `cond.Broadcast()`. Var olan bilgiyi yok
sayıp aynı soruyu saniyede bir sormak yerine olay güdümlü tasarıma geçildi.
Dakikalık tur mekanizma değil, kaçan bir uyandırmaya karşı sigorta olarak
kaldı.

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

**Container elle silinirse sinyal kaybolmuyor** — bu da kullanıcı sorusuyla
ortaya çıktı ve ölçüldü: slot\'u bırakan şey container değil, goroutine\'in
`defer finish()`\'i. Container silinince bağlantı kopuyor, goroutine çözülüyor,
slot bırakılıyor. Tek etkisi gecikme.
