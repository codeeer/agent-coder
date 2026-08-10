package scripts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store, betik deposu.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const columns = `id, name, description, content, created_at, updated_at`

func scan(row pgx.Row) (Script, error) {
	var s Script
	err := row.Scan(&s.ID, &s.Name, &s.Description, &s.Content, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return Script{}, err
	}
	return s, nil
}

func collect(rows pgx.Rows) ([]Script, error) {
	defer rows.Close()

	out := []Script{}
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("betik taranamadı: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// List, betikleri sayfalı olarak döner. total, sayfalamadan bağımsız toplam.
func (s *Store) List(ctx context.Context, limit, offset int) (items []Script, total int, err error) {
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM scripts`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("betik sayısı okunamadı: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT `+columns+` FROM scripts ORDER BY name LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("betikler listelenemedi: %w", err)
	}
	items, err = collect(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Get, tek bir betiği döner.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Script, error) {
	out, err := scan(s.pool.QueryRow(ctx, `SELECT `+columns+` FROM scripts WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Script{}, ErrNotFound
	}
	if err != nil {
		return Script{}, fmt.Errorf("betik okunamadı: %w", err)
	}
	return out, nil
}

// ForAgent, bir agent'a atanmış betikleri döner.
//
// Sıralama ada göre sabit: talimat dosyasındaki liste her çalıştırmada aynı
// sırada olsun. Değişen bir sıra, aynı agent'ın farklı çalıştırmalarında farklı
// bir talimat dosyası üretirdi.
func (s *Store) ForAgent(ctx context.Context, agentID uuid.UUID) ([]Script, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+columns+` FROM scripts s
		JOIN agent_scripts a ON a.script_id = s.id
		WHERE a.agent_id = $1
		ORDER BY s.name`, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent'ın betikleri okunamadı: %w", err)
	}
	return collect(rows)
}

// SetAgentScripts, bir agent'ın çalıştırabileceği betikleri belirler.
//
// Tümü birden yazılır (sil + ekle): kısmi güncelleme, arayüzden gelen listeyle
// veritabanındaki durumun ayrışmasına açık olurdu.
func (s *Store) SetAgentScripts(ctx context.Context, agentID uuid.UUID, scriptIDs []uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `DELETE FROM agent_scripts WHERE agent_id = $1`, agentID); err != nil {
		return fmt.Errorf("eski betik atamaları silinemedi: %w", err)
	}

	for _, id := range scriptIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO agent_scripts (agent_id, script_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, agentID, id)
		if err != nil {
			// Var olmayan betik kimliği yabancı anahtarla yakalanır.
			if isForeignKeyViolation(err) {
				return ErrNotFound
			}
			return fmt.Errorf("betik ataması kaydedilemedi: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("transaction tamamlanamadı: %w", err)
	}
	return nil
}

// CreateInput, yeni betik için alanlar.
type CreateInput struct {
	Name        string
	Description string
	Content     string
}

// Create, yeni betik kaydeder.
func (s *Store) Create(ctx context.Context, in CreateInput) (Script, error) {
	next := Script{
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Content:     normalizeContent(in.Content),
	}
	if err := next.Validate(); err != nil {
		return Script{}, err
	}

	out, err := scan(s.pool.QueryRow(ctx, `
		INSERT INTO scripts (name, description, content)
		VALUES ($1, $2, $3)
		RETURNING `+columns, next.Name, next.Description, next.Content))
	if err != nil {
		if isUniqueViolation(err) {
			return Script{}, ErrDuplicateName
		}
		return Script{}, fmt.Errorf("betik kaydedilemedi: %w", err)
	}
	return out, nil
}

// UpdateInput, güncellenebilir alanlar. nil olanlar değiştirilmez.
type UpdateInput struct {
	Name        *string
	Description *string
	Content     *string
}

// Update, mevcut betiği günceller.
//
// Yeni içerik BİR SONRAKİ çalıştırmada geçerli olur: dosyalar container
// başlatılmadan önce yazılıyor, süren bir çalıştırmaya müdahale edilmiyor.
func (s *Store) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Script, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Script{}, err
	}

	next := Script{
		Name:        strings.TrimSpace(valueOr(in.Name, current.Name)),
		Description: strings.TrimSpace(valueOr(in.Description, current.Description)),
		Content:     normalizeContent(valueOr(in.Content, current.Content)),
	}
	if err := next.Validate(); err != nil {
		return Script{}, err
	}

	out, err := scan(s.pool.QueryRow(ctx, `
		UPDATE scripts SET name = $2, description = $3, content = $4, updated_at = now()
		WHERE id = $1
		RETURNING `+columns, id, next.Name, next.Description, next.Content))
	if errors.Is(err, pgx.ErrNoRows) {
		return Script{}, ErrNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return Script{}, ErrDuplicateName
		}
		return Script{}, fmt.Errorf("betik güncellenemedi: %w", err)
	}
	return out, nil
}

// Delete, betiği siler. Agent atamaları da düşer (ON DELETE CASCADE).
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM scripts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("betik silinemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// normalizeContent, satır sonlarını LF'e çevirir ve dosyayı yeni satırla bitirir.
//
// Windows'ta yazılıp yapıştırılan bir betiğin `\r` taşıması, kabukta
// `command not found` gibi ANLAŞILMAZ bir hataya yol açıyor — shebang satırı
// `#!/bin/bash\r` olarak okunuyor. Kullanıcının göremediği bir karakter yüzünden
// çalışmayan bir betik, hata ayıklaması en pahalı sorunlardan biri.
func normalizeContent(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}

func valueOr(v *string, def string) string {
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
