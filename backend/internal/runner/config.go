package runner

import (
	"encoding/json"
	"fmt"
	"strings"
)

// APIKeyEnvVar, sağlayıcı anahtarının container içinde taşındığı ortam değişkeni.
//
// Anahtar yapılandırma DOSYASINA düz metin yazılmaz; dosya yalnızca bu değişkene
// referans verir. Böylece dosya container içinde okunsa bile anahtar görünmez.
const APIKeyEnvVar = "AGENT_CODER_PROVIDER_KEY"

// ConfigFile, container'a kopyalanacak tek bir dosya.
type ConfigFile struct {
	// Path, container içindeki mutlak yol.
	Path string
	// Content, dosya içeriği.
	Content []byte
	// Mode, dosya izinleri.
	Mode int64
}

// configDir, çalıştırma motorunun yapılandırmayı okuduğu global dizin.
//
// Klonlanan depoya YAZILMAZ: aksi halde bizim dosyalarımız kullanıcının diff'inde
// görünürdü (spec 003 davranış kuralı).
const configDir = "/home/agent/.config/opencode"

// BuildConfigFiles, bir çalıştırma için container'a kopyalanacak dosyaları üretir.
//
// Dosyalar container BAŞLATILMADAN ÖNCE kopyalanır: çalıştırma motoru agent
// tanımlarını yalnızca açılışta okur, sonradan yazılan dosyayı görmez (ölçüldü).
func BuildConfigFiles(p ProviderSpec, a AgentSpec) ([]ConfigFile, error) {
	if a.Slug == "" {
		return nil, fmt.Errorf("agent slug boş olamaz")
	}
	if p.Slug == "" {
		return nil, fmt.Errorf("sağlayıcı slug boş olamaz")
	}

	providerCfg, err := buildProviderConfig(p)
	if err != nil {
		return nil, err
	}

	return []ConfigFile{
		{Path: configDir + "/opencode.json", Content: providerCfg, Mode: 0o600},
		{Path: configDir + "/agents/" + a.Slug + ".md", Content: buildAgentFile(a), Mode: 0o600},
	}, nil
}

// buildProviderConfig, sağlayıcıya göre çalıştırma motoru yapılandırmasını üretir.
func buildProviderConfig(p ProviderSpec) ([]byte, error) {
	// Anahtar yerine ortam değişkeni referansı yazılır.
	options := map[string]any{
		"apiKey":  "{env:" + APIKeyEnvVar + "}",
		"timeout": 600000,
	}

	provider := map[string]any{"options": options}

	switch p.Kind {
	case "openrouter":
		// Bu sağlayıcı motorda yerleşik tanımlı; yalnızca anahtarı veriyoruz.
		options["baseURL"] = defaultIfEmpty(p.BaseURL, "https://openrouter.ai/api/v1")

	case "litellm", "openai_compatible":
		if p.BaseURL == "" {
			return nil, fmt.Errorf("%s sağlayıcısı için adres zorunlu", p.Kind)
		}
		options["baseURL"] = strings.TrimRight(p.BaseURL, "/")
		// Yerleşik olmayan sağlayıcılar OpenAI-uyumlu sürücüyle konuşur.
		provider["npm"] = "@ai-sdk/openai-compatible"
		provider["name"] = p.Slug

	default:
		return nil, fmt.Errorf("bilinmeyen sağlayıcı türü: %q", p.Kind)
	}

	cfg := map[string]any{
		"$schema":  "https://opencode.ai/config.json",
		"provider": map[string]any{p.Slug: provider},
	}

	return json.MarshalIndent(cfg, "", "  ")
}

// buildAgentFile, agent tanımını çalıştırma motorunun okuduğu markdown biçimine çevirir.
//
// Yetkiler burada YAZILMAZ; session açılışında permission kuralı olarak gönderilir
// (ölçüldü: o yol çalışıyor ve tek kaynak olması karışıklığı önlüyor).
func buildAgentFile(a AgentSpec) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString("description: " + yamlSafe(a.Description) + "\n")
	b.WriteString("mode: primary\n")
	b.WriteString("---\n\n")
	b.WriteString(a.Prompt)
	if !strings.HasSuffix(a.Prompt, "\n") {
		b.WriteString("\n")
	}

	return []byte(b.String())
}

// yamlSafe, tek satırlık bir YAML değeri üretir.
//
// Açıklama kullanıcı tarafından yazılıyor; satır sonu veya özel karakter
// frontmatter'ı bozarsa agent hiç yüklenmez.
func yamlSafe(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		s = "Agent"
	}
	// Tırnak içine al ve içerideki tırnakları kaçır.
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// PermissionRule, çalıştırma motoruna gönderilen tek bir yetki kuralı.
type PermissionRule struct {
	Permission string `json:"permission"`
	Pattern    string `json:"pattern"`
	Action     string `json:"action"`
}

// BuildPermissions, agent yetkilerini çalıştırma motorunun kurallarına çevirir.
//
// İki kural agent'tan bağımsız olarak HER ZAMAN eklenir: insan onayı bekleyen bir
// agent, kimsenin izlemediği bir sandbox'ta sonsuza kadar bekler ve slot'u işgal eder.
func BuildPermissions(a AgentSpec) []PermissionRule {
	rules := []PermissionRule{
		{Permission: "question", Pattern: "*", Action: "deny"},
		{Permission: "plan_enter", Pattern: "*", Action: "deny"},
	}

	if !a.AllowEdit {
		rules = append(rules,
			PermissionRule{Permission: "edit", Pattern: "*", Action: "deny"},
			PermissionRule{Permission: "write", Pattern: "*", Action: "deny"},
		)
	}
	if !a.AllowBash {
		rules = append(rules, PermissionRule{Permission: "bash", Pattern: "*", Action: "deny"})
	}
	if !a.AllowWebfetch {
		rules = append(rules, PermissionRule{Permission: "webfetch", Pattern: "*", Action: "deny"})
	}

	return rules
}

func defaultIfEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
