package catalog

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/llm"
)

// SyncInterval, kendiliğinden yenileme aralığı (spec 001 kararı).
const SyncInterval = 24 * time.Hour

// Syncer, sağlayıcıların model kataloglarını indirip veritabanına yazar.
type Syncer struct {
	// rt, sağlayıcı çağrılarının taşıyıcısı (kurumsal sertifika için).
	rt        http.RoundTripper
	pool      *pgxpool.Pool
	providers *llm.Store
}

// NewSyncer yeni senkronlayıcı üretir.
func NewSyncer(pool *pgxpool.Pool, providers *llm.Store, rt http.RoundTripper) *Syncer {
	return &Syncer{pool: pool, providers: providers, rt: rt}
}

// Result, tek bir sağlayıcının senkron sonucu.
type Result struct {
	ProviderID   uuid.UUID `json:"providerId"`
	ProviderName string    `json:"providerName"`
	OK           bool      `json:"ok"`
	Count        int       `json:"count,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// SyncOne, tek bir sağlayıcının katalogunu günceller.
//
// Başarısızlıkta o sağlayıcının mevcut modelleri OLDUĞU GİBİ KALIR ve hata
// provider_sync'e yazılır; kullanıcı eski listeyi "güncellenemedi" uyarısıyla görür.
func (s *Syncer) SyncOne(ctx context.Context, p llm.Provider) (int, error) {
	key, err := s.providers.Reveal(ctx, p.ID)
	if err != nil {
		s.recordFailure(ctx, p.ID, err)
		return 0, err
	}

	client, err := llm.NewClient(p, s.rt)
	if err != nil {
		s.recordFailure(ctx, p.ID, err)
		return 0, err
	}

	models, err := client.ListModels(ctx, key)
	if err != nil {
		s.recordFailure(ctx, p.ID, err)
		return 0, err
	}

	count, err := s.replaceProviderModels(ctx, p.ID, models)
	if err != nil {
		s.recordFailure(ctx, p.ID, err)
		return 0, err
	}

	s.recordSuccess(ctx, p.ID, count)
	return count, nil
}

// SyncAll, tüm sağlayıcıları sırayla günceller ve her birinin sonucunu döner.
//
// Bir sağlayıcının hatası diğerlerini ETKİLEMEZ (spec 002 H2).
func (s *Syncer) SyncAll(ctx context.Context) ([]Result, error) {
	providers, err := s.providers.List(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(providers))
	for _, p := range providers {
		count, err := s.SyncOne(ctx, p)
		r := Result{ProviderID: p.ID, ProviderName: p.Name, OK: err == nil, Count: count}
		if err != nil {
			r.Error = UserFacingError(err)
		}
		results = append(results, r)
	}
	return results, nil
}

// replaceProviderModels, bir sağlayıcının modellerini tek transaction içinde değiştirir.
//
// Ya hepsi ya hiçbiri: kısmi yazma sonucu tutarsız bir liste oluşamaz.
// Yalnızca bu sağlayıcının satırlarına dokunulur.
func (s *Syncer) replaceProviderModels(ctx context.Context, providerID uuid.UUID, models []llm.Model) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const upsert = `
		INSERT INTO models (
			provider_id, id, provider, name, description, context_length,
			max_output_tokens, prompt_price, completion_price, supports_tools,
			is_free, is_preview, modality, raw, synced_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14, now())
		ON CONFLICT (provider_id, id) DO UPDATE SET
			provider          = EXCLUDED.provider,
			name              = EXCLUDED.name,
			description       = EXCLUDED.description,
			context_length    = EXCLUDED.context_length,
			max_output_tokens = EXCLUDED.max_output_tokens,
			prompt_price      = EXCLUDED.prompt_price,
			completion_price  = EXCLUDED.completion_price,
			supports_tools    = EXCLUDED.supports_tools,
			is_free           = EXCLUDED.is_free,
			is_preview        = EXCLUDED.is_preview,
			modality          = EXCLUDED.modality,
			raw               = EXCLUDED.raw,
			synced_at         = now()`

	batch := &pgx.Batch{}
	ids := make([]string, 0, len(models))

	for _, m := range models {
		ids = append(ids, m.ID)
		raw := m.Raw
		if len(raw) == 0 {
			raw = []byte("{}")
		}

		batch.Queue(upsert,
			providerID, m.ID, llm.ProviderOf(m.ID), m.Name, "",
			m.ContextLength, m.MaxOutputTokens,
			m.PromptPrice, m.CompletionPrice, m.SupportsTools,
			m.IsFree(), m.IsPreview(), m.Modality, raw,
		)
	}

	results := tx.SendBatch(ctx, batch)
	for range models {
		if _, err := results.Exec(); err != nil {
			_ = results.Close()
			return 0, err
		}
	}
	if err := results.Close(); err != nil {
		return 0, err
	}

	// Sağlayıcıda artık olmayan modeller silinir — kullanıcı var olmayan bir
	// modeli seçebilir durumda kalmasın. Diğer sağlayıcılara dokunulmaz.
	if _, err := tx.Exec(ctx,
		`DELETE FROM models WHERE provider_id = $1 AND id <> ALL($2)`,
		providerID, ids); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(models), nil
}

func (s *Syncer) recordSuccess(ctx context.Context, providerID uuid.UUID, count int) {
	const q = `
		INSERT INTO provider_sync (provider_id, last_attempt_at, last_success_at, model_count, last_error)
		VALUES ($1, now(), now(), $2, NULL)
		ON CONFLICT (provider_id) DO UPDATE SET
			last_attempt_at = now(), last_success_at = now(),
			model_count = EXCLUDED.model_count, last_error = NULL`

	if _, err := s.pool.Exec(ctx, q, providerID, count); err != nil {
		slog.ErrorContext(ctx, "senkron durumu yazılamadı", "provider_id", providerID, "error", err)
	}
}

func (s *Syncer) recordFailure(ctx context.Context, providerID uuid.UUID, cause error) {
	const q = `
		INSERT INTO provider_sync (provider_id, last_attempt_at, last_error)
		VALUES ($1, now(), $2)
		ON CONFLICT (provider_id) DO UPDATE SET
			last_attempt_at = now(), last_error = EXCLUDED.last_error`

	// Ayrı ve iptal edilmemiş context: asıl işlem iptal edildiği için hata
	// kaydının da düşmesi istenmez.
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	if _, err := s.pool.Exec(writeCtx, q, providerID, UserFacingError(cause)); err != nil {
		slog.ErrorContext(ctx, "senkron hatası yazılamadı", "provider_id", providerID, "error", err)
	}
}

// UserFacingError, teknik hatayı kullanıcıya gösterilebilir kısa bir mesaja çevirir.
//
// Yalnızca bilinen sentinel hatalar metne çevrilir; bilinmeyen hatalar genel
// mesaja düşer ki iç detaylar (ve olası gizli değerler) arayüze sızmasın.
func UserFacingError(err error) string {
	switch {
	case errors.Is(err, llm.ErrUnauthorized):
		return "anahtar geçersiz"
	case errors.Is(err, llm.ErrUnreachable):
		return "adrese ulaşılamadı"
	case errors.Is(err, llm.ErrBadCatalog):
		return "model listesi okunamadı"
	case errors.Is(err, llm.ErrNotFound):
		return "sağlayıcı bulunamadı"
	default:
		return "katalog güncellenemedi"
	}
}

// Run, açılışta bir kez ve sonra SyncInterval aralıklarla tüm sağlayıcıları senkronlar.
//
// Başarısızlık açılışı ENGELLEMEZ: anahtar henüz girilmemiş olabilir veya
// internet yoktur; sistem ayakta kalır.
func (s *Syncer) Run(ctx context.Context) {
	s.syncAndLog(ctx, "açılış")

	ticker := time.NewTicker(SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "katalog senkronu durduruldu")
			return
		case <-ticker.C:
			s.syncAndLog(ctx, "zamanlanmış")
		}
	}
}

func (s *Syncer) syncAndLog(ctx context.Context, sebep string) {
	syncCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	results, err := s.SyncAll(syncCtx)
	if err != nil {
		slog.WarnContext(ctx, "katalog senkronu başlatılamadı", "sebep", sebep, "error", err)
		return
	}

	for _, r := range results {
		if r.OK {
			slog.InfoContext(ctx, "katalog güncellendi",
				"sebep", sebep, "sağlayıcı", r.ProviderName, "model_sayısı", r.Count)
		} else {
			slog.WarnContext(ctx, "katalog güncellenemedi",
				"sebep", sebep, "sağlayıcı", r.ProviderName, "durum", r.Error)
		}
	}
}
