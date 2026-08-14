# Tasks: Bitbucket grubundan toplu proje ekleme

- **Spec no:** 021 — [spec.md](spec.md) · [plan.md](plan.md)
- **Tarih:** 2026-08-14

Sıra plandaki gerekçeye uyar: en çok bilinmezlik taşıyan parça (adres
ayrıştırma ve sayfalı listeleme) başta. İlk üç blok veritabanı ve arayüz
olmadan test edilebilir.

Gerçek bir kurumsal sunucuda ölçüm **yapılmayacak** (spec kararı). Bu yüzden
testlerin işi iki katına çıkıyor: sahte sunucu kendi kodumuzu kilitler, ham
yanıtı saklamayan hata mesajları ise sahadan gelecek bildirimi işe yarar kılar.

---

## Blok 1 — Grup adresinin çözülmesi

- [ ] T01 `internal/bitbucket` paketi: `ParseGroupURL` düz grup adresini çözer →
      `https://bb.sirket.com/projects/ODEME` → base `https://bb.sirket.com`,
      key `ODEME`
- [ ] T02 Context path korunur → `https://sirket.com/bitbucket/projects/ODEME`
      → base `https://sirket.com/bitbucket`
- [ ] T03 Sondaki eğik çizgi ve fazladan yol parçası tolere edilir →
      `…/projects/ODEME/`, `…/projects/ODEME/repos/api/browse` aynı sonucu verir
- [ ] T04 Kişisel repository yolu çözülür → `…/users/ahmet` → key `~AHMET`
- [ ] T05 Grup adresi olmayan girdi reddedilir → `ErrNotGroupURL`; mesaj
      beklenen biçimi **örnekle** söyler
- [ ] T06 Bulut adresi ayrı hata döner → `api.bitbucket.org` ve
      `bitbucket.org/…` için `ErrCloudAddress`; denetim ayrıştırmadan ÖNCE
      koşar (plan → tuzaklar)

## Blok 2 — Repository listesinin çekilmesi

- [ ] T10 `Client.ListRepos` tek sayfalık yanıtı ayrıştırır → `httptest`
      sunucusundan 3 repository döner
- [ ] T11 **Sayfalama tükenene kadar devam eder** → `httptest` iki sayfa
      döndürür (`isLastPage:false` + `nextPageStart`), sonuç 30 kayıt olur.
      Tek çağrı yapan kod bu testte düşer
- [ ] T12 Klonlama adresi `links.clone` içinden **http** olanı seçer →
      http+ssh karışık girdide http seçilir; ssh yoksa da çalışır
- [ ] T13 Adrese gömülü kullanıcı adı ayıklanır →
      `https://ahmet@bb/scm/ODEME/api.git` → `https://bb/scm/ODEME/api.git`.
      Ayıklanmazsa `Input.Normalize` her kaydı reddederdi
- [ ] T14 `archived` alanı yoksa `false` sayılır → alanı taşımayan yanıtta
      hiçbir repository arşivli işaretlenmez
- [ ] T15 Hata sınıflandırma → 404 `ErrGroupNotFound`, 401/403 `ErrForbidden`,
      bağlantı hatası `ErrUnreachable`; her biri ayrı test
- [ ] T16 Ayrıştırılamayan gövde sessiz boş liste DÖNMEZ → hata döner ve
      mesaj ham gövdenin kısaltılmış halini taşır (plan → riskler)

## Blok 3 — Varsayılan branch ve mükerrer denetimi

- [ ] T20 `Verifier.DefaultBranch`: `ls-remote --symref` çıktısından branch
      okunur → `ref: refs/heads/develop\tHEAD` girdisinden `develop` çıkar
- [ ] T21 HEAD okunamazsa **hata döner, varsayılan uydurulmaz** → boş çıktıda
      hata; `main` dönmez (spec → davranış kuralları)
- [ ] T22 Aynı çağrı erişimi de sınar → erişim reddinde `ErrRepoAuth`,
      ulaşılamamada `ErrRepoUnreachable` (mevcut sınıflandırma paylaşılır)
