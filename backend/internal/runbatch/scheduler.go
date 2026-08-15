package runbatch

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Starter, bir öğenin akış çalışmasını başlatan şey.
//
// `workflow.Launcher` yerine dar bir arayüz: zamanlayıcının sıra ve sınır
// mantığı en çok hata barındıran kısım ve container'a hiç dokunmadan test
// edilebilmeli.
type Starter interface {
	Start(ctx context.Context, workflowID, projectID uuid.UUID, task string) (uuid.UUID, error)
}

// Outcome, bir akış çalışmasının o anki sonucu.
type Outcome struct {
	// Finished false ise çalışma sürüyor; diğer alanlar anlamsızdır.
	Finished bool

	// Status, öğeye yazılacak durum: succeeded · failed · cancelled.
	Status string
	Error  string

	// LimitHit, çalışmanın eşzamanlılık sınırı yüzünden düştüğünü söyler.
	// Bu bir BAŞARISIZLIK DEĞİL, zamanlama sonucudur: öğe kuyruğa geri döner.
	LimitHit bool
}

// Tracker, başlatılmış bir akış çalışmasının sonucunu bildirir.
type Tracker interface {
	Outcome(ctx context.Context, runID uuid.UUID) (Outcome, error)
}

/*
Slots, eşzamanlılık sınırını SORAR, kopyalamaz.

Kuyruk kendi paralellik sayısını tutmuyor: ikinci bir sayı, ayar değiştiğinde
geride kalırdı. Fonksiyon olarak taşınırlar çünkü ayar değişikliği yeniden
başlatma gerektirmiyor.
*/
type Slots struct {
	// Max, aynı anda çalışabilecek iş sayısı (ayardan).
	Max func() int
	// Active, o an çalışan iş sayısı (çalıştırma yöneticisinden).
	Active func() int
}

// Scheduler, kuyruğu işletir: boş slot oldukça sıradakini başlatır.
type Scheduler struct {
	store   *Store
	starter Starter
	tracker Tracker
	slots   Slots

	// wake, tamponu 1 olan uyandırma kanalı. "Bak" demek bir kez yeter:
	// zamanlayıcı o an meşgulse sinyal kaybolmaz, birikmez de.
	wake chan struct{}

	/*
		interval, emniyet turunun aralığı — AYARDAN, koddan değil.

		Fonksiyon olarak taşınıyor çünkü her turda yeniden okunuyor: kullanıcı
		değeri değiştirince sunucuyu yeniden başlatmak gerekmesin (projedeki
		diğer çalışma parametrelerinin aynısı).
	*/
	interval func() time.Duration
}

// NewScheduler yeni zamanlayıcı üretir.
//
// interval, emniyet turu aralığını her çağrıda yeniden okur (ayardan).
func NewScheduler(store *Store, starter Starter, tracker Tracker, slots Slots,
	interval func() time.Duration,
) *Scheduler {
	return &Scheduler{
		store: store, starter: starter, tracker: tracker, slots: slots,
		wake:     make(chan struct{}, 1),
		interval: interval,
	}
}

/*
tur, bir sonraki emniyet turuna kalan süre.

Sıfır ya da eksi bir değer buraya GELMEMELİ — ayar katmanı bozuk değeri zaten
varsayılana düşürüyor. Yine de korunuyor: sıfırlık bir bekleme, kuyruğu boşuna
dönen sıcak bir döngüye çevirirdi.
*/
func (s *Scheduler) tur() time.Duration {
	if s.interval == nil {
		return time.Minute
	}
	d := s.interval()
	if d <= 0 {
		return time.Minute
	}
	return d
}

