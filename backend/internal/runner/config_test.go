package runner

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func findFile(t *testing.T, files []ConfigFile, suffix string) ConfigFile {
	t.Helper()
	for _, f := range files {
		if strings.HasSuffix(f.Path, suffix) {
			return f
		}
	}
	t.Fatalf("%q ile biten dosya üretilmedi", suffix)
	return ConfigFile{}
}

func TestBuildConfigFiles_AnahtarDosyayaDuzMetinYazilmaz(t *testing.T) {
	// En kritik test: anahtar yapılandırma dosyasına gömülmemeli, yalnızca
	// ortam değişkenine referans verilmeli.
	const gizli = "sk-or-v1-cok-gizli-anahtar-fd36"

	files, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter", APIKey: gizli},
		AgentSpec{Slug: "reviewer", Description: "İnceler", Prompt: "Sen incelemecisin."},
	)
	require.NoError(t, err)

	for _, f := range files {
		require.NotContains(t, string(f.Content), gizli,
			"%s dosyasında anahtar düz metin geçiyor", f.Path)
		require.NotContains(t, string(f.Content), "sk-or",
			"%s dosyasında anahtarın öneki bile geçmemeli", f.Path)
	}

	cfg := findFile(t, files, "opencode.json")
	require.Contains(t, string(cfg.Content), "{env:"+APIKeyEnvVar+"}")
}

func TestBuildConfigFiles_OpenRouter(t *testing.T) {
	files, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter"},
		AgentSpec{Slug: "reviewer", Prompt: "x"},
	)
	require.NoError(t, err)

	var cfg struct {
		Provider map[string]struct {
			NPM     string `json:"npm"`
			Options struct {
				APIKey  string `json:"apiKey"`
				BaseURL string `json:"baseURL"`
			} `json:"options"`
		} `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(findFile(t, files, "opencode.json").Content, &cfg))

	p := cfg.Provider["openrouter"]
	require.Equal(t, "https://openrouter.ai/api/v1", p.Options.BaseURL)
	require.Empty(t, p.NPM, "yerleşik sağlayıcı için npm sürücüsü belirtilmemeli")
}

func TestBuildConfigFiles_OzelSaglayiciOpenAIUyumluSurucuKullanir(t *testing.T) {
	for _, kind := range []string{"litellm", "openai_compatible"} {
		t.Run(kind, func(t *testing.T) {
			files, err := BuildConfigFiles(
				ProviderSpec{Slug: "sirket-litellm", Kind: kind, BaseURL: "https://llm.sirket.local/v1/"},
				AgentSpec{Slug: "coder", Prompt: "x"},
			)
			require.NoError(t, err)

			var cfg struct {
				Provider map[string]struct {
					NPM     string `json:"npm"`
					Name    string `json:"name"`
					Options struct {
						BaseURL string `json:"baseURL"`
					} `json:"options"`
				} `json:"provider"`
			}
			require.NoError(t, json.Unmarshal(findFile(t, files, "opencode.json").Content, &cfg))

			p := cfg.Provider["sirket-litellm"]
			require.Equal(t, "@ai-sdk/openai-compatible", p.NPM)
			require.Equal(t, "https://llm.sirket.local/v1", p.Options.BaseURL,
				"sondaki eğik çizgi kırpılmalı")
			require.Equal(t, "sirket-litellm", p.Name)
		})
	}
}

func TestBuildConfigFiles_OzelSaglayiciAdressizReddedilir(t *testing.T) {
	_, err := BuildConfigFiles(
		ProviderSpec{Slug: "x", Kind: "litellm"},
		AgentSpec{Slug: "coder", Prompt: "x"},
	)
	require.Error(t, err)
}

func TestBuildConfigFiles_BilinmeyenTurReddedilir(t *testing.T) {
	_, err := BuildConfigFiles(
		ProviderSpec{Slug: "x", Kind: "uydurma", BaseURL: "https://x/v1"},
		AgentSpec{Slug: "coder", Prompt: "x"},
	)
	require.Error(t, err)
}

func TestBuildConfigFiles_BosSlugReddedilir(t *testing.T) {
	_, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter"},
		AgentSpec{Slug: "", Prompt: "x"},
	)
	require.Error(t, err)

	_, err = BuildConfigFiles(
		ProviderSpec{Slug: "", Kind: "openrouter"},
		AgentSpec{Slug: "a", Prompt: "x"},
	)
	require.Error(t, err)
}

func TestBuildAgentFile_Bicim(t *testing.T) {
	files, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter"},
		AgentSpec{Slug: "reviewer", Description: "Kodu inceler", Prompt: "Sen bir incelemecisin."},
	)
	require.NoError(t, err)

	md := string(findFile(t, files, "reviewer.md").Content)
	require.True(t, strings.HasPrefix(md, "---\n"), "frontmatter ile başlamalı")
	require.Contains(t, md, `description: "Kodu inceler"`)
	require.Contains(t, md, "mode: primary")
	require.Contains(t, md, "Sen bir incelemecisin.")
	require.True(t, strings.HasSuffix(md, "\n"))
}

func TestBuildAgentFile_CokSatirliAciklamaFrontmatteriBozmaz(t *testing.T) {
	// Açıklamayı kullanıcı yazıyor; satır sonu frontmatter'ı bozarsa agent
	// hiç yüklenmez ve çalıştırma sessizce yanlış davranır.
	files, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter"},
		AgentSpec{
			Slug:        "x",
			Description: "Birinci satır\nikinci satır\r\nüçüncü \"tırnaklı\" satır",
			Prompt:      "talimat",
		},
	)
	require.NoError(t, err)

	md := string(findFile(t, files, "x.md").Content)

	// Frontmatter tam olarak iki '---' satırı içermeli.
	require.Equal(t, 2, strings.Count(md, "---\n"))

	desc := ""
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "description: ") {
			desc = line
		}
	}
	require.NotEmpty(t, desc)
	require.NotContains(t, desc[len("description: "):], "\n")
	require.Contains(t, desc, `\"tırnaklı\"`, "tırnaklar kaçırılmalı")
}

