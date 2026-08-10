package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/reports"
)

// reportSummary, GET /api/reports/summary — yönetici özeti.
//
// Tüm bölümler tek yanıtta döner: sayfa altı kırılım gösteriyor ve bunları ayrı
// isteklere bölmek hem yavaş olur hem de aralarında yeni kayıt düşerse
// birbirini tutmayan rakamlar gösterilirdi.
func (h *Handler) reportSummary(w http.ResponseWriter, r *http.Request) {
	if h.deps.Reports == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	q := reports.Query{Days: atoiOr(r.URL.Query().Get("days"), 0)}
	if q.Days < 0 || q.Days > reports.MaxDays {
		respondError(w, http.StatusBadRequest, "invalid_days",
			"dönem 1 ile 365 gün arasında olmalı")
		return
	}

	if raw := r.URL.Query().Get("project"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_project", "geçersiz proje kimliği")
			return
		}
		q.ProjectID = &id
	}

	summary, err := h.deps.Reports.Summary(r.Context(), q)
	if err != nil {
		slog.ErrorContext(r.Context(), "rapor üretilemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "rapor hazırlanamadı")
		return
	}

	respondJSON(w, http.StatusOK, summary)
}
