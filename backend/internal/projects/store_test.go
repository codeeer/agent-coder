package projects

import "testing"

/*
 * Adres normalizasyonu — mükerrer denetiminin kalbi.
 *
 * NEDEN GEREKLİ: `projects.repo_url` üzerinde unique kısıt YOK. Gruba yeni
 * repository eklendikçe içe aktarma tekrar çalıştırılacak; denetim olmasaydı
 * her tekrar kopya üretirdi.
 *
 * TAMAMI KÜÇÜK HARFE ÇEVRİLİYOR ve bu bilinçli bir ödünç. Gerçek senaryo şu:
 * kullanıcı bir repository'yi elle `…/scm/odeme/api.git` diye eklemiş, kaynak
 * ise `…/scm/ODEME/api.git` veriyor. Yalnızca host'u küçültseydik ikisi ayrı
 * sayılır ve kopya oluşurdu. Aynı sunucuda yalnızca harf büyüklüğüyle ayrılan
 * iki farklı repository ise pratikte yok — git barındırma sistemleri bu adları
 * zaten büyük/küçük harf duyarsız ele alıyor.
 */

func TestNormalizeRepoURL_SondakiGitVeEgikCizgiAtilir(t *testing.T) {
	esitler := []string{
		"https://bb.sirket.com/scm/ODEME/api.git",
		"https://bb.sirket.com/scm/ODEME/api",
		"https://bb.sirket.com/scm/ODEME/api/",
		"https://bb.sirket.com/scm/ODEME/api.git/",
	}

	ilk := NormalizeRepoURL(esitler[0])
	for _, a := range esitler[1:] {
		if got := NormalizeRepoURL(a); got != ilk {
			t.Errorf("%q → %q, beklenen %q", a, got, ilk)
		}
	}
}

func TestNormalizeRepoURL_HarfBuyuklugu(t *testing.T) {
	a := NormalizeRepoURL("https://BB.Sirket.COM/scm/ODEME/api.git")
	b := NormalizeRepoURL("https://bb.sirket.com/scm/odeme/api")

	if a != b {
		t.Errorf("harf büyüklüğü farkı aynı anahtara düşmeli: %q vs %q", a, b)
	}
}

// Farklı depolar birbirine karışmaz — normalizasyon fazla birleştirmemeli.
func TestNormalizeRepoURL_FarkliDepolarAyriKalir(t *testing.T) {
	farklilar := []string{
		"https://bb.sirket.com/scm/ODEME/api.git",
		"https://bb.sirket.com/scm/ODEME/apix.git",
		"https://bb.sirket.com/scm/IK/api.git",
		"https://baska.sirket.com/scm/ODEME/api.git",
	}

	gorulen := map[string]string{}
	for _, a := range farklilar {
		k := NormalizeRepoURL(a)
		if onceki, varsa := gorulen[k]; varsa {
			t.Errorf("%q ve %q aynı anahtara düştü: %q", onceki, a, k)
		}
		gorulen[k] = a
	}
}

// Ayrıştırılamayan adres kaybolmaz: kendi metni anahtar olur.
func TestNormalizeRepoURL_AyristirilamayanKorunur(t *testing.T) {
	if NormalizeRepoURL("bu bir adres değil") == "" {
		t.Error("ayrıştırılamayan adres boş anahtara düşmemeli")
	}
}
