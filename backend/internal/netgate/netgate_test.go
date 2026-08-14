package netgate

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/agent-coder/backend/internal/hostlist"
	"github.com/stretchr/testify/require"
)

/*
 * Kapının testleri GERÇEK SOKET kullanır, mock değil.
 *
 * Sebebi: bu paketin işi protokol konuşmak. Bir mock'a "CONNECT geldi" dedirtmek,
 * gerçek bir istemcinin gönderdiği baytların doğru ayrıştırıldığını GÖSTERMEZ —
 * ve tam olarak orada hata yapılır. Testler rastgele portta gerçek dinleyici
 * açar, ham CONNECT satırı yazar ve dönen durum satırını okur.
 */

// kapiAc, test için kapı açar ve kapanışını t'ye bağlar.
func kapiAc(t *testing.T) *Gate {
	t.Helper()
	g, err := New("127.0.0.1:0")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = g.Serve(ctx) }()

	return g
}

// connectRaw, kapıya ham CONNECT yollar; bağlantıyı ve durum kodunu döner.
// Bağlantı açık bırakılır ki tünelden veri akışı da sınanabilsin.
func connectRaw(t *testing.T, g *Gate, hedef string) (net.Conn, *bufio.Reader, int) {
	t.Helper()
	c, err := net.Dial("tcp", g.Addr())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })

	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = c.Write([]byte("CONNECT " + hedef + " HTTP/1.1\r\nHost: " + hedef + "\r\n\r\n"))
	require.NoError(t, err)

	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	require.NoError(t, err)
	return c, br, resp.StatusCode
}

func connect(t *testing.T, g *Gate, hedef string) int {
	t.Helper()
	_, _, kod := connectRaw(t, g, hedef)
	return kod
}

/*
 * sahteUpstream, CONNECT kabul edip tünelde yankı yapan bir proxy.
 *
 * GERÇEK bir upstream yerine bunun kullanılması bilinçli: testin internete
 * çıkması, testi ağa ve üçüncü tarafa bağımlı kılardı. Ama sahte olan yalnızca
 * karşı taraf — kapının konuştuğu protokol gerçek.
 */
func sahteUpstream(t *testing.T) (adres string, gorulen *[]string) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	var mu sync.Mutex
	hedefler := []string{}

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				req, err := http.ReadRequest(br)
				if err != nil {
					return
				}
				mu.Lock()
				hedefler = append(hedefler, req.Host)
				mu.Unlock()

				// Düz HTTP: proxy'ye mutlak URL'li istek gelir, yanıt döner.
				if req.Method != http.MethodConnect {
					_, _ = c.Write([]byte(
						"HTTP/1.1 200 OK\r\nContent-Length: 5\r\nConnection: close\r\n\r\nselam"))
					return
				}

				_, _ = c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
				_, _ = io.Copy(c, br) // tünelde yankı
			}(c)
		}
	}()

	return l.Addr().String(), &hedefler
}

func TestGate_DinleyiciAcilirVeAdresVerir(t *testing.T) {
	g := kapiAc(t)

	require.NotEmpty(t, g.Addr())
	c, err := net.Dial("tcp", g.Addr())
	require.NoError(t, err)
	_ = c.Close()
}

/*
 * Kayıtsız kaynaktan gelen istek REDDEDİLİR.
 *
 * Bu, "ayar boşken kapı fiilen kapalıdır" kuralının testi. Kapı backend ile
 * birlikte hep açık duruyor; onu kapalı tutan şey, hiçbir çalıştırmanın kayıtlı
 * olmaması. Varsayılan "geçir" olsaydı, denetim kapalıyken kapıyı bulan herkes
 * onu açık bir proxy olarak kullanabilirdi.
 */
func TestGate_KayitsizKaynakReddedilir(t *testing.T) {
	g := kapiAc(t)

	require.Equal(t, http.StatusForbidden, connect(t, g, "ornek.com:443"))
}

// İzinli host upstream'e devredilir VE tünelden veri akar. Yalnızca "200 döndü"
// demek yetmez — 200 dönüp tüneli bağlamayan bir kapı da o testi geçerdi.
func TestGate_IzinliHostUpstreameDevredilirVeVeriAkar(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	g := kapiAc(t)
	desenler, err := hostlist.Parse("ornek.com")
	require.NoError(t, err)
	g.Register("127.0.0.1", Run{ID: "k1", Upstream: upstream, Allow: desenler})

	c, br, kod := connectRaw(t, g, "ornek.com:443")
	require.Equal(t, http.StatusOK, kod)

	_, err = c.Write([]byte("merhaba\n"))
	require.NoError(t, err)
	yankı, err := br.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "merhaba\n", yankı)

	require.Equal(t, []string{"ornek.com:443"}, *gorulen)
}

// İzinsiz host reddedilir, upstream'e HİÇ ulaşılmaz ve OnDeny bir kez çağrılır.
func TestGate_IzinsizHostReddedilir(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	g := kapiAc(t)
	desenler, err := hostlist.Parse("ornek.com")
	require.NoError(t, err)

	var reddedilen []string
	g.Register("127.0.0.1", Run{
		ID: "k1", Upstream: upstream, Allow: desenler,
		OnDeny: func(host string) { reddedilen = append(reddedilen, host) },
	})

	require.Equal(t, http.StatusForbidden, connect(t, g, "yasak.com:443"))
	require.Equal(t, []string{"yasak.com"}, reddedilen)
	require.Empty(t, *gorulen, "reddedilen istek upstream'e ulaşmamalı")
}

