package opencode

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/agent-coder/backend/internal/runner/sandbox"
)

/*
 * Motor loglarının toplanması — GERÇEK container üzerinde.
 *
 * Neden birim testi yetmiyor: `Redact` saf bir fonksiyon ve kendi testi var,
 * ama asıl soru "maskeleme çalışıyor mu" değil, "toplama yolu maskelemeden
 * GEÇİYOR mu". Bu iki farklı iddia; ikincisi ancak logun container'dan
 * gerçekten okunduğu yerde sınanabilir. Bir refactor `Redact` çağrısını
 * düşürürse birim testi yeşil kalır, bu test kırmızıya döner.
 *
 * Model çağrılmıyor, para harcanmıyor: container ayağa kalkıyor, içine
 * sırların bilerek yazıldığı bir log dosyası konuyor ve toplama sonucuna
 * bakılıyor.
 */
func TestCollectEngineLogs_SirlarMaskelenir(t *testing.T) {
	image := os.Getenv("RUNNER_IMAGE")
	if image == "" {
		image = "agent-coder/opencode-runner:latest"
	}

	mgr, err := sandbox.NewManager(Port)
	if err != nil {
		t.Skipf("docker yok — atlanıyor: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })

	ctx, iptal := context.WithTimeout(context.Background(), 90*time.Second)
	defer iptal()

	if err := mgr.Ping(ctx); err != nil {
		t.Skipf("docker erişilemiyor — atlanıyor: %v", err)
	}
	if err := mgr.EnsureImage(ctx, image); err != nil {
		t.Skipf("runner imajı yok (%s) — atlanıyor: %v", image, err)
	}

	network := os.Getenv("RUNNER_NETWORK")
	if network == "" {
		network = "agent-coder_internal"
	}

	const (
		saglayiciAnahtari = "sk-test-SAGLAYICI-ANAHTARI-1234567890"
		gitTokeni         = "ghp-test-GIT-TOKENI-1234567890"
		paketTokeni       = "npm-test-PAKET-TOKENI-1234567890"
		paketKullanicisi  = "ci-kullanici"
	)
	// Paket kimliği `.npmrc`'de base64 duruyor; ham hâli aranırsa kaçar.
	npmrcKimligi := base64.StdEncoding.EncodeToString(
		[]byte(paketKullanicisi + ":" + paketTokeni))

	sizintili := strings.Join([]string{
		"level=INFO message=baslangic",
		"level=ERROR message=istek reddedildi authorization=Bearer " + saglayiciAnahtari,
		"level=ERROR message=klon basarisiz url=https://x-access-token:" + gitTokeni + "@github.com/a/b.git",
		"level=WARN message=npmrc _auth=" + npmrcKimligi,
		"level=INFO message=bitis",
	}, "\n")

	ct, err := mgr.Create(ctx, sandbox.Spec{
		RunID:    uuid.NewString(),
		Image:    image,
		Network:  network,
		CPUCores: 1,
		MemoryGB: 1,
		Files: []sandbox.File{{
			Path:    engineLogDir + "/sizinti.log",
			Content: []byte(sizintili),
			Mode:    0o600,
		}},
	})
	require.NoError(t, err, "container ayağa kalkmalı")
	t.Cleanup(func() { ct.Remove(context.Background()) })

	loglar := collectEngineLogs(ctx, ct, nil, "", runner.Request{
		Provider: runner.ProviderSpec{APIKey: saglayiciAnahtari},
		Repo:     runner.RepoSpec{Secret: gitTokeni},
		Packages: runner.PackageRegistry{
			NPMRegistry: "https://nexus.ornek.local/repository/npm/",
			Username:    paketKullanicisi,
			Token:       paketTokeni,
		},
	})

	var dosya, cikti string
	for _, l := range loglar {
		switch l.Source {
		case runner.EngineLogFile:
			dosya = l.Content
		case runner.EngineLogStdout:
			cikti = l.Content
		}
	}
	require.NotEmpty(t, dosya, "motorun log dosyası toplanmalı")

	// Container çıktısı ÇÖZÜLMÜŞ gelmeli. Docker akışı 8 baytlık başlıklarla
	// çerçeveleniyor; ham bırakılırsa satır başlarında kontrol baytları
	// görünür ve arayüzde okunmaz bir metin çıkar.
	for _, b := range []string{"\x00\x00\x00", "\x01\x00", "\x02\x00"} {
		require.NotContains(t, cikti, b, "container çıktısında docker çerçeve baytı kaldı")
	}

	// Asıl iddia: sırların HİÇBİRİ saklanacak içerikte durmuyor.
	for ad, sir := range map[string]string{
		"sağlayıcı anahtarı": saglayiciAnahtari,
		"git token'ı":        gitTokeni,
		"paket token'ı":      paketTokeni,
		"npmrc kimliği":      npmrcKimligi,
	} {
		require.NotContains(t, dosya, sir, "%s saklanan logda görünüyor", ad)
	}

	// Ve maskeleme logu yok etmiyor: teşhis için gereken satırlar duruyor.
	require.Contains(t, dosya, "***", "maskeleme uygulanmalı")
	require.Contains(t, dosya, "istek reddedildi", "hata satırı korunmalı")
	require.Contains(t, dosya, "message=bitis", "son satır korunmalı")
}

/*
 * Oturum geçmişinin çekilmesi.
 *
 * Bu kaynak ilerleme akışının YEDEĞİ: ilerleme kayıtları SSE üzerinden
 * besleniyor ve o bağlantı kopabiliyor. Koptuğunda agent'ın ne konuştuğu ve
 * hangi araçları çağırdığı yalnızca burada kalıyor — bu yüzden biçiminin
 * okunabilir olması ve bozuk yanıtta bile veri kaybedilmemesi gerekiyor.
 */
func TestSessionTranscript(t *testing.T) {
	t.Run("JSON girintilenir", func(t *testing.T) {
		var istenenYol string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			istenenYol = r.URL.Path
			_, _ = w.Write([]byte(`[{"info":{"role":"user"},"parts":[{"type":"text"}]}]`))
		}))
		defer srv.Close()

		c := &client{base: srv.URL, http: srv.Client()}
		got, err := c.sessionTranscript(context.Background(), "ses_1")
		require.NoError(t, err)

		require.Equal(t, "/session/ses_1/message", istenenYol)
		// Tek satırlık JSON, satır numaralı görüntüleyicide okunmaz.
		require.Greater(t, len(strings.Split(got, "\n")), 3, "çıktı girintilenmeli")
		require.Contains(t, got, `"role": "user"`)
	})

	t.Run("bozuk JSON yine de saklanır", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{yarim`))
		}))
		defer srv.Close()

		c := &client{base: srv.URL, http: srv.Client()}
		got, err := c.sessionTranscript(context.Background(), "ses_1")
		// Ayrıştırılamayan yanıtın kendisi de teşhis verisidir; atılmaz.
		require.NoError(t, err)
		require.Equal(t, "{yarim", got)
	})

	t.Run("hata durumu boş döner", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		c := &client{base: srv.URL, http: srv.Client()}
		got, err := c.sessionTranscript(context.Background(), "ses_1")
		require.Error(t, err)
		require.Empty(t, got)
	})
}

/*
 * Dev araç çıktılarının kırpılması.
 *
 * ÖLÇÜLMÜŞ bir olaydan geliyor: `npm install && build` koşusunda oturum
 * geçmişi 4,29 MB çıktı ve %96'sı iki alandı. Saklama katmanı bunu bayt
 * bayt kırpınca geriye GEÇERSİZ JSON kaldı; arayüz konuşma görünümüne
 * düşemedi ve ham metin gösterdi — tam da en çok gerektiği koşuda.
 *
 * Buradaki iddia: kırpma yapıyı bozmaz. Her mesaj, her araç çağrısı yerinde
 * kalır; yalnızca uzun metinlerin ortası çıkar.
 */
func TestSessionTranscript_DevCiktiKirpilir(t *testing.T) {
	dev := strings.Repeat("x", 2<<20) // 2 MB'lık araç çıktısı
	govde := `[{"info":{"role":"assistant"},"parts":[
		{"type":"tool","tool":"bash","state":{"output":"` + dev + `"}},
		{"type":"text","text":"kısa cevap"}]}]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(govde))
	}))
	defer srv.Close()

	c := &client{base: srv.URL, http: srv.Client()}
	got, err := c.sessionTranscript(context.Background(), "ses_1")
	require.NoError(t, err)

	// 1. Sonuç GEÇERLİ JSON — konuşma görünümünün tek şartı.
	var kok []map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &kok),
		"kırpılmış geçmiş ayrıştırılabilir olmalı")

	// 2. Yapı duruyor: mesaj ve iki parça yerinde.
	require.Len(t, kok, 1)
	parts, _ := kok[0]["parts"].([]any)
	require.Len(t, parts, 2, "araç çağrısı da metin de korunmalı")
	require.Contains(t, got, "kısa cevap", "kısa metinler dokunulmadan geçmeli")
	require.Contains(t, got, `"tool": "bash"`, "araç adı korunmalı")

	// 3. Dev alan gerçekten küçüldü ve kırpıldığı SÖYLENİYOR.
	require.Less(t, len(got), 64<<10, "geçmiş saklama sınırının çok altında olmalı")
	require.Contains(t, got, "bayt kırpıldı", "kırpma sessizce yapılmaz")
}
