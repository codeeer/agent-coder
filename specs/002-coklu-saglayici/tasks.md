# Görevler: Çoklu LLM ve Git Sağlayıcı Desteği

- **Spec no:** 002 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı (2026-08-09)

---

## LLM sağlayıcı çekirdeği

- [x] T01 `internal/llm/provider.go` — `Type`, `Provider`, `Model` tipleri; `base_url` doğrulaması; slug türetme → derlenir
- [x] T02 `internal/llm/provider_test.go` — slug (Türkçe karakter, boşluk, çakışma), base_url (şema yok, boşluk, sondaki `/`) → `make test` yeşil
- [x] T03 `internal/llm/client.go` — `Client` arayüzü, sentinel hatalar, `NewClient` fabrikası → derlenir
- [x] T04 `internal/llm/openrouter.go` — `internal/openrouter` buradan taşınır, nullable alanlara uyarlanır → mevcut testler geçer
- [x] T05 [P] `internal/llm/litellm.go` — `/model/info`, başarısızsa `/models` yedeği → derlenir
- [x] T06 [P] `internal/llm/openaicompat.go` — `/models`, tüm meta veri bilinmiyor → derlenir
- [x] T07 Adaptör testleri (httptest): dolu/boş `model_info`, `/model/info` 404 → yedek, bozuk JSON, 401, 5xx → `make test` yeşil
- [x] T08 "Bilinmeyen araç desteği false değildir" testi → `make test` yeşil

## Git sağlayıcı çekirdeği

- [x] T10 `internal/gitprovider/provider.go` — tipler, tür başına zorunlu alan doğrulaması → derlenir
- [x] T11 `internal/gitprovider/validator.go` — github (Bearer), bitbucket (Basic), generic (`ErrNotVerifiable`) → derlenir
- [x] T12 Doğrulayıcı testleri (httptest): 200/401/404/5xx, eksik kullanıcı adı, generic → `make test` yeşil

## Veri katmanı

- [x] T20 Migration `000002_coklu_saglayici.sql` — yeni tablolar, `models` değişikliği, 001'den veri taşıma → `make migrate` hatasız
- [x] T21 `internal/llm/store.go` — CRUD, varsayılan yönetimi, `secret` boşsa mevcut anahtar korunur → derlenir
- [x] T22 `internal/gitprovider/store.go` — CRUD → derlenir
- [x] T23 Entegrasyon testi: migration 001 verisini taşır, anahtar **çözülebilir** kalır → `make test-integration` yeşil
- [x] T24 Entegrasyon testi: iki sağlayıcı + aynı isimli model → ayrı satırlar; sağlayıcı silinince modelleri de gider
- [x] T25 Entegrasyon testi: ikinci varsayılan işaretlemek öncekini düşürür

## Katalog

- [x] T30 `internal/catalog/store.go` — `provider_id`, sağlayıcı filtresi, nullable bağlam/araç → derlenir
- [x] T31 `internal/catalog/syncer.go` — sağlayıcı bazlı senkron, kısmi başarı, `provider_sync` → derlenir
- [x] T32 Entegrasyon testi: bir sağlayıcı hata verir, diğerinin modelleri güncellenir

## HTTP

