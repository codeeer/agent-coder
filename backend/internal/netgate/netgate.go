// Package netgate, runner container'larının tek çıkış kapısıdır.
//
// NEDEN VAR: sızıntı ölçümü (docs/veri-sizintisi-analizi.md) iki şey gösterdi —
// sandbox internetteki her adrese çıkabiliyor ve ortam değişkeniyle verilen
// proxy ATLANABİLİYOR (26 bağlantının 5'i atladı). Bu yüzden denetim ayardan
// değil ağdan geliyor: runner'ın internete rotası olmayan bir network'e alınması
// ve dışarıya tek yolun bu kapı olması.
//
// KAPI TLS AÇMAZ. Yalnızca `CONNECT host:port` satırındaki adı görür ve
// baytları tünneller. Bilinçli bir sınır: ölçüm düzeneğinin aksine üretimde
// sağlayıcı anahtarını görebilen bir ara nokta oluşmuyor. Bunun bedeli, izin
// kararının yalnızca host'a dayanması — izinli bir host üzerinden sızdırma
// kapının kapatabileceği bir şey değil (spec 020, kapsam dışı).
//
// ÇALIŞTIRMA BAŞINA AYRI DİNLEYİCİ — ölçülerek seçildi. Tasarım önce "tek
// dinleyici + kaynak IP ile kimlik" idi; ama Docker container'ın IP'sini
// YALNIZCA BAŞLATMADA atıyor (ölçüldü: başlatmadan önce `invalid IP`, sonra
// `172.27.0.2`). Kayıt ancak başlatmadan sonra yapılabilirdi ve container
// başlar başlamaz depoyu klonluyor: kaydın yetişmediği her seferde klonlama
// sessizce reddedilirdi. Port ise container yaratılmadan ÖNCE biliniyor, yani
// yarış hiç oluşmuyor. Kimlik doğrulamalı proxy de bu yüzden seçilmedi: JVM'in
// CONNECT üzerinde Basic auth'u varsayılan olarak kapalı.
package netgate

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/agent-coder/backend/internal/hostlist"
)

// upstreamTimeout, kurumsal proxy'ye bağlanma süresi. Ulaşılamayan bir adreste
// çalıştırmanın dakikalarca asılı kalmaması için kısa tutuluyor.
const upstreamTimeout = 10 * time.Second

// Gate, çıkış oturumları açar.
type Gate struct {
	// host, runner container'larının kapıya ulaşmak için kullandığı ad.
	// Docker ağında bu backend servisinin takma adıdır.
	host string
}

// New, kapıyı üretir. Dinleyici AÇILMAZ — her çalıştırma kendi oturumunu açar.
func New(host string) *Gate { return &Gate{host: host} }

/*
 * Run, tek bir çalıştırmanın çıkış kuralları.
 *
 * Upstream ve Allow OTURUM AÇILIRKEN dondurulur, canlı okunmaz: ayar çalıştırma
 * sürerken değiştirilirse süren iş başladığı kurallarla bitmeli. Aksi halde
 * yarı yolda kural değişir ve çalıştırmanın neden düştüğü açıklanamaz olurdu.
 */
type Run struct {
	ID string
	// Upstream, kurumsal proxy adresi (host:port).
	Upstream string
	// Allow boşsa TÜM host'lar izinlidir — spec 020: boş whitelist kısıt
	// değil kısıtsızlıktır.
	Allow []hostlist.Pattern

	/*
	 * Direct, kurumsal proxy'ye UĞRAMADAN gidilecek hedefler (spec 026).
	 *
	 * Allow'un TERSİ semantik: boşsa hiçbir hedef doğrudan gitmez, hepsi
	 * proxy'den geçer. Bu yüzden eşleştirmesi `hostlist.Listed` ile yapılır,
	 * `Match` ile DEĞİL — `Match` boş listeye `true` döndüğü için burada
	 * kullanılsaydı liste boşken kurumsal proxy tamamen devre dışı kalırdı.
	 *
	 * İZİN VERMEZ. Bir hedefin buraya yazılmış olması Allow kontrolünü
	 * atlatmaz; sıra her zaman önce izin, sonra yönlendirmedir.
	 */
	Direct []hostlist.Pattern
	// OnDeny, reddedilen her host için çağrılır. Çalıştırmanın olay akışına
	// uyarı yazmak buradan yapılıyor.
	OnDeny func(host string)
}

