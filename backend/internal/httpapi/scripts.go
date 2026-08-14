package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"log/slog"
	"net/http"

	"github.com/agent-coder/backend/internal/scripts"
)

type createScriptRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Content     string     `json:"content"`
	FolderID    *uuid.UUID `json:"folderId"`
}

type updateScriptRequest struct {
	Name        *string    `json:"name"`
	Description *string    `json:"description"`
	Content     *string    `json:"content"`
	FolderID    *uuid.UUID `json:"folderId"`

	// ClearFolder, script'i klasörden çıkarır.
	//
	// AYRI BİR ALAN olmak zorunda: `folderId: null` ile alanın hiç
	// gönderilmemesi JSON'da aynı görünüyor ve ilki "klasörden çıkar",
	// ikincisi "dokunma" demek.
	ClearFolder bool `json:"clearFolder"`
}

func (h *Handler) listScripts(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scripts == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	page := pageOf(r)
	list, total, err := h.deps.Scripts.List(r.Context(), page.Limit, page.Offset)
	if err != nil {
		slog.ErrorContext(r.Context(), "betikler listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "betikler okunamadı")
		return
	}
	respondJSON(w, http.StatusOK, paged(list, total, page))
}

func (h *Handler) createScript(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scripts == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	var req createScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	s, err := h.deps.Scripts.Create(r.Context(), scripts.CreateInput{
		Name: req.Name, Description: req.Description, Content: req.Content,
		FolderID: req.FolderID,
	})
	if err != nil {
		h.respondScriptError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "betik oluşturuldu", "id", s.ID, "name", s.Name)
	respondJSON(w, http.StatusCreated, s)
}

func (h *Handler) updateScript(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scripts == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req updateScriptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	/*
	 * `folderId` alanı GÖNDERİLMEDİYSE klasör değişmez; AÇIKÇA null
	 * gönderildiyse klasörden çıkarılır. İkisini ayırt etmek için ham gövdeye
	 * bakılıyor — tek bir işaretçide nil hem "dokunma" hem "boşalt" olurdu.
	 */
	s, err := h.deps.Scripts.Update(r.Context(), id, scripts.UpdateInput{
		Name: req.Name, Description: req.Description, Content: req.Content,
		FolderID: req.FolderID,
	}, req.ClearFolder)
	if err != nil {
		h.respondScriptError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "betik güncellendi", "id", s.ID, "name", s.Name)
	respondJSON(w, http.StatusOK, s)
}

func (h *Handler) deleteScript(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scripts == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.deps.Scripts.Delete(r.Context(), id); err != nil {
		h.respondScriptError(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "betik silindi", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) respondScriptError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scripts.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "betik bulunamadı")
	case errors.Is(err, scripts.ErrDuplicateName):
		respondError(w, http.StatusConflict, "duplicate_name", "bu adda bir betik zaten var")
	case errors.Is(err, scripts.ErrMissingName), errors.Is(err, scripts.ErrInvalidName),
		errors.Is(err, scripts.ErrMissingContent):
		respondError(w, http.StatusBadRequest, "invalid_script", err.Error())
	default:
		slog.ErrorContext(r.Context(), "betik işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "işlem tamamlanamadı")
	}
}