/*
Wake, zamanlayıcıyı uyandırır. BLOKLAMAZ.

Çağrıldığı yerler: slot boşaldığında (`runs.Limits.OnSlotFree`), toplu iş
oluşturulduğunda ve "kaldığı yerden devam" denildiğinde.

Bloklamayan gönderim bilinçli: bu kanal `runs.Manager.release()` içinden de
besleniyor ve orada beklemek, biten bir çalıştırmanın temizliğini kuyruğun
hızına bağlardı.
*/
func (s *Scheduler) Wake() {
	select {
	case s.wake <- struct{}{}:
	default: // sinyal zaten bekliyor; ikincisi bilgi taşımıyor
	}
}

// Run, zamanlayıcıyı çalıştırır. Kapatma sinyaline kadar döner.
func (s *Scheduler) Run(ctx context.Context) {
	slog.InfoContext(ctx, "toplu çalıştırma kuyruğu başladı", "emniyet_turu", s.tur())

	// İlk tur hemen: açılışta bekleyen bir kuyruk varsa ilk uyandırmayı
	// beklemesin — o uyandırma hiç gelmeyebilir.
	s.Tick(ctx)

	for {
		// Ticker DEĞİL timer: aralık ayardan geliyor ve değişebiliyor. Ticker
		// kurulduğu andaki değere sabitlenirdi, yani ayar değişikliği ancak
		// yeniden başlatmayla geçerli olurdu.
		timer := time.NewTimer(s.tur())

		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.wake:
			timer.Stop()
			s.Tick(ctx)
		case <-timer.C:
			s.Tick(ctx)
		}
	}
}

/*
Tick, bir tur işletir: önce biten öğeleri toplar, sonra boş slot kadar başlatır.

SIRA ÖNEMLİ: toplama önce yapılır, çünkü biten bir öğe yer açar. Ters sırada
kuyruk her turda bir tur geriden gelirdi.

Dışa açık olması testler için: turun kendisi ölçülebilir olmalı.
*/
func (s *Scheduler) Tick(ctx context.Context) {
	running, err := s.harvest(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "toplu iş sonuçları toplanamadı", "error", err)
		return
	}
	s.launch(ctx, running)
}

// harvest, çalışan öğelerin sonucunu toplar ve GERİYE KALAN çalışan sayısını
// döner.
func (s *Scheduler) harvest(ctx context.Context) (int, error) {
	items, err := s.store.RunningItems(ctx)
	if err != nil {
		return 0, err
	}

	running := 0
	for _, it := range items {
		// Çalışma kimliği henüz yazılmamış: öğe sahiplenildi, başlatma sürüyor.
		// Bu öğe slot tutuyor sayılır — yoksa aynı anda ikinci bir öğe başlardı.
		if it.WorkflowRunID == nil {
			running++
			continue
		}

		out, err := s.tracker.Outcome(ctx, *it.WorkflowRunID)
		if err != nil {
			slog.ErrorContext(ctx, "öğenin çalışma sonucu okunamadı",
				"item_id", it.ID, "workflow_run_id", *it.WorkflowRunID, "error", err)
			running++
			continue
		}
		if !out.Finished {
			running++
			continue
		}

		// SINIR HATASI BAŞARISIZLIK DEĞİLDİR: iş hiç çalışmadı, zamanı değildi.
		// Öğe kuyruğa döner ve sırasını bekler.
		if out.LimitHit {
			if err := s.store.Requeue(ctx, it.ID); err != nil {
				slog.ErrorContext(ctx, "öğe kuyruğa geri konamadı",
					"item_id", it.ID, "error", err)
			} else {
				slog.InfoContext(ctx, "öğe sınır dolu olduğu için kuyruğa geri kondu",
					"item_id", it.ID, "proje", it.ProjectName)
			}
			continue
		}

		if err := s.store.MarkFinished(ctx, it.ID, out.Status, out.Error); err != nil {
			slog.ErrorContext(ctx, "öğe sonucu yazılamadı", "item_id", it.ID, "error", err)
			running++
			continue
		}
		slog.InfoContext(ctx, "toplu iş öğesi bitti",
			"item_id", it.ID, "proje", it.ProjectName, "durum", out.Status)
	}
	return running, nil
}

