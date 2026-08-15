package jiratrigger

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/credentials"
	"github.com/agent-coder/backend/internal/integrations/jira"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Jira tetikleyicisi.
 *
 * Bu paket kendiliğinden çalıştırma açıyor: buradaki bir hata kimse istemeden
 * para harcar ve branch gönderir. Testler o yüzden "ne yapıyor"u değil "ne
 * yapmalı"yı sorguluyor.
 */

// ── Tarama aralığı ──────────────────────────────────────────────────────────

/*
BOZUK ARALIK SICAK DÖNGÜ YAPMAMALI.

`Run`, aralığı doğrudan `timer.Reset()`'e veriyor. Sıfır ya da eksi bir değer
zamanlayıcıyı anında ateşler ve tarama Jira'yı aralıksız dövmeye başlar.

Buraya sıfır GELMEMELİ: ayar `Min: 1` ile doğrulanıyor. Ama `settings.Int`
bilinmeyen bir anahtar için sıfır dönüyor (service.go), yani anahtar adı bir
gün değişirse tetikleyici sessizce sıcak döngüye girer — dışarıya çıkan ve
kota harcayan bir döngüye.

`runbatch.Scheduler` tam bu duruma karşı korumalı ve gerekçesi orada yazılı;
aynı sınıftaki bu tetikleyicide koruma yok.
*/
func TestTaramaAraligi_BozukDegerVarsayilanaDuser(t *testing.T) {
	sifir := &Trigger{interval: func() time.Duration { return 0 }}
	require.Equal(t, time.Minute, sifir.tarama(), "sıfır aralık sıcak döngü demek")

	eksi := &Trigger{interval: func() time.Duration { return -3 * time.Minute }}
	require.Equal(t, time.Minute, eksi.tarama(), "eksi aralık da ateşlemeyi anında yapar")

	yok := &Trigger{interval: nil}
	require.Equal(t, time.Minute, yok.tarama(), "aralık verilmemişse de dönmemeli")
}

func TestTaramaAraligi_GecerliDegerAynenKullanilir(t *testing.T) {
	tr := &Trigger{interval: func() time.Duration { return 5 * time.Minute }}
	require.Equal(t, 5*time.Minute, tr.tarama())
}

// ── Görev metni ─────────────────────────────────────────────────────────────

/*
Agent'a verilen metin ANAHTARI da taşır.

İlk adım `{{ input }}` yazdığında ekranda yalnızca özet görünseydi, agent hangi
task üzerinde çalıştığını yazamaz ve PR başlığı task'a bağlanamazdı.
*/
func TestTaskText_AnahtarVeOzetBirlikte(t *testing.T) {
	metin := taskText(jira.Issue{Key: "SCRUM-42", Summary: "Node 24'e yükselt"})

	require.Equal(t, "SCRUM-42: Node 24'e yükselt", metin)
}

// Açıklama varsa BOŞ SATIRLA ayrılır: özetle açıklamanın birbirine yapışması
// modele tek bir cümle gibi görünürdü.
func TestTaskText_AciklamaBosSatirlaEklenir(t *testing.T) {
	metin := taskText(jira.Issue{
		Key: "SCRUM-42", Summary: "Node 24'e yükselt",
		Description: "pom.xml içindeki parent sürümü de güncellensin.",
	})

	require.Equal(t,
		"SCRUM-42: Node 24'e yükselt\n\npom.xml içindeki parent sürümü de güncellensin.",
		metin)
}

// Açıklama yoksa metin ORADA biter — sonda boş satır kalmaz.
func TestTaskText_AciklamaYokkenBosSatirKalmaz(t *testing.T) {
	metin := taskText(jira.Issue{Key: "SCRUM-42", Summary: "Kısa iş"})

	require.NotContains(t, metin, "\n", "açıklama yokken satır sonu eklenmemeli")
}

// ── Tetikleyici düğümün bulunması ───────────────────────────────────────────

func TestJiraTriggerOf_DugumBulunur(t *testing.T) {
	g := workflow.Graph{Nodes: []workflow.Node{
		{ID: "a", Kind: workflow.KindAgent},
		{ID: "t", Kind: workflow.KindTriggerJira},
	}}

	node, ok := jiraTriggerOf(g)
	require.True(t, ok)
	require.Equal(t, "t", node.ID)
}

