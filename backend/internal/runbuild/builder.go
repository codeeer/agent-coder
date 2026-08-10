// Package runbuild, bir çalıştırma isteğini motorun anlayacağı girdiye çevirir.
//
// Bu mantık önce HTTP handler'ının içindeydi. Workflow motoru da aynı çözümlemeye
// (proje, agent, sağlayıcı, anahtar, depo erişimi) ihtiyaç duyunca buraya taşındı:
// iki yerde kopyalansaydı biri güncellenip diğeri unutulurdu. Ayrıca projenin
// kuralı zaten "iş mantığı handler'da durmaz" diyor.
package runbuild

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/agentreg"
	"github.com/agent-coder/backend/internal/gitprovider"
	"github.com/agent-coder/backend/internal/llm"
	"github.com/agent-coder/backend/internal/mcp"
	"github.com/agent-coder/backend/internal/projects"
	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runs"
)

// ErrNoModel, ne istekte ne agent'ta model belirtilmemiş.
var ErrNoModel = errors.New("model seçilmedi")

// Builder, çalıştırma girdisi üretir.
type Builder struct {
	projects *projects.Store
	agents   *agentreg.Store
	llm      *llm.Store
	git      *gitprovider.Store
	mcp      *mcp.Store
}

// New yeni bir Builder üretir.
func New(p *projects.Store, a *agentreg.Store, l *llm.Store, g *gitprovider.Store,
	m *mcp.Store,
) *Builder {
	return &Builder{projects: p, agents: a, llm: l, git: g, mcp: m}
}

/*
 * mcpServers, agent'a atanmış MCP sunucularını erişim anahtarlarıyla çözer.
 *
 * Anahtar burada, çalıştırma anında çözülür ve yalnızca container'ın ortam
 * değişkenine geçer. Hiçbir kayda, hiçbir dosyaya yazılmaz.
 *
 * Bir sunucunun anahtarı çözülemezse (şifreleme anahtarı değişmiş olabilir)
 * çalıştırma BAŞLATILMAZ: yarısı çalışan bir araç kümesiyle koşmak, agent'ın
 * neden yetersiz kaldığını gizlerdi.
 */
func (b *Builder) mcpServers(ctx context.Context, agentID uuid.UUID) ([]runner.MCPServerSpec, error) {
	if b.mcp == nil {
		return nil, nil
	}

	servers, err := b.mcp.ForAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}

	out := make([]runner.MCPServerSpec, 0, len(servers))
	for _, s := range servers {
		secret := ""
		if s.HasSecret {
			secret, err = b.mcp.Reveal(ctx, s.ID)
			if err != nil {
				return nil, fmt.Errorf("MCP sunucusu %q erişimi çözülemedi: %w", s.Name, err)
			}
		}
		out = append(out, runner.MCPServerSpec{
			Name: s.Name, Transport: string(s.Transport), URL: s.URL, Secret: secret,
		})
	}
	return out, nil
}

// RepoAccess, bir projenin depo adresini ve erişim bilgisini çözer.
//
// Üç yerde gerekiyor: elle gönderme (HTTP), akıştaki otomatik gönderme ve PR
// açma. Her birinde ayrı ayrı yazılmıştı ve biri eksik kaldığı için akış
// "git erişimi tanımlı değil" diyordu — oysa erişim tanımlıydı. Paketin var
// olma sebebi zaten buydu.
func (b *Builder) RepoAccess(ctx context.Context, projectID uuid.UUID) (
	repoURL string, creds *projects.Credentials, err error,
) {
	project, err := b.projects.Get(ctx, projectID)
	if err != nil {
		return "", nil, err
	}
	if project.GitProviderID == nil {
		return "", nil, runs.ErrNoGitAccess
	}

	gp, err := b.git.Get(ctx, *project.GitProviderID)
	if err != nil {
		return "", nil, err
	}
	secret, err := b.git.Reveal(ctx, gp.ID)
	if err != nil {
		return "", nil, err
	}

	username := gp.Username
	// Token ile klonlarken kullanıcı adı zorunlu; GitHub bunu bekler.
	if username == "" {
		username = "x-access-token"
	}
	return project.RepoURL, &projects.Credentials{Username: username, Secret: secret}, nil
}

// Request, çözümlenecek çalıştırma isteği.
type Request struct {
	ProjectID uuid.UUID
	AgentID   uuid.UUID
	// ProviderID boşsa agent'ın varsayılanı, o da yoksa sistemin varsayılanı.
	ProviderID *uuid.UUID
	// Model boşsa agent'ın varsayılanı kullanılır.
	Model string
	// Branch boşsa projenin varsayılanı kullanılır.
	Branch string
	Task   string
}

// Build, isteği çalıştırma girdisine çevirir.
//
// Agent talimatı ve model burada KOPYALANIR: kayıt, agent sonradan düzenlense
// bile neyle çalıştığını doğru göstermeli.
func (b *Builder) Build(ctx context.Context, req Request) (runs.StartInput, error) {
	project, err := b.projects.Get(ctx, req.ProjectID)
	if err != nil {
		return runs.StartInput{}, err
	}

	agent, err := b.agents.Get(ctx, req.AgentID)
	if err != nil {
		return runs.StartInput{}, err
	}

	providerID := req.ProviderID
	if providerID == nil {
		providerID = agent.DefaultProviderID
	}

	var provider llm.Provider
	if providerID != nil {
		provider, err = b.llm.Get(ctx, *providerID)
	} else {
		provider, err = b.llm.Default(ctx)
	}
	if err != nil {
		return runs.StartInput{}, err
	}

	apiKey, err := b.llm.Reveal(ctx, provider.ID)
	if err != nil {
		return runs.StartInput{}, err
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = agent.DefaultModel
	}
	if model == "" {
		return runs.StartInput{}, ErrNoModel
	}

	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = project.DefaultBranch
	}

	repo := runner.RepoSpec{URL: project.RepoURL, Branch: branch}
	if project.GitProviderID != nil {
		gp, err := b.git.Get(ctx, *project.GitProviderID)
		if err != nil {
			return runs.StartInput{}, err
		}
		secret, err := b.git.Reveal(ctx, gp.ID)
		if err != nil {
			return runs.StartInput{}, err
		}
		repo.Username = gp.Username
		// Token ile klonlarken kullanıcı adı zorunlu; GitHub bunu bekler.
		if repo.Username == "" {
			repo.Username = "x-access-token"
		}
		repo.Secret = secret
	}

	mcpServers, err := b.mcpServers(ctx, agent.ID)
	if err != nil {
		return runs.StartInput{}, err
	}

	return runs.StartInput{
		Create: runs.CreateInput{
			ProjectID: project.ID, AgentID: agent.ID, ProviderID: &provider.ID,
			AgentSlug: agent.Slug, AgentPrompt: agent.Prompt,
			ProviderSlug: provider.Slug, ModelID: model,
			Branch: branch, Task: req.Task,
		},
		Repo: repo,
		Agent: runner.AgentSpec{
			Slug: agent.Slug, Description: agent.Description, Prompt: agent.Prompt,
			AllowEdit: agent.AllowEdit, AllowBash: agent.AllowBash,
			AllowWebfetch: agent.AllowWebfetch,
			MCPServers:    mcpServers,
		},
		Provider: runner.ProviderSpec{
			Slug: provider.Slug, Kind: string(provider.Type),
			BaseURL: provider.BaseURL, APIKey: apiKey,
		},
	}, nil
}
