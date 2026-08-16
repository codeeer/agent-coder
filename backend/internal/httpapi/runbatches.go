package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/runbatch"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Toplu çalıştırma uçları (spec 023).
 *
 * Toplu iş bir çalıştırma DEĞİLDİR: kendi kaydı var ve içindeki her öğe kendi
 * akış çalışmasına bağlanıyor. Bu yüzden `/workflow-runs` altına değil kendi
 * ucuna oturuyor — "otuz işin durumu" ile "bir işin durumu" ayrı sorular.
 *
 * Uçlar KUYRUĞU BEKLETMEZ: başlatma yalnızca sıraya koyar ve zamanlayıcıyı
 * uyandırır. Bir toplu iş saatler sürebilir; HTTP isteği onu bekleyemez.
 */

type createBatchRequest struct {
	WorkflowID string   `json:"workflowId"`
	Task       string   `json:"task"`
	ProjectIDs []string `json:"projectIds"`
}

// batchResponse, toplu iş + öğeleri.
//
// Öğeler listede DEĞİL yalnızca detayda döner: liste ekranı otuz öğenin
// tamamını değil sayıları gösteriyor.
type batchResponse struct {
	runbatch.Batch
	Items []runbatch.Item `json:"items"`
}

func (h *Handler) createRunBatch(w http.ResponseWriter, r *http.Request) {
	if h.deps.RunBatches == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	var req createBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	workflowID, err := uuid.Parse(req.WorkflowID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_workflow", "akış kimliği geçersiz")
		return
	}

	// Kimlikler BURADA ayrıştırılır, veritabanına bırakılmaz: geçersiz bir
	// kimlik orada "işlem tamamlanamadı" olurdu.
	projectIDs := make([]uuid.UUID, 0, len(req.ProjectIDs))
	for _, raw := range req.ProjectIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid_project", "proje kimliği geçersiz")
			return
		}
		projectIDs = append(projectIDs, id)
	}
	if len(projectIDs) == 0 {
		respondError(w, http.StatusBadRequest, "no_projects",
			"en az bir proje seçin — toplu iş boş başlatılamaz")
		return
	}

	/*
	 * Akışın kayıtlı bir tanımı var mı — SIRAYA KOYMADAN ÖNCE.
	 *
	 * Yoksa otuz öğe de tek tek başlatılıp tek tek düşerdi: kullanıcı otuz
	 * satırlık bir başarısızlık listesi görür ve sebebi ancak satırların
	 * içinde okurdu. Yapılandırma eksiği arıza değildir; ne yapılacağını
	 * söyleyen bir 4xx ile döner.
	 */
	if h.deps.Workflows != nil {
		if _, err := h.deps.Workflows.ActiveVersion(r.Context(), workflowID); err != nil {
			h.respondWorkflowError(w, r, err)
			return
		}
	}

	batch, err := h.deps.RunBatches.Create(r.Context(), workflowID, req.Task, projectIDs)
	if err != nil {
		h.respondBatchError(w, r, err)
		return
	}

	// Kuyruk hemen uyandırılır: ilk öğe bir sonraki emniyet turunu beklemesin.
	if h.deps.BatchQueue != nil {
		h.deps.BatchQueue.Wake()
	}

	slog.InfoContext(r.Context(), "toplu iş sıraya alındı",
		"batch_id", batch.ID, "workflow_id", workflowID, "adet", batch.Counts.Total)
	respondJSON(w, http.StatusCreated, batchResponse{Batch: batch, Items: nil})
}

func (h *Handler) listRunBatches(w http.ResponseWriter, r *http.Request) {
	if h.deps.RunBatches == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	page := pageOf(r)
	items, total, err := h.deps.RunBatches.List(r.Context(), page.Limit, page.Offset)
	if err != nil {
		h.respondBatchError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, paged(items, total, page))
}

func (h *Handler) getRunBatch(w http.ResponseWriter, r *http.Request) {
	if h.deps.RunBatches == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	batch, items, err := h.deps.RunBatches.Get(r.Context(), id)
	if err != nil {
		h.respondBatchError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, batchResponse{Batch: batch, Items: items})
}

// batchActionResponse, iptal ve devam sonucu.
//
// SAYI DÖNER çünkü kullanıcıya ne olduğu söylenmeli: "iptal edildi" değil
// "12 bekleyen düştü, 2 iş sürüyor". Durum da döner — bitmiş bir işi iptal
// etmek hata değil, sonucu "zaten bitmişti"dir.
type batchActionResponse struct {
	Affected int             `json:"affected"`
	Status   string          `json:"status"`
	Counts   runbatch.Counts `json:"counts"`
}

