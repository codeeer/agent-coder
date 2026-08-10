// Package events, çalıştırma olaylarını canlı dinleyicilere yayınlar.
//
// Olaylar ayrıca veritabanına yazılır; bus yalnızca "şu an bağlı olanlara
// anında ulaştırma" işini yapar. Sayfa yenilendiğinde geçmiş veritabanından
// okunur, bu yüzden bus'ın kalıcı olması gerekmez.
package events

import (
	"sync"

	"github.com/google/uuid"
)

// Event, bir çalıştırmadaki tek bir ilerleme bildirimi.
type Event struct {
	Seq     int    `json:"seq"`
	Level   string `json:"level"`
	Message string `json:"message"`
	TS      string `json:"ts"`

	// Terminal true ise çalıştırma bitmiştir; dinleyiciler bağlantıyı kapatır.
	Terminal bool   `json:"terminal,omitempty"`
	Status   string `json:"status,omitempty"`
}

// bufferSize, abone başına tampon.
//
// Yavaş bir dinleyici (ağı tıkalı bir tarayıcı) yayıncıyı BLOKLAMAMALI:
// tampon dolarsa o abonenin olayı düşürülür. Kayıp olay veritabanında
// durduğu için dinleyici yeniden bağlandığında geçmişten toparlar.
const bufferSize = 64

// Bus, çalıştırma başına abonelikleri yönetir.
type Bus struct {
	mu     sync.RWMutex
	topics map[uuid.UUID]map[int]chan Event
	nextID int
}

// New yeni bir bus üretir.
func New() *Bus {
	return &Bus{topics: make(map[uuid.UUID]map[int]chan Event)}
}

// Publish, bir çalıştırmanın olayını o an bağlı tüm dinleyicilere gönderir.
//
// Dinleyici yoksa hiçbir şey yapmaz — olay zaten veritabanına yazılıyor.
func (b *Bus) Publish(runID uuid.UUID, e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, ch := range b.topics[runID] {
		select {
		case ch <- e:
		default:
			// Tampon dolu: bu abone geride kalmış. Yayıncıyı bekletmek
			// çalıştırmayı yavaşlatırdı; olay düşürülür.
		}
	}
}

// Subscribe, bir çalıştırmanın olaylarını dinlemeye başlar.
//
// Dönen fonksiyon MUTLAKA çağrılmalıdır (defer ile): çağrılmazsa abonelik
// ve kanalı sonsuza kadar bellekte kalır.
func (b *Bus) Subscribe(runID uuid.UUID) (<-chan Event, func()) {
	ch := make(chan Event, bufferSize)

	b.mu.Lock()
	id := b.nextID
	b.nextID++
	if b.topics[runID] == nil {
		b.topics[runID] = make(map[int]chan Event)
	}
	b.topics[runID][id] = ch
	b.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			b.mu.Lock()
			defer b.mu.Unlock()

			if subs := b.topics[runID]; subs != nil {
				delete(subs, id)
				if len(subs) == 0 {
					delete(b.topics, runID)
				}
			}
			// Kanal yalnızca kilit altında ve aboneliği kaldırdıktan sonra
			// kapatılır; böylece Publish kapalı kanala yazamaz.
			close(ch)
		})
	}

	return ch, unsubscribe
}

// SubscriberCount, bir çalıştırmayı kaç dinleyicinin izlediği. Testler için.
func (b *Bus) SubscriberCount(runID uuid.UUID) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.topics[runID])
}