// Boş Allow her host'u geçirir — spec 020: boş whitelist kısıtsızlıktır.
func TestGate_BosWhitelistHerHostuGecirir(t *testing.T) {
	upstream, _ := sahteUpstream(t)
	g := kapiAc(t)
	g.Register("127.0.0.1", Run{ID: "k1", Upstream: upstream})

	require.Equal(t, http.StatusOK, connect(t, g, "herhangi.com:443"))
}

// Unregister sonrası aynı kaynak yeniden reddedilir. Container silinince IP
// yeniden kullanılabiliyor; kayıt kapanmazsa sonraki container önceki
// çalıştırmanın izinleriyle çıkardı.
func TestGate_UnregisterSonrasiReddedilir(t *testing.T) {
	upstream, _ := sahteUpstream(t)
	g := kapiAc(t)
	g.Register("127.0.0.1", Run{ID: "k1", Upstream: upstream})
	require.Equal(t, http.StatusOK, connect(t, g, "herhangi.com:443"))

	g.Unregister("127.0.0.1")
	require.Equal(t, http.StatusForbidden, connect(t, g, "herhangi.com:443"))
}

/*
 * Düz HTTP de aynı kararı alır.
 *
 * Gerekli, çünkü kurumsal Nexus ve dahili registry'ler sıklıkla TLS'siz
 * (`http://nexus.sirket.local:8081`) sunuluyor. Yalnızca CONNECT desteklenseydi
 * bu adresler kapıdan hiç geçemez ve "whitelist'e yazdım ama çalışmıyor"
 * durumunu üretirdi.
 */
func TestGate_DuzHTTPAyniKarariAlir(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	g := kapiAc(t)
	desenler, err := hostlist.Parse("izinli.com")
	require.NoError(t, err)
	g.Register("127.0.0.1", Run{ID: "k1", Upstream: upstream, Allow: desenler})

	istemci := &http.Client{Transport: &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { return url.Parse("http://" + g.Addr()) },
	}}
	resp, err := istemci.Get("http://izinli.com/paket")
	require.NoError(t, err)
	defer resp.Body.Close()

	govde, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "selam", string(govde))
	require.Equal(t, []string{"izinli.com"}, *gorulen,
		"düz HTTP de kurumsal proxy üzerinden gitmeli, hedefe doğrudan değil")
}

func TestGate_DuzHTTPIzinsizHostReddedilir(t *testing.T) {
	g := kapiAc(t)
	desenler, err := hostlist.Parse("izinli.com")
	require.NoError(t, err)
	g.Register("127.0.0.1", Run{ID: "k1", Upstream: "127.0.0.1:1", Allow: desenler})

	istemci := &http.Client{Transport: &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { return url.Parse("http://" + g.Addr()) },
	}}
	resp, err := istemci.Get("http://yasak.com/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// Kayıt/silme eşzamanlı çağrılarda güvenli olmalı: her çalıştırma kendi
// goroutine'inde başlıyor ve kapı aynı anda istek karşılıyor.
func TestGate_EszamanliKayitGuvenli(t *testing.T) {
	g := kapiAc(t)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) { defer wg.Done(); g.Register("10.0.0."+strconv.Itoa(i), Run{ID: "k"}) }(i)
		go func(i int) { defer wg.Done(); g.Unregister("10.0.0." + strconv.Itoa(i)) }(i)
	}
	wg.Wait()
}

/*
 * Gövde TAMPONLANMAZ, akıtılır — ÖLÇÜLEREK doğrulanır.
 *
 * Koşu B'de agent 190 MB'lık bir JDK indirmişti. Kapı gövdeyi belleğe alsaydı
 * her büyük indirme backend'in belleğine yazılırdı. `io.Copy` yapı gereği 32 KB
 * tamponla akıtır; bu test o iddiayı sayıyla sınıyor.
 */
func TestGate_BuyukGovdeTamponlanmaz(t *testing.T) {
	upstream, _ := sahteUpstream(t)
	g := kapiAc(t)
	g.Register("127.0.0.1", Run{ID: "k1", Upstream: upstream})

	c, br, kod := connectRaw(t, g, "ornek.com:443")
	require.Equal(t, http.StatusOK, kod)

	const boyut = 8 << 20 // 8 MB
	veri := bytes.Repeat([]byte("x"), boyut)

	runtime.GC()
	var once runtime.MemStats
	runtime.ReadMemStats(&once)

	go func() { _, _ = c.Write(veri) }()

	okunan, err := io.ReadFull(br, make([]byte, boyut))
	require.NoError(t, err)
	require.Equal(t, boyut, okunan)

	runtime.GC()
	var sonra runtime.MemStats
	runtime.ReadMemStats(&sonra)

	artis := int64(sonra.HeapAlloc) - int64(once.HeapAlloc)
	require.Less(t, artis, int64(4<<20),
		"8 MB akarken heap %d bayt arttı — gövde tamponlanıyor olabilir", artis)
}

// Upstream ölüyken anlaşılır hata dönmeli — "bilinmeyen hata" değil.
func TestGate_UpstreamUlasilamazkenAnlasilirHata(t *testing.T) {
	g := kapiAc(t)
	// Kapalı bir port: bağlantı reddedilecek.
	g.Register("127.0.0.1", Run{ID: "k1", Upstream: "127.0.0.1:1"})

	require.Equal(t, http.StatusBadGateway, connect(t, g, "ornek.com:443"))
}
