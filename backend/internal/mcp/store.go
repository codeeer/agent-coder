package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/secrets"
)

// Store, MCP sunucu deposu.
type Store struct {
	pool   *pgxpool.Pool
	cipher *secrets.Cipher
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool, cipher *secrets.Cipher) *Store {
	return &Store{pool: pool, cipher: cipher}
}

const serverColumns = `id, name, transport, url, hint, (secret_enc IS NOT NULL), tools, created_at, updated_at`

func scanServer(row pgx.Row) (Server, error) {
	var s Server
	var tools []byte
	err := row.Scan(&s.ID, &s.Name, &s.Transport, &s.URL, &s.Hint, &s.HasSecret,
		&tools, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return Server{}, err
	}
	s.Tools = []string{}
	if len(tools) > 0 {
		// Bozuk JSON kaydı listeyi boş bırakır; sunucu tanımı yine kullanılabilir.
		_ = json.Unmarshal(tools, &s.Tools)
	}
	return s, nil
}

// List, tanımlı tüm sunucuları döner.
func (s *Store) List(ctx context.Context) ([]Server, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+serverColumns+` FROM mcp_servers ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("MCP sunucuları listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Server{}
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("MCP sunucusu taranamadı: %w", err)
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

// Get, tek bir sunucuyu döner.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Server, error) {
	srv, err := scanServer(s.pool.QueryRow(ctx,
		`SELECT `+serverColumns+` FROM mcp_servers WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	if err != nil {
		return Server{}, fmt.Errorf("MCP sunucusu okunamadı: %w", err)
	}
	return srv, nil
}

// ForAgent, bir agent'a atanmış sunucuları döner.
func (s *Store) ForAgent(ctx context.Context, agentID uuid.UUID) ([]Server, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+serverColumns+` FROM mcp_servers m
		JOIN agent_mcp_servers a ON a.mcp_server_id = m.id
		WHERE a.agent_id = $1
		ORDER BY m.name`, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent'ın MCP sunucuları okunamadı: %w", err)
	}
	defer rows.Close()

	out := []Server{}
	for rows.Next() {
		srv, err := scanServer(rows)
		if err != nil {
			return nil, fmt.Errorf("MCP sunucusu taranamadı: %w", err)
		}
		out = append(out, srv)
	}
	return out, rows.Err()
}

// SetAgentServers, bir agent'ın erişebileceği sunucuları belirler.
//
// Tümü birden yazılır (sil + ekle): kısmi güncelleme, arayüzden gelen listeyle
// veritabanındaki durumun ayrışmasına açık olurdu.
func (s *Store) SetAgentServers(ctx context.Context, agentID uuid.UUID, serverIDs []uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM agent_mcp_servers WHERE agent_id = $1`, agentID); err != nil {
		return fmt.Errorf("eski MCP atamaları silinemedi: %w", err)
	}

	for _, id := range serverIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO agent_mcp_servers (agent_id, mcp_server_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, agentID, id)
		if err != nil {
			// Var olmayan sunucu kimliği yabancı anahtarla yakalanır.
			if isForeignKeyViolation(err) {
				return ErrNotFound
			}
			return fmt.Errorf("MCP ataması kaydedilemedi: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("transaction tamamlanamadı: %w", err)
	}
	return nil
}

// Reveal, sunucunun erişim anahtarını çözer.
//
// Adı bilinçli olarak dikkat çekicidir: gizli değere ulaşmanın tek yolu budur.
// Anahtarsız sunucularda boş dize döner — hata değil.
func (s *Store) Reveal(ctx context.Context, id uuid.UUID) (string, error) {
	var blob []byte
	err := s.pool.QueryRow(ctx, `SELECT secret_enc FROM mcp_servers WHERE id = $1`, id).Scan(&blob)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("MCP anahtarı okunamadı: %w", err)
	}
	if len(blob) == 0 {
		return "", nil
	}

	secret, err := s.cipher.DecryptString(blob)
	if err != nil {
		return "", fmt.Errorf("MCP anahtarı çözülemedi (şifreleme anahtarı değişmiş olabilir): %w", err)
	}
	return secret, nil
}

// CreateInput, yeni sunucu için alanlar.
type CreateInput struct {
	Name      string
	Transport Transport
	URL       string
	// Secret boş olabilir: anahtarsız çalışan sunucular var.
	Secret string
	Tools  []string
}

// Create, yeni sunucu kaydeder.
func (s *Store) Create(ctx context.Context, in CreateInput) (Server, error) {
	srv := Server{Name: strings.TrimSpace(in.Name), Transport: in.Transport, URL: strings.TrimSpace(in.URL)}
	if err := srv.Validate(); err != nil {
		return Server{}, err
	}

	blob, hint, err := s.encrypt(in.Secret)
	if err != nil {
		return Server{}, err
	}
	tools, err := json.Marshal(defaultTools(in.Tools))
	if err != nil {
		return Server{}, fmt.Errorf("araç listesi kaydedilemedi: %w", err)
	}

	out, err := scanServer(s.pool.QueryRow(ctx, `
		INSERT INTO mcp_servers (name, transport, url, secret_enc, hint, tools)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+serverColumns,
		srv.Name, string(srv.Transport), srv.URL, blob, hint, tools))
	if err != nil {
		if isUniqueViolation(err) {
			return Server{}, ErrDuplicateName
		}
		return Server{}, fmt.Errorf("MCP sunucusu kaydedilemedi: %w", err)
	}
	return out, nil
}

