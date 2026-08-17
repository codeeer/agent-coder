package netgate

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"sync/atomic"
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

// oturumAc, test için bir çalıştırma oturumu açar ve kapanışını t'ye bağlar.
func oturumAc(t *testing.T, r Run) *Session {
	t.Helper()
	s, err := New("127.0.0.1").Open(r)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// adres, oturumun dinlediği host:port.
func adres(t *testing.T, s *Session) string {
	t.Helper()
	u, err := url.Parse(s.ProxyURL())
	require.NoError(t, err)
	return u.Host
}

// connectRaw, kapıya ham CONNECT yollar; bağlantıyı ve durum kodunu döner.
// Bağlantı açık bırakılır ki tünelden veri akışı da sınanabilsin.
func connectRaw(t *testing.T, s *Session, hedef string) (net.Conn, *bufio.Reader, int) {
	t.Helper()
	c, err := net.Dial("tcp", adres(t, s))
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

func connect(t *testing.T, s *Session, hedef string) int {
	t.Helper()
	_, _, kod := connectRaw(t, s, hedef)
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

func TestGate_OturumAcilirVeAdresVerir(t *testing.T) {
	s := oturumAc(t, Run{ID: "k1"})

	require.Contains(t, s.ProxyURL(), "http://127.0.0.1:")
	c, err := net.Dial("tcp", adres(t, s))
	require.NoError(t, err)
	_ = c.Close()
}

/*
 * Kapatılan oturum artık bağlantı KABUL ETMEZ.
 *
 * "Ayar boşken kapı kapalıdır" kuralının testi: denetim kapalı olan bir
 * çalıştırma için hiç oturum açılmıyor, dolayısıyla ortada dinlenen bir port
 * da olmuyor. Oturum kapandıktan sonra da aynısı geçerli — açık kalan bir
 * dinleyici, çalıştırma bittikten sonra kullanılabilir bir çıkış bırakırdı.
 */
func TestGate_KapatilanOturumBaglantiKabulEtmez(t *testing.T) {
	s, err := New("127.0.0.1").Open(Run{ID: "k1"})
	require.NoError(t, err)
	hedef := adres(t, s)

	require.NoError(t, s.Close())

	c, err := net.Dial("tcp", hedef)
	if err == nil {
		_ = c.Close()
		t.Fatal("kapatılmış oturum hâlâ bağlantı kabul ediyor")
	}
}

// İzinli host upstream'e devredilir VE tünelden veri akar. Yalnızca "200 döndü"
// demek yetmez — 200 dönüp tüneli bağlamayan bir kapı da o testi geçerdi.
func TestGate_IzinliHostUpstreameDevredilirVeVeriAkar(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	desenler, err := hostlist.Parse("ornek.com")
	require.NoError(t, err)
	s := oturumAc(t, Run{ID: "k1", Upstream: upstream, Allow: desenler})

	c, br, kod := connectRaw(t, s, "ornek.com:443")
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
	desenler, err := hostlist.Parse("ornek.com")
	require.NoError(t, err)

	var reddedilen []string
	s := oturumAc(t, Run{
		ID: "k1", Upstream: upstream, Allow: desenler,
		OnDeny: func(host string) { reddedilen = append(reddedilen, host) },
	})

	require.Equal(t, http.StatusForbidden, connect(t, s, "yasak.com:443"))
	require.Equal(t, []string{"yasak.com"}, reddedilen)
	require.Empty(t, *gorulen, "reddedilen istek upstream'e ulaşmamalı")
}

// Boş Allow her host'u geçirir — spec 020: boş whitelist kısıtsızlıktır.
func TestGate_BosWhitelistHerHostuGecirir(t *testing.T) {
	upstream, _ := sahteUpstream(t)
	s := oturumAc(t, Run{ID: "k1", Upstream: upstream})

	require.Equal(t, http.StatusOK, connect(t, s, "herhangi.com:443"))
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
	desenler, err := hostlist.Parse("izinli.com")
	require.NoError(t, err)
	s := oturumAc(t, Run{ID: "k1", Upstream: upstream, Allow: desenler})

	istemci := &http.Client{Transport: &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(s.ProxyURL()) },
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
	desenler, err := hostlist.Parse("izinli.com")
	require.NoError(t, err)
	s := oturumAc(t, Run{ID: "k1", Upstream: "127.0.0.1:1", Allow: desenler})

	istemci := &http.Client{Transport: &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(s.ProxyURL()) },
	}}
	resp, err := istemci.Get("http://yasak.com/")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

// Eşzamanlı oturumlar birbirinin kurallarını GÖRMEZ: her çalıştırma kendi
// dinleyicisinde, kendi izin listesiyle.
func TestGate_EszamanliOturumlarIzoledir(t *testing.T) {
	upstream, _ := sahteUpstream(t)
	g := New("127.0.0.1")

	dar, err := hostlist.Parse("izinli.com")
	require.NoError(t, err)

	s1, err := g.Open(Run{ID: "k1", Upstream: upstream, Allow: dar})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s1.Close() })

	s2, err := g.Open(Run{ID: "k2", Upstream: upstream}) // kısıtsız
	require.NoError(t, err)
	t.Cleanup(func() { _ = s2.Close() })

	require.NotEqual(t, s1.ProxyURL(), s2.ProxyURL(), "her oturum kendi portunda")
	require.Equal(t, http.StatusForbidden, connect(t, s1, "baska.com:443"))
	require.Equal(t, http.StatusOK, connect(t, s2, "baska.com:443"))
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
	s := oturumAc(t, Run{ID: "k1", Upstream: upstream})

	c, br, kod := connectRaw(t, s, "ornek.com:443")
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
	s := oturumAc(t, Run{ID: "k1", Upstream: "127.0.0.1:1"})

	require.Equal(t, http.StatusBadGateway, connect(t, s, "ornek.com:443"))
}

