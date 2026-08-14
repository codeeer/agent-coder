# Plan: Ayar denetimi ve ayar araması

- **Spec no:** 016 — [spec.md](spec.md)
- **Tarih:** 2026-08-14
- **Durum:** Taslak

---

## Yaklaşım

İki iş de **tek bir bileşende** toplanıyor: ayar satırlarını çizen bileşen
zaten ayarın tipini biliyor ve listeyi kayıt defterinden alıyor. İki durumlu
tip için eksik olan denetimi oraya eklemek (H1), ve aynı bileşene bir süzgeç
parametresi vermek (H2) yeterli — ekranın düzenine, bölüm yapısına ve diğer
bölümlere dokunulmuyor.

Arama, bölüm sekmelerinin **yerine geçen bir görünüm** olarak çalışıyor: sorgu
varken içerik sütunu bölüm içeriği yerine süzülmüş ayar listesini gösteriyor,
sorgu temizlenince seçili bölüme geri dönüyor. Sekme durumu korunuyor, yani
aramadan çıkış kullanıcıyı bulunduğu yere bırakıyor.

Eşleştirme mantığı React bileşeninin içine değil **saf bir fonksiyona**
alınıyor; projenin frontend testleri (`node --test`) tam olarak bu tür saf
mantığı sınıyor ve Türkçe büyük/küçük harf katlaması sınanmadan
bırakılamayacak kadar hataya açık.

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
| --- | --- | --- | --- |
| İki durumlu ayar için mevcut `Checkbox` kullanmak | Yeni kalıp yok | `size-3.5` ve kendi etiketini taşıyor; ayar satırında etiket zaten var, denetim sütununa hizalanmıyor | Elendi |
| Yeni `Switch` kalıbı | Denetim sütununa hizalanır, hedef alanı büyük, `role="switch"` ile klavye ve ekran okuyucu doğal | Bileşen katmanına bir kalıp daha | **Seçildi** |
| Anahtar değişince anında kaydetmek | Tek tıkla biter | Diğer 16 ayar açık kaydetme kullanıyor; tek denetimin farklı davranması modeli bozar (spec: Belirsizlik 1) | Elendi |
| Aramayı sunucuya taşımak | Büyük veri kümesinde ölçeklenir | 17 ayar; uç eklemek karşılığı olmayan bir maliyet | Elendi |
| Aramayı bölüm sekmelerinin yanına rozet olarak koymak | Bölüm bağlamı korunur | Kullanıcı yine bölüm bölüm gezmek zorunda; asıl şikâyeti çözmez | Elendi |
| Aramayı ayrı bir sekme yapmak | Durum yönetimi basit | "Ara" diye bir bölüm, bölüm listesinin anlamını bozar | Elendi |

---

## Veri Modeli

**Değişiklik yok.** Yeni tablo, kolon veya migration gerekmiyor. İki durumlu
ayar zaten kayıt defterinde `bool` tipiyle tanımlı ve veritabanında diğerleri
gibi metin olarak duruyor.

Geri alma stratejisi: bu iş yalnızca arayüz tarafında; geri alma commit'i geri
almaktan ibaret.

## Arayüzler

### Go tipleri

**Değişiklik yok.** Backend'e dokunulmuyor. İki durumlu ayarın doğrulaması
(`KindBool` → `strconv.ParseBool`) ve kaydetme yolu bugünkü hâliyle kullanılıyor;
arayüz değeri yine `"true"` / `"false"` metni olarak gönderiyor.

### HTTP API

**Yeni uç yok.** Mevcut uçlar aynen kullanılıyor: ayar listesi, ayar yazma,
ayar sıfırlama.

### Frontend tipleri

```ts
// frontend/src/lib/types.ts — DEĞİŞMİYOR
// SettingKind = "int" | "bool" | "text" zaten tanımlı,
// SettingValue.kind zaten geliyor. Yeni tip gerekmiyor.
```

```ts
// frontend/src/components/settings/setting-search.ts — YENİ (saf mantık)
export function settingMatches(s: SettingValue, q: string): boolean;
export function filterSettings(items: SettingValue[], q: string): SettingValue[];
```

