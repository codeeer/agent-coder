# Plan: Arayüz Denetimi

- **Spec:** [spec.md](spec.md) · **Görevler:** [tasks.md](tasks.md)

---

## Yaklaşım

Üç geçiş, her biri bir öncekinin bulgusunu kullanır:

1. **Kullanıcı gibi gezme** — tüm ekranlar masaüstü ve telefon genişliğinde.
2. **Ölçüm** — hesaplanmış renkler, kontrast, erişilebilir adlar.
3. **Kök neden** — bulunan her sorun bileşende değil, mümkünse **token'da** düzeltilir.

Üçüncüsü önemli: aynı sorun on ekranda görünüyorsa on ekranı düzeltmek yanlış
cevaptır. Örneğin `ink-3` koyu temada eşiğin altındaydı ve on dört ayrı bileşende
kalıyordu — düzeltme tek satır oldu.

## Araç: `scripts/theme-audit.mjs`

Playwright ile her sayfayı iki temada açar ve şunları ölçer:

| Ölçüm | Eşik | Gerekçe |
|---|---|---|
| Metin kontrastı | 4,5:1 (iri metin 3:1) | WCAG 2.1 AA, 1.4.3 |
| Denetim sınırı | 3:1 | WCAG 2.1 AA, 1.4.11 |
| Tema eşliği | — | bir temada geçip diğerinde kalan bileşen |
| Düğme durumları | — | hover / focus / disabled iki temada da var mı |

**Rengi tarayıcıya çözdürür.** Düzenli ifadeyle `rgb(...)` ayrıştırmak yetmiyor:
Tailwind v4 saydamlık ekini (`/35`) `oklab(... / 0.35)` olarak üretiyor. İlk
uygulama bunu tanımayıp elemanı sessizce atlıyordu. Tuval (`canvas`) üzerinden
çözüm, tarayıcının kendi renk motorunu kullanır.

## Token modeli

Mevcut ölçek korunur; tek bir sorumluluk eklenir:

```
--color-line          süsleme: kart kenarı, ayraç, düğüm çerçevesi
--color-line-strong   süsleme (güçlü): vurgulanmış çerçeve
--color-control-line  DENETİM sınırı: düğme, girdi, açılır liste  ← yeni
```

Ayrımın testi basit: *bu çizgi olmasaydı kullanıcı burada tıklanabilir bir şey
olduğunu anlar mıydı?* Cevap hayırsa `control-line`.

## Riskler

| Risk | Önlem |
|---|---|
| Token koyulaştırmak arayüzü ağırlaştırır | Yalnızca denetim sınırları değişir; süsleme çizgileri aynı kalır |
| Ölçüm aracı sessizce eleman atlar | Ham hesaplanmış değer tek tek doğrulandı; atlanan bulundu ve düzeltildi |
| `ink-3` koyulaşınca `ink-2` ile ayrımı kaybolur | İkisi arasındaki fark ölçüldü; hiyerarşi zaten punto ve ağırlıkla taşınıyor |

## Doğrulama

1. `node scripts/theme-audit.mjs` → 0 kalan, 0 tema eşliği hatası
2. Telefon genişliğinde yatay taşma 0px, çekmece açılıp kapanıyor
3. Her sayfada adsız düğme/etiketsiz alan yok, tek `h1`
4. Açılır liste, disabled düğme ve iskelet iki temada da ölçülüyor
5. `make lint`, `make test`, birim testler temiz
