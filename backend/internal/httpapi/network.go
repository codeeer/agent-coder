package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"github.com/agent-coder/backend/internal/cacert"
	"github.com/agent-coder/backend/internal/certfmt"
	"github.com/agent-coder/backend/internal/certinfo"
)

/*
 * Kurumsal ağ uçları.
 *
 * İkisi de SERTİFİKA KAYDETMEZ. Kaydetme, diğer bütün ayarlarla aynı yerden
 * yapılıyor (`PUT /api/settings/network.corporate_ca`); buraya ikinci bir
 * yazma yolu koymak, aynı değerin iki farklı doğrulamadan geçmesi demekti.
 */

// caNormalizeMaxBytes, kabul edilen azami gövde.
//
// Bir sertifika zinciri bunun çok altındadır; sınır, ayarlar tablosuna büyük
// bir gövdenin yazılmasını da engelliyor.
const caNormalizeMaxBytes = 64 << 10

// caStatusResponse, sertifikanın durumu.
type caStatusResponse struct {
	// Source: "settings" | "env" | "none".
	//
	// İki kaynak birden mümkün olduğu için "tanımlı" demek yetmiyor;
	// kullanıcı hangisinin geçerli olduğunu bilmeli (spec 017 H3).
	Source       cacert.Source   `json:"source"`
	Certificates []certinfo.Info `json:"certificates"`
}

// caCert, kurumsal sertifikanın durumunu ve içinden okunan bilgiyi döner.
func (h *Handler) caCert(w http.ResponseWriter, r *http.Request) {
	if h.deps.CACert == nil {
		respondJSON(w, http.StatusOK, caStatusResponse{
			Source: cacert.SourceNone, Certificates: []certinfo.Info{}})
		return
	}

	pemText, source := h.deps.CACert.Resolve()
	out := caStatusResponse{Source: source, Certificates: []certinfo.Info{}}
	if pemText != "" {
		infos, err := certinfo.Parse(pemText)
		if err != nil {
			// Kaydederken doğrulanıyor; buraya düşmek tutarsızlıktır.
			// Kaynak yine de bildirilir — kullanıcı "tanımlı ama okunamıyor"
			// durumunu görebilmeli.
			respondJSON(w, http.StatusOK, out)
			return
		}
		out.Certificates = infos
	}
	respondJSON(w, http.StatusOK, out)
}

type caNormalizeRequest struct {
	// Data, seçilen dosyanın baytlarının base64'ü.
	//
	// İkili dosyalar (DER, PKCS#7) JSON'a doğrudan yazılamıyor; base64 hem
	// metin hem ikili girdiyi tek bir alanla taşıyor.
	Data string `json:"data"`
}

type caNormalizeResponse struct {
	PEM          string          `json:"pem"`
	Certificates []certinfo.Info `json:"certificates"`
}

/*
 * caNormalize, seçilen dosyayı PEM'e çevirir — SAKLAMAZ.
 *
 * Arayüz akışı: kullanıcı dosya seçer → baytlar buraya gelir → dönen PEM
 * metin alanına düşer → kullanıcı GÖRÜR → normal "Kaydet" akışıyla kaydeder.
 * Dosya seçmenin tek başına kaydetmemesi bir spec kararı (H1): kullanıcı ne
 * kaydettiğini görmeden kaydetmemeli.
 */
func (h *Handler) caNormalize(w http.ResponseWriter, r *http.Request) {
	var req caNormalizeRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, caNormalizeMaxBytes*2)).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "geçersiz gövde")
		return
	}

	raw, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "dosya içeriği çözülemedi")
		return
	}
	if len(raw) > caNormalizeMaxBytes {
		respondError(w, http.StatusRequestEntityTooLarge, "too_large",
			"dosya çok büyük — sertifika dosyaları 64 KB'ın altındadır")
		return
	}

	pemText, err := certfmt.ToPEM(raw)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_certificate",
			"dosya sertifika içermiyor")
		return
	}
	infos, err := certinfo.Parse(pemText)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_certificate",
			"sertifika okunamadı")
		return
	}

	respondJSON(w, http.StatusOK, caNormalizeResponse{PEM: pemText, Certificates: infos})
}
