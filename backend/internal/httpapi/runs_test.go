package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/llm"
	"github.com/agent-coder/backend/internal/runs"
)

/*
 * Yapılandırma eksikleri 500 DÖNMEZ.
 *
 * Hiç LLM sağlayıcı tanımlı değilken çalıştırma başlatmak bir süre 500
 * "internal_error" veriyordu: yeni kurulum yapan biri ilk denemesinde
 * uygulamanın bozuk olduğunu sanıyordu. Oysa eksik olan tek şey bir ayardı.
 *
 * Bu test o ayrımı kilitliyor — eksik yapılandırma 4xx, gerçek arıza 5xx.
 */
func TestRespondRunError_YapilandirmaEksigi500Donmez(t *testing.T) {
	h := &Handler{}

	durumlar := []struct {
		ad   string
		err  error
		kod  int
		code string
	}{
		{"sağlayıcı yok", llm.ErrNotFound, http.StatusPreconditionFailed, "no_llm_provider"},
		{"git erişimi yok", runs.ErrNoGitAccess, http.StatusPreconditionFailed, "no_git_access"},
	}

	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.respondRunError(rec, httptest.NewRequest(http.MethodPost, "/api/runs", nil), d.err)

			require.Equal(t, d.kod, rec.Code)

			var body ErrorBody
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, d.code, body.Error.Code)
			// Mesaj kullanıcıya NE YAPACAĞINI söylemeli; "hata oluştu" yetmez.
			require.NotEmpty(t, body.Error.Message)
		})
	}

	t.Run("gerçek arıza 500 döner", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.respondRunError(rec, httptest.NewRequest(http.MethodPost, "/api/runs", nil),
			fmt.Errorf("beklenmedik"))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

/*
 * Gönderim engelinin SEBEBİ her vakada yazılır.
 *
 * GERÇEK BİR ŞİKÂYETİN KAYDI: zaten gönderilmiş bir koşuda kullanıcı
 * değişiklikleri görüyor ama "Branch'e gönder" düğmesini bulamıyordu — düğme
 * gizleniyordu ve tek iz, başlıktaki küçük branch rozetiydi. Rozet durumu
 * söylüyor, düğmenin YOKLUĞUNU açıklamıyor.
 *
 * Kural (silme düğmesindekinin aynısı): eylem gizlenmez, pasifleşir ve sebebi
 * yazılır. Sebep BRANCH ADINI taşır; "zaten gönderildi" tek başına "nereye?"
 * sorusunu açıkta bırakır.
 *
 * ErrNoChanges bunun DIŞINDA: gönderilecek bir değişiklik yokken düğmenin
 * konusu da yok. Orada gizlemek "yok" demek değil, doğruyu söylemektir.
 */
func TestPushEngeli_ZatenGonderilmisBranchiSoyler(t *testing.T) {
	h := &Handler{}
	dal := "agent-coder/coder-2cd9aa07"

	sebep := h.pushEngeli(context.Background(),
		runs.Run{Diff: "diff --git a/x b/x", PushedBranch: &dal})

	require.Contains(t, sebep, dal, "hangi branch'e gönderildiği yazılmalı")
}

func TestPushEngeli_DegisiklikYoksaSebepYazilmaz(t *testing.T) {
	h := &Handler{}

	require.Empty(t, h.pushEngeli(context.Background(), runs.Run{}))
}

// TestRespondRunError_SilmeEngelleri — silinemeyen bir kayıt "arıza" değil,
// KURAL. 409 döner ve mesaj kullanıcıya ne yapacağını söyler.
func TestRespondRunError_SilmeEngelleri(t *testing.T) {
	h := &Handler{}

	durumlar := []struct {
		ad   string
		err  error
		code string
	}{
		{"süren çalıştırma", runs.ErrActive, "run_active"},
		{"akış adımı", runs.ErrWorkflowStep, "run_workflow_step"},
	}

	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.respondRunError(rec,
				httptest.NewRequest(http.MethodDelete, "/api/runs/x", nil), d.err)

			require.Equal(t, http.StatusConflict, rec.Code)

			var body ErrorBody
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, d.code, body.Error.Code)
			require.NotEmpty(t, body.Error.Message)
		})
	}
}
