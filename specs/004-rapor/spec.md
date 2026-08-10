# Spec: Rapor Ekranı

- **Spec no:** 004
- **Kapsam:** `/reports` ekranının tamamı
- **Durum:** Uygulandı
- **Sürümler:** 2026-08-09 (ilk hal) · 2026-08-11 (yönetici bakışı — eski spec 012 buraya katıldı)

> **Bu ekranın TEK spec'i budur.** Yönetici bakışı ayrı bir spec olarak
> (`012-rapor-yonetici`) yazılmıştı; aynı ekranın iki spec'i olması sonraki
> oturumlarda hangisinin geçerli olduğunu belirsiz bırakıyordu. Birleştirildi.
> Değişen kararlar silinmedi, [Karar geçmişi](#karar-geçmişi) altında duruyor.

---

## Problem

Çalıştırmalar sayfası **tek tek kayıtları** gösteriyor. Sorulan soru ise tek bir
kaydın ne yaptığı değil: bir dönem boyunca **ne üretildi, güvenilir miydi, neye
mal oldu.**

Bu ekran iki kez yanlış cevap verdi:

1. **Başlangıçta hiç cevap yoktu.** Kullanıcı listeyi kaydırıp maliyetleri
   kafadan toplamak zorundaydı; liste sayfalı olduğu için toplam zaten
   görünmüyordu.
2. **Sonra yanlış soruya cevap verdi.** Ekranın en büyük rakamı toplam maliyetti
   (`$0,03`, 48px). Maliyet bir **girdidir**, sonuç değil; bir yöneticinin ilk
   sorusu "ne kadar harcadık" değil **"karşılığında ne aldık"**tır. Üstelik
   küçük bir maliyet rakamı sistemi değersiz gösteriyordu.

## Amaç

Agent'ın **nasıl çalıştırıldığından bağımsız** olarak (arayüz, API, akış, Jira,
MCP) tüm çalışma geçmişini tek sayfada özetlemek — ve bunu bir yöneticinin
okuyabileceği hiyerarşiyle sunmak:

> **Ne üretildi → güvenilir mi → birimi kaça mal oldu.**

Rapor **hiçbir yeni veri toplamaz**, var olanı toplar. Her çalıştırma yolu aynı
kayıtları yazdığı için rapor eksik kalamaz.

## Kullanıcı hikâyeleri

1. **Yönetici olarak** ekranı açtığımda önce **üretilen işi** görmek istiyorum.
2. **Yönetici olarak** harcamayı da görmek istiyorum ama üretilen işe bölünmüş
   halde ("PR başına"), çünkü karşılaştırabileceğim biçim bu.
3. **Yönetici olarak** bir hız rakamının yanında onu dengeleyen bir kalite
   rakamı görmek istiyorum; yoksa yalnızca hızlandığımızı sanırım.
4. **Yönetici olarak** sistemin bilmediği bir şeyi biliyormuş gibi sunmasını
   istemiyorum.
5. **Yönetici olarak** dönem içindeki **seyri** görmek istiyorum.
6. **Ekip olarak** hangi agent'ın, hangi modelin, hangi projenin ne yaptığını ve
   neye mal olduğunu karşılaştırmak istiyorum.
7. **Ekip olarak** tekrar eden hataları görmek istiyorum ki düzeltilecek şeyi
   bileyim.
8. **Kullanıcı olarak** dönemi (7/30/90 gün) ve projeyi süzebilmeliyim.

## Kabul kriterleri

### Doğruluk

- [x] Dönem toplamları tek tek kayıtların toplamıyla **birebir tutar**.
- [x] Dönem dışındaki kayıtlar toplamlara karışmaz.
- [x] Kaydı olmayan günler grafikte **sıfır olarak durur**, atlanmaz.
- [x] Önceki dönemde kayıt yoksa değişim oranı **uydurulmaz**.
- [x] Hiç kayıt yokken sayfa çökmez; anlamlı bir boş durum gösterir.
- [x] Rapor yalnızca okur; hiçbir uç veri değiştirmez.
- [x] Dönem ve saat dilimi kodda gömülü değil, ayarlardan gelir.

### Hiyerarşi

- [x] En büyük rakam **üretilen iş**; toplam maliyet küçük puntoda ve bir birime
      bölünmüş halde.
