package db_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
)

// newCipher, test için rastgele anahtarlı bir cipher üretir.
func newCipher(t *testing.T) *secrets.Cipher {
	t.Helper()
	key := make([]byte, secrets.KeySize)
	_, err := rand.Read(key)
	require.NoError(t, err)

	c, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(key))
	require.NoError(t, err)
	return c
}

// TestMigration002_001VerisiniTasir, spec 002 H6'nın çekirdeği:
// kullanıcının 001 ile girdiği anahtarlar kaybolmadan yeni yapıya geçmeli.
func TestMigration002_001VerisiniTasir(t *testing.T) {
	database := testutil.TempDB(t)
	ctx := context.Background()
	cipher := newCipher(t)

	// 1) 001 dünyasına geri dön.
	require.NoError(t, database.MigrateUpTo(ctx, 1))

	// 2) Kullanıcının 001 ile girdiği verileri yaz.
	const openRouterKey = "sk-or-v1-kullanicinin-anahtari-fd36"
	const githubToken = "ghp_kullanicinin-tokeni-9999"
	const jiraToken = "ATATT-jira-tokeni-1234"

	for _, c := range []struct {
		kind   string
		secret string
		hint   string
		meta   string
	}{
		{"openrouter", openRouterKey, "fd36", `{}`},
		{"github", githubToken, "9999", `{}`},
		{"jira", jiraToken, "1234", `{"base_url":"https://x.atlassian.net","email":"a@b.c"}`},
	} {
		blob, err := cipher.EncryptString(c.secret)
		require.NoError(t, err)

		_, err = database.Pool.Exec(ctx, `
			INSERT INTO credentials (kind, secret_enc, hint, metadata)
			VALUES ($1, $2, $3, $4::jsonb)`, c.kind, blob, c.hint, c.meta)
		require.NoError(t, err)
	}

	// Katalogda da modeller olsun.
	_, err := database.Pool.Exec(ctx, `
		INSERT INTO models (id, provider, name, context_length, prompt_price,
			completion_price, supports_tools, is_free, is_preview, raw)
		VALUES ('anthropic/claude-haiku-4.5', 'anthropic', 'Haiku', 200000,
			0.000001, 0.000005, true, false, false, '{}'::jsonb)`)
	require.NoError(t, err)

	// 3) 002'yi uygula.
	require.NoError(t, database.MigrateUp(ctx))

	// 4) OpenRouter anahtarı sağlayıcıya taşındı ve HÂLÂ ÇÖZÜLEBİLİR olmalı.
	var (
		typ, name, slug, baseURL, hint string
		isDefault                      bool
		blob                           []byte
	)
	err = database.Pool.QueryRow(ctx, `
		SELECT type, name, slug, base_url, hint, is_default, secret_enc
		FROM llm_providers`).Scan(&typ, &name, &slug, &baseURL, &hint, &isDefault, &blob)
	require.NoError(t, err, "OpenRouter sağlayıcısı oluşturulmalıydı")

	require.Equal(t, "openrouter", typ)
	require.Equal(t, "openrouter", slug)
	require.Equal(t, "https://openrouter.ai/api/v1", baseURL)
	require.Equal(t, "fd36", hint)
	require.True(t, isDefault, "taşınan tek sağlayıcı varsayılan olmalı")

	cozulmus, err := cipher.DecryptString(blob)
	require.NoError(t, err, "taşınan anahtar çözülebilmeli")
	require.Equal(t, openRouterKey, cozulmus, "anahtar değişmemeli")

	// 5) GitHub token'ı git sağlayıcısına taşındı.
	var gitType, gitHint string
	var gitBlob []byte
	err = database.Pool.QueryRow(ctx,
		`SELECT type, hint, secret_enc FROM git_providers`).Scan(&gitType, &gitHint, &gitBlob)
	require.NoError(t, err, "GitHub erişimi oluşturulmalıydı")
	require.Equal(t, "github", gitType)

	cozulmusGit, err := cipher.DecryptString(gitBlob)
	require.NoError(t, err)
	require.Equal(t, githubToken, cozulmusGit)

	// 6) Jira credentials tablosunda KALMALI — git sağlayıcısı değil.
	var kalanKind string
	var kalanBlob []byte
	err = database.Pool.QueryRow(ctx,
		`SELECT kind, secret_enc FROM credentials`).Scan(&kalanKind, &kalanBlob)
	require.NoError(t, err)
	require.Equal(t, "jira", kalanKind)

	cozulmusJira, err := cipher.DecryptString(kalanBlob)
	require.NoError(t, err)
	require.Equal(t, jiraToken, cozulmusJira)

	// 7) Mevcut modeller taşınan sağlayıcıya bağlandı.
	var modelCount int
	require.NoError(t, database.Pool.QueryRow(ctx,
		`SELECT count(*) FROM models WHERE provider_id IS NOT NULL`).Scan(&modelCount))
	require.Equal(t, 1, modelCount)

	// 8) provider_sync satırı oluştu.
	var syncCount int
	require.NoError(t, database.Pool.QueryRow(ctx,
		`SELECT count(*) FROM provider_sync`).Scan(&syncCount))
	require.Equal(t, 1, syncCount)
}