- [ ] T23 `Store.ExistingRepoURLs` kayıtlı adresleri normalize edilmiş
      anahtarlarla döner → büyük/küçük harf, sondaki `.git` ve `/` farkı olan
      iki adres aynı anahtara düşer

## Blok 4 — Önizleme ucu

- [ ] T30 `POST /api/projects/import/preview` grup listesini durum etiketiyle
      döner → sahte sunucuya karşı `status: new` ve `already_registered`
      birlikte görülür
- [ ] T31 Arşivli repository `archived: true` ile işaretlenir → arayüz onu
      seçimsiz gösterebilsin diye
- [ ] T32 Erişim tanımı yoksa veya seçilmediyse 4xx ve **ne yapılacağını**
      söyleyen mesaj → "Ayarlar → Git repository'ler" yolu yazılı
- [ ] T33 Hata durumlarının tamamı spec tablosuna uyar → grup yok, yetki yok,
      ulaşılamıyor, bulut adresi, grup adresi değil: beşi için ayrı test

## Blok 5 — İçe aktarma ucu

- [ ] T40 `POST /api/projects/import` seçilenleri kaydeder → 3 repository
      gönderilir, 3 proje oluşur, her biri kendi adı/adresi/branch'iyle
- [ ] T41 Erişim bilgisi **bir kez** çözülür, N repository'de kullanılır →
      sağlayıcı deposundan tek okuma yapıldığı testle kilitlenir
- [ ] T42 Sınama paralel koşar, sınırı 8 → eşzamanlı çağrı sayısı ölçülür ve
      8'i aşmaz
- [ ] T43 Sınamayı geçemeyen repository **eklenmez**, sebebi yazılır →
      erişimi reddedilen bir repository için `result: failed` satırı
- [ ] T44 Kısmi başarı geri alınmaz → biri düşerken diğerleri kaydedilmiş kalır
- [ ] T45 Zaten kayıtlı olan atlanır; başka bir erişimle kayıtlı olan da
      atlanır ve mevcut kaydın erişimi **değişmez**
- [ ] T46 Yanıt NDJSON akışı → her repository için bir satır, sonda özet satırı;
      ilk satır işlem bitmeden gönderilir
- [ ] T47 Bir repository'nin takılması diğerlerini durdurmaz → repository başına
      kendi süre sınırı; sahte sunucuda biri yavaşlatılır

## Blok 6 — Arayüz

- [ ] T50 Projeler ekranında "Gruptan içe aktar" eylemi → tıklanınca grup
      adresi ve erişim seçimi istenir
- [ ] T51 Önizleme listesi: ad, adres, durum etiketi → zaten kayıtlı olanlar
      **görünür** ve işaretli, seçilemez
- [ ] T52 Seçim varsayılanı: yeni olanlar seçili, arşivli olanlar seçimsiz →
      kullanıcı arşivliyi elle seçebilir
- [ ] T53 Seçim sayacı ve toplu seç/bırak → saf mantık kendi modülünde,
      `npm test` ile
- [ ] T54 İçe aktarma sırasında ilerleme: tamamlanan / toplam → akıştan gelen
      satırlarla güncellenir
- [ ] T55 Sonuç özeti: eklenen, atlanan, başarısız ayrı ayrı; başarısızlar adı
      ve sebebiyle listelenir
- [ ] T56 Boş sonuçlar hata gibi gösterilmez → "grupta repository yok" ve
      "hepsi zaten kayıtlı" ayrı ve sakin mesajlar
- [ ] T57 `ui.md` doğrulaması: açık ve koyu tema, geniş ve dar masaüstü,
      yükleniyor / boş / hata durumları → tarayıcıda ölçülür

## Blok 7 — Kapanış

- [ ] T60 `README` ve `docs/`: kurumsal Bitbucket'tan toplu ekleme anlatılır;
      bulut için geçerli olmadığı ve gerçek sunucuda ölçülmediği yazılır
- [ ] T61 Kapı temiz → `make test` · `npx tsc --noEmit` · `npx eslint .` ·
      `make lint-backend`
- [ ] T62 Spec kabul kriterleri tek tek elle doğrulanır; `spec.md` durumu
      "Uygulandı" olur

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.