/*
launch, boş kapasite kadar bekleyen öğe başlatır.

İKİ KOŞUL birlikte aranır:

  - çalışan ÖĞE sayısı sınırın altında olmalı — kuyruğun kendi ölçüsü
  - çalıştırma yöneticisinde gerçekten boş slot olmalı — makinenin ölçüsü

İkincisi olmasa, slotları başka çalıştırmalar tutarken başlatılan öğe adım
seviyesinde anında düşer ve kuyruk kendini yerdi. Birincisi olmasa, adımlar
arasında slot bırakan akışlar yüzünden sınırdan fazla öğe başlar ve hepsi aynı
anda adıma girdiğinde çoğu düşerdi.
*/
func (s *Scheduler) launch(ctx context.Context, running int) {
	for {
		limit := s.slots.Max()
		if running >= limit || s.slots.Active() >= limit {
			return
		}

		p, ok, err := s.store.NextPending(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "sıradaki öğe okunamadı", "error", err)
			return
		}
		if !ok {
			return
		}

		if !s.startItem(ctx, p) {
			return
		}
		running++
	}
}

// startItem, tek bir öğeyi başlatır. Kuyruğun devam edip etmeyeceğini döner.
func (s *Scheduler) startItem(ctx context.Context, p Pending) bool {
	// Sahiplenme başlatmadan ÖNCE: aradaki bir çökme öğeyi ikinci kez
	// başlatılabilir bırakmamalı.
	if err := s.store.Claim(ctx, p.ID); err != nil {
		// Başka bir tur aynı öğeyi kapmış olabilir — hata değil, tekrar bak.
		slog.DebugContext(ctx, "öğe sahiplenilemedi", "item_id", p.ID, "error", err)
		return false
	}

	runID, err := s.starter.Start(ctx, p.WorkflowID, p.ProjectID, p.Task)
	if err != nil {
		// Başlatma hiç olmadı. Sınır hatasıysa öğe kuyruğa döner; değilse
		// yapılandırma eksiği demektir ve öğe sebebiyle düşer — kuyruk DEVAM
		// eder (spec 023 H4).
		if IsLimitError(err) {
			if rerr := s.store.Requeue(ctx, p.ID); rerr != nil {
				slog.ErrorContext(ctx, "öğe kuyruğa geri konamadı",
					"item_id", p.ID, "error", rerr)
			}
			return false
		}
		if merr := s.store.MarkFinished(ctx, p.ID, ItemFailed, err.Error()); merr != nil {
			slog.ErrorContext(ctx, "öğe başarısız işaretlenemedi",
				"item_id", p.ID, "error", merr)
		}
		slog.WarnContext(ctx, "toplu iş öğesi başlatılamadı",
			"item_id", p.ID, "proje", p.ProjectName, "error", err)
		// Kuyruk durmaz: bir projedeki eksik ayar, sonrakini denememek için
		// sebep değil.
		return true
	}

	if err := s.store.Attach(ctx, p.ID, runID); err != nil {
		slog.ErrorContext(ctx, "öğe çalışmaya bağlanamadı",
			"item_id", p.ID, "workflow_run_id", runID, "error", err)
	}
	slog.InfoContext(ctx, "toplu iş öğesi başlatıldı",
		"item_id", p.ID, "proje", p.ProjectName, "workflow_run_id", runID)
	return true
}

// Reconcile, açılışta yarım kalmış öğeleri uzlaştırır.
//
// KAÇ ÖĞEYİ DÜŞÜRDÜĞÜNÜ LOGLAR: bu adım sessizce çalışmazsa öğeler sonsuza
// kadar `running` görünür ve kimse fark etmez.
func (s *Scheduler) Reconcile(ctx context.Context) error {
	n, err := s.store.InterruptRunning(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		slog.InfoContext(ctx, "yarım kalmış toplu iş öğeleri kesildi olarak işaretlendi",
			"adet", n)
	}
	return nil
}
