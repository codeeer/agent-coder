# Görevler: Jira ve Kod Deposu Düğümleri

- **Spec no:** 009 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Uygulandı — Aşama 1 (T01–T42) ve Aşama 2 (T50–T90) tamam

---

## Aşama 1 — çıkış düğümleri

### Düğüm türü kayıt defteri

- [x] T01 `workflow/handler.go` — `NodeHandler` arayüzü, tür kayıt defteri
- [x] T02 `agent` düğümünün doğrulaması `graph.go`'dan kendi handler'ına taşınır;
      **graf testleri değişmeden geçmeli** (davranış aynı, yeri farklı)
- [x] T03 `executor.go` adımı kayıt defterinden çalıştırır; motor mantığı
      (seviye, paralellik, hata, iptal) DEĞİŞMEZ
- [x] T04 `StepResult`'a `url` alanı; `{{ steps.<düğüm>.url }}` şablonda çalışır

### Entegrasyon istemcileri

- [x] T10 `internal/integrations/github/` — PR açma; `httptest` ile testler
      (başarı, çakışma, yetkisiz, branch yok)
- [x] T11 `internal/integrations/jira/` — yorum yazma; `httptest` ile testler
- [x] T12 Kimlik bilgileri `credentials`/`gitprovider` üzerinden çözülür;
      **yanıtlarda ve loglarda görünmez** → sızıntı testi

### Düğümler

- [x] T20 `github.pr` handler — başlık/gövde şablonlu, branch bağlamdan;
      branch yoksa açık hata
- [x] T21 `jira.comment` handler — issue anahtarı ve gövde şablonlu
- [x] T22 Agent düğümüne `autoPush` seçeneği (varsayılan KAPALI)
- [x] T23 Bu düğümler `runs` kaydı üretmez (handler'lar `runs.Manager`'a hiç
      dokunmuyor); rapor `runs` üzerinden toplandığı için rakamlar değişmiyor

### Arayüz

- [x] T30 Tuvale düğüm paleti: "Agent adımı", "PR aç", "Jira'ya yorum yaz"
- [x] T31 Her tür için `NodeInspector` alanları
- [x] T32 Düğüm görselleri türe göre ayrışır (simge + renk değil, simge + etiket)
- [x] T33 Agent düğümünde `autoPush` anahtarı ve ne yaptığının açıklaması

### Doğrulama

- [x] T40 [plan.md](plan.md) doğrulama listesinin 1–5. adımları
- [x] T41 **Gerçek PR açılır** (`agentTestProject` deposunda)
- [x] T42 `make test`, `make test-integration`, `make lint` temiz

## Aşama 2 — Jira tetikleyici

- [x] T50 Migration: `workflow_processed_issues` (workflow_id, issue_key,
      issue_updated_at) + benzersiz kısıt
- [x] T51 `trigger.jira` düğümü + akış başına JQL yapılandırması
- [x] T52 `processIssue` — ortak başlatma yolu; tekrar kontrolü veritabanı
      kısıtıyla (uygulama içi kontrol yarışa açık olurdu)
- [x] T53 Tarama işçisi; aralık ayarlardan okunur (H7)
- [x] T54 `POST /hooks/jira/{token}` — webhook yolu, aynı `processIssue`'ya girer
- [x] T55 Jira alanları `{{ trigger.* }}` bağlamına geçer
- [x] T56 Arayüz: JQL alanı, webhook adresi, son tarama durumu
- [x] T60 [plan.md](plan.md) doğrulama listesinin 6–9. adımları
- [x] T61 **Gerçek Jira task'ı ile uçtan uca:** task → akış → PR → Jira yorumu

## Kapanış

- [x] T90 `AGENTS.md` ve `plans/01` güncellenir; `spec.md` "Uygulandı" olur

---

## Sıra ve gerekçesi

Kayıt defteri (T01–T04) önce: üç yeni düğüm türü ekleneceği için, onları
`executor.go` ve `graph.go` içine `switch` ile serpiştirmek her yeni tür için
aynı iki dosyayı değiştirmek demek olurdu.

T02 özellikle dikkat isteyen adım: agent doğrulaması yer değiştiriyor ama
**davranışı değişmiyor** — mevcut graf testleri hiç değişmeden geçmeli. Geçmezse
taşıma sırasında bir şey kaybolmuş demektir.

Aşama 1 bittiğinde uçtan uca örneğin yarısı çalışıyor olacak: agent kod yazar,
push eder, PR açar, Jira'ya yorum düşer. Kalan tek eksik task'ın nereden geldiği.

---

## Notlar

### T02 doğrulandı — davranış değişmedi

