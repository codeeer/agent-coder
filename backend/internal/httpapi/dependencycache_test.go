package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
)

/*
 * Bağımlılık önbelleği bakım uçları (spec 027 H3).
 *
 * Docker'sız: sorulan şey uç katmanının KARARLARI — hangi durumda hangi kod ve
 * hangi mesaj. Önbelleğin gerçekten silinip silinmediği `sandbox` paketinin
 * gerçek Docker testlerinde ölçülüyor.
 */

// sahteRunner, runner.CacheAdmin sağlayan asgari bir uygulama.
type sahteRunner struct {
	runner.Runner
	durum   []runner.CacheInfo
	freed   int64
	clearHt error
	silinen runner.CacheID

	dogrulama  runner.VerifyResult
	dogrulaHt  error
	dogrulanan runner.CacheID
}

func (s *sahteRunner) VerifyCache(_ context.Context, id runner.CacheID) (runner.VerifyResult, error) {
	s.dogrulanan = id
	return s.dogrulama, s.dogrulaHt
}

func (s *sahteRunner) CacheStatus(context.Context) ([]runner.CacheInfo, error) {
	return s.durum, nil
}

func (s *sahteRunner) ClearCache(_ context.Context, id runner.CacheID) (int64, error) {
	s.silinen = id
	return s.freed, s.clearHt
}

// onbellekIstegi, uca bir temizleme isteği gönderir.
func onbellekIstegi(t *testing.T, h *Handler, id string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/{id}/clear", h.clearDependencyCache)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/"+id+"/clear", nil))
	return w
}

func hataMesaji(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var gövde struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gövde))
	return gövde.Error.Message
}

func TestClearDependencyCache_BilinmeyenKimlik404(t *testing.T) {
	h := &Handler{deps: Deps{Runner: &sahteRunner{
		clearHt: fmt.Errorf("%w: %q", runner.ErrUnknownCache, "gradle"),
	}}}

	w := onbellekIstegi(t, h, "gradle")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestClearDependencyCache_BasariliTemizlemeBosalanBaytiDoner(t *testing.T) {
	sahte := &sahteRunner{freed: 1234567}
	h := &Handler{deps: Deps{Runner: sahte}}

	w := onbellekIstegi(t, h, "maven")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, runner.CacheMaven, sahte.silinen)

	var gövde clearCacheResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gövde))
	require.EqualValues(t, 1234567, gövde.FreedBytes,
		"boşalan bayt SAYIYLA dönmeli; onay şeridi ve arayüz bunu gösteriyor")
}

/*
İKİ FARKLI 409, İKİ FARKLI CEVAP.

Aynı kodu döndürmek kolay olurdu ama kullanıcıya söylenecek şey farklı:

  - Koşu sürüyorsa: beklemesi yeterli, yapacak bir şey yok.
  - Koşu yokken volume bağlıysa: beklemek işe yaramaz; sahipsiz bir çalışma
    ortamı kalmış ve bakması gereken bir yer var.

Tek mesaj kullanılsaydı ikinci durumdaki kullanıcı sonsuza kadar beklerdi.
*/
func TestClearDependencyCache_KosuYokkenBagliysaSahipsizContainerDenir(t *testing.T) {
	// `RunManager` yok → süren koşu sayısı sıfır, yani çalıştırma kapısından
	// geçiliyor. Buna rağmen volume bağlıysa sebep başkadır.
	h := &Handler{deps: Deps{Runner: &sahteRunner{
		clearHt: fmt.Errorf("%w: hangi çalışma ortamının tuttuğunu görmek için: "+
			"docker ps -a --filter volume=agent-coder-cache-maven", runner.ErrCacheBusy),
	}}}

	w := onbellekIstegi(t, h, "maven")
	require.Equal(t, http.StatusConflict, w.Code)

	mesaj := hataMesaji(t, w)
	require.Contains(t, mesaj, "çalışan bir koşu yok",
		"koşu yokken sebebin farklı olduğu söylenmeli — kullanıcı boşuna beklememeli")
	require.Contains(t, mesaj, "docker ps",
		"kullanıcıya NEREYE bakacağı söylenmeli — ürün container'ı kendisi öldürmüyor")
}

/*
ÇALIŞTIRMA YÖNETİCİSİ YOKKEN BAKIM KİLİTLENMEZ.

Kapı `activeRunCount` üzerinden okunuyor ve yönetici bağlı değilse sıfır
sayılıyor. Ters tasarlansaydı (yönetici yoksa "bilinmiyor, izin verme")
bakım hiç yapılamaz hâle gelirdi.

NOT: kapının ASIL dalı — sayaç sıfırdan büyükken 409 — burada ölçülmüyor;
`runs.Manager` somut bir tip ve sayacı testten doldurulamıyor. O dal Blok 2
ile birlikte gerçek ortamda doğrulanacak (tasks.md T34).
*/
func TestClearDependencyCache_YoneticiYokkenBakimKilitlenmez(t *testing.T) {
	sahte := &sahteRunner{}
	h := &Handler{deps: Deps{Runner: sahte}}

	require.Zero(t, h.activeRunCount())

	w := onbellekIstegi(t, h, "maven")
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, runner.CacheMaven, sahte.silinen)
}

