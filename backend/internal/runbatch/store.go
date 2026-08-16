package runbatch

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

// Store, toplu çalıştırma deposu.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore yeni depo üretir.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

/*
Seçilen sütunlar ve sayılar TEK sorguda.

Sayılar `FILTER` ile hesaplanıyor; her durum için ayrı sorgu atmak liste
ekranını sayı × toplu iş kadar sorguya çıkarırdı. LEFT JOIN: öğesi silinmiş
(projesi kaldırılmış) bir toplu iş de listelenmeli — sayıları sıfır görünür.
*/
const batchColumns = `b.id, b.workflow_id, w.name, b.task, b.status,
	b.created_at, b.updated_at,
	count(i.id),
	count(i.id) FILTER (WHERE i.status = 'pending'),
	count(i.id) FILTER (WHERE i.status = 'running'),
	count(i.id) FILTER (WHERE i.status = 'succeeded'),
	count(i.id) FILTER (WHERE i.status = 'failed'),
	count(i.id) FILTER (WHERE i.status = 'interrupted'),
	count(i.id) FILTER (WHERE i.status = 'cancelled')`

const batchSource = ` FROM run_batches b
	JOIN workflows w ON w.id = b.workflow_id
	LEFT JOIN run_batch_items i ON i.batch_id = b.id`

const batchGroup = ` GROUP BY b.id, w.name`

func scanBatch(row pgx.Row) (Batch, error) {
	var b Batch
	err := row.Scan(&b.ID, &b.WorkflowID, &b.WorkflowName, &b.Task, &b.Status,
		&b.CreatedAt, &b.UpdatedAt,
		&b.Counts.Total, &b.Counts.Pending, &b.Counts.Running,
		&b.Counts.Succeeded, &b.Counts.Failed, &b.Counts.Interrupted,
		&b.Counts.Cancelled)
	return b, err
}

const itemColumns = `i.id, i.batch_id, i.project_id, p.name, i.position, i.status,
	i.workflow_run_id, i.error, i.started_at, i.finished_at`

func scanItem(row pgx.Row) (Item, error) {
	var it Item
	err := row.Scan(&it.ID, &it.BatchID, &it.ProjectID, &it.ProjectName,
		&it.Position, &it.Status, &it.WorkflowRunID, &it.Error,
		&it.StartedAt, &it.FinishedAt)
	return it, err
}

/*
Create, toplu işi ve öğelerini TEK transaction'da yazar.

`position` gönderilen sıranın kendisidir (0..n-1): sıra eklenme sırasıdır,
öncelik yok (spec → davranış kuralları). Kullanıcı sırayı seçim yaparken kurar.

Öğeler tek tek değil toplu yazılıyor — otuz proje otuz round-trip demek olurdu.
*/
func (s *Store) Create(ctx context.Context, workflowID uuid.UUID, task string,
	projectIDs []uuid.UUID) (Batch, error) {

	if len(projectIDs) == 0 {
		return Batch{}, ErrNoProjects
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Batch{}, fmt.Errorf("toplu iş kaydedilemedi: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var batchID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO run_batches (workflow_id, task) VALUES ($1, $2) RETURNING id`,
		workflowID, strings.TrimSpace(task)).Scan(&batchID)
	if err != nil {
		if isForeignKeyViolation(err) {
			return Batch{}, ErrWorkflowNotFound
		}
		return Batch{}, fmt.Errorf("toplu iş kaydedilemedi: %w", err)
	}

	// unnest: n öğe tek INSERT'te. Sıra numarası generate_subscripts ile
	// dizinin KENDİ sırasından üretilir — Go tarafında sayaç tutup ikinci bir
	// diziyle göndermek aynı bilgiyi iki kez taşımak olurdu.
	_, err = tx.Exec(ctx, `
		INSERT INTO run_batch_items (batch_id, project_id, position)
		SELECT $1, p, ord - 1
		FROM unnest($2::uuid[]) WITH ORDINALITY AS t(p, ord)`,
		batchID, projectIDs)
	if err != nil {
		if isUniqueViolation(err) {
			return Batch{}, ErrDuplicateProject
		}
		if isForeignKeyViolation(err) {
			return Batch{}, ErrProjectNotFound
		}
		return Batch{}, fmt.Errorf("toplu iş öğeleri kaydedilemedi: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Batch{}, fmt.Errorf("toplu iş kaydedilemedi: %w", err)
	}

	b, _, err := s.Get(ctx, batchID)
	return b, err
}

// List, toplu işleri sayfalı olarak döner. total, sayfalamadan bağımsız toplam.
func (s *Store) List(ctx context.Context, limit, offset int) (items []Batch, total int, err error) {
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM run_batches`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("toplu iş sayısı okunamadı: %w", err)
	}

	rows, err := s.pool.Query(ctx, `SELECT `+batchColumns+batchSource+batchGroup+
		` ORDER BY b.created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("toplu işler listelenemedi: %w", err)
	}
	defer rows.Close()

	out := []Batch{}
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("toplu iş taranamadı: %w", err)
		}
		out = append(out, b)
	}
	return out, total, rows.Err()
}

