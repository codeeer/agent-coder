package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/config"
	"github.com/agent-coder/backend/internal/events"
	"github.com/agent-coder/backend/internal/runs"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * Canlı olay akışı (SSE).
 *
 * İki ayrı arıza sınıfı var ve ikisi de sessiz: bağlantının kapanması
 * gerekirken açık kalması (tarayıcı boşuna bağlantı tutar, sunucuda goroutine
 * birikir) ve aynı olayın iki kez görünmesi (kullanıcı işin iki kez olduğunu
 * sanır).
 */

type sseFixture struct {
	routes http.Handler
	store  *runs.Store
	bus    *events.Bus
	run    runs.Run
}

func sseSetup(t *testing.T) sseFixture {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "runs", "workflows", "projects", "agents")

	ctx := context.Background()
	var projectID, agentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, repo_url) VALUES ('Deneme','https://example.com/r.git')
		 RETURNING id`).Scan(&projectID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO agents (slug, name, prompt, source) VALUES ('coder','Coder','p','builtin')
		 RETURNING id`).Scan(&agentID))

	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	cfg, err := config.Load()
	require.NoError(t, err)

	store := runs.NewStore(pool)
	run, err := store.Create(ctx, runs.CreateInput{
		ProjectID: projectID, AgentID: agentID,
		AgentSlug: "coder", AgentPrompt: "p", ProviderSlug: "openrouter",
		ModelID: "m1", Branch: "main", Task: "iş",
	})
	require.NoError(t, err)

	bus := events.New()
	h := NewHandler(Deps{Config: cfg, Runs: store, Bus: bus})
	t.Cleanup(h.Shutdown)

	return sseFixture{routes: h.Routes(), store: store, bus: bus, run: run}
}

// akis, olay akışını açar ve gövdeyi döner. Handler dönene kadar bekler.
func (f sseFixture) akis(t *testing.T, ctx context.Context) string {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/runs/"+f.run.ID.String()+"/events", nil)
	f.routes.ServeHTTP(rec, req.WithContext(ctx))
	return rec.Body.String()
}

// olaylar, SSE gövdesindeki `data:` satırlarını çözer.
func olaylar(t *testing.T, govde string) []events.Event {
	t.Helper()

	var out []events.Event
	for _, satir := range strings.Split(govde, "\n") {
		veri, ok := strings.CutPrefix(satir, "data: ")
		if !ok {
			continue
		}
		var e events.Event
		require.NoError(t, json.Unmarshal([]byte(veri), &e))
		out = append(out, e)
	}
	return out
}

/*
BİTMİŞ ÇALIŞTIRMADA BAĞLANTI KAPANIR.

Canlı akışa geçilseydi tarayıcı hiç olay gelmeyecek bir bağlantıyı süresiz
açık tutardı ve sunucuda her açılan sekme için bir goroutine birikirdi.

Testin kendisi bunu ölçüyor: handler BLOKLAMADAN dönüyor. Dönmeseydi bu test
zaman aşımına düşerdi.
*/
func TestRunEvents_BitmisCalistirmadaBaglantiKapanir(t *testing.T) {
	f := sseSetup(t)
	ctx := context.Background()

	_, _, err := f.store.AppendEvent(ctx, f.run.ID, "info", "başladı")
	require.NoError(t, err)
	require.NoError(t, f.store.Finish(ctx, f.run.ID, runs.StatusSucceeded, nil, nil))

	govde := f.akis(t, ctx)

	es := olaylar(t, govde)
	require.NotEmpty(t, es)
	son := es[len(es)-1]
	require.True(t, son.Terminal, "bitiş sinyali gönderilmeli")
	require.Equal(t, string(runs.StatusSucceeded), son.Status)
}

/*
BİTİŞ SİNYALİNİN MESAJI BOŞ.

Bitiş satırı geçmişte zaten var; buraya bir metin konsaydı aynı cümle ekranda
iki kez görünürdü.
*/
func TestRunEvents_BitisSinyaliMesajTasimaz(t *testing.T) {
	f := sseSetup(t)
	ctx := context.Background()

	_, _, err := f.store.AppendEvent(ctx, f.run.ID, "info", "çalıştırma tamamlandı")
	require.NoError(t, err)
	require.NoError(t, f.store.Finish(ctx, f.run.ID, runs.StatusSucceeded, nil, nil))

	es := olaylar(t, f.akis(t, ctx))
	son := es[len(es)-1]
	require.True(t, son.Terminal)
	require.Empty(t, son.Message, "bitiş satırı geçmişte zaten var")
}

