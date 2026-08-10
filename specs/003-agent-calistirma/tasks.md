# Görevler: Projeler, Agent Tanımları ve Agent Çalıştırma

- **Spec no:** 003 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı

---

## Ayarlar (H7)

- [x] T01 `internal/settings/registry.go` — `Definition`, `Kind`, yedi parametrelik kayıt defteri → derlenir
- [x] T02 `internal/settings/service.go` — `Load`/`Int`/`Set`/`Reset`/`All`, önbellek + doğrulama → derlenir
- [x] T03 Testler: aralık dışı reddi, bilinmeyen anahtar, tip uyuşmazlığı, sıfırlama, **her varsayılanın kendi aralığında olduğu** → `make test` yeşil

## Runner çekirdeği

- [x] T10 `internal/runner/runner.go` — `Runner` arayüzü, `Request`/`Result`/`Event`, sentinel hatalar → derlenir
- [x] T11 `internal/runner/config.go` — sağlayıcıdan `opencode.json`, agent'tan `.md` üretimi; yetki → permission eşlemesi → derlenir
- [x] T12 Testler: üç sağlayıcı türü için config, **anahtarın dosyaya düz metin yazılmadığı**, `allow_*` kombinasyonları → `make test` yeşil
- [x] T13 `internal/runner/sandbox/docker.go` — create (başlatmadan) → CopyToContainer → start → wait → rm; kaynak limitleri ayarlardan → derlenir
- [x] T14 Sandbox testi: sahte imajla create→copy→start→rm; **hata yolunda da temizlik** → `docker ps -a` artık bırakmıyor
- [x] T15 `internal/runner/opencode/{client.go,runner.go}` — health bekle, session aç, SSE dinle, mesaj gönder, diff al → derlenir
- [x] T16 `httptest` ile testler: başarılı akış, session hatası, mesaj hatası, SSE kopması, boş diff → `make test` yeşil
- [x] T17 **Uçtan uca duman testi (arayüzsüz):** koddan `reviewer` çalıştır → çıktı + token + maliyet gelir, container ve volume temizlenir

## Veri katmanı

- [x] T20 Migration `000003_calistirma.sql` — `settings`, `projects`, `agents`, `runs`, `run_events` → `make migrate` hatasız
- [x] T21 `internal/agentreg/{builtin/*.md,builtin.go,store.go}` — beş agent `.opencode/agents/`'dan taşınır, gömülür, tohumlanır; CRUD → derlenir
- [x] T22 `internal/projects/{store.go,verify.go}` — CRUD + `git ls-remote` ile erişim doğrulaması (kimlik `GIT_ASKPASS` ile) → derlenir
- [x] T23 `internal/runs/store.go` — kayıt, durum geçişleri, olay yazma, listeleme → derlenir
- [x] T24 Entegrasyon: tohumlama, düzenleme sonrası "değiştirilmiş" işareti, sıfırlama → `make test-integration` yeşil
- [x] T25 Entegrasyon: **agent düzenlenince geçmiş çalıştırmanın kopyası değişmiyor**; çalıştırması olan agent silinemiyor
- [x] T26 Entegrasyon: `run_events` sıralı yazılıyor, geçmiş doğru sırada okunuyor

## Çalıştırma yönetimi

- [x] T30 `internal/events/bus.go` — çok aboneli yayın, abonelik kapatma → derlenir
- [x] T31 Bus testleri: çok abone, kapanan abone, kapalı kanala yazmama, yarış (`-race`) → `make test` yeşil
- [x] T32 `internal/runs/manager.go` — sayaç + `sync.Cond` ile sınır (ayardan okunur), iptal, timeout → derlenir
- [x] T33 `RecoverInterrupted` — açılışta yarım kayıtlar `interrupted` olur → entegrasyon testi
- [x] T34 Manager testleri: `ErrTooManyRuns`, iptal, timeout, **sınır değişince yeniden başlatmadan geçerli oluyor** → `make test` yeşil
- [x] T35 `internal/runs/push.go` — diff'i branch'e gönderme, iki kez gönderme koruması → derlenir

## HTTP

- [x] T40 `httpapi/settings.go` — liste (kayıt defteri + değer + sapma), güncelle, sıfırla → curl ile doğrulanır
- [x] T41 `httpapi/projects.go` — CRUD + oluştururken erişim doğrulaması → curl ile doğrulanır
- [x] T42 `httpapi/agents.go` — CRUD + sıfırla; hazır agent silinemez (409) → curl ile doğrulanır
- [x] T43 `httpapi/runs.go` — başlat (429 sınır), oku, listele, iptal, push → curl ile doğrulanır
- [x] T44 `httpapi/sse.go` — `GET /api/runs/{id}/events`: önce geçmiş, sonra canlı; bağlantı kapanınca abonelik kapanır
- [x] T45 Sızıntı testi: yeni uçların hiçbir yanıtında gizli değer yok → `make test` yeşil
- [x] T46 `main.go` wiring — settings yükle, agent tohumla, `RecoverInterrupted`, `Manager.Shutdown` → açılışta çalışır
- [x] T47 `config.go` + `.env.example` — davranış `RUNNER_*` değişkenleri kaldırılır, ayarlara taşındığı yazılır
- [x] T48 `catalog/syncer.go` — sabit `SyncInterval` yerine her turda ayardan okunan aralık
- [x] T49 `backend/Dockerfile` — `git` eklenir (depo erişim doğrulaması için)

## Arayüz

