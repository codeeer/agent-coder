package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// mcpAccessResponse, dışarıya verilecek MCP adresi.
type mcpAccessResponse struct {
	// URL, MCP istemcisine yapıştırılacak tam adres.
	URL string `json:"url"`
}

// getMCPAccess, MCP erişim adresini döner (yoksa üretir).
func (h *Handler) getMCPAccess(w http.ResponseWriter, r *http.Request) {
	if h.deps.MCPAccess == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	token, err := h.deps.MCPAccess.Token(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "MCP erişim anahtarı okunamadı", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "adres okunamadı")
		return
	}
	respondJSON(w, http.StatusOK, mcpAccessResponse{URL: mcpURL(r, token)})
}

// rotateMCPAccess, adresi yeniler. Eski adres anında geçersiz olur.
func (h *Handler) rotateMCPAccess(w http.ResponseWriter, r *http.Request) {
	if h.deps.MCPAccess == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	token, err := h.deps.MCPAccess.Rotate(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "MCP erişim anahtarı yenilenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "adres yenilenemedi")
		return
	}
	slog.InfoContext(r.Context(), "MCP erişim adresi yenilendi")
	respondJSON(w, http.StatusOK, mcpAccessResponse{URL: mcpURL(r, token)})
}

/*
 * mcpServe, dışarıdan gelen MCP isteklerini karşılar.
 *
 * Kimlik doğrulama yok; ADRESİN KENDİSİ anahtardır (webhook uçlarındaki
 * desenin aynısı, spec 007 S3). Bu yüzden uç `/api` altında değil: dışarıya
 * açılan noktalar bir arada dursun ki ileride farklı bir güvenlik politikası
 * uygulanabilsin.
 */
func (h *Handler) mcpServe(w http.ResponseWriter, r *http.Request) {
	if h.deps.MCPAccess == nil || h.deps.MCPServer == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	if !h.deps.MCPAccess.Valid(r.Context(), chi.URLParam(r, "token")) {
		// Yanlış ve var olmayan adres AYNI cevabı alır.
		respondError(w, http.StatusNotFound, "not_found", "adres bulunamadı")
		return
	}

	h.deps.MCPServer.Handler().ServeHTTP(w, r)
}

// mcpURL, istekten tam adresi kurar.
//
// Şema ve host isteğin kendisinden okunur: kurulum localhost'ta da, ters vekil
// arkasında da olabilir ve kullanıcıya YAPIŞTIRILABİLİR bir adres verilmeli.
func mcpURL(r *http.Request, token string) string {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = fwd
	}
	return scheme + "://" + host + "/mcp/" + token
}
