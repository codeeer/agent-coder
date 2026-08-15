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
