package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/agent-coder/backend/internal/events"
	"github.com/agent-coder/backend/internal/runs"
)

// sseHeartbeat, bağlantıyı canlı tutmak için gönderilen yorum satırı aralığı.
//
// Aradaki vekil sunucular sessiz bağlantıları kapatır; bu satır bunu önler.
const sseHeartbeat = 25 * time.Second

// runEvents, bir çalıştırmanın olaylarını SSE ile yayınlar.
//
// Önce VERİTABANINDAKİ GEÇMİŞ gönderilir, sonra canlı akışa geçilir.
// Bu sıra sayesinde sayfa yenilense de o ana kadarki ilerleme kaybolmaz
// (spec 003 H4).
func (h *Handler) runEvents(w http.ResponseWriter, r *http.Request) {
	if h.deps.Runs == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	run, err := h.deps.Runs.Get(ctx, id)
	if err != nil {
		h.respondRunError(w, r, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		respondError(w, http.StatusInternalServerError, "no_streaming",
			"bu bağlantı canlı akışı desteklemiyor")
		return
	}

	// Aboneliğe GEÇMİŞTEN ÖNCE başlanır: aradaki boşlukta üretilen olaylar
	// kaçmasın. Tekrar eden olaylar istemcide seq ile ayıklanır.
	stream, unsubscribe := h.deps.Bus.Subscribe(id)
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// Vekil sunucular SSE'yi tamponlamasın.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	history, err := h.deps.Runs.Events(ctx, id)
	if err != nil {
		slog.WarnContext(ctx, "olay geçmişi okunamadı", "run_id", id, "error", err)
	}

	lastSeq := 0
	for _, e := range history {
		writeSSE(w, events.Event{
			Seq: e.Seq, Level: e.Level, Message: e.Message,
			TS: e.TS.UTC().Format(time.RFC3339),
		})
		lastSeq = e.Seq
	}

	// Çalıştırma zaten bitmişse canlı akışa gerek yok: bitiş SİNYALİ gönderilip
	// bağlantı kapatılır. Aksi halde tarayıcı boşuna açık bağlantı tutar.
	//
	// Mesaj bilinçli olarak BOŞ: bitiş satırı geçmişte zaten var. Buraya bir
	// metin konsaydı aynı cümle iki kez görünürdü.
	if run.Status.Terminal() {
		writeSSE(w, events.Event{
			Seq: lastSeq, Level: "info",
			Terminal: true, Status: string(run.Status),
			TS: time.Now().UTC().Format(time.RFC3339),
		})
		flusher.Flush()
		return
	}
	flusher.Flush()

	ticker := time.NewTicker(sseHeartbeat)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Tarayıcı bağlantıyı kapattı; abonelik defer ile kaldırılır.
			return

		case e, open := <-stream:
			if !open {
				return
			}
			// Geçmişte gönderilenleri tekrar etme.
			if e.Seq > 0 && e.Seq <= lastSeq {
				continue
			}
			if e.Seq > lastSeq {
				lastSeq = e.Seq
			}
			writeSSE(w, e)
			flusher.Flush()

			if e.Terminal {
				return
			}

		case <-ticker.C:
			// SSE yorum satırı: istemci yok sayar, bağlantı canlı kalır.
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, e events.Event) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
}

// runStatusTone, durumu arayüzün kullanacağı renk tonuna çevirir.
// Burada tutulur ki backend ve frontend aynı eşlemeyi paylaşsın.
func runStatusTone(s runs.Status) string {
	switch s {
	case runs.StatusSucceeded:
		return "success"
	case runs.StatusRunning, runs.StatusPending:
		return "accent"
	case runs.StatusCancelled, runs.StatusInterrupted:
		return "neutral"
	default:
		return "danger"
	}
}

var _ = runStatusTone
