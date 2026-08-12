package workflow

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/paging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound   = errors.New("akış bulunamadı")
	ErrNoVersion  = errors.New("akışın kayıtlı bir sürümü yok")
	ErrRunning    = errors.New("akış zaten çalışıyor")
	ErrNotRunning = errors.New("akış çalışması zaten bitmiş")
)

// RunStatus, bir akış çalışmasının durumu.
type RunStatus string

const (
	RunPending     RunStatus = "pending"
	RunRunning     RunStatus = "running"
	RunSucceeded   RunStatus = "succeeded"
	RunFailed      RunStatus = "failed"
	RunCancelled   RunStatus = "cancelled"
	RunInterrupted RunStatus = "interrupted"
)

// Terminal, çalışmanın bitmiş olup olmadığı.
func (s RunStatus) Terminal() bool { return s != RunPending && s != RunRunning }

// StepStatus, bir adımın durumu.
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepSucceeded StepStatus = "succeeded"
	StepFailed    StepStatus = "failed"
	// StepSkipped, önceki bir adım hata verdiği için hiç çalışmamış adım.
	StepSkipped   StepStatus = "skipped"
	StepCancelled StepStatus = "cancelled"
)

// TriggerKind, akışın nasıl başlatıldığı.
type TriggerKind string

const (
	TriggerManual  TriggerKind = "manual"
	TriggerWebhook TriggerKind = "webhook"
	TriggerJira    TriggerKind = "jira"
	TriggerMCP     TriggerKind = "mcp"
)

