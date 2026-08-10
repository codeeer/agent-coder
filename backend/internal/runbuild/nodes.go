package runbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/agent-coder/backend/internal/credentials"
	"github.com/agent-coder/backend/internal/integrations/github"
	"github.com/agent-coder/backend/internal/integrations/jira"
	"github.com/agent-coder/backend/internal/projects"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Agent olmayan akış düğümleri.
 *
 * Bu düğümler MODEL ÇAĞIRMIYOR: `runs` kaydı üretmiyorlar, yalnızca adım
 * kaydına sonuç yazıyorlar. Rapor `runs` üzerinden toplandığı için rakamlar
 * bozulmuyor — bir PR açmak "çalıştırma" değildir ve maliyeti yoktur.
 */

// PRHandler, `github.pr` düğümünü çalıştırır.
type PRHandler struct {
	builder  *Builder
	projects *projects.Store
	client   *github.Client
	// graph, PR'ın kaynak branch'ini bulmak için gerekli.
	graphOf func(ctx context.Context, versionID string) (workflow.Graph, error)
}

// NewPRHandler yeni PR düğümü çalıştırıcısı üretir.
func NewPRHandler(b *Builder, p *projects.Store, c *github.Client,
	graphOf func(ctx context.Context, versionID string) (workflow.Graph, error),
) *PRHandler {
	return &PRHandler{builder: b, projects: p, client: c, graphOf: graphOf}
}

// Execute, pull request açar.
func (h *PRHandler) Execute(ctx context.Context, in workflow.ExecInput) (
	workflow.StepOutcome, error,
) {
	project, err := h.projects.Get(ctx, in.Run.ProjectID)
	if err != nil {
		return workflow.StepOutcome{}, err
	}
	_, creds, err := h.builder.RepoAccess(ctx, in.Run.ProjectID)
	if err != nil {
		return workflow.StepOutcome{}, fmt.Errorf("PR açılamaz: %w", err)
	}

	head, err := h.headBranch(ctx, in)
	if err != nil {
		return workflow.StepOutcome{}, err
	}

	title, err := in.Render(in.Node.Config.Title)
	if err != nil {
		return workflow.StepOutcome{}, fmt.Errorf("PR başlığı çözümlenemedi: %w", err)
	}
	body, err := in.Render(in.Node.Config.Body)
	if err != nil {
		return workflow.StepOutcome{}, fmt.Errorf("PR açıklaması çözümlenemedi: %w", err)
	}

	base := strings.TrimSpace(in.Node.Config.BaseBranch)
	if base == "" {
		base = project.DefaultBranch
	} else if base, err = in.Render(base); err != nil {
		return workflow.StepOutcome{}, err
	}

	pr, err := h.client.Open(ctx, github.OpenInput{
		RepoURL: project.RepoURL, Token: creds.Secret,
		Title: title, Body: body, Head: head, Base: base,
	})
	if err != nil {
		return workflow.StepOutcome{}, err
	}

	return workflow.StepOutcome{
		Output: fmt.Sprintf("PR #%d açıldı: %s", pr.Number, pr.URL),
		URL:    pr.URL,
		Branch: head,
	}, nil
}

// headBranch, PR'ın açılacağı kaynak branch'i bulur.
//
// Açıkça yazıldıysa o kullanılır. Boşsa, doğrulamanın tek olduğunu garanti
// ettiği gönderim yapan ata adımın branch'i alınır — tahmin yok, doğrulama
// zaten kaydetme anında karar verdi.
func (h *PRHandler) headBranch(ctx context.Context, in workflow.ExecInput) (string, error) {
	if raw := strings.TrimSpace(in.Node.Config.HeadBranch); raw != "" {
		branch, err := in.Render(raw)
		if err != nil {
			return "", fmt.Errorf("kaynak branch çözümlenemedi: %w", err)
		}
		if strings.TrimSpace(branch) == "" {
			return "", errors.New("kaynak branch boş geldi — önceki adım gönderim yapmamış olabilir")
		}
		return branch, nil
	}

	graph, err := h.graphOf(ctx, in.Run.VersionID.String())
	if err != nil {
		return "", err
	}
	source, ok := graph.PushingAncestor(in.Node.ID)
	if !ok {
		return "", errors.New("PR açılacak branch yok — önceki bir agent adımı gönderim yapmalı")
	}

	step, ok := in.Context.Steps[source]
	if !ok || step.Branch == "" {
		return "", fmt.Errorf("`%s` adımı branch göndermedi — değişiklik üretmemiş olabilir", source)
	}
	return step.Branch, nil
}

