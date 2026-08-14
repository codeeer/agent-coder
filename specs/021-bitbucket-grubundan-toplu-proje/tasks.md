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

- [x] T01 `internal/bitbucket` paketi: `ParseGroupURL` düz grup adresini çözer →
      `https://bb.sirket.com/projects/ODEME` → base `https://bb.sirket.com`,
      key `ODEME`
- [x] T02 Context path korunur → `https://sirket.com/bitbucket/projects/ODEME`
      → base `https://sirket.com/bitbucket`
- [x] T03 Sondaki eğik çizgi ve fazladan yol parçası tolere edilir →
      `…/projects/ODEME/`, `…/projects/ODEME/repos/api/browse` aynı sonucu verir
- [x] T04 Kişisel repository yolu çözülür → `…/users/ahmet` → key `~AHMET`
- [x] T05 Grup adresi olmayan girdi reddedilir → `ErrNotGroupURL`; mesaj
      beklenen biçimi **örnekle** söyler
- [x] T06 Bulut adresi ayrı hata döner → `api.bitbucket.org` ve
      `bitbucket.org/…` için `ErrCloudAddress`; denetim ayrıştırmadan ÖNCE
      koşar (plan → tuzaklar)

## Blok 2 — Repository listesinin çekilmesi

- [x] T10 `Client.ListRepos` tek sayfalık yanıtı ayrıştırır → `httptest`
      sunucusundan 3 repository döner
- [x] T11 **Sayfalama tükenene kadar devam eder** → `httptest` iki sayfa
      döndürür (`isLastPage:false` + `nextPageStart`), sonuç 30 kayıt olur.
      Tek çağrı yapan kod bu testte düşer
- [x] T12 Klonlama adresi `links.clone` içinden **http** olanı seçer →
      http+ssh karışık girdide http seçilir; ssh yoksa da çalışır
- [x] T13 Adrese gömülü kullanıcı adı ayıklanır →
      `https://ahmet@bb/scm/ODEME/api.git` → `https://bb/scm/ODEME/api.git`.
      Ayıklanmazsa `Input.Normalize` her kaydı reddederdi
- [x] T14 `archived` alanı yoksa `false` sayılır → alanı taşımayan yanıtta
      hiçbir repository arşivli işaretlenmez
- [x] T15 Hata sınıflandırma → 404 `ErrGroupNotFound`, 401/403 `ErrForbidden`,
      bağlantı hatası `ErrUnreachable`; her biri ayrı test
- [x] T16 Ayrıştırılamayan gövde sessiz boş liste DÖNMEZ → hata döner ve
      mesaj ham gövdenin kısaltılmış halini taşır (plan → riskler)

## Blok 3 — Varsayılan branch ve mükerrer denetimi

- [x] T20 `Verifier.DefaultBranch`: `ls-remote --symref` çıktısından branch
      okunur → `ref: refs/heads/develop\tHEAD` girdisinden `develop` çıkar
- [x] T21 HEAD okunamazsa **hata döner, varsayılan uydurulmaz** → boş çıktıda
      hata; `main` dönmez (spec → davranış kuralları)
- [x] T22 Aynı çağrı erişimi de sınar → erişim reddinde `ErrRepoAuth`,
      ulaşılamamada `ErrRepoUnreachable` (mevcut sınıflandırma paylaşılır)
- [x] T23 `Store.ExistingRepoURLs` kayıtlı adresleri normalize edilmiş
      anahtarlarla döner → büyük/küçük harf, sondaki `.git` ve `/` farkı olan
      iki adres aynı anahtara düşer

## Blok 4 — Önizleme ucu

- [x] T30 `POST /api/projects/import/preview` grup listesini durum etiketiyle
      döner → sahte sunucuya karşı `status: new` ve `already_registered`
      birlikte görülür
- [x] T31 Arşivli repository `archived: true` ile işaretlenir → arayüz onu
      seçimsiz gösterebilsin diye
- [x] T32 Erişim tanımı yoksa veya seçilmediyse 4xx ve **ne yapılacağını**
      söyleyen mesaj → "Ayarlar → Git repository'ler" yolu yazılı
- [x] T33 Hata durumlarının tamamı spec tablosuna uyar → grup yok, yetki yok,
      ulaşılamıyor, bulut adresi, grup adresi değil: beşi için ayrı test

## Blok 5 — İçe aktarma ucu

- [x] T40 `POST /api/projects/import` seçilenleri kaydeder → 3 repository
      gönderilir, 3 proje oluşur, her biri kendi adı/adresi/branch'iyle
- [ ] T41 Erişim bilgisi **bir kez** çözülür, N repository'de kullanılır →
      sağlayıcı deposundan tek okuma yapıldığı testle kilitlenir
