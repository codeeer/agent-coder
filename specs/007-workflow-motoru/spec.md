# Spec: Workflow Motoru — Agent'ları Birbirine Bağlamak

- **Spec no:** 007
- **Tarih:** 2026-08-09
- **Durum:** Uygulandı
- **Faz:** 3 — [plans/01](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Bugün bir agent **tek başına, elle** çalıştırılıyor. Ürünün asıl vaadi ise adımların
birbirine bağlanması: "task'ı analiz et → kodu yaz → incele → gönder".

Kullanıcı bunu bugün ancak şöyle yapabiliyor: analyst'i çalıştır, çıktısını **kopyala**,
coder'ı aç, çıktıyı **yapıştır**, çalıştır, diff'i al, reviewer'a **yapıştır**. Üç
sorun var:

1. **Elle taşıma.** Her adım arasında kopyala-yapıştır var; kullanıcı akışın kendisi
   oluyor. Bir adımı unutmak veya yanlış çıktıyı yapıştırmak sessizce yanlış sonuç verir.
2. **Tekrarlanamıyor.** Aynı akış ikinci bir task için baştan elle kuruluyor. "Bizim
   kod inceleme sürecimiz" diye kaydedilebilecek bir şey yok.
3. **Bir kere kurup bırakılamıyor.** Akış ancak kullanıcı başında dururken ilerliyor;
   dışarıdan (bir olayla) tetiklenemiyor.

## Amaç

Kullanıcı, adımları birbirine bağlı **kaydedilebilir bir akış** tanımlayabilsin; bu
akışı elle veya dışarıdan gelen bir tetikleyiciyle başlatabilsin; ilerlemesini adım
adım izleyip her adımın çıktısını, ürettiği değişikliği ve maliyetini görebilsin.

Her adım **kendi agent'ı ve kendi modeliyle** çalışır — analiz için ucuz bir model,
kod yazımı için güçlü bir model kullanmak akışın doğal parçası olmalı.

## Kullanıcı hikâyeleri

1. Kullanıcı olarak, bir projeye bağlı **akış tanımlayabilmeliyim**: sıralı adımlar,
   her biri bir agent + bir model + bir talimat.
2. Kullanıcı olarak, bir adımın çıktısını **sonraki adımın talimatında
   kullanabilmeliyim** — analiz metnini kod adımına, üretilen değişikliği inceleme
   adımına geçirmek gibi.
3. Kullanıcı olarak, akışı **elle başlatabilmeli** ve başlarken bir giriş metni
   (task açıklaması) verebilmeliyim.
4. Kullanıcı olarak, akışın **canlı ilerlemesini** görebilmeliyim: hangi adım
   bekliyor, hangisi çalışıyor, hangisi bitti.
5. Kullanıcı olarak, bir adım **başarısız olduğunda** ne olduğunu ve sonraki adımların
   neden çalışmadığını görebilmeliyim.
6. Kullanıcı olarak, akışın **toplam maliyetini ve süresini** görebilmeliyim; rapor
   sayfası bu çalışmaları da kapsamalı.
7. Kullanıcı olarak, bir akışı **dışarıdan tetikleyebilmeliyim** (bir adres çağrılınca
   başlasın) ki başka bir sistemle bağlayabileyim.
8. Kullanıcı olarak, bir akışı **durdurabilmeliyim**; süren adım kesilmeli, sonrakiler
   başlamamalı.
9. Kullanıcı olarak, akışı değiştirdiğimde **geçmiş çalışmalar değişmemeli** — hangi
   tanımla çalıştığını doğru göstermeli.

## Kabul kriterleri

- [x] Üç adımlı bir akış (analiz → kod → inceleme) uçtan uca çalışır; her adım
      **farklı modelle**.
- [x] Adım çıktısı sonraki adımın talimatına geçer; geçmediğinde akış açık bir hata
      verir, sessizce boş metinle çalışmaz.
- [x] Birbirine bağlı olmayan adımlar **aynı anda** çalışır; bağlı olanlar sırayla.
- [x] Geçersiz akış **kaydedilemez**: döngü, birden fazla başlangıç, hiçbir yere
      bağlanmayan adım, var olmayan agent veya model.
- [x] Bir adım başarısız olunca akış durur; sonraki adımlar "atlandı" olarak
      işaretlenir — belirsiz kalmaz.
- [x] Akış çalışırken canlı izlenebilir; sayfa yenilense de o ana kadarki ilerleme
      kaybolmaz.
- [x] Akış durdurulabilir; duran akış artık container veya kaynak tutmaz.
- [x] Akış tanımı değiştirilse bile geçmiş çalışma **neyle çalıştığını** doğru gösterir.
- [x] **Rapor sayfası akış adımlarını da kapsar** ve rakamlar tutar; rapor kodunda
      değişiklik gerekmemeli.
- [x] Eşzamanlı çalışma sınırı akış adımları için de geçerlidir; bir akış tek başına
      bütün kapasiteyi yutamaz.

## Kapsam dışı

- **Sürükle-bırak tuval.** Bu spec akışın *motorunu* ve verisini kurar; tuval
  editörü Faz 4. Bu fazda akış listelenip çalıştırılabilir ve izlenebilir, ama
  düzenleme basit bir form/metin üzerinden olur.
- **Jira ve kod deposu entegrasyonları.** Task çekme, PR açma, yorum yazma Faz 5.
  Bu fazda tetikleyici "elle" ve "dışarıdan çağrı" ile sınırlı.
- **Koşullu dallanma ve döngüler.** İlk sürümde adımlar doğrusal veya paralel
  bağlanır; "şu koşulda şu dala git" bir sonraki işe bırakılıyor. Gerekçe: koşul
  ifadelerinin kendi dili, doğrulaması ve hata modeli var; motoru önce en basit
  haliyle çalıştırıp gerçek ihtiyacı görmek daha sağlıklı.
- **Yeniden deneme (retry).** Başarısız adım tekrarlanmaz. Model çağrıları pahalı ve
  yan etkili (kod değiştirir); körlemesine tekrar etmek zararlı olabilir.
- **Akış sürümleri arasında geçiş / geri alma.** Geçmiş çalışma hangi tanımla
  çalıştığını gösterir, ama eski sürüme "dönmek" bu fazda yok.

## Kararlar

Aşağıdakiler başlamadan önce soruldu ve karara bağlandı.

**K1 — Faz kapsamı: önce motor, tuval sonra.** Bu fazda veri modeli, doğrulama,
çalıştırma motoru, canlı izleme ve dışarıdan tetikleme var; düzenleme basit bir
adım listesi formuyla. Sürükle-bırak tuval Faz 4'te bunun üzerine gelir ve veri
modeli değişmez. Gerekçe: motoru çalışır görmeden arayüz yazmak, bir tasarım
hatasında ikisini birden değiştirmek demek.

**K2 — Bir adım başarısız olunca akış tamamen durur.** Kalan adımlar "atlandı"
olarak işaretlenir. Yarım kalmış bir akışın ürettiği kod değişikliği güvenilmez;
"bir kısmı çalıştı" hali kullanıcıyı yanıltır. Adım bazında "hata olsa da devam et"
seçeneği ileride eklenebilir.

**K3 — Koşullu dallanma bu fazda YOK.** Orijinal yol haritasında Faz 3'te
listelenmişti, kapsam dışına alındı: koşul ifadelerinin kendi dili, doğrulaması ve
hata modeli var. Motor önce en basit haliyle çalıştırılıp gerçek ihtiyaç görülecek.
Veri modeli sonradan eklenebilecek şekilde kurulur.

## Kalan açık sorular

Bunlar davranışı daha az etkiliyor; önerilerle ilerlenecek.

**S1 — Adım çalışmaları "Çalıştırmalar" listesinde tek tek görünsün mü?**
*Önerim: evet.* Her adım zaten bir agent çalıştırması; ayrı bir yerde tutulursa
geçmiş ikiye bölünür ve rapor iki kaynağı toplamak zorunda kalır. Listede hangi
akışa ait olduğu belirtilir.

**S3 — Dışarıdan tetikleme nasıl korunacak?**
*Önerim: her akış için tahmin edilemez bir adres (gizli anahtar içeren) üretilsin.*
Kimlik doğrulama v1'de yok; adresin kendisi anahtar olur. Adres yeniden üretilebilir.

**S4 — Bir akış aynı anda birden fazla kez çalışabilsin mi?**
*Önerim: evet ama uyarıyla.* Aynı depo üzerinde iki akış aynı anda çalışırsa her biri
kendi kopyasında çalıştığı için teknik sorun yok; ancak ikisi de aynı branch'e
göndermek isterse çakışır. Başlatırken "bu akışın çalışan bir örneği var" uyarısı
gösterilir, engellenmez.

## Karar geçmişi

### 2026-08-12 — akışın projesi mühür değil, varsayılan oldu

**Sorun.** Kullanıcı hikâyesi 1 "bir projeye bağlı akış tanımlayabilmeliyim"
diyordu ve şema bunu harfiyen uyguluyordu: `workflows.project_id NOT NULL`,
oluşturmada zorunlu, güncellemede değiştirilemez, çalıştırmada seçilemez.
Çalışmanın projesi de kendi kaydında değil, akıştan JOIN ile okunuyordu.

Sonuç: aynı süreci yirmi projede işletmek isteyen kullanıcı **yirmi ayrı akış
kaydı** açmak zorundaydı. Tanım tekti, kayıt yirmi taneydi; biri güncellenip
diğerleri unutulduğunda hangisinin doğru olduğu anlaşılmıyordu.

**Karar.**

1. **Akışın projesi VARSAYILAN oldu.** Sütun `NOT NULL` kaldı; kaldırılsaydı
   Jira taraması, webhook ve MCP tetikleyicileri projesiz kalırdı — dışarıdan
   gelen bir olayın hangi projeye ait olduğuna karar verecek bilgisi yok.
   Onlar varsayılanı kullanmaya devam ediyor, yani mevcut kurulumların hiçbiri
   değişmedi.

2. **Proje çalıştırma anında değiştirilebiliyor.** `POST /api/workflows/{id}/runs`
   gövdesi isteğe bağlı `projectId` alıyor; boşsa varsayılan kullanılıyor.
   Geçersiz kimlik FK hatasına bırakılmıyor, 400/404 ile ayrıştırılıyor.

3. **`workflow_runs.project_id` kendi sütunu oldu** (migration 000012). JOIN ile
   akıştan okunmaya devam etseydi, akışın varsayılanı sonradan değiştirildiğinde
   GEÇMİŞ çalışmaların projesi de geriye dönük değişirdi. Aynı sorun sürüm için
   zaten `version_id` ile çözülmüştü; proje de aynı yere oturdu. Rapor
   sorgularındaki proje süzgeci de bu sütuna çevrildi (`flowScope`) — akışın
   varsayılanına bakılsaydı başka projede açılmış bir PR yanlış projenin
   raporuna sayılırdı.

4. **Proje ekranlarda görünür oldu.** Akış çalışma detayında ilk alan, "Son
   çalışmalar" listesinde ise YALNIZCA varsayılandan farklıysa yazılıyor: her
   satırda aynı değeri tekrarlamak, gerçekten başka bir projede koşan satırı
   görünmez yapardı.

**Kapsam dışı.** Düğüm başına proje seçimi eklenmedi; bir akışın adımlarının
farklı projelerde koşması istenmiyor.

### 2026-08-12 — akış ve çalıştırma kayıtları silinebiliyor

**Sorun.** `DELETE /api/workflows/{id}` yazılmıştı ama arayüzde düğmesi yoktu ve
hiç çalıştırılmamış görünüyordu; kullanıcı biriken akışları temizleyemiyordu.
Tekil çalıştırmalar için ise silme hiç yoktu.

**Ölçüm önce yapıldı.** `workflow_runs.version_id` **ON DELETE RESTRICT**
taşıyor ve `workflows` silinince hem `workflow_versions` hem `workflow_runs`
CASCADE'leniyor; sürüm kaskadı önce koşarsa silmenin FK ihlaliyle düşmesi
bekleniyordu. Geri alınan bir işlemle gerçek veritabanında denendi: **düşmüyor**,
çalışması olan bir akış sorunsuz siliniyor. Kısıt değiştirilmedi; davranış
`TestDelete_GecmisiBirlikteGider` ile kilitlendi.

**Karar.**

1. **Süren çalışması olan akış silinmiyor** (409). Silinseydi kayıt giderdi ama
   motorun goroutine'i yaşamaya devam ederdi: sonraki yazmalar sessizce 0 satır
   etkiler, container kaydı olmayan bir işi çalıştırmayı sürdürürdü. Zaten
   tanımlı ama hiç kullanılmayan `ErrRunning` sabiti bu iş için kullanıldı.

2. **Çalıştırma silme yalnızca bitmiş kayıtlarda.** "Çalışıyor mu" sorusunun tek
   doğru kaynağı veritabanı değil: iptal asenkron, `Cancel` hemen dönerken
   container silme ve durum yazımı arka planda sürüyor. Bu yüzden kapı
   `Manager`'da ve hem DB durumuna hem bellekteki canlı iş listesine bakıyor.

3. **Akış adımı olan çalıştırma silinmiyor** (409). `workflow_steps.run_id`
   ON DELETE SET NULL: kayıt gitse de adım "başarılı" görünmeye devam eder ama
   agent'ı, modeli, maliyeti ve token'ı boşalır — akış geçmişinde sessiz bir
   delik açılırdı. Kullanıcı akış çalışmasını siler, tek adımını değil.

4. **Sonuç onayda SAYIYLA yazılıyor.** Maliyet ve token doğrudan `runs`
   satırının sütunlarında; ayrı bir özet tablo yok, yani silme geçmiş raporları
   gerçekten ve geri alınamaz biçimde değiştiriyor. Onay şeridi bunu
   "$0,0041 ve 7,4 B token raporlardan düşecek" gibi somut yazıyor —
   "emin misiniz?" bunu söylemezdi. Tarayıcıda ölçüldü: silme sonrası rapor
   toplamı tam o kadar düştü.

**Kapsam dışı.** Toplu silme, arşivleme (soft delete) ve `workflow_runs` için
ayrı bir silme ucu eklenmedi.
