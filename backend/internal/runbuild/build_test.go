package runbuild

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/agentreg"
	"github.com/agent-coder/backend/internal/gitprovider"
	"github.com/agent-coder/backend/internal/llm"
	"github.com/agent-coder/backend/internal/mcp"
	"github.com/agent-coder/backend/internal/projects"
	"github.com/agent-coder/backend/internal/scripts"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * İsteğin çalıştırma girdisine çevrilmesi.
 *
 * Bu katmanın tamamı ÖNCELİK KURALLARINDAN ibaret: hangi model, hangi
 * sağlayıcı, hangi branch, hangi kimlik. Yanlış çözülen bir öncelik sessizce
 * yanlış işi çalıştırır — agent koşar, bir şey üretir, ama istenen şey
 * değildir.
 */

type buildFixture struct {
	b        *Builder
	projects *projects.Store
	agents   *agentreg.Store
	git      *gitprovider.Store
	llm      *llm.Store
}

func buildSetup(t *testing.T) buildFixture {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "runs", "workflows", "projects", "agents",
		"llm_providers", "git_providers", "mcp_servers", "scripts")

	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(make([]byte, secrets.KeySize)))
	require.NoError(t, err)

	pr := projects.NewStore(pool)
	ag := agentreg.NewStore(pool, func() int { return 32 << 10 })
	lm := llm.NewStore(pool, cipher)
	gp := gitprovider.NewStore(pool, cipher)

	return buildFixture{
		b:        New(pr, ag, lm, gp, mcp.NewStore(pool, cipher), scripts.NewStore(pool)),
		projects: pr, agents: ag, git: gp, llm: lm,
	}
}

// saglayici, varsayılan bir LLM sağlayıcısı kurar. İlk kayıt kendiliğinden
// varsayılan olur.
func (f buildFixture) saglayici(t *testing.T) llm.Provider {
	t.Helper()
	p, err := f.llm.Create(context.Background(), llm.CreateInput{
		Type: llm.TypeOpenRouter, Name: "OpenRouter", Secret: "sk-test-anahtar-1234",
	})
	require.NoError(t, err)
	return p
}

func (f buildFixture) proje(t *testing.T, in projects.Input) projects.Project {
	t.Helper()
	if in.Name == "" {
		in.Name = "Deneme"
	}
	if in.RepoURL == "" {
		in.RepoURL = "https://example.com/takim/proje.git"
	}
	if in.DefaultBranch == "" {
		in.DefaultBranch = "main"
	}
	p, err := f.projects.Create(context.Background(), in)
	require.NoError(t, err)
	return p
}

func (f buildFixture) agent(t *testing.T, in agentreg.CreateInput) agentreg.Agent {
	t.Helper()
	if in.Slug == "" {
		in.Slug = "coder"
	}
	if in.Name == "" {
		in.Name = "Coder"
	}
	if in.Prompt == "" {
		in.Prompt = "Sen bir kod yazarsın."
	}
	a, err := f.agents.Create(context.Background(), in)
	require.NoError(t, err)
	return a
}

// ── Model önceliği ──────────────────────────────────────────────────────────

/*
İSTEĞİN MODELİ AGENT'IN VARSAYILANINI YENER.

Kullanıcı bir koşuda modeli değiştirebiliyor; değiştirdiğinde agent'ın
varsayılanı geçerli olsaydı seçim ekranda görünür ama hiçbir işe yaramazdı.
*/
func TestBuild_IstegindekiModelAgentVarsayilaniniYener(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)
	p := f.proje(t, projects.Input{})
	a := f.agent(t, agentreg.CreateInput{DefaultModel: "varsayilan-model"})

	in, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Model: "istenen-model", Task: "iş",
	})
	require.NoError(t, err)
	require.Equal(t, "istenen-model", in.Create.ModelID)
}