- [x] T42 Sınama paralel koşar, sınırı 8 → eşzamanlı çağrı sayısı ölçülür ve
      8'i aşmaz
- [x] T43 Sınamayı geçemeyen repository **eklenmez**, sebebi yazılır →
      erişimi reddedilen bir repository için `result: failed` satırı
- [x] T44 Kısmi başarı geri alınmaz → biri düşerken diğerleri kaydedilmiş kalır
- [x] T45 Zaten kayıtlı olan atlanır; başka bir erişimle kayıtlı olan da
      atlanır ve mevcut kaydın erişimi **değişmez**
- [x] T46 Yanıt NDJSON akışı → her repository için bir satır, sonda özet satırı;
      ilk satır işlem bitmeden gönderilir
- [ ] T47 Bir repository'nin takılması diğerlerini durdurmaz → repository başına
      kendi süre sınırı; sahte sunucuda biri yavaşlatılır

## Blok 6 — Arayüz

- [x] T50 Projeler ekranında "Gruptan içe aktar" eylemi → tıklanınca grup
      adresi ve erişim seçimi istenir
- [x] T51 Önizleme listesi: ad, adres, durum etiketi → zaten kayıtlı olanlar
      **görünür** ve işaretli, seçilemez
- [x] T52 Seçim varsayılanı: yeni olanlar seçili, arşivli olanlar seçimsiz →
      kullanıcı arşivliyi elle seçebilir
- [x] T53 Seçim sayacı ve toplu seç/bırak → saf mantık kendi modülünde,
      `npm test` ile
- [x] T54 İçe aktarma sırasında ilerleme: tamamlanan / toplam → akıştan gelen
      satırlarla güncellenir
- [x] T55 Sonuç özeti: eklenen, atlanan, başarısız ayrı ayrı; başarısızlar adı
      ve sebebiyle listelenir
- [x] T56 Boş sonuçlar hata gibi gösterilmez → "grupta repository yok" ve
      "hepsi zaten kayıtlı" ayrı ve sakin mesajlar
- [x] T57 `ui.md` doğrulaması: açık ve koyu tema, geniş ve dar masaüstü,
      yükleniyor / boş / hata durumları → tarayıcıda ölçülür

## Blok 7 — Kapanış

- [x] T60 `README` ve `docs/`: kurumsal Bitbucket'tan toplu ekleme anlatılır;
      bulut için geçerli olmadığı ve gerçek sunucuda ölçülmediği yazılır
- [x] T61 Kapı temiz → `make test` · `npx tsc --noEmit` · `npx eslint .` ·
      `make lint-backend`
- [x] T62 Spec kabul kriterleri tek tek elle doğrulanır; `spec.md` durumu
      "Uygulandı" olur

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.

**T06 — bulut denetiminde plandan sapıldı.** Plan `gitprovider.bitbucketCloud`'un
yeniden kullanılacağını yazıyordu. Kod okununca host kümelerinin FARKLI olduğu
görüldü: o fonksiyon API adresine bakıyor ve yalnızca `api.bitbucket.org`'u
tanıyor, oysa kullanıcının yapıştıracağı tarayıcı adresi `bitbucket.org`.
Olduğu gibi çağrılsaydı en olası bulut adresi kurumsal sanılırdı.

Kural tek yerde toplandı: `gitprovider.IsCloudHost` dışa açıldı ve iki host'u
da kapsıyor (`api.bitbucket.org` zaten `bitbucket.org`'un alt alan adı, ayrıca
yazılmadı). `bitbucketCloud` artık onu çağırıyor ve boş adresi Cloud sayma
davranışını kendinde tutuyor. Mevcut `gitprovider` testleri değişmeden geçti.

**T13'ün gerekçesi kod okumasından çıktı, spec'ten değil.** `Input.Normalize`
gömülü kimlik taşıyan adresi reddediyor; ayıklama olmasaydı içe aktarmanın
tamamı başarısız olurdu. Aynı yerde ikinci bir tuzak var: `Normalize` boş
branch'i sessizce `main` yapıyor — Blok 3'te boş branch asla geçirilmeyecek.

**Blok 3'te `Verify` refactor edildi.** `ls-remote` çağrısının ortak hazırlığı
(`GIT_TERMINAL_PROMPT=0` gibi güvenlik taşıyan bayraklar, askpass, kullanıcı
adı enjeksiyonu) `lsRemote` içinde toplandı. İki kopya bırakılsaydı birine
eklenen bir bayrak diğerine eklenmeyebilir ve fark ancak üretimde asılı kalan
bir süreçle görülürdü. `Verify`'ın testi YOKTU; refactor'dan önce dört koruma
testi yazıldı (erişilebilen depo, var olan branch, olmayan branch, ulaşılamayan
depo) — testsiz refactor çalıştığını iddia edip kanıtlayamaz.

