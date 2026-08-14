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
package netgate

import (
	"bufio"
	"context"
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

// Gate, çıkış kapısı.
type Gate struct {
	dinleyici net.Listener

	mu     sync.RWMutex
	kayit  map[string]Run // kaynak IP → çalıştırma
	sunucu *http.Server
}

// Run, kapıya kayıtlı tek bir çalıştırma.
//
// Upstream ve Allow KAYIT ANINDA DONDURULUR, canlı okunmaz: ayar çalıştırma
// sürerken değiştirilirse süren iş başladığı kurallarla bitmeli. Aksi halde
// yarı yolda kural değişir ve çalıştırmanın neden düştüğü açıklanamaz olurdu.
type Run struct {
	ID string
	// Upstream, kurumsal proxy adresi (host:port).
	Upstream string
	// Allow boşsa TÜM host'lar izinlidir — spec 020: boş whitelist kısıt
	// değil kısıtsızlıktır.
	Allow []hostlist.Pattern
	// OnDeny, reddedilen her host için çağrılır. Çalıştırmanın olay akışına
	// uyarı yazmak buradan yapılıyor.
	OnDeny func(host string)
}

// Register, bir çalıştırmayı kaynak IP'sine bağlar.
func (g *Gate) Register(ip string, r Run) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.kayit[ip] = r
}

// Unregister, kaydı kapatır.
//
// HER YOLDA çağrılmalı: container silinince IP havuza dönüyor ve yeniden
// kullanılabiliyor. Kayıt açık kalsaydı sonraki container, önceki çalıştırmanın
// izinleriyle dışarı çıkardı.
func (g *Gate) Unregister(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.kayit, ip)
}

// New, dinleyiciyi HEMEN açar.
//
// Bind burada yapılıyor ki hata AÇILIŞTA görünsün. Dinleyici ayara bağlı
// başlatılsaydı, ayar çalışma anında girildiği için "ayar kaydedildi ama kapı
// henüz ayakta değil" gibi bir aralık oluşurdu.
func New(listen string) (*Gate, error) {
	l, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, err
	}

	g := &Gate{dinleyici: l, kayit: map[string]Run{}}
	g.sunucu = &http.Server{Handler: g}
	return g, nil
}

// Addr, dinlenen gerçek adres. Port 0 verildiğinde işletim sisteminin seçtiği
// portu öğrenmenin tek yolu budur.
func (g *Gate) Addr() string { return g.dinleyici.Addr().String() }

// Serve, kapıyı çalıştırır ve ctx iptal edilene kadar bloklar.
func (g *Gate) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = g.sunucu.Close()
	}()

	err := g.sunucu.Serve(g.dinleyici)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (g *Gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	run, tamam := g.calistirmaBul(r.RemoteAddr)
	if !tamam {
		// Kayıtsız kaynak: denetim kapalıyken kapı fiilen kapalı olsun diye.
		// Varsayılan "geçir" olsaydı kapı açık bir proxy'ye dönerdi.
		http.Error(w, "bu kaynak için kayıtlı çalıştırma yok", http.StatusForbidden)
		return
	}

	host := hostAdi(r)
	if !hostlist.Match(run.Allow, host) {
		if run.OnDeny != nil {
			run.OnDeny(host)
		}
		http.Error(w, "bu adrese çıkış izinli değil: "+host, http.StatusForbidden)
		return
	}

	if r.Method == http.MethodConnect {
		g.tunel(w, r, run)
		return
	}
	g.duzHTTP(w, r, run)
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
func (g *Gate) duzHTTP(w http.ResponseWriter, r *http.Request, run Run) {
	dis := r.Clone(r.Context())
	dis.RequestURI = ""

	tasiyici := &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) {
			return &url.URL{Scheme: "http", Host: run.Upstream}, nil
		},
	}
	defer tasiyici.CloseIdleConnections()

	resp, err := tasiyici.RoundTrip(dis)
	if err != nil {
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
func (g *Gate) tunel(w http.ResponseWriter, r *http.Request, run Run) {
	ust, err := net.DialTimeout("tcp", run.Upstream, upstreamTimeout)
	if err != nil {
		// Anlaşılır hata: "bilinmeyen hata" yerine sebebi yazılır.
		http.Error(w, "kurumsal proxy'ye bağlanılamadı: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer ust.Close()

	if err := ustCONNECT(ust, r.Host); err != nil {
		http.Error(w, "kurumsal proxy CONNECT'i kabul etmedi: "+err.Error(), http.StatusBadGateway)
		return
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

func (g *Gate) calistirmaBul(remoteAddr string) (Run, bool) {
	ip, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		ip = remoteAddr
	}

	g.mu.RLock()
	defer g.mu.RUnlock()
	r, tamam := g.kayit[ip]
	return r, tamam
}
