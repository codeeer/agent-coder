package runbatch

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/runs"
	"github.com/agent-coder/backend/internal/workflow"
)

/*
Bridge, kuyruk ile akış motoru arasındaki köprü.

Zamanlayıcı `Starter` ve `Tracker` arayüzlerini görür; bu tip onları var olan
`workflow.Launcher` ve `workflow.Store` üzerinden karşılar. Ayrı durmasının
sebebi zamanlayıcının test edilebilirliği: sıra ve sınır mantığı sahte bir
başlatıcıyla, veritabanına ve container'a hiç dokunmadan ölçülüyor.

Yeni bir başlatma yolu AÇILMIYOR: toplu iş de elle tetikleme, webhook ve Jira
ile aynı `Launcher`'dan geçer.
*/
type Bridge struct {
	launcher *workflow.Launcher
	store    *workflow.Store
}

// NewBridge yeni köprü üretir.
func NewBridge(l *workflow.Launcher, s *workflow.Store) *Bridge {
	return &Bridge{launcher: l, store: s}
}

// Start, öğenin akış çalışmasını başlatır ve çalışma kimliğini döner.
func (b *Bridge) Start(ctx context.Context, workflowID, projectID uuid.UUID, task string) (uuid.UUID, error) {
	run, err := b.launcher.Launch(ctx, workflow.LaunchInput{
		WorkflowID: workflowID,
		ProjectID:  projectID,
		Trigger:    workflow.TriggerManual,
		Input:      task,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return run.ID, nil
}

/*
Outcome, akış çalışmasının o anki sonucunu döner.

`kesildi` (interrupted) durumu öğeye AYNEN geçer: o çalışma yarım kaldı ve
"kaldığı yerden devam et" onu yeniden sıraya alabilmeli. `failed` ile
karıştırılsaydı düğme onu hiç görmezdi.
*/
func (b *Bridge) Outcome(ctx context.Context, runID uuid.UUID) (Outcome, error) {
	run, err := b.store.GetRun(ctx, runID)
	if err != nil {
		return Outcome{}, err
	}

	var msg string
	if run.Error != nil {
		msg = *run.Error
	}

	switch run.Status {
	case workflow.RunSucceeded:
		return Outcome{Finished: true, Status: ItemSucceeded}, nil
	case workflow.RunFailed:
		return Outcome{Finished: true, Status: ItemFailed, Error: msg,
			LimitHit: IsLimitError(errors.New(msg))}, nil
	case workflow.RunCancelled:
		return Outcome{Finished: true, Status: ItemCancelled, Error: msg}, nil
	case workflow.RunInterrupted:
		return Outcome{Finished: true, Status: ItemInterrupted, Error: msg}, nil
	default:
		return Outcome{}, nil // pending / running — sürüyor
	}
}

/*
IsLimitError, hatanın eşzamanlılık sınırından kaynaklandığını söyler.

İki yoldan da tanınmalı: doğrudan dönen hata (`errors.Is`) ve veritabanına
yazılmış hata METNİ. İkincisi metin karşılaştırmasıdır çünkü hata bir süreç ve
bir tablo sınırını geçerek geliyor — sarmalama orada hayatta kalmıyor.
Karşılaştırılan metin `runs.ErrTooManyRuns`'un kendisi olduğu için tek kaynak
korunuyor: mesaj değişirse burası da değişmiş olur.
*/
func IsLimitError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, runs.ErrTooManyRuns) {
		return true
	}
	return strings.Contains(err.Error(), runs.ErrTooManyRuns.Error())
}
