# Görevler: Ayar denetimi ve ayar araması

- **Spec no:** 016 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Uygulandı (2026-08-14)

Sıra riske göre: önce anahtarın mevcut kaydetme akışına oturması (yeni denetim
tipi oraya giriyor), sonra arama. İlk dört görev bitene kadar arama hiç yok —
hata çıkarsa sebebi tek olsun.

---

## H1 — İki durumlu ayar denetimi

- [x] T01 `Switch` kalıbı bileşen katmanına eklenir (`role="switch"`,
      `aria-checked`, açık/kapalı konumu farklı) → tarayıcıda tıklayınca durum
      değişir, sekme ile odaklanılır, boşlukla değişir
- [x] T02 `Switch`in sınırı ve odak halkası iki temada doğrulanır →
      `getComputedStyle` ile sınır rengi okunur; iki temada da denetim sınırı
      `control-line` jetonundan geliyor
- [x] T03 `SettingRow` ayarın tipi `bool` olduğunda `Switch` çizer → Çalıştırma
      bölümünde "Motor loglarını sakla" anahtar olarak görünür, içinde `true`
      yazan metin kutusu kalmaz
- [x] T04 Anahtarın yanında durum **yazıyla** da okunur → "Açık" / "Kapalı"
      metni görünür; renk tek kanal değil
- [x] T05 Anahtar mevcut taslak/kaydet akışına bağlanır → tıklayınca "Kaydet"
      belirir; kaydetmeden başka bölüme geçip dönünce sunucudaki eski değer
      durur
- [x] T06 Kaydetme çalışır → "Kaydedildi" görünür; sayfa yenilenince yeni değer
      gelir
- [x] T07 Denetim hizası ölçülür → `getBoundingClientRect` ile anahtarın sol
      kenarı, aynı bölümdeki sayı alanlarının sol kenarıyla aynı

## H2 — Ayar araması

- [x] T10 `setting-search.ts` — `settingMatches` ve `filterSettings` saf
      fonksiyonları → boş sorgu hepsini döndürür, süzmez
- [x] T11 [P] Eşleştirme testleri (`node --test`) → etikette ve açıklamada
      eşleşme, eşleşmeme, Türkçe katlama ("çalışma"↔"ÇALIŞMA", "ışık"↔"IŞIK",
      "iş"↔"İŞ"), boşluk kırpma, çok kelimeli sorgu, ham anahtarın
      **aranmadığı** → `npm run test` yeşil
- [x] T20 `RuntimeSettings` opsiyonel `query` parametresi alır → sorgu verilince
      yalnızca eşleşen ayarlar çizilir
- [x] T21 Sorgu varken hiç eşleşme yoksa "eşleşme yok" gösterilir → "zzz"
      araması boş kart yığını değil, ne olduğunu söyleyen bir durum gösterir
- [x] T22 Mevcut altı çağrı yerinin bozulmadığı doğrulanır → Modeller, Jira,
      Dış araçlar, Paket deposu, Çalıştırma, Rapor bölümleri tek tek açılır ve
      bugünkü hâlleriyle aynı
- [x] T30 Ayarlar ekranına araç çubuğu ve arama alanı eklenir (`Toolbar` +
      `SearchField`, `hint` **verilmez**) → alan görünür ve yazılabilir
- [x] T31 Sorgu varken içerik sütunu bölüm içeriği yerine süzülmüş listeyi
      gösterir; eşleşmeler **hangi bölüme ait olduklarıyla** görünür → "süre"
      araması farklı bölümlerden sonuçları bölüm başlıklarıyla getirir
- [x] T32 Sonuç içinden bir ayar değiştirilip kaydedilebilir → arama açıkken
      kaydetme bugünküyle aynı çalışır
- [x] T33 Bölüm seçimi sorguyu temizler → arama açıkken bir bölüme tıklanınca
      o bölüm içeriği gelir, arama alanı boşalır
- [x] T34 Arama temizlenince önceki bölüme dönülür → sorgu silinince kullanıcı
      aramadan önce baktığı bölümde olur
- [x] T35 Aramanın kapsamı kullanıcıya belli edilir → sorgu etkinken
      "yalnızca ayarlar aranır" bilgisi görünür

## Doğrulama

- [x] T40 İki tema **ayrı ayrı** gözden geçirilir → anahtar, arama alanı ve
      eşleşme yok durumu her iki temada okunur; hesaplanmış renkler ölçülür
- [x] T41 Üç genişlikte denenir (geniş masaüstü, dar masaüstü, telefon) →
      yatay taşma yok
