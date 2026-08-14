package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/mcp"
)

// fakeServer, gerçek bir MCP sunucusu ayağa kaldırır.
//
// Elle JSON-RPC yazmak yerine SDK'nın sunucu tarafı kullanılıyor: protokolün
// el sıkışmasını taklit etmeye çalışmak, testin protokolü değil kendi
// varsayımımı doğrulaması demek olurdu.
func fakeServer(t *testing.T, tools []string, wantAuth string) *httptest.Server {
	t.Helper()

	srv := sdk.NewServer(&sdk.Implementation{Name: "sahte"}, nil)
	for _, name := range tools {
		sdk.AddTool(srv, &sdk.Tool{Name: name, Description: name + " aracı"},
			func(context.Context, *sdk.CallToolRequest, map[string]any) (
				*sdk.CallToolResult, map[string]any, error,
			) {
				return &sdk.CallToolResult{}, nil, nil
			})
	}

	handler := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return srv }, nil)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wantAuth != "" && r.Header.Get("Authorization") != "Bearer "+wantAuth {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}

func client() *mcp.Client {
	return mcp.NewClient(func() time.Duration { return 10 * time.Second }, nil)
}

func TestListTools_AraclarSirali(t *testing.T) {
	ts := fakeServer(t, []string{"zebra", "alpha", "mango"}, "")

	tools, err := client().ListTools(context.Background(),
		mcp.Server{Name: "sahte", Transport: mcp.TransportHTTP, URL: ts.URL}, "")

	require.NoError(t, err)
	require.Equal(t, []string{"alpha", "mango", "zebra"}, tools,
		"liste her doğrulamada aynı sırada görünmeli")
}

// TestListTools_ErisimBasligiGonderilir — anahtar isteğe başlık olarak eklenmeli;
// aksi halde kimlik doğrulayan hiçbir sunucu doğrulanamaz.
func TestListTools_ErisimBasligiGonderilir(t *testing.T) {
	ts := fakeServer(t, []string{"issue"}, "gizli-anahtar")

	tools, err := client().ListTools(context.Background(),
		mcp.Server{Name: "sentry", Transport: mcp.TransportHTTP, URL: ts.URL}, "gizli-anahtar")

	require.NoError(t, err)
	require.Equal(t, []string{"issue"}, tools)
}

func TestListTools_YanlisAnahtarHataVerir(t *testing.T) {
	ts := fakeServer(t, []string{"issue"}, "dogru-anahtar")

	_, err := client().ListTools(context.Background(),
		mcp.Server{Name: "sentry", Transport: mcp.TransportHTTP, URL: ts.URL}, "yanlis")

	require.ErrorIs(t, err, mcp.ErrUnreachable)
}

// TestListTools_UlasilamayanSunucu — kullanıcı kaydet düğmesinin altında
// bekliyor; bağlanılamayan bir sunucu sessizce kaydedilmemeli.
func TestListTools_UlasilamayanSunucu(t *testing.T) {
	_, err := client().ListTools(context.Background(),
		mcp.Server{Name: "yok", Transport: mcp.TransportHTTP, URL: "http://127.0.0.1:1"}, "")

	require.ErrorIs(t, err, mcp.ErrUnreachable)
}

// TestListTools_AnahtarHataMesajindaGecmez — sızıntı testi.
func TestListTools_AnahtarHataMesajindaGecmez(t *testing.T) {
	const secret = "sk-cok-gizli-bir-anahtar-1234"
	ts := fakeServer(t, []string{"issue"}, "baska-sey")

	_, err := client().ListTools(context.Background(),
		mcp.Server{Name: "sentry", Transport: mcp.TransportHTTP, URL: ts.URL}, secret)

	require.Error(t, err)
	require.NotContains(t, err.Error(), secret, "anahtar hata mesajında görünmemeli")
	require.NotContains(t, strings.ToLower(err.Error()), "bearer")
}
