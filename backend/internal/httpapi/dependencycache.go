package httpapi

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/settings"
)

/*
 * Bağımlılık önbelleği bakımı (spec 027 H3).
 *
 * Bu katman çalışma ortamının iç bilgisini görmez: volume adları, container
 * içi yollar ve Docker `runner.CacheAdmin` arayüzünün arkasında kalır. Buraya
 * gelen tek şey "maven"/"npm" kimlikleri ve bayt sayıları.
 *
 * YETKİ: bu uçlar bugün `PUT /api/settings` ile aynı durumda — kimlik
 * doğrulaması yok, çünkü üründe henüz kimlik yok. Temizleme ayar yazma
 * sınıfındadır ve kimlik geldiğinde aynı kapıdan geçmelidir; spec 024 H1'e
 * kabul kriteri olarak yazıldı.
 */

// dependencyCacheResponse, önbellek durumu yanıtı.
type dependencyCacheResponse struct {
	// Enabled, ayarın açık olup olmadığı. Kapalıyken önbellek dolmaz ama
	// biriken içerik durur — kullanıcı yine boyutunu görüp temizleyebilmeli.
	Enabled bool               `json:"enabled"`
	Caches  []runner.CacheInfo `json:"caches"`
}

// clearCacheResponse, temizleme sonucu.
type clearCacheResponse struct {
	// FreedBytes, boşalan bayt. Arayüz bunu kullanıcıya SAYIYLA gösterir.
	FreedBytes int64              `json:"freedBytes"`
	Caches     []runner.CacheInfo `json:"caches"`
}

// cacheAdmin, bakım arayüzünü sağlayan runner'ı döner.
//
// Runner bu arayüzü sağlamıyorsa bakım uçları kapalıdır: bakım isteğe bağlı
// bir yetenek, çalıştırmanın önkoşulu değil.
func (h *Handler) cacheAdmin() (runner.CacheAdmin, bool) {
	admin, ok := h.deps.Runner.(runner.CacheAdmin)
	return admin, ok
}

func (h *Handler) dependencyCacheStatus(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.cacheAdmin()
	if !ok {
		respondError(w, http.StatusServiceUnavailable, "cache_unavailable",
			"bu kurulumda önbellek bakımı yapılamıyor")
		return
	}

	caches, err := admin.CacheStatus(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "önbellek durumu okunamadı", "error", err)
		respondError(w, http.StatusBadGateway, "cache_status_failed", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, dependencyCacheResponse{
		Enabled: h.dependencyCacheEnabled(),
		Caches:  caches,
	})
}

func (h *Handler) clearDependencyCache(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.cacheAdmin()
	if !ok {
		respondError(w, http.StatusServiceUnavailable, "cache_unavailable",
			"bu kurulumda önbellek bakımı yapılamıyor")
		return
	}

	id := runner.CacheID(chi.URLParam(r, "id"))

	/*
	 * SÜREN KOŞU VARKEN TEMİZLENMEZ (spec 027 H3).
	 *
	 * Kapı burada çünkü yalnızca bu katman hem çalıştırma yöneticisini hem
	 * önbelleği görüyor. Kaç koşunun sürdüğü de söyleniyor: "şu an yapılamaz"
	 * tek başına kullanıcıyı ne zaman deneyeceğini bilmeden bırakır.
	 */
	if aktif := h.activeRunCount(); aktif > 0 {
		respondError(w, http.StatusConflict, "runs_active",
			fmt.Sprintf("%d çalıştırma sürüyor — önbellek onlar bitince temizlenebilir",
				aktif))
		return
	}

	freed, err := admin.ClearCache(r.Context(), id)
	switch {
	case errors.Is(err, runner.ErrUnknownCache):
		respondError(w, http.StatusNotFound, "unknown_cache", "böyle bir önbellek yok")
		return
	case errors.Is(err, runner.ErrCacheBusy):
		/*
		 * ÇALIŞAN KOŞU YOK AMA VOLUME BAĞLI.
		 *
		 * Yukarıdaki kapıdan geçtiğine göre kayıtlı bir çalıştırma sürmüyor;
		 * yine de bir container volume'ü tutuyor. En olası sebep sahipsiz
		 * kalmış bir container.
		 *
		 * ÜRÜN ONU KENDİLİĞİNDEN ÖLDÜRMEZ: hangi container olduğunu bilmiyoruz
		 * ve yanlış bir şeyi durdurmak, temizlemenin çözdüğünden büyük bir
		 * sorun açar. Kullanıcıya NEREYE BAKACAĞI söylenir, karar onun.
		 */
		respondError(w, http.StatusConflict, "cache_in_use",
			"çalışan bir koşu yok ama önbellek hâlâ bağlı — büyük olasılıkla "+
				"sahipsiz kalmış bir çalışma ortamı tutuyor. "+err.Error())
		return
	case err != nil:
		slog.ErrorContext(r.Context(), "önbellek temizlenemedi", "cache", id, "error", err)
		respondError(w, http.StatusBadGateway, "cache_clear_failed", err.Error())
		return
	}

	/*
	 * KAYDA GEÇER (spec 027 T36).
	 *
	 * "Kim" alanı YOK ve uydurulmuyor: üründe henüz kimlik yok (spec 024).
	 * Bugün gerçekten bilinen şey kaydediliyor — ne zaman, hangi ekosistem,
	 * ne kadar boşaldı, hangi istek. Kimlik geldiğinde buraya bir alan eklenir.
	 */
	slog.InfoContext(r.Context(), "bağımlılık önbelleği temizlendi",
		"cache", id, "freed_bytes", freed,
		"request_id", middleware.GetReqID(r.Context()))

	caches, err := admin.CacheStatus(r.Context())
	if err != nil {
		// Temizleme BAŞARILI oldu; durum okunamadıysa da bunu bildirmek gerekir.
		slog.WarnContext(r.Context(), "temizleme sonrası durum okunamadı", "error", err)
	}
	respondJSON(w, http.StatusOK, clearCacheResponse{FreedBytes: freed, Caches: caches})
}