// TestMigration002_SaglayiciYokkenKatalogBosaltilir, anahtar yalnızca .env'deyse
// (veritabanında credentials kaydı yoksa) modellerin bağlanacağı sağlayıcı yoktur.
func TestMigration002_SaglayiciYokkenKatalogBosaltilir(t *testing.T) {
	database := testutil.TempDB(t)
	ctx := context.Background()

	require.NoError(t, database.MigrateUpTo(ctx, 1))

	_, err := database.Pool.Exec(ctx, `
		INSERT INTO models (id, provider, name, context_length, prompt_price,
			completion_price, supports_tools, is_free, is_preview, raw)
		VALUES ('a/b', 'a', 'B', 1000, 0, 0, true, true, false, '{}'::jsonb)`)
	require.NoError(t, err)

	require.NoError(t, database.MigrateUp(ctx))

	var modelCount, providerCount int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM models`).Scan(&modelCount))
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM llm_providers`).Scan(&providerCount))

	require.Equal(t, 0, providerCount)
	require.Equal(t, 0, modelCount, "bağlanacak sağlayıcı yoksa katalog boşaltılır")
}

// TestMigration002_TekVarsayilanKuraliDayatilir, kısmi UNIQUE index'in
// gerçekten çalıştığını doğrular.
func TestMigration002_TekVarsayilanKuraliDayatilir(t *testing.T) {
	database := testutil.TempDB(t)
	ctx := context.Background()
	require.NoError(t, database.MigrateUp(ctx))

	insert := `INSERT INTO llm_providers (type, name, slug, base_url, secret_enc, hint, is_default)
	           VALUES ('openrouter', $1, $2, 'https://x/v1', '\x01'::bytea, 'aaaa', true)`

	_, err := database.Pool.Exec(ctx, insert, "Bir", "bir")
	require.NoError(t, err)

	_, err = database.Pool.Exec(ctx, insert, "İki", "iki")
	require.Error(t, err, "ikinci varsayılan veritabanı tarafından reddedilmeli")
}

// TestMigration002_GeriAlmaSemayiDondurur — geri alma veri kaybettirir,
// bu bilinçli; burada yalnızca şemanın tutarlı döndüğü doğrulanır.
//
// MigrateUp DEĞİL MigrateUpTo(2) kullanılıyor: bu test 002'nin geri almasını
// sınıyor, "son migration" hangisiyse onunkini değil. Yeni migration eklendiğinde
// sessizce anlamını yitirmesin.
func TestMigration002_GeriAlmaSemayiDondurur(t *testing.T) {
	database := testutil.TempDB(t)
	ctx := context.Background()

	require.NoError(t, database.MigrateUpTo(ctx, 2))
	require.NoError(t, database.MigrateDown(ctx))

	var n int
	require.NoError(t, database.Pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_name IN ('llm_providers','git_providers','provider_sync')`).Scan(&n))
	require.Equal(t, 0, n)

	require.NoError(t, database.Pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_name = 'catalog_sync'`).Scan(&n))
	require.Equal(t, 1, n, "001'in tablosu geri gelmeli")

	// Tekrar ileri gidebilmeli — bu sefer sonuna kadar.
	require.NoError(t, database.MigrateUp(ctx))

	require.NoError(t, database.Pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_name IN ('projects','agents','runs','run_events','settings')`).Scan(&n))
	require.Equal(t, 5, n, "003'ün tabloları da kurulmalı")
}

// MigrateUpTo(4) kullanılıyor, MigrateUp değil: bu test 004'ün geri almasını
// sınıyor. Yeni migration eklendiğinde sessizce anlamını yitirmesin.
func TestMigration004_GeriAlinabilir(t *testing.T) {
	database := testutil.TempDB(t)
	ctx := context.Background()

	require.NoError(t, database.MigrateUpTo(ctx, 4))

	var n int
	require.NoError(t, database.Pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_name IN ('workflows','workflow_versions','workflow_runs',
		                     'workflow_steps','workflow_hooks')`).Scan(&n))
	require.Equal(t, 5, n)

	require.NoError(t, database.MigrateDown(ctx))

	require.NoError(t, database.Pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_name LIKE 'workflow%'`).Scan(&n))
	require.Zero(t, n, "geri alma akış tablolarının hiçbirini bırakmamalı")

	// Tipler de temizlenmeli; kalırsa ileri gitmek "type already exists" ile patlar.
	require.NoError(t, database.Pool.QueryRow(ctx, `
		SELECT count(*) FROM pg_type WHERE typname LIKE 'workflow%'`).Scan(&n))
	require.Zero(t, n)

	// 003'ün tabloları yerinde durmalı.
	require.NoError(t, database.Pool.QueryRow(ctx, `
		SELECT count(*) FROM information_schema.tables
		WHERE table_name IN ('projects','agents','runs')`).Scan(&n))
	require.Equal(t, 3, n)

	require.NoError(t, database.MigrateUp(ctx))
}
