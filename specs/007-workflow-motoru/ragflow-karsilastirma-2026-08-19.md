# RAGFlow karşılaştırması — kanvas ve motor

- **Tarih:** 2026-08-19
- **Karşılaştırılan sürüm:** infiniflow/ragflow `ec9c08d` (v0.27.0)
- **Kapsam:** yalnızca karar belgesi. Kod değiştirilmedi, spec metni düzenlenmedi.
- **İlgili spec'ler:** [007 motor](spec.md), [008 tuval](../008-tuval-editoru/spec.md)

> RAGFlow bir RAG/agent ürünü, biz kod ajanı orkestratörüyüz. "Onlarda var" tek
> başına gerekçe sayılmadı; her madde **bakım kampanyası senaryosuna** göre
> ölçüldü (50+ projede Node/Spring/WildFly upgrade, Jira tetikli koşular,
> toplu çalıştırma [023](../023-toplu-calistirma/spec.md), script klasörleri
> [022](../022-script-klasorleri/spec.md)).
>
> RAGFlow kodu alıntılanmadı; tespitler `dosya:satır` ile verildi.

---

## 1. Özet — en önemli üç karar

1. **Düğüm başına hata politikası alınacak (spec 007 K2 kısmen geri alınıyor).**
   Bugün bir adım düşünce akış tamamen duruyor. RAGFlow'da her düğüm kendi
   politikasını taşıyor: dur / varsayılan değerle devam / başka dala atla
   (`agent/component/base.py:51-53`, `agent/canvas.py:798-808`). Kampanya
   senaryosunda 20 dakikalık agent emeğinin sonundaki "Jira yorumu düşemedi"
   hatasının bütün koşuyu çöpe atması kabul edilemez.
2. **Koşullu dal alınacak (spec 007 K3 geri alınıyor).** K3'ün gerekçesi
   "gerçek ihtiyacı görelim"di. İhtiyaç kendi spec'imizde yazılı:
   [022 spec.md:16-17](../022-script-klasorleri/spec.md) "bir adımdan sonra
   derleme kırılabiliyor, agent onu düzeltip devam etmeli." RAGFlow'un Switch
   **şeması** alınacak (`agent/component/switch.py:33-49`), **çalışma zamanı
   alınmayacak** — onların yürütücüsü `path` listesine ekleme yapıyor, bizim
   seviye tabanlı yürütmemizle uyuşmuyor.
3. **Iteration/Loop düğümü ALINMAYACAK.** Bizde tekrarın yeri düğüm değil,
   [023 toplu çalıştırma](../023-toplu-calistirma/spec.md): kampanya "50 proje
   üzerinde döngü" olarak değil, "50 çalışma" olarak modelleniyor ve her biri
   ayrı container, ayrı branch, ayrı PR üretiyor. Grafın içine ikinci bir
   tekrar mekanizması koymak aynı işi iki yerde ifade etmek olurdu.

Ek olarak ölçülen iki şey beklentiyi bozdu: **RAGFlow'da auto-layout ve minimap
yok** (bizde auto-layout var), ve **çözülemeyen değişken referansı sessizce boş
string'e dönüyor** (`agent/canvas.py:227-228`) — bu bizim spec 007 kabul
kriterimizin açıkça yasakladığı davranış.

---

## 2. Kanvas (editör) tablosu

