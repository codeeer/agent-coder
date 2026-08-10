# Plan: MCP Desteği

- **Spec:** [spec.md](spec.md) · **Görevler:** [tasks.md](tasks.md)

---

## Neden ucuz

Çalıştırma motoru MCP'yi zaten konuşuyor. Bizim yapmamız gereken tek şey ona
doğru yapılandırmayı vermek — ve o yol **hazır**:

```
BuildConfigFiles()            container BAŞLATILMADAN önce
  → opencode.json üretilir  ─────────────────────────────►  /home/agent/.config/opencode/
     { provider: {...} }                                     (tar ile kopyalanır)
     { mcp: {...} }  ← eklenecek tek şey
```

`backend/internal/runner/config.go:35` her çalıştırmada bu dosyayı Go'da
üretiyor; `sandbox/docker.go:163` container'ı **oluşturup başlatmadan** içine
kopyalıyor. İmaja, entrypoint'e ve motor istemcisine dokunulmuyor.

Aynı dosyada iki desen daha hazır:

| Desen | Nerede | MCP'de karşılığı |
|---|---|---|
| Gizli değer `{env:VAR}` ile referanslanır | `config.go:58` | MCP başlığı da öyle |
| Yetkiler session açılışında kural olarak gider | `config.go:135` | MCP araç kuralları da |

Motorun yetki sözlüğü **rastgele araç adı/pattern** kabul ediyor. Yani
`sentry_*: deny` yazmak, `edit: deny` yazmakla aynı mekanizma — ikinci bir yetki
kaynağı doğmuyor.

## Sessiz başarısızlık — tasarımın en kritik noktası

Ölçüldü (araştırma): MCP sunucusu ayağa kalkmazsa motor **çökmüyor ve
uyarmıyor**. O sunucunun araçlarını modele hiç sunmuyor, agent hiçbir şey
olmamış gibi devam ediyor.

Kullanıcı açısından sonuç: agent "bakamadım" diyor ya da uyduruyor; sebep
görünmüyor. Bu, hata ayıklaması en zor sınıftan bir arıza.

Karşılığı: mesaj gönderilmeden önce motorun MCP durum ucu sorgulanır ve
bağlanamayan sunucu **olay akışına uyarı** olarak düşer. Çalışma başarısız
sayılmaz — araç olmadan da iş bitebilir — ama sessiz kalmaz.

## Veri modeli

```
mcp_servers                          agent_mcp_servers
  id, name, transport, url             agent_id  ──► agents
  secret_enc, hint                     server_id ──► mcp_servers
  created_at, updated_at               (çoka-çok)
```

`gitprovider` kalıbı: çok kayıt, tür başına farklı zorunlu alan, kaydetmeden
önce doğrulama, doğrulanamayan tür için açık bir yol. `llm` kalıbı fazlalık
taşıyordu (slug benzersizliği, tek-varsayılan kısıtı, katalog senkronu).

## Aşama 1 — dokunulan yerler

**Yeni paket `backend/internal/mcp/`** — `server.go` (tip + doğrulama),
`store.go` (CRUD + `Reveal`), `client.go` (resmi Go SDK ile bağlan, `tools/list`),
`validator.go`.

**Runner:**

| Dosya | Değişiklik |
|---|---|
| `runner/runner.go` | `AgentSpec.MCPServers []MCPServerSpec` |
| `runner/config.go` | `BuildConfigFiles` → `"mcp"` bloğu; her sunucuda **açık** `timeout` |
| `runner/config.go` | `BuildPermissions` → atanmamış sunucular için `deny` |
| `opencode/runner.go` | `buildEnv` → sunucu başına gizli değer; hazır olduktan sonra durum kontrolü |
| `opencode/client.go` | MCP durum ucu |
| `runbuild/builder.go` | agent'ın sunucularını çözüp `AgentSpec`'e aktar |

**HTTP + arayüz:** `httpapi/mcpservers.go`, router, `main.go` wiring;
`McpServerSection.tsx` (Ayarlar), agent formunda çoklu seçim.

**Ayar:** `mcp.timeout_seconds` (30, 5–300). Motorun varsayılanı belirsiz
(doküman 5s, kaynak 30s diyor) — bu yüzden her sunucuda açıkça yazılır.

## Riskler

| Risk | Önlem |
|---|---|
| Sessiz başarısızlık | Durum ön kontrolü + görünür uyarı |
| MCP, yetki modelini deler | Atanmayan her sunucu için açık `deny`; araç listesi arayüzde |
| Bağlam şişmesi | Agent başına atama; araç sayısı görünür |
| Asılı sunucu | Sunucu başına `timeout` + çalıştırmanın genel zaman aşımı |
| Anahtar sızıntısı | `{env:}` referansı; sızıntı testi |

## Doğrulama

1. Sunucu ekle — doğrulamadan geçmeli, yanlış anahtar açık hata vermeli
2. `SELECT secret_enc FROM mcp_servers` düz metin içermemeli
3. Agent'a ata → araç çağrısı olay akışında görünmeli
4. Atanmamış agent aynı aracı **kullanamamalı**
5. Sunucuyu boz → sessiz başarı yok, uyarı görünmeli
6. `make test`, `make test-integration`, `make lint`
7. `node scripts/theme-audit.mjs /settings`
