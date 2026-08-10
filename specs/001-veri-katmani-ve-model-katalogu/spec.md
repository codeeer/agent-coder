# Spec: Veri Katmanı ve Model Kataloğu

- **Spec no:** 001
- **Tarih:** 2026-08-09
- **Durum:** Uygulandı (2026-08-09) — **kısmen geçersiz kılındı**

> **Bu spec'in H1, H3 ve H5 hikâyeleri [spec 002](../002-coklu-saglayici/spec.md) ile
> geçersiz kılınmıştır.** 001 tek bir LLM sağlayıcıya (OpenRouter) ve tek bir git
> sağlayıcıya (GitHub, token ile) göre yazılmıştı; 002 bunları çoğullaştırdı.
> Bu dosya tarihsel kayıt olarak olduğu gibi bırakılmıştır — değiştirilmemelidir.
> Hâlâ geçerli olanlar: H2 (model kataloğu), H4 (arama/filtre), H6 (kalıcılık) ve
> gizli değerlere dair tüm davranış kuralları.
- **İlgili plan:** [plans/01-mimari-ve-yol-haritasi-2026-08-09.md](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md) — Faz 1

---

## Problem

Sistem şu an hiçbir şeyi hatırlamıyor. Üç somut sonucu var:

1. **Kimlik bilgileri dosyada.** OpenRouter anahtarı `.env` dosyasında düz metin duruyor.
   Değiştirmek için dosyayı elle düzenleyip servisleri yeniden başlatmak gerekiyor.
   GitHub ve Jira erişimi için de aynı yol izlenecek — sürdürülebilir değil.
2. **Hangi modellerin var olduğu bilinmiyor.** Kullanıcının bir agent'a model seçebilmesi için
   önce hangi modellerin bulunduğunu, ne kadar bağlam aldıklarını, ne kadara mal olduklarını ve
   araç kullanmayı destekleyip desteklemediklerini görebilmesi gerekiyor. Bugün bu bilgi
   yalnızca OpenRouter'ın kendi sitesinde.
3. **Kalıcı depolama hiç kullanılmıyor.** Veritabanı ayakta ama boş; üzerine hiçbir şey
   yazılmıyor. Workflow'lar, agent'lar ve çalışma geçmişi bu temel olmadan yazılamaz.

## Amaç

Kullanıcı arayüzden kimlik bilgilerini girip yönetebilecek ve kullanabileceği modellerin
tam listesini fiyatlarıyla birlikte görebilecek. Girilen bilgiler yeniden başlatmalara
dayanacak ve gizli değerler bir daha okunabilir biçimde ortaya çıkmayacak.

## Kapsam dışı

- **Agent çalıştırma.** Model seçilebilir hale gelecek ama bir agent'ı çalıştırmak Faz 2.
- **Workflow tanımları ve çalışma geçmişi.** Bunların veri yapısı Faz 3'te tasarlanacak.
- **GitHub ve Jira'ya bağlanmak.** Bu fazda yalnızca kimlik bilgileri *saklanır*.
  Gerçek entegrasyon Faz 5.
- **Kullanıcı hesapları, giriş ekranı, yetkilendirme.** Sistem tek kullanıcılı.
- **Model karşılaştırma, öneri veya otomatik seçim.** Katalog yalnızca listeler.

---

## Kullanıcı Hikâyeleri

### H1 — OpenRouter anahtarını arayüzden kaydetme

**Kullanıcı** olarak, OpenRouter anahtarımı ayarlar ekranından girmek istiyorum,
çünkü dosya düzenleyip servis yeniden başlatmak istemiyorum.

Kabul kriterleri:

- [ ] Ayarlar ekranında OpenRouter anahtarı girilebilen bir alan vardır
- [ ] Geçerli bir anahtar kaydedildiğinde başarı bildirimi gösterilir
- [ ] Kaydetmeden önce anahtarın gerçekten çalıştığı doğrulanır; çalışmıyorsa
      kaydedilmez ve nedeni kullanıcıya söylenir
