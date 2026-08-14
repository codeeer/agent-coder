package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/agent-coder/backend/internal/scripts"
)

/*
 * Script klasörleri — standart upgrade kampanyaları (spec 022).
 *
 * Klasör, betiklerin üzerinde bir gruplama: kampanyaya isim veriyor ve agent'a
 * tek hamlede bağlanmasını sağlıyor. Betik uçlarından AYRI duruyor çünkü
 * yaşam döngüleri ayrı — klasör silindiğinde betikler kalıyor.
 */

type folderRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// folderResponse, klasör + silmeden önce gösterilecek kullanım sayıları.
type folderResponse struct {
	scripts.Folder
	// AgentCount, klasörü kullanan agent sayısı. Listede gösterilir ki
	// kullanıcı bir kampanyayı silmeden önce kimin etkileneceğini bilsin.
	AgentCount int `json:"agentCount"`
}

func (h *Handler) listScriptFolders(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scripts == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	list, err := h.deps.Scripts.ListFolders(ctx)
	if err != nil {
		h.respondFolderError(w, err)
		return
	}

	out := make([]folderResponse, 0, len(list))
	for _, f := range list {
		// Agent sayısı klasör başına AYRI sorgu: klasör sayısı onlarla ölçülür,
		// script sayısı yüzlerle. Burada N+1 gerçek bir maliyet değil ve
		// listeyi tek sorguya sıkıştırmak okunurluğu bozardı.
		_, agents, err := h.deps.Scripts.FolderUsage(ctx, f.ID)
		if err != nil {
			slog.WarnContext(ctx, "klasör kullanımı okunamadı", "id", f.ID, "error", err)
		}
		out = append(out, folderResponse{Folder: f, AgentCount: agents})
	}

	respondJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) createScriptFolder(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scripts == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	var req folderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	f, err := h.deps.Scripts.CreateFolder(r.Context(),
		scripts.FolderInput{Name: req.Name, Description: req.Description})
	if err != nil {
		h.respondFolderError(w, err)
		return
	}

	slog.InfoContext(r.Context(), "betik klasörü açıldı", "id", f.ID, "name", f.Name)
	respondJSON(w, http.StatusCreated, folderResponse{Folder: f})
}

func (h *Handler) updateScriptFolder(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scripts == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	var req folderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	f, err := h.deps.Scripts.UpdateFolder(r.Context(), id,
		scripts.FolderInput{Name: req.Name, Description: req.Description})
	if err != nil {
		h.respondFolderError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, folderResponse{Folder: f})
}

/*
deleteScriptFolder, klasörü siler.

İçindeki betikler SİLİNMEZ, klasörsüz kalır. Arayüz silmeden önce kaç betiğin
klasörsüz kalacağını ve kaç agent'ın etkileneceğini `usage` ucundan okuyup
kullanıcıya söyler — "silinsin mi?" sorusundan farklı bir karar aldıran şey bu.
*/
func (h *Handler) deleteScriptFolder(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scripts == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := h.deps.Scripts.DeleteFolder(r.Context(), id); err != nil {
		h.respondFolderError(w, err)
		return
	}

	slog.InfoContext(r.Context(), "betik klasörü silindi", "id", id)
	w.WriteHeader(http.StatusNoContent)
}

// scriptFolderUsage, silme onayında gösterilecek sayılar.
func (h *Handler) scriptFolderUsage(w http.ResponseWriter, r *http.Request) {
	if h.deps.Scripts == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}

	scriptCount, agentCount, err := h.deps.Scripts.FolderUsage(r.Context(), id)
	if err != nil {
		h.respondFolderError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]int{
		"scriptCount": scriptCount, "agentCount": agentCount,
	})
}

func (h *Handler) respondFolderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, scripts.ErrFolderNotFound):
		respondError(w, http.StatusNotFound, "not_found", "klasör bulunamadı")

	case errors.Is(err, scripts.ErrDuplicateFolder):
		respondError(w, http.StatusConflict, "duplicate_folder",
			"Bu adda bir klasör zaten var.")

	case errors.Is(err, scripts.ErrMissingName):
		respondError(w, http.StatusBadRequest, "missing_name", "klasör adı zorunlu")

	case errors.Is(err, scripts.ErrInvalidName):
		// Mesaj NE YAZILABİLECEĞİNİ söyler: ad dizin adına dönüştüğü için
		// karakter kümesi dar ve sessiz dönüştürme yapılmıyor.
		respondError(w, http.StatusBadRequest, "invalid_name",
			"Klasör adı yalnızca küçük harf, rakam ve - içerebilir (örn. node-24-upgrade).")

	default:
		slog.Error("betik klasörü işlemi başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error",
			"klasör işlemi yapılamadı")
	}
}
