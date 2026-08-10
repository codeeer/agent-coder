# Spec: Projeler, Agent Tanımları ve Agent Çalıştırma

- **Spec no:** 003
- **Tarih:** 2026-08-09
- **Durum:** Uygulandı
- **Faz:** 2 — [plans/01](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Sistem bugün model kataloğunu gösterebiliyor ama **hiçbir iş yapamıyor.** Kullanıcı bir
agent'ı bir kod deposu üzerinde çalıştıramıyor; ürünün asıl vaadi olan "AI agent'ı senin
kodun üzerinde çalıştır" kısmı henüz yok.

Üç eksik var:

1. **Depo tanımı yok.** Hangi depo, hangi branch, hangi erişimle — bunları tutacak bir yer
   yok. Her çalıştırmada elle girmek sürdürülebilir değil ve workflow'lar da (Faz 3) aynı
   bilgiye ihtiyaç duyacak.
2. **Agent tanımları düzenlenemiyor.** Beş hazır agent dosyada duruyor; kullanıcı ne
   görebiliyor, ne kendi agent'ını oluşturabiliyor, ne de prompt'larını kendi işine
   uyarlayabiliyor. Oysa "code review agent'ımız şu kurallara baksın" ihtiyacı en temel
   özelleştirme.
3. **Çalıştırma yok.** Agent'ı başlatacak, ilerlemesini gösterecek, sonucunu (çıktı, diff,
   maliyet) sunacak hiçbir ekran veya uç yok.

## Amaç

Kullanıcı bir kod deposunu proje olarak tanımlayabilecek, agent'larını görüp
düzenleyebilecek ve seçtiği bir agent'ı seçtiği modelle o proje üzerinde çalıştırıp
sonucunu — canlı ilerleme, üretilen değişiklikler, harcanan tutar — görebilecek.
İsterse üretilen değişiklikleri yeni bir branch'e gönderebilecek.

## Kapsam dışı

- **Workflow'lar, tuval, adımların birbirine bağlanması.** Bu spec tek bir agent'ın tek
  bir çalıştırılmasıyla ilgilenir. Zincirleme Faz 3.
- **Pull request açmak, Jira'ya yorum yazmak.** Push edilir, PR Faz 5.
- **Eşzamanlı çok sayıda çalıştırma için kuyruk yönetimi.** Bir üst sınır olacak; sıraya
  alma ve öncelik Faz 3'te workflow motoruyla gelecek.
- **Agent'ın kullanabileceği araçların tek tek seçilmesi.** Yetkiler kaba düzeyde
  (dosya değiştirme, komut çalıştırma, ağ erişimi) ayarlanır.
- **Çalıştırma geçmişinde arama, raporlama, maliyet analizi.** Geçmiş saklanır ve
  listelenir; analiz sonraki spec'lere kalır.

---

## Kullanıcı Hikâyeleri

### H1 — Proje tanımlama

**Kullanıcı** olarak, üzerinde çalışılacak kod depolarını bir kez tanımlamak istiyorum,
çünkü her çalıştırmada adres ve branch girmek istemiyorum.

Kabul kriterleri:

- [ ] Proje eklenirken ad, depo adresi ve varsayılan branch girilir
- [ ] Hangi git erişiminin kullanılacağı, tanımlı git erişimleri arasından seçilir
- [ ] Erişim gerektirmeyen açık depolar için git erişimi seçilmeyebilir
- [ ] Kaydetmeden önce depoya gerçekten erişilebildiği doğrulanır; erişilemiyorsa
      kaydedilmez ve nedeni söylenir
- [ ] Projeler listelenebilir, düzenlenebilir ve silinebilir
- [ ] Bir projenin geçmiş çalıştırmaları varken silinirse ne olacağı kullanıcıya sorulur

### H2 — Agent tanımlarını görme ve düzenleme

**Kullanıcı** olarak, agent'ların ne yaptığını görmek ve kendi ihtiyacıma göre
değiştirmek istiyorum, çünkü ekibimin kod inceleme kuralları hazır prompt'takinden farklı.

Kabul kriterleri:

- [ ] Hazır beş agent (analyst, coder, reviewer, tester, upgrader) listede görünür
- [ ] Her agent'ın adı, açıklaması, talimatı ve yetkileri görüntülenebilir
- [ ] Hazır agent'ların talimatı düzenlenebilir; düzenlenmiş olan açıkça işaretlenir
- [ ] Düzenlenmiş bir hazır agent, tek tıkla özgün haline döndürülebilir
- [ ] Kullanıcı sıfırdan yeni agent oluşturabilir
- [ ] Her agent için varsayılan sağlayıcı ve model seçilebilir
- [ ] Her agent için yetkiler ayarlanabilir: dosya değiştirme, komut çalıştırma, ağ erişimi
- [ ] Kullanıcının oluşturduğu agent silinebilir; hazır agent silinemez, yalnızca sıfırlanabilir