/* ── Jira yorumu ─────────────────────────────────────────────────────────── */

// JiraHandler, `jira.comment` düğümünü çalıştırır.
type JiraHandler struct {
	creds  *credentials.Store
	client *jira.Client
	// flows, kendi yazdığımız yorumun tetiklediği güncellemeyi işaretlemek için.
	flows *workflow.Store
}

// NewJiraHandler yeni Jira yorum düğümü çalıştırıcısı üretir.
func NewJiraHandler(c *credentials.Store, client *jira.Client, flows *workflow.Store) *JiraHandler {
	return &JiraHandler{creds: c, client: client, flows: flows}
}

// Execute, issue'ya yorum yazar.
func (h *JiraHandler) Execute(ctx context.Context, in workflow.ExecInput) (
	workflow.StepOutcome, error,
) {
	token, meta, err := h.creds.Reveal(ctx, credentials.KindJira)
	if err != nil {
		return workflow.StepOutcome{}, fmt.Errorf(
			"Jira erişimi tanımlı değil — Ayarlar'dan ekleyin: %w", err)
	}

	key, err := in.Render(in.Node.Config.IssueKey)
	if err != nil {
		return workflow.StepOutcome{}, fmt.Errorf("issue anahtarı çözümlenemedi: %w", err)
	}
	body, err := in.Render(in.Node.Config.Body)
	if err != nil {
		return workflow.StepOutcome{}, fmt.Errorf("yorum metni çözümlenemedi: %w", err)
	}

	out, err := h.client.AddComment(ctx, jira.CommentInput{
		BaseURL:  meta["base_url"],
		Email:    meta["email"],
		Token:    token,
		IssueKey: strings.TrimSpace(key),
		Body:     body,
	})
	if err != nil {
		return workflow.StepOutcome{}, err
	}

	h.claimOwnUpdate(ctx, in, jira.CommentInput{
		BaseURL: meta["base_url"], Email: meta["email"], Token: token,
		IssueKey: strings.TrimSpace(key),
	})

	return workflow.StepOutcome{
		Output: fmt.Sprintf("%s issue'suna yorum yazıldı", key),
		URL:    out.URL,
	}, nil
}

/*
 * Kendi yazdığımız yorumu tetikleyici saymamak.
 *
 * ÖLÇÜLDÜ (spec 009, Ölçüm 4): akış Jira'ya yorum yazınca task'ın "güncellenme
 * zamanı" değişiyor. Tekrar-işleme koruması bu zamanı anahtar olarak
 * kullandığından, akış kendi bıraktığı izi yeni bir iş sanıp yeniden başlıyordu
 * — her turda yeni bir PR ve yeni bir yorum. Beş dakikalık tarayıcıyla bu
 * sonsuz döngüdür.
 *
 * Çözüm: yorumdan hemen sonra task yeniden okunur ve OLUŞAN yeni güncellenme
 * zamanı işlenmiş olarak kaydedilir. Böylece o değişiklik "bizim" sayılır;
 * bundan sonra gelen gerçek bir güncelleme yine akışı başlatır.
 *
 * Yalnızca akışı BU task başlattıysa yapılır: farklı bir issue'ya yorum yazmak
 * tetikleyiciyle ilgisiz bir iştir ve onun işaretine dokunulmamalı.
 *
 * Kabul edilen sınır: bir insan tam aynı anda task'ı düzenlerse aynı zaman
 * damgasını paylaşır ve o düzenleme yutulur. Zamanı saniye altında ayırmanın
 * yolu yok; alternatif — kendi değişikliğimize tepki vermek — çok daha kötü.
 */
func (h *JiraHandler) claimOwnUpdate(ctx context.Context, in workflow.ExecInput, cred jira.CommentInput) {
	if h.flows == nil || in.Run.TriggerKind != workflow.TriggerJira {
		return
	}
	if !strings.EqualFold(in.Run.TriggerPayload["key"], cred.IssueKey) {
		return
	}

	issue, err := h.client.GetIssue(ctx, cred)
	if err != nil {
		slog.WarnContext(ctx, "kendi güncellememiz işaretlenemedi — task yeniden okunamadı",
			"issue", cred.IssueKey, "error", err)
		return
	}
	if _, err := h.flows.MarkProcessed(ctx, in.Run.WorkflowID, issue.Key, issue.UpdatedAt); err != nil {
		slog.WarnContext(ctx, "kendi güncellememiz işaretlenemedi",
			"issue", cred.IssueKey, "error", err)
	}
}
