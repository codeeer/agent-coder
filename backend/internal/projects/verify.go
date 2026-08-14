package projects

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrRepoUnreachable, depoya ulaşılamadı (adres yanlış veya sunucu yanıt vermiyor).
	ErrRepoUnreachable = errors.New("depoya ulaşılamadı")

	// ErrRepoAuth, depo erişimi reddedildi.
	ErrRepoAuth = errors.New("depo erişimi reddedildi")

	// ErrBranchNotFound, belirtilen branch depoda yok.
	ErrBranchNotFound = errors.New("branch depoda bulunamadı")

	// ErrDefaultBranchUnknown, deponun varsayılan branch'i okunamadı.
	//
	// Erişim hatasından AYRI tutuluyor: depoya ulaşılabiliyor ama HEAD'in
	// gösterdiği yer okunamıyor. İkisini karıştırmak, kullanıcıyı yanlış
	// yere — kimlik bilgisine — bakmaya yönlendirirdi.
	ErrDefaultBranchUnknown = errors.New("deponun varsayılan branch'i okunamadı")
)

// verifyTimeout, erişim kontrolünün azami süresi.
const verifyTimeout = 20 * time.Second

// Verifier, depo erişimini sınar.
//
// Kısa ömürlü bir container açmak yerine backend imajındaki git kullanılır:
// `git ls-remote` depo içeriğini indirmeden erişimi ve branch listesini verir.
type Verifier struct{}

// NewVerifier yeni doğrulayıcı üretir.
func NewVerifier() *Verifier { return &Verifier{} }

// Credentials, depo erişimi için kimlik bilgisi.
type Credentials struct {
	Username string
	Secret   string
}

// Verify, depoya erişilebildiğini ve branch'in var olduğunu doğrular.
//
// branch boşsa yalnızca erişim sınanır.
func (v *Verifier) Verify(ctx context.Context, repoURL, branch string, creds *Credentials) error {
	var ekArgs []string
	if branch != "" {
		ekArgs = append(ekArgs, "refs/heads/"+branch)
	}

	out, err := lsRemote(ctx, repoURL, creds, []string{"--heads"}, ekArgs)
	if err != nil {
		return err
	}

	// Branch istendiyse ls-remote onu bulamazsa BOŞ döner ve hata vermez.
	if branch != "" && strings.TrimSpace(out) == "" {
		return fmt.Errorf("%w: %q", ErrBranchNotFound, branch)
	}
	return nil
}

/*
DefaultBranch, deponun HEAD'inin gösterdiği branch'i döner ve AYNI çağrıda
erişimi sınar.

NEDEN KAYNAK SİSTEMİN API'Sİ DEĞİL (spec 021): toplu içe aktarmada erişim
sınaması zaten koşuyor; `--symref` eklemek ek maliyet getirmiyor. Buna karşılık
kaynak sistemin "varsayılan branch" ucu sürümler arasında değişenlerin başında
ve ölçüm yapılacak bir kurumsal sunucumuz yok. Git protokolü sürümden bağımsız
— üstelik klonlama anında geçerli olan cevabı zaten o veriyor.

BOŞ DÖNMEZ: HEAD okunamazsa `ErrDefaultBranchUnknown` döner. `main` varsaymak,
her çalıştırmada patlayacak bir projeyi sessizce kaydetmek olurdu.
*/
func (v *Verifier) DefaultBranch(ctx context.Context, repoURL string, creds *Credentials) (string, error) {
	out, err := lsRemote(ctx, repoURL, creds, []string{"--symref"}, []string{"HEAD"})
	if err != nil {
		return "", err
	}

	branch := symrefBranch(out)
	if branch == "" {
		return "", fmt.Errorf("%w: %q", ErrDefaultBranchUnknown, repoURL)
	}
	return branch, nil
}

/*
lsRemote, ortak ortam ve kimlik hazırlığıyla `git ls-remote` çalıştırır.

TEK YERDE: `GIT_TERMINAL_PROMPT=0` gibi bayraklar güvenlik taşıyor. İki kopya
olsaydı birine eklenen bir bayrak diğerine eklenmeyebilir ve fark ancak
üretimde asılı kalan bir süreçle görülürdü.

`oncekiArgs` URL'den ÖNCE, `sonrakiArgs` sonra gelir — `ls-remote`'un
sözdizimi buna duyarlı.
*/
func lsRemote(ctx context.Context, repoURL string, creds *Credentials,
	oncekiArgs, sonrakiArgs []string) (string, error) {

	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	env := []string{
		"GIT_TERMINAL_PROMPT=0", // parola sorup asılı kalmasın
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME=" + os.TempDir(),
		"PATH=" + os.Getenv("PATH"),
	}

	// Kimlik bilgisi komut satırına veya URL'e YAZILMAZ: ikisi de süreç
	// listesinde ve loglarda görünür. GIT_ASKPASS ile geçirilir.
	if creds != nil && creds.Secret != "" {
		askpass, cleanup, err := WriteAskpass(creds.Secret)
		if err != nil {
			return "", fmt.Errorf("kimlik bilgisi hazırlanamadı: %w", err)
		}
		defer cleanup()

		env = append(env,
			"GIT_ASKPASS="+askpass,
			"GIT_USERNAME="+creds.Username,
		)
		repoURL = InjectUsername(repoURL, creds.Username)
	}

	args := append([]string{"ls-remote"}, oncekiArgs...)
	args = append(args, repoURL)
	args = append(args, sonrakiArgs...)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", classifyGitError(ctx, string(out), creds != nil)
	}
	return string(out), nil
}