func (h *Handler) cancelRunBatch(w http.ResponseWriter, r *http.Request) {
	h.batchAction(w, r, func(id uuid.UUID) (int, error) {
		return h.deps.RunBatches.Cancel(r.Context(), id)
	})
}

func (h *Handler) resumeRunBatch(w http.ResponseWriter, r *http.Request) {
	h.batchAction(w, r, func(id uuid.UUID) (int, error) {
		n, err := h.deps.RunBatches.Resume(r.Context(), id)
		if err == nil && n > 0 && h.deps.BatchQueue != nil {
			// Sıraya alınan öğeler bir sonraki emniyet turunu beklemesin.
			h.deps.BatchQueue.Wake()
		}
		return n, err
	})
}

// batchAction, iptal ve devamın ortak gövdesi: kimlik, eylem, güncel durum.
func (h *Handler) batchAction(w http.ResponseWriter, r *http.Request,
	do func(uuid.UUID) (int, error),
) {
	if h.deps.RunBatches == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	n, err := do(id)
	if err != nil {
		h.respondBatchError(w, r, err)
		return
	}

	// Eylemden SONRAKİ durum okunur: ekran kendi tahminiyle değil sistemin
	// söylediğiyle tazelenir.
	batch, _, err := h.deps.RunBatches.Get(r.Context(), id)
	if err != nil {
		h.respondBatchError(w, r, err)
		return
	}
	respondJSON(w, http.StatusOK, batchActionResponse{
		Affected: n, Status: batch.Status, Counts: batch.Counts})
}

func (h *Handler) respondBatchError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, runbatch.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "toplu iş bulunamadı")
	case errors.Is(err, runbatch.ErrNoProjects):
		respondError(w, http.StatusBadRequest, "no_projects",
			"en az bir proje seçin — toplu iş boş başlatılamaz")
	case errors.Is(err, runbatch.ErrDuplicateProject):
		respondError(w, http.StatusBadRequest, "duplicate_project",
			"aynı proje birden fazla kez seçilmiş")
	case errors.Is(err, runbatch.ErrWorkflowNotFound):
		respondError(w, http.StatusNotFound, "workflow_not_found", "akış bulunamadı")
	case errors.Is(err, runbatch.ErrRunning):
		respondError(w, http.StatusConflict, "batch_running",
			"toplu iş sürüyor — önce iptal edin")
	case errors.Is(err, runbatch.ErrProjectNotFound):
		respondError(w, http.StatusNotFound, "project_not_found",
			"seçilen projelerden biri bulunamadı")
	case errors.Is(err, workflow.ErrNotFound):
		respondError(w, http.StatusNotFound, "workflow_not_found", "akış bulunamadı")
	default:
		slog.ErrorContext(r.Context(), "toplu iş işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "işlem tamamlanamadı")
	}
}

/*
deleteRunBatch, bitmiş bir toplu işi geçmişiyle birlikte siler.

Toplu iş listesi birikip temizlenemiyordu: otuz projelik bir kampanya bitince
listede kalıyor ve kaldırmanın hiçbir yolu yoktu.
*/
func (h *Handler) deleteRunBatch(w http.ResponseWriter, r *http.Request) {
	if h.deps.RunBatches == nil || h.deps.Workflows == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.deps.RunBatches.Delete(r.Context(), id, calismaSilici{h.deps.Workflows}); err != nil {
		h.respondBatchError(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "toplu iş silindi", "batch_id", id)
	w.WriteHeader(http.StatusNoContent)
}

// calismaSilici, `workflow.Store`u `runbatch.RunDeleter` sözleşmesine uydurur.
//
// Tek işi var: ZATEN SİLİNMİŞ çalışmayı hata saymamak. Yarıda kalmış bir silme
// tekrar denendiğinde ilk turda gidenler `ErrNotFound` döndürür ve bu, işlemi
// bir daha asla tamamlanamaz hâle getirirdi.
type calismaSilici struct{ store *workflow.Store }

func (c calismaSilici) DeleteRun(ctx context.Context, id uuid.UUID) error {
	err := c.store.DeleteRun(ctx, id)
	if errors.Is(err, workflow.ErrNotFound) {
		return nil
	}
	return err
}
