package runbatch_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runbatch"
	"github.com/agent-coder/backend/internal/runs"
)

/*
 * Zamanlayıcı — spec 023'ün en riskli parçası.
 *
 * Burada ölçülen iki şey var ve ikisi de sessiz arızalar:
 *
 *   1. Sınırın AŞILMAMASI. Aşılırsa makine otuz eşzamanlı işe boğulur.
 *   2. Sınır hatası alan öğenin BAŞARISIZ SAYILMAMASI. Sayılırsa kuyruk kendini
 *      yer: sınırın dolu olduğu her an bir öğe düşer ve otuz projenin çoğu hiç
 *      çalışmadan "başarısız" görünür.
 *
 * Başlatıcı sahte: motorun ve container'ın davranışı burada ölçülmüyor, sıra ve
 * sınır mantığı ölçülüyor.
 */

// sahteBaslatici, akış başlatmayı taklit eder ve çalışmaların ne zaman
// biteceğini teste bırakır.
type sahteBaslatici struct {
	mu sync.Mutex

	// yeni, gerçek bir akış çalışması kaydı üretir. Sahte bir UUID yeterli
	// olmazdı: `workflow_run_id` yabancı anahtar ve öğeyi bağlamak gerçekten
	// var olan bir çalışma ister.
	yeni func(context.Context, uuid.UUID) (uuid.UUID, error)

	// sonuc, çalışma kimliğine göre o an geçerli sonuç. Yoksa "sürüyor".
	sonuc map[uuid.UUID]runbatch.Outcome
	// baslatilan, başlatma sırası (proje kimlikleri).
	baslatilan []uuid.UUID
	// hata, bir sonraki Start çağrısının döneceği hata.
	hata error

	// tepe, aynı anda kaç çalışmanın sürdüğünün en yüksek değeri.
	acik, tepe int

	// disAktif, kuyruğun DIŞINDA slot tutan çalıştırmalar (kullanıcının elle
	// başlattığı bir iş gibi). Kuyruk makinenin tamamını kendine ait saymamalı.
	disAktif int
}

func (f *sahteBaslatici) Start(ctx context.Context, _, projectID uuid.UUID, _ string) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.hata != nil {
		err := f.hata
		f.hata = nil
		return uuid.Nil, err
	}

	id, err := f.yeni(ctx, projectID)
	if err != nil {
		return uuid.Nil, err
	}
	f.baslatilan = append(f.baslatilan, projectID)
	f.acik++
	if f.acik > f.tepe {
		f.tepe = f.acik
	}
	return id, nil
}

func (f *sahteBaslatici) Outcome(_ context.Context, runID uuid.UUID) (runbatch.Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sonuc[runID], nil
}

// bitir, sıradaki (en eski) açık çalışmayı verilen sonuçla kapatır.
func (f *sahteBaslatici) bitir(t *testing.T, s *runbatch.Store, out runbatch.Outcome) {
	t.Helper()

	items, err := s.RunningItems(context.Background())
	require.NoError(t, err)
	for _, it := range items {
		if it.WorkflowRunID == nil {
			continue
		}
		f.mu.Lock()
		_, zatenBitti := f.sonuc[*it.WorkflowRunID]
		if !zatenBitti {
			f.sonuc[*it.WorkflowRunID] = out
			f.acik--
			f.mu.Unlock()
			return
		}
		f.mu.Unlock()
	}
	t.Fatal("bitirilecek açık çalışma yok")
}

func (f *sahteBaslatici) tepeDeger() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tepe
}

// sabitSinir, ayarları taklit eder: sınır sabit, çalışan iş sayısı sahte
// başlatıcının açık çalışma sayısı.
func sabitSinir(f *sahteBaslatici, n int) runbatch.Slots {
	return runbatch.Slots{
		Max: func() int { return n },
		Active: func() int {
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.acik + f.disAktif
		},
	}
}

