# Tasks: Script klasörleri

- **Spec no:** 022 — [spec.md](spec.md) · [plan.md](plan.md)
- **Tarih:** 2026-08-14

Sıra plandaki gerekçeye uyar: riskli iki parça başta. Biri veritabanının
kendisinde (NULL benzersizliği), diğeri container'da (alt dizin sahipliği) —
ikisi de sessizce bozan cinsten ve ikisi de ancak gerçek Postgres ve gerçek
Docker ile kanıtlanabilir.

---

## Blok 1 — Şema ve benzersizlik

- [x] T01 `000016_script_klasorleri.sql`: `script_folders`, `scripts.folder_id`
      (ON DELETE SET NULL), `agent_script_folders` → migration uygulanır,
      `make test-integration` şemayı kurar
- [x] T02 Aynı klasörde aynı ad REDDEDİLİR → iki script aynı klasöre aynı adla
      eklenince ikincisi hata verir
- [x] T03 Farklı klasörde aynı ad KABUL EDİLİR → iki klasörde `01-baslat`
      birlikte var olabilir
- [x] T04 **İki klasörsüz script aynı adı ALAMAZ** → `NULLS NOT DISTINCT`
      olmadan bu test geçer ve iki script container'da aynı dosyaya yazılırdı;
      testin varlık sebebi bu
- [x] T05 Klasör adı benzersiz → aynı adla ikinci klasör reddedilir

## Blok 2 — Klasör deposu ve çözümleme

- [x] T10 Klasör CRUD → oluştur, listele, güncelle, sil; liste her klasörün
      script sayısını taşır
- [x] T11 Klasör adı doğrulaması script kuralının AYNISI → geçersiz karakter
      için aynı hata; kural kopyalanmadığı testle kilitlenir
- [x] T12 `Folder.Path()` ve klasörlü `Script.Path()` → `/home/agent/scripts/
      node-24/01-x.sh`; klasörsüzde bugünkü yol birebir korunur
- [x] T13 Klasör silinince script'ler KALIR, `folder_id` NULL olur → silme
      sonrası script hâlâ okunabilir
- [x] T14 `FolderUsage` kaç script ve kaç agent → silme onayında kullanılacak
- [x] T15 `SetAgentFolders` sil+ekle tek transaction → ikinci çağrı öncekini
      tamamen değiştirir
- [x] T16 `ForAgent` birleşimi → tekil atama + atanmış klasörlerin TÜM
      script'leri döner
- [x] T17 `ForAgent` MÜKERRER DÖNMEZ → bir script hem tekil hem klasör
      üzerinden atanmışsa listede bir kez görünür
- [x] T18 `ForAgent` sırası: önce klasörsüzler, sonra klasör adı, sonra script
      adı → sıra testle kilitlenir (talimat metni buna dayanıyor)
- [x] T19 **Klasöre sonradan eklenen script atama tazelenmeden görünür** →
      H3'ün kanıtı; `agent_scripts`'e yazan bir tasarım bu testte düşer

## Blok 3 — Container yerleşimi

- [x] T20 `ConfigFile.IsDir` → dizin girdisi taşıyabilir
- [x] T21 `copyFiles` dizin başlığı yazar (`tar.TypeDir`, 0o755, uid/gid 10001)
      → tar akışı incelenerek doğrulanır
- [x] T22 Klasörlü script alt dizine yazılır → `BuildConfigFiles` çıktısında
      hem dizin hem dosya var, sıra doğru (dizin ÖNCE)
