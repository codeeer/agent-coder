package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * "Her zaman izinli" listesi GERÇEK yapılandırmadan gelir (spec 020 H4).
 *
 * Uydurulmuş bir örnek liste göstermek, kullanıcının bilmediği bir kapının
 * açık olduğunu gizlerdi — kuralın tam tersi.
 */
func TestHostlariTekille_AdresteYalnizcaHostKalir(t *testing.T) {
	hostlar := hostlariTekille([]string{
		"https://openrouter.ai/api/v1",
		"https://nexus.sirket.local:8081/repository/npm/",
	})

	require.Equal(t, []string{"openrouter.ai", "nexus.sirket.local"}, hostlar)
}

func TestHostlariTekille_YinelenenVeBosAtlanir(t *testing.T) {
	hostlar := hostlariTekille([]string{
		"https://ayni.local/a", "https://ayni.local/b", "", "bu adres değil",
	})

	require.Equal(t, []string{"ayni.local"}, hostlar)
}

// Proxy tanımlı değilken kaynak "none" olmalı: arayüz "denetim kapalı"
// diyebilmek için bunu bilmek zorunda.
func TestEgress_ProxyTanimsizkenKaynakNone(t *testing.T) {
	h := &Handler{deps: Deps{EngineHosts: func() []string { return []string{"models.opencode.ai"} }}}

	rec := httptest.NewRecorder()
	h.egressStatus(rec, httptest.NewRequest(http.MethodGet, "/api/network/egress", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var out egressResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, "none", out.Proxy.Source)
	require.Equal(t, []string{"models.opencode.ai"}, out.AlwaysAllowed.Engine,
		"motorun kendi adresleri denetim kapalıyken de bildirilir")
}

// Ortam değişkeninden gelen proxy "env" olarak bildirilir — sertifikadaki
// kalıbın aynısı: kullanıcı hangi kaynağın geçerli olduğunu görmeli.
func TestEgress_OrtamDegiskeniKaynagiBildirilir(t *testing.T) {
	h := &Handler{deps: Deps{
		EgressProxyEnv: "http://proxy.local:8080",
		EngineHosts:    func() []string { return nil },
	}}

	rec := httptest.NewRecorder()
	h.egressStatus(rec, httptest.NewRequest(http.MethodGet, "/api/network/egress", nil))

	var out egressResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, "env", out.Proxy.Source)
	require.Equal(t, "proxy.local:8080", out.Proxy.Host)
}
