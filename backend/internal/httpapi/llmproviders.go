package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/catalog"
	"github.com/agent-coder/backend/internal/llm"
)

// llmProviderResponse, sağlayıcı ve senkron durumu birlikte döner.
// Arayüz her sağlayıcının kaç modeli olduğunu ve son hatayı tek istekte görür.
type llmProviderResponse struct {
	llm.Provider
	Sync *catalog.ProviderSync `json:"sync"`
}

type createLLMProviderRequest struct {
	Type      llm.Type `json:"type"`
	Name      string   `json:"name"`
	BaseURL   string   `json:"baseUrl"`
	Secret    string   `json:"secret"`
	IsDefault bool     `json:"isDefault"`
}

type updateLLMProviderRequest struct {
	Name      *string `json:"name"`
	BaseURL   *string `json:"baseUrl"`
	Secret    *string `json:"secret"` // boş bırakılırsa mevcut anahtar korunur
	IsDefault *bool   `json:"isDefault"`
}

// listLLMProviders, tanımlı sağlayıcıları senkron durumlarıyla döner.
func (h *Handler) listLLMProviders(w http.ResponseWriter, r *http.Request) {
	if h.deps.LLMProviders == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	providers, err := h.deps.LLMProviders.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "sağlayıcılar listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "sağlayıcılar okunamadı")
		return
	}

	syncs, err := h.deps.Catalog.SyncStatus(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "senkron durumu okunamadı", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "senkron durumu okunamadı")
		return
	}
	byID := make(map[uuid.UUID]catalog.ProviderSync, len(syncs))
	for _, s := range syncs {
		byID[s.ProviderID] = s
	}

	out := make([]llmProviderResponse, 0, len(providers))
	for _, p := range providers {
		item := llmProviderResponse{Provider: p}
		if s, ok := byID[p.ID]; ok {
			item.Sync = &s
		}
		out = append(out, item)
	}
	respondJSON(w, http.StatusOK, out)
}

// createLLMProvider, sağlayıcıyı doğrular, kaydeder ve katalogunu indirir.
//
// Doğrulama BAŞARISIZSA kayıt yapılmaz.
func (h *Handler) createLLMProvider(w http.ResponseWriter, r *http.Request) {
	if h.deps.LLMProviders == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	var req createLLMProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}
	if req.Secret == "" {
		respondError(w, http.StatusBadRequest, "missing_secret", "anahtar boş olamaz")
		return
	}

	// Adres ve tür, doğrulama isteği atılmadan önce biçimsel olarak kontrol edilir.
	baseURL, err := llm.NormalizeBaseURL(req.Type, req.BaseURL)
	if err != nil {
		h.respondLLMError(w, r, err)
		return
	}
	if !req.Type.Valid() {
		respondError(w, http.StatusBadRequest, "invalid_type", "geçersiz sağlayıcı türü")
		return
	}

	client, err := llm.NewClient(llm.Provider{Type: req.Type, BaseURL: baseURL}, h.deps.HTTPTransport)
	if err != nil {
		h.respondLLMError(w, r, err)
		return
	}
	if err := client.Verify(ctx, req.Secret); err != nil {
		h.respondLLMError(w, r, err)
		return
	}

	p, err := h.deps.LLMProviders.Create(ctx, llm.CreateInput{
		Type: req.Type, Name: req.Name, BaseURL: baseURL,
		Secret: req.Secret, IsDefault: req.IsDefault,
	})
	if err != nil {
		h.respondLLMError(w, r, err)
		return
	}

	slog.InfoContext(ctx, "llm sağlayıcı eklendi", "id", p.ID, "type", p.Type, "slug", p.Slug)

	// Katalog arka planda indirilir; kullanıcı kaydetme yanıtını beklemez.
	h.syncProviderInBackground(p)

	respondJSON(w, http.StatusCreated, llmProviderResponse{Provider: p})
}

