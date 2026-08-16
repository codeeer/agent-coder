package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/paging"
	"github.com/jackc/pgx/v5"
)

// CreateRunInput, yeni bir akış çalışmasının alanları.
type CreateRunInput struct {
	Workflow Workflow
	Version  Version
	Trigger  TriggerKind
	Payload  map[string]string
	Input    string

	// ProjectID, çalışmanın koşacağı proje. uuid.Nil ise akışın varsayılanı
	// kullanılır — tetikleyicilerin (Jira, webhook, MCP) yolu budur.
	ProjectID uuid.UUID
}

// CreateRun, çalışmayı ve TÜM adımlarını 'pending' olarak kaydeder.
//
// Adımlar baştan yazılır: kullanıcı akış daha ilk saniyesindeyken kaç adım
// olduğunu ve nerede olduğunu görebilmeli. Adımlar çalıştıkça eklenseydi ekran
// akışın uzunluğunu ancak bittiğinde gösterirdi.
func (s *Store) CreateRun(ctx context.Context, in CreateRunInput) (Run, error) {
	levels, err := in.Version.Graph.Levels()
	if err != nil {
		return Run{}, err
	}

	payload := in.Payload
	if payload == nil {
		payload = map[string]string{}
	}
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return Run{}, fmt.Errorf("tetikleyici verisi kodlanamadı: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Run{}, fmt.Errorf("akış çalışması kaydedilemedi: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Proje ÇALIŞMAYA yazılır, akıştan JOIN ile okunmaz: akışın varsayılanı
	// sonradan değişirse geçmiş çalışmaların projesi de geriye dönük değişirdi.
	projectID := in.ProjectID
	if projectID == uuid.Nil {
		projectID = in.Workflow.ProjectID
	}

	var runID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO workflow_runs (workflow_id, project_id, version_id, version,
			trigger_kind, trigger_payload, input)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		in.Workflow.ID, projectID, in.Version.ID, in.Version.Version,
		string(in.Trigger), rawPayload, in.Input).Scan(&runID)
	if err != nil {
		return Run{}, fmt.Errorf("akış çalışması kaydedilemedi: %w", err)
	}

	for level, nodes := range levels {
		for _, n := range nodes {
			// Tetikleyici çalıştırılan bir adım değil, giriş noktasıdır.
			if n.Kind.IsTrigger() {
				continue
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO workflow_steps (workflow_run_id, node_id, node_kind, node_name, level)
				VALUES ($1,$2,$3,$4,$5)`,
				runID, n.ID, string(n.Kind), n.Name, level); err != nil {
				return Run{}, fmt.Errorf("akış adımı kaydedilemedi: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Run{}, fmt.Errorf("akış çalışması kaydedilemedi: %w", err)
	}
	return s.GetRun(ctx, runID)
}

// Proje ÇALIŞMANIN kendi sütunundan okunur (`r.project_id`), akıştan değil:
// akışınki yalnızca varsayılan ve sonradan değişebiliyor. JOIN akış ADI için
// duruyor — o, geçmişte de bugünkü adıyla anılmalı.
const runColumns = `
	r.id, r.workflow_id, w.name, r.project_id, p.name, r.version_id, r.version,
	r.status, r.trigger_kind, r.trigger_payload, r.input, r.error,
	r.created_at, r.started_at, r.finished_at`

// runFrom, runColumns'un beklediği tablolar. Proje adı olmadan kullanıcı, aynı
// akışın hangi çalışmasının nerede koştuğunu ayırt edemez.
const runFrom = `
	FROM workflow_runs r
	JOIN workflows w ON w.id = r.workflow_id
	JOIN projects p ON p.id = r.project_id`

func scanRun(row pgx.Row) (Run, error) {
	var (
		r       Run
		payload []byte
	)
	err := row.Scan(&r.ID, &r.WorkflowID, &r.WorkflowName, &r.ProjectID,
		&r.ProjectName, &r.VersionID, &r.Version, &r.Status, &r.TriggerKind,
		&payload, &r.Input, &r.Error, &r.CreatedAt, &r.StartedAt, &r.FinishedAt)
	if err != nil {
		return Run{}, err
	}
	r.TriggerPayload = map[string]string{}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &r.TriggerPayload)
	}
	return r, nil
}

// GetRun, çalışmayı adımlarıyla birlikte döner.
func (s *Store) GetRun(ctx context.Context, id uuid.UUID) (Run, error) {
	r, err := scanRun(s.pool.QueryRow(ctx, `SELECT `+runColumns+runFrom+`
		WHERE r.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("akış çalışması okunamadı: %w", err)
	}

	if r.Steps, err = s.steps(ctx, id); err != nil {
		return Run{}, err
	}
	for _, st := range r.Steps {
		r.CostUSD += st.CostUSD
		r.Tokens += st.Tokens
	}
	return r, nil
}

// steps, çalışmanın adımlarını sırayla döner.
//
// Maliyet ve token ADIMDA TUTULMAZ, çalıştırma kaydından okunur: aynı sayıyı iki
// yerde tutmak, ikisinin ayrışması demektir.
func (s *Store) steps(ctx context.Context, runID uuid.UUID) ([]Step, error) {
	const q = `
		SELECT st.id, st.node_id, st.node_kind, st.node_name, st.level,
			st.status, st.error, st.run_id, st.result_text, st.result_url,
			coalesce(r.agent_slug, ''), coalesce(r.model_id, ''),
			coalesce(r.cost_usd, 0),
			coalesce(r.prompt_tokens, 0) + coalesce(r.completion_tokens, 0),
			st.started_at, st.finished_at
		FROM workflow_steps st
		LEFT JOIN runs r ON r.id = st.run_id
		WHERE st.workflow_run_id = $1
		ORDER BY st.level, st.node_id`

	rows, err := s.pool.Query(ctx, q, runID)
	if err != nil {
		return nil, fmt.Errorf("akış adımları okunamadı: %w", err)
	}
	defer rows.Close()

	out := []Step{}
	for rows.Next() {
		var st Step
		if err := rows.Scan(&st.ID, &st.NodeID, &st.Kind, &st.Name, &st.Level,
			&st.Status, &st.Error, &st.RunID, &st.ResultText, &st.ResultURL,
			&st.AgentSlug, &st.ModelID,
			&st.CostUSD, &st.Tokens, &st.StartedAt, &st.FinishedAt); err != nil {
			return nil, fmt.Errorf("akış adımı taranamadı: %w", err)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// ListRunsFilter, çalışma listesi parametreleri.
type ListRunsFilter struct {
	WorkflowID *uuid.UUID
	Limit      int
	Offset     int
}

// ListRuns, akış çalışmalarını en yeniden eskiye döner (adımlar olmadan).
// İkinci dönüş değeri süzgeçten geçen toplam kayıt sayısıdır.
func (s *Store) ListRuns(ctx context.Context, f ListRunsFilter) ([]Run, int, error) {
	page := paging.Clamp(f.Limit, f.Offset)

	where := ""
	args := []any{}
	if f.WorkflowID != nil {
		args = append(args, *f.WorkflowID)
		where = " WHERE r.workflow_id = $1"
	}

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM workflow_runs r`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("akış çalışması sayısı okunamadı: %w", err)
	}

	args = append(args, page.Limit, page.Offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`SELECT `+runColumns+runFrom+where+`
		ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("akış çalışmaları listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("akış çalışması taranamadı: %w", err)
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

/* ── Durum geçişleri ─────────────────────────────────────────────────────── */

// MarkRunStarted, çalışmayı 'running' yapar.
func (s *Store) MarkRunStarted(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE workflow_runs SET status = 'running', started_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("akış başlatılamadı: %w", err)
	}
	return nil
}

// FinishRun, çalışmayı sonuçlandırır.
func (s *Store) FinishRun(ctx context.Context, id uuid.UUID, status RunStatus, cause error) error {
	var errText *string
	if cause != nil {
		msg := cause.Error()
		errText = &msg
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE workflow_runs SET status = $2, error = $3, finished_at = now() WHERE id = $1`,
		id, string(status), errText)
	if err != nil {
		return fmt.Errorf("akış sonuçlandırılamadı: %w", err)
	}
	return nil
}

// MarkStepRunning, adımı 'running' yapar ve başlangıç zamanını damgalar.
//
// Çalıştırma kaydından ÖNCE çağrılır: kayıt ancak iş bittiğinde elde ediliyor
// ve zaman damgası o an atılsaydı her adımın süresi sıfır görünürdü.
func (s *Store) MarkStepRunning(ctx context.Context, stepID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE workflow_steps SET status = 'running', started_at = now() WHERE id = $1`, stepID)
	if err != nil {
		return fmt.Errorf("adım başlatılamadı: %w", err)
	}
	return nil
}

// LinkStepRun, adımı çalıştırma kaydına bağlar.
func (s *Store) LinkStepRun(ctx context.Context, stepID, runID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE workflow_steps SET run_id = $2 WHERE id = $1`, stepID, runID)
	if err != nil {
		return fmt.Errorf("adım çalıştırmaya bağlanamadı: %w", err)
	}
	return nil
}

// FinishStep, adımı sonuçlandırır.
//
// result, agent olmayan adımların sonucudur (açılan PR, yazılan yorum). Agent
// adımlarında boştur: onların çıktısı çalıştırma kaydında durur.
func (s *Store) FinishStep(ctx context.Context, stepID uuid.UUID, status StepStatus,
	result StepOutcome, cause error,
) error {
	var errText *string
	if cause != nil {
		msg := cause.Error()
		errText = &msg
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE workflow_steps
		SET status = $2, error = $3, result_text = $4, result_url = $5, finished_at = now()
		WHERE id = $1`,
		stepID, string(status), errText, result.Output, result.URL)
	if err != nil {
		return fmt.Errorf("adım sonuçlandırılamadı: %w", err)
	}
	return nil
}

// SkipPending, henüz başlamamış adımları 'skipped' yapar.
//
// Bir adım hata verince (spec 007 K2) akış durur; kalan adımların 'pending'
// kalması onları sonsuza kadar "sırada bekliyor" gösterirdi.
func (s *Store) SkipPending(ctx context.Context, runID uuid.UUID, status StepStatus) (int, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE workflow_steps SET status = $2, finished_at = now()
		 WHERE workflow_run_id = $1 AND status = 'pending'`, runID, string(status))
	if err != nil {
		return 0, fmt.Errorf("kalan adımlar işaretlenemedi: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// RecoverInterrupted, açılışta yarım kalmış akış çalışmalarını kapatır.
//
// Sunucu bir akış sürerken ölürse kayıt 'running' kalır ve arayüzde sonsuza
// kadar dönen bir akış görünür.
func (s *Store) RecoverInterrupted(ctx context.Context) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("yarım akışlar kapatılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		UPDATE workflow_runs
		SET status = 'interrupted',
		    error = 'sunucu yeniden başladığı için akış kesildi',
		    finished_at = now()
		WHERE status IN ('pending', 'running')`)
	if err != nil {
		return 0, fmt.Errorf("yarım akışlar kapatılamadı: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE workflow_steps SET status = 'cancelled', finished_at = now()
		WHERE status IN ('pending', 'running')`); err != nil {
		return 0, fmt.Errorf("yarım adımlar kapatılamadı: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("yarım akışlar kapatılamadı: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// ActiveRuns, belirli bir akışın süren çalışmalarını sayar.
//
// Engellemek için değil UYARMAK için (spec 007 S4): aynı akışın ikinci kez
// başlatılması teknik olarak sorunsuz, ama kullanıcı bilerek yapsın.
func (s *Store) ActiveRuns(ctx context.Context, workflowID uuid.UUID) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM workflow_runs
		 WHERE workflow_id = $1 AND status IN ('pending','running')`, workflowID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("süren çalışmalar sayılamadı: %w", err)
	}
	return n, nil
}

// StepByNode, çalışmadaki bir düğümün adım kaydını döner.
func (s *Store) StepByNode(ctx context.Context, runID uuid.UUID, nodeID string) (Step, error) {
	steps, err := s.steps(ctx, runID)
	if err != nil {
		return Step{}, err
	}
	for _, st := range steps {
		if st.NodeID == nodeID {
			return st, nil
		}
	}
	return Step{}, ErrNotFound
}

/*
DeleteRun, bitmiş bir akış çalışmasını ve ondan doğan her şeyi siler.

ADIMLARIN ÇALIŞTIRMALARI AÇIKÇA SİLİNİYOR. Şemada `workflow_steps.run_id`
SET NULL: adım satırları çalışmayla birlikte kaskadla gidiyor ama adımın
ÇALIŞTIRMA kaydı yerinde kalıyor. Temizlenmezse otuz projelik bir kampanyayı
silen kullanıcı Çalıştırmalar ekranında otuz yetim satırla kalırdı — üstelik
onları tek tek de silemezdi, çünkü tekil silme "bu bir akış adımı" diyerek
reddediyor.

Sıra önemli: önce çalıştırmalar (olayları ve motor logları onlarla kaskad),
sonra çalışmanın kendisi. Ters sırada adım satırları gider ve hangi
çalıştırmaların silineceği bilgisi kaybolurdu.

SÜREN ÇALIŞMA SİLİNMEZ: container ve goroutine hâlâ canlı, kaydı silmek
onları sahipsiz bırakırdı. Kullanıcı önce iptal eder.
*/
func (s *Store) DeleteRun(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("akış çalışması silinemedi: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status RunStatus
	err = tx.QueryRow(ctx,
		`SELECT status FROM workflow_runs WHERE id = $1 FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("akış çalışması okunamadı: %w", err)
	}
	if !status.Terminal() {
		return ErrRunning
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM runs WHERE id IN (
			SELECT run_id FROM workflow_steps
			 WHERE workflow_run_id = $1 AND run_id IS NOT NULL)`, id); err != nil {
		return fmt.Errorf("adım çalıştırmaları silinemedi: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM workflow_runs WHERE id = $1`, id); err != nil {
		return fmt.Errorf("akış çalışması silinemedi: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("akış çalışması silinemedi: %w", err)
	}
	return nil
}
