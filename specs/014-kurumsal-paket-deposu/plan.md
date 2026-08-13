# Plan: Kurumsal paket deposu

- **Spec:** [spec.md](spec.md)
- **Durum:** Uygulandı

---

## Kapsam ayrımı (işin temeli)

`.npmrc` npm'de kapsamlı okunur: **proje** (cwd), **kullanıcı** (`$HOME`),
global, yerleşik. Bu, motor ile agent'ı ayırmaya yetiyor:

| Dosya | Kimi bağlar |
|---|---|
| `/home/agent/.config/opencode/.npmrc` → `offline=true` | yalnızca motor (kendi dizininde kurar) |
| `/home/agent/.npmrc` → `registry=…` | herkesi (kullanıcı kapsamı) |

Ölçüm: motorun dizininde `npm config get offline` → `true`, `/work` altında
→ `false`.

## Ayardan container'a

```
settings ──runs.Limits(fonksiyon)──> runner.Request.Packages
                                          ├─> BuildConfigFiles → ~/.npmrc (0600)
                                          └─> packageSection() → agent talimatı
```

`Limits` alanları **fonksiyon** taşıyor: ayar değişince yeniden başlatma
gerekmiyor. Precedent `CloneDepth`.

## `~/.npmrc` içeriği

```
registry=https://nexus.sirket.local/repository/npm-group/
//nexus.sirket.local/repository/npm-group/:_auth=<base64 user:token>   # varsa
always-auth=true                                                       # varsa
```

`_auth` **adrese bağlanır**: kimlik yalnızca o host/yol için gönderilir.

## Agent talimatı

Adres tanımlıysa talimatın sonuna kısa bir bölüm eklenir: nereden çektiği,
`--registry` vermemesi, `.npmrc` yazmaması. Adres yoksa **bölüm hiç yazılmaz**
— boş başlık modelin dikkatini harcar.

## Değişen dosyalar

| Dosya | Ne |
|---|---|
| `runner/Dockerfile` | `ENV NPM_CONFIG_OFFLINE` kaldırıldı, motora `.npmrc` |
| `backend/internal/settings/registry.go` | `packages` grubu, `Optional` |
| `backend/internal/settings/service.go` | adres doğrulaması |
| `backend/internal/db/migrations/000014_paket_deposu_kimligi.sql` | `nexus` kimlik türü |
| `backend/internal/runner/config.go` | `buildNPMRC`, `packageSection` |
| `backend/internal/credentials/store.go` | `KindNexus` + doğrulayıcı |
| `backend/internal/runs/manager.go` | `Limits.Packages` — fonksiyon alanı |
| `frontend/src/app/settings/page.tsx` | Paket deposu sekmesi |

## Doğrulama

1. Önce hata yeniden üretilir: mevcut imajda `npm install is-odd` → `ENOTCACHED`
2. Düzeltmeden sonra aynı komut geçer; motorun dizininde `offline` hâlâ `true`
3. `runner/offline_test.sh` geçmeye devam eder
4. Token hiçbir ortam değişkeninde görünmez
5. Gerçek koşu: agent `npm install` çalıştırır ve tamamlanır
