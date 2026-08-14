# Plan: Projeler ekranının yoğunlaştırılması

- **Spec no:** 019 — [spec.md](spec.md)
- **Tarih:** 2026-08-14
- **Durum:** Taslak

---

## Yaklaşım

**Yeni bir kalıp icat edilmiyor — ürünün kendi yoğun liste kalıbı kullanılıyor.**

Çalıştırmalar ekranı bu soruyu zaten çözmüş: `Card padded={false}` içinde
yatay kaydırılabilir gerçek bir `<table>`, `divide-y divide-line` satırlar,
üstveri sütunları sağa hizalı, eylemler `RowAction` ile hover ve odakta
beliriyor. Projeler ekranı bugün ızgara içinde kart kullanıyor ve tek başına
farklı davranıyor.

Aynı kalıba geçmek üç şeyi birden veriyor: satır yüksekliği düşüyor, sütunlar
hizalandığı için **karşılaştırma mümkün oluyor** (bugün kartlarda imkânsız), ve
iki liste ekranı aynı dili konuşmaya başlıyor.

**Hiçbir bilgi kaldırılmıyor** (spec: Kapsam dışı). Bugünkü beş yatay bant,
sütunlara dağılıyor:

```text
bugün (kart, 227px)                    yarın (satır)
─────────────────────                  ────────────────────────────────────
① ikon + ad + depo + eylemler          PROJE      ad + depo (iki satır)
② branch · sunucu · kimlik rozetleri   ERİŞİM     branch · sunucu · kimlik
③ akış adları (3 + fazlası)            AKIŞLAR    adlar (2 + fazlası)
④ 3 ölçü kutusu (çalıştırma/başarı/    KULLANIM   tek satır sessiz üstveri
   maliyet)
⑤ son çalışma durumu + zaman           SON ÇALIŞMA  durum + göreli zaman
                                       (başlıksız)  eylemler — RowAction
```

Sayfa boyutu ekrana uydurulacak. Ölçüm sonrası kesinleşecek ama satır ~56px
olursa 665px'lik alanda ~11 satır görünür; sayfa boyutu **12** civarında
oturur (bugün 24, yani ekrana sığanın dört katı).

## Değerlendirilen alternatifler

