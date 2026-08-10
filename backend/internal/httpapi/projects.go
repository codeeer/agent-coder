package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/gitprovider"
	"github.com/agent-coder/backend/internal/projects"
)

// projectResponse, proje ve kaç çalıştırması olduğu.
type projectResponse struct {
	projects.Project
	RunCount int `json:"runCount"`
}

type projectRequest struct {
	Name          string     `json:"name"`
	RepoURL       string     `json:"repoUrl"`
	DefaultBranch string     `json:"defaultBranch"`
	GitProviderID *uuid.UUID `json:"gitProviderId"`
	// ClearGitProvider true ise erişim kaldırılır (açık depoya dönüştürülür).
	ClearGitProvider bool `json:"clearGitProvider"`
}

func (h *Handler) listProjects(w http.ResponseWriter, r *http.Request) {
	if h.deps.Projects == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	page := pageOf(r)
	list, total, err := h.deps.Projects.List(ctx, page)
	if err != nil {
		slog.ErrorContext(ctx, "projeler listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "projeler okunamadı")
		return
	}

	out := make([]projectResponse, 0, len(list))
	for _, p := range list {
		n, err := h.deps.Projects.RunCount(ctx, p.ID)
		if err != nil {
			slog.WarnContext(ctx, "çalıştırma sayısı okunamadı", "project_id", p.ID, "error", err)
		}
		out = append(out, projectResponse{Project: p, RunCount: n})
	}
	respondJSON(w, http.StatusOK, paged(out, total, page))
}

// createProject, projeyi kaydetmeden ÖNCE depoya erişilebildiğini doğrular.
//
// Erişilemeyen bir depoyu kaydetmek, hatanın ilk çalıştırmada ve anlaşılmaz
// biçimde ortaya çıkması demek olurdu.
func (h *Handler) createProject(w http.ResponseWriter, r *http.Request) {
	if h.deps.Projects == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	var req projectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	in := projects.Input{
		Name: req.Name, RepoURL: req.RepoURL,
		DefaultBranch: req.DefaultBranch, GitProviderID: req.GitProviderID,
	}
	if err := in.Normalize(); err != nil {
		h.respondProjectError(w, r, err)
		return
	}

	if err := h.verifyRepoAccess(ctx, in); err != nil {
		h.respondProjectError(w, r, err)
		return
	}

	p, err := h.deps.Projects.Create(ctx, in)
	if err != nil {
		h.respondProjectError(w, r, err)
		return
	}

	slog.InfoContext(ctx, "proje eklendi", "id", p.ID, "name", p.Name)
	respondJSON(w, http.StatusCreated, projectResponse{Project: p})
}

func (h *Handler) updateProject(w http.ResponseWriter, r *http.Request) {
	if h.deps.Projects == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req projectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	in := projects.Input{
		Name: req.Name, RepoURL: req.RepoURL,
		DefaultBranch: req.DefaultBranch, GitProviderID: req.GitProviderID,
	}
	if err := in.Normalize(); err != nil {
		h.respondProjectError(w, r, err)
		return
	}
	if err := h.verifyRepoAccess(ctx, in); err != nil {
		h.respondProjectError(w, r, err)
		return
	}

	p, err := h.deps.Projects.Update(ctx, id, in, req.ClearGitProvider)
	if err != nil {
		h.respondProjectError(w, r, err)
		return
	}

	slog.InfoContext(ctx, "proje güncellendi", "id", p.ID)
	respondJSON(w, http.StatusOK, projectResponse{Project: p})
}

func (h *Handler) deleteProject(w http.ResponseWriter, r *http.Request) {
	if h.deps.Projects == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	err := h.deps.Projects.Delete(r.Context(), id)
	switch {
	case err == nil:
		slog.InfoContext(r.Context(), "proje silindi", "id", id)
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, projects.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "proje bulunamadı")
	default:
		slog.ErrorContext(r.Context(), "proje silinemedi", "id", id, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "proje silinemedi")
	}
}

// verifyRepoAccess, depoya erişilebildiğini ve branch'in var olduğunu sınar.
func (h *Handler) verifyRepoAccess(ctx contextT, in projects.Input) error {
	if h.deps.RepoVerifier == nil {
		return nil
	}

	var creds *projects.Credentials
	if in.GitProviderID != nil && h.deps.GitProviders != nil {
		gp, err := h.deps.GitProviders.Get(ctx, *in.GitProviderID)
		if err != nil {
			return err
		}
		secret, err := h.deps.GitProviders.Reveal(ctx, gp.ID)
		if err != nil {
			return err
		}
		username := gp.Username
		if username == "" {
			// GitHub token'ı kullanıcı adı istemez; git yine de bir ad bekler.
			username = "x-access-token"
		}
		creds = &projects.Credentials{Username: username, Secret: secret}
	}

	return h.deps.RepoVerifier.Verify(ctx, in.RepoURL, in.DefaultBranch, creds)
}

func (h *Handler) respondProjectError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, projects.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "proje bulunamadı")

	case errors.Is(err, projects.ErrEmptyName):
		respondError(w, http.StatusBadRequest, "invalid_name", "proje adı boş olamaz")

	case errors.Is(err, projects.ErrInvalidRepoURL):
		respondError(w, http.StatusBadRequest, "invalid_repo_url", err.Error())

	case errors.Is(err, projects.ErrRepoAuth):
		respondError(w, http.StatusUnprocessableEntity, "repo_auth",
			"depo erişimi reddedildi — git erişim bilgilerini kontrol edin")

	case errors.Is(err, projects.ErrBranchNotFound):
		respondError(w, http.StatusUnprocessableEntity, "branch_not_found", err.Error())

	case errors.Is(err, projects.ErrRepoUnreachable):
		respondError(w, http.StatusUnprocessableEntity, "repo_unreachable", err.Error())

	case errors.Is(err, gitprovider.ErrNotFound):
		respondError(w, http.StatusBadRequest, "git_provider_not_found",
			"seçilen git erişimi bulunamadı")

	default:
		slog.ErrorContext(r.Context(), "proje işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "işlem tamamlanamadı")
	}
}