/*
verifyDependencyCache, önbelleğin bütünlüğünü denetler (spec 027 H5).

SÜREN KOŞU VARKEN ÇALIŞMAZ ve bu kapı temizlemedekinden daha kritik: koşu
sürerken tarama yapılırsa yarım inmiş bir artefakt "bozuk" görünür ve silinir
— yani doğrulama, düzeltmesi gereken şeyi kendisi üretir. İkinci kat koruma
tarama mantığında: özeti okunamayan artefakt zaten silinmiyor.
*/
func (h *Handler) verifyDependencyCache(w http.ResponseWriter, r *http.Request) {
	admin, ok := h.cacheAdmin()
	if !ok {
		respondError(w, http.StatusServiceUnavailable, "cache_unavailable",
			"bu kurulumda önbellek bakımı yapılamıyor")
		return
	}

	if aktif := h.activeRunCount(); aktif > 0 {
		respondError(w, http.StatusConflict, "runs_active",
			fmt.Sprintf("%d çalıştırma sürüyor — inmekte olan artefaktı bozuk "+
				"saymamak için doğrulama onlar bitince yapılır", aktif))
		return
	}

	id := runner.CacheID(chi.URLParam(r, "id"))
	sonuc, err := admin.VerifyCache(r.Context(), id)
	switch {
	case errors.Is(err, runner.ErrUnknownCache):
		respondError(w, http.StatusNotFound, "unknown_cache", "böyle bir önbellek yok")
		return
	case err != nil:
		slog.ErrorContext(r.Context(), "önbellek doğrulanamadı", "cache", id, "error", err)
		respondError(w, http.StatusBadGateway, "cache_verify_failed", err.Error())
		return
	}

	slog.InfoContext(r.Context(), "bağımlılık önbelleği doğrulandı",
		"cache", id, "checked", sonuc.Checked, "mismatched", sonuc.Mismatched,
		"unverifiable", sonuc.Unverifiable, "removed", sonuc.Removed,
		"request_id", middleware.GetReqID(r.Context()))

	respondJSON(w, http.StatusOK, sonuc)
}

// dependencyCacheEnabled, ayarın açık olup olmadığı.
func (h *Handler) dependencyCacheEnabled() bool {
	if h.deps.Settings == nil {
		return false
	}
	return h.deps.Settings.Bool(settings.KeyDependencyCache)
}

// activeRunCount, süren çalıştırma sayısı.
func (h *Handler) activeRunCount() int {
	if h.deps.RunManager == nil {
		return 0
	}
	return h.deps.RunManager.Active()
}
