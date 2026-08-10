package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/catalog"
	"github.com/agent-coder/backend/internal/paging"
)

// modelListResponse, /api/models yanıtı.
type modelListResponse struct {
	Items []catalog.Model `json:"items"`
	Total int             `json:"total"`

	// Providers, her sağlayıcının senkron durumu. Arayüz buradan hangi
	// sağlayıcının güncellenemediğini gösterir (spec 002 H2).
	Providers []catalog.ProviderSync `json:"providers"`

	// Configured, en az bir LLM sağlayıcı tanımlı mı.
	Configured bool `json:"configured"`
}

// listModels, filtrelenmiş model listesini ve sağlayıcı durumlarını döner.
func (h *Handler) listModels(w http.ResponseWriter, r *http.Request) {
	if h.deps.Catalog == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()
	q := r.URL.Query()

	filter := catalog.ListFilter{
		Query:    q.Get("q"),
		Tools:    catalog.ToolsFilter(q.Get("tools")),
		FreeOnly: q.Get("free") == "1",
		Sort:     catalog.SortField(q.Get("sort")),
		Desc:     q.Get("order") == "desc",
		Limit:    atoiOr(q.Get("limit"), 50),
		Offset:   atoiOr(q.Get("offset"), 0),
	}

	if raw := q.Get("provider"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_provider", "geçersiz sağlayıcı kimliği")
			return
		}
		filter.ProviderID = &id
	}

	items, total, err := h.deps.Catalog.List(ctx, filter)
	if err != nil {
		slog.ErrorContext(ctx, "modeller listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "modeller okunamadı")
		return
	}

	syncs, err := h.deps.Catalog.SyncStatus(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "senkron durumu okunamadı", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "senkron durumu okunamadı")
		return
	}

	respondJSON(w, http.StatusOK, modelListResponse{
		Items:      items,
		Total:      total,
		Providers:  syncs,
		Configured: len(syncs) > 0,
	})
}

// refreshModels, tüm sağlayıcıların katalogunu yeniden indirir.
//
// KISMİ BAŞARI döner: bir sağlayıcının hatası diğerlerini engellemez, yanıt
// her sağlayıcının sonucunu ayrı ayrı taşır (spec 002 H2).
func (h *Handler) refreshModels(w http.ResponseWriter, r *http.Request) {
	if h.deps.Syncer == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	results, err := h.deps.Syncer.SyncAll(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "katalog yenilenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "katalog yenilenemedi")
		return
	}

	if len(results) == 0 {
		respondError(w, http.StatusPreconditionRequired, "no_provider",
			"önce ayarlardan bir LLM sağlayıcı ekleyin")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"results": results})
}

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

/*
 * Sayfalı liste yanıtı.
 *
 * Tüm liste uçları AYNI zarfı döner: `{items, total, limit, offset}`. Kimi uç
 * çıplak dizi, kimi zarf dönseydi istemci her uç için ayrı bir okuma yolu
 * yazmak zorunda kalırdı — ve sayfalama denetimi hangi listede olup olmadığını
 * bilemezdi.
 *
 * `total` süzgeçten GEÇEN kayıt sayısıdır, tablodaki toplam değil: kullanıcı
 * "1–25 / 138" ifadesinde 138'i aradığı şeyin sayısı olarak okur.
 */
type pageResponse[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// pageOf, istekten sayfa penceresini okur.
func pageOf(r *http.Request) paging.Page {
	return paging.Clamp(
		atoiOr(r.URL.Query().Get("limit"), paging.Default),
		atoiOr(r.URL.Query().Get("offset"), 0),
	)
}

// paged, zarfı kurar.
func paged[T any](items []T, total int, p paging.Page) pageResponse[T] {
	return pageResponse[T]{Items: items, Total: total, Limit: p.Limit, Offset: p.Offset}
}