func TestBuildAgentFile_BosAciklamaVarsayilanAlir(t *testing.T) {
	files, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter"},
		AgentSpec{Slug: "x", Description: "   ", Prompt: "talimat"},
	)
	require.NoError(t, err)
	require.Contains(t, string(findFile(t, files, "x.md").Content), `description: "Agent"`)
}

func TestBuildConfigFiles_KullanicininDeposunaYazilmaz(t *testing.T) {
	files, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter"},
		AgentSpec{Slug: "x", Prompt: "y"},
	)
	require.NoError(t, err)

	for _, f := range files {
		require.False(t, strings.HasPrefix(f.Path, "/work"),
			"yapılandırma klonlanan depoya yazılmamalı, diff'e karışır: %s", f.Path)
		require.True(t, strings.HasPrefix(f.Path, "/home/agent/.config/"))
	}
}

func TestBuildPermissions(t *testing.T) {
	has := func(rules []PermissionRule, perm string) bool {
		for _, r := range rules {
			if r.Permission == perm && r.Action == "deny" {
				return true
			}
		}
		return false
	}

	t.Run("her zaman insan etkileşimi kapatılır", func(t *testing.T) {
		// Onay bekleyen bir agent sandbox'ta sonsuza kadar bekler.
		rules := BuildPermissions(AgentSpec{AllowEdit: true, AllowBash: true, AllowWebfetch: true})
		require.True(t, has(rules, "question"))
		require.True(t, has(rules, "plan_enter"))
	})

	t.Run("tam yetkili agent", func(t *testing.T) {
		rules := BuildPermissions(AgentSpec{AllowEdit: true, AllowBash: true, AllowWebfetch: true})
		require.False(t, has(rules, "edit"))
		require.False(t, has(rules, "write"))
		require.False(t, has(rules, "bash"))
		require.False(t, has(rules, "webfetch"))
	})

	t.Run("salt okunur agent", func(t *testing.T) {
		rules := BuildPermissions(AgentSpec{AllowEdit: false, AllowBash: true, AllowWebfetch: false})
		require.True(t, has(rules, "edit"))
		require.True(t, has(rules, "write"), "edit kapalıysa write da kapalı olmalı")
		require.False(t, has(rules, "bash"))
		require.True(t, has(rules, "webfetch"))
	})

	t.Run("hiçbir yetkisi olmayan agent", func(t *testing.T) {
		rules := BuildPermissions(AgentSpec{})
		for _, perm := range []string{"edit", "write", "bash", "webfetch", "question", "plan_enter"} {
			require.True(t, has(rules, perm), "%s kapalı olmalı", perm)
		}
	})

	t.Run("her kural geçerli biçimde", func(t *testing.T) {
		for _, r := range BuildPermissions(AgentSpec{}) {
			require.NotEmpty(t, r.Permission)
			require.Equal(t, "*", r.Pattern)
			require.Equal(t, "deny", r.Action)
		}
	})
}