### H3 — Agent çalıştırma

**Kullanıcı** olarak, seçtiğim agent'ı seçtiğim modelle bir projem üzerinde çalıştırmak
istiyorum, çünkü ürünün varlık sebebi bu.

Kabul kriterleri:

- [ ] Çalıştırma ekranında proje, branch, agent ve model seçilir
- [ ] Agent'a verilecek görev metni girilir
- [ ] Model seçimi katalogdan yapılır; **araç desteği olmayan modeller uyarıyla işaretlenir**
- [ ] Model seçilmezse agent'ın varsayılan modeli kullanılır
- [ ] Çalıştırma başlatıldığında kullanıcı beklemek zorunda kalmaz; ilerleme ekranına geçilir
- [ ] Aynı anda çalışabilecek iş sayısının bir üst sınırı vardır; sınır aşılırsa
      kullanıcıya bunu söyleyen bir mesaj gösterilir

### H4 — Çalışmayı canlı izleme

**Kullanıcı** olarak, agent çalışırken ne yaptığını görmek istiyorum,
çünkü dakikalarca boş ekrana bakmak istemiyorum.

Kabul kriterleri:

- [ ] Çalışma sırasında ilerleme canlı olarak akar (hangi adımda, hangi dosyaya dokundu)
- [ ] Durum her an görünür: hazırlanıyor, çalışıyor, tamamlandı, başarısız, iptal edildi
- [ ] Sayfa yenilense veya kapatılıp açılsa bile o ana kadarki ilerleme kaybolmaz
- [ ] Çalışan bir iş kullanıcı tarafından iptal edilebilir
- [ ] İptal edilen işin container'ı ve geçici verisi temizlenir

### H5 — Sonucu görme

**Kullanıcı** olarak, çalışma bitince ne yapıldığını ve ne kadara mal olduğunu görmek
istiyorum, çünkü hem sonucu değerlendirmem hem bütçemi bilmem gerekiyor.

Kabul kriterleri:

- [ ] Agent'ın metin çıktısı görüntülenir
- [ ] Kod değişikliği olduysa dosya bazında diff görüntülenir
- [ ] Harcanan token sayısı ve tutar görüntülenir
- [ ] Hangi agent, hangi model, hangi proje ve branch ile çalışıldığı görünür
- [ ] Çalışma başarısız olduysa nedeni okunabilir biçimde gösterilir
- [ ] Geçmiş çalıştırmalar listelenir ve tekrar açılabilir

### H6 — Değişiklikleri branch'e gönderme

**Kullanıcı** olarak, beğendiğim değişiklikleri depoma göndermek istiyorum,
çünkü sonucu elle kopyalamak anlamsız.

Kabul kriterleri:

- [ ] Kod değişikliği üreten bir çalışma sonunda "branch'e gönder" seçeneği sunulur
- [ ] Gönderim onay ister; kendiliğinden gerçekleşmez
- [ ] Yeni branch adı önerilir ve kullanıcı değiştirebilir
- [ ] Gönderim sonucu (başarılı / başarısız, branch adı) gösterilir
- [ ] Projenin git erişimi tanımlı değilse bu seçenek sunulmaz ve nedeni belirtilir
- [ ] Aynı çalışma iki kez gönderilmeye çalışılırsa kullanıcı uyarılır

### H7 — Çalışma parametrelerini ayarlardan yönetme

**Kullanıcı** olarak, süre sınırı ve eşzamanlı iş sayısı gibi değerleri arayüzden
değiştirebilmek istiyorum, çünkü bunlar makinemin gücüne ve işimin türüne göre değişir;
kodun içinde sabit kalmaları kabul edilemez.

Kabul kriterleri:

- [ ] Ayarlar ekranında "Çalışma ayarları" bölümü vardır
- [ ] Şunlar değiştirilebilir: **çalışma süre sınırı** (varsayılan 30 dakika),
      **aynı anda çalışabilecek iş sayısı** (varsayılan 3), **iş başına CPU ve bellek
      sınırı**, **depo klonlama derinliği**, **model kataloğu yenileme aralığı**
      (varsayılan 24 saat)
- [ ] Her ayarın ne işe yaradığı ekranda açıklanır
- [ ] Geçersiz değerler kabul edilmez ve sınırları söylenir (örn. "1 ile 240 arasında olmalı")
- [ ] Değiştirilen ayar veritabanında saklanır ve yeniden başlatmaya dayanır
- [ ] Her ayar tek tıkla varsayılanına döndürülebilir
- [ ] Varsayılandan farklı olan ayarlar listede işaretlenir
- [ ] Değişiklik, sunucu yeniden başlatılmadan geçerli olur

