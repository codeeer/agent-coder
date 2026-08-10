# Görevler: Workflow Motoru

- **Spec no:** 007 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı

---

## Graf çekirdeği (arayüz ve veritabanı olmadan test edilebilir)

- [x] T01 `internal/workflow/graph.go` — `Graph`, `Node`, `Edge` tipleri; JSON çözümleme
- [x] T02 `Validate()` — plan.md'deki yedi kural → derlenir
- [x] T03 `Levels()` — topolojik seviyeler; döngüde hata döner
- [x] T04 Graf testleri: döngü, çift tetikleyici, erişilemez düğüm, var olmayan kenar
      hedefi, boş talimat, **ata olmayan düğüme şablon referansı** → `make test` yeşil
- [x] T05 `internal/workflow/template.go` — `{{ input }}`, `{{ trigger.* }}`,
      `{{ steps.<düğüm>.output|diff|branch }}`; bilinmeyen referans **hata**
- [x] T06 Şablon testleri: çözümleme, bilinmeyen referans, süslü parantez içeren
      metin, boşluklu yazım → `make test` yeşil

## Veri katmanı

- [x] T10 Migration `000004_workflow.sql` — beş tablo + durum tipleri + indeksler
      → `make migrate` hatasız, geri alınabilir
- [x] T11 `internal/workflow/store.go` — akış CRUD, sürüm kaydetme (doğrulamadan
      geçmeden yazmaz), çalışma ve adım kayıtları → derlenir
- [x] T12 `RecoverInterrupted` akış çalışmalarını da kapatır
- [x] T13 Entegrasyon: sürüm numarası artışı, **akış değişince geçmiş çalışmanın
      sürümü sabit kalır**, adım–çalıştırma bağı → `make test-integration` yeşil

## Motor

- [x] T20 `runs.Manager.Execute` — bloklayan giriş noktası; gövde `Start` ile ortak,
      eşzamanlılık sayacı ve timeout aynı yerden geçer → derlenir
- [x] T21 `Execute` gövdeyi `Start` ile paylaşır (`begin`); mevcut Manager testleri
      sınır/iptal davranışının bozulmadığını doğruluyor
- [x] T22 `internal/workflow/executor.go` — seviye seviye yürütme, paralel grup,
      bağlam biriktirme, hata halinde durdurma ve `skipped` işaretleme
- [x] T23 Olaylar `events.Bus`'a hem adım hem akış kanalına düşer
- [x] T24 Executor testleri (sahte runner ile): sıralı akış, paralel seviye,
      **hatada sonrakiler skipped**, iptal, şablonun bir sonraki adıma geçmesi
- [x] T25 Entegrasyon: gerçek veritabanıyla üç adımlı zincir (`TestExecutor_UcAdimliZincir`),
      her adım `runs` kaydına bağlanıyor. Container katmanı sahte runner ile
      temsil ediliyor; gerçek uçtan uca T90'da

## HTTP

- [x] T30 `httpapi/workflows.go` — CRUD + sürüm kaydetme (geçersiz graf 400)
- [x] T31 `httpapi/workflowruns.go` — başlat, liste, detay (adımlarıyla), iptal
- [x] T32 SSE ucu — önce geçmiş, sonra canlı
- [x] T33 `POST /hooks/{token}` — dışarıdan tetikleme; yanlış adres 404, gövde
      `trigger` bağlamına geçer
- [x] T34 `main.go` wiring + `RecoverInterrupted` çağrısı
- [x] T35 Tetikleme anahtarı yalnızca akış uçlarında dönüyor (çalıştırma listesi,
      rapor ve olay akışında yok). `/hooks/{token}` var olmayan ve yanlış anahtar
      için AYNI 404'ü veriyor — hangi akışların var olduğu sızmıyor. Not: v1'de
      kimlik doğrulama yok, "sahibi" = uygulamaya erişen herkes

## Arayüz

- [x] T40 `lib/types.ts` + `lib/api.ts` — akış tipleri ve uçları
- [x] T41 `/workflows` — liste, oluştur, sil, son çalışma durumu
- [x] T42 `/workflows/[id]` — adım listesi editörü (ekle/sil/sırala, agent + model +
      talimat), şablon değişkeni yardımı, kaydet → yeni sürüm
