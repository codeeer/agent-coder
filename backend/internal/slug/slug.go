// Package slug, kullanıcının verdiği adlardan makine kimliği üretir.
//
// İki yerde kullanılır — LLM sağlayıcı kimlikleri ve agent kısa adları — ve
// kuralların ayrışmaması için tek yerde tutulur.
package slug

import (
	"regexp"
	"strings"
)

// MaxLength, üretilen kimliğin azami uzunluğu.
const MaxLength = 48

var (
	invalid = regexp.MustCompile(`[^a-z0-9]+`)
	edges   = regexp.MustCompile(`^-+|-+$`)
	// gecerli, dosyadaki diğer iki desenle aynı yerde: `Valid` her çağrıda
	// yeniden derliyordu ve bu fonksiyon liste ekranlarında kayıt başına
	// çağrılıyor.
	gecerli = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
)

// turkishFolding, Türkçe harflerin ASCII karşılıkları.
//
// strings.ToLower tek başına yetmez: "İ" birleşik bir karaktere dönüşür ve
// "ş", "ğ" gibi harfler ASCII'ye hiç düşmez — sessizce atılırlar.
// "Şirket" → "irket" gibi anlamsız kimlikler bu yüzden oluşur.
var turkishFolding = strings.NewReplacer(
	"ç", "c", "Ç", "c",
	"ğ", "g", "Ğ", "g",
	"ı", "i", "I", "i",
	"İ", "i", "i̇", "i",
	"ö", "o", "Ö", "o",
	"ş", "s", "Ş", "s",
	"ü", "u", "Ü", "u",
)

// Make, bir adı küçük harf, rakam ve tireden oluşan kimliğe çevirir.
//
// fallback, ad hiçbir kullanılabilir karakter içermediğinde döner —
// kimliksiz kayıt oluşamaz.
func Make(name, fallback string) string {
	s := turkishFolding.Replace(name)
	s = strings.ToLower(s)
	s = invalid.ReplaceAllString(s, "-")
	s = edges.ReplaceAllString(s, "")

	if s == "" {
		return fallback
	}
	if len(s) > MaxLength {
		s = strings.TrimRight(s[:MaxLength], "-")
	}
	return s
}

// Valid, bir kimliğin biçimsel olarak geçerli olup olmadığı.
func Valid(s string) bool {
	if s == "" || len(s) > MaxLength {
		return false
	}
	return gecerli.MatchString(s)
}
