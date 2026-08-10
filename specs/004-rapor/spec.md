# Spec: Rapor — Yönetici Özeti

- **Spec no:** 004
- **Tarih:** 2026-08-09
- **Durum:** Uygulandı
- **Faz:** 2 — [plans/01](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Çalıştırmalar sayfası **tek tek kayıtları** gösteriyor. Sorulan soru ise tek bir kaydın
ne yaptığı değil:

> Bu ay agent'lara ne kadar para harcadık, karşılığında ne üretildi, ne kadarı tuttu?

Bugün bunun cevabı yok. Kullanıcı listeyi kaydırıp maliyetleri kafadan toplamak zorunda;
üstelik liste sayfalı olduğu için toplam zaten görünmüyor. Hangi agent'ın veya hangi
modelin parayı yediği, hangi hatanın tekrar ettiği hiç görünmüyor.

## Amaç

Agent'ın **nasıl çalıştırıldığından bağımsız** olarak (bugün arayüz ve API, yarın
workflow) tüm çalışma geçmişini tek sayfada, sayılarla özetlemek: harcanan tutar,
üretilen değişiklik, başarı oranı, zaman içindeki seyir ve agent/model/proje kırılımı.

Bu "tek gerçek" özelliği tasarımdan gelir: her çalıştırma yolu `runs` tablosuna yazdığı
için rapor eksik kalamaz. Rapor **hiçbir yeni veri toplamaz**, var olanı toplar.

## Kullanıcı hikâyeleri

1. Yönetici olarak, dönem seçip **toplam maliyeti** ve önceki döneme göre değişimini
   görebilmeliyim.
2. Yönetici olarak, kaç iş çalıştığını, kaçının **başarılı** olduğunu ve ne kadar kod
   değişikliği üretildiğini (dosya, +/− satır, gönderilen branch) görebilmeliyim.
3. Yönetici olarak, **zaman içindeki seyri** görebilmeliyim: hangi gün kaç iş çalıştı,
   hangi gün ne kadar harcandı.
4. Yönetici olarak, **hangi agent'ın, hangi modelin, hangi projenin** ne kadar iş yaptığını
   ve neye mal olduğunu karşılaştırabilmeliyim.
5. Yönetici olarak, **tekrar eden hataları** görebilmeliyim ki düzeltilecek şeyi bileyim.
6. Kullanıcı olarak, dönemi (7/30/90 gün) ve projeyi **filtreleyebilmeliyim**.

## Kabul kriterleri

- [x] Dönem toplamları listedeki tek tek kayıtların toplamıyla **birebir tutar**.
- [x] Dönem dışındaki kayıtlar toplamlara **karışmaz**.
- [x] Kaydı olmayan günler grafikte **sıfır olarak durur**, atlanmaz.
- [x] Hiç kayıt yokken sayfa çökmez; anlamlı bir boş durum gösterir.
- [x] Önceki dönemde kayıt yoksa değişim oranı uydurulmaz, "kayıt yok" denir.
- [x] Grafiklerdeki hiçbir bilgi **yalnızca renge** dayanmaz: gösterge etiketli, yığılmış
      grafiğin tablo görünümü var.
- [x] Rapor dönemi ve saat dilimi **kodda gömülü değil**, ayarlardan gelir.
- [x] Rapor yalnızca okur; hiçbir uç veri değiştirmez.

## Kapsam dışı

- **CSV/PDF dışa aktarma.** İhtiyaç doğarsa ayrı bir iş.
- **Bütçe uyarısı / maliyet tavanı.** Rapor gösterir, sınır koymaz.
- **Kullanıcı bazında kırılım.** Sistem tek kullanıcılı (`user_id` şemada var, kullanılmıyor).
- **Workflow bazında kırılım.** Workflow'lar Faz 3; geldiklerinde kırılım eklenir,
  toplamlar zaten kapsayacak.

## Açık sorular

Yok. Dönem varsayılanı (30 gün) ve saat dilimi (Europe/Istanbul) ayar olarak
değiştirilebilir tutuldu — parametre kodda kalmasın diye.

---

## Sonradan değişen karar (2026-08-11)

Bu spec'in çerçevesi **maliyet-önce**ydi: amaç bölümü *"bu ay agent'lara ne kadar
para harcadık"* diye başlıyordu ve kahraman rakam toplam maliyet olarak seçildi.

**Yanlış çıktı.** Maliyet bir girdidir; bir yöneticinin ilk sorusu "ne kadar
harcadık" değil "karşılığında ne aldık"tır. Üstelik küçük bir maliyet rakamı
(`$0,03`) sistemi değersiz gösteriyordu.

Kahraman rakam [spec 012](../012-rapor-yonetici/spec.md) ile **üretilen işe**
geçti; maliyet ekranda kaldı ama bir paydaya bölünmüş halde ("PR başına").

Bu spec silinmedi: burada alınan karar o günkü bilgiyle tutarlıydı ve neyin
neden değiştiği, kararın kendisi kadar kayda değer.
