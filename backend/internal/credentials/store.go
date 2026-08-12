// Package credentials, Jira kimlik bilgisini şifreli saklar.
//
// Spec 002 öncesinde LLM ve git erişimleri de buradaydı; artık internal/llm ve
// internal/gitprovider altındalar.
//
// Gizli değerler yalnızca Reveal ile okunur. Dışarı verilen Credential tipi gizli
// değeri hiç taşımaz — bir handler yanlışlıkla onu döndürse bile anahtar sızmaz.
package credentials

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/secrets"
)

// Kind, kimlik bilgisi türü. Veritabanındaki credential_kind enum'u ile eşleşir.
type Kind string

const (
	// KindJira, spec 002'den sonra bu paketin ilgilendiği ilk tür.
	// LLM sağlayıcılar internal/llm, kod deposu erişimleri internal/gitprovider
	// altına taşındı; veritabanı enum'unda eski değerler durur ama kullanılmaz.
	KindJira Kind = "jira"

	// KindNexus, kurumsal paket deposunun token'ı.
	//
	// Adres ve kullanıcı adı ayarlarda (packages.npm_*); burada YALNIZCA sır
	// durur. Kimlik doğrulama opsiyonel olduğu için kayıt hiç olmayabilir.
	KindNexus Kind = "nexus"
)

// AllKinds, bu paketin yönettiği türler.
var AllKinds = []Kind{KindJira, KindNexus}

// Valid, türün bilinen bir değer olup olmadığını söyler.
func (k Kind) Valid() bool {
	for _, v := range AllKinds {
		if k == v {
			return true
		}
	}
	return false
}

var (
	// ErrNotConfigured, istenen türde kayıt yok.
	ErrNotConfigured = errors.New("kimlik bilgisi tanımlı değil")

	// ErrInvalidKind, desteklenmeyen tür.
	ErrInvalidKind = errors.New("geçersiz kimlik bilgisi türü")
)

// Credential, bir kimlik bilgisinin dışarı verilebilir hali.
//
// Gizli değeri BİLİNÇLİ OLARAK içermez. Yeni alan eklerken bu kuralı bozmayın.
type Credential struct {
	Kind      Kind              `json:"kind"`
	Hint      string            `json:"hint"`     // son 4 karakter, örn. "fd36"
	Metadata  map[string]string `json:"metadata"` // jira: base_url, email
	UpdatedAt time.Time         `json:"updatedAt"`
}

// Store, kimlik bilgisi deposu.
type Store struct {
	pool   *pgxpool.Pool
	cipher *secrets.Cipher
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool, cipher *secrets.Cipher) *Store {
	return &Store{pool: pool, cipher: cipher}
}

// List, kayıtlı tüm kimlik bilgilerini gizli değer olmadan döner.
func (s *Store) List(ctx context.Context) ([]Credential, error) {
	const q = `
		SELECT kind, hint, metadata, updated_at
		FROM credentials
		ORDER BY kind`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("kimlik bilgileri listelenemedi: %w", err)
	}
	defer rows.Close()

	var out []Credential
	for rows.Next() {
		var (
			c       Credential
			rawMeta []byte
		)
		if err := rows.Scan(&c.Kind, &c.Hint, &rawMeta, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("kimlik bilgisi taranamadı: %w", err)
		}
		if err := json.Unmarshal(rawMeta, &c.Metadata); err != nil {
			return nil, fmt.Errorf("metadata ayrıştırılamadı: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("kimlik bilgileri okunamadı: %w", err)
	}
	return out, nil
}

// Get, tek bir kimlik bilgisini gizli değer olmadan döner.
func (s *Store) Get(ctx context.Context, kind Kind) (*Credential, error) {
	if !kind.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrInvalidKind, kind)
	}

	const q = `SELECT kind, hint, metadata, updated_at FROM credentials WHERE kind = $1`

	var (
		c       Credential
		rawMeta []byte
	)
	err := s.pool.QueryRow(ctx, q, string(kind)).Scan(&c.Kind, &c.Hint, &rawMeta, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrNotConfigured, kind)
	}
	if err != nil {
		return nil, fmt.Errorf("kimlik bilgisi okunamadı: %w", err)
	}
	if err := json.Unmarshal(rawMeta, &c.Metadata); err != nil {
		return nil, fmt.Errorf("metadata ayrıştırılamadı: %w", err)
	}
	return &c, nil
}

