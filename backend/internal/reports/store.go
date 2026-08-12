// Package reports, çalıştırma geçmişinden yönetici özeti üretir.
//
// Kural: rapor SADECE okur. Hiçbir uç nokta burada veri yazmaz, bu yüzden
// sorgular ağırlaşsa bile çalışan işleri etkilemez.
//
// Rakamların kaynağı `runs` tablosudur — agent hangi yoldan çalıştırılmış olursa
// olsun (arayüz, API, ileride workflow) kayıt oraya düştüğü için rapor tek
// gerçeğe bakar.
package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/settings"
)

// MaxDays, tek raporda bakılabilecek en uzun dönem.
const MaxDays = 365

// Totals, bir dönemin toplamları.
type Totals struct {
	Runs int `json:"runs"`

	Succeeded   int `json:"succeeded"`
	Failed      int `json:"failed"`
	Cancelled   int `json:"cancelled"`
	Timeout     int `json:"timeout"`
	Interrupted int `json:"interrupted"`
	// Active, henüz bitmemiş (bekleyen/çalışan) kayıtlar.
	Active int `json:"active"`

	CostUSD          float64 `json:"costUsd"`
	PromptTokens     int64   `json:"promptTokens"`
	CompletionTokens int64   `json:"completionTokens"`

	FilesChanged int64 `json:"filesChanged"`
	Additions    int64 `json:"additions"`
	Deletions    int64 `json:"deletions"`

	PushedBranches int `json:"pushedBranches"`

	// RunsWithCode, gerçekten kod değiştiren çalıştırmalar.
	//
	// "Çalıştı" ile "bir şey üretti" aynı şey değil: model çalışıp hiçbir
	// dosyaya dokunmadan da bitebilir.
	RunsWithCode int `json:"runsWithCode"`

	/*
	 * PRsOpened, açılan pull request sayısı.
	 *
	 * Bu sayı `runs` tablosunda YOKTUR: PR açan düğüm model çağırmadığı için
	 * çalıştırma kaydı üretmiyor, sonucu adım kaydına yazıyor (spec 003).
	 * Bu yüzden ayrı bir sorgudan geliyor.
	 *
	 * AÇILAN PR sayısıdır — birleştirilen değil. Bu sistem PR'ın sonrasını
	 * takip etmiyor ve arayüz bunu açıkça yazmak zorunda (spec 012 K4).
	 */
	PRsOpened int `json:"prsOpened"`

	// JiraTasks, insan dokunmadan işlenen Jira task'ı sayısı.
	JiraTasks int `json:"jiraTasks"`

	// AvgDurationSec, yalnızca başlayıp bitmiş kayıtların ortalaması.
	AvgDurationSec float64 `json:"avgDurationSec"`
}

// Day, tek bir takvim gününün kırılımı.
//
// Grafik yalnızca üç yığın çizer (başarılı / diğer / başarısız); geri kalan
// alanlar tablo ve ipucu içindir — sayı renkten değil metinden okunsun diye.
type Day struct {
	Date string `json:"date"` // YYYY-AA-GG, rapor saat diliminde

	Runs        int `json:"runs"`
	Succeeded   int `json:"succeeded"`
	Failed      int `json:"failed"`
	Cancelled   int `json:"cancelled"`
	Timeout     int `json:"timeout"`
	Interrupted int `json:"interrupted"`
	Active      int `json:"active"`

	CostUSD float64 `json:"costUsd"`

	// PRsOpened, o gün açılan pull request sayısı.
	//
	// Ayrı bir sorgudan gelir ve gün anahtarıyla birleştirilir: `runs` ile
	// `workflow_steps` arasında günlük kırılımda doğrudan bir bağ yok.
	PRsOpened int `json:"prsOpened"`
}

// Other, başarılı ve başarısız dışında kalanlar.
func (d Day) Other() int { return d.Runs - d.Succeeded - d.Failed }