// Session, tek bir çalıştırmaya ait dinleyici.
type Session struct {
	run      Run
	dinleyci net.Listener
	sunucu   *http.Server
	proxyURL string
}

/*
 * Open, çalıştırma için dinleyici açar.
 *
 * Container YARATILMADAN ÖNCE çağrılır: dönen adres ona HTTP_PROXY olarak
 * verilecek. Böylece container'ın ilk paketi bile hazır bir kapı buluyor.
 */
func (g *Gate) Open(r Run) (*Session, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, fmt.Errorf("çıkış kapısı açılamadı: %w", err)
	}

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		_ = l.Close()
		return nil, fmt.Errorf("kapı portu okunamadı: %w", err)
	}

	s := &Session{
		run:      r,
		dinleyci: l,
		proxyURL: "http://" + net.JoinHostPort(g.host, port),
	}
	s.sunucu = &http.Server{Handler: s}

	go func() { _ = s.sunucu.Serve(l) }()
	return s, nil
}

// ProxyURL, runner'a HTTP_PROXY olarak verilecek adres.
func (s *Session) ProxyURL() string { return s.proxyURL }

// Close, dinleyiciyi kapatır. HER YOLDA çağrılmalı — açık kalan bir oturum,
// çalıştırma bittikten sonra da kullanılabilir bir çıkış bırakırdı.
func (s *Session) Close() error {
	err := s.sunucu.Close()

	/*
	 * Dinleyici AYRICA kapatılır — testle yakalanmış bir yarış.
	 *
	 * `http.Server.Close` yalnızca KENDİ İZLEDİĞİ dinleyicileri kapatıyor ve
	 * bir dinleyiciyi ancak `Serve` çağrıldığında izlemeye alıyor. Serve ayrı
	 * bir goroutine'de başlıyor; oturum açılır açılmaz kapatılırsa Close,
	 * Serve'den önce çalışıp dinleyiciyi hiç görmeyebiliyor. Sonuç: çalıştırma
	 * bittiği hâlde açık kalan, hâlâ kullanılabilir bir çıkış.
	 */
	if kapatmaHatasi := s.dinleyci.Close(); kapatmaHatasi != nil && err == nil {
		if !errors.Is(kapatmaHatasi, net.ErrClosed) {
			err = kapatmaHatasi
		}
	}

	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (s *Session) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := hostAdi(r)
	if !hostlist.Match(s.run.Allow, host) {
		if s.run.OnDeny != nil {
			s.run.OnDeny(host)
		}
		http.Error(w, "bu adrese çıkış izinli değil: "+host, http.StatusForbidden)
		return
	}

	if r.Method == http.MethodConnect {
		s.tunel(w, r)
		return
	}
	s.duzHTTP(w, r)
}

/*
 * duzHTTP, TLS'siz isteği kurumsal proxy'ye iletir.
 *
 * GEREKLİ: kurumsal Nexus ve dahili registry'ler sıklıkla TLS'siz sunuluyor
 * (`http://nexus.sirket.local:8081`). Yalnızca CONNECT desteklenseydi bu
 * adresler kapıdan hiç geçemez, kullanıcı da whitelist'e yazdığı satırın neden
 * çalışmadığını anlayamazdı.
 *
 * İstek HEDEFE DEĞİL, upstream'e gönderilir — mutlak URL'li biçimde, yani
 * proxy protokolünün kendi kuralıyla. Hedefe doğrudan gitmek, kurumsal proxy'yi
 * atlamak olurdu ki bu özelliğin tam olarak engellediği şey.
 */
