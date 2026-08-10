// Package catalog, LLM sağlayıcıların model kataloglarının yerel kopyasını yönetir.
package catalog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Model, katalogdaki bir modelin dışarı verilen hali.
//
// Fiyatlar MİLYON TOKEN başına USD'ye çevrilmiş olarak durur; veritabanında
// token başına saklanır, dönüşüm yalnızca gösterim için yapılır.
type Model struct {
	ProviderID   uuid.UUID `json:"providerId"`
	ProviderName string    `json:"providerName"`

	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`

	// null = bilinmiyor. Sıfır ile karıştırılmamalı.
	ContextLength   *int `json:"contextLength"`
	MaxOutputTokens *int `json:"maxOutputTokens"`

	// Fiyatlar bilinmiyorsa 0'dır ve model ücretsiz görünür (spec 002 kullanıcı kararı).
	PromptPricePerMTok     float64 `json:"promptPricePerMTok"`
	CompletionPricePerMTok float64 `json:"completionPricePerMTok"`

	// null = bilinmiyor. "Desteklemiyor" DEĞİL — agent olarak kullanılabilirliği belirler.
	SupportsTools *bool `json:"supportsTools"`

	IsFree    bool   `json:"isFree"`
	IsPreview bool   `json:"isPreview"`
	Modality  string `json:"modality"`
}

// ProviderSync, bir sağlayıcının katalog senkron durumu.
type ProviderSync struct {
	ProviderID    uuid.UUID  `json:"providerId"`
	ProviderName  string     `json:"providerName"`
	LastAttemptAt *time.Time `json:"lastAttemptAt"`
	LastSuccessAt *time.Time `json:"lastSuccessAt"`
	ModelCount    int        `json:"modelCount"`
	LastError     *string    `json:"lastError"`
}

// Stale, bu sağlayıcının son senkronunun başarısız olup olmadığı.
func (s ProviderSync) Stale() bool {
	return s.LastError != nil && *s.LastError != ""
}

// SortField, izin verilen sıralama alanları.
type SortField string

const (
	SortName     SortField = "name"
	SortPrice    SortField = "price"
	SortContext  SortField = "context"
	SortProvider SortField = "provider"
)

// sortColumns, sıralama alanlarını SQL kolonlarına eşler.
//
// Kolon adı ASLA kullanıcı girdisinden doğrudan alınmaz — yalnızca bu haritadan
// gelir. Sıralama alanı SQL parametresi olamayacağı için tek güvenli yol budur.
var sortColumns = map[SortField]string{
	SortName:     "m.name",
	SortPrice:    "m.prompt_price",
	SortContext:  "m.context_length",
	SortProvider: "p.name",
}

// ToolsFilter, araç desteği filtresi. Üç durumlu olması gerekiyor çünkü
// "bilinmiyor" ayrı bir durum.
type ToolsFilter string

const (
	ToolsAny     ToolsFilter = ""        // filtre yok
	ToolsOnly    ToolsFilter = "yes"     // yalnızca destekleyenler
	ToolsUnknown ToolsFilter = "unknown" // yalnızca bilinmeyenler
)

// ListFilter, model listesi sorgusunun parametreleri.
type ListFilter struct {
	ProviderID *uuid.UUID
	Query      string
	Tools      ToolsFilter
	FreeOnly   bool
	Sort       SortField
	Desc       bool
	Limit      int
	Offset     int
}

// Normalize, güvenli varsayılanları uygular ve sınırları dayatır.
func (f *ListFilter) Normalize() {
	if _, ok := sortColumns[f.Sort]; !ok {
		f.Sort = SortName
	}
	if f.Tools != ToolsOnly && f.Tools != ToolsUnknown {
		f.Tools = ToolsAny
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	// Üst sınır, model seçicisinin tüm katalogu tek istekte alabilmesi için
	// geniş tutuluyor: yüzlerce model arasında arama istemci tarafında
	// yapılıyor ve her tuş vuruşunda ağa çıkmak arama deneyimini bozardı.
	// Kayıt başına yaklaşık 200 bayt; 500 kayıt ~100 KB.
	if f.Limit > maxPageSize {
		f.Limit = maxPageSize
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	f.Query = strings.TrimSpace(f.Query)
}

// Store, model kataloğunun veritabanı erişimi.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// tokensPerM, token başına fiyatı milyon token başına çevirmek için.
const tokensPerM = 1_000_000

// maxPageSize, tek istekte dönebilecek azami model sayısı.
const maxPageSize = 500

// List, filtreye uyan modelleri ve toplam sayıyı döner.
func (s *Store) List(ctx context.Context, f ListFilter) (models []Model, total int, err error) {
	f.Normalize()

	// Değerler her zaman $N parametresi olarak gider; SQL metnine hiçbir
	// kullanıcı girdisi gömülmez.
	var (
		conds []string
		args  []any
	)

	if f.ProviderID != nil {
		args = append(args, *f.ProviderID)
		conds = append(conds, fmt.Sprintf("m.provider_id = $%d", len(args)))
	}
	if f.Query != "" {
		args = append(args, "%"+f.Query+"%")
		conds = append(conds, fmt.Sprintf("(m.id ILIKE $%d OR m.name ILIKE $%d)", len(args), len(args)))
	}
	switch f.Tools {
	case ToolsOnly:
		conds = append(conds, "m.supports_tools IS TRUE")
	case ToolsUnknown:
		conds = append(conds, "m.supports_tools IS NULL")
	}
	if f.FreeOnly {
		conds = append(conds, "m.is_free")
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	countQuery := `SELECT count(*) FROM models m
		JOIN llm_providers p ON p.id = m.provider_id ` + where
	if err := s.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("model sayısı alınamadı: %w", err)
	}

	direction := "ASC"
	if f.Desc {
		direction = "DESC"
	}

	args = append(args, f.Limit, f.Offset)
	query := fmt.Sprintf(`
		SELECT m.provider_id, p.name,
		       m.id, m.name, m.description, m.context_length, m.max_output_tokens,
		       m.prompt_price, m.completion_price, m.supports_tools,
		       m.is_free, m.is_preview, m.modality
		FROM models m
		JOIN llm_providers p ON p.id = m.provider_id
		%s
		ORDER BY %s %s NULLS LAST, p.name ASC, m.id ASC
		LIMIT $%d OFFSET $%d`,
		where, sortColumns[f.Sort], direction, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("modeller listelenemedi: %w", err)
	}
	defer rows.Close()

	models = []Model{}
	for rows.Next() {
		var (
			m                          Model
			promptPrice, completePrice float64
			isFree                     *bool
		)
		if err := rows.Scan(
			&m.ProviderID, &m.ProviderName,
			&m.ID, &m.Name, &m.Description, &m.ContextLength, &m.MaxOutputTokens,
			&promptPrice, &completePrice, &m.SupportsTools,
			&isFree, &m.IsPreview, &m.Modality,
		); err != nil {
			return nil, 0, fmt.Errorf("model taranamadı: %w", err)
		}
		m.PromptPricePerMTok = promptPrice * tokensPerM
		m.CompletionPricePerMTok = completePrice * tokensPerM
		m.IsFree = isFree != nil && *isFree
		models = append(models, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("modeller okunamadı: %w", err)
	}
	return models, total, nil
}

// SyncStatus, tüm sağlayıcıların senkron durumunu döner.
//
// Arayüz bunlara bakarak hangi sağlayıcının güncellenemediğini gösterir
// (spec 002 H2: biri düşerse diğerleri etkilenmez).
func (s *Store) SyncStatus(ctx context.Context) ([]ProviderSync, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.name, s.last_attempt_at, s.last_success_at,
		       s.model_count, s.last_error
		FROM llm_providers p
		LEFT JOIN provider_sync s ON s.provider_id = p.id
		ORDER BY p.name`)
	if err != nil {
		return nil, fmt.Errorf("senkron durumu okunamadı: %w", err)
	}
	defer rows.Close()

	out := []ProviderSync{}
	for rows.Next() {
		var (
			ps         ProviderSync
			modelCount *int
		)
		if err := rows.Scan(&ps.ProviderID, &ps.ProviderName, &ps.LastAttemptAt,
			&ps.LastSuccessAt, &modelCount, &ps.LastError); err != nil {
			return nil, fmt.Errorf("senkron durumu taranamadı: %w", err)
		}
		if modelCount != nil {
			ps.ModelCount = *modelCount
		}
		out = append(out, ps)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("senkron durumu okunamadı: %w", err)
	}
	return out, nil
}