| Alternatif | Artı | Eksi | Karar |
| --- | --- | --- | --- |
| Çalıştırmalar ekranının tablo kalıbı | Ürünün kendi kalıbı; sütun hizası karşılaştırmayı mümkün kılıyor; iki ekran aynı dili konuşuyor | Dar ekranda yatay kaydırma gerekir (Çalıştırmalar'da da öyle) | **Seçildi** |
| Kartı koru, yalnızca `PAGE_SIZE`'ı düşür | Tek satırlık değişiklik | Asıl sorun 227px'lik kart; 6 kart görünmeye devam eder, karşılaştırma yine imkânsız. Spec H1'in "en az iki katı" ölçütü karşılanmaz | Elendi |
| Kartı koru, içeriğini kısalt | Izgara düzeni korunur | Bilgi kaldırmak gerekirdi — spec bunu açıkça yasakladı | Elendi |
| `List` primitifiyle serbest satır | Tabloya göre esnek | Sütun hizası yok; asıl kazanç olan karşılaştırma kaybolur | Elendi |
| Yeni bir `DataTable` bileşeni | İki ekran ortaklaşır | Çalıştırmalar ekranı çalışıyor; ortak bileşen çıkarmak onu da değiştirmek demek ve bu spec'in kapsamı dışında. Kalıp izlenir, soyutlama ertelenir | Elendi (bkz. Riskler) |
| Sanal kaydırma (virtualization) | Binlerce satır | Sayfalama zaten var ve çalışıyor; yeni bağımlılık | Elendi |

---

## Veri Modeli

**Değişiklik yok.** Yeni tablo, kolon, migration veya API ucu gerekmiyor. Ekran
bugün hangi verileri çekiyorsa aynılarını çekmeye devam ediyor: projeler
(sayfalı), git sağlayıcılar, kullanım özeti, son çalıştırmalar, akışlar.

Geri alma: commit'i geri almaktan ibaret.

## Arayüzler

### Go tipleri ve HTTP API

**Değişiklik yok.** Backend'e hiç dokunulmuyor.

### Frontend

```tsx
// app/projects/page.tsx — ProjectCard yerine ProjectRow
//
// Aynı props, aynı davranış; değişen yalnızca çizim.
function ProjectRow(props: {
  project: Project;
  provider?: GitProvider;
  usage?: ReportGroup;
  lastRun?: Run;
  workflows: Workflow[];
  onEdit: () => void;
}): React.ReactElement;
```

```tsx
// PAGE_SIZE: 24 → ölçümle belirlenecek (~12)
```

Yeni bileşen **eklenmiyor**; `primitives.tsx`'e dokunulmuyor. Gerekli her kalıp
zaten orada: `Card`, `Badge`, `RowAction`, `Button`, `ConfirmStrip`,
`EmptyState`, `Metric`, `formatRelative`.

---

## Değişecek Dosyalar

| Dosya | Değişiklik |
| --- | --- |
| `frontend/src/app/projects/page.tsx` | `ProjectCard` → `ProjectRow`; ızgara → tablo; `PAGE_SIZE` |

Tek dosya. Backend, `primitives.tsx` ve diğer ekranlar değişmiyor.

## Yeniden Kullanılacak Mevcut Kod

Bu işin neredeyse tamamı **yeniden kullanım**; yazılan yeni mantık yok.

- `app/runs/page.tsx` — tablo kalıbının kaynağı: `Card padded={false}`,
  `overflow-x-auto`, `min-w-*`, `thead` şeridi (`bg-raised/60`, `text-2xs`,
  `uppercase`), `tbody` `divide-y`, sağa hizalı üstveri sütunları, başlıksız
  eylem sütunu. **Kopyalanmıyor, izleniyor.**
- `RowAction` (`primitives.tsx`) — hover ve odakta beliren satır eylemi. Kartın
  bugün elle yazdığı `sm:opacity-0 sm:group-hover:opacity-100` zinciri bu
  bileşende zaten var; elle yazılan hal `RowAction`'ın belgelediği hatayı
  (disabled düğmede saydamlığın tersine dönmesi) tekrar etme riski taşıyor.
- `ConfirmStrip` (`primitives.tsx`) — silme onayı. Kart bugün onayı elle
  çiziyor; bileşen aynı işi yapıyor ve "ne kaybedilecek" alanını zorunlu
  kılıyor.
- `Badge`, `Button`, `IconTile`, `toneFromKey`, `Metric`, `formatRelative`,
  `formatCount`, `formatMoney`, `formatPercent`, `repoLabel` — hepsi olduğu
  gibi kalıyor.
- `Pagination` (`components/ui/Pagination.tsx`) — davranışı değişmiyor.
- Arama, süzgeç segmentleri ve `useQuery` çağrıları — dokunulmuyor.

## Riskler

| Risk | Etki | Önlem |
| --- | --- | --- |
| Tablo dar ekranda yatay kaydırma gerektirir | Telefonda kullanım zorlaşır | Çalıştırmalar ekranındaki çözümün aynısı: `overflow-x-auto` + `min-w-*`. Üç genişlikte ölçülür; **sayfa gövdesi** yatay kaymamalı, yalnızca tablo kabı |
| Bilgi sütunlara dağılırken bir alan düşer | Spec'in "hiçbir bilgi kaldırılmaz" kuralı ihlal edilir | Uygulamadan sonra bugünkü kartın alanları **tek tek** sayılarak karşılaştırılır; sekiz alanın sekizi de listede |
| Satır yüksekliği tahmin edilir, ölçülmez | "İki katı" iddiası doğrulanmaz | `PAGE_SIZE` ölçümden SONRA konur; görünen satır sayısı `getBoundingClientRect` ile sayılır |
| `RowAction`'a geçerken silme düğmesinin görünürlüğü ters döner | En tehlikeli eylem yalnızca silinemeyen satırlarda görünür | `RowAction`'ın kendi belgesindeki hata; saydamlık düğmeye değil sarmalayıcıya verilir. Hover ve odak iki temada denenir |
| Akış adları sütunu uzun adlarda taşar | Satır yüksekliği kayar | Sütun sabit genişlik + kırpma; ikiden fazlası sayıyla belirtilir (kartta bugün üç) |
| Ortak `DataTable` çıkarma isteği | Kapsam kayması | Bu spec'te soyutlama YOK. Üçüncü bir ekran aynı kalıbı isterse o zaman çıkarılır; iki örnek soyutlama için erken |
| Boş/hata/yükleniyor durumları tabloda unutulur | Ekran boş bir çerçeve gösterir | Üçü de ayrı ayrı tetiklenip görülür (spec: Hata durumları) |

## Test Stratejisi

- **Birim:** yok — saf mantık eklenmiyor, değişen yalnızca çizim.
- **Elle doğrulama (tarayıcıda, iki temada):**
  1. **Sayım:** görünen satır sayısı `getBoundingClientRect` ile ölçülür;
     bugünkü ~6'nın en az iki katı olmalı (H1)
  2. **Bilgi eksiksizliği:** bugünkü kartın sekiz alanı listede tek tek
     doğrulanır (H5)
  3. **Kurulum doğruluğu:** branch, sunucu ve git erişimi satırda görünür;
     doğrulanmamış erişim renkten bağımsız olarak metinle ayırt edilir (H2)
  4. **Sayfalama:** `PAGE_SIZE` dolduğunda denetime ulaşmak için gereken
     kaydırma ölçülür ve bugünküyle karşılaştırılır (H3)
  5. **Eylemler:** düzenle, sil, akışa geçiş; hover **ve** klavye odağı (H4)
  6. **Silme onayı** yerinde açılır ve ne kaybedileceğini söyler
  7. **Durumlar:** boş liste, eşleşmeyen arama, yükleme hatası
  8. **Üç genişlik:** geniş masaüstü, dar masaüstü, telefon — sayfa gövdesinde
     yatay taşma yok
  9. **İki tema ayrı ayrı**; satır hover ve seçili durumları hesaplanmış
     renkle doğrulanır
- **Statik:** `npx tsc --noEmit`, `npx eslint .` temiz.

## Uygulama Sırası

Riskli parça **bilgi kaybı**: sekiz alan sütunlara dağılırken birinin düşmesi
sessizce olur. Bu yüzden önce alanlar sayılıyor, sonra düzen değişiyor.

1. **Bugünkü alanların envanteri** — kartta görünen her alan yazılır; bu liste
   sonraki adımların kontrol listesi olur
2. **`ProjectRow`** — tablo satırı, sekiz alanın hepsiyle. `PAGE_SIZE` henüz
   değişmiyor: tek değişken olsun
3. **Ölçüm** — satır yüksekliği ve görünen satır sayısı; `PAGE_SIZE` bu
   ölçüme göre konur (H1, H3)
4. **Eylemler ve onay** — `RowAction` ve `ConfirmStrip`'e geçiş (H4)
5. **Durumlar** — boş, eşleşme yok, hata
6. **Tam doğrulama** — iki tema, üç genişlik, hover/focus, statik denetimler
