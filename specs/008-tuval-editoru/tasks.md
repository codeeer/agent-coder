# Görevler: Tuval Editörü

- **Spec no:** 008 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı

---

## Saf mantık (React'tan bağımsız, test edilebilir)

- [x] T01 `lib/workflow-graph.ts` — doğrusal varsayım kalkar; genel graf ↔ tuval
      dönüşümü (`graphToFlow` / `flowToGraph`), `makeStepId` korunur
- [x] T02 `lib/flow-layout.ts` — konumu olmayan düğümleri seviyeye göre
      yerleştirir; konumu olan düğüme DOKUNMAZ
- [x] T03 `flow-layout.ts` içinde döngü tespiti (çizim anında engelleme için)
- [x] T04 Testler: dönüşüm gidiş-dönüş (graf → tuval → graf aynı), otomatik
      yerleşim (dallanan akış üst üste binmiyor), konumu olan düğüm korunuyor,
      döngü tespiti → `npm run test` yeşil

## Tuval

- [x] T10 `@xyflow/react` eklenir; `package-lock.json` güncellenir
- [x] T11 `components/flow/theme.ts` + CSS — React Flow değişkenleri projenin
      jetonlarına bağlanır (açık ve koyu tema)
- [x] T12 `AgentNode` — ad, agent, model, durum; kusur varsa kırmızı çerçeve;
      odaklanabilir
- [x] T13 `TriggerNode` — giriş noktası; gelen bağ kabul etmez
- [x] T14 `FlowCanvas` — düzenleme modu: düğüm ekle/taşı/sil, bağ çek/sil,
      yakınlaştır, kaydır, "hepsini sığdır"
- [x] T15 `isValidConnection` — kendine bağ ve döngü çizim anında engellenir
- [x] T16 `NodeInspector` — sağ panel: ad, agent, model, talimat, şablon
      referansı yardımı (yalnızca ATA düğümler listelenir)

## Ekranlar

- [x] T20 `/workflows/[id]` — adım listesi editörü **kaldırılır**, yerine tuval
      (K2); kaydet → yeni sürüm; kaydedilmemiş değişiklik uyarısı
- [x] T21 Geçersiz graf kusurları düğümlerin üzerinde gösterilir
- [x] T22 `/workflows/[id]/runs/[runId]` — tuval izleme modu; adım listesi
      yanında özet olarak kalır (süre, maliyet, çalıştırma bağlantısı)
- [x] T23 Canlı akış tuvali boyar; sayfa yenilenince durum doğru
- [x] T24 Üç durum (yükleniyor/hata/boş) her iki ekranda

## Doğrulama ve kapanış

- [x] T90 [plan.md](plan.md) doğrulama listesinin sekiz adımı yürütülür
- [x] T91 **Dallanan akış tuvalden kurulup paralel çalıştığı ölçülür** (zaman
      damgaları örtüşür) — bu fazın asıl kabul kriteri
- [x] T92 Görsel doğrulama: tuval açık ve koyu temada (`scripts/screenshot.mjs`)
- [x] T93 `npm run test`, `typecheck`, `lint` temiz; `next build` geçer
- [x] T94 `AGENTS.md` ve `plans/01` güncellenir; `spec.md` "Uygulandı" olur

---

## Sıra ve gerekçesi

Saf mantık (T01–T04) önce: dönüşüm ve yerleşim, tuval açılmadan test edilebilir.
Faz 3'te (spec 005, Ölçüm 1) öğrenilen kural bu — mantık bileşenin içine gömülürse
hatası ancak tarayıcıda görülür.

Backend'e dokunulmuyor: graf modeli Faz 3'te tuval düşünülerek kuruldu.

### T90 — doğrulama listesi sonuçları

| # | Adım | Sonuç |
|---|------|-------|
| 1 | Tuvalden kurulan dallanan akış paralel çalışır | ✓ **10 sn örtüşme** |
| 2 | Konumlar kaydedilir | ✓ (40,40) (320,40) (600,40) |
| 3 | Döngü çizilirken engellenir | ✓ bağ sayısı değişmedi |
| 4 | Kusur ilgili düğümde görünür | ✓ "bu adıma akışın başından ulaşılamıyor" |
| 5 | Çalışma tuvalde izlenir | ✓ adımlar durumlarına göre renkli |
| 6 | Konumsuz eski akış düzgün yerleşir | ✓ üst üste binmedi |
| 7 | Açık ve koyu tema | ✓ ikisi de |
| 8 | Klavyeyle düğüm silme | ✓ |

### Not 1 — "kaydedilmemiş değişiklik" hiçbir şey yapılmadan çıkıyordu

Tuval ilk açıldığında React Flow ölçüm (`dimensions`) ve seçim (`select`)
olayları gönderiyor. Bunları da değişiklik saymak, kullanıcı hiçbir şeye
dokunmadan "kaydedilmemiş değişiklikler var" uyarısının çıkması demekti — ve
uyarı çıktığı için "Akışı çalıştır" düğmesi de kapalı kalıyordu.

Artık `FlowCanvas` değişikliğin kayıtlı veriyi etkileyip etkilemediğini söylüyor
(`meaningful`); yalnızca gerçek düzenleme kirli işaretliyor. Headless tarayıcıda,
hiç etkileşim olmadan yakalandı.

### Not 2 — eklenen adım ekranın dışına düşüyordu

Yeni adım en sağdaki düğümün sağına konuyor ama tuval oraya kaymıyordu:
kullanıcı "Adım ekle" diyor, ekranda hiçbir şey olmuyor sanıyor. Playwright ile
akış kurmaya çalışırken ikinci adım görünmez kaldığı için ortaya çıktı —
bağ da çekilemedi ve kaydetme reddedildi.

Ekleme sonrası tuval yeniden sığdırılıyor (`fitSignal`).

### Not 3 — izleme tuvalinde "agent seçilmedi" yazıyordu

Düzenleme tuvalinde agent adı ekrandaki listeden çözülüyor; izleme modunda o
liste geçilmediği için her düğüm "agent seçilmedi" diyordu — oysa adım
çalışmıştı. Artık agent adı ÇALIŞMA KAYDINDAN okunuyor: agent sonradan silinmiş
olsa bile o çalışmada neyin koştuğu doğru görünür.

Ayrıca izleme modunda etiket yoksa satır hiç yazılmıyor; "seçilmedi" demek
çalışmış bir adım için yanlış olurdu.

### Not 4 — testin kendi doğruluğu

İlk tuval otomasyonum "kaydetme BAŞARILI" dedi, oysa kaydetme reddedilmişti:
yalnızca genel hata kutusuna bakıyordum, kusur ise düğüm panelindeydi. Ekran
görüntüsüne bakınca görüldü.

Kontrol üçe çıkarıldı: genel hata yok + düğüm kusuru yok + düğme "Kayıtlı"
diyor. Bir doğrulama, doğruladığını sandığı şeyi gerçekten doğrulamalı.
