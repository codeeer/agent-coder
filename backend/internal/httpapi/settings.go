package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/agent-coder/backend/internal/certinfo"
	"github.com/agent-coder/backend/internal/hostlist"
	"github.com/agent-coder/backend/internal/settings"
)

// settingsResponse, ayarlar listesi ve grup etiketleri.
//
// Arayüz kendini bu yanıttan çizer: her ayarın etiketi, açıklaması, tipi,
// birimi ve aralığı burada gelir. Yeni bir parametre eklendiğinde frontend'e
// dokunmak gerekmez.
type settingsResponse struct {
	Items  []settings.Value  `json:"items"`
	Groups map[string]string `json:"groups"`
}

type putSettingRequest struct {
	Value string `json:"value"`
}

func (h *Handler) listSettings(w http.ResponseWriter, _ *http.Request) {
	if h.deps.Settings == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	respondJSON(w, http.StatusOK, settingsResponse{
		Items:  h.deps.Settings.All(),
		Groups: settings.GroupLabels,
	})
}

func (h *Handler) putSetting(w http.ResponseWriter, r *http.Request) {
	if h.deps.Settings == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	key := chi.URLParam(r, "key")

	var req putSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	if err := h.deps.Settings.Set(r.Context(), key, req.Value); err != nil {
		h.respondSettingsError(w, r, key, err)
		return
	}

	slog.InfoContext(r.Context(), "ayar değiştirildi", "key", key, "value", logDegeri(key, req.Value))
	respondJSON(w, http.StatusOK, settingsResponse{
		Items:  h.deps.Settings.All(),
		Groups: settings.GroupLabels,
	})
}

func (h *Handler) resetSetting(w http.ResponseWriter, r *http.Request) {
	if h.deps.Settings == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}

	key := chi.URLParam(r, "key")

	if err := h.deps.Settings.Reset(r.Context(), key); err != nil {
		h.respondSettingsError(w, r, key, err)
		return
	}

	slog.InfoContext(r.Context(), "ayar varsayılana döndürüldü", "key", key)
	respondJSON(w, http.StatusOK, settingsResponse{
		Items:  h.deps.Settings.All(),
		Groups: settings.GroupLabels,
	})
}

// respondSettingsError, ayar hatalarını uygun HTTP koduna çevirir.
//
// Aralık hatasının mesajı olduğu gibi geçirilir: kullanıcının ne yapması
// gerektiğini ("1 ile 20 arasında olmalı") tam olarak o söylüyor.
func (h *Handler) respondSettingsError(w http.ResponseWriter, r *http.Request, key string, err error) {
	switch {
	case errors.Is(err, settings.ErrUnknownKey):
		respondError(w, http.StatusNotFound, "unknown_setting", "bilinmeyen ayar")

	case errors.Is(err, settings.ErrOutOfRange), errors.Is(err, settings.ErrInvalidValue):
		respondError(w, http.StatusBadRequest, "invalid_value", err.Error())

	default:
		slog.ErrorContext(r.Context(), "ayar işlemi başarısız", "key", key, "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error", "ayar kaydedilemedi")
	}
}

/*
 * logDegeri, ayar değerinin loga yazılacak hâli.
 *
 * Ayar değerleri BİLEREK loglanıyor: hangi ayarın ne yapıldığı, sonradan
 * "bu neden böyle" sorusunun tek kaydı. Sertifika bunun istisnası — sır
 * olduğu için değil (kök sertifika herkese dağıtılır), ÖLÇÜSÜ yüzünden:
 * her kaydetmede ~2KB base64 loga düşüyor ve o satırdan sonra logu okumak
 * imkânsızlaşıyordu. Yerine ne kaydedildiği özetle yazılır.
 */
func logDegeri(key, value string) string {
	def, ok := settings.Lookup(key)
	if !ok {
		return value
	}

	/*
	 * İzinli domain listesi de özetlenir — sertifikayla aynı ÖLÇÜ gerekçesi.
	 * Kurumsal bir listede onlarca satır olabiliyor; ham hâliyle loglanırsa
	 * o satırdan sonrasını okumak zorlaşır. Değer sır değil, ama logu
	 * kullanılamaz hale getirmesi de kabul edilemez.
	 */
	if def.Kind == settings.KindHostList {
		if strings.TrimSpace(value) == "" {
			return "(temizlendi)"
		}
		desenler, err := hostlist.Parse(value)
		if err != nil {
			return "(izinli domain listesi, okunamadı)"
		}
		return fmt.Sprintf("izinli domain listesi: %d domain", len(desenler))
	}

	if def.Kind != settings.KindCertificate {
		return value
	}
	if strings.TrimSpace(value) == "" {
		return "(temizlendi)"
	}
	infos, err := certinfo.Parse(value)
	if err != nil {
		return "(sertifika, okunamadı)"
	}
	adlar := make([]string, 0, len(infos))
	for _, i := range infos {
		adlar = append(adlar, i.Subject)
	}
	return "sertifika: " + strings.Join(adlar, ", ")
}
