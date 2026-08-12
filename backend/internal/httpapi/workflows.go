package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/workflow"
)

type workflowRequest struct {
	ProjectID   uuid.UUID `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    *bool     `json:"isActive"`
}

// versionRequest, kaydedilecek akış tanımı.
type versionRequest struct {
	Graph workflow.Graph `json:"graph"`
}

// validationResponse, geçersiz grafın kusurlarını DÜĞÜM BAZINDA döner.
//
// Tek bir "geçersiz" mesajı kullanıcıyı hatayı aramaya bırakırdı; arayüz
// hangi adımda ne yanlış olduğunu gösterebilmeli (spec 007 T43).
type validationResponse struct {
	Error validationDetail `json:"error"`
}

type validationDetail struct {
	Code     string             `json:"code"`
	Message  string             `json:"message"`
	Problems []workflow.Problem `json:"problems"`
}

func (h *Handler) listWorkflows(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	var projectID *uuid.UUID
	if raw := r.URL.Query().Get("project"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_project", "geçersiz proje kimliği")
			return
		}
		projectID = &id
	}

	page := pageOf(r)
	items, total, err := h.deps.Workflows.List(r.Context(), projectID, page)
	if err != nil {
		slog.ErrorContext(r.Context(), "akışlar listelenemedi", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "akışlar okunamadı")
		return
	}

	out, err := h.withRunCounts(r.Context(), items)
	if err != nil {
		slog.ErrorContext(r.Context(), "çalışma sayıları okunamadı", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "akışlar okunamadı")
		return
	}
	respondJSON(w, http.StatusOK, paged(out, total, page))
}

// workflowResponse, akışa silme onayı için çalışma sayısını ekler.
//
// Kullanıcı bir akışı silerken geçmişinin de gideceğini SİLMEDEN ÖNCE
// görmeli; sayı olmadan onay penceresi ne kaybedildiğini söyleyemez.
type workflowResponse struct {
	workflow.Workflow
	RunCount int `json:"runCount"`
}

func (h *Handler) withRunCounts(ctx contextT, items []workflow.Workflow) (
	[]workflowResponse, error,
) {
	ids := make([]uuid.UUID, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	counts, err := h.deps.Workflows.RunCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]workflowResponse, 0, len(items))
	for _, it := range items {
		out = append(out, workflowResponse{Workflow: it, RunCount: counts[it.ID]})
	}
	return out, nil
}

func (h *Handler) createWorkflow(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	var req workflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		respondError(w, http.StatusBadRequest, "empty_name", "akış adı boş olamaz")
		return
	}
	if req.ProjectID == uuid.Nil {
		respondError(w, http.StatusBadRequest, "no_project", "akış bir projeye bağlı olmalı")
		return
	}

	wf, err := h.deps.Workflows.Create(r.Context(), workflow.CreateInput{
		ProjectID: req.ProjectID, Name: strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
	})
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, wf)
}

func (h *Handler) getWorkflow(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	wf, err := h.deps.Workflows.Get(r.Context(), id)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}

	// Etkin sürümün grafı da döner: düzenleme ekranı ayrı bir istek atmasın.
	// Çalışma sayısı da: editördeki silme onayı da ne kaybedileceğini yazmalı.
	resp := struct {
		workflowResponse
		Graph *workflow.Graph `json:"graph"`
	}{workflowResponse: workflowResponse{Workflow: wf}}

	counts, err := h.deps.Workflows.RunCounts(r.Context(), []uuid.UUID{id})
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	resp.RunCount = counts[id]

	if wf.ActiveVersionID != nil {
		v, err := h.deps.Workflows.Version(r.Context(), *wf.ActiveVersionID)
		if err == nil {
			resp.Graph = &v.Graph
		}
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *Handler) updateWorkflow(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req workflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		respondError(w, http.StatusBadRequest, "empty_name", "akış adı boş olamaz")
		return
	}

	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}

	wf, err := h.deps.Workflows.Update(r.Context(), id, workflow.UpdateInput{
		Name: strings.TrimSpace(req.Name), Description: strings.TrimSpace(req.Description),
		IsActive: active,
	})
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, wf)
}

func (h *Handler) deleteWorkflow(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.deps.Workflows.Delete(r.Context(), id); err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "akış silindi", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// saveVersion, yeni bir akış tanımı kaydeder.
//
// Geçersiz graf 400 ile ve KUSUR LİSTESİYLE döner — kullanıcı akışını tek turda
// düzeltebilsin diye.
func (h *Handler) saveVersion(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req versionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	// Kayıt defteri veritabanına BAKAMAZ (bağımlılıksız olmak zorunda), bu yüzden
	// "seçilen sunucu ve araç gerçekten var mı" kontrolü burada yapılır.
	// Çalışma anına bırakılsaydı akışın yarısı koştuktan sonra durması demekti.
	if problems := h.checkMCPNodes(r.Context(), req.Graph); len(problems) > 0 {
		respondJSON(w, http.StatusBadRequest, validationResponse{
			Error: validationDetail{
				Code:     "invalid_graph",
				Message:  "akış tanımı geçersiz",
				Problems: problems,
			},
		})
		return
	}

	version, err := h.deps.Workflows.SaveVersion(r.Context(), id, req.Graph)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "akış sürümü kaydedildi",
		"workflow_id", id, "version", version.Version)
	respondJSON(w, http.StatusCreated, version)
}

/*
 * checkMCPNodes, MCP düğümlerinin seçtiği sunucu ve aracın var olduğunu sınar.
 *
 * Araç adı, sunucunun SON DOĞRULAMADA bildirdiği listeye göre kontrol edilir.
 * Sunucuya her kaydetmede yeniden bağlanmak, tuvalde kaydet düğmesine basan
 * kullanıcıyı ağ gecikmesi kadar bekletirdi.
 *
 * Bunun sonucu: sunucu araç listesini değiştirmişse kayıt geçer ama çalışma
 * anında hata alınır. Kabul edilebilir — kullanıcı sunucuyu yeniden
 * doğruladığında liste tazelenir.
 */
func (h *Handler) checkMCPNodes(ctx contextT, graph workflow.Graph) []workflow.Problem {
	if h.deps.MCPServers == nil {
		return nil
	}

	var problems []workflow.Problem
	for _, n := range graph.Nodes {
		if n.Kind != workflow.KindMCPCall {
			continue
		}

		id, err := uuid.Parse(strings.TrimSpace(n.Config.MCPServerID))
		if err != nil {
			// Boş seçim zaten kayıt defterinde yakalanıyor; buraya yalnızca
			// bozuk bir kimlik düşer.
			continue
		}

		server, err := h.deps.MCPServers.Get(ctx, id)
		if err != nil {
			problems = append(problems, workflow.Problem{
				NodeID: n.ID, Message: "seçilen MCP sunucusu bulunamadı — silinmiş olabilir",
			})
			continue
		}

		tool := strings.TrimSpace(n.Config.ToolName)
		if tool != "" && !slices.Contains(server.Tools, tool) {
			problems = append(problems, workflow.Problem{
				NodeID:  n.ID,
				Message: fmt.Sprintf("%q sunucusunda %q adında bir araç yok", server.Name, tool),
			})
		}
	}
	return problems
}

// getVersion, tek bir sürümün grafını döner.
//
// Geçmiş bir çalışmayı doğru çizmek için gerekli: akış sonradan değişmiş olsa
// bile o çalışmanın KULLANDIĞI tanım gösterilmeli.
func (h *Handler) getVersion(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	v, err := h.deps.Workflows.Version(r.Context(), id)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, v)
}

// rotateHook, tetikleme adresini yeniler.
func (h *Handler) rotateHook(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	token, err := h.deps.Workflows.RotateHook(r.Context(), id)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "tetikleme adresi yenilendi", "workflow_id", id)
	respondJSON(w, http.StatusOK, map[string]string{"hookToken": token})
}

// respondWorkflowError, akış hatalarını uygun HTTP koduna çevirir.
func (h *Handler) respondWorkflowError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *workflow.ValidationError
	switch {
	case errors.As(err, &ve):
		respondJSON(w, http.StatusBadRequest, validationResponse{
			Error: validationDetail{
				Code:     "invalid_graph",
				Message:  "akış tanımı geçersiz",
				Problems: ve.Problems,
			},
		})
	case errors.Is(err, workflow.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "akış bulunamadı")
	case errors.Is(err, workflow.ErrNoVersion):
		respondError(w, http.StatusConflict, "no_version",
			"akışın kayıtlı bir tanımı yok — önce adımları kaydedin")
	case errors.Is(err, workflow.ErrNotRunning):
		respondError(w, http.StatusConflict, "not_running", "akış zaten bitmiş")
	case errors.Is(err, workflow.ErrRunning):
		respondError(w, http.StatusConflict, "workflow_running",
			"akışın süren bir çalışması var — önce onu durdurun")
	default:
		slog.ErrorContext(r.Context(), "akış işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "işlem tamamlanamadı")
	}
}
