# Spec: Ayar denetimi ve ayar araması

- **Spec no:** 016
- **Tarih:** 2026-08-14
- **Durum:** Taslak

---

## Problem

**1. Açık/kapalı bir ayar, yazılarak değiştiriliyor.**

"Motor loglarını sakla" iki durumlu bir ayar. Ekranda içinde `true` yazan bir
metin kutusu olarak duruyor; kullanıcı değeri **yazmak** zorunda. Yanlış
yazınca ne olacağını ekran söylemiyor.

Kutu ayrıca çok geniş: sayı ayarları dar bir kutuya sığarken bu ayar satırın
büyük kısmını kaplıyor ve bölümdeki denetim sütununun hizasını kırıyor —
diğer sekiz ayarın denetimi aynı hizada, bu biri değil.

Sorun tek bir ayara özgü değil, **tipin çizilme biçimine** özgü: ekran ayarı
tipine göre çiziyor ve iki durumlu tip için bir karşılık tanımlanmamış, o da
metin kutusuna düşüyor. Eklenecek her iki durumlu ayar aynısını yaşar.

**2. Bir ayarın nerede olduğunu bilmiyorsan bulamıyorsun.**

On yedi ayar sekiz bölüme dağılmış. Hangi bölümde olduğunu hatırlamayan
kullanıcının tek yolu bölümleri tek tek açmak. Ekran ayrıca büyüyecek:
kurumsal ağ ayarları ve diğer paket yöneticileri iki bölüm daha ekleyecek.

## Amaç

Her ayarın tipine uygun bir denetimle değiştirilebilmesi ve herhangi bir
ayarın hangi bölümde olduğu bilinmeden bulunabilmesi.

## Kapsam dışı

- **Ekranın yeniden tasarımı.** Bu iş bir redesign değil. Bölüm yapısı,
  satır düzeni, tipografik kademeler, kart kullanımı ve eylem hiyerarşisi
  **olduğu gibi kalır** — bunlar daha önce ölçülerek karara bağlandı ve bu
  spec onlara dokunmaz.
- **Yıkıcı eylemlerin görünümü.** "Sil" düğmesinin durgunken sessiz,
  etkileşimde kırmızı olması ölçülmüş bir karardır; değişmez.
- **Ayarların kendisi.** Hiçbir ayar eklenmiyor, kaldırılmıyor; adı, tipi,
  varsayılanı veya kabul ettiği aralık değişmiyor.
- **Kaydetme davranışı.** Her değişiklik bugünkü gibi açıkça kaydedilir.
- **Kurumsal ağ alanları** (sertifika, vekil sunucu) — spec 017.
- **Diğer paket yöneticileri** — spec 018.

---

## Kullanıcı hikâyeleri

### H1 — İki durumlu ayarı tıklayarak değiştirmek

**Yönetici** olarak, açık/kapalı bir ayarı **tıklayarak** değiştirmek
istiyorum, çünkü bugün kutuya değeri yazmak zorundayım.

Kabul kriterleri:

- [ ] İki durumlu ayarlar açık/kapalı bir denetim olarak görünür, serbest
      metin kutusu olarak değil
- [ ] Denetimin açık ve kapalı hâli yalnızca renkle değil, konum veya
      şekille de ayrılır
- [ ] Denetimin yanında hangi durumda olduğu **yazıyla** da okunur
- [ ] Değiştirildiğinde değer anında uygulanmaz; diğer ayarlarda olduğu gibi
      açıkça kaydedilir ve kaydedilmemiş satır ayırt edilir
- [ ] Denetim, bölümdeki diğer ayarların denetimleriyle aynı dikey hizada
- [ ] Bu davranış ayarın **tipinden** gelir; belirli bir ayarın adına
      bağlanmaz — eklenecek yeni bir iki durumlu ayar aynı denetimi
      kendiliğinden alır
- [ ] Klavyeyle erişilebilir: sekme ile odaklanır, boşluk/enter ile değişir

### H2 — Ayarı adını bilmeden bulmak

**Yönetici** olarak, bir ayarı hangi bölümde olduğunu bilmeden **aramak**
istiyorum, çünkü bugün sekiz bölümü tek tek açmam gerekiyor.

Kabul kriterleri:

- [ ] Ekranda bir arama alanı var ve bütün bölümlerdeki ayarları süzer
- [ ] Arama, ayarın görünen adında ve açıklamasında eşleşme arar
- [ ] Eşleşen ayarlar, hangi bölüme ait olduklarıyla birlikte görünür
- [ ] Arama sırasında ayarlar bugünkü gibi değiştirilip kaydedilebilir
- [ ] Hiçbir eşleşme yoksa "eşleşme yok" denir; sıfır bir sayı olarak
      gösterilmez
- [ ] Arama temizlenince ekran önceki hâline döner
- [ ] Arama yalnızca ayarları kapsar; sağlayıcı, sunucu, betik ve kimlik
      kayıtları kapsam dışıdır ve bu, kullanıcıya belli olur

---

## Davranış kuralları

- **Mevcut işlevsellik korunur.** Bugün yapılabilen her şey aynı sonucu verir.
- **Doğrulama kuralları korunur.** Bir değer bugün hangi gerekçeyle
  reddediliyorsa yarın da aynı gerekçeyle reddedilir, hata aynı yerde görünür.
- **Renk tek kanal değildir.** Durum bildiren her göstergenin yanında metni de
  bulunur.
- **İki tema ayrı ayrı değerlendirilir.**
- **Ölçülmeyen hiçbir şey gösterilmez.** Arama sonucu sayısı dışında ekrana
  yeni bir sayaç veya istatistik eklenmez; o sayı da gerçekten sayılan
  eşleşmedir.

## Hata durumları

| Durum | Beklenen davranış |
| --- | --- |
| İki durumlu ayarın kaydı başarısız | Denetim, sunucudaki gerçek değere döner; hata satırın yanında görünür |
| Kaydetme sırasında sunucuya ulaşılamıyor | Değer alanda kalır, hata gösterilir, tekrar denenebilir |
| Arama hiçbir şeyle eşleşmiyor | "Eşleşme yok" denir; arama alanı ve bölüm listesi erişilebilir kalır |
| Ayarlar hiç yüklenemedi | Arama alanı işlevsiz bir süs olarak durmaz; ne olduğu yazılır |

---

## Belirsizlikler

- [x] İki durumlu ayar anında mı kaydedilsin? → **Cevap:** Hayır, açık
      "Kaydet". Diğer bütün alanlar açık kaydetme kullanıyor; tek bir
      denetimin farklı davranması kullanıcının modelini bozar.
- [x] Arama tüm ekranı mı kapsasın, yalnızca ayarları mı? → **Cevap:**
      Yalnızca ayarları. Sağlayıcı ve sunucu kayıtları farklı bir şey; hepsini
      tek aramaya koymak sonuçları karşılaştırılamaz hale getirirdi. Sınır
      kullanıcıya belli edilir.
- [x] Ekranın tamamı yeniden mi tasarlansın? → **Cevap:** Hayır. İlk taslak
      öyle yazılmıştı; kod okununca problem listesinin çoğunun yanlış olduğu
      görüldü (satırlar zaten hairline ayraçlı, tipografik kademeler zaten
      var, "Sil"in sessizliği ölçülmüş bir karar). Kapsam ölçülebilen iki
      maddeye indirildi.

## Bağımlılıklar

Yok.

Spec 017 (kurumsal ağ ayarları) bu ekrana yeni bir bölüm ekleyecek; H1'in
tipten gelen denetim kuralı ve H2'nin araması o bölümü kendiliğinden kapsar.
