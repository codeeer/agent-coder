// Package httpapi, HTTP katmanını kurar: router, middleware ve handler'lar.
//
// Handler'lar ince tutulur — parse, doğrula, servisi çağır, yanıtla.
// İş mantığı ilgili internal/ paketinde durur, burada değil.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/agent-coder/backend/internal/agentreg"
	"github.com/agent-coder/backend/internal/catalog"
	"github.com/agent-coder/backend/internal/config"
	"github.com/agent-coder/backend/internal/credentials"
	"github.com/agent-coder/backend/internal/db"
	"github.com/agent-coder/backend/internal/events"
	"github.com/agent-coder/backend/internal/gitprovider"
	"github.com/agent-coder/backend/internal/jiratrigger"
	"github.com/agent-coder/backend/internal/llm"
	"github.com/agent-coder/backend/internal/mcp"
	"github.com/agent-coder/backend/internal/mcpserver"
	"github.com/agent-coder/backend/internal/projects"
	"github.com/agent-coder/backend/internal/reports"
	"github.com/agent-coder/backend/internal/runbuild"
	"github.com/agent-coder/backend/internal/runs"
	"github.com/agent-coder/backend/internal/scripts"
	"github.com/agent-coder/backend/internal/settings"
	"github.com/agent-coder/backend/internal/workflow"
)

// contextT, handler imzalarında context.Context için kısa ad.
type contextT = context.Context

// Deps, API'nin ihtiyaç duyduğu bağımlılıklar.
//
// Faz 0'daki iki parametreli imza yerine struct kullanılıyor: fazlar ilerledikçe
// bağımlılık sayısı artacak ve konumsal parametreler okunmaz hale gelirdi.
//
// DB olmadan da bir Handler kurulabilir (testler için); bu durumda veritabanına
// dokunan uçlar 503 döner.
type Deps struct {
	Config *config.Config
	DB     *db.DB

	// LLM sağlayıcılar ve model kataloğu
	LLMProviders *llm.Store
	Catalog      *catalog.Store
	Syncer       *catalog.Syncer

	// Kod deposu erişimleri
	GitProviders *gitprovider.Store
	GitValidator *gitprovider.Validator

	// Agent'ların erişebileceği dış araç sunucuları
	MCPServers *mcp.Store
	MCPClient  *mcp.Client

	// Agent'ların çalıştırabileceği hazır kabuk betikleri (spec 012)
	Scripts *scripts.Store

	// Agent Coder'ın kendisini dışarıya MCP olarak açması (spec 011 Aşama 3)
	MCPAccess *mcpserver.Access
	MCPServer *mcpserver.Server

	// Jira (spec 002'den sonra credentials paketinin ilgilendiği tek şey)
	Credentials   *credentials.Store
	JiraValidator *credentials.Validator

	// Çalışma ayarları, projeler ve agent tanımları (spec 003)
	Settings     *settings.Service
	Projects     *projects.Store
	RepoVerifier *projects.Verifier
	Agents       *agentreg.Store

	// Çalıştırma
	Runs       *runs.Store
	RunManager *runs.Manager
	// RunBuilder, istekten çalıştırma girdisi üretir (workflow motoru da kullanır).
	RunBuilder *runbuild.Builder
	Pusher     *runs.Pusher
	Bus        *events.Bus

	// Rapor — yalnızca okur, çalıştırma geçmişini toplar.
	Reports *reports.Store

	// Akışlar (spec 007)
	Workflows    *workflow.Store
	WorkflowExec *workflow.Executor
	Launcher     *workflow.Launcher
	JiraTrigger  *jiratrigger.Trigger
}

// Handler, API'nin bağımlılıklarını ve durumunu taşır.
type Handler struct {
	deps      Deps
	startedAt time.Time

	// bg, istek yanıtından sonra devam eden işleri (katalog tazeleme) izler;
	// Shutdown bunları bekler, kaçak goroutine kalmaz.
	bg       sync.WaitGroup
	bgCtx    context.Context
	bgCancel context.CancelFunc
}

// NewHandler yeni bir API handler'ı oluşturur.
func NewHandler(deps Deps) *Handler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Handler{
		deps:      deps,
		startedAt: time.Now(),
		bgCtx:     ctx,
		bgCancel:  cancel,
	}
}

// Shutdown, arka plan işlerini iptal eder ve bitmelerini bekler.
func (h *Handler) Shutdown() {
	h.bgCancel()
	h.bg.Wait()
}