// Group, bir kırılım satırı (agent, model veya proje).
type Group struct {
	Key   string `json:"key"`
	Label string `json:"label"`

	Runs      int `json:"runs"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`

	CostUSD float64 `json:"costUsd"`
	Tokens  int64   `json:"tokens"`

	FilesChanged int64 `json:"filesChanged"`
	Additions    int64 `json:"additions"`
	Deletions    int64 `json:"deletions"`

	AvgDurationSec float64 `json:"avgDurationSec"`
}

// Failure, yinelenen bir hata metni.
type Failure struct {
	Message string    `json:"message"`
	Count   int       `json:"count"`
	LastAt  time.Time `json:"lastAt"`
}

// Summary, rapor sayfasının tüm verisi.
//
// Tek istekte döner: sayfada altı ayrı bölüm var ve her biri için ayrı istek
// atmak hem tutarsız (aralarında yeni kayıt düşebilir) hem de yavaş olurdu.
type Summary struct {
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	Days     int       `json:"days"`
	Timezone string    `json:"timezone"`

	Totals Totals `json:"totals"`
	// Previous, aynı uzunlukta bir önceki dönem — yüzdesel değişim için.
	Previous Totals `json:"previous"`

	Daily     []Day     `json:"daily"`
	ByAgent   []Group   `json:"byAgent"`
	ByModel   []Group   `json:"byModel"`
	ByProject []Group   `json:"byProject"`
	Failures  []Failure `json:"failures"`
}

// Store, rapor sorgularını çalıştırır.
type Store struct {
	pool     *pgxpool.Pool
	settings *settings.Service
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool, s *settings.Service) *Store {
	return &Store{pool: pool, settings: s}
}

// Query, rapor parametreleri.
type Query struct {
	// Days 0 ise ayarlardaki varsayılan dönem kullanılır.
	Days      int
	ProjectID *uuid.UUID
}

// Location, rapor saat dilimini döner.
//
// Tanımsız bir dilim adı raporu bozmaz, UTC'ye düşer: yanlış saat dilimiyle
// rapor göstermek, hiç rapor gösterememekten iyidir.
func (s *Store) Location() *time.Location {
	loc, err := time.LoadLocation(s.settings.Text(settings.KeyReportTimezone))
	if err != nil {
		return time.UTC
	}
	return loc
}

// Summary, dönem özetini üretir.
func (s *Store) Summary(ctx context.Context, q Query) (Summary, error) {
	days := q.Days
	if days <= 0 {
		days = s.settings.Int(settings.KeyReportDefaultDays)
	}
	if days <= 0 {
		days = 1
	}
	if days > MaxDays {
		days = MaxDays
	}

	loc := s.Location()

	// Dönem TAKVİM GÜNÜ sınırlarına oturur: "son 30 gün" yöneticinin
	// takvimindeki 30 günü anlatmalı, 720 saatlik kayan pencereyi değil.
	now := time.Now().In(loc)
	to := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1)
	from := to.AddDate(0, 0, -days)
	prevFrom := from.AddDate(0, 0, -days)

	out := Summary{
		From:     from,
		To:       to,
		Days:     days,
		Timezone: loc.String(),
	}

	var err error
	if out.Totals, err = s.totals(ctx, from, to, q.ProjectID); err != nil {
		return Summary{}, err
	}
	if out.Previous, err = s.totals(ctx, prevFrom, from, q.ProjectID); err != nil {
		return Summary{}, err
	}
	if out.Daily, err = s.daily(ctx, from, to, loc, days, q.ProjectID); err != nil {
		return Summary{}, err
	}
	if out.ByAgent, err = s.group(ctx, groupByAgent, from, to, q.ProjectID); err != nil {
		return Summary{}, err
	}
	if out.ByModel, err = s.group(ctx, groupByModel, from, to, q.ProjectID); err != nil {
		return Summary{}, err
	}
	if out.ByProject, err = s.group(ctx, groupByProject, from, to, q.ProjectID); err != nil {
		return Summary{}, err
	}
	if out.Failures, err = s.failures(ctx, from, to, q.ProjectID); err != nil {
		return Summary{}, err
	}
	return out, nil
}

