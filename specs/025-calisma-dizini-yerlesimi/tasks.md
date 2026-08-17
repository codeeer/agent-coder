# Görevler: Çalışma dizini yerleşimi

- **Spec no:** 025 — [spec.md](spec.md) · [plan.md](plan.md)
- **Tarih:** 2026-08-17

Her görev tek oturumda biter ve gözlenebilir bir sonucu vardır.
`[P]` işaretli görevler paralel yürütülebilir.

---

## Commit (a) — Yol türetme + tek kaynak

- [ ] **T1** `runner.go`'daki `const ProjectDir = "/work"` ikiye ayrılır:
      `const WorkRoot` ve `type WorkdirLayout` + `LayoutRoot`/`LayoutRepo`
      sabitleri → `go build ./...` geçer, `ProjectDir` henüz kullanılmadığı
      için derleme kırılmaz
- [ ] **T2** `func ProjectDir(layout, repoURL) string` yazılır: son segment,
      sondaki `/` ve `.git` atılır; kısa SSH biçimi ayrı dalda; güvensiz ad
      `WorkRoot`'a düşer → fonksiyon derlenir
- [ ] **T3** `workdir_test.go` tablo testi → `go test ./internal/runner/ -run
      ProjectDir` yeşil. Vakalar: https+`.git`, https `.git`'siz, sonu `/`,
      kısa SSH, `http`, boş adres, `..` üreten adres, ayraç üreten adres,
      adı boş çıkan adres — güvensiz olanların hepsi `/work` döner
- [ ] **T4** `runner.Request`'e `ProjectDir string` alanı eklenir →
      `go build ./...` geçer
- [ ] **T5** `buildEnv` sabit yerine `req.ProjectDir` okur; boşsa `WorkRoot`
      → `go test ./internal/runner/opencode/` yeşil (mevcut testler
      değişmeden geçer)
- [ ] **T6** `buildEnv` için iki test: alan boşken `PROJECT_DIR=/work`,
      doluyken verilen değer → `go test ./internal/runner/opencode/` yeşil
- [ ] **T7** `BuildConfigFiles` ve `scriptSection` proje kökünü parametre
      alır; ~10 test çağrısı `WorkRoot` geçecek şekilde güncellenir →
      `go test ./internal/runner/` yeşil, talimat metni **birebir aynı**
- [ ] **T8** `runs.Limits`'e `WorkdirLayout func() runner.WorkdirLayout`
      eklenir; `execute` içinde bir kez hesaplanıp `Request.ProjectDir`'e
      yazılır; closure nil ise `LayoutRoot` → `go test ./...` yeşil
- [ ] **T9** Entegrasyon testi: ayar `repo` iken `execute`'un ürettiği
      `Request`'te env değeri ile talimat metnindeki yol **aynı** →
      `go test ./internal/runs/` yeşil
- [ ] **T10** `go vet ./...` ve `go test ./...` temiz → commit (a)

## Commit (b) — Ayar + arayüz

- [ ] **T11** `settings/registry.go`'ya yeni `Key` + `KindBool` tanımı,
      varsayılan kapalı, `Help` metni iki yerleşimi de anlatır →
      `go test ./internal/settings/` yeşil
- [ ] **T12** `main.go`'da closure ayara bağlanır → sunucu açılır, ayar
      `GET /api/settings` yanıtında görünür
- [ ] **T13** [P] `types.ts`'teki `PROJECT_DIR` sabiti `WORK_ROOT` +
      `projectDirLabel()` ile değiştirilir → `npx tsc --noEmit` temiz
- [ ] **T14** [P] `ScriptSection.tsx`'teki proje kökü metni ayardan türer →
      ayar kapalıyken bugünkü cümle aynen görünür, açıkken `/work/<repo-adı>`
      kalıbı görünür
- [ ] **T15** [P] `diagrams.tsx`'teki `/work` kutusu yerleşimden bağımsız
      ifadeye çevrilir → belge diyagramı iki yerleşimde de doğru okunur
- [ ] **T16** Arayüz elle doğrulama: Ayarlar ekranında seçenek mevcut kart
      üslubunda görünür, varsayılan kapalı → ekran görüntüsü
- [ ] **T17** `npx tsc --noEmit` ve `npx eslint .` temiz → commit (b)

## Commit (c) — Belge

- [ ] **T18** `docs/` içinde çalışma dizini / `$PROJECT_DIR` anlatılan yere
      yeni seçenek ve "betikler `$PROJECT_DIR` kullandığı sürece her iki
      yerleşimde çalışır" notu eklenir → belge okunduğunda seçeneğin ne
      yaptığı anlaşılır
- [ ] **T19** İlgili mevcut spec'in karar geçmişine tarihli madde eklenir
      (hangisi olduğu `specs/` taranarak bulunur; yoksa docs notu yeter) →
      karar izlenebilir
- [ ] **T20** commit (c)

## Doğrulama (commit'lerden sonra)

