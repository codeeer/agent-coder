package opencode

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runner/sandbox"
)

// readyTimeout, container'ın ayağa kalkması için tanınan süre.
// Depo klonlaması da bu sürenin içinde gerçekleşir.
const readyTimeout = 3 * time.Minute

// Runner, runner.Runner arayüzünün opencode uygulaması.
type Runner struct {
	sandbox *sandbox.Manager
	image   string
	network string
}

// New yeni runner üretir.
//
// Kurumsal sertifika BURADA DEĞİL, her isteğin içinde taşınır
// (runner.Request.CACert): ayar çalışma anında değişebiliyor ve
// yapılandırmanın kurucuya bağlanması yeniden başlatma gerektirirdi.
func New(mgr *sandbox.Manager, image, network string) *Runner {
	return &Runner{
		sandbox: mgr,
		image:   image,
		network: network,
	}
}

// Ping, altyapının hazır olduğunu doğrular (Docker + runner imajı).
func (r *Runner) Ping(ctx context.Context) error {
	if err := r.sandbox.Ping(ctx); err != nil {
		return err
	}
	return r.sandbox.EnsureImage(ctx, r.image)
}

// CleanupOrphans, önceki çalışmalardan kalan container'ları siler.
func (r *Runner) CleanupOrphans(ctx context.Context) (int, error) {
	return r.sandbox.CleanupOrphans(ctx)
}