func (f fixture) scheduler(t *testing.T, sinir int) (*runbatch.Scheduler, *sahteBaslatici) {
	t.Helper()
	b := &sahteBaslatici{
		sonuc: map[uuid.UUID]runbatch.Outcome{},
		yeni:  f.yeniCalisma,
	}
	return runbatch.NewScheduler(f.store, b, b, sabitSinir(b, sinir)), b
}

// T10 — boş slot varken sıradaki başlar.
func TestZamanlayici_BosSlottaSiradakiBaslar(t *testing.T) {
	f := setup(t, "alfa", "beta")
	ctx := context.Background()
	f.create(t, f.projects[0], f.projects[1])

	s, b := f.scheduler(t, 2)
	s.Tick(ctx)

	require.Equal(t, []uuid.UUID{f.projects[0], f.projects[1]}, b.baslatilan,
		"öğeler eklenme sırasında başlamalı")

	_, items, err := f.store.Get(ctx, f.batchID(t))
	require.NoError(t, err)
	for _, it := range items {
		require.Equal(t, runbatch.ItemRunning, it.Status)
		require.NotNil(t, it.WorkflowRunID, "başlayan öğe çalışmasına bağlanmalı")
	}
}

/*
T11 — AYNI ANDA ÇALIŞAN SAYISI SINIRI AŞMAZ.

Sınır 2, beş öğe. Turlar art arda işletilirken tepe eşzamanlılık ölçülüyor;
2'yi geçtiği an test kırmızıya döner.
*/
func TestZamanlayici_SinirAsilmaz(t *testing.T) {
	f := setup(t, "a", "b", "c", "d", "e")
	ctx := context.Background()
	f.create(t, f.projects...)

	s, b := f.scheduler(t, 2)

	// Beş öğe, ikişer ikişer: her turda bir iş bitiriliyor ve yerine bir yenisi
	// başlıyor. Fazladan turlar bilinçli — boşta dönen tur da sınırı aşmamalı.
	s.Tick(ctx)
	for i := 0; i < 5; i++ {
		b.bitir(t, f.store, runbatch.Outcome{Finished: true, Status: runbatch.ItemSucceeded})
		s.Tick(ctx)
		s.Tick(ctx)
	}

	require.Equal(t, 2, b.tepeDeger(), "tepe eşzamanlılık sınırı aşmamalı")
	require.Len(t, b.baslatilan, 5, "beş öğenin hepsi çalışmalı")

	batch, _, err := f.store.Get(ctx, f.batchID(t))
	require.NoError(t, err)
	require.Equal(t, 5, batch.Counts.Succeeded)
}

/*
T12 — SINIR HATASI ALAN ÖĞE `pending` KALIR.

Kuyruğun kendini yemediğinin kanıtı. İki yoldan da gelir: başlatma anında dönen
hata ve çalışmanın sınır hatasıyla düşmesi.
*/
func TestZamanlayici_SinirHatasiOgeyiDusurmez(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	s, sahte := f.scheduler(t, 2)
	sahte.hata = fmt.Errorf("%w: şu an 2 iş çalışıyor", runs.ErrTooManyRuns)

	s.Tick(ctx)

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.ItemPending, items[0].Status,
		"sınır dolu diye düşen öğe başarısız sayılmamalı")
	require.Empty(t, items[0].Error)

	// Sınır açılınca aynı öğe çalışır.
	s.Tick(ctx)
	_, items, err = f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.ItemRunning, items[0].Status)
}

