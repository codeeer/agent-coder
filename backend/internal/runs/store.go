// Package runs, agent çalıştırmalarını kaydeder ve yürütür.
package runs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/runner"
)

// Status, bir çalıştırmanın durumu.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusTimeout   Status = "timeout"
	// StatusInterrupted, sunucu çalışma ortasında yeniden başladığında verilir.
	// Bu durum olmadan kayıtlar sonsuza kadar "çalışıyor" görünürdü.
	StatusInterrupted Status = "interrupted"
)

// Terminal, çalıştırmanın bitmiş olup olmadığı.
func (s Status) Terminal() bool {
	return s != StatusPending && s != StatusRunning
}

var (
	ErrNotFound      = errors.New("çalıştırma bulunamadı")
	ErrNotRunning    = errors.New("çalıştırma zaten bitmiş")
	ErrNoChanges     = errors.New("gönderilecek değişiklik yok")
	ErrAlreadyPushed = errors.New("bu çalıştırma zaten gönderilmiş")
	ErrNoGitAccess   = errors.New("projede git erişimi tanımlı değil")
	ErrActive        = errors.New("çalıştırma hâlâ sürüyor")
	ErrWorkflowStep  = errors.New("çalıştırma bir akış adımı")
)

// Run, bir agent çalıştırması.
//
// agent_prompt ve model_id ANLIK KOPYADIR: agent veya model sonradan
// değiştirilse bile geçmiş kayıt neyle çalıştığını doğru gösterir.
type Run struct {
	ID         uuid.UUID  `json:"id"`
	ProjectID  uuid.UUID  `json:"projectId"`
	AgentID    uuid.UUID  `json:"agentId"`
	ProviderID *uuid.UUID `json:"providerId"`

	ProjectName  string `json:"projectName"`
	AgentSlug    string `json:"agentSlug"`
	AgentPrompt  string `json:"agentPrompt"`
	ProviderSlug string `json:"providerSlug"`
	ModelID      string `json:"modelId"`

	Branch string `json:"branch"`
	Task   string `json:"task"`

	Status Status              `json:"status"`
	Error  *string             `json:"error"`
	Output string              `json:"output"`
	Diff   string              `json:"diff"`
	Files  []runner.FileChange `json:"files"`

	PromptTokens     int     `json:"promptTokens"`
	CompletionTokens int     `json:"completionTokens"`
	CostUSD          float64 `json:"costUsd"`

	// Akış bağlamı (spec 007): bu çalıştırma bir akış adımıysa dolu.
	//
	// Kolon olarak DEĞİL, workflow_steps üzerinden birleştirilerek geliyor:
	// aynı bilgiyi iki yerde tutmak ikisinin ayrışması demek olurdu.
	WorkflowRunID *uuid.UUID `json:"workflowRunId"`
	WorkflowName  string     `json:"workflowName"`
	StepName      string     `json:"stepName"`

	PushedBranch *string    `json:"pushedBranch"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
}

// HasChanges, çalıştırmanın kod değişikliği üretip üretmediği.
func (r Run) HasChanges() bool { return r.Diff != "" }

// Store, çalıştırma deposu.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const runColumns = `
	r.id, r.project_id, r.agent_id, r.provider_id,
	p.name,
	r.agent_slug, r.agent_prompt, r.provider_slug, r.model_id,
	r.branch, r.task,
	r.status, r.error, r.output, r.diff, r.files,
	r.prompt_tokens, r.completion_tokens, r.cost_usd,
	ws.workflow_run_id, coalesce(wf.name, ''),
	coalesce(nullif(ws.node_name, ''), ws.node_id, ''),
	r.pushed_branch, r.created_at, r.started_at, r.finished_at`

// workflowJoin, çalıştırmanın hangi akışın adımı olduğunu bulur.
// Adım değilse tüm alanlar boş kalır.
const workflowJoin = `
	LEFT JOIN workflow_steps ws ON ws.run_id = r.id
	LEFT JOIN workflow_runs wr ON wr.id = ws.workflow_run_id
	LEFT JOIN workflows wf ON wf.id = wr.workflow_id`

func scanRun(row pgx.Row) (Run, error) {
	var (
		r       Run
		rawFile []byte
	)
	err := row.Scan(&r.ID, &r.ProjectID, &r.AgentID, &r.ProviderID,
		&r.ProjectName,
		&r.AgentSlug, &r.AgentPrompt, &r.ProviderSlug, &r.ModelID,
		&r.Branch, &r.Task,
		&r.Status, &r.Error, &r.Output, &r.Diff, &rawFile,
		&r.PromptTokens, &r.CompletionTokens, &r.CostUSD,
		&r.WorkflowRunID, &r.WorkflowName, &r.StepName,
		&r.PushedBranch, &r.CreatedAt, &r.StartedAt, &r.FinishedAt)
	if err != nil {
		return Run{}, err
	}
	if len(rawFile) > 0 {
		_ = json.Unmarshal(rawFile, &r.Files)
	}
	return r, nil
}

// CreateInput, yeni çalıştırma kaydının alanları.
type CreateInput struct {
	ProjectID  uuid.UUID
	AgentID    uuid.UUID
	ProviderID *uuid.UUID

	AgentSlug    string
	AgentPrompt  string
	ProviderSlug string
	ModelID      string

	Branch string
	Task   string
}

// Create, çalıştırmayı 'pending' olarak kaydeder.
func (s *Store) Create(ctx context.Context, in CreateInput) (Run, error) {
	const q = `
		WITH inserted AS (
			INSERT INTO runs (project_id, agent_id, provider_id,
				agent_slug, agent_prompt, provider_slug, model_id, branch, task)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING *
		)
		SELECT ` + runColumns + `
		FROM inserted r JOIN projects p ON p.id = r.project_id` + workflowJoin

	return scanRun(s.pool.QueryRow(ctx, q,
		in.ProjectID, in.AgentID, in.ProviderID,
		in.AgentSlug, in.AgentPrompt, in.ProviderSlug, in.ModelID,
		in.Branch, in.Task))
}

// Get, tek bir çalıştırmayı döner.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Run, error) {
	const q = `SELECT ` + runColumns + `
		FROM runs r JOIN projects p ON p.id = r.project_id` + workflowJoin + `
		WHERE r.id = $1`

	r, err := scanRun(s.pool.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	if err != nil {
		return Run{}, fmt.Errorf("çalıştırma okunamadı: %w", err)
	}
	return r, nil
}

// ListFilter, çalıştırma listesi parametreleri.
type ListFilter struct {
	ProjectID *uuid.UUID
	Limit     int
	Offset    int
}

// List, çalıştırmaları en yeniden eskiye döner.
//
// Diff ve çıktı gövdesi listede TAŞINMAZ: yüzlerce kilobayt olabilirler ve
// liste ekranında gösterilmezler.
func (s *Store) List(ctx context.Context, f ListFilter) ([]Run, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	where := ""
	args := []any{}
	if f.ProjectID != nil {
		args = append(args, *f.ProjectID)
		where = "WHERE r.project_id = $1"
	}

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM runs r `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("çalıştırma sayısı okunamadı: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	q := fmt.Sprintf(`
		SELECT r.id, r.project_id, r.agent_id, r.provider_id, p.name,
		       r.agent_slug, '', r.provider_slug, r.model_id,
		       r.branch, r.task,
		       r.status, r.error, '', '', r.files,
		       r.prompt_tokens, r.completion_tokens, r.cost_usd,
		       ws.workflow_run_id, coalesce(wf.name, ''),
		       coalesce(nullif(ws.node_name, ''), ws.node_id, ''),
		       r.pushed_branch, r.created_at, r.started_at, r.finished_at
		FROM runs r JOIN projects p ON p.id = r.project_id
		LEFT JOIN workflow_steps ws ON ws.run_id = r.id
		LEFT JOIN workflow_runs wr ON wr.id = ws.workflow_run_id
		LEFT JOIN workflows wf ON wf.id = wr.workflow_id
		%s
		ORDER BY r.created_at DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("çalıştırmalar listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("çalıştırma taranamadı: %w", err)
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// MarkRunning, çalıştırmayı başlatılmış olarak işaretler.
func (s *Store) MarkRunning(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE runs SET status = 'running', started_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("çalıştırma başlatılamadı: %w", err)
	}
	return nil
}

// Finish, çalıştırmayı sonuçlandırır.
//
// result nil olabilir (hata yolu); o durumda yalnızca durum ve hata yazılır.
// Maliyet BAŞARISIZLIKTA DA kaydedilir: harcama gerçekleşmiştir.
func (s *Store) Finish(ctx context.Context, id uuid.UUID, status Status,
	result *runner.Result, cause error) error {

	var (
		output, diff string
		files        = []byte("[]")
		pTok, cTok   int
		cost         float64
	)
	if result != nil {
		output, diff = result.Output, result.Diff
		pTok, cTok, cost = result.PromptTokens, result.CompletionTokens, result.CostUSD
		if b, err := json.Marshal(result.Files); err == nil && result.Files != nil {
			files = b
		}
	}

	var errText *string
	if cause != nil {
		msg := cause.Error()
		errText = &msg
	}

	const q = `
		UPDATE runs SET status = $2, error = $3, output = $4, diff = $5, files = $6,
			prompt_tokens = $7, completion_tokens = $8, cost_usd = $9,
			finished_at = now()
		WHERE id = $1`

	if _, err := s.pool.Exec(ctx, q, id, string(status), errText,
		output, diff, files, pTok, cTok, cost); err != nil {
		return fmt.Errorf("çalıştırma sonuçlandırılamadı: %w", err)
	}
	return nil
}

// SetPushedBranch, gönderilen branch adını kaydeder.
func (s *Store) SetPushedBranch(ctx context.Context, id uuid.UUID, branch string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE runs SET pushed_branch = $2 WHERE id = $1`, id, branch)
	if err != nil {
		return fmt.Errorf("branch adı kaydedilemedi: %w", err)
	}
	return nil
}

// RecoverInterrupted, açılışta yarım kalmış kayıtları kapatır.
//
// Sunucu bir çalıştırma sürerken ölürse kayıt 'running' kalır ve arayüzde
// sonsuza kadar dönen bir iş görünür. Açılışta hepsi 'interrupted' yapılır.
func (s *Store) RecoverInterrupted(ctx context.Context) (int, error) {
	const q = `
		UPDATE runs
		SET status = 'interrupted',
		    error = 'sunucu yeniden başladığı için çalışma kesildi',
		    finished_at = now()
		WHERE status IN ('pending', 'running')`

	tag, err := s.pool.Exec(ctx, q)
	if err != nil {
		return 0, fmt.Errorf("yarım çalıştırmalar kapatılamadı: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// AppendEvent, bir ilerleme olayını kaydeder ve sıra numarasını döner.
//
// Sıra numarası veritabanında üretilir: aynı çalıştırma için birden fazla
// yazıcı olmasa da, numaranın kayıtla tutarlı olması SSE'nin geçmişi doğru
// sırada göndermesini garantiler.
func (s *Store) AppendEvent(ctx context.Context, runID uuid.UUID, level, message string) (int, time.Time, error) {
	const q = `
		INSERT INTO run_events (run_id, seq, level, message)
		VALUES ($1, (SELECT coalesce(max(seq), 0) + 1 FROM run_events WHERE run_id = $1), $2, $3)
		RETURNING seq, ts`

	var (
		seq int
		ts  time.Time
	)
	if err := s.pool.QueryRow(ctx, q, runID, level, message).Scan(&seq, &ts); err != nil {
		return 0, time.Time{}, fmt.Errorf("olay kaydedilemedi: %w", err)
	}
	return seq, ts, nil
}

// StoredEvent, veritabanındaki bir olay.
type StoredEvent struct {
	Seq     int       `json:"seq"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	TS      time.Time `json:"ts"`
}

// Events, bir çalıştırmanın olay geçmişini sırayla döner.
func (s *Store) Events(ctx context.Context, runID uuid.UUID) ([]StoredEvent, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT seq, level, message, ts FROM run_events WHERE run_id = $1 ORDER BY seq`, runID)
	if err != nil {
		return nil, fmt.Errorf("olaylar okunamadı: %w", err)
	}
	defer rows.Close()

	out := []StoredEvent{}
	for rows.Next() {
		var e StoredEvent
		if err := rows.Scan(&e.Seq, &e.Level, &e.Message, &e.TS); err != nil {
			return nil, fmt.Errorf("olay taranamadı: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Delete, bir çalıştırma kaydını kalıcı olarak siler.
//
// GERİ ALINAMAZ ve yalnızca kaydı silmez: maliyet, token ve değişen dosya
// sayıları doğrudan bu satırın sütunlarında duruyor, ayrı bir özet tablo yok.
// Yani rapor ekranındaki geçmiş rakamlar da o kadar azalır. Kullanıcıya bunu
// silmeden önce söylemek arayüzün işi.
//
// Olay geçmişi `run_events` üzerinden CASCADE ile gider (migration 000003).
//
// İki durumda reddedilir:
//
//   - AKIŞ ADIMI ise. `workflow_steps.run_id` ON DELETE SET NULL: kayıt gitse
//     de adım "başarılı" görünmeye devam eder ama agent'ı, modeli, maliyeti ve
//     token'ı boşalır — akış geçmişinde sessiz bir delik açılır. Kullanıcı akış
//     çalışmasını silmeli, tek adımını değil.
//   - SÜRÜYORSA. Burada DB durumuna bakılır ama bu tek başına yetmez: iptal
//     asenkron olduğu için `Manager` katmanı ayrıca bellekteki canlı iş
//     listesine de bakar (bkz. Manager.Running).
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	var (
		status     Status
		isWorkflow bool
	)
	err := s.pool.QueryRow(ctx, `
		SELECT r.status,
		       EXISTS (SELECT 1 FROM workflow_steps st WHERE st.run_id = r.id)
		  FROM runs r WHERE r.id = $1`, id).Scan(&status, &isWorkflow)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("çalıştırma okunamadı: %w", err)
	}
	if isWorkflow {
		return ErrWorkflowStep
	}
	if !status.Terminal() {
		return ErrActive
	}

	tag, err := s.pool.Exec(ctx, `DELETE FROM runs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("çalıştırma silinemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
