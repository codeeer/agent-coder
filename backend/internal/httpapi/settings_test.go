package httpapi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/settings"
)

/*
 * Uzun ayar değerleri loga ÖZETLE yazılır.
 *
 * Sertifikada ölçülmüştü: her kaydetmede ~2KB base64 loga düşünce o satırdan
 * sonrasını okumak imkânsızlaşıyordu. İzinli domain listesi aynı sorunu üretir
 * — kurumsal bir listede onlarca satır olabilir. Değerin kendisi sır değil,
 * ama logu kullanılamaz hale getirmesi de kabul edilemez.
 */
func TestLogDegeri_IzinliHostlarOzetlenir(t *testing.T) {
	uzun := strings.Repeat("ornek.com\n", 40)

	cikti := logDegeri(settings.KeyAllowedHosts, uzun)

	require.NotContains(t, cikti, "ornek.com\nornek.com", "liste ham hâliyle loglanmamalı")
	require.Contains(t, cikti, "40", "kaç domain olduğu yazılmalı")
}

func TestLogDegeri_BosIzinliHostlar(t *testing.T) {
	require.Contains(t, logDegeri(settings.KeyAllowedHosts, "  "), "temizlendi")
}

// Proxy adresi ÖZETLENMEZ: kısa, sır içeremiyor (doğrulama kimlik gömülü adresi
// reddediyor) ve "hangi proxy tanımlandı" sorusunun tek kaydı.
func TestLogDegeri_ProxyAdresiOlduguGibiLoglanir(t *testing.T) {
	adres := "http://proxy.sirket.local:8080"
	require.Equal(t, adres, logDegeri(settings.KeyEgressProxy, adres))
}
