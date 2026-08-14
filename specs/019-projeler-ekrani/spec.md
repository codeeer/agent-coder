# Spec: Projeler ekranının yoğunlaştırılması

- **Spec no:** 019
- **Tarih:** 2026-08-14
- **Durum:** Taslak

---

## Problem

**Proje sayısı arttıkça ekran kaydırmaya dönüşüyor ve sayfalama devreye hiç
girmiyor.**

Ölçüldü (geniş masaüstü, 1440×900):

| Ne | Değer |
| --- | --- |
| Bir proje kartının yüksekliği | **227 piksel** |
| Görünür içerik alanı | 665 piksel |
| Aynı anda görünen proje | **~6** |
| Sayfa boyutu | **24** |
| 7 projede kaydırma | zaten 1,35 kat |

Sayfa boyutu, ekrana sığanın **dört katı**. Yani bir sayfa dolduğunda kullanıcı
dört ekran boyu kaydırıp en sonunda sayfalama denetimine varıyor. Kaydırma,
sayfalamanın yerine geçmiş durumda; sayfalama denetimi ise 24 projeden azına
sahip kurulumlarda hiç görünmüyor.

**Asıl sebep sayfa boyutu değil, kartın kendisi.** Bir proje kartı beş yatay
bant taşıyor: kimlik, bağlantı rozetleri, akış adları, otuz günlük ölçüler ve
son çalışma satırı. Her bant kendi dolgusuyla. Sonuç, az bilgiyi çok yere yayan
bir kart ve karşılaştırma yapılamayan bir liste: "hangi projelerim var" sorusu
tek bakışta cevaplanamıyor.

Kartın en büyük dikey payı otuz günlük ölçülere (çalıştırma sayısı, başarı
yüzdesi, maliyet) ayrılmış. Bu bilgi **değerli ve kalacak** — sorun bilginin
kendisi değil, üç ayrı ölçü kutusu olarak kapladığı yer.

## Amaç

Projelerin aynı anda karşılaştırılabileceği kadarının tek ekranda görünmesi ve
sayfalamanın gerçekten sayfalama olarak çalışması.

## Kapsam dışı

- **İş mantığı, veri modeli ve API.** Yeni uç, yeni alan, yeni bağımlılık yok.
  Ekran bugün ne yapıyorsa yarın da onu yapar.
- **Proje ekleme ve düzenleme akışı.** Form ve doğrulama kuralları değişmez.
- **Silme onayının davranışı.** Yerinde onay ve sonucun sayıyla yazılması
  ölçülmüş kararlardır.