- [x] T43 Geçersiz graf hatası kullanıcıya **hangi düğümde ne yanlış** diye gösterilir
- [x] T44 `/workflows/[id]/runs/[runId]` — canlı ilerleme, adım durumları, maliyet,
      adımdan çalıştırma detayına bağlantı
- [x] T45 `/runs` satırında "X akışının Y adımı" rozeti
- [x] T46 Tetikleme adresi gösterimi + yenileme
- [x] T47 Üç durum (yükleniyor/hata/boş) tüm yeni ekranlarda

## Doğrulama ve kapanış

- [x] T90 Doğrulama listesinin dokuz adımı yürütüldü (aşağıdaki tablo)
- [x] T91 Görsel doğrulama: akış listesi, editör ve çalışma ekranı açık ve koyu temada
- [x] T92 `make test`, `make test-integration` (15 paket) yeşil; `make lint` temiz
- [x] T93 İptal ve yeniden başlatma sonrası runner container'ı ve volume artığı yok
- [x] T94 `AGENTS.md` ve `plans/01` güncellendi; `spec.md` "Uygulandı"

---

## Sıra ve gerekçesi

Graf çekirdeği (T01–T06) **önce** geliyor çünkü veritabanı ve arayüz olmadan tamamen
test edilebilir; motorun en çok hata barındıran kısmı orası ve en ucuz doğrulama yeri.

Motor (T20–T25) arayüzden önce bitiriliyor — K1 kararının gerekçesi buydu: motor
çalışır görülmeden arayüz yazmak, bir tasarım hatasında ikisini birden değiştirmek
demek.

### Not 1 — şablon doğrulaması kaydetme anına çekildi

Spec "adım çıktısı geçmediğinde akış açık bir hata verir, sessizce boş metinle
çalışmaz" diyor. Bunu yalnızca çalışma anında yapmak, kullanıcının hatayı ancak
akış yarıya geldiğinde ve para harcandıktan sonra görmesi demekti.

Doğrulama iki yere birden kondu:

- **Kaydetme anında:** `{{ steps.X.output }}` referansı X'in bu adımın ATASI
  olmasını gerektirir. Sonraki bir adıma veya paralel bir kardeşe bakan referans
  akışı kaydettirmez — paralel kardeş özellikle sinsi, çünkü bazen çalışır
  bazen çalışmaz.
- **Çalışma anında:** çözümlenemeyen referans yine hata döner (savunma katmanı).

### Not 2 — `{{ }}` her zaman değişken değildir

Referans kalıbı bilerek dar tutuldu (harf, rakam, alt çizgi, nokta, tire).
Kullanıcının talimatında geçen `{{ "a": 1 }}` gibi bir JSON örneği veya
`{{ iki kelime }}` gibi bir metin referans SAYILMAZ, olduğu gibi kalır. Aksi
halde agent'a verilen kod örneği sessizce bozulurdu; testi var.

### Not 3 — maliyet adımda tutulmuyor

Adım kaydında maliyet ve token alanı YOK; ikisi de `runs` kaydından okunuyor
(`workflow_steps LEFT JOIN runs`). Adıma kopyalansaydı aynı sayı iki yerde
dururdu ve er geç ayrışırdı — özellikle bir çalıştırma sonradan güncellenirse.

Akışın toplamı da saklanmıyor, adımlardan hesaplanıyor. Aynı gerekçe.

### Not 4 — 'skipped' yalnızca akışta var

`runs.status` listesine 'skipped' EKLENMEDİ. Hiç çalışmamış bir adımın `runs`
kaydı olmaz; oraya sahte "atlandı" satırları yazmak rapor rakamlarını kirletirdi
(çalıştırma sayısı, başarı oranı). Atlanmışlık akışa özgü bir durumdur ve yeri
`workflow_steps`.

### Not 5 — geri alma testi sürüme sabitlendi