// Run, bir agent çalıştırmasını yürütür.
//
// Container HER YOLDA silinir: başarı, hata, panik, iptal, zaman aşımı.
func (r *Runner) Run(ctx context.Context, req runner.Request, emit runner.EventFunc) (*runner.Result, error) {
	if emit == nil {
		emit = func(runner.Event) {}
	}

	// Süre sınırı burada uygulanır; aşılırsa ErrTimeout'a çevrilir.
	runCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	configFiles, err := runner.BuildConfigFiles(req.Provider, req.Agent, req.Model, req.Packages)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", runner.ErrSandbox, err)
	}
	// Kurumsal sertifika, diğer yapılandırma dosyalarıyla aynı yoldan gider:
	// container'a KOPYALANIR, host'tan bağlanmaz.
	if caFile, ok := runner.CACertFile(req.CACert); ok {
		configFiles = append(configFiles, caFile)
	}

	/*
	 * İmaj, seçilen Node sürümüne göre belirlenir; sürüm boşsa taban imaj.
	 *
	 * VARLIĞI BURADA, klonlama başlamadan sınanır. Eskiden `EnsureImage`
	 * yalnızca açılıştaki `Ping`te çağrılıyordu ve hatası log'a düşüp
	 * yutuluyordu; eksik imaj ancak `ContainerCreate` hatasıyla, kullanıcının
	 * beklemesinden sonra görünürdü. Sürümlü imajlarda bu daha da kötü olurdu:
	 * taban imaj yerinde ama seçilen sürüm çekilmemiş olabilir.
	 */
	image := runner.ImageFor(r.image, req.NodeVersion)

	hazirlik := "çalışma ortamı hazırlanıyor"
	if req.NodeVersion != "" {
		hazirlik += " (node " + req.NodeVersion + ")"
	}
	emit(runner.Event{Level: runner.LevelInfo, Message: hazirlik})

	if err := r.sandbox.EnsureImage(runCtx, image); err != nil {
		return nil, classify(err, runCtx, ctx)
	}

	ct, err := r.sandbox.Create(runCtx, sandbox.Spec{
		RunID:    req.RunID.String(),
		Image:    image,
		Network:  r.network,
		Env:      buildEnv(req),
		CPUCores: req.Limits.CPUCores,
		MemoryGB: req.Limits.MemoryGB,
		Files:    toSandboxFiles(configFiles),
	})
	if err != nil {
		return nil, classify(err, runCtx, ctx)
	}

	// Temizlik iptal edilmiş context ile çalışmaz; kendi context'ini kullanır.
	defer ct.Remove(context.WithoutCancel(ctx))

	/*
	 * Motor logları container SİLİNMEDEN ÖNCE toplanır.
	 *
	 * `defer` sırası kritik: LIFO çalıştığı için bu satır yukarıdaki
	 * `ct.Remove`tan SONRA yazılmak zorunda — böylece önce toplanır, sonra
	 * silinir. Ters sırada container yok olmuş olurdu.
	 *
	 * Koşu nasıl biterse bitsin çalışır; asıl ihtiyaç başarısız ve zaman
	 * aşımına uğramış koşularda.
	 */
	cli := newClient(ct.Host)

	// Oturum kimliği aşağıda doluyor; toplama `defer` olduğu için çalıştığı
	// anda güncel değeri okur. Oturum hiç açılamadan düşen bir koşuda boş
	// kalır ve geçmiş kaynağı atlanır.
	var oturumKimligi string

	if req.EngineLogs != nil {
		defer func() {
			req.EngineLogs(collectEngineLogs(
				context.WithoutCancel(ctx), ct, cli, oturumKimligi, req))
		}()
	}

	if err := cli.waitReady(runCtx, readyTimeout); err != nil {
		// Hazır olmama sebebi genelde klonlamanın başarısız olmasıdır;
		// container logları kullanıcıya asıl nedeni söyler.
		return nil, r.readyFailure(ctx, runCtx, ct, emit, err)
	}
	emit(runner.Event{Level: runner.LevelInfo, Message: "depo hazır, agent başlatılıyor"})

	// MCP sunucuları bağlanabildi mi?
	//
	// Motor bağlanamayan bir sunucuyu SESSİZCE yok sayıyor: araçlarını modele
	// hiç sunmuyor, hata da vermiyor (ölçüldü — spec 011). Kontrol edilmezse
	// arıza "agent neden aptallaştı" sorusuyla, günler sonra fark edilir.
	//
	// Çalıştırma başarısız SAYILMAZ: araç olmadan da iş bitebilir. Ama sessiz
	// kalmaz.
	r.warnFailedMCP(runCtx, cli, req, emit)

	// Olay akışı arka planda dinlenir; kopması çalıştırmayı düşürmez.
	streamCtx, stopStream := context.WithCancel(runCtx)
	defer stopStream()
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		cli.streamEvents(streamCtx, emit)
	}()
	// Kaçak goroutine kalmasın: akış kapanana kadar beklenir.
	defer func() {
		stopStream()
		<-streamDone
	}()

	sessionID, err := cli.createSession(runCtx,
		"run-"+req.RunID.String(), runner.BuildPermissions(req.Agent))
	if err != nil {
		return nil, classify(err, runCtx, ctx)
	}
	oturumKimligi = sessionID

	msg, err := cli.sendMessage(runCtx, sessionID,
		req.Agent.Slug, req.Provider.Slug, req.Model, req.Task)
	if err != nil {
		// İptal veya zaman aşımında oturumu da durdurmaya çalış; container
		// zaten silinecek ama bu, model çağrısının erken kesilmesini sağlar.
		cli.abort(context.WithoutCancel(ctx), sessionID)
		return nil, classify(err, runCtx, ctx)
	}

	if msg.Info.Error != nil {
		// Sürücü yüklenememişse asıl sebep motorun logunda; model hatası
		// diye göstermek kullanıcıyı yanlış yere bakmaya gönderir.
		if cause, ok := driverFailure(ctx, ct); ok {
			return nil, fmt.Errorf("%w: %s", runner.ErrProviderDriver, cause)
		}
		return nil, fmt.Errorf("%w: %v", runner.ErrModel, msg.Info.Error)
	}

	diff, err := cli.diff(runCtx)
	if err != nil {
		// Diff alınamaması sonucu düşürmez; çıktı ve maliyet geçerlidir.
		emit(runner.Event{Level: runner.LevelWarn, Message: "değişiklikler okunamadı"})
	}
	changed, _ := cli.changedFiles(runCtx)

	tokens := msg.Info.Tokens
	result := &runner.Result{
		Output:           msg.Text(),
		Diff:             strings.TrimSpace(diff),
		Files:            toRunnerFiles(changed),
		PromptTokens:     tokens.Input + tokens.Cache.Read + tokens.Cache.Write,
		CompletionTokens: tokens.Output + tokens.Reasoning,
		CostUSD:          msg.Info.Cost,
	}

	if result.HasChanges() {
		emit(runner.Event{Level: runner.LevelInfo, Message: fmt.Sprintf(
			"çalışma tamamlandı, %d dosya değişti", len(result.Files))})
	} else {
		emit(runner.Event{Level: runner.LevelInfo, Message: "çalışma tamamlandı, değişiklik yok"})
	}
	return result, nil
}