// İstek model vermezse agent'ın varsayılanı kullanılır — akış adımları model
// vermiyor ve orada tek kaynak agent'ın kendisi.
func TestBuild_ModelVerilmezseAgentVarsayilani(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)
	p := f.proje(t, projects.Input{})
	a := f.agent(t, agentreg.CreateInput{DefaultModel: "varsayilan-model"})

	in, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Task: "iş",
	})
	require.NoError(t, err)
	require.Equal(t, "varsayilan-model", in.Create.ModelID)
}

/*
HİÇBİR MODEL YOKSA ÇALIŞTIRMA BAŞLAMAZ.

Sessizce boş bir modelle devam edilseydi hata sağlayıcıdan dönerdi ve
kullanıcı sebebini "model seçilmedi" olarak değil, sağlayıcının ham
mesajından okumaya çalışırdı.
*/
func TestBuild_ModelHicYoksaReddedilir(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)
	p := f.proje(t, projects.Input{})
	a := f.agent(t, agentreg.CreateInput{})

	_, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Task: "iş",
	})
	require.ErrorIs(t, err, ErrNoModel)
}

// ── Branch önceliği ─────────────────────────────────────────────────────────

func TestBuild_BranchVerilmezseProjeninVarsayilani(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)
	p := f.proje(t, projects.Input{DefaultBranch: "develop"})
	a := f.agent(t, agentreg.CreateInput{DefaultModel: "m"})

	in, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Task: "iş",
	})
	require.NoError(t, err)
	require.Equal(t, "develop", in.Create.Branch)
	require.Equal(t, "develop", in.Repo.Branch, "container da aynı branch'i klonlamalı")
}

func TestBuild_IstegindekiBranchProjeyiYener(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)
	p := f.proje(t, projects.Input{DefaultBranch: "main"})
	a := f.agent(t, agentreg.CreateInput{DefaultModel: "m"})

	in, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Branch: "hotfix/1", Task: "iş",
	})
	require.NoError(t, err)
	require.Equal(t, "hotfix/1", in.Repo.Branch)
}

// ── Kayda kopyalanan alanlar ────────────────────────────────────────────────

/*
AGENT TALİMATI VE MODELİ KAYDA KOPYALANIR.

Agent sonradan düzenlenebiliyor. Kayıt referans tutsaydı, eski bir çalıştırmayı
açan kullanıcı bugünkü talimatı görür ve o koşunun neyle çalıştığını bir daha
asla öğrenemezdi.
*/
func TestBuild_AgentTalimatiKaydaKopyalanir(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)
	p := f.proje(t, projects.Input{})
	a := f.agent(t, agentreg.CreateInput{
		Slug: "inceleyici", Prompt: "O günkü talimat", DefaultModel: "m"})

	in, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Task: "iş",
	})
	require.NoError(t, err)
	require.Equal(t, "O günkü talimat", in.Create.AgentPrompt)
	require.Equal(t, "inceleyici", in.Create.AgentSlug)
}

// ── Git kimliği ─────────────────────────────────────────────────────────────

/*
KULLANICI ADI BOŞSA `x-access-token` KULLANILIR.

Token ile klonlarken kullanıcı adı zorunlu; GitHub token'ı kullanıcı adı
alanında bekliyor. Boş bırakılsaydı klonlama kimlik doğrulama hatasıyla
düşerdi ve sebebi "erişim tanımlı değil" gibi görünürdü — oysa tanımlı.
*/
func TestBuild_KullaniciAdiBosaVarsayilanTokenAdi(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)

	gp, err := f.git.Create(context.Background(), gitprovider.CreateInput{
		Type: gitprovider.TypeGitHub, Name: "GitHub", Secret: "ghp-test-token-123456",
	})
	require.NoError(t, err)
	require.Empty(t, gp.Username, "ön koşul: GitHub erişimi kullanıcı adı istemiyor")

	p := f.proje(t, projects.Input{GitProviderID: &gp.ID})
	a := f.agent(t, agentreg.CreateInput{DefaultModel: "m"})

	in, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Task: "iş",
	})
	require.NoError(t, err)
	require.Equal(t, "x-access-token", in.Repo.Username)
	require.NotEmpty(t, in.Repo.Secret, "gizli değer çözülmüş olmalı")
}

