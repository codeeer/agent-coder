package opencode_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runner/opencode"
	"github.com/agent-coder/backend/internal/runner/sandbox"
)

// Bu dosya GERÇEK Docker ve GERÇEK model kullanır; para harcar.
// Yalnızca SMOKE_TEST_API_KEY tanımlıyken çalışır: `make smoke`.
//
// Amacı, planın 5. adımı: arayüz yazılmadan önce zincirin tamamının
// çalıştığını ve container'ın temizlendiğini kanıtlamak.

func smokeSetup(t *testing.T) (*opencode.Runner, string) {
	t.Helper()

	key := os.Getenv("SMOKE_TEST_API_KEY")
	if key == "" {
		t.Skip("SMOKE_TEST_API_KEY tanımlı değil — duman testi atlanıyor (make smoke)")
	}

	image := os.Getenv("RUNNER_IMAGE")
	if image == "" {
		image = "agent-coder/opencode-runner:latest"
	}
	network := os.Getenv("RUNNER_NETWORK")
	if network == "" {
		network = "agent-coder_internal"
	}

	mgr, err := sandbox.NewManager(opencode.Port)
	require.NoError(t, err)
	t.Cleanup(func() { _ = mgr.Close() })

	r := opencode.New(mgr, image, network)
	require.NoError(t, r.Ping(context.Background()), "docker ve runner imajı hazır olmalı")

	return r, key
}

func smokeRequest(key string) runner.Request {
	return runner.Request{
		RunID: uuid.New(),
		Repo: runner.RepoSpec{
			URL:        "https://github.com/octocat/Hello-World.git",
			CloneDepth: 1,
		},
		Agent: runner.AgentSpec{
			Slug:        "incelemeci",
			Description: "Depoyu inceler",
			Prompt: "Sen bir kod incelemecisisin. Yanıtlarını Türkçe ver ve çok kısa tut. " +
				"Kod DEĞİŞTİRMEZSİN.",
			AllowEdit:     false,
			AllowBash:     true,
			AllowWebfetch: false,
		},
		Provider: runner.ProviderSpec{
			Slug:   "openrouter",
			Kind:   "openrouter",
			APIKey: key,
		},
		Model:   "anthropic/claude-haiku-4.5",
		Task:    "Bu depodaki README dosyasini oku ve tek cumleyle ozetle.",
		Timeout: 5 * time.Minute,
		Limits:  runner.Limits{CPUCores: 2, MemoryGB: 2},
	}
}

// TestSmoke_UctanUcaCalistirma, zincirin tamamını doğrular:
// container oluştur → config kopyala → başlat → klonla → agent çalıştır →
// çıktı + maliyet → temizlik.
func TestSmoke_UctanUcaCalistirma(t *testing.T) {
	r, key := smokeSetup(t)

	var (
		mu     sync.Mutex
		events []runner.Event
	)
	emit := func(e runner.Event) {
		mu.Lock()
		events = append(events, e)
		mu.Unlock()
	}

	req := smokeRequest(key)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	result, err := r.Run(ctx, req, emit)
	require.NoError(t, err)
	require.NotNil(t, result)

	t.Logf("çıktı: %s", result.Output)
	t.Logf("token: girdi=%d çıktı=%d  maliyet: $%.6f",
		result.PromptTokens, result.CompletionTokens, result.CostUSD)

	require.NotEmpty(t, result.Output, "agent bir şey söylemiş olmalı")
	require.Greater(t, result.CostUSD, 0.0, "gerçek model çağrısı maliyet üretmeli")
	require.Greater(t, result.PromptTokens, 0)

	// Özel agent tanımı container'a kopyalanıp UYGULANDI mı?
	// Dosyada tanımlı olmayan "incelemeci" agent'ı kullanıldı; opencode onu
	// bulamasaydı çalıştırma hata verirdi.
	require.NotEmpty(t, result.Output)

	// edit yetkisi kapalıydı: değişiklik üretilmemeli.
	require.False(t, result.HasChanges(), "salt okunur agent değişiklik üretmemeli")

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, events, "ilerleme olayları bildirilmiş olmalı")

	var joined []string
	for _, e := range events {
		joined = append(joined, e.Message)
	}
	require.Contains(t, strings.Join(joined, " | "), "çalışma tamamlandı")
}

// TestSmoke_ContainerArtigiBirakmaz, en kritik dayanıklılık kuralı:
// çalışma bitince geçici hiçbir şey kalmaz.
func TestSmoke_ContainerArtigiBirakmaz(t *testing.T) {
	r, key := smokeSetup(t)

	req := smokeRequest(key)
	req.Task = "Merhaba de, baska bir sey yapma."

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	_, err := r.Run(ctx, req, nil)
	require.NoError(t, err)

	// Silme asenkron tamamlanabilir; kısa bir pay bırakılır.
	time.Sleep(2 * time.Second)

	n, err := r.CleanupOrphans(context.Background())
	require.NoError(t, err)
	require.Zero(t, n, "çalışma sonrası container artığı kalmamalı")
}

// TestSmoke_IptalContaineriTemizler, iptal yolunda da temizlik yapıldığını doğrular.
func TestSmoke_IptalContaineriTemizler(t *testing.T) {
	r, key := smokeSetup(t)

	req := smokeRequest(key)
	req.Task = "Bu depodaki tum dosyalari tek tek incele ve cok uzun bir rapor yaz."

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := r.Run(ctx, req, nil)
		done <- err
	}()

	// Container kurulumu/klonlama sırasında iptal et.
	//
	// Uzun beklemek işe yaramıyor: küçük bir depoda çalışma ~9 saniyede bitiyor
	// ve iptal edilecek bir şey kalmıyor. İptalin ERKEN yolda (sandbox kurulumu
	// ve klonlama) da çalıştığını doğrulamak zaten daha değerli — orada
	// container yarım kalır ve temizlenmesi gerekir.
	time.Sleep(3 * time.Second)
	cancel()

	select {
	case err := <-done:
		require.ErrorIs(t, err, runner.ErrCancelled)
	case <-time.After(60 * time.Second):
		t.Fatal("iptal 60 saniyede tamamlanmadı")
	}

	time.Sleep(2 * time.Second)
	n, err := r.CleanupOrphans(context.Background())
	require.NoError(t, err)
	require.Zero(t, n, "iptal sonrası container artığı kalmamalı")
}

// TestSmoke_GecersizDepoAnlamliHataDoner, kullanıcının anlayacağı hata üretildiğini doğrular.
func TestSmoke_GecersizDepoAnlamliHataDoner(t *testing.T) {
	r, key := smokeSetup(t)

	req := smokeRequest(key)
	req.Repo.URL = "https://github.com/bu-kullanici-yok-12345/bu-depo-yok-67890.git"
	req.Timeout = 2 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	_, err := r.Run(ctx, req, nil)
	require.Error(t, err)
	require.ErrorIs(t, err, runner.ErrRepoAccess,
		"kullanıcı 'depoya erişilemedi' görmeli, genel bir hata değil")

	time.Sleep(2 * time.Second)
	n, cleanErr := r.CleanupOrphans(context.Background())
	require.NoError(t, cleanErr)
	require.Zero(t, n, "hata yolunda da container artığı kalmamalı")
}
