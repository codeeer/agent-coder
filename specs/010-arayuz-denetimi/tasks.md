# Görevler: Arayüz Denetimi

- **Spec no:** 010 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Uygulandı (Aşama 3 dahil)

---

## Aşama 1 — Kullanılabilirlik

- [x] T01 `AppShell` — dar ekranda çekmece, geniş ekranda sabit kenar çubuğu
- [x] T02 `PageHeader` eylem sırası sarmalanır (telefonda "Kaydet" ekran dışındaydı)
- [x] T03 Açılış ekranı yeniden yazılır: kurulum kontrol listesi / son durum
- [x] T04 Terminoloji birleştirilir — her yerde "akış"
- [x] T05 Çalıştırmalar listesine durum süzgeci ve arama
- [x] T06 Akış listesi: kart tıklanabilir, rozetler anlaşılır ("taslak", "duraklatıldı")
- [x] T07 Ayarlar: ham anahtarlar kaldırılır, düğmeler yalnızca gerektiğinde çıkar
- [x] T08 Eskimiş "Faz 5'te kullanılacak" rozetleri silinir
- [x] T09 Agent yetkileri: yalnızca açık olanlar, başarı rengi olmadan
- [x] T10 Rapordaki ham JSON hatası okunur hale getirilir (`lib/failure.ts` + 5 test)
- [x] T11 Etiketsiz alanlara erişilebilir ad (Ayarlar 11 kutu, Modeller 3 denetim)

## Aşama 2 — Tema eşliği

- [x] T20 `scripts/theme-audit.mjs` — iki temayı ölçen denetim aracı
- [x] T21 `--color-control-line` token'ı üç kapsama da eklenir
- [x] T22 Düğme, girdi, açılır liste ve referans çipleri bu token'a geçer
- [x] T23 `ink-3` iki temada da AA'ya çekilir
- [x] T24 Açık tema `info` rengi rozet zemininde AA'ya çekilir
- [x] T25 `danger` düğme kenarı görünür hale gelir
- [x] T26 Disabled düğme okunurluğu
- [x] T27 `ModelPicker` girdisi ortak token'lara hizalanır

## Doğrulama

- [x] T40 `theme-audit` → 346 kontrol, 0 kalan, 0 tema eşliği hatası
- [x] T41 Telefon: yatay taşma 0px, çekmece açılıyor/kapanıyor, düğmeler erişilebilir
- [x] T42 `make lint`, `make test`, 18 birim testi temiz

## Kapanış

- [x] T90 `AGENTS.md` ve `plans/01` güncellenir

---

## Ölçüm 1 — bir temada geçen renk, diğerinde kalıyordu

`--color-ink-3` açık temada kart üzerinde 4,51:1 ile geçiyor, koyu temada üç
zeminde de kalıyordu (3,62–4,25). **On dört ayrı bileşen** etkileniyordu: kenar
çubuğu alt başlığı, zaman damgaları, tablo başlıkları, grafik ekseni, ayar
ipuçları. Hepsi tek bir token satırıydı.

Ters yönde bir örnek de vardı: `info` rengi kendi rozet zemininde açık temada
4,11:1 kalıyor, koyu temada 6,87:1 ile rahat geçiyordu.

Bu, **gözle bulunamayacak** bir hata sınıfıdır: iki temayı aynı anda görmek
mümkün değil ve 4,1 ile 4,6 arasındaki fark bakışla ayırt edilmiyor.

## Ölçüm 2 — süsleme token'ı denetim sınırı olarak kullanılıyordu

| Bileşen | Açık | Koyu | Gerekli |
|---|---|---|---|
| İkincil düğme kenarı | 1,80:1 | 1,63:1 | 3:1 |
| Girdi kenarı | 1,31:1 | 1,26:1 | 3:1 |
| "Sil" düğmesi kenarı | 1,76:1 | 1,85:1 | 3:1 |
| Açılır liste kenarı | 1,31:1 (beyaz üstünde beyaz) | — | 3:1 |

Kök neden, tek tek bileşenler değil: **`line` ve `line-strong` süsleme
ölçekleridir**, ama denetim sınırı diye bir rol tanımlı olmadığı için bileşenler
en yakın olanı ödünç almıştı. Süsleme çizgilerini koyulaştırmak arayüzü
gereksizce ağırlaştırırdı; doğru cevap eksik rolü tanımlamaktı.

## Ölçüm 3 — denetim aracının kendi kör noktası

İlk çalıştırmada araç "0 kalan" dedi. Fazla iyiydi: "Sil" düğmesinin kenarı
**hiç ölçülmemişti**.