/*
symrefBranch, `ls-remote --symref` çıktısındaki HEAD satırından branch adını
çıkarır.

Beklenen biçim:

	ref: refs/heads/develop	HEAD
	9f1a2b3c…	HEAD

`ref:` satırı yoksa BOŞ döner — çağıran hata üretir. Varsayılan uydurmak
ürünün en sert kuralını çiğnerdi.
*/
func symrefBranch(out string) string {
	const onek = "ref: refs/heads/"

	for _, satir := range strings.Split(out, "\n") {
		satir = strings.TrimSpace(satir)
		if !strings.HasPrefix(satir, onek) {
			continue
		}
		// Satırın kalanı "<branch>\tHEAD" — sekmeden önceki parça ad.
		kalan := strings.TrimPrefix(satir, onek)
		if i := strings.IndexAny(kalan, " \t"); i >= 0 {
			kalan = kalan[:i]
		}
		if kalan != "" {
			return kalan
		}
	}
	return ""
}

// WriteAskpass, parolayı stdout'a yazan geçici bir betik üretir.
//
// git bu betiği çağırıp çıktısını parola olarak kullanır; değer komut satırına
// veya ortam değişkenine doğrudan girmez. Hem doğrulama hem push kullanır.
func WriteAskpass(secret string) (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "agent-coder-askpass-")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	path = filepath.Join(dir, "askpass.sh")

	// Parola betiğe gömülür; dosya yalnızca sahibine okunur ve hemen silinir.
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  *[Uu]sername*) printf '%s' \"$GIT_USERNAME\" ;;\n" +
		"  *) cat <<'AGENT_CODER_SECRET_EOF'\n" + secret + "\nAGENT_CODER_SECRET_EOF\n ;;\n" +
		"esac\n"

	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

// InjectUsername, adrese yalnızca KULLANICI ADINI koyar (parolayı değil).
// git böylece hangi kullanıcı için parola isteyeceğini bilir.
func InjectUsername(repoURL, username string) string {
	if username == "" {
		return repoURL
	}
	for _, scheme := range []string{"https://", "http://"} {
		if strings.HasPrefix(repoURL, scheme) {
			return scheme + username + "@" + strings.TrimPrefix(repoURL, scheme)
		}
	}
	return repoURL
}

// classifyGitError, git çıktısını kullanıcının anlayacağı hataya çevirir.
//
// hasCredentials, kimlik bilgisi verilip verilmediği. Bu ayrım önemli:
// GitHub ve Bitbucket, var olmayan bir depoyu gizlemek için kasten 401 döner.
// Kimlik bilgisi HİÇ verilmemişken "erişim reddedildi" demek kullanıcıyı
// yanlış yere baktırır — asıl olasılık adresin yanlış olmasıdır.
func classifyGitError(ctx context.Context, output string, hasCredentials bool) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w: zaman aşımı", ErrRepoUnreachable)
	}

	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "authentication failed"),
		strings.Contains(lower, "could not read username"),
		strings.Contains(lower, "invalid username or password"),
		strings.Contains(lower, "403"):
		if !hasCredentials {
			return fmt.Errorf("%w: depo bulunamadı veya özel — adresi kontrol edin, "+
				"özelse ayarlardan bir git erişimi tanımlayıp projeye bağlayın",
				ErrRepoUnreachable)
		}
		return ErrRepoAuth

	case strings.Contains(lower, "repository not found"),
		strings.Contains(lower, "not found"),
		strings.Contains(lower, "does not exist"):
		// Var olmayan özel depolar da "not found" der; kullanıcıya ikisini birden söyle.
		return fmt.Errorf("%w: depo bulunamadı veya erişim yetkiniz yok", ErrRepoUnreachable)

	case strings.Contains(lower, "could not resolve host"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "timed out"):
		return fmt.Errorf("%w: sunucuya bağlanılamadı", ErrRepoUnreachable)
	}

	return fmt.Errorf("%w: %s", ErrRepoUnreachable, firstLine(output))
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			if len(l) > 200 {
				return l[:200]
			}
			return l
		}
	}
	return "bilinmeyen hata"
}
