package httpapi

import (
	"net/http"

	"github.com/agent-coder/backend/internal/runner"
)

/*
 * Çalıştırma ortamı hakkında sabit bilgiler.
 *
 * Veritabanına dokunmaz: bu liste bir DERLEME ÇIKTISI ENVANTERİ — hangi
 * runner imajlarının yayınlandığını söyler. Ne topoloji (.env) ne davranış
 * (settings) olduğu için ikisine de girmiyor, ikiliye gömülü duruyor.
 */

type nodeVersionsResponse struct {
	Versions []string `json:"versions"`
}

// nodeVersions, imajı yayınlanmış Node sürümleri.
//
// Arayüz bu listeyi açılır listeye basar; serbest metin YOKTUR, çünkü
// yayınlanmamış bir sürüm seçmek koşuyu "imaj bulunamadı" ile düşürürdü.
func (h *Handler) nodeVersions(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, nodeVersionsResponse{Versions: runner.NodeVersions()})
}
