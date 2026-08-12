package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/agentreg"
	"github.com/agent-coder/backend/internal/llm"
	"github.com/agent-coder/backend/internal/projects"
	"github.com/agent-coder/backend/internal/runbuild"
	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runs"
)

type startRunRequest struct {
	ProjectID  uuid.UUID  `json:"projectId"`
	AgentID    uuid.UUID  `json:"agentId"`
	ProviderID *uuid.UUID `json:"providerId"`
	Model      string     `json:"model"`
	Branch     string     `json:"branch"`
	Task       string     `json:"task"`
	// NodeVersion boşsa projenin varsayılanı kullanılır.
	NodeVersion string `json:"nodeVersion"`
}

/*
 * Çalıştırma listesi yanıtı.
 *
 * `limit`/`offset` SAYFA penceresidir — diğer tüm liste uçlarıyla aynı anlam.
 * Eşzamanlılık sınırı ayrı bir alanda (`concurrencyLimit`) durur: ikisi de
 * `limit` diye adlandırıldığında arayüzdeki sayfalama denetimi sayfa boyutu
 * sanıp eşzamanlılık sınırını okuyordu ve "1–3 / 29" gibi yanlış bir aralık
 * çiziyordu.
 */
type runListResponse struct {
	Items  []runs.Run `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`

	// Active ve ConcurrencyLimit sayfalamayla ilgisizdir: o an kaç iş çalıştığı
	// ve aynı anda kaç işe izin verildiği.
	Active           int `json:"active"`
	ConcurrencyLimit int `json:"concurrencyLimit"`
}

type pushRunRequest struct {
	Branch string `json:"branch"`
}

func (h *Handler) listRuns(w http.ResponseWriter, r *http.Request) {
	if h.deps.Runs == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	q := r.URL.Query()

	page := pageOf(r)
	filter := runs.ListFilter{Limit: page.Limit, Offset: page.Offset}
	if raw := q.Get("project"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_project", "geçersiz proje kimliği")
			return
		}
		filter.ProjectID = &id
	}

	items, total, err := h.deps.Runs.List(r.Context(), filter)
	if err != nil {
		slog.ErrorContext(r.Context(), "çalıştırmalar listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "çalıştırmalar okunamadı")
		return
	}

	respondJSON(w, http.StatusOK, runListResponse{
		Items:            items,
		Total:            total,
		Limit:            page.Limit,
		Offset:           page.Offset,
		Active:           h.deps.RunManager.Active(),
		ConcurrencyLimit: h.deps.Settings.Int("runner.max_concurrent"),
	})
}

func (h *Handler) getRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.Runs == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	run, err := h.deps.Runs.Get(r.Context(), id)
	if err != nil {
		h.respondRunError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, run)
}

// startRun, çalıştırmayı başlatır ve HEMEN döner.
//
// İş arka planda sürer; kullanıcı ilerleme ekranına geçip SSE ile izler.
func (h *Handler) startRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.RunManager == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	var req startRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}
	if strings.TrimSpace(req.Task) == "" {
		respondError(w, http.StatusBadRequest, "empty_task", "görev metni boş olamaz")
		return
	}
	// Yayınlanmamış bir sürüm kabul edilseydi çalıştırma başlar ve ancak
	// "imaj bulunamadı" ile düşerdi; istek daha uca varmadan reddediliyor.
	if !runner.SupportsNodeVersion(strings.TrimSpace(req.NodeVersion)) {
		respondError(w, http.StatusBadRequest, "invalid_node_version",
			"bu Node sürümünün imajı yayınlanmamış")
		return
	}

	input, err := h.deps.RunBuilder.Build(ctx, runbuild.Request{
		ProjectID:  req.ProjectID,
		AgentID:    req.AgentID,
		ProviderID: req.ProviderID,
		Model:      req.Model,
		Branch:     req.Branch,
		Task:       req.Task,

		NodeVersion: strings.TrimSpace(req.NodeVersion),
	})
	if err != nil {
		h.respondRunError(w, r, err)
		return
	}

	run, err := h.deps.RunManager.Start(ctx, input)
	if err != nil {
		h.respondRunError(w, r, err)
		return
	}

	slog.InfoContext(ctx, "çalıştırma başlatıldı",
		"run_id", run.ID, "agent", run.AgentSlug, "model", run.ModelID)
	respondJSON(w, http.StatusCreated, run)
}