// Reveal, gizli değeri çözer.
//
// Adı bilinçli olarak dikkat çekicidir: çağrı yerini okuyan biri gizli değere
// erişildiğini görmeli. Dönen değer loglanmaz ve HTTP yanıtına konmaz.
func (s *Store) Reveal(ctx context.Context, kind Kind) (secret string, meta map[string]string, err error) {
	if !kind.Valid() {
		return "", nil, fmt.Errorf("%w: %q", ErrInvalidKind, kind)
	}

	const q = `SELECT secret_enc, metadata FROM credentials WHERE kind = $1`

	var (
		blob    []byte
		rawMeta []byte
	)
	err = s.pool.QueryRow(ctx, q, string(kind)).Scan(&blob, &rawMeta)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, fmt.Errorf("%w: %s", ErrNotConfigured, kind)
	}
	if err != nil {
		return "", nil, fmt.Errorf("kimlik bilgisi okunamadı: %w", err)
	}

	secret, err = s.cipher.DecryptString(blob)
	if err != nil {
		// Şifreleme anahtarı değişmiş olabilir. Hata mesajı gizli değer
		// hakkında hiçbir şey söylemez.
		return "", nil, fmt.Errorf("%s kimlik bilgisi çözülemedi (şifreleme anahtarı değişmiş olabilir): %w", kind, err)
	}

	if err := json.Unmarshal(rawMeta, &meta); err != nil {
		return "", nil, fmt.Errorf("metadata ayrıştırılamadı: %w", err)
	}
	return secret, meta, nil
}

// Put, kimlik bilgisini kaydeder veya mevcut olanı değiştirir.
//
// Tür başına tek kayıt tutulur (spec 001) — ikinci kez kaydetmek yeni satır
// oluşturmaz, mevcudu günceller.
func (s *Store) Put(ctx context.Context, kind Kind, secret string, meta map[string]string) error {
	if !kind.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidKind, kind)
	}
	if secret == "" {
		return errors.New("gizli değer boş olamaz")
	}

	blob, err := s.cipher.EncryptString(secret)
	if err != nil {
		return fmt.Errorf("kimlik bilgisi şifrelenemedi: %w", err)
	}

	if meta == nil {
		meta = map[string]string{}
	}
	rawMeta, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("metadata kodlanamadı: %w", err)
	}

	const q = `
		INSERT INTO credentials (kind, secret_enc, hint, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, now(), now())
		ON CONFLICT (kind) DO UPDATE SET
			secret_enc = EXCLUDED.secret_enc,
			hint       = EXCLUDED.hint,
			metadata   = EXCLUDED.metadata,
			updated_at = now()`

	if _, err := s.pool.Exec(ctx, q, string(kind), blob, secrets.Mask(secret), rawMeta); err != nil {
		return fmt.Errorf("kimlik bilgisi kaydedilemedi: %w", err)
	}
	return nil
}

// Delete, kimlik bilgisini siler. Kayıt yoksa ErrNotConfigured döner.
func (s *Store) Delete(ctx context.Context, kind Kind) error {
	if !kind.Valid() {
		return fmt.Errorf("%w: %q", ErrInvalidKind, kind)
	}

	tag, err := s.pool.Exec(ctx, `DELETE FROM credentials WHERE kind = $1`, string(kind))
	if err != nil {
		return fmt.Errorf("kimlik bilgisi silinemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %s", ErrNotConfigured, kind)
	}
	return nil
}
