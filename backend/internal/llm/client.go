package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

var (
	// ErrUnauthorized, anahtar reddedildi (401/403).
	ErrUnauthorized = errors.New("anahtar geçersiz")

	// ErrUnreachable, ağ hatası veya sunucu tarafı hata.
	// "Anahtarın yanlış" demek yerine "şu an denenemedi" demek için ayrı tutulur.
	ErrUnreachable = errors.New("adrese ulaşılamıyor")

	// ErrBadCatalog, servis yanıt verdi ama model listesi anlaşılamadı.
	ErrBadCatalog = errors.New("model listesi okunamadı")
)

// Client, bir sağlayıcının doğrulama ve katalog işlemleri.
// Tür başına ayrı uygulanır; çağıran hangi tür olduğunu bilmek zorunda değildir.
type Client interface {
	// Verify, anahtarın ve adresin çalıştığını doğrular.
	Verify(ctx context.Context, key string) error
	// ListModels, sağlayıcının sunduğu modelleri döner.
	ListModels(ctx context.Context, key string) ([]Model, error)
}

// NewClient, sağlayıcı türüne uygun istemciyi üretir.
func NewClient(p Provider) (Client, error) {
	base := p.BaseURL
	if fixed := p.Type.FixedBaseURL(); fixed != "" {
		base = fixed
	}

	http := newHTTPClient(30 * time.Second)

	switch p.Type {
	case TypeOpenRouter:
		return &openRouterClient{base: base, http: http}, nil
	case TypeLiteLLM:
		return &liteLLMClient{base: base, http: http}, nil
	case TypeOpenAICompatible:
		return &openAICompatClient{base: base, http: http}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrInvalidType, p.Type)
	}
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

// maxResponseBody, bozuk veya kötü niyetli bir yanıtın belleği tüketmesini engeller.
const maxResponseBody = 32 << 20 // 32 MiB

// getJSON, ortak istek/hata işleme yolu. Tüm adaptörler bunu kullanır ki
// durum kodu yorumlaması tek yerde kalsın.
func getJSON(ctx context.Context, c *http.Client, url, key string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		// Hata metni adresi içerebilir ama anahtar başlıkta olduğu için
		// sarmalamak güvenli.
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		// devam
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return ErrUnauthorized
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: sunucu %d döndü", ErrUnreachable, resp.StatusCode)
	default:
		return fmt.Errorf("%w: durum %d", ErrBadCatalog, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return fmt.Errorf("%w: yanıt okunamadı: %w", ErrUnreachable, err)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%w: yanıt ayrıştırılamadı: %w", ErrBadCatalog, err)
	}
	return nil
}

// parsePrice, metin fiyatı sayıya çevirir.
//
// Ayrıştırılamayan veya negatif değer 0 sayılır: tek bir bozuk alan yüzünden
// tüm katalog indirmesini düşürmek daha zararlı olurdu.
func parsePrice(s string) float64 {
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < 0 {
		return 0
	}
	return f
}

// ptr, işaretçi alanları doldurmak için küçük yardımcı.
func ptr[T any](v T) *T { return &v }
