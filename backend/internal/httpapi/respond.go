package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorBody, API'nin tek tip hata yanıtı.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail hatanın makine okunur kodu ve insan okunur mesajı.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// respondJSON gövdeyi JSON olarak yazar.
//
// Kodlama hatası, gövdenin bir kısmı zaten yazılmış olabileceği için yalnızca
// loglanır — bu noktada istemciye anlamlı bir hata gönderilemez.
func respondJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("yanıt kodlanamadı", "error", err)
	}
}

// respondError tek tip hata yanıtı yazar.
//
// message istemciye gösterilir; iç hata detayları ve secret'lar buraya konmaz.
func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, ErrorBody{Error: ErrorDetail{Code: code, Message: message}})
}