func TestClearDependencyCache_MesgulHatasiOldurmeOnermez(t *testing.T) {
	h := &Handler{deps: Deps{Runner: &sahteRunner{
		clearHt: fmt.Errorf("%w: docker ps -a --filter volume=x", runner.ErrCacheBusy),
	}}}

	mesaj := strings.ToLower(hataMesaji(t, onbellekIstegi(t, h, "maven")))
	for _, yasak := range []string{"docker rm", "docker kill", "docker stop"} {
		require.NotContains(t, mesaj, yasak,
			"ürün kendisi öldürmeyi ÖNERMEZ: yanlış bir şeyi durdurmak, "+
				"temizlemenin çözdüğünden büyük bir sorun açar")
	}
}

/*
BAKIM SAĞLAMAYAN KURULUMDA UÇ AÇIK KALMAZ.

Bakım isteğe bağlı bir yetenek; sağlanmıyorsa 503 ile açıkça söylenir. Sessizce
boş bir liste dönmek, kullanıcıya "önbellek yok" der ve yanlış olur.
*/
func TestDependencyCacheStatus_BakimYoksa503(t *testing.T) {
	h := &Handler{deps: Deps{}}

	w := httptest.NewRecorder()
	h.dependencyCacheStatus(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestDependencyCacheStatus_KullanilmamisOnbellekBoyutuylaKaristirilmaz(t *testing.T) {
	h := &Handler{deps: Deps{Runner: &sahteRunner{durum: []runner.CacheInfo{
		{ID: runner.CacheMaven, Label: "Maven", SizeBytes: 0, Used: true},
		{ID: runner.CacheNPM, Label: "npm", SizeBytes: 0, Used: false},
	}}}}

	w := httptest.NewRecorder()
	h.dependencyCacheStatus(w, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var gövde dependencyCacheResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gövde))
	require.Len(t, gövde.Caches, 2)
	require.True(t, gövde.Caches[0].Used, "boşaltılmış önbellek KULLANILMIŞ sayılır")
	require.False(t, gövde.Caches[1].Used, "hiç çalıştırılmamış önbellek kullanılmamıştır")
}

// Sentinel'lerin sarmalanınca da tanınabildiğinin güvencesi.
func TestCacheSentinelleri_SarmalanincaTanininir(t *testing.T) {
	require.True(t, errors.Is(
		fmt.Errorf("dış: %w", runner.ErrCacheBusy), runner.ErrCacheBusy))
	require.True(t, errors.Is(
		fmt.Errorf("dış: %w", runner.ErrUnknownCache), runner.ErrUnknownCache))
}

/*
DOĞRULAMA SONUCU SAYILARIYLA DÖNER — SESSİZ BİTİŞ YOK.

Spec 027 H5: "hiçbir uyumsuzluk yoksa sonuç bunu açıkça söyler". Boş bir 200,
kullanıcıyı "çalıştı mı çalışmadı mı" sorusuyla bırakırdı.

Denetlenemeyenler AYRI sayılıyor: bozuk sayısına eklenselerdi kullanıcı
olmayan bir sorun görür, sıfıra eklenselerdi tarama eksik çalıştığı hâlde
eksiksiz görünürdü.
*/
func TestVerifyDependencyCache_SayilarlaDoner(t *testing.T) {
	sahte := &sahteRunner{dogrulama: runner.VerifyResult{
		Checked: 120, Mismatched: 2, Unverifiable: 7, Removed: 2,
	}}
	h := &Handler{deps: Deps{Runner: sahte}}

	rt := chi.NewRouter()
	rt.Post("/{id}/verify", h.verifyDependencyCache)
	w := httptest.NewRecorder()
	rt.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/maven/verify", nil))

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, runner.CacheMaven, sahte.dogrulanan)

	var gövde runner.VerifyResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &gövde))
	require.Equal(t, 120, gövde.Checked)
	require.Equal(t, 7, gövde.Unverifiable,
		"denetlenemeyenler AYRI sayılmalı — ne bozuk sayılır ne yok sayılır")
	require.Equal(t, 2, gövde.Removed)
}

func TestVerifyDependencyCache_BilinmeyenKimlik404(t *testing.T) {
	h := &Handler{deps: Deps{Runner: &sahteRunner{
		dogrulaHt: fmt.Errorf("%w: %q", runner.ErrUnknownCache, "gradle"),
	}}}

	rt := chi.NewRouter()
	rt.Post("/{id}/verify", h.verifyDependencyCache)
	w := httptest.NewRecorder()
	rt.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/gradle/verify", nil))

	require.Equal(t, http.StatusNotFound, w.Code)
}