// ─── Spec 026: kurum içi domain'ler ─────────────────────────────────────────

/*
sahteHedef, kapının DOĞRUDAN bağlandığı hedefi taklit eder.

`sahteUpstream`'in ikizi ve testlerin ölçme aracı: yönlendirme testlerinin
tamamı "hangi taraf bağlantı aldı" sorusuna dayanıyor. İki taraf da sayaç
tuttuğu için iddia "200 döndü" değil, "şu taraf arandı, diğeri ARANMADI"
biçiminde kurulabiliyor — 200 dönmesi yönlendirmenin doğru olduğunu
kanıtlamazdı.

`localhost` üzerinde dinliyor çünkü desen listesi IP kabul etmiyor (spec 020:
whitelist yalnızca domain). `localhost` tek parçalı geçerli bir addır.
*/
func sahteHedef(t *testing.T) (adres string, sayac *atomic.Int32) {
	t.Helper()
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })

	// ATOMİK: test sayacı yoklarken kabul döngüsü onu yazıyor. Mutex'le
	// yazıp kilitsiz okumak yarış — yarış dedektörüyle yakalandı.
	var n atomic.Int32

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			n.Add(1)

			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)

				// Düz HTTP mi tünel mi: proxy'siz taşıyıcı origin-form
				// gönderir, tünelde ise ham baytlar gelir.
				if bas, err := br.Peek(4); err == nil && string(bas) == "GET " {
					_, _ = http.ReadRequest(br)
					_, _ = c.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 6\r\n" +
						"Connection: close\r\n\r\nhedefe"))
					return
				}
				_, _ = io.Copy(c, br) // tünelde yankı
			}(c)
		}
	}()

	return l.Addr().String(), &n
}

// Düzenek kendi kendini doğrular: sahte hedef gerçekten sayıyor mu?
func TestSahteHedef_BaglantiSayar(t *testing.T) {
	hedef, sayac := sahteHedef(t)
	require.EqualValues(t, 0, sayac.Load())

	c, err := net.Dial("tcp", hedef)
	require.NoError(t, err)
	defer c.Close()

	require.Eventually(t, func() bool { return sayac.Load() == 1 },
		2*time.Second, 10*time.Millisecond)
}

// Direct BOŞKEN hiçbir hedef doğrudan gitmez — hepsi proxy'den geçer.
// Bu, `hostlist.Match` yerine `Listed` kullanılmasının tek görünür sonucu.
func TestGate_DirectBossaUpstreameGider(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	hedef, hedefSayac := sahteHedef(t)
	_, port, err := net.SplitHostPort(hedef)
	require.NoError(t, err)

	s := oturumAc(t, Run{ID: "k1", Upstream: upstream})

	require.Equal(t, http.StatusOK, connect(t, s, "localhost:"+port))
	require.Equal(t, []string{"localhost:" + port}, *gorulen)
	require.EqualValues(t, 0, hedefSayac.Load(), "hedefe doğrudan gidilmemeliydi")
}

func TestGate_DirectEslesirseHedefeDogrudanGider(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	hedef, hedefSayac := sahteHedef(t)
	_, port, err := net.SplitHostPort(hedef)
	require.NoError(t, err)

	dogrudan, err := hostlist.Parse("localhost")
	require.NoError(t, err)
	s := oturumAc(t, Run{ID: "k1", Upstream: upstream, Direct: dogrudan})

	c, br, kod := connectRaw(t, s, "localhost:"+port)
	require.Equal(t, http.StatusOK, kod)

	// Tünel gerçekten hedefe bağlı mı: yankı hedeften geliyor.
	_, err = c.Write([]byte("merhaba"))
	require.NoError(t, err)
	tampon := make([]byte, 7)
	_, err = io.ReadFull(br, tampon)
	require.NoError(t, err)
	require.Equal(t, "merhaba", string(tampon))

	require.EqualValues(t, 1, hedefSayac.Load(), "hedefe doğrudan bağlanılmalıydı")
	require.Empty(t, *gorulen, "kurumsal proxy HİÇ aranmamalıydı")
}