- [ ] Kaydedilen anahtar servisler yeniden başlatıldıktan sonra da durur
- [ ] Anahtar kaydedildikten sonra ekranda **bir daha tam haliyle gösterilmez** —
      yalnızca son birkaç karakteri görünür

### H2 — Kaydedilmiş kimlik bilgisini değiştirme ve silme

**Kullanıcı** olarak, kaydettiğim bir anahtarı değiştirebilmek veya silebilmek istiyorum,
çünkü anahtarlar süresi dolduğunda veya sızdığında hızla döndürülmeli.

Kabul kriterleri:

- [ ] Kayıtlı her kimlik bilgisi için "değiştir" ve "sil" seçenekleri vardır
- [ ] Değiştirme, yeni değeri aynı doğrulamadan geçirir
- [ ] Silme işlemi onay ister
- [ ] Silinen kimlik bilgisi listeden kaybolur ve yeniden başlatmadan sonra da geri gelmez

### H3 — Kullanılabilir modelleri görme

**Kullanıcı** olarak, hangi modelleri kullanabileceğimi fiyatlarıyla görmek istiyorum,
çünkü her agent adımı için doğru maliyet/yetenek dengesini kurmam gerekiyor.

Kabul kriterleri:

- [ ] Modeller ekranı, anahtarımla erişebildiğim modellerin listesini gösterir
- [ ] Her model için şunlar görünür: adı, sağlayıcısı, bağlam uzunluğu,
      milyon token başına girdi ve çıktı fiyatı
- [ ] Araç (tool) kullanımını destekleyen modeller açıkça işaretlenir —
      agent olarak yalnızca bunlar kullanılabilir
- [ ] Ücretsiz modeller "ücretsiz", önizleme/deneysel modeller "önizleme" rozetiyle
      işaretlenir; listeden çıkarılmaz
- [ ] Liste, anahtar kaydedildikten sonra elle bir işlem yapmaya gerek kalmadan dolar
- [ ] Listenin ne zaman güncellendiği görünür ve kullanıcı "Yenile" ile zorlayabilir
- [ ] Liste servis açılışında bir kez, sonra günde bir kendiliğinden tazelenir

### H4 — Katalogda arama ve filtreleme

**Kullanıcı** olarak, 400 model arasından aradığımı bulabilmek istiyorum,
çünkü düz bir liste bu boyutta kullanışsız.

Kabul kriterleri:

- [ ] Model adına veya sağlayıcısına göre arama yapılabilir
- [ ] "Yalnızca araç destekleyenler" ve "yalnızca ücretsizler" filtreleri vardır
- [ ] Fiyata ve bağlam uzunluğuna göre sıralanabilir
- [ ] Arama sonuç vermezse bunu açıkça söyleyen bir mesaj görünür

### H5 — GitHub ve Jira kimlik bilgilerini saklama

**Kullanıcı** olarak, GitHub ve Jira erişim bilgilerimi şimdiden kaydedebilmek istiyorum,
çünkü entegrasyonlar geldiğinde hazır olsun.

Kabul kriterleri:

- [ ] Ayarlar ekranında GitHub ve Jira için de kimlik bilgisi alanları vardır
- [ ] Bu bilgiler OpenRouter anahtarıyla aynı gizlilik kurallarına tabidir
      (kaydedildikten sonra tam haliyle gösterilmez)
- [ ] Bu bilgilerin henüz hiçbir yerde kullanılmadığı arayüzde belirtilir
- [ ] Her türden yalnızca bir kayıt tutulur; ikinci kez kaydetmek mevcudu değiştirir

### H6 — Sistemin durumu koruması

**Kullanıcı** olarak, sistemi kapatıp açtığımda ayarlarımın yerinde olmasını istiyorum,
çünkü her açılışta yeniden yapılandırmak kabul edilemez.

