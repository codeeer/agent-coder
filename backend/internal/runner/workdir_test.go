package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestProjectDir_YerlesimRoot, varsayılan yerleşimde adresin HİÇ okunmadığını
// gösterir. Spec 025 H2: ayara dokunulmamış kurulumda davranış değişmez.
func TestProjectDir_YerlesimRoot(t *testing.T) {
	for _, url := range []string{
		"https://github.com/kurum/proje.git",
		"git@github.com:kurum/proje.git",
		"",
		"../../etc",
	} {
		require.Equal(t, "/work", ProjectDir(LayoutRoot, url),
			"root yerleşiminde adres ne olursa olsun kök döner: %q", url)
	}
}

// TestProjectDir_TanimsizYerlesim, tanınmayan bir değerin köke düştüğünü
// gösterir. Ayar bozuk diye çalıştırma DÜŞMEZ (spec 025 H2).
func TestProjectDir_TanimsizYerlesim(t *testing.T) {
	require.Equal(t, "/work", ProjectDir("", "https://h/x/proje.git"))
	require.Equal(t, "/work", ProjectDir("saçmalık", "https://h/x/proje.git"))
}

func TestProjectDir_RepoAdiTuretme(t *testing.T) {
	tests := []struct {
		ad      string
		repoURL string
		beklsen string
	}{
		// ── Yaygın biçimler ────────────────────────────────────────────
		{"https .git ile", "https://github.com/kurum/proje.git", "/work/proje"},
		{"https .git olmadan", "https://github.com/kurum/proje", "/work/proje"},
		{"http", "http://git.local/kurum/proje.git", "/work/proje"},
		{"sonu ayraçla biten", "https://github.com/kurum/proje.git/", "/work/proje"},
		{"sonu ayraçla, .git yok", "https://github.com/kurum/proje/", "/work/proje"},
		{"birden çok sondaki ayraç", "https://github.com/kurum/proje//", "/work/proje"},

		// ── SSH ────────────────────────────────────────────────────────
		{"kısa ssh", "git@github.com:kurum/proje.git", "/work/proje"},
		{"kısa ssh, alt gruba sahip", "git@bitbucket.org:takim/alt/proje.git", "/work/proje"},
		{"kısa ssh, ayraçsız", "git@github.com:proje.git", "/work/proje"},
		{"ssh şeması", "ssh://git@github.com/kurum/proje.git", "/work/proje"},
		{"ssh şeması, portlu", "ssh://git@git.local:2222/kurum/proje.git", "/work/proje"},

		// ── Port taşıyan https: iki nokta ADI BOZMAMALI ────────────────
		{"https portlu", "https://git.local:8443/kurum/proje.git", "/work/proje"},

		// ── Ad içinde nokta ve tire ────────────────────────────────────
		{"nokta içeren ad", "https://h/k/proje.api.git", "/work/proje.api"},
		{"tire içeren ad", "https://h/k/agent-coder.git", "/work/agent-coder"},

		// ── Ad İÇİNDE iki nokta: sınır sayılmamalı ─────────────────────
		// Ayraç varken iki nokta kesilirse `pro:je` → `je` olurdu.
		{"adda iki nokta", "https://h/k/pro:je.git", "/work/pro:je"},
		{"adda çift iki nokta", "https://h/k/a:b:c.git", "/work/a:b:c"},

		// ── Sorgu ve parça `.git` soyulmasını engellememeli ────────────
		{"sorgu dizesi", "https://h/k/proje.git?ref=main", "/work/proje"},
		{"parça", "https://h/k/proje.git#v1", "/work/proje"},
		{"sorgu, .git yok", "https://h/k/proje?ref=main", "/work/proje"},

		// ── Türetilemeyen: KÖKE DÜŞER, hata vermez ─────────────────────
		{"boş adres", "", "/work"},
		{"yalnızca boşluk", "   ", "/work"},
		{"yalnızca ayraç", "///", "/work"},
		{"yalnızca .git", "https://h/k/.git", "/work"},
		{"nokta", "https://h/k/.", "/work"},

		// ── Path traversal: KÖKÜN DIŞINA ÇIKAMAZ (spec 025 H3) ─────────
		{"üst dizin", "https://h/k/..", "/work"},
		// `...git` → `.git` soyulunca `..` kalır. Adres masum görünüyor ama
		// türetilen ad üst dizin; reddedilmeli.
		{"üst dizin, .git soyulunca ortaya çıkan", "https://h/k/...git", "/work"},
		{"ters ayraç", "https://h/k/..\\..\\etc.git", "/work"},
		{"NUL bayt", "https://h/k/pro\x00je.git", "/work"},

		/*
		 * ÇOK UZUN AD — köke düşer, çalıştırmayı düşürmez.
		 *
		 * Tek dizin adı 255 baytı aşarsa `git clone` ENAMETOOLONG ile düşer.
		 * Aynı depo varsayılan yerleşimde sorunsuz klonlandığı için, ayarı
		 * açmanın çalışan bir kurulumu bozmaması gerekiyor (spec 025 H2).
		 */
		{"255 bayt sınırında", "https://h/k/" + strings.Repeat("a", 255) + ".git",
			"/work/" + strings.Repeat("a", 255)},
		{"255 baytı aşan", "https://h/k/" + strings.Repeat("a", 256) + ".git", "/work"},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			require.Equal(t, tt.beklsen, ProjectDir(LayoutRepo, tt.repoURL))
		})
	}
}

