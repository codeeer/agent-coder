package runner

import (
	"encoding/base64"
	"strings"
	"testing"
)

/*
 * Kurumsal paket deposu.
 *
 * En kritik iki iddia:
 *   1. Adres tanımlı DEĞİLKEN hiçbir şey değişmez — dosya yazılmaz, talimata
 *      blok eklenmez. Yani özellik kapalıyken container bugünküyle aynıdır.
 *   2. Token hiçbir ORTAM DEĞİŞKENİNDE görünmez; yalnızca 0600 bir dosyada.
 *      Agent `env` yazdırabiliyor.
 */

func kurulum() PackageRegistry {
	return PackageRegistry{
		NPMRegistry: "https://nexus.sirket.local/repository/npm-group/",
		Username:    "ci-kullanici",
		Token:       "gizli-token",
	}
}

func TestBuildNPMRC_AdresYoksaDosyaYok(t *testing.T) {
	if got := buildNPMRC(PackageRegistry{}); got != nil {
		t.Fatalf("adres yokken dosya yazılmamalı, %q geldi", got)
	}
	// Kimlik verilmiş ama adres yoksa yine kapalı: nereye göndereceğimizi
	// bilmediğimiz bir token'ı dosyaya yazmak anlamsız.
	yarim := PackageRegistry{Username: "u", Token: "t"}
	if got := buildNPMRC(yarim); got != nil {
		t.Fatalf("adres yokken dosya yazılmamalı, %q geldi", got)
	}
}

func TestBuildNPMRC_KimliksizDepo(t *testing.T) {
	p := PackageRegistry{NPMRegistry: "https://nexus.sirket.local/repository/npm-group/"}
	got := string(buildNPMRC(p))

	if !strings.Contains(got, "registry=https://nexus.sirket.local/repository/npm-group/") {
		t.Fatalf("registry satırı yok:\n%s", got)
	}
	// Anonim okumaya açık depoda kimlik satırı OLMAMALI.
	if strings.Contains(got, "_auth") {
		t.Fatalf("kimlik verilmediği hâlde _auth yazılmış:\n%s", got)
	}
}

func TestBuildNPMRC_KimlikAdreseBaglanir(t *testing.T) {
	got := string(buildNPMRC(kurulum()))

	beklenen := base64.StdEncoding.EncodeToString([]byte("ci-kullanici:gizli-token"))
	if !strings.Contains(got, ":_auth="+beklenen) {
		t.Fatalf("kimlik base64 olarak yazılmamış:\n%s", got)
	}
	// Kimlik ADRESE bağlanmalı; genel bir `_auth` satırı başka kayıt
	// defterlerine de gönderilirdi.
	if !strings.Contains(got, "//nexus.sirket.local/repository/npm-group/:_auth=") {
		t.Fatalf("kimlik adrese bağlanmamış:\n%s", got)
	}
	// Şema YALNIZCA `registry=` satırında kalmalı; kimlik anahtarında şema
	// olursa npm adresi eşleştiremez.
	for _, satir := range strings.Split(strings.TrimSpace(got), "\n") {
		if strings.HasPrefix(satir, "registry=") {
			continue
		}
		if strings.Contains(satir, "https:") || strings.Contains(satir, "http:") {
			t.Fatalf("kimlik anahtarında şema kalmış — npm eşleştiremez: %q", satir)
		}
	}
	// Token düz metin olarak GÖRÜNMEMELİ (base64 dışında).
	if strings.Contains(got, "gizli-token") {
		t.Fatalf("token düz metin yazılmış:\n%s", got)
	}
}

/*
 * TestBuildNPMRC_OlmeyenAyarYazilmaz — npm'in tanımadığı anahtar yazılmaz.
 *
 * `always-auth` npm 9'da KALDIRILDI; runner npm 11 taşıyor (Node 24, bkz.
 * node-versions.txt). Etkisi yok ama zararsız da değil: dosya KULLANICI
 * seviyesinde (`$HOME/.npmrc`) olduğu için npm HER çağrıda
 * "Unknown user config" uyarısı basıyor. O uyarı agent'ın okuduğu her araç
 * çıktısına ve derleme loglarına düşüyor.
 *
 * Kimlik doğrulama bu satır olmadan da çalışıyor — üstveri VE tarball
 * istekleri kimlikli gidiyor (kurumsal depoya karşı ölçüldü).
 */
