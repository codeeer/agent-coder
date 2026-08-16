package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * API'nin tek tip yanıt biçimi.
 *
 * Bu iki fonksiyondan HER uç geçiyor. Buradaki bir kayma tek bir ekranı değil
 * arayüzün tamamının hata işlemesini bozar: istemci `error.code` üzerinden
 * karar veriyor.
 */

func TestRespondError_TekTipGovde(t *testing.T) {
	rec := httptest.NewRecorder()
	respondError(rec, http.StatusConflict, "inactive", "akış pasif durumda")

	require.Equal(t, http.StatusConflict, rec.Code)

	var body ErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "inactive", body.Error.Code)
	require.Equal(t, "akış pasif durumda", body.Error.Message)
}

/*
TÜRKÇE KARAKTERLER BOZULMADAN GİDER.

Ürünün her mesajı Türkçe ve `charset=utf-8` bunun sözü. Başlık yanlış olsaydı
bazı istemciler "akış pasif" yerine "akÄ±ÅŸ pasif" gösterirdi — ve bu, hata
mesajını okunamaz kılan en sinsi arıza türü.
*/
func TestRespondError_TurkceKarakterlerKorunur(t *testing.T) {
	rec := httptest.NewRecorder()
	respondError(rec, http.StatusBadRequest, "invalid_body",
		"gövde ayrıştırılamadı — şema geçersiz, çığır açan bir şey değil")

	require.Contains(t, rec.Header().Get("Content-Type"), "charset=utf-8")

	var body ErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Contains(t, body.Error.Message, "ayrıştırılamadı")
	require.Contains(t, body.Error.Message, "çığır")
}

func TestRespondJSON_IcerikTuruVeDurum(t *testing.T) {
	rec := httptest.NewRecorder()
	respondJSON(rec, http.StatusCreated, map[string]string{"a": "b"})

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
	require.JSONEq(t, `{"a":"b"}`, rec.Body.String())
}

/*
GÖVDESİZ YANIT GERÇEKTEN BOŞ.

`nil` gövde `null` olarak yazılsaydı, 204 benzeri yanıtları okuyan istemci
gövdeyi ayrıştırmaya çalışır ve "null" değeriyle uğraşırdı.
*/
func TestRespondJSON_NilGovdeBosBirakir(t *testing.T) {
	rec := httptest.NewRecorder()
	respondJSON(rec, http.StatusNoContent, nil)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String())
}

// Boş dizi `null` DEĞİL `[]` olarak gider: arayüz doğrudan üzerinde döngü
// kuruyor ve `null` üzerinde hata verirdi.
func TestRespondJSON_BosDiziNullOlmaz(t *testing.T) {
	rec := httptest.NewRecorder()
	respondJSON(rec, http.StatusOK, []string{})

	require.Equal(t, "[]", trimJSON(rec.Body.String()))
}
