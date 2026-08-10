package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/agent-coder/backend/internal/db"
)

// tempDBCounter, aynı çalıştırmada benzersiz veritabanı adı üretir.
var tempDBCounter atomic.Int64

// TempDB, teste özel boş bir veritabanı oluşturur ve sonunda siler.
//
// Şema DEĞİŞTİREN testler için gereklidir: paylaşılan test veritabanında
// migration'ları geri alıp ilerletmek diğer testleri bozardı.
//
// Migration uygulanmaz — çağıran istediği sürüme kendi ilerler.
func TempDB(t *testing.T) *db.DB {
	t.Helper()

	baseURL := os.Getenv("TEST_DATABASE_URL")
	if baseURL == "" {
		t.Skip("TEST_DATABASE_URL tanımlı değil — entegrasyon testi atlanıyor " +
			"(çalıştırmak için: make test-integration)")
	}

	name := fmt.Sprintf("tmp_%s_%d",
		sanitizeDBName(t.Name()), tempDBCounter.Add(1))

	adminURL, err := replaceDBName(baseURL, "postgres")
	if err != nil {
		t.Fatalf("yönetici bağlantı adresi üretilemedi: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("yönetici bağlantısı kurulamadı: %v", err)
	}
	defer admin.Close(context.Background())

	// Veritabanı adı test adından türetilir ve yalnızca [a-z0-9_] içerir;
	// yine de yalnızca burada, test kodundan gelir — kullanıcı girdisi değildir.
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		t.Fatalf("geçici veritabanı oluşturulamadı: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		conn, err := pgx.Connect(cleanupCtx, adminURL)
		if err != nil {
			t.Logf("geçici veritabanı silinemedi (bağlantı): %v", err)
			return
		}
		defer conn.Close(context.Background())

		if _, err := conn.Exec(cleanupCtx, `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`); err != nil {
			t.Logf("geçici veritabanı silinemedi: %v", err)
		}
	})

	tempURL, err := replaceDBName(baseURL, name)
	if err != nil {
		t.Fatalf("geçici bağlantı adresi üretilemedi: %v", err)
	}

	database, err := db.Connect(ctx, db.Options{
		URL:            tempURL,
		MaxConns:       2,
		ConnectTimeout: 5 * time.Second,
		ConnectRetries: 3,
	})
	if err != nil {
		t.Fatalf("geçici veritabanına bağlanılamadı: %v", err)
	}
	t.Cleanup(database.Close)

	return database
}

// replaceDBName, bağlantı adresindeki veritabanı adını değiştirir.
func replaceDBName(rawURL, name string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.Path = "/" + name
	return u.String(), nil
}

// sanitizeDBName, test adını geçerli bir veritabanı adı parçasına çevirir.
func sanitizeDBName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}
