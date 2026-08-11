# 02 — İmaj Yayını ve Sürüm Kayması

- **Tarih:** 2026-08-11
- **Durum:** Uygulandı
- **Kapsam:** Yayınlama katmanı (CI) — uygulama koduna dokunulmadı

---

## Özet

Yayınlanan GHCR imajları main'in gerisinde kaldı. Kod düzeltildi ama imaj
yayınlanmadığı için `make quickstart` kullanan herkes **aylarca eski backend ile
koştu.** Hata, kodda olmayan bir yerde aranmasına yol açacak biçimde göründü.

Bu doküman ne olduğunu, nasıl teşhis edildiğini ve neyin değiştiğini kayda
geçirir — çünkü aynı sessiz kayma tekrar edebilir ve belirtisi yine yanlış yeri
işaret eder.

## Belirti

Özel bir depoda agent koşusu şununla düşüyordu:

```
[runner] repo klonlanıyor: https://<host>/<repo>.git (branch: master)
fatal: could not read Username for 'https://<host>': No such device or address
[runner] HATA: klonlama başarısız
```

Mesaj "kimlik bilgisi eksik" diyor. Bu, bakan kişiyi **yapılandırmaya**
yönlendiriyor: token yanlış mı, kullanıcı adı boş mu, sağlayıcı bağlı mı?
Hiçbiri değildi.

## Teşhis

Adım adım, her biri ölçülerek:

| # | Kontrol | Sonuç |
|---|---|---|
| 1 | `projects.git_provider_id` ve `git_providers` kaydı | dolu ve geçerli — kullanıcı hatası değil |
| 2 | Koşan runner container'ının env'i (`docker inspect`) | `REPO_URL`/`REPO_BRANCH` var, **`GIT_USERNAME`/`GIT_TOKEN` yok** |
| 3 | İmajdaki `entrypoint.sh` | doğru — `GIT_TOKEN` doluysa credential store kuruyor |
| 4 | main'deki `runbuild/builder.go` ve `opencode/runner.go` | **doğru** — secret türden bağımsız çözülüyor ve env'e ekleniyor |
| 5 | `docker pull` | "Image is up to date" |

5. satır teşhisin döndüğü nokta: `pull` çalışıyordu, **bayat olan registry'nin
kendisiydi.** Yayındaki `latest`, düzeltmeleri içermeyen eski bir derlemeydi.

Kanıt, düzeltme yayınlanmadan önce registry'den okundu:

```
backend imajı
  latest : 336b44293253
  0.1.1  : 336b44293253   ← aynı digest
```

`latest`, aylar önceki `v0.1.1` derlemesiyle birebir aynıydı.

## Kök neden

Yayın yalnızca `v*` etiketlerini dinliyordu ve `latest`, ön-sürüm olmayan
etiketlerde basılıyordu. Son etiket `v0.1.1` olduğu için `latest` orada dondu;
main ilerledi.

Gerekçe savunulabilirdi: *"`latest` her commit'te değişirse, kullanıcının çektiği
imaj ile deponun beklediği şema ayrışabilir."* Uygulamada bunun **tersi** oldu.
Hareketli bir `latest` en fazla bir migration uyumsuzluğu üretirdi; donmuş bir
`latest` aylarca yanlış kod çalıştırdı ve kimse fark etmedi.

> **Bayat imaj, hareketli `latest`ten tehlikelidir.** Birincisi sessizdir,
> ikincisi gürültülüdür — ve gürültülü arıza teşhis edilebilir olandır.

## Ne değişti

Yayın `.github/workflows/release-images.yml`e taşındı (eski `yayin.yml`
**silindi** — ikisi birlikte kalsaydı bir `v*` etiketinde aynı registry
etiketlerine yarışırlardı).

| Olay | `latest` | `sha-<kısa>` | semver |
|---|:---:|:---:|:---:|
| `main`'e push | ✅ | ✅ | — |
| `v*` etiketi | — | ✅ | ✅ |
| elle tetikleme (main'den) | ✅ | ✅ | — |

- **`latest` artık main'in ucudur.** Her main commit'i imaj üretir.
- **`latest`, en yeni sürüm etiketinden ileride olabilir** — bu bilinçlidir.
  Kurulumunun sabit kalmasını isteyen semver veya `sha-<kısa>` kullanır.
- **İmaj hangi commit'ten geldiğini üzerinde taşır**
  (`org.opencontainers.image.revision`). `make quickstart` bunu ekrana yazar ve
  `docker inspect` ile her zaman sorulabilir.
- Duman testi, imajdaki `revision` etiketinin koşulan commit ile eşleştiğini
  doğrular. Bu iş akışının var olma sebebi kaymayı görünür kılmaksa, kaymayı
  kontrol etmemesi tutarsız olurdu.

## Alınan dersler

**1. Yayınlama katmanı da bir çalışma zamanıdır.** "Kod doğru" demek "kullanıcı
doğru kodu çalıştırıyor" demek değildir. `main`'de yeşil testler, yayındaki
davranış hakkında hiçbir şey söylemez.

**2. Belirti kök nedeni işaret etmeyebilir.** Buradaki mesaj kimlik bilgisi
eksikliğini gösteriyordu ve teknik olarak doğruydu — env'de gerçekten yoktu. Ama
"neden yok" sorusunun cevabı yapılandırmada değil, çalışan ikilinin yaşında
duruyordu. **Container'ın env'ini canlı okumak** (adım 2) teşhisi kilitten
kurtaran şey oldu; koda bakmak yetmezdi.

**3. Sürüm görünür olmalı.** Kullanıcının elindeki imajın hangi commit'ten
geldiğini soramaması, kaymanın aylarca sürmesinin sebebiydi. `docker pull`'un
"up to date" demesi güven verici ve yanıltıcıydı.

**4. İki iş akışı aynı etiketi basmamalı.** Yeni dosya eklenirken eskisinin
silinmesi bir düzen tercihi değil, doğruluk gereğiydi.

## İlgili

- `.github/workflows/release-images.yml` — iş akışı ve gerekçe yorumları
- `README.md` → "Hangi etiketi kullanmalı"
- `AGENTS.md` → yayınlama kuralları
- Commit `21dc1cd`