`TestMigration004_GeriAlinabilir` `MigrateUpTo(4)` kullanıyor, `MigrateUp`
değil — 002'de öğrenilen ders: "son migration" hangisiyse onu test etmek,
yeni migration eklendiğinde testin sessizce anlamını yitirmesi demek. Test
tabloların yanında ENUM tiplerinin de düştüğünü sınıyor; kalsalardı ileri gitmek
"type already exists" ile patlardı.

### Not 6 — `buildRunInput` HTTP katmanından çıkarıldı

Motorun da aynı çözümlemeye ihtiyacı vardı: proje, agent, sağlayıcı, çözülmüş
anahtar, depo erişimi. HTTP handler'ının içinde kalsaydı ya kopyalanacaktı (biri
güncellenip diğeri unutulur) ya da motor handler'a bağımlı olacaktı.

Yeni paket `internal/runbuild`. Bu aynı zamanda projenin kendi kuralını yerine
getiriyor: "handler'lar ince tutulur, iş mantığı internal/ altında durur."

### Not 7 — iptal başarısızlık değildir

Motorun ilk hali iptal edilen akışı `failed` işaretliyordu: iptal edilen adım
hata döndürüyor, motor da "bir adım hata verdi" diye akışı başarısız sayıyordu.
Kullanıcının bilerek durdurduğu her akış geçmişte "başarısız" görünecekti — ve
rapordaki başarı oranını da bozacaktı.

Sıra değişti: seviye bittikten sonra ÖNCE context iptal edilmiş mi bakılıyor,
sonra hataya. Testi var (`TestExecutor_Iptal`).

### Not 8 — yabancı anahtar kısıtı testi düzeltti

Sahte runner rastgele bir UUID döndürüyordu; `workflow_steps.run_id` yabancı
anahtarı bunu reddetti. Kısıt haklıydı: gerçek hayatta o kimlik var olan bir
çalıştırmayı gösterir. Sahte runner artık gerçek bir `runs` kaydı üretiyor.

Bu, testin gerçeğe uymadığı yeri kısıtın yakalamasıydı — tersi olsaydı (kısıt
yok, test geçer) hata üretimde çıkardı.

### Not 9 — canlı akışta ikinci bir olay tablosu yok

Çalıştırma SSE'si önce veritabanındaki geçmişi gönderiyor. Akış SSE'si
GÖNDERMİYOR — ve buna gerek de yok: akışın o ana kadarki durumu zaten adım
kayıtlarında duruyor, istemci onu `GET /api/workflow-runs/{id}` ile alıyor.
Olayları ayrıca bir `workflow_run_events` tablosunda saklamak, aynı bilgiyi iki
yerde tutmak olurdu. "Sayfa yenilense de ilerleme kaybolmaz" kriteri adım
durumlarıyla karşılanıyor.

### Not 10 — uçtan uca canlı doğrulama yapıldı

Webhook ile tetiklenen iki adımlı bir akış gerçek modelle çalıştırıldı:

```
durum      : succeeded
tetikleyici: webhook {'input': 'depoyu incele'}
maliyet    : $0.002019 | token: 13221
  Analiz     succeeded  anthropic/claude-haiku-4.5  $0.001057
  İnceleme   succeeded  anthropic/claude-haiku-4.5  $0.000962
```

İki şey doğrulandı:

1. **Şablon gerçekten aktarıldı.** İkinci adımın talimatı birincinin çıktısını
   içeriyordu ("Önceki adımın özeti şu: ## Özet Bu depo, basit bir Hello World…").
2. **Rapor koda dokunmadan akışları kapsadı.** `GET /api/reports/summary` akış
   adımlarını da sayıyor — "adım = çalıştırma" kararının asıl karşılığı buydu.

Ayrıca geçersiz graf 400 ile ve düğüm bazında kusur listesiyle dönüyor:

```json
{"error":{"code":"invalid_graph","problems":[
  {"nodeId":"a1","message":"{{ steps.a2.output }}: `a2` bu adımdan önce çalışmıyor…"}]}}
```

### Not 11 — düğüm kimliği adla birlikte DEĞİŞMEZ

