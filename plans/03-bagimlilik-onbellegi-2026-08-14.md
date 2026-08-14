# 03 — Bağımlılık Önbelleği: Ölçüldü, Çözülmedi

- **Tarih:** 2026-08-14
- **Durum:** Açık — çözüm düşünülecek
- **Kapsam:** Çalışma ortamı imajı ve koşu süresi (npm + Maven ortak)

---

## Özet

Çalışma ortamı her koşuda sıfırdan doğup silindiği için indirilen bağımlılıklar
da siliniyor. Gerçek bir Java projesinde bunun bedeli ölçüldü: **5 dakika 49
saniye ve 569 MB** — üstelik kurumsal Nexus iç ağdayken.

Sorun gerçek. Ama ölçüm, çözüm olarak konuşulan yöntemi de çürüttü: hazır
bağımlılıkları imaja gömmek, tahmin edilen bir listeyle yapıldığında bu projede
ağırlığın **%3'üne** dokunuyordu. Bu yüzden spec 018'den çıkarıldı ve buraya,
veriyle birlikte yazıldı.

## Ölçüm

**Ne:** `mybatis-spring-boot-starter` (çok modüllü: autoconfigure + starter +
samples), `mvn dependency:go-offline`, soğuk `.m2`, kurumsal Nexus'tan
(`maven-public` grubu, iç ağ, TLS yok — ölçülen şey indirme, el sıkışma değil).

| Ölçü | Değer |
| --- | --- |
| Süre | **349 saniye** (5:49) |
| Boyut | **569 MB** |
| Artefakt | 797 jar |
| Sonuç | BUILD SUCCESS |

**569 MB'ın dağılımı** — asıl bulgu burada:

| Grup | Boyut | Ne |
| --- | --- | --- |
| org/jetbrains | 138 MB | Kotlin — biçimlendirme eklentisinden |
| org/apache | 87 MB | Maven çekirdeği ve eklentileri |
| org/rocksdb | 59 MB | OpenRewrite'ın bağımlılığı |
| com/github | 30 MB | |
| org/bouncycastle | 29 MB | İmzalama |
| org/eclipse | 27 MB | Biçimlendirici |
| org/openrewrite | 16 MB | |
| **org/springframework** | **16 MB** | **toplamın %3'ü** |

## Ölçümün çürüttüğü şey

Tasarım şuydu: sık kullanılan Spring Boot sürümlerinin bağımlılıkları imaja
gömülsün, koşular onları hazır bulsun. İki varsayıma dayanıyordu:

1. *Ağırlık uygulama bağımlılıklarındadır.* **Yanlış.** Ağırlık, projenin ana
   POM'unun sürüklediği **derleme araç zincirinde**: Kotlin, OpenRewrite,
   RocksDB, BouncyCastle, Eclipse formatter. Spring 569 MB'ın 16'sı.
2. *Sürümden bağımsız çekirdek evrenseldir.* **Kısmen yanlış.** Sürümden
   bağımsız kısım gerçekten büyük, ama evrensel değil — o araç zinciri
   `mybatis-parent`'ın seçimi. Başka bir Spring projesi bambaşka ve çok daha
   hafif bir küme çeker.

Sonuç: tahmine dayalı bir seed listesi, isabet oranı projeden projeye savrulan
ve çoğu zaman düşük kalan bir yatırım. Karşılığında imaj ~600 MB büyüyor.

## Denenmemiş yollar

| Yol | Artı | Eksi |
| --- | --- | --- |
| Seed'i **gerçek projelerden** üretmek (bir kez derle, çıkan `.m2`'yi göm) | Ölçülen projede %100 isabet | Seed o projelerin araç zincirine bağlı; imaj ~600 MB büyür; proje değişince bayatlar |
| Kalıcı paylaşılan volume | Sürüm tahmini yok, kendi kendine ısınır | **Koşular arası yazılabilir kanal** — çalıştırılan şey modelin yazdığı kod; bir koşu diğerinin bağımlılığını zehirleyebilir. Ayrı tehdit modeli ister |
| Proje başına volume | Çapraz proje zehirlenmesi yok | Aynı projenin paralel koşularında Maven'ın yerel deposu eşzamanlı yazmaya güvenli değil; saklama politikası gerekir |
| Salt okunur seed + yazılabilir katman | İzolasyon korunur | Maven tek yerel depo okuyor; katmanlama ayrıcalıklı mount ister |

İmaj katmanının kendisi salt okunur ve container'lar ona copy-on-write baktığı
için **gömme yönteminin izolasyon sorunu yok** — volume tabanlı yolların en zor
problemi orada kendiliğinden çözülüyor. Zayıf tarafı isabet, izolasyon değil.

## Bir sonraki adımın ölçüsü

Karar tahminle değil veriyle verilmeli. Gereken ölçüm:

1. İmaja bir zaman damgası konur.
2. Gerçek bir koşudan sonra `.m2` içinde o damgadan **yeni** olan dosyalar
   listelenir — bu, tam olarak ıskalanan artefakt kümesidir, boyutlarıyla.
3. Aynı ölçüm **birkaç farklı proje** için tekrarlanır.

Kesişim büyükse gömme işe yarar ve neyin gömüleceği bellidir. Kesişim küçükse
gömme yanlış araçtır ve volume tarafının tehdit modeli konuşulmalıdır.

Aynı yöntem npm için de geçerli; npm tarafı hiç ölçülmedi (bkz. spec 018:
Kapsam dışı).

## İlgili

- [spec 018](../specs/018-maven-paket-deposu/spec.md) — Java/Maven; bu konu
  oradan çıkarıldı, gerekçesi orada da yazılı
- [spec 015](../specs/015-motor-loglari/spec.md) — saklama politikası olan bir
  özelliğin emsali; volume yolu seçilirse aynı soru orada da vardı
