package bitbucket

import (
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Grup adresinin çözülmesi.
 *
 * Kullanıcı tarayıcıdan kopyaladığı adresi yapıştırıyor; ayrı bir "sunucu
 * adresi" ve "grup anahtarı" alanı doldurmuyor. Dolayısıyla ayrıştırma
 * kullanıcının eline geçen HER biçimi tolere etmek zorunda.
 *
 * ÖLÇÜM YOK: gerçek bir kurumsal sunucuya erişimimiz olmadığı için bu testler
 * belgeye dayanan varsayımları kilitliyor, gerçeği değil (spec 021 →
 * Belirsizlikler). Bu yüzden hata mesajları ham girdiyi saklamıyor.
 */

func TestParseGroupURL_DuzAdres(t *testing.T) {
	g, err := ParseGroupURL("https://bb.sirket.com/projects/ODEME")

	require.NoError(t, err)
	require.Equal(t, "https://bb.sirket.com", g.BaseURL)
	require.Equal(t, "ODEME", g.Key)
}

/*
 * CONTEXT PATH — kurumsal kurulumların çoğu kökte değil.
 *
 * Taban adres `/projects/{KEY}` parçasından ÖNCESİDİR. Sabit host varsayan bir
 * ayrıştırma bu kurulumlarda API'yi yanlış yere sorar ve kullanıcı sebebi
 * anlaşılmayan bir 404 alır.
 */
func TestParseGroupURL_ContextPath(t *testing.T) {
	g, err := ParseGroupURL("https://sirket.com/bitbucket/projects/ODEME")

	require.NoError(t, err)
	require.Equal(t, "https://sirket.com/bitbucket", g.BaseURL)
	require.Equal(t, "ODEME", g.Key)
}

// Adres çubuğundan gelen artıklar: sondaki eğik çizgi, derinlere inen yol,
// sorgu ve çapa. Hiçbiri grubun kimliğini değiştirmiyor.
func TestParseGroupURL_FazlalikTolereEdilir(t *testing.T) {
	girdiler := []string{
		"https://bb.sirket.com/projects/ODEME/",
		"https://bb.sirket.com/projects/ODEME/repos/api/browse",
		"https://bb.sirket.com/projects/ODEME?avatarSize=64",
		"  https://bb.sirket.com/projects/ODEME#tab  ",
	}

	for _, girdi := range girdiler {
		t.Run(girdi, func(t *testing.T) {
			g, err := ParseGroupURL(girdi)

			require.NoError(t, err)
			require.Equal(t, "https://bb.sirket.com", g.BaseURL)
			require.Equal(t, "ODEME", g.Key)
		})
	}
}

/*
 * Kişisel repository'ler ayrı bir yol altında duruyor ama API'de yine bir
 * "project" — anahtarı `~KULLANICI`. Büyük harfe çevriliyor: Bitbucket
 * kişisel proje anahtarını bu biçimde saklıyor.
 */
func TestParseGroupURL_KisiselAlan(t *testing.T) {
	g, err := ParseGroupURL("https://bb.sirket.com/users/ahmet")

	require.NoError(t, err)
	require.Equal(t, "https://bb.sirket.com", g.BaseURL)
	require.Equal(t, "~AHMET", g.Key)
}

func TestParseGroupURL_KisiselAlanDerinYol(t *testing.T) {
	g, err := ParseGroupURL("https://bb.sirket.com/users/ahmet/repos/deneme/browse")

	require.NoError(t, err)
	require.Equal(t, "~AHMET", g.Key)
}

/*
 * Grup adresi OLMAYAN girdiler.
 *
 * Depo adresi yapıştırmak en olası yanlış: kullanıcının elinde ikisi de var ve
 * biri diğerine benziyor. Hata bunu ayırt edip ne beklendiğini söylemeli.
 */
func TestParseGroupURL_GrupAdresiDegil(t *testing.T) {
	girdiler := map[string]string{
		"depo klonlama adresi": "https://bb.sirket.com/scm/ODEME/api.git",
		"kök adres":            "https://bb.sirket.com",
		"anahtarsız":           "https://bb.sirket.com/projects",
		"anahtarsız eğik":      "https://bb.sirket.com/projects/",
		"şemasız":              "bb.sirket.com/projects/ODEME",
		"boş":                  "   ",
	}

	for ad, girdi := range girdiler {
		t.Run(ad, func(t *testing.T) {
			_, err := ParseGroupURL(girdi)
			require.ErrorIs(t, err, ErrNotGroupURL)
		})
	}
}

/*
 * BULUT ADRESİ AYRI HATA VERİR ve denetim ayrıştırmadan ÖNCE koşar.
 *
 * Bulut adresi de `/projects/…` içerebiliyor. Sıra ters olsaydı kullanıcı
 * "bu yol kurumsal kurulumlar için" mesajı yerine anlamsız bir grup hatası
 * alırdı — spec 021 H4 tam olarak bunu yasaklıyor.
 */
func TestParseGroupURL_BulutAdresi(t *testing.T) {
	girdiler := []string{
		"https://bitbucket.org/takimim/depo",
		"https://bitbucket.org/takimim/workspace/projects/ODEME",
		"https://api.bitbucket.org/2.0/repositories",
		"https://BITBUCKET.ORG/takimim",
	}

	for _, girdi := range girdiler {
		t.Run(girdi, func(t *testing.T) {
			_, err := ParseGroupURL(girdi)
			require.ErrorIs(t, err, ErrCloudAddress)
		})
	}
}

// Kurumsal sunucunun adı "bitbucket" içerebilir; bu onu bulut yapmaz.
// Karşılaştırma host üzerinden, metin araması değil.
func TestParseGroupURL_BulutBenzeriKurumsalAdres(t *testing.T) {
	g, err := ParseGroupURL("https://bitbucket.sirket.local/projects/ODEME")

	require.NoError(t, err)
	require.Equal(t, "https://bitbucket.sirket.local", g.BaseURL)
}

// API adresi, taban adresten türetilir — context path dahil.
func TestGroupRef_APIYolu(t *testing.T) {
	g := GroupRef{BaseURL: "https://sirket.com/bitbucket", Key: "ODEME"}

	require.Equal(t,
		"https://sirket.com/bitbucket/rest/api/1.0/projects/ODEME/repos",
		g.reposURL())
}
