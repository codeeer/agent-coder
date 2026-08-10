# Spec: MCP Desteği — Agent'ların Dış Araçlara Erişimi

- **Spec no:** 011
- **Tarih:** 2026-08-10
- **Durum:** Onaylandı
- **İlgili plan:** [plans/01-mimari-ve-yol-haritasi-2026-08-09.md](../../plans/01-mimari-ve-yol-haritasi-2026-08-09.md)

---

## Problem

Bir agent adımı bugün yalnızca **klonlanmış depoyla** çalışabiliyor. Şirket
wiki'si, hata takip sistemi, veritabanı şeması, tasarım dosyaları — hiçbirine
erişemiyor.

Sonuç: "şu hatayı düzelt" demek için hatanın yığın izini elle bulup talimata
yapıştırmak gerekiyor. Agent'ın kendisi bakamıyor. Aynı şey veritabanı şeması,
API dokümantasyonu ve ürün kararları için de geçerli.

Her yeni kaynak için ayrı bir entegrasyon yazmak da çözüm değil: Jira ve GitHub
için yazdığımız iki istemci, üçüncü ve dördüncü kaynak geldiğinde aynı işi
tekrar yazmak anlamına gelir.

## Amaç

Agent'ların dış araçlara **standart bir protokolle** erişebilmesi ve bu erişimin
kullanıcı tarafından görülüp sınırlanabilmesi.

## Kullanıcı hikâyeleri

- **Geliştirici olarak** agent'a "bu hatayı düzelt" derken hata detayını elle
  yapıştırmak istemiyorum; agent hata kaydına kendisi baksın.
- **Yönetici olarak** hangi agent'ın hangi dış kaynağa erişebildiğini görmek ve
  değiştirmek istiyorum.
- **Ekip olarak** kurum içi bir bilgi kaynağını, kod yazmadan agent'lara
  açabilmek istiyorum.
- **Akış kuran biri olarak** bir dış aracı agent'ın insafına bırakmadan, akışın
  belirli bir adımında **kesin olarak** çağırabilmek istiyorum.
- **Başka bir aracı kullanan biri olarak** (Claude Desktop, Cursor) buradaki
  akışları oradan tetikleyebilmek istiyorum.

## Kabul kriterleri

### Aşama 1 — agent'lara MCP araçları

- [ ] Ayarlar'dan MCP sunucusu tanımlanabiliyor; erişim bilgisi **şifreli**
      saklanıyor ve bir daha tam haliyle gösterilmiyor.
- [ ] Sunucu **kaydedilmeden önce doğrulanıyor**; hangi araçları sunduğu
      kullanıcıya gösteriliyor.
- [ ] Bir agent'a hangi sunucuların açık olduğu seçilebiliyor.
- [ ] Atanmamış bir sunucunun araçları o agent'a **hiç sunulmuyor**.
- [ ] MCP sunucusuna ulaşılamazsa çalışma **sessizce başarılı görünmüyor**;
      kullanıcı durumu görüyor.
- [ ] Erişim bilgisi hiçbir API yanıtında, hiçbir log satırında ve agent'ın
      okuyabileceği hiçbir dosyada görünmüyor.

### Aşama 2 — akıştan araç çağırma

- [ ] Tuvale "MCP aracı çağır" düğümü eklenebiliyor.
- [ ] Araç, elle yazılarak değil **listeden** seçiliyor.
- [ ] Argümanlar önceki adımların çıktısından şablonla kurulabiliyor.
- [ ] Aracın çıktısı sonraki adımlara geçiyor.

### Aşama 3 — dışarıya açılma

- [ ] Akışlar dışarıdan bir MCP istemcisiyle listelenip başlatılabiliyor.
- [ ] Başlatılan çalışma arayüzde normal bir çalışma gibi görünüyor.

## Kapsam dışı

- **Yerel (stdio) MCP sunucuları.** Komutun runner imajının içinde olmasını
  gerektiriyor; her çalıştırmada paket indirmek yavaş ve ağa bağımlı. Uzak
  sunucular ilginç vakaların hepsini kapsıyor. Sonraya bırakıldı.
- **OAuth ile kimlik doğrulayan MCP sunucuları.** Tarayıcı akışı gerektiriyor;
  başlık tabanlı kimlik doğrulama bu fazda yeterli.
- **Adım düzeyinde MCP geçersiz kılma.** Model adımda değiştirilebiliyor ama MCP
  erişimi bu fazda agent'a bağlı.
- **MCP resources ve prompts.** Yalnızca araçlar (tools).

## Kararlar

**K1 — MCP erişimi agent'a bağlanır, adıma değil.** Yetkiler (dosya değiştirme,
komut çalıştırma, ağ erişimi) bugün agent tanımında duruyor ve MCP erişimi aynı
türden bir yetenek: "bu agent neler yapabilir" sorusunun parçası. Adıma
bağlansaydı aynı agent'ın iki adımda iki farklı yetenek kümesiyle çalışması
gerekirdi ve "reviewer kod değiştiremez" güvencesi adım adım denetlenmek
zorunda kalırdı.

**K2 — Yalnızca uzak sunucular.** Yerel sunucular çalıştırma imajını her yeni
araç için değiştirmek demek; bu, "yeni entegrasyon için kod yazma" sorununu
çözmek yerine yer değiştirmesi olurdu.

*Riski:* bazı popüler MCP sunucuları yalnızca yerel çalışıyor. Kabul edildi;
arayüz bu sınırı açıkça söylüyor.

**K3 — Sunucu kaydedilmeden önce doğrulanır ve araçları gösterilir.** Git
erişimlerinde ve LLM sağlayıcılarında aynı kural var. Burada ek bir sebep daha
var: kullanıcı bir agent'a **neye erişim verdiğini** görmeden karar veremez.

**K4 — Araç kısıtı çalıştırma motorunun kendi yetki modeliyle yapılır.** Ayrı
bir süzgeç yazmak, iki farklı yetki kaynağı demek olurdu; ikisi er geç ayrışır.

**K5 — Erişim bilgisi yapılandırma dosyasına yazılmaz.** Sağlayıcı anahtarında
olduğu gibi ortam değişkenine referans verilir. Agent kendi container'ında bu
dosyayı okuyabilir; okusa bile anahtarı göremez.

## Açık uçlar

- **Klonlanan deponun kendi `.opencode/` yapılandırması.** Çalıştırma motoru
  proje dizinindeki yapılandırmayı da okuyor; bir depo kendi MCP sunucusunu
  tanımlayabilir. Bugün kabul edilebilir — depo zaten kullanıcının kendi
  deposu. Çok kullanıcılı kuruluma geçilirken kapatılmalı.
- **Yetki kuralı sıralaması.** Motorun kurallarında ilk eşleşenin mi son
  eşleşenin mi kazandığı doğrulanmadı. Bu yüzden toptan bir "geri kalan yasak"
  kuralı yazılmadı; erişim yapılandırmayla sınırlanıyor. Sıralama semantiği
  ölçüldüğünde beyaz liste kurulabilir.
