package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/agentreg"
)

type createAgentRequest struct {
	Slug              string     `json:"slug"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	Prompt            string     `json:"prompt"`
	DefaultProviderID *uuid.UUID `json:"defaultProviderId"`
	DefaultModel      string     `json:"defaultModel"`
	AllowEdit         bool       `json:"allowEdit"`
	AllowBash         bool       `json:"allowBash"`
	AllowWebfetch     bool       `json:"allowWebfetch"`
}

type updateAgentRequest struct {
	Name              *string    `json:"name"`
	Description       *string    `json:"description"`
	Prompt            *string    `json:"prompt"`
	DefaultProviderID *uuid.UUID `json:"defaultProviderId"`
	ClearProvider     bool       `json:"clearProvider"`
	DefaultModel      *string    `json:"defaultModel"`
	AllowEdit         *bool      `json:"allowEdit"`
	AllowBash         *bool      `json:"allowBash"`
	AllowWebfetch     *bool      `json:"allowWebfetch"`
}

func (h *Handler) listAgents(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	page := pageOf(r)
	list, total, err := h.deps.Agents.List(r.Context(), page)
	if err != nil {
		slog.ErrorContext(r.Context(), "agent'lar listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "agent'lar okunamadı")
		return
	}
	respondJSON(w, http.StatusOK, paged(list, total, page))
}

func (h *Handler) createAgent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	var req createAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	a, err := h.deps.Agents.Create(r.Context(), agentreg.CreateInput{
		Slug: req.Slug, Name: req.Name, Description: req.Description, Prompt: req.Prompt,
		DefaultProviderID: req.DefaultProviderID, DefaultModel: req.DefaultModel,
		AllowEdit: req.AllowEdit, AllowBash: req.AllowBash, AllowWebfetch: req.AllowWebfetch,
	})
	if err != nil {
		h.respondAgentError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "agent oluşturuldu", "id", a.ID, "slug", a.Slug)
	respondJSON(w, http.StatusCreated, a)
}

func (h *Handler) updateAgent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req updateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	a, err := h.deps.Agents.Update(r.Context(), id, agentreg.UpdateInput{
		Name: req.Name, Description: req.Description, Prompt: req.Prompt,
		DefaultProviderID: req.DefaultProviderID, ClearProvider: req.ClearProvider,
		DefaultModel: req.DefaultModel,
		AllowEdit:    req.AllowEdit, AllowBash: req.AllowBash, AllowWebfetch: req.AllowWebfetch,
	})
	if err != nil {
		h.respondAgentError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "agent güncellendi", "id", a.ID, "değiştirilmiş", a.IsModified)
	respondJSON(w, http.StatusOK, a)
}

// resetAgent, hazır bir agent'ı özgün haline döndürür.
func (h *Handler) resetAgent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	a, err := h.deps.Agents.Reset(r.Context(), id)
	if err != nil {
		h.respondAgentError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "agent sıfırlandı", "id", a.ID)
	respondJSON(w, http.StatusOK, a)
}

func (h *Handler) deleteAgent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	err := h.deps.Agents.Delete(r.Context(), id)
	if err != nil {
		h.respondAgentError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "agent silindi", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) respondAgentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, agentreg.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "agent bulunamadı")

	case errors.Is(err, agentreg.ErrSlugTaken):
		respondError(w, http.StatusConflict, "slug_taken", "bu kısa ad zaten kullanılıyor")

	case errors.Is(err, agentreg.ErrBuiltinDelete):
		respondError(w, http.StatusConflict, "builtin_delete",
			"hazır agent silinemez — özgün haline sıfırlayabilirsiniz")

	case errors.Is(err, agentreg.ErrNotBuiltin):
		respondError(w, http.StatusConflict, "not_builtin",
			"yalnızca hazır agent'lar sıfırlanabilir")

	case errors.Is(err, agentreg.ErrInUse):
		respondError(w, http.StatusConflict, "agent_in_use",
			"bu agent'ın çalıştırma geçmişi var — geçmiş kaydı için silinemez")

	case errors.Is(err, agentreg.ErrEmptyPrompt):
		respondError(w, http.StatusBadRequest, "empty_prompt", "agent talimatı boş olamaz")

	case errors.Is(err, agentreg.ErrPromptTooLarge):
		respondError(w, http.StatusBadRequest, "prompt_too_large", err.Error())

	case errors.Is(err, agentreg.ErrInvalidSlug):
		respondError(w, http.StatusBadRequest, "invalid_slug", err.Error())

	default:
		slog.ErrorContext(r.Context(), "agent işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "işlem tamamlanamadı")
	}
}
