# Tasks: Bağımlılık önbelleği

- **Spec no:** 027 — [spec.md](spec.md) · [plan.md](plan.md)
- **Tarih:** 2026-08-19

Riskli parçalar başta ve ikisi de "çalışmıyor" cevabını en ucuz veren maddeler:
**sahiplik** (R1) ve **eşzamanlı yazma** (R2). Sahiplik yanlışsa özellik hiç
çalışmaz — agent kendi önbelleğine yazamaz. Eşzamanlı yazma korunmuyorsa
özellik çalışır ama **bozuk artefakt üretir**; bu ikincisi daha tehlikeli,
çünkü sessizdir ve haftalar sonra anlaşılmayan derleme hatası olarak döner.

Blok 1 ve 2 bitmeden Blok 3'e geçilmez: ikisinin sonucu tasarımı değiştirebilir.

---

## Blok 1 — Sahiplik ve bağlama (riskli)

- [x] T01 `runner/Dockerfile`: `/home/agent/.m2/repository` ve
      `/home/agent/.npm/_cacache` oluşturulur, `chown agent:agent` yapılır →
      **ölçüldü:** ikisi de `agent:agent 755`
- [x] T02 `sandbox`: `CacheMount` tipi ve `Spec.Caches` alanı eklenir →
      `cache.go` + `cache_test.go`; `TestCacheMounts_*` üçü de geçiyor.
      Container tarafı `TestCreate_OnbellekMountlariKurulur` ve
      `TestCreate_OnbellekKapaliykenHicMountYok` ile kilitli
- [x] T03 Volume container'dan ÖNCE `EnsureCaches` ile oluşturulur, idempotent →
      `TestEnsureCaches_AyniVolumeIkiKezOlusturulabilir` geçiyor
- [x] T04 **R1 kanıtı:** boş volume bağlıyken sahiplik `agent`, yazma başarılı,
      içerik container'lar arasında kalıyor →
      `TestRunnerImaji_OnbellekDizinleriAgentaAitVeYazilabilir`,
      `TestRunnerImaji_OnbellekKosularArasindaKalir`,
      `TestRunnerImaji_OnbellekSettingsDosyasiniEzmez`.
      **Karşı ölçüm de yapıldı:** imajda OLMAYAN bir yola boş volume bağlanınca
      sahiplik `root:root` çıkıyor ve agent yazamıyor — yani `chown` satırı
      hurafe değil, taşıyıcı
- [x] T04b **R1 uzak daemon'da: ÖLÇÜLDÜ, YEŞİL.** Ayrı bir Docker daemon
      (docker-in-docker, 27.5.1) TCP üzerinden ayağa kaldırıldı, runner imajı
      oraya aktarıldı ve `DOCKER_HOST=tcp://…` ile ölçüldü:
      sahiplik `agent:agent`, yazma başarılı, içerik container'lar arasında
      kaldı. **Bizim Go kodumuz da uzak daemon'a karşı koştu** —
      `EnsureCaches`, `CacheSize`, `ClearCache` ve bütün önbellek testleri geçti.
      **Belirleyici kanıt:** volume, istemci host'unun dosya sisteminde YOK
      (`/var/lib/docker/volumes/…` bulunamadı) — yani planın "backend volume'ün
      içini göremez, bu yüzden boyut/doğrulama yardımcı container'da çalışır"
      öncülü doğrulandı, adlandırılmış volume tercihi de gerekçesini kanıtladı
