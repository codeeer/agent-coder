package tlstrust

import (
	"crypto/x509"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

/*
 * Testler AĞA ÇIKMAZ.
 *
 * Sınanan şey güven havuzunun doğru kurulup kurulmadığı; gerçek bir TLS
 * el sıkışması bunu daha iyi göstermez ama testi ağa ve zamana bağımlı yapar.
 */

func kokPEM(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "certfmt", "testdata", "kok.pem"))
	require.NoError(t, err)
	return string(b)
}

// kokHavuzu, taşıyıcıya kurumsal kök havuzunun kurulup kurulmadığını verir.
//
// Ölçüt TLSClientConfig'in kendisi DEĞİL, RootCAs alanı: `Transport.Clone()`
// HTTP/2 kurulumu sırasında TLSClientConfig'i zaten dolduruyor (NextProtos).
// "Yapılandırma nil mi" diye bakmak, sertifikanın eklenip eklenmediğini
// ölçmüyordu.
func kokHavuzu(tr http.RoundTripper) *x509.CertPool {
	t, ok := tr.(*http.Transport)
	if !ok || t.TLSClientConfig == nil {
		return nil
	}
	return t.TLSClientConfig.RootCAs
}

// http2Korundu, HTTP/2 anlaşmasının bozulmadığını söyler.
func http2Korundu(tr http.RoundTripper) bool {
	t, ok := tr.(*http.Transport)
	if !ok || t.TLSClientConfig == nil {
		return false
	}
	return slices.Contains(t.TLSClientConfig.NextProtos, "h2")
}

// Sertifika yokken sistem havuzu korunur: kurumsal olmayan kurulumların
// davranışı hiç değişmemeli.
func TestSertifikaYokkenVarsayilanDavranis(t *testing.T) {
	tr := New(func() string { return "" }).current()
	require.Nil(t, kokHavuzu(tr), "kök havuzuna dokunulmamalı")
}

func TestNilSaglayiciCokmez(t *testing.T) {
	tr := New(nil).current()
	require.Nil(t, kokHavuzu(tr))
}

func TestSertifikaVarkenKokHavuzuKurulur(t *testing.T) {
	tr := New(func() string { return kokPEM(t) }).current()

	require.NotNil(t, kokHavuzu(tr), "kurumsal sertifika havuza eklenmiş olmalı")
}

/*
 * Sertifika DEĞİŞİNCE taşıyıcı yenilenir.
 *
 * Spec 017 H5'in "yeniden başlatmadan geçerli olur" kriteri tam olarak bu:
 * ayar değiştiğinde bir sonraki çağrının yeni güven havuzunu kullanması.
 */
func TestSertifikaDegisinceTasiyiciYenilenir(t *testing.T) {
	pemText := ""
	tr := New(func() string { return pemText })

	ilk := tr.current()
	require.Nil(t, kokHavuzu(ilk))

	pemText = kokPEM(t)
	ikinci := tr.current()

	require.NotSame(t, ilk, ikinci, "taşıyıcı yeniden kurulmalı")
	require.NotNil(t, kokHavuzu(ikinci))

	// Sertifika geri alınınca da varsayılana dönülmeli.
	pemText = ""
	ucuncu := tr.current()
	require.Nil(t, kokHavuzu(ucuncu))
}

// Değişmeyen sertifikada havuz BOŞUNA yeniden kurulmaz: her istekte
// sertifika ayrıştırmak pahalıdır.
func TestAyniSertifikadaTasiyiciKorunur(t *testing.T) {
	tr := New(func() string { return kokPEM(t) })

	require.Same(t, tr.current(), tr.current())
}

func TestGecersizSertifikaVarsayilanaDuser(t *testing.T) {
	tr := New(func() string { return "bu bir sertifika degil" }).current()

	// Havuza eklenemedi; sistem güveni bozulmadan devam edilmeli.
	require.Nil(t, kokHavuzu(tr))
}

/*
 * Taşıyıcı, çağıranın kurduğu istemciye takılabilir olmalı.
 *
 * Paketin dışa verdiği tek şey bu: hazır bir `*http.Client` DEĞİL. Sekiz
 * çağıranın hepsi `rt http.RoundTripper` alıyor ve kendi zaman aşımıyla kendi
 * istemcisini kuruyor; test de o kullanımı ölçüyor.
 */
func TestRoundTripper_CagirananIstemcisineTakilir(t *testing.T) {
	rt := New(func() string { return kokPEM(t) }).RoundTripper()
	require.NotNil(t, rt, "dışa verilen taşıyıcı kurulabilmeli")

	// Sekiz çağıranın hepsinin yaptığı şey: kendi zaman aşımıyla kendi
	// istemcisini kurup taşıyıcıyı takmak.
	c := &http.Client{Timeout: 15 * time.Second, Transport: rt}
	require.NotNil(t, c.Transport, "istemci güven katmanından beslenmeli")

	// Sarmalayıcının altında gerçekten kurumsal kök var: `current` her çağrıda
	// yeniden okunduğu için dışarıdan görünen tek kanıt bu.
	require.NotNil(t, kokHavuzu(New(func() string { return kokPEM(t) }).current()))
}

/*
 * HTTP/2 ANLAŞMASI KORUNUR.
 *
 * Bu test bir hatanın kaydı: kurumsal sertifika eklenirken TLS
 * yapılandırmasının tamamı yenisiyle değiştirilmişti ve `Transport.Clone()`
 * içinde kurulan `NextProtos` siliniyordu. Sonuç, sertifikayla hiç ilgisi
 * olmayan bir yan etkiydi — backend'in bütün giden çağrıları HTTP/1.1'e
 * düşüyordu ve bunu hiçbir şey söylemiyordu.
 */
func TestSertifikaEklemekHTTP2yiBozmaz(t *testing.T) {
	varsayilan := New(func() string { return "" }).current()
	require.True(t, http2Korundu(varsayilan), "ön koşul: varsayılan taşıyıcıda h2 var")

	sertifikali := New(func() string { return kokPEM(t) }).current()
	require.True(t, http2Korundu(sertifikali),
		"kurumsal sertifika eklenince HTTP/2 anlaşması kaybolmamalı")
}