| Konu | RAGFlow | agent-coder | Karar | Etki | Zorluk |
|---|---|---|---|---|---|
| Undo/redo | Var. Elle yazılmış `HistoryManager`, 50 kayıtlık halka, Ctrl/Cmd+Z ve Shift+Z (`use-agent-history-manager.ts:5-100`, `:126-158`) | Yok — [spec 008 kapsam dışı](../008-tuval-editoru/spec.md) | **Uyarlayalım** | Orta | Düşük |
| Undo geçmişine yazma sıklığı | Her `nodes/edges` değişiminde `push`, karşılaştırma tam graf `JSON.stringify` ile (`use-agent-history-manager.ts:22-26`, `:121`) | — | **Almayalım (bu hâliyle)** | — | — |
| Copy/paste | Var ama **kenarlar taşınmıyor**: yalnızca `selected` düğümler serileştiriliyor (`hooks.tsx:237`), yapıştırma düğüm başına `duplicateNode` çağırıyor (`:254`) | Yok | **Uyarlayalım (kenarlarla)** | Orta | Orta |
| Copy/paste sekmeler arası | Çalışır — `clipboardData` üzerinde `agent:nodes` MIME'ı (`hooks.tsx:240`) | — | **Alalım (desen)** | Düşük | Düşük |
| Kopyalamanın metin girişini gasp etmesi | Korunmuş: `srcElement.tagName !== 'BODY'` ise çıkılıyor (`hooks.tsx:233`) | — | **Alalım (desen)** | Düşük | Düşük |
| Çoklu seçim | Var (`store.ts:149-150`, `onSelectionChange`) | Yok (tek seçim, dışarıdan `selectedId`) — [FlowCanvas.tsx:119-124](../../frontend/src/components/flow/FlowCanvas.tsx#L119-L124) | **Uyarlayalım** | Orta | Orta |
| Backspace güvenliği (input odaklıyken) | Elle yazılmış kısayollarda ayrı guard var (`use-agent-history-manager.ts:129-135`) | **Sorun yok — ölçüldü.** xyflow 12.11.2 `useKeyPress` içinde `isInputDOMNode` guard'ı zaten uyguluyor (`@xyflow/system/dist/esm/index.js:859-865`, `@xyflow/react/dist/esm/index.js:426-429`) | **Almayalım (gereksiz)** | — | — |
| Silmede zincir koruması | `onBeforeDelete` ile: Begin silinemiyor, iteration silinince içi de siliniyor, agent silinince alt araçları (`canvas/index.tsx:346`, `hooks/use-before-delete.tsx:22-77`) | Yok — silinen düğümün kenarları xyflow tarafından temizleniyor, başka koruma yok | **Uyarlayalım (dar)** | Düşük | Düşük |
| Auto-layout | **Yok.** `package.json`'da dagre/elkjs/d3-hierarchy yok | **Var** — [flow-layout.ts:80](../../frontend/src/lib/flow-layout.ts#L80), yalnızca konumsuz eski akışlarda ([page.tsx:101](../../frontend/src/app/workflows/%5Bid%5D/page.tsx#L101)) ve izleme ekranında ([runs page:269](../../frontend/src/app/workflows/%5Bid%5D/runs/%5BrunId%5D/page.tsx#L269)) | **Bizde kalsın** | — | — |
| Minimap | Yok (grep: `MiniMap` eşleşmiyor) | Yok — [FlowCanvas.tsx:208-209](../../frontend/src/components/flow/FlowCanvas.tsx#L208-L209) yalnızca `Background` + `Controls` | **İzleyelim** | Düşük | Düşük |
| fitView | `fitView` prop'u (`canvas/index.tsx:323`) | Var, ayrıca `FIT_MAX_ZOOM` ve ekleme sonrası sığdırma sinyali ([FlowCanvas.tsx:34](../../frontend/src/components/flow/FlowCanvas.tsx#L34), `:42-57`) | **Bizde kalsın** | — | — |
| Büyük grafik performansı | `onlyRenderVisibleElements` kullanılmıyor (grep: eşleşme yok); store zustand, sürükleme her frame store'a yazıyor | Sürükleme her `NodeChange`'de ebeveyne çıkıyor ([FlowCanvas.tsx:126-135](../../frontend/src/components/flow/FlowCanvas.tsx#L126-L135)) ama `dimensions`/`select` "anlamlı değil" diye ayrılmış (`:70-72`) | **İzleyelim** | Düşük | — |
| Grup / alt graf düğümü | Var: iteration ve loop `parentId` + `extent:'parent'` ile kapsayıcı, içeride otomatik bir "start" düğümü (`hooks/use-add-node.ts:74-96`, `:108`); bağ çekme aynı ebeveyn içinde kısıtlı (`hooks.tsx:131-142`) | Yok; koşullu dal ve döngü hiç çizilmiyor | **Almayalım (grup düğümü), koşullu dalı düz kenarla çizelim** | Yüksek | Yüksek |
| Düğüm formu | Sağ sheet (`canvas/index.tsx:397-409`, `form-sheet/`) | Sağ panel — [NodeInspector.tsx](../../frontend/src/components/flow/NodeInspector.tsx) | **Aynı; değişiklik yok** | — | — |
| Değişken referansı — sözdizimi | `{cpn_id@alan}`, ayrıca `{sys.*}` ve `{env.*}` (`agent/component/base.py:363`) | `{{ steps.<id>.output }}`, `{{ input }}`, `{{ trigger.* }}` | **Bizde kalsın** | — | — |
| Değişken referansı — seçim yardımı | Serbest; form alanlarında referans seçici | **Daha iyi:** yalnızca **ata** düğümler öneriliyor ([NodeInspector.tsx:107](../../frontend/src/components/flow/NodeInspector.tsx#L107), `:217-224`) | **Bizde kalsın** | — | — |
| Kaydetme modeli | **Hibrit:** 20 sn debounce ile autosave (`hooks/use-save-graph.ts:96-102`), ayrıca açık `release` bayrağı (`:32`, `:40-41`) ve sürüm kayıtları (`api/apps/restful_apis/agent_api.py:1116`, `:1131`) | Açık "Kaydet" düğmesi + her kayıtta yeni sürüm; `dirty` bayrağı, kirliyken başlatma kapalı ([page.tsx:105-113](../../frontend/src/app/workflows/%5Bid%5D/page.tsx#L105-L113), `:199`) | **Bizde kalsın** (gerekçe §6) | — | — |
| Sürüme geri dönme | **Yok** — sürüm listeleme/okuma var, geri yükleme ucu yok (`agent_api.py:1116-1140`) | Yok — spec 007 kapsam dışı | **İkisinde de yok; izleyelim** | Düşük | Orta |
| Kaydetmeden önce graf doğrulama | Yalnızca **bağ** doğrulaması (kendine bağ, döngü, aynı ebeveyn — `hooks.tsx:128-173`). Dangling edge / erişilemeyen düğüm / başlangıçsız graf kontrolü **yok** | **Daha güçlü:** tam graf doğrulaması, kusurlar düğüm başına dönüyor ([graph.go:174-338](../../backend/internal/workflow/graph.go#L174-L338)) ve tuvalde ilgili düğümde gösteriliyor ([page.tsx:167-188](../../frontend/src/app/workflows/%5Bid%5D/page.tsx#L167-L188)) | **Bizde kalsın** | — | — |
| Doğrulamanın çalıştığı an | Anlık (bağ çekerken) | Kaydetmede, sunucu turuyla | **İzleyelim** | Düşük | Orta |
| Çalışırken görselleştirme | Düğüm üstünde "başladı ama bitmedi" işareti (`hooks/use-node-loading.ts:78-84`, `node-wrapper.tsx:9-20`) + ayrı log sheet (`log-sheet/`) | Aynı tuval readOnly modda + adım kartları + SSE ([runs page:283](../../frontend/src/app/workflows/%5Bid%5D/runs/%5BrunId%5D/page.tsx#L283), `:373-376`) | **Bizde kalsın** | — | — |

---

## 3. Motor (çalıştırma) tablosu

| Konu | RAGFlow | agent-coder | Karar | Etki | Zorluk |
|---|---|---|---|---|---|
| Şema sözleşmesi | Frontend DSL üretiyor (`hooks/use-build-dsl.ts`, `utils/dsl-bridge`), backend aynı DSL'i `Canvas` ile yüklüyor; ayrı bir şema doğrulayıcı yok. Ayrıca `agent/dsl_migration.py` ile taşıma | Tek şema: [workflow-graph.ts:125-142](../../frontend/src/lib/workflow-graph.ts#L125-L142) ↔ [graph.go:95-113](../../backend/internal/workflow/graph.go#L95-L113); doğrulama tek yerde, backend'de | **Bizde kalsın** | — | — |
| Şema sürümleme | `dsl_migration.py` var | Yok; `NodeKind` bilerek açık uçlu metin ([graph.go:14-18](../../backend/internal/workflow/graph.go#L14-L18)) | **İzleyelim** | Düşük | Orta |
| Koşul (switch) | Var. Koşul listesi, operatör kümesi (`contains`, `=`, `>` …), koşul başına hedef, ayrıca zorunlu ELSE hedefi (`agent/component/switch.py:33-49`) | **Yok** — spec 007 K3 | **Uyarlayalım** | Yüksek | Yüksek |
| Koşul (categorize) | Var — LLM ile sınıflandırıp dala ayırma (`agent/component/categorize.py`) | Yok | **Almayalım** | — | — |
| Döngü | Iteration (dizi üzerinde) ve Loop + ExitLoop (`agent/component/iteration.py`, `loop.py`, `exit_loop.py`) | Yok | **Almayalım** — bizde karşılığı [023 toplu çalıştırma](../023-toplu-calistirma/spec.md) | — | — |
| Paralel dal | `_run_batch` ile aynı turdaki düğümler birlikte (`agent/canvas.py:560`, `:663`) | Topolojik **seviye** hesabı, aynı seviye eşzamanlı ([graph.go:359-401](../../backend/internal/workflow/graph.go#L359-L401), [executor.go:163-208](../../backend/internal/workflow/executor.go#L163-L208)) | **Bizde kalsın** | — | — |
| Birleşim (join) | Örtük — `path` üzerinden | Örtük — seviye hesabının doğal sonucu | **Aynı** | — | — |
| Hata semantiği — kapsam | **Düğüm başına.** `exception_method`, `exception_default_value`, `exception_goto` (`base.py:51-53`); çalışma anında: goto varsa dala atla, varsayılan değer varsa devam et, yoksa akışı durdur (`canvas.py:798-808`) | **Akış başına.** İlk hata akışı durduruyor, kalanlar "atlandı" ([executor.go:217-221](../../backend/internal/workflow/executor.go#L217-L221), spec 007 K2) | **Uyarlayalım** | Yüksek | Orta |
| Retry | **Genel değil.** `max_retries`/`delay_after_error` `base.py:49-50`'de tanımlı ama yalnızca `agent/tools/*.py` içinde kullanılıyor (tavily, arxiv, pubmed…); genel `invoke` döngüsünde retry yok (`base.py:403-415`) | Yok — spec 007 kapsam dışı | **Almayalım (agent adımı için)** | — | — |
| Timeout | **Var ve genel.** `@timeout(COMPONENT_EXEC_TIMEOUT, varsayılan 600 sn)` dekoratörü (`base.py:445`); Switch gibi ucuz düğümlerde daha kısa (`switch.py:58`) | **Yok.** Adım süresini yalnızca kullanıcı iptali veya sunucu kapanışı sınırlıyor | **Alalım** | Yüksek | Düşük |
| Kısmi başarı raporlama | Düğüm başına `_ERROR` çıktısı; akış "hata" alanıyla kapanıyor (`canvas.py:808`) | `workflow_steps.status` altı durumlu (`pending/running/succeeded/failed/skipped/cancelled`), akış tek durum | **Uyarlayalım** (§4 H1 ile birlikte) | Orta | Düşük |
| Global değişkenler | Var — `canvas.globals`, `{sys.*}` / `{env.*}` kökleri (`canvas.py:245-248`, `gobal-variable-sheet/`) | Yok; ortak değer her adımın talimatına elle yazılıyor | **Uyarlayalım** | Orta | Orta |
| Adım çıktısının akışı | Bileşen çıktıları canvas üzerinde tutuluyor; referans çözülürken **çözülemeyen referans boş string oluyor** (`canvas.py:227-228`) | Bağlam adım adım birikiyor, her adıma **kopyası** veriliyor ([executor.go:188-191](../../backend/internal/workflow/executor.go#L188-L191)); çözülemeyen referans **kaydetmede reddediliyor** ([graph.go:284-299](../../backend/internal/workflow/graph.go#L284-L299)) | **Bizde kalsın** | — | — |
| Büyük çıktılar (diff/dosya) | Bellekte, DSL'e serileşiyor | Agent çıktısı `runs` kaydında; adım sonucu yalnızca özet taşıyor ([executor.go:275-291](../../backend/internal/workflow/executor.go#L275-L291)) | **Bizde kalsın** | — | — |
| Resume (başarısız adımdan devam) | Var. Çalışılan düğüm listesi `path` DSL'e yazılıyor (`canvas.py:111`, `:127`); `is_resume` (`canvas.py:530`) ve `idx = 0 if is_resume` (`canvas.py:646`) ile kaldığı yerden | **Yok.** Yeniden çalıştırma baştan başlıyor | **Uyarlayalım (dar kapsam)** | Yüksek | Yüksek |
| Gözlemlenebilirlik — süre | Düğüm başına `_elapsed_time` (`base.py:414`) | Adım başına `started_at`/`finished_at` (spec 007 plan, `workflow_steps`) | **Aynı** | — | — |
| Gözlemlenebilirlik — token/maliyet | Koşu düzeyinde toplam (`canvas.py:359`), düğüm başına değil | **Daha iyi:** her agent adımı bir `runs` kaydı, maliyet ve token adım başına (spec 007 plan "adım = çalıştırma") | **Bizde kalsın** | — | — |
| Olay akışı | Generator + SSE, `node_started` / `node_finished` olayları (`canvas.py:651-663`) | SSE, seviye/adım olayları ([executor.go:354-358](../../backend/internal/workflow/executor.go#L354-L358)) | **Aynı** | — | — |

---

## 4. "Hemen" maddeleri

En fazla beş madde; sıralama **bakım kampanyası senaryosundaki kazanca** göre.

### H1 — Düğüm başına hata politikası

**Gerekçe.** Bugün akış ilk hatada tamamen duruyor (spec 007 K2). Kampanya
senaryosunda tipik bir akış "analiz → upgrade → derle → PR aç → Jira'ya yaz"
şeklinde ve son iki adım **dış servise** dokunuyor. Jira'nın o an cevap
vermemesi, 20 dakikalık agent emeğiyle üretilmiş ve zaten branch'e gönderilmiş
bir değişikliği "başarısız koşu" olarak damgalıyor; 50 projelik toplu
çalıştırmada bu, elle ayıklanacak 50 satır demek. K2'nin gerekçesi hâlâ geçerli
— *"yarım kalmış bir akışın ürettiği kod değişikliği güvenilmez"* — ama bu
gerekçe **kod üreten** adımlar için doğru, **bildirim** adımları için değil.
RAGFlow ayrımı düğüme taşımış (`base.py:51-53`); üç seçenekli hâlinin
(`goto`/`default_value`/dur) yalnızca ilk ikisi bize uygun: `goto` koşullu dalın
işi, ayrı madde. Bize gereken iki değerli bir alan: **dur** (varsayılan, bugünkü
davranış) ve **hatayı işaretle, devam et**. K2 kaldırılmıyor, varsayılanı
kalıyor.

**Kapsam.** `backend/internal/workflow/graph.go` (`NodeConfig`'e `onError`
alanı), `kinds.go` (hangi türlerde açılabileceği — agent adımında bilerek
kapalı), `executor.go:163-221` (seviye sonundaki `first != nil` dalı politika
okuyacak), `store.go` + migration (`workflow_steps.status`'a `failed_ignored`
veya `workflow_runs`'a "kısmi başarı" durumu), `NodeInspector.tsx` (kutucuk),
`WorkflowStatusBadge.tsx` + `RunBatchBadges.tsx` (yeni durum). Yeni bağımlılık
yok. **Spec 007'ye etkisi:** K2 kısmen geri alınıyor, karar geçmişine madde.

### H2 — Adım başına timeout

**Gerekçe.** Bugün bir adımın süresini yalnızca kullanıcı iptali veya sunucu
kapanışı sınırlıyor; asılı kalan bir agent container'ı **süresiz** duruyor.
Eşzamanlılık sınırı adım seviyesinde uygulandığı için ([023](../023-toplu-calistirma/spec.md))
asılı tek bir adım, arkasındaki 49 projelik kuyruğu da durduruyor — yani hata
tek koşuda kalmıyor, kampanyanın tamamını kilitliyor. Bu, gecelik/gözetimsiz
koşuların önündeki tek somut engel. RAGFlow'un çözümü basit ve doğru: ortam
değişkeninden gelen genel bir üst sınır, ucuz düğümlerde daha kısası
(`base.py:445`, `switch.py:58`). Retry'ı **almıyoruz** — spec 007'nin gerekçesi
("model çağrıları pahalı ve yan etkili") ayakta ve RAGFlow'da da genel retry
zaten yok, yalnızca arama araçlarında var.

**Kapsam.** `backend/internal/workflow/executor.go:253` (handler çağrısını
`context.WithTimeout` ile sarma), ayar için `config` paketi (genel varsayılan) ve
isteğe bağlı düğüm başına aşım (`NodeConfig.TimeoutSeconds`), zaman aşımının
`StepFailed`'dan ayrı bir sebep olarak mesajlaşması. Yeni bağımlılık yok.
**Spec 007'ye etkisi:** kabul kriteri eklenmesi; kapsam dışı listesinde timeout
hiç yazmıyordu, yani karar değil boşluk.

### H3 — Koşullu dal (switch düğümü)

**Gerekçe.** Spec 007 K3 bunu "gerçek ihtiyacı görmek daha sağlıklı" diye
ertelemişti. İhtiyaç görüldü ve kendi spec'imizde yazılı:
[022 spec.md:16-17](../022-script-klasorleri/spec.md) yedi adımlı bir upgrade
kampanyasını anlatırken *"bir adımdan sonra derleme kırılabiliyor, agent onu
düzeltip devam etmeli"* diyor. Bugün bu ancak tek bir agent adımının içine
gömülü talimatla ifade edilebiliyor — yani akışın şekli değil, prompt metni
oluyor; kanvas o kararı göstermiyor ve izleme ekranında hangi yolun seçildiği
okunmuyor. RAGFlow'un **şeması** doğru: koşul listesi, sınırlı bir operatör
kümesi, koşul başına hedef ve **zorunlu ELSE** (`switch.py:33-49`) — zorunlu
else, sessizce hiçbir yere gitmeyen dalı imkânsız kılıyor. **Çalışma zamanını
almıyoruz:** RAGFlow yürütücüsü çalışırken `path` listesine ekleme yapıyor
(`canvas.py:824-826`), bizim topolojik seviye hesabımız ise grafın tamamını
önceden biliyor. Bizim uyarlamamız: seçilmeyen dalın alt ağacı `skipped`
işaretlenir — `SkipPending` zaten bu işi yapıyor.

**Kapsam.** En büyük madde. `graph.go`: yeni `KindSwitch`, `Validate` içinde
"erişilemeyen düğüm" kuralının koşullu dalları hata saymaması (bugün
[graph.go:262-268](../../backend/internal/workflow/graph.go#L262-L268) her
ulaşılamayan düğümü reddediyor), `Levels()`'ın dal seçimiyle birlikte çalışması.
`kinds.go` (koşul alanları + doğrulama), `executor.go` (dal seçimi ve seçilmeyen
alt ağacın atlanması), `handler.go` (koşul değerlendirici — **kendi dilimizi
yazmıyoruz**, RAGFlow gibi sabit operatör listesi). Frontend: `nodes.tsx`
(çok çıkışlı düğüm, `sourceHandle`), `workflow-graph.ts` (`FlowEdge`'e
`sourceHandle`), `flow-layout.ts` (`wouldCreateCycle` handle'a duyarlı olmalı),
`NodeInspector.tsx` (koşul editörü). Yeni bağımlılık yok. **Spec 007'ye etkisi:**
K3 geri alınıyor — bu tek başına ayrı bir spec'i hak edecek büyüklükte.

### H4 — Başarısız adımdan devam (dar kapsamlı resume)

**Gerekçe.** 50 projelik bir kampanyada koşuların bir kısmı hep son adımlarda
düşer (PR açma, Jira). Bugün tek çare baştan başlatmak: agent adımı yeniden
koşar, model parası **ikinci kez** ödenir ve üretilen diff bu sefer farklı
çıkabilir — yani yeniden çalıştırma, düzeltmek istediğiniz koşuyu tekrarlamıyor,
yenisini üretiyor. RAGFlow bunu `path`'i DSL'e yazıp `is_resume` ile çözmüş
(`canvas.py:111`, `:530`, `:646`). Bizde altyapının çoğu **zaten var**:
`workflow_steps` düğüm başına durum ve `run_id` tutuyor, `workflow_runs.version_id`
grafı sabitliyor. Eksik olan, bağlamın yeniden kurulması — ve burada dürüst
sınır şu: **agent adımının çalışma dizini gitmiş olur.** Bu yüzden kapsam dar
tutulmalı: yalnızca `succeeded` adımların sonuçları geri yüklenip
`failed`/`skipped` olanlar yeniden koşturulur; agent adımı yeniden koşacaksa
kullanıcı bunu görerek onaylar. Bu hâliyle bile PR/Jira/MCP hatalarının
tamamını, yani gerçek hataların çoğunu karşılıyor.

**Kapsam.** `workflow/store.go` (bitmiş adımların `StepOutcome`'ını okuma —
bugün agent adımının sonucu `runs`'da, adımda değil: [executor.go:275-280](../../backend/internal/workflow/executor.go#L275-L280)
sonucu bilerek boşaltıyor, bu **değişmeli** ya da `runs`'dan geri okunmalı),
`executor.go` (önceden dolu `Context` ile başlama), `launcher.go` + `handler.go`
(yeni uç: `POST /api/workflow-runs/{id}/resume`), frontend runs sayfası (düğme +
neyin tekrar koşacağının önizlemesi). Yeni bağımlılık yok. **Spec 007'ye etkisi:**
kapsam dışı listesinde yoktu; yeni yetenek, kabul kriteri eklenir.

### H5 — Undo/redo

**Gerekçe.** Senaryo kazancı bu listedeki en düşük madde; buraya **maliyeti
düşük ve spec 008'in en çok sırıtan boşluğu** olduğu için giriyor. 008 bunu
*"kaydedilmemiş değişiklik uyarısı yeterli koruma"* diyerek kapsam dışı
bırakmıştı — ama o uyarı **kaybı önlemiyor, sadece haber veriyor**: yanlışlıkla
silinen bir düğümü geri getirmenin tek yolu kaydetmeden sayfayı yenilemek, yani
o oturumdaki bütün işi atmak. Kampanya akışları büyüdükçe (022 yedi adımdan
söz ediyor) bu bedel büyüyor. RAGFlow'un yaklaşımı alınabilir ama
**uygulaması alınmamalı**: her değişimde tam grafı `JSON.stringify` ile
karşılaştırıp yığına basıyorlar (`use-agent-history-manager.ts:22-26`, `:121`),
yani sürükleme sırasında 50 kayıtlık geçmiş piksel hareketleriyle doluyor ve tek
bir Ctrl+Z bir düğümü bir piksel geri alıyor. Bizim `alters()` ayrımımız
([FlowCanvas.tsx:70-72](../../frontend/src/components/flow/FlowCanvas.tsx#L70-L72))
doğru yeri zaten işaretliyor; geçmişe yazma **sürükleme bitişinde**
(`onNodeDragStop`) ve yapısal değişikliklerde olmalı.

**Kapsam.** Yeni bir `frontend/src/lib/flow-history.ts` (saf, test edilebilir —
`workflow-graph.ts` gibi), [page.tsx](../../frontend/src/app/workflows/%5Bid%5D/page.tsx)
içinde `nodes`/`edges` durumunun sarılması, `FlowCanvas`'a `onNodeDragStop`
eklenmesi, araç çubuğuna iki düğme. Klavye kısayolu için elle listener
**yazılmamalı**; xyflow'un `useKeyPress`'i `isInputDOMNode` guard'ını zaten
uyguluyor (`@xyflow/react/dist/esm/index.js:426-429`). Yeni bağımlılık yok.
**Spec 007'ye etkisi yok; spec 008 kapsam dışı maddesi geri alınıyor.**

---

## 5. "Sonra" maddeleri

| # | Madde | Tek cümle |
|---|---|---|
| S1 | Global/akış değişkenleri (`canvas.py:245-248`) | Kampanyada "hedef Node sürümü" gibi ortak değer bugün her adımın talimatına elle yazılıyor; tek yerde tanımlanıp `{{ vars.* }}` ile okunması 50 projelik bir kampanyayı tek düzenlemeyle Node 22'den 24'e taşırdı — ama H1–H3 bittikten sonra sırası gelir. |
| S2 | Çoklu seçim + kenarlarıyla kopyala-yapıştır | Yedi adımlı bir kampanya akışını ikinciye uyarlarken tüm bloğu kopyalamak elle yeniden çizmekten hızlı; RAGFlow'un kenarsız hâli (`hooks.tsx:237`) örnek değil uyarı. |
| S3 | Silmede zincir koruması (`use-before-delete.tsx`) | Koşullu dal (H3) geldikten sonra anlamlı olur: dal düğümü silinince alt ağacın ne olacağı sorulmalı, sessizce kopuk düğüm bırakılmamalı. |
| S4 | Kanvasta anlık (sunucusuz) doğrulama | Bugün kusurlar ancak "Kaydet"ten sonra görünüyor ([page.tsx:113](../../frontend/src/app/workflows/%5Bid%5D/page.tsx#L113)); doğrulamanın tek doğru kaynağı backend kalmalı, ama ucuz kuralların anında gösterilmesi düzenlemeyi hızlandırır. |
| S5 | Minimap | İkisinde de yok; akışlar 15+ düğüme çıkarsa gerekir, bugün gerekmiyor. |

---

## 6. "Almayalım" maddeleri

- **Iteration / Loop / ExitLoop düğümleri** — tekrarın yeri bizde graf değil,
  [023 toplu çalıştırma](../023-toplu-calistirma/spec.md); aynı işi iki yerde
  ifade etmek olurdu.
- **Grup (parentId + `extent:'parent'`) düğümleri** — Iteration/Loop alınmayınca
  tek kullanıcısı kalmıyor; koşullu dal düz kenarla çizilebilir.
- **Categorize (LLM ile dallanma)** — dalı modele seçtirmek, akışın hangi yoldan
  gittiğini tekrarlanamaz kılar; kampanya senaryosunun ihtiyacı belirlenimci koşul.
- **Agent → tool alt düğümleri** — araç seçimi bizde MCP düğümü ve agent
  yapılandırmasıyla çözülü; tuvale ikinci bir araç hiyerarşisi getirmek gerekmiyor.
- **Genel retry** — spec 007'nin gerekçesi ("model çağrıları pahalı ve yan
  etkili") ayakta; RAGFlow'da da genel retry yok, yalnızca arama araçlarında var
  (`agent/tools/*.py`).
- **20 sn'lik autosave** — çalıştırılabilir tanımı kullanıcı bakmadan
  değiştirmek, kirli akışın yanlışlıkla koşması demek; bizim `dirty` + açık
  "Kaydet" modelimiz ([page.tsx:199](../../frontend/src/app/workflows/%5Bid%5D/page.tsx#L199))
  bunu bilerek engelliyor (RAGFlow bile `release`'i ayrı tutmak zorunda kalmış).
- **Çözülemeyen referansın boş string'e dönmesi** (`canvas.py:227-228`) — spec
  007 kabul kriterinin adıyla yasakladığı şey: "sessizce boş metinle çalışmaz".
- **Her değişiklikte tam graf `JSON.stringify` ile undo geçmişi** — sürükleme
  geçmişi doldurur, Ctrl+Z anlamını yitirir (H5'te ayrıntısı).
- **Elle yazılmış klavye kısayolu listener'ı** — xyflow'un `useKeyPress`'i
  input odak guard'ını zaten sağlıyor; ikinci bir yol ikinci bir hata kaynağı.
- **Backspace koruması eklemek** — ölçüldü, gerek yok: guard xyflow 12.11.2'de var.

---

## 7. Spec'lere önerilen karar-geçmişi maddeleri

### Spec 007 için

> ### 2026-08-19 — hata ve süre kararları düğüme iniyor, koşullu dal geri geliyor
>
> RAGFlow v0.27.0 (`ec9c08d`) motoru okundu ve üç kararımız yeniden tartıldı.
> **K2 kısmen geri alınıyor:** "bir adım düşerse akış tamamen durur" varsayılan
> olarak kalıyor ama düğüm başına "hatayı işaretle, devam et" seçeneği geliyor —
> gerekçesi ("yarım kalmış kod değişikliği güvenilmez") kod üreten adımlar için
> doğru, PR/Jira gibi bildirim adımları için değil; toplu çalıştırmada bu ayrım
> 50 koşunun elle ayıklanmasıyla otomatik raporlanması arasındaki fark. **K3 geri
> alınıyor:** koşullu dalı "gerçek ihtiyacı görelim" diye ertelemiştik, ihtiyaç
> spec 022'de yazılı ("bir adımdan sonra derleme kırılabiliyor"); RAGFlow'un
> Switch **şeması** (sabit operatör kümesi, koşul başına hedef, zorunlu ELSE)
> alınıyor, `path`'e ekleme yapan çalışma zamanı alınmıyor — bizde seçilmeyen
> dalın alt ağacı `skipped` işaretlenecek. Ayrıca kapsam dışı listesinde hiç
> yazmayan iki boşluk kapatılıyor: **adım başına timeout** (bugün asılı bir
> container kuyruğun tamamını kilitliyor) ve **başarısız adımdan devam** (bugün
> yeniden çalıştırma agent'ı ikinci kez ücretlendiriyor ve farklı bir diff
> üretiyor). **Retry alınmıyor** — gerekçesi ayakta, üstelik RAGFlow'da da genel
> retry yok, yalnızca arama araçlarında var. Ölçülen ve bizim lehimize çıkan
> yerler korunuyor: tam graf doğrulaması, referansın yalnızca ataya verilebilmesi
> ve adım başına maliyet.

### Spec 008 için

> ### 2026-08-19 — undo/redo kapsam dışından çıkıyor
>
> RAGFlow kanvası okundu. Kapsam dışı bıraktığımız maddelerden **undo/redo geri
> alınıyor**: "kaydedilmemiş değişiklik uyarısı yeterli koruma" gerekçesi
> yanlıştı — o uyarı kaybı önlemiyor, haber veriyor; yanlışlıkla silinen bir
> düğümü geri almanın tek yolu kaydetmeden sayfayı yenilemek, yani oturumdaki
> bütün işi atmak. RAGFlow'un uygulaması **örnek alınmıyor**: her değişimde tam
> grafı serileştirip yığına basıyorlar, böylece sürükleme geçmişi doldurup
> Ctrl+Z'yi anlamsızlaştırıyor; bizde geçmişe yazma sürükleme bitişinde ve
> yapısal değişikliklerde olacak (`alters()` ayrımı doğru yeri zaten
> işaretliyor). Klavye kısayolu için elle listener yazılmayacak — ölçüldü,
> xyflow 12.11.2 `useKeyPress` input-odak guard'ını zaten uyguluyor; aynı ölçüm
> "input odaklıyken Backspace düğüm siler mi" endişesinin de yersiz olduğunu
> gösterdi. Çoklu seçim ve kenarlarıyla kopyala-yapıştır kapsam dışı kalmaya
> devam ediyor ama "sonra" listesine alındı. Auto-layout ve tam graf doğrulaması
> RAGFlow'da **yok**; bu iki alanda karşılaştırma bizim lehimize çıktı ve
> değiştirilmiyor.

---

## 8. Doğrulama listesi — okunan dosyalar

### agent-coder

| Dosya | Ne için |
|---|---|
| [frontend/src/components/flow/FlowCanvas.tsx](../../frontend/src/components/flow/FlowCanvas.tsx) (214 satır, tamamı) | Kanvas prop'ları, `alters()`, `isValidConnection`, `deleteKeyCode`, minimap/perf yokluğu |
| [frontend/src/lib/workflow-graph.ts](../../frontend/src/lib/workflow-graph.ts) (166 satır, tamamı) | Graf ↔ tuval dönüşümü, `FlowEdge` şeması, `ancestorsOf` |
| [frontend/src/lib/flow-layout.ts](../../frontend/src/lib/flow-layout.ts) (dışa açılan yüzey) | `autoLayout`, `needsLayout`, `wouldCreateCycle` |
| [frontend/src/components/flow/NodeInspector.tsx](../../frontend/src/components/flow/NodeInspector.tsx) (referans yerleri) | Şablon referans yardımı, ata kısıtı |
| [Akış düzenleme ekranı](../../frontend/src/app/workflows/%5Bid%5D/page.tsx) (84-116, doğrulama/kayıt yerleri) | Kaydetme modeli, `dirty`, kusurların düğüme dağıtılması |
| [Akış çalışma izleme ekranı](../../frontend/src/app/workflows/%5Bid%5D/runs/%5BrunId%5D/page.tsx) (izleme yerleri) | Çalışma zamanı görselleştirme, SSE |
| [backend/internal/workflow/graph.go](../../backend/internal/workflow/graph.go) (459 satır, tamamı) | Şema, `Validate`, `Levels`, `ancestors`, PR branch çıkarımı |
| [backend/internal/workflow/executor.go](../../backend/internal/workflow/executor.go) (388 satır, tamamı) | Seviye yürütme, hata/iptal semantiği, bağlam kopyalama |
| [backend/internal/workflow/kinds.go](../../backend/internal/workflow/kinds.go) (140 satır, tamamı) | Tür kayıt defteri, alan doğrulaması, şablon alanları |
| [specs/007-workflow-motoru/spec.md](spec.md), [plan.md](plan.md) | Kararlar, kapsam dışı, veri modeli |
| [specs/008-tuval-editoru/spec.md](../008-tuval-editoru/spec.md) | Kanvas kapsam dışı listesi ve K1–K3 |
| [specs/022-script-klasorleri/spec.md](../022-script-klasorleri/spec.md), [specs/023-toplu-calistirma/spec.md](../023-toplu-calistirma/spec.md) | Kampanya senaryosunun kendi ifadesi |
| `frontend/node_modules/@xyflow/{react,system}/dist/esm/index.js` | Backspace guard iddiasının ölçümü (sürüm 12.11.2) |

### infiniflow/ragflow — `ec9c08d`

| Dosya | Ne için |
|---|---|
| `web/src/pages/agent/canvas/index.tsx` | ReactFlow prop'ları, sheet'ler, `onBeforeDelete`, Controls |
| `web/src/pages/agent/hooks.tsx` | `useValidateConnection`, `useCopyPaste`, `useDuplicateNode` |
| `web/src/pages/agent/use-agent-history-manager.ts` | Undo/redo uygulaması ve klavye guard'ı |
| `web/src/pages/agent/hooks/use-save-graph.ts` | 20 sn autosave, `release`, kaydetme sonrası zaman |
| `web/src/pages/agent/hooks/use-add-node.ts` | Grup düğümleri, `parentId`, `extent:'parent'` |
| `web/src/pages/agent/hooks/use-before-delete.tsx` | Silmede zincir koruması |
| `web/src/pages/agent/hooks/use-build-dsl.ts` | Frontend'in ürettiği DSL |
| `web/src/pages/agent/store.ts` (yüzey), `web/package.json` | Store eylemleri; auto-layout kütüphanesi yokluğu |
| `web/src/pages/agent/hooks/use-node-loading.ts`, `canvas/node/node-wrapper.tsx` | `startButNotFinishedNodeIds` |
| `agent/canvas.py` | Çalıştırma döngüsü, `path`, resume, değişken çözümleme, hata dalı |
| `agent/component/base.py` | `max_retries`/`exception_*` parametreleri, timeout dekoratörü, `invoke` |
| `agent/component/switch.py`, `categorize.py`, `iteration.py`, `loop.py`, `exit_loop.py` | Akış kontrolü bileşenleri |
| `agent/tools/*.py` (grep) | Retry'ın nerede gerçekten kullanıldığı |
| `api/apps/restful_apis/agent_api.py`, `api/db/services/user_canvas_version.py` | Sürüm uçları, release modu, geri dönme yokluğu |
