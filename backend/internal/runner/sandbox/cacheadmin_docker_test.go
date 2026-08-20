package sandbox_test

import (
	"context"
	"strings"
	"testing"
	"time"

	dockerapi "github.com/docker/docker/api/types/container"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner/sandbox"
)

/*
 * Önbellek bakımı — GERÇEK Docker üzerinde (spec 027 H3).
 *
 * Yardımcı container'ın gerçekten çalıştığı, çıktısının okunduğu ve
 * temizlemenin gerçekten sildiği yalnızca burada ölçülebilir. Çıktı
 * ayrıştırması ayrı ve Docker'sız test ediliyor (`cacheinspect_test.go`).
 */

func bakimYoneticisi(t *testing.T) (*sandbox.Manager, context.Context) {
	t.Helper()
	m := yonetici(t)
	ctx, iptal := context.WithTimeout(context.Background(), 180*time.Second)
	t.Cleanup(iptal)

	if err := m.EnsureImage(ctx, imaj()); err != nil {
		t.Skipf("runner imajı yok — atlanıyor: %v", err)
	}
	return m, ctx
}

/*
HİÇ KULLANILMAMIŞ ÖNBELLEK "BOŞ" DEĞİL, "YOK".

Spec 027 H3: sıfır göstermek, çalışmış ama boşalmış bir önbellekle karıştırır.
Ayrım -1 ile taşınıyor; arayüz bundan "henüz kullanılmadı" yazıyor.
*/
func TestCacheSize_HicOlusturulmamisOnbellekEksiBirDoner(t *testing.T) {
	m, ctx := bakimYoneticisi(t)

	boyut, err := m.CacheSize(ctx, imaj(),
		sandbox.CacheMount{Volume: onbellekVolume(t, "yok"), Target: m2Hedef})
	require.NoError(t, err)
	require.EqualValues(t, -1, boyut, "hiç oluşturulmamış önbellek -1 dönmeli")
}

func TestCacheSize_YazilanVeriBoyutaYansir(t *testing.T) {
	m, ctx := bakimYoneticisi(t)
	c := sandbox.CacheMount{Volume: onbellekVolume(t, "boyut"), Target: m2Hedef}
	require.NoError(t, m.EnsureCaches(ctx, []sandbox.CacheMount{c}))

	bos, err := m.CacheSize(ctx, imaj(), c)
	require.NoError(t, err)
	require.GreaterOrEqual(t, bos, int64(0), "oluşturulmuş önbellek -1 dönmemeli")

	cli := istemci(t)
	id := yardimci(t, cli, []sandbox.CacheMount{c})
	// 1 MB yazılıyor: dosya sistemi blok yuvarlamasından belirgin biçimde büyük.
	calistir(t, cli, id, "dd if=/dev/zero of="+m2Hedef+"/dolgu bs=1024 count=1024 2>/dev/null")

	dolu, err := m.CacheSize(ctx, imaj(), c)
	require.NoError(t, err)
	require.Greater(t, dolu, bos+900_000, "yazılan veri boyuta yansımalı")
}

/*
TEMİZLEME GERÇEKTEN SİLER, BOŞALAN BAYTI SÖYLER VE SAHİPLİĞİ KORUR.

Üçü birlikte: silmeyen bir temizleme işe yaramaz, kaç bayt boşaldığını
söylemeyen bir temizleme onay şeridini besleyemez, sahipliği bozan bir
temizleme ise önbelleği kalıcı olarak kullanılamaz hâle getirir — agent bir
daha yazamaz ve bunu kimse fark etmez.
*/
func TestClearCache_SilerBoyutuBildirirVeSahipligiKorur(t *testing.T) {
	m, ctx := bakimYoneticisi(t)
	c := sandbox.CacheMount{Volume: onbellekVolume(t, "temizle"), Target: m2Hedef}
	require.NoError(t, m.EnsureCaches(ctx, []sandbox.CacheMount{c}))

	cli := istemci(t)
	id := yardimci(t, cli, []sandbox.CacheMount{c})
	calistir(t, cli, id, "dd if=/dev/zero of="+m2Hedef+"/dolgu bs=1024 count=1024 2>/dev/null")

	oncesi, err := m.CacheSize(ctx, imaj(), c)
	require.NoError(t, err)

	// Volume'ü tutan container KALDIRILIR: Docker bağlı bir volume'ü silmiyor
	// (ölçüldü) ve gerçek temizleme de koşu yokken yapılıyor.
	require.NoError(t, cli.ContainerRemove(ctx, id,
		dockerapi.RemoveOptions{Force: true}))

	bosalan, err := m.ClearCache(ctx, imaj(), c)
	require.NoError(t, err)
	require.Equal(t, oncesi, bosalan, "boşalan bayt, silinmeden önceki boyut olmalı")

	sonrasi, err := m.CacheSize(ctx, imaj(), c)
	require.NoError(t, err)
	require.Less(t, sonrasi, int64(100_000), "temizlemeden sonra önbellek boş olmalı")

	// SAHİPLİK: temizleme sonrası agent yine yazabilmeli. Volume yeniden
	// oluşturulduğu için sahiplik imajdan yeniden kuruluyor; bu adım o
	// zincirin kopmadığını gösteriyor.
	yeni := yardimci(t, cli, []sandbox.CacheMount{c})
	require.Equal(t, "agent",
		strings.TrimSpace(calistir(t, cli, yeni, "stat -c %U "+m2Hedef)),
		"temizlemeden sonra sahiplik agent'ta kalmalı")
	calistir(t, cli, yeni, "touch "+m2Hedef+"/deneme")
}

// Hiç kullanılmamış önbelleği temizlemek hata değil.
func TestClearCache_HicKullanilmamisOnbellekHataVermez(t *testing.T) {
	m, ctx := bakimYoneticisi(t)

	bosalan, err := m.ClearCache(ctx, imaj(),
		sandbox.CacheMount{Volume: onbellekVolume(t, "bos"), Target: m2Hedef})
	require.NoError(t, err)
	require.Zero(t, bosalan)
}

/*
KULLANIMDAKİ ÖNBELLEK SİLİNMEZ — VE SEBEBİ ANLAŞILIR.

Ölçülmüş davranış: Docker, bağlı bir volume'ü silmeyi reddediyor. Kullanıcıya
"bakım başarısız" demek yanlış olurdu; söylenmesi gereken "şu an yapılamaz".
Çalışan koşu kapısı bunu çoğu zaman önler ama yarışa açıktır.
*/
func TestClearCache_KullanimdaykenSilinmezVeSebebiSoylenir(t *testing.T) {
	m, ctx := bakimYoneticisi(t)
	c := sandbox.CacheMount{Volume: onbellekVolume(t, "mesgul"), Target: m2Hedef}
	require.NoError(t, m.EnsureCaches(ctx, []sandbox.CacheMount{c}))

	// Volume'ü tutan bir container açık kalıyor.
	yardimci(t, istemci(t), []sandbox.CacheMount{c})

	_, err := m.ClearCache(ctx, imaj(), c)
	require.ErrorIs(t, err, sandbox.ErrCacheInUse)
}