// Workflow, kaydedilmiş bir akış.
type Workflow struct {
	ID          uuid.UUID `json:"id"`
	ProjectID   uuid.UUID `json:"projectId"`
	ProjectName string    `json:"projectName"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"isActive"`

	ActiveVersionID *uuid.UUID `json:"activeVersionId"`
	ActiveVersion   *int       `json:"activeVersion"`

	// HookToken, dışarıdan tetikleme adresinin gizli parçası.
	HookToken string `json:"hookToken"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// LastRun, liste ekranı için son çalışmanın özeti.
	LastRun *RunSummary `json:"lastRun"`
}

// RunSummary, liste ekranlarında gösterilen kısa çalışma bilgisi.
type RunSummary struct {
	ID         uuid.UUID  `json:"id"`
	Status     RunStatus  `json:"status"`
	CreatedAt  time.Time  `json:"createdAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// Version, akışın değişmez bir sürümü.
type Version struct {
	ID         uuid.UUID `json:"id"`
	WorkflowID uuid.UUID `json:"workflowId"`
	Version    int       `json:"version"`
	Graph      Graph     `json:"graph"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Step, bir akış çalışmasının tek adımı.
type Step struct {
	ID     uuid.UUID  `json:"id"`
	NodeID string     `json:"nodeId"`
	Kind   string     `json:"nodeKind"`
	Name   string     `json:"nodeName"`
	Level  int        `json:"level"`
	Status StepStatus `json:"status"`
	Error  *string    `json:"error"`

	// RunID, adım gerçekten çalıştıysa karşılık gelen çalıştırma kaydı.
	RunID *uuid.UUID `json:"runId"`

	// ResultText / ResultURL, agent OLMAYAN adımların sonucu (açılan PR,
	// yazılan yorum). Agent adımının çıktısı çalıştırma kaydında durduğu için
	// burada boş kalır.
	ResultText string `json:"resultText"`
	ResultURL  string `json:"resultUrl"`

	// Aşağıdakiler çalıştırma kaydından gelir; adım hiç çalışmadıysa sıfırdır.
	AgentSlug string  `json:"agentSlug"`
	ModelID   string  `json:"modelId"`
	CostUSD   float64 `json:"costUsd"`
	Tokens    int64   `json:"tokens"`

	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// Run, bir akış çalışması.
type Run struct {
	ID           uuid.UUID `json:"id"`
	WorkflowID   uuid.UUID `json:"workflowId"`
	WorkflowName string    `json:"workflowName"`
	// Proje çalışmanın kendi kaydından gelir; akışınki yalnızca varsayılan.
	// Aynı akış farklı projelerde koşabildiği için hangisinde koştuğu ekranda
	// görünmek zorunda — bu yüzden ad da taşınıyor.
	ProjectID   uuid.UUID `json:"projectId"`
	ProjectName string    `json:"projectName"`
	VersionID   uuid.UUID `json:"versionId"`
	Version     int       `json:"version"`

	Status         RunStatus         `json:"status"`
	TriggerKind    TriggerKind       `json:"triggerKind"`
	TriggerPayload map[string]string `json:"triggerPayload"`
	Input          string            `json:"input"`
	Error          *string           `json:"error"`

	CreatedAt  time.Time  `json:"createdAt"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`

	Steps []Step `json:"steps"`

	// Toplamlar adımların çalıştırma kayıtlarından hesaplanır — ayrı bir yerde
	// tutulsaydı iki kaynak olur ve ayrışabilirdi.
	CostUSD float64 `json:"costUsd"`
	Tokens  int64   `json:"tokens"`
}

// Store, akış deposu.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

/* ── Akış tanımı ─────────────────────────────────────────────────────────── */

// CreateInput, yeni akışın alanları.
type CreateInput struct {
	ProjectID   uuid.UUID
	Name        string
	Description string
}

// Create, akışı ve tetikleme adresini oluşturur.
//
// Hook token'ı akışla BİRLİKTE üretilir: sonradan "ilk kullanımda oluştur"
// demek, iki isteğin aynı anda gelmesi halinde yarış demek olurdu.
func (s *Store) Create(ctx context.Context, in CreateInput) (Workflow, error) {
	token, err := newToken()
	if err != nil {
		return Workflow{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Workflow{}, fmt.Errorf("akış oluşturulamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO workflows (project_id, name, description) VALUES ($1,$2,$3) RETURNING id`,
		in.ProjectID, in.Name, in.Description).Scan(&id)
	if err != nil {
		return Workflow{}, fmt.Errorf("akış oluşturulamadı: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO workflow_hooks (workflow_id, token) VALUES ($1,$2)`, id, token); err != nil {
		return Workflow{}, fmt.Errorf("tetikleme adresi oluşturulamadı: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Workflow{}, fmt.Errorf("akış oluşturulamadı: %w", err)
	}
	return s.Get(ctx, id)
}

const workflowColumns = `
	w.id, w.project_id, p.name, w.name, w.description, w.is_active,
	w.active_version_id, av.version, h.token, w.created_at, w.updated_at`

const workflowFrom = `
	FROM workflows w
	JOIN projects p ON p.id = w.project_id
	LEFT JOIN workflow_versions av ON av.id = w.active_version_id
	LEFT JOIN workflow_hooks h ON h.workflow_id = w.id`

func scanWorkflow(row pgx.Row) (Workflow, error) {
	var (
		w     Workflow
		token *string
	)
	err := row.Scan(&w.ID, &w.ProjectID, &w.ProjectName, &w.Name, &w.Description,
		&w.IsActive, &w.ActiveVersionID, &w.ActiveVersion, &token,
		&w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return Workflow{}, err
	}
	if token != nil {
		w.HookToken = *token
	}
	return w, nil
}

// Get, tek bir akışı döner.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Workflow, error) {
	w, err := scanWorkflow(s.pool.QueryRow(ctx,
		`SELECT `+workflowColumns+workflowFrom+` WHERE w.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Workflow{}, ErrNotFound
	}
	if err != nil {
		return Workflow{}, fmt.Errorf("akış okunamadı: %w", err)
	}
	return w, nil
}

// List, akışları en yeniden eskiye döner; her birinin son çalışmasıyla.
// İkinci dönüş değeri süzgeçten geçen toplam kayıt sayısıdır.
func (s *Store) List(ctx context.Context, projectID *uuid.UUID, page paging.Page) (
	[]Workflow, int, error,
) {
	where := ""
	args := []any{}
	if projectID != nil {
		args = append(args, *projectID)
		where = " WHERE w.project_id = $1"
	}

	var total int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM workflows w`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("akış sayısı okunamadı: %w", err)
	}

	args = append(args, page.Limit, page.Offset)
	rows, err := s.pool.Query(ctx,
		fmt.Sprintf(`SELECT `+workflowColumns+workflowFrom+where+
			` ORDER BY w.created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)),
		args...)
	if err != nil {
		return nil, 0, fmt.Errorf("akışlar listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Workflow{}
	ids := []uuid.UUID{}
	for rows.Next() {
		w, err := scanWorkflow(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("akış taranamadı: %w", err)
		}
		out = append(out, w)
		ids = append(ids, w.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("akışlar listelenemedi: %w", err)
	}
	if len(out) == 0 {
		return out, total, nil
	}

	// Son çalışmalar TEK sorguda alınır; akış başına sorgu atmak N+1 olurdu.
	last, err := s.lastRuns(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		if r, ok := last[out[i].ID]; ok {
			summary := r
			out[i].LastRun = &summary
		}
	}
	return out, total, nil
}

func (s *Store) lastRuns(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]RunSummary, error) {
	const q = `
		SELECT DISTINCT ON (workflow_id)
			workflow_id, id, status, created_at, finished_at
		FROM workflow_runs
		WHERE workflow_id = ANY($1)
		ORDER BY workflow_id, created_at DESC`

	rows, err := s.pool.Query(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("son çalışmalar okunamadı: %w", err)
	}
	defer rows.Close()

	out := map[uuid.UUID]RunSummary{}
	for rows.Next() {
		var (
			wid uuid.UUID
			r   RunSummary
		)
		if err := rows.Scan(&wid, &r.ID, &r.Status, &r.CreatedAt, &r.FinishedAt); err != nil {
			return nil, fmt.Errorf("son çalışma taranamadı: %w", err)
		}
		out[wid] = r
	}
	return out, rows.Err()
}

// UpdateInput, akışın düzenlenebilir alanları.
type UpdateInput struct {
	Name        string
	Description string
	IsActive    bool
}

// Update, akışın üst bilgilerini değiştirir. Graf ayrı bir sürümle kaydedilir.
func (s *Store) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (Workflow, error) {
	tag, err := s.pool.Exec(ctx,
		`UPDATE workflows SET name = $2, description = $3, is_active = $4, updated_at = now()
		 WHERE id = $1`, id, in.Name, in.Description, in.IsActive)
	if err != nil {
		return Workflow{}, fmt.Errorf("akış güncellenemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return Workflow{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Delete, akışı ve ona bağlı her şeyi siler: sürümler, çalışmalar, adımlar,
// hook ve Jira tarama durumu (hepsi CASCADE).
//
// SÜREN ÇALIŞMASI OLAN AKIŞ SİLİNMEZ. Silinseydi kayıt giderdi ama motorun
// goroutine'i yaşamaya devam ederdi: sonraki `MarkStepRunning`/`FinishRun`
// yazmaları sessizce 0 satır etkiler, container ise kaydı olmayan bir işi
// çalıştırmayı sürdürürdü. Kullanıcı önce durdurur, sonra siler.
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	active, err := s.ActiveRuns(ctx, id)
	if err != nil {
		return err
	}
	if active > 0 {
		return ErrRunning
	}

	tag, err := s.pool.Exec(ctx, `DELETE FROM workflows WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("akış silinemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RunCounts, akış başına toplam çalışma sayısı — silmeden önce kullanıcıya ne
// kaybedeceği söylenebilsin diye.
//
// TEK sorgu: `lastRuns` ile aynı toplu kalıp. Akış başına sorgu atmak listede
// N+1 olurdu.
func (s *Store) RunCounts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int, error) {
	out := map[uuid.UUID]int{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT workflow_id, count(*) FROM workflow_runs
		  WHERE workflow_id = ANY($1) GROUP BY workflow_id`, ids)
	if err != nil {
		return nil, fmt.Errorf("çalışma sayıları okunamadı: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id uuid.UUID
			n  int
		)
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("çalışma sayısı taranamadı: %w", err)
		}
		out[id] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("çalışma sayıları okunamadı: %w", err)
	}
	return out, nil
}

/* ── Sürümler ────────────────────────────────────────────────────────────── */

// SaveVersion, yeni bir sürüm kaydeder ve etkin sürüm yapar.
//
// Graf ÖNCE doğrulanır: geçersiz bir akış veritabanına hiç girmez. Kaydedip
// çalıştırma anında patlamak, kullanıcıya hatayı en pahalı yerde söylemek olurdu.
func (s *Store) SaveVersion(ctx context.Context, workflowID uuid.UUID, g Graph) (Version, error) {
	if err := g.Validate(); err != nil {
		return Version{}, err
	}

	raw, err := json.Marshal(g)
	if err != nil {
		return Version{}, fmt.Errorf("akış tanımı kodlanamadı: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, fmt.Errorf("sürüm kaydedilemedi: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Sürüm numarası kilit altında hesaplanır; iki eşzamanlı kayıt aynı numarayı
	// almasın diye (UNIQUE kısıtı zaten var, bu onu hataya düşmeden karşılar).
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT true FROM workflows WHERE id = $1 FOR UPDATE`, workflowID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Version{}, ErrNotFound
		}
		return Version{}, fmt.Errorf("sürüm kaydedilemedi: %w", err)
	}

	var v Version
	err = tx.QueryRow(ctx, `
		INSERT INTO workflow_versions (workflow_id, version, graph)
		VALUES ($1,
			(SELECT coalesce(max(version), 0) + 1 FROM workflow_versions WHERE workflow_id = $1),
			$2)
		RETURNING id, workflow_id, version, created_at`,
		workflowID, raw).Scan(&v.ID, &v.WorkflowID, &v.Version, &v.CreatedAt)
	if err != nil {
		return Version{}, fmt.Errorf("sürüm kaydedilemedi: %w", err)
	}
	v.Graph = g

	if _, err := tx.Exec(ctx,
		`UPDATE workflows SET active_version_id = $2, updated_at = now() WHERE id = $1`,
		workflowID, v.ID); err != nil {
		return Version{}, fmt.Errorf("etkin sürüm ayarlanamadı: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Version{}, fmt.Errorf("sürüm kaydedilemedi: %w", err)
	}
	return v, nil
}

// Version, tek bir sürümü döner.
func (s *Store) Version(ctx context.Context, id uuid.UUID) (Version, error) {
	var (
		v   Version
		raw []byte
	)
	err := s.pool.QueryRow(ctx,
		`SELECT id, workflow_id, version, graph, created_at FROM workflow_versions WHERE id = $1`,
		id).Scan(&v.ID, &v.WorkflowID, &v.Version, &raw, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrNotFound
	}
	if err != nil {
		return Version{}, fmt.Errorf("sürüm okunamadı: %w", err)
	}
	if v.Graph, err = ParseGraph(raw); err != nil {
		return Version{}, err
	}
	return v, nil
}

// ActiveVersion, akışın çalıştırılacak sürümünü döner.
func (s *Store) ActiveVersion(ctx context.Context, workflowID uuid.UUID) (Version, error) {
	w, err := s.Get(ctx, workflowID)
	if err != nil {
		return Version{}, err
	}
	if w.ActiveVersionID == nil {
		return Version{}, ErrNoVersion
	}
	return s.Version(ctx, *w.ActiveVersionID)
}

/* ── Tetikleme adresi ────────────────────────────────────────────────────── */

// newToken, tahmin edilemez bir tetikleme anahtarı üretir.
func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("tetikleme anahtarı üretilemedi: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// RotateHook, tetikleme adresini yeniler. Eski adres anında geçersiz olur.
func (s *Store) RotateHook(ctx context.Context, workflowID uuid.UUID) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO workflow_hooks (workflow_id, token) VALUES ($1,$2)
		ON CONFLICT (workflow_id) DO UPDATE SET token = EXCLUDED.token`,
		workflowID, token)
	if err != nil {
		return "", fmt.Errorf("tetikleme adresi yenilenemedi: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return "", ErrNotFound
	}
	return token, nil
}

// ByHookToken, tetikleme anahtarına karşılık gelen akışı döner.
func (s *Store) ByHookToken(ctx context.Context, token string) (Workflow, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT workflow_id FROM workflow_hooks WHERE token = $1`, token).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Workflow{}, ErrNotFound
	}
	if err != nil {
		return Workflow{}, fmt.Errorf("tetikleme adresi okunamadı: %w", err)
	}
	return s.Get(ctx, id)
}
