# Spec: Agent Çıktısının Biçimlendirilmiş Gösterimi

- **Spec no:** 005
- **Tarih:** 2026-08-09
- **Durum:** Uygulandı
- **Faz:** 2 — [plans/01](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Agent'lar çıktılarını **Markdown** olarak üretiyor — başlıklar, listeler, tablolar, kod
blokları. Çalıştırma detay sayfası bunu `whitespace-pre-wrap` ile **ham metin** olarak
basıyor. Sonuç okunmuyor:

```
## Özet
Bu depo yalnızca tek bir dosya (`README`) ve tek bir commit içeriyor.
| Öğe | Değer |
|---|---|
| Branch | `master` |
```

Kullanıcı `##`, `|---|---|` ve backtick işaretlerini gözüyle ayıklamak zorunda kalıyor.
Tablolar hizasız, başlıklar diğer metinden ayrışmıyor, kod blokları düz yazıyla aynı.

Bu, ürünün asıl çıktısının sunulduğu yer. Okunmaz olması sonucu değersizleştiriyor.

## Amaç

Agent çıktısı, agent'ın kastettiği biçimde görünsün: başlık başlık, tablo tablo, kod kod.

## Kullanıcı hikâyeleri

1. Kullanıcı olarak, agent'ın **bulgular tablosunu** hizalı bir tablo olarak görmeliyim.
2. Kullanıcı olarak, **başlıkları** metinden ayırt edebilmeliyim ki uzun çıktıda
   gezinebileyim.
3. Kullanıcı olarak, **kod blokları ve dosya adları** tek aralıklı yazıyla görünmeli ki
   düz metinden ayrılsın.
4. Kullanıcı olarak, çıktıyı **ham hâliyle kopyalayabilmeliyim** — başka bir yere
   (PR açıklaması, Jira yorumu) yapıştıracağım.

## Kabul kriterleri

- [x] Başlık, liste, tablo, kod bloğu, satır içi kod, kalın/italik, bağlantı, alıntı ve
      yatay çizgi doğru biçimlenir.
- [x] Biçimlendirme **tanınmayan** bir şey varsa metin kaybolmaz, olduğu gibi görünür.
- [x] Çıktıdaki HTML **çalıştırılmaz**; metin olarak görünür.
- [x] Bağlantılar yalnızca `http`, `https` ve `mailto` şemalarına izin verir.
- [x] Ham metne erişim korunur ("Ham metin" düğmesi).
- [x] Geniş tablo ve uzun kod satırı sayfayı **yatay kaydırmaz**, kendi içinde kayar.

## Kapsam dışı

- **Markdown düzenleme.** Çıktı salt okunur.
- **Görsel, formül, mermaid, dipnot** gibi genişletmeler. Agent çıktısında görülmedi;
  ihtiyaç doğarsa eklenir.
- **Diff gösterimi.** Zaten kendi bileşeni var (`DiffView`), Markdown'dan geçmez.

## Güvenlik notu

Agent çıktısı **güvenilmeyen metindir** — modelin ürettiği, üstelik depodaki içerikten
etkilenebilen bir çıktı. Bu yüzden çıktı hiçbir koşulda HTML olarak yorumlanmaz; render
doğrudan React elemanlarına yapılır ve `dangerouslySetInnerHTML` kullanılmaz. Bağlantı
şemaları beyaz listeyle sınırlanır (`javascript:` reddedilir).
