# Plan: Node sürümlü runner imajları

- **Spec:** [spec.md](spec.md)
- **Durum:** Uygulandı

---

## Tek kaynak

`backend/internal/runner/node-versions.txt` — yorum satırları `#`, her satırda
bir sürüm.

**Neden `runner/` altında değil:** backend imajının derleme bağlamı `./backend`.
`runner/` altındaki bir dosya `go:embed` ile görülemezdi.

Üç tüketici:

| Tüketici | Nasıl okur |
|---|---|
| Backend | `//go:embed node-versions.txt` → `runner.NodeVersions()` |
| CI | `hazirlik` işi dosyayı JSON'a çevirir, `fromJSON` ile matris kurar |
| Makefile | `$(shell grep -vE '^[[:space:]]*(\#\|$$)' …)` — bkz. [tasks.md → Ölçüm 4](tasks.md) |

## İmaj adı üretimi

`runner.ImageFor(base, nodeVersion)`: taban etiketi atar, `node-<sürüm>`
ekler. Registry adresindeki port iki noktası (`ghcr.io:5000/x:tag`) etiketle
karışmasın diye son `/`'tan sonrasına bakılır.

```
ghcr.io/x/agent-coder-runner:latest + "24.13.0"
  → ghcr.io/x/agent-coder-runner:node-24.13.0
```

## Sürümün akışı

```
StartRunForm (seçim)
   └─> POST /api/runs {nodeVersion}
         └─> runbuild.resolveNodeVersion(istek, projeVarsayilani)
               └─> runner.Request.NodeVersion
                     └─> ImageFor(base, sürüm) → EnsureImage → Create
```

`resolveNodeVersion` saf fonksiyon: istek doluysa o, değilse proje
varsayılanı. Öncelik kuralının tek yeri burası.

## Dockerfile

`ARG NODE_VERSION=24` + `FROM node:${NODE_VERSION}-slim`. Varsayılan `24`
(minör serbest); varyantlar tam sürümle derlenir.

## Değişen dosyalar

| Dosya | Ne |
|---|---|
| `backend/internal/runner/node-versions.txt` | yeni — tek kaynak |
| `runner/Dockerfile` | `ARG NODE_VERSION` |
| `backend/internal/runner/image.go` | yeni — `NodeVersions`, `SupportsNodeVersion`, `ImageFor` |
| `backend/internal/runbuild/builder.go` | `resolveNodeVersion` — öncelik kuralının tek yeri |
| `backend/internal/db/migrations/000013_node_surumu.sql` | `projects.default_node_version`, `runs.node_version` |
| `backend/internal/httpapi/runnerinfo.go` | `GET /api/runner/node-versions` |
| `backend/internal/runner/sandbox/docker.go` | `imageHint()` — eksik imaj mesajı |
| `frontend/src/components/runs/StartRunForm.tsx` | koşu başına sürüm seçici |
| `frontend/src/app/projects/page.tsx` | proje varsayılanı (H2'nin tek arayüzü) |
| `.github/workflows/release-images.yml` | `runner-node` işi (sürüm × mimari) |
| `Makefile` | `runner` hedefi tüm varyantları derler |

## Doğrulama

1. `node:24-slim`in gerçek sürümü ölçülür (varsayım yapılmaz)
2. Listeye olmayan sürüm → 400
3. Seçili sürümle koşu → container `docker inspect` ile doğru imajı gösterir
4. İmajı silip koşu → klonlamadan önce açık hata
5. CI: `docker buildx imagetools inspect` iki mimariyi gösterir