- [x] T05 Süreçler arası dosya kilidi açılır (`settings.xml`'e dokunulmadan) →
      `MAVEN_OPTS` ile DEĞİL, `mvn` sarmalayıcısıyla (gerekçe T06/T07)
- [x] T06 **Kilit ezilebiliyor mu?** → **EVET, ezilebiliyor.** Geçersiz bir JVM
      bayrağı dedektör olarak kullanıldı: `MAVEN_OPTS` ile verilen bayrak JVM'e
      ULAŞIYOR (mvn düşüyor), ama `export MAVEN_OPTS=-Xmx1g` sonrası mvn geçiyor
      — yani bayrak sessizce kayboluyor. Kullanıcının uyarısı doğruydu
- [x] T07 **Ezilmeye dayanıklı yol: `mvn` sarmalayıcısı** →
      `/opt/maven/bin/mvn.real` (gerçek başlatıcı) + `/opt/maven/bin/mvn`
      (sarmalayıcı). Ölçüldü: agent `export MAVEN_OPTS=-Xmx2g` yaptıktan sonra
      bile Maven'ın argv'si `-Daether.syncContext.named.factory=file-lock`
      taşıyor; `mvn -v` hâlâ çalışıyor (Maven home bozulmadı).
      **Sarmalayıcı `/usr/local/bin`'e KONMADI** — sebebi Notlar'da

## Blok 2 — Eşzamanlı yazma (riskli) · **AÇIK — kullanıcı ortamında ölçülecek**

> Bu blok yapay bir projede ölçülmez: kurumsal ortamı temsil etmez. Gerçek
> Nexus ve gerçek bir proje ile kullanıcının ortamında koşulacak.
> **T60 kapanışı bu blok yeşil olmadan yapılmaz.**

- [ ] T10 **İki koşu aynı anda aynı artefaktı çözer → ikisi de başarıyla biter**
      (spec H4). Ölçüm gerçek Docker ile, soğuk önbellekle
- [ ] T11 **Eşzamanlı koşulardan sonra bozuk artefakt yok** → önbellekteki her
      `*.jar`, yanındaki `*.jar.sha1` ile uyuşur; uyuşmayan sayısı sıfır
- [ ] T12 Ölçüm sonucu `plan.md`'ye "R2 — ölçüm sonucu" başlığıyla yazılır →
      kilit **yeterliyse** öyle, yetmiyorsa hangi ek önlemin gerektiği yazılı
      olur. Sonuç ne olursa olsun yazılır; sessiz geçilmez

## Blok 3 — Ayar ve düşerek devam

- [x] T20 `settings/registry.go`'ya tek `Definition` kaydı (`KindBool`,
      varsayılan `false`) → `KeyDependencyCache` eklendi;
      `TestKayitDefteri_BagimlilikOnbellegiVarsayilanKapali` varsayılanın
      sessizce açığa çevrilmesini engelliyor. HTTP yüzeyi kayıt defterinden
      üretiliyor, ayrı uç gerekmedi
- [x] T21 `runs.Manager`'a `m.dependencyCache()` erişimcisi; erişimci nil ise
      KAPALI → `TestDependencyCache_AyarYoksaKapali` + `…AyarKapaliysaKapali`.
      Kapalıyken hiç mount eklenmediği `TestDependencyCaches_KapaliykenHicBagYok`
      ve `TestCreate_OnbellekKapaliykenHicMountYok` ile iki katmanda kilitli
- [x] T22 Ayar açıkken iki mount eklenir →
      `TestDependencyCaches_AcikkenMavenVeNpmBaglanir`,
      `TestCreate_OnbellekMountlariKurulur`. Ayrıca ayar değişikliğinin yeniden
      başlatma gerektirmediği `…DegisiklikYenidenBaslatmaGerektirmez` ile ölçüldü
- [x] T23 **Volume oluşturulamazsa koşu DURMAZ**, önbelleksiz sürer ve olay
      akışına `warn` düşer → `TestCachesOrNone_HataVarsaOnbelleksizDevamVeUyari`
      (nil dönüyor + `LevelWarn` olayı asıl sebebi taşıyor) ve
      `…HataYoksaOnbellekAynenGecer`. Karar `Run`'dan ayrı bir fonksiyona
      çıkarıldı — sebebi Notlar'da
- [x] T24 [P] **Gerileme:** `go test ./...` → **35 paket, sıfır başarısız**;
      mevcut testlerin hiçbiri değiştirilmedi. `gofmt` ve `go vet` temiz

## Blok 4 — Boyut ve temizleme

- [x] T30 Yardımcı container deseni: runner imajı, `Entrypoint` geçersiz
      kılınır, `NetworkMode: "none"`, iş bitince silinir → `runHelper`
      (`sandbox/cacheadmin.go`); gerçek Docker testleriyle çalıştığı ölçüldü
- [x] T31 Boyut `du -sb` ile bayt cinsinden okunur → `CacheSize`;
      `TestCacheSize_YazilanVeriBoyutaYansir` 1 MB yazıp boyutun arttığını
      ölçüyor. Ayrıştırma sentetik çıktıyla ayrıca test edildi
      (`TestParseDuBytes_*`, dört kenar durum dahil)
- [x] T31b `GET /api/dependency-cache` ucu → `dependencyCacheStatus`;
      ekosistem başına boyut ve `used` bilgisi döner. Runner bakım sağlamıyorsa
      503 (`TestDependencyCacheStatus_BakimYoksa503`). Uç katmanı `sandbox`'ı
      hiç görmüyor — bakım `runner.CacheAdmin` arayüzünden geçiyor