- [x] T40 `httpapi/llmproviders.go` — liste, ekle, güncelle, sil, tek sağlayıcı senkronu → curl ile doğrulanır
- [x] T41 `httpapi/gitproviders.go` — liste, ekle, güncelle, sil → curl ile doğrulanır
- [x] T42 `httpapi/models.go` — `provider` filtresi, yeni alanlar → curl ile doğrulanır
- [x] T43 `httpapi/credentials.go` — yalnızca Jira kalır; openrouter/github kodu kaldırılır
- [x] T44 Sızıntı testi: iki yeni uç grubunun hiçbir yanıtında gizli değer yok → `make test` yeşil
- [x] T45 `cmd/server/main.go` — bootstrap (tablo boşsa `.env`'den OpenRouter sağlayıcısı), wiring → açılışta çalışır

## Arayüz

- [x] T50 `lib/types.ts` + `lib/api.ts` — yeni tipler ve uçlar → `npm run typecheck` temiz
- [x] T51 `components/settings/LLMProviderCard.tsx` + form — tür seçimi, adres, anahtar, varsayılan → ekranda çalışır
- [x] T52 `components/settings/GitProviderCard.tsx` + form — tür başına farklı alanlar → ekranda çalışır
- [x] T53 `app/settings/page.tsx` — üç bölüm: LLM sağlayıcılar, git erişimleri, Jira → ekranda çalışır
- [x] T54 `components/models/ModelTable.tsx` — sağlayıcı sütunu, "—" ve "araç bilinmiyor" gösterimi → ekranda çalışır
- [x] T55 `app/models/page.tsx` — sağlayıcı filtresi, kısmi senkron sonucu → ekranda çalışır

## Doğrulama ve kapanış

- [x] T90 [plan.md](plan.md) doğrulama listesinin 12 adımı elle yürütülür
- [x] T91 `secret_enc` okunabilir değil; loglarda `sk-or`, `ghp_`, `ATATT` yok
- [x] T92 `make test`, `make test-integration` yeşil; `make lint` temiz
- [x] T93 **OpenRouter'a sabitlenmiş kalan dosyalar düzeltilir:** `AGENTS.md`, `.env.example`, `.opencode/opencode.json`, `.opencode/agents/*.md` (beş agent'ın `model:` satırı), `.claude/skills/go-conventions/SKILL.md`, `.claude/agents/go-backend-dev.md`
- [x] T94 `spec.md` durumu "Uygulandı" yapılır

---

## Notlar

### Sapma 1 — fiyat için nullable kullanılmadı

Plan başta `PromptPrice *float64` diyordu ("bilinmiyor" ile "sıfır" ayrı tutulsun).
Kullanıcı, bilinmeyen fiyatın ücretsiz sayılmasının sorun olmadığını söyledi; fiyat
alanları düz `float64` olarak kaldı ve `is_free` bilinmeyende `true` oluyor.

**Bu kolaylığın dışında tutulan alan:** `SupportsTools *bool`. Bilinmeyeni "desteklemiyor"
saymak, `model_info` doldurulmamış bir LiteLLM'deki hiçbir modelin agent olarak
seçilememesi demekti — yani hedef kurum senaryosunda sistem işe yaramazdı. Bu alan
üç durumlu kaldı ve arayüzde "araç bilinmiyor" rozetiyle gösteriliyor.

### Sapma 2 — `credentials.Resolver` silindi, yerine bootstrap geldi

001'de `.env` anahtarı bir "yedek çözümleme" idi (`Resolver`). Sağlayıcılar veritabanı
satırı olunca bu anlamını yitirdi. Yerine `llm.Bootstrap` geldi: tablo **tamamen boşsa**
`.env`'deki anahtardan bir OpenRouter sağlayıcısı oluşturulur.

Yan etkisi bilinçli: kullanıcı sağlayıcıyı silip yeniden başlatırsa geri gelir.
İstemeyen `.env`'deki değişkeni boşaltır. Davranış `AGENTS.md` ve ayarlar ekranında yazılı.

### Sapma 3 — `internal/openrouter` paketi silindi

Plan "taşınır" diyordu; paket tamamen kaldırılıp içeriği `internal/llm/openrouter.go`
olarak yeniden yazıldı. Sebep: `Model` tipi artık ortak (`llm.Model`) ve nullable alanlar
taşıyor; eski paketin tipleri korunamazdı.

### Sapma 4 — agent md'lerinden `model:` satırı kaldırıldı (T93)

`.opencode/agents/*.md` dosyalarında `model: openrouter/...` sabitliydi. Model her zaman
platform tarafından mesaj başına belirlendiği için bu satır hem gereksiz hem de çoklu
sağlayıcı desteğini kıracak nitelikteydi. Kaldırıldı.

### Doğrulanmayan nokta

**LiteLLM adaptörü canlı bir LiteLLM örneğine karşı test edilmedi** — elimizde yok.
`/model/info` yanıt yapısı dokümandan okundu; adaptör savunmacı yazıldı ve
`/model/info` başarısız olursa OpenAI-uyumlu `/models` ucuna düşüyor. Bu yedek yol
sahte bir sunucuyla doğrulandı. Gerçek bir LiteLLM adresine erişim olursa
`listFromModelInfo` yolunun canlı teyidi yapılmalı.
