# Spec: Çalışma dizini yerleşimi

- **Spec no:** 025
- **Tarih:** 2026-08-17
- **Durum:** Taslak

---

## Problem

Klonlanan proje bugün **her zaman** çalışma kökünün kendisine açılıyor:
`/work`. Bu tek seçenek ve değiştirilemiyor.

Kurumsal ekiplerin elinde, projenin `<kök>/<repo-adı>` altında durduğunu
varsayan mevcut runbook'lar, Jenkins işleri ve yardımcı script'ler var. Bu
varsayım yaygın çünkü çoğu CI aracı ve geliştiricinin kendi makinesi böyle
çalışıyor. Agent'ın içinde ise proje doğrudan kökte duruyor; o script'ler
olduğu gibi çalışmıyor ve her biri elle uyarlanmak zorunda kalıyor.

Uyarlama maliyeti bir kereye mahsus da değil: dışarıdaki runbook güncellendiğinde
agent tarafındaki kopyası geride kalıyor. Ürün, dışarıda zaten çalışan bir
otomasyonu içeri alamıyorsa, agent'a devredilebilecek iş kümesi daralıyor.

Bu seçenek olmazsa ürün, mevcut otomasyonunu olduğu gibi taşımak isteyen
ekipler için uyarlama gerektiren bir araç olarak kalır.

## Amaç

Yönetici, ayarlardan projenin container içinde nereye açılacağını
seçebilecek: doğrudan çalışma kökü ya da kökün altında repo adını taşıyan bir
klasör. **Varsayılan davranış değişmeyecek**; ayara dokunmayan her kurulum
bugünkü gibi çalışmaya devam edecek.

## Kapsam dışı

- **Birden fazla repo'nun aynı çalıştırmada klonlanması.** Bu spec tek
  repo'nun nereye açılacağını belirler; kaç repo klonlandığını değiştirmez.
- **Proje bazında yerleşim.** Ayar global'dir. Projeye göre farklılaştırma
  gerekirse ayrı bir iştir; şimdilik ihtiyaç ölçülmedi.
- **Mevcut çalıştırmaların taşınması.** Yerleşim her çalıştırmanın başında
  hesaplandığı ve çalışma ortamı geçici olduğu için taşınacak bir şey yok.
- **Script'lerin ve paket ayarlarının yeri.** Bunlar çalışma kökünün dışında
  duruyor ve bu değişiklikten etkilenmiyor.

---

## Kullanıcı Hikâyeleri

### H1 — Yönetici yerleşimi seçebilir

**Yönetici** olarak, projenin çalışma ortamında nereye açılacağını
**seçebilmek** istiyorum, çünkü elimdeki runbook'lar ve script'ler projenin
repo adını taşıyan bir klasörde olduğunu varsayıyor.

Kabul kriterleri:

- [ ] Ayarlar ekranında iki seçenekli bir tercih var: çalışma kökünün kendisi
      (varsayılan) veya kökün altında repo adını taşıyan klasör
- [ ] Seçim yapıldıktan sonra başlatılan çalıştırmada proje, seçilen yere
      klonlanır
- [ ] Script'lere verilen proje dizini bilgisi ile agent'a verilen talimat
      metnindeki yol **aynıdır**; ikisi hiçbir koşulda birbirinden ayrışmaz
- [ ] Proje dizinini kendi değerinden değil ortamdan okuyan script'ler her iki
      yerleşimde de değişiklik gerektirmeden çalışır

### H2 — Varsayılan davranış değişmez

**Mevcut kullanıcı** olarak, bu özellik eklendiğinde **hiçbir şeyin
değişmemesini** istiyorum, çünkü çalışan kurulumumu bozacak bir güncellemeyi
alamam.

Kabul kriterleri:

- [ ] Ayara hiç dokunulmamış bir kurulumda proje, bugünkü yere klonlanır
- [ ] Ayar okunamıyor, boş veya tanınmayan bir değerdeyse varsayılan yerleşim
      kullanılır; çalıştırma bu yüzden başarısız olmaz
- [ ] Mevcut script'ler ve talimat metni varsayılan yerleşimde birebir aynı
      kalır

### H3 — Repo adı güvenli türetilir

**Kurum** olarak, klasör adının depo adresinden türetilirken **çalışma
kökünün dışına çıkamamasını** istiyorum, çünkü depo adresini girebilen biri
aksi halde çalışma ortamında istediği yere yazabilir.

Kabul kriterleri:

- [ ] Ad, depo adresinin son parçasından türetilir; sondaki ayraç ve depo
      uzantısı atılır
- [ ] Yaygın adres biçimlerinin hepsinde doğru ad çıkar: şifreli ve şifresiz
      web adresleri, uzantılı ve uzantısız, sonu ayraçla bitenler ve kısa
      SSH biçimi
- [ ] Türetilen ad dizin ayracı veya üst dizine çıkma ifadesi **içeremez**
- [ ] Ad boş veya anlamsız çıkarsa varsayılan yerleşime düşülür; çalıştırma
      hata vermez

---

## Davranış Kuralları

- **Yerleşim çalıştırma başına bir kez hesaplanır** ve o çalıştırma boyunca
  tek bir değer olarak kullanılır. Aynı bilgiyi iki yerde ayrı ayrı üretmek,
  er geç ayrışan iki gerçek yaratır.
- **Belirsizlikte varsayılana düşülür.** Bu bir güvenlik kontrolü değil, bir
  kolaylık ayarıdır; okunamayan bir değer yüzünden çalıştırma düşmez.
- **Çalışma ortamının giriş betiği kendi değerini dayatmaz**, kendisine
  verilen yeri kullanır. Bu sözleşme korunur.

## Hata Durumları

| Durum | Beklenen davranış |
| ----- | ----------------- |
| Ayar tanımlı değil | Varsayılan yerleşim kullanılır |
| Ayar tanınmayan bir değerde | Varsayılan yerleşim kullanılır; çalıştırma sürer |
| Depo adresinden anlamlı bir ad çıkmıyor | Varsayılan yerleşim kullanılır |
| Türetilen ad dizin ayracı veya üst dizin ifadesi içeriyor | Ad kullanılmaz; varsayılan yerleşime düşülür |

---

## Belirsizlikler

- [ ] Yok. Davranış brief'te net tanımlı.

## Bağımlılıklar

- Yok.
