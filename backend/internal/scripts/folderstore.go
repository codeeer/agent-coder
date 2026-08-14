package scripts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const folderColumns = `f.id, f.name, f.description, f.created_at, f.updated_at`

func scanFolder(row pgx.Row) (Folder, error) {
	var f Folder
	err := row.Scan(&f.ID, &f.Name, &f.Description, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

/*
ListFolders, klasörleri script sayılarıyla döner.

Sayı AYRI SORGU DEĞİL: her klasör için bir sorgu atmak, on kampanyalı bir
kurulumda on bir sorgu demekti. Boş klasörler de listelenir — kullanıcı
kampanyayı script eklemeden önce açabilmeli.
*/
func (s *Store) ListFolders(ctx context.Context) ([]Folder, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+folderColumns+`, count(sc.id)
		FROM script_folders f
		LEFT JOIN scripts sc ON sc.folder_id = f.id
		GROUP BY f.id
		ORDER BY f.name`)
	if err != nil {
		return nil, fmt.Errorf("klasörler listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Folder{}
	for rows.Next() {
		var f Folder
		if err := rows.Scan(&f.ID, &f.Name, &f.Description,
			&f.CreatedAt, &f.UpdatedAt, &f.ScriptCount); err != nil {
			return nil, fmt.Errorf("klasör taranamadı: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetFolder, tek bir klasörü döner.
func (s *Store) GetFolder(ctx context.Context, id uuid.UUID) (Folder, error) {
	f, err := scanFolder(s.pool.QueryRow(ctx,
		`SELECT `+folderColumns+` FROM script_folders f WHERE f.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Folder{}, ErrFolderNotFound
	}
	if err != nil {
		return Folder{}, fmt.Errorf("klasör okunamadı: %w", err)
	}
	return f, nil
}

// CreateFolder, yeni klasör açar.
func (s *Store) CreateFolder(ctx context.Context, in FolderInput) (Folder, error) {
	if err := in.Validate(); err != nil {
		return Folder{}, err
	}

	f, err := scanFolder(s.pool.QueryRow(ctx, `
		INSERT INTO script_folders (name, description) VALUES ($1, $2)
		RETURNING `+strings.ReplaceAll(folderColumns, "f.", ""),
		strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)))
	if err != nil {
		if isUniqueViolation(err) {
			return Folder{}, ErrDuplicateFolder
		}
		return Folder{}, fmt.Errorf("klasör kaydedilemedi: %w", err)
	}
	return f, nil
}

/*
UpdateFolder, klasörü günceller.

AD DEĞİŞİKLİĞİ CONTAINER YOLUNU DEĞİŞTİRİR — ama yalnızca SONRAKİ
çalıştırmada. Dosyalar container başlatılmadan önce yazıldığı için süren bir
çalıştırma kendi kopyasıyla devam eder; script içeriği güncellemesindeki
davranışın aynısı.
*/
func (s *Store) UpdateFolder(ctx context.Context, id uuid.UUID, in FolderInput) (Folder, error) {
	if err := in.Validate(); err != nil {
		return Folder{}, err
	}

	f, err := scanFolder(s.pool.QueryRow(ctx, `
		UPDATE script_folders SET name = $2, description = $3, updated_at = now()
		WHERE id = $1
		RETURNING `+strings.ReplaceAll(folderColumns, "f.", ""),
		id, strings.TrimSpace(in.Name), strings.TrimSpace(in.Description)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Folder{}, ErrFolderNotFound
	}
	if err != nil {
		if isUniqueViolation(err) {
			return Folder{}, ErrDuplicateFolder
		}
		return Folder{}, fmt.Errorf("klasör güncellenemedi: %w", err)
	}
	return f, nil
}

/*
DeleteFolder, klasörü siler.

İÇİNDEKİ SCRIPT'LER SİLİNMEZ, klasörsüz kalır (`ON DELETE SET NULL`).
Kullanıcı kampanyayı kaldırıyor, yazdığı script'leri değil — bir düzenleme
kararı veri kaybına dönüşmemeli.
*/
func (s *Store) DeleteFolder(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM script_folders WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("klasör silinemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrFolderNotFound
	}
	return nil
}

/*
FolderUsage, klasörün kaç script taşıdığını ve kaç agent'a atandığını döner.

Silmeden önce kullanıcıya söylenir: "üç script klasörsüz kalacak, iki agent
etkilenecek" cümlesi, "silinsin mi?" sorusundan farklı bir karar aldırır.
*/
func (s *Store) FolderUsage(ctx context.Context, id uuid.UUID) (scriptCount, agentCount int, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM scripts WHERE folder_id = $1),
		       (SELECT count(*) FROM agent_script_folders WHERE folder_id = $1)`,
		id).Scan(&scriptCount, &agentCount)
	if err != nil {
		return 0, 0, fmt.Errorf("klasör kullanımı okunamadı: %w", err)
	}
	return scriptCount, agentCount, nil
}

/*
SetAgentFolders, bir agent'a atanmış klasörleri belirler.

Tümü birden yazılır (sil + ekle, tek transaction): `SetAgentScripts`'teki
kalıbın aynısı. Kısmi güncelleme, arayüzden gelen listeyle veritabanındaki
durumun ayrışmasına açık olurdu.
*/
func (s *Store) SetAgentFolders(ctx context.Context, agentID uuid.UUID, folderIDs []uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("işlem başlatılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`DELETE FROM agent_script_folders WHERE agent_id = $1`, agentID); err != nil {
		return fmt.Errorf("klasör atamaları temizlenemedi: %w", err)
	}

	for _, id := range folderIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_script_folders (agent_id, folder_id) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, agentID, id); err != nil {
			if isForeignKeyViolation(err) {
				return ErrFolderNotFound
			}
			return fmt.Errorf("klasör ataması yazılamadı: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("işlem tamamlanamadı: %w", err)
	}
	return nil
}

// FoldersForAgent, agent'a atanmış klasörleri döner.
//
// Talimat metni klasörün AÇIKLAMASINA ihtiyaç duyuyor; script listesi onu
// taşımıyor.
func (s *Store) FoldersForAgent(ctx context.Context, agentID uuid.UUID) ([]Folder, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+folderColumns+` FROM script_folders f
		JOIN agent_script_folders a ON a.folder_id = f.id
		WHERE a.agent_id = $1
		ORDER BY f.name`, agentID)
	if err != nil {
		return nil, fmt.Errorf("agent'ın klasörleri okunamadı: %w", err)
	}
	defer rows.Close()

	out := []Folder{}
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, fmt.Errorf("klasör taranamadı: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
