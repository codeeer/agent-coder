// Package testutil, yalnızca testlerden kullanılan yardımcıları barındırır.
//
// Gerçek Postgres'e karşı çalışan entegrasyon testleri için gereklidir:
// sqlc'den vazgeçildiği için SQL–struct uyumu derleme zamanında değil,
// bu testlerle doğrulanır.
package testutil

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agent-coder/backend/internal/db"
)

var (
	// Havuz ve şema kurulumu süreç başına bir kez yapılır. Aynı pakette
	// onlarca test varsa her biri için yeniden bağlanmak ve migration
	// çalıştırmak gereksiz — ve goose'u aynı anda iki kez çağırmak hataya yol açar.
	once     sync.Once
	testPool *pgxpool.Pool
	setupErr error
)

// TestDB, TEST_DATABASE_URL ile belirtilen veritabanına bağlanır ve şemayı kurar.
//
// Değişken tanımlı değilse test atlanır — `make test` geliştiricinin makinesinde
// veritabanı olmadan da çalışabilmeli. Entegrasyon testleri için:
// `make test-integration`
//
// NOT: Test paketleri aynı veritabanını paylaştığı için `make test-integration`
// bunları `-p 1` ile, yani sırayla çalıştırır. Aksi halde iki paket şemayı
// aynı anda kurmaya çalışır.
func TestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil — entegrasyon testi atlanıyor " +
			"(çalıştırmak için: make test-integration)")
	}

	once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		database, err := db.Connect(ctx, db.Options{
			URL:            url,
			MaxConns:       4,
			ConnectTimeout: 5 * time.Second,
			ConnectRetries: 3,
		})
		if err != nil {
			setupErr = err
			return
		}
		if err := database.MigrateUp(ctx); err != nil {
			database.Close()
			setupErr = err
			return
		}
		// Havuz süreç sonuna kadar açık kalır; kapatmak sonraki testleri bozar.
		testPool = database.Pool
	})

	if setupErr != nil {
		t.Fatalf("test veritabanı hazırlanamadı: %v", setupErr)
	}
	return testPool
}

// Truncate, verilen tabloları boşaltır. Testler birbirinin bıraktığı veriye
// güvenmesin diye her testin başında çağrılır.
func Truncate(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()

	ctx := context.Background()
	for _, table := range tables {
		// Tablo adları test kodundan gelen sabitlerdir, kullanıcı girdisi değil.
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE"); err != nil {
			t.Fatalf("%s tablosu boşaltılamadı: %v", table, err)
		}
	}
}
