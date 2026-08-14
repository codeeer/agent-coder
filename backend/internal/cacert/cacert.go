// Package cacert, kurumsal kök sertifikanın nereden geldiğini çözer.
//
// İKİ KAYNAK VAR ve sırası önemlidir:
//
//  1. Ayar (arayüzden girilen) — kazanır
//  2. Ortam değişkeniyle verilen dosya yolu — yedek
//
// Yedek KALDIRILMADI çünkü kaldırmak, güncelleyen kurumsal kurulumların
// sertifikasını sessizce devre dışı bırakırdı; bunu ancak ilk başarısız
// çalıştırmada fark ederlerdi (spec 017: Belirsizlikler).
//
// Çözülen değer HER ZAMAN normalleştirilmiş PEM'dir. Kullanıcı DER veya
// PKCS#7 vermiş olabilir; sertifikayı tüketen her yer yalnızca PEM okuyor.
package cacert

import (
	"log/slog"
	"os"
	"strings"

	"github.com/agent-coder/backend/internal/certfmt"
)

// Source, geçerli sertifikanın nereden geldiği.
//
// Kullanıcıya SÖYLENİR: iki kaynak birden mümkünken "tanımlı" demek yetmiyor,
// hangisinin geçerli olduğu yazılmalı (spec 017: Davranış kuralları).
type Source string

const (
	SourceSettings Source = "settings"
	SourceEnv      Source = "env"
	SourceNone     Source = "none"
)

// Resolver, geçerli sertifikayı üretir.
type Resolver struct {
	fromSettings func() string
	// envPEM, ortam değişkeniyle verilen dosyanın normalleştirilmiş içeriği.
	//
	// AÇILIŞTA BİR KEZ okunur: dosya host'ta duruyor ve yeniden başlatmadan
	// değişmesi beklenmiyor — canlı değişebilen kaynak ayar tarafı. Her
	// çağrıda diske gitmek, giden HER HTTP isteğinde dosya okumak demekti.
	envPEM string
}

// NewResolver, çözümleyiciyi kurar.
//
// envPath boşsa yedek kaynak yoktur. Dosya okunamaz veya sertifika içermiyorsa
// hata DÖNDÜRÜLMEZ: yanlış yapılandırılmış bir yedek yüzünden sunucunun hiç
// açılmaması, ayarı arayüzden düzeltme imkânını da ortadan kaldırırdı. Durum
// loglanır ve kaynak yokmuş gibi devam edilir.
func NewResolver(fromSettings func() string, envPath string) *Resolver {
	r := &Resolver{fromSettings: fromSettings}
	if envPath == "" {
		return r
	}

	raw, err := os.ReadFile(envPath)
	if err != nil {
		slog.Warn("kurumsal sertifika dosyası okunamadı — yedek kaynak devre dışı",
			"path", envPath, "error", err)
		return r
	}
	pemText, err := certfmt.ToPEM(raw)
	if err != nil {
		slog.Warn("kurumsal sertifika dosyası geçerli bir sertifika içermiyor",
			"path", envPath, "error", err)
		return r
	}
	r.envPEM = pemText
	slog.Info("kurumsal sertifika ortam değişkeninden yüklendi", "path", envPath)
	return r
}

// Resolve, geçerli sertifikayı ve kaynağını verir.
//
// Ayardaki değer her çağrıda okunur — ayar değişince yeniden başlatma
// gerekmesin diye (spec 017 H1, H5).
func (r *Resolver) Resolve() (string, Source) {
	if r.fromSettings != nil {
		if raw := strings.TrimSpace(r.fromSettings()); raw != "" {
			pemText, err := certfmt.ToPEM([]byte(raw))
			if err != nil {
				// Ayar kaydedilirken doğrulanıyor; buraya düşmek bir tutarsızlık.
				// Yine de sunucu çökmez: yedeğe düşülür.
				slog.Warn("ayardaki kurumsal sertifika çözülemedi", "error", err)
			} else {
				return pemText, SourceSettings
			}
		}
	}
	if r.envPEM != "" {
		return r.envPEM, SourceEnv
	}
	return "", SourceNone
}

// PEM, yalnızca sertifikayı verir. Kaynağı önemsemeyen çağıranlar için.
func (r *Resolver) PEM() string {
	pemText, _ := r.Resolve()
	return pemText
}

// String, log ve hata mesajları için.
func (s Source) String() string {
	switch s {
	case SourceSettings:
		return "ayarlardan"
	case SourceEnv:
		return "ortam değişkeninden"
	default:
		return "tanımsız"
	}
}