// AYNI KURAL `RepoAccess` yolunda da geçerli: gönderme ve PR açma oradan
// besleniyor ve iki yol aynı kimliği üretmek zorunda.
func TestRepoAccess_KullaniciAdiBosaVarsayilanTokenAdi(t *testing.T) {
	f := buildSetup(t)

	gp, err := f.git.Create(context.Background(), gitprovider.CreateInput{
		Type: gitprovider.TypeGitHub, Name: "GitHub", Secret: "ghp-test-token-123456",
	})
	require.NoError(t, err)

	p := f.proje(t, projects.Input{GitProviderID: &gp.ID})

	url, creds, err := f.b.RepoAccess(context.Background(), p.ID)
	require.NoError(t, err)
	require.Equal(t, p.RepoURL, url)
	require.NotNil(t, creds)
	require.Equal(t, "x-access-token", creds.Username)
}

// Kullanıcı adı VARSA korunur: Bitbucket kendi kullanıcı adını istiyor ve
// yerine token adı yazılsaydı doğrulama düşerdi.
func TestBuild_KullaniciAdiVarsaKorunur(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)

	gp, err := f.git.Create(context.Background(), gitprovider.CreateInput{
		Type: gitprovider.TypeBitbucket, Name: "Bitbucket",
		Username: "ahmet", Secret: "app-password-123456",
	})
	require.NoError(t, err)

	p := f.proje(t, projects.Input{GitProviderID: &gp.ID})
	a := f.agent(t, agentreg.CreateInput{DefaultModel: "m"})

	in, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Task: "iş",
	})
	require.NoError(t, err)
	require.Equal(t, "ahmet", in.Repo.Username)
}

/*
GİT ERİŞİMİ OLMAYAN PROJE YİNE ÇALIŞTIRILIR.

Açık bir depo klonlamak için kimlik gerekmiyor. Erişim zorunlu tutulsaydı,
yalnızca okuma yapan bir agent bile tanımsız bir kimlik yüzünden hiç
başlayamazdı.
*/
func TestBuild_GitErisimiYoksaKimliksizKlonlanir(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)
	p := f.proje(t, projects.Input{})
	a := f.agent(t, agentreg.CreateInput{DefaultModel: "m"})

	in, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Task: "iş",
	})
	require.NoError(t, err)
	require.Empty(t, in.Repo.Username)
	require.Empty(t, in.Repo.Secret)
	require.NotEmpty(t, in.Repo.URL, "adres yine de verilmeli")
}

// Ama `RepoAccess` YOLUNDA erişim zorunlu: orası gönderme ve PR içindir,
// ikisi de kimlik olmadan yapılamaz.
func TestRepoAccess_GitErisimiYoksaReddedilir(t *testing.T) {
	f := buildSetup(t)
	p := f.proje(t, projects.Input{})

	_, _, err := f.b.RepoAccess(context.Background(), p.ID)
	require.Error(t, err, "gönderme kimliksiz yapılamaz")
}

// ── Node sürümü ─────────────────────────────────────────────────────────────

// Sürüm kayda VE container'a aynı değerle gider; ikisi ayrışsaydı kayıt
// hangi sürümle koşulduğunu yanlış gösterirdi.
func TestBuild_NodeSurumuKaydaVeContainerAAyniGider(t *testing.T) {
	f := buildSetup(t)
	f.saglayici(t)
	p := f.proje(t, projects.Input{DefaultNodeVersion: "24.13.0"})
	a := f.agent(t, agentreg.CreateInput{DefaultModel: "m"})

	in, err := f.b.Build(context.Background(), Request{
		ProjectID: p.ID, AgentID: a.ID, Task: "iş",
	})
	require.NoError(t, err)
	require.Equal(t, "24.13.0", in.Create.NodeVersion)
	require.Equal(t, "24.13.0", in.NodeVersion)
}