- [x] Hız gösteren her rakamın yanında onu dengeleyen bir rakam var.
- [x] Ekran **ölçmediği hiçbir şeyi ima etmiyor**; ölçmediğini açıkça yazıyor.
- [x] Hiç PR açılmamış bir kurulumda ekran "sıfır" göstererek sistemin
      çalışmadığı izlenimi vermiyor.
- [x] İşletme detayı (agent/model/proje kırılımı, hatalar) aynı ekranda,
      yönetici bölümünün altında duruyor.

### Erişilebilirlik

- [x] Hiçbir bilgi **yalnızca renge** dayanmaz: gösterge etiketli, yığılmış
      grafiğin tablo görünümü var.

## Kapsam dışı

- **Türetilmiş değer rakamları.** "Şu kadar saat kazandınız" bir varsayıma
  dayanır; yanlış çıktığında yalnızca kendisi değil tüm raporun güvenilirliği
  gider. Yalnızca ölçülen veri gösterilir.
- **PR'ın sonrası.** Birleştirildi mi, incelemeden geçti mi, testler geçti mi —
  bu sistem takip etmiyor.
- **CSV/PDF dışa aktarma.**
- **Bütçe uyarısı / maliyet tavanı.** Rapor gösterir, sınır koymaz.
- **Kullanıcı bazında kırılım.** Şemada kullanıcı kavramı yok.

## Kararlar

**K1 — Rapor türetilmiş veridir.** Yeni tablo veya özet kaydı yok; her şey
çalıştırma kayıtları üzerinden toplanır. Özet tablosu ikinci bir gerçek kaynağı
yaratır ve er geç ayrışır.

**K2 — Kahraman rakam sonuçtur, girdi değil.** Yönetici ekranları için en keskin
filtre: *bir sayı bir kararı değiştirmiyorsa ana ekranda yeri yoktur.* `$0,03`
hiçbir kararı değiştirmez; "12 PR açıldı" değiştirir.

**K3 — Maliyet her zaman bir paydayla gösterilir.** Toplam harcama ölçekle
birlikte zaten büyür ve büyümesi kötü haber değildir; yönetilebilir olan birim
maliyettir ("PR başına $0,004").

**K4 — Hız metriği asla yalnız gösterilmez.** Yanına bir kalite/risk metriği
konur. Sektör ölçümlerinde PR sayısının katlanarak arttığı ama PR başına olay
sayısının çok daha hızlı arttığı örnekler var; tek başına PR sayısı ilerleme
gibi görünüp gerilemeyi gizleyebilir. Dengeleyici olarak **PR başına değişen
satır** seçildi.

**K5 — Ölçmediğimiz şey ekranda ima edilmez.** "Açılan PR", "işe yarayan PR"
değildir. Aradaki fark ekranda yazılı olur; bu bir dipnot değil, tasarımın
parçasıdır — kaldırılırsa ekran yanlış bir şey iddia etmeye başlar.

**K6 — Tek istek, altı parça.** Bölümler ayrı uçlara bölünseydi aralarında yeni
kayıt düştüğünde birbirini tutmayan rakamlar gösterilirdi.

---

## Karar geçmişi

### 2026-08-11 — kahraman rakam maliyetten üretilen işe geçti

**Eski karar (2026-08-09):** *"Yöneticinin ilk sorusu maliyet olduğu için o
seçildi."* Amaç bölümü de *"bu ay agent'lara ne kadar para harcadık"* diye
başlıyordu.

**Neden değişti:** yukarıdaki K2/K3. Araştırma (DORA 2024/2025, SPACE, DevEx,
DX Core 4, METR, Faros, GitClear) tek yöne işaret etti — sonuç önce, maliyet
paydaya bölünmüş halde, her hız metriği bir kalite metriğiyle dengeli.

**Ne değişti:** kahraman rakam açılan PR oldu; maliyet ekranda kaldı ama "PR
başına" biçiminde. Rapor ilk kez `runs` dışındaki bir kaynağa da bakmaya başladı
— çünkü PR sayısı orada yoktu.

O günkü karar o günkü bilgiyle tutarlıydı. Değişimin kendisi, kararın kendisi
kadar kayda değer.
