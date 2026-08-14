// Command server, agent-coder API sunucusunu başlatır.
//
// Bu dosya yalnızca wiring yapar: yapılandırmayı oku, bağımlılıkları kur,
// sunucuyu başlat, kapatma sinyalini bekle. İş mantığı internal/ altındadır.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/agentreg"
	"github.com/agent-coder/backend/internal/cacert"
	"github.com/agent-coder/backend/internal/catalog"
	"github.com/agent-coder/backend/internal/config"
	"github.com/agent-coder/backend/internal/credentials"
	"github.com/agent-coder/backend/internal/db"
	"github.com/agent-coder/backend/internal/events"
	"github.com/agent-coder/backend/internal/gitprovider"
	"github.com/agent-coder/backend/internal/httpapi"
	"github.com/agent-coder/backend/internal/integrations/github"
	"github.com/agent-coder/backend/internal/integrations/jira"
	"github.com/agent-coder/backend/internal/jiratrigger"
	"github.com/agent-coder/backend/internal/llm"
	"github.com/agent-coder/backend/internal/mcp"
	"github.com/agent-coder/backend/internal/mcpserver"
	"github.com/agent-coder/backend/internal/projects"
	"github.com/agent-coder/backend/internal/reports"
	"github.com/agent-coder/backend/internal/runbuild"
	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runner/opencode"
	"github.com/agent-coder/backend/internal/runner/sandbox"
	"github.com/agent-coder/backend/internal/runs"
	"github.com/agent-coder/backend/internal/scripts"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/settings"
	"github.com/agent-coder/backend/internal/tlstrust"
	"github.com/agent-coder/backend/internal/workflow"
)

