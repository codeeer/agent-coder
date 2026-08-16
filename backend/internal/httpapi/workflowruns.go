package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/events"
	"github.com/agent-coder/backend/internal/workflow"
)

type startWorkflowRequest struct {
	Input string `json:"input"`
	// ProjectID isteğe bağlı: boşsa akışın varsayılan projesi kullanılır.
	// Aynı akış farklı projelerde koşabilsin diye var (spec 007, 2026-08-12).
	ProjectID string `json:"projectId"`
}

type startWorkflowResponse struct {
	workflow.Run
	// ActiveOthers, aynı akışın süren diğer çalışmaları (spec 007 S4).
	// Engellemek için değil UYARMAK için: kullanıcı bilerek yapsın.
	ActiveOthers int `json:"activeOthers"`
}

func (h *Handler) startWorkflowRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.WorkflowExec == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req startWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	// Proje ayrıştırması BURADA yapılır, veritabanına bırakılmaz: geçersiz bir
	// kimlik FK ihlaline dönüşür ve kullanıcı "işlem tamamlanamadı" görürdü.
	var projectID uuid.UUID
	if req.ProjectID != "" {
		parsed, err := uuid.Parse(req.ProjectID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_project", "proje kimliği geçersiz")
			return
		}
		if h.deps.Projects != nil {
			if _, err := h.deps.Projects.Get(r.Context(), parsed); err != nil {
				respondError(w, http.StatusNotFound, "project_not_found", "proje bulunamadı")
				return
			}
		}
		projectID = parsed
	}

	run, others, err := h.launchWorkflow(r.Context(), id, workflow.TriggerManual,
		nil, req.Input, projectID)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	respondJSON(w, http.StatusCreated, startWorkflowResponse{Run: run, ActiveOthers: others})
}

// launchWorkflow, akışı başlatır. Gövde `workflow.Launcher` içinde — elle,
// dışarıdan ve Jira tetiklemesi aynı kapıdan geçsin diye.
//
// projectID boş (uuid.Nil) geçilebilir: akışın varsayılanı kullanılır.
// Tetikleyiciler hep böyle çağırır.
func (h *Handler) launchWorkflow(ctx contextT, workflowID uuid.UUID,
	trigger workflow.TriggerKind, payload map[string]string, input string,
	projectID uuid.UUID,
) (workflow.Run, int, error) {
	others, err := h.deps.Launcher.ActiveRuns(ctx, workflowID)
	if err != nil {
		return workflow.Run{}, 0, err
	}

	run, err := h.deps.Launcher.Launch(ctx, workflow.LaunchInput{
		WorkflowID: workflowID, Trigger: trigger, Payload: payload, Input: input,
		ProjectID: projectID,
	})
	if err != nil {
		return workflow.Run{}, 0, err
	}
	return run, others, nil
}

func (h *Handler) listWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	page := pageOf(r)
	filter := workflow.ListRunsFilter{Limit: page.Limit, Offset: page.Offset}
	if raw := r.URL.Query().Get("workflow"); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_workflow", "geçersiz akış kimliği")
			return
		}
		filter.WorkflowID = &id
	}

	items, total, err := h.deps.Workflows.ListRuns(r.Context(), filter)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, paged(items, total, page))
}

func (h *Handler) getWorkflowRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	run, err := h.deps.Workflows.GetRun(r.Context(), id)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, run)
}