func (h *Handler) cancelRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.RunManager == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.deps.RunManager.Cancel(id); err != nil {
		h.respondRunError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "çalıştırma iptal edildi", "run_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// deleteRun, bitmiş bir çalıştırma kaydını kalıcı olarak siler.
//
// Manager üzerinden geçer, doğrudan store'dan değil: "çalışıyor mu" sorusunun
// tek doğru cevabı bellekteki canlı iş listesiyle birlikte verilebiliyor.
func (h *Handler) deleteRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.RunManager == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.deps.RunManager.Delete(r.Context(), id); err != nil {
		h.respondRunError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "çalıştırma silindi", "run_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// pushRun, çalıştırmanın değişikliklerini yeni bir branch'e gönderir.
func (h *Handler) pushRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.Pusher == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req pushRunRequest
	// Gövde isteğe bağlı: branch adı verilmezse önerilen kullanılır.
	_ = json.NewDecoder(r.Body).Decode(&req)

	run, err := h.deps.Runs.Get(ctx, id)
	if err != nil {
		h.respondRunError(w, r, err)
		return
	}

	project, err := h.deps.Projects.Get(ctx, run.ProjectID)
	if err != nil {
		h.respondRunError(w, r, err)
		return
	}
	repoURL, creds, err := h.deps.RunBuilder.RepoAccess(ctx, project.ID)
	if err != nil {
		h.respondRunError(w, r, err)
		return
	}

	branch, err := h.deps.Pusher.Push(ctx, runs.PushRequest{
		Run:    run,
		Repo:   repoURL,
		Creds:  creds,
		Branch: req.Branch,
	})
	if err != nil {
		h.respondRunError(w, r, err)
		return
	}

	slog.InfoContext(ctx, "değişiklikler gönderildi", "run_id", id, "branch", branch)
	respondJSON(w, http.StatusOK, map[string]string{"branch": branch})
}

func (h *Handler) respondRunError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, runs.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "çalıştırma bulunamadı")

	case errors.Is(err, runs.ErrTooManyRuns):
		respondError(w, http.StatusTooManyRequests, "too_many_runs", err.Error())

	case errors.Is(err, runs.ErrNotRunning):
		respondError(w, http.StatusConflict, "not_running",
			"çalıştırma zaten bitmiş, iptal edilemez")

	case errors.Is(err, runs.ErrActive):
		respondError(w, http.StatusConflict, "run_active",
			"çalıştırma hâlâ sürüyor — önce iptal edin")

	// Adım tek başına silinemez: kayıt gitse de akış adımı "başarılı" görünmeye
	// devam eder, ama maliyeti ve hangi agent'ın çalıştığı boşalır.
	case errors.Is(err, runs.ErrWorkflowStep):
		respondError(w, http.StatusConflict, "run_workflow_step",
			"bu çalıştırma bir akışın adımı — akış çalışmasını silin")

	case errors.Is(err, runs.ErrNoChanges):
		respondError(w, http.StatusConflict, "no_changes",
			"bu çalıştırma kod değişikliği üretmedi")

	case errors.Is(err, runs.ErrAlreadyPushed):
		respondError(w, http.StatusConflict, "already_pushed", err.Error())

	case errors.Is(err, runs.ErrNoGitAccess):
		respondError(w, http.StatusPreconditionFailed, "no_git_access",
			"projede git erişimi tanımlı değil — projeyi düzenleyip bir erişim seçin")

	case errors.Is(err, runbuild.ErrNoModel):
		respondError(w, http.StatusBadRequest, "no_model",
			"model seçilmedi ve agent'ın varsayılan modeli yok")

	/*
	 * Hiç LLM sağlayıcı tanımlı değil.
	 *
	 * Bu bir YAPILANDIRMA eksiği, sistem arızası değil. Haritalanmadığı sürece
	 * `default` dalına düşüp 500 "internal_error" dönüyordu: yeni kurulum yapan
	 * biri ilk çalıştırmasında uygulamanın bozuk olduğunu sanırdı. Git erişimi
	 * eksikliğinde zaten aynı özenle davranılıyor.
	 *
	 * Sağlayıcı türü kasıtlı olarak anılmıyor: OpenRouter zorunlu değil, LiteLLM
	 * ve OpenAI-uyumlu servisler de aynı ekrandan tanımlanıyor.
	 */
	case errors.Is(err, llm.ErrNotFound):
		respondError(w, http.StatusPreconditionFailed, "no_llm_provider",
			"tanımlı LLM sağlayıcı yok — Ayarlar → Modeller bölümünden "+
				"bir sağlayıcı ekleyin (OpenRouter, LiteLLM veya OpenAI-uyumlu servis)")

	case errors.Is(err, projects.ErrNotFound):
		respondError(w, http.StatusBadRequest, "project_not_found", "proje bulunamadı")

	case errors.Is(err, agentreg.ErrNotFound):
		respondError(w, http.StatusBadRequest, "agent_not_found", "agent bulunamadı")

	default:
		slog.ErrorContext(r.Context(), "çalıştırma işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}