func TestBuildNPMRC_OlmeyenAyarYazilmaz(t *testing.T) {
	got := string(buildNPMRC(kurulum()))
	if strings.Contains(got, "always-auth") {
		t.Fatalf("npm 9+'ta kaldırılan ayar yazılmış — her komutta uyarı basar:\n%s", got)
	}
}

func TestBuildNPMRC_SondakiSlashEklenir(t *testing.T) {
	p := kurulum()
	p.NPMRegistry = "https://nexus.sirket.local/repository/npm-group"
	got := string(buildNPMRC(p))

	if !strings.Contains(got, "//nexus.sirket.local/repository/npm-group/:_auth=") {
		t.Fatalf("npm'in beklediği biçim kurulmamış:\n%s", got)
	}
}

// TestBuildConfigFiles_PaketDeposuDosyasi — dosya doğru yolda ve 0600.
func TestBuildConfigFiles_PaketDeposuDosyasi(t *testing.T) {
	p := ProviderSpec{Slug: "openrouter", Kind: "openrouter", APIKey: "k"}
	a := AgentSpec{Slug: "coder", Prompt: "yaz"}

	files, err := BuildConfigFiles(p, a, "model-x", kurulum())
	if err != nil {
		t.Fatal(err)
	}

	var bulundu bool
	for _, f := range files {
		if f.Path != npmrcPath {
			continue
		}
		bulundu = true
		if f.Mode != 0o600 {
			t.Fatalf("izinler 0600 olmalı, %o geldi", f.Mode)
		}
		if !strings.HasPrefix(f.Path, "/home/agent/") {
			t.Fatalf(".npmrc $HOME altında olmalı: %q", f.Path)
		}
		// /work klonlama hedefi; oraya yazılan dosya kullanıcının diff'ine düşer.
		if strings.HasPrefix(f.Path, "/work") {
			t.Fatalf(".npmrc kullanıcının deposuna yazılmamalı: %q", f.Path)
		}
	}
	if !bulundu {
		t.Fatal(".npmrc dosyası üretilmemiş")
	}

	// Kapalıyken hiç oluşmamalı.
	files, err = BuildConfigFiles(p, a, "model-x", PackageRegistry{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == npmrcPath {
			t.Fatal("özellik kapalıyken .npmrc yazılmamalı")
		}
	}
}

/*
 * TestPackageSection — model yapılandırmayı görmüyor, ona SÖYLENMELİ.
 *
 * Betiklerdeki kararın aynısı: dosyayı koymak yetmez. Kurumsal ağda model
 * "kurulum başarısız, registry'yi değiştireyim" derse asla ulaşamayacağı bir
 * adrese yönelir; talimat bunu açıkça yasaklıyor.
 */
func TestPackageSection(t *testing.T) {
	if got := packageSection(PackageRegistry{}); got != "" {
		t.Fatalf("adres yokken blok yazılmamalı, %q geldi", got)
	}

	got := packageSection(kurulum())
	for _, beklenen := range []string{
		"## Paket deposu",
		"https://nexus.sirket.local/repository/npm-group/",
		"~/.npmrc",
		"--registry",
	} {
		if !strings.Contains(got, beklenen) {
			t.Fatalf("blokta %q yok:\n%s", beklenen, got)
		}
	}
	// Token talimata SIZMAMALI — talimat metni çalıştırma kaydına da yazılıyor.
	if strings.Contains(got, "gizli-token") {
		t.Fatalf("token agent talimatına yazılmış:\n%s", got)
	}
}

// TestBuildAgentFile_PaketDeposuBlogu — blok agent dosyasına giriyor mu.
func TestBuildAgentFile_PaketDeposuBlogu(t *testing.T) {
	a := AgentSpec{Slug: "coder", Prompt: "Sen kod yazarsın."}

	kapali := string(buildAgentFile(a, PackageRegistry{}))
	if strings.Contains(kapali, "Paket deposu") {
		t.Fatalf("kapalıyken talimatta blok var:\n%s", kapali)
	}

	acik := string(buildAgentFile(a, kurulum()))
	if !strings.Contains(acik, "## Paket deposu") {
		t.Fatalf("açıkken talimatta blok yok:\n%s", acik)
	}
	if !strings.Contains(acik, "Sen kod yazarsın.") {
		t.Fatal("kullanıcının talimatı kaybolmuş")
	}
	if strings.Contains(acik, "gizli-token") {
		t.Fatalf("token agent dosyasına yazılmış:\n%s", acik)
	}
}
