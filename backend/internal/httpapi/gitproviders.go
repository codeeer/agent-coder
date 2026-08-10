package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/agent-coder/backend/internal/gitprovider"
)

// gitProviderResponse, git erişimi ve doğrulanıp doğrulanamadığı.
type gitProviderResponse struct {
	gitprovider.Provider
	// Verified false ise erişim bilgisi doğrulanamadan kaydedilmiştir
	// (genel Git türü). Arayüz bunu uyarı olarak gösterir.
	Verified bool `json:"verified"`
}

type createGitProviderRequest struct {
	Type     gitprovider.Type `json:"type"`
	Name     string           `json:"name"`
	BaseURL  string           `json:"baseUrl"`
	Username string           `json:"username"`
	Secret   string           `json:"secret"`
}

type updateGitProviderRequest struct {
	Name     *string `json:"name"`
	BaseURL  *string `json:"baseUrl"`
	Username *string `json:"username"`
	Secret   *string `json:"secret"`
}

func (h *Handler) listGitProviders(w http.ResponseWriter, r *http.Request) {
	if h.deps.GitProviders == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	providers, err := h.deps.GitProviders.List(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "git sağlayıcılar listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "git erişimleri okunamadı")
		return
	}

	out := make([]gitProviderResponse, 0, len(providers))
	for _, p := range providers {
		out = append(out, gitProviderResponse{
			Provider: p,
			Verified: p.Type != gitprovider.TypeGeneric,
		})
	}
	respondJSON(w, http.StatusOK, out)
}

// createGitProvider, git erişimini doğrular ve kaydeder.
//
// Genel Git türünde doğrulama mümkün değildir; kayıt yine yapılır ve yanıtta
// verified=false döner (spec 002 H5).
func (h *Handler) createGitProvider(w http.ResponseWriter, r *http.Request) {
	if h.deps.GitProviders == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	var req createGitProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}
	if req.Secret == "" {
		respondError(w, http.StatusBadRequest, "missing_secret", "erişim bilgisi boş olamaz")
		return
	}

	name, baseURL, username, err := gitprovider.Validate(req.Type, req.Name, req.BaseURL, req.Username)
	if err != nil {
		h.respondGitError(w, r, err)
		return
	}

	verified := true
	err = h.deps.GitValidator.Validate(ctx, gitprovider.Provider{
		Type: req.Type, BaseURL: baseURL, Username: username,
	}, req.Secret)
	switch {
	case err == nil:
	case errors.Is(err, gitprovider.ErrNotVerifiable):
		// Doğrulanamadı ama kayıt engellenmez.
		verified = false
	default:
		h.respondGitError(w, r, err)
		return
	}

	p, err := h.deps.GitProviders.Create(ctx, gitprovider.CreateInput{
		Type: req.Type, Name: name, BaseURL: baseURL,
		Username: username, Secret: req.Secret,
	})
	if err != nil {
		h.respondGitError(w, r, err)
		return
	}

	slog.InfoContext(ctx, "git erişimi eklendi", "id", p.ID, "type", p.Type, "verified", verified)
	respondJSON(w, http.StatusCreated, gitProviderResponse{Provider: p, Verified: verified})
}

func (h *Handler) updateGitProvider(w http.ResponseWriter, r *http.Request) {
	if h.deps.GitProviders == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req updateGitProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	p, err := h.deps.GitProviders.Update(ctx, id, gitprovider.UpdateInput{
		Name: req.Name, BaseURL: req.BaseURL, Username: req.Username, Secret: req.Secret,
	})
	if err != nil {
		h.respondGitError(w, r, err)
		return
	}

	slog.InfoContext(ctx, "git erişimi güncellendi", "id", p.ID)
	respondJSON(w, http.StatusOK, gitProviderResponse{
		Provider: p, Verified: p.Type != gitprovider.TypeGeneric,
	})
}

func (h *Handler) deleteGitProvider(w http.ResponseWriter, r *http.Request) {
	if h.deps.GitProviders == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	err := h.deps.GitProviders.Delete(r.Context(), id)
	switch {
	case err == nil:
		slog.InfoContext(r.Context(), "git erişimi silindi", "id", id)
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, gitprovider.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "git erişimi bulunamadı")
	default:
		slog.ErrorContext(r.Context(), "git erişimi silinemedi", "id", id, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "git erişimi silinemedi")
	}
}

func (h *Handler) respondGitError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, gitprovider.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "git erişimi bulunamadı")

	case errors.Is(err, gitprovider.ErrInvalidSecret):
		respondError(w, http.StatusUnprocessableEntity, "invalid_credential",
			"erişim bilgisi doğrulanamadı — değeri kontrol edin")

	case errors.Is(err, gitprovider.ErrMissingUsername):
		respondError(w, http.StatusBadRequest, "missing_username",
			"bu tür için kullanıcı adı zorunlu")

	case errors.Is(err, gitprovider.ErrMissingBaseURL):
		respondError(w, http.StatusBadRequest, "missing_base_url",
			"bu tür için servis adresi zorunlu")

	case errors.Is(err, gitprovider.ErrInvalidBaseURL):
		respondError(w, http.StatusBadRequest, "invalid_base_url", err.Error())

	case errors.Is(err, gitprovider.ErrInvalidType):
		respondError(w, http.StatusBadRequest, "invalid_type", "geçersiz git sağlayıcı türü")

	case errors.Is(err, gitprovider.ErrEmptyName):
		respondError(w, http.StatusBadRequest, "invalid_name", err.Error())

	case errors.Is(err, gitprovider.ErrUnreachable):
		respondError(w, http.StatusServiceUnavailable, "service_unreachable",
			"servise ulaşılamadı — tekrar deneyin")

	default:
		slog.ErrorContext(r.Context(), "git erişimi işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "işlem tamamlanamadı")
	}
}
