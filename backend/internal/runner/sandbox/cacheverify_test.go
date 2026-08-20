package sandbox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Doğrulama kararı (spec 027 H5).
 *
 * Docker'sız ve SENTETİK çıktıyla: sorulan şey "hangi artefakt silinir".
 * Bu kararın yanlış olması veri kaybı demek, o yüzden testinin ucuz ve
 * eksiksiz olması gerekiyor — gerçek bir önbellekte bozuk özet dosyası
 * üretmek zor, burada bir satır.
 */

const (
	ozetA = "2fd4e1c67a2d28fced849ee1bb76e7391b93eb12"
	ozetB = "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"
)

func TestParseVerifyOutput_UcAlanliSatirlariOkur(t *testing.T) {
	rows := parseVerifyOutput("/a/x.jar\t" + ozetA + "\t" + ozetA + "\n" +
		"/a/y.jar\t-\t" + ozetB + "\n")

	require.Len(t, rows, 2)
	require.Equal(t, "/a/x.jar", rows[0].Path)
	require.Equal(t, "-", rows[1].Recorded)
}

/*
BOZUK SATIR TARAMAYI DÜŞÜRMEZ.

`find` çalışırken bir koşu dosya yazıyor olabilir. Yarım bir satır yüzünden
bütün taramayı hataya çevirmek, taramayı kullanışsız kılardı.
*/
func TestParseVerifyOutput_BozukSatirlarAtlanir(t *testing.T) {
	rows := parseVerifyOutput("\n" + "eksik-alan\n" + "\t\t\n" +
		"/a/x.jar\t" + ozetA + "\t" + ozetA + "\n")

	require.Len(t, rows, 1)
	require.Equal(t, "/a/x.jar", rows[0].Path)
}

func TestClassifyVerifyRow_UyusanArtefaktSilinmez(t *testing.T) {
	sil, denetlendi := classifyVerifyRow(verifyRow{Recorded: ozetA, Computed: ozetA})
	require.True(t, denetlendi)
	require.False(t, sil)
}

func TestClassifyVerifyRow_UyusmayanArtefaktSilinir(t *testing.T) {
	sil, denetlendi := classifyVerifyRow(verifyRow{Recorded: ozetA, Computed: ozetB})
	require.True(t, denetlendi)
	require.True(t, sil, "özetiyle uyuşmayan artefakt silinmeli")
}

/*
BOZUK VEYA EKSİK ÖZET ASLA SİLMEYE YOL AÇMAZ (spec 027 H5).

Bu testin koruduğu şey doğrudan veri kaybı. Aşağıdaki girdilerin HİÇBİRİ
silmeye yol açmamalı; hepsi "denetlenemedi" sayılmalı.

Özellikle son iki satır: `.sha1` dosyasının iki geçerli biçimi de kabul
ediliyor, ama bir tarafı bozuksa karşılaştırma yapılmıyor — "uyuşmuyor"
demek için ÖNCE ikisinin de okunabilmesi gerekiyor.
*/
func TestClassifyVerifyRow_OkunamayanOzetSilmeyeYolAcmaz(t *testing.T) {
	for ad, row := range map[string]verifyRow{
		"özet dosyası yok":     {Recorded: "-", Computed: ozetA},
		"özet boş":             {Recorded: "", Computed: ozetA},
		"özet kırpılmış":       {Recorded: "2fd4e1c6", Computed: ozetA},
		"özet onaltılık değil": {Recorded: "zzzz" + ozetA[4:], Computed: ozetA},
		"hesaplanamadı":        {Recorded: ozetA, Computed: "-"},
		"ikisi de bozuk":       {Recorded: "x", Computed: "y"},
	} {
		sil, denetlendi := classifyVerifyRow(row)
		require.False(t, sil, "SİLİNMEMELİ: %s", ad)
		require.False(t, denetlendi, "denetlenmiş sayılmamalı: %s", ad)
	}
}

/*
"ÖZET  AD" BİÇİMİ DE KARŞILAŞTIRILIR.

Tarama betiği ilk alanı alıyor; bu test, biçimin Go tarafında da kabul
edildiğinin güvencesi. Yalnız biri kabul edilseydi o artefaktlar sessizce
denetlenemeyenler kutusuna düşerdi.
*/
func TestClassifyVerifyRow_Sha1sumBicimiDeCalisir(t *testing.T) {
	sil, denetlendi := classifyVerifyRow(verifyRow{
		Recorded: ozetA + "  kutuphane-1.0.jar", Computed: ozetA,
	})
	require.True(t, denetlendi)
	require.False(t, sil)
}

/*
npm ÇIKTISI SENTETİK OLARAK AYRIŞTIRILIYOR.

Gerçek çıktı ölçüldü ve biçimi bu; ama "garbage-collected" satırı yalnızca bir
şey toplandığında görünüyor — yani asıl kenar durum, satırın HİÇ OLMAMASI.
*/
func TestParseNPMVerify_SayilariOkur(t *testing.T) {
	sonuc := parseNPMVerify(`Cache verified and compressed (~/.npm/_cacache)
Content verified: 1234 (567890 bytes)
Content garbage-collected: 12 (3456 bytes)
Index entries: 890
Finished in 1.234s`)

	require.Equal(t, 1234, sonuc.Checked)
	require.Equal(t, 12, sonuc.Removed)
	require.Zero(t, sonuc.Mismatched,
		"npm bozulmayı referanssız içerikten AYIRMIYOR; olmayan bir bozulma "+
			"rapor edilmemeli")
}

// Toplanacak bir şey yoksa npm o satırı hiç yazmıyor (ölçüldü).
func TestParseNPMVerify_ToplananYoksaSifir(t *testing.T) {
	sonuc := parseNPMVerify(`Cache verified and compressed (~/.npm/_cacache)
Content verified: 0 (0 bytes)
Index entries: 0
Finished in 0.02s`)

	require.Zero(t, sonuc.Checked)
	require.Zero(t, sonuc.Removed)
}

func TestParseNPMVerify_AnlasilmayanCiktiSifirDoner(t *testing.T) {
	require.Equal(t, VerifyResult{}, parseNPMVerify("npm ERR! bir şeyler ters gitti"))
}
