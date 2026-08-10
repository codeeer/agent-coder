package llm

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/secrets"
)

// Store, LLM sağlayıcı deposu.
//
// Gizli değerler yalnızca Reveal ile okunur; Provider tipi onları taşımaz.
type Store struct {
	pool   *pgxpool.Pool
	cipher *secrets.Cipher
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool, cipher *secrets.Cipher) *Store {
	return &Store{pool: pool, cipher: cipher}
}

const providerColumns = `id, type, name, slug, base_url, hint, is_default, created_at, updated_at`

func scanProvider(row pgx.Row) (Provider, error) {
	var p Provider
	err := row.Scan(&p.ID, &p.Type, &p.Name, &p.Slug, &p.BaseURL, &p.Hint,
		&p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// List, tanımlı tüm sağlayıcıları döner. Varsayılan olan başta gelir.
func (s *Store) List(ctx context.Context) ([]Provider, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+providerColumns+` FROM llm_providers ORDER BY is_default DESC, name`)
	if err != nil {
		return nil, fmt.Errorf("sağlayıcılar listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Provider{}
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, fmt.Errorf("sağlayıcı taranamadı: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sağlayıcılar okunamadı: %w", err)
	}
	return out, nil
}

// Get, tek bir sağlayıcıyı döner.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Provider, error) {
	p, err := scanProvider(s.pool.QueryRow(ctx,
		`SELECT `+providerColumns+` FROM llm_providers WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Provider{}, ErrNotFound
	}
	if err != nil {
		return Provider{}, fmt.Errorf("sağlayıcı okunamadı: %w", err)
	}
	return p, nil
}

// Default, varsayılan sağlayıcıyı döner; yoksa ErrNotFound.
func (s *Store) Default(ctx context.Context) (Provider, error) {
	p, err := scanProvider(s.pool.QueryRow(ctx,
		`SELECT `+providerColumns+` FROM llm_providers WHERE is_default ORDER BY created_at LIMIT 1`))
	if errors.Is(err, pgx.ErrNoRows) {
		return Provider{}, ErrNotFound
	}
	if err != nil {
		return Provider{}, fmt.Errorf("varsayılan sağlayıcı okunamadı: %w", err)
	}
	return p, nil
}

// Reveal, sağlayıcının gizli değerini çözer.
//
// Adı bilinçli olarak dikkat çekicidir: çağrı yerini okuyan biri gizli değere
// erişildiğini görmeli. Dönen değer loglanmaz ve HTTP yanıtına konmaz.
func (s *Store) Reveal(ctx context.Context, id uuid.UUID) (string, error) {
	var blob []byte
	err := s.pool.QueryRow(ctx, `SELECT secret_enc FROM llm_providers WHERE id = $1`, id).Scan(&blob)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("sağlayıcı anahtarı okunamadı: %w", err)
	}

	secret, err := s.cipher.DecryptString(blob)
	if err != nil {
		return "", fmt.Errorf("sağlayıcı anahtarı çözülemedi (şifreleme anahtarı değişmiş olabilir): %w", err)
	}
	return secret, nil
}

// CreateInput, yeni sağlayıcı için gerekli alanlar.
type CreateInput struct {
	Type      Type
	Name      string
	BaseURL   string
	Secret    string
	IsDefault bool
}

// Create, yeni sağlayıcı kaydeder.
//
// İlk sağlayıcı kendiliğinden varsayılan olur (spec 002 H4).
func (s *Store) Create(ctx context.Context, in CreateInput) (Provider, error) {
	if !in.Type.Valid() {
		return Provider{}, fmt.Errorf("%w: %q", ErrInvalidType, in.Type)
	}
	name, err := ValidateName(in.Name)
	if err != nil {
		return Provider{}, err
	}
	baseURL, err := NormalizeBaseURL(in.Type, in.BaseURL)
	if err != nil {
		return Provider{}, err
	}
	if in.Secret == "" {
		return Provider{}, errors.New("anahtar boş olamaz")
	}

	blob, err := s.cipher.EncryptString(in.Secret)
	if err != nil {
		return Provider{}, fmt.Errorf("anahtar şifrelenemedi: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Provider{}, fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	slug, err := s.uniqueSlug(ctx, tx, Slugify(name))
	if err != nil {
		return Provider{}, err
	}

	// İlk sağlayıcı her hâlükârda varsayılan olur.
	var mevcut int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM llm_providers`).Scan(&mevcut); err != nil {
		return Provider{}, fmt.Errorf("sağlayıcı sayısı okunamadı: %w", err)
	}
	isDefault := in.IsDefault || mevcut == 0

	if isDefault {
		// Kısmi UNIQUE index ihlal edilmesin diye önce mevcut varsayılan düşürülür.
		if _, err := tx.Exec(ctx, `UPDATE llm_providers SET is_default = false WHERE is_default`); err != nil {
			return Provider{}, fmt.Errorf("varsayılan güncellenemedi: %w", err)
		}
	}

	p, err := scanProvider(tx.QueryRow(ctx, `
		INSERT INTO llm_providers (type, name, slug, base_url, secret_enc, hint, is_default)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING `+providerColumns,
		string(in.Type), name, slug, baseURL, blob, secrets.Mask(in.Secret), isDefault))
	if err != nil {
		return Provider{}, fmt.Errorf("sağlayıcı kaydedilemedi: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO provider_sync (provider_id) VALUES ($1)`, p.ID); err != nil {
		return Provider{}, fmt.Errorf("senkron kaydı oluşturulamadı: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Provider{}, fmt.Errorf("transaction tamamlanamadı: %w", err)
	}
	return p, nil
}

// UpdateInput, güncellenebilir alanlar. nil olanlar değiştirilmez.
type UpdateInput struct {
	Name      *string
	BaseURL   *string
	Secret    *string // boş veya nil ise mevcut anahtar korunur
	IsDefault *bool
}

// Update, mevcut sağlayıcıyı günceller.
func (s *Store) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Provider, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Provider{}, fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanProvider(tx.QueryRow(ctx,
		`SELECT `+providerColumns+` FROM llm_providers WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Provider{}, ErrNotFound
	}
	if err != nil {
		return Provider{}, fmt.Errorf("sağlayıcı okunamadı: %w", err)
	}

	name, slug := current.Name, current.Slug
	if in.Name != nil {
		if name, err = ValidateName(*in.Name); err != nil {
			return Provider{}, err
		}
		if name != current.Name {
			if slug, err = s.uniqueSlugExcept(ctx, tx, Slugify(name), id); err != nil {
				return Provider{}, err
			}
		}
	}

	baseURL := current.BaseURL
	if in.BaseURL != nil {
		if baseURL, err = NormalizeBaseURL(current.Type, *in.BaseURL); err != nil {
			return Provider{}, err
		}
	}

	if in.IsDefault != nil && *in.IsDefault {
		if _, err := tx.Exec(ctx,
			`UPDATE llm_providers SET is_default = false WHERE is_default AND id <> $1`, id); err != nil {
			return Provider{}, fmt.Errorf("varsayılan güncellenemedi: %w", err)
		}
	}
	isDefault := current.IsDefault
	if in.IsDefault != nil {
		isDefault = *in.IsDefault
	}

	// Anahtar yalnızca yeni bir değer verildiyse değişir; kullanıcı adını
	// değiştirmek için anahtarını tekrar girmek zorunda kalmaz.
	if in.Secret != nil && *in.Secret != "" {
		blob, err := s.cipher.EncryptString(*in.Secret)
		if err != nil {
			return Provider{}, fmt.Errorf("anahtar şifrelenemedi: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE llm_providers SET secret_enc = $2, hint = $3 WHERE id = $1`,
			id, blob, secrets.Mask(*in.Secret)); err != nil {
			return Provider{}, fmt.Errorf("anahtar güncellenemedi: %w", err)
		}
	}

	p, err := scanProvider(tx.QueryRow(ctx, `
		UPDATE llm_providers
		SET name = $2, slug = $3, base_url = $4, is_default = $5, updated_at = now()
		WHERE id = $1
		RETURNING `+providerColumns,
		id, name, slug, baseURL, isDefault))
	if err != nil {
		return Provider{}, fmt.Errorf("sağlayıcı güncellenemedi: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Provider{}, fmt.Errorf("transaction tamamlanamadı: %w", err)
	}
	return p, nil
}

// Delete, sağlayıcıyı ve ona ait modelleri siler (CASCADE).
//
// Silinen varsayılansa kalanlardan biri varsayılan olur (spec 002 H4).
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var wasDefault bool
	err = tx.QueryRow(ctx,
		`DELETE FROM llm_providers WHERE id = $1 RETURNING is_default`, id).Scan(&wasDefault)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("sağlayıcı silinemedi: %w", err)
	}

	if wasDefault {
		if _, err := tx.Exec(ctx, `
			UPDATE llm_providers SET is_default = true
			WHERE id = (SELECT id FROM llm_providers ORDER BY created_at LIMIT 1)`); err != nil {
			return fmt.Errorf("yeni varsayılan atanamadı: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// Count, tanımlı sağlayıcı sayısı. Bootstrap kararı için kullanılır.
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM llm_providers`).Scan(&n); err != nil {
		return 0, fmt.Errorf("sağlayıcı sayısı okunamadı: %w", err)
	}
	return n, nil
}

// uniqueSlug, çakışma varsa sonuna sayı ekleyerek benzersiz slug üretir.
func (s *Store) uniqueSlug(ctx context.Context, tx pgx.Tx, base string) (string, error) {
	return s.uniqueSlugExcept(ctx, tx, base, uuid.Nil)
}

func (s *Store) uniqueSlugExcept(ctx context.Context, tx pgx.Tx, base string, except uuid.UUID) (string, error) {
	candidate := base
	for i := 2; i < 100; i++ {
		var exists bool
		err := tx.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM llm_providers WHERE slug = $1 AND id <> $2)`,
			candidate, except).Scan(&exists)
		if err != nil {
			return "", fmt.Errorf("slug kontrol edilemedi: %w", err)
		}
		if !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return "", errors.New("benzersiz slug üretilemedi")
}
