# Spec dizini

**Bir konunun tek bir spec'i vardır.** Bir karar sonradan değişirse yeni bir
klasör açılmaz; o spec'in içindeki **Karar geçmişi** bölümüne yazılır.

Bu kural bir hatadan doğdu: rapor ekranının iki spec'i olmuştu (`004-rapor` ve
`012-rapor-yonetici`) ve hangisinin geçerli olduğu ancak ikisini de okuyunca
anlaşılıyordu. Birleştirildi.

| # | Konu | Kapsadığı | Durum |
|---|------|-----------|-------|
| [001](001-veri-katmani-ve-model-katalogu/spec.md) | Veri katmanı ve model kataloğu | şema, migration, şifreleme, OpenRouter kataloğu | Uygulandı · **kısmen 002 ile değişti** |
| [002](002-coklu-saglayici/spec.md) | Çoklu LLM ve git sağlayıcı | birden fazla sağlayıcı, şifreli kimlik bilgileri | Uygulandı |
| [003](003-agent-calistirma/spec.md) | Projeler, agent tanımları, çalıştırma | sandbox, runner arayüzü, canlı olay akışı | Uygulandı |
| [004](004-rapor/spec.md) | **Rapor ekranı** | `/reports` sayfasının tamamı — eski 012 buraya katıldı | Uygulandı |
| [005](005-agent-ciktisi-bicimleme/spec.md) | Agent çıktısının gösterimi | markdown ayrıştırma ve çizim | Uygulandı |
| [006](006-tema-secimi/spec.md) | Tema seçimi | sistem/açık/koyu, renk jetonları | Uygulandı · **değerleri 010 ile ölçülüp düzeltildi** |
| [007](007-workflow-motoru/spec.md) | Akış motoru | graf modeli, seviyeler, paralellik, şablon | Uygulandı |
| [008](008-tuval-editoru/spec.md) | Tuval editörü | akışı çizerek kurma, canlı izleme | Uygulandı |
| [009](009-jira-ve-depo-dugumleri/spec.md) | Jira ve kod deposu düğümleri | PR açma, Jira yorumu, Jira tetikleyici | Uygulandı |
| [010](010-arayuz-denetimi/spec.md) | Arayüz denetimi | mobil kabuk, açılış ekranı, sayfalama, tema eşliği | Uygulandı |
| [011](011-mcp/spec.md) | MCP desteği | agent'lara dış araçlar, `mcp.call` düğümü, MCP sunucusu | Uygulandı |

## Hangi spec'e bakmalı?

| Soru | Spec |
|---|---|
| Bir ekranın **bugünkü** kuralları ne? | o konunun spec'i — `spec.md` |
| Neden böyle yapılmış? | aynı spec'in **Kararlar** bölümü |
| Neden değişmiş? | aynı spec'in **Karar geçmişi** bölümü |
| Nasıl yapılmış? | `plan.md` |
| Yapılırken ne öğrenilmiş? | `tasks.md` → **Ölçüm** notları |

`tasks.md` dosyalarının sonundaki **Ölçüm** notları bu deponun en değerli
kısmıdır: geliştirme sırasında yapılan yanlışlar, nasıl bulundukları ve kök
nedenleri. Bir şeyin neden öyle yazıldığını merak ediyorsanız cevap çoğu zaman
oradadır.

## Yeni spec ne zaman açılır?

**Yeni bir konu** olduğunda. Var olan bir ekranın veya özelliğin davranışı
değişiyorsa yeni klasör değil, o spec'in içine bir karar ve **Karar geçmişi**
kaydı.

Şablon: [`000-template/`](000-template/)