/*
 * driverFailure, sağlayıcı sürücüsünün yüklenip yüklenemediğini loglardan okur.
 *
 * NEDEN LOGDAN: motor, sürücü paketini indiremediğinde çalıştırmayı
 * durdurmuyor — yalnızca WARN düşürüp devam ediyor ve sağlayıcı ayağa
 * kalkmadığı için mesaj "durum 500: UnknownError" ile geri geliyor. O 500
 * kullanıcıya hiçbir şey anlatmıyor; asıl cümle logda duruyor.
 *
 * Sürücüler imaja gömüldükten sonra (spec 003, 2026-08-12) bu yol nadiren
 * tetikleniyor. Yine de duruyor: bir sonraki motor sürümü yeni bir paket
 * isteyebilir ve o gün ekranda yine sebepsiz bir 500 görmek istemiyoruz.
 */
func driverFailure(ctx context.Context, ct *sandbox.Container) (string, bool) {
	logs, err := ct.Logs(context.WithoutCancel(ctx), "200")
	if err != nil || logs == "" {
		return "", false
	}
	if !strings.Contains(logs, "NpmInstallFailedError") &&
		!strings.Contains(logs, "background dependency install failed") {
		return "", false
	}
	return firstMeaningfulLine(logs), true
}

// readyFailure, container hazır olmadığında asıl nedeni loglardan çıkarır.
func (r *Runner) readyFailure(parent, runCtx context.Context, ct *sandbox.Container,
	emit runner.EventFunc, cause error) error {

	logs, logErr := ct.Logs(context.WithoutCancel(parent), "50")
	if logErr == nil && logs != "" {
		emit(runner.Event{Level: runner.LevelError, Message: firstMeaningfulLine(logs)})

		// Klonlama hatası en sık görülen sebep; kullanıcıya doğru olanı söyle.
		if strings.Contains(logs, "klonlama başarısız") ||
			strings.Contains(logs, "Could not find remote branch") ||
			strings.Contains(logs, "Authentication failed") ||
			strings.Contains(logs, "not found") {
			return fmt.Errorf("%w: %s", runner.ErrRepoAccess, firstMeaningfulLine(logs))
		}
	}

	if err := classifyCtx(runCtx, parent); err != nil {
		return err
	}
	return fmt.Errorf("%w: %w", runner.ErrSandbox, cause)
}

// firstMeaningfulLine, log yığınından kullanıcıya gösterilecek satırı seçer.
func firstMeaningfulLine(logs string) string {
	lines := strings.Split(logs, "\n")
	// Sondan başa: asıl hata genelde en sonda olur.
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(stripControl(lines[i]))
		if l == "" {
			continue
		}
		if strings.Contains(l, "HATA") || strings.Contains(l, "fatal:") ||
			strings.Contains(l, "error") || strings.Contains(l, "[runner]") {
			return truncate(l, 200)
		}
	}
	return "çalışma ortamı hazır olmadı"
}

// stripControl, Docker log çerçevelerinin bıraktığı kontrol baytlarını atar.
func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' {
			return -1
		}
		return r
	}, s)
}

