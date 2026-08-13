# Görevler: Node sürümlü runner imajları

- **Spec no:** 013 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı

---

## Yapılanlar

- [x] T01 `runner/node-versions.txt` — tek kaynak
- [x] T02 `Dockerfile`'a `ARG NODE_VERSION`
- [x] T10 Migration `000013` — proje ve koşu sürüm sütunları
- [x] T20 `runner.ImageFor` + `SupportsNodeVersion` (+ birim testleri)
- [x] T21 `GET /api/runner/node-versions`
- [x] T22 `POST /api/runs` → `nodeVersion`, geçersizde 400
- [x] T23 `EnsureImage` klonlamadan önce + `imageHint`
- [x] T30 Koşu formunda seçici, proje ayarında varsayılan
- [x] T31 Koşu detayında sürüm (yalnızca seçilmişse)
- [x] T40 CI `runner-node` işi, `make runner` tüm varyantlar
- [x] T90 Gerçek koşuyla uçtan uca doğrulama
- [x] T92 `AGENTS.md`: `make runner` kuralı ve Durum kaydı

---

## Ölçüm notları

### Ölçüm 1 — Etiket sürümü, çalışan sürüm değildir

`node:24-slim` imajının içindeki Node **v24.19.0** çıktı. Yani `24` etiketi
bir sürüm değil, bir seri. Kullanıcı arayüzünde "24" göstermek, çalışanın
24.19.0 olduğu bir sistemde yanlış bilgi olurdu.

Bu yüzden liste **tam sürüm** taşıyor ve varyantlar tam sürümle derleniyor.

### Ölçüm 2 — Motorun bağımlılıkları seçilen sürümü kısıtlıyor

`24.13.0` ile derlenen imajda opencode'un bağımlılığı `ini@7.0.0`
**EBADENGINE** uyarısı verdi: paket en az 24.15.0 istiyor. Kurulum yine de
tamamlandı ve motor çalıştı, ama bu sessiz bir sınır.

Ders: desteklenen sürüm listesi keyfî değil; her yeni sürüm eklendiğinde
imajın **derlenip ayağa kalktığı** görülmeli. CI'ın sürüm matrisini duman
testine bağlaması bunun içindir.

### Ölçüm 3 — Emülasyon değil, yerel koşucu

Frontend'in arm64 derlemesi QEMU altında **10 dk 23 sn** sürüyordu ve iki
ardışık koşu orada düştü (`SIGILL`, öncesinde Google Fonts zaman aşımı).

Çapraz derleme (`--platform=$BUILDPLATFORM`) denenmeden önce ölçüldü:
`.next/standalone` içine `@img/sharp-linux-arm64/lib/sharp-linux-arm64.node`
giriyor — yani native modül var, çapraz derleme güvenli değil.

Çözüm mimariyi kendi koşucusunda derlemek oldu (`ubuntu-24.04-arm`):
**10:23 → 73 sn**. Ölçmeden "cross-compile yaparız" denseydi imaj bozuk
çıkardı.

### Ölçüm 4 — Aynı satırda iki ayrı dil, iki ayrı tuzak

`NODE_VERSIONS := $(shell grep -vE '^\s*(#|$$)' …)` satırı iki kez düzeltildi:

1. `unterminated call to function 'shell'` — kaçırılmamış `#` **Make** için
   yorum başlatıyor. `\#` ile çözüldü. Hata mesajı `shell`i işaret ediyor,
   sebep ise iki karakter ötede.
2. `\s` **POSIX ERE'de yok** — `grep -E` onu "s harfi" sanar. `[[:space:]]`
   ile çözüldü.

Ders: bir satır Make, kabuk ve `grep` tarafından sırayla okunuyorsa her
katmanın kendi kaçış ve sözdizim kuralı var. Çalıştığını görmeden yazılmaz.

### Ölçüm 5 — Sürümlü imajlar sessizce bayatlıyor (2026-08-13)

`runner/Dockerfile` düzeltildi (`ENV NPM_CONFIG_OFFLINE` kaldırıldı, bkz.
[spec 014](../014-kurumsal-paket-deposu/spec.md)), `latest` yeniden derlendi,
düzeltme doğrulandı. Bir gün sonra aynı hata gerçek koşuda **hâlâ** görüldü.

Sebep: koşular `latest` kullanmıyordu. İki projenin varsayılan Node sürümü
`24.13.0` olduğu için gerçekte çalışan imaj `node-24.13.0` idi — ve o varyant
düzeltmeden **önce** derlenmişti.

```
node-24.13.0  derlendi  2026-08-12 21:45
latest        derlendi  2026-08-12 23:44   ← düzeltme burada doğrulandı
düzeltme commit'i       2026-08-13 00:50
```

Teşhis, imaj ID'lerini karşılaştırınca çıktı:

```
container imaj adı : agent-coder/opencode-runner:node-24.13.0
container imaj ID  : sha256:a8207b99…
etiketin ID'si     : sha256:a7f84dcb…   ← latest, başka bir imaj
```

**Ders:** `docker build` tek başına yetmez. Dockerfile'a dokunulduğunda
`make runner` çalıştırılır — o listedeki her sürümü de derler. Bir düzeltmenin
"doğrulandı" sayılması için doğrulamanın **gerçekte kullanılan imaj** üzerinde
yapılmış olması gerekir; `latest` üzerinde yapılan doğrulama, varsayılanı
sürümlü imaj olan bir kurulumda hiçbir şey kanıtlamaz.

### Ölçüm 6 — Bu spec geriye dönük yazıldı

Özellik uygulandı, doğrulandı ve gönderildi; spec'i **bir gün sonra** yazıldı.
Metodoloji `spec.md → plan.md → tasks.md → kod` diyor.

Bedeli somut: yukarıdaki beş ölçümün tamamı bir gün boyunca yalnızca commit
mesajlarında ve kod yorumlarında durdu. Ölçüm 5'teki hata, Ölçüm 3'teki karar
yazılı olsaydı belki daha erken görülürdü — çünkü "hangi imaj gerçekten
kullanılıyor" sorusu K3'ün doğal devamı.

Kural: geriye dönük spec, spec'siz koddan iyidir; ama sırayla yazılan
spec'ten kötüdür.
