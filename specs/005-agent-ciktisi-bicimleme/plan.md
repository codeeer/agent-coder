# Plan: Agent Çıktısının Biçimlendirilmiş Gösterimi

- **Spec no:** 005 — [spec.md](spec.md)
- **Durum:** Uygulandı

---

## Karar: kütüphane değil, kendi render'ımız

Değerlendirilen seçenekler:

| Seçenek | Neden seçilmedi / seçildi |
|---------|---------------------------|
| `react-markdown` + `remark-gfm` | Üç-dört paket ve geçişli bağımlılıkları; tablo için ayrı eklenti; stil vermek için yine her eleman elle eşlenir |
| `marked` / `markdown-it` + `dangerouslySetInnerHTML` | **Reddedildi.** Güvenilmeyen metni HTML'e çevirip DOM'a basmak, ayrıca bir arındırıcı (sanitizer) gerektirir; bir bağımlılık daha ve hata payı |
| **Kendi render'ımız → React elemanları** | **Seçildi.** İhtiyaç duyulan sözdizimi dar; HTML hiç üretilmediği için arındırıcıya gerek yok; stil zaten projenin jetonlarıyla veriliyor |

Grafiklerde de aynı karar verilmişti: dar bir ihtiyaç için paket eklemek yerine
yazmak. Sözdizimi genişlerse (mermaid, formül) karar yeniden değerlendirilir.

## Uygulama

`frontend/src/components/markdown/`

| Dosya | İş |
|-------|-----|
| `parse.ts` | Metin → blok listesi (saf fonksiyon, React'tan bağımsız) |
| `inline.tsx` | Satır içi işaretleme → `ReactNode[]` |
| `Markdown.tsx` | Blokları projenin jetonlarıyla çizen bileşen |

### Blok ayrıştırma

Satır tabanlı, tek geçiş. Desteklenenler:

- `#`…`######` başlık
- ```` ``` ```` çitli kod bloğu (dil etiketi opsiyonel, kapanmasa da blok biter)
- `|…|` tablo — ikinci satır `|---|:--:|` ayıracıysa; hizalama kolonu okunur
- `-` / `*` / `+` madde listesi, `1.` numaralı liste (iki seviye girinti)
- `>` alıntı
- `---` / `***` / `___` yatay çizgi
- Geri kalan ardışık satırlar paragraf

**Tanınmayan hiçbir şey atılmaz** — paragraf olarak görünür. Bir çıktının sessizce
kaybolması, biçimlenmemesinden çok daha kötü olurdu.

### Satır içi ayrıştırma

`` `kod` `` · `**kalın**` · `*italik*` / `_italik_` · `~~üstü çizili~~` ·
`[metin](adres)`. Kod, içindeki her şeyi ham bırakır (öncelikli).

Bağlantı adresi **beyaz listeyle** sınırlanır: `http://`, `https://`, `mailto:` ve
göreli yollar. Diğerleri bağlantı değil, düz metin olarak çizilir — `javascript:` bu
yolla etkisiz kalır. Bağlantılar `target="_blank" rel="noopener noreferrer"` taşır.

### Güvenlik

Render **yalnızca React elemanı** üretir; `dangerouslySetInnerHTML` yoktur. Çıktıdaki
`<script>` metni React tarafından kaçırılır ve ekranda metin olarak görünür.

### Yerleşim

- Kod blokları ve tablolar `overflow-x: auto` kendi kabında kayar; sayfa gövdesi
  yatay kaymaz.
- Tablo başlığı, satır ayıracı ve rakam hizalaması rapor tablolarıyla aynı jetonları
  kullanır — iki ekran birbirine benzemeli.

### Ham metin

Çıktı kartının başlığında **Biçimli / Ham metin** anahtarı. Ham görünüm
`whitespace-pre-wrap` ile bugünkü davranışı korur; kopyala-yapıştır ihtiyacı için.

## Riskler

| Risk | Önlem |
|------|-------|
| Ayrıştırıcı bir sözdizimini kaçırır | Tanınmayan satır paragraf olur; metin kaybolmaz + "Ham metin" her zaman erişilebilir |
| Güvenilmeyen çıktıdan XSS | HTML hiç üretilmiyor; bağlantı şeması beyaz listede |
| Çok uzun çıktı sayfayı yavaşlatır | Ayrıştırma tek geçiş ve `useMemo` ile çıktı değişmedikçe tekrarlanmıyor |

## Doğrulama

1. Birim testler: her blok türü, tanınmayan girdi, `javascript:` bağlantısı,
   kapanmamış kod bloğu → `make test-frontend`
2. Ekranda: ekran görüntüsündeki gerçek agent çıktısı (başlık + tablo + liste + satır
   içi kod) doğru görünüyor
3. Ham metin anahtarı bugünkü davranışı veriyor
