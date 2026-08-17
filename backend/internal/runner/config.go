package runner

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
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

	/*
	 * IsDir, girdinin bir DİZİN olduğunu söyler.
	 *
	 * ÖLÇÜLDÜ: dizin girdisi olmadan da çalışıyor — Docker eksik ara dizini
	 * kendisi `root:root 0755` olarak açıyor ve agent (uid 10001) onu
	 * listeleyip içindeki betiği çalıştırabiliyor.
	 *
	 * Yine de açıkça yazılıyor: o davranış Docker'ın belgelenmemiş bir ayrıntısı
	 * ve izinleri bir gün daralırsa agent dizine erişemez. Bu ürün o hatayı bir
	 * kez yaşadı (runner/Dockerfile: "Dosyanın yazılmış olması dizinin
	 * kullanılabilir olduğu anlamına gelmiyordu"). On satırlık bir girdi,
	 * belgelenmemiş davranışa olan bağımlılığı kaldırıyor.
	 */
	IsDir bool
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

/*
BuildConfigFiles, bir çalıştırma için container'a kopyalanacak dosyaları üretir.

Dosyalar container BAŞLATILMADAN ÖNCE kopyalanır: çalıştırma motoru agent
tanımlarını yalnızca açılışta okur, sonradan yazılan dosyayı görmez (ölçüldü).

projectDir DIŞARIDAN GELİYOR, burada hesaplanmıyor (spec 025): aynı değer
container'ın ortam değişkenine de yazılıyor ve ikisi ayrışırsa model ile
betikler farklı yola bakar — üstelik sessizce. Boş verilirse WorkRoot
kullanılır; çağıranın unutması çalıştırmayı düşürmez.
*/
func BuildConfigFiles(p ProviderSpec, a AgentSpec, model string, pkg PackageRegistry,
	projectDir string) ([]ConfigFile, error) {
	if a.Slug == "" {
		return nil, fmt.Errorf("agent slug boş olamaz")
	}
	if p.Slug == "" {
		return nil, fmt.Errorf("sağlayıcı slug boş olamaz")
	}

	providerCfg, err := buildConfig(p, a, model)
	if err != nil {
		return nil, err
	}

	files := []ConfigFile{
		{Path: configDir + "/opencode.json", Content: providerCfg, Mode: 0o600},
		{Path: configDir + "/agents/" + a.Slug + ".md",
			Content: buildAgentFile(a, pkg, projectDir), Mode: 0o600},
	}

	/*
	 * Kurumsal paket deposu.
	 *
	 * Dosya olarak yazılıyor, ORTAM DEĞİŞKENİ OLARAK DEĞİL: token `env`
	 * çıktısında görünmemeli ve agent o çıktıyı okuyabiliyor. `$HOME` altında,
	 * `/work`'ün dışında — yani kullanıcının diff'ine de düşmez.
	 *
	 * Aynı kalıp git token'ında da var: `git-credentials.sh` onu
	 * `~/.git-credentials` dosyasına yazıyor.
	 */
	if npmrc := buildNPMRC(pkg); npmrc != nil {
		files = append(files, ConfigFile{Path: npmrcPath, Content: npmrc, Mode: 0o600})
	}
	// Maven yapılandırması npm'den BAĞIMSIZ: bir kurum yalnızca birini
	// kurumsal depoya almış olabilir.
	if sx := buildSettingsXML(pkg); sx != nil {
		files = append(files, ConfigFile{Path: settingsXMLPath, Content: sx, Mode: 0o600})
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
	// Dizinler ÖNCE: tar akışı sırayla çıkarılıyor ve dosya kendi dizininden
	// önce gelirse ara dizini Docker açar — bizim sahiplik/izin ayarımız
	// uygulanmaz.
	for _, f := range kullanilanKlasorler(a) {
		files = append(files, ConfigFile{
			Path:  folderPath(f.Name),
			Mode:  0o755,
			IsDir: true,
		})
	}

	for _, s := range scriptsFor(a) {
		files = append(files, ConfigFile{
			Path:    scriptPath(s),
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

/*
scriptPath, bir betiğin container içindeki mutlak yolu.

TEK ÜRETİCİ: hem dosyayı yazan kod hem talimatı üreten kod buradan geçiyor.
İki ayrı üretici olsaydı biri klasörü unuttuğunda dosya köke yazılır, talimatta
alt dizin yazardı ve model var olmayan bir yolu denerdi — hata ancak
çalıştırma anında, üstelik "betik çalışmadı" kılığında görünürdü.
*/
func scriptPath(s ScriptSpec) string {
	if s.Folder == "" {
		return scriptsDir + "/" + s.Name + ".sh"
	}
	return scriptsDir + "/" + s.Folder + "/" + s.Name + ".sh"
}

// folderPath, bir kampanya klasörünün container içindeki dizini.
func folderPath(name string) string { return scriptsDir + "/" + name }

/*
kullanilanKlasorler, gerçekten betiği olan klasörleri sırayla döner.

BOŞ KLASÖR ATLANIR: kullanamayacağı bir dizini modele göstermek, onu var
olmayan bir adımı aramaya iter. Klasör agent'a atanmış ama içi boşsa (ya da
bash kapalı olduğu için betikler elenmişse) ortada anlatılacak bir şey yok.
*/
func kullanilanKlasorler(a AgentSpec) []FolderSpec {
	dolu := map[string]bool{}
	for _, s := range scriptsFor(a) {
		if s.Folder != "" {
			dolu[s.Folder] = true
		}
	}

	out := make([]FolderSpec, 0, len(a.ScriptFolders))
	for _, f := range a.ScriptFolders {
		if dolu[f.Name] {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// mcpTimeoutMS, MCP sunucusuna bağlanma ve araç çağırma süre sınırı.
//
// Her sunucuda AÇIKÇA yazılır: çalıştırma motorunun belgeleri 5 saniye,
// kaynağı 30 saniye diyor. Varsayılana güvenmek, hangi değerin geçerli
// olduğunu sürüme bırakmak olurdu.
var mcpTimeoutMS = 30_000

// buildConfig, sağlayıcı ve MCP sunucularıyla motor yapılandırmasını üretir.
func buildConfig(p ProviderSpec, a AgentSpec, model string) ([]byte, error) {
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

		/*
		 * Model AÇIKÇA tanımlanır — yerleşik olmayan sağlayıcının en kritik satırı.
		 *
		 * Motor, yerleşik sağlayıcıların model listesini kendi taşır; custom
		 * sağlayıcıda ise modeli yapılandırmada bekler. Tanımsız bir modelle mesaj
		 * gönderildiğinde sağlayıcıya HİÇ HTTP isteği atmadan iç hata veriyor ve
		 * bize yalnızca şu yansıyordu:
		 *     durum 500: {"name":"UnknownError","data":{"message":"Unexpected..."}}
		 *
		 * Teşhisi zor kılan şey buydu: adres, anahtar ve model elle denendiğinde
		 * 200 dönüyor, model listesi arayüzde görünüyor, ama sağlayıcının loguna
		 * hiçbir istek düşmüyordu — çünkü istek hiç çıkmıyordu.
		 *
		 * YALNIZCA seçili model yazılır, sağlayıcının tüm kataloğu değil:
		 * bir çalıştırma tek modelle koşuyor (sendMessage tek bir modelID
		 * gönderiyor), katalogun tamamını taşımak `runner` paketine katalog
		 * bağımlılığı sokardı ve katalog bayatsa var olmayan modeller yazılırdı.
		 */
		if model == "" {
			return nil, fmt.Errorf("%s sağlayıcısı için model zorunlu", p.Kind)
		}
		provider["models"] = map[string]any{model: map[string]any{}}

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
func buildAgentFile(a AgentSpec, pkg PackageRegistry, projectDir string) []byte {
	var b strings.Builder

	b.WriteString("---\n")
	b.WriteString("description: " + yamlSafe(a.Description) + "\n")
	b.WriteString("mode: primary\n")
	b.WriteString("---\n\n")
	b.WriteString(a.Prompt)
	if !strings.HasSuffix(a.Prompt, "\n") {
		b.WriteString("\n")
	}
	b.WriteString(scriptSection(a, projectDir))
	b.WriteString(packageSection(pkg))
	/*
	 * Java bölümü KOŞULSUZ yazılır.
	 *
	 * Runner, projenin hangi dilde olduğunu bilmiyor — ve bilemez, çünkü depo
	 * container başlatıldıktan sonra klonlanıyor. Bölümü "Maven tanımlıysa"
	 * gibi bir koşula bağlamak, kurumsal deposu olmayan bir kurulumda Java
	 * projesiyle çalışan agent'ı ikinci JDK'dan habersiz bırakırdı.
	 *
	 * Bedeli altı satırlık talimat; karşılığı agent'ın "bu proje Java 17
	 * istiyor ama ortamda 25 var" diye rapor edip durmaması.
	 */
	b.WriteString(javaSection())

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
func scriptSection(a AgentSpec, projectDir string) string {
	list := scriptsFor(a)
	if len(list) == 0 {
		return ""
	}

	// Çağıran vermemişse kök: yerleşim ayarı gelmeden önce yazılmış
	// çağrılar ve testler bugünkü metni aynen üretmeye devam eder.
	if projectDir == "" {
		projectDir = WorkRoot
	}

	var b strings.Builder
	b.WriteString("\n## Kullanabileceğin betikler\n\n")
	b.WriteString("Aşağıdaki betikler önceden yazılmış ve gözden geçirilmiştir. ")
	b.WriteString("Açıklamasına uyan bir iş yapman gerekirse komutu kendin kurma, ")
	b.WriteString("ilgili betiği çalıştır — sonucun her seferinde aynı olması için.\n\n")

	/*
	 * PROJE DİZİNİ. Betikler `$PROJECT_DIR` üzerinden çalışıyor ve modelin de
	 * projenin nerede olduğunu bilmesi gerekiyor — kaynağı okuyarak
	 * öğrenemez.
	 */
	b.WriteString("Proje `" + projectDir + "` altında; betikler bu yolu ")
	b.WriteString("`$PROJECT_DIR` değişkeninden okur. ")
	b.WriteString("Kalıcı olması gereken her değişiklik bu dizinin altında olmalı — ")
	b.WriteString("başka bir yere yazılan dosya değişiklik kaydına girmez.\n\n")

	// Önce klasörsüz betikler: bugünkü biçim aynen korunuyor.
	for _, s := range list {
		if s.Folder != "" {
			continue
		}
		b.WriteString(betikSatiri(s))
	}

	for _, f := range kullanilanKlasorler(a) {
		b.WriteString("\n### " + f.Name)
		if d := oneLine(f.Description); d != "" {
			b.WriteString(" — " + d)
		}
		b.WriteString("\n\n")
		b.WriteString("Dizin: `" + folderPath(f.Name) + "`\n")
		/*
		 * SIRA ANLATILIYOR, DAYATILMIYOR. Adımlar arasında karar veren sensin;
		 * ürün sırayı işletmiyor, yalnızca var olduğunu söylüyor (spec 022).
		 */
		b.WriteString("Adımlar numara sırasıyla yazılmıştır.\n\n")

		for _, s := range list {
			if s.Folder == f.Name {
				b.WriteString(betikSatiri(s))
			}
		}
	}

	return b.String()
}

// betikSatiri, tek bir betiğin liste satırı.
func betikSatiri(s ScriptSpec) string {
	out := "- `" + scriptPath(s) + "`"
	// Açıklama, betiğin NE ZAMAN çağrılacağını anlatan tek ipucu; boşsa
	// satırı kırpmak yerine yalnızca yolu yazıyoruz.
	if d := oneLine(s.Description); d != "" {
		out += " — " + d
	}
	return out + "\n"
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
	/*
	 * SORAN HER İZİN REDDEDİLİR.
	 *
	 * Koşu HEADLESS: soruyu cevaplayacak kimse yok. `ask` davranışı, motorun
	 * yanıt beklerken sonsuza kadar durmasına yol açıyor — çalıştırma 0
	 * token'da asılı kalıyor ve ancak zaman aşımıyla (varsayılan 30 dk)
	 * kapanıyor. ÖLÇÜLDÜ: agent `~/.npmrc` okumak isteyince motor
	 * `permission=external_directory … action=ask` yazıp bekledi; koşu
	 * dokuz dakika sonra hâlâ 0 token'daydı.
	 *
	 * `external_directory`: /work dışındaki bir yola erişim. Reddetmek ayrıca
	 * DOĞRU olan: agent'ın işi klonlanan depo; container'ın ev dizininde
	 * yapılandırma dosyalarımız ve kimlik bilgileri duruyor.
	 */
	rules := []PermissionRule{
		{Permission: "question", Pattern: "*", Action: "deny"},
		{Permission: "plan_enter", Pattern: "*", Action: "deny"},
		{Permission: "external_directory", Pattern: "*", Action: "deny"},
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

// npmrcPath, agent'ın npm yapılandırması.
//
// Kullanıcı seviyesinde (`$HOME/.npmrc`): agent nerede çalışırsa çalışsın
// geçerli. Motorun kendi çevrimdışı bayrağı ise imajda, motorun dizinine
// kapsanmış durumda (`runner/Dockerfile`) — ikisi çakışmaz.
const npmrcPath = "/home/agent/.npmrc"

/*
 * CACertPath, kurumsal kök sertifikanın container içindeki yolu.
 *
 * `/etc/ssl/certs` ALTINDA DEĞİL, `$HOME` altında — ve bu bilinçli: sistem
 * sertifika dizinine yazmak root ayrıcalığı ve dizin sahipliği varsayımları
 * getiriyordu. Sertifikayı gösteren üç ortam değişkeni mutlak yol aldığı için
 * dosyanın nerede durduğu önemli değil; önemli olan agent kullanıcısının onu
 * okuyabilmesi.
 *
 * Dışa açık çünkü ortam değişkenlerini kuran paket (opencode) da bu yolu
 * bilmek zorunda.
 */
const CACertPath = "/home/agent/kurumsal-ca.pem"

/*
 * CACertFile, kurumsal sertifikayı container'a yazılacak dosyaya çevirir.
 *
 * BuildConfigFiles'a parametre olarak EKLENMEDİ: o fonksiyonun imzası yirmiden
 * fazla yerden çağrılıyor ve sertifika, sağlayıcı/agent/model üçlüsüyle aynı
 * cümlenin parçası değil — bağımsız bir ortam gerçeği. Ayrı bir fonksiyon
 * ikisini de olduğu yerde bırakıyor.
 *
 * Mod 0644: dosya sır değil ve agent'ın çalıştırdığı her araç okuyabilmeli.
 */
func CACertFile(pemText string) (ConfigFile, bool) {
	if strings.TrimSpace(pemText) == "" {
		return ConfigFile{}, false
	}
	return ConfigFile{Path: CACertPath, Content: []byte(pemText), Mode: 0o644}, true
}

/*
 * buildNPMRC, kurumsal paket deposu yapılandırması.
 *
 * Adres yoksa nil döner ve dosya HİÇ yazılmaz: özellik kapalıyken container
 * bugünküyle birebir aynı davranır.
 *
 * Kimlik doğrulama opsiyonel — anonim okumaya açık depolar için yalnızca
 * registry satırı yazılır.
 */
func buildNPMRC(p PackageRegistry) []byte {
	if !p.Enabled() {
		return nil
	}

	var b strings.Builder
	b.WriteString("registry=" + p.NPMRegistry + "\n")

	if p.HasAuth() {
		/*
		 * `_auth` anahtarı ADRESE göre yazılır: npm kimlik bilgisini
		 * host+yol eşleşmesiyle seçer, genel bir `_auth` satırı başka
		 * kayıt defterlerine de gönderilirdi.
		 *
		 * Şema atılır ve sondaki `/` korunur — npm'in beklediği biçim bu.
		 *
		 * `always-auth` BİLEREK YAZILMIYOR: npm 9'da kaldırıldı ve runner
		 * npm 11 taşıyor. Yazılırsa npm HER çağrıda "Unknown user config"
		 * uyarısı basar; dosya kullanıcı seviyesinde olduğu için o uyarı
		 * agent'ın okuduğu her araç çıktısına düşer. Kimlik zaten `_auth`
		 * ile hem üstveri hem tarball isteğinde gidiyor (ölçüldü).
		 */
		anahtar := strings.TrimPrefix(strings.TrimPrefix(p.NPMRegistry, "https:"), "http:")
		if !strings.HasSuffix(anahtar, "/") {
			anahtar += "/"
		}
		kimlik := base64.StdEncoding.EncodeToString([]byte(p.Username + ":" + p.Token))
		b.WriteString(anahtar + ":_auth=" + kimlik + "\n")
	}

	/*
	 * Süre sınırı — ÖLÇÜLMÜŞ bir sorunun karşılığı.
	 *
	 * npm'in varsayılanı tek istek için 300 saniye ve üstüne iki kez daha
	 * deniyor. Kurumsal depoya ulaşılamayan bir çalıştırmada tek bir paket
	 * için ~4 dakika harcandığı ölçüldü (spec 017 doğrulaması); kullanıcı o
	 * süre boyunca ekranda yalnızca "çalışıyor" görüyordu.
	 *
	 * `fetch-retries=1`: kurumsal ağda depo ya ayaktadır ya değildir. Ölü bir
	 * adresi üç kez denemek sonucu değiştirmiyor, yalnızca koşunun süre
	 * bütçesini yakıyor.
	 */
	if p.TimeoutSec > 0 {
		b.WriteString("fetch-timeout=" + strconv.Itoa(p.TimeoutSec*1000) + "\n")
		b.WriteString("fetch-retries=1\n")
	}

	return []byte(b.String())
}

// settingsXMLPath, agent'ın Maven yapılandırması.
//
// Kullanıcı seviyesinde (`$HOME/.m2/settings.xml`): agent hangi dizinde
// çalışırsa çalışsın geçerli — `~/.npmrc` ile aynı karar.
const settingsXMLPath = "/home/agent/.m2/settings.xml"

/*
 * buildSettingsXML, kurumsal Maven deposu yapılandırması.
 *
 * Adres yoksa nil döner ve dosya HİÇ yazılmaz: özellik kapalıyken container
 * bugünküyle birebir aynı davranır (buildNPMRC'deki kararın aynısı).
 *
 * `mirrorOf=*` BU İŞİN CAN DAMARI. npm'de agent'ın kaçış yolu `--registry`
 * bayrağıydı; Maven'da kaçış yolu daha geniş: projenin kendi `pom.xml`'i
 * `<repositories>` ile başka bir depo ilan edebiliyor. `*` onu da kuruma
 * çeviriyor — aksi halde kapalı ağda derleme, sebebi anlaşılmayan bir zaman
 * aşımıyla düşerdi.
 *
 * XML `encoding/xml` ile üretiliyor, metin şablonuyla değil: parola bu dosyaya
 * giriyor ve kaçırılmamış tek bir `&` dosyayı sessizce bozardı.
 */
func buildSettingsXML(p PackageRegistry) []byte {
	if !p.MavenEnabled() {
		return nil
	}

	type server struct {
		ID       string `xml:"id"`
		Username string `xml:"username"`
		Password string `xml:"password"`
	}
	type mirror struct {
		ID       string `xml:"id"`
		MirrorOf string `xml:"mirrorOf"`
		URL      string `xml:"url"`
	}
	type settings struct {
		XMLName xml.Name `xml:"settings"`
		NS      string   `xml:"xmlns,attr"`
		Servers []server `xml:"servers>server,omitempty"`
		Mirrors []mirror `xml:"mirrors>mirror"`
	}

	const ayna = "kurumsal"
	s := settings{
		NS:      "http://maven.apache.org/SETTINGS/1.0.0",
		Mirrors: []mirror{{ID: ayna, MirrorOf: "*", URL: p.MavenRegistry}},
	}
	// Kimlik OPSİYONEL: anonim okumaya açık depolarda `<server>` hiç yazılmaz.
	// Boş kullanıcı adıyla bir sunucu bloğu yazmak, Maven'ı kimliksiz bir
	// kimlik doğrulama denemesine sokardı.
	if p.HasAuth() {
		s.Servers = []server{{ID: ayna, Username: p.Username, Password: p.Token}}
	}

	gövde, err := xml.MarshalIndent(s, "", "  ")
	if err != nil {
		// Yapı sabit; buraya düşmek programlama hatası olurdu. Yine de
		// çalıştırmayı düşürmek yerine yapılandırmasız devam edilir.
		return nil
	}
	return append([]byte(xml.Header), append(gövde, '\n')...)
}

/*
 * packageSection, agent'a paketlerin nereden geldiğini anlatan blok.
 *
 * Yapılandırmayı container'a koymak YETMEZ: model `.npmrc`'yi görmediği için
 * "kurulum başarısız, registry'yi değiştireyim" diyebilir ve kurumsal ağda
 * asla ulaşamayacağı bir adrese yönelir. Betiklerdeki karar burada da geçerli
 * (bkz. scriptSection): modele NE YAPMAYACAĞINI da söylüyoruz.
 *
 * Adres yoksa blok hiç yazılmaz — boş bir başlık modelin dikkatini harcar.
 */
func packageSection(p PackageRegistry) string {
	if !p.Enabled() && !p.MavenEnabled() {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n## Paket deposu\n\n")

	if p.Enabled() {
		b.WriteString("Bu ortamda npm paketleri kurumsal depodan çekiliyor: ")
		b.WriteString(p.NPMRegistry + "\n\n")
		b.WriteString("Yapılandırma `~/.npmrc` içinde hazır. `--registry` bayrağı verme, ")
		b.WriteString("`.npmrc` yazma veya değiştirme, kayıt defteri adresini değiştirme — ")
		b.WriteString("bu ağdan npm'in genel deposuna erişilemez. ")
		b.WriteString("Bir paket bulunamıyorsa kurumsal depoda gerçekten yok demektir; ")
		b.WriteString("başka bir kaynak denemek yerine bunu bildir.\n")
	}

	if p.MavenEnabled() {
		if p.Enabled() {
			b.WriteString("\n")
		}
		b.WriteString("Maven bağımlılıkları da kurumsal depodan çekiliyor: ")
		b.WriteString(p.MavenRegistry + "\n\n")
		/*
		 * Maven'da kaçış yolu npm'dekinden GENİŞ: `-s` ile başka bir
		 * yapılandırma vermek, `settings.xml`'i değiştirmek ve pom'a
		 * `<repositories>` eklemek. Üçü de ayrı ayrı yasaklanıyor; yalnızca
		 * "adresi değiştirme" demek yetmezdi.
		 */
		b.WriteString("Yapılandırma `~/.m2/settings.xml` içinde hazır ve tüm depoları ")
		b.WriteString("kuruma yönlendiriyor. `-s` veya `--settings` bayrağı verme, ")
		b.WriteString("`settings.xml` yazma veya değiştirme, `pom.xml` içine ")
		b.WriteString("`<repositories>` veya `<pluginRepositories>` ekleme — ")
		b.WriteString("bu ağdan Maven Central'a erişilemez. ")
		b.WriteString("Bir bağımlılık bulunamıyorsa kurumsal depoda gerçekten yok ")
		b.WriteString("demektir; başka bir kaynak denemek yerine bunu bildir.\n")
	}

	return b.String()
}

/*
 * javaSection, agent'a hangi Java sürümlerinin nerede olduğunu söyler.
 *
 * NEDEN TALİMATTA: Java sürümü projeden projeye değişiyor ama koşuda bir sürüm
 * seçici YOK (spec 018: Kapsam dışı). Seçim, agent'a bu bilgiyi vererek
 * yapılıyor. Yazılmasaydı agent ikinci sürümün varlığını bilemez, `java
 * -version` gördüğünü tek seçenek sanar ve "bu proje Java 17 istiyor ama
 * ortamda 25 var" diye rapor edip dururdu.
 *
 * Yollar MİMARİDEN BAĞIMSIZ sabit bağlar (`runner/Dockerfile`): Temurin'in
 * gerçek kurulum dizini `…-amd64` / `…-arm64` ile bitiyor ve talimata o yol
 * yazılsaydı imajın koştuğu mimariye göre yanlış olurdu.
 */
func javaSection() string {
	/*
	 * "java" ile "mvn" AYNI ŞEKİLDE sürüm değiştirmiyor — bu bir ölçümün
	 * kaydı. Gerçek bir koşuda agent `JAVA_HOME=/opt/java/17 java -version`
	 * çalıştırdı ve **25** aldı; sonra "dizin yok galiba" diye rapor etti.
	 *
	 * Sebep: `java` kabuktan `PATH` ile bulunuyor ve `PATH` imajda varsayılan
	 * JDK'ya işaret ediyor; `JAVA_HOME` onu etkilemiyor. Maven'ın başlatıcısı
	 * ise `JAVA_HOME`'u okuyor, yani `mvn` için çalışıyor.
	 *
	 * Talimat bu farkı AÇIKÇA söylemek zorunda: söylemezse agent, çalışan bir
	 * ortamı bozuk sanıp raporunu yanlış yazıyor.
	 */
	return "\n## Java\n\n" +
		"Ortamda iki JDK var:\n\n" +
		"- `/opt/java/25` — **varsayılan** (`java` ve `mvn` bunu kullanır)\n" +
		"- `/opt/java/17`\n\n" +
		"Başka bir sürüm gerekiyorsa:\n\n" +
		"- **Maven için** o komuta `JAVA_HOME` ver: `JAVA_HOME=/opt/java/17 mvn -B test`\n" +
		"- **Doğrudan `java` için** tam yolu kullan: `/opt/java/17/bin/java -version`\n\n" +
		"`JAVA_HOME=... java` ÇALIŞMAZ: `java` kabuktan `PATH` ile bulunur ve " +
		"`PATH` varsayılan JDK'ya işaret eder. Bu bir arıza değil.\n\n" +
		"Yeni bir JDK kurmaya çalışma; bu ağda indirilemez.\n"
}
