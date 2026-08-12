package runbuild

import "testing"

// TestResolveNodeVersion — öncelik sırası: koşu > proje > taban imaj.
//
// Üçüncü basamak burada boş dize olarak görünür; onu imaj etiketine çeviren
// runner.ImageFor kendi testinde sınanıyor.
func TestResolveNodeVersion(t *testing.T) {
	durumlar := []struct {
		ad     string
		istek  string
		proje  string
		beklsn string
	}{
		{"hiçbiri seçilmezse taban imaj", "", "", ""},
		{"projenin varsayılanı", "", "24.13.0", "24.13.0"},
		{"koşu seçimi projeyi ezer", "24.19.0", "24.13.0", "24.19.0"},
		{"koşu seçimi tek başına", "24.13.0", "", "24.13.0"},
		{"boşluklu değer temizlenir", "  24.13.0 ", "", "24.13.0"},
		{"yalnızca boşluk seçim sayılmaz", "   ", "24.13.0", "24.13.0"},
	}

	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			if got := resolveNodeVersion(d.istek, d.proje); got != d.beklsn {
				t.Fatalf("resolveNodeVersion(%q, %q) = %q, beklenen %q",
					d.istek, d.proje, got, d.beklsn)
			}
		})
	}
}
