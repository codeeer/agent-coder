# Plan: Jira ve Kod Deposu Düğümleri

- **Spec no:** 009 — [spec.md](spec.md)
- **Durum:** İnceleme — onay bekliyor

---

## En önemli karar: düğüm türü artık genişleyebilir olmalı

Bugün motor yalnızca `agent` düğümünü çalıştırıyor; `executor.go` içinde tür
kontrolü elle yapılıyor. Üç yeni tür (`github.pr`, `jira.comment`,
`trigger.jira`) eklemek için orayı `switch` ile şişirmek yerine **düğüm türü
kayıt defteri** kuruluyor:

```go
// Her düğüm türü kendini tanıtır: nasıl doğrulanır, nasıl çalıştırılır.
type NodeHandler interface {
    Validate(n Node, g Graph) []Problem
    Execute(ctx context.Context, in ExecInput) (StepResult, error)
}
var handlers = map[NodeKind]NodeHandler{...}
```

Bunun karşılığı doğrudan görülüyor: `agent` düğümünün doğrulaması (agent seçili
mi, talimat boş mu) bugün `graph.go` içinde gömülü — kayıt defteriyle kendi
yerine taşınıyor ve yeni türler `graph.go`'ya hiç dokunmadan ekleniyor.

**Motor değişmiyor:** seviye seviye yürütme, paralellik, hata halinde durdurma,
iptal — hepsi olduğu gibi kalıyor. Değişen tek şey "bir adım nasıl çalıştırılır"
sorusunun cevabının nereden geldiği.

## Aşama 1 — çıkış düğümleri (bu plan)

### `github.pr` düğümü

Yapılandırma: `title`, `body`, `baseBranch` (boşsa projenin varsayılanı).
Şablon çözümlenir — başlık ve gövde önceki adımların çıktısından gelir.

Hangi branch'ten PR açılacak? **Önceki adımların gönderdiği branch'ten.** Akış
bağlamında `{{ steps.<adım>.branch }}` zaten var; düğüm onu kullanır. Branch
yoksa açık hata: "önce bir adımın değişikliği branch'e göndermesi gerekir".

Bu, mevcut `runs.Pusher`'ı akışa bağlıyor: agent adımı diff üretiyor, push
ediliyor, PR düğümü o branch'i açıyor.

> **Not:** bugün push ELLE yapılıyor (çalıştırma detayındaki düğme). Akışta
> otomatik olması gerekiyor; bu yüzden agent düğümüne `autoPush` seçeneği
> ekleniyor. Varsayılan KAPALI — akışın sessizce depoya yazması sürpriz olurdu.

### `jira.comment` düğümü

Yapılandırma: `issueKey` (şablonlu — genelde `{{ trigger.key }}`), `body`.
Jira erişimi zaten `credentials` içinde tanımlı (Faz 1'den beri duruyor,
kullanılmıyordu).

### Ortak davranış

- Bu düğümler **model çağırmıyor**: `runs` kaydı üretmiyorlar, yalnızca
  `workflow_steps` kaydı taşıyorlar. Rapor rakamları bozulmuyor (kabul kriteri).
- Sonuçları akış bağlamına giriyor: `{{ steps.<pr-düğümü>.url }}` gibi.
  Bu, `StepResult`'a `url` alanı eklemeyi gerektiriyor.
- Hataları adım hatası olarak görünüyor; akış K2 gereği duruyor.

### Yeni paketler

```
internal/integrations/jira/    REST v3 istemcisi (yorum yazma; ileride arama)
internal/integrations/github/  PR açma
```

Her ikisi de `httptest` ile test edilebilir olacak — gerçek servise bağlanmadan.

## Aşama 2 — Jira tetikleyici (ayrı tasks bölümü)

`trigger.jira` düğümü + iki giriş yolu (K1):

- **Tarama:** arka plan işçisi, akış başına JQL, aralık ayarlardan
- **Webhook:** `POST /hooks/jira/{token}` — mevcut hook altyapısının yanına

İkisi de aynı yerde buluşuyor:

```
tarama ──┐
         ├──> processIssue(workflow, issue) ──> tekrar kontrolü ──> akışı başlat
webhook ─┘
```

**Tekrar-işleme koruması:** `workflow_processed_issues (workflow_id, issue_key,
issue_updated_at)` — benzersiz kısıt. Aynı task ikinci kez gelirse kayıt
çakışıyor ve akış başlatılmıyor. Task Jira'da güncellenirse `updated_at`
değiştiği için yeniden işlenebiliyor.

Kısıtın veritabanında olması bilinçli: iki tetikleme yolu aynı anda gelirse
uygulama içi bir kontrol yarışa açık olurdu.

## Riskler

| Risk | Önlem |
|---|---|
| Düğüm türü kayıt defteri motoru karmaşıklaştırır | Motor dokunulmuyor; yalnızca "adımı çalıştır" çağrısı arayüzden geçiyor |
| Akış sessizce depoya yazar | `autoPush` varsayılan kapalı, açıkça seçilir |
| Jira/GitHub erişimi yoksa motor bozulur | Düğüm açık hatayla başarısız olur, akış K2 gereği durur |
| İki tetikleme yolu ayrışır | Ortak `processIssue` yolu + veritabanı kısıtı |
| PR açılacak branch belirsiz | Bağlamdaki branch kullanılır; yoksa açık hata |

## Doğrulama

**Aşama 1:**
1. Agent → push → PR düğümü olan akış çalışır, **gerçek PR açılır**
2. PR başlığı ve gövdesi önceki adımın çıktısından gelir
3. Jira yorum düğümü gerçek bir issue'ya yorum yazar
4. Bu düğümler rapor rakamlarını değiştirmez (maliyet 0, çalıştırma sayısı artmaz)
5. Erişim tanımsızken düğüm açık hatayla başarısız olur, akış durur

**Aşama 2:**
6. JQL'e uyan yeni task akışı başlatır
7. Aynı task ikinci taramada **yeniden başlatılmaz**
8. Webhook ile gelen task da aynı korumadan geçer
9. Task Jira'da güncellenince yeniden işlenir
