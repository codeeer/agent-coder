package opencode

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
)

/*
 * İPTAL İLE ZAMAN AŞIMININ AYRIMI.
 *
 * İkisi de context iptali üretiyor ama kullanıcı için farklı anlamlar taşıyor:
 * biri KENDİ kararı, diğeri SİSTEMİN sınırı. Çalıştırmalar ekranında biri
 * "iptal edildi", diğeri "zaman aşımı" olarak duruyor ve kullanıcı ikisinde
 * farklı şey yapıyor — birinde bir şey yapmıyor, diğerinde süre sınırını
 * artırıyor ya da görevi bölüyor.
 *
 * Mutasyon taraması bu ayrımın korunmadığını gösterdi: `parent.Err()`
 * kontrolü kaldırıldığında hiçbir test kırmızıya dönmüyordu.
 */

/*
İPTAL, ZAMAN AŞIMINDAN ÖNCE GELİR.

Sıra önemli ve tam da burada bir tuzak var: kullanıcı işi durdurduğunda üst
context iptal oluyor, türetilmiş `runCtx` de onunla birlikte. Yalnızca
`runCtx`'e bakılsaydı sonuç "iptal" değil, sınıflandırılamamış ham hata
olurdu — kullanıcı kendi durdurduğu işi ekranda BAŞARISIZ olarak görürdü ve
neyin bozulduğunu aramaya başlardı.
*/
func TestClassifyCtx_KullaniciIptaliZamanAsimiSayilmaz(t *testing.T) {
	parent, iptal := context.WithCancel(context.Background())
	runCtx, bitir := context.WithTimeout(parent, time.Hour)
	defer bitir()

	iptal() // kullanıcı "Durdur"a bastı

	require.ErrorIs(t, classifyCtx(runCtx, parent), runner.ErrCancelled)
}

/*
SÜRE SINIRI İPTAL SAYILMAZ.

Karşı yön de yazılı olmalı: her şeye "iptal" diyen bir kontrol de yukarıdaki
testi geçerdi. Burada üst context CANLI, yalnızca çalıştırmanın kendi süresi
doldu.
*/
func TestClassifyCtx_SureSiniriZamanAsimidir(t *testing.T) {
	parent := context.Background()
	runCtx, bitir := context.WithTimeout(parent, time.Nanosecond)
	defer bitir()

	<-runCtx.Done()
	require.ErrorIs(t, classifyCtx(runCtx, parent), runner.ErrTimeout)
}

// Hiçbiri bitmemişse context kaynaklı bir hata YOK: sınıflandırma ham hataya
// devam etmeli, yoksa her arıza "iptal" ya da "zaman aşımı" görünürdü.
func TestClassifyCtx_CalisanIsSiniflandirilmaz(t *testing.T) {
	parent := context.Background()
	runCtx, bitir := context.WithTimeout(parent, time.Hour)
	defer bitir()

	require.NoError(t, classifyCtx(runCtx, parent))
}

/*
`classify` de aynı sırayı korur.

Ayrım yalnızca `classifyCtx` içinde doğru olup çağıran yerde kaybolsaydı hiçbir
işe yaramazdı: iptal edilmiş bir koşuda motor da kendi hatasını üretiyor ve o
hata öne geçseydi ekranda yine "iptal" değil, anlamsız bir motor hatası
görünürdü.
*/
func TestClassify_IptalMotorHatasinaBaskinGelir(t *testing.T) {
	parent, iptal := context.WithCancel(context.Background())
	runCtx, bitir := context.WithTimeout(parent, time.Hour)
	defer bitir()
	iptal()

	motorHatasi := errors.New("oturum kapandı")
	require.ErrorIs(t, classify(motorHatasi, runCtx, parent), runner.ErrCancelled)
}
