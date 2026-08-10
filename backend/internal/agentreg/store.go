package agentreg

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/agent-coder/backend/internal/paging"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/slug"
)

// Source, agent'ın nereden geldiği.
type Source string

const (
	// SourceBuiltin, gömülü hazır agent. Silinemez, yalnızca sıfırlanabilir.
	SourceBuiltin Source = "builtin"
	// SourceCustom, kullanıcının oluşturduğu agent.
	SourceCustom Source = "custom"
)

var (
	ErrNotFound       = errors.New("agent bulunamadı")
	ErrSlugTaken      = errors.New("bu kısa ad zaten kullanılıyor")
	ErrBuiltinDelete  = errors.New("hazır agent silinemez, yalnızca sıfırlanabilir")
	ErrNotBuiltin     = errors.New("yalnızca hazır agent sıfırlanabilir")
	ErrInUse          = errors.New("bu agent'ın çalıştırma geçmişi var")
	ErrEmptyPrompt    = errors.New("agent talimatı boş olamaz")
	ErrPromptTooLarge = errors.New("agent talimatı çok büyük")
	ErrInvalidSlug    = errors.New("kısa ad yalnızca küçük harf, rakam ve tire içerebilir")
)

// Agent, tanımlı bir agent.
type Agent struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Prompt      string    `json:"prompt"`
	Source      Source    `json:"source"`

	// IsModified, hazır bir agent'ın özgün halinden farklı olup olmadığı.
	IsModified bool `json:"isModified"`

	DefaultProviderID *uuid.UUID `json:"defaultProviderId"`
	DefaultModel      string     `json:"defaultModel"`

	AllowEdit     bool `json:"allowEdit"`
	AllowBash     bool `json:"allowBash"`
	AllowWebfetch bool `json:"allowWebfetch"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store, agent deposu.
type Store struct {
	pool *pgxpool.Pool
	// maxPromptBytes, talimat boyut sınırı. Ayarlardan gelir.
	maxPrompt func() int
}

// NewStore yeni depo üretir. maxPromptBytes ayarlardan okunacak bir fonksiyondur;
// değer çalışma anında okunur ki ayar değişikliği yeniden başlatma gerektirmesin.
func NewStore(pool *pgxpool.Pool, maxPromptBytes func() int) *Store {
	return &Store{pool: pool, maxPrompt: maxPromptBytes}
}

const agentColumns = `
	id, slug, name, description, prompt, source,
	(source = 'builtin' AND (prompt IS DISTINCT FROM builtin_prompt
	                      OR description IS DISTINCT FROM builtin_description)) AS is_modified,
	default_provider_id, default_model,
	allow_edit, allow_bash, allow_webfetch,
	created_at, updated_at`

func scanAgent(row pgx.Row) (Agent, error) {
	var a Agent
	err := row.Scan(&a.ID, &a.Slug, &a.Name, &a.Description, &a.Prompt, &a.Source,
		&a.IsModified, &a.DefaultProviderID, &a.DefaultModel,
		&a.AllowEdit, &a.AllowBash, &a.AllowWebfetch, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// Seed, gömülü hazır agent'ları veritabanına yazar.
//
// Var olanların TALİMATINA DOKUNMAZ: kullanıcı düzenlemiş olabilir. Yalnızca
// özgün hal referansı (builtin_prompt/builtin_description) tazelenir — böylece
// kod güncellendiğinde "sıfırla" yeni özgün hale döner.
func (s *Store) Seed(ctx context.Context) (int, error) {
	const q = `
		INSERT INTO agents (slug, name, description, prompt, source,
			builtin_prompt, builtin_description,
			allow_edit, allow_bash, allow_webfetch)
		VALUES ($1, $2, $3, $4, 'builtin', $4, $3, $5, $6, $7)
		ON CONFLICT (slug) DO UPDATE SET
			builtin_prompt      = EXCLUDED.builtin_prompt,
			builtin_description = EXCLUDED.builtin_description`

	seeded := 0
	for _, b := range Builtins() {
		if _, err := s.pool.Exec(ctx, q, b.Slug, b.Name, b.Description, b.Prompt,
			b.AllowEdit, b.AllowBash, b.AllowWebfetch); err != nil {
			return seeded, fmt.Errorf("hazır agent %q yazılamadı: %w", b.Slug, err)
		}
		seeded++
	}

	slog.InfoContext(ctx, "hazır agent'lar tohumlandı", "adet", seeded)
	return seeded, nil
}

// List, agent'ları sayfa sayfa döner. Hazır olanlar önce gelir.
func (s *Store) List(ctx context.Context, page paging.Page) ([]Agent, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM agents`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("agent sayısı okunamadı: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT `+agentColumns+` FROM agents ORDER BY source DESC, slug LIMIT $1 OFFSET $2`,
		page.Limit, page.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("agent'lar listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Agent{}
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("agent taranamadı: %w", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// Get, tek bir agent'ı döner.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Agent, error) {
	a, err := scanAgent(s.pool.QueryRow(ctx,
		`SELECT `+agentColumns+` FROM agents WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, fmt.Errorf("agent okunamadı: %w", err)
	}
	return a, nil
}

// CreateInput, yeni agent alanları.
type CreateInput struct {
	Slug              string
	Name              string
	Description       string
	Prompt            string
	DefaultProviderID *uuid.UUID
	DefaultModel      string
	AllowEdit         bool
	AllowBash         bool
	AllowWebfetch     bool
}

// Create, kullanıcının tanımladığı yeni bir agent kaydeder.
func (s *Store) Create(ctx context.Context, in CreateInput) (Agent, error) {
	id := slug.Make(in.Slug, "")
	if id == "" {
		id = slug.Make(in.Name, "")
	}
	if !slug.Valid(id) {
		return Agent{}, ErrInvalidSlug
	}
	if err := s.validatePrompt(in.Prompt); err != nil {
		return Agent{}, err
	}

	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = titleFromSlug(id)
	}

	a, err := scanAgent(s.pool.QueryRow(ctx, `
		INSERT INTO agents (slug, name, description, prompt, source,
			default_provider_id, default_model, allow_edit, allow_bash, allow_webfetch)
		VALUES ($1, $2, $3, $4, 'custom', $5, $6, $7, $8, $9)
		RETURNING `+agentColumns,
		id, name, strings.TrimSpace(in.Description), strings.TrimSpace(in.Prompt),
		in.DefaultProviderID, in.DefaultModel,
		in.AllowEdit, in.AllowBash, in.AllowWebfetch))
	if err != nil {
		if isUniqueViolation(err) {
			return Agent{}, ErrSlugTaken
		}
		return Agent{}, fmt.Errorf("agent kaydedilemedi: %w", err)
	}
	return a, nil
}

// UpdateInput, güncellenebilir alanlar. nil olanlar değiştirilmez.
type UpdateInput struct {
	Name              *string
	Description       *string
	Prompt            *string
	DefaultProviderID *uuid.UUID
	ClearProvider     bool
	DefaultModel      *string
	AllowEdit         *bool
	AllowBash         *bool
	AllowWebfetch     *bool
}

// Update, agent'ı günceller. Hazır agent'ların da talimatı düzenlenebilir.
func (s *Store) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Agent, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Agent{}, err
	}

	if in.Prompt != nil {
		if err := s.validatePrompt(*in.Prompt); err != nil {
			return Agent{}, err
		}
	}

	providerID := current.DefaultProviderID
	if in.ClearProvider {
		providerID = nil
	} else if in.DefaultProviderID != nil {
		providerID = in.DefaultProviderID
	}

	a, err := scanAgent(s.pool.QueryRow(ctx, `
		UPDATE agents SET
			name                = $2,
			description         = $3,
			prompt              = $4,
			default_provider_id = $5,
			default_model       = $6,
			allow_edit          = $7,
			allow_bash          = $8,
			allow_webfetch      = $9,
			updated_at          = now()
		WHERE id = $1
		RETURNING `+agentColumns,
		id,
		strOr(in.Name, current.Name),
		strOr(in.Description, current.Description),
		strings.TrimSpace(strOr(in.Prompt, current.Prompt)),
		providerID,
		strOr(in.DefaultModel, current.DefaultModel),
		boolOr(in.AllowEdit, current.AllowEdit),
		boolOr(in.AllowBash, current.AllowBash),
		boolOr(in.AllowWebfetch, current.AllowWebfetch)))
	if err != nil {
		return Agent{}, fmt.Errorf("agent güncellenemedi: %w", err)
	}
	return a, nil
}

// Reset, hazır bir agent'ı özgün haline döndürür.
func (s *Store) Reset(ctx context.Context, id uuid.UUID) (Agent, error) {
	a, err := scanAgent(s.pool.QueryRow(ctx, `
		UPDATE agents SET
			prompt      = builtin_prompt,
			description = builtin_description,
			updated_at  = now()
		WHERE id = $1 AND source = 'builtin' AND builtin_prompt IS NOT NULL
		RETURNING `+agentColumns, id))
	if errors.Is(err, pgx.ErrNoRows) {
		// Ya yok ya da hazır değil; ikisini ayırt et.
		if _, getErr := s.Get(ctx, id); errors.Is(getErr, ErrNotFound) {
			return Agent{}, ErrNotFound
		}
		return Agent{}, ErrNotBuiltin
	}
	if err != nil {
		return Agent{}, fmt.Errorf("agent sıfırlanamadı: %w", err)
	}
	return a, nil
}

// Delete, kullanıcının oluşturduğu bir agent'ı siler.
//
// Hazır agent silinemez. Çalıştırma geçmişi olan agent da silinemez:
// geçmiş kaydı kime ait olduğunu bilmeye devam etmeli.
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	a, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if a.Source == SourceBuiltin {
		return ErrBuiltinDelete
	}

	tag, err := s.pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, id)
	if err != nil {
		if isForeignKeyViolation(err) {
			return ErrInUse
		}
		return fmt.Errorf("agent silinemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// validatePrompt, talimatın boş olmadığını ve boyut sınırını aşmadığını doğrular.
func (s *Store) validatePrompt(prompt string) error {
	if strings.TrimSpace(prompt) == "" {
		return ErrEmptyPrompt
	}
	maxBytes := 32 << 10
	if s.maxPrompt != nil {
		if kb := s.maxPrompt(); kb > 0 {
			maxBytes = kb << 10
		}
	}
	if len(prompt) > maxBytes {
		return fmt.Errorf("%w: en fazla %d KB", ErrPromptTooLarge, maxBytes>>10)
	}
	return nil
}

func strOr(v *string, def string) string {
	if v == nil {
		return def
	}
	return *v
}

func boolOr(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
