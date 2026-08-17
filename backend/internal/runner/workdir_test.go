package runner

import (
	"path"
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
	}

	for _, url := range kotu {
		dir := ProjectDir(LayoutRepo, url)
		require.True(t, dir == WorkRoot || path.Dir(dir) == WorkRoot,
			"yol kökün doğrudan altında olmalı: %q → %q", url, dir)
	}
}