Agent doğrulaması `graph.go`'daki büyük `switch`'ten `kinds.go`'daki kayıt
defterine taşındı. **22 graf ve şablon testi hiç değişmeden geçti**; yalnızca
`AgentNodes()` → `ExecutableNodes()` isim değişikliği bir satırda güncellendi.
Taşıma sırasında bir kural kaybolsaydı testler söylerdi.

Kazanç somut: `graph.go` artık düğüm türlerini bilmiyor. Şablon içeren alanların
listesi de türün kendi tanımından geliyor — `github.pr`'ın başlığı ve gövdesi
eklenince `graph.go` değişmeyecek.

### Jira bağlantısı gerçek veriyle doğrulandı (2026-08-10)

Kimlik bilgisi arayüzden girildi ve kaydetmeden önce `/rest/api/3/myself` ile
doğrulandı. Ardından zincirin tamamı gerçek veriyle sınandı — şifreli kayıt →
çözme → Jira API:

```
site : https://ornek.atlassian.net
  SCRUM-1    ✓ "Test yazılım geliştir"      [Görev / Yapılacaklar]
  SCRUM-2    ✓ "Backlog 1 hatalı yazılım"   [Görev / Yapılacaklar]
```

Bu sırada `GetIssue` öne çekildi (Aşama 2'nin tetikleyicisi zaten buna ihtiyaç
duyacaktı). Jira açıklamayı **ADF ağacı** olarak döndürüyor; agent'ın talimatına
ağaç vermenin anlamı yok, düz metne indirgeniyor. Eski kurulumların düz metin
döndürebildiği durum da kapsanıyor — dördü de testli.

### Jira: Docker yerine Cloud ücretsiz plan

Docker'da çalışan Jira araştırıldı. Resmî `atlassian/jira-software` imajı var
ama **lisans içermiyor** ve 2-4 GB RAM istiyor. Asıl sorun teknik: kodumuz
**Jira Cloud** API v3'e göre yazılı (`base_url` + e-posta + API token basic
auth). Docker'da çalışan sürüm **Data Center**'dır; kimlik doğrulaması ve API
yüzeyi farklıdır. Docker'da test etmek, yazmadığımız API'yi doğrulamak olurdu.

Karar: otomatik testler `httptest` ile (hiçbir servise bağlanmıyor), uçtan uca
doğrulama **Jira Cloud ücretsiz planıyla** (10 kullanıcıya kadar ücretsiz, REST
API dahil).

### v3 düz metin kabul etmiyor

Jira Cloud API v3 yorum gövdesini **Atlassian Document Format** olarak istiyor.
Metin satırlara bölünüp paragraflara çevriliyor; tek paragrafa sıkıştırılsaydı
agent'ın çok satırlı çıktısı Jira'da tek bloğa yapışırdı. Boş gövde ADF'de
geçersiz olduğu için en az bir paragraf garanti ediliyor — testi var.

### GitHub'ın 422'si iki farklı şey anlatıyor

"Zaten açık bir PR var" ile "branch ile hedef arasında fark yok" aynı durum
kodundan geliyor ama farklı eylemler gerektiriyor: ilki "PR'a git", ikincisi
"agent bir şey değiştirmemiş". İkisi ayrı sentinel hataya çevriliyor.

### UÇTAN UCA ÇALIŞTI (2026-08-10)

`agentTestProject` deposu ve `SCRUM-1` issue'su üzerinde, gerçek modelle:

```
AKIŞ: succeeded | maliyet: $0.001121
  Kod          succeeded  $0.001121
  PR aç        succeeded  $0.000000  PR #3 açıldı
      → https://github.com/kullanici/agentTestProject/pull/3
  Jira yorumu  succeeded  $0.000000  SCRUM-1 issue'suna yorum yazıldı
      → https://ornek.atlassian.net/browse/SCRUM-1
```

Ürünün ilk gün anlatılan örneğinin ikinci yarısı çalışıyor: **agent kod yazar →
branch'e gönderir → PR açar → Jira'ya link yorumu düşer.** Kalan tek eksik
task'ın nereden geldiği (Aşama 2).

Maliyet sütunu doğrulamanın kendisi: PR ve Jira adımları **$0,000000** — model
çağırmıyorlar ve rapor rakamlarını bozmuyorlar.

### İki hata bulundu

**1. `runbuild`'in önlemesi gereken hatayı ben yaptım.** İlk deneme "projede git
erişimi tanımlı değil" ile düştü — oysa erişim tanımlıydı. Sebep: `PushRequest`
depo adresi ve kimlik bilgisi istiyor, ben yalnızca `Run` geçmiştim. Aynı
çözümleme (proje → git sağlayıcı → anahtarı çöz) üç yerde ayrı ayrı yazılmıştı;
biri eksik kaldı.

