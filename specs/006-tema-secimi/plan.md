# Plan: Tema Seçimi — Sistem / Açık / Koyu

- **Spec no:** 006 — [spec.md](spec.md)
- **Durum:** Uygulandı

---

## Mekanizma

Seçim `<html data-theme="light|dark">` özniteliğine yazılır. Öznitelik **yoksa**
işletim sisteminin tercihi geçerlidir.

CSS üç kapsam tanır ve sırası önemlidir:

```css
@theme { /* açık jetonlar — :root üzerine */ }

@media (prefers-color-scheme: dark) {
  :root:not([data-theme]) { /* koyu jetonlar */ }   /* henüz seçim yok */
}

:root[data-theme="light"] { color-scheme: light; }
:root[data-theme="dark"]  { /* koyu jetonlar */ }   /* açık seçim, sonda */
```

- Medya bloğu yalnızca öznitelik **yokken** çalışır: betik henüz çalışmadıysa veya
  JavaScript kapalıysa sistem tercihi yine de uygulanır.
- İki seçicinin özgüllüğü eşit olduğu için açık seçim **sonda** durur ve kazanır.
- `color-scheme` her kapsamda ayrıca verilir: tarayıcının kendi çizdiği kaydırma
  çubuğu ve form denetimleri de doğru tarafa geçsin diye. Yalnızca jetonları
  değiştirip bunu unutmak, açık temada koyu kaydırma çubuğu bırakır.

Koyu jeton listesi **iki kez** yazılıyor. Tek seçiciye indirilemez: biri medya
sorgusuna bağlı, diğeri değil. Kod içine "burası bilerek tekrar" notu düşüldü ki
sonradan "sadeleştirme" adına biri birleştirmeye çalışmasın.

## Sıçramanın (FOUC) önlenmesi

Tema React yüklendikten sonra uygulansaydı sayfa önce yanlış temayla boyanır,
sonra atlardı. Bu yüzden `<head>` içine **engelleyici, satır içi** bir betik konur;
`localStorage`'ı okur ve özniteliği ilk boyamadan önce yazar.

Betik bilerek küçük ve bağımsız — bir modülü beklemesi, beklemenin kendisi olurdu.
`<html>` üzerinde `suppressHydrationWarning` var: sunucunun ürettiği HTML ile
istemcininki bu öznitelikte **kasıtlı olarak** ayrışır.

## Dosyalar

| Dosya | İş |
|-------|-----|
| `app/globals.css` | Üç kapsam + `color-scheme` |
| `lib/theme.ts` | `readMode` / `writeMode` / `applyMode` + `<head>` betiği |
| `components/ThemeToggle.tsx` | Üç düğmeli anahtar; "sistem" seçiliyken `matchMedia` dinler |
| `components/Sidebar.tsx` | Anahtar kenar çubuğunun altında — her sayfadan erişilir |
| `app/layout.tsx` | Betiği `<head>`'e koyar |

`ThemeToggle` ilk çizimde her zaman "sistem" gösterir ve bağlandıktan sonra
gerçek seçime düzeltir — sunucuda `localStorage` yok. Sayfanın **renkleri** bundan
etkilenmez; onları `<head>` betiği zaten ayarladı. Ayrışan tek şey hangi düğmenin
vurgulu olduğu ve o da bir kare sürer.

## Riskler

| Risk | Önlem |
|------|-------|
| Yanlış temanın bir an görünmesi | Engelleyici satır içi betik, ilk boyamadan önce |
| Gizli sekmede `localStorage` hata verir | `try/catch`; tema yüzünden uygulama düşmez |
| Koyu jeton listesi iki yerde ayrışır | Kod içinde "bilerek tekrar" notu |
| Açık tema hiç görülmemiş olabilir | Doğrulama listesinin ilk maddesi budur |

## Doğrulama

1. Güneş simgesi → arayüz açık temaya geçer; yenilemede öyle kalır
2. Ay simgesi → koyu; ekran simgesi → sistemi takip eder
3. "Sistem" seçiliyken işletim sistemi teması değiştirilince arayüz anında döner
4. Sayfa açılışında beyaz/siyah çakma yok
5. **Açık temada her ekran gözden geçirilir** — jetonlar baştan tanımlıydı ama
   makinede koyu tema kullanıldığı için bu ekranlar bugüne kadar hiç görülmedi