func TestGate_DirectEslesmezseUpstreameGider(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	hedef, hedefSayac := sahteHedef(t)
	_, port, err := net.SplitHostPort(hedef)
	require.NoError(t, err)

	dogrudan, err := hostlist.Parse("baska.local")
	require.NoError(t, err)
	s := oturumAc(t, Run{ID: "k1", Upstream: upstream, Direct: dogrudan})

	require.Equal(t, http.StatusOK, connect(t, s, "localhost:"+port))
	require.Equal(t, []string{"localhost:" + port}, *gorulen)
	require.EqualValues(t, 0, hedefSayac.Load())
}

/*
SIRA: önce izin, sonra yönlendirme (spec 026 H2).

Direct listesine yazmak izin VERMEZ. Sıra tersine dönseydi, kurum içi
listesine bir adres yazan yönetici farkında olmadan izin listesini de
genişletmiş olurdu.
*/
func TestGate_IzinsizHostDirectOlsaBileReddedilir(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	hedef, hedefSayac := sahteHedef(t)
	_, port, err := net.SplitHostPort(hedef)
	require.NoError(t, err)

	izin, err := hostlist.Parse("baska.com")
	require.NoError(t, err)
	dogrudan, err := hostlist.Parse("localhost")
	require.NoError(t, err)

	s := oturumAc(t, Run{ID: "k1", Upstream: upstream, Allow: izin, Direct: dogrudan})

	require.Equal(t, http.StatusForbidden, connect(t, s, "localhost:"+port))
	require.EqualValues(t, 0, hedefSayac.Load(), "reddedilen istek hedefe gitmemeli")
	require.Empty(t, *gorulen, "reddedilen istek proxy'ye de gitmemeli")
}

/*
GERİ DÜŞME YOK (spec 026 H4).

İddia NEGATİF: doğrudan bağlantı düştüğünde upstream'in ARANMADIĞI. Yalnızca
dönen koda bakmak yetmezdi — geri düşme olsaydı ve proxy de ulaşılamaz olsaydı
yine 502 dönerdi. Ölçülen şey sahte upstream'in sayacı.
*/
func TestGate_DogrudanBaglantiDuserseProxyDenenmez(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)

	// `.invalid` hiçbir zaman çözülmez (RFC 2606).
	dogrudan, err := hostlist.Parse("yok.invalid")
	require.NoError(t, err)
	s := oturumAc(t, Run{ID: "k1", Upstream: upstream, Direct: dogrudan})

	require.Equal(t, http.StatusBadGateway, connect(t, s, "yok.invalid:443"))
	require.Empty(t, *gorulen, "doğrudan bağlantı düşünce proxy DENENMEMELİ")
}

// ─── Aynı üç karar düz HTTP için ────────────────────────────────────────────

func TestGate_DuzHTTPDirectEslesirseHedefeGider(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	hedef, hedefSayac := sahteHedef(t)
	_, port, err := net.SplitHostPort(hedef)
	require.NoError(t, err)

	dogrudan, err := hostlist.Parse("localhost")
	require.NoError(t, err)
	s := oturumAc(t, Run{ID: "k1", Upstream: upstream, Direct: dogrudan})

	istemci := &http.Client{Transport: &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(s.ProxyURL()) },
	}}
	resp, err := istemci.Get("http://localhost:" + port + "/paket")
	require.NoError(t, err)
	defer resp.Body.Close()

	govde, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "hedefe", string(govde), "yanıt hedeften gelmeliydi")
	require.EqualValues(t, 1, hedefSayac.Load())
	require.Empty(t, *gorulen, "kurumsal proxy HİÇ aranmamalıydı")
}

func TestGate_DuzHTTPDirectBossaUpstreameGider(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)
	hedef, hedefSayac := sahteHedef(t)
	_, port, err := net.SplitHostPort(hedef)
	require.NoError(t, err)

	s := oturumAc(t, Run{ID: "k1", Upstream: upstream})

	istemci := &http.Client{Transport: &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(s.ProxyURL()) },
	}}
	resp, err := istemci.Get("http://localhost:" + port + "/paket")
	require.NoError(t, err)
	defer resp.Body.Close()

	govde, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "selam", string(govde), "yanıt proxy'den gelmeliydi")
	require.EqualValues(t, 0, hedefSayac.Load())
	require.Len(t, *gorulen, 1)
}

func TestGate_DuzHTTPDogrudanDuserseProxyDenenmez(t *testing.T) {
	upstream, gorulen := sahteUpstream(t)

	dogrudan, err := hostlist.Parse("yok.invalid")
	require.NoError(t, err)
	s := oturumAc(t, Run{ID: "k1", Upstream: upstream, Direct: dogrudan})

	istemci := &http.Client{Transport: &http.Transport{
		Proxy: func(*http.Request) (*url.URL, error) { return url.Parse(s.ProxyURL()) },
	}}
	resp, err := istemci.Get("http://yok.invalid/paket")
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	require.Empty(t, *gorulen, "doğrudan bağlantı düşünce proxy DENENMEMELİ")
}
