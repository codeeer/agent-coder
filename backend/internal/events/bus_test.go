package events

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPublish_TumAbonelereUlasir(t *testing.T) {
	b := New()
	runID := uuid.New()

	ch1, un1 := b.Subscribe(runID)
	defer un1()
	ch2, un2 := b.Subscribe(runID)
	defer un2()

	b.Publish(runID, Event{Seq: 1, Message: "merhaba"})

	for _, ch := range []<-chan Event{ch1, ch2} {
		select {
		case e := <-ch:
			require.Equal(t, "merhaba", e.Message)
		case <-time.After(time.Second):
			t.Fatal("olay ulaşmadı")
		}
	}
}

func TestPublish_BaskaCalistirmayaSizmaz(t *testing.T) {
	b := New()
	a, other := uuid.New(), uuid.New()

	chA, unA := b.Subscribe(a)
	defer unA()
	chB, unB := b.Subscribe(other)
	defer unB()

	b.Publish(a, Event{Message: "a-olayı"})

	select {
	case e := <-chA:
		require.Equal(t, "a-olayı", e.Message)
	case <-time.After(time.Second):
		t.Fatal("kendi olayı ulaşmadı")
	}

	select {
	case e := <-chB:
		t.Fatalf("başka çalıştırmanın olayı sızdı: %v", e)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestPublish_AboneYokkenSorunsuz(t *testing.T) {
	b := New()
	// Panik etmemeli: çalıştırma kimse izlemezken de olay üretir.
	b.Publish(uuid.New(), Event{Message: "kimse yok"})
}

func TestUnsubscribe_KanaliKapatirVeSayimiDusurur(t *testing.T) {
	b := New()
	runID := uuid.New()

	ch, unsubscribe := b.Subscribe(runID)
	require.Equal(t, 1, b.SubscriberCount(runID))

	unsubscribe()
	require.Equal(t, 0, b.SubscriberCount(runID))

	_, open := <-ch
	require.False(t, open, "abonelik bitince kanal kapanmalı")
}

func TestUnsubscribe_IkiKezCagrilabilir(t *testing.T) {
	b := New()
	runID := uuid.New()

	_, unsubscribe := b.Subscribe(runID)
	unsubscribe()
	// SSE handler'ı hem defer hem hata yolunda çağırabilir; ikincisi
	// kapalı kanalı tekrar kapatıp panik ETMEMELİ.
	require.NotPanics(t, unsubscribe)
}

func TestPublish_KapaliAboneyeYazmaz(t *testing.T) {
	b := New()
	runID := uuid.New()

	_, unsubscribe := b.Subscribe(runID)
	unsubscribe()

	// Abonelik kaldırıldıktan sonra yayın kapalı kanala yazmayı denememelidir.
	require.NotPanics(t, func() {
		b.Publish(runID, Event{Message: "geç kalan olay"})
	})
}

func TestPublish_YavasAboneYayinciyiBloklamaz(t *testing.T) {
	b := New()
	runID := uuid.New()

	// Hiç okumayan bir abone: tamponu dolduracak.
	_, unsubscribe := b.Subscribe(runID)
	defer unsubscribe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < bufferSize*3; i++ {
			b.Publish(runID, Event{Seq: i})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("yavaş abone yayıncıyı blokladı — çalıştırma yavaşlardı")
	}
}

func TestBus_EszamanliKullanim(t *testing.T) {
	// -race ile çalıştığında kilitleme hatalarını yakalar.
	b := New()
	runID := uuid.New()

	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch, unsubscribe := b.Subscribe(runID)
			defer unsubscribe()
			for range 5 {
				select {
				case <-ch:
				case <-time.After(20 * time.Millisecond):
				}
			}
		}()
	}

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := range 20 {
				b.Publish(runID, Event{Seq: n*100 + j})
			}
		}(i)
	}

	wg.Wait()
}