- [ ] **T21** `bash -n runner/entrypoint.sh` → söz dizimi temiz
- [ ] **T22** Yerel bare repo ile yalnızca klonlama bloğu denenir
      (`PROJECT_DIR=/work/x`) → klon `/work/x` altına açılır; gerekiyorsa
      tek satır `mkdir -p` eklenir ve commit (a)'ya iliştirilir
- [ ] **T23** Ayar kapalı bir çalıştırma → `PROJECT_DIR=/work`, betik metni
      bugünküyle aynı
- [ ] **T24** Ayar açık bir çalıştırma → `PROJECT_DIR=/work/<ad>`, talimat
      metninde aynı yol
- [ ] **T25** PR açıklaması yazılır: motivasyon (dış runbook/Jenkins uyumu,
      repo adını varsayan script'ler), varsayılanın değişmediği, nasıl
      açıldığı → upstream'e yapıştırılabilir kısa metin

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.

- T7'nin alternatifi `BuildConfigFiles` için girdi struct'ıydı; bu
  değişikliğe göre büyük olduğu için elendi (bkz. plan → Riskler). Test
  çağrıları büyürse yeniden değerlendirilmeli.
- T22 gerçek bir Docker ortamı gerektiriyor. Ortam yoksa görev
  **atlanmaz**, engellendiği açıkça bildirilir — `entrypoint.sh`'ın
  değişmediği varsayımı yalnızca bu adımda doğrulanıyor.

### Uygulama sırasında ortaya çıkanlar (2026-08-17)

- **T9 planlandığından farklı yazıldı.** Plan `execute` üzerinden bir
  entegrasyon testi öngörüyordu; o yol veritabanı gerektiriyor. Bunun yerine
  asıl iddia iki ayrı yerde sınandı: `runs` paketinde ayar → yerleşim
  çözümü, `opencode` paketinde ise env ile talimat metninin **aynı istekten
  üretilip karşılaştırılması**. İkincisi ayrışmayı doğrudan yakalıyor;
  `execute` testi yalnızca bağlantıyı görürdü.
- **Mevcut bir test eski sabiti kullanıyordu ve sessizce derleniyordu.**
  `require.Contains(t, talimat, ProjectDir)` — `Contains` `interface{}`
  aldığı için sabit fonksiyona dönüşünce de derlendi, yalnızca çalışma
  anında düştü. Yerine `WorkRoot` kondu.
- **Ayar yardım metnindeki backtick'ler kaldırıldı.** Tarayıcıda görüldü:
  arayüz markdown işlemiyor ve Çalıştırma grubundaki diğer ayarlar düz metin
  kullanıyor; işaretler kullanıcıya ham görünüyordu.
- **İnceleme sonrası düzeltilenler (bkz. commit `fix(runner)`):**
  1. Arayüz ayarı `=== "true"` ile okuyordu; backend `strconv.ParseBool`
     kullandığı için `"1"` ve `"t"` de geçerli "açık". Aynı klasörde tam bu
     tuzağı belgeleyen `truthy` yardımcısı vardı ve kopyalanmamıştı — ortak
     bir yere taşındı. `"1"` senaryosu tarayıcıda doğrulandı.
  2. Ayar yüklenirken metin varsayılana düşüp sonra sessizce değişiyordu;
     artık okunana kadar yol hiç yazılmıyor. Yazdığım "önbellekten geliyor"
     yorumu da yanlıştı: betik sekmesine doğrudan girildiğinde sorgu soğuk
     başlıyor.
  3. `repoAdi` iki noktayı koşulsuz kesiyordu — `.../pro:je.git` adresi
     `je` klasörüne inerdi. Kod kendi yorumuyla uyumlu hale getirildi.
  4. Sorgu ve parça `.git` soyulmasını engelliyordu (`proje.git?ref=main`).
  5. 255 baytı aşan ad çalıştırmayı düşürüyordu; artık köke düşülüyor —
     aynı depo varsayılan yerleşimde klonlanabildiği için ayarı açmak
     çalışan bir kurulumu bozmamalı (H2).
  6. `TestProjectDir_KokunAltindaKalir` uygulamanın kendi koruma ifadesini
     tekrar ediyordu, yani koruma kaldırılmadıkça düşemezdi. İddia üretilen
     **adın kendisi** üzerinden bağımsız kuruldu.
- **T23/T24 kısmen gözlemle, kısmen dolaylı doğrulandı.** Gerçek bir
  çalıştırmada container'ın env'i `PROJECT_DIR=/work/Hello-World` olarak
  **doğrudan görüldü** (`docker inspect`) — ayar → servis → closure →
  `Request` → container zinciri uçtan uca çalışıyor. Klonun o dizine indiği
  ise canlı container'da gözlenemedi: model sağlayıcısı 500 döndürdüğü için
  çalıştırma düştü ve container hemen silindi. Klonlama bunun yerine aynı
  imajda gerçek depo adresiyle ayrıca sınandı (T22) ve `/work/Hello-World`
  altına indiği görüldü.
