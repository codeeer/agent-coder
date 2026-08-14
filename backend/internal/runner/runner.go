// Package runner, bir agent'ı izole bir sandbox içinde çalıştırır.
//
// KRİTİK SINIR: opencode'a dair hiçbir tip, sabit veya varsayım bu dosyada geçmez.
// Sistemin geri kalanı yalnızca Runner arayüzünü bilir. Bu sınır, ileride opencode'u
// kendi motorumuzla değiştirebilmemizin tek garantisidir.
package runner

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Runner, bir agent çalıştırmasını yürütür.
type Runner interface {
	// Run, çalıştırmayı yürütür ve ilerlemeyi emit ile bildirir.
	//
	// ctx iptal edilirse çalıştırma durdurulur ve ErrCancelled döner.
	// Uygulama, hangi yolla çıkarsa çıksın (başarı, hata, panik, iptal, zaman aşımı)
	// oluşturduğu tüm kaynakları temizlemekle yükümlüdür.
	Run(ctx context.Context, req Request, emit EventFunc) (*Result, error)
}

// EventFunc, çalıştırma sırasındaki ilerlemeyi alır.
// Uygulama bunu birden fazla goroutine'den ÇAĞIRMAZ; sıralı çağrılır.
type EventFunc func(Event)

// Level, olay önem düzeyi.
type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Event, çalıştırma sırasındaki bir ilerleme bildirimi.
type Event struct {
	Level   Level
	Message string
}

// Request, bir çalıştırmanın tüm girdisi.
type Request struct {
	RunID    uuid.UUID
	Repo     RepoSpec
	Agent    AgentSpec
	Provider ProviderSpec
	Model    string
	Task     string

	// Packages, kurumsal paket deposu yapılandırması. Boşsa agent npm'in
	// kendi kayıt defterini kullanır.
	Packages PackageRegistry

	/*
	 * CACert, kurumsal kök sertifikanın PEM İÇERİĞİ — dosya yolu değil.
	 *
	 * İÇERİK TAŞINIYOR çünkü sertifika artık host'ta bir dosya olmak zorunda
	 * değil: arayüzden girilebiliyor ve container'a diğer yapılandırma
	 * dosyaları gibi KOPYALANIYOR. Yol taşınsaydı dosyanın Docker host'unda
	 * bulunması gerekirdi — uzak bir Docker host'ta çalışmayan, üstelik
	 * ürünün kendi üretim tavsiyesiyle çelişen bir varsayım (spec 017).
	 *
	 * Boşsa hiçbir dosya yazılmaz ve hiçbir ortam değişkeni tanımlanmaz.
	 */
	CACert string

	// Egress, sandbox çıkış denetimi (spec 020).
	Egress EgressSpec

	// NodeVersion, sandbox'ın koşacağı Node sürümü. Boşsa taban imaj kullanılır.
	//
	// Sürüm KOŞU ANINDA İNDİRİLMEZ: her desteklenen sürümün imajı derleme
	// anında hazırlanmıştır (bkz. ImageFor, node-versions.txt).
	NodeVersion string

	// EngineLogs, motorun ham loglarını alır — koşu NASIL biterse bitsin
	// çağrılır. nil olabilir (saklama kapalıysa toplama da yapılmaz).
	//
	// Callback olarak taşınıyor çünkü `Run` başarısızlıkta nil `Result`
	// döndürüyor; oysa loglara asıl ihtiyaç duyulan an odur.
	EngineLogs EngineLogFunc

	// Timeout, çalıştırmanın azami süresi. Ayarlardan gelir.
	Timeout time.Duration

	// Limits, container kaynak sınırları. Ayarlardan gelir.
	Limits Limits
}

/*
 * EgressSpec, çıkış denetiminin bir çalıştırmadaki hâli (spec 020).
 *
 * Değerler ÇALIŞTIRMA BAŞINDA dondurulur, canlı okunmaz: ayar iş sürerken
 * değişirse çalıştırma başladığı kurallarla bitmeli. Yarı yolda kural değişmesi
 * "neden düştü" sorusunu cevaplanamaz yapardı.
 */
type EgressSpec struct {
	/*
	 * ProxyURL, ANA ANAHTAR. Boşsa denetim tamamen kapalıdır: runner bugünkü
	 * ağda doğar, hiçbir proxy değişkeni yazılmaz, whitelist yok sayılır.
	 */
	ProxyURL string

	/*
	 * AllowedHosts, kullanıcının whitelist'i — HAM METİN, satır başına bir
	 * domain.
	 *
	 * BOŞ OLMASI KISIT YOKLUĞUDUR. Bu yüzden zorunlu adresler buraya
	 * körlemesine EKLENEMEZ: boş bir listeye ekleme yapmak listeyi boş
	 * olmaktan çıkarır ve kısıtsız olması gereken bir çalıştırmayı yalnızca
	 * o adreslerle sınırlardı.
	 */
	AllowedHosts string

	/*
	 * Required, ürünün çalışması için her hâlükârda gereken adresler:
	 * LLM provider, git repository, tanımlıysa registry'ler. Yalnızca
	 * AllowedHosts DOLUYKEN listeye eklenir.
	 */
	Required []string
}