/* ── MCP (spec 011) ──────────────────────────────────────────────────────── */

func mcpAgent(servers ...MCPServerSpec) AgentSpec {
	return AgentSpec{
		Slug: "coder", Description: "kod yazar", Prompt: "yaz",
		AllowEdit: true, AllowBash: true, AllowWebfetch: true,
		MCPServers: servers,
	}
}

func configJSON(t *testing.T, a AgentSpec) map[string]any {
	t.Helper()
	files, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter"}, a)
	require.NoError(t, err)

	var cfg map[string]any
	require.NoError(t, json.Unmarshal(files[0].Content, &cfg))
	return cfg
}

func TestBuildConfigFiles_MCPSunucusuYokkaBlokYazilmaz(t *testing.T) {
	cfg := configJSON(t, mcpAgent())
	_, has := cfg["mcp"]
	require.False(t, has, "sunucu yoksa boş bir mcp bloğu yazılmamalı")
}

func TestBuildConfigFiles_MCPSunucusuYazilir(t *testing.T) {
	cfg := configJSON(t, mcpAgent(MCPServerSpec{
		Name: "sentry", Transport: "http", URL: "https://mcp.sentry.dev/mcp", Secret: "gizli",
	}))

	mcp, ok := cfg["mcp"].(map[string]any)
	require.True(t, ok, "mcp bloğu yazılmalı")

	entry, ok := mcp["sentry"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "remote", entry["type"])
	require.Equal(t, "https://mcp.sentry.dev/mcp", entry["url"])
	require.Equal(t, true, entry["enabled"])
	// Süre sınırı AÇIKÇA yazılmalı: motorun varsayılanı sürüme göre değişiyor.
	require.NotNil(t, entry["timeout"], "her sunucuda süre sınırı yazılmalı")
	// Tarayıcı akışı kimsenin izlemediği bir sandbox'ta sonsuza kadar beklerdi.
	require.Equal(t, false, entry["oauth"])
}

// TestBuildConfigFiles_MCPAnahtariDosyayaYazilmaz — sızıntı testi.
//
// Agent kendi container'ında bu dosyayı okuyabilir; okusa bile anahtarı
// görmemeli (spec 011 K5).
func TestBuildConfigFiles_MCPAnahtariDosyayaYazilmaz(t *testing.T) {
	const secret = "sk-cok-gizli-anahtar-9876"

	files, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter"},
		mcpAgent(MCPServerSpec{
			Name: "sentry", Transport: "http", URL: "https://x.dev/mcp", Secret: secret,
		}))
	require.NoError(t, err)

	for _, f := range files {
		require.NotContains(t, string(f.Content), secret,
			"anahtar %s dosyasında düz metin görünmemeli", f.Path)
	}
	require.Contains(t, string(files[0].Content), "{env:AGENT_CODER_MCP_SENTRY}",
		"dosya anahtara ortam değişkeniyle referans vermeli")
}

func TestBuildConfigFiles_AnahtarsizSunucudaBaslikYok(t *testing.T) {
	cfg := configJSON(t, mcpAgent(MCPServerSpec{
		Name: "acik", Transport: "http", URL: "https://x.dev/mcp",
	}))

	entry := cfg["mcp"].(map[string]any)["acik"].(map[string]any)
	_, has := entry["headers"]
	require.False(t, has, "anahtarsız sunucuda erişim başlığı yazılmamalı")
}

// TestBuildPermissions_MCPAraclariAcikcaIzinli — atanmış sunucunun araçları
// açıkça izinli olmalı; atanmamış bir sunucunun deseni hiç görünmemeli.
func TestBuildPermissions_MCPAraclariAcikcaIzinli(t *testing.T) {
	rules := BuildPermissions(mcpAgent(
		MCPServerSpec{Name: "sentry", Transport: "http", URL: "https://x.dev"},
	))

	var found bool
	for _, r := range rules {
		require.NotEqual(t, "notion_*", r.Permission, "atanmamış sunucu kuralda görünmemeli")
		if r.Permission == "sentry_*" {
			found = true
			require.Equal(t, "allow", r.Action)
		}
	}
	require.True(t, found, "atanmış sunucunun araçları izinli yazılmalı")
}
