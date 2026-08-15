// Package jiratrigger, Jira task'larından akış başlatır.
//
// İKİ GİRİŞ YOLU var (spec 009 K1): düzenli JQL taraması ve Jira webhook'u.
// İkisi de `Process` fonksiyonundan geçer — ayrı ayrı yazılsalardı iki farklı
// tekrar-işleme koruması ve iki farklı hata modeli olurdu.
//
//	tarama ──┐
//	         ├──> Process ──> tekrar kontrolü ──> akışı başlat
//	webhook ─┘
package jiratrigger

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/credentials"
	"github.com/agent-coder/backend/internal/integrations/jira"
	"github.com/agent-coder/backend/internal/workflow"
)

// Trigger, Jira tetikleyicisini yürütür.
type Trigger struct {
	store    *workflow.Store
	launcher *workflow.Launcher
	creds    *credentials.Store
	client   *jira.Client

	// interval ve limit ayarlardan okunur; değişiklik yeniden başlatma
	// gerektirmesin diye fonksiyon olarak taşınır (spec 003 H7).
	interval func() time.Duration
	limit    func() int
}

// New yeni tetikleyici üretir.
func New(store *workflow.Store, launcher *workflow.Launcher, creds *credentials.Store,
	client *jira.Client, interval func() time.Duration, limit func() int,
) *Trigger {
	return &Trigger{
		store: store, launcher: launcher, creds: creds, client: client,
		interval: interval, limit: limit,
	}
}

/*
tarama, bir sonraki taramaya kalan süre.

Sıfır ya da eksi bir değer buraya GELMEMELİ — ayar `Min: 1` ile doğrulanıyor.
Yine de korunuyor ve sebebi bu paket için daha sert: sıfırlık bir bekleme
zamanlayıcıyı anında ateşler ve tarama, DIŞARIDAKİ bir servisi aralıksız
dövmeye başlar. Kendi kuyruğunu boşuna döndürmek gibi değil; karşı taraf
istekleri sayıyor.

Sıfır gerçekten gelebiliyor: `settings.Int` bilinmeyen anahtar için sıfır
dönüyor, yani ayar anahtarı bir gün registry'den düşerse tetikleyici sessizce
sıcak döngüye girer.
*/
func (t *Trigger) tarama() time.Duration {
	if t.interval == nil {
		return time.Minute
	}
	d := t.interval()
	if d <= 0 {
		return time.Minute
	}
	return d
}

// Run, tarama döngüsünü çalıştırır. ctx iptal edilene kadar sürer.
func (t *Trigger) Run(ctx context.Context) {
	// İlk tarama açılışta değil, bir aralık sonra: sunucu her yeniden
	// başladığında anında Jira'ya yüklenmek istemiyoruz.
	timer := time.NewTimer(t.tarama())
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			t.ScanAll(ctx)
			timer.Reset(t.tarama())
		}
	}
}

// ScanAll, Jira tetikleyicisi olan tüm etkin akışları tarar.
func (t *Trigger) ScanAll(ctx context.Context) {
	versions, flows, err := t.store.ActiveWithVersions(ctx)
	if err != nil {
		slog.WarnContext(ctx, "akışlar taranamadı", "error", err)
		return
	}

	for i, v := range versions {
		node, ok := jiraTriggerOf(v.Graph)
		if !ok {
			continue
		}
		t.scanOne(ctx, flows[i], node)
	}
}

// jiraTriggerOf, graftaki Jira tetikleyici düğümünü bulur.
func jiraTriggerOf(g workflow.Graph) (workflow.Node, bool) {
	for _, n := range g.Nodes {
		if n.Kind == workflow.KindTriggerJira {
			return n, true
		}
	}
	return workflow.Node{}, false
}

