package runs

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/projects"
)

// pushTimeout, gönderim işleminin azami süresi.
const pushTimeout = 2 * time.Minute

// PushRequest, bir çalıştırmanın değişikliklerini branch'e gönderme isteği.
type PushRequest struct {
	Run    Run
	Repo   string
	Creds  *projects.Credentials
	Branch string
}

// Pusher, çalıştırma diff'ini yeni bir branch'e gönderir.
//
// Çalıştırma container'ı iş biter bitmez silindiği için depo yeniden
// klonlanır ve kaydedilmiş diff uygulanır. Alternatifi container'ı push
// ihtimaline karşı ayakta tutmaktı; onlarca çalıştırmadan sonra disk dolardı.
type Pusher struct {
	store *Store
}

// NewPusher yeni gönderici üretir.
func NewPusher(store *Store) *Pusher {
	return &Pusher{store: store}
}

// SuggestBranch, çalıştırmadan bir branch adı önerir.
func SuggestBranch(r Run) string {
	short := r.ID.String()
	if len(short) > 8 {
		short = short[:8]
	}
	return fmt.Sprintf("agent-coder/%s-%s", r.AgentSlug, short)
}

/*
 * PushResult, gönderimin sonucu.
 *
 * SkippedBinaries BOŞ BIRAKILMAZ, taşınır: uygulanamayan ikili blokları sessizce
 * atmak, kullanıcıya eksik bir branch'i tam gibi göstermek olurdu.
 */
type PushResult struct {
	Branch string
	// SkippedBinaries, yamada veri taşımadığı için gönderilemeyen dosyalar.
	SkippedBinaries []string
}

// Push, diff'i yeni bir branch olarak depoya gönderir.
//
// Aynı çalıştırma ikinci kez gönderilemez: iki farklı branch'in aynı işten
// çıkması karışıklık yaratır ve kullanıcı hangisinin geçerli olduğunu bilemez.
func (p *Pusher) Push(ctx context.Context, req PushRequest) (PushResult, error) {
	if !req.Run.HasChanges() {
		return PushResult{}, ErrNoChanges
	}
	if req.Run.PushedBranch != nil && *req.Run.PushedBranch != "" {
		return PushResult{}, fmt.Errorf("%w: %s", ErrAlreadyPushed, *req.Run.PushedBranch)
	}
	if req.Creds == nil || req.Creds.Secret == "" {
		return PushResult{}, ErrNoGitAccess
	}

	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = SuggestBranch(req.Run)
	}

	ctx, cancel := context.WithTimeout(ctx, pushTimeout)
	defer cancel()

	dir, err := os.MkdirTemp("", "agent-coder-push-")
	if err != nil {
		return PushResult{}, fmt.Errorf("geçici dizin oluşturulamadı: %w", err)
	}
	defer os.RemoveAll(dir)

	askpass, cleanupAskpass, err := projects.WriteAskpass(req.Creds.Secret)
	if err != nil {
		return PushResult{}, fmt.Errorf("kimlik bilgisi hazırlanamadı: %w", err)
	}
	defer cleanupAskpass()

	env := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"HOME=" + dir,
		"PATH=" + os.Getenv("PATH"),
		"GIT_ASKPASS=" + askpass,
		"GIT_USERNAME=" + req.Creds.Username,
		"GIT_AUTHOR_NAME=Agent Coder",
		"GIT_AUTHOR_EMAIL=agent-coder@localhost",
		"GIT_COMMITTER_NAME=Agent Coder",
		"GIT_COMMITTER_EMAIL=agent-coder@localhost",
	}

	repoURL := projects.InjectUsername(req.Repo, req.Creds.Username)
	work := filepath.Join(dir, "repo")

	run := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Env = env
		cmd.Dir = work
		if args[0] == "clone" {
			cmd.Dir = dir
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %s", args[0], maskURL(firstLines(string(out), 3)))
		}
		return nil
	}

	if err := run("clone", "--depth", "1", "--branch", req.Run.Branch, repoURL, work); err != nil {
		return PushResult{}, err
	}
	if err := run("checkout", "-b", branch); err != nil {
		return PushResult{}, err
	}

	// Diff dosyaya yazılıp uygulanır; boru hattı yerine dosya kullanmak
	// hata mesajlarını okunabilir tutuyor.
	/*
	 * UYGULANAMAZ İKİLİ BLOKLAR AYIKLANIYOR.
	 *
	 * opencode'un diff'i ikili dosyalar için yalnızca "Binary files … differ"
	 * yazıyor — yük yok. `git apply` bunu uygulayamıyor ve TEK blok tüm yamayı
	 * düşürüyordu: dokuz dosyanın yedisi düzgün metin olduğu hâlde hiçbiri
	 * gönderilemiyordu (ölçüldü, `mvn` çalıştıran bir koşuda).
	 *
	 * Atılan blokta uygulanacak veri zaten yok; ama atlananlar çağırana
	 * bildiriliyor ve kullanıcıya yazılıyor.
	 */
	temizDiff, atlanan := stripUnappliableBinary(req.Run.Diff)
	if strings.TrimSpace(temizDiff) == "" {
		return PushResult{}, fmt.Errorf(
			"%w: değişikliklerin tamamı yamada veri taşımayan ikili dosya (%s)",
			ErrNoChanges, strings.Join(atlanan, ", "))
	}

	patch := filepath.Join(dir, "changes.patch")
	if err := os.WriteFile(patch, []byte(ensureTrailingNewline(temizDiff)), 0o600); err != nil {
		return PushResult{}, fmt.Errorf("diff yazılamadı: %w", err)
	}

	// --3way: hedef branch çalıştırmadan sonra ilerlediyse birleştirmeyi dener.
	if err := run("apply", "--3way", "--whitespace=nowarn", patch); err != nil {
		return PushResult{}, fmt.Errorf("değişiklikler uygulanamadı (depo bu arada değişmiş olabilir): %w", err)
	}

	if err := run("add", "-A"); err != nil {
		return PushResult{}, err
	}

	message := fmt.Sprintf("%s: %s", req.Run.AgentSlug, firstLines(req.Run.Task, 1))
	if err := run("commit", "-m", message); err != nil {
		return PushResult{}, err
	}

	if err := run("push", "-u", "origin", branch); err != nil {
		return PushResult{}, err
	}

	if err := p.store.SetPushedBranch(context.WithoutCancel(ctx), req.Run.ID, branch); err != nil {
		// Push başarılı ama kayıt tutulamadı: kullanıcıya branch adını yine
		// söyleriz, aksi halde iki kez göndermeye çalışır.
		return PushResult{Branch: branch, SkippedBinaries: atlanan},
			fmt.Errorf("branch gönderildi (%s) ama kayıt güncellenemedi: %w", branch, err)
	}
	return PushResult{Branch: branch, SkippedBinaries: atlanan}, nil
}

func ensureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func firstLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	out := strings.Join(lines, " ")
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}

// maskURL, hata metnindeki kimlik bilgilerini gizler.
func maskURL(s string) string {
	for {
		at := strings.Index(s, "@")
		scheme := strings.Index(s, "://")
		if at < 0 || scheme < 0 || at < scheme {
			return s
		}
		s = s[:scheme+3] + "***@" + s[at+1:]
		return s
	}
}

// ResolveRunID, metin kimliği UUID'e çevirir.
func ResolveRunID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
