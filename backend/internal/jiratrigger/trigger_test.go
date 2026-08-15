package jiratrigger

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/integrations/jira"
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