// filesLateral, her çalıştırmanın files jsonb'sinden dosya/satır sayılarını çıkarır.
//
// LATERAL kullanılıyor çünkü toplamlar iki kez (satır başına ve dönem geneli)
// gerekiyor; alt sorgu olarak yazılsaydı her toplam için tekrar taranırdı.
const filesLateral = `
	CROSS JOIN LATERAL (
		SELECT count(*)                                    AS files,
		       coalesce(sum((e->>'additions')::bigint), 0) AS adds,
		       coalesce(sum((e->>'deletions')::bigint), 0) AS dels
		FROM jsonb_array_elements(r.files) e
	) f`

// scope, dönem ve (varsa) proje koşulunu üretir.
//
// Proje filtresi opsiyonel olduğu için argüman sayısı değişir; koşul metnini
// elle kurmak yerine tek yerden üretiliyor ki her sorguda aynı olsun.
func scope(from, to time.Time, projectID *uuid.UUID) (string, []any) {
	args := []any{from, to}
	where := `WHERE r.created_at >= $1 AND r.created_at < $2`
	if projectID != nil {
		args = append(args, *projectID)
		where += fmt.Sprintf(` AND r.project_id = $%d`, len(args))
	}
	return where, args
}

func (s *Store) totals(ctx context.Context, from, to time.Time, projectID *uuid.UUID) (Totals, error) {
	where, args := scope(from, to, projectID)

	q := `
		SELECT
			count(*),
			count(*) FILTER (WHERE r.status = 'succeeded'),
			count(*) FILTER (WHERE r.status = 'failed'),
			count(*) FILTER (WHERE r.status = 'cancelled'),
			count(*) FILTER (WHERE r.status = 'timeout'),
			count(*) FILTER (WHERE r.status = 'interrupted'),
			count(*) FILTER (WHERE r.status IN ('pending', 'running')),
			coalesce(sum(r.cost_usd), 0),
			coalesce(sum(r.prompt_tokens), 0),
			coalesce(sum(r.completion_tokens), 0),
			coalesce(sum(f.files), 0),
			coalesce(sum(f.adds), 0),
			coalesce(sum(f.dels), 0),
			count(*) FILTER (WHERE r.pushed_branch IS NOT NULL),
			count(*) FILTER (WHERE r.diff <> ''),
			coalesce(avg(extract(epoch FROM (r.finished_at - r.started_at)))
				FILTER (WHERE r.started_at IS NOT NULL AND r.finished_at IS NOT NULL), 0)
		FROM runs r` + filesLateral + `
		` + where

	var t Totals
	err := s.pool.QueryRow(ctx, q, args...).Scan(
		&t.Runs, &t.Succeeded, &t.Failed, &t.Cancelled, &t.Timeout,
		&t.Interrupted, &t.Active,
		&t.CostUSD, &t.PromptTokens, &t.CompletionTokens,
		&t.FilesChanged, &t.Additions, &t.Deletions,
		&t.PushedBranches, &t.RunsWithCode, &t.AvgDurationSec)
	if err != nil {
		return Totals{}, fmt.Errorf("rapor toplamları okunamadı: %w", err)
	}

	// Akış tarafındaki iki ölçü ayrı sorgulardan gelir: `runs` tablosunda
	// karşılıkları yok.
	if t.PRsOpened, err = s.prsOpened(ctx, from, to, projectID); err != nil {
		return Totals{}, err
	}
	if t.JiraTasks, err = s.jiraTasks(ctx, from, to, projectID); err != nil {
		return Totals{}, err
	}
	return t, nil
}

/*
 * flowScope, akış tarafındaki sorgular için dönem ve proje koşulu.
 *
 * `scope()` `runs` üzerinden yazılmış (`r.project_id`); bu ise akış
 * çalışmaları üzerinden. İkisini tek fonksiyona sıkıştırmak, her çağrıda
 * hangi tablodan bahsedildiğini takip etmek demekti.
 *
 * Proje ÇALIŞMADAN okunur (`wr.project_id`), akıştan değil: aynı akış farklı
 * projelerde koşabiliyor (migration 000012). Akışın varsayılanına bakılsaydı
 * başka bir projede açılmış bir PR yanlış projenin raporuna sayılırdı.
 */
