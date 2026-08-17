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

	/*
	 * KindCertificate, bir veya birden çok X.509 sertifikası.
	 *
	 * ADI BİÇİM DEĞİL, ŞEY: saklanan değer normalleştirilmiş PEM'dir ama tip
	 * "pem" diye adlandırılmadı. PEM bir yazılış biçimi; ayarın ne olduğu ise
	 * sertifika. Kullanıcı DER veya PKCS#7 de verebiliyor (bkz. certfmt) —
	 * tip adı biçime bağlansaydı ilk başka biçimde yalan olurdu.
	 *
	 * Değer ÇOK SATIRLIDIR; arayüz denetimi bu tipten türetir.
	 */
	KindCertificate Kind = "certificate"

	/*
	 * KindHostList, satır başına bir domain taşıyan izin listesi.
	 *
	 * KindCertificate ile aynı adlandırma kuralı: tip biçime değil ŞEYE göre
	 * adlandırıldı. "multiline" denseydi tip, değerin ne olduğunu değil nasıl
	 * yazıldığını söylerdi ve ikinci bir çok satırlı ayarda anlamsızlaşırdı.
	 *
	 * Değer ÇOK SATIRLIDIR; arayüz denetimi bu tipten türetir.
	 * Dilbilgisi ve doğrulama `internal/hostlist` paketindedir — aynı kuralı
	 * çıkış kapısı da kullanıyor.
	 */
	KindHostList Kind = "host_list"
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
	KeyEngineLogPersist   = "runner.engine_log_persist"
	KeyEngineLogRetention = "runner.engine_log_retention_days"
	KeyEngineLogMaxKB     = "runner.engine_log_max_kb"

	KeyNPMRegistry     = "packages.npm_registry"
	KeyNPMUsername     = "packages.npm_username"
	KeyMavenRegistry   = "packages.maven_registry"
	KeyPackagesTimeout = "packages.timeout_seconds"

	KeyCorporateCA = "network.corporate_ca"
	// KeyEgressProxy, sandbox çıkış denetiminin ANA ANAHTARI (spec 020).
	// Boşsa denetim tamamen kapalıdır; whitelist bile yok sayılır.
	KeyEgressProxy = "network.proxy_url"
	// KeyAllowedHosts, sandbox'ın çıkabileceği domain'ler. Yalnızca
	// KeyEgressProxy doluyken anlam taşır.
	KeyAllowedHosts = "network.allowed_hosts"

	KeyRunTimeoutMinutes  = "runner.timeout_minutes"
	KeyMaxConcurrentRuns  = "runner.max_concurrent"
	KeyRunnerCPULimit     = "runner.cpu_limit"
	KeyRunnerMemoryLimitG = "runner.memory_limit_gb"
	KeyCloneDepth         = "runner.clone_depth"
	// KeyRepoSubdir açıkken proje `/work` yerine `/work/<repo-adı>` altına
	// klonlanır (spec 025). Kapalıyken bugünkü davranış.
	KeyRepoSubdir         = "runner.repo_subdir"
	KeyMaxPromptKB        = "runner.max_prompt_kb"
	KeyCatalogSyncHours   = "catalog.sync_interval_hours"
	KeyReportDefaultDays  = "reports.default_days"
	KeyReportTimezone     = "reports.timezone"
	KeyJiraPollMinutes    = "jira.poll_interval_minutes"
	KeyJiraScanLimit      = "jira.scan_limit"
	KeyMCPTimeoutSeconds  = "mcp.timeout_seconds"
	KeyBatchSafetyMinutes = "runner.batch_safety_interval_minutes"
)

// Gruplar — arayüzde başlık olarak kullanılır.
const (
	GroupRunner   = "runner"
	GroupCatalog  = "catalog"
	GroupReports  = "reports"
	GroupJira     = "jira"
	GroupMCP      = "mcp"
	GroupPackages = "packages"
	GroupNetwork  = "network"
)