// Aynı tuzak ikinci yoldan: çalışma başladı ama adım seviyesinde sınıra takılıp
// düştü. Bu hata veritabanından METİN olarak geliyor.
func TestZamanlayici_SinirHatasiylaDusenCalismaKuyrugaDoner(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	s, sahte := f.scheduler(t, 2)
	s.Tick(ctx)

	hataMetni := "adım çalıştırılamadı: " + runs.ErrTooManyRuns.Error() + ": şu an 2 iş çalışıyor"
	require.True(t, runbatch.IsLimitError(errors.New(hataMetni)),
		"veritabanından METİN olarak gelen sınır hatası da tanınmalı")

	sahte.bitir(t, f.store, runbatch.Outcome{
		Finished: true, Status: runbatch.ItemFailed, Error: hataMetni, LimitHit: true,
	})

	// Slotları kuyruğun DIŞINDAKİ çalıştırmalar tutuyor: öğe kuyruğa dönüyor ve
	// orada bekliyor. Bu koşul olmasaydı öğe aynı turda yeniden başlar ve
	// "pending kaldı mı" sorusu ölçülemezdi.
	sahte.disAktif = 2
	s.Tick(ctx)

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.ItemPending, items[0].Status,
		"sınır hatasıyla düşen çalışma öğeyi başarısız yapmamalı")
	require.Nil(t, items[0].WorkflowRunID, "hiç çalışmamış kayıt öğeye bağlı kalmamalı")
	require.Empty(t, items[0].Error)

	// Slot açılınca aynı öğe yeniden denenir — kuyruk kendini yemedi.
	sahte.disAktif = 0
	s.Tick(ctx)

	_, items, err = f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.ItemRunning, items[0].Status)
}

// Kuyruk makinenin tamamını kendine ait saymaz: slotları başka çalıştırmalar
// tutuyorsa hiç öğe başlatmaz.
//
// Bu koşul olmasa başlatılan öğe adım seviyesinde anında düşerdi — kuyruk boşa
// dönerek kendini yerdi.
func TestZamanlayici_SlotlarDolukenBaslatmaz(t *testing.T) {
	f := setup(t, "alfa")
	ctx := context.Background()
	b := f.create(t, f.projects[0])

	s, sahte := f.scheduler(t, 2)
	sahte.disAktif = 2
	s.Tick(ctx)

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.ItemPending, items[0].Status)
	require.Empty(t, sahte.baslatilan)
}

// T13 — bir öğe düşünce kuyruk DEVAM eder.
func TestZamanlayici_DusenOgeKuyruguDurdurmaz(t *testing.T) {
	f := setup(t, "alfa", "beta")
	ctx := context.Background()
	b := f.create(t, f.projects[0], f.projects[1])

	s, sahte := f.scheduler(t, 1)
	s.Tick(ctx)
	require.Len(t, sahte.baslatilan, 1)

	sahte.bitir(t, f.store, runbatch.Outcome{
		Finished: true, Status: runbatch.ItemFailed, Error: "derleme hatası"})
	s.Tick(ctx)

	require.Len(t, sahte.baslatilan, 2, "düşen öğeden sonra sıradaki başlamalı")

	batch, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Counts.Failed)
	require.Equal(t, "derleme hatası", items[0].Error, "sebep kaydedilmeli")
}

// Yapılandırma eksiği yüzünden hiç başlayamayan öğe sebebiyle düşer — ve kuyruk
// yine devam eder (spec 023 hata tablosu).
func TestZamanlayici_BaslatilamayanOgeSebebiyleDuser(t *testing.T) {
	f := setup(t, "alfa", "beta")
	ctx := context.Background()
	b := f.create(t, f.projects[0], f.projects[1])

	s, sahte := f.scheduler(t, 2)
	sahte.hata = errors.New("akışın etkin sürümü yok")
	s.Tick(ctx)

	_, items, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.ItemFailed, items[0].Status)
	require.Equal(t, "akışın etkin sürümü yok", items[0].Error)
	require.Equal(t, runbatch.ItemRunning, items[1].Status, "kuyruk devam etmeli")
}

// T17 — toplu iş bitince durumu `done` olur.
func TestZamanlayici_BitenTopluIsDoneOlur(t *testing.T) {
	f := setup(t, "alfa", "beta")
	ctx := context.Background()
	b := f.create(t, f.projects[0], f.projects[1])

	s, sahte := f.scheduler(t, 2)
	s.Tick(ctx)
	sahte.bitir(t, f.store, runbatch.Outcome{Finished: true, Status: runbatch.ItemSucceeded})
	sahte.bitir(t, f.store, runbatch.Outcome{Finished: true, Status: runbatch.ItemFailed,
		Error: "derleme hatası"})
	s.Tick(ctx)

	batch, _, err := f.store.Get(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, runbatch.StatusDone, batch.Status,
		"bekleyeni ve çalışanı kalmayan toplu iş bitmiştir")
	require.Equal(t, 1, batch.Counts.Succeeded)
	require.Equal(t, 1, batch.Counts.Failed)
}

