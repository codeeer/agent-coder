// Package mcpserver, Agent Coder'ı bir MCP sunucusu olarak dışarıya açar.
//
// Yön TERSTİR: `internal/mcp` bizim dış sunuculara bağlanmamızı sağlar; bu paket
// başkalarının BİZE bağlanmasını. Claude Desktop, Cursor veya başka bir agent
// buradaki akışları listeleyip başlatabilir.
//
// Yeni bir başlatma yolu AÇILMIYOR: her şey mevcut `workflow.Launcher`'dan
// geçiyor — elle, webhook ve Jira tetiklemesiyle aynı kapı. Ayrı bir yol,
// dördüncü bir hata modeli ve dördüncü bir "akış pasif mi" kontrolü demekti.
package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Access, erişim adresinin gizli parçasını yönetir.
//
// Kimlik doğrulama v1'de yok; adresin kendisi anahtardır (webhook deseni).
type Access struct {
	pool *pgxpool.Pool
}

// NewAccess yeni erişim yöneticisi üretir.
func NewAccess(pool *pgxpool.Pool) *Access { return &Access{pool: pool} }

// Token, kayıtlı anahtarı döner; yoksa üretip kaydeder.
//
// İlk açılışta kendiliğinden üretiliyor: kullanıcının "önce anahtar oluştur"
// diye bir adım atması gerekmesin. Adres zaten gösterilmeden işe yaramaz.
func (a *Access) Token(ctx context.Context) (string, error) {
	var token string
	err := a.pool.QueryRow(ctx, `SELECT token FROM mcp_access WHERE only_row`).Scan(&token)
	if err == nil {
		return token, nil
	}
	if err != pgx.ErrNoRows {
		return "", fmt.Errorf("MCP erişim anahtarı okunamadı: %w", err)
	}

	fresh, err := newToken()
	if err != nil {
		return "", err
	}
	// Eşzamanlı iki istek aynı anda üretebilir; çakışmada kazananın değeri
	// okunur, ikinci bir anahtar oluşmaz.
	err = a.pool.QueryRow(ctx, `
		INSERT INTO mcp_access (token) VALUES ($1)
		ON CONFLICT (only_row) DO UPDATE SET token = mcp_access.token
		RETURNING token`, fresh).Scan(&token)
	if err != nil {
		return "", fmt.Errorf("MCP erişim anahtarı kaydedilemedi: %w", err)
	}
	return token, nil
}

// Rotate, adresi yeniler. Eski adres anında geçersiz olur.
func (a *Access) Rotate(ctx context.Context) (string, error) {
	fresh, err := newToken()
	if err != nil {
		return "", err
	}

	var token string
	err = a.pool.QueryRow(ctx, `
		INSERT INTO mcp_access (token) VALUES ($1)
		ON CONFLICT (only_row) DO UPDATE SET token = $1, updated_at = now()
		RETURNING token`, fresh).Scan(&token)
	if err != nil {
		return "", fmt.Errorf("MCP erişim anahtarı yenilenemedi: %w", err)
	}
	return token, nil
}

// Valid, gelen anahtarın doğru olup olmadığı.
//
// Karşılaştırma SABİT ZAMANLI: normal karşılaştırma, ilk farklı bayta kadar
// geçen süreyle anahtarı harf harf tahmin etmeye açık kapı bırakır.
func (a *Access) Valid(ctx context.Context, candidate string) bool {
	if candidate == "" {
		return false
	}
	token, err := a.Token(ctx)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(candidate)) == 1
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("erişim anahtarı üretilemedi: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