// Enabled, çıkış denetiminin bu çalıştırmada açık olup olmadığı.
func (e EgressSpec) Enabled() bool { return strings.TrimSpace(e.ProxyURL) != "" }

/*
 * PackageRegistry, agent'ın bağımlılıkları nereden çekeceği.
 *
 * Kurumsal ağlarda paketler internet yerine iç bir depodan (Nexus,
 * Artifactory…) gelir. Adres boşsa özellik kapalıdır ve hiçbir yapılandırma
 * yazılmaz — bugünkü davranış.
 *
 * Kimlik doğrulama OPSİYONEL: anonim okumaya açık depolarda kullanıcı adı ve
 * token boş kalır.
 */
type PackageRegistry struct {
	/*
	 * MavenRegistry, kurumsal Maven deposunun adresi. Boşsa Maven
	 * yapılandırması HİÇ yazılmaz ve davranış bugünküyle aynı kalır.
	 */
	MavenRegistry string

	/*
	 * TimeoutSec, paket deposu çağrılarının bekleme süresi — npm ve Maven
	 * için ORTAK.
	 *
	 * Neden var: paket yöneticilerinin varsayılanı tek istek için beş dakika
	 * ve üstüne birkaç kez yeniden deniyorlar. Ulaşılamayan bir depoda
	 * çalıştırma dakikalarca sessizce bekliyor, kullanıcı ekranda yalnızca
	 * "çalışıyor" görüyor ve koşunun süre bütçesi boşa gidiyor (spec 017
	 * doğrulamasında ~4 dakika ölçüldü).
	 *
	 * Sıfırsa süre satırları yazılmaz — paket yöneticisinin kendi varsayılanı
	 * geçerli olur.
	 */
	TimeoutSec int

	// NPMRegistry, npm kayıt defterinin tam adresi.
	NPMRegistry string
	Username    string
	// Token, parola veya erişim anahtarı. Dosyaya yazılır, ORTAM DEĞİŞKENİNE
	// KONMAZ — agent `env` yazdırdığında görünmemeli.
	Token string
}

// HasAuth, kimlik doğrulama bilgisi verilmiş mi.
func (p PackageRegistry) HasAuth() bool { return p.Username != "" && p.Token != "" }

// MavenEnabled, Maven deposu tanımlı mı.
//
// npm'den AYRI: bir kurum npm'i kurumsal depoya alıp Maven'ı almamış olabilir
// (ya da tersi). İkisini tek bayrağa bağlamak, yalnızca birini tanımlayan
// kurulumda diğerinin yapılandırmasını da yazmak demekti.
func (p PackageRegistry) MavenEnabled() bool { return p.MavenRegistry != "" }

// Enabled, kurumsal depo tanımlı mı.
func (p PackageRegistry) Enabled() bool { return p.NPMRegistry != "" }

// RepoSpec, üzerinde çalışılacak depo.
type RepoSpec struct {
	URL    string
	Branch string

	// Username boşsa kimlik doğrulamasız klonlama yapılır (açık depolar).
	Username string
	Secret   string

	// CloneDepth, kaç commit'lik geçmiş klonlanacağı.
	CloneDepth int
}

// HasCredentials, depo erişimi için kimlik bilgisi verilip verilmediği.
func (r RepoSpec) HasCredentials() bool { return r.Secret != "" }

// AgentSpec, çalıştırılacak agent'ın tanımı.
//
// Prompt burada TAM METİN olarak taşınır, referans olarak değil: çalıştırma
// kaydı hangi talimatla çalıştığını kalıcı olarak bilmeli.
type AgentSpec struct {
	Slug        string
	Description string
	Prompt      string

	AllowEdit     bool
	AllowBash     bool
	AllowWebfetch bool

	// MCPServers, bu agent'ın erişebileceği dış araç sunucuları.
	//
	// Yetkiler gibi AGENT'A bağlıdır (spec 011 K1): "bu agent neler yapabilir"
	// sorusunun parçası. Boş liste, hiçbir dış araca erişim yok demektir.
	MCPServers []MCPServerSpec

	// Scripts, bu agent'ın çalıştırabileceği hazır kabuk betikleri.
	//
	// Yalnızca AllowBash açıkken işleme alınır (spec 012 K3): kapalıyken dosya
	// container'a hiç girmez.
	Scripts []ScriptSpec

	// ScriptFolders, betiklerin ait olduğu kampanya klasörleri.
	//
	// Betik listesinden TÜRETİLEMEZ: klasörün kendi açıklaması yalnızca burada
	// ve model kampanyanın ne olduğunu ondan öğreniyor.
	ScriptFolders []FolderSpec
}

