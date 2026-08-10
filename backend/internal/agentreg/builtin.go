// Package agentreg, agent tanımlarını yönetir.
//
// Agent tanımlarının tek yaşadığı yer veritabanıdır; kaynağı ise bu pakete
// gömülü hazır tanımlardır. Runner imajında agent dosyası bulunmaz — her
// çalıştırmada tanım veritabanından okunup container'a kopyalanır.
package agentreg

import (
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
)

// builtinFS, hazır agent tanımlarını Go ikilisine gömer.
//
//go:embed builtin/*.md
var builtinFS embed.FS

// Builtin, gömülü bir hazır agent tanımı.
type Builtin struct {
	Slug        string
	Name        string
	Description string
	Prompt      string

	AllowEdit     bool
	AllowBash     bool
	AllowWebfetch bool
}

// Builtins, gömülü tanımları slug sırasıyla döner.
//
// Ayrıştırma hatası programlama hatasıdır (dosyalar ikiliye gömülü) — panik eder,
// çünkü bozuk bir hazır agent'la çalışmaya devam etmek sessiz bir arızadır.
func Builtins() []Builtin {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		panic(fmt.Sprintf("agentreg: gömülü agent'lar okunamadı: %v", err))
	}

	out := make([]Builtin, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := builtinFS.ReadFile(path.Join("builtin", e.Name()))
		if err != nil {
			panic(fmt.Sprintf("agentreg: %s okunamadı: %v", e.Name(), err))
		}

		b, err := parseBuiltin(strings.TrimSuffix(e.Name(), ".md"), string(data))
		if err != nil {
			panic(fmt.Sprintf("agentreg: %s ayrıştırılamadı: %v", e.Name(), err))
		}
		out = append(out, b)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// parseBuiltin, markdown frontmatter'ını ve gövdesini okur.
//
// Tam bir YAML ayrıştırıcısı kullanılmıyor: dosyalar bizim yazdığımız, biçimi
// sabit ve alanları basit. Bağımlılık eklemeye değmez.
func parseBuiltin(slug, content string) (Builtin, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	if !strings.HasPrefix(content, "---\n") {
		return Builtin{}, fmt.Errorf("frontmatter bulunamadı")
	}
	rest := content[len("---\n"):]

	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return Builtin{}, fmt.Errorf("frontmatter kapanmamış")
	}

	front := rest[:end]
	body := strings.TrimSpace(rest[end+len("\n---\n"):])
	if body == "" {
		return Builtin{}, fmt.Errorf("talimat gövdesi boş")
	}

	b := Builtin{
		Slug:   slug,
		Name:   titleFromSlug(slug),
		Prompt: body,
		// Varsayılan yetkiler; frontmatter'daki permission bloğu bunları daraltır.
		AllowEdit:     true,
		AllowBash:     true,
		AllowWebfetch: false,
	}

	inPermission := false
	for _, line := range strings.Split(front, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// permission bloğunun altındaki girintili satırlar.
		if inPermission && strings.HasPrefix(line, " ") {
			key, val, ok := splitKeyValue(line)
			if !ok {
				continue
			}
			allowed := val != "deny"
			switch key {
			case "edit", "write":
				b.AllowEdit = b.AllowEdit && allowed
			case "bash":
				b.AllowBash = allowed
			case "webfetch":
				b.AllowWebfetch = allowed
			}
			continue
		}
		inPermission = false

		key, val, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		switch key {
		case "description":
			b.Description = strings.Trim(val, `"`)
		case "permission":
			inPermission = true
		}
	}

	if b.Description == "" {
		return Builtin{}, fmt.Errorf("description alanı boş")
	}
	return b, nil
}

func splitKeyValue(line string) (key, value string, ok bool) {
	i := strings.Index(line, ":")
	if i < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
}

// titleFromSlug, slug'dan gösterilecek ad üretir: "code-reviewer" → "Code Reviewer".
func titleFromSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