`Builder.RepoAccess` olarak tek yere alındı ve üçü de oradan geçiyor. Paketin
var olma sebebi zaten buydu — kural yazmak yetmiyor, uygulamak gerekiyor.

**2. PR adresi hiçbir yerde görünmüyordu.** Akış "PR aç ✓" diyordu ama adres
kayıp: agent olmayan adımların `runs` kaydı yok (bilerek), sonuçları da hiçbir
yere yazılmıyordu. En işe yarar bilgi uçuyordu.

Migration `000005` ile `workflow_steps` tablosuna `result_text` ve `result_url`
eklendi; adım kartında bağlantı olarak görünüyor. Ekran görüntüsüne bakılmasaydı
fark edilmezdi.

### "Hangi branch'ten PR?" sorusu kaydetme anına çekildi

PR düğümünün en belirsiz noktası buydu. Üç seçenek vardı: kullanıcıya her
seferinde yazdırmak, çalışma anında tahmin etmek, ya da kaydetme anında karara
bağlamak.

Üçüncüsü seçildi. `headBranch` boş bırakılabiliyor; doğrulama o zaman
**gönderim yapan ata adımları sayıyor**:

- Sıfırsa: "PR açılacak branch yok — önceki bir agent adımında gönderim açılmalı"
- Tam bir taneyse: sorun yok, çalışma anında o kullanılır
- Birden fazlaysa: "hangisinden PR açılacağı belirtilmeli" (adları listelenir)

Böylece yaygın durumda (tek zincir) kullanıcı hiçbir şey yazmıyor, belirsiz
durumda ise hata **para harcanmadan önce** çıkıyor. Çalışma anında tahmin
yapılmıyor — dört testi var.

### `autoPush` varsayılan KAPALI

Agent adımının ürettiği değişikliğin branch'e gönderilmesi açıkça seçilmeli.
Bir akışın habersizce depoya yazması sürpriz olurdu; PR açan akışta kullanıcı
bunu bilerek açar. Değişiklik yoksa gönderim de yok — boş branch gürültüdür.

### Bulunan hata — rapor testi saat dilimi sınırında kırılıyordu

`TestSummaryDailyFillsGaps` gece yarısından sonra kırıldı: test kaydı **UTC gün
ortasına** sabitliyordu, rapor ise günleri **Europe/Istanbul** takvimine göre
bölüyor. UTC 22:05 iken İstanbul 01:05 — yani "bugün eklenen kayıt" raporun dünü
oluyordu.

Ürün doğruydu, test kırılgandı. Zaman damgası artık şu andan geriye sayılarak
üretiliyor; hangi saat diliminde çalışırsa çalışsın doğru güne düşüyor.

Bu, `secrets` testindeki kararsızlıkla aynı sınıf hata: **testin kendi
varsayımı**, ürünün değil.

---

### T60 — Aşama 2 doğrulama listesi sonuçları

Gerçek Jira sitesi (`SCRUM-2 · "Backlog 1 hatalı yazılım"`) ve gerçek depo
(`agentTestProject`) ile ölçüldü.

| # | Adım | Sonuç |
|---|------|-------|
| 6 | JQL eşleşince akış kendiliğinden başlar | ✓ tarama: `found=1, started=1` |
| 7 | Aynı task ikinci taramada yeniden başlamaz | ✓ üç tur boyunca `started=0` |
| 8 | Webhook yolu aynı korumadan geçer | ✓ ikinci çağrı `started:false` |
| 9 | Task güncellenirse yeniden işlenir | ✓ farklı `updated` → yeni çalışma |
| — | **Uçtan uca (T61):** task → agent → push → PR → Jira yorumu | ✓ PR #4, `SCRUM-2`'ye yorum, $0.0014 |

### Ölçüm 3 — başlatılamayan task kalıcı olarak bloke oluyordu

İlk canlı denemede webhook `502` döndü (`workflow_trigger_kind` enum'una `jira`
eklenmemişti). Asıl sorun bu değildi: **ikinci çağrı da başlatmadı.** Sebebi
sıralamaydı — işaret akış başlatılmadan önce konuyor, başlatma hata verince
işaret geride kalıyordu. Task hiç çalışmadan "işlendi" sayılmıştı.