// ScriptSpec, container'a konacak tek bir betik.
//
// `internal/scripts` tipini doğrudan taşımıyoruz: `runner` paketi bilinçli
// olarak depolama katmanını tanımıyor, yalnızca çalıştırmak için gerekeni alıyor
// (MCPServerSpec'teki kararın aynısı).
type ScriptSpec struct {
	// Name, `.sh` uzantısı olmadan dosya adı.
	Name string
	// Description, agent'ın talimatına yazılır — betiğin NE ZAMAN çağrılacağını
	// modele anlatan tek şey budur.
	Description string
	Content     string
	// Folder, betiğin ait olduğu kampanya klasörü. Boşsa betik kökte durur —
	// klasör öncesi davranışın aynısı.
	Folder string
}

/*
FolderSpec, bir kampanya klasörünün talimatta anlatılması için gereken bilgi.

AÇIKLAMA AYRI TAŞINIYOR çünkü script'lerin kendi açıklamaları kampanyanın NE
OLDUĞUNU anlatmıyor: "engines alanını günceller" tek bir adımı tarif eder,
"Node 18'den 24'e standart yükseltme" ise işin tamamını.
*/
type FolderSpec struct {
	Name        string
	Description string
}

/*
ProjectDir, klonlanan projenin container içindeki kökü.

TEK TANIM. Değer `entrypoint.sh`'a `PROJECT_DIR` olarak geçiyor ve betik onu
okuyor; iki yerde sabit yazılsaydı biri değiştiğinde diğeri geride kalır ve
kullanıcının yazdığı script'ler var olmayan bir dizine bakardı.

Betikler bu değişkeni kullanır ve ÇALIŞMA DİZİNİNE GÜVENMEZ: bash aracını
hangi dizinde çalıştırdığı motorun kararı, bizim değil.
*/
const ProjectDir = "/work"

// MCPServerSpec, tek bir MCP sunucusuna bağlanmak için gerekenler.
//
// `internal/mcp` tipini doğrudan taşımıyoruz: `runner` paketi bilinçli olarak
// depolama katmanını tanımıyor, yalnızca çalıştırmak için gerekeni alıyor.
type MCPServerSpec struct {
	// Name, araç adlarının önekidir: `sentry` → `sentry_issue`.
	Name string
	// Transport, "http" veya "sse".
	Transport string
	URL       string
	// Secret boş olabilir: anahtarsız çalışan sunucular var.
	Secret string
}

// ProviderSpec, modele erişim için gereken sağlayıcı bilgisi.
type ProviderSpec struct {
	// Slug, çalıştırma motorunun yapılandırmasındaki sağlayıcı kimliği.
	Slug string
	// Kind, sağlayıcı türü: openrouter | litellm | openai_compatible
	Kind    string
	BaseURL string
	APIKey  string
}

// Limits, container kaynak sınırları.
type Limits struct {
	CPUCores int
	MemoryGB int
}

// FileChange, değişen bir dosyanın özeti.
type FileChange struct {
	File      string `json:"file"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"` // modified | added | deleted
}

// Result, tamamlanmış bir çalıştırmanın çıktısı.
type Result struct {
	Output string
	Diff   string
	// Files, değişen dosyaların özeti. Arayüz "3 dosya, +42 −8" için kullanır.
	Files []FileChange

	PromptTokens     int
	CompletionTokens int
	CostUSD          float64
}

// HasChanges, agent'ın kodda değişiklik üretip üretmediği.
func (r Result) HasChanges() bool { return r.Diff != "" }

var (
	// ErrTimeout, çalıştırma süre sınırını aştı.
	ErrTimeout = errors.New("çalışma süre sınırını aştı")

	// ErrCancelled, çalıştırma iptal edildi.
	ErrCancelled = errors.New("çalışma iptal edildi")

	// ErrRepoAccess, depo klonlanamadı (yanlış adres, branch yok, yetki yok).
	ErrRepoAccess = errors.New("depoya erişilemedi")

	// ErrProviderAuth, sağlayıcı anahtarı reddedildi.
	ErrProviderAuth = errors.New("sağlayıcı anahtarı geçersiz")

	// ErrModel, model çağrısı başarısız oldu.
	ErrModel = errors.New("model çağrısı başarısız")

	// ErrSandbox, sandbox kurulamadı veya beklenmedik şekilde öldü.
	ErrSandbox = errors.New("çalışma ortamı hazırlanamadı")

	/*
	 * ErrProviderDriver, yerleşik olmayan sağlayıcının sürücüsü yüklenemedi.
	 *
	 * Sürücüler artık imaja gömülü (spec 003, 2026-08-12) ve bu yol nadiren
	 * tetikleniyor; ama kalmak zorunda. Öncesinde sürücü indirilemediğinde
	 * motor yalnızca WARN düşürüyor, çalıştırma ise kullanıcıya "durum 500:
	 * UnknownError" olarak yansıyordu — yani ekranda sebebi olmayan bir
	 * hata vardı ve asıl neden yalnızca backend logunda duruyordu.
	 */
	ErrProviderDriver = errors.New("sağlayıcı sürücüsü yüklenemedi")
)
