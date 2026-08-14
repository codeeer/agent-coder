package certinfo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func oku(t *testing.T, ad string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", ad))
	require.NoError(t, err)
	return string(b)
}

func TestParse_KokSertifikaOkunur(t *testing.T) {
	got, err := Parse(oku(t, "kok.pem"))
	require.NoError(t, err)
	require.Len(t, got, 1)

	require.Equal(t, "Ornek Kurum Kok CA", got[0].Subject)
	require.Equal(t, "Ornek Kurum Kok CA", got[0].Issuer, "kök sertifikayı kendisi imzalar")
	require.True(t, got[0].SelfSigned)
	require.False(t, got[0].Expired)
	require.True(t, got[0].NotAfter.After(time.Now()))
}

// Ara sertifikanın imzalayanı kendisi DEĞİL — kök. Bu ayrım ekranda
// "bu bir kök mü, ara mı" sorusunu cevaplıyor.
func TestParse_AraSertifikaninImzalayaniKok(t *testing.T) {
	got, err := Parse(oku(t, "ara.pem"))
	require.NoError(t, err)
	require.Len(t, got, 1)

	require.Equal(t, "Ornek Kurum Ara CA", got[0].Subject)
	require.Equal(t, "Ornek Kurum Kok CA", got[0].Issuer)
	require.False(t, got[0].SelfSigned)
}

func TestParse_ZincirdekiHerSertifikaAyriGosterilir(t *testing.T) {
	got, err := Parse(oku(t, "zincir.pem"))
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "Ornek Kurum Kok CA", got[0].Subject)
	require.Equal(t, "Ornek Kurum Ara CA", got[1].Subject)
}

/*
 * Süresi dolmuş sertifika REDDEDİLMEZ.
 *
 * Kurum hâlâ o sertifikayı kullanıyor olabilir ve ürünün onu kabul etmemesi,
 * çalışan bir kurulumu kilitlerdi. Yapılacak şey durumu SÖYLEMEK.
 */
func TestParse_SuresiDolmusSertifikaKabulEdilirAmaIsaretlenir(t *testing.T) {
	got, err := Parse(oku(t, "suresi-dolmus.pem"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.True(t, got[0].Expired, "süresi dolmuş olarak işaretlenmeli")
	require.Equal(t, "Suresi Dolmus CA", got[0].Subject)
}

// "Dolmuş mu" sorusu hangi ana göre bakıldığına bağlı; sabit bir anla sınanır.
func TestParse_DolmaDurumuVerilenAnaGoreHesaplanir(t *testing.T) {
	pemText := oku(t, "kok.pem")

	gecmis, err := parseAt(pemText, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.False(t, gecmis[0].Expired)

	gelecek, err := parseAt(pemText, time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.True(t, gelecek[0].Expired, "bitiş tarihinden sonra dolmuş sayılmalı")
}

func TestParse_SertifikaOlmayanMetinReddedilir(t *testing.T) {
	_, err := Parse("bu bir sertifika degil")
	require.ErrorIs(t, err, ErrNoCertificate)
}

func TestParse_BosMetinReddedilir(t *testing.T) {
	_, err := Parse("")
	require.ErrorIs(t, err, ErrNoCertificate)
}

// Bozuk bir blok, yanındaki geçerli sertifikayı gölgelememeli.
func TestParse_BozukBlokAtlanirGecerliOlanGosterilir(t *testing.T) {
	bozuk := "-----BEGIN CERTIFICATE-----\nYnV6dWs=\n-----END CERTIFICATE-----\n"
	got, err := Parse(bozuk + oku(t, "kok.pem"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "Ornek Kurum Kok CA", got[0].Subject)
}
