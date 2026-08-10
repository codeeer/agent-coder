package db

import (
	"context"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// migrationsFS, migration dosyalarını ikiliye gömer — çalışma anında dosya
// sisteminde bulunmalarına gerek kalmaz, container'a ayrıca kopyalanmazlar.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

func init() {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		panic(fmt.Sprintf("goose dialect ayarlanamadı: %v", err))
	}
}

// MigrateUp bekleyen tüm migration'ları uygular.
//
// Açılışta çalışır. Sistem tek örnekli olduğu için eşzamanlı çalışma riski yok;
// yine de goose kendi kilit tablosuyla aynı migration'ın iki kez uygulanmasını
// engeller.
func (d *DB) MigrateUp(ctx context.Context) error {
	sqlDB := stdlib.OpenDBFromPool(d.Pool)
	defer sqlDB.Close()

	before, err := goose.GetDBVersionContext(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("migration sürümü okunamadı: %w", err)
	}

	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("migration uygulanamadı: %w", err)
	}

	after, err := goose.GetDBVersionContext(ctx, sqlDB)
	if err != nil {
		return fmt.Errorf("migration sürümü okunamadı: %w", err)
	}

	if before == after {
		slog.InfoContext(ctx, "şema güncel", "sürüm", after)
	} else {
		slog.InfoContext(ctx, "şema güncellendi", "önce", before, "sonra", after)
	}
	return nil
}

// MigrateUpTo, yalnızca belirtilen sürüme kadar migration uygular.
//
// Testler için: bir migration'ın veri taşımasını doğrulamak, önceki sürümde
// durup veri yazmayı ve sonra ilerlemeyi gerektirir.
func (d *DB) MigrateUpTo(ctx context.Context, version int64) error {
	sqlDB := stdlib.OpenDBFromPool(d.Pool)
	defer sqlDB.Close()

	if err := goose.UpToContext(ctx, sqlDB, "migrations", version); err != nil {
		return fmt.Errorf("migration %d'e kadar uygulanamadı: %w", version, err)
	}
	return nil
}

// MigrateDown son migration'ı geri alır. Yalnızca elle kullanım içindir.
func (d *DB) MigrateDown(ctx context.Context) error {
	sqlDB := stdlib.OpenDBFromPool(d.Pool)
	defer sqlDB.Close()

	if err := goose.DownContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("migration geri alınamadı: %w", err)
	}
	return nil
}

// MigrateStatus uygulanmış ve bekleyen migration'ları stdout'a yazar.
func (d *DB) MigrateStatus(ctx context.Context) error {
	sqlDB := stdlib.OpenDBFromPool(d.Pool)
	defer sqlDB.Close()

	// Durum çıktısı insan içindir; goose'un yazdıklarını stdout'a açıyoruz.
	// goose.Logger arayüzünü standart log.Logger karşılıyor.
	goose.SetLogger(log.New(os.Stdout, "", 0))
	defer goose.SetLogger(goose.NopLogger())

	if err := goose.StatusContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("migration durumu alınamadı: %w", err)
	}
	return nil
}
