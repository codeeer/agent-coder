package runner

import "testing"

// TestImageFor — sürüm seçimi imaj etiketine doğru çevrilmeli.
//
// En kritik hâl registry portu: düz bir "son iki nokta" araması port
// numarasını etiket sanır ve `localhost:5000/x` referansını
// `localhost:node-24.13.0` yapardı.
func TestImageFor(t *testing.T) {
	durumlar := []struct {
		ad     string
		taban  string
		surum  string
		beklsn string
	}{
		{
			ad:     "sürüm yoksa taban aynen döner",
			taban:  "ghcr.io/codeeer/agent-coder-runner:latest",
			surum:  "",
			beklsn: "ghcr.io/codeeer/agent-coder-runner:latest",
		},
		{
			ad:     "etiket sürümle değişir",
			taban:  "ghcr.io/codeeer/agent-coder-runner:latest",
			surum:  "24.13.0",
			beklsn: "ghcr.io/codeeer/agent-coder-runner:node-24.13.0",
		},
		{
			ad:     "latest olmayan etiket de değişir",
			taban:  "ghcr.io/codeeer/agent-coder-runner:sha-7972d67",
			surum:  "24.13.0",
			beklsn: "ghcr.io/codeeer/agent-coder-runner:node-24.13.0",
		},
		{
			ad:     "etiketsiz referansa etiket eklenir",
			taban:  "agent-coder/opencode-runner",
			surum:  "24.13.0",
			beklsn: "agent-coder/opencode-runner:node-24.13.0",
		},
		{
			ad:     "yerel varsayılan",
			taban:  "agent-coder/opencode-runner:latest",
			surum:  "24.13.0",
			beklsn: "agent-coder/opencode-runner:node-24.13.0",
		},
		{
			ad:     "registry portu etiket sanılmaz",
			taban:  "localhost:5000/agent-coder-runner:latest",
			surum:  "24.13.0",
			beklsn: "localhost:5000/agent-coder-runner:node-24.13.0",
		},
		{
			ad:     "registry portu var, etiket yok",
			taban:  "localhost:5000/agent-coder-runner",
			surum:  "24.13.0",
			beklsn: "localhost:5000/agent-coder-runner:node-24.13.0",
		},
	}

	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			if got := ImageFor(d.taban, d.surum); got != d.beklsn {
				t.Fatalf("ImageFor(%q, %q) = %q, beklenen %q", d.taban, d.surum, got, d.beklsn)
			}
		})
	}
}

// TestNodeVersions — liste dosyası yorum ve boş satır taşıyor; onlar sürüm değil.
func TestNodeVersions(t *testing.T) {
	surumler := NodeVersions()
	if len(surumler) == 0 {
		t.Fatal("liste boş — en az bir sürüm yayınlanmalı")
	}
	for _, s := range surumler {
		if s == "" || s[0] == '#' {
			t.Fatalf("yorum veya boş satır sürüm sanıldı: %q", s)
		}
	}
	// İlk sürüm listedeki ilk satır olmalı: arayüzdeki sıra buradan geliyor.
	if surumler[0] != "24.13.0" {
		t.Fatalf("ilk sürüm 24.13.0 olmalı, %q geldi", surumler[0])
	}
}

func TestSupportsNodeVersion(t *testing.T) {
	// Boş = seçim yok; taban imaj kullanılır, her zaman geçerli.
	if !SupportsNodeVersion("") {
		t.Fatal("boş sürüm geçerli sayılmalı")
	}
	if !SupportsNodeVersion("24.13.0") {
		t.Fatal("listedeki sürüm geçerli sayılmalı")
	}
	// Yayınlanmamış bir sürüm seçilirse koşu "imaj bulunamadı" ile düşerdi;
	// istek daha uca varmadan reddedilmeli.
	if SupportsNodeVersion("18.0.0") {
		t.Fatal("listede olmayan sürüm geçerli sayılmamalı")
	}
}