// warnFailedMCP, bağlanamayan MCP sunucularını kullanıcıya bildirir.
func (r *Runner) warnFailedMCP(ctx context.Context, cli *client, req runner.Request, emit runner.EventFunc) {
	if len(req.Agent.MCPServers) == 0 {
		return
	}

	status, err := cli.mcpStatus(ctx)
	if err != nil {
		// Durum ucu okunamadıysa çalıştırmayı düşürmüyoruz; yalnızca
		// doğrulayamadığımızı söylüyoruz.
		emit(runner.Event{
			Level:   runner.LevelWarn,
			Message: "MCP sunucularının durumu doğrulanamadı: " + err.Error(),
		})
		return
	}

	for _, m := range req.Agent.MCPServers {
		st, ok := status[m.Name]
		switch {
		case !ok:
			emit(runner.Event{Level: runner.LevelWarn,
				Message: fmt.Sprintf("MCP sunucusu %q motorda görünmüyor — araçları kullanılamayacak", m.Name)})
		case st.Status == "connected":
			emit(runner.Event{Level: runner.LevelInfo,
				Message: fmt.Sprintf("MCP sunucusu %q bağlandı", m.Name)})
		default:
			msg := fmt.Sprintf("MCP sunucusu %q bağlanamadı (%s) — araçları kullanılamayacak",
				m.Name, st.Status)
			if st.Error != "" {
				msg += ": " + st.Error
			}
			emit(runner.Event{Level: runner.LevelWarn, Message: msg})
		}
	}
}

// buildEnv, container'a geçilecek ortam değişkenleri.
//
// Sağlayıcı anahtarı buradan geçer; yapılandırma dosyası ona referans verir.
// Depo kimlik bilgisi de burada — entrypoint bunu credential store'a yazar ve
// remote URL'e gömmez.
func buildEnv(req runner.Request) map[string]string {
	env := map[string]string{
		runner.APIKeyEnvVar: req.Provider.APIKey,
		"REPO_URL":          req.Repo.URL,
		"OPENCODE_PORT":     fmt.Sprint(Port),
	}

	/*
	 * Kurumsal CA — yalnızca tanımlıysa.
	 *
	 * NODE_EXTRA_CA_CERTS Node'un güven deposuna EKLER, yerine geçmez:
	 * genel sertifikalar geçerli kalır. GIT_SSL_CAINFO ise git'in klonlama
	 * yaptığı HTTPS bağlantısı için — kurumsal ağda ilk kırılan yer orası,
	 * çünkü container daha hazır olmadan klonlama düşüyor.
	 *
	 * TLS doğrulaması KAPATILMIYOR; yalnızca güvenilen kök ekleniyor.
	 */
	if req.CACert != "" {
		env["NODE_EXTRA_CA_CERTS"] = runner.CACertPath
		env["GIT_SSL_CAINFO"] = runner.CACertPath
		/*
		 * CURL_CA_BUNDLE SONRADAN EKLENDİ ve bir ölçümün kaydıdır: kurumsal
		 * Nexus provasında (spec 017) node ve git sertifikayı tanırken curl
		 * `unable to get local issuer certificate` ile düşüyordu. Agent'ın en
		 * sık kullandığı araçlardan biri, sertifika tanıtılmış bir kurulumda
		 * sessizce çalışmıyordu.
		 */
		env["CURL_CA_BUNDLE"] = runner.CACertPath
	}

	/*
	 * Maven süre sınırı — ÖLÇÜLEREK bulunmuş özellik adları.
	 *
	 * Ulaşılamayan bir adrese karşı tutuldu: varsayılanla 98 saniye, 3
	 * saniyelik sınırla 31 saniye. `maven.wagon.http.*` özellikleri HİÇBİR
	 * ETKİ YAPMIYOR — Maven 3.9 wagon'u değil kendi çözümleyici taşıyıcısını
	 * kullanıyor. Adı tahmin edilseydi ayar sessizce etkisiz kalır ve bunu
	 * hiçbir test yakalamazdı.
	 *
	 * MAVEN_OPTS burada KURULUR, entrypoint ise kurumsal sertifika varsa
	 * üstüne EKLER (`${MAVEN_OPTS:+…}`). İkisi ayrı bilgiye sahip: süre ayardan
	 * gelir, sertifikanın varlığı container içinde belli olur.
	 */
	if req.Packages.MavenEnabled() && req.Packages.TimeoutSec > 0 {
		ms := fmt.Sprint(req.Packages.TimeoutSec * 1000)
		env["MAVEN_OPTS"] = "-Daether.connector.connectTimeout=" + ms +
			" -Daether.connector.requestTimeout=" + ms
	}
	if req.Repo.Branch != "" {
		env["REPO_BRANCH"] = req.Repo.Branch
	}
	if req.Repo.CloneDepth > 0 {
		env["GIT_CLONE_DEPTH"] = fmt.Sprint(req.Repo.CloneDepth)
	}
	if req.Repo.HasCredentials() {
		env["GIT_USERNAME"] = req.Repo.Username
		env["GIT_TOKEN"] = req.Repo.Secret
	}

	// MCP sunucularının erişim anahtarları: yapılandırma dosyası bunlara
	// `{env:...}` ile referans verir, değeri içermez (spec 011 K5).
	for _, m := range req.Agent.MCPServers {
		if m.Secret != "" {
			env[runner.MCPEnvVar(m.Name)] = m.Secret
		}
	}
	return env
}

