package workflow

import (
	"context"
	"fmt"
)

/*
 * Adım çalıştırıcıları.
 *
 * Doğrulama `kinds.go` içinde (türün sabit bilgisi), çalıştırma burada
 * (Docker'a ve dış servislere bağlı). Ayrı durmaları grafın ağ olmadan
 * doğrulanabilmesini sağlıyor.
 */

// ExecInput, bir adımı çalıştırmak için gereken her şey.
//
// Şablon ÇÖZÜMLENMEMİŞ halde geçiyor: her tür kendi alanlarını çözüyor
// (agent talimatı, PR başlığı ve gövdesi, Jira issue anahtarı…). Motorun
// hangi alanın şablon olduğunu bilmesi gerekmiyor.
type ExecInput struct {
	Run     Run
	Node    Node
	Context Context
}

// Render, düğümün bir alanını çözümler.
func (in ExecInput) Render(text string) (string, error) {
	return Render(text, in.Context)
}

// StepHandler, bir düğüm türünü çalıştırır.
type StepHandler interface {
	Execute(ctx context.Context, in ExecInput) (StepOutcome, error)
}

// Handlers, düğüm türünden çalıştırıcıya eşleme.
//
// Wiring anında kurulur: her tür kendi bağımlılıklarını (runner, git istemcisi,
// Jira istemcisi) o noktada alır. Motor yalnızca bu haritayı görür.
type Handlers map[NodeKind]StepHandler

// agentHandler, agent düğümünü mevcut çalıştırma altyapısına bağlar.
type agentHandler struct {
	runner StepRunner
}

// NewAgentHandler, agent düğümü çalıştırıcısını üretir.
func NewAgentHandler(r StepRunner) StepHandler {
	return agentHandler{runner: r}
}

func (h agentHandler) Execute(ctx context.Context, in ExecInput) (StepOutcome, error) {
	prompt, err := in.Render(in.Node.Config.Prompt)
	if err != nil {
		// Kaydetme anında doğrulanmıştı; buraya düşmesi beklenmez ama düşerse
		// sessizce boş talimatla çalışmaktansa akış durmalı.
		return StepOutcome{}, fmt.Errorf("talimat çözümlenemedi: %w", err)
	}

	return h.runner.RunStep(ctx, StepRequest{
		ProjectID: in.Run.ProjectID,
		Node:      in.Node,
		Prompt:    prompt,
	})
}