/*
Kuyruk AYARDAKİ SINIRA UYAR — sınır değişince yeniden başlatma gerekmez.

Sınır kuyrukta kopyalanmıyor, her turda soruluyor. Kopyalansaydı ayar
değiştiğinde geride kalır ve kullanıcı sınırı düşürdüğü hâlde makine eski
sayıda iş koşturmaya devam ederdi.
*/
func TestZamanlayici_SinirDegisince_YeniSiniraUyar(t *testing.T) {
	f := setup(t, "a", "b", "c", "d")
	ctx := context.Background()
	f.create(t, f.projects...)

	b := &sahteBaslatici{sonuc: map[uuid.UUID]runbatch.Outcome{}, yeni: f.yeniCalisma}
	sinir := 1
	s := runbatch.NewScheduler(f.store, b, b, runbatch.Slots{
		Max: func() int { return sinir },
		Active: func() int {
			b.mu.Lock()
			defer b.mu.Unlock()
			return b.acik + b.disAktif
		},
	})

	s.Tick(ctx)
	require.Len(t, b.baslatilan, 1, "sınır 1 iken tek öğe çalışır")

	// Kullanıcı ayardan sınırı yükseltti — yeniden başlatma YOK.
	sinir = 3
	s.Tick(ctx)

	require.Len(t, b.baslatilan, 3, "yeni sınır sonraki turda geçerli olmalı")
	require.Equal(t, 3, b.tepeDeger())
}

/*
T14 — `Wake` BLOKLAMAZ ve sinyal KAYBOLMAZ.

Kanal `runs.Manager.release()` içinden besleniyor: orada beklemek, biten bir
çalıştırmanın temizliğini kuyruğun hızına bağlardı. Tampon 1 olduğu için art
arda çağrı birikmez — "bak" demek bir kez yeter.
*/
func TestWake_BloklamazVeSinyalKaybolmaz(t *testing.T) {
	f := setup(t, "alfa")
	s, _ := f.scheduler(t, 1)

	bitti := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			s.Wake()
		}
		close(bitti)
	}()

	select {
	case <-bitti:
	case <-time.After(2 * time.Second):
		t.Fatal("Wake bloklamamalı — hiçbir dinleyici yokken bile")
	}
}

// T16 — EMNİYET TURU kuyruğu ilerletir: uyandırma hiç gelmese de bekleyen başlar.
//
// Bu tur bir mekanizma değil sigorta; ama sigortanın çalıştığı ölçülmezse
// kaçan bir uyandırma kuyruğu sessizce dondurur.
func TestZamanlayici_EmniyetTuruKuyruguIlerletir(t *testing.T) {
	f := setup(t, "alfa")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f.create(t, f.projects[0])

	s, sahte := f.scheduler(t, 1)
	s.SetInterval(20 * time.Millisecond)

	// Kuyruk oluşturulduktan sonra HİÇ uyandırma gönderilmiyor.
	go s.Run(ctx)

	require.Eventually(t, func() bool {
		sahte.mu.Lock()
		defer sahte.mu.Unlock()
		return len(sahte.baslatilan) == 1
	}, 3*time.Second, 20*time.Millisecond,
		"uyandırma gelmese de emniyet turu bekleyeni başlatmalı")
}

// batchID, testteki tek toplu işin kimliği.
func (f fixture) batchID(t *testing.T) uuid.UUID {
	t.Helper()
	list, _, err := f.store.List(context.Background(), 25, 0)
	require.NoError(t, err)
	require.Len(t, list, 1)
	return list[0].ID
}
