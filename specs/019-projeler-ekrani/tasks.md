# Görevler: Projeler ekranının yoğunlaştırılması

- **Spec no:** 019 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Uygulandı (2026-08-14)

Sıra riske göre: en tehlikeli şey bilgi kaybı — sekiz alan sütunlara dağılırken
birinin düşmesi sessizce olur. Bu yüzden önce alanlar sayılıyor, sonra düzen
değişiyor. `PAGE_SIZE` en sona bırakılıyor ki ölçüm tek değişkenle yapılsın.

---

## Envanter

- [x] T01 Bugünkü kartta görünen alanlar tek tek yazılır → sekiz alan
      (ad, depo yolu, branch, sunucu, git erişimi, akışlar, otuz günlük
      ölçüler, son çalışma) sonraki görevlerin kontrol listesi olur

## H1, H5 — Satır düzeni ve bilgi eksiksizliği

- [x] T10 `ProjectRow` yazılır: Çalıştırmalar ekranının tablo kalıbı
      (`Card padded={false}` + `overflow-x-auto` + `thead` şeridi) → liste
      tablo olarak çiziliyor
- [x] T11 Sekiz alanın **sekizi de** satırda → envanterle tek tek karşılaştırılır;
      eksik alan yok
- [x] T12 Otuz günlük ölçüler tek satır sessiz üstveri olarak görünür → üç ayrı
      kutu değil; sayılar okunabilir
- [x] T13 Kaydı olmayan dönem "0" değil, ne olduğunu söyleyen ifade → "Son 30
      günde çalıştırma yok" bugünkü gibi duruyor
- [x] T14 Uzun ad ve depo yolu kırpılıyor, tamamı erişilebilir → `title` ile
      görülebiliyor, satır yüksekliği kaymıyor
- [x] T15 Akış adları sütunu taşmıyor → ikiden fazlası sayıyla belirtiliyor,
      adlar tıklanabilir kalıyor

## H2 — Kurulumun doğruluğu satırda

- [x] T20 Branch, sunucu ve git erişimi satırda görünür
- [x] T21 Doğrulanmamış erişim, doğrulanmıştan ayırt edilir ve ayrım **yalnızca
      renkle değil** → rozetin metni de farkı söylüyor
- [x] T22 Git erişimi silinmiş proje "açık depo" gibi görünmüyor

## H4 — Eylemler

- [x] T30 Düzenle ve sil `RowAction` ile → hover'da ve **klavye odağında**
      beliriyor; dar ekranda hep görünür
- [x] T31 Silme düğmesinin görünürlüğü ters dönmüyor → silinebilir satırlarda
      görünüyor (bkz. `RowAction` belgesindeki hata)
- [x] T32 Silme onayı `ConfirmStrip` ile yerinde açılıyor ve ne kaybedileceğini
      söylüyor
- [x] T33 Akışlara satırdan geçilebiliyor
- [x] T34 Klavyeyle gezinme: sekme ile satır eylemlerine ulaşılıyor, odak
      halkası iki temada görünüyor

## H3 — Sayfalama

- [x] T40 Satır yüksekliği ve görünen satır sayısı `getBoundingClientRect` ile
      **ölçülür** → sayı kaydedilir
- [x] T41 `PAGE_SIZE` ölçüme göre konur → ekranda görünen satır sayısıyla uyumlu
- [x] T42 Görünen proje sayısı bugünkünün **en az iki katı** → ölçülerek
      doğrulanır; "daha çok görünüyor" denmez
- [x] T43 Sayfalama denetimine ulaşmak için gereken kaydırma ölçülür →
      bugünkünün belirgin altında; toplam sayı görünür kalıyor
- [x] T44 Sayfalama davranışı bugünküyle aynı → ileri, geri, sınırlar

## H6 — Görünüm anahtarı

- [x] T60 Araç çubuğuna görünüm anahtarı eklenir (arama ve süzgeçle aynı yerde)
      → iki görünüm arasında geçiş yapılıyor
- [x] T61 Kart görünümü geri getirilir → git'teki hâli korunarak, yeniden
      yazılmadan
- [x] T62 Tercih hatırlanır → sayfa yenilendiğinde seçili görünüm duruyor
- [x] T63 Tercih BAKAN KİŞİYE ait → tarayıcıda saklanıyor, sunucuda değil;
      aynı kurulumu kullanan başkasının ekranını değiştirmiyor
- [x] T64 Sayfa boyutu görünüme uyar → liste 12, kart 6; kart görünümünde
      sayfalama denetimi **görünür oluyor**
- [x] T65 Görünüm değişince sayfa başa döner → eski offset yeni boyutta
      anlamsız olabilir
- [x] T66 Etkin görünüm yalnızca renkle değil → zemin + ikon; `aria-pressed`
      ekran okuyucuya söylüyor
- [x] T67 İki görünüm de iki temada görülür

## Durumlar ve kapanış

- [x] T50 Boş liste, eşleşmeyen arama ve yükleme hatası ayrı ayrı görülür
- [x] T51 Genişlik → geniş (1440) ve dar (1024) masaüstünde **sayfa gövdesi
      yatay kaymıyor**; yalnızca tablo kabı kendi içinde kayıyor. Telefon
      genişliği ölçülmüyor (kullanıcı kararı, 2026-08-14) ama responsive
      davranış korunuyor
- [x] T54 Düzenleme satırın yerinde açılıyor ve kaydediyor → ad değiştirildi,
      kaydedildi, satır güncellendi, form kapandı; geri alındı
