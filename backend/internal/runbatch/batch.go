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
	Total       int `json:"total"`
	Pending     int `json:"pending"`
	Running     int `json:"running"`
	Succeeded   int `json:"succeeded"`
	Failed      int `json:"failed"`
	Interrupted int `json:"interrupted"`
	Cancelled   int `json:"cancelled"`
}

// Batch, bir toplu çalıştırma kaydı.
type Batch struct {
	ID           uuid.UUID `json:"id"`
	WorkflowID   uuid.UUID `json:"workflowId"`
	WorkflowName string    `json:"workflowName"` // JOIN'den; ekran akışın adını gösteriyor
	Task         string    `json:"task"`
	Status       string    `json:"status"`
	Counts       Counts    `json:"counts"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Item, toplu işin bir öğesi — tek bir projede tek bir akış çalışması.
type Item struct {
	ID          uuid.UUID `json:"id"`
	BatchID     uuid.UUID `json:"batchId"`
	ProjectID   uuid.UUID `json:"projectId"`
	ProjectName string    `json:"projectName"` // JOIN'den; ekran proje adını gösteriyor
	Position    int       `json:"position"`
	Status      string    `json:"status"`

	// WorkflowRunID, çalışma başlatıldıktan SONRA dolar. Öğe kendi akış
	// çalışmasına bu alanla bağlanır.
	WorkflowRunID *uuid.UUID `json:"workflowRunId"`

	Error      string     `json:"error"`
	StartedAt  *time.Time `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt"`
}

// Pending, sıradaki öğe ve onu BAŞLATMAK için gereken toplu iş bilgisi.
//
// Ayrı tip olmasının sebebi: akış kimliği ve görev metni öğenin değil toplu
// işin alanları. `Item`'a eklenselerdi her öğe listesinde otuz kez tekrarlanan
// aynı iki değer taşınırdı.
type Pending struct {
	Item
	WorkflowID uuid.UUID
	Task       string
}
