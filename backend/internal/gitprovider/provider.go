// Package gitprovider, kod deposu erişim bilgilerini yönetir.
//
// GitHub tek bir token ile çalışırken Bitbucket kullanıcı adı + parola çifti
// ister. Bu fark tür başına doğrulama ve alan kuralarıyla ele alınır.
package gitprovider

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Type, git sağlayıcı türü. Veritabanındaki git_provider_type enum'u ile eşleşir.
type Type string

const (
	TypeGitHub    Type = "github"
	TypeBitbucket Type = "bitbucket"
	TypeGeneric   Type = "generic"
)

// AllTypes, desteklenen tüm türler.
var AllTypes = []Type{TypeGitHub, TypeBitbucket, TypeGeneric}

// Valid, türün bilinen bir değer olup olmadığını söyler.
func (t Type) Valid() bool {
	for _, v := range AllTypes {
		if t == v {
			return true
		}
	}
	return false
}

// RequiresUsername, türün kullanıcı adı isteyip istemediği.
//
// GitHub token'ı tek başına yeterlidir; Bitbucket app password kullanıcı adıyla
// birlikte kullanılır; genel Git'te de kullanıcı adı gerekir.
func (t Type) RequiresUsername() bool {
	return t == TypeBitbucket || t == TypeGeneric
}

// RequiresBaseURL, türün adres isteyip istemediği.
// GitHub ve Bitbucket'ın bulut adresleri varsayılandır; genel Git'te zorunlu.
func (t Type) RequiresBaseURL() bool {
	return t == TypeGeneric
}

// DefaultAPIURL, türün bulut API adresi.
func (t Type) DefaultAPIURL() string {
	switch t {
	case TypeGitHub:
		return "https://api.github.com"
	case TypeBitbucket:
		return "https://api.bitbucket.org/2.0"
	default:
		return ""
	}
}

var (
	ErrInvalidType     = errors.New("geçersiz git sağlayıcı türü")
	ErrMissingUsername = errors.New("kullanıcı adı zorunlu")
	ErrMissingBaseURL  = errors.New("servis adresi zorunlu")
	ErrInvalidBaseURL  = errors.New("servis adresi geçersiz")
	ErrEmptyName       = errors.New("sağlayıcı adı boş olamaz")
	ErrNotFound        = errors.New("git sağlayıcı bulunamadı")

	// ErrInvalidSecret, sağlayıcı erişim bilgisini reddetti.
	ErrInvalidSecret = errors.New("erişim bilgisi doğrulanamadı")

	// ErrUnreachable, doğrulama yapılamadı — servise ulaşılamıyor.
	ErrUnreachable = errors.New("servise ulaşılamıyor")

	// ErrNotVerifiable, bu tür için doğrulama mümkün değil.
	// Kayıt yine de yapılır; kullanıcı uyarılır (spec 002 H5).
	ErrNotVerifiable = errors.New("bu tür için doğrulama yapılamıyor")
)

// Provider, tanımlı bir git erişimi. Gizli değeri TAŞIMAZ.
type Provider struct {
	ID        uuid.UUID `json:"id"`
	Type      Type      `json:"type"`
	Name      string    `json:"name"`
	BaseURL   string    `json:"baseUrl"`
	Username  string    `json:"username"`
	Hint      string    `json:"hint"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// APIURL, doğrulama isteklerinin gideceği adres.
func (p Provider) APIURL() string {
	if p.BaseURL != "" {
		return strings.TrimRight(p.BaseURL, "/")
	}
	return p.Type.DefaultAPIURL()
}

// Validate, tür başına zorunlu alanların doldurulduğunu doğrular.
//
// Adres normalize edilmiş haliyle geri döner; boş bırakılabildiği durumlarda
// boş kalır ve çalışma anında türün varsayılanı kullanılır.
func Validate(t Type, name, baseURL, username string) (normalizedName, normalizedURL, normalizedUser string, err error) {
	if !t.Valid() {
		return "", "", "", fmt.Errorf("%w: %q", ErrInvalidType, t)
	}

	normalizedName = strings.TrimSpace(name)
	if normalizedName == "" {
		return "", "", "", ErrEmptyName
	}

	normalizedUser = strings.TrimSpace(username)
	if t.RequiresUsername() && normalizedUser == "" {
		return "", "", "", ErrMissingUsername
	}

	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		if t.RequiresBaseURL() {
			return "", "", "", ErrMissingBaseURL
		}
		return normalizedName, "", normalizedUser, nil
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: ayrıştırılamadı", ErrInvalidBaseURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", "", fmt.Errorf("%w: http:// veya https:// ile başlamalı", ErrInvalidBaseURL)
	}
	if u.Host == "" {
		return "", "", "", fmt.Errorf("%w: sunucu adı yok", ErrInvalidBaseURL)
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")

	return normalizedName, u.String(), normalizedUser, nil
}
