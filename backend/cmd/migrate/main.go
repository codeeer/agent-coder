// Command migrate, şema migration'larını elle çalıştırır.
//
// Normal akışta migration'lar sunucu açılışında kendiliğinden uygulanır;
// bu ikili geri alma ve durum kontrolü için vardır.
//
// Kullanım: migrate up | down | status
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/agent-coder/backend/internal/config"
	"github.com/agent-coder/backend/internal/db"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migration başarısız", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("kullanım: migrate up|down|status")
	}
	komut := os.Args[1]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	database, err := db.Connect(ctx, db.Options{
		URL:            cfg.DB.URL,
		MaxConns:       2,
		ConnectTimeout: 10 * time.Second,
		ConnectRetries: 5,
	})
	if err != nil {
		return err
	}
	defer database.Close()

	switch komut {
	case "up":
		return database.MigrateUp(ctx)
	case "down":
		return database.MigrateDown(ctx)
	case "status":
		return database.MigrateStatus(ctx)
	default:
		return fmt.Errorf("bilinmeyen komut %q — up|down|status bekleniyordu", komut)
	}
}
