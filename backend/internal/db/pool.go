// Package db, PostgreSQL bağlantısını ve şema migration'larını yönetir.
package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Options, bağlantı havuzunun ayarları.
type Options struct {
	URL            string
	MaxConns       int32
	MaxConnIdle    time.Duration
	ConnectTimeout time.Duration
	// ConnectRetries, ilk bağlantı için kaç kez deneneceği. Compose'da postgres
	// healthy olsa bile ilk saniyelerde bağlantı reddedilebiliyor.
	ConnectRetries int
}

// DB, uygulamanın veritabanı erişimi.
type DB struct {
	Pool *pgxpool.Pool
}

// ErrUnavailable, veritabanına ulaşılamadığında döner.
var ErrUnavailable = errors.New("veritabanına ulaşılamıyor")

// Connect havuzu kurar ve bağlantıyı doğrular.
//
// İlk ping başarısız olursa üstel geri çekilmeyle yeniden denenir; bu, postgres
// container'ı henüz istek kabul etmeye hazır değilken başlayan backend'in
// gereksiz yere çökmesini önler.
func Connect(ctx context.Context, opts Options) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL ayrıştırılamadı: %w", err)
	}

	if opts.MaxConns > 0 {
		cfg.MaxConns = opts.MaxConns
	}
	if opts.MaxConnIdle > 0 {
		cfg.MaxConnIdleTime = opts.MaxConnIdle
	}
	if opts.ConnectTimeout > 0 {
		cfg.ConnConfig.ConnectTimeout = opts.ConnectTimeout
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("bağlantı havuzu kurulamadı: %w", err)
	}

	retries := max(opts.ConnectRetries, 1)

	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		lastErr = pool.Ping(pingCtx)
		cancel()

		if lastErr == nil {
			slog.InfoContext(ctx, "veritabanına bağlanıldı", "deneme", attempt)
			return &DB{Pool: pool}, nil
		}

		if attempt == retries {
			break
		}

		wait := time.Duration(attempt) * 500 * time.Millisecond
		slog.WarnContext(ctx, "veritabanına bağlanılamadı, yeniden denenecek",
			"deneme", attempt, "bekleme", wait, "error", lastErr)

		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}

	pool.Close()
	return nil, fmt.Errorf("%w: %d denemeden sonra: %w", ErrUnavailable, retries, lastErr)
}

// Ping, veritabanının yanıt verdiğini doğrular. /readyz bunu kullanır.
func (d *DB) Ping(ctx context.Context) error {
	if err := d.Pool.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return nil
}

// Close havuzu kapatır.
func (d *DB) Close() {
	if d.Pool != nil {
		d.Pool.Close()
	}
}