func (h *Handler) cancelWorkflowRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.WorkflowExec == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.deps.WorkflowExec.Cancel(id); err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "akış durduruldu", "workflow_run_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// workflowRunEvents, akış ilerlemesini SSE ile yayınlar.
//
// Çalıştırma akışından farklı olarak GEÇMİŞ GÖNDERİLMEZ: akışın o ana kadarki
// durumu zaten adım kayıtlarında duruyor ve istemci onu `GET /workflow-runs/{id}`
// ile alıyor. Olayları ayrı bir tabloda ikinci kez saklamak, aynı bilgiyi iki
// yerde tutmak olurdu.
func (h *Handler) workflowRunEvents(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	run, err := h.deps.Workflows.GetRun(ctx, id)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "no_streaming",
			"bu bağlantı canlı akışı desteklemiyor")
		return
	}

	stream, unsubscribe := h.deps.Bus.Subscribe(id)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Akış zaten bitmişse canlı bağlantıya gerek yok.
	if run.Status.Terminal() {
		writeSSE(w, events.Event{
			Level: "info", Terminal: true, Status: string(run.Status),
			TS: time.Now().UTC().Format(time.RFC3339),
		})
		flusher.Flush()
		return
	}

	ticker := time.NewTicker(sseHeartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case e, open := <-stream:
			if !open {
				return
			}
			writeSSE(w, e)
			flusher.Flush()
			if e.Terminal {
				return
			}

		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// triggerHook, dışarıdan gelen çağrıyla akışı başlatır.
//
// Kimlik doğrulama v1'de yok; ADRESİN KENDİSİ anahtardır (spec 007 S3). Bu
// yüzden uç `/api` altında değil: dışarıya açılan tek nokta ayrı dursun ki
// ileride farklı bir güvenlik politikası uygulanabilsin.
func (h *Handler) triggerHook(w http.ResponseWriter, r *http.Request) {
	if h.deps.WorkflowExec == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	token := chi.URLParam(r, "token")
	wf, err := h.deps.Workflows.ByHookToken(r.Context(), token)
	if err != nil {
		// Var olmayan ve yanlış adres AYNI cevabı alır: hangi akışların var
		// olduğu bilgisi dışarı sızmasın.
		respondError(w, http.StatusNotFound, "not_found", "adres bulunamadı")
		return
	}

	if !wf.IsActive {
		respondError(w, http.StatusConflict, "inactive", "akış pasif durumda")
		return
	}

	// Gövde serbest biçimli; düz metin alanlar `{{ trigger.<alan> }}` olur.
	payload := map[string]string{}
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err == nil {
		for k, v := range raw {
			if s, ok := v.(string); ok {
				payload[k] = s
			}
		}
	}

	// Proje verilmez: dışarıdan gelen bir olayın hangi projeye ait olduğuna
	// karar verecek bilgisi yok, akışın varsayılanı kullanılır.
	run, _, err := h.launchWorkflow(r.Context(), wf.ID, workflow.TriggerWebhook,
		payload, payload["input"], uuid.Nil)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	respondJSON(w, http.StatusAccepted, map[string]any{"workflowRunId": run.ID})
}

// jiraScanState, akışın son Jira taramasının sonucunu döner.
//
// Tarama arka planda sessizce çalışıyor; JQL yanlış yazıldığında veya Jira
// erişimi bozulduğunda kullanıcının bunu görebileceği tek yer burası. Hiç
// taranmamış akış için boş durum döner — hata değil.
func (h *Handler) jiraScanState(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	state, err := h.deps.Workflows.ScanState(r.Context(), id)
	if err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, state)
}

/*
 * Jira webhook'u.
 *
 * Gövdedeki alanlara GÜVENİLMEZ: yalnızca issue anahtarı alınır ve task
 * API'den yeniden okunur. Böylece webhook ile tarama AYNI veriyi görür ve
 * tekrar kontrolünün anahtarı (güncellenme zamanı) doğru olur.
 */
func (h *Handler) jiraHook(w http.ResponseWriter, r *http.Request) {
	if h.deps.JiraTrigger == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	wf, err := h.deps.Workflows.ByHookToken(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		respondError(w, http.StatusNotFound, "not_found", "adres bulunamadı")
		return
	}
	if !wf.IsActive {
		respondError(w, http.StatusConflict, "inactive", "akış pasif durumda")
		return
	}

	var body struct {
		Issue struct {
			Key string `json:"key"`
		} `json:"issue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Issue.Key == "" {
		respondError(w, http.StatusBadRequest, "no_issue", "gövdede issue anahtarı yok")
		return
	}

	started, err := h.deps.JiraTrigger.HandleWebhook(r.Context(), wf.ID, body.Issue.Key)
	if err != nil {
		slog.WarnContext(r.Context(), "Jira webhook işlenemedi",
			"workflow_id", wf.ID, "issue", body.Issue.Key, "error", err)
		respondError(w, http.StatusBadGateway, "jira_error", "task okunamadı")
		return
	}

	// Zaten işlenmiş task da 202 alır: Jira için bu bir hata değil, tekrar
	// gönderim. 4xx dönmek Jira'yı gereksizce yeniden denemeye iter.
	respondJSON(w, http.StatusAccepted, map[string]any{
		"issue": body.Issue.Key, "started": started,
	})
}

// deleteWorkflowRun, bitmiş bir akış çalışmasını adım çalıştırmalarıyla siler.
//
// Adım çalıştırmaları çalıştırmalar ekranından TEK TEK silinemiyor (bir akışa
// bağlılar); silinebilecekleri tek yer burası.
func (h *Handler) deleteWorkflowRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.deps.Workflows.DeleteRun(r.Context(), id); err != nil {
		h.respondWorkflowError(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "akış çalışması silindi", "workflow_run_id", id)
	w.WriteHeader(http.StatusNoContent)
}
