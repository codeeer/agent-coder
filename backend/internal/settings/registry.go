// Package settings, çalışma davranışını belirleyen parametreleri yönetir.
//
// Kural (spec 003 H7): davranışı belirleyen hiçbir parametre kodda gömülü kalmaz.
// Kodda yalnızca TANIMLAR durur — anahtar, varsayılan, geçerli aralık, açıklama;
// veritabanında ise yalnızca varsayılandan SAPAN değerler tutulur.
//
// Bu ayrım bilinçli: kod güncellendiğinde yeni varsayılanlar, kullanıcı elle
// değiştirmediği her ayarda kendiliğinden geçerli olur.
//
// Ortam değişkeni mi, ayar mı?
//   - Ortam değişkeni: veritabanına BAĞLANMAK için gerekenler ve dağıtım topolojisi
//     (DATABASE_URL, SECRET_ENCRYPTION_KEY, portlar, RUNNER_IMAGE, RUNNER_NETWORK).
//   - Ayar: çalışma davranışını belirleyenler (aşağıdaki kayıt defteri).
package settings

import "fmt"

// Kind, bir ayarın değer tipi.
type Kind string

const (
	KindInt  Kind = "int"
	KindBool Kind = "bool"
	KindText Kind = "text"
)

// Definition, bir ayarın kod tarafındaki tanımı.
// Arayüz bu bilgiden kendini çizer; yeni parametre eklemek frontend değişikliği gerektirmez.
type Definition struct {
	Key     string `json:"key"`
	Group   string `json:"group"`
	Label   string `json:"label"`
	Help    string `json:"help"`
	Kind    Kind   `json:"kind"`
	Default string `json:"default"`
	Unit    string `json:"unit,omitempty"`
	Min     *int   `json:"min,omitempty"`
	Max     *int   `json:"max,omitempty"`

	// Optional true ise BOŞ değer geçerlidir ve "bu özellik kapalı" demektir.
	//
	// Zorunlu bir ayarın boş bırakılması yapılandırma hatasıdır; kapalı bir
	// özellik ise normal bir durum. İkisi ayrı şeyler ve tek bir "boş olamaz"
	// kuralı ikisini birbirine karıştırıyordu.
	Optional bool `json:"optional,omitempty"`
}

// Ayar anahtarları. Kod bu sabitleri kullanır, düz metin yazmaz.
const (
	KeyNPMRegistry = "packages.npm_registry"
	KeyNPMUsername = "packages.npm_username"

	KeyRunTimeoutMinutes  = "runner.timeout_minutes"
	KeyMaxConcurrentRuns  = "runner.max_concurrent"
	KeyRunnerCPULimit     = "runner.cpu_limit"
	KeyRunnerMemoryLimitG = "runner.memory_limit_gb"
	KeyCloneDepth         = "runner.clone_depth"
	KeyMaxPromptKB        = "runner.max_prompt_kb"
	KeyCatalogSyncHours   = "catalog.sync_interval_hours"
	KeyReportDefaultDays  = "reports.default_days"
	KeyReportTimezone     = "reports.timezone"
	KeyJiraPollMinutes    = "jira.poll_interval_minutes"
	KeyJiraScanLimit      = "jira.scan_limit"
	KeyMCPTimeoutSeconds  = "mcp.timeout_seconds"
)

// Gruplar — arayüzde başlık olarak kullanılır.
const (
	GroupRunner   = "runner"
	GroupCatalog  = "catalog"
	GroupReports  = "reports"
	GroupJira     = "jira"
	GroupMCP      = "mcp"
	GroupPackages = "packages"
)

// GroupLabels, grup kimliklerinin insan okunur karşılıkları.
var GroupLabels = map[string]string{
	GroupRunner:   "Çalıştırma",
	GroupCatalog:  "Model kataloğu",
	GroupReports:  "Rapor",
	GroupJira:     "Jira tetikleyici",
	GroupMCP:      "Dış araçlar (MCP)",
	GroupPackages: "Paket deposu",
}

func p(v int) *int { return &v }

