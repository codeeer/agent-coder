# Arayüz yeniden tasarımı

Bir ekranın görsel tasarımı elden geçirilirken uyulacak kurallar. Tek bir
bileşene dokunmak değil, **bir ekranı yeniden tasarlamak** söz konusuysa bu
belge geçerlidir.

Kaynak: `AGENTS.md` → [Visual Design / UI Redesign](../../AGENTS.md#visual-design--ui-redesign).
Orası ne zaman devreye girdiğini, burası nasıl yapılacağını anlatır.

---

## Değişmeyecek olan

**İş mantığı, API sözleşmesi, veri modeli ve mevcut işlevsellik korunur.**
Ekran bugün ne yapıyorsa yarın da onu yapar; değişen, yaptığı şeyi nasıl
gösterdiğidir.

Buna karşılık **görsel yapı korunmak zorunda değildir.** Layout, bileşen
hiyerarşisi, bilgi mimarisi, boşluk, tipografi, renk, kart yapısı, tablo,
süzgeç ve durum göstergeleri tamamen yeniden kurulabilir. "Mevcut tasarım
çalışıyor" bir gerekçe değildir — görev zaten onu değiştirmektir.

Yeni bir uç, yeni bir alan veya yeni bir bağımlılık **gerekmez**. Ekranda
göstermek istediğiniz bir veri yoksa, onu uydurmak yerine göstermeyin.

---

## Redesign derinliği

**Mevcut düzeni küçük CSS değişiklikleriyle korumak varsayılan yaklaşım
değildir.** "Mevcut bileşenleri kullan" kuralı, "mevcut layout'u koru"
demek değildir.

Bilgi mimarisi zayıfsa yapılacak iş şudur:

- bölümleri yeniden grupla, içerik sırasını değiştir
- araç çubuğu oluştur (arama, süzgeç, görünüm anahtarı tek yerde)
- ikincil bilgileri üstveri seviyesine indir
- ilişkili bilgileri aynı görsel grubun altında topla
- gereksiz alanları kaldır
- var olan ama görünmeyen bilgiyi görünür kıl
- tablo / liste / detay ilişkisini yeniden kur

Redesign'ın amacı mevcut ekranı süslemek değil, **aynı işlevi daha iyi bir
kullanıcı deneyimiyle yeniden sunmaktır.**

---

## Görsel referans

Kullanıcı bir ekran görüntüsü, mockup veya görsel referans verip bunu
tasarım hedefi olarak belirttiğinde, o görsel **birincil referanstır.**

**İlham kaynağı değildir.** "Şuna benzer bir şey" değil, "şu" demektir.

Önce mevcut ekranla referans arasındaki farklar çıkarılır — göz kararıyla
değil, madde madde:

- düzen (layout) ve boşluk
- tipografi ve bilgi hiyerarşisi
- bileşen hiyerarşisi
- renkler, yüzeyler, kenarlıklar
- denetimler (düğme, süzgeç, arama, sayfalama)
- bilgi yoğunluğu

Sonra referansın **görsel dili ve tasarım yaklaşımı** uygulanır; mevcut
işlevsellik ve gerçek veri korunarak.

Birebir kopyalamak gerekmez. Ama sonuç referanstan **belirgin biçimde daha
basit, daha eski veya daha düşük kaliteli olmamalıdır** — "yaklaştım" demek
için ölçüt budur.

**Referansta olup üründe karşılığı olmayan şey uydurulmaz:** veri,
istatistik, kullanıcı, kota, bildirim, eylem. Referansta olması gerekçe
değildir (bkz. [Uydurulmayacaklar](#uydurulmayacaklar)). O kutunun yerine
üründe gerçekten var olan bir bilgi konur ya da kutu hiç konmaz.

---

## Sıra

Kod yazmadan önce:

1. **Ekranın amacı ne?** Kullanıcı buraya neden geliyor?
2. **Ana görevi ne?** Buradan çıkarken ne yapmış olmak istiyor?
3. **Hangi bilgi birincil, hangisi ikincil, hangisi üstveri?**
4. **Hangi bilgiler birbiriyle ilişkili?** (Yan yana durmaları gerekenler.)
5. **Mevcut hiyerarşinin sorunu ne?** Somut yaz: "altı satır aynı görsel
   ağırlıkta", "asıl içerik bir düğmenin arkasında" gibi.
6. **Yeni düzen önerisi** ve kullanılacak bileşenler.

Sonra kısa bir plan, sonra uygulama, sonra tarayıcıda doğrulama.

**Analiz atlanırsa ortaya çıkan şey yeniden tasarım değil, süslemedir.**

---

## Kalite hedefi

Ekran bir CRUD/admin paneli gibi değil, **modern bir AI mühendislik
ürününün üretim arayüzü** gibi görünmeli.

Kalite ölçüsü olarak Linear, Vercel, Raycast, GitHub, Stripe ve Datadog'un
ortak prensiplerine bakılır — **birebir kopyalanmaz.** Alınan şey görüntü
değil, karar biçimi: yoğunluk, hiyerarşi, sessiz görsellik.

### Tercih edilenler

- güçlü ama **küçük** tipografik kontrastlar
- kompakt araç çubukları
- iyi gruplanmış üstveri
- bağlama duyarlı eylemler (satırın/seçimin kendi eylemleri)
- ince hover ve focus geri bildirimi
- net seçili / etkin durumlar
- tutarlı 4/8px boşluk ritmi
- kontrollü kenarlık ve yüzey katmanları
- kısa ve anlamlı durum göstergeleri
- kademeli açığa çıkarma (progressive disclosure)
- gerektiğinde zaman çizelgesi / etkinlik akışı / çalıştırma izi
- gerektiğinde bölünmüş görünüm veya detay paneli
- gerektiğinde sekme ve segment düğmeleri

### Kaçınılacaklar

- Başlık + dört KPI kartı + kocaman grafik + genel tablo şablonu
- Az bilgiyi büyük kartlara yayan "generic SaaS dashboard" düzeni
- **Her şeyi kart yapmak**; her bölümü başlık + açıklama + kart olarak
  tekrarlamak
- Aşırı büyük başlıklar, aşırı yuvarlak köşeler
- **Her düğmeyi dolu yapmak**, her metriği renklendirmek
- Gereksiz gradient, glow, büyük gölge, dekoratif grafik
- Yalnızca boşluk doldurmak için eklenen görsel öğe
- Her şeyi kenarlık, gölge ve renkle ayırmak
- Emoji, karışık ikon kümesi, tutarsız ikon boyutu

---

## İlkeler

### Hiyerarşi

Kullanıcı ilk bakışta şu sırayla görebilmeli: sayfanın ne olduğu → en önemli
bilgi → ana eylem → durum → detay. Her şey aynı punto ve kalınlıktaysa
hiyerarşi yok demektir.

### Eylem hiyerarşisi

Renk ve dolgu **bilinçli** kullanılır; ikisi birlikte bir sıra kurar.

Kural: **bir ekranda genellikle tek bir birincil eylem öne çıkar** ve
yalnızca o doludur. İkincil eylemler kenarlıklı, ghost veya düz bağlantı
olarak durur.

Gerçekten gerekmedikçe aynı görsel ağırlıkta **birden fazla dolu düğme
kullanılmaz.** Aksan her satırda tekrar ederse vurgu olmaktan çıkar: altı
satırlık bir listede altı dolu düğme, sayfanın asıl eylemini
görünmez yapar.

Bir eylemin düğme olduğu **kenarlığından** anlaşılır. "Bu çizgi olmasaydı
kullanıcı orada tıklanabilir bir şey olduğunu anlar mıydı?" — cevap hayırsa
kenarlık zorunludur, sessizleştirmek için kaldırılamaz.

### Bilgi yoğunluğu

Geliştirici araçlarında yoğunluk yüksektir. Ekranı boşlukla doldurmayın —
ama okunabilirliği de bozmayın. Bir liste ekranı, aynı anda karşılaştırma
yapılabilecek kadar satır göstermeli.

### Katmanlı bilgi

Her şeyi tek düz listede göstermeyin. Gerektiğinde bölüm, pano, sekme,
rozet, zaman çizelgesi, açılır detay ve gruplama kullanın.

### Sessiz görsellik

İnce kenarlıklar, sakin yüzeyler ve boşluk. İki şeyi ayırmak için önce
boşluğu, sonra kenarlığı deneyin.

### Anlamlı renk

Renk süs değil, anlam taşır: başarı → yeşil, uyarı → amber, hata → kırmızı,
etkin → aksan, nötr → sönük. Sıradan içerik sürekli renkli olmaz.

Renk **tek kanal değildir**: her renkli göstergenin yanında metni de durur.
Renk körlüğünde ve siyah-beyaz çıktıda okunabilen tek kanal etikettir.

### Tipografi

Sayfa başlığı → açıklama → bölüm başlığı → birincil bilgi → ikincil bilgi →
üstveri. Altı kademe, altı ayrı görünüm.

---

## Bileşenler

**Projenin kendi katmanı kullanılır:** `frontend/src/components/ui/primitives.tsx`.
`Card`, `Panel`, `Toolbar`, `SearchField`, `Segmented`, `IconTile`,
`StatCard`, `Badge`, `List` … Burada olmayan bir kalıp gerekiyorsa **önce
buraya** eklenir, ekranın içine değil — aksi halde aynı kalıbın ikinci ve
üçüncü kopyası çıkar ve er geç ayrışırlar.

> Bu proje **shadcn/ui kullanmıyor.** `primitives.tsx` onun yerini tutuyor.
> Yeni bir bileşen kitaplığı eklemeyin; gereksiz bağımlılık, ürünün en
> kolay eskiyen parçasıdır.

Görsel kalite için mevcut bileşen hiyerarşisini değiştirmek serbesttir —
körü körüne korumayın.

### İkonlar

Tek kaynak **lucide-react**, tek kapı `components/ui/icons.tsx`. Boyut 16px,
kalınlık 2, renk `currentColor`. Ayrıntı: `AGENTS.md` → İkon sistemi.

---

## Açık ve koyu tema

**Açık tema, koyu temanın ters çevrilmiş kopyası değildir.** Her ikisi de
ayrı ayrı değerlendirilir: düğme kontrastı, metin kontrastı, kenarlıklar,
kartlar, sönük metin, rozetler, girdiler, seçili ve hover durumları.

Aynı renk çifti iki zeminde aynı işi yapmaz; koyu zeminde "daha parlak =
daha önemli", açık zeminde "daha koyu = daha önemli"dir. Değerleri ters
çevirmek vurguyu ters çevirir.

Koyu tema güzel görünüyor diye açık temayı ihmal etmeyin — ve tersi.

Burası temanın **nasıl değerlendirileceğini** anlatır. Temanın **mekanizması**
— seçimin nerede saklandığı, üç durum, ilk boyamada sıçramanın önlenmesi ve
Tailwind'in `dark:` varyantının işletim sistemini takip etmesi tuzağı —
[spec 006](../../specs/006-tema-secimi/spec.md) dosyasındadır. İkisi ayrı
şeydir: biri tasarım kuralı, diğeri ürün kararı.

---

## Uydurulmayacaklar

Bu ürünün en sert kuralı: **ölçülmeyen hiçbir şey ekranda gösterilmez.**

- Sistemin bilmediği bir rakam (kazanılan süre, birleştirilen PR) kutu
  olmaz.
- Kaydı olmayan bir dönem "0" değil "—" gösterir. Sıfır, hiç
  çalıştırılmamış bir şeyi başarısız gibi gösterir.
- Sınırlı bir pencereye bakan liste "hiç yok" demez, "yakın geçmişte yok"
  der.
- Sahte kullanıcı, sahte kota, sahte bildirim eklenmez. Referans görselde
  olması gerekçe değildir; bu üründe karşılığı yoksa yeri de yoktur.

---

## Doğrulama

**İlk çalışan tasarım final değildir.**

Uygulama bitince Chrome DevTools MCP ile gerçek tarayıcıda açılır, ekran
görüntüsü alınır ve şunlar tek tek kontrol edilir:

- hizalama, boşluk, tipografi, taşma (`overflow`)
- görsel hiyerarşi, düğme yerleşimi, durum göstergeleri
- responsive davranış — en az geniş masaüstü, dar masaüstü ve telefon
- açık tema ve koyu tema **ayrı ayrı**
- hover, focus, yükleniyor, boş ve hata durumları

### Tarayıcı yalnızca ekran görüntüsü için değildir

DevTools'un işi resim çekmek değil, **iddiaları sınamaktır.** Gerektiğinde:

- DOM yapısını incele — beklediğin ağaç gerçekten o mu
- hesaplanmış stilleri kontrol et
- eleman boyutlarını ölç
- taşma (`overflow`) sorunlarını tespit et
- konsol hatalarına bak
- farklı viewport genişliklerini dene
- hover ve focus durumlarını tetikle
- iki temanın davranışını ayrı ayrı doğrula

**Tarayıcı doğrulaması kod incelemesinin yerine geçmez; ikisi birlikte
kullanılır.** Ekran görüntüsü bir şeyin yanlış olduğunu gösterir, nedenini
söylemez; kod nedenini söyler ama ekranda ne çıktığını söylemez.

### Renk ve kontrast iddiaları ölçülür

Göze güvenilmez. İki tema aynı anda görülemiyor ve 4,1 ile 4,6 arası bakışla
ayırt edilmiyor; "iyi görünüyor" bir ölçüm değildir.

Bir renk, kontrast veya boyut iddiasında bulunulacaksa **gerçek değer**
okunur:

- tarayıcıda hesaplanmış CSS (`getComputedStyle`) — jetonun ekrana ne
  olarak çıktığını yalnızca bu söyler
- gerçek DOM ölçüsü (`getBoundingClientRect`, `scrollHeight`) — taşma ve
  kırpılma iddiaları için
- ekran görüntüsünden piksel okumak — iki temanın gerçekten farklı
  boyandığını doğrulamanın en ucuz yolu
- gerekiyorsa hesaplanmış kontrast oranı (metin 4,5:1 · iri metin ve
  denetim sınırı 3:1)

Ölçüm yapılmadan "kontrast yeterli" veya "iki tema da doğru" denmez.

Sonuç referanstan veya hedeften belirgin biçimde uzaksa **tekrar düzenlenir.**

Son olarak `npx tsc --noEmit` ve `npx eslint .` temiz olmalı.
