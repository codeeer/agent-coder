package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

/*
 * Jira tetikleyicinin veri katmanı.
 *
 * İki giriş yolu (tarama ve webhook) BURADA buluşuyor: ikisi de aynı
 * `MarkProcessed` kontrolünden geçiyor. Ayrı ayrı korunsalardı iki yol aynı
 * task'ı aynı anda işleyebilirdi.
 */

// ScanState, bir akışın son tarama durumu.
type ScanState struct {
	WorkflowID uuid.UUID  `json:"workflowId"`
	ScannedAt  *time.Time `json:"scannedAt"`
	Found      int        `json:"found"`
	Started    int        `json:"started"`
	Error      *string    `json:"error"`
}

// MarkProcessed, bir task'ı işlenmiş olarak kaydeder.
//
// İşin tamamı bu fonksiyonun DÖNÜŞ DEĞERİNDE: `true` dönerse bu task bu akış
// için ilk kez görülüyor demektir ve akış başlatılmalıdır. `false` dönerse
// başka bir yol (veya önceki bir tarama) onu zaten işlemiştir.
//
// Kontrol veritabanı kısıtıyla yapılıyor; iki tetikleme yolu aynı anda gelse
// bile yalnızca biri `true` alır.
func (s *Store) MarkProcessed(ctx context.Context, workflowID uuid.UUID,
	issueKey, issueUpdatedAt string,
) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO workflow_processed_issues (workflow_id, issue_key, issue_updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT DO NOTHING`,
		workflowID, issueKey, issueUpdatedAt)
	if err != nil {
		return false, fmt.Errorf("task işaretlenemedi: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// UnmarkProcessed, işareti geri alır.
//
// Yalnızca bir yerde çağrılır: işaret konduktan SONRA akış başlatılamazsa.
// İşaret bırakılsaydı task "işlendi" sayılır ve bir daha hiç denenmezdi —
// oysa hiçbir şey çalışmamıştır. Sıralamanın tersi (önce başlat, sonra
// işaretle) yarışı geri getirirdi; bu yüzden işaret önce konur, hata halinde
// geri alınır.
func (s *Store) UnmarkProcessed(ctx context.Context, workflowID uuid.UUID,
	issueKey, issueUpdatedAt string,
) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM workflow_processed_issues
		WHERE workflow_id = $1 AND issue_key = $2 AND issue_updated_at = $3
		  AND workflow_run_id IS NULL`,
		workflowID, issueKey, issueUpdatedAt)
	if err != nil {
		return fmt.Errorf("task işareti geri alınamadı: %w", err)
	}
	return nil
}

// LinkProcessed, işlenmiş task'ı başlattığı akış çalışmasına bağlar.
func (s *Store) LinkProcessed(ctx context.Context, workflowID uuid.UUID,
	issueKey, issueUpdatedAt string, runID uuid.UUID,
) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE workflow_processed_issues SET workflow_run_id = $4
		WHERE workflow_id = $1 AND issue_key = $2 AND issue_updated_at = $3`,
		workflowID, issueKey, issueUpdatedAt, runID)
	if err != nil {
		return fmt.Errorf("task çalışmaya bağlanamadı: %w", err)
	}
	return nil
}

// SaveScanState, taramanın sonucunu kaydeder.
func (s *Store) SaveScanState(ctx context.Context, st ScanState) error {
	var errText *string
	if st.Error != nil && *st.Error != "" {
		errText = st.Error
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO workflow_scan_state (workflow_id, scanned_at, found, started, error)
		VALUES ($1, now(), $2, $3, $4)
		ON CONFLICT (workflow_id) DO UPDATE
		SET scanned_at = now(), found = EXCLUDED.found,
		    started = EXCLUDED.started, error = EXCLUDED.error`,
		st.WorkflowID, st.Found, st.Started, errText)
	if err != nil {
		return fmt.Errorf("tarama durumu kaydedilemedi: %w", err)
	}
	return nil
}

// ScanState, bir akışın son tarama durumunu döner.
func (s *Store) ScanState(ctx context.Context, workflowID uuid.UUID) (ScanState, error) {
	st := ScanState{WorkflowID: workflowID}
	err := s.pool.QueryRow(ctx, `
		SELECT scanned_at, found, started, error FROM workflow_scan_state
		WHERE workflow_id = $1`, workflowID).Scan(&st.ScannedAt, &st.Found, &st.Started, &st.Error)
	if err != nil {
		// Hiç taranmamış akış hata değildir; boş durum döner.
		return ScanState{WorkflowID: workflowID}, nil
	}
	return st, nil
}

// ActiveWithVersions, tetikleme taraması için etkin akışları sürümleriyle döner.
func (s *Store) ActiveWithVersions(ctx context.Context) ([]Version, []Workflow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+workflowColumns+workflowFrom+`
		WHERE w.is_active AND w.active_version_id IS NOT NULL
		ORDER BY w.created_at`)
	if err != nil {
		return nil, nil, fmt.Errorf("etkin akışlar okunamadı: %w", err)
	}
	defer rows.Close()

	var flows []Workflow
	for rows.Next() {
		w, err := scanWorkflow(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("akış taranamadı: %w", err)
		}
		flows = append(flows, w)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("etkin akışlar okunamadı: %w", err)
	}

	versions := make([]Version, 0, len(flows))
	kept := make([]Workflow, 0, len(flows))
	for _, w := range flows {
		v, err := s.Version(ctx, *w.ActiveVersionID)
		if err != nil {
			continue // sürümü okunamayan akış taramayı durdurmasın
		}
		versions = append(versions, v)
		kept = append(kept, w)
	}
	return versions, kept, nil
}
