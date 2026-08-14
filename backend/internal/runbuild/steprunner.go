package runbuild

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/runs"
	"github.com/agent-coder/backend/internal/workflow"
)

// StepRunner, akış motorunun bir adımı çalıştırmasını sağlar.
//
// Motor ile çalıştırma altyapısı arasındaki köprü. Motor `workflow.StepRunner`
// arayüzünü görür; bu tip onu `runbuild` + `runs.Manager` üzerinden karşılar.
// Ayrı durmasının sebebi motorun test edilebilirliği: motor testleri sahte bir
// runner kullanıyor ve Docker'a hiç dokunmuyor.
type StepRunner struct {
	builder *Builder
	manager *runs.Manager
	pusher  *runs.Pusher
}

// NewStepRunner yeni köprü üretir.
func NewStepRunner(b *Builder, m *runs.Manager, p *runs.Pusher) *StepRunner {
	return &StepRunner{builder: b, manager: m, pusher: p}
}

// RunStep, adımı çalıştırır ve BİTMESİNİ bekler.
func (r *StepRunner) RunStep(ctx context.Context, req workflow.StepRequest) (
	workflow.StepOutcome, error,
) {
	agentID, err := uuid.Parse(req.Node.Config.AgentID)
	if err != nil {
		return workflow.StepOutcome{}, fmt.Errorf("adımın agent kimliği geçersiz: %w", err)
	}

	var providerID *uuid.UUID
	if raw := req.Node.Config.ProviderID; raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			return workflow.StepOutcome{}, fmt.Errorf("adımın sağlayıcı kimliği geçersiz: %w", err)
		}
		providerID = &id
	}

	input, err := r.builder.Build(ctx, Request{
		ProjectID:  req.ProjectID,
		AgentID:    agentID,
		ProviderID: providerID,
		Model:      req.Node.Config.Model,
		Branch:     req.Node.Config.Branch,
		Task:       req.Prompt,
	})
	if err != nil {
		return workflow.StepOutcome{}, err
	}

	run, err := r.manager.Execute(ctx, input)
	if err != nil {
		// Kayıt hiç oluşmadı (sınır dolu, çözümleme hatası): bağlanacak
		// çalıştırma da yok.
		return workflow.StepOutcome{}, err
	}

	outcome := workflow.StepOutcome{
		RunID:  run.ID,
		Output: run.Output,
		Diff:   run.Diff,
	}
	if run.PushedBranch != nil {
		outcome.Branch = *run.PushedBranch
	}

	// Gönderim yalnızca AÇIKÇA istendiğinde yapılır (spec 009): bir akışın
	// habersizce depoya yazması sürpriz olurdu. Değişiklik yoksa gönderim de
	// yok — boş bir branch açmak gürültü olurdu.
	if req.Node.Config.AutoPush && run.Status == runs.StatusSucceeded && run.HasChanges() {
		repoURL, creds, err := r.builder.RepoAccess(ctx, req.ProjectID)
		if err != nil {
			return outcome, fmt.Errorf("değişiklik branch'e gönderilemedi: %w", err)
		}
		sonuc, err := r.pusher.Push(ctx, runs.PushRequest{
			Run: run, Repo: repoURL, Creds: creds,
		})
		if err != nil {
			return outcome, fmt.Errorf("değişiklik branch'e gönderilemedi: %w", err)
		}
		outcome.Branch = sonuc.Branch
	}

	// Çalıştırma başarısızsa akış da durmalı. Kayıt oluştuğu için RunID yine
	// döner: kullanıcı başarısız adımın çıktısını ve maliyetini görebilmeli.
	if run.Status != runs.StatusSucceeded {
		msg := string(run.Status)
		if run.Error != nil {
			msg = *run.Error
		}
		return outcome, fmt.Errorf("%s", msg)
	}
	return outcome, nil
}
