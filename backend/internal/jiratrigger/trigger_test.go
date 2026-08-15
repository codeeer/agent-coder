package jiratrigger

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/integrations/jira"
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