---

## Davranış Kuralları

- **Her çalıştırma izole edilir.** Bir çalışma başka bir çalışmanın dosyalarını göremez,
  değiştiremez.
- **Çalışma bitince geçici hiçbir şey kalmaz.** Başarı, hata, iptal ve zaman aşımı —
  dördünde de container ve geçici veri temizlenir.
- **Sonsuza kadar çalışan iş olmaz.** Her çalıştırmanın bir süre sınırı vardır; aşılırsa
  iş durdurulur ve bu durum kullanıcıya "zaman aşımı" olarak bildirilir.
- **Gizli değerler için 001 ve 002'nin kuralları aynen geçerlidir.** Ayrıca: agent'a
  verilen kimlik bilgileri çalışma çıktısına, loglara ve diff'e sızmamalıdır.
- **Kullanıcının deposu kirletilmez.** Sistemin kendi yapılandırma dosyaları klonlanan
  depoya yazılmaz; üretilen diff yalnızca agent'ın yaptığı değişiklikleri içerir.
- **Maliyet her zaman kaydedilir**, çalışma başarısız olsa bile — harcama gerçekleşmiştir.
- **Davranışı belirleyen parametreler kodda sabit tutulmaz.** Süre sınırı, eşzamanlılık,
  kaynak limitleri, yenileme aralığı gibi değerler veritabanında saklanır ve arayüzden
  değiştirilebilir. Kodda yalnızca *varsayılanları* ve *geçerli aralıkları* tanımlıdır.
- **Altyapı ayarları bunun dışındadır.** Veritabanı adresi, portlar, şifreleme anahtarı,
  runner imajının adı gibi sistemin ayağa kalkması için gereken değerler ortam
  değişkeninde kalır — veritabanından okunamazlar, çünkü veritabanına bağlanmak için
  gerekirler.

## Hata Durumları

| Durum | Beklenen davranış |
|-------|-------------------|
| Depo adresi geçersiz veya erişilemiyor | Proje kaydedilmez; nedeni söylenir |
| Seçilen branch depoda yok | Çalışma "başarısız" olur, neden açıkça yazılır |
| LLM sağlayıcı anahtarı geçersiz | Çalışma başlamadan durur, ayarlara yönlendirilir |
| Model araç desteklemiyor | Uyarı gösterilir; kullanıcı yine de çalıştırabilir |
| Agent çalışırken model hatası | Çalışma "başarısız", o ana kadarki çıktı ve maliyet korunur |
| Süre sınırı aşıldı | Çalışma "zaman aşımı", container temizlenir, kısmi çıktı korunur |
| Eşzamanlı iş sınırı dolu | Yeni çalıştırma reddedilir, kaç iş çalıştığı söylenir |
| Sistem çalışma ortasında yeniden başlatıldı | İş "kesildi" olarak işaretlenir, sonsuza kadar "çalışıyor" görünmez |
| Push sırasında yetki hatası | Çalışma sonucu korunur; yalnızca gönderim başarısız sayılır |
| Agent hiç değişiklik üretmedi | Başarılı sayılır; "değişiklik yok" olarak gösterilir |

---

## Belirsizlikler

Üç belirleyici soru 2026-08-09'da cevaplandı:

- [x] **Depo nasıl belirlenir?** → Proje tanımı; çalıştırırken listeden seçilir.
- [x] **Değişikliklere ne olur?** → Diff gösterilir, kullanıcı isterse branch'e gönderir.
- [x] **Agent'lar düzenlenebilir mi?** → Evet, tam CRUD; prompt ve yetkiler düzenlenebilir.

- [x] **Süre sınırı ve eşzamanlı iş sınırı ne olmalı?**
      → **Cevap:** Varsayılan 30 dakika ve 3 iş; ancak bu değerler **kodda sabit kalmayacak**.
      Ayarlar ekranından değiştirilip veritabanında saklanacaklar (H7). Kural genelleştirildi:
      davranışı belirleyen hiçbir parametre kodda gömülü kalmaz.

## Bağımlılıklar

- Spec 001 ve 002 uygulanmış olmalı — **tamamlandı**. Çalıştırma, tanımlı bir LLM
  sağlayıcıya ve katalogdan seçilen bir modele ihtiyaç duyar; push, tanımlı bir git
  erişimine.
- Bu spec Faz 3'ün (workflow motoru) önkoşuludur: workflow adımları aynı çalıştırma
  altyapısını kullanacak.
