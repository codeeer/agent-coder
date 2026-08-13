package runs

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/runner"
)

/*
 * Motor loglarının saklanması.
 *
 * Runner container'ı geçici; saklanmazsa opencode'un teşhis bilgisi koşu
 * bitince yok oluyor ve "neden başarısız oldu" sorusunun cevabı kalmıyor.
 *
 * SATIR SATIR DEĞİL, kaynak başına tek gzip'li blob. Bu içerik sorgulanmıyor,
 * bütün olarak okunuyor; satır tablosu koşu başına yüzlerce satır ve karşılığı
 * olmayan bir yazma maliyeti demekti.
 */

// EngineLog, saklanmış bir motor logu.
type EngineLog struct {
	Source string `json:"source"`
	// Content, açılmış (gunzip) hâli — API bunu döndürür.
	Content string `json:"content"`
	// RawSize, sıkıştırılmamış boyut. Arayüz "2,1 MB" diyebilsin diye ayrı
	// tutuluyor; `length(content)` sıkıştırılmış boyutu verirdi.
	RawSize int `json:"rawSize"`
	// Truncated true ise BAŞTAN kırpılmıştır — son kısım korunur.
	Truncated bool      `json:"truncated"`
	CreatedAt time.Time `json:"createdAt"`
}

/*
 * SaveEngineLogs, toplanan logları kaydeder.
 *
 * maxBytes aşılırsa SON kısım korunur: hata genelde sonda olur, baştaki
 * açılış satırları teşhis için en az değerli olanlardır.
 *
 * Hiçbir hata çağırana taşınmaz — log saklamak çalıştırmanın sonucunu
 * değiştiremez; başarısızlık loglanır ve geçilir.
 */
func (s *Store) SaveEngineLogs(ctx context.Context, runID uuid.UUID, logs []runner.EngineLog, maxBytes int) error {
	for _, l := range logs {
		if l.Content == "" {
			continue
		}

		icerik := l.Content
		kirpildi := false
		if maxBytes > 0 && len(icerik) > maxBytes {
			icerik = icerik[len(icerik)-maxBytes:]
			kirpildi = true
		}

		blob, err := gzipS(icerik)
		if err != nil {
			return fmt.Errorf("motor logu sıkıştırılamadı: %w", err)
		}

		// Aynı koşu iki kez log yazmaz; yine de ON CONFLICT var çünkü
		// tekrar denenen bir kayıt sessizce düşmemeli.
		_, err = s.pool.Exec(ctx, `
			INSERT INTO run_engine_logs (run_id, source, content, raw_size, truncated)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (run_id, source) DO UPDATE
			   SET content = EXCLUDED.content,
			       raw_size = EXCLUDED.raw_size,
			       truncated = EXCLUDED.truncated,
			       created_at = now()`,
			runID, string(l.Source), blob, len(l.Content), kirpildi)
		if err != nil {
			return fmt.Errorf("motor logu kaydedilemedi: %w", err)
		}
	}
	return nil
}

// EngineLogs, bir çalıştırmanın saklanmış loglarını açılmış olarak döner.
func (s *Store) EngineLogs(ctx context.Context, runID uuid.UUID) ([]EngineLog, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT source, content, raw_size, truncated, created_at
		  FROM run_engine_logs WHERE run_id = $1 ORDER BY source`, runID)
	if err != nil {
		return nil, fmt.Errorf("motor logları okunamadı: %w", err)
	}
	defer rows.Close()

	out := []EngineLog{}
	for rows.Next() {
		var (
			l    EngineLog
			blob []byte
		)
		if err := rows.Scan(&l.Source, &blob, &l.RawSize, &l.Truncated, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("motor logu taranamadı: %w", err)
		}
		if l.Content, err = gunzipS(blob); err != nil {
			return nil, fmt.Errorf("motor logu açılamadı: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

/*
 * PurgeEngineLogs, süresi dolmuş logları siler.
 *
 * ÇALIŞTIRMA KAYDI SİLİNMEZ, yalnızca ham logu: koşu geçmişi ve maliyet
 * raporları yerinde kalır. İki farklı yaşam süresi olmasının sebebi bu —
 * bir koşunun kaydı yıllarca değerli olabilir, iki megabaytlık ham logu
 * bir haftadan sonra değil.
 */
func (s *Store) PurgeEngineLogs(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM run_engine_logs WHERE created_at < now() - $1::interval`,
		fmt.Sprintf("%d seconds", int(olderThan.Seconds())))
	if err != nil {
		return 0, fmt.Errorf("eski motor logları silinemedi: %w", err)
	}
	return tag.RowsAffected(), nil
}

func gzipS(s string) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(s)); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gunzipS(b []byte) (string, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer r.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