/*
TestProjectDir_KokunAltindaKalir, üretilen her yolun kökün DOĞRUDAN altında
kaldığını sınar.

Tablodaki tek tek beklentilerden AYRI duruyor: orada "bu adres şu adı verir"
denir, burada "hangi adres verilirse verilsin sınır aşılmaz" denir. İlki
regresyon, ikincisi güvenlik kriteri (spec 025 H3) — biri diğerinin yerine
geçmez.
*/
func TestProjectDir_KokunAltindaKalir(t *testing.T) {
	kotu := []string{
		"https://h/k/../../../etc/passwd",
		"https://h/k/..%2f..%2fetc",
		"git@h:../../kacis.git",
		"https://h/k/a/b/c/../../../../../../tmp.git",
		"/etc/passwd",
		"..",
		"../",
		"....//....//etc",
		"https://h/k/" + strings.Repeat("u", 400) + ".git",
		"https://h/k/pro\x00je.git",
		"https://h/k/..\\..\\etc.git",
		"file:///etc/shadow",
		"https://h/k/.",
	}

	for _, url := range kotu {
		dir := ProjectDir(LayoutRepo, url)

		if dir == WorkRoot {
			continue // köke düşmüş; sınır zaten aşılmadı
		}

		/*
		 * İDDİA UYGULAMADAN BAĞIMSIZ KURULUYOR.
		 *
		 * `path.Dir(dir) == WorkRoot` demek, uygulamanın kendi koruma
		 * ifadesini tekrar etmek olurdu: koruma kaldırılmadıkça test asla
		 * düşmezdi. Bunun yerine üretilen ADIN kendisi sınanıyor — kökten
		 * sonra gelen parça tek bir bileşen mi, içinde ayraç veya üst dizin
		 * ifadesi var mı.
		 */
		ad := strings.TrimPrefix(dir, WorkRoot+"/")
		require.NotEqual(t, dir, ad, "yol kökün altında olmalı: %q → %q", url, dir)
		require.NotContains(t, ad, "/", "ad ayraç içeremez: %q → %q", url, ad)
		require.NotContains(t, ad, `\`, "ad ters ayraç içeremez: %q → %q", url, ad)
		require.NotContains(t, ad, "\x00", "ad NUL içeremez: %q → %q", url, ad)
		require.NotEqual(t, "..", ad, "ad üst dizin olamaz: %q", url)
		require.NotEqual(t, ".", ad, "ad geçerli bir dizin adı olmalı: %q", url)
		require.LessOrEqual(t, len(ad), 255, "ad dosya sistemi sınırını aşamaz: %q", url)
	}
}
