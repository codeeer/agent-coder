package workflow

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Şablon referansının kökleri.
const (
	RootInput   = "input"
	RootTrigger = "trigger"
	RootSteps   = "steps"
)

// Adım çıktısının erişilebilir alanları.
const (
	FieldOutput = "output"
	FieldDiff   = "diff"
	FieldBranch = "branch"
	// FieldURL, adımın ürettiği adres (açılan PR, yazılan yorum).
	FieldURL = "url"
)

// refPattern, `{{ ... }}` kalıplarını yakalar.
//
// İzin verilen karakter kümesi dar: harf, rakam, alt çizgi, nokta ve tire.
// Bu bilinçli — kalıba uymayan bir `{{ ... }}` referans değil, DÜZ METİNDİR ve
// olduğu gibi kalır. Kullanıcının talimatında geçen süslü parantezli bir kod
// örneği sessizce bozulmasın diye.
var refPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.\-]+)\s*\}\}`)

// Ref, çözümlenmiş bir şablon referansı.
type Ref struct {
	// Raw, metindeki tam hali (`{{ steps.a1.output }}`).
	Raw string
	// Expr, süslü parantezsiz ifade (`steps.a1.output`).
	Expr string
	// Root: input | trigger | steps
	Root string
	// Node, yalnızca steps referanslarında dolu.
	Node string
	// Field, steps referanslarında alan; trigger referanslarında alan adı.
	Field string
}

// References, metindeki tüm referansları çözümler.
//
// Biçimsel olarak bozuk bir referans (`{{ steps.a1 }}` gibi eksik alan) hata
// döndürür — kaydetme anında yakalansın diye. Çalışma anında fark edilmesi,
// kullanıcının saatler sonra boş bir talimatla karşılaşması demek olurdu.
func References(text string) ([]Ref, error) {
	matches := refPattern.FindAllStringSubmatch(text, -1)
	out := make([]Ref, 0, len(matches))

	for _, m := range matches {
		ref, err := parseRef(m[0], m[1])
		if err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, nil
}

func parseRef(raw, expr string) (Ref, error) {
	parts := strings.Split(expr, ".")

	switch parts[0] {
	case RootInput:
		if len(parts) != 1 {
			return Ref{}, fmt.Errorf("%s: `input` alan almaz", raw)
		}
		return Ref{Raw: raw, Expr: expr, Root: RootInput}, nil

	case RootTrigger:
		if len(parts) != 2 || parts[1] == "" {
			return Ref{}, fmt.Errorf("%s: `trigger.<alan>` biçiminde olmalı", raw)
		}
		return Ref{Raw: raw, Expr: expr, Root: RootTrigger, Field: parts[1]}, nil

	case RootSteps:
		if len(parts) != 3 || parts[1] == "" {
			return Ref{}, fmt.Errorf(
				"%s: `steps.<adım>.%s|%s|%s|%s` biçiminde olmalı",
				raw, FieldOutput, FieldDiff, FieldBranch, FieldURL)
		}
		switch parts[2] {
		case FieldOutput, FieldDiff, FieldBranch, FieldURL:
		default:
			return Ref{}, fmt.Errorf("%s: `%s` diye bir alan yok (%s, %s, %s, %s)",
				raw, parts[2], FieldOutput, FieldDiff, FieldBranch, FieldURL)
		}
		return Ref{Raw: raw, Expr: expr, Root: RootSteps, Node: parts[1], Field: parts[2]}, nil

	default:
		return Ref{}, fmt.Errorf("%s: bilinmeyen değişken `%s` (kullanılabilir: %s, %s.<alan>, %s.<adım>.<alan>)",
			raw, parts[0], RootInput, RootTrigger, RootSteps)
	}
}

// StepResult, bir adımın sonraki adımlara açtığı değerler.
type StepResult struct {
	Output string
	Diff   string
	Branch string
	// URL, adımın ürettiği adres (açılan PR, yazılan yorum).
	URL string
}

// Context, şablonun çözümleneceği değerler.
type Context struct {
	Input   string
	Trigger map[string]string
	Steps   map[string]StepResult
}

// Render, şablonu çözümler.
//
// Bilinmeyen bir referans BOŞ METNE ÇEVRİLMEZ, hata döner. Sessizce boş
// bırakmak, agent'ın eksik bir talimatla çalışıp yanlış iş yapması demek olurdu;
// üstelik kullanıcı bunu ancak sonucu okuyunca fark ederdi.
func Render(text string, ctx Context) (string, error) {
	refs, err := References(text)
	if err != nil {
		return "", err
	}

	replacements := make(map[string]string, len(refs))
	for _, ref := range refs {
		value, err := resolve(ref, ctx)
		if err != nil {
			return "", err
		}
		replacements[ref.Raw] = value
	}

	// Uzun eşleşmeler önce: `{{a}}` ile `{{ a }}` ayrı anahtarlar olduğu için
	// karışmazlar, ama sıra deterministik olsun diye yine de sıralanıyor.
	keys := make([]string, 0, len(replacements))
	for k := range replacements {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	out := text
	for _, k := range keys {
		out = strings.ReplaceAll(out, k, replacements[k])
	}
	return out, nil
}

func resolve(ref Ref, ctx Context) (string, error) {
	switch ref.Root {
	case RootInput:
		return ctx.Input, nil

	case RootTrigger:
		v, ok := ctx.Trigger[ref.Field]
		if !ok {
			return "", fmt.Errorf("%s: tetikleyici böyle bir alan göndermedi", ref.Raw)
		}
		return v, nil

	case RootSteps:
		step, ok := ctx.Steps[ref.Node]
		if !ok {
			return "", fmt.Errorf("%s: `%s` adımı henüz çalışmadı", ref.Raw, ref.Node)
		}
		switch ref.Field {
		case FieldOutput:
			return step.Output, nil
		case FieldDiff:
			return step.Diff, nil
		case FieldURL:
			return step.URL, nil
		default:
			return step.Branch, nil
		}
	}
	return "", fmt.Errorf("%s: çözümlenemedi", ref.Raw)
}