// GroupLabels, grup kimliklerinin insan okunur karşılıkları.
var GroupLabels = map[string]string{
	GroupRunner:   "Çalıştırma",
	GroupCatalog:  "Model kataloğu",
	GroupReports:  "Rapor",
	GroupJira:     "Jira tetikleyici",
	GroupMCP:      "MCP Server",
	GroupPackages: "Package repository",
	GroupNetwork:  "Kurumsal ağ",
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
		Help: "Sınır doluyken elle başlatılan yeni işler beklemeye alınmaz, reddedilir. " +
			"Toplu çalıştırma bunun istisnasıdır: sıraya alınan işler bekler ve " +
			"slot boşaldıkça başlar. Sınırı düşürmek çalışan işleri kesmez.",
		Default: "3", Min: p(1), Max: p(20),
	},
	{
		/*
		 * Toplu iş kuyruğunun SİGORTASI — mekanizması değil.
		 *
		 * Kuyruk olay güdümlü çalışıyor: iş bitince ve slot boşalınca
		 * kendiliğinden ilerliyor. Bu tur yalnızca bir sinyalin her nasılsa
		 * kaçtığı durum için var ve kuyruğun donmuş kalabileceği AZAMİ süreyi
		 * belirliyor.
		 */
		Key: KeyBatchSafetyMinutes, Group: GroupRunner, Kind: KindInt,
		Label: "Toplu iş kuyruğu emniyet turu", Unit: "dakika",
		Help: "Toplu çalıştırma kuyruğu normalde olay güdümlüdür — bir iş bitince " +
			"sıradaki anında başlar. Bu tur yalnızca bir sinyalin kaçtığı duruma " +
			"karşı sigortadır ve kuyruğun en fazla ne kadar durabileceğini söyler. " +
			"Düşürmek boşta birkaç sorgu daha demek. Değişiklik SÜREN turu kesmez, " +
			"bir sonrakinden itibaren geçerli olur.",
		Default: "1", Min: p(1), Max: p(60),
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
		Label: "Repository clone depth", Unit: "commit",
		Help: "Kaç commit'lik geçmiş klonlanacak. 1 en hızlısıdır; agent'ın geçmişe " +
			"bakması gerekiyorsa arttırın.",
		Default: "1", Min: p(1), Max: p(1000),
	},
	/*
	 * Çalışma dizini yerleşimi — spec 025.
	 *
	 * VARSAYILAN KAPALI ve öyle kalmalı: açmak, çalışan her kurulumdaki proje
	 * yolunu değiştirir. Betikler `$PROJECT_DIR` okuduğu sürece iki yerleşimde
	 * de çalışır; yolu elle yazan bir betik varsa kırılır.
	 */
	{
		Key: KeyRepoSubdir, Group: GroupRunner, Kind: KindBool,
		Label: "Projeyi repo adlı klasöre klonla",
		// Yardım metninde backtick YOK: bu grup düz metin kullanıyor ve
		// arayüz markdown işlemiyor — işaretler kullanıcıya ham görünürdü.
		Help: "Açıkken proje /work/<repo-adı> altına, kapalıyken doğrudan " +
			"/work altına klonlanır. Repo adının klasör olmasını bekleyen " +
			"dış runbook ve CI betikleri için. Betikler yolu $PROJECT_DIR " +
			"değişkeninden okuduğu sürece her iki yerleşimde de çalışır.",
		Default: "false",
	},
	{
		Key: KeyMaxPromptKB, Group: GroupRunner, Kind: KindInt,
		Label: "Azami agent prompt boyutu", Unit: "KB",
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
		Label: "MCP Server süre sınırı", Unit: "saniye",
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
	/*
	 * Motor logları — koşunun ham teşhis katmanı.
	 *
	 * Runner container'ı geçici; loglar saklanmazsa koşu bitince kaybolur ve
	 * "neden başarısız oldu" sorusunun cevabı kalmaz. Saklama AÇIK geliyor
	 * çünkü asıl ihtiyaç duyulan an, kimsenin önceden açmayı düşünmediği
	 * başarısız koşudur.
	 */
	{
		Key: KeyEngineLogPersist, Group: GroupRunner, Kind: KindBool,
		Label: "Engine loglarını sakla",
		Help: "Çalıştırma bitince motorun ham logları veritabanına yazılır ve " +
			"koşu detayında görünür. Kapatılırsa loglar container ile birlikte " +
			"silinir ve sonradan incelenemez.",
		Default: "true",
	},
	{
		Key: KeyEngineLogRetention, Group: GroupRunner, Kind: KindInt,
		Label: "Engine logu saklama süresi", Unit: "gün",
		Help: "Bu süreden eski motor logları düzenli olarak silinir. " +
			"Çalıştırma kaydının kendisi silinmez, yalnızca ham logu.",
		Default: "7", Min: p(1), Max: p(365),
	},
	{
		Key: KeyEngineLogMaxKB, Group: GroupRunner, Kind: KindInt,
		Label: "Engine logu boyut sınırı", Unit: "KB",
		Help: "Kaynak başına saklanacak azami ham boyut. Aşılırsa SON kısım " +
			"korunur — hata genelde sonda olur — ve kayıt kırpılmış olarak " +
			"işaretlenir.",
		Default: "2048", Min: p(64), Max: p(65536),
	},

	{
		Key: KeyNPMRegistry, Group: GroupPackages, Kind: KindText, Optional: true,
		Label: "npm registry URL",
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

	{
		Key: KeyMavenRegistry, Group: GroupPackages, Kind: KindText, Optional: true,
		Label: "Maven repository URL",
		Help: "Kurumsal paket deposunun Maven adresi " +
			"(örn. https://nexus.sirket.local/repository/maven-public/). " +
			"Boş bırakılırsa Maven kendi genel deposunu kullanır. " +
			"Kimlik bilgisi npm ile ORTAKTIR; aşağıdaki kimlik doğrulama " +
			"bölümünden girilir.",
		Default: "",
	},

	/*
	 * Paket deposu süre sınırı — npm ve Maven için ORTAK.
	 *
	 * Neden ayar, neden sabit değil: ölçüldü ki paket yöneticilerinin
	 * varsayılanı tek istek için beş dakika ve üstüne birkaç kez yeniden
	 * deniyorlar; ulaşılamayan bir depoda çalıştırma dakikalarca sessizce
	 * bekliyor. Ama doğru değer kuruma göre değişiyor — iç ağdaki bir depo
	 * için kısa olan süre, yavaş bir hattın ucundaki depoda büyük bir paketi
	 * keserdi.
	 */
	{
		Key: KeyPackagesTimeout, Group: GroupPackages, Kind: KindInt,
		Label: "Paket deposu süre sınırı", Unit: "saniye",
		Help: "Paket deposuna yapılan tek bir isteğin azami süresi. " +
			"Depoya ulaşılamadığında çalıştırmanın dakikalarca beklememesi " +
			"için. npm ve Maven için aynı değer geçerlidir. Yavaş bağlantıda " +
			"büyük paketler kesiliyorsa yükseltin.",
		Default: "60", Min: p(5), Max: p(600),
	},

	/*
	 * Kurumsal kök sertifika.
	 *
	 * SIR DEĞİL, o yüzden şifreli kimlik deposunda değil burada: kök sertifika
	 * tanımı gereği her istemciye dağıtılan genel bir belgedir. Sır deposuna
	 * konsaydı maskelenir ve "hangi sertifika tanımlı" sorusu ekranda
	 * cevaplanamazdı.
	 *
	 * Boş bırakmak varsayılandır ve hiçbir şeyi değiştirmez.
	 */
	{
		Key: KeyCorporateCA, Group: GroupNetwork, Kind: KindCertificate, Optional: true,
		Label: "Kurumsal kök sertifika",
		Help: "SSL denetimi yapan ağlarda kurumun kök sertifikası. " +
			"Dosya seçebilir veya içeriğini yapıştırabilirsiniz; " +
			"zincir taşıyan dosyalarda ara sertifikalar da alınır. " +
			"Tanımlanırsa güvenilen kök listesine EKLENİR, genel sertifikalar " +
			"geçerli kalmaya devam eder. Boş bırakılırsa hiçbir şey değişmez.",
		Default: "",
	},

	/*
	 * Çıkış denetimi — spec 020.
	 *
	 * Proxy ANA ANAHTARDIR: boşken agent ortamı bugünkü gibi her adrese
	 * çıkabilir ve whitelist yok sayılır. Doluyken çıkış proxy'ye MECBUR olur
	 * (yalnızca yönlendirilmez — ölçüldü, yönlendirme atlanabiliyordu) ve
	 * whitelist devreye girer.
	 *
	 * Kimlik gömülü adres reddedilir: `putSetting` değeri loga yazıyor.
	 */
	{
		Key: KeyEgressProxy, Group: GroupNetwork, Kind: KindText, Optional: true,
		Label: "Çıkış proxy'si",
		Help: "Agent ortamının internete çıkarken kullanacağı proxy " +
			"(örnek: http://proxy.sirket.local:8080). Tanımlanırsa agent " +
			"ortamı YALNIZCA bu proxy üzerinden dışarı çıkabilir; başka yol " +
			"bırakılmaz. Boş bırakılırsa çıkış denetimi tamamen kapalıdır ve " +
			"aşağıdaki izinli domain listesi de uygulanmaz. " +
			"Adres kullanıcı adı/parola içeremez.",
		Default: "",
	},
	{
		Key: KeyAllowedHosts, Group: GroupNetwork, Kind: KindHostList, Optional: true,
		Label: "İzinli domain'ler",
		Help: "Agent ortamının çıkabileceği domain'ler, satır başına bir tane. " +
			"`ornek.com` yalnızca o adresi, `*.ornek.com` alt alan adlarını açar. " +
			"İzinli bir domain'e tüm portlar açıktır. Boş bırakılırsa domain " +
			"kısıtı uygulanmaz, ama çıkış yine proxy'den geçmek zorundadır. " +
			"YALNIZCA çıkış proxy'si tanımlıyken etkilidir.",
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