```tsx
// frontend/src/components/ui/primitives.tsx — YENİ kalıp
export function Switch(props: {
  checked: boolean;
  onChange: (v: boolean) => void;
  disabled?: boolean;
  /** Ekran okuyucu için; satırda görünen etiket ayarın adıdır. */
  label: string;
}): React.ReactElement;
```

```tsx
// frontend/src/components/settings/RuntimeSettings.tsx — genişletme
export function RuntimeSettings(props?: {
  groups?: string[];
  showHeadings?: boolean;
  /** Verilirse yalnızca eşleşen ayarlar çizilir; boşsa süzgeç yok. */
  query?: string;
}): React.ReactElement | null;
```

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
| --- | --- |
| `frontend/src/components/ui/primitives.tsx` | yeni — `Switch` kalıbı (H1) |
| `frontend/src/components/settings/setting-search.ts` | yeni — eşleştirme mantığı (H2) |
| `frontend/src/components/settings/setting-search.test.ts` | yeni — eşleştirme testleri (H2) |
| `frontend/src/components/settings/RuntimeSettings.tsx` | düzenleme — `bool` dalı (H1), `query` süzgeci ve eşleşme yok durumu (H2) |
| `frontend/src/app/settings/page.tsx` | düzenleme — araç çubuğu, arama alanı, sorgu varken görünüm değişimi (H2) |

Backend'de değişen dosya yok.

## Yeniden Kullanılacak Mevcut Kod

Bu işin büyük kısmı **zaten yazılmış**; yeni yazılan yalnızca eksik olan iki
parça.

- `frontend/src/components/settings/RuntimeSettings.tsx` — `SettingRow`
  bileşeni taslak durumu, kaydetme, sıfırlama, "değiştirilmiş" rozeti,
  "Kaydedildi" geri bildirimi, hata gösterimi ve üstveri satırını **zaten**
  yapıyor. İki durumlu ayar için yalnızca denetimin çizildiği yer dallanıyor;
  kaydetme yolu ortak kalıyor.
- `frontend/src/components/settings/RuntimeSettings.tsx` — bileşen ayar
  listesini kayıt defterinden alıyor ve hangi ayarların var olduğunu bilmiyor.
  H1'in "davranış tipten gelir" kriteri bu sayede kendiliğinden sağlanıyor.
- `Toolbar` ve `SearchField` (`primitives.tsx`) — araç çubuğu ve büyüteçli
  arama alanı hazır. `SearchField`'in `hint` özelliği **kullanılmayacak**:
  kendi belgesi "kısayol gerçekten bağlıysa verilir" diyor, bu işte kısayol
  bağlanmıyor.
- `EmptyState` (`primitives.tsx`) — "eşleşme yok" durumu için.
- `Badge` (`primitives.tsx`) — anahtarın yanındaki "Açık"/"Kapalı" metni
  için değerlendirilecek; sade bir metin yeterliyse rozet kullanılmaz.
- `buttonVariants.danger`, `PanelCard`, bölüm düzeni, `SettingsNav` —
  **hiç dokunulmuyor** (spec: Kapsam dışı).
- Test kalıbı: `frontend/src/lib/*.test.ts` — saf fonksiyonların `node --test`
  ile sınanması. Yeni test aynı kalıbı izler.

---

## Riskler