- **Diğer liste ekranları** (Akışlar, Agent'lar, Çalıştırmalar). Yalnızca
  Projeler ele alınır; ortaya çıkan kalıp başkalarına uygulanacaksa ayrı iş.
- **Görünüm tercihinin sunucuda saklanması.** Tercih bakan kişiye aittir,
  kuruluma değil — tema seçimindeki kararın aynısı.
- **Raporlar ekranı.** Ayrıntılı analiz orada; oraya dokunulmuyor.
- **Bilgi kaldırmak.** Bugün gösterilen hiçbir alan listeden çıkarılmaz.
- **Arama ve süzgeçler.** Bugün var ve çalışıyor; yerleri değişebilir ama
  davranışları değişmez.

---

## Kullanıcı Hikâyeleri

### H1 — Projeleri tek bakışta görmek

**Yönetici** olarak, tanımlı projelerimi **kaydırmadan** görmek istiyorum,
çünkü bugün yedi projede bile kaydırmam gerekiyor.

Kabul kriterleri:

- [ ] Aynı ekranda görünen proje sayısı bugünkünün **en az iki katı**
- [ ] Sayı ölçülerek doğrulanır; "daha çok görünüyor" denmez
- [ ] Bir projenin adı ve hangi depoya baktığı tek bakışta okunur
- [ ] Uzun proje adları ve depo yolları taşmaz, kırpılır ve tamamı erişilebilir
      kalır
- [ ] Dar masaüstü ve telefon genişliğinde yatay taşma yok

### H2 — Kurulumun doğru olduğunu görmek

**Yönetici** olarak, bir projenin **çalıştırılmaya hazır olup olmadığını**
listede görmek istiyorum, çünkü buraya en sık bunu kontrol etmeye geliyorum.

Kabul kriterleri:

- [ ] Hangi branch'ten klonlanacağı listede görünür
- [ ] Hangi kimlikle klonlanacağı (ya da kimliksiz olduğu) listede görünür
- [ ] Kimliği doğrulanmamış bir erişim, doğrulanmış olandan **ayırt edilir**
- [ ] Bu ayrım yalnızca renkle yapılmaz; yanında metni de bulunur

### H3 — Sayfalamanın gerçekten çalışması

**Yönetici** olarak, sayfa değiştirmek istediğimde denetimin **görünür**
olmasını istiyorum, çünkü bugün ona ulaşmak için dört ekran boyu kaydırmam
gerekiyor.

Kabul kriterleri:

- [ ] Sayfa boyutu, ekranda görünen satır sayısıyla uyumlu
- [ ] Bir sayfa dolduğunda sayfalama denetimine ulaşmak için gereken kaydırma
      bugünkünün belirgin biçimde altında
- [ ] Toplam proje sayısı görünür kalır
- [ ] Sayfalama davranışı (ileri, geri, sınırlar) bugünküyle aynı

### H4 — Bir projeyle iş yapmak

**Yönetici** olarak, listeden çıkmadan bir projeyi **düzenleyip
silebilmek** istiyorum, çünkü bugün yapabildiğim şey bu.

Kabul kriterleri:

- [ ] Düzenleme ve silme listeden erişilebilir
- [ ] Bir eylemin tıklanabilir olduğu, üzerine gelmeden de anlaşılır
- [ ] Silme onayı bugünkü gibi **yerinde** açılır ve ne kaybedileceğini söyler
- [ ] Klavyeyle gezinen kullanıcı eylemlere ulaşabilir
- [ ] Bu depoyu kullanan akışlara listeden geçilebilir

### H5 — Bilgi kaybetmeden yoğunlaşmak

**Yönetici** olarak, ekran yoğunlaşırken **hiçbir bilgiyi kaybetmek
istemiyorum**, çünkü bugün görebildiğim her şeyin bir karşılığı var.

Yoğunluk, bilgi silerek değil **düzen değiştirerek** kazanılır: bugün beş ayrı
yatay bant ve üç ölçü kutusu olarak duran içerik, satır düzeninde daha az yer
kaplar.

Kabul kriterleri:

- [ ] Bugün kartta görünen her bilgi listede de görünür: ad, depo, branch,
      sunucu, git erişimi, akışlar, otuz günlük ölçüler, son çalışma
- [ ] Otuz günlük ölçüler okunabilir kalır ama **üç ayrı kutu** olarak yer
      kaplamaz
- [ ] Kaydı olmayan dönem "0" değil, ne olduğunu söyleyen bir ifadeyle gösterilir
- [ ] Hiçbir yeni sayı, oran veya istatistik eklenmez
- [ ] Hiçbir mevcut bilgi kaldırılmaz

### H6 — Görünümü seçebilmek

**Yönetici** olarak, listeyle kart görünümü arasında **geçiş yapabilmek**
istiyorum, çünkü ikisinin farklı işleri var: liste karşılaştırma için, kart az
sayıda projeyi rahat okumak için.

Kabul kriterleri:

- [ ] Ekranda iki görünüm arasında geçiş yapan bir denetim var ve arama/süzgeç
      ile aynı yerde durur
- [ ] Seçim **hatırlanır**; her ziyarette yeniden seçmek gerekmez
- [ ] Seçim bakan kişiye aittir; aynı kurulumu kullanan başkasının ekranını
      değiştirmez
- [ ] İki görünümde de **aynı bilgiler** görünür; biri diğerinden eksik değildir
- [ ] Sayfa boyutu seçilen görünüme uyar — kart görünümünde de sayfalama
      denetimine ulaşmak dört ekran boyu kaydırma gerektirmez
- [ ] Görünüm değiştirmek arama, süzgeç ve toplam sayıyı bozmaz
- [ ] Etkin görünüm yalnızca renkle değil, konum veya şekille de bellidir
- [ ] İki görünüm de iki temada ayrı ayrı doğrulanır

---

## Davranış Kuralları

- **Mevcut işlevsellik korunur.** Bugün yapılabilen her şey aynı sonucu verir.
- **Ölçülmeyen gösterilmez.** Ekrana yeni bir sayaç, oran veya tahmin
  eklenmez; bugün gösterilenlerin kaynağı değişmez.
- **Renk tek kanal değildir.** Durum bildiren her göstergenin yanında metni de
  bulunur.
- **İki tema ayrı ayrı değerlendirilir.**
- **Bir ekranda tek birincil eylem öne çıkar.** Satır başına dolu düğme
  yığmak, sayfanın asıl eylemini görünmez yapar.
- **İddialar ölçülür.** "Daha çok proje görünüyor", "hiza bozulmadı",
  "kontrast yeterli" — hepsi tarayıcıda gerçek değer okunarak doğrulanır.

## Hata Durumları

| Durum | Beklenen davranış |
| --- | --- |
| Hiç proje yok | Ne yapılacağını söyleyen bir durum gösterilir; boş bir tablo değil |
| Arama hiçbir şeyle eşleşmiyor | "Eşleşme yok" denir; arama ve süzgeçler erişilebilir kalır |
| Projeler yüklenemedi | Ne olduğu yazılır; ekran boş bırakılmaz |
| Bir projenin git erişimi silinmiş | Satır bunu belli eder; sessizce "açık depo" gibi görünmez |
| Silme başarısız | Hata satırın yanında görünür, kayıt listede kalır |
| Çok uzun proje adı veya depo yolu | Kırpılır; tamamı üzerine gelince veya başka bir yolla erişilebilir |

---

## Belirsizlikler

- [x] Otuz günlük ölçüler listeden kaldırılsın mı? → **Cevap:** Hayır,
      **hiçbir bilgi kaldırılmıyor**. Ölçüler değerli; sorun kapladıkları yer.
      Üç ayrı ölçü kutusu yerine sessiz bir üstveri satırına inerler. Yoğunluk
      bilgi silerek değil düzen değiştirerek kazanılır.
- [x] Proje bulmak zorlaşır mı? → **Cevap:** Hayır. Ekranda zaten proje adı,
      depo ve branch üzerinden çalışan bir arama var; "hangi projem vardı"
      sorusunun cevabı kaydırmaya bağlı değil. Sayfalama da ekrana uygun hale
      getirilerek gerçekten sayfalama olarak çalışacak.

## Bağımlılıklar

Yok.