func toRunnerFiles(files []FileChange) []runner.FileChange {
	if len(files) == 0 {
		return nil
	}
	out := make([]runner.FileChange, 0, len(files))
	for _, f := range files {
		out = append(out, runner.FileChange{
			File: f.File, Additions: f.Additions, Deletions: f.Deletions, Status: f.Status,
		})
	}
	return out
}

func toSandboxFiles(files []runner.ConfigFile) []sandbox.File {
	out := make([]sandbox.File, 0, len(files))
	for _, f := range files {
		out = append(out, sandbox.File{Path: f.Path, Content: f.Content, Mode: f.Mode})
	}
	return out
}

// classify, ham hatayı arayüzün sentinel hatalarına çevirir.
func classify(err error, runCtx, parent context.Context) error {
	if ctxErr := classifyCtx(runCtx, parent); ctxErr != nil {
		return ctxErr
	}
	switch {
	case errors.Is(err, sandbox.ErrCreate), errors.Is(err, sandbox.ErrStart):
		return fmt.Errorf("%w: %w", runner.ErrSandbox, err)
	case errors.Is(err, ErrSession), errors.Is(err, ErrMessage):
		return fmt.Errorf("%w: %w", runner.ErrModel, err)
	default:
		return err
	}
}

// classifyCtx, iptal ile zaman aşımını ayırır.
//
// İkisi de context iptali üretir ama kullanıcı için farklı anlamlar taşır:
// biri kendi kararı, diğeri sistemin sınırı.
func classifyCtx(runCtx, parent context.Context) error {
	if parent.Err() != nil {
		return runner.ErrCancelled
	}
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return runner.ErrTimeout
	}
	return nil
}

// engineLogDir, motorun kendi log dosyalarını yazdığı dizin.
//
// Dizin ilk oturumdan SONRA oluşuyor (ölçüldü): hiç oturum açılmadan biten
// bir koşuda burası boştur ve bu bir arıza değildir.
const engineLogDir = "/home/agent/.local/share/opencode/log"

/*
 * transcriptTimeout, oturum geçmişini çekmek için tanınan süre.
 *
 * Kısa tutuluyor: bu adım çalıştırma BİTTİKTEN sonra koşuyor ve container'ın
 * silinmesini geciktiriyor. Cevap vermeyen bir motor için beklemek, teşhis
 * verisi uğruna kaynak sızdırmak olurdu.
 */
const transcriptTimeout = 15 * time.Second

