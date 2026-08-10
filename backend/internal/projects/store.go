// Package projects, üzerinde çalışılacak kod depolarını yönetir.
package projects

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/paging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound       = errors.New("proje bulunamadı")
	ErrEmptyName      = errors.New("proje adı boş olamaz")
	ErrInvalidRepoURL = errors.New("depo adresi geçersiz")
)

// Project, tanımlı bir kod deposu.
type Project struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	RepoURL       string    `json:"repoUrl"`
	DefaultBranch string    `json:"defaultBranch"`

	// GitProviderID nil ise depo kimlik doğrulamasız klonlanır (açık depo).
	GitProviderID *uuid.UUID `json:"gitProviderId"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Store, proje deposu.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const columns = `id, name, repo_url, default_branch, git_provider_id, created_at, updated_at`

func scan(row pgx.Row) (Project, error) {
	var p Project
	err := row.Scan(&p.ID, &p.Name, &p.RepoURL, &p.DefaultBranch,
		&p.GitProviderID, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// List, projeleri sayfa sayfa döner; ikinci dönüş değeri toplam kayıt sayısıdır.
//
// Toplam AYRI bir sorgu: sayfadaki kayıt sayısıyla toplam farklı şeylerdir ve
// arayüz "1–25 / 138" diyebilmek için ikisine de ihtiyaç duyar.
func (s *Store) List(ctx context.Context, page paging.Page) ([]Project, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM projects`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("proje sayısı okunamadı: %w", err)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT `+columns+` FROM projects ORDER BY name LIMIT $1 OFFSET $2`,
		page.Limit, page.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("projeler listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Project{}
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("proje taranamadı: %w", err)
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

// Get, tek bir projeyi döner.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Project, error) {
	p, err := scan(s.pool.QueryRow(ctx, `SELECT `+columns+` FROM projects WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("proje okunamadı: %w", err)
	}
	return p, nil
}

// RunCount, projenin çalıştırma sayısı. Silmeden önce kullanıcıya söylenir.
func (s *Store) RunCount(ctx context.Context, id uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM runs WHERE project_id = $1`, id).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("çalıştırma sayısı okunamadı: %w", err)
	}
	return n, nil
}

// Input, oluşturma ve güncelleme alanları.
type Input struct {
	Name          string
	RepoURL       string
	DefaultBranch string
	GitProviderID *uuid.UUID
}

// Normalize, girdiyi doğrular ve tekilleştirir.
func (in *Input) Normalize() error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return ErrEmptyName
	}

	in.RepoURL = strings.TrimSpace(in.RepoURL)
	if in.RepoURL == "" {
		return fmt.Errorf("%w: adres boş", ErrInvalidRepoURL)
	}

	u, err := url.Parse(in.RepoURL)
	if err != nil {
		return fmt.Errorf("%w: ayrıştırılamadı", ErrInvalidRepoURL)
	}
	// Yalnızca HTTPS: SSH anahtarı yönetimi bu fazın kapsamında değil ve
	// kimlik bilgisi akışımız (credential store) HTTP üzerine kurulu.
	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("%w: https:// ile başlamalı", ErrInvalidRepoURL)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: sunucu adı yok", ErrInvalidRepoURL)
	}
	// Kimlik bilgisi adrese gömülmüş olabilir; kabul edilmez — bizim
	// kimlik yönetimimizden geçmeli ve veritabanına düz metin girmemeli.
	if u.User != nil {
		return fmt.Errorf("%w: adres kullanıcı adı/parola içermemeli, git erişimi tanımlayın", ErrInvalidRepoURL)
	}

	in.DefaultBranch = strings.TrimSpace(in.DefaultBranch)
	if in.DefaultBranch == "" {
		in.DefaultBranch = "main"
	}
	return nil
}

// Create, yeni proje kaydeder.
func (s *Store) Create(ctx context.Context, in Input) (Project, error) {
	if err := in.Normalize(); err != nil {
		return Project{}, err
	}

	p, err := scan(s.pool.QueryRow(ctx, `
		INSERT INTO projects (name, repo_url, default_branch, git_provider_id)
		VALUES ($1, $2, $3, $4)
		RETURNING `+columns,
		in.Name, in.RepoURL, in.DefaultBranch, in.GitProviderID))
	if err != nil {
		return Project{}, fmt.Errorf("proje kaydedilemedi: %w", err)
	}
	return p, nil
}

// Update, projeyi günceller.
func (s *Store) Update(ctx context.Context, id uuid.UUID, in Input, clearProvider bool) (Project, error) {
	if err := in.Normalize(); err != nil {
		return Project{}, err
	}

	current, err := s.Get(ctx, id)
	if err != nil {
		return Project{}, err
	}

	providerID := current.GitProviderID
	if clearProvider {
		providerID = nil
	} else if in.GitProviderID != nil {
		providerID = in.GitProviderID
	}

	p, err := scan(s.pool.QueryRow(ctx, `
		UPDATE projects SET name = $2, repo_url = $3, default_branch = $4,
			git_provider_id = $5, updated_at = now()
		WHERE id = $1
		RETURNING `+columns,
		id, in.Name, in.RepoURL, in.DefaultBranch, providerID))
	if err != nil {
		return Project{}, fmt.Errorf("proje güncellenemedi: %w", err)
	}
	return p, nil
}

// Delete, projeyi ve çalıştırma geçmişini siler (CASCADE).
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("proje silinemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