/*
Jira tetikleyicisi OLMAYAN akış taranmaz.

Tarama bütün etkin akışları geziyor; düğümü olmayanı atlamak yalnızca boşa iş
yapmamak değil, JQL'siz bir akışı yanlışlıkla tetiklememek demek.
*/
func TestJiraTriggerOf_DugumYoksaFalse(t *testing.T) {
	g := workflow.Graph{Nodes: []workflow.Node{
		{ID: "a", Kind: workflow.KindAgent},
		{ID: "m", Kind: workflow.KindTriggerManual},
	}}

	_, ok := jiraTriggerOf(g)
	require.False(t, ok)
}

func TestJiraTriggerOf_BosGraf(t *testing.T) {
	_, ok := jiraTriggerOf(workflow.Graph{})
	require.False(t, ok)
}

// ── Process: işaret, geri alma ve tekrar kontrolü ───────────────────────────

/*
processFixture, GERÇEK bağımlılıklarla bir tetikleyici kurar.

Etkin sürümü OLMAYAN bir akış veriyor ve kaldıraç bu: `Launch` o akış için
`ErrNoVersion` ile düşüyor. Böylece "başlatma başarısız" durumu, tek bir
container çalıştırmadan ve tek bir sahte nesne yazmadan üretilebiliyor.

`creds` ve `client` nil: `Process` yolunda ikisi de kullanılmıyor.
*/
func processFixture(t *testing.T) (*Trigger, *workflow.Store, uuid.UUID) {
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
	require.Nil(t, wf.ActiveVersionID, "ön koşul: akışın etkin sürümü olmamalı")

	launcher := workflow.NewLauncher(store, workflow.NewExecutor(store, workflow.Handlers{}, nil))
	return New(store, launcher, nil, nil, nil, nil), store, wf.ID
}

/*
BAŞLATMA DÜŞERSE İŞARET GERİ ALINIR.

Bu paketin en sert değişmezi. İşaret başlatmadan ÖNCE konuyor — iki tetikleme
yolu aynı anda gelirse yalnızca biri kaydı oluşturabilsin diye. Başlatma sonra
düşer ve işaret kalırsa, o task "işlendi" sayılır ve BİR DAHA HİÇ denenmez;
oysa hiçbir şey çalışmamıştır. Kullanıcı da bunu hiçbir yerde göremez.
*/
func TestProcess_BaslatmaDusunceIsaretGeriAlinir(t *testing.T) {
	tr, store, wfID := processFixture(t)
	ctx := context.Background()

	started, err := tr.Process(ctx, wfID,
		jira.Issue{Key: "SCRUM-1", Summary: "iş", UpdatedAt: "2026-08-10T00:00:00"})
	require.Error(t, err, "etkin sürümü olmayan akış başlatılamaz")
	require.False(t, started)

	// İşaret geri alınmış olmalı: aynı task yeniden İŞARETLENEBİLİYORSA
	// yeniden denenebilir demektir.
	fresh, err := store.MarkProcessed(ctx, wfID, "SCRUM-1", "2026-08-10T00:00:00")
	require.NoError(t, err)
	require.True(t, fresh, "işaret geri alınmadı — bu task bir daha hiç denenmezdi")
}

/*
ZATEN İŞLENMİŞ TASK BAŞLATILMAZ — ve hata da değildir.

Hata sayılsaydı her tarama, daha önce işlenmiş her task için tarama durumuna
hata yazardı ve gerçek arızalar o gürültünün içinde kaybolurdu.

Başlatmaya HİÇ kalkışılmadığının kanıtı `err == nil`: bu akışın etkin sürümü
yok, yani kalkışan her çağrı düşerdi.
*/
func TestProcess_ZatenIslenmisTaskBaslatilmaz(t *testing.T) {
	tr, store, wfID := processFixture(t)
	ctx := context.Background()

	fresh, err := store.MarkProcessed(ctx, wfID, "SCRUM-1", "2026-08-10T00:00:00")
	require.NoError(t, err)
	require.True(t, fresh)

	started, err := tr.Process(ctx, wfID,
		jira.Issue{Key: "SCRUM-1", UpdatedAt: "2026-08-10T00:00:00"})
	require.NoError(t, err, "zaten işlenmiş task bir arıza değil")
	require.False(t, started)
}