| Risk | Etki | Önlem |
| --- | --- | --- |
| Anahtar tıklanınca hiçbir şey olmuyor sanılması (değer açık kaydetme bekliyor) | Kullanıcı ayarı değiştirdiğini sanıp kaydetmeden çıkar | Satır zaten değişiklikte "Kaydet" düğmesini çıkarıyor; anahtarın yanındaki durum metni taslağı gösteriyor. Tarayıcıda ayrıca doğrulanacak |
| Türkçe büyük/küçük harf: "ı/I" ve "i/İ" katlaması | "Çalışma" araması "ÇALIŞMA"yı bulamaz | Katlama `toLocaleLowerCase("tr")` ile; saf fonksiyonda birim testle sınanır |
| `query` parametresi mevcut altı çağrı yerinin davranışını değiştirmek | Bölümler bozulur | Parametre opsiyonel; verilmediğinde bugünkü kod yolu birebir aynı. Altı bölüm elle gözden geçirilir |
| Sorgu varken bölüm sekmesine tıklanması | Sekme seçili görünür ama içeriği gösterilmez | Bölüm seçimi sorguyu temizler — gezinmeye dönüş jesti |
| Aramanın yalnızca ayarları kapsadığının anlaşılmaması | Kullanıcı sağlayıcı arar, bulamaz, arama bozuk sanır | Sorgu etkinken sonuçların altında kapsam cümlesi gösterilir (spec H2 son kriteri) |
| `Switch`in iki temada sınırının görünmemesi | Denetimin nerede olduğu anlaşılmaz | Sınır `control-line` jetonu (ölçülmüş 3:1); iki temada `getComputedStyle` ile doğrulanır |

## Test Stratejisi

- **Birim (`node --test`):** `setting-search.test.ts`
  - etikette eşleşme, açıklamada eşleşme, ikisinde de yokken eşleşmeme
  - Türkçe katlama: "çalışma" ↔ "ÇALIŞMA", "ışık" ↔ "IŞIK", "iş" ↔ "İŞ"
  - baştaki/sondaki boşluk kırpılır; boş sorgu **süzmez** (hepsi döner)
  - çok kelimeli sorgu davranışı: her kelime ayrı ayrı eşleşmeli
  - ham anahtar (`runner.timeout_minutes`) aranmaz — ekranda görünmeyen bir
    alanda eşleşme, kullanıcıya açıklanamayan sonuç üretir
- **Entegrasyon:** yok — yeni uç ve yeni veri yolu yok.
- **Elle doğrulama (tarayıcıda, iki temada):**
  1. Çalıştırma bölümü → "Motor loglarını sakla" bir anahtar olarak görünüyor,
     metin kutusu yok
  2. Anahtar tıklanınca "Kaydet" beliriyor; kaydetmeden bölüm değiştirip
     dönünce eski değer duruyor
  3. Kaydedince değer değişiyor ve "Kaydedildi" görünüyor
  4. Anahtara sekme ile odaklanılıyor, boşlukla değişiyor, odak halkası
     iki temada da görünüyor
  5. Anahtarın denetimi diğer ayarlarla aynı dikey hizada — `getBoundingClientRect`
     ile ölçülerek
  6. Arama: "süre" yazınca farklı bölümlerden eşleşmeler bölüm başlıklarıyla
     geliyor; sonuç içinden bir ayar kaydedilebiliyor
  7. "zzz" yazınca "eşleşme yok"; arama temizlenince önceki bölüme dönülüyor
  8. Dar masaüstü ve telefon genişliğinde yatay taşma yok
- **Statik:** `npx tsc --noEmit` ve `npx eslint .` temiz.

## Uygulama Sırası

Riskli parça **arama değil, anahtarın kaydetme akışına oturması**: mevcut
satırın taslak/kaydet mantığına yeni bir denetim tipi giriyor. Önce o
doğrulanıyor.

1. **`Switch` kalıbı** — bileşen katmanına eklenir, iki temada ve klavyeyle
   tek başına doğrulanır (H1)
2. **`SettingRow`'da `bool` dalı** — anahtar kaydetme akışına bağlanır;
   tarayıcıda 1-5. adımlar koşulur (H1). Buraya kadar arama hiç yok, yani
   hata çıkarsa sebebi tektir
3. **`setting-search.ts` + testleri** — saf mantık, ekrana bağlanmadan yeşile
   alınır (H2)
4. **`RuntimeSettings`'e `query`** — süzgeç ve "eşleşme yok" durumu; mevcut
   altı çağrı yerinin bozulmadığı gözden geçirilir (H2)
5. **Ayarlar ekranına araç çubuğu ve görünüm değişimi** — arama alanı, sorgu
   varken içerik değişimi, bölüm seçiminin sorguyu temizlemesi (H2)
6. **Tam doğrulama** — iki tema, üç genişlik, hover/focus, boş ve hata
   durumları, statik denetimler
