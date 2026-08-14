# Tasks: Script klasörleri

- **Spec no:** 022 — [spec.md](spec.md) · [plan.md](plan.md)
- **Tarih:** 2026-08-14

Sıra plandaki gerekçeye uyar: riskli iki parça başta. Biri veritabanının
kendisinde (NULL benzersizliği), diğeri container'da (alt dizin sahipliği) —
ikisi de sessizce bozan cinsten ve ikisi de ancak gerçek Postgres ve gerçek
Docker ile kanıtlanabilir.

---

## Blok 1 — Şema ve benzersizlik

- [ ] T01 `000016_script_klasorleri.sql`: `script_folders`, `scripts.folder_id`
      (ON DELETE SET NULL), `agent_script_folders` → migration uygulanır,
      `make test-integration` şemayı kurar
- [ ] T02 Aynı klasörde aynı ad REDDEDİLİR → iki script aynı klasöre aynı adla
      eklenince ikincisi hata verir
- [ ] T03 Farklı klasörde aynı ad KABUL EDİLİR → iki klasörde `01-baslat`
      birlikte var olabilir
- [ ] T04 **İki klasörsüz script aynı adı ALAMAZ** → `NULLS NOT DISTINCT`
      olmadan bu test geçer ve iki script container'da aynı dosyaya yazılırdı;
      testin varlık sebebi bu
- [ ] T05 Klasör adı benzersiz → aynı adla ikinci klasör reddedilir

## Blok 2 — Klasör deposu ve çözümleme

- [ ] T10 Klasör CRUD → oluştur, listele, güncelle, sil; liste her klasörün
      script sayısını taşır
- [ ] T11 Klasör adı doğrulaması script kuralının AYNISI → geçersiz karakter
      için aynı hata; kural kopyalanmadığı testle kilitlenir
- [ ] T12 `Folder.Path()` ve klasörlü `Script.Path()` → `/home/agent/scripts/
      node-24/01-x.sh`; klasörsüzde bugünkü yol birebir korunur
- [ ] T13 Klasör silinince script'ler KALIR, `folder_id` NULL olur → silme
      sonrası script hâlâ okunabilir
- [ ] T14 `FolderUsage` kaç script ve kaç agent → silme onayında kullanılacak
- [ ] T15 `SetAgentFolders` sil+ekle tek transaction → ikinci çağrı öncekini
      tamamen değiştirir
- [ ] T16 `ForAgent` birleşimi → tekil atama + atanmış klasörlerin TÜM
      script'leri döner
- [ ] T17 `ForAgent` MÜKERRER DÖNMEZ → bir script hem tekil hem klasör
      üzerinden atanmışsa listede bir kez görünür
- [ ] T18 `ForAgent` sırası: önce klasörsüzler, sonra klasör adı, sonra script
      adı → sıra testle kilitlenir (talimat metni buna dayanıyor)
- [ ] T19 **Klasöre sonradan eklenen script atama tazelenmeden görünür** →
      H3'ün kanıtı; `agent_scripts`'e yazan bir tasarım bu testte düşer

## Blok 3 — Container yerleşimi

- [ ] T20 `ConfigFile.IsDir` → dizin girdisi taşıyabilir
- [ ] T21 `copyFiles` dizin başlığı yazar (`tar.TypeDir`, 0o755, uid/gid 10001)
      → tar akışı incelenerek doğrulanır
- [ ] T22 Klasörlü script alt dizine yazılır → `BuildConfigFiles` çıktısında
      hem dizin hem dosya var, sıra doğru (dizin ÖNCE)
- [ ] T23 **Gerçek container'da agent kullanıcısı dizini LİSTELEYEBİLİR** →
      `ls` çalışır. Dosyanın varlığına bakmak yetmez: daha önce yaşanan hata
      tam olarak buydu (Dockerfile'daki `AccessDeniedException` kaydı)
- [ ] T24 Klasörsüz script bugünkü yolunda kalır → mevcut kurulumların
      bozulmadığı testle kilitlenir

## Blok 4 — Talimat metni

- [ ] T30 `scriptSection` klasör başlığı yazar: klasör adı, açıklaması, dizin
      yolu → çıktı metni testle karşılaştırılır
- [ ] T31 Klasördeki script'ler ad sırasıyla listelenir → `02` `01`'den sonra
- [ ] T32 Klasörsüz script'ler ayrı ve bugünkü biçimde listelenir
- [ ] T33 Hiç script yoksa blok HİÇ yazılmaz → boş başlık modelin dikkatini
      boşa harcar
- [ ] T34 **Bash yetkisi kapalıyken klasörlü script de yazılmaz ve
      anlatılmaz** → `scriptsFor` tek kapı; "yeni yetenek açmıyor" iddiasının
      kanıtı

## Blok 5 — Uçlar

- [ ] T40 `GET/POST/PUT/DELETE /api/script-folders` → CRUD çalışır
- [ ] T41 Script uçları `folderId` alır → oluştururken ve güncellerken klasör
      atanır, kaldırılır
- [ ] T42 Agent uçları `scriptFolderIds` alır; `scriptIds` korunur
- [ ] T43 Klasör silme, kullanım sayılarını söyler → yanıt kaç script ve kaç
      agent etkileneceğini taşır
- [ ] T44 Hata durumları spec tablosuna uyar → boş ad, geçersiz karakter,
      çift ad, çift script adı: dördü için ayrı test

## Blok 6 — Arayüz

- [ ] T50 Ayarlar → Script'ler gruplu listelenir: klasörler ve klasörsüzler
      ayrı → boş kurulumda da anlamlı görünür
- [ ] T51 Klasör oluşturma, yeniden adlandırma, silme → silmede kaç script'in
      klasörsüz kalacağı yazılı onay
- [ ] T52 Script formunda klasör seçimi (klasörsüz dahil) → var olan script'in
      klasörü değiştirilebilir
- [ ] T53 Agent formunda klasör + tekil script seçimi → ikisi bir arada
- [ ] T54 Agent kartında atanmış klasörler görünür → tekil script'lerden ayırt
      edilebilir
- [ ] T55 **Bash yetkisi kapalı agent'a klasör atanmışsa uyarı görünür** →
      spec kararı: engellemek yerine söylemek
- [ ] T56 `ui.md` doğrulaması: iki tema, geniş ve dar masaüstü, boş ve dolu
      hâller → tarayıcıda ölçülür

## Blok 7 — Kapanış

- [ ] T60 `README`: kampanya kurma anlatımı (klasör aç, adımları `01-` ile
      adlandır, agent'a klasörü ata, prompt'ta sırayı anlat)
- [ ] T61 Kapı temiz → `make test-integration` · `npx tsc --noEmit` ·
      `npx eslint .` · `make lint-backend`
- [ ] T62 Spec kabul kriterleri elle doğrulanır; `spec.md` durumu "Uygulandı"

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.
