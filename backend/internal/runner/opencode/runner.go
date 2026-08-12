package opencode

import (
	"context"
	"errors"
	"fmt"
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
	// extraCACert, HOST üzerindeki kurumsal kök sertifikanın yolu; boşsa
	// hiçbir şey bağlanmaz (bkz. sandbox.Spec.ExtraCACert).
	extraCACert string
}

// New yeni runner üretir.
func New(mgr *sandbox.Manager, image, network, extraCACert string) *Runner {
	return &Runner{
		sandbox:     mgr,
		image:       image,
		network:     network,
		extraCACert: extraCACert,
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
		RunID:       req.RunID.String(),
		Image:       image,
		Network:     r.network,
		Env:         buildEnv(req, r.extraCACert),
		CPUCores:    req.Limits.CPUCores,
		MemoryGB:    req.Limits.MemoryGB,
		Files:       toSandboxFiles(configFiles),
		ExtraCACert: r.extraCACert,
	})
	if err != nil {
		return nil, classify(err, runCtx, ctx)
	}

	// Temizlik iptal edilmiş context ile çalışmaz; kendi context'ini kullanır.
	defer ct.Remove(context.WithoutCancel(ctx))

	cli := newClient(ct.Host)

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
func buildEnv(req runner.Request, extraCACert string) map[string]string {
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
	if extraCACert != "" {
		env["NODE_EXTRA_CA_CERTS"] = sandbox.ExtraCACertPath
		env["GIT_SSL_CAINFO"] = sandbox.ExtraCACertPath
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