// updateLLMProvider, mevcut sağlayıcıyı günceller.
func (h *Handler) updateLLMProvider(w http.ResponseWriter, r *http.Request) {
	if h.deps.LLMProviders == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req updateLLMProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	current, err := h.deps.LLMProviders.Get(ctx, id)
	if err != nil {
		h.respondLLMError(w, r, err)
		return
	}

	// Anahtar veya adres değiştiyse yeni kombinasyon doğrulanır; aksi halde
	// çalışmayan bir yapılandırma kaydedilmiş olurdu.
	if req.Secret != nil && *req.Secret != "" || req.BaseURL != nil {
		baseURL := current.BaseURL
		if req.BaseURL != nil {
			if baseURL, err = llm.NormalizeBaseURL(current.Type, *req.BaseURL); err != nil {
				h.respondLLMError(w, r, err)
				return
			}
		}

		secret := ""
		if req.Secret != nil && *req.Secret != "" {
			secret = *req.Secret
		} else {
			if secret, err = h.deps.LLMProviders.Reveal(ctx, id); err != nil {
				h.respondLLMError(w, r, err)
				return
			}
		}

		client, err := llm.NewClient(llm.Provider{Type: current.Type, BaseURL: baseURL}, h.deps.HTTPTransport)
		if err != nil {
			h.respondLLMError(w, r, err)
			return
		}
		if err := client.Verify(ctx, secret); err != nil {
			h.respondLLMError(w, r, err)
			return
		}
	}

	p, err := h.deps.LLMProviders.Update(ctx, id, llm.UpdateInput{
		Name: req.Name, BaseURL: req.BaseURL, Secret: req.Secret, IsDefault: req.IsDefault,
	})
	if err != nil {
		h.respondLLMError(w, r, err)
		return
	}

	slog.InfoContext(ctx, "llm sağlayıcı güncellendi", "id", p.ID)
	h.syncProviderInBackground(p)

	respondJSON(w, http.StatusOK, llmProviderResponse{Provider: p})
}

// deleteLLMProvider, sağlayıcıyı ve modellerini siler.
func (h *Handler) deleteLLMProvider(w http.ResponseWriter, r *http.Request) {
	if h.deps.LLMProviders == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	err := h.deps.LLMProviders.Delete(r.Context(), id)
	switch {
	case err == nil:
		slog.InfoContext(r.Context(), "llm sağlayıcı silindi", "id", id)
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, llm.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "sağlayıcı bulunamadı")
	default:
		slog.ErrorContext(r.Context(), "sağlayıcı silinemedi", "id", id, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "sağlayıcı silinemedi")
	}
}

// syncLLMProvider, tek bir sağlayıcının katalogunu hemen tazeler.
func (h *Handler) syncLLMProvider(w http.ResponseWriter, r *http.Request) {
	if h.deps.Syncer == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	p, err := h.deps.LLMProviders.Get(ctx, id)
	if err != nil {
		h.respondLLMError(w, r, err)
		return
	}

	count, err := h.deps.Syncer.SyncOne(ctx, p)
	if err != nil {
		slog.WarnContext(ctx, "sağlayıcı senkronu başarısız", "id", id, "error", err)
		respondError(w, http.StatusServiceUnavailable, "sync_failed", catalog.UserFacingError(err))
		return
	}

	respondJSON(w, http.StatusOK, catalog.Result{
		ProviderID: p.ID, ProviderName: p.Name, OK: true, Count: count,
	})
}

// syncProviderInBackground, istek yanıtını bekletmeden katalogu tazeler.
func (h *Handler) syncProviderInBackground(p llm.Provider) {
	if h.deps.Syncer == nil {
		return
	}
	h.bg.Go(func() {
		if _, err := h.deps.Syncer.SyncOne(h.bgCtx, p); err != nil {
			slog.WarnContext(h.bgCtx, "arka plan senkronu başarısız",
				"sağlayıcı", p.Name, "durum", catalog.UserFacingError(err))
		}
	})
}

// respondLLMError, sağlayıcı hatalarını uygun HTTP koduna çevirir.
//
// "Anahtar yanlış", "adres yanlış" ve "servise ulaşılamadı" ayrımı korunur;
// kullanıcı üçünde farklı şey yapar.
func (h *Handler) respondLLMError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, llm.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "sağlayıcı bulunamadı")

	case errors.Is(err, llm.ErrUnauthorized):
		respondError(w, http.StatusUnprocessableEntity, "invalid_credential",
			"anahtar doğrulanamadı — değeri kontrol edin")

	case errors.Is(err, llm.ErrInvalidBaseURL):
		respondError(w, http.StatusBadRequest, "invalid_base_url", err.Error())

	case errors.Is(err, llm.ErrInvalidType):
		respondError(w, http.StatusBadRequest, "invalid_type", "geçersiz sağlayıcı türü")

	case errors.Is(err, llm.ErrEmptyName):
		respondError(w, http.StatusBadRequest, "invalid_name", err.Error())

	case errors.Is(err, llm.ErrUnreachable):
		respondError(w, http.StatusServiceUnavailable, "service_unreachable",
			"adrese ulaşılamadı — adresi ve bağlantınızı kontrol edip tekrar deneyin")

	case errors.Is(err, llm.ErrBadCatalog):
		// Sağlayıcı yanıt veriyor ama beklenen biçimde değil.
		respondError(w, http.StatusUnprocessableEntity, "bad_catalog",
			"servis beklenen biçimde model listesi vermiyor")

	default:
		slog.ErrorContext(r.Context(), "sağlayıcı işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "işlem tamamlanamadı")
	}
}

// parseUUIDParam, yol parametresini UUID olarak okur.
func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_id", "geçersiz kimlik")
		return uuid.Nil, false
	}
	return id, true
}
