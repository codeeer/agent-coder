package runner

import (
	_ "embed"
	"strings"
)

/*
 * Node sürümüne göre runner imajı seçimi.
 *
 * Agent'ın koşturduğu build komutları belirli bir Node sürümü isteyebiliyor.
 * Sürüm KOŞU ANINDA İNDİRİLMEZ: desteklenen her sürüm için derleme anında ayrı
 * bir imaj yayınlanır ve kullanıcı koşuyu başlatmadan önce seçer.
 *
 * Bu dosya opencode'a dair hiçbir şey bilmez — paket sınırı korunuyor.
 */

// nodeVersionsRaw, yayınlanan sürümlerin listesi.
//
// Liste bu paketin içinde duruyor çünkü backend imajının derleme bağlamı
// `./backend`; `runner/` altındaki bir dosya `go:embed` ile görülemezdi.
// Aynı dosyayı CI matrisi ve Makefile depo kökünden okuyor — tek kaynak.
//
//go:embed node-versions.txt
var nodeVersionsRaw string

// nodeImagePrefix, sürümlü etiketlerin öneki.
//
// Araç adı ETİKETTE taşınıyor ki ileride `temurin-21` gibi başka çalışma
// zamanları aynı kalıba eklenebilsin; yalnızca `24.13.0` yazılsaydı ikinci
// bir araç geldiğinde etiketler ayırt edilemezdi.
const nodeImagePrefix = "node-"

// NodeVersions, imajı yayınlanmış Node sürümleri — listedeki sırayla.
//
// Boş satırlar ve `#` ile başlayanlar atlanır.
func NodeVersions() []string {
	out := []string{}
	for _, satir := range strings.Split(nodeVersionsRaw, "\n") {
		satir = strings.TrimSpace(satir)
		if satir == "" || strings.HasPrefix(satir, "#") {
			continue
		}
		out = append(out, satir)
	}
	return out
}

// SupportsNodeVersion, sürümün listede olup olmadığı.
//
// Boş sürüm "seçim yok" demektir ve her zaman geçerlidir; taban imaj kullanılır.
func SupportsNodeVersion(v string) bool {
	if v == "" {
		return true
	}
	for _, s := range NodeVersions() {
		if s == v {
			return true
		}
	}
	return false
}

/*
 * ImageFor, taban imaj referansından sürüme özel etiketi üretir.
 *
 *   ghcr.io/x/agent-coder-runner:latest + "24.13.0"
 *     → ghcr.io/x/agent-coder-runner:node-24.13.0
 *
 * Sürüm boşsa taban referans AYNEN döner: sürüm seçmeyen kullanıcı için
 * hiçbir davranış değişmez.
 *
 * Etiket, SON `/`'den sonraki son `:`'tir. Düz `strings.LastIndex(":")`
 * yazılsaydı registry portu olan referanslar (`localhost:5000/x`) bozulur,
 * port numarası etiket sanılırdı.
 */
func ImageFor(base, nodeVersion string) string {
	if nodeVersion == "" {
		return base
	}

	depo := base
	if i := strings.LastIndex(base, "/"); i >= 0 {
		if j := strings.LastIndex(base[i:], ":"); j >= 0 {
			depo = base[:i+j]
		}
	} else if j := strings.LastIndex(base, ":"); j >= 0 {
		depo = base[:j]
	}

	return depo + ":" + nodeImagePrefix + nodeVersion
}
