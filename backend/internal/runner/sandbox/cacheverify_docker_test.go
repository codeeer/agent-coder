package sandbox_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner/sandbox"
)

/*
 * Doğrulama — GERÇEK Docker üzerinde (spec 027 H5).
 *
 * Karar mantığı Docker'sız test ediliyor; burada sorulan şey uçtan uca
 * çalışıp çalışmadığı: tarama betiği gerçekten dosyaları buluyor mu, silme
 * gerçekten siliyor mu, ve en önemlisi — silmemesi gerekeni bırakıyor mu.
 */
func TestVerifyCache_BozuguSilerSaglamiVeDenetlenemeyeniBirakir(t *testing.T) {
	m, ctx := bakimYoneticisi(t)
	c := sandbox.CacheMount{Volume: onbellekVolume(t, "dogrula"), Target: m2Hedef}
	require.NoError(t, m.EnsureCaches(ctx, []sandbox.CacheMount{c}))

	cli := istemci(t)
	id := yardimci(t, cli, []sandbox.CacheMount{c})

	// Üç artefakt kuruluyor:
	//   saglam  → özeti doğru        (kalmalı)
	//   bozuk   → özeti yanlış       (silinmeli)
	//   ozetsiz → `.sha1` dosyası yok (KALMALI — denetlenemedi)
	calistir(t, cli, id, strings.Join([]string{
		"cd " + m2Hedef,
		"echo saglam-icerik > saglam.jar && sha1sum saglam.jar | awk '{print $1}' > saglam.jar.sha1",
		"echo bozuk-icerik > bozuk.jar && echo aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d > bozuk.jar.sha1",
		"echo ozetsiz-icerik > ozetsiz.jar",
	}, " && "))

	sonuc, err := m.VerifyCache(ctx, imaj(), c)
	require.NoError(t, err)

	require.Equal(t, 2, sonuc.Checked, "özeti okunabilen iki artefakt denetlenmeli")
	require.Equal(t, 1, sonuc.Mismatched, "yalnızca bozuk olan uyuşmamalı")
	require.Equal(t, 1, sonuc.Unverifiable, "özeti olmayan denetlenemedi sayılmalı")
	require.Equal(t, 1, sonuc.Removed)

	kalan := calistir(t, cli, id, "ls "+m2Hedef)
	require.Contains(t, kalan, "saglam.jar", "sağlam artefakt kalmalı")
	require.Contains(t, kalan, "ozetsiz.jar",
		"özeti olmayan artefakt SİLİNMEMELİ — denetlenemedi, bozuk değil")
	require.NotContains(t, kalan, "bozuk.jar", "uyuşmayan artefakt silinmeli")
}

/*
BOZUK `.sha1` DOSYASI SAĞLAM ARTEFAKTI SİLDİRMEZ.

Spec 027 H5'in son kriteri. Kırpılmış bir özet dosyası yüzünden sağlam bir
artefaktı silmek, doğrulamayı düzeltmesi gereken sorunun kaynağına çevirirdi.
*/
func TestVerifyCache_KirpilmisOzetDosyasiSilmeyeYolAcmaz(t *testing.T) {
	m, ctx := bakimYoneticisi(t)
	c := sandbox.CacheMount{Volume: onbellekVolume(t, "kirpik"), Target: m2Hedef}
	require.NoError(t, m.EnsureCaches(ctx, []sandbox.CacheMount{c}))

	cli := istemci(t)
	id := yardimci(t, cli, []sandbox.CacheMount{c})
	calistir(t, cli, id, "cd "+m2Hedef+
		" && echo icerik > k.jar && echo 2fd4e1c6 > k.jar.sha1")

	sonuc, err := m.VerifyCache(ctx, imaj(), c)
	require.NoError(t, err)

	require.Zero(t, sonuc.Removed, "kırpılmış özet hiçbir şeyi sildirmemeli")
	require.Equal(t, 1, sonuc.Unverifiable)
	require.Contains(t, calistir(t, cli, id, "ls "+m2Hedef), "k.jar")
}

// Boş önbellekte tarama sıfır döner, hata vermez.
func TestVerifyCache_BosOnbellekSifirDoner(t *testing.T) {
	m, ctx := bakimYoneticisi(t)
	c := sandbox.CacheMount{Volume: onbellekVolume(t, "bosdogrula"), Target: m2Hedef}
	require.NoError(t, m.EnsureCaches(ctx, []sandbox.CacheMount{c}))

	sonuc, err := m.VerifyCache(ctx, imaj(), c)
	require.NoError(t, err)
	require.Equal(t, sandbox.VerifyResult{}, sonuc)
}
