package gitprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/secrets"
)

// Store, git sağlayıcı deposu.
type Store struct {
	pool   *pgxpool.Pool
	cipher *secrets.Cipher
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool, cipher *secrets.Cipher) *Store {
	return &Store{pool: pool, cipher: cipher}
}

const providerColumns = `id, type, name, base_url, username, hint, created_at, updated_at`

func scanProvider(row pgx.Row) (Provider, error) {
	var p Provider
	err := row.Scan(&p.ID, &p.Type, &p.Name, &p.BaseURL, &p.Username, &p.Hint,
		&p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// List, tanımlı tüm git erişimlerini döner.
func (s *Store) List(ctx context.Context) ([]Provider, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+providerColumns+` FROM git_providers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("git sağlayıcılar listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Provider{}
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, fmt.Errorf("git sağlayıcı taranamadı: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("git sağlayıcılar okunamadı: %w", err)
	}
	return out, nil
}

// Get, tek bir git erişimini döner.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Provider, error) {
	p, err := scanProvider(s.pool.QueryRow(ctx,
		`SELECT `+providerColumns+` FROM git_providers WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Provider{}, ErrNotFound
	}
	if err != nil {
		return Provider{}, fmt.Errorf("git sağlayıcı okunamadı: %w", err)
	}
	return p, nil
}

// Reveal, git erişiminin gizli değerini çözer.
func (s *Store) Reveal(ctx context.Context, id uuid.UUID) (string, error) {
	var blob []byte
	err := s.pool.QueryRow(ctx, `SELECT secret_enc FROM git_providers WHERE id = $1`, id).Scan(&blob)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("git anahtarı okunamadı: %w", err)
	}

	secret, err := s.cipher.DecryptString(blob)
	if err != nil {
		return "", fmt.Errorf("git anahtarı çözülemedi (şifreleme anahtarı değişmiş olabilir): %w", err)
	}
	return secret, nil
}

// CreateInput, yeni git erişimi için gerekli alanlar.
type CreateInput struct {
	Type     Type
	Name     string
	BaseURL  string
	Username string
	Secret   string
}

// Create, yeni git erişimi kaydeder.
func (s *Store) Create(ctx context.Context, in CreateInput) (Provider, error) {
	name, baseURL, username, err := Validate(in.Type, in.Name, in.BaseURL, in.Username)
	if err != nil {
		return Provider{}, err
	}
	if in.Secret == "" {
		return Provider{}, errors.New("erişim bilgisi boş olamaz")
	}

	blob, err := s.cipher.EncryptString(in.Secret)
	if err != nil {
		return Provider{}, fmt.Errorf("erişim bilgisi şifrelenemedi: %w", err)
	}

	p, err := scanProvider(s.pool.QueryRow(ctx, `
		INSERT INTO git_providers (type, name, base_url, username, secret_enc, hint)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+providerColumns,
		string(in.Type), name, baseURL, username, blob, secrets.Mask(in.Secret)))
	if err != nil {
		return Provider{}, fmt.Errorf("git sağlayıcı kaydedilemedi: %w", err)
	}
	return p, nil
}

// UpdateInput, güncellenebilir alanlar. nil olanlar değiştirilmez.
type UpdateInput struct {
	Name     *string
	BaseURL  *string
	Username *string
	Secret   *string // boş veya nil ise mevcut değer korunur
}

// Update, mevcut git erişimini günceller.
func (s *Store) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Provider, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Provider{}, fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanProvider(tx.QueryRow(ctx,
		`SELECT `+providerColumns+` FROM git_providers WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Provider{}, ErrNotFound
	}
	if err != nil {
		return Provider{}, fmt.Errorf("git sağlayıcı okunamadı: %w", err)
	}

	name := valueOr(in.Name, current.Name)
	baseURL := valueOr(in.BaseURL, current.BaseURL)
	username := valueOr(in.Username, current.Username)

	name, baseURL, username, err = Validate(current.Type, name, baseURL, username)
	if err != nil {
		return Provider{}, err
	}

	if in.Secret != nil && *in.Secret != "" {
		blob, err := s.cipher.EncryptString(*in.Secret)
		if err != nil {
			return Provider{}, fmt.Errorf("erişim bilgisi şifrelenemedi: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE git_providers SET secret_enc = $2, hint = $3 WHERE id = $1`,
			id, blob, secrets.Mask(*in.Secret)); err != nil {
			return Provider{}, fmt.Errorf("erişim bilgisi güncellenemedi: %w", err)
		}
	}

	p, err := scanProvider(tx.QueryRow(ctx, `
		UPDATE git_providers
		SET name = $2, base_url = $3, username = $4, updated_at = now()
		WHERE id = $1
		RETURNING `+providerColumns,
		id, name, baseURL, username))
	if err != nil {
		return Provider{}, fmt.Errorf("git sağlayıcı güncellenemedi: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Provider{}, fmt.Errorf("transaction tamamlanamadı: %w", err)
	}
	return p, nil
}

// Delete, git erişimini siler.
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM git_providers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("git sağlayıcı silinemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func valueOr(v *string, def string) string {
	if v == nil {
		return def
	}
	return *v
}