/*
GÜNCELLENEN TASK YENİDEN DENENİR.

Tekrar anahtarı üçlü: akış + task anahtarı + güncellenme zamanı. Yalnızca task
anahtarına bakılsaydı, birine yeni bir bilgi eklendiğinde akış onu hiç görmezdi
— kullanıcı açıklamayı düzeltip beklerdi ve hiçbir şey olmazdı.

Denendiğinin kanıtı yine HATA: atlansaydı sessizce (false, nil) dönerdi.
*/
func TestProcess_GuncellenenTaskYenidenDenenir(t *testing.T) {
	tr, store, wfID := processFixture(t)
	ctx := context.Background()

	fresh, err := store.MarkProcessed(ctx, wfID, "SCRUM-1", "2026-08-10T00:00:00")
	require.NoError(t, err)
	require.True(t, fresh)

	_, err = tr.Process(ctx, wfID,
		jira.Issue{Key: "SCRUM-1", UpdatedAt: "2026-08-11T09:30:00"})
	require.Error(t, err, "güncellenmiş task atlanmamalı, başlatmaya kalkışmalı")
}

// ── Tarama: sayaçlar ve hata kaydı ──────────────────────────────────────────

// sessizBus, akış olaylarını yutan yayıncı. Testte kimse dinlemiyor.
type sessizBus struct{}

func (sessizBus) Publish(uuid.UUID, string, string) {}

/*
sahteJira, arama ucuna sabit bir sonuç kümesi döndüren sunucu.

Atlassian'ın gerçek yanıtı değil KENDİ kodumuz doğrulanıyor: kaç task
bulunduğu, kaçının başladığı ve hatanın kaydedilip kaydedilmediği.
*/
func sahteJira(t *testing.T, anahtarlar []string, durum int) *httptest.Server {
	t.Helper()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if durum != http.StatusOK {
			w.WriteHeader(durum)
			return
		}
		var issues []map[string]any
		for _, k := range anahtarlar {
			issues = append(issues, map[string]any{
				"key": k,
				"fields": map[string]any{
					"summary": k + " özeti",
					"updated": "2026-08-10T00:00:00.000+0300",
				},
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"issues": issues})
	}))
	t.Cleanup(s.Close)
	return s
}

type taramaFixture struct {
	tr    *Trigger
	store *workflow.Store
	wf    workflow.Workflow
	node  workflow.Node
}

/*
scanFixture, taranabilir bir akış kurar.

`baslatilabilir` false ise akışın etkin sürümü olmaz ve her başlatma düşer —
"bulundu ama başlamadı" durumunu üretmenin yolu bu.

Handler kümesi BOŞ: başlatma başarılı olsa bile arka plandaki çalışma ilk
adımda "bu türü çalıştıramıyorum" diyip duruyor. Böylece sayaçlar gerçek
başlatma yolundan geçerek ölçülüyor ama tek bir container açılmıyor.
*/
func scanFixture(t *testing.T, jiraURL string, kimlikVar, baslatilabilir bool) taramaFixture {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "workflows", "runs", "projects", "agents", "credentials")

	ctx := context.Background()
	var projectID, agentID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO projects (name, repo_url) VALUES ('Deneme','https://example.com/r.git')
		 RETURNING id`).Scan(&projectID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO agents (slug, name, prompt, source) VALUES ('coder','Coder','p','builtin')
		 RETURNING id`).Scan(&agentID))

	store := workflow.NewStore(pool)
	wf, err := store.Create(ctx, workflow.CreateInput{ProjectID: projectID, Name: "Jira akışı"})
	require.NoError(t, err)

	node := workflow.Node{ID: "t1", Kind: workflow.KindTriggerJira,
		Config: workflow.NodeConfig{JQL: "project = SCRUM"}}

	if baslatilabilir {
		_, err = store.SaveVersion(ctx, wf.ID, workflow.Graph{
			Nodes: []workflow.Node{node,
				{ID: "a1", Kind: workflow.KindAgent, Name: "Analiz",
					Config: workflow.NodeConfig{AgentID: agentID.String(), Model: "m1",
						Prompt: "{{ input }}"}}},
			Edges: []workflow.Edge{{From: "t1", To: "a1"}},
		})
		require.NoError(t, err)
	}
	wf, err = store.Get(ctx, wf.ID)
	require.NoError(t, err)

	cipher, err := secrets.NewCipher(base64.StdEncoding.EncodeToString(make([]byte, secrets.KeySize)))
	require.NoError(t, err)
	creds := credentials.NewStore(pool, cipher)
	if kimlikVar {
		require.NoError(t, creds.Put(ctx, credentials.KindJira, "token",
			map[string]string{"base_url": jiraURL, "email": "u@e.com"}))
	}

	launcher := workflow.NewLauncher(store,
		workflow.NewExecutor(store, workflow.Handlers{}, sessizBus{}))

	return taramaFixture{
		tr:    New(store, launcher, creds, jira.New(nil), nil, func() int { return 20 }),
		store: store, wf: wf, node: node,
	}
}

