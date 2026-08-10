package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/agent-coder/backend/internal/mcp"
)

type createMCPServerRequest struct {
	Name      string        `json:"name"`
	Transport mcp.Transport `json:"transport"`
	URL       string        `json:"url"`
	Secret    string        `json:"secret"`
}

type updateMCPServerRequest struct {
	Name      *string        `json:"name"`
	Transport *mcp.Transport `json:"transport"`
	URL       *string        `json:"url"`
	Secret    *string        `json:"secret"`
}

func (h *Handler) listMCPServers(w http.ResponseWriter, r *http.Request) {
	if h.deps.MCPServers == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	servers, err := h.deps.MCPServers.List(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "MCP sunucuları listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "MCP sunucuları okunamadı")
		return
	}
	respondJSON(w, http.StatusOK, servers)
}

/*
 * createMCPServer, sunucuya BAĞLANIR, araçlarını okur ve öyle kaydeder.
 *
 * Doğrulanamayan bir sunucu kaydedilmez — git erişimlerindeki "doğrulanamıyor
 * ama kaydet" yolu burada YOK, çünkü burada doğrulama her zaman mümkün: elimizde
 * bir adres ve protokol var. Kaydedip sonra çalışma anında keşfetmek, agent'ın
 * neden araçsız kaldığını gizlerdi.
 *
 * Araç listesi de bu sırada saklanır: kullanıcı bir agent'a erişim verirken
 * NEYE erişim verdiğini görmeli.
 */
func (h *Handler) createMCPServer(w http.ResponseWriter, r *http.Request) {
	if h.deps.MCPServers == nil || h.deps.MCPClient == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	var req createMCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	candidate := mcp.Server{Name: req.Name, Transport: req.Transport, URL: req.URL}
	if err := candidate.Validate(); err != nil {
		h.respondMCPError(w, ctx, err)
		return
	}

	tools, err := h.deps.MCPClient.ListTools(ctx, candidate, req.Secret)
	if err != nil {
		h.respondMCPError(w, ctx, err)
		return
	}

	server, err := h.deps.MCPServers.Create(ctx, mcp.CreateInput{
		Name: candidate.Name, Transport: candidate.Transport, URL: candidate.URL,
		Secret: req.Secret, Tools: tools,
	})
	if err != nil {
		h.respondMCPError(w, ctx, err)
		return
	}
	respondJSON(w, http.StatusCreated, server)
}

// updateMCPServer, değişikliği yine doğrulayarak kaydeder.
func (h *Handler) updateMCPServer(w http.ResponseWriter, r *http.Request) {
	if h.deps.MCPServers == nil || h.deps.MCPClient == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req updateMCPServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	current, err := h.deps.MCPServers.Get(ctx, id)
	if err != nil {
		h.respondMCPError(w, ctx, err)
		return
	}

	// Doğrulama, GÜNCELLENMİŞ haliyle yapılır: adres değiştiyse yeni adrese
	// bağlanılabildiği görülmeli.
	candidate := current
	if req.Name != nil {
		candidate.Name = *req.Name
	}
	if req.Transport != nil {
		candidate.Transport = *req.Transport
	}
	if req.URL != nil {
		candidate.URL = *req.URL
	}
	if err := candidate.Validate(); err != nil {
		h.respondMCPError(w, ctx, err)
		return
	}

	// Anahtar gönderilmediyse mevcut anahtarla doğrulanır; kullanıcı yalnızca
	// adı değiştirmek için anahtarı yeniden yazmak zorunda kalmasın.
	secret := ""
	if req.Secret != nil && *req.Secret != "" {
		secret = *req.Secret
	} else if current.HasSecret {
		if secret, err = h.deps.MCPServers.Reveal(ctx, id); err != nil {
			h.respondMCPError(w, ctx, err)
			return
		}
	}

	tools, err := h.deps.MCPClient.ListTools(ctx, candidate, secret)
	if err != nil {
		h.respondMCPError(w, ctx, err)
		return
	}

	server, err := h.deps.MCPServers.Update(ctx, id, mcp.UpdateInput{
		Name: req.Name, Transport: req.Transport, URL: req.URL,
		Secret: req.Secret, Tools: tools,
	})
	if err != nil {
		h.respondMCPError(w, ctx, err)
		return
	}
	respondJSON(w, http.StatusOK, server)
}

func (h *Handler) deleteMCPServer(w http.ResponseWriter, r *http.Request) {
	if h.deps.MCPServers == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.deps.MCPServers.Delete(r.Context(), id); err != nil {
		h.respondMCPError(w, r.Context(), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// respondMCPError, alan hatalarını okunur yanıtlara çevirir.
//
// Bağlantı hatası 422: istek biçimsel olarak doğru ama karşı taraf cevap
// vermiyor. 500 dönmek, sorunun BİZDE olduğunu söylerdi.
func (h *Handler) respondMCPError(w http.ResponseWriter, ctx contextT, err error) {
	switch {
	case errors.Is(err, mcp.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "MCP sunucusu bulunamadı")
	case errors.Is(err, mcp.ErrDuplicateName):
		respondError(w, http.StatusConflict, "duplicate_name", "bu adda bir sunucu zaten var")
	case errors.Is(err, mcp.ErrUnreachable):
		respondError(w, http.StatusUnprocessableEntity, "unreachable",
			"Sunucuya bağlanılamadı. Adresi ve erişim anahtarını kontrol edin.")
	case errors.Is(err, mcp.ErrMissingName), errors.Is(err, mcp.ErrInvalidName),
		errors.Is(err, mcp.ErrMissingURL), errors.Is(err, mcp.ErrInvalidURL),
		errors.Is(err, mcp.ErrInvalidTransport):
		respondError(w, http.StatusBadRequest, "invalid_server", err.Error())
	default:
		slog.ErrorContext(ctx, "MCP işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "işlem tamamlanamadı")
	}
}