func main() {
	if err := run(); err != nil {
		slog.Error("sunucu başlatılamadı", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	setupLogging(cfg)

	slog.Info("agent-coder backend başlıyor",
		"version", httpapi.Version,
		"env", cfg.Env,
		"addr", cfg.Addr(),
	)

	// SIGINT/SIGTERM geldiğinde ctx iptal olur.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ── Bağımlılıklar ───────────────────────────────────────────────────────

	// Şifreleme anahtarı config tarafından doğrulandı; burada hata beklenmez.
	cipher, err := secrets.NewCipher(cfg.SecretEncryptionKey)
	if err != nil {
		return err
	}

	database, err := db.Connect(ctx, db.Options{
		URL:            cfg.DB.URL,
		MaxConns:       cfg.DB.MaxConns,
		MaxConnIdle:    cfg.DB.MaxConnIdle,
		ConnectTimeout: 10 * time.Second,
		ConnectRetries: 10,
	})
	if err != nil {
		return err
	}
	defer database.Close()

	// Migration'lar açılışta uygulanır — ayrı bir servis veya elle adım yok.
	if err := database.MigrateUp(ctx); err != nil {
		return err
	}

	// Çalışma ayarları önce yüklenir: sonraki bileşenler parametrelerini
	// buradan okur ve değer çalışma anında okunduğu için ayar değişikliği
	// yeniden başlatma gerektirmez.
	settingsSvc := settings.NewService(database.Pool)
	if err := settingsSvc.Load(ctx); err != nil {
		return err
	}

	/*
	 * Kurumsal kök sertifika — tek çözümleyici, üç tüketici.
	 *
	 * BURADA kuruluyor çünkü aşağıdaki her giden çağrı istemcisi ondan
	 * besleniyor. Ayar kazanır, ortam değişkeni yedekte kalır; ikisini de tek
	 * yerde çözmek, üç tüketicinin farklı sertifikalara bakmasını önlüyor.
	 */
	caResolver := cacert.NewResolver(
		func() string { return settingsSvc.Text(settings.KeyCorporateCA) },
		cfg.Runner.ExtraCACert,
	)
	// Giden HTTPS çağrılarının taşıyıcısı. Sertifika değişince kendini
	// yeniler — yeniden başlatma gerekmez (spec 017 H5).
	trustRT := tlstrust.New(caResolver.PEM).RoundTripper()

	llmStore := llm.NewStore(database.Pool, cipher)
	gitStore := gitprovider.NewStore(database.Pool, cipher)
	credStore := credentials.NewStore(database.Pool, cipher)
	mcpStore := mcp.NewStore(database.Pool, cipher)
	scriptStore := scripts.NewStore(database.Pool)
	mcpClient := mcp.NewClient(func() time.Duration {
		return time.Duration(settingsSvc.Int(settings.KeyMCPTimeoutSeconds)) * time.Second
	}, trustRT)

	// Hiç sağlayıcı yoksa .env'deki OpenRouter anahtarından biri oluşturulur;
	// mevcut kurulumlar hiçbir şey yapmadan çalışmaya devam eder.
	if err := llm.Bootstrap(ctx, llmStore, cfg.OpenRouterAPIKey); err != nil {
		slog.WarnContext(ctx, "ortam değişkeninden sağlayıcı oluşturulamadı", "error", err)
	}

	catalogStore := catalog.NewStore(database.Pool)
	syncer := catalog.NewSyncer(database.Pool, llmStore, trustRT)

	projectStore := projects.NewStore(database.Pool)
	agentStore := agentreg.NewStore(database.Pool, func() int {
		return settingsSvc.Int(settings.KeyMaxPromptKB)
	})

	// Hazır agent'lar veritabanına tohumlanır. Kullanıcının düzenlemelerini
	// ezmez; yalnızca "sıfırla" için özgün hal referansını tazeler.
	if _, err := agentStore.Seed(ctx); err != nil {
		return err
	}

	// ── Çalıştırma altyapısı ────────────────────────────────────────────────

	sandboxMgr, err := sandbox.NewManager(opencode.Port)
	if err != nil {
		return err
	}
	defer sandboxMgr.Close()

	agentRunner := opencode.New(sandboxMgr, cfg.Runner.Image, cfg.Runner.Network, cfg.Runner.HTTPProxy)
	if cfg.Runner.HTTPProxy != "" {
		// Sessiz kalmamalı: vekil, çalıştırmanın TÜM dış trafiğini üçüncü bir
		// sürece yönlendiriyor ve o süreç TLS'i açabiliyorsa sağlayıcı
		// anahtarını da görür. Denetim bitip ayar kaldırılmadığında bunu
		// loglardan görmek gerekir.
		slog.WarnContext(ctx, "runner trafiği denetim vekiline yönlendiriliyor",
			"proxy", cfg.Runner.HTTPProxy)
	}

	// Docker ve runner imajı hazır mı? Eksikse çalıştırma denenene kadar
	// fark edilmez; açılışta uyarmak kullanıcıyı erken bilgilendirir.
	if err := agentRunner.Ping(ctx); err != nil {
		slog.WarnContext(ctx, "çalıştırma altyapısı hazır değil — agent çalıştırılamaz",
			"error", err)
	} else if n, err := agentRunner.CleanupOrphans(ctx); err != nil {
		slog.WarnContext(ctx, "sahipsiz container taraması başarısız", "error", err)
	} else if n > 0 {
		slog.InfoContext(ctx, "önceki çalışmalardan kalan container'lar silindi", "adet", n)
	}

	bus := events.New()
	runStore := runs.NewStore(database.Pool)

	// Sınırlar fonksiyon olarak geçilir: ayar değişikliği sunucu yeniden
	// başlatılmadan geçerli olsun diye her kullanımda yeniden okunurlar.
	runManager := runs.NewManager(runStore, agentRunner, bus, runs.Limits{
		MaxConcurrent: func() int { return settingsSvc.Int(settings.KeyMaxConcurrentRuns) },
		Timeout: func() time.Duration {
			return time.Duration(settingsSvc.Int(settings.KeyRunTimeoutMinutes)) * time.Minute
		},
		CPUCores:   func() int { return settingsSvc.Int(settings.KeyRunnerCPULimit) },
		MemoryGB:   func() int { return settingsSvc.Int(settings.KeyRunnerMemoryLimitG) },
		CloneDepth: func() int { return settingsSvc.Int(settings.KeyCloneDepth) },

		// Motor logları: saklama, boyut sınırı ve yaşam süresi.
		EngineLogPersist: func() bool { return settingsSvc.Bool(settings.KeyEngineLogPersist) },
		EngineLogMaxKB:   func() int { return settingsSvc.Int(settings.KeyEngineLogMaxKB) },
		EngineLogRetention: func() time.Duration {
			return time.Duration(settingsSvc.Int(settings.KeyEngineLogRetention)) * 24 * time.Hour
		},

		// Paket deposu: adres ve kullanıcı adı ayarlardan, token şifreli
		// kimlik bilgisi deposundan. Token okunamazsa çalıştırma DURMAZ —
		// anonim okumaya açık depolarda kayıt zaten yoktur ve olmayan bir
		// kimlik bilgisi hata değildir.
		Packages: func() runner.PackageRegistry {
			pkg := runner.PackageRegistry{
				NPMRegistry:   settingsSvc.Text(settings.KeyNPMRegistry),
				MavenRegistry: settingsSvc.Text(settings.KeyMavenRegistry),
				Username:      settingsSvc.Text(settings.KeyNPMUsername),
				// Süre sınırı npm ve Maven için ORTAK: "paket deposu ne kadar
				// bekler" sorusunun tek cevabı olmalı.
				TimeoutSec: settingsSvc.Int(settings.KeyPackagesTimeout),
			}
			// Kimlik yalnızca bir depo tanımlıysa ve kullanıcı adı verilmişse
			// aranır; anonim okumaya açık depolarda kayıt zaten yoktur.
			if (pkg.NPMRegistry == "" && pkg.MavenRegistry == "") || pkg.Username == "" {
				return pkg
			}
			token, _, err := credStore.Reveal(ctx, credentials.KindNexus)
			if err != nil {
				if !errors.Is(err, credentials.ErrNotConfigured) {
					slog.WarnContext(ctx, "paket deposu kimlik bilgisi okunamadı", "error", err)
				}
				return pkg
			}
			pkg.Token = token
			return pkg
		},

		// Kurumsal kök sertifika: ayardan ya da ortam değişkeniyle verilen
		// dosyadan. Her koşuda okunur — arayüzden değiştirilince yeniden
		// başlatma beklenmez.
		CACert: caResolver.PEM,
	})
	defer runManager.Shutdown()

	// Sunucu bir çalıştırma sürerken ölmüşse kayıtlar 'running' kalır ve
	// arayüzde sonsuza kadar dönen iş görünür.
	if n, err := runManager.RecoverInterrupted(ctx); err != nil {
		slog.WarnContext(ctx, "yarım çalıştırmalar kapatılamadı", "error", err)
	} else if n > 0 {
		slog.WarnContext(ctx, "yarım kalmış çalıştırmalar kapatıldı", "adet", n)
	}

	// ── Akış motoru ─────────────────────────────────────────────────────────

	runBuilder := runbuild.New(projectStore, agentStore, llmStore, gitStore, mcpStore, scriptStore)
	runPusher := runs.NewPusher(runStore)
	workflowStore := workflow.NewStore(database.Pool)

	// PR düğümü kaynak branch'i bulmak için çalışmanın grafına bakar.
	graphOf := func(ctx context.Context, versionID string) (workflow.Graph, error) {
		id, err := uuid.Parse(versionID)
		if err != nil {
			return workflow.Graph{}, err
		}
		v, err := workflowStore.Version(ctx, id)
		if err != nil {
			return workflow.Graph{}, err
		}
		return v.Graph, nil
	}
	// Düğüm türü başına çalıştırıcı burada kuruluyor: her tür kendi
	// bağımlılığını bu noktada alıyor, motor yalnızca haritayı görüyor.
	workflowExec := workflow.NewExecutor(
		workflowStore,
		workflow.Handlers{
			workflow.KindAgent: workflow.NewAgentHandler(
				runbuild.NewStepRunner(runBuilder, runManager, runPusher),
			),
			workflow.KindGitHubPR: runbuild.NewPRHandler(
				runBuilder, projectStore, github.New(cfg.GitHubAPIURL, trustRT), graphOf,
			),
			workflow.KindJiraComment: runbuild.NewJiraHandler(credStore, jira.New(trustRT), workflowStore),
			workflow.KindMCPCall:     runbuild.NewMCPHandler(mcpStore, mcpClient),
		},
		busPublisher{bus},
	)
	defer workflowExec.Shutdown()

	if n, err := workflowStore.RecoverInterrupted(ctx); err != nil {
		slog.WarnContext(ctx, "yarım akışlar kapatılamadı", "error", err)
	} else if n > 0 {
		slog.WarnContext(ctx, "yarım kalmış akışlar kapatıldı", "adet", n)
	}

	workflowLauncher := workflow.NewLauncher(workflowStore, workflowExec)

	// Agent Coder'ın kendisi bir MCP sunucusu: dışarıdaki istemciler akışları
	// listeleyip başlatabilir. Başlatma mevcut Launcher'dan geçer.
	mcpAccess := mcpserver.NewAccess(database.Pool)
	mcpOut := mcpserver.New(workflowStore, workflowLauncher, httpapi.Version)

	// Jira tetikleyici: tarama ve webhook aynı işleme yolundan geçer.
	jiraTrigger := jiratrigger.New(
		workflowStore, workflowLauncher, credStore, jira.New(trustRT),
		func() time.Duration {
			return time.Duration(settingsSvc.Int(settings.KeyJiraPollMinutes)) * time.Minute
		},
		func() int { return settingsSvc.Int(settings.KeyJiraScanLimit) },
	)

	handler := httpapi.NewHandler(httpapi.Deps{
		Config:        cfg,
		DB:            database,
		LLMProviders:  llmStore,
		Catalog:       catalogStore,
		Syncer:        syncer,
		GitProviders:  gitStore,
		MCPServers:    mcpStore,
		Scripts:       scriptStore,
		MCPClient:     mcpClient,
		MCPAccess:     mcpAccess,
		MCPServer:     mcpOut,
		HTTPTransport: trustRT,
		CACert:        caResolver,
		GitValidator:  gitprovider.NewValidator(trustRT),
		Credentials:   credStore,
		JiraValidator: credentials.NewValidator(trustRT),
		Settings:      settingsSvc,
		Projects:      projectStore,
		RepoVerifier:  projects.NewVerifier(),
		Agents:        agentStore,
		Runs:          runStore,
		RunManager:    runManager,
		RunBuilder:    runBuilder,
		Pusher:        runPusher,
		Bus:           bus,
		Reports:       reports.NewStore(database.Pool, settingsSvc),
		Workflows:     workflowStore,
		WorkflowExec:  workflowExec,
		Launcher:      workflowLauncher,
		JiraTrigger:   jiraTrigger,
	})
	defer handler.Shutdown()

	// ── Arka plan işleri ────────────────────────────────────────────────────

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Katalog senkronu açılışı engellemez: anahtar henüz girilmemiş veya
		// internet yoksa sistem yine de ayağa kalkar.
		syncer.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Eski motor loglarının temizliği. Ayrı bir zamanlayıcı altyapısı
		// yok; katalog senkronuyla aynı kalıp.
		runManager.PurgeEngineLogs(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		// Jira taraması da açılışı engellemez: erişim tanımlı değilse veya
		// hiç Jira tetikleyicisi yoksa sessizce boşta durur.
		jiraTrigger.Run(ctx)
	}()

	// ── HTTP sunucusu ───────────────────────────────────────────────────────

	srv := &http.Server{
		Addr:         cfg.Addr(),
		Handler:      handler.Routes(),
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		slog.Info("kapatma sinyali alındı, açık istekler bekleniyor",
			"timeout", cfg.HTTP.ShutdownTimeout)
	}

	// Kapatma, iptal edilmiş ctx ile çalışmaz — kendi timeout'unu kullanır.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}

	stop() // sinyal dinlemeyi bırak, arka plan ctx'i iptal olsun
	wg.Wait()

	slog.Info("sunucu düzgün kapandı")
	return nil
}

// setupLogging varsayılan slog logger'ını kurar.
// Üretimde JSON (log toplayıcılar için), geliştirmede okunabilir metin.
func setupLogging(cfg *config.Config) {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}

	var handler slog.Handler
	if cfg.IsProduction() {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// busPublisher, akış motorunun olaylarını canlı yayına bağlar.
//
// Motor `events` paketini tanımıyor: kendi dar arayüzünü (Publisher) görüyor.
// Böylece motor testleri yayın altyapısı olmadan çalışabiliyor.
type busPublisher struct{ bus *events.Bus }

func (p busPublisher) Publish(topic uuid.UUID, level, message string) {
	p.bus.Publish(topic, events.Event{
		Level:   level,
		Message: message,
		TS:      time.Now().UTC().Format(time.RFC3339),
	})
}
