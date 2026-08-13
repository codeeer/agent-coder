package httpapi

import (
	"log/slog"
	"net/http"
)

/*
 * Motorun ham logları — koşu detayındaki teşhis katmanı.
 *
 * İlerleme akışı (`/events`) kullanıcı dostu olay listesidir; bu uç ham
 * gerçeği döndürür. İkisi birbirinin yerine geçmez: biri "ne oldu", diğeri
 * "tam olarak ne yazıldı".
 *
 * İçerik AÇILMIŞ döner (gunzip); sıkıştırma bir saklama detayıdır ve
 * istemciyi ilgilendirmez.
 */

type engineLogsResponse struct {
	Items []engineLogItem `json:"items"`
}

type engineLogItem struct {
	Source    string `json:"source"`
	Content   string `json:"content"`
	RawSize   int    `json:"rawSize"`
	Truncated bool   `json:"truncated"`
	CreatedAt string `json:"createdAt"`
}

func (h *Handler) runEngineLogs(w http.ResponseWriter, r *http.Request) {
	if h.deps.Runs == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	// Çalıştırmanın var olduğu ÖNCE doğrulanır: olmayan bir koşu için boş
	// liste dönmek, "log yok" ile "koşu yok" durumlarını karıştırırdı.
	if _, err := h.deps.Runs.Get(r.Context(), id); err != nil {
		h.respondRunError(w, r, err)
		return
	}

	logs, err := h.deps.Runs.EngineLogs(r.Context(), id)
	if err != nil {
		slog.ErrorContext(r.Context(), "motor logları okunamadı", "run_id", id, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "motor logları okunamadı")
		return
	}

	out := make([]engineLogItem, 0, len(logs))
	for _, l := range logs {
		out = append(out, engineLogItem{
			Source: l.Source, Content: l.Content, RawSize: l.RawSize,
			Truncated: l.Truncated, CreatedAt: l.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	respondJSON(w, http.StatusOK, engineLogsResponse{Items: out})
}
