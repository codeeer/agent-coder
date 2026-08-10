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

/*
 * scriptsDir, hazır betiklerin container içindeki dizini.
 *
 * `/work` altında DEĞİL: orası klonlama hedefi ve boş olmak zorunda; ayrıca
 * bizim dosyalarımız kullanıcının diff'inde görünürdü (spec 012 K6, spec 003
 * davranış kuralı).
 *
 * Dizin imajda önceden açılır (`runner/Dockerfile`): tar kopyalaması dizin
 * oluşturmuyor.
 */
const scriptsDir = "/home/agent/scripts"

// scriptMode, betiklerin dosya izinleri.
//
// Diğer yapılandırma dosyaları 0o600; betik ÇALIŞTIRILABİLİR olmak zorunda.
const scriptMode int64 = 0o755

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

	providerCfg, err := buildConfig(p, a)
	if err != nil {
		return nil, err
	}

	files := []ConfigFile{
		{Path: configDir + "/opencode.json", Content: providerCfg, Mode: 0o600},
		{Path: configDir + "/agents/" + a.Slug + ".md", Content: buildAgentFile(a), Mode: 0o600},
	}

	/*
	 * Hazır betikler.
	 *
	 * GÜVENLİK KAPISI: yalnızca bash yetkisi AÇIKKEN yazılırlar (spec 012 K3).
	 *
	 * Yetkisi kapalıyken dosyayı yine de koymak teknik olarak zararsız olurdu —
	 * çalıştırılamazdı zaten. Ama orada durması yanlış bir izlenim verir ve bir
	 * sonraki geliştiriciyi "madem duruyor, izin de verelim" demeye davet eder.
	 * O izin, yetki eşleşmesi ham komut metnine yapıldığı için (`betik.sh; env`)
	 * kapalı bir kapıyı açmak olurdu.
	 *
	 * Bu özellik `BuildPermissions`'a HİÇ dokunmaz; "yeni yetenek açmıyor"
	 * iddiasının tek kanıtı budur.
	 */
	for _, s := range scriptsFor(a) {
		files = append(files, ConfigFile{
			Path:    scriptPath(s.Name),
			Content: []byte(s.Content),
			Mode:    scriptMode,
		})
	}

	return files, nil
}

// scriptsFor, agent'ın gerçekten kullanabileceği betikler.
//
// Tek kapı: bash yetkisi kapalıysa liste boştur. Hem dosya yazımı hem talimat
// metni buradan geçer ki ikisi ayrışmasın — agent'a çalıştıramayacağı bir betik
// anlatmak, onu var olmayan bir yolu denemeye iter.
func scriptsFor(a AgentSpec) []ScriptSpec {
	if !a.AllowBash {
		return nil
	}
	return a.Scripts
}

// scriptPath, bir betiğin container içindeki mutlak yolu.
func scriptPath(name string) string { return scriptsDir + "/" + name + ".sh" }

// mcpTimeoutMS, MCP sunucusuna bağlanma ve araç çağırma süre sınırı.
//
// Her sunucuda AÇIKÇA yazılır: çalıştırma motorunun belgeleri 5 saniye,
// kaynağı 30 saniye diyor. Varsayılana güvenmek, hangi değerin geçerli
// olduğunu sürüme bırakmak olurdu.
var mcpTimeoutMS = 30_000

// buildConfig, sağlayıcı ve MCP sunucularıyla motor yapılandırmasını üretir.
func buildConfig(p ProviderSpec, a AgentSpec) ([]byte, error) {
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

	if mcp := buildMCPConfig(a.MCPServers); len(mcp) > 0 {
		cfg["mcp"] = mcp
	}

	return json.MarshalIndent(cfg, "", "  ")
}

/*
 * MCP sunucuları.
 *
 * Yalnızca UZAK sunucular (spec 011 K2). Erişim anahtarı dosyaya YAZILMAZ:
 * sağlayıcı anahtarındaki desenin aynısıyla ortam değişkenine referans verilir.
 * Agent kendi container'ında bu dosyayı okuyabilir; okusa bile anahtarı göremez.
 */