- [x] T50 `lib/types.ts` + `lib/api.ts` + `lib/sse.ts` — yeni tipler, uçlar, SSE yardımcısı → `npm run typecheck` temiz
- [x] T51 `components/settings/RuntimeSettings.tsx` — **kayıt defterinden kendini çizer**; sapma işareti, sıfırla → ekranda çalışır
- [x] T52 `app/projects/page.tsx` — liste, ekle/düzenle/sil, git erişimi seçimi → ekranda çalışır
- [x] T53 `app/agents/page.tsx` — liste, düzenle, oluştur, sıfırla, yetki anahtarları → ekranda çalışır
- [x] T54 Çalıştırma formu — proje, branch, model (araç desteği uyarısıyla), görev metni → ekranda çalışır
- [x] T55 `app/runs/page.tsx` — geçmiş listesi → ekranda çalışır
- [x] T56 `app/runs/[id]/page.tsx` — canlı olay akışı, durum, çıktı, diff, maliyet, iptal, push → ekranda çalışır
- [x] T57 Üç durum (yükleniyor/hata/boş) tüm yeni ekranlarda → elle doğrulanır

## Doğrulama ve kapanış

- [x] T90 [plan.md](plan.md) doğrulama listesinin 19 adımı yürütüldü — push dahil (bkz. Not 4)
- [x] T91 Container ve volume artığı yok; loglarda gizli değer yok
- [x] T92 `make test`, `make test-integration` yeşil; `make lint` temiz
- [x] T93 `AGENTS.md` güncellenir: ayarlar mekanizması, agent tanımlarının yeri, ortam/veritabanı sınırı
- [x] T94 `spec.md` durumu "Uygulandı" yapılır

---

## Notlar

### Ölçüm 1 — diff kaynağı değişti (T15)

Plan `GET /session/:id/diff` diyordu. Duman testi bunu yakaladı: o uç boşken JSON
dizi `[]` döndürüyor ve kod bunu "değişiklik var" sanıyordu. Daha önemlisi, oturuma
özel diff **yalnızca o oturumun kendi araçlarıyla yaptığı** değişiklikleri izliyor.

Yerine çalışma alanına bakan **`GET /vcs/diff/raw`** kullanılıyor: düzgün unified diff
veriyor, yeni dosyaları da kapsıyor. Her çalıştırma kendi container'ını aldığı için
"çalışma alanı" = "bu çalıştırma" eşitliği geçerli.

Yanında `GET /vcs/status` da okunuyor; dosya bazında `+42 −8` özeti veriyor ve
`Result.Files` olarak arayüze taşınıyor.

### Ölçüm 2 — iptal testinin kurgusu yanlıştı (T17)

İlk hali 20 saniye bekleyip iptal ediyordu; küçük bir depoda çalışma ~9 saniyede
bittiği için iptal edilecek bir şey kalmıyordu. 3 saniyeye çekildi — böylece iptal
**erken yolda** (sandbox kurulumu ve klonlama) test ediliyor ki container'ın yarım
kaldığı ve temizlenmesi gereken durum tam olarak orası.

### Ölçüm 3 — slug üretimi Türkçe harfleri atıyordu (T21)

`agentreg` için yazdığım slug üretimi ASCII olmayan harfleri düşürüyordu:
"Şirket Kod İncelemecisi" → `irket-kod-incelemecisi`. `internal/llm`'de bunun
doğrusu (Türkçe katlama tablosu) zaten vardı.

İki yerde ayrışmasın diye ortak **`internal/slug`** paketine çıkarıldı; `llm` ve
`agentreg` ikisi de onu kullanıyor. Sonuç: `sirket-kod-incelemecisi`.

Bunu bir testin yanlış beklentisi ortaya çıkardı — beklenti düzeltilirken asıl
kusurun kodda olduğu görüldü.

### Doğrulanan tasarım kararı (T17)

Duman testi, veritabanından gelen agent tanımının çalıştığını kanıtlıyor: testte
dosyada tanımlı olmayan `incelemeci` agent'ı kullanıldı. Container başlatılmadan
içine kopyalanan tanım uygulandı, `edit` yetkisi kapalı olduğu için değişiklik
üretilmedi. Spec 003'ün "tam CRUD agent" kararı bu yolla çalışıyor.

### Not 4 — push gerçek bir depoya karşı doğrulandı (2026-08-10)

Uzun süre açık kalan tek madde kapandı. `github.com/kullanici/agentTestProject`
üzerinde, fine-grained token'la:

| Kontrol | Sonuç |
|---|---|
| `coder` agent'ı README'ye tek satır ekledi | ✓ `README.md +1 −0`, $0,001095 |
| Diff yeni branch'e gönderildi | ✓ `agent-coder/coder-5755aa1d` |
| Branch GitHub'da gerçekten oluştu | ✓ `e86251b7` |
| Branch içeriği doğru | ✓ eklenen satır orada |
| **`main` dokunulmadı** | ✓ içerik değişmemiş |
| İkinci gönderme reddedildi | ✓ HTTP 409 `already_pushed` |
| Token loglarda | ✓ yok |
| Token API yanıtlarında | ✓ yok, yalnızca son 4 karakter ipucu |
| Token veritabanında düz metin | ✓ değil (AES-GCM) |
| Artık container / volume | ✓ yok |

Branch adı `agent-coder/<agent>-<8 karakter>` biçiminde üretiliyor ve ana branch'e
asla yazılmıyor — geri alınması en zor adım olduğu için tasarım bu yönde.
