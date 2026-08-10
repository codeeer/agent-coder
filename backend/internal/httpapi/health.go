package httpapi

import (
	"context"
	"net/http"
	"time"
)

// Version derleme sırasında ldflags ile doldurulur:
//
//	-ldflags "-X github.com/agent-coder/backend/internal/httpapi.Version=$(git rev-parse --short HEAD)"
var Version = "dev"

// HealthResponse /health yanıtı.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Env     string `json:"env"`
	Uptime  string `json:"uptime"`
}

// health servisin ayakta olduğunu bildirir.
//
// Bağımlılıkları KONTROL ETMEZ — sadece sürecin yanıt verdiğini söyler.
// Bağımlılık kontrolü için /readyz kullanılır. İkisinin ayrı olması,
// veritabanı düşse bile container'ın gereksiz yere yeniden başlatılmamasını sağlar.
func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		Version: Version,
		Env:     h.deps.Config.Env,
		Uptime:  time.Since(h.startedAt).Round(time.Second).String(),
	})
}

// ReadyResponse /readyz yanıtı.
type ReadyResponse struct {
	Ready    bool   `json:"ready"`
	Database string `json:"database"` // "ok" | "unavailable" | "not_configured"
}

// ready, servisin istek karşılamaya hazır olup olmadığını bildirir.
//
// Veritabanına erişilemiyorsa 503 döner; yük dengeleyici veya compose bu uca
// bakarak trafiği yönlendirmeyi bekletebilir.
func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if h.deps.DB == nil {
		respondJSON(w, http.StatusServiceUnavailable, ReadyResponse{
			Ready: false, Database: "not_configured",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := h.deps.DB.Ping(ctx); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, ReadyResponse{
			Ready: false, Database: "unavailable",
		})
		return
	}

	respondJSON(w, http.StatusOK, ReadyResponse{Ready: true, Database: "ok"})
}
