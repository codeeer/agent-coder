package runner

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func mavenKurulum() PackageRegistry {
	return PackageRegistry{
		MavenRegistry: "https://nexus.sirket.local/repository/maven-public/",
		Username:      "ci",
		Token:         "s3cr3t",
	}
}

// ── Kapalıyken hiçbir şey yazılmaz ──────────────────────────────────────────

func TestSettingsXML_AdresYokkenDosyaUretilmez(t *testing.T) {
	require.Nil(t, buildSettingsXML(PackageRegistry{}))
	// npm tanımlı ama Maven değil: yalnızca npm dosyası yazılmalı.
	require.Nil(t, buildSettingsXML(PackageRegistry{NPMRegistry: "https://n.local/"}))
}

func TestBuildConfigFiles_MavenKapaliykenSettingsXMLYok(t *testing.T) {
	files, err := BuildConfigFiles(
		ProviderSpec{Slug: "openrouter", Kind: "openrouter"},
		AgentSpec{Slug: "a"}, "m", PackageRegistry{NPMRegistry: "https://n.local/"})
	require.NoError(t, err)

	for _, f := range files {
		require.NotEqual(t, settingsXMLPath, f.Path, "Maven kapalıyken dosya yazılmamalı")
	}
}

// ── mirrorOf=* : bu işin can damarı ─────────────────────────────────────────

/*
 * npm'de agent'ın kaçış yolu `--registry` bayrağıydı. Maven'da kaçış yolu daha
 * geniş: projenin KENDİ pom'u `<repositories>` ile başka bir depo ilan
 * edebiliyor. `*` onu da kuruma çeviriyor.
 */
func TestSettingsXML_TumDepolarKurumaYonlendirilir(t *testing.T) {
	got := string(buildSettingsXML(mavenKurulum()))

	require.Contains(t, got, "<mirrorOf>*</mirrorOf>",
		"yıldız olmadan pom'un ilan ettiği depolar kuruma gitmez")
	require.Contains(t, got, "<url>https://nexus.sirket.local/repository/maven-public/</url>")
}

func TestSettingsXML_AynaVeSunucuAyniKimlikleEslesir(t *testing.T) {
	var s struct {
		Servers []struct {
			ID       string `xml:"id"`
			Username string `xml:"username"`
			Password string `xml:"password"`
		} `xml:"servers>server"`
		Mirrors []struct {
			ID string `xml:"id"`
		} `xml:"mirrors>mirror"`
	}
	require.NoError(t, xml.Unmarshal(buildSettingsXML(mavenKurulum()), &s))

	require.Len(t, s.Mirrors, 1)
	require.Len(t, s.Servers, 1)
	// Maven kimliği aynaya ID ÜZERİNDEN bağlar; ikisi tutmazsa kimlik hiç
	// gönderilmez ve depo 401 döner.
	require.Equal(t, s.Mirrors[0].ID, s.Servers[0].ID)
	require.Equal(t, "ci", s.Servers[0].Username)
	require.Equal(t, "s3cr3t", s.Servers[0].Password)
}

// ── Kimlik opsiyonel ────────────────────────────────────────────────────────

func TestSettingsXML_KimlikYokkenSunucuBlogyYazilmaz(t *testing.T) {
	p := mavenKurulum()
	p.Username, p.Token = "", ""

	got := string(buildSettingsXML(p))
	require.NotContains(t, got, "<server>",
		"anonim okumaya açık depoda kimliksiz bir sunucu bloğu Maven'ı boş bir "+
			"kimlik doğrulama denemesine sokardı")
	require.Contains(t, got, "<mirrorOf>*</mirrorOf>")
}

// ── XML kaçırma ─────────────────────────────────────────────────────────────

/*
 * Parola bu dosyaya giriyor. Kaçırılmamış tek bir `&`, dosyayı SESSİZCE
 * bozardı: Maven yapılandırmayı okuyamaz ve hata "depoya ulaşılamadı" gibi
 * görünürdü. XML kütüphaneyle üretiliyor, metin şablonuyla değil — bu test
 * o kararın bekçisi.
 */
func TestSettingsXML_OzelKarakterliParolaGecerliXMLUretir(t *testing.T) {
	p := mavenKurulum()
	p.Token = `a&b<c>d"e'f`

	ham := buildSettingsXML(p)

	var s struct {
		Servers []struct {
			Password string `xml:"password"`
		} `xml:"servers>server"`
	}
	require.NoError(t, xml.Unmarshal(ham, &s), "üretilen XML ayrıştırılabilmeli")
	require.Equal(t, `a&b<c>d"e'f`, s.Servers[0].Password, "parola aynen geri okunmalı")
	require.NotContains(t, string(ham), "a&b", "ham `&` kaçırılmadan yazılmamalı")
}