// engineLogReadCap, tar akışından okunacak azami ham bayt.
//
// Saklama sınırı ayrı ve ayarlardan geliyor; bu yalnızca bellekte sınırsız
// büyümeyi engelleyen üst bir bariyer.
const engineLogReadCap = 16 << 20

/*
 * collectEngineLogs, container silinmeden önce ham teşhis verisini toplar.
 *
 * Üç kaynak var ve hiçbiri diğerinin yerini tutmaz: container'ın
 * stdout/stderr'i entrypoint ile klonlamayı anlatır, motorun log dosyaları
 * sağlayıcı çözümleme ve izin kararlarını, oturum geçmişi ise agent'ın
 * konuşmasının ve araç çağrılarının tamamını.
 *
 * Oturum geçmişi ayrıca İLERLEME AKIŞININ YEDEĞİ: ilerleme kayıtları SSE
 * üzerinden besleniyor ve o bağlantı kopabiliyor. Koptuğunda olan biten
 * kaybolmasın diye tam geçmiş burada saklanıyor.
 *
 * HİÇBİR HATA YUKARI TAŞINMAZ: log toplamak çalıştırmanın sonucunu
 * değiştiremez. Toplanamayan bir kaynak sessizce atlanır.
 */
func collectEngineLogs(ctx context.Context, ct *sandbox.Container, cli *client, sessionID string, req runner.Request) []runner.EngineLog {
	sirlar := runner.SecretsOf(req)
	out := []runner.EngineLog{}

	/*
	 * Oturum geçmişi İLK toplanır: motor hâlâ ayakta ve API cevap veriyor.
	 * Dosya kopyalama container'ı durdurmasa da, en kırılgan adımı sona
	 * bırakmak için sebep yok.
	 *
	 * Kendi zaman aşımı var: koşu zaten bitmiş, teşhis verisi uğruna
	 * çalıştırma kapanışı süresiz beklemez.
	 */
	if cli != nil && sessionID != "" {
		gecmisCtx, iptal := context.WithTimeout(ctx, transcriptTimeout)
		gecmis, err := cli.sessionTranscript(gecmisCtx, sessionID)
		iptal()
		if err == nil && gecmis != "" {
			out = append(out, runner.EngineLog{
				Source:  runner.EngineLogSession,
				Content: runner.Redact(gecmis, sirlar),
			})
		}
	}

	if ham, err := ct.Logs(ctx, "all"); err == nil && ham != "" {
		out = append(out, runner.EngineLog{
			Source:  runner.EngineLogStdout,
			Content: runner.Redact(ham, sirlar),
		})
	}

	if icerik := dizinMetni(ctx, ct, engineLogDir); icerik != "" {
		out = append(out, runner.EngineLog{
			Source:  runner.EngineLogFile,
			Content: runner.Redact(icerik, sirlar),
		})
	}
	return out
}

/*
 * dizinMetni, bir container dizinini tek metne çevirir.
 *
 * Dosyalar YOL SIRASINA göre birleşiyor ve her birinin başına yolu yazılıyor:
 * oturum deposunda sıra anlam taşıyor (mesajlar ve parçaları), ve hangi
 * satırın nereden geldiği kaybolursa metin teşhis değerini yitirir.
 *
 * Okunamayan dizin boş metin döner — hata değil: hiç oturum açılmadan biten
 * bir koşuda o dizin hiç oluşmamış olabilir.
 */
func dizinMetni(ctx context.Context, ct *sandbox.Container, dizin string) string {
	dosyalar, err := ct.ReadDir(ctx, dizin, engineLogReadCap)
	if err != nil || len(dosyalar) == 0 {
		return ""
	}

	adlar := make([]string, 0, len(dosyalar))
	for ad := range dosyalar {
		adlar = append(adlar, ad)
	}
	sort.Strings(adlar)

	var b strings.Builder
	for _, ad := range adlar {
		b.WriteString("== " + ad + " ==\n")
		b.Write(dosyalar[ad])
		b.WriteString("\n")
	}
	return b.String()
}