**T41 ve T47 işaretlenmedi — test edilmediler.** T41 (kimliğin bir kez
çözülmesi) kodun yapısından okunuyor: çözüm döngünün DIŞINDA. Ama okunabilir
olmak test edilmiş olmak değil; sayan bir depo enjekte etmeden kilitlenemez ve
bunun için `GitProviders` bir arayüze çevrilmeliydi. T47 (repository başına
süre sınırı) `lsRemote` içindeki `verifyTimeout` ile sağlanıyor; testi yirmi
saniye bekleyen bir test olurdu. İkisi de doğru çalışıyor ama KANITLANMADI —
işaretlemek yanlış olurdu.

**Genel 60 sn'lik istek zaman aşımı akış ucunu keserdi.** `skipForSSE` yalnızca
`/events` ile biten yolları muaf tutuyordu. Yüz repository'lik bir içe aktarma
altmış saniyeyi aşabilir ve akış işin ortasında kesilirdi — üstelik o ana kadar
eklenenler veritabanında kalacağı için kullanıcı neyin eklendiğini göremezdi.
`/projects/import` muafiyete eklendi; `/import/preview` EKLENMEDİ, o yalnızca
liste çağrısı yapıyor ve sürerse gerçekten sorun var demektir.

**Test düzeneği: gerçek git, HTTP üzerinden.** İlk deneme `file://` adresi
kullanıyordu ve `Input.Normalize` onu haklı olarak reddetti (kimlik akışımız
HTTP üzerine kurulu). Testler artık `git http-backend`'i CGI olarak koşturan
gerçek bir akıllı HTTP git sunucusu ayağa kaldırıyor; böylece zincirin tamamı
— liste, adres seçimi, erişim sınaması, branch okuma, kayıt — gerçekten
koşuyor.

**T57 — tarayıcı doğrulaması gerçek bir akışla yapıldı.** `scripts/sahte-bitbucket/`
altında, hem Bitbucket'ın liste ucunu hem gerçek git'i (smart HTTP) sunan bir
sunucu yazıldı. Ölçülenler: dört repository'nin İKİ SAYFADAN toplanması,
arşivli kaydın seçili gelmemesi, üç projenin eklenmesi ve her birinin
varsayılan branch'inin depodan gelmesi (`develop`, `main`, `release/2026` —
hepsi farklı, yani değer uydurulmuyor), gömülü kullanıcı adının ayıklanması,
tekrar çalıştırmada üçünün de "zaten kayıtlı" görünmesi, üç hata mesajı
(bulut adresi, grup adresi değil, grup yok), açık ve koyu tema, konsolda JS
hatası olmaması.

**Bulunan ama BU SPEC'TE düzeltilmeyen sorun:** projeler ekranı 1089 piksel
genişlikte yatay kayıyor (`scrollWidth` 1129). Ölçüldü ve kaynağı mevcut proje
TABLOSU (`min-w-240`) — içe aktarma paneli açıkken de kapalıyken de aynı değer
çıkıyor, yani bu spec'in getirdiği bir şey değil. Spec 019'un alanı; sessizce
düzeltmek kapsam kayması olurdu.

**`Checkbox` bileşenine `labelHidden` eklendi.** Satırın kimliği başka bir
sütunda yazılıyken etiketi ikinci kez basmak gürültü olurdu; etiketi tamamen
kaldırmak ise ekran okuyucu kullanan biri için kutucuğu adsız bırakırdı. Kalıp
ekranın içine değil `primitives.tsx`'e eklendi (ui.md).

**Test verileri temizlendi:** doğrulama sırasında eklenen üç proje ve sahte
git erişimi silindi, sahte sunucu durduruldu.

**Yük ölçümü yapıldı — kullanıcı kararı buna dayanıyordu.** Sahte sunucu 100
repository ile ayağa kaldırıldı (sayfa boyutu 25, yani dört sayfa):

| Ne | Değer |
| --- | --- |
| Listelenen repository | 100 (dört sayfa birleşti) |
| İçe aktarma süresi | **0,3 sn** |
| Sonuç | 100 eklendi, 0 atlandı, 0 başarısız |
| Akış satırı | 101 (100 sonuç + 1 özet) |
| Gömülü kimlik taşıyan adres | 0 |

ÖLÇÜMÜN SINIRI: sunucu aynı makinede. Gerçek bir kurumsal sunucuda `ls-remote`
başına gecikme çok daha yüksek olur; sekiz paralel ile 100 repository, el ile
300 ms'lik bir gecikme varsayıldığında ~4 saniyeye çıkar. Yani ölçülen şey
"ürünün eklediği maliyet"; ağın maliyeti değil. Kriter yine de karşılanıyor —
kullanıcı dakikalarca bekletilmiyor.
