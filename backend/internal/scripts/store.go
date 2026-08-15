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

/*
 * Seçilen sütunlar. Klasör adı JOIN'den geliyor: yol üretmek için gerekli ve
 * her script için ayrı sorgu atmak, yüz script'lik bir agent'ta yüz sorgu
 * demekti.
 */
const columns = `s.id, s.name, s.description, s.content, s.folder_id,
	s.created_at, s.updated_at, COALESCE(f.name, '')`

// kaynak, `columns` ile birlikte kullanılan FROM bölümü.
//
// LEFT JOIN: klasörsüz script'ler de dönmeli — bugünkü davranış değişmiyor.
const kaynak = ` FROM scripts s LEFT JOIN script_folders f ON f.id = s.folder_id`

func scan(row pgx.Row) (Script, error) {
	var s Script
	err := row.Scan(&s.ID, &s.Name, &s.Description, &s.Content, &s.FolderID,
		&s.CreatedAt, &s.UpdatedAt, &s.FolderName)
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

/*
Filter, betik listesinin süzgeci.

ARAMA SUNUCUDA YAPILIR, ekranda değil: liste sayfalı ve yalnızca açık sayfada
aramak kullanıcıya var olan bir betik için "yok" dedirtirdi — hem de sessizce.
*/
type Filter struct {
	// Query, ad ve açıklamada geçen metin. Boşsa süzmez.
	Query string

	// FolderID, klasör süzgeci. nil süzmez.
	FolderID *uuid.UUID
	// Unfiled true ise YALNIZCA klasörsüz betikler döner (FolderID yok sayılır).
	Unfiled bool

	Limit, Offset int
}

// where, süzgecin SQL karşılığını ve parametrelerini üretir.
//
// Metin `$n` parametresiyle geçer, SQL'e birleştirilmez (AGENTS.md → Go).
func (f Filter) where() (string, []any) {
	cond := []string{}
	args := []any{}

	if q := strings.TrimSpace(f.Query); q != "" {
		args = append(args, "%"+q+"%")
		// Açıklama da aranır: kullanıcı betiği çoğu zaman ne yaptığıyla
		// hatırlıyor, adıyla değil.
		cond = append(cond, fmt.Sprintf("(s.name ILIKE $%d OR s.description ILIKE $%d)", len(args), len(args)))
	}
	switch {
	case f.Unfiled:
		cond = append(cond, "s.folder_id IS NULL")
	case f.FolderID != nil:
		args = append(args, *f.FolderID)
		cond = append(cond, fmt.Sprintf("s.folder_id = $%d", len(args)))
	}

	if len(cond) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(cond, " AND "), args
}

// List, betikleri süzerek ve sayfalayarak döner. total, SÜZGECE UYAN toplam —
// sayfalama ondan bağımsız.
func (s *Store) List(ctx context.Context, f Filter) (items []Script, total int, err error) {
	where, args := f.where()

	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM scripts s`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("betik sayısı okunamadı: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, `SELECT `+columns+kaynak+where+
		fmt.Sprintf(` ORDER BY COALESCE(f.name, ''), s.name LIMIT $%d OFFSET $%d`,
			len(args)-1, len(args)), args...)
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
	out, err := scan(s.pool.QueryRow(ctx, `SELECT `+columns+kaynak+` WHERE s.id = $1`, id))
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
	/*
	 * İKİ KÜMENİN BİRLEŞİMİ: agent'a doğrudan atanmış script'ler ve atanmış
	 * KLASÖRLERİN tüm script'leri.
	 *
	 * Klasör içeriği burada, çalıştırma anında çözülüyor. Atama sırasında
	 * çözülseydi klasöre sonradan eklenen bir script o agent'ta geçerli olmaz
	 * ve kullanıcı her eklemede bütün agent'ları tekrar düzenlerdi (spec 022 H3).
	 *
	 * DISTINCT şart: bir script hem tekil hem klasör üzerinden atanmış
	 * olabilir; iki kez dönseydi talimatta iki kez yazılırdı.
	 */
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT `+columns+kaynak+`
		WHERE s.id IN (SELECT script_id FROM agent_scripts WHERE agent_id = $1)
		   OR s.folder_id IN (SELECT folder_id FROM agent_script_folders WHERE agent_id = $1)
		ORDER BY COALESCE(f.name, ''), s.name`, agentID)
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
	// FolderID nil ise script klasörsüz olur.
	FolderID *uuid.UUID
}

// Create, yeni betik kaydeder.
func (s *Store) Create(ctx context.Context, in CreateInput) (Script, error) {
	next := Script{
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
		Content:     normalizeContent(in.Content),
		FolderID:    in.FolderID,
	}
	if err := next.Validate(); err != nil {
		return Script{}, err
	}

	// Yeni kayıt kendi kimliğiyle geri okunuyor: `RETURNING` klasör adını
	// veremez (JOIN yok) ve yol üretimi ona bağlı.
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO scripts (name, description, content, folder_id)
		VALUES ($1, $2, $3, $4) RETURNING id`,
		next.Name, next.Description, next.Content, next.FolderID).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return Script{}, ErrDuplicateName
		}
		if isForeignKeyViolation(err) {
			return Script{}, ErrFolderNotFound
		}
		return Script{}, fmt.Errorf("betik kaydedilemedi: %w", err)
	}
	return s.Get(ctx, id)
}

// UpdateInput, güncellenebilir alanlar. nil olanlar değiştirilmez.
type UpdateInput struct {
	Name        *string
	Description *string
	Content     *string

	// FolderID nil ise klasör DEĞİŞMEZ. Klasörden çıkarmak için `clearFolder`
	// kullanılır — `projects.Store.Update`'teki `clearProvider` kalıbının
	// aynısı. Tek bir işaretçiyle ikisini ayırt etmek mümkün değil: nil hem
	// "dokunma" hem "boşalt" anlamına gelirdi.
	FolderID *uuid.UUID
}

// Update, mevcut betiği günceller.
//
// Yeni içerik BİR SONRAKİ çalıştırmada geçerli olur: dosyalar container
// başlatılmadan önce yazılıyor, süren bir çalıştırmaya müdahale edilmiyor.
func (s *Store) Update(ctx context.Context, id uuid.UUID, in UpdateInput, clearFolder bool) (Script, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return Script{}, err
	}

	folderID := current.FolderID
	switch {
	case clearFolder:
		folderID = nil
	case in.FolderID != nil:
		folderID = in.FolderID
	}

	next := Script{
		Name:        strings.TrimSpace(valueOr(in.Name, current.Name)),
		Description: strings.TrimSpace(valueOr(in.Description, current.Description)),
		Content:     normalizeContent(valueOr(in.Content, current.Content)),
		FolderID:    folderID,
	}
	if err := next.Validate(); err != nil {
		return Script{}, err
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE scripts SET name = $2, description = $3, content = $4,
			folder_id = $5, updated_at = now()
		WHERE id = $1`,
		id, next.Name, next.Description, next.Content, next.FolderID)
	if err != nil {
		if isUniqueViolation(err) {
			return Script{}, ErrDuplicateName
		}
		if isForeignKeyViolation(err) {
			return Script{}, ErrFolderNotFound
		}
		return Script{}, fmt.Errorf("betik güncellenemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Script{}, ErrNotFound
	}
	return s.Get(ctx, id)
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