// Get, toplu işi öğeleriyle birlikte döner. Öğeler sıra (position) sırasında.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (Batch, []Item, error) {
	b, err := scanBatch(s.pool.QueryRow(ctx,
		`SELECT `+batchColumns+batchSource+` WHERE b.id = $1`+batchGroup, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Batch{}, nil, ErrNotFound
	}
	if err != nil {
		return Batch{}, nil, fmt.Errorf("toplu iş okunamadı: %w", err)
	}

	rows, err := s.pool.Query(ctx, `SELECT `+itemColumns+`
		FROM run_batch_items i JOIN projects p ON p.id = i.project_id
		WHERE i.batch_id = $1 ORDER BY i.position`, id)
	if err != nil {
		return Batch{}, nil, fmt.Errorf("toplu iş öğeleri okunamadı: %w", err)
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return Batch{}, nil, fmt.Errorf("öğe taranamadı: %w", err)
		}
		out = append(out, it)
	}
	return b, out, rows.Err()
}

/*
NextPending, sıradaki bekleyen öğeyi döner.

İPTAL EDİLMİŞ VE BİTMİŞ TOPLU İŞLERİN ÖĞELERİ GELMEZ: toplu iş durumu
süzgeçte. Aksi halde iptalden sonra kuyrukta kalan bir bekleyen, iptal edilmiş
bir işi başlatırdı.

Sıra: önce eski toplu iş, sonra öğe sırası. Kullanıcı ikinci kampanyayı
başlattı diye birincisinin kuyruğu beklemeye geçmez.
*/
func (s *Store) NextPending(ctx context.Context) (Pending, bool, error) {
	var p Pending
	row := s.pool.QueryRow(ctx, `SELECT `+itemColumns+`, b.workflow_id, b.task
		FROM run_batch_items i
		JOIN run_batches b ON b.id = i.batch_id
		JOIN projects p ON p.id = i.project_id
		WHERE i.status = 'pending' AND b.status IN ('queued', 'running')
		ORDER BY b.created_at, i.position
		LIMIT 1`)

	// Başlatmak için gereken her şey TEK sorguda: akış kimliği ve görev metni
	// ayrı bir çağrıyla okunsaydı kuyruk her öğede iki gidiş-dönüş yapardı.
	err := row.Scan(&p.ID, &p.BatchID, &p.ProjectID, &p.ProjectName,
		&p.Position, &p.Status, &p.WorkflowRunID, &p.Error,
		&p.StartedAt, &p.FinishedAt, &p.WorkflowID, &p.Task)
	if errors.Is(err, pgx.ErrNoRows) {
		return Pending{}, false, nil
	}
	if err != nil {
		return Pending{}, false, fmt.Errorf("sıradaki öğe okunamadı: %w", err)
	}
	return p, true, nil
}