Kabul kriterleri:

- [ ] Servisler durdurulup başlatıldıktan sonra kayıtlı kimlik bilgileri durur
- [ ] Model kataloğu yeniden indirilmeye gerek kalmadan görünür
- [ ] Sistem, veritabanı henüz hazır değilken açılışta çökmez; hazır olana kadar bekler

---

## Davranış Kuralları

- **Gizli değerler okunabilir biçimde saklanmaz.** Veritabanına doğrudan bakan biri
  anahtarları göremez.
- **Gizli değerler hiçbir log satırına, hata mesajına veya sunucu yanıtına düşmez.**
  Kaydedildikten sonra sunucudan dışarı yalnızca maskelenmiş hali çıkar.
- **Dosyadaki anahtar yedek olarak geçerli kalır.** Arayüzden kayıt yapılmamışsa
  `.env` dosyasındaki değer kullanılır. Arayüzden kayıt varsa o öncelikli olur.
  Bu, Faz 0'daki çalışan kurulumun bozulmamasını sağlar.
- **Katalog, kullanıcının gerçekten erişebildiği modelleri yansıtır.** Anahtarla
  erişilemeyen modeller listelenmez.
- **Katalog yaşlanabilir.** İnternet erişimi yoksa en son indirilen liste gösterilir
  ve kullanıcıya listenin güncel olmadığı belirtilir.
- **Fiyatlar okunabilir birimde gösterilir** — milyon token başına ABD doları.

## Hata Durumları

| Durum | Beklenen davranış |
|-------|-------------------|
| Girilen anahtar geçersiz | Kaydedilmez; "anahtar doğrulanamadı" mesajı gösterilir |
| Anahtar doğrulanırken servise ulaşılamıyor | Kaydedilmez; ağ sorunu olduğu belirtilir, tekrar denenebilir |
| Katalog indirilemiyor, elde eski liste var | Eski liste gösterilir + "güncellenemedi, son güncelleme: ..." uyarısı |
| Katalog indirilemiyor, elde liste yok | Boş liste yerine "modeller yüklenemedi, tekrar deneyin" mesajı |
| Kayıtlı anahtar yok, katalog isteniyor | "Önce ayarlardan OpenRouter anahtarınızı girin" yönlendirmesi |
| Veritabanı erişilemez durumda | Servis hata döndürür ve durumu loglar; sessizce boş sonuç dönmez |
| Aynı türden ikinci kimlik bilgisi ekleniyor | Değiştirme olarak işlem görür, sessizce ikinci kayıt oluşmaz |

---

## Belirsizlikler

Üçü de 2026-08-09'da cevaplandı.

- [x] **Aynı türden birden fazla kimlik bilgisi gerekli mi?**
      → **Cevap:** Hayır. Tür başına tek kayıt (bir OpenRouter, bir GitHub, bir Jira).
      Ayarlar ekranında kimlik bilgisi seçimi yapılmaz. Çoklu desteğe sonradan geçilebilir.
- [x] **Katalog ne sıklıkta kendiliğinden yenilensin?**
      → **Cevap:** Servis açılışında bir kez, sonra 24 saatte bir arka planda.
      Kullanıcı "Yenile" ile her an zorlayabilir.
- [x] **Ücretsiz ve deneysel modeller listelensin mi?**
      → **Cevap:** Listelensin, ama "ücretsiz" / "önizleme" rozetiyle işaretlensin.
      Kullanıcı riski görerek seçsin.

Bu cevaplar aşağıdaki kabul kriterlerine yansıtıldı.

## Bağımlılıklar

- Faz 0 (iskelet) tamamlanmış olmalı — **tamamlandı**.
- Bu spec, Faz 2'nin (agent çalıştırma) önkoşuludur: agent'a model seçmek için katalog,
  OpenRouter'a bağlanmak için kimlik bilgisi gerekir.
