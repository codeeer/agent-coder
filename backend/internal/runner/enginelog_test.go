package runner

import (
	"encoding/base64"
	"strings"
	"testing"
)

/*
 * TestRedact — sır loga sızmışsa VERİTABANINA YAZILMADAN önce maskelenmeli.
 *
 * Motor yapılandırma hatasında isteğin tamamını basabiliyor, git ise uzak
 * adresi hata metnine koyabiliyor. Log arayüzde gösteriliyor ve indirilebiliyor;
 * bir kez yazılmış sırrı geri almak mümkün değil.
 */
func TestRedact(t *testing.T) {
	log := `Authorization: Bearer sk-cok-gizli-anahtar
git clone https://x-access-token:ghp_gizlitoken123@github.com/a/b.git
npm _auth=bXktbmV4dXMtdG9rZW4=`

	got := Redact(log, []string{
		"sk-cok-gizli-anahtar",
		"ghp_gizlitoken123",
		"bXktbmV4dXMtdG9rZW4=",
	})

	for _, sir := range []string{"sk-cok-gizli-anahtar", "ghp_gizlitoken123", "bXktbmV4dXMtdG9rZW4="} {
		if strings.Contains(got, sir) {
			t.Fatalf("sır maskelenmemiş: %q\n%s", sir, got)
		}
	}
	if strings.Count(got, "***") != 3 {
		t.Fatalf("üç maskeleme bekleniyordu:\n%s", got)
	}
	// Çevresindeki metin korunmalı — maskeleme logu okunamaz yapmamalı.
	if !strings.Contains(got, "github.com/a/b.git") {
		t.Fatalf("gizli olmayan içerik kaybolmuş:\n%s", got)
	}
}

// TestRedact_KisaDegerAtlanir — üç karakterlik bir "sır" metnin her yerinde
// eşleşir ve logu okunamaz hale getirirdi.
func TestRedact_KisaDegerAtlanir(t *testing.T) {
	log := "abc tanımlı değil, abc bulunamadı"
	if got := Redact(log, []string{"abc"}); got != log {
		t.Fatalf("kısa değer maskelenmemeliydi: %q", got)
	}
}

// TestRedact_BosSirGuvenli — boş sır her yeri maskelemez.
func TestRedact_BosSirGuvenli(t *testing.T) {
	log := "sıradan bir satır"
	if got := Redact(log, []string{"", "  "}); got != log {
		t.Fatalf("boş sır metni bozdu: %q", got)
	}
}

// TestSecretsOf — yeni bir sır eklendiğinde maskeleme listesine girmesi
// gerekiyor; bu test o listeyi kilitliyor.
func TestSecretsOf(t *testing.T) {
	req := Request{
		Provider: ProviderSpec{APIKey: "saglayici-anahtari"},
		Repo:     RepoSpec{Secret: "git-tokeni"},
		Packages: PackageRegistry{Token: "nexus-tokeni"},
		Agent: AgentSpec{MCPServers: []MCPServerSpec{
			{Name: "sentry", Secret: "mcp-tokeni"},
		}},
	}

	got := SecretsOf(req)
	for _, beklenen := range []string{"saglayici-anahtari", "git-tokeni", "nexus-tokeni", "mcp-tokeni"} {
		var bulundu bool
		for _, s := range got {
			if s == beklenen {
				bulundu = true
			}
		}
		if !bulundu {
			t.Fatalf("%q maskeleme listesinde yok — loga sızabilir", beklenen)
		}
	}
}

/*
 * TestSecretsOf_NpmrcBase64 — kimlik `~/.npmrc` içinde KODLANMIŞ duruyor.
 *
 * Agent o dosyayı okuyabiliyor (`cat ~/.npmrc` sıradan bir teşhis adımı) ve
 * çıktısı motor loguna düşüyor. Yalnızca ham token aransaydı maskeleme onu
 * görmez, kimlik base64 hâliyle veritabanına yazılırdı — base64 şifreleme
 * değildir.
 */
func TestSecretsOf_NpmrcBase64(t *testing.T) {
	req := Request{
		Packages: PackageRegistry{
			NPMRegistry: "https://nexus.local/repository/npm/",
			Username:    "ci",
			Token:       "cok-gizli-token",
		},
	}

	kodlu := base64.StdEncoding.EncodeToString([]byte("ci:cok-gizli-token"))
	got := SecretsOf(req)

	var bulundu bool
	for _, s := range got {
		if s == kodlu {
			bulundu = true
		}
	}
	if !bulundu {
		t.Fatalf("base64 kimlik maskeleme listesinde yok: %q", kodlu)
	}

	// Uçtan uca: `.npmrc` içeriği maskelenmeli.
	npmrc := "registry=https://nexus.local/repository/npm/\n" +
		"//nexus.local/repository/npm/:_auth=" + kodlu + "\n"
	if strings.Contains(Redact(npmrc, got), kodlu) {
		t.Fatal("base64 kimlik maskelenmemiş")
	}
}
