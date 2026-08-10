# Spec: Jira ve Kod Deposu Düğümleri

- **Spec no:** 009
- **Tarih:** 2026-08-10
- **Durum:** Uygulandı (2026-08-10)
- **Faz:** 5 — [plans/01](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Akış **iki ucundan da kopuk**. Tuvale yalnızca agent adımı eklenebiliyor:

- **Girişte:** işin nereden geldiğini kullanıcı elle yazıyor. Jira'daki task'ın
  başlığını, açıklamasını, kabul kriterlerini kopyalayıp görev kutusuna
  yapıştırmak gerekiyor — Faz 3'te ortadan kaldırdığımız kopyala-yapıştır,
  akışın başında hâlâ duruyor.
- **Çıkışta:** agent kodu yazıyor, branch'e gönderiliyor, sonra iş yine elle
  devam ediyor: PR'ı insan açıyor, Jira'ya linki insan yazıyor.

Ürünün ilk gün anlatılan örneği tam olarak buydu:
**"Jira'dan task çek → analiz et → kod geliştir → code review → PR aç."**
Ortadaki üç adım çalışıyor; baştaki ve sondaki yok.

## Amaç

Tuvale **agent olmayan düğümler** eklenebilsin: Jira'dan task çeken bir başlangıç,
PR açan ve Jira'ya yorum yazan bitiş adımları. Akış uçtan uca insansız
tamamlanabilsin.

## Kullanıcı hikâyeleri

1. Kullanıcı olarak, tuvale **"Jira'dan task çek"** düğümü ekleyebilmeliyim ve
   task'ın alanlarını (`{{ trigger.summary }}`, `{{ trigger.description }}`)
   sonraki adımların talimatında kullanabilmeliyim.
2. Kullanıcı olarak, akışın **belirli bir Jira sorgusuna (JQL) bağlı** olmasını
   ve yeni task geldiğinde kendiliğinden çalışmasını isteyebilmeliyim.
3. Kullanıcı olarak, tuvale **"PR aç"** düğümü ekleyebilmeliyim; başlığı ve
   açıklaması önceki adımların çıktısından gelsin.
4. Kullanıcı olarak, tuvale **"Jira'ya yorum yaz"** düğümü ekleyebilmeliyim ki
   sonuç ve PR linki task'a düşsün.
5. Kullanıcı olarak, bir düğüm başarısız olduğunda (Jira erişilemedi, PR
   çakıştı) **neden** başarısız olduğunu görebilmeliyim.
6. Kullanıcı olarak, aynı task'ın **iki kez işlenmediğinden** emin olmalıyım.

## Kabul kriterleri

- [x] Tuvale Jira ve depo düğümleri eklenebiliyor; agent düğümleriyle aynı
      şekilde bağlanıyor ve doğrulanıyor.
- [x] Jira task'ının alanları sonraki adımların talimatına geçiyor.
- [x] JQL sorgusuna uyan yeni bir task akışı kendiliğinden başlatıyor.
- [x] Aynı task ikinci kez işlenmiyor.
- [x] PR gerçekten açılıyor; adresi çalışma kaydında görünüyor.
- [x] Jira'ya yorum gerçekten düşüyor.
- [x] Bu düğümler **model çağırmıyor**: maliyetleri sıfır, rapor bunları
      "çalıştırma" olarak saymıyor.
- [x] Jira ve depo erişimi olmayan bir kurulumda akış motoru bozulmuyor; bu
      düğümler açık bir hatayla başarısız oluyor.

## Kapsam dışı

- **Jira dışı iş takip sistemleri** (Linear, Azure DevOps…). Sağlayıcı kavramı
  genişletilebilir kurulacak ama bu fazda yalnızca Jira.
- **PR'a yorum yazma, PR birleştirme.** Açmak yeter; birleştirme insan kararı.
- **Jira alan yazma** (durum değiştirme, atama). Yalnızca yorum.
- **Bitbucket PR.** Depo sağlayıcı soyutlaması var ama bu fazda GitHub.

## Kararlar

**K1 — Jira tetikleme HEM tarama HEM webhook ile olacak.** JQL taraması Jira
tarafında ayar gerektirmiyor (yalnızca API token); webhook ise anında tetikliyor
ama Jira yöneticisinin tanımlaması gerekiyor. İkisi birden istendi.

*Riski ve önlemi:* iki tetikleme yolu, iki ayrı tekrar-işleme koruması ve iki
ayrı hata modeli anlamına gelebilirdi. Bunu engellemek için ikisi de **aynı
başlatma yolundan** ve **aynı "bu task işlendi" kaydından** geçecek. Yollar
yalnızca "task nereden duyuldu" noktasında ayrılıyor; ondan sonrası ortak.

**K2 — PR açma ve Jira yorumu tuvalde düğüm olarak.** Kullanıcı nereye
koyacağına kendisi karar verir, başlığını ve gövdesini önceki adımların
çıktısından şablonla kurar. Akış ayarı olsaydı dallanan bir akışta hangi dalın
sonucunun kullanılacağı belirsiz kalırdı.

**K4 — Akışın kendi yazdığı yorum tetikleyici sayılmaz.** Yorum, Jira'da
task'ın güncellenme zamanını değiştiriyor; tekrar-işleme koruması bu zamanı
anahtar aldığı için akış kendi izini yeni bir iş sanıp yeniden başlıyordu
(ölçüldü — [tasks.md](tasks.md) Ölçüm 4). Yorum adımı artık ürettiği
güncellemeyi kendi adına işaretliyor.

*Kabul edilen sınır:* aynı anda yapılan bir insan düzenlemesi aynı zaman
damgasına düşerse yutulur. Kendi değişikliğimize tepki verip sonsuz döngüye
girmekten iyidir.

**K3 — Faz ikiye bölünüyor.** Önce **çıkış** düğümleri (PR aç, Jira'ya yorum
yaz), sonra **giriş** (Jira tetikleyici). Gerekçe: çıkış düğümleri mevcut motorla
aynı şekilde çalışıyor ve hemen görülebilir; tetikleme ise yeni bir arka plan
işçisi ve tekrar-işleme koruması gerektiriyor.
