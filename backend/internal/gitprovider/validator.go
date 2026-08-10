package gitprovider

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Validator, erişim bilgisinin gerçekten çalıştığını sınar.
type Validator struct {
	http *http.Client
}

// NewValidator yeni doğrulayıcı üretir.
func NewValidator() *Validator {
	return &Validator{http: &http.Client{Timeout: 15 * time.Second}}
}

// Validate, türe uygun doğrulama isteğini atar.
//
// Genel Git türünde doğrulama mümkün değildir: elimizde yalnızca bir depo
// adresi ve kimlik bilgisi vardır, sorgulanacak standart bir API ucu yoktur.
// Bu durumda ErrNotVerifiable döner ve çağıran kaydı yine de yapar (spec 002 H5).
func (v *Validator) Validate(ctx context.Context, p Provider, secret string) error {
	switch p.Type {
	case TypeGitHub:
		return v.validateGitHub(ctx, p, secret)
	case TypeBitbucket:
		return v.validateBitbucket(ctx, p, secret)
	case TypeGeneric:
		return ErrNotVerifiable
	default:
		return fmt.Errorf("%w: %q", ErrInvalidType, p.Type)
	}
}

func (v *Validator) validateGitHub(ctx context.Context, p Provider, secret string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.APIURL()+"/user", nil)
	if err != nil {
		return fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Accept", "application/vnd.github+json")

	return v.check(req)
}

func (v *Validator) validateBitbucket(ctx context.Context, p Provider, secret string) error {
	if p.Username == "" {
		return ErrMissingUsername
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.APIURL()+"/user", nil)
	if err != nil {
		return fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	// Bitbucket app password: kullanıcı adı + parola çifti.
	req.SetBasicAuth(p.Username, secret)
	req.Header.Set("Accept", "application/json")

	return v.check(req)
}

// check, doğrulama isteğinin sonucunu ortak kurallara göre yorumlar.
func (v *Validator) check(req *http.Request) error {
	resp, err := v.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return ErrInvalidSecret
	case resp.StatusCode == http.StatusNotFound:
		// Yanlış adres veya bu ucu sunmayan bir sunucu.
		return fmt.Errorf("%w: adres bulunamadı (404)", ErrInvalidSecret)
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: sunucu %d döndü", ErrUnreachable, resp.StatusCode)
	default:
		return fmt.Errorf("%w: beklenmeyen durum %d", ErrUnreachable, resp.StatusCode)
	}
}
