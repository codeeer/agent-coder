package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/agent-coder/backend/internal/credentials"
)

// putCredentialRequest, kimlik bilgisi kaydetme gövdesi.
type putCredentialRequest struct {
	Secret   string            `json:"secret"`
	Metadata map[string]string `json:"metadata"`
}

// listCredentials, kayıtlı kimlik bilgilerini gizli değer olmadan döner.
//
// Yanıt tipi credentials.Credential'dır ve o tip gizli değeri hiç taşımaz —
// buradan anahtar sızdırmak mümkün değil.
func (h *Handler) listCredentials(w http.ResponseWriter, r *http.Request) {
	if h.deps.Credentials == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	creds, err := h.deps.Credentials.List(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "kimlik bilgileri listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "kimlik bilgileri okunamadı")
		return
	}
	if creds == nil {
		creds = []credentials.Credential{}
	}
	respondJSON(w, http.StatusOK, creds)
}

// putCredential, kimlik bilgisini doğrular ve kaydeder.
//
// Doğrulama BAŞARISIZSA kayıt yapılmaz: geçersiz bir anahtarın veritabanına girip
// ilk gerçek kullanımda patlaması, hatanın kaynağını çok daha zor bulunur kılardı.
func (h *Handler) putCredential(w http.ResponseWriter, r *http.Request) {
	if h.deps.Credentials == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	kind := credentials.Kind(chi.URLParam(r, "kind"))
	if !kind.Valid() {
		respondError(w, http.StatusBadRequest, "invalid_kind", "geçersiz kimlik bilgisi türü")
		return
	}

	var req putCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}
	if req.Secret == "" {
		respondError(w, http.StatusBadRequest, "missing_secret", "gizli değer boş olamaz")
		return
	}

	if err := h.deps.JiraValidator.Validate(r.Context(), kind, req.Secret, req.Metadata); err != nil {
		h.respondValidationError(w, r, kind, err)
		return
	}

	if err := h.deps.Credentials.Put(r.Context(), kind, req.Secret, req.Metadata); err != nil {
		slog.ErrorContext(r.Context(), "kimlik bilgisi kaydedilemedi", "kind", kind, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "kimlik bilgisi kaydedilemedi")
		return
	}

	saved, err := h.deps.Credentials.Get(r.Context(), kind)
	if err != nil {
		slog.ErrorContext(r.Context(), "kaydedilen kimlik bilgisi okunamadı", "kind", kind, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "kimlik bilgisi okunamadı")
		return
	}

	slog.InfoContext(r.Context(), "kimlik bilgisi kaydedildi", "kind", kind)

	respondJSON(w, http.StatusOK, saved)
}

// deleteCredential, kimlik bilgisini siler.
func (h *Handler) deleteCredential(w http.ResponseWriter, r *http.Request) {
	if h.deps.Credentials == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	kind := credentials.Kind(chi.URLParam(r, "kind"))
	if !kind.Valid() {
		respondError(w, http.StatusBadRequest, "invalid_kind", "geçersiz kimlik bilgisi türü")
		return
	}

	err := h.deps.Credentials.Delete(r.Context(), kind)
	switch {
	case err == nil:
		slog.InfoContext(r.Context(), "kimlik bilgisi silindi", "kind", kind)
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, credentials.ErrNotConfigured):
		respondError(w, http.StatusNotFound, "not_configured", "bu türde kayıtlı kimlik bilgisi yok")
	default:
		slog.ErrorContext(r.Context(), "kimlik bilgisi silinemedi", "kind", kind, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "kimlik bilgisi silinemedi")
	}
}

// respondValidationError, doğrulama hatasını uygun HTTP koduna çevirir.
//
// "Anahtar yanlış" ile "servise ulaşılamadı" ayrımı korunur; kullanıcı ikisinde
// farklı şey yapar (birinde anahtarı düzeltir, diğerinde tekrar dener).
func (h *Handler) respondValidationError(w http.ResponseWriter, r *http.Request, kind credentials.Kind, err error) {
	switch {
	case errors.Is(err, credentials.ErrInvalidSecret):
		slog.InfoContext(r.Context(), "kimlik bilgisi doğrulanamadı", "kind", kind)
		respondError(w, http.StatusUnprocessableEntity, "invalid_credential",
			"kimlik bilgisi doğrulanamadı — değeri kontrol edin")

	case errors.Is(err, credentials.ErrMissingMetadata):
		respondError(w, http.StatusBadRequest, "missing_metadata", err.Error())

	case errors.Is(err, credentials.ErrServiceUnreachable):
		slog.WarnContext(r.Context(), "doğrulama servisine ulaşılamadı", "kind", kind, "error", err)
		respondError(w, http.StatusServiceUnavailable, "service_unreachable",
			"doğrulama servisine şu an ulaşılamıyor — tekrar deneyin")

	default:
		slog.ErrorContext(r.Context(), "doğrulama başarısız", "kind", kind, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "doğrulama yapılamadı")
	}
}