- [x] T32 Hiç kullanılmamış önbellek `-1` döner (boş olan `0`) →
      `TestCacheSize_HicOlusturulmamisOnbellekEksiBirDoner`. "Bilinmiyor" ile
      "boş" ayrımı taşıyıcıda kuruldu; arayüz bundan "henüz kullanılmadı" yazacak
- [x] T33 `ClearCache` ilgili volume'ü boşaltır, boşalan baytı döner ve
      **sahipliği korur** → `TestClearCache_SilerBoyutuBildirirVeSahipligiKorur`
      (üçü tek testte: silmeyen temizleme işe yaramaz, baytı söylemeyen onay
      şeridini besleyemez, sahipliği bozan önbelleği kalıcı kullanılamaz yapar)
- [x] T33b `POST /api/dependency-cache/{id}/clear` ucu → boşalan baytı SAYIYLA
      döner; **iki farklı 409** ayrıldı: koşu sürüyorsa "N çalıştırma sürüyor",
      koşu yokken volume bağlıysa "sahipsiz kalmış çalışma ortamı" + `docker ps`
      ipucu. Ürün container'ı KENDİ ÖLDÜRMÜYOR ve bunu bir test koruyor
      (`TestClearDependencyCache_MesgulHatasiOldurmeOnermez`)
- [x] T43b `POST /api/dependency-cache/{id}/verify` ucu → sayılarla döner
      (`TestVerifyDependencyCache_SayilarlaDoner`); denetlenemeyenler ayrı
      sayılıyor — ne bozuk sayılır ne yok sayılır
- [ ] T34 Süren koşu varken temizleme `409` döner ve **kaç koşunun sürdüğünü**
      söyler → sahte `Active()` ile ölçülür.
      **Ek bulgu (ölçüldü):** Docker, bağlı bir volume'ü silmiyor. `Active()`
      kapısı yarışa açık ve sahipsiz container da volume tutabiliyor; bu yüzden
      `ErrCacheInUse` sentinel'i eklendi
      (`TestClearCache_KullanimdaykenSilinmezVeSebebiSoylenir`). Uç bu sentinel'i
      de 409'a çevirecek — kullanıcıya "bakım başarısız" değil "şu an yapılamaz"
- [x] T35 Bilinmeyen ekosistem kimliği `404` döner →
      `TestClearDependencyCache_BilinmeyenKimlik404`
- [x] T36 Temizleme ve doğrulama **kayda geçer**: `slog.InfoContext` ile
      ekosistem, boşalan bayt / tarama sayıları ve `middleware.GetReqID`.
      "Kim" alanı YOK ve uydurulmadı — üründe henüz kimlik yok (spec 024);
      kimlik gelince buraya bir alan eklenecek

## Blok 5 — Doğrulama

- [x] T40 Maven doğrulaması `*.jar` ↔ `*.jar.sha1` karşılaştırır → bilerek
      bozulmuş bir jar `mismatched` sayılır ve silinir; sağlam olan kalır
      (`TestVerifyCache_BozuguSilerSaglamiVeDenetlenemeyeniBirakir`, gerçek
      Docker). Karar mantığı ayrıca Docker'sız test edildi
      (`TestClassifyVerifyRow_*`)
- [x] T41 **Özeti bulunmayan VEYA okunamayan artefakt SİLİNMEZ**,
      `unverifiable` sayılır → altı bozuk biçim tek tek ölçüldü
      (`TestClassifyVerifyRow_OkunamayanOzetSilmeyeYolAcmaz`), ayrıca gerçek
      Docker'da kırpılmış `.sha1` hiçbir şeyi sildirmedi
      (`TestVerifyCache_KirpilmisOzetDosyasiSilmeyeYolAcmaz`).
      `.sha1`'in iki biçimi de kabul ediliyor (spec H5 son kriteri)
