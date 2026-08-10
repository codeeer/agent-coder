# Plan: Betikler

- **Spec:** [spec.md](spec.md) · **Görevler:** [tasks.md](tasks.md)

---

## Neden ucuz

Container'a dosya sokma yolu **zaten var** ve bu özellik onun üzerine bir satır
ekliyor:

```
BuildConfigFiles()                container BAŞLATILMADAN önce
  → opencode.json      ────────►
  → agents/<slug>.md   ────────►  /home/agent/...   (tar ile kopyalanır)
  → scripts/<ad>.sh    ────────►  ← eklenecek tek şey
```

`internal/runner/config.go:35` bu dosyaları her çalıştırmada Go'da üretiyor;
`sandbox/docker.go` container'ı **oluşturup başlatmadan** içine kopyalıyor.
İmaj, entrypoint ve motor istemcisi değişmiyor — Dockerfile'a yalnızca bir
`mkdir` giriyor.

Kodda doğrulanan üç şey:

| Gerçek | Nerede |
|---|---|
| `ConfigFile.Mode` maskelenmeden `tar.Header.Mode`'a gidiyor → `0o755` çalışır | `sandbox/docker.go` |
| `/work` altına dosya konamaz (klonlama hedefi boş olmalı) | `runner/config_test.go` |
| Tar kopyalaması dizin oluşturmuyor → önceden `mkdir` gerekiyor | `runner/Dockerfile:42` |

## Güvenlik: tek bir kapı, ve o kapı zaten açık

Bu özelliğin tamamı şu tek cümleye dayanıyor:

> `AllowBash` açık bir agent, betiği bugün de kendisi yazıp çalıştırabilir.

Doğrulaması:

```
config.go:207        AllowBash kapalı → bash: * deny   (hiçbir komut çalışmaz)
runner.go:250-262    container ortamında GIT_TOKEN + AGENT_CODER_PROVIDER_KEY
```

Buradan iki kural çıkıyor ve ikisi de kodda uygulanacak:

1. **Betikler yalnızca `AllowBash` açıkken kopyalanır.** Kapalıyken dosya
   container'a hiç girmez. (Girse de çalıştırılamazdı; ama orada durması yanlış
   bir izlenim verirdi ve bir sonraki geliştirici "madem duruyor, izin de
   verelim" derdi.)
2. **`BuildPermissions` hiç değişmez.** Bu özelliğin yetki katmanına
   dokunmaması, "yeni yetenek açmıyor" iddiasının tek kanıtı.

Reddedilen alternatif ve neden reddedildiği spec K2'de.

## Betiği agent nasıl öğrenecek

Dosyayı koymak yetmez — agent varlığından haberdar olmalı. Talimat dosyasının
(`buildAgentFile`) sonuna bir blok eklenir:

```markdown
## Kullanabileceğin betikler

Bu betikler önceden yazılmış ve gözden geçirilmiştir. Aşağıdaki işlerden birini
yapman gerekirse komutu kendin kurmak yerine ilgili betiği çalıştır.

- `/home/agent/scripts/upgrade-deps.sh` — Bağımlılıkları güvenli sürümlere yükseltir
```

Bloğun MCP'deki karşılığından farkı: MCP araçları modele **araç** olarak
sunuluyor, betikler ise sadece dosya. Bu yüzden listenin talimatta yazması
zorunlu, isteğe bağlı değil.

Blok yalnızca atanmış betik varken yazılır; boş bir başlık modelin dikkatini
boşa harcar.

## Ad = dosya adı

`Name` doğrudan dosya adına dönüşüyor, o yüzden dar tutulur: `[a-z0-9-]+`.
Sebebi MCP'deki (`mcp/server.go:126`) ile aynı — kullanıcının yazdığı ad ile
sistemin kullandığı yol **aynı** olmalı, sessizce dönüştürülmemeli. Aksi halde
`my script` yazan kullanıcı talimatta `my_script.sh` görür ve neden
tutmadığını anlamaz.

`.sh` uzantısı kullanıcının işi değil, sistem ekler.

## Değişecek dosyalar

**Yeni:**

- `internal/scripts/script.go` — `Script` tipi, doğrulama, `FileName()`/`Path()`
- `internal/scripts/store.go` — CRUD + `ForAgent` + `SetAgentScripts`
- `internal/db/migrations/000011_betikler.sql` — `scripts` + `agent_scripts`
- `internal/httpapi/scripts.go` — CRUD, `internal/paging` ile sayfalı
- `frontend/src/components/settings/ScriptSection.tsx`

**Değişecek:**

| Dosya | Değişiklik |
|---|---|
| `internal/runner/runner.go` | `AgentSpec.Scripts []ScriptSpec` |
| `internal/runner/config.go` | `BuildConfigFiles` betikleri yazar (`0o755`); `buildAgentFile` listeyi ekler |
| `runner/Dockerfile` | `mkdir -p /home/agent/scripts` |
| `internal/runbuild/builder.go` | `scripts` store'u çözer, `AgentSpec`'e aktarır |
| `internal/httpapi/router.go` + `cmd/server` | yeni store ve uçların bağlanması |
| `internal/httpapi/agents.go` | agent'a betik atama |
| `frontend/src/app/settings/page.tsx` | `TABS`'a bir satır |
| `frontend/src/components/agents/*` | betik seçimi (MCP seçiminin aynısı) |

## Doğrulama

1. `make test` — birim testleri, özellikle **`AllowBash` kapalıyken dosya
   üretilmemesi** ve yolun `/work` dışında olması
2. Ayarlar → Betikler → betik ekle; bash yetkili bir agent'a ata
3. Gerçek çalıştırma: agent betiği çağırmalı, çıktısı olay akışında görünmeli
4. Aynı betiği bash yetkisi **kapalı** bir agent'a ata → container'da dosya
   bulunmamalı
5. Betiği değiştir → yeniden çalıştır → yeni içerik geçerli olmalı (imaj
   derlenmeden)
6. `make test-integration`, `make lint`
7. `node scripts/theme-audit.mjs /settings` — iki temada 0 kalan