func (s *Session) duzHTTP(w http.ResponseWriter, r *http.Request) {
	dis := r.Clone(r.Context())
	dis.RequestURI = ""

	// Kurum içi hedefte taşıyıcı proxy'siz kurulur (spec 026); `Proxy` nil
	// olduğunda Go doğrudan hedefe bağlanır.
	dogrudan := s.dogrudanMi(hostAdi(r))

	tasiyici := &http.Transport{}
	if !dogrudan {
		tasiyici.Proxy = func(*http.Request) (*url.URL, error) {
			return &url.URL{Scheme: "http", Host: s.run.Upstream}, nil
		}
	}
	defer tasiyici.CloseIdleConnections()

	resp, err := tasiyici.RoundTrip(dis)
	if err != nil {
		if dogrudan {
			// Geri düşme yok — tünelle aynı gerekçe (spec 026 H4).
			http.Error(w, "kurum içi adrese iletilemedi: "+err.Error(), http.StatusBadGateway)
			return
		}
		http.Error(w, "kurumsal proxy'ye iletilemedi: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for ad, degerler := range resp.Header {
		for _, d := range degerler {
			w.Header().Add(ad, d)
		}
	}
	w.WriteHeader(resp.StatusCode)
	// Gövde AKITILIR, tamponlanmaz — büyük paket indirmeleri buradan geçiyor.
	_, _ = io.Copy(w, resp.Body)
}

// hostAdi, port olmadan hedef host.
//
// CONNECT'te hedef `r.Host` içinde `host:port` olarak gelir; düz HTTP'de
// mutlak URL'in host kısmıdır. Karar PORT'a bakmaz — izinli bir domain'e tüm
// portlar açıktır (spec 020: kurumsal Nexus 8081'de çalışıyor, port kısıtı
// onu kırardı).
func hostAdi(r *http.Request) string {
	h := r.Host
	if r.Method != http.MethodConnect && r.URL != nil && r.URL.Host != "" {
		h = r.URL.Host
	}
	if ad, _, err := net.SplitHostPort(h); err == nil {
		return ad
	}
	return h
}

/*
 * tunel, CONNECT'i upstream'e devreder ve baytları iki yöne akıtır.
 *
 * TLS AÇILMAZ: kapı upstream'e kendi CONNECT'ini yollar, 200 alırsa iki soketi
 * birbirine bağlar ve aradan çekilir. Gövde TAMPONLANMAZ — Koşu B'de agent
 * 190 MB'lık bir JDK indirmişti; tamponlansaydı kapı o boyutu belleğe alırdı.
 */
/*
dogrudanMi, hedefe kurumsal proxy'ye uğramadan gidilip gidilmeyeceğini söyler.

TEK KARAR NOKTASI. Tünel ve düz HTTP ayrı kod yolları ama aynı soruyu soruyor;
ayrı ayrı yazılsaydı biri güncellenip diğeri geride kalırdı ve fark yalnızca
tek bir protokolde ortaya çıkardı — yani üretimde.
*/
func (s *Session) dogrudanMi(host string) bool {
	return hostlist.Listed(s.run.Direct, host)
}

func (s *Session) tunel(w http.ResponseWriter, r *http.Request) {
	/*
	 * Kurum içi hedefe DOĞRUDAN bağlanılır (spec 026).
	 *
	 * `ustCONNECT` ATLANIR: CONNECT bir proxy'ye "şuraya bağlan" demektir,
	 * hedefin kendisine gönderilirse hedef onu HTTP isteği sanır. Doğrudan
	 * bağlantıda taşınacak baytlar zaten TLS el sıkışmasıyla başlıyor.
	 */
	hedef := s.run.Upstream
	dogrudan := s.dogrudanMi(hostAdi(r))
	if dogrudan {
		hedef = r.Host
	}

	ust, err := net.DialTimeout("tcp", hedef, upstreamTimeout)
	if err != nil {
		if dogrudan {
			/*
			 * PROXY'YE GERİ DÜŞÜLMEZ (spec 026 H4). Yönetici bu adres için
			 * "proxy'den geçme" dedi; geri düşmek tam da kaçınmak istediği
			 * yoldan kimlik bilgisi geçirmek olurdu.
			 */
			http.Error(w, "kurum içi adrese bağlanılamadı: "+err.Error(), http.StatusBadGateway)
			return
		}
		// Anlaşılır hata: "bilinmeyen hata" yerine sebebi yazılır.
		http.Error(w, "kurumsal proxy'ye bağlanılamadı: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer ust.Close()

	if !dogrudan {
		if err := ustCONNECT(ust, r.Host); err != nil {
			http.Error(w, "kurumsal proxy CONNECT'i kabul etmedi: "+err.Error(), http.StatusBadGateway)
			return
		}
	}

	istemci, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		http.Error(w, "bağlantı devralınamadı: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer istemci.Close()

	if _, err := istemci.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n")); err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _, _ = io.Copy(ust, istemci) }()
	go func() { defer wg.Done(); _, _ = io.Copy(istemci, ust) }()
	wg.Wait()
}

// ustCONNECT, upstream proxy'ye CONNECT yollar ve 2xx bekler.
func ustCONNECT(ust net.Conn, hedef string) error {
	if _, err := fmt.Fprintf(ust, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", hedef, hedef); err != nil {
		return err
	}
	resp, err := http.ReadResponse(bufio.NewReader(ust), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("durum %d", resp.StatusCode)
	}
	return nil
}