/*
Claim, öğeyi SAHİPLENİR: `pending` → `running`.

Akış çalışması başlatılmadan ÖNCE yazılır. Ters sırada, iki çağrı arasında düşen
bir süreç öğeyi `pending` bırakırdı ve açılışta aynı iş ikinci kez başlatılırdı —
yan etkisi (branch'e gönderilmiş bir değişiklik) habersizce tekrarlanmış olurdu.

Yalnızca `pending` öğe sahiplenilir: aynı öğeyi iki kez başlatan bir yarış
ikinci çağrıda sessizce değil, `ErrNotFound` ile döner.
*/
func (s *Store) Claim(ctx context.Context, itemID uuid.UUID) error {
	return s.withBatch(ctx, itemID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE run_batch_items
			SET status = 'running', started_at = now(), error = '',
			    workflow_run_id = NULL, finished_at = NULL
			WHERE id = $1 AND status = 'pending'`, itemID)
		if err != nil {
			return fmt.Errorf("öğe sahiplenilemedi: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// Attach, sahiplenilmiş öğeyi akış çalışmasına bağlar.
//
// Ayrı çağrı olmasının sebebi `Claim`'in sırası: çalışma kimliği ancak başlatma
// döndükten sonra biliniyor.
func (s *Store) Attach(ctx context.Context, itemID, workflowRunID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE run_batch_items SET workflow_run_id = $2
		WHERE id = $1 AND status = 'running'`, itemID, workflowRunID)
	if err != nil {
		return fmt.Errorf("öğe çalışmaya bağlanamadı: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

/*
Requeue, öğeyi kuyruğa geri koyar: `running` → `pending`.

SINIR HATASININ karşılığı. Eşzamanlılık sınırı dolduğu için düşen bir öğe
BAŞARISIZ SAYILMAZ — o iş hiç çalışmadı, yalnızca zamanı değildi. Aksi hâlde
kuyruk kendini yerdi: sınırın dolu olduğu her an bir öğe "başarısız" diye
işaretlenir ve otuz projenin çoğu hiç çalışmadan başarısız görünürdü.

Çalışma kimliği SİLİNİR: o kayıt hiç iş yapmadan düştü, öğeye bağlı kalması
kullanıcıya bakacak bir şey varmış gibi gösterirdi.
*/
func (s *Store) Requeue(ctx context.Context, itemID uuid.UUID) error {
	return s.withBatch(ctx, itemID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE run_batch_items
			SET status = 'pending', workflow_run_id = NULL, error = '',
			    started_at = NULL, finished_at = NULL
			WHERE id = $1 AND status = 'running'`, itemID)
		if err != nil {
			return fmt.Errorf("öğe kuyruğa geri konamadı: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// RunningItems, o an çalışan TÜM öğeleri döner (toplu iş farkı gözetmeden).
//
// Zamanlayıcının iki sorusunun da cevabı burada: kaç öğe çalışıyor (sınır) ve
// hangileri bitti mi (sonuç toplama).
func (s *Store) RunningItems(ctx context.Context) ([]Item, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+itemColumns+`
		FROM run_batch_items i JOIN projects p ON p.id = i.project_id
		WHERE i.status = 'running'
		ORDER BY i.started_at`)
	if err != nil {
		return nil, fmt.Errorf("çalışan öğeler okunamadı: %w", err)
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("öğe taranamadı: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

/*
MarkFinished, öğeyi sonlandırır: succeeded · failed · interrupted · cancelled.

Öğe düşse de kuyruk durmaz (spec 023 H4); burada yalnızca öğenin sonucu
yazılır, sıradakini başlatmak zamanlayıcının işi.
*/
func (s *Store) MarkFinished(ctx context.Context, itemID uuid.UUID, status, errMsg string) error {
	return s.withBatch(ctx, itemID, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE run_batch_items
			SET status = $2::run_batch_item_status, error = $3, finished_at = now()
			WHERE id = $1`, itemID, status, errMsg)
		if err != nil {
			return fmt.Errorf("öğe sonucu yazılamadı: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

/*
Cancel, toplu işi iptal eder ve DÜŞEN ÖĞE SAYISINI döner.

Yalnızca BEKLEYENLER düşer; çalışan öğe dokunulmaz ve kendi hâlinde devam eder
(spec 023 H6). Süren bir container'ı yarıda kesmek, yan etkisi yarım kalmış bir
iş bırakırdı.

Bitmiş bir toplu işi iptal etmek HATA DEĞİLDİR: düşecek öğe yoksa 0 döner ve
durumu olduğu gibi kalır.
*/
func (s *Store) Cancel(ctx context.Context, id uuid.UUID) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("toplu iş iptal edilemedi: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	err = tx.QueryRow(ctx, `SELECT status FROM run_batches WHERE id = $1 FOR UPDATE`,
		id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("toplu iş okunamadı: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE run_batch_items
		SET status = 'cancelled', finished_at = now()
		WHERE batch_id = $1 AND status = 'pending'`, id)
	if err != nil {
		return 0, fmt.Errorf("bekleyen öğeler düşürülemedi: %w", err)
	}

	// Bitmiş bir toplu iş 'done' kalır; iptal onu geriye almaz. Devam eden bir
	// işte durum 'cancelled' olur — çalışan öğe bitince toplu işin 'done'a
	// dönmemesi bu sayede.
	if status == StatusQueued || status == StatusRunning {
		if _, err := tx.Exec(ctx,
			`UPDATE run_batches SET status = 'cancelled', updated_at = now() WHERE id = $1`,
			id); err != nil {
			return 0, fmt.Errorf("toplu iş iptal edilemedi: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("toplu iş iptal edilemedi: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

/*
Resume, KESİLMİŞ öğeleri yeniden sıraya alır ve sayısını döner.

Yalnızca `interrupted` olanlar: `succeeded` tamamlandı, `failed` çalıştı ve bir
sonuç üretti (spec 023 H5a). Başarısız olanı kendiliğinden tekrarlamak, derleme
hatası veren bir projeyi sonsuza kadar denemek olurdu.

`workflow_run_id` SİLİNMEZ — kesilen çalışmanın izi öğe yeniden başlatılana
kadar duruyor; kullanıcı neyin yarım kaldığını görebilmeli. Başlatma anında
`MarkRunning` üzerine yazar.
*/
func (s *Store) Resume(ctx context.Context, id uuid.UUID) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("toplu iş sürdürülemedi: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var exists bool
	err = tx.QueryRow(ctx, `SELECT true FROM run_batches WHERE id = $1 FOR UPDATE`,
		id).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("toplu iş okunamadı: %w", err)
	}

	tag, err := tx.Exec(ctx, `
		UPDATE run_batch_items
		SET status = 'pending', error = '', started_at = NULL, finished_at = NULL
		WHERE batch_id = $1 AND status = 'interrupted'`, id)
	if err != nil {
		return 0, fmt.Errorf("kesilen öğeler sıraya alınamadı: %w", err)
	}

	dirilen := int(tag.RowsAffected())

	/*
		Toplu işin durumu da geri gelmeli: 'done' ya da 'cancelled' kalsaydı
		NextPending bu öğeleri hiç görmez ve kuyruk sessizce donardı.

		Ama iptal koruması YALNIZCA gerçekten diriltilen öğe varsa kaldırılır.
		Koşulsuz kaldırıldığında, iptal edilmiş bir toplu işte "kaldığı yerden
		devam et" sıfır öğe diriltip durumu 'cancelled' → 'done' diye yeniden
		yazıyordu: hiçbir şey değişmeden iptal kaydı siliniyordu.
	*/
	if err := refreshStatus(ctx, tx, id, dirilen > 0); err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("toplu iş sürdürülemedi: %w", err)
	}
	return dirilen, nil
}

/*
InterruptRunning, açılışta `running` kalan öğeleri `interrupted` yapar ve kaçını
düşürdüğünü döner.

Backend kapandığında container'lar gitti; o çalışmalar tamamlanamaz. Kendiliğinden
DENENMEZ (spec 023 kararı): yarım kalmış bir işin yan etkisi — branch'e
gönderilmiş bir değişiklik — habersizce tekrarlanmamalı. Kullanıcı "Kaldığı
yerden devam et" ile sıraya alır.
*/
func (s *Store) InterruptRunning(ctx context.Context) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("kesilen öğeler uzlaştırılamadı: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		UPDATE run_batch_items
		SET status = 'interrupted', finished_at = now(),
		    error = 'Backend yeniden başladığında kesildi'
		WHERE status = 'running'
		RETURNING batch_id`)
	if err != nil {
		return 0, fmt.Errorf("kesilen öğeler işaretlenemedi: %w", err)
	}
	// Sayı RETURNING'den okunuyor, sonradan sayılmıyor: "durumu interrupted
	// olan öğeler" bir önceki yeniden başlatmadan kalanları da kapsardı ve
	// açılış logu her seferinde büyüyen bir rakam yazardı.
	count := 0
	batchIDs := map[uuid.UUID]struct{}{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, fmt.Errorf("kesilen öğe taranamadı: %w", err)
		}
		batchIDs[id] = struct{}{}
		count++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("kesilen öğeler işaretlenemedi: %w", err)
	}

	for id := range batchIDs {
		if err := refreshStatus(ctx, tx, id, false); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("kesilen öğeler uzlaştırılamadı: %w", err)
	}
	return count, nil
}

// withBatch, öğe üzerinde bir güncelleme yapar ve ardından toplu işin durumunu
// tazeler — ikisi tek transaction'da.
//
// Durum ayrı bir çağrıya bırakılsaydı arada düşen bir süreç, bekleyeni kalmamış
// bir toplu işi sonsuza kadar "çalışıyor" gösterirdi.
func (s *Store) withBatch(ctx context.Context, itemID uuid.UUID, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("öğe güncellenemedi: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var batchID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT batch_id FROM run_batch_items WHERE id = $1`,
		itemID).Scan(&batchID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("öğe okunamadı: %w", err)
	}

	if err := fn(tx); err != nil {
		return err
	}
	if err := refreshStatus(ctx, tx, batchID, false); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

/*
refreshStatus, toplu işin durumunu öğelerinden yeniden hesaplar.

Durum türetilmiş bir değer: ayrıca elle yönetilseydi öğelerle er geç çelişirdi.

`reviveCancelled` yalnızca Resume'da doğru: normal akışta iptal edilmiş bir iş
son çalışan öğesi bitince 'done'a dönmemeli — kullanıcı onu iptal etti.
*/
func refreshStatus(ctx context.Context, tx pgx.Tx, batchID uuid.UUID, reviveCancelled bool) error {
	guard := ` AND b.status <> 'cancelled'`
	if reviveCancelled {
		guard = ``
	}

	// CASE'in sonucu metin; sütun ENUM. Açık dönüşüm olmadan Postgres
	// reddediyor — tip güvenliğinin bedeli ve faydası aynı yerde.
	_, err := tx.Exec(ctx, `
		UPDATE run_batches b SET status = (CASE
			WHEN EXISTS (SELECT 1 FROM run_batch_items i
			             WHERE i.batch_id = b.id AND i.status = 'running') THEN 'running'
			WHEN EXISTS (SELECT 1 FROM run_batch_items i
			             WHERE i.batch_id = b.id AND i.status = 'pending') THEN 'queued'
			ELSE 'done' END)::run_batch_status,
			updated_at = now()
		WHERE b.id = $1`+guard, batchID)
	if err != nil {
		return fmt.Errorf("toplu iş durumu güncellenemedi: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

// RunDeleter, bir akış çalışmasını ve altındaki adım çalıştırmalarını silen şey.
//
// `Starter` ile aynı gerekçe: `workflow` paketine bağlanmadan dar bir arayüz.
// Kaskadın nasıl işlediği — `workflow_steps.run_id` SET NULL olduğu için adım
// çalıştırmalarının AÇIKÇA silinmesi gerektiği — tek bir yerde, `workflow`
// paketinde durur. Buraya kopyalansaydı iki kopya er geç ayrışırdı.
//
// SÖZLEŞME: var olmayan çalışmayı silmek hata DEĞİLDİR. Yarıda kalmış bir
// silme tekrar denenebilsin diye böyle.
type RunDeleter interface {
	DeleteRun(ctx context.Context, id uuid.UUID) error
}

/*
Delete, bitmiş bir toplu işi ve ürettiği tüm geçmişi siler.

SÜREN İŞ SİLİNMEZ: kuyruk hâlâ iş başlatıyorsa ya da bir öğe çalışıyorsa
`ErrRunning` döner. Kullanıcı önce iptal eder. (Bir toplu iş iptal edilmiş ama
o sırada çalışan öğesi henüz bitmemiş olabilir — durum 'cancelled' iken bile
canlı bir çalışma kalabildiği için öğelere ayrıca bakılır.)

SIRA ÖNEMLİ: önce akış çalışmaları, sonra toplu iş. `run_batch_items` toplu işle
birlikte kaskadla gidiyor ve çalışmalara giden tek işaretçi orada duruyor. Toplu
iş önce silinseydi, arada bir hata çıktığında geri kalan çalışmalara ulaşmanın
hiçbir yolu kalmazdı; bu sırayla yarıda kalan silme tekrar denenebilir.

ATOMİK DEĞİL: çalışmalar tek tek, kendi işlemlerinde siliniyor. Ortada bir hata
çıkarsa bir kısmı gitmiş olur ve toplu iş yerinde kalır — kullanıcı aynı silmeyi
tekrar çalıştırıp tamamlar.
*/
func (s *Store) Delete(ctx context.Context, id uuid.UUID, deleter RunDeleter) error {
	var status string
	var canli int
	err := s.pool.QueryRow(ctx, `
		SELECT b.status,
		       (SELECT count(*) FROM run_batch_items
		         WHERE batch_id = b.id AND status IN ('pending','running'))
		  FROM run_batches b WHERE b.id = $1`, id).Scan(&status, &canli)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("toplu iş okunamadı: %w", err)
	}
	if status == StatusQueued || status == StatusRunning || canli > 0 {
		return ErrRunning
	}

	rows, err := s.pool.Query(ctx, `
		SELECT workflow_run_id FROM run_batch_items
		 WHERE batch_id = $1 AND workflow_run_id IS NOT NULL`, id)
	if err != nil {
		return fmt.Errorf("toplu işin çalışmaları okunamadı: %w", err)
	}
	runIDs, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return fmt.Errorf("toplu işin çalışmaları okunamadı: %w", err)
	}

	for _, runID := range runIDs {
		if err := deleter.DeleteRun(ctx, runID); err != nil {
			return fmt.Errorf("akış çalışması silinemedi: %w", err)
		}
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM run_batches WHERE id = $1`, id); err != nil {
		return fmt.Errorf("toplu iş silinemedi: %w", err)
	}
	return nil
}