// Routes tüm uç noktaları bağlar ve http.Handler döner.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	// Zaman aşımı SSE dışındaki yollara uygulanır: canlı olay akışı
	// dakikalarca açık kalır ve genel timeout onu keserdi.
	r.Use(skipForSSE(middleware.Timeout(60 * time.Second)))
	r.Use(corsMiddleware(h.deps.Config.HTTP.CORSOrigins))

	r.Get("/health", h.health)
	r.Get("/readyz", h.ready)

	// Dışarıya açılan tek uç; bilinçli olarak /api altında değil.
	r.Post("/hooks/{token}", h.triggerHook)
	r.Post("/hooks/jira/{token}", h.jiraHook)

	// MCP sunucusu: dışarıdaki bir istemci akışları listeler ve başlatır.
	// Adres anahtar olduğu için /api altında değil.
	r.Handle("/mcp/{token}", http.HandlerFunc(h.mcpServe))

	r.Route("/api", func(r chi.Router) {
		r.Route("/llm-providers", func(r chi.Router) {
			r.Get("/", h.listLLMProviders)
			r.Post("/", h.createLLMProvider)
			r.Put("/{id}", h.updateLLMProvider)
			r.Delete("/{id}", h.deleteLLMProvider)
			r.Post("/{id}/sync", h.syncLLMProvider)
		})

		r.Route("/git-providers", func(r chi.Router) {
			r.Get("/", h.listGitProviders)
			r.Post("/", h.createGitProvider)
			r.Put("/{id}", h.updateGitProvider)
			r.Delete("/{id}", h.deleteGitProvider)
		})

		r.Get("/mcp-access", h.getMCPAccess)
		r.Post("/mcp-access/rotate", h.rotateMCPAccess)

		r.Route("/mcp-servers", func(r chi.Router) {
			r.Get("/", h.listMCPServers)
			r.Post("/", h.createMCPServer)
			r.Put("/{id}", h.updateMCPServer)
			r.Delete("/{id}", h.deleteMCPServer)
		})

		r.Route("/scripts", func(r chi.Router) {
			r.Get("/", h.listScripts)
			r.Post("/", h.createScript)
			r.Put("/{id}", h.updateScript)
			r.Delete("/{id}", h.deleteScript)
		})

		r.Route("/settings", func(r chi.Router) {
			r.Get("/", h.listSettings)
			r.Put("/{key}", h.putSetting)
			r.Delete("/{key}", h.resetSetting)
		})

		r.Route("/projects", func(r chi.Router) {
			r.Get("/", h.listProjects)
			r.Post("/", h.createProject)
			r.Put("/{id}", h.updateProject)
			r.Delete("/{id}", h.deleteProject)
		})

		r.Route("/agents", func(r chi.Router) {
			r.Get("/", h.listAgents)
			r.Post("/", h.createAgent)
			r.Put("/{id}", h.updateAgent)
			r.Post("/{id}/reset", h.resetAgent)
			r.Delete("/{id}", h.deleteAgent)
		})

		// Jira; LLM ve git erişimleri kendi uçlarına taşındı.
		r.Get("/credentials", h.listCredentials)
		r.Put("/credentials/{kind}", h.putCredential)
		r.Delete("/credentials/{kind}", h.deleteCredential)

		// Çalıştırma ortamı envanteri — DB'siz, sabit.
		r.Get("/runner/node-versions", h.nodeVersions)

		r.Route("/runs", func(r chi.Router) {
			r.Get("/", h.listRuns)
			r.Post("/", h.startRun)
			r.Get("/{id}", h.getRun)
			r.Delete("/{id}", h.deleteRun)
			r.Get("/{id}/events", h.runEvents)
			r.Get("/{id}/engine-logs", h.runEngineLogs)
			r.Post("/{id}/cancel", h.cancelRun)
			r.Post("/{id}/push", h.pushRun)
		})

		r.Route("/workflows", func(r chi.Router) {
			r.Get("/", h.listWorkflows)
			r.Post("/", h.createWorkflow)
			r.Get("/{id}", h.getWorkflow)
			r.Put("/{id}", h.updateWorkflow)
			r.Delete("/{id}", h.deleteWorkflow)
			r.Post("/{id}/versions", h.saveVersion)
			r.Post("/{id}/runs", h.startWorkflowRun)
			r.Post("/{id}/hook/rotate", h.rotateHook)
			r.Get("/{id}/jira-scan", h.jiraScanState)
		})

		r.Get("/workflow-versions/{id}", h.getVersion)

		r.Route("/workflow-runs", func(r chi.Router) {
			r.Get("/", h.listWorkflowRuns)
			r.Get("/{id}", h.getWorkflowRun)
			r.Get("/{id}/events", h.workflowRunEvents)
			r.Post("/{id}/cancel", h.cancelWorkflowRun)
		})

		r.Get("/reports/summary", h.reportSummary)

		r.Get("/models", h.listModels)
		r.Post("/models/refresh", h.refreshModels)

		r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			respondError(w, http.StatusNotFound, "not_found", "uç nokta bulunamadı")
		})
	})

	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		respondError(w, http.StatusNotFound, "not_found", "uç nokta bulunamadı")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		respondError(w, http.StatusMethodNotAllowed, "method_not_allowed", "bu metot desteklenmiyor")
	})

	return r
}

// skipForSSE, canlı olay akışı yollarında verilen middleware'i atlar.
func skipForSSE(mw func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		wrapped := mw(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/events") {
				next.ServeHTTP(w, r)
				return
			}
			wrapped.ServeHTTP(w, r)
		})
	}
}

// requestLogger her isteği yapılandırılmış biçimde loglar.
// Sağlık kontrolleri gürültü yapmasın diye debug seviyesine düşürülür.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(ww, r)

		level := slog.LevelInfo
		if r.URL.Path == "/health" || r.URL.Path == "/readyz" {
			level = slog.LevelDebug
		}
		// Sorgu dizesi bilinçli olarak loglanmaz: ileride gizli değer taşıyan
		// bir parametre eklenirse loglara sızmasın.
		slog.Log(r.Context(), level, "http isteği",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(r.Context()),
		)
	})
}

// corsMiddleware yalnızca izin verilen origin'lere CORS başlıkları ekler.
// Joker "*" bilinçli olarak desteklenmez — izinli liste .env üzerinden gelir.
func corsMiddleware(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowed, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "300")
				w.Header().Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
