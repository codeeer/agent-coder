package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/agentreg"
	"github.com/agent-coder/backend/internal/gitprovider"
	"github.com/agent-coder/backend/internal/projects"
)

/*
 * Alan hatalarının HTTP karşılıkları.
 *
 * Bu eşlemeler sessizce yanlış olabilir ve iki yönde de pahalı:
 *
 *   Kullanıcının DÜZELTEBİLECEĞİ bir sorun 500 dönerse, kullanıcı ürünün
 *   bozulduğunu sanır ve kendi yazdığı yanlış adresi hiç aramaz.
 *
 *   Gerçek bir arıza 4xx dönerse, kullanıcı saatlerce kendi girdisini
 *   düzeltmeye çalışır — düzeltilecek bir şey yokken.
 */

// eslesme, bir alan hatasının beklenen HTTP karşılığı.
type eslesme struct {
	ad   string
	err  error
	kod  int
	code string
}

func sinaEslesme(t *testing.T, yaz func(http.ResponseWriter, *http.Request, error), d eslesme) {
	t.Helper()

	rec := httptest.NewRecorder()
	yaz(rec, httptest.NewRequest(http.MethodGet, "/", nil), d.err)

	require.Equal(t, d.kod, rec.Code)

	var body ErrorBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, d.code, body.Error.Code)
	require.NotEmpty(t, body.Error.Message, "istemciye gösterilecek bir mesaj olmalı")
}

/*
DEPO DOĞRULAMA HATALARI 422 — 500 DEĞİL.

Depoya ulaşılamaması, erişimin reddedilmesi ve branch'in bulunmaması ürünün
arızası değil; kullanıcının girdisiyle ilgili ve düzeltilebilir. 500 dönselerdi
kullanıcı adresi ya da anahtarı kontrol etmek yerine sunucunun çöktüğünü
sanardı.
*/
func TestRespondProjectError_Eslesmeler(t *testing.T) {
	h := &Handler{}

	for _, d := range []eslesme{
		{"proje yok", projects.ErrNotFound, http.StatusNotFound, "not_found"},
		{"ad boş", projects.ErrEmptyName, http.StatusBadRequest, "invalid_name"},
		{"adres geçersiz", projects.ErrInvalidRepoURL, http.StatusBadRequest, "invalid_repo_url"},
		{"erişim reddedildi", projects.ErrRepoAuth, http.StatusUnprocessableEntity, "repo_auth"},
		{"branch yok", projects.ErrBranchNotFound, http.StatusUnprocessableEntity, "branch_not_found"},
		{"depoya ulaşılamadı", projects.ErrRepoUnreachable, http.StatusUnprocessableEntity, "repo_unreachable"},
		{"git erişimi yok", gitprovider.ErrNotFound, http.StatusBadRequest, "git_provider_not_found"},
	} {
		t.Run(d.ad, func(t *testing.T) { sinaEslesme(t, h.respondProjectError, d) })
	}
}

/*
AGENT ENGELLERİ 409 — çünkü hiçbiri "yanlış istek" değil, ÇAKIŞMA.

Hazır agent'ın silinememesi ya da çalıştırma geçmişi olan bir agent'ın
silinememesi kullanıcının hatası değil; sistemin o an içinde bulunduğu durum.
400 dönselerdi kullanıcı gönderdiği veriyi düzeltmeye çalışırdı.
*/
func TestRespondAgentError_Eslesmeler(t *testing.T) {
	h := &Handler{}

	for _, d := range []eslesme{
		{"agent yok", agentreg.ErrNotFound, http.StatusNotFound, "not_found"},
		{"kısa ad dolu", agentreg.ErrSlugTaken, http.StatusConflict, "slug_taken"},
		{"hazır agent silinemez", agentreg.ErrBuiltinDelete, http.StatusConflict, "builtin_delete"},
		{"yalnızca hazır sıfırlanır", agentreg.ErrNotBuiltin, http.StatusConflict, "not_builtin"},
		{"kullanımda", agentreg.ErrInUse, http.StatusConflict, "agent_in_use"},
		{"talimat boş", agentreg.ErrEmptyPrompt, http.StatusBadRequest, "empty_prompt"},
		{"talimat büyük", agentreg.ErrPromptTooLarge, http.StatusBadRequest, "prompt_too_large"},
		{"kısa ad geçersiz", agentreg.ErrInvalidSlug, http.StatusBadRequest, "invalid_slug"},
	} {
		t.Run(d.ad, func(t *testing.T) { sinaEslesme(t, h.respondAgentError, d) })
	}
}

/*
SARILMIŞ HATA DA TANINIR.

Alan katmanı hatayı bağlamla sarmalıyor (`fmt.Errorf("%w: ...")`). Eşleme
`errors.Is` yerine doğrudan karşılaştırma yapsaydı, sarmalanan her hata
sessizce 500'e düşerdi — ve bu, tam da hata mesajının en açıklayıcı olduğu
durumda olurdu.
*/
func TestRespondError_SarilmisHataTanini(t *testing.T) {
	h := &Handler{}

	sinaEslesme(t, h.respondProjectError, eslesme{
		err: fmt.Errorf("depo sınanamadı: %w", projects.ErrRepoAuth),
		kod: http.StatusUnprocessableEntity, code: "repo_auth",
	})
	sinaEslesme(t, h.respondAgentError, eslesme{
		err: fmt.Errorf("kaydedilemedi: %w", agentreg.ErrSlugTaken),
		kod: http.StatusConflict, code: "slug_taken",
	})
}

/*
TANINMAYAN HATA 500 DÖNER — ve içeriğini SIZDIRMAZ.

Beklenmeyen bir arızanın ham metni istemciye gitseydi, veritabanı hatası ya da
bağlantı dizesi gibi iç ayrıntılar tarayıcıya düşerdi. Ayrıntı loga yazılıyor,
kullanıcıya tek tip bir cümle gidiyor.
*/
func TestRespondError_TaninmayanHata500VeSizdirmaz(t *testing.T) {
	h := &Handler{}
	gizli := errors.New("pq: connection to host 10.0.0.5 user agentcoder failed")

	for _, yaz := range []func(http.ResponseWriter, *http.Request, error){
		h.respondProjectError, h.respondAgentError,
	} {
		rec := httptest.NewRecorder()
		yaz(rec, httptest.NewRequest(http.MethodGet, "/", nil), gizli)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
		require.NotContains(t, rec.Body.String(), "10.0.0.5", "iç ayrıntı tele çıkmamalı")
		require.NotContains(t, rec.Body.String(), "agentcoder")
	}
}
