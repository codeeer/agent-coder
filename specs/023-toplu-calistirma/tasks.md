# Tasks: Toplu çalıştırma

- **Spec no:** 023 — [spec.md](spec.md) · [plan.md](plan.md)
- **Tarih:** 2026-08-15

Riskli parça başta: zamanlayıcının sınıra uyması ve `ErrTooManyRuns` tuzağı.
Kuyruğun kendini yemesi sessiz bir arıza — otuz projenin çoğu hiç çalışmadan
"başarısız" görünürdü.

---

## Blok 1 — Şema ve depo

- [ ] T01 `000017_toplu_calistirma.sql`: `run_batches`, `run_batch_items`,
      bekleyen indeksi → migration uygulanır
- [ ] T02 `Create` öğeleri sırayla yazar → `position` 0..n-1
- [ ] T03 Aynı proje iki kez REDDEDİLİR → `UNIQUE (batch_id, project_id)`
- [ ] T04 `NextPending` en küçük `position`\'ı verir → sıra eklenme sırasıdır
- [ ] T05 `NextPending` iptal edilmiş toplu işin öğesini VERMEZ
- [ ] T06 `Get` toplu işi öğeleriyle ve proje adlarıyla döner (JOIN)
- [ ] T07 Sayılar doğru: bekleyen · çalışan · tamamlanan · başarısız · kesilen

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

**Container elle silinirse sinyal kaybolmuyor** — bu da kullanıcı sorusuyla
ortaya çıktı ve ölçüldü: slot\'u bırakan şey container değil, goroutine\'in
`defer finish()`\'i. Container silinince bağlantı kopuyor, goroutine çözülüyor,
slot bırakılıyor. Tek etkisi gecikme.