func flowScope(from, to time.Time, projectID *uuid.UUID) (string, []any) {
	args := []any{from, to}
	where := `WHERE wr.created_at >= $1 AND wr.created_at < $2`
	if projectID != nil {
		args = append(args, *projectID)
		where += fmt.Sprintf(` AND wr.project_id = $%d`, len(args))
	}
	return where, args
}

// prsOpened, dönemde açılan pull request sayısı.
//
// Başarılı PR adımları sayılır; başarısız bir adım PR açmamıştır.
func (s *Store) prsOpened(ctx context.Context, from, to time.Time, projectID *uuid.UUID) (int, error) {
	where, args := flowScope(from, to, projectID)

	q := `
		SELECT count(*)
		FROM workflow_steps s
		JOIN workflow_runs wr ON wr.id = s.workflow_run_id
		` + where + ` AND s.node_kind = 'github.pr' AND s.status = 'succeeded'`

	var n int
	if err := s.pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("açılan PR sayısı okunamadı: %w", err)
	}
	return n, nil
}

// jiraTasks, dönemde otomatik işlenen Jira task'ı sayısı.
//
// Yalnızca gerçekten bir çalışma BAŞLATMIŞ kayıtlar sayılır: işaret konup
// başlatma başarısız olduysa o task işlenmemiştir.
func (s *Store) jiraTasks(ctx context.Context, from, to time.Time, projectID *uuid.UUID) (int, error) {
	where, args := flowScope(from, to, projectID)

	q := `
		SELECT count(DISTINCT p.issue_key)
		FROM workflow_processed_issues p
		JOIN workflow_runs wr ON wr.id = p.workflow_run_id
		` + where

	var n int
	if err := s.pool.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		return 0, fmt.Errorf("işlenen Jira task sayısı okunamadı: %w", err)
	}
	return n, nil
}

func (s *Store) daily(ctx context.Context, from, to time.Time, loc *time.Location,
	days int, projectID *uuid.UUID,
) ([]Day, error) {
	where, args := scope(from, to, projectID)
	args = append(args, loc.String())
	tz := fmt.Sprintf("$%d", len(args))

	q := `
		SELECT to_char((r.created_at AT TIME ZONE ` + tz + `)::date, 'YYYY-MM-DD'),
			count(*),
			count(*) FILTER (WHERE r.status = 'succeeded'),
			count(*) FILTER (WHERE r.status = 'failed'),
			count(*) FILTER (WHERE r.status = 'cancelled'),
			count(*) FILTER (WHERE r.status = 'timeout'),
			count(*) FILTER (WHERE r.status = 'interrupted'),
			count(*) FILTER (WHERE r.status IN ('pending', 'running')),
			coalesce(sum(r.cost_usd), 0)
		FROM runs r
		` + where + `
		GROUP BY 1`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("günlük kırılım okunamadı: %w", err)
	}
	defer rows.Close()

	found := make(map[string]Day, days)
	for rows.Next() {
		var d Day
		if err := rows.Scan(&d.Date, &d.Runs, &d.Succeeded, &d.Failed,
			&d.Cancelled, &d.Timeout, &d.Interrupted, &d.Active, &d.CostUSD); err != nil {
			return nil, fmt.Errorf("gün satırı taranamadı: %w", err)
		}
		found[d.Date] = d
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("günlük kırılım okunamadı: %w", err)
	}

	prs, err := s.dailyPRs(ctx, from, to, loc.String(), projectID)
	if err != nil {
		return nil, err
	}

	// Boş günler DOLDURULUR: eksik gün grafikte atlanırsa zaman ekseni sıkışır
	// ve iki uzak gün yan yana görünür — trend yanlış okunur.
	out := make([]Day, 0, days)
	for i := 0; i < days; i++ {
		key := from.AddDate(0, 0, i).Format("2006-01-02")
		d, ok := found[key]
		if !ok {
			d = Day{Date: key}
		}
		d.PRsOpened = prs[key]
		out = append(out, d)
	}
	return out, nil
}