Şablon referansları düğüm kimliğine bakıyor (`{{ steps.analiz.output }}`).
Editörde adım yeniden adlandırıldığında kimlik değişseydi, diğer adımların
talimatındaki referanslar sessizce kırılırdı. Kimlik bir kez addan türetiliyor
(Türkçe harfler katlanarak) ve sonra sabit kalıyor; ekranda rozet olarak
gösteriliyor ki kullanıcı neye referans vereceğini bilsin.

Referans ekleme düğmeleri yalnızca **önceki** adımları listeliyor — sonrakine
referans zaten kaydettirilmiyor, seçenek olarak sunmak kullanıcıyı hataya davet
etmek olurdu.

### Not 12 — adım süresi sıfır görünüyordu

İlk halde `started_at`, çalıştırma kaydı elde edildikten SONRA yazılıyordu; kayıt
ise ancak iş bittiğinde dönüyor. Sonuç: her adımın süresi "0 sn". Ekran
görüntüsüne bakınca çıktı.

`MarkStepRunning` (durum + başlangıç zamanı) ile `LinkStepRun` (çalıştırma bağı)
ayrıldı. Durum artık iş BAŞLAMADAN yazılıyor — bunun ikinci bir faydası da var:
kullanıcı adımın çalıştığını anında görüyor, bitene kadar "sırada" görmüyor.

### Not 13 — arayüz canlı akıştan DURUM kurmuyor

Akış ekranı SSE olaylarını yalnızca "bir şey değişti" sinyali olarak kullanıyor
ve her olayda kaydı yeniden çekiyor. Durumu olay akışından kurmaya çalışmak,
adım durumlarının iki kaynaktan (veritabanı + olaylar) gelmesi ve ayrışması
demek olurdu. Sayfa yenilendiğinde de tek kaynak yerinde duruyor.

### T90 — doğrulama listesi sonuçları

| # | Adım | Sonuç |
|---|------|-------|
| 1 | Çok adımlı akış uçtan uca | ✓ webhook ile iki adım, $0,002019 |
| 2 | `{{ steps.X.output }}` dolu geliyor | ✓ ikinci adımın talimatında birincinin özeti okundu |
| 3 | Paralel adımlar aynı anda | ✓ ikisi de 20:49:34'te başladı, **5 sn örtüşme** |
| 4 | Geçersiz graf kaydedilemiyor | ✓ döngü, erişilemez adım, agent yok, iki tetikleyici — dördü de reddedildi |
| 5 | Hatalı adım → akış durur, sonrakiler atlanır | ✓ `failed` + `skipped`, hatalı adım yine çalıştırmaya bağlı |
| 6 | İptal → adım kesilir, container kalmaz | ✓ `cancelled`, artık container/volume yok |
| 7 | Webhook çalışıyor, yanlış adres 404 | ✓ |
| 8 | Rapor akış adımlarını kapsıyor | ✓ kod değişikliği gerekmedi |
| 9 | Yeniden başlatma → `interrupted` | ✓ (bkz. Not 14) |

### Not 14 — "kullanıcı durdurdu" ile "sunucu kapandı" ayrıldı

Doğrulamanın 9. adımı ilk denemede **`cancelled`** döndü. Sebep: sunucu kapanırken
motorun context'i düşüyor, motor da bunu iptal sayıyordu. Sonuç, kullanıcıya
"bu akışı sen durdurdun" demek olurdu — oysa sunucu yeniden başlamıştı.

`stopped()` artık ikisini ayırıyor: kapatma sinyali gelmişse `interrupted`
("sunucu kapandığı için akış kesildi"), gelmemişse `cancelled`. Geçmiş artık
neyin olduğunu doğru anlatıyor.

`RecoverInterrupted` yine duruyor ama artık yalnızca **sert ölüm** (SIGKILL,
çökme) için: orada hiçbir şey yazılamadan süreç bittiği için açılışta toparlıyor.

### Not 15 — ölçmeden önce dağıtmak

Paralellik ölçümü ilk denemede "SIRAYLA ÇALIŞTI" dedi ve adım süreleri sıfır
göründü. Kod doğruydu; `make up` yapmadığım için **eski imaj** çalışıyordu.
Doğrulama, ölçülen sürümün ölçülmek istenen sürüm olduğunu teyit etmeden
yapılmamalı.
