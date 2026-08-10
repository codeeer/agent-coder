# Görevler: Agent Çıktısının Biçimlendirilmiş Gösterimi

- **Spec no:** 005 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı

---

## Ayrıştırma

- [x] T01 `markdown/parse.ts` — blok ayrıştırıcı: başlık, kod bloğu, tablo, liste,
      alıntı, yatay çizgi, paragraf; tanınmayan satır paragraf olur
- [x] T02 `markdown/inline.ts` — satır içi ayrıştırıcı: kod, kalın, italik, üstü
      çizili, bağlantı; `safeHref` beyaz listesi; derinlik ve parça sayısı sınırı
- [x] T03 `parse.test.ts` — 14 test: her blok türü, kapanmayan kod bloğu, ayıraçsız
      boru işareti, eksik hücre, CRLF, tanınmayan girdi
- [x] T04 `inline.test.ts` — 11 test: her işaret türü, **aynı satırda birden fazla
      kalın metin**, iç içe işaretleme, kod içindeki yıldız, güvensiz bağlantı,
      derin iç içe girdi

## Gösterim

- [x] T10 `markdown/Markdown.tsx` — blok ve parçaları projenin jetonlarıyla çizer;
      `dangerouslySetInnerHTML` kullanılmaz
- [x] T11 Tablo ve kod bloğu kendi kabında yatay kayar; sayfa gövdesi kaymaz
- [x] T12 `runs/[id]` sayfasında `AgentOutput` — **Biçimli / Ham metin** anahtarı
- [x] T13 Doğrulama: `npm run test`, `npm run typecheck`, `npm run lint` temiz

## Test altyapısı

- [x] T20 Frontend birim testi Node'un yerleşik koşucusuyla çalışır (`node --test`);
      ek test kütüphanesi eklenmedi. `tsconfig.json`'a `allowImportingTsExtensions`,
      `package.json`'a `test` betiği, `Makefile`'daki `test-frontend`'e testler eklendi

---

## Notlar

### Ölçüm 1 — SONSUZ DÖNGÜ: paylaşılan `g` bayraklı düzenli ifade

**Belirti:** Çalıştırma detay sayfası açıldığında sekme donuyor, bellek kullanımı
saniyeler içinde onlarca GB'a çıkıyor ve **işletim sistemi kilitleniyordu.**

**Sebep:** İlk sürümde satır içi ayrıştırma `Markdown.tsx` içine gömülüydü ve modül
düzeyinde tek bir `g` bayraklı düzenli ifade paylaşıyordu:

```ts
const INLINE = /…|\*\*([\s\S]+?)\*\*|…/g;   // modül düzeyinde, TEK nesne

function inline(text) {
  INLINE.lastIndex = 0;                      // her çağrıda sıfırlanıyor
  while ((m = INLINE.exec(text)) !== null) {
    …
    out.push(inline(strong));                // ÖZYİNELEME → lastIndex yine 0
  }
}
```

`g` bayraklı bir `RegExp` nesnesi `lastIndex` durumunu kendi içinde taşır. İç çağrı
bittiğinde `lastIndex` sıfır kalıyor; dış döngü **aynı eşleşmeyi yeniden buluyor**,
yeniden özyineleniyor ve her turda diziye bir eleman ekliyordu. Döngünün çıkışı yok.

Tetikleyici, çıktıda **tek bir `**kalın**` işareti** olması. Kilitlenmenin görüldüğü
gerçek çıktıda `**İncelemeye uygun kod yok.**` vardı.

Ölçüldü — eski mantık, tek satırlık girdiyle:

```
üretilen eleman sayısı: 100000 (doğrusu: 1)   ← sınır konmasa artmaya devam ediyor
```

**Düzeltme:** Ayrıştırma çizim kodundan `inline.ts`'e çıkarıldı ve düzenli ifade
**her çağrıda yeniden üretiliyor** — `lastIndex` durumu artık çağrılar arasında
paylaşılmıyor. Ayrıca girdi güvenilmeyen olduğu için iki emniyet kemeri eklendi:
özyineleme derinliği (12) ve çağrı başına parça sayısı (20.000) sınırı. Sınıra
gelinirse metin ham haliyle döner — hiçbir şey kaybolmaz.

**Asıl ders — testin nerede olduğu meselesi:** Ayrıştırma çizim bileşeninin içindeydi;
`parse.ts` test edilirken `inline` edilemiyordu. Tip kontrolü, linter ve blok
testlerinin hepsi temizdi. Hata ancak tarayıcı makineyi kilitlediğinde görüldü.

Kural olarak yazıldı: **saf mantık React bileşeninin içine gömülmez**, kendi modülünde
durur ve test edilir. `inline.test.ts` içindeki "aynı satırda birden fazla kalın metin"
testi tam olarak bu hatayı yakalar.

**İkinci ders:** Ekran görüntüsü alma girişimi bu sayfada iki kez sessizce başarısız
olmuştu (dosya hiç oluşmadı). Bunu "araç kaprisi" sayıp geçmek yanlıştı — sayfanın
yüklenmemesi hatanın kendisiydi. Bir doğrulama adımı açıklanamayan şekilde
başarısız olduğunda sebebi bulunmadan devam edilmez.

### Ölçüm 2 — `safeHref` iki dosyada durmamalı

İlk yazımda `safeHref` `parse.ts` içindeydi ama yalnızca satır içi ayrıştırma
kullanıyordu. Ayrıştırma ikiye bölününce `inline.ts`'e taşındı — tek kullanıcısının
yanında.
