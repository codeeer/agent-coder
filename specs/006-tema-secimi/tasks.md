# Görevler: Tema Seçimi — Sistem / Açık / Koyu

- **Spec no:** 006 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı

---

## CSS

- [x] T01 `globals.css` — koyu jetonlar `@media`+`@theme` içinden çıkarıldı; üç kapsam:
      `:root:not([data-theme])` medya sorgusu, `[data-theme="light"]`, `[data-theme="dark"]`
- [x] T02 `color-scheme` her kapsamda ayrıca veriliyor (kaydırma çubuğu, form denetimleri)
- [x] T03 Grafik durum renkleri kapsam dışında bırakıldı — temaya göre değişmiyorlar

## Uygulama

- [x] T10 `lib/theme.ts` — `ThemeMode`, `readMode`, `writeMode`, `applyMode`;
      `localStorage` erişimi `try/catch` içinde (gizli sekme)
- [x] T11 `themeBootstrapScript` — `<head>` içinde engelleyici, ilk boyamadan önce
- [x] T12 `layout.tsx` — betik + `suppressHydrationWarning`
- [x] T13 `ThemeToggle` — üç düğme; "sistem" seçiliyken `matchMedia` değişimini dinler
- [x] T14 Kenar çubuğuna yerleştirildi — her sayfadan erişilir
- [x] T15 `IconMonitor` / `IconSun` / `IconMoon`

## Doğrulama

- [x] T16 `npm run test` (25), `npm run typecheck`, `npm run lint` temiz
- [x] T17 `make up` — derlenmiş CSS'te üç kapsam da var (`:root[data-theme=dark]`,
      `:root[data-theme=light]`, `@media (prefers-color-scheme:dark){:root:not([data-theme])}`)
- [x] T18 Başlatma betiği sunucudan gelen HTML'in `<head>`'inde
- [x] T19 Anahtarın üç durumu Playwright ile denendi; `html[data-theme]` her üç
      seçimde doğru
- [x] T20 `dark:` varyantı `data-theme`'e bağlandı (`@custom-variant`); koda dağılmış
      sekiz `dark:` kullanımı ve on beş sabit palet rengi jetonlara çevrildi
- [x] T21 Açık tema jetonları ÖLÇÜLEREK yeniden değerlendi (aşağıdaki not)
- [x] T22 **Açık temada tüm ekranlar gözden geçirildi** — Panel, Projeler, Agent'lar,
      Çalıştırmalar, Rapor, Modeller, Ayarlar; koyu tema gerilemedi
- [x] T23 `scripts/screenshot.mjs` — görsel doğrulama tekrarlanabilir hale getirildi;
      `.mcp.json`'a Playwright MCP eklendi

---

## Notlar

### Ölçüm 1 — `dark:` varyantı işletim sistemini takip ediyordu

Tema anahtarı çalışıyordu ama arayüzün bir kısmı koyu kalıyordu. Sebep: Tailwind'in
`dark:` varyantı varsayılan olarak `prefers-color-scheme` medya sorgusuna bağlıdır,
bizim `data-theme` özniteliğimize değil. Sistemi koyu olan bir kullanıcı açık temayı
seçtiğinde `dark:` ile yazılmış her kural koyu davranmaya devam ediyordu.

İki adımda çözüldü: varyant `@custom-variant dark (&:where([data-theme="dark"], …))`
ile özniteliğe bağlandı; ayrıca kodda kalan sekiz `dark:` kullanımı ve on beş sabit
palet rengi (`text-red-600`, `bg-red-500/5`, `text-amber-600` …) jetonlara çevrildi.
Jetonun iki temada da karşılığı olduğu için artık `dark:` yazmaya gerek yok.

### Ölçüm 2 — açık tema göz kararıyla yazılmıştı

"Renksiz görünüyor" şikâyeti ölçülünce doğrulandı. Beyaz kart zemininde:

| Jeton | Eskisi | Kontrast | Yenisi | Kontrast |
|---|---|---|---|---|
| canvas ↔ kart | `#f7f8fa` | **1,06:1** | `#eef1f5` | 1,13:1 |
| line-strong | `#cfd5de` | 1,48:1 | `#b9c2cf` | 1,80:1 |
| ink-3 (tüm küçük metin) | `#858e9c` | **3,31:1** | `#6e7787` | 4,51:1 |
| ok | `#0d8a5f` | **4,36:1** | `#0a7350` | 5,86:1 |
| warn | `#a56200` | 4,83:1 | `#8a5200` | 6,39:1 |

Kalın yazılanlar eşiğin altındaydı: kart sayfadan ayrılmıyor, küçük punto metin
(WCAG AA için 4,5:1) okunmuyordu. İkincil düğmenin kenarı da `line` yerine
`line-strong` oldu — beyaz düğmenin beyaza yakın zemindeki ince kenarı görünmüyor,
düğme düğme gibi durmuyordu.

Koyu tema değerlerine dokunulmadı.

### Ölçüm 3 — katmansız bir CSS kuralı bütün metin renklerini eziyordu

Açık temaya geçildiğinde düğmeler "çirkin" görünüyordu. Ekran görüntüsü yerine
hesaplanmış değer okununca sebep anında çıktı:

```
Çalıştır   zemin=rgb(91,91,214)   metin=rgb(13,17,23)     <- beyaz olmalıydı
```

Birincil düğmenin zemini doğru (`accent`), yazısı yanlıştı: `accent-ink` (beyaz)
yerine `ink` (siyaha yakın) geliyordu. Yani `text-accent-ink` utility'si hiç
uygulanmıyordu.

Sebep `globals.css` içindeki şu kuraldı:

```css
input, select, textarea, button { font: inherit; color: inherit; }
```

İki sorunu birden vardı. Birincisi **gereksizdi** — Tailwind'in preflight'ı zaten
aynısını yapıyor. İkincisi ve asıl olanı: **katman dışında yazılmıştı.** CSS'te
katmansız bir kural, katmanlı olanların hepsini yener; Tailwind utility'leri
`@layer utilities` içinde olduğu için `color: inherit` düğmelerdeki BÜTÜN metin
rengi utility'lerini eziyordu.

Hata iki temayı da bozuyordu, koyu tema sadece daha iyi gizliyordu: orada da
düğme yazısı `accent-ink` (koyu) yerine `ink` (açık) çıkıyordu.

Düzeltme: gereksiz kural silindi, kalan öğe düzeyindeki kurallar `@layer base`
içine alındı. Odak halkası bilerek katman dışında bırakıldı — o her zaman
kazanmalı.

**Ders:** Bu hatayı tip kontrolü, linter ve birim testler göremez; ancak ekrana
bakılarak veya hesaplanmış değer okunarak görülür. Görsel doğrulama artık
`scripts/screenshot.mjs` ile tekrarlanabilir.

### Neden T22 ayrı bir maddeydi

Açık tema jetonları projenin ilk gününden beri tanımlıydı ve her ekran onlarla
yazıldı. Ama geliştirme makinesi koyu temada olduğu için **bu ekranlar açık temada
bugüne kadar hiç görülmedi.** "Jetonlar tanımlıydı" ile "açık temada iyi görünüyor"
aynı şey değil; ikincisi ancak bakılarak bilinir.

Ölçüyle düzeltilenler yukarıda. Elle bakılmadan bilinemeyecek olan kalan şüpheli:

- Grafiklerdeki sarı ("diğer") açık zeminde 3:1 kontrastın altında. Bu bilinçliydi
  ve karşılığı var (etiketli gösterge + zorunlu tablo görünümü), ama açık temada
  gerçekten okunup okunmadığı görülmeli.

`select` okunun sabit rengi düzeltildi: veri URI'sinin tamamı `--select-arrow`
jetonuna alındı ve her temada yeniden tanımlanıyor.

### Ekran görüntüsü — çözüldü

Chrome'un `--screenshot` bayrağı bu makinede çıktı üretmiyordu. Playwright ile
çözüldü: `scripts/screenshot.mjs` temayı `colorScheme` **ve** `localStorage` ile
birlikte zorluyor, böylece geliştirme makinesi koyu temada olsa bile açık tema
görülebiliyor. `--probe` seçeneği ekran görüntüsü yerine hesaplanmış renkleri
yazdırır — renk şikâyetlerinde daha kesin.
