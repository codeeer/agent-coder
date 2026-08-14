/*
Package runbatch, bir akışı birden çok projede sıraya koyan toplu çalıştırmayı
yönetir (spec 023).

Toplu iş bir çalıştırma DEĞİLDİR: kendi kaydı vardır ve içindeki her öğe kendi
akış çalışmasına bağlanır. İkisini tek kayda sıkıştırmak, "otuz işin durumu" ile
"bir işin durumu" sorularını aynı ekrana sıkıştırmak olurdu.

Kuyruk kendi paralelliğini TANIMLAMAZ; mevcut eşzamanlılık sınırına uyar. "Aynı
anda kaç iş çalışır" sorusunun tek bir cevabı olmalı.
*/
package runbatch

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Toplu iş durumları.
//
//   - queued:    henüz hiçbir öğesi çalışmıyor
//   - running:   en az bir öğesi çalışıyor
//   - done:      bekleyen ve çalışan öğesi kalmadı
//   - cancelled: kullanıcı vazgeçti (çalışanlar kendi hâlinde sürer)
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusDone      = "done"
	StatusCancelled = "cancelled"
)

// Öğe durumları.
//
// `interrupted` ile `failed` AYNI ŞEY DEĞİL: kesilen iş hiç sonuç üretmedi,
// başarısız olan çalıştı ve bir sonuç üretti. "Kaldığı yerden devam et"
// yalnızca birincisini yeniden sıraya alır (spec 023 H5a).
const (
	ItemPending     = "pending"
	ItemRunning     = "running"
	ItemSucceeded   = "succeeded"
	ItemFailed      = "failed"
	ItemInterrupted = "interrupted"
	ItemCancelled   = "cancelled"
)

// Hatalar.
var (
	ErrNotFound = errors.New("toplu iş bulunamadı")

	// ErrNoProjects: hiç proje seçilmeden başlatılamaz (spec 023 H1).
	ErrNoProjects = errors.New("hiç proje seçilmedi")

	// ErrDuplicateProject: aynı proje aynı toplu işte iki kez yer alamaz.
	ErrDuplicateProject = errors.New("aynı proje birden fazla kez seçildi")

	ErrWorkflowNotFound = errors.New("akış bulunamadı")
	ErrProjectNotFound  = errors.New("proje bulunamadı")
)

// Counts, bir toplu işin öğe sayıları.
//
// Sayılar ayrı sorgularla değil tek sorguda hesaplanır: otuz öğelik bir listede
// öğe başına bir sorgu, liste ekranını otuz bir sorguya çıkarırdı.
type Counts struct {
	Total       int
	Pending     int
	Running     int
	Succeeded   int
	Failed      int
	Interrupted int
	Cancelled   int
}

// Batch, bir toplu çalıştırma kaydı.
type Batch struct {
	ID           uuid.UUID
	WorkflowID   uuid.UUID
	WorkflowName string // JOIN'den; ekran akışın adını gösteriyor
	Task         string
	Status       string
	Counts       Counts
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Item, toplu işin bir öğesi — tek bir projede tek bir akış çalışması.
type Item struct {
	ID          uuid.UUID
	BatchID     uuid.UUID
	ProjectID   uuid.UUID
	ProjectName string // JOIN'den; ekran proje adını gösteriyor
	Position    int
	Status      string

	// WorkflowRunID, çalışma başlatıldıktan SONRA dolar. Öğe kendi akış
	// çalışmasına bu alanla bağlanır.
	WorkflowRunID *uuid.UUID

	Error      string
	StartedAt  *time.Time
	FinishedAt *time.Time
}