/*
GEÇMİŞ ÖNCE GÖNDERİLİR.

Sayfa yenilendiğinde o ana kadarki ilerleme kaybolmamalı (spec 003 H4). Yalnızca
canlı akış verilseydi, işin ortasında sayfayı yenileyen kullanıcı boş bir ekran
görür ve çalıştırmanın durduğunu sanardı.
*/
func TestRunEvents_GecmisSirasiylaGonderilir(t *testing.T) {
	f := sseSetup(t)
	ctx := context.Background()

	for _, m := range []string{"birinci", "ikinci", "üçüncü"} {
		_, _, err := f.store.AppendEvent(ctx, f.run.ID, "info", m)
		require.NoError(t, err)
	}
	require.NoError(t, f.store.Finish(ctx, f.run.ID, runs.StatusSucceeded, nil, nil))

	es := olaylar(t, f.akis(t, ctx))
	require.GreaterOrEqual(t, len(es), 3)
	require.Equal(t, "birinci", es[0].Message)
	require.Equal(t, "ikinci", es[1].Message)
	require.Equal(t, "üçüncü", es[2].Message)
}

/*
GEÇMİŞTE GÖNDERİLEN OLAY CANLI AKIŞTAN TEKRAR GELMEZ.

Abonelik geçmişten ÖNCE başlıyor ki aradaki boşlukta üretilen olay kaçmasın;
bunun bedeli, geçmişte zaten gönderilmiş bir olayın canlı akıştan ikinci kez
gelebilmesi. `seq` ile ayıklanmasaydı kullanıcı aynı adımın iki kez
çalıştığını sanırdı.
*/
func TestRunEvents_TekrarEdenOlayAtlanir(t *testing.T) {
	f := sseSetup(t)
	ctx := context.Background()

	seq, _, err := f.store.AppendEvent(ctx, f.run.ID, "info", "tekrar edecek")
	require.NoError(t, err)

	// Çalıştırma SÜRÜYOR: handler canlı akışa geçecek.
	require.NoError(t, f.store.MarkRunning(ctx, f.run.ID))

	akisCtx, iptal := context.WithTimeout(ctx, 5*time.Second)
	defer iptal()

	bitti := make(chan string, 1)
	go func() { bitti <- f.akis(t, akisCtx) }()

	// Abonenin bağlanmasını bekle, sonra geçmişteki olayı YENİDEN yayınla.
	require.Eventually(t, func() bool { return f.bus.SubscriberCount(f.run.ID) > 0 },
		3*time.Second, 10*time.Millisecond, "abone bağlanmalı")

	f.bus.Publish(f.run.ID, events.Event{Seq: seq, Level: "info", Message: "tekrar edecek"})
	f.bus.Publish(f.run.ID, events.Event{Seq: seq + 1, Level: "info", Message: "yeni olay",
		Terminal: true, Status: string(runs.StatusSucceeded)})

	var govde string
	select {
	case govde = <-bitti:
	case <-time.After(5 * time.Second):
		t.Fatal("bitiş sinyalinde bağlantı kapanmadı")
	}

	require.Equal(t, 1, strings.Count(govde, "tekrar edecek"),
		"geçmişte gönderilen olay ikinci kez yazılmamalı")
	require.Contains(t, govde, "yeni olay")
}

// Akış başlıkları vekil sunucuların tamponlamasını da engelliyor; tamponlanan
// bir SSE akışı kullanıcıya "hiçbir şey olmuyor" gibi görünür.
func TestRunEvents_AkisBasliklari(t *testing.T) {
	f := sseSetup(t)
	ctx := context.Background()
	require.NoError(t, f.store.Finish(ctx, f.run.ID, runs.StatusSucceeded, nil, nil))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/runs/"+f.run.ID.String()+"/events", nil)
	f.routes.ServeHTTP(rec, req.WithContext(ctx))

	require.Equal(t, "text/event-stream", rec.Header().Get("Content-Type"))
	require.Equal(t, "no-cache", rec.Header().Get("Cache-Control"))
	require.Equal(t, "no", rec.Header().Get("X-Accel-Buffering"),
		"vekil sunucu SSE'yi tamponlamamalı")
}

// Olmayan çalıştırma akış açmaz: istemci sonsuza kadar boş bir akış dinlemek
// yerine hatayı görmeli.
func TestRunEvents_OlmayanCalistirmaAkisAcmaz(t *testing.T) {
	f := sseSetup(t)

	rec := httptest.NewRecorder()
	f.routes.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/runs/"+uuid.NewString()+"/events", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.NotContains(t, rec.Header().Get("Content-Type"), "event-stream")
}
