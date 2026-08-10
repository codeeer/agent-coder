# Spec: Arayüz Denetimi — Kullanılabilirlik ve Tema Eşliği

- **Spec no:** 010
- **Tarih:** 2026-08-10
- **Durum:** Uygulandı
- **İlgili plan:** [plans/01-mimari-ve-yol-haritasi-2026-08-09.md](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Arayüz beş faz boyunca özellik özellik büyüdü ve hiç **bütün olarak** kullanılmadı.
Her ekran kendi başına çalışıyordu; kimse baştan sona bir kullanıcı gibi gezmemişti.

İki ayrı boşluk vardı:

1. **Kullanım.** Açılış ekranı geliştirme yol haritasını gösteriyordu, telefonda
   içerik okunmuyordu, bir listede aranan kayıt bulunamıyordu.
2. **Tema.** Renkler iki kez elle düzeltildi ama hiç **ölçülmedi**. Göz 4,4:1 ile
   4,6:1 arasını ayırt edemez ve iki temayı aynı anda tutamaz; dolayısıyla "bir
   temada doğru, diğerinde bozuk" hatası gözle bulunamaz.

## Amaç

Uygulamanın iki temada da tutarlı, erişilebilir ve kendi kendini anlatan olması.
"Daha güzel görünmesi" hedef değil.

## Kullanıcı hikâyeleri

- **İlk kez açan biri olarak** ne yapmam gerektiğini ekranın kendisinden anlamak
  istiyorum; belge okumadan.
- **Telefonundan bakan biri olarak** aynı işleri yapabilmek istiyorum.
- **Açık tema kullanan biri olarak** koyu temadakiyle aynı netlikte görmek
  istiyorum — düğmelerin nerede bittiğini, hangi yazının ikincil olduğunu.
- **Az gören biri olarak** metni ve denetim sınırlarını seçebilmek istiyorum.

## Kabul kriterleri

- [x] Açılış ekranı kurulumu bitmemiş kullanıcıya sıradaki adımı, bitmiş
      kullanıcıya son durumu gösterir. Geliştirme yol haritası ekranda durmaz.
- [x] Telefon genişliğinde yatay taşma yoktur ve her ekranın birincil eylemi
      erişilebilir durumdadır.
- [x] Aynı kavram her yerde aynı adla anılır.
- [x] Metin kontrastı iki temada da AA eşiğini geçer (normal 4,5:1, iri 3:1).
- [x] Denetim sınırları (düğme, girdi, açılır liste) iki temada da 3:1 geçer.
- [x] Hiçbir bileşen bir temada geçip diğerinde kalmaz.
- [x] Kontrol edilebilir her alanın erişilebilir bir adı vardır.
- [x] Bu kriterler **ölçülerek** doğrulanır, göz kararıyla değil.

## Kapsam dışı

- **Yeniden tasarım.** Mevcut tasarım dili korunur; sorun renklerin *değerlerinde*
  ve token'ların *yanlış yerde kullanılmasında*, düzeninde değil.
- **Otomatik regresyon kapısı.** Denetim aracı elle çalıştırılır; CI'a bağlamak
  ayrı bir iştir.
- **AAA seviyesi erişilebilirlik.** Hedef AA.

## Kararlar

**K1 — Denetim ölçerek yapılır, bakarak değil.** Ekran görüntüsü yerleşim ve
gözden kaçan bir bozukluk için iyidir; renk için yetersizdir. `scripts/theme-audit.mjs`
hesaplanmış renkleri okuyup WCAG oranını hesaplar ve **iki temanın sonucunu
karşılaştırır**. Tema eşliği hatası ancak böyle görünür.

**K2 — Denetim sınırı ayrı bir token'dır.** Bir düğmenin nerede başlayıp bittiği
yalnızca kenarından anlaşılıyorsa o kenar 3:1 olmak zorundadır (WCAG 1.4.11);
bir kart ayracı için böyle bir zorunluluk yoktur. Bu ayrım yokken bileşenler
süsleme token'ını ödünç alıyordu. Çözüm, süsleme çizgilerini koyulaştırmak değil
— arayüz gereksizce ağırlaşırdı — ayrı bir sorumluluk tanımlamaktı.

**K3 — Yetki rozetlerinde "açık" durumu başarı rengiyle boyanmaz.** Yeşil "iyi"
demektir; "bu agent dosyalarınızı değiştirebilir" iyi bir haber değil, bir
uyarıdır.
