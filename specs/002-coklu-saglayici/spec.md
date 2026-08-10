# Spec: Çoklu LLM ve Git Sağlayıcı Desteği

- **Spec no:** 002
- **Tarih:** 2026-08-09
- **Durum:** Uygulandı (2026-08-09) — tüm kabul kriterleri doğrulandı
- **Geçersiz kıldığı:** [spec 001](../001-veri-katmani-ve-model-katalogu/spec.md) — H1, H3, H5
- **İlgili plan:** [plans/01-mimari-ve-yol-haritasi-2026-08-09.md](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Spec 001 iki noktada gereğinden dar kaldı ve bu, gerçek kullanımı engelliyor:

1. **Model erişimi tek bir servise sabitlendi.** Sistem yalnızca OpenRouter'a bağlanabiliyor.
   Kurumların çoğu kendi LLM proxy'sini (LiteLLM veya OpenAI-uyumlu başka bir servis)
   işletiyor: verinin dışarı çıkmaması, merkezi bütçe ve kota yönetimi, kurum içi
   modellere erişim gibi nedenlerle. Böyle bir kurum bugün bu sistemi hiç kullanamaz.
   Kendi proxy'sinin de içinde birden fazla model vardır ve bunlar da seçilebilmelidir.

2. **Kod deposu erişimi GitHub'a sabitlendi.** Bitbucket kullanan bir ekip destek dışı.
   Bitbucket kimlik doğrulaması GitHub'dan farklı da çalışır: tek bir token yerine
   kullanıcı adı + parola (app password) çifti ister. "Token" varsayımı yanlış.

Ayrıca 001 "tür başına tek kimlik bilgisi" kararı verdi. Bu karar, aynı anda hem kurum
içi proxy'ye hem dışarıdaki bir servise erişme ihtiyacını karşılamıyor.

## Amaç

Kullanıcı birden fazla LLM sağlayıcıyı aynı anda tanımlayabilecek ve her birinin
modellerini tek katalogda, hangi sağlayıcıya ait olduğu belli olacak şekilde görebilecek.
Kod deposu erişimi için GitHub dışındaki sağlayıcılar ve token dışındaki kimlik doğrulama
yöntemleri kullanılabilecek.

## Kapsam dışı

- **Bir agent'a veya workflow adımına model atamak.** Katalog seçilebilir hale gelir,
  atama Faz 2 ve Faz 3.
- **Pull request açmak, issue okumak.** Bu spec yalnızca *erişim bilgisinin tanımlanması*
  ve doğrulanmasıyla ilgilenir. Depo işlemleri Faz 5.
- **Sağlayıcılar arası yönlendirme, yedekleme, maliyet optimizasyonu.** Bir model tek bir
  sağlayıcıya aittir; biri düşerse diğerine düşme davranışı yoktur.
- **GitLab ve doğrudan Anthropic/OpenAI API'leri.** Bilinçli olarak kapsam dışı;
  gerekirse sonraki bir spec'te eklenir.
- **Kullanıcı hesapları ve yetkilendirme.** Sistem tek kullanıcılı.

---

## Kullanıcı Hikâyeleri

### H1 — Birden fazla LLM sağlayıcı tanımlama

**Kullanıcı** olarak, birden fazla LLM sağlayıcıyı aynı anda tanımlayabilmek istiyorum,
çünkü hassas işleri kurum içi proxy'ye, gerisini dış servise yönlendirmek istiyorum.

Kabul kriterleri:

- [ ] Ayarlar ekranından yeni bir LLM sağlayıcı eklenebilir
- [ ] Her sağlayıcı için tür seçilir: **OpenRouter**, **LiteLLM proxy**,
      **genel OpenAI-uyumlu**
- [ ] Her sağlayıcıya kullanıcının verdiği bir ad girilir (örn. "Şirket LiteLLM")
- [ ] OpenRouter dışındaki türler için servis adresi girilir
- [ ] Erişim anahtarı girilir ve **kaydetmeden önce gerçekten çalıştığı doğrulanır**;
      çalışmıyorsa kaydedilmez ve nedeni söylenir
- [ ] Aynı anda birden fazla sağlayıcı tanımlı olabilir
- [ ] Her sağlayıcı ayrı ayrı düzenlenebilir ve silinebilir
- [ ] Anahtar kaydedildikten sonra ekranda bir daha tam haliyle gösterilmez

### H2 — Sağlayıcı bazlı model kataloğu

**Kullanıcı** olarak, tüm sağlayıcılarımın modellerini tek listede görmek istiyorum,
çünkü aralarında karşılaştırma yapıp doğru olanı seçeceğim.

Kabul kriterleri:

- [ ] Modeller ekranı tüm tanımlı sağlayıcıların modellerini birlikte listeler
- [ ] Her modelin hangi sağlayıcıya ait olduğu görünür
- [ ] Sağlayıcıya göre filtrelenebilir
- [ ] Bir sağlayıcı silindiğinde onun modelleri de katalogdan kalkar
- [ ] Bir sağlayıcının kataloğu indirilemezse diğerlerininki etkilenmez;
      hangi sağlayıcının güncellenemediği ayrıca belirtilir
- [ ] Farklı sağlayıcılarda aynı isimli model bulunabilir ve bunlar karışmaz

### H3 — Eksik model bilgisiyle çalışabilme

**Kullanıcı** olarak, proxy'm fiyat ve bağlam bilgisi vermese de modellerini
kullanabilmek istiyorum, çünkü bu bilgiler bende zaten yok.

Kabul kriterleri:

- [ ] Fiyat veya bağlam uzunluğu bilinmeyen model listede yine de görünür
- [ ] **Fiyatı bilinmeyen model ücretsiz sayılır** ve öyle gösterilir
      (kullanıcı kararı, 2026-08-09 — aşağıdaki nota bkz.)
- [ ] Bağlam uzunluğu bilinmiyorsa sıfır değil, **"—"** gösterilir
- [ ] Araç (tool) desteği bilinmiyorsa model "araç desteği bilinmiyor" olarak işaretlenir;
      "desteklemiyor" denmez, çünkü agent olarak kullanılabilirliği buna bağlıdır
- [ ] Araç desteği bilinmeyen modeller filtrelenebilir

> **Not — fiyat kararı:** Fiyatı bilinmeyen bir modeli "ücretsiz" göstermenin, kullanıcının
> bedava sandığı bir modeli seçip proxy tarafında ücretlendirilmesine yol açabileceğini
> belirttim. Kullanıcı bunun sorun olmadığını söyledi; karar böyle uygulanıyor ve fiyat
> alanları için ayrı bir "bilinmiyor" durumu tutulmuyor. Araç desteği bu kolaylığın dışında
> tutuldu: yanlış varsayım orada modelin agent olarak hiç çalışmaması demek.

### H4 — Varsayılan sağlayıcı

**Kullanıcı** olarak, bir sağlayıcıyı varsayılan işaretleyebilmek istiyorum,
çünkü her yerde tek tek seçim yapmak istemiyorum.

Kabul kriterleri:

- [ ] Tanımlı sağlayıcılardan biri varsayılan olarak işaretlenebilir
- [ ] Her zaman en fazla bir varsayılan olur
- [ ] İlk eklenen sağlayıcı kendiliğinden varsayılan olur
- [ ] Varsayılan sağlayıcı silinirse kalanlardan biri varsayılan olur

### H5 — Git sağlayıcı ve kimlik doğrulama yöntemi

**Kullanıcı** olarak, GitHub dışındaki depolara da erişim tanımlayabilmek istiyorum,
çünkü ekibim Bitbucket kullanıyor.

Kabul kriterleri:

- [ ] Git erişimi eklenirken tür seçilir: **GitHub**, **Bitbucket**, **genel Git**
- [ ] GitHub için token girilir
- [ ] Bitbucket için kullanıcı adı ve parola (app password) çifti girilir
- [ ] Genel Git için adres, kullanıcı adı ve parola/token girilir
- [ ] Kendi sunucusunda barındırılan kurulumlar için servis adresi girilebilir
- [ ] Girilen bilgi kaydedilmeden önce doğrulanır; genel Git türünde doğrulama
      yapılamıyorsa bu kullanıcıya açıkça söylenir ve kayıt yine de yapılabilir
- [ ] Birden fazla git erişimi aynı anda tanımlı olabilir

### H6 — Mevcut kurulumun bozulmaması

**Kullanıcı** olarak, sistemi güncellediğimde daha önce girdiğim OpenRouter anahtarımın
yerinde durmasını istiyorum, çünkü yeniden girmek istemiyorum.

Kabul kriterleri:

- [ ] Spec 001 ile kaydedilmiş OpenRouter anahtarı, güncelleme sonrası
      "OpenRouter" türünde bir sağlayıcı olarak görünür ve çalışır
- [ ] Aynı şekilde kaydedilmiş GitHub ve Jira bilgileri de korunur
- [ ] Ortam dosyasındaki anahtar yedek olarak geçerliliğini korur
- [ ] Güncelleme sonrası model kataloğu elle işlem gerektirmeden çalışır

---

## Davranış Kuralları

- **Bir model her zaman bir sağlayıcıya aittir.** Katalogda bir modeli tanımlayan şey
  tek başına adı değil, sağlayıcı + ad çiftidir.
- **Fiyat bilinmiyorsa sıfır sayılır** (kullanıcı kararı). Bağlam uzunluğu ve araç desteği
  için bu geçerli değildir: bilinmeyen bağlam sıfır bağlam, bilinmeyen araç desteği
  "desteklemiyor" anlamına gelmez.
- **Bir sağlayıcının arızası diğerlerini etkilemez.** Katalog güncellemesi sağlayıcı
  bazında başarılı veya başarısız olur.
- **Gizli değerler için 001'in kuralları aynen geçerlidir:** şifreli saklanır, loglara ve
  yanıtlara düşmez, kaydedildikten sonra maskeli gösterilir.
- **Doğrulama, sağlayıcının kendi yöntemiyle yapılır.** Her türün geçerlilik kontrolü farklıdır.

## Hata Durumları

| Durum | Beklenen davranış |
|-------|-------------------|
| Servis adresi biçimsel olarak geçersiz | Kaydedilmez; adres alanı hatalı olarak işaretlenir |
| Adres doğru ama servis yanıt vermiyor | Kaydedilmez; "adrese ulaşılamadı" denir, tekrar denenebilir |
| Adres yanıt veriyor ama anahtar reddediliyor | Kaydedilmez; "anahtar doğrulanamadı" denir |
| Servis beklenen biçimde model listesi vermiyor | Sağlayıcı kaydedilir ama "katalog okunamadı" uyarısı gösterilir |
| Bir sağlayıcının kataloğu indirilemiyor | Diğerleri güncellenir; yalnızca o sağlayıcı "güncellenemedi" işaretlenir |
| Hiç sağlayıcı tanımlı değil | Modeller ekranı "önce bir LLM sağlayıcı ekleyin" yönlendirmesi gösterir |
| Bitbucket'ta kullanıcı adı girilmemiş | Kaydedilmez; eksik alan belirtilir |
| Silinen sağlayıcının modelleri başka yerde kullanımda | Bu spec kapsamında kullanım yok; Faz 3'te ele alınacak |

---

## Belirsizlikler

Yok — üç belirleyici soru 2026-08-09'da cevaplandı:

- Birden fazla LLM sağlayıcı aynı anda tanımlı olabilir.
- Desteklenen türler: OpenRouter, LiteLLM proxy, genel OpenAI-uyumlu.
- Git türleri: GitHub (token), Bitbucket (kullanıcı adı + parola), genel Git.
  GitLab ve doğrudan Anthropic/OpenAI kapsam dışı.

## Bağımlılıklar

- Spec 001 uygulanmış olmalı — **tamamlandı**. Bu spec onun şemasını değiştirir ve
  verisini taşır.
- Faz 2 (agent çalıştırma) bu spec'e bağlıdır: runner içindeki opencode'un yapılandırması
  artık sabit değil, seçilen sağlayıcıya göre üretilecek.
