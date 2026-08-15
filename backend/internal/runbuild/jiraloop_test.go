package runbuild

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/integrations/jira"
	"github.com/agent-coder/backend/internal/testutil"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Kendi yazdığımız yorumun tetikleyici sayılmaması.
 *
 * SONSUZ DÖNGÜ RİSKİ. Akış Jira'ya yorum yazınca task'ın güncellenme zamanı
 * değişiyor; tekrar-işleme koruması o zamanı anahtar olarak kullandığı için
 * akış kendi bıraktığı izi yeni bir iş sanıp yeniden başlıyordu — her turda
 * yeni bir PR ve yeni bir yorum (spec 009, Ölçüm 4).
 *
 * Koruma üç koşula bağlı ve üçü de burada ölçülüyor. Hiçbiri test edilmiyordu.
 */

// yorumSonrasiZaman, sahte Jira'nın yorumdan sonra bildirdiği güncellenme anı.
const yorumSonrasiZaman = "2026-08-15T12:00:00.000+0300"

// sahteIssueSunucu, tek task okuma ucunu karşılar.
func sahteIssueSunucu(t *testing.T) *httptest.Server {
	t.Helper()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, _ := strings.CutPrefix(r.URL.Path, "/rest/api/3/issue/")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key": key,
			"fields": map[string]any{
				"summary": "iş",
				"updated": yorumSonrasiZaman,
			},
		})
	}))
	t.Cleanup(s.Close)
	return s
}

type jiraLoopFixture struct {
	h     *JiraHandler
	store *workflow.Store
	wfID  uuid.UUID
	cred  jira.CommentInput
}

func jiraLoopSetup(t *testing.T, issueKey string) jiraLoopFixture {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "workflows", "runs", "projects", "agents")

	ctx := context.Background()
	var projectID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, repo_url) VALUES ('Deneme','https://example.com/r.git')
		 RETURNING id`).Scan(&projectID))

	store := workflow.NewStore(pool)
	wf, err := store.Create(ctx, workflow.CreateInput{ProjectID: projectID, Name: "Jira akışı"})
	require.NoError(t, err)

	srv := sahteIssueSunucu(t)

	return jiraLoopFixture{
		// creds gerekmiyor: `claimOwnUpdate` kimliği çağırandan hazır alıyor.
		h:     NewJiraHandler(nil, jira.New(nil), store),
		store: store,
		wfID:  wf.ID,
		cred:  jira.CommentInput{BaseURL: srv.URL, IssueKey: issueKey},
	}
}

// tetikli, Jira'nın başlattığı bir çalıştırmayı taklit eder.
func tetikli(wfID uuid.UUID, payloadKey string) workflow.ExecInput {
	return workflow.ExecInput{Run: workflow.Run{
		WorkflowID:     wfID,
		TriggerKind:    workflow.TriggerJira,
		TriggerPayload: map[string]string{"key": payloadKey},
	}}
}

/*
DÖNGÜYÜ KIRAN ADIM: yorumdan sonra oluşan güncellenme "bizim" sayılır.

Kanıt, o zaman damgasının artık işlenmiş olması — bir sonraki tarama aynı
task'ı gördüğünde yeni iş sanmayacak.
*/
func TestClaimOwnUpdate_KendiYazdigimizGuncellemeIsaretlenir(t *testing.T) {
	f := jiraLoopSetup(t, "SCRUM-1")
	ctx := context.Background()

	f.h.claimOwnUpdate(ctx, tetikli(f.wfID, "SCRUM-1"), f.cred)

	fresh, err := f.store.MarkProcessed(ctx, f.wfID, "SCRUM-1", yorumSonrasiZaman)
	require.NoError(t, err)
	require.False(t, fresh,
		"yorumdan sonraki güncelleme işaretlenmedi — tarama onu yeni iş sanıp döngüye girerdi")
}

/*
ELLE BAŞLATILAN ÇALIŞTIRMADA İŞARET KONMAZ.

Ortada tetikleyici yok, dolayısıyla kırılacak bir döngü de yok. İşaret
konsaydı, o task'a sonradan gelen GERÇEK bir güncelleme yutulurdu.
*/
func TestClaimOwnUpdate_ElleBaslatilanCalistirmadaIsaretKonmaz(t *testing.T) {
	f := jiraLoopSetup(t, "SCRUM-1")
	ctx := context.Background()

	in := tetikli(f.wfID, "SCRUM-1")
	in.Run.TriggerKind = workflow.TriggerManual

	f.h.claimOwnUpdate(ctx, in, f.cred)

	fresh, err := f.store.MarkProcessed(ctx, f.wfID, "SCRUM-1", yorumSonrasiZaman)
	require.NoError(t, err)
	require.True(t, fresh, "tetikleyicisiz çalıştırma başkasının işaretine dokunmamalı")
}

/*
BAŞKA BİR ISSUE'YA YAZILAN YORUM İŞARETLENMEZ.

Akış, kendisini başlatan task'tan farklı bir issue'ya da yorum yazabilir; o
tetikleyiciyle ilgisiz bir iştir. İşaret konsaydı, o issue'ya gelen gerçek bir
güncelleme sessizce yutulur ve kullanıcı akışının neden çalışmadığını
anlayamazdı.
*/
func TestClaimOwnUpdate_FarkliIssueyaYorumIsaretlenmez(t *testing.T) {
	f := jiraLoopSetup(t, "BASKA-9")
	ctx := context.Background()

	// Çalıştırmayı SCRUM-1 başlattı, yorum BASKA-9'a yazıldı.
	f.h.claimOwnUpdate(ctx, tetikli(f.wfID, "SCRUM-1"), f.cred)

	fresh, err := f.store.MarkProcessed(ctx, f.wfID, "BASKA-9", yorumSonrasiZaman)
	require.NoError(t, err)
	require.True(t, fresh, "ilgisiz issue'nun işaretine dokunulmamalı")
}

/*
ANAHTAR KARŞILAŞTIRMASI BÜYÜK/KÜÇÜK HARF AYIRMAZ.

Jira anahtarları büyük harfle yazılıyor ama şablondan ya da webhook'tan küçük
harfle gelebiliyor. Harfe duyarlı karşılaştırılsaydı koruma sessizce devre dışı
kalır ve döngü geri gelirdi — üstelik yalnızca bazı kurulumlarda.
*/
func TestClaimOwnUpdate_AnahtarKarsilastirmasiHarfeDuyarsiz(t *testing.T) {
	f := jiraLoopSetup(t, "SCRUM-1")
	ctx := context.Background()

	f.h.claimOwnUpdate(ctx, tetikli(f.wfID, "scrum-1"), f.cred)

	fresh, err := f.store.MarkProcessed(ctx, f.wfID, "SCRUM-1", yorumSonrasiZaman)
	require.NoError(t, err)
	require.False(t, fresh, "küçük harfli anahtar korumayı devre dışı bırakmamalı")
}
