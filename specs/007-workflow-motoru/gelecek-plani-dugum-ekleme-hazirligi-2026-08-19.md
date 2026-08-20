# Gelecek planı: düğüm ekleme hazırlığı

- **Tarih:** 2026-08-19
- **Durum:** **Planlandı — uygulanmadı.** Kod değiştirilmedi.
- **Bağlı olduğu spec:** [007 workflow motoru](spec.md)
- **Tetikleyen belge:** [RAGFlow karşılaştırması](ragflow-karsilastirma-2026-08-19.md)

---

## Neden bu belge var

Akışa iki yetenek eklenmesi konuşuldu: **zamanlanmış tetikleme** ("haftanın şu
günü hep çalışsın") ve **sonucu mail ile gönderme**. İkisi de bakım kampanyası
senaryosunun doğal parçası — 50 projelik bir kampanya gece koşup sabaha rapor
bırakabilmeli, birinin başında durması gerekmemeli.

Soru şuydu: bunlar RAGFlow'daki koda bakılarak mı yazılmalı, ve eklemek kodu
spagettiye çevirir mi?

**Ölçüldü, iki cevap da beklentiden farklı çıktı.**

### RAGFlow'dan öğrenilecek bir şey yok

- **Zamanlanmış tetikleme RAGFlow'da yok.** Agent tarafında cron/scheduler
  araması boş döndü; `schedule` geçen yerler TTS kuyruğu ve doküman indeksleme
  görevleri. Bakılacak kod yok.
- **Mail var ama düğüm değil, agent aracı — üstelik düğüm hâli iptal edilmiş.**
  `emailNode` kanvasta yorum satırı (`web/src/pages/agent/canvas/index.tsx:91`),
  `Operator.Email` jenerik `ragNode`'a düşürülmüş (`constant/index.tsx:814`),
  agent'ın araç açılırında "communication" başlığı altında duruyor
  (`form/agent-form/tool-popover/tool-command.tsx:44`). Kodu
  `agent/tools/email.py`, 211 satır ve çoğu parametre tanımı; asıl iş standart
  `smtplib`. Go'da `net/smtp` karşılığı ~40 satır.

Bizde ikisinin de şablonu **zaten var**: zamanlanmış tetikleyici için
[jiratrigger/trigger.go](../../backend/internal/jiratrigger/trigger.go) (228
satır, kendi paketi, ayardan okunan `interval func() time.Duration`, sonuç
`workflow.Launcher`'a giriyor) ve [runbatch/scheduler.go:132](../../backend/internal/runbatch/scheduler.go#L132);
mail düğümü için `jira.comment` düğümünün kendisi.

### Spagetti riski gerçek — ama RAGFlow'dan gelmiyor, bizde duruyor

Yeni düğüm eklemek mimariyi bozmuyor; kod bunun için kurulmuş.
[kinds.go:8-19](../../backend/internal/workflow/kinds.go#L8-L19) bunu açıkça
yazıyor: tür kayıt defterinden geliyor, `graph.go` ve `executor.go` içindeki
switch'lere dal eklenmiyor. Bir mail düğümü pratikte üç dokunuş — `kinds.go`'ya
bir `KindSpec`, `handler.go`'daki `Handlers` haritasına bir `StepHandler`,
panele bir form. Executor hiç değişmiyor.

Risk **iki somut yerde** ve bugün ölçülebilir durumda:

| Yer | Bugünkü ölçü | Üç tür daha eklenince |
|---|---|---|
| [`NodeConfig`](../../backend/internal/workflow/graph.go#L45-L92) | 7 tür için **15 alan**, tek düz struct | ~25 alan; hangi alanın hangi türe ait olduğunu yalnızca yorum satırları söyler |
| [`NodeInspector.tsx`](../../frontend/src/components/flow/NodeInspector.tsx) | **696 satır**, tür başına bölüm + gövdeye serpilmiş `kind === "…"` dalları | ~950 satır |

Bu yüzden sıra şu: **önce iki yarma, sonra düğüm eklemek serbest.** İkisi de
davranış değiştirmeyen, mevcut testlerin koruduğu işler.

---

## İş 1 — `NodeConfig`: gömülü, tür başına struct

### Kilit kısıt: kablo biçimi değişmeyecek

`workflow_versions.graph` JSONB olarak saklanıyor. Kablo biçimi değişirse
kayıtlı bütün grafların migration'ı ve frontend değişikliği gerekir — bu artık
"davranış değişmeyen düzenleme" olmaktan çıkar, ayrı bir iş olur.

Go, gömülü struct'ların alanlarını JSON'da düzleştirir. Bu yüzden aşağıdaki
biçim bugünküyle **birebir aynı** JSON üretir:

```go
type NodeConfig struct {
    // Birden fazla türün paylaştığı alan — PR gövdesi ve Jira yorum metni.
    Body string `json:"body,omitempty"`

    AgentConfig  // agentId, providerId, model, prompt, branch, autoPush
    PRConfig     // title, headBranch, baseBranch
    JiraConfig   // issueKey, jql
    MCPConfig    // mcpServerId, toolName, arguments
}
```

### `Body` neden dışarıda kalıyor

İki gömülü struct **aynı JSON etiketini** taşırsa Go o alanı sessizce düşürür —
hata vermez, alan yok olur. `Body` bugün hem `github.pr` hem `jira.comment`
tarafından kullanılıyor ([kinds.go:107](../../backend/internal/workflow/kinds.go#L107),
[:123](../../backend/internal/workflow/kinds.go#L123)), yani ikisine birden
konsaydı PR gövdesi ve Jira yorumu birlikte kaybolurdu. Dış seviyede durması
şart; tuzak bilinerek açıkça belgeleniyor.

### Çağrı yerleri değişmiyor

Alan yükseltmesi (promotion) sayesinde mevcut **~25 çağrı yerinin hiçbiri**
değişmiyor — `n.Config.Prompt` aynen derlenir. Dokunulan tek dosya `graph.go`
(tanım) ve eklenen tür dosyaları. `kinds.go`, `handler.go`,
`runbuild/nodes.go`, `runbuild/steprunner.go` **elden geçmiyor.**

### Bekçi testi ÖNCE yazılır

Sessiz düşürme tuzağı ileride biri iki gruba aynı etiketi koyarsa patlar. Bu
yüzden düzenlemeden önce bir test yazılır: 15 alanın hepsi dolu bir graf →
marshal → unmarshal → alan alan karşılaştırma, ayrıca JSON çıktısının
düzenleme öncesi ve sonrası **bayt olarak aynı** kaldığının doğrulanması. Test
bugünkü kodda geçmeli, düzenlemeden sonra da geçmeli. Projedeki
`bekcilerkorunmali_test.go` ile aynı ruhta.

---

## İş 2 — `NodeInspector`: tür başına dosya

696 satır zaten doğal sınırlar taşıyor. `frontend/src/components/flow/inspector/`
altına bölünür:

| Dosya | Bugün nerede | ~satır |
|---|---|---|
| `index.tsx` (dağıtıcı) | 29-107 | 90 |
| `AgentFields.tsx` | 108-243 | 135 |
| `MCPFields.tsx` | 244-384 | 140 |
| `TriggerFields.tsx` (+ `ScanSummary`) | 385-531 | 145 |
| `ActionFields.tsx` (PR + Jira) | 532-676 | 145 |
| `ReferencePicker.tsx` (+ `RefButton`) | 677-696 | 60 |

### Bölerken yapılacak tek gerçek iyileştirme

"Ata adımların `{{ steps.x }}` düğmeleri" mantığı bugün **üç yerde tekrar
ediyor** ([217-224](../../frontend/src/components/flow/NodeInspector.tsx#L217-L224),
[357-364](../../frontend/src/components/flow/NodeInspector.tsx#L357-L364),
[653-663](../../frontend/src/components/flow/NodeInspector.tsx#L653-L663)) ve
üçü de `ancestorsOf`'u ayrı ayrı çağırıp hangi alanın (`output` mu `url` mü)
önerileceğine kendi karar veriyor. Tek bileşene indirilir — dördüncü kopyayı
mail düğümü doğuracaktı.

### Dışarıya etki

Tek satır: [page.tsx:22](../../frontend/src/app/workflows/%5Bid%5D/page.tsx#L22)
içindeki import yolu. Başka hiçbir yer `NodeInspector` kullanmıyor (ölçüldü).

---

## Doğrulama ölçütü

- Backend: yeni round-trip bekçi testi + `go test ./...` temiz.
- Frontend: `npx tsc --noEmit` ve `npx eslint .` temiz.
- Tarayıcıda panelin dört türde de (agent, MCP, PR/Jira, tetikleyici) açıldığı
  doğrulanır. **Görsel değişiklik yok**, bu bir refactor —
  [ui.md](../../.claude/rules/ui.md) yeniden tasarım kuralları burada geçerli
  değil.

---

## Bunlar bitince ne kolaylaşır

Aşağıdakiler bu belgenin kapsamında **değil**; hazırlığın neyi ucuzlattığını
göstermek için yazıldı.

| Yetenek | Hazırlıktan sonraki kapsam |
|---|---|
| Bildirim (mail) düğümü | `NotifyConfig` grubu + bir `KindSpec` + bir `StepHandler` + `inspector/NotifyFields.tsx`. Go'da SMTP `net/smtp` ile; yeni bağımlılık gerekmez. Executor ve graph değişmez. |
| Zamanlanmış tetikleyici | `jiratrigger` kalıbında yeni paket (`crontrigger`), ayardan okunan aralık, `workflow.Launcher`'a giriş. Ortak hiçbir şeye dokunmuyor; `NodeConfig`'e yalnızca zamanlama alanları eklenir. |

İkisi de kendi spec'ini hak eder mi, yoksa tek spec altında mı gider — o karar
bu belgeye bırakılmadı, ayrıca verilecek.

---

## Kapsam dışı

- **`NodeConfig`'i `json.RawMessage`'a çevirmek.** Daha temiz ayrışma verirdi
  ama kayıtlı grafların migration'ını ve frontend değişikliğini gerektirir;
  davranış değiştirmeyen bir düzenleme olmaktan çıkar.
- **RAGFlow kodunu Go'ya çevirmek.** `agent/canvas.py` 1100 satır ve içinde
  yürütme, TTS, mesaj streaming ve RAG referansları iç içe; bizim
  [executor.go](../../backend/internal/workflow/executor.go) 388 satır ve
  onlarda olmayan bir şeyi yapıyor — topolojik seviye tabanlı paralellik.
  Alınacak olan şema/kavram kararlarıydı ve onlar
  [karşılaştırma belgesinde](ragflow-karsilastirma-2026-08-19.md) zaten yazıldı.
- **Panelin görsel tasarımı.** Bu iş dosya bölme; ekranın görünümü aynı kalır.

---

## Bu belge yazılırken ölçülen durum

Sonradan okuyanın kaymayı fark edebilmesi için:

| Ölçü | Değer |
|---|---|
| `NodeConfig` alan sayısı | 15 (`graph.go:45-92`) |
| Tanımlı düğüm türü | 7 (`kinds.go:36-125`) |
| `NodeInspector.tsx` | 696 satır |
| `NodeInspector` kullanan yer | 1 (`page.tsx:22`) |
| `NodeConfig` alanlarına erişen yer | ~25, beş dosyada |
| `executor.go` / `graph.go` | 388 / 459 satır |
| RAGFlow sürümü | `ec9c08d` (v0.27.0) |
