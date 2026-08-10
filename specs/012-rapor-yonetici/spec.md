# Spec: Rapor Ekranı — Yönetici Bakışı

- **Spec no:** 012
- **Tarih:** 2026-08-11
- **Durum:** Onaylandı
- **Önceki:** [spec 004](../004-rapor/spec.md) — bu spec onun bir kararını değiştirir

---

## Problem

Rapor ekranının en büyük rakamı **toplam maliyet**. Karar spec 004'te bilinçli
alınmıştı ve gerekçesi koda da yazılmıştı: *"Yöneticinin ilk sorusu maliyet
olduğu için o seçildi."*

Yanlış çıktı. Üç sebeple:

1. **Maliyet bir girdidir, sonuç değil.** Bir yöneticinin ilk sorusu "ne kadar
   harcadık" değil, **"karşılığında ne aldık"**tır.
2. **Küçük bir maliyet rakamı sistemi değersiz gösterir.** `$0,03` yazan bir
   tabela, arkasındaki işin büyüklüğünü değil ölçeğin küçüklüğünü anlatır.
3. **Toplam maliyet yönetilebilir bir sayı değildir.** Ölçek arttıkça büyür ve
   büyümesi bir sorun sinyali değildir. Yönetilebilir olan **birim maliyet**tir.

Bunun üstüne, ekran sistemin **en somut çıktısını hiç göstermiyor**: kaç pull
request açıldığını. Çünkü o bilgi raporun baktığı yerde durmuyor.

## Amaç

Ekranın ilk bakışta cevapladığı soru değişiyor:

> ~~"Bu ay ne kadar harcadık?"~~ → **"Bu ay ne üretildi, güvenilir miydi, birimi
> kaça mal oldu?"**

## Kullanıcı hikâyeleri

- **Yönetici olarak** ekranı açtığımda üretilen işi görmek istiyorum; harcamayı
  da görmek istiyorum ama üretilen işe bölünmüş halde.
- **Yönetici olarak** bir hız rakamının yanında onu dengeleyen bir kalite
  rakamı görmek istiyorum; yoksa yalnızca hızlandığımızı sanırım.
- **Yönetici olarak** sistemin bilmediği bir şeyi biliyormuş gibi sunmasını
  istemiyorum.
- **Ekip olarak** aynı ekranın altında hangi agent'ın, hangi modelin, hangi
  projenin ne yaptığını görmeye devam etmek istiyorum.

## Kabul kriterleri

- [ ] En büyük rakam **üretilen iş**; toplam maliyet küçük puntoda ve bir birime
      bölünmüş halde ("PR başına").
- [ ] Hız gösteren her rakamın yanında onu dengeleyen bir rakam var.
- [ ] Ekran, **ölçmediği hiçbir şeyi ima etmiyor**; ölçmediğini açıkça yazıyor.
- [ ] Hiç PR açılmamış bir kurulumda ekran "sıfır" göstererek sistemin
      çalışmadığı izlenimi vermiyor.
- [ ] İşletme detayı (agent/model/proje kırılımı, hatalar) aynı ekranda,
      yönetici bölümünün altında duruyor.
- [ ] Rapor hâlâ yalnızca okuyor; yeni tablo veya özet kaydı üretmiyor.

## Kapsam dışı

- **Türetilmiş değer rakamları.** "Şu kadar saat kazandınız" gibi bir sayı bir
  varsayıma dayanır; yanlış çıktığında yalnızca kendisi değil tüm raporun
  güvenilirliği gider. Yalnızca ölçülen veri gösterilir.
- **PR'ın sonrası.** Birleştirildi mi, review'dan geçti mi, testler geçti mi —
  bu sistem takip etmiyor ve bu fazda takip etmeye başlamayacak.
- **Kullanıcı/takım kırılımı.** Şemada kullanıcı kavramı yok.
- **Bütçe, hedef, uyarı eşiği.**

## Kararlar

**K1 — Kahraman rakam sonuçtur, girdi değil.** Yönetici ekranları için en keskin
filtre şudur: *bir sayı bir kararı değiştirmiyorsa ana ekranda yeri yoktur.*
`$0,03` hiçbir kararı değiştirmez; "12 PR açıldı" değiştirir.

**K2 — Maliyet her zaman bir paydayla gösterilir.** "PR başına $0,004", bir
yöneticinin bir insan saatiyle kıyaslayabileceği tek biçimdir. Toplam maliyet
küçük puntoda, o birimin altında durur.

*Gerekçe:* toplam maliyet ölçekle birlikte zaten büyür ve büyümesi kötü haber
değildir. Birim maliyet ise model seçimine ve verimliliğe göre hareket eder —
yani yönetilebilir.

**K3 — Hız metriği asla yalnız gösterilmez.** Yanına bir kalite/risk metriği
konur. Sektör ölçümlerinde PR sayısının katlanarak arttığı ama PR başına
olay sayısının çok daha hızlı arttığı örnekler var; tek başına PR sayısı
ilerleme gibi görünüp gerilemeyi gizleyebilir.

Dengeleyici olarak **PR başına değişen satır** seçildi: büyük değişiklik
kümeleri riskle en tutarlı biçimde ilişkilendirilen ölçüdür ve elimizde ölçülmüş
veri vardır.

**K4 — Ölçmediğimiz şey ekranda ima edilmez.** "Açılan PR" sayısı, "işe yarayan
PR" değildir. Aradaki fark ekranda yazılı olur. Bu bir dipnot değil, tasarımın
parçasıdır: kaldırılırsa ekran yanlış bir şey iddia etmeye başlar.