// dailyPRs, gün başına açılan PR sayısı.
//
// Ayrı sorgu: PR adımlarının `runs` kaydı yok, dolayısıyla günlük kırılıma
// mevcut sorgudan katılamıyorlar.
func (s *Store) dailyPRs(ctx context.Context, from, to time.Time, tz string,
	projectID *uuid.UUID,
) (map[string]int, error) {
	where, args := flowScope(from, to, projectID)
	args = append(args, tz)

	q := fmt.Sprintf(`
		SELECT to_char(wr.created_at AT TIME ZONE $%d, 'YYYY-MM-DD') AS day, count(*)
		FROM workflow_steps s
		JOIN workflow_runs wr ON wr.id = s.workflow_run_id
		%s AND s.node_kind = 'github.pr' AND s.status = 'succeeded'
		GROUP BY day`, len(args), where)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("günlük PR kırılımı okunamadı: %w", err)
	}
	defer rows.Close()

	out := map[string]int{}
	for rows.Next() {
		var day string
		var n int
		if err := rows.Scan(&day, &n); err != nil {
			return nil, fmt.Errorf("günlük PR satırı taranamadı: %w", err)
		}
		out[day] = n
	}
	return out, rows.Err()
}

// Kırılım anahtarları. SQL parçası kod sabitidir, kullanıcı girdisi buraya girmez.
const (
	groupByAgent   = `r.agent_slug, r.agent_slug`
	groupByModel   = `r.provider_slug || ' / ' || r.model_id, r.model_id`
	groupByProject = `r.project_id::text, p.name`
)

func (s *Store) group(ctx context.Context, key string, from, to time.Time,
	projectID *uuid.UUID,
) ([]Group, error) {
	where, args := scope(from, to, projectID)

	q := `
		SELECT ` + key + `,
			count(*),
			count(*) FILTER (WHERE r.status = 'succeeded'),
			count(*) FILTER (WHERE r.status IN ('failed', 'timeout')),
			coalesce(sum(r.cost_usd), 0),
			coalesce(sum(r.prompt_tokens + r.completion_tokens), 0),
			coalesce(sum(f.files), 0),
			coalesce(sum(f.adds), 0),
			coalesce(sum(f.dels), 0),
			coalesce(avg(extract(epoch FROM (r.finished_at - r.started_at)))
				FILTER (WHERE r.started_at IS NOT NULL AND r.finished_at IS NOT NULL), 0)
		FROM runs r
		JOIN projects p ON p.id = r.project_id` + filesLateral + `
		` + where + `
		GROUP BY 1, 2
		ORDER BY 3 DESC, 1`

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("kırılım okunamadı: %w", err)
	}
	defer rows.Close()

	out := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.Key, &g.Label, &g.Runs, &g.Succeeded, &g.Failed,
			&g.CostUSD, &g.Tokens, &g.FilesChanged, &g.Additions, &g.Deletions,
			&g.AvgDurationSec); err != nil {
			return nil, fmt.Errorf("kırılım satırı taranamadı: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("kırılım okunamadı: %w", err)
	}
	return out, nil
}

// failuresLimit, hata listesinin uzunluğu. Rapor hata ayıklama ekranı değil;
// tekrar eden ilk birkaç neden yeter, gerisi çalıştırma listesinde.
const failuresLimit = 8

func (s *Store) failures(ctx context.Context, from, to time.Time,
	projectID *uuid.UUID,
) ([]Failure, error) {
	where, args := scope(from, to, projectID)

	q := `
		SELECT coalesce(nullif(r.error, ''), 'Sebep kaydedilmemiş'),
			count(*), max(r.created_at)
		FROM runs r
		` + where + ` AND r.status IN ('failed', 'timeout', 'interrupted')
		GROUP BY 1
		ORDER BY 2 DESC, 3 DESC
		LIMIT ` + fmt.Sprint(failuresLimit)

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("hata kırılımı okunamadı: %w", err)
	}
	defer rows.Close()

	out := []Failure{}
	for rows.Next() {
		var f Failure
		if err := rows.Scan(&f.Message, &f.Count, &f.LastAt); err != nil {
			return nil, fmt.Errorf("hata satırı taranamadı: %w", err)
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("hata kırılımı okunamadı: %w", err)
	}
	return out, nil
}