func TestSettingsXML_XMLBasligiVar(t *testing.T) {
	require.True(t, strings.HasPrefix(string(buildSettingsXML(mavenKurulum())), "<?xml"))
}

// ── Süre sınırı (npm) ───────────────────────────────────────────────────────

/*
 * Ölçülmüş bir sorunun karşılığı: kurumsal depoya ulaşılamayan bir
 * çalıştırmada tek bir paket için ~4 dakika harcandığı görüldü (spec 017
 * doğrulaması). npm'in varsayılanı tek istek için 300 saniye ve iki kez daha
 * deniyor.
 */
func TestNPMRC_SureSinirlariYazilir(t *testing.T) {
	got := string(buildNPMRC(PackageRegistry{
		NPMRegistry: "https://n.local/", TimeoutSec: 60,
	}))

	require.Contains(t, got, "fetch-timeout=60000", "saniye milisaniyeye çevrilmeli")
	require.Contains(t, got, "fetch-retries=1")
}

func TestNPMRC_SureSinirsizkenSatirYazilmaz(t *testing.T) {
	got := string(buildNPMRC(PackageRegistry{NPMRegistry: "https://n.local/"}))

	require.NotContains(t, got, "fetch-timeout")
	require.NotContains(t, got, "fetch-retries")
}

// Kurumsal adres tanımlı DEĞİLKEN süre sınırı da yazılmaz: dosya hiç
// üretilmiyor, yani ayar kurumsal olmayan kurulumları etkilemiyor.
func TestNPMRC_AdresYokkenSureSiniriDaYok(t *testing.T) {
	require.Nil(t, buildNPMRC(PackageRegistry{TimeoutSec: 60}))
}

// ── Agent talimatı ──────────────────────────────────────────────────────────

func TestPackageSection_MavenKacisYollariYasaklanir(t *testing.T) {
	got := packageSection(mavenKurulum())

	require.Contains(t, got, "nexus.sirket.local")
	// Üç kaçış yolunun üçü de ayrı ayrı yazılmalı; "adresi değiştirme" tek
	// başına yetmiyor.
	require.Contains(t, got, "-s")
	require.Contains(t, got, "settings.xml")
	require.Contains(t, got, "<repositories>")
}

func TestPackageSection_YalnizcaMavenTanimliyken(t *testing.T) {
	p := PackageRegistry{MavenRegistry: "https://m.local/"}
	got := packageSection(p)

	require.Contains(t, got, "Maven")
	require.NotContains(t, got, "~/.npmrc", "npm kapalıyken npm talimatı yazılmamalı")
}

func TestPackageSection_IkisiDeKapaliykenBos(t *testing.T) {
	require.Empty(t, packageSection(PackageRegistry{}))
}

// ── Java talimatı ───────────────────────────────────────────────────────────

/*
 * Koşuda Java sürüm seçici YOK (spec 018: Kapsam dışı); seçim agent'a bu
 * bilgiyi vererek yapılıyor. Yollar mimariden bağımsız sabit bağlar olmalı —
 * Temurin'in gerçek dizini `…-amd64`/`…-arm64` ile bitiyor ve talimata o yol
 * yazılsaydı imajın koştuğu mimariye göre yanlış olurdu.
 */
func TestJavaSection_IkiSurumVeVarsayilanYazili(t *testing.T) {
	got := javaSection()

	require.Contains(t, got, "/opt/java/25")
	require.Contains(t, got, "/opt/java/17")
	require.Contains(t, got, "varsayılan")
	require.Contains(t, got, "JAVA_HOME=/opt/java/17", "Maven için geçiş komutu örneklenmeli")

	require.NotContains(t, got, "amd64")
	require.NotContains(t, got, "arm64")
}

/*
 * `java` ile `mvn` aynı şekilde sürüm değiştirmiyor ve talimat bunu SÖYLEMELİ.
 *
 * Gerçek bir koşuda ölçüldü: agent `JAVA_HOME=/opt/java/17 java -version`
 * çalıştırıp 25 aldı ve "dizin yok galiba" diye rapor etti. `java` kabuktan
 * PATH ile bulunuyor; JAVA_HOME yalnızca Maven'ın başlatıcısını etkiliyor.
 * Bu test o düzeltmenin bekçisi.
 */
func TestJavaSection_DogrudanJavaIcinTamYolYazili(t *testing.T) {
	got := javaSection()

	require.Contains(t, got, "/opt/java/17/bin/java",
		"doğrudan java çağrısı için tam yol örneklenmeli")
	require.Contains(t, got, "ÇALIŞMAZ",
		"JAVA_HOME'un java'yı etkilemediği açıkça yazılmalı")
}