- [x] T42 npm doğrulaması `npm cache verify` ile çalışır → `VerifyNPMCache`;
      çıktı biçimi gerçek imajda ölçüldü, ayrıştırma sentetik çıktıyla test
      edildi (`TestParseNPMVerify_*`).
      **SEMANTİK UYUŞMAZLIK KAYDEDİLDİ:** npm, bozulmayı referanssız içeriğin
      toplanmasından AYIRMIYOR — ikisini de "garbage-collected" sayıyor. Bu
      yüzden npm için `Mismatched` her zaman 0 ve `Removed` "bozuktu" demek
      değil. Maven'da `Removed` "özetiyle uyuşmadı" demek; ikisi aynı iddia
      DEĞİL. **Arayüz npm için "bozuk" dememeli** (T50'de dikkat edilecek)
- [x] T43 Süren koşu varken doğrulama `409` döner → `verifyDependencyCache`
      kapısı; mesaj sebebi söylüyor ("inmekte olan artefaktı bozuk saymamak
      için"). **İkinci kat koruma da yerinde:** özeti okunamayan artefakt
      zaten hiç silinmiyor (T41). `Active() > 0` dalı T34 ile aynı seansta
      kullanıcı ortamında ölçülecek
- [ ] T44 Doğrulamanın sildiği artefakt sonraki koşuda **yeniden iner** ve koşu
      başarıyla biter (spec H5 ikinci kriteri)

## Blok 6 — Arayüz

- [x] T50 [P] `DependencyCacheStatus.tsx`: Maven ve npm **iki ayrı satır**, her
      biri kendi boyutu, "Temizle" ve "Doğrula" eylemiyle → tarayıcıda ölçüldü.
      Kullanılmamış önbellekte iki düğme de DEVRE DIŞI (olmayan bir şey
      temizlenemez). **npm için "bozuk" denmiyor** (T42 semantik uyuşmazlığı):
      npm cümlesi "kayıt denetlendi/temizlendi", Maven cümlesi "uyuşmadı ve
      silindi" diyor
- [x] T51 [P] Hiç kullanılmamış önbellek "henüz kullanılmadı" yazar → gerçek
      veriyle ölçüldü: Maven'a 6 MB yazıldı, Maven "5,8 MB" gösterdi, npm
      "henüz kullanılmadı" demeye devam etti. Silme sonrası boyut kendiliğinden
      güncellendi (5,3 MB)
- [x] T52 [P] Temizleme onayı sayıyla yazıyor → tarayıcıda ölçülen metin:
      "Maven önbelleği temizlensin mi? 5,8 MB yer boşalacak. Sonraki koşular bu
      bağımlılıkları yeniden indirecek ve ilk koşu belirgin biçimde yavaş olacak."
      `ConfirmStrip` kullanıldı; projede `window.confirm` yok
- [x] T53 [P] Açık ve koyu tema AYRI AYRI ölçüldü — göz kararıyla değil,
      hesaplanmış kontrast oranıyla:
      | ölçüm | açık | koyu | eşik |
      | --- | --- | --- | --- |
      | "henüz kullanılmadı" | 5,16 | 5,23 | 4,5 |
      | boyut | 8,71 | 8,78 | 4,5 |
      | tarama sonucu | 9,52 | 9,53 | 4,5 |
      | düğme metni | 18,92 | 15,00 | 4,5 |
      | düğme kenarlığı | 3,46 | 3,47 | 3,0 |
      1440 ve 1280 genişlikte yatay taşma yok, konsol hatası yok.
      `npx tsc --noEmit` ve `npx eslint .` temiz
- [x] T54 **Uçtan uca doğrulama tarayıcıdan ölçüldü (H5 kanıtı).** Önbelleğe üç
      artefakt kondu: özeti doğru, özeti yanlış, özeti YOK. Arayüzden "Doğrula"
      dendi; sonuç cümlesi *"2 artefakt denetlendi, 1 tanesi uyuşmadı ve silindi,
      1 tanesinin özeti okunamadı (dokunulmadı)"*. Volume'de kalanlar ölçüldü:
      sağlam **kaldı**, özetsiz **kaldı**, yalnızca bozuk silindi

## Blok 7 — Kapanış

- [ ] T60 Spec kabul kriterlerinin **tamamı** elle doğrulanır → karşılanmayan
      varsa burada açıkça yazılır, "tamam" denmez.
      **ÖNKOŞUL: T04b ve Blok 2 (T10–T12) yeşil olmadan bu görev kapatılamaz.**
      İkisi de kullanıcının ortamında ölçülecek; ölçülmeden "Uygulandı" denmez
- [x] T61 `plans/03`'e kapanış bölümü yazıldı → durum **Çözüldü**. İçerik:
      seçilen yol, zehirlenme itirazına verilen cevap (ortadan kalkmadı,
      **bilerek kabul edildi** + iki hafifletici), gömme yolunun neden
      gereksizleştiği (önerilen ölçüm GÖMME kararı içindi, volume sürüm
      tahmini gerektirmiyor), ölçülenler ve **kapanışta açık kalan iki ölçüm**
- [ ] T62 `spec.md` durumu "Uygulandı" yapılır; yeni komut veya konvansiyon
      çıktıysa `AGENTS.md` güncellenir

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır. Sessiz sapma yok — plan
yanlışsa plan güncellenir.

- Blok 2'nin ölçüm sonucu (T12) buraya değil `plan.md`'ye yazılır; orası kararın
  yaşadığı yer.

### Sapma 1 — sarmalayıcı `/usr/local/bin`'e değil `/opt/maven/bin`'e kondu

Plan `/usr/local/bin/mvn` symlink'ini aday göstermişti. **Ölçüldü: çalışmazdı.**
İmajın `PATH`'i `/opt/java/25/bin:/opt/maven/bin:/usr/local/sbin:/usr/local/bin:…`
sırasında; `which mvn` → `/opt/maven/bin/mvn`. Sarmalayıcı `/usr/local/bin`'e
konsaydı hiç çağrılmaz, kilit hiç açılmaz ve bunu yalnızca bozuk bir jar
haber verirdi. Gerçek başlatıcı `mvn.real` olarak yeniden adlandırıldı.

### Sapma 2 — birim testler "sahte Docker istemcisi" ile değil

Plan "Docker'sız — `sandbox` istemcisi arayüz üzerinden sahte" diyordu. Paketin
mevcut testleri öyle çalışmıyor: gerçek Docker'a karşı koşup Docker yoksa
`t.Skipf` ile atlıyorlar (`docker_test.go`). Pakete yabancı bir sahteleme
kalıbı sokmak yerine mevcut desen izlendi; saf mantık (`cacheMounts`) ayrı bir
paket içi testte Docker'sız ölçülüyor.

### Sapma 3 — sahiplik testleri koşu container'ıyla yapılamadı

Runner container'ı yapılandırma dosyaları olmadan hemen çıkıyor, `exec`
tutunamıyor. Sorulan şey zaten sandbox'ın değil **imajın** davranışı; testler
`sleep` entrypoint'li ağsız bir yardımcı container kullanıyor — bu, planın
Blok 4'te tarif ettiği yardımcı container deseninin erken uygulaması oldu.

### Sapma 4 — düşerek devam kararı `runs.Manager`'da değil, `opencode`'da

Plan "karar noktası tek yerde — `runs.Manager`" diyordu. **Uygulanamadı ve
uygulanmamalıydı:** `runs` paketi `sandbox`'ı import edemez (go-conventions →
"`internal/runner/` sızmaz"). Volume adları ve container içi yollar çalışma
ortamının iç bilgisi; `runs` katmanına giden tek şey `Request.DependencyCache`
boolean'ı oldu — kurumsal sertifikanın içerik olarak taşınmasıyla aynı kalıp.

Kararın kendisi `opencode/cache.go` içinde **ayrı bir fonksiyona** (`cachesOrNone`)
çıkarıldı. `Run` içine gömülü kalsaydı sınanması gerçek bir Docker hatası
üretmeyi gerektirir ve pratikte hiç sınanmazdı; şimdi spec'in hata kuralı
(düşme + duyur) doğrudan test ediliyor.

### Kullanıcı ortamında ölçülecek iki madde — AÇIK BIRAKILDI

**T04b — KAPANDI.** Erişilebilir bir uzak daemon yoktu; ayrı bir daemon
docker-in-docker ile ayağa kaldırılıp TCP üzerinden ölçüldü. Sonuç yeşil ve
volume'ün istemci host'unda bulunmaması, uzaklığın gerçek olduğunun kanıtı.

**Blok 2 (T10–T12) ve T34'ün `Active() > 0` dalı — eşzamanlı yazma.** Yapay bir Maven projesiyle ölçmek
kurumsal ortamı temsil etmez (gerçek Nexus, gerçek bağımlılık ağacı, gerçek
eşzamanlılık). Kullanıcı kendi ortamında ölçüp sonucu verecek.

Ölçülen ve ölçülmeyen ayrımı burada nettir: sarmalayıcının kilit bayrağını
Maven'a **geçirdiği** ölçüldü (T07); o kilidin eşzamanlı iki koşuda **yeterli
olduğu** ölçülmedi. İkisi ayrı iddia. **T60 ikisi de yeşil olmadan
kapatılmaz.**
