package credentials

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	// ErrInvalidSecret, kimlik bilgisi servis tarafından reddedildi.
	ErrInvalidSecret = errors.New("kimlik bilgisi doğrulanamadı")

	// ErrServiceUnreachable, doğrulama yapılamadı — servise ulaşılamıyor.
	// Kullanıcıya "değer yanlış" demek yerine "şu an denenemedi" demek için ayrı.
	ErrServiceUnreachable = errors.New("doğrulama servisine ulaşılamıyor")

	// ErrMissingMetadata, tür için gerekli ek alanlar eksik.
	ErrMissingMetadata = errors.New("zorunlu alan eksik")
)

// Validator, kimlik bilgisinin gerçekten çalıştığını sınar.
//
// Spec 002'den sonra bu paket yalnızca Jira ile ilgilenir; LLM sağlayıcılar
// internal/llm, kod deposu erişimleri internal/gitprovider altındadır.
type Validator struct {
	http *http.Client
}

// NewValidator yeni doğrulayıcı üretir.
func NewValidator() *Validator {
	return &Validator{http: &http.Client{Timeout: 15 * time.Second}}
}

/*
 * Validate, türe uygun doğrulamayı çalıştırır.
 *
 * HER TÜRÜN DOĞRULANABİLİR BİR UCU YOKTUR. Jira'nın `myself` ucu var ve
 * anahtarın gerçekten çalıştığını söylüyor; paket deposu token'ı için böyle
 * bir uç yok — Nexus kurulumları farklı yollar sunuyor ve yanlış tahmin
 * edilen bir uç, çalışan bir token'ı reddederdi. Doğrulanamayan tür sessizce
 * kabul edilir; hata ilk koşuda npm'in kendi mesajıyla görünür.
 */
func (v *Validator) Validate(ctx context.Context, kind Kind, secret string, meta map[string]string) error {
	switch kind {
	case KindJira:
		return v.validateJira(ctx, secret, meta)
	case KindNexus:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrInvalidKind, kind)
	}
}

func (v *Validator) validateJira(ctx context.Context, secret string, meta map[string]string) error {
	baseURL := strings.TrimSuffix(strings.TrimSpace(meta["base_url"]), "/")
	email := strings.TrimSpace(meta["email"])

	if baseURL == "" {
		return fmt.Errorf("%w: base_url", ErrMissingMetadata)
	}
	if email == "" {
		return fmt.Errorf("%w: email", ErrMissingMetadata)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/rest/api/3/myself", nil)
	if err != nil {
		return fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	// Jira Cloud: e-posta kullanıcı adı, API token parola.
	req.SetBasicAuth(email, secret)
	req.Header.Set("Accept", "application/json")

	resp, err := v.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrServiceUnreachable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return ErrInvalidSecret
	case resp.StatusCode == http.StatusNotFound:
		// Yanlış base_url veya bu ucu sunmayan bir sunucu.
		return fmt.Errorf("%w: adres bulunamadı (404)", ErrInvalidSecret)
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: sunucu %d döndü", ErrServiceUnreachable, resp.StatusCode)
	default:
		return fmt.Errorf("%w: beklenmeyen durum %d", ErrServiceUnreachable, resp.StatusCode)
	}
}
