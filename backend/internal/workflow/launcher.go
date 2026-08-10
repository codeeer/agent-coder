package workflow

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// Launcher, bir akışı başlatır.
//
// TEK KAPI: elle başlatma, dışarıdan tetikleme ve Jira tetikleyicisi hepsi
// buradan geçer. Ayrı ayrı yazılsalardı "etkin sürümü bul, çalışma kaydını
// oluştur, motoru başlat" üçlüsü üç kez tekrarlanır ve biri güncellenip
// diğerleri unutulurdu.
type Launcher struct {
	store *Store
	exec  *Executor
}

// NewLauncher yeni başlatıcı üretir.
func NewLauncher(store *Store, exec *Executor) *Launcher {
	return &Launcher{store: store, exec: exec}
}

// LaunchInput, akış başlatma isteği.
type LaunchInput struct {
	WorkflowID uuid.UUID
	Trigger    TriggerKind
	Payload    map[string]string
	Input      string
}

// Launch, akışı başlatır ve hemen döner.
//
// Motor arka planda çalışır: bir akış dakikalarca sürebilir, çağıran bekleyemez.
func (l *Launcher) Launch(ctx context.Context, in LaunchInput) (Run, error) {
	wf, err := l.store.Get(ctx, in.WorkflowID)
	if err != nil {
		return Run{}, err
	}

	version, err := l.store.ActiveVersion(ctx, in.WorkflowID)
	if err != nil {
		return Run{}, err
	}

	run, err := l.store.CreateRun(ctx, CreateRunInput{
		Workflow: wf, Version: version,
		Trigger: in.Trigger, Payload: in.Payload, Input: in.Input,
	})
	if err != nil {
		return Run{}, err
	}

	l.exec.Start(run, version.Graph)

	slog.InfoContext(ctx, "akış başlatıldı",
		"workflow_id", in.WorkflowID, "workflow_run_id", run.ID,
		"version", run.Version, "trigger", in.Trigger)
	return run, nil
}

// ActiveRuns, akışın süren çalışma sayısı (uyarı için).
func (l *Launcher) ActiveRuns(ctx context.Context, workflowID uuid.UUID) (int, error) {
	return l.store.ActiveRuns(ctx, workflowID)
}
