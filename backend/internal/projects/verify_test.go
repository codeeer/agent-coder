package projects

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Varsayılan branch'in okunması.
 *
 * NEDEN GIT'TEN, BITBUCKET'TAN DEĞİL (spec 021 → plan): erişim sınaması zaten
 * `ls-remote` koşuyor, `--symref` eklemek bedava. Ayrıca Bitbucket'ın varsayılan
 * branch ucu sürümler arasında değişenlerin başında ve elimizde ölçüm
 * yapılacak bir kurumsal sunucu yok. Git protokolü ise sunucu sürümünden
 * bağımsız — ve klonlama anında geçerli olan cevabı zaten o veriyor.
 */

func TestSymrefBranch_OkurVeAyristirir(t *testing.T) {
	cikti := "ref: refs/heads/develop\tHEAD\n" +
		"9f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f90\tHEAD\n"

	require.Equal(t, "develop", symrefBranch(cikti))
}

// Eğik çizgili branch adları bozulmadan çıkar.
func TestSymrefBranch_EgikCizgiliAd(t *testing.T) {
	require.Equal(t, "release/2026.1",
		symrefBranch("ref: refs/heads/release/2026.1\tHEAD\n"))
}

/*
 * ÇIKTIDA `ref:` SATIRI YOKSA BOŞ DÖNER — "main" UYDURULMAZ.
 *
 * Ürünün en sert kuralı: ölçülmeyen hiçbir şey yazılmaz. Boş dönmek, çağıranın
 * hata üretmesini sağlar; `main` dönmek ise yanlış branch'le kaydedilmiş ve
 * her çalıştırmada patlayacak bir proje üretirdi.
 */
func TestSymrefBranch_RefSatiriYoksaBos(t *testing.T) {
	require.Empty(t, symrefBranch(""))
	require.Empty(t, symrefBranch("9f1a2b3c\tHEAD\n"))
	require.Empty(t, symrefBranch("ref: refs/remotes/origin/main\tHEAD\n"))
}

// yerelDepo, gerçek bir git deposu üretir ve `file://` adresini döner.
// Sahte çıktı yerine gerçek git kullanılıyor: ayrıştırdığımız biçimin git'in
// ürettiği biçim olduğunu ancak bu doğrular.
func yerelDepo(t *testing.T, branch string) string {
	t.Helper()

	dizin := t.TempDir()
	calistir := func(arg ...string) {
		t.Helper()
		cmd := exec.Command("git", arg...)
		cmd.Dir = dizin
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	calistir("init", "-b", branch)
	calistir("commit", "--allow-empty", "-m", "ilk")
	return "file://" + dizin
}

/*
 * `Verify` için koruma testleri.
 *
 * Bu testler yeni davranış eklemiyor; ortak `ls-remote` hazırlığı iki çağıran
 * arasında paylaşılacağı için MEVCUT davranışı kilitliyorlar. Testsiz bir
 * refactor, çalıştığını iddia edip kanıtlayamaz.
 */
func TestVerify_ErisilebilenDepo(t *testing.T) {
	err := NewVerifier().Verify(context.Background(), yerelDepo(t, "main"), "", nil)
	require.NoError(t, err)
}

func TestVerify_VarOlanBranch(t *testing.T) {
	err := NewVerifier().Verify(context.Background(), yerelDepo(t, "develop"), "develop", nil)
	require.NoError(t, err)
}

func TestVerify_OlmayanBranch(t *testing.T) {
	err := NewVerifier().Verify(context.Background(), yerelDepo(t, "main"), "yok", nil)
	require.ErrorIs(t, err, ErrBranchNotFound)
}

func TestVerify_UlasilamayanDepo(t *testing.T) {
	err := NewVerifier().Verify(context.Background(), "file:///olmayan/depo.git", "", nil)
	require.Error(t, err)
}

func TestDefaultBranch_GercekDepodanOkunur(t *testing.T) {
	v := NewVerifier()

	branch, err := v.DefaultBranch(context.Background(), yerelDepo(t, "develop"), nil)

	require.NoError(t, err)
	require.Equal(t, "develop", branch)
}

// Aynı çağrı erişimi de sınıyor: olmayan bir depo hata döner ve mevcut
// sınıflandırma kullanılır.
func TestDefaultBranch_UlasilamayanDepo(t *testing.T) {
	v := NewVerifier()

	_, err := v.DefaultBranch(context.Background(), "file:///olmayan/dizin/depo.git", nil)

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrDefaultBranchUnknown,
		"erişim hatası, branch bilinmiyor hatasıyla karıştırılmamalı")
}
