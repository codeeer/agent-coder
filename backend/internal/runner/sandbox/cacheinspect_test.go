package sandbox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Yardımcı container ÇIKTISININ ayrıştırılması (spec 027).
 *
 * Çıktı makinece okunur olmak zorunda: insan için biçimlenmiş bir metin
 * ("1,2 GB") yerelleştirmeye ve yuvarlamaya tabidir; boyut göstergesi de,
 * temizleme onayındaki "ne kadar yer boşalacak" sayısı da ondan besleniyor.
 *
 * Ayrıştırma SENTETİK çıktıyla test ediliyor: gerçek Docker'a ihtiyaç yok ve
 * asıl kırılgan kısım burası — kenar durumlar (boş çıktı, hatalı satır, eksik
 * dosya) gerçek bir önbellekte kolay üretilemez.
 */

func TestParseDuBytes_ByteCinsindenOkur(t *testing.T) {
	// `du -sb` çıktısı: bayt sayısı, sekme, yol.
	n, err := parseDuBytes("1234567\t/home/agent/.m2/repository\n")
	require.NoError(t, err)
	require.Equal(t, int64(1234567), n)
}

func TestParseDuBytes_BosOnbellekSifirDoner(t *testing.T) {
	n, err := parseDuBytes("0\t/home/agent/.npm/_cacache\n")
	require.NoError(t, err)
	require.Zero(t, n)
}

// Boşluk ayraçlı çıktı da kabul edilir; `du` uygulamaları arasında değişiyor.
func TestParseDuBytes_BosluklaAyrilmisCiktiyiDaOkur(t *testing.T) {
	n, err := parseDuBytes("4096 /home/agent/.m2/repository")
	require.NoError(t, err)
	require.Equal(t, int64(4096), n)
}

/*
ANLAŞILMAYAN ÇIKTI SIFIR DEĞİL, HATA.

Sıfır dönmek "önbellek boş" demektir ve kullanıcıya YANLIŞ bilgi verir —
üstelik "temizle" onayında "0 B boşalacak" yazarak temizlemeyi anlamsız
gösterir. Bilinmeyen ile boş ayrı şeyler (spec 027 H3).
*/
func TestParseDuBytes_AnlasilmayanCiktiHataDoner(t *testing.T) {
	for _, girdi := range []string{"", "\n", "du: erişim reddedildi", "abc\t/yol"} {
		_, err := parseDuBytes(girdi)
		require.Error(t, err, "girdi: %q", girdi)
	}
}

/*
`.sha1` DOSYASININ İKİ BİÇİMİ DE KABUL EDİLİR.

Maven'ın yerel deposunda her iki biçim de bulunuyor: kimi artefakt yalnızca
özeti taşıyor, kimi `sha1sum` çıktısı gibi "özet  ad". Yalnız birini kabul
etmek, diğerini "özeti yok" sayıp denetlenemeyenler kutusuna atardı — yani
tarama sessizce eksik çalışırdı.
*/
func TestNormalizeSHA1_IkiBicimiDeKabulEder(t *testing.T) {
	const ozet = "2fd4e1c67a2d28fced849ee1bb76e7391b93eb12"

	for ad, girdi := range map[string]string{
		"yalnız özet":        ozet,
		"sonunda satır sonu": ozet + "\n",
		"özet ve ad":         ozet + "  kutuphane-1.0.jar",
		"tek boşluk":         ozet + " kutuphane-1.0.jar",
		"sekmeli":            ozet + "\tkutuphane-1.0.jar",
		"baştaki boşluk":     "  " + ozet + "\n",
		"BÜYÜK HARF":         "2FD4E1C67A2D28FCED849EE1BB76E7391B93EB12",
	} {
		got, ok := normalizeSHA1(girdi)
		require.True(t, ok, "biçim kabul edilmeli: %s", ad)
		require.Equal(t, ozet, got, "biçim: %s", ad)
	}
}

/*
BOZUK `.sha1` "UYUŞMUYOR" DEĞİL, "DENETLENEMEDİ".

Fark hayati: uyuşmayan artefakt SİLİNİYOR. Kırpılmış veya bozulmuş bir özet
dosyası yüzünden sağlam bir artefaktı silmek, doğrulamayı düzeltmesi gereken
şeyin kaynağına çevirirdi (spec 027 H5).
*/
func TestNormalizeSHA1_BozukOzetKabulEdilmez(t *testing.T) {
	for ad, girdi := range map[string]string{
		"boş":             "",
		"yalnız boşluk":   "   \n",
		"kısa":            "2fd4e1c6",
		"onaltılık değil": "zzzze1c67a2d28fced849ee1bb76e7391b93eb12",
		"uzun":            "2fd4e1c67a2d28fced849ee1bb76e7391b93eb12ab",
	} {
		_, ok := normalizeSHA1(girdi)
		require.False(t, ok, "kabul edilmemeli: %s", ad)
	}
}