func buildMCPConfig(servers []MCPServerSpec) map[string]any {
	if len(servers) == 0 {
		return nil
	}

	out := make(map[string]any, len(servers))
	for _, s := range servers {
		entry := map[string]any{
			"type":    "remote",
			"url":     s.URL,
			"enabled": true,
			"timeout": mcpTimeoutMS,
			// OAuth otomatik algılaması kapalı: tarayıcı akışı gerektiriyor ve
			// kimsenin izlemediği bir sandbox'ta sonsuza kadar beklerdi.
			"oauth": false,
		}
		if s.Secret != "" {
			entry["headers"] = map[string]any{
				"Authorization": "Bearer {env:" + MCPEnvVar(s.Name) + "}",
			}
		}
		out[s.Name] = entry
	}
	return out
}

// MCPEnvVar, bir MCP sunucusunun anahtarını taşıyan ortam değişkeni.
func MCPEnvVar(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - 32)
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return "AGENT_CODER_MCP_" + b.String()
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
	b.WriteString(scriptSection(a))

	return []byte(b.String())
}

/*
 * scriptSection, agent'a hangi betiklerin hazır olduğunu anlatan blok.
 *
 * Dosyayı container'a koymak YETMEZ: model, varlığını bilmediği bir dosyayı
 * çağırmaz. MCP araçlarından farkı bu — onlar modele araç olarak sunuluyor,
 * betikler ise sadece dosya. Bu yüzden liste talimatta yazmak ZORUNDA.
 *
 * Betik yoksa blok hiç yazılmaz: boş bir başlık modelin dikkatini boşa harcar.
 */
func scriptSection(a AgentSpec) string {
	list := scriptsFor(a)
	if len(list) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n## Kullanabileceğin betikler\n\n")
	b.WriteString("Aşağıdaki betikler önceden yazılmış ve gözden geçirilmiştir. ")
	b.WriteString("Açıklamasına uyan bir iş yapman gerekirse komutu kendin kurma, ")
	b.WriteString("ilgili betiği çalıştır — sonucun her seferinde aynı olması için.\n\n")

	for _, s := range list {
		b.WriteString("- `" + scriptPath(s.Name) + "`")
		// Açıklama, betiğin NE ZAMAN çağrılacağını anlatan tek ipucu; boşsa
		// satırı kırpmak yerine yalnızca yolu yazıyoruz.
		if d := oneLine(s.Description); d != "" {
			b.WriteString(" — " + d)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// oneLine, markdown liste öğesini bozmayacak tek satırlık metin üretir.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
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

	/*
	 * MCP araçları.
	 *
	 * Atanmış sunucuların araçları AÇIKÇA izinli yazılır.
	 *
	 * Toptan bir "geri kalan her şey yasak" kuralı BİLİNÇLİ OLARAK yok: kural
	 * sıralamasında ilk eşleşenin mi son eşleşenin mi kazandığı doğrulanmadı ve
	 * yanlış tahmin, ya tüm araçları kapatır ya da hiçbirini. Erişim zaten
	 * yapılandırmayla sınırlı — o dosyayı biz üretiyoruz ve yalnızca atanmış
	 * sunucular içinde.
	 *
	 * Bilinen açık uç: klonlanan deponun içindeki bir `.opencode/` yapılandırması
	 * kendi MCP sunucusunu tanımlayabilir. Bu, kullanıcının kendi deposu olduğu
	 * için bugün kabul edilebilir; çok kullanıcılı kuruluma geçilirken
	 * kapatılmalı (spec 011, açık uçlar).
	 */
	for _, s := range a.MCPServers {
		rules = append(rules, PermissionRule{
			Permission: mcpToolPattern(s.Name), Pattern: "*", Action: "allow",
		})
	}

	return rules
}

// mcpToolPattern, bir sunucunun tüm araçlarını kapsayan desen.
//
// Motor araçları `{sunucu}_{araç}` biçiminde adlandırıyor.
func mcpToolPattern(name string) string { return name + "_*" }

func defaultIfEmpty(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}
