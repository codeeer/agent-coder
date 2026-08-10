package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/agent-coder/backend/internal/settings"
)

// settingsResponse, ayarlar listesi ve grup etiketleri.
//
// Arayüz kendini bu yanıttan çizer: her ayarın etiketi, açıklaması, tipi,
// birimi ve aralığı burada gelir. Yeni bir parametre eklendiğinde frontend'e
// dokunmak gerekmez.
type settingsResponse struct {
	Items  []settings.Value  `json:"items"`
	Groups map[string]string `json:"groups"`
}

type putSettingRequest struct {
	Value string `json:"value"`
}

func (h *Handler) listSettings(w http.ResponseWriter, _ *http.Request) {
	if h.deps.Settings == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	respondJSON(w, http.StatusOK, settingsResponse{
		Items:  h.deps.Settings.All(),
		Groups: settings.GroupLabels,
	})
}

func (h *Handler) putSetting(w http.ResponseWriter, r *http.Request) {
	if h.deps.Settings == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	key := chi.URLParam(r, "key")

	var req putSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	if err := h.deps.Settings.Set(r.Context(), key, req.Value); err != nil {
		h.respondSettingsError(w, r, key, err)
		return
	}

	slog.InfoContext(r.Context(), "ayar değiştirildi", "key", key, "value", req.Value)
	respondJSON(w, http.StatusOK, settingsResponse{
		Items:  h.deps.Settings.All(),
		Groups: settings.GroupLabels,
	})
}

func (h *Handler) resetSetting(w http.ResponseWriter, r *http.Request) {
	if h.deps.Settings == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	key := chi.URLParam(r, "key")

	if err := h.deps.Settings.Reset(r.Context(), key); err != nil {
		h.respondSettingsError(w, r, key, err)
		return
	}

	slog.InfoContext(r.Context(), "ayar varsayılana döndürüldü", "key", key)
	respondJSON(w, http.StatusOK, settingsResponse{
		Items:  h.deps.Settings.All(),
		Groups: settings.GroupLabels,
	})
}

// respondSettingsError, ayar hatalarını uygun HTTP koduna çevirir.
//
// Aralık hatasının mesajı olduğu gibi geçirilir: kullanıcının ne yapması
// gerektiğini ("1 ile 20 arasında olmalı") tam olarak o söylüyor.
func (h *Handler) respondSettingsError(w http.ResponseWriter, r *http.Request, key string, err error) {
	switch {
	case errors.Is(err, settings.ErrUnknownKey):
		respondError(w, http.StatusNotFound, "unknown_setting", "bilinmeyen ayar")

	case errors.Is(err, settings.ErrOutOfRange), errors.Is(err, settings.ErrInvalidValue):
		respondError(w, http.StatusBadRequest, "invalid_value", err.Error())

	default:
		slog.ErrorContext(r.Context(), "ayar işlemi başarısız", "key", key, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "ayar kaydedilemedi")
	}
}