/*
BULUNAN İLE BAŞLAYAN AYRI SAYILIR.

Panelde tek görünür iz bu iki sayı: "N task eşleşti, M akış başlatıldı".
Aynı sayı olsalardı kullanıcı, JQL'in hiç eşleşmediği durumla hepsinin daha
önce işlendiği durumu ayırt edemezdi.
*/
func TestScanOne_BulunanVeBaslayanAyriSayilir(t *testing.T) {
	f := scanFixture(t, sahteJira(t, []string{"SCRUM-1", "SCRUM-2", "SCRUM-3"}, 200).URL, true, true)
	ctx := context.Background()

	f.tr.scanOne(ctx, f.wf, f.node)

	st, err := f.store.ScanState(ctx, f.wf.ID)
	require.NoError(t, err)
	require.Equal(t, 3, st.Found)
	require.Equal(t, 3, st.Started)
	require.Nil(t, st.Error)
}

// Zaten işlenmiş task BULUNANA dahildir, BAŞLAYANA değil: JQL onu hâlâ
// eşleştiriyor ve kullanıcı sorgusunun kapsamını bu sayıdan okuyor.
func TestScanOne_ZatenIslenmisTaskBaslayanaSayilmaz(t *testing.T) {
	f := scanFixture(t, sahteJira(t, []string{"SCRUM-1", "SCRUM-2", "SCRUM-3"}, 200).URL, true, true)
	ctx := context.Background()

	fresh, err := f.store.MarkProcessed(ctx, f.wf.ID, "SCRUM-2", "2026-08-10T00:00:00.000+0300")
	require.NoError(t, err)
	require.True(t, fresh)

	f.tr.scanOne(ctx, f.wf, f.node)

	st, err := f.store.ScanState(ctx, f.wf.ID)
	require.NoError(t, err)
	require.Equal(t, 3, st.Found, "eşleşen task sayısı değişmez")
	require.Equal(t, 2, st.Started, "daha önce işlenmiş olan yeniden başlatılmaz")
}

/*
BAŞLATILAMAYAN TASK'LAR SESSİZ KALMAZ.

Bulundu ama hiçbiri başlamadıysa kullanıcı sebebini görebilmeli; sayılar tek
başına "0 başladı" diyor ama NEDEN demiyor.
*/
func TestScanOne_BaslatmaDusersaHataKaydedilir(t *testing.T) {
	f := scanFixture(t, sahteJira(t, []string{"SCRUM-1", "SCRUM-2"}, 200).URL, true, false)
	ctx := context.Background()

	f.tr.scanOne(ctx, f.wf, f.node)

	st, err := f.store.ScanState(ctx, f.wf.ID)
	require.NoError(t, err)
	require.Equal(t, 2, st.Found)
	require.Equal(t, 0, st.Started)
	require.NotNil(t, st.Error, "başlatılamayan task sessizce kaybolmamalı")
}

/*
JIRA ERİŞİMİ YOKKEN TARAMA DURUMU YİNE YAZILIR.

Sessizce dönülseydi panel "henüz taranmadı" derdi ve kullanıcı eksik olanın
kimlik bilgisi olduğunu hiçbir yerden öğrenemezdi.
*/
func TestScanOne_KimlikYokkenSebepYazilir(t *testing.T) {
	f := scanFixture(t, "http://kullanilmayacak.local", false, true)
	ctx := context.Background()

	f.tr.scanOne(ctx, f.wf, f.node)

	st, err := f.store.ScanState(ctx, f.wf.ID)
	require.NoError(t, err)
	require.NotNil(t, st.Error)
	require.Contains(t, *st.Error, "Jira erişimi")
	require.Equal(t, 0, st.Started, "kimlik yokken hiçbir şey başlatılmamalı")
}

// Arama düşerse sebep kaydedilir ve bulunan sayısı sıfır kalır — yarım bir
// sonuç, tam sonuç gibi gösterilmemeli.
func TestScanOne_AramaDusersaSebepYazilir(t *testing.T) {
	f := scanFixture(t, sahteJira(t, nil, http.StatusInternalServerError).URL, true, true)
	ctx := context.Background()

	f.tr.scanOne(ctx, f.wf, f.node)

	st, err := f.store.ScanState(ctx, f.wf.ID)
	require.NoError(t, err)
	require.NotNil(t, st.Error)
	require.Equal(t, 0, st.Found)
	require.Equal(t, 0, st.Started)
}
