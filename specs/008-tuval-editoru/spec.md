# Spec: Tuval Editörü — Akışı Çizerek Kurmak

- **Spec no:** 008
- **Tarih:** 2026-08-10
- **Durum:** Uygulandı
- **Faz:** 4 — [plans/01](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Akış motoru **paralel dalları destekliyor** ama arayüz onları kuramıyor: adım
listesi editörü yalnızca doğrusal zincir üretiyor (her adım bir öncekine bağlı).
Paralel bir akış ancak API'ye elle JSON göndererek kurulabiliyor.

Üç eksik var:

1. **Şekil görünmüyor.** Akış bir liste olarak gösteriliyor; hangi adımın hangi
   adımdan sonra geldiği, nerede dallandığı, nerede birleştiği okunmuyor. Beş
   adımlı bir akışta "bu neye bağlı?" sorusunun cevabı yok.
2. **Dallanma kurulamıyor.** Bağımsız iki adımı aynı anda çalıştırmak motorun
   doğal davranışı, ama kullanıcı bunu ifade edemiyor.
3. **Çalışma izlenirken şekil kayboluyor.** İlerleme ekranı da liste; hangi
   dalın ilerlediği, hangisinin beklediği görünmüyor.

Ürünün baştan beri anlattığı şey buydu: **n8n benzeri bir tuval üzerinde
agent'ları birbirine bağlamak.**

## Amaç

Kullanıcı akışı **çizerek** kursun: adımları tuvale koysun, aralarına bağ çeksin,
bir adıma tıklayınca sağ panelde agent'ını, modelini ve talimatını düzenlesin.
Akış çalışırken aynı tuvalde adımların canlı renk değiştirdiğini görsün.

## Kullanıcı hikâyeleri

1. Kullanıcı olarak, tuvale **adım ekleyip yerleştirebilmeliyim**; koyduğum yer
   kaydedilmeli, ertesi gün açtığımda akış aynı görünmeli.
2. Kullanıcı olarak, iki adım arasına **bağ çekebilmeli** ve bağı silebilmeliyim.
3. Kullanıcı olarak, **dallanma ve birleşme** kurabilmeliyim — bir adımdan iki
   adım çıksın, ikisi de üçüncüde birleşsin.
4. Kullanıcı olarak, bir adıma tıklayınca **sağ panelde** agent, model ve
   talimatını düzenleyebilmeliyim.
5. Kullanıcı olarak, geçersiz bir akış çizdiğimde hatanın **hangi adımda**
   olduğunu tuval üzerinde görmeliyim.
6. Kullanıcı olarak, akış çalışırken **tuvalde canlı** izleyebilmeliyim: çalışan
   adım belirgin olsun, biten yeşil, hatalı kırmızı, atlanan soluk.
7. Kullanıcı olarak, tuvalde **kaybolmamalıyım**: yakınlaştırma, kaydırma ve
   "hepsini sığdır" olmalı.

## Kabul kriterleri

- [x] Dallanan ve birleşen bir akış tuvalden kurulup kaydedilebiliyor; motor onu
      **paralel** çalıştırıyor.
- [x] Düğüm konumları kaydediliyor ve geri yüklendiğinde aynı yerde duruyor.
- [x] Bağ çekmek ve silmek çalışıyor; kendine bağ ve döngü **çizilirken**
      engelleniyor veya kaydetmede açık hatayla reddediliyor.
- [x] Geçersiz akışın kusurları ilgili düğümün üzerinde görünüyor.
- [x] Çalışan akış tuvalde canlı izleniyor; sayfa yenilense de durum doğru.
- [x] Konumu olmayan eski akışlar (adım listesiyle kurulmuş olanlar) tuvalde
      **düzgün yerleşmiş** görünüyor, üst üste yığılmıyor.
- [x] Klavye ile de kullanılabiliyor: düğüm seçme, silme, panelde gezinme.
- [x] Açık ve koyu temada okunuyor; durum renkleri renk körlüğünde de ayırt
      edilebiliyor (etiket + biçim, yalnızca renk değil).

## Kapsam dışı

- **Geri al / yinele (undo/redo).** Ayrı bir iş; kaydedilmemiş değişiklik uyarısı
  yeterli koruma.
- **Kopyala-yapıştır, çoklu seçim, hizalama kılavuzları.** Tuvalin konforu; önce
  temel çalışsın.
- **Koşullu dallanma düğümü.** Hâlâ kapsam dışı (spec 007 K3). Tuval geldiğinde
  eklenmesi kolaylaşır ama karar ayrı verilecek.
- **Akış şablonları / hazır akışlar.** Faz 6.
- **Sürüm karşılaştırma (iki sürümün farkını tuvalde gösterme).** İhtiyaç
  görülürse sonra.

## Kararlar

**K1 — Tuval için React Flow (`@xyflow/react`) kullanılacak.** Bu projede grafikler
ve Markdown bilerek elle yazıldı, ama o ihtiyaçlar dardı. Tuval değil: kaydırma,
yakınlaştırma, sürükleme, bağ çekme, tutamak hizalama, dokunmatik ve klavye
erişimi — hepsi ayrı birer problem ve hepsi çözülmüş durumda. Elle yazmak fazı
belirgin şekilde uzatır ve kütüphanenin çoktan çözdüğü hataları yeniden keşfetmek
demek olur. Orijinal yol haritasında da bu vardı.

**K2 — Tuval, adım listesi editörünün yerini alır.** Aynı veriyi düzenleyen iki
ekran ikisinin de bakımını gerektirir ve er geç ayrışırlar; "iki editör aynı grafı
aynı şekilde üretiyor mu" sorusu sürekli doğrulanmak zorunda kalırdı.

**K3 — Canlı izleme tuvalde olacak.** Hangi dalın ilerlediğini, hangisinin
beklediğini yalnızca şekil gösterebilir. Adım listesi tuvalin yanında özet olarak
kalır (süre, maliyet, çalıştırma detayına bağlantı).

## Kalan açık soru

**S1 — Konumu olmayan eski akışlar ne olacak?**
*Önerim: açılışta otomatik yerleştirilsin* (seviyeye göre satır, sıraya göre
sütun) ve kullanıcı kaydettiğinde konumlar kalıcı olsun. Eski akışların üst üste
yığılması kabul edilemez; elle taşıma istemek de gereksiz.
