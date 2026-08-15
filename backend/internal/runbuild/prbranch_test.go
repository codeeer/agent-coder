package runbuild

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * PR'ın hangi branch'ten açılacağı.
 *
 * Yanlış branch'ten açılan bir PR, gözden geçirene başka birinin işini gösterir
 * ve yanlış değişiklik birleştirilebilir.
 *
 * Buradaki testler AÇIK branch yolunu kapsıyor — kullanıcı düğümde bir değer
 * yazmışsa. Yazmadığında graf üzerinden gönderim yapan ata aranıyor; o yol
 * akış sürümünü okumayı gerektiriyor ve ayrı bir tur.
 */

func prGirdi(headBranch string, ctx workflow.Context) workflow.ExecInput {
	return workflow.ExecInput{
		Node:    workflow.Node{Config: workflow.NodeConfig{HeadBranch: headBranch}},
		Context: ctx,
	}
}

// Açık branch şablon içerebilir: kullanıcı genelde önceki adımın gönderdiği
// branch'i yazıyor.
func TestHeadBranch_AcikBranchSablondanCozulur(t *testing.T) {
	branch, err := (&PRHandler{}).headBranch(context.Background(),
		prGirdi("{{ steps.a1.branch }}", workflow.Context{
			Steps: map[string]workflow.StepResult{"a1": {Branch: "agent/node-24"}},
		}))

	require.NoError(t, err)
	require.Equal(t, "agent/node-24", branch)
}

/*
BOŞ ÇÖZÜLEN BRANCH SEBEBİYLE REDDEDİLİR.

Önceki adım değişiklik üretmediyse branch alanı boş kalıyor. Boş bir değerle
devam edilseydi PR isteği kaynak branch olmadan gider ve GitHub'ın ham hatası
kullanıcıya hiçbir şey anlatmazdı; asıl sebep "önceki adım gönderim yapmadı".
*/
func TestHeadBranch_BosCozulenBranchSebebiyleReddedilir(t *testing.T) {
	_, err := (&PRHandler{}).headBranch(context.Background(),
		prGirdi("{{ steps.a1.branch }}", workflow.Context{
			Steps: map[string]workflow.StepResult{"a1": {Branch: ""}},
		}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "gönderim", "hata sebebi kullanıcının anlayacağı dilde olmalı")
}

// Çözülemeyen referans sessizce boş branch'e dönüşmez.
func TestHeadBranch_CozulemeyenReferansHataVerir(t *testing.T) {
	_, err := (&PRHandler{}).headBranch(context.Background(),
		prGirdi("{{ steps.olmayan.branch }}", workflow.Context{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "kaynak branch")
}

// ── Graf yolu: branch yazılmadığında ────────────────────────────────────────

/*
prGrafi, gönderim yapan bir agent adımı ve ona bağlı bir PR düğümü üretir.

`autoPush` false ise adım branch göndermiyor demektir — PR'ın kaynağı yok.
*/
func prGrafi(autoPush bool) workflow.Graph {
	return workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "a1", Kind: workflow.KindAgent,
				Config: workflow.NodeConfig{AutoPush: autoPush}},
			{ID: "pr", Kind: workflow.KindGitHubPR},
		},
		Edges: []workflow.Edge{{From: "a1", To: "pr"}},
	}
}

// grafVeren, sabit bir graf döndüren enjeksiyon; çağrıldığını da sayar.
func grafVeren(g workflow.Graph, sayac *int) func(context.Context, string) (workflow.Graph, error) {
	return func(context.Context, string) (workflow.Graph, error) {
		if sayac != nil {
			*sayac++
		}
		return g, nil
	}
}

// prDugumu, PR düğümünü ve verilen adım sonuçlarını taşıyan girdi.
func prDugumu(headBranch string, adimlar map[string]workflow.StepResult) workflow.ExecInput {
	return workflow.ExecInput{
		Node:    workflow.Node{ID: "pr", Kind: workflow.KindGitHubPR, Config: workflow.NodeConfig{HeadBranch: headBranch}},
		Context: workflow.Context{Steps: adimlar},
	}
}

/*
BRANCH YAZILMADIYSA GÖNDERİM YAPAN ATADAN ALINIR.

Kullanıcının her PR düğümünde branch adı yazmasını beklemek, akışın zaten
bildiği bir şeyi elle tekrarlatmak olurdu — ve elle yazılan ad, gönderilen
branch değişince sessizce yanlışa düşerdi.
*/
func TestHeadBranch_GrafYolundaGonderimYapanAtadanAlinir(t *testing.T) {
	h := NewPRHandler(nil, nil, nil, grafVeren(prGrafi(true), nil))

	branch, err := h.headBranch(context.Background(),
		prDugumu("", map[string]workflow.StepResult{"a1": {Branch: "agent/node-24"}}))

	require.NoError(t, err)
	require.Equal(t, "agent/node-24", branch)
}

// Açık branch varsa graf HİÇ OKUNMAZ: kullanıcının yazdığı değer kesindir ve
// gereksiz bir sürüm okuması yapılmamalı.
func TestHeadBranch_AcikBranchVarsaGrafOkunmaz(t *testing.T) {
	sayac := 0
	h := NewPRHandler(nil, nil, nil, grafVeren(prGrafi(true), &sayac))

	branch, err := h.headBranch(context.Background(),
		prDugumu("elle/yazilan", nil))

	require.NoError(t, err)
	require.Equal(t, "elle/yazilan", branch)
	require.Zero(t, sayac, "açık branch varken graf okunmamalı")
}

/*
GÖNDERİM YAPAN ATA YOKSA SEBEBİYLE REDDEDİLİR.

Doğrulama bunu kaydetme anında da yakalıyor; buradaki kontrol o kapıdan
geçmemiş bir sürüm için son savunma. Sessizce boş branch'le devam etmek,
GitHub'ın anlaşılmaz hatasıyla dakikalar sonra patlamak demekti.
*/
func TestHeadBranch_GonderimYapanAtaYoksaReddedilir(t *testing.T) {
	h := NewPRHandler(nil, nil, nil, grafVeren(prGrafi(false), nil))

	_, err := h.headBranch(context.Background(), prDugumu("", nil))

	require.Error(t, err)
	require.Contains(t, err.Error(), "gönderim yapmalı")
}

/*
ATA VAR AMA BRANCH GÖNDERMEDİYSE HANGİ ADIM OLDUĞU YAZILIR.

Agent çalıştı ama değişiklik üretmediyse branch oluşmuyor. Hata yalnızca
"branch yok" deseydi, çok adımlı bir akışta kullanıcı hangi adımın boş
döndüğünü aramak zorunda kalırdı.
*/
func TestHeadBranch_AtaBranchGondermediysAdimAdlandirilir(t *testing.T) {
	h := NewPRHandler(nil, nil, nil, grafVeren(prGrafi(true), nil))

	_, err := h.headBranch(context.Background(),
		prDugumu("", map[string]workflow.StepResult{"a1": {Branch: ""}}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "a1", "hata hangi adımın göndermediğini söylemeli")
}