Sebep: Tailwind v4 `border-danger/35` ekini `oklab(0.546 0.177 0.061 / 0.35)`
olarak üretiyor; elle yazdığım `rgba(...)` ayrıştırıcısı bunu tanımayıp `null`
dönüyor, ölçüm de o elemanı sessizce atlıyordu. Yani `/25` ve `/35` taşıyan
**bütün rozet ve uyarı kenarları** hiç kontrol edilmemişti.

Düzeltme: rengi tuval (`canvas`) üzerinden tarayıcıya çözdürmek. Tarayıcı hangi
sözdizimini gördüyse onu doğru çevirir.

Ders — bu projede ikinci kez: **açıklanamayan bir "temiz" sonuç, gürültü değil
bulgudur.** İlk seferinde iki sessiz ekran görüntüsü hatasıydı (spec 005);
bu sefer sıfır hata veren bir denetimdi.

## Ölçüm 4 — kalabalığın kaynağı bileşen değil, kararsızlıktı

Ayarlar ekranındaki on bir "Kaydet" düğmesi zaten `disabled` idi; yani ekranda
hiçbir zaman tıklanamayacak on bir düğme duruyordu. Sönük düğme *bilgi* değil
*gürültü*: kullanıcı hangisinin canlı olduğunu ayırt etmek için taramak zorunda.
Artık düğme yalnızca değişiklik varken çiziliyor.

Aynı sınıf: on bir ham ayar anahtarı (`runner.timeout_minutes`), üç yetki
rozetinin ikisinin "kapalı" hali, her satırdaki "Aç" düğmesi. Hiçbiri yanlış
değildi; hiçbiri de gerekli değildi.

## Ölçüm 5 — aynı kutunun iki farklı kenarı

`ModelPicker` girdi stilini ortak `fieldBase`'den almak yerine elle kopyalamıştı.
Sonuç: `control-line` düzeltmesinden payını almadı ve aynı görünmesi gereken iki
kutu iki farklı kenar rengi taşıdı. Görsel tutarsızlık çoğu zaman bir tasarım
kararı değil, **paylaşılan tanımın kopyalanmasıdır**.

## Aşama 3 — Sayfalama ve kalan bulgular

- [x] T50 `internal/paging` — ortak sınır kuralları (3 test)
- [x] T51 `projects`, `agents`, `workflows`, `workflow-runs` listeleri
      `limit`/`offset` alır ve toplam döner
- [x] T52 Tüm liste uçları AYNI zarfı döner: `{items, total, limit, offset}`
- [x] T53 `components/ui/Pagination` — beş liste ekranında ortak bileşen
- [x] T54 Modeller sayfasındaki elle yazılmış sayfalama ortak bileşene geçer
- [x] T55 Model seçici seçili modeli listenin başında gösterir

### Ölçüm 6 — `limit` iki farklı şey demek oluyordu

`/api/runs` yanıtındaki `limit` **eşzamanlılık sınırıydı** (aynı anda kaç iş
çalışabilir), sayfa boyutu değil. Ortak sayfalama bileşenine `data.limit`
geçirince aralık `1–3 / 29` çıkıyordu — üç kayıt gösterilmiyordu, üç iş aynı
anda çalışabiliyordu.

Aynı adın iki anlamı vardı ve ikisi de doğruydu; hata adlandırmadaydı. Yanıt
artık ikisini ayırıyor: `limit`/`offset` sayfa penceresi, `active`/
`concurrencyLimit` çalıştırma kapasitesi.

### Ölçüm 7 — seçili model listede yok

Model seçici arama boşken ilk 60 modeli gösteriyordu. Seçili model —
`anthropic/claude-haiku-4.5` — alfabetik olarak o 60'ın dışında kalıyordu:
kullanıcı listeyi açtığında kendi seçimini göremiyordu.

İlk düzeltmem yalnızca vurguyu seçili satıra taşımaktı; ölçünce işe yaramadığı
görüldü, çünkü satır zaten listede yoktu. Doğru düzeltme, arama boşken seçili
modeli listenin başına koymak.

**Ders:** görünürlük sorununu "vurgula" diye çözmeye çalışmadan önce, öğenin
orada olup olmadığına bak.

### Sayfalama nerede görünür

Aralık **her zaman** yazılır ("2 proje"), ileri/geri düğmeleri yalnızca birden
fazla sayfa varken. Tek sayfalık bir listede iki kapalı ok bilgi değil gürültü;
"2 proje" ise listenin tamamını gördüğünü söyleyen bir bilgi.

## Düzeltilmeyenler

- **Denetim aracı CI'da çalışmıyor.** Elle çalıştırılıyor. **Low.**
