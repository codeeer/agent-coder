package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/mcp"
)

func srv(name, url string) mcp.Server {
	return mcp.Server{Name: name, Transport: mcp.TransportHTTP, URL: url}
}

func TestValidate_GecerliSunucu(t *testing.T) {
	require.NoError(t, srv("sentry", "https://mcp.sentry.dev/mcp").Validate())
}

// TestValidate_AdAracAdlandirmasinaUygunOlmali — ad, araç adlarının önekidir
// (`sentry_issue`). Çalıştırma motoru izin verilmeyen karakterleri alt çizgiye
// çeviriyor; baştan reddediyoruz ki kullanıcının yazdığı ad ile modelin gördüğü
// araç adı AYNI olsun.
func TestValidate_AdAracAdlandirmasinaUygunOlmali(t *testing.T) {
	for _, ad := range []string{"my.server", "boşluk var", "tırnak'lı", "slash/li"} {
		require.ErrorIs(t, srv(ad, "https://x.dev/mcp").Validate(), mcp.ErrInvalidName, ad)
	}
	for _, ad := range []string{"sentry", "my-server", "my_server", "db2"} {
		require.NoError(t, srv(ad, "https://x.dev/mcp").Validate(), ad)
	}
}

func TestValidate_EksikAlanlar(t *testing.T) {
	require.ErrorIs(t, srv("", "https://x.dev").Validate(), mcp.ErrMissingName)
	require.ErrorIs(t, srv("a", "").Validate(), mcp.ErrMissingURL)
	require.ErrorIs(t, mcp.Server{Name: "a", Transport: "stdio", URL: "https://x.dev"}.Validate(),
		mcp.ErrInvalidTransport, "yerel sunucular bu fazda desteklenmiyor")
}

// TestValidate_AdresSemasi — `stdio://` veya dosya yolu yazan kullanıcı,
// hatayı kaydetme anında görmeli; çalıştırma anında değil.
func TestValidate_AdresSemasi(t *testing.T) {
	for _, u := range []string{"ftp://x.dev", "/usr/local/bin/mcp", "mcp.sentry.dev"} {
		require.ErrorIs(t, srv("a", u).Validate(), mcp.ErrInvalidURL, u)
	}
}
