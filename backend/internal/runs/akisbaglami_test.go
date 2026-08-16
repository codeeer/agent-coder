package runs_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runs"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * Çalıştırmanın AKIŞ BAĞLAMI.
 *
 * Bir çalıştırma bir akışın adımı olabiliyor ve o zaman tek başına silinemiyor;
 * silinebileceği tek yer akış çalışmasının kendi ekranı. Çalıştırmalar ekranı
 * kullanıcıyı oraya YÖNLENDİREBİLMEK için akışın kimliğini bilmek zorunda —
 * yalnızca adını bilmek "şu ekrana git" diyip adresi vermemek demek.
 */

// akisAdimi, bir akış çalışmasının adımı olan bir çalıştırma kurar ve akışın
// kimliğini döner.
func (f fixture) akisAdimi(t *testing.T, runID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	flows := workflow.NewStore(f.pool)
	wf, err := flows.Create(ctx, workflow.CreateInput{ProjectID: f.projectID, Name: "Akış"})
	require.NoError(t, err)

	_, err = flows.SaveVersion(ctx, wf.ID, workflow.Graph{
		Nodes: []workflow.Node{
			{ID: "t1", Kind: workflow.KindTriggerManual},
			{ID: "a1", Kind: workflow.KindAgent, Name: "Analiz",
				Config: workflow.NodeConfig{
					AgentID: f.agentID.String(), Model: "m1", Prompt: "{{ input }}"}},
		},
		Edges: []workflow.Edge{{From: "t1", To: "a1"}},
	})
	require.NoError(t, err)

	run, err := workflow.NewLauncher(flows,
		workflow.NewExecutor(flows, workflow.Handlers{}, sessizOtobus{})).
		Launch(ctx, workflow.LaunchInput{
			WorkflowID: wf.ID, Trigger: workflow.TriggerManual, Input: "iş",
		})
	require.NoError(t, err)

	// Adımı bu çalıştırmaya bağla.
	_, err = f.pool.Exec(ctx,
		`UPDATE workflow_steps SET run_id = $1
		  WHERE workflow_run_id = $2 AND node_id = 'a1'`, runID, run.ID)
	require.NoError(t, err)

	return wf.ID
}

// sessizOtobus, akış olaylarını yutar.
type sessizOtobus struct{}

func (sessizOtobus) Publish(uuid.UUID, string, string) {}

/*
AKIŞ ADIMI, AKIŞIN KİMLİĞİNİ TAŞIR.

Adı taşınıyordu, kimliği taşınmıyordu. Çalıştırmalar ekranı bu yüzden "akış
çalışmasını silin" diyebiliyor ama oraya BAĞLANAMIYORDU — kullanıcıyı adı
bilinen, adresi bilinmeyen bir ekrana yolluyordu.
*/
func TestList_AkisAdimiAkisKimliginiTasir(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	run := f.create(t, "akışın adımı")
	wfID := f.akisAdimi(t, run.ID)

	okunan, err := f.store.Get(ctx, run.ID)
	require.NoError(t, err)
	require.NotNil(t, okunan.WorkflowID, "akış adımının akış kimliği olmalı")
	require.Equal(t, wfID, *okunan.WorkflowID)
	require.NotNil(t, okunan.WorkflowRunID)

	// Liste de aynı bilgiyi taşımalı: yönlendirme orada yapılıyor.
	liste, _, err := f.store.List(ctx, runs.ListFilter{Limit: 50})
	require.NoError(t, err)
	var bulundu bool
	for _, r := range liste {
		if r.ID == run.ID {
			bulundu = true
			require.NotNil(t, r.WorkflowID, "listede de akış kimliği olmalı")
			require.Equal(t, wfID, *r.WorkflowID)
		}
	}
	require.True(t, bulundu)
}

// Akışa bağlı OLMAYAN çalıştırmada alan boş kalır; uydurma bir kimlik,
// tıklandığında var olmayan bir akışa götürürdü.
func TestList_SiradanCalistirmadaAkisKimligiBos(t *testing.T) {
	f := setup(t)

	run := f.create(t, "sıradan")
	okunan, err := f.store.Get(context.Background(), run.ID)
	require.NoError(t, err)
	require.Nil(t, okunan.WorkflowID)
	require.Nil(t, okunan.WorkflowRunID)
}