Sıralamayı tersine çevirmek (önce başlat, sonra işaretle) yarışı geri getirirdi.
Bu yüzden işaret önce konuyor, başlatma başarısız olursa `UnmarkProcessed` ile
geri alınıyor — ve yalnızca **çalışmaya bağlanmamış** işaret siliniyor, ki
gerçekten başlamış bir akışın işareti yanlışlıkla temizlenmesin.

Not: hatanın kendisi (eksik enum değeri) tek başına görünürdü; onu bulduran şey
**ikinci çağrının sessizce başarısız olması** oldu.

### Ölçüm 4 — akış kendi kendini tetikliyordu

Akış Jira'ya yorum yazınca task'ın güncellenme zamanı değişiyor. Tekrar-işleme
koruması bu zamanı anahtar olarak kullandığından, akış kendi bıraktığı izi yeni
bir iş sanıp yeniden başlıyordu. Ölçüldü: yorum yazıldıktan sonra aynı webhook
çağrısı `started:true` döndü.

Beş dakikalık tarayıcıyla bunun anlamı **sonsuz döngü**: her turda yeni bir PR,
yeni bir yorum, yeni bir model maliyeti. Kimse bir şey yapmadan.

Çözüm, yorum adımının kendi ürettiği güncellemeyi sahiplenmesi: yorumdan hemen
sonra task yeniden okunuyor ve oluşan yeni güncellenme zamanı işlenmiş olarak
kaydediliyor. Yalnızca akışı BU task başlattıysa yapılıyor — başka bir issue'ya
yorum yazmak tetikleyiciyle ilgisiz bir iştir.

Kabul edilen sınır: bir insan tam aynı anda task'ı düzenlerse aynı zaman
damgasını paylaşır ve o düzenleme yutulur. Alternatif — kendi değişikliğimize
tepki vermek — çok daha kötü.

### Ölçüm 5 — kapalı küme, açık varsayılan

Tetikleyici adı iki ekranda `kind === "webhook" ? "dışarıdan" : "elle"` diye
yazılmıştı. `jira` eklenince ikisi de sessizce **"elle"** göstermeye başladı:
Jira'nın başlattığı dört çalışma listede "elle" yazıyordu. Ekran görüntüsünde
görüldü, tip kontrolünden temiz geçmişti.

Artık `TRIGGER_TEXT` eksiksiz bir `Record<TriggerKind, …>`: yeni bir tetikleyici
türü eklendiğinde TypeScript eksik satırı derleme zamanında söylüyor.

### Not — görsel doğrulama aracı kaybolmuştu

`scripts/screenshot.mjs` çalışmadı: `playwright` paketi kurulu değildi (tarayıcı
ikilileri duruyordu). Araç ancak elle kurulduğunda çalışıyorsa bir sonraki
oturumda yine yok olur — kök dizine yalnızca geliştirme araçları için bir
`package.json` eklendi.

### Not — tarama aralığı ayardır, hata değildir

`jira.poll_interval_minutes` (varsayılan 5, aralık 1–1440) ve `jira.scan_limit`
(varsayılan 20, aralık 1–200) ayar kayıt defterindedir; H7 gereği kodda sabit
değer bırakılmadı. Etkin bir akış bu aralıkla taranmaya devam eder — eşleşen
task zaten işlenmişse hiçbir şey başlamaz. Durdurmanın yolu akışı pasife almak.

Ölçüm 4'teki döngüde 5 dakika **hatanın kendisi değil, katsayısıydı**: döngü
aralıktan bağımsız olarak vardı, aralık yalnızca ne kadar hızlı para yakacağını
belirliyordu.

### Ölçüm 6 — "akışı pasife alın" diyordum, öyle bir düğme yoktu

Tarama aralığını belgeye yazarken durdurma yolunu "akışı pasife almak" diye
tarif ettim. `is_active` alanı en baştan vardı, backend onu doğru kullanıyordu
(tarama pasif akışı atlıyor, webhook uçları `409` dönüyor), liste sayfası
**"pasif" rozetini çizmeye hazırdı** — ama alanı değiştirecek hiçbir arayüz
yoktu. `api.workflows.update` tanımlıydı ve hiçbir yerden çağrılmıyordu, yani
o rozet hiçbir koşulda görünemezdi.

Akış ekranına **Duraklat / Etkinleştir** eklendi. Anlamı bilinçli olarak dar:
otomatik tetikleme (Jira taraması + dış adresler) durur, elle çalıştırma açık
kalır.

Ders: **arka uçta bir alanın olması, kullanıcının ona erişebildiği anlamına
gelmiyor.** Belgeye bir kullanım yolu yazarken o yolun ekranda karşılığı olup
olmadığı ayrıca kontrol edilmeli — burada bir kullanıcı sorusuyla ortaya çıktı.