// UpdateInput, güncellenebilir alanlar. nil olanlar değiştirilmez.
type UpdateInput struct {
	Name      *string
	Transport *Transport
	URL       *string
	// Secret nil veya boşsa mevcut anahtar KORUNUR — kullanıcı adı değiştirmek
	// için anahtarı yeniden yazmak zorunda kalmasın.
	Secret *string
	// ClearSecret, kayıtlı anahtarı siler.
	//
	// Ayrı bir bayrak gerekiyor çünkü boş dize "değiştirme" anlamına geliyor.
	// Bu olmadan, herkese açık bir sunucuya yanlışlıkla anahtar yazan kullanıcı
	// sunucuyu silip yeniden kurmak zorunda kalır — agent atamalarını da
	// kaybederek.
	ClearSecret bool
	Tools       []string
}

// Update, mevcut sunucuyu günceller.
func (s *Store) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Server, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Server{}, fmt.Errorf("transaction başlatılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanServer(tx.QueryRow(ctx,
		`SELECT `+serverColumns+` FROM mcp_servers WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Server{}, ErrNotFound
	}
	if err != nil {
		return Server{}, fmt.Errorf("MCP sunucusu okunamadı: %w", err)
	}

	next := Server{
		Name:      strings.TrimSpace(valueOr(in.Name, current.Name)),
		Transport: current.Transport,
		URL:       strings.TrimSpace(valueOr(in.URL, current.URL)),
	}
	if in.Transport != nil {
		next.Transport = *in.Transport
	}
	if err := next.Validate(); err != nil {
		return Server{}, err
	}

	switch {
	case in.ClearSecret:
		if _, err := tx.Exec(ctx,
			`UPDATE mcp_servers SET secret_enc = NULL, hint = '' WHERE id = $1`, id); err != nil {
			return Server{}, fmt.Errorf("erişim bilgisi silinemedi: %w", err)
		}
	case in.Secret != nil && *in.Secret != "":
		blob, hint, err := s.encrypt(*in.Secret)
		if err != nil {
			return Server{}, err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE mcp_servers SET secret_enc = $2, hint = $3 WHERE id = $1`,
			id, blob, hint); err != nil {
			return Server{}, fmt.Errorf("erişim bilgisi güncellenemedi: %w", err)
		}
	}

	tools, err := json.Marshal(defaultTools(in.Tools))
	if err != nil {
		return Server{}, fmt.Errorf("araç listesi kaydedilemedi: %w", err)
	}
	if in.Tools == nil {
		tools, _ = json.Marshal(current.Tools)
	}

	out, err := scanServer(tx.QueryRow(ctx, `
		UPDATE mcp_servers
		SET name = $2, transport = $3, url = $4, tools = $5, updated_at = now()
		WHERE id = $1
		RETURNING `+serverColumns,
		id, next.Name, string(next.Transport), next.URL, tools))
	if err != nil {
		if isUniqueViolation(err) {
			return Server{}, ErrDuplicateName
		}
		return Server{}, fmt.Errorf("MCP sunucusu güncellenemedi: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Server{}, fmt.Errorf("transaction tamamlanamadı: %w", err)
	}
	return out, nil
}

// Delete, sunucuyu siler. Agent atamaları da düşer (ON DELETE CASCADE).
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM mcp_servers WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("MCP sunucusu silinemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) encrypt(secret string) ([]byte, string, error) {
	if strings.TrimSpace(secret) == "" {
		return nil, "", nil
	}
	blob, err := s.cipher.EncryptString(secret)
	if err != nil {
		return nil, "", fmt.Errorf("erişim bilgisi şifrelenemedi: %w", err)
	}
	return blob, secrets.Mask(secret), nil
}

func defaultTools(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
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