- [x] T23 **Gerçek container'da agent kullanıcısı dizini LİSTELEYEBİLİR** →
      `ls` çalışır. Dosyanın varlığına bakmak yetmez: daha önce yaşanan hata
      tam olarak buydu (Dockerfile'daki `AccessDeniedException` kaydı)
- [x] T24 Klasörsüz script bugünkü yolunda kalır → mevcut kurulumların
      bozulmadığı testle kilitlenir

## Blok 3b — Proje dizini (H6)

- [x] T25 `runner.ProjectDir` sabiti ve container'a `PROJECT_DIR` değişkeni →
      çalıştırmada ortamda görünür
- [x] T26 `entrypoint.sh` `WORKDIR`'ü `PROJECT_DIR`'den okur, varsayılanı korur
      → değişken verilmezse eski davranış aynen sürer
- [x] T27 **Gerçek container'da `$PROJECT_DIR` klonlanan projeyi gösterir** →
      script oradan bir dosyayı okuyabilir
- [x] T28 Script çalışma dizininden BAĞIMSIZ çalışır → başka bir dizinden
      çağrılan script yine doğru dosyayı bulur

## Blok 4 — Talimat metni

- [x] T30 `scriptSection` klasör başlığı yazar: klasör adı, açıklaması, dizin
      yolu → çıktı metni testle karşılaştırılır
- [x] T31 Klasördeki script'ler ad sırasıyla listelenir → `02` `01`'den sonra
- [x] T32 Klasörsüz script'ler ayrı ve bugünkü biçimde listelenir
- [x] T33 Hiç script yoksa blok HİÇ yazılmaz → boş başlık modelin dikkatini
      boşa harcar
- [x] T35 Talimatta `$PROJECT_DIR` ve "kalıcı değişiklikler orada olmalı"
      kuralı yazar → metin testle karşılaştırılır
- [x] T34 **Bash yetkisi kapalıyken klasörlü script de yazılmaz ve
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
- [ ] T57 Script düzenleme ekranında `$PROJECT_DIR` notu görünür → kullanıcı
      kaynağı okumadan öğrenir
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

**`Update` imzası değişti: `clearFolder bool` eklendi.** Tek bir `*uuid.UUID`
ile "klasöre dokunma" ile "klasörden çıkar" ayırt edilemiyordu — ikisi de nil.
`projects.Store.Update`'teki `clearProvider` kalıbının aynısı kullanıldı; yeni
bir kalıp icat edilmedi.

**`NULLS NOT DISTINCT` gerçekten uygulandı, göze güvenilmedi.** Testler geçtikten
sonra indeks tanımı veritabanından okundu:
`CREATE UNIQUE INDEX scripts_klasor_ad ON public.scripts USING btree (folder_id, name) NULLS NOT DISTINCT`

**Dizin girdisi ÖLÇÜLDÜ, sonra yine de eklendi.** Planda "dizin başlığı
yazılmazsa agent dizini listeleyemeyebilir" yazıyordu. Gerçek runner imajında
sınandı: dizin başlığı OLMADAN da çalışıyor — Docker eksik ara dizini
`root:root 0755` açıyor ve agent onu listeleyip içindeki betiği
çalıştırabiliyor. Yani planın gerekçesi ölçümle çürüdü.

Girdi yine de eklendi ve gerekçesi değiştirildi: o davranış Docker'ın
belgelenmemiş bir ayrıntısı, izinleri daralırsa agent kilitlenir ve bu ürün o
hatayı bir kez yaşadı. On satır karşılığında belgelenmemiş davranışa olan
bağımlılık kalktı. Ölçümle sonrası: dizin artık `10001:10001`, root'un değil.

**Gerçek container'da ölçülenler (T23, T27, T28):**

| Ne | Sonuç |
| --- | --- |
| Dizin sahipliği (girdi eklendikten sonra) | `drwxr-xr-x 10001 10001` |
| `/tmp`'den çağrılan betik `$PROJECT_DIR`'i buluyor mu | evet — `package.json` okundu |
| `PROJECT_DIR` verilmişse entrypoint onu kullanıyor mu | evet (`/ozel`) |
| Verilmezse varsayılan | `/work` — eski davranış korunuyor |

**`make lint-shell` KIRIK ama bu değişiklikten değil.** SC1091, `entrypoint.sh`
satır 15'te `java-truststore.sh`'ı izleyememekten geliyor ve spec 018'den beri
var. Bu spec'te düzeltilmedi — ayrı iş.
