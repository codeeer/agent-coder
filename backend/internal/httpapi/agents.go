package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/agentreg"
)

type createAgentRequest struct {
	Slug              string     `json:"slug"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	Prompt            string     `json:"prompt"`
	DefaultProviderID *uuid.UUID `json:"defaultProviderId"`
	DefaultModel      string     `json:"defaultModel"`
	AllowEdit         bool       `json:"allowEdit"`
	AllowBash         bool       `json:"allowBash"`
	AllowWebfetch     bool       `json:"allowWebfetch"`
}

type updateAgentRequest struct {
	Name              *string    `json:"name"`
	Description       *string    `json:"description"`
	Prompt            *string    `json:"prompt"`
	DefaultProviderID *uuid.UUID `json:"defaultProviderId"`
	ClearProvider     bool       `json:"clearProvider"`
	DefaultModel      *string    `json:"defaultModel"`
	AllowEdit         *bool      `json:"allowEdit"`
	AllowBash         *bool      `json:"allowBash"`
	AllowWebfetch     *bool      `json:"allowWebfetch"`
	// MCPServerIDs, agent'ın erişebileceği dış araç sunucuları.
	// nil ise dokunulmaz; boş dizi "hiçbiri" demektir.
	MCPServerIDs *[]uuid.UUID `json:"mcpServerIds"`
	// ScriptIDs, agent'ın çalıştırabileceği hazır betikler. Aynı kural.
	ScriptIDs *[]uuid.UUID `json:"scriptIds"`
	// ScriptFolderIDs, agent'a atanmış kampanya klasörleri (spec 022).
	ScriptFolderIDs *[]uuid.UUID `json:"scriptFolderIds"`
}

/*
 * agentResponse, agent kaydı + erişebildiği dış araçlar ve betikler.
 *
 * Ayrı uçlardan istenseydi arayüz her agent için iki istek daha atardı; liste
 * ekranında beş agent = on fazladan istek.
 */
type agentResponse struct {
	agentreg.Agent
	MCPServerIDs []uuid.UUID `json:"mcpServerIds"`
	ScriptIDs    []uuid.UUID `json:"scriptIds"`
	// ScriptFolderIDs, agent'a atanmış kampanya klasörleri (spec 022).
	ScriptFolderIDs []uuid.UUID `json:"scriptFolderIds"`
}

// withRelations, agent kaydına erişebildiği MCP sunucularını ve betikleri ekler.
//
// Okuma hatası agent'ı gizlemez: liste boş görünür ve log'a düşer. Bir yan
// tablo sorunu yüzünden agent ekranının hiç açılmaması orantısız olurdu.
func (h *Handler) withRelations(ctx contextT, a agentreg.Agent) agentResponse {
	out := agentResponse{Agent: a, MCPServerIDs: []uuid.UUID{},
		ScriptIDs: []uuid.UUID{}, ScriptFolderIDs: []uuid.UUID{}}

	if h.deps.MCPServers != nil {
		servers, err := h.deps.MCPServers.ForAgent(ctx, a.ID)
		if err != nil {
			slog.WarnContext(ctx, "agent'ın MCP sunucuları okunamadı", "agent_id", a.ID, "error", err)
		}
		for _, s := range servers {
			out.MCPServerIDs = append(out.MCPServerIDs, s.ID)
		}
	}

	if h.deps.Scripts != nil {
		/*
		 * DOĞRUDAN atamalar okunuyor, `ForAgent` DEĞİL.
		 *
		 * `ForAgent` klasörden gelenleri de içeren birleşimi döner ve o liste
		 * çalıştırma katmanı için doğru. Ama bu alan arayüzde tekil kutucukları
		 * işaretliyor ve aynen geri kaydediliyor: birleşim buradan geçseydi
		 * klasör üyeliği ilk kaydetmede kalıcı tekil atamaya dönüşür,
		 * script'i klasörden çıkarmak onu agent'tan düşürmezdi.
		 */
		ids, err := h.deps.Scripts.DirectScriptIDsForAgent(ctx, a.ID)
		if err != nil {
			slog.WarnContext(ctx, "agent'ın betikleri okunamadı", "agent_id", a.ID, "error", err)
		}
		out.ScriptIDs = append(out.ScriptIDs, ids...)

		klasorler, err := h.deps.Scripts.FoldersForAgent(ctx, a.ID)
		if err != nil {
			slog.WarnContext(ctx, "agent'ın klasörleri okunamadı", "agent_id", a.ID, "error", err)
		}
		for _, f := range klasorler {
			out.ScriptFolderIDs = append(out.ScriptFolderIDs, f.ID)
		}
	}

	return out
}

func (h *Handler) listAgents(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	page := pageOf(r)
	list, total, err := h.deps.Agents.List(r.Context(), page)
	if err != nil {
		slog.ErrorContext(r.Context(), "agent'lar listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "agent'lar okunamadı")
		return
	}
	out := make([]agentResponse, 0, len(list))
	for _, a := range list {
		out = append(out, h.withRelations(r.Context(), a))
	}
	respondJSON(w, http.StatusOK, paged(out, total, page))
}

func (h *Handler) createAgent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	var req createAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	a, err := h.deps.Agents.Create(r.Context(), agentreg.CreateInput{
		Slug: req.Slug, Name: req.Name, Description: req.Description, Prompt: req.Prompt,
		DefaultProviderID: req.DefaultProviderID, DefaultModel: req.DefaultModel,
		AllowEdit: req.AllowEdit, AllowBash: req.AllowBash, AllowWebfetch: req.AllowWebfetch,
	})
	if err != nil {
		h.respondAgentError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "agent oluşturuldu", "id", a.ID, "slug", a.Slug)
	respondJSON(w, http.StatusCreated, a)
}

func (h *Handler) updateAgent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req updateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	a, err := h.deps.Agents.Update(r.Context(), id, agentreg.UpdateInput{
		Name: req.Name, Description: req.Description, Prompt: req.Prompt,
		DefaultProviderID: req.DefaultProviderID, ClearProvider: req.ClearProvider,
		DefaultModel: req.DefaultModel,
		AllowEdit:    req.AllowEdit, AllowBash: req.AllowBash, AllowWebfetch: req.AllowWebfetch,
	})
	if err != nil {
		h.respondAgentError(w, r, err)
		return
	}

	if req.MCPServerIDs != nil && h.deps.MCPServers != nil {
		if err := h.deps.MCPServers.SetAgentServers(r.Context(), a.ID, *req.MCPServerIDs); err != nil {
			h.respondMCPError(w, r.Context(), err)
			return
		}
	}

	if req.ScriptIDs != nil && h.deps.Scripts != nil {
		if err := h.deps.Scripts.SetAgentScripts(r.Context(), a.ID, *req.ScriptIDs); err != nil {
			h.respondScriptError(w, r, err)
			return
		}
	}

	if req.ScriptFolderIDs != nil && h.deps.Scripts != nil {
		if err := h.deps.Scripts.SetAgentFolders(r.Context(), a.ID, *req.ScriptFolderIDs); err != nil {
			h.respondScriptError(w, r, err)
			return
		}
	}

	slog.InfoContext(r.Context(), "agent güncellendi", "id", a.ID, "değiştirilmiş", a.IsModified)
	respondJSON(w, http.StatusOK, h.withRelations(r.Context(), a))
}

// resetAgent, hazır bir agent'ı özgün haline döndürür.
func (h *Handler) resetAgent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	a, err := h.deps.Agents.Reset(r.Context(), id)
	if err != nil {
		h.respondAgentError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "agent sıfırlandı", "id", a.ID)
	respondJSON(w, http.StatusOK, a)
}

func (h *Handler) deleteAgent(w http.ResponseWriter, r *http.Request) {
	if h.deps.Agents == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	err := h.deps.Agents.Delete(r.Context(), id)
	if err != nil {
		h.respondAgentError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "agent silindi", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) respondAgentError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, agentreg.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "agent bulunamadı")

	case errors.Is(err, agentreg.ErrSlugTaken):
		respondError(w, http.StatusConflict, "slug_taken", "bu kısa ad zaten kullanılıyor")

	case errors.Is(err, agentreg.ErrBuiltinDelete):
		respondError(w, http.StatusConflict, "builtin_delete",
			"hazır agent silinemez — özgün haline sıfırlayabilirsiniz")

	case errors.Is(err, agentreg.ErrNotBuiltin):
		respondError(w, http.StatusConflict, "not_builtin",
			"yalnızca hazır agent'lar sıfırlanabilir")

	case errors.Is(err, agentreg.ErrInUse):
		respondError(w, http.StatusConflict, "agent_in_use",
			"bu agent'ın çalıştırma geçmişi var — geçmiş kaydı için silinemez")

	case errors.Is(err, agentreg.ErrEmptyPrompt):
		respondError(w, http.StatusBadRequest, "empty_prompt", "agent talimatı boş olamaz")

	case errors.Is(err, agentreg.ErrPromptTooLarge):
		respondError(w, http.StatusBadRequest, "prompt_too_large", err.Error())

	case errors.Is(err, agentreg.ErrInvalidSlug):
		respondError(w, http.StatusBadRequest, "invalid_slug", err.Error())

	default:
		slog.ErrorContext(r.Context(), "agent işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "işlem tamamlanamadı")
	}
}