- [x] T42 Hover, focus, yükleniyor, boş ve hata durumları görülür
- [x] T43 `npx tsc --noEmit` ve `npx eslint .` temiz, `npm run test` yeşil

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.

**1. Durum metni birim sütununda duruyor (T04 + T07 birlikte).**
Plan "anahtarın yanında Açık/Kapalı" diyordu, yeri belirtilmemişti. Sayı
ayarlarının birim etiketi (`dakika`, `KB`) zaten anahtarın hemen sağındaki
`w-16` sütunda duruyor; durum metni oraya kondu. İki durumlu ayarın birimi
olmadığı için sütun boş kalıyordu ve metin başka bir yere konsaydı denetim
sütununun hizası (T07) bozulurdu. Anahtarın kabı da sayı alanlarıyla aynı
`w-28`: ölçüldü, sol kenar 1196px'te — dokuz satırın hepsinde aynı.

**2. "Varsayılan: true" → "Varsayılan: Açık".**
Planda yoktu. Satırın altındaki üstveri ham değeri yazıyordu; denetim "Açık"
derken üstveri "true" diyordu. Ayarın kendisi değişmedi (spec: Kapsam dışı),
yalnızca aynı değerin ekrandaki karşılığı iki yerde aynı kelimeyle yazılıyor.

**3. Arama görünümünde hiçbir bölüm seçili görünmüyor.**
Plan yalnızca "bölüm seçimi sorguyu temizler" diyordu (T33). Tarayıcıda
görüldü ki tersi de gerekiyor: sorgu varken yan menü hâlâ "Çalıştırma"yı
`aria-current="page"` ile işaretliyor ama o bölümün içeriği ekranda değil —
menü, gösterilmeyen bir bölümü "bulunduğunuz yer" diye gösteriyordu.
`SettingsNav` artık `active={null}` kabul ediyor. `tab` durumu korunuyor,
T34 (aramadan çıkınca eski bölüme dönme) bundan etkilenmiyor.

**4. Süzgeç görünümü kendi yüzeyini taşıyor.**
Bölümlerden çağrıldığında satırlar bir `Panel` içinde; arama sonucu ise bir
bölüme ait değil. Yüzey çağıran tarafa bırakılsaydı "eşleşme yok"
(`EmptyState`) kenarlıklı bir kartın içindeki kesik kenarlıklı ikinci bir
kutu olarak çıkardı. `RuntimeSettings` sorgu etkinken kendi kartını çiziyor.

**5. `truthy` yardımcısı.**
Backend `strconv.ParseBool` kullanıyor, yani "1" ve "t" de geçerli bir
"açık". Arayüz her zaman "true"/"false" yazsa da API'ye doğrudan yazılmış bir
değer anahtarı sessizce kapalı göstermemeli.

**6. Yükleniyor durumu (T42) tarayıcıda yakalanamadı.**
Yerelde ayar sorgusu 300ms'den kısa sürüyor. `isPending` dalı bu işte hiç
değişmedi (kod okunarak doğrulandı); hover, focus, boş ve hata durumları
tarayıcıda görüldü.

**7. `make dev` bu makinede ayağa kalkmıyor — bu işle ilgisiz.**
`backend/Dockerfile.dev` `golang:1.25` üzerinde `air@latest` kuruyor; air
1.67.4 artık Go 1.26 istiyor ve derleme orada kırılıyor. Doğrulama için
backend+postgres üretim compose'undan, frontend ise host'ta `next dev` ile
(3002 — `.env`'deki `FRONTEND_PORT`, CORS ona bağlı) çalıştırıldı. Bu
kırıklık spec 016'nın kapsamında değil, ayrıca ele alınmalı.

## Ölçümler

| Ne | Açık tema | Koyu tema |
| --- | --- | --- |
| Anahtar sınırı (jeton) | `#7e8b9e` = `control-line` | `#606d86` = `control-line` |
| Sınır / yüzey kontrastı | — | 3,47:1 (denetim sınırı ≥3:1) |
| Topuz / raylı zemin (açık) | beyaz topuz, aksan zemin | 5,53:1 |
| Topuz / raylı zemin (kapalı) | koyu topuz, `raised` zemin | 8,78:1 |
| "Açık"/"Kapalı" metni | — | 9,53:1 |
| Odak halkası | `#5b5bd6`, 2px, offset 2px | `#7b7bf0`, 2px, offset 2px |
| Denetim hizası | anahtar ve dokuz sayı alanı: sol kenar 1196px | aynı |

Koyu tema açığın ters çevrilmiş kopyası değil: `control-line` iki temada
farklı iki değer, topuzun rolü de tersine dönüyor (açık zeminde koyu topuz,
aksan zeminde açık/koyu).
