package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// clientName, uzak sunucuya kendimizi tanıttığımız ad.
const clientName = "agent-coder"

// Client, MCP sunucularına bağlanır.
//
// NEDEN VAR: agent'ın araçları çağırması için buna gerek yok — onu çalıştırma
// motoru kendi içindeki MCP istemcisiyle yapıyor. Bu istemci BİZİM için:
// kullanıcı bir sunucu tanımlarken gerçekten bağlanılabildiğini ve hangi
// araçları sunduğunu kaydetmeden önce göstermek zorundayız (spec 011 K3).
type Client struct {
	timeout func() time.Duration
}

// NewClient yeni istemci üretir.
//
// Zaman aşımı fonksiyon olarak alınır: ayar değişikliği sunucuyu yeniden
// başlatmayı gerektirmemeli (AGENTS.md, ayarlar kuralı).
func NewClient(timeout func() time.Duration) *Client {
	return &Client{timeout: timeout}
}

// ListTools, sunucuya bağlanır ve sunduğu araç adlarını döner.
//
// Dönen adlar sunucunun kendi adlandırmasıdır (`issue`), modelin göreceği hali
// değil (`sentry_issue`) — önek çalıştırma motoru tarafından eklenir.
func (c *Client) ListTools(ctx context.Context, s Server, secret string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()

	session, err := c.connect(ctx, s, secret)
	if err != nil {
		return nil, err
	}
	defer func() { _ = session.Close() }()

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: araç listesi alınamadı: %v", ErrUnreachable, err)
	}

	names := make([]string, 0, len(res.Tools))
	for _, t := range res.Tools {
		names = append(names, t.Name)
	}
	// Sıralı: liste her doğrulamada aynı sırada görünsün, kullanıcı "değişti mi"
	// sorusunu sırasız bir listeye bakarak sormasın.
	sort.Strings(names)
	return names, nil
}

/*
 * CallTool, bir aracı çağırır ve metin sonucunu döner.
 *
 * Akış düğümü (spec 011 Aşama 2) bunu kullanır: agent'ın kararına bırakmadan,
 * akışın belirlediği aracı belirlediği argümanlarla çağırmak.
 *
 * Sonuç METİN olarak döner çünkü akıştaki bir sonraki adım agent ise girdisi
 * zaten metindir. Yapılandırılmış sonuç varsa JSON'a çevrilir — bilgi kaybı
 * olmaz ama tek bir tip taşınır.
 */
func (c *Client) CallTool(ctx context.Context, s Server, secret, tool string, args map[string]any) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()

	session, err := c.connect(ctx, s, secret)
	if err != nil {
		return "", err
	}
	defer func() { _ = session.Close() }()

	res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return "", fmt.Errorf("%w: %q aracı çağrılamadı: %v", ErrToolFailed, tool, err)
	}

	text := resultText(res)

	// Aracın KENDİSİ hata bildirdiyse (protokol düzeyinde başarı, iş düzeyinde
	// hata) adım başarısız olmalı: yoksa akış hata metnini bir sonraki adıma
	// veri diye taşırdı.
	if res.IsError {
		return "", fmt.Errorf("%w: %q aracı hata döndürdü: %s", ErrToolFailed, tool, text)
	}
	return text, nil
}

// resultText, araç sonucunu tek bir metne indirger.
func resultText(res *sdk.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*sdk.TextContent); ok && t.Text != "" {
			parts = append(parts, t.Text)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, "\n")
	}

	// Metin içerik yoksa yapılandırılmış sonuç JSON olarak taşınır.
	if res.StructuredContent != nil {
		if b, err := json.Marshal(res.StructuredContent); err == nil {
			return string(b)
		}
	}
	return ""
}

func (c *Client) connect(ctx context.Context, s Server, secret string) (*sdk.ClientSession, error) {
	httpClient := &http.Client{
		Timeout:   c.timeout(),
		Transport: authTransport{secret: secret, base: http.DefaultTransport},
	}

	var transport sdk.Transport
	switch s.Transport {
	case TransportSSE:
		transport = &sdk.SSEClientTransport{Endpoint: s.URL, HTTPClient: httpClient}
	case TransportHTTP:
		transport = &sdk.StreamableClientTransport{
			Endpoint:   s.URL,
			HTTPClient: httpClient,
			// Yeniden bağlanma denemesi yok: burada tek seferlik bir doğrulama
			// yapıyoruz, kullanıcı kaydet düğmesinin altında bekliyor.
			MaxRetries: -1,
			// Sunucudan gelen kendiliğinden bildirimleri dinlemiyoruz; araç
			// listesini alıp kapatıyoruz.
			DisableStandaloneSSE: true,
		}
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidTransport, s.Transport)
	}

	client := sdk.NewClient(&sdk.Implementation{Name: clientName}, nil)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	return session, nil
}

// authTransport, isteklere erişim başlığını ekler.
//
// Anahtar yalnızca burada, bellekte kullanılır; hiçbir dosyaya yazılmaz.
type authTransport struct {
	secret string
	base   http.RoundTripper
}

func (t authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.secret != "" {
		// İstek KOPYALANIR: RoundTripper'ın kendisine verilen isteği
		// değiştirmesi yasak (net/http sözleşmesi).
		clone := req.Clone(req.Context())
		clone.Header.Set("Authorization", "Bearer "+t.secret)
		req = clone
	}
	return t.base.RoundTrip(req)
}