// scanOne, tek bir akışın JQL sorgusunu çalıştırır.
func (t *Trigger) scanOne(ctx context.Context, wf workflow.Workflow, node workflow.Node) {
	state := workflow.ScanState{WorkflowID: wf.ID}

	token, meta, err := t.creds.Reveal(ctx, credentials.KindJira)
	if err != nil {
		msg := "Jira erişimi tanımlı değil"
		state.Error = &msg
		_ = t.store.SaveScanState(ctx, state)
		return
	}

	issues, err := t.client.Search(ctx, jira.SearchInput{
		BaseURL: meta["base_url"], Email: meta["email"], Token: token,
		JQL: node.Config.JQL, Limit: t.limit(),
	})
	if err != nil {
		msg := err.Error()
		state.Error = &msg
		_ = t.store.SaveScanState(ctx, state)
		slog.WarnContext(ctx, "Jira taraması başarısız",
			"workflow_id", wf.ID, "error", err)
		return
	}

	state.Found = len(issues)
	for _, issue := range issues {
		started, err := t.Process(ctx, wf.ID, issue)
		if err != nil {
			msg := err.Error()
			state.Error = &msg
			continue
		}
		if started {
			state.Started++
		}
	}
	_ = t.store.SaveScanState(ctx, state)
}

// Process, bir task için akışı başlatır — ZATEN İŞLENDİYSE başlatmaz.
//
// Tekrar kontrolü veritabanı kısıtıyla yapılıyor: iki tetikleme yolu aynı anda
// gelse bile yalnızca biri kaydı oluşturabiliyor. Uygulama içi bir kontrol
// (önce sor, sonra yaz) yarışa açık olurdu.
func (t *Trigger) Process(ctx context.Context, workflowID uuid.UUID, issue jira.Issue) (
	bool, error,
) {
	fresh, err := t.store.MarkProcessed(ctx, workflowID, issue.Key, issue.UpdatedAt)
	if err != nil {
		return false, err
	}
	if !fresh {
		return false, nil
	}

	run, err := t.launcher.Launch(ctx, workflow.LaunchInput{
		WorkflowID: workflowID,
		Trigger:    workflow.TriggerJira,
		// Task alanları `{{ trigger.<alan> }}` ile okunur.
		Payload: issue.Fields(),
		// Giriş metni ÖZET + AÇIKLAMA: ilk adım `{{ input }}` yazsa bile
		// anlamlı bir görev görsün.
		Input: taskText(issue),
	})
	if err != nil {
		// İşaret duruyor ama hiçbir şey çalışmadı: geri alınmazsa bu task bir
		// daha hiç denenmez. Geri alma başarısız olsa bile asıl hata dönülür.
		if uerr := t.store.UnmarkProcessed(ctx, workflowID, issue.Key, issue.UpdatedAt); uerr != nil {
			slog.WarnContext(ctx, "task işareti geri alınamadı",
				"issue", issue.Key, "error", uerr)
		}
		return false, err
	}

	if err := t.store.LinkProcessed(ctx, workflowID, issue.Key, issue.UpdatedAt, run.ID); err != nil {
		slog.WarnContext(ctx, "task çalışmaya bağlanamadı", "error", err)
	}
	slog.InfoContext(ctx, "Jira task'ı akışı başlattı",
		"workflow_id", workflowID, "issue", issue.Key, "workflow_run_id", run.ID)
	return true, nil
}

// HandleWebhook, Jira'dan gelen bildirimi işler.
//
// Gövdedeki alanlara GÜVENİLMEZ: yalnızca issue anahtarı alınır ve task
// API'den yeniden okunur. Böylece webhook ve tarama aynı veriyi görür ve
// güncellenme zamanı (tekrar kontrolünün anahtarı) doğru olur.
func (t *Trigger) HandleWebhook(ctx context.Context, workflowID uuid.UUID, issueKey string) (
	bool, error,
) {
	token, meta, err := t.creds.Reveal(ctx, credentials.KindJira)
	if err != nil {
		return false, err
	}

	issue, err := t.client.GetIssue(ctx, jira.CommentInput{
		BaseURL: meta["base_url"], Email: meta["email"], Token: token, IssueKey: issueKey,
	})
	if err != nil {
		return false, err
	}
	return t.Process(ctx, workflowID, issue)
}

// taskText, agent'a verilecek görev metni.
func taskText(issue jira.Issue) string {
	text := issue.Key + ": " + issue.Summary
	if issue.Description != "" {
		text += "\n\n" + issue.Description
	}
	return text
}