// Registry, tanımlı tüm ayarlar.
//
// YENİ PARAMETRE EKLEMEK: buraya tek bir satır. Migration gerekmez, frontend
// değişikliği gerekmez — arayüz listeyi buradan çizer.
var Registry = []Definition{
	{
		Key: KeyRunTimeoutMinutes, Group: GroupRunner, Kind: KindInt,
		Label: "Çalışma süre sınırı", Unit: "dakika",
		Help: "Bir agent çalıştırması bu süreyi aşarsa durdurulur ve " +
			"'zaman aşımı' olarak işaretlenir. O ana kadarki çıktı ve maliyet korunur.",
		Default: "30", Min: p(1), Max: p(240),
	},
	{
		Key: KeyMaxConcurrentRuns, Group: GroupRunner, Kind: KindInt,
		Label: "Aynı anda çalışabilecek iş", Unit: "iş",
		Help: "Sınır doluyken başlatılan yeni işler beklemeye alınmaz, reddedilir. " +
			"Sınırı düşürmek çalışan işleri kesmez.",
		Default: "3", Min: p(1), Max: p(20),
	},
	{
		Key: KeyRunnerCPULimit, Group: GroupRunner, Kind: KindInt,
		Label: "İş başına CPU sınırı", Unit: "çekirdek",
		Help:    "Her çalıştırma container'ının kullanabileceği azami çekirdek sayısı.",
		Default: "2", Min: p(1), Max: p(32),
	},
	{
		Key: KeyRunnerMemoryLimitG, Group: GroupRunner, Kind: KindInt,
		Label: "İş başına bellek sınırı", Unit: "GB",
		Help:    "Her çalıştırma container'ının kullanabileceği azami bellek.",
		Default: "4", Min: p(1), Max: p(64),
	},
	{
		Key: KeyCloneDepth, Group: GroupRunner, Kind: KindInt,
		Label: "Depo klonlama derinliği", Unit: "commit",
		Help: "Kaç commit'lik geçmiş klonlanacak. 1 en hızlısıdır; agent'ın geçmişe " +
			"bakması gerekiyorsa arttırın.",
		Default: "1", Min: p(1), Max: p(1000),
	},
	{
		Key: KeyMaxPromptKB, Group: GroupRunner, Kind: KindInt,
		Label: "Azami agent talimatı boyutu", Unit: "KB",
		Help:    "Bir agent'ın talimatı bu boyutu aşamaz.",
		Default: "32", Min: p(1), Max: p(256),
	},
	{
		Key: KeyCatalogSyncHours, Group: GroupCatalog, Kind: KindInt,
		Label: "Katalog yenileme aralığı", Unit: "saat",
		Help: "Model kataloğu açılışta bir kez, sonra bu aralıkla kendiliğinden " +
			"tazelenir. Elle yenileme her zaman mümkün.",
		Default: "24", Min: p(1), Max: p(720),
	},
	{
		Key: KeyReportDefaultDays, Group: GroupReports, Kind: KindInt,
		Label: "Varsayılan rapor dönemi", Unit: "gün",
		Help: "Rapor sayfası açıldığında hangi dönemin gösterileceği. " +
			"Sayfadan tek seferlik başka bir dönem de seçilebilir.",
		Default: "30", Min: p(1), Max: p(365),
	},
	{
		Key: KeyReportTimezone, Group: GroupReports, Kind: KindText,
		Label: "Rapor saat dilimi",
		Help: "Günlük kırılımın hangi takvime göre bölüneceği (IANA adı, " +
			"örn. Europe/Istanbul). Tanınmayan bir ad girilirse UTC kullanılır.",
		Default: "Europe/Istanbul",
	},
	{
		Key: KeyJiraPollMinutes, Group: GroupJira, Kind: KindInt,
		Label: "Jira tarama aralığı", Unit: "dakika",
		Help: "Jira tetikleyicisi olan akışlar bu aralıkla taranır. Webhook " +
			"tanımlıysa tarama yedek yoldur; ikisi aynı korumadan geçer.",
		Default: "5", Min: p(1), Max: p(1440),
	},
	{
		Key: KeyJiraScanLimit, Group: GroupJira, Kind: KindInt,
		Label: "Tarama başına azami task", Unit: "task",
		Help: "Bir taramada en fazla kaç task işleneceği. Geniş bir JQL'in " +
			"yüzlerce akış başlatmasını engeller.",
		Default: "20", Min: p(1), Max: p(200),
	},
	{
		Key: KeyMCPTimeoutSeconds, Group: GroupMCP, Kind: KindInt,
		Label: "MCP sunucu süre sınırı", Unit: "saniye",
		Help: "Bir dış araç sunucusuna bağlanma ve araç çağırma süresi. " +
			"Değer her sunucuya AÇIKÇA yazılır; çalıştırma motorunun kendi " +
			"varsayılanı sürümden sürüme değişebiliyor.",
		Default: "30", Min: p(5), Max: p(300),
	},

	/*
	 * Kurumsal paket deposu (Nexus, Artifactory, Verdaccio…).
	 *
	 * Boş = kapalı: agent npm'in kendi kayıt defterini kullanır. Doluysa
	 * container'a `~/.npmrc` yazılır ve agent'ın talimatına da yazılır —
	 * modelin bilmediği bir yapılandırmayı bozması işten değil.
	 *
	 * Kimlik bilgisi BURADA DEĞİL: token bir sırdır, `credentials` tablosunda
	 * şifreli durur (kind: nexus). Anonim okumaya açık depolarda hiç
	 * tanımlanmaz.
	 */
	{
		Key: KeyNPMRegistry, Group: GroupPackages, Kind: KindText, Optional: true,
		Label: "npm kayıt defteri adresi",
		Help: "Kurumsal paket deposunun npm adresi " +
			"(örn. https://nexus.sirket.local/repository/npm-group/). " +
			"Boş bırakılırsa npm'in kendi kayıt defteri kullanılır. " +
			"Kimlik bilgisi adrese GÖMÜLMEZ; aşağıdaki kimlik doğrulama " +
			"bölümünden girilir.",
		Default: "",
	},
	{
		Key: KeyNPMUsername, Group: GroupPackages, Kind: KindText, Optional: true,
		Label: "Kullanıcı adı",
		Help: "Kayıt defteri kimlik doğrulama istiyorsa kullanıcı adı. " +
			"Parola/token buraya değil, kimlik doğrulama bölümüne girilir.",
		Default: "",
	},
}

// byKey, hızlı arama için kayıt defterinin haritası.
var byKey = func() map[string]Definition {
	m := make(map[string]Definition, len(Registry))
	for _, d := range Registry {
		if _, dup := m[d.Key]; dup {
			// Kayıt defteri kod sabitidir; çakışma programlama hatasıdır.
			panic(fmt.Sprintf("settings: yinelenen anahtar %q", d.Key))
		}
		m[d.Key] = d
	}
	return m
}()

// Lookup, anahtarın tanımını döner.
func Lookup(key string) (Definition, bool) {
	d, ok := byKey[key]
	return d, ok
}