- [x] T55 Boş sonuç metni HANGİ ölçütün sonuç vermediğini söylüyor → arama,
      süzgeç ve ikisi birlikte için ayrı cümleler; her biri çıkış yolunu
      gösteriyor
- [x] T52 İki tema **ayrı ayrı**; satır hover ve odak durumları hesaplanmış
      renkle doğrulanır
- [x] T53 `npx tsc --noEmit` ve `npx eslint .` temiz; `npm run test` yeşil

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.

**1. `IconTile` küçültüldü — ekrana sığan satır sayısını belirleyen tek şey oydu.**
Plan satır yüksekliğini konuşmuyordu. Ölçüldü: `md` boyutundaki karo (36px)
satırı 53 pikselde tutuyor ve ekrana 11 satır sığıyordu (1,8 kat — ölçütün
altında). Bileşenin kendi belgesi `sm` boyutunu zaten "liste satırı içi" diye
tanımlıyor; doğru boyuta geçince satır 48 piksele, sığan satır 13'e çıktı
(2,2 kat). Primitife dokunulmadı, var olan seçenek kullanıldı.

**2. `table-fixed` eklendi.**
Otomatik tablo düzeninde hücre içeriğe göre genişliyor ve `truncate` hiç
devreye girmiyordu: uzun bir depo yolu komşu sütunun üstüne taşıyordu.

## Uygulama sırasında bulunan üç hata

**1. JSX içinde çıplak `/* */` yorumu EKRANDA GÖRÜNDÜ.**
Tablonun üstüne yazdığım açıklama metin düğümüne dönüşüp sayfada bir paragraf
olarak çıktı. `{/* */}` ile düzeltildi. Ekran görüntüsü almasaydım fark
edilmezdi — `tsc` ve `eslint` ikisi de temizdi.

**2. Sütun dengesi ilk denemede yanlıştı.**
Erişim sütununu genişletince birincil bilgi (proje adı) daraldı ve adlar
"agentTestProject ..." diye kırpıldı. Birincil sütun en geniş olmalı;
genişlikler yeniden dağıtıldı.

**4. Kart görünümü İSTEK ÜZERİNE geri geldi.**
İlk uygulamada kart düzeni tamamen listeye çevrilmişti. Kullanıcı kartın da
kendi işi olduğunu belirtti: liste karşılaştırma için, kart az sayıda projeyi
rahat okumak için. İkisi arasında geçiş yapan bir anahtar eklendi (spec 019
H6). Kart bileşeni yeniden yazılmadı — git'teki hâli olduğu gibi geri alındı,
yani ondaki ölçülmüş kararlar (eşit yüksekliğe çekilmeme, akışların sayıyla
değil adla gösterilmesi) korundu.

Sayfa boyutu artık görünüme bağlı: liste 12, kart 6. Tek bir boyut ikisine
birden uymuyordu — 12 kart görünümünde iki ekran boyu kaydırma, 6 ise liste
görünümünde ekranın yarısını boş bırakırdı.

**3. "Sayfa yatay kayıyor" tespiti YANLIŞTI — ölçüm hatasıydı.**
İlk ölçümde `documentElement.scrollWidth > clientWidth` görüp sayfanın yatay
kaydığını yazmıştım. Doğru ölçüt bu değil: `documentElement.scrollWidth`, bir
üst kap tarafından KIRPILAN tablo kutusunu da sayıyor. Doğru ölçüt
`document.body.scrollWidth` ve o **kaymıyor**.

Dar masaüstünde (1024) gerçek durum şu: tablo kabı kendi içinde kayıyor
(`overflow-x: auto`, 731px görünür / 960px içerik), sayfa gövdesi kaymıyor —
yani istenen davranışın ta kendisi. `min-w-0` eklemeleri zinciri doğru
kurmuştu; sorun benim okuduğum sayıdaydı.

**5. Boş sonuç metni yanlış yeri gösteriyordu.**
"Bu süzgece uyan proje yok" her durumda yazılıyordu; kullanıcı bir kelime
ARADIYSA bu yanlıştı — süzgeç "Tümü"de dururken sorun aramadaydı. Artık üç
ayrı cümle var (yalnızca arama / yalnızca süzgeç / ikisi birlikte) ve her biri
nasıl geri dönüleceğini söylüyor.

## Ölçümler

| Ne | Önce | Sonra |
| --- | --- | --- |
| Satır/kart yüksekliği | **227 px** | **48 px** |
| Ekrana sığan proje | **6** | **13** → **2,2 kat** |
| 7 projede kaydırma | 1,35 kat | **yok** |
| `PAGE_SIZE` | 24 (ekrana sığanın 4 katı) | **12** (13 sığıyor) |
| Bilgi alanı sayısı | 14 | **14 — hiçbiri kaybolmadı** |
| Yatay taşma 1440px | yok | **yok** |
| Sayfa gövdesi yatay kayması 1024px | — | **yok** — yalnızca tablo kabı kayıyor |
| Eylemler klavye odağında | — | beliriyor, odak halkası 2px |
| Silme düğmesi | — | pasif değil; görünürlük tersine dönmüyor |
| Telefonda eylemler | — | hep görünür (hover yok) |
| Statik | — | `tsc` ve `eslint` temiz, 66 test yeşil |
| Boş sonuç metni | "Bu süzgece uyan proje yok" (her durumda) | ölçüte göre üç ayrı cümle + çıkış yolu |
| Görünüm anahtarı | — | liste ⇄ kart; tercih `localStorage`'da kalıcı |
| Sayfa boyutu | 24 (tek) | **liste 12 · kart 6** — ikisi de ölçümle |
| Kart görünümünde sayfalama | görünmüyordu | **görünüyor** (8 projede 2 sayfa) |
