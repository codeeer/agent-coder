package runner

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Klasörlü betiklerin container'a yerleşimi ve talimatta anlatılması
 * (spec 022 Blok 3-4).
 *
 * BU DOSYADAKİ EN ÖNEMLİ TEST `TestKlasor_TalimattakiYolDiskeYazilanlaAyni`.
 * Yolu iki yerde üretiyoruz — biri dosyayı yazarken, biri modele anlatırken —
 * ve ikisi ayrışırsa model var olmayan bir yolu dener. Hata çalıştırma anına
 * kadar görünmez ve orada da "betik çalışmadı" diye görünür, "yol yanlış"
 * diye değil.
 */

func agentIle(scriptler []ScriptSpec, klasorler []FolderSpec) AgentSpec {
	return AgentSpec{
		Slug: "kampanya", Description: "d",
		Prompt: "p", AllowBash: true,
		Scripts: scriptler, ScriptFolders: klasorler,
	}
}

func TestKlasor_DosyaAltDizineYazilir(t *testing.T) {
	a := agentIle([]ScriptSpec{
		{Name: "01-baslat", Content: "echo bir", Folder: "node-24"},
		{Name: "ortak", Content: "echo iki"},
	}, []FolderSpec{{Name: "node-24", Description: "Node yükseltmesi"}})

	files, err := BuildConfigFiles(ProviderSpec{Kind: "openrouter", Slug: "or"}, a, "m", PackageRegistry{})
	require.NoError(t, err)

	yollar := map[string]bool{}
	for _, f := range files {
		yollar[f.Path] = true
	}
	require.True(t, yollar["/home/agent/scripts/node-24/01-baslat.sh"])
	require.True(t, yollar["/home/agent/scripts/ortak.sh"],
		"klasörsüz betiğin yolu DEĞİŞMEMELİ — mevcut kurulumlar bozulmaz")
}

/*
 * DİZİN GİRDİSİ DOSYADAN ÖNCE gelir.
 *
 * Tar akışı sırayla çıkarılıyor; dizin sonra gelseydi dosya çıkarılırken ara
 * dizin Docker tarafından oluşturulurdu ve bizim sahiplik/izin ayarımız
 * uygulanmazdı.
 */
func TestKlasor_DizinGirdisiDosyadanOnce(t *testing.T) {
	a := agentIle([]ScriptSpec{
		{Name: "01-baslat", Content: "echo", Folder: "node-24"},
	}, []FolderSpec{{Name: "node-24"}})

	files, err := BuildConfigFiles(ProviderSpec{Kind: "openrouter", Slug: "or"}, a, "m", PackageRegistry{})
	require.NoError(t, err)

	var dizinIdx, dosyaIdx = -1, -1
	for i, f := range files {
		switch f.Path {
		case "/home/agent/scripts/node-24":
			dizinIdx = i
			require.True(t, f.IsDir, "klasör girdisi dizin olarak işaretlenmeli")
		case "/home/agent/scripts/node-24/01-baslat.sh":
			dosyaIdx = i
		}
	}
	require.NotEqual(t, -1, dizinIdx, "dizin girdisi olmalı")
	require.Less(t, dizinIdx, dosyaIdx, "dizin dosyadan ÖNCE gelmeli")
}

// Boş klasör için dizin de bölüm de yazılmaz: kullanamayacağı bir dizini
// modele göstermek onu var olmayan bir adımı aramaya iter.
func TestKlasor_BosKlasorYazilmaz(t *testing.T) {
	a := agentIle(nil, []FolderSpec{{Name: "bos", Description: "içi boş"}})

	files, err := BuildConfigFiles(ProviderSpec{Kind: "openrouter", Slug: "or"}, a, "m", PackageRegistry{})
	require.NoError(t, err)

	for _, f := range files {
		require.NotEqual(t, "/home/agent/scripts/bos", f.Path)
	}
	require.NotContains(t, string(buildAgentFile(a, PackageRegistry{})), "bos")
}

/*
 * TALİMATTAKİ YOL, DİSKE YAZILAN DOSYANIN YOLUYLA AYNI.
 *
 * Bu tek iddia bir hata SINIFINI kapatıyor: klasör bilgisi bir yolda taşınıp
 * diğerinde taşınmazsa (örneğin depo katmanı klasör adını JOIN'den doldurmayı
 * unutursa) dosya köke yazılır, talimatta alt dizin yazar ve model olmayan
 * bir yolu dener.
 */
func TestKlasor_TalimattakiYolDiskeYazilanlaAyni(t *testing.T) {
	a := agentIle([]ScriptSpec{
		{Name: "01-baslat", Content: "echo", Folder: "node-24"},
		{Name: "02-devam", Content: "echo", Folder: "node-24"},
		{Name: "ortak", Content: "echo"},
	}, []FolderSpec{{Name: "node-24", Description: "Node yükseltmesi"}})

	files, err := BuildConfigFiles(ProviderSpec{Kind: "openrouter", Slug: "or"}, a, "m", PackageRegistry{})
	require.NoError(t, err)

	talimat := string(buildAgentFile(a, PackageRegistry{}))

	for _, f := range files {
		if f.IsDir || !strings.HasPrefix(f.Path, scriptsDir+"/") {
			continue
		}
		require.Contains(t, talimat, f.Path,
			"diske yazılan her betiğin yolu talimatta AYNEN geçmeli")
	}
}

func TestKlasor_TalimatKlasoruAnlatir(t *testing.T) {
	a := agentIle([]ScriptSpec{
		{Name: "01-baslat", Description: "engines alanını günceller", Content: "echo", Folder: "node-24"},
		{Name: "02-devam", Content: "echo", Folder: "node-24"},
	}, []FolderSpec{{Name: "node-24", Description: "Node 18'den 24'e standart adımlar"}})

	talimat := string(buildAgentFile(a, PackageRegistry{}))

	require.Contains(t, talimat, "node-24")
	require.Contains(t, talimat, "Node 18'den 24'e standart adımlar",
		"kampanyanın ne olduğunu model klasör açıklamasından öğrenir")
	require.Contains(t, talimat, "/home/agent/scripts/node-24",
		"dizin yolu yazılmalı")
	require.Contains(t, talimat, "engines alanını günceller")

	// Sıra: 01 önce, 02 sonra.
	require.Less(t, strings.Index(talimat, "01-baslat"), strings.Index(talimat, "02-devam"))
}

/*
 * PROJE DİZİNİ TALİMATTA YAZAR (spec 022 H6).
 *
 * Script yazarı projesinin içindeki yolu biliyor, kökün nereye açıldığını
 * bilmiyor. Model de aynı durumda.
 */
func TestKlasor_TalimatProjeDizininiSoyler(t *testing.T) {
	a := agentIle([]ScriptSpec{{Name: "ortak", Content: "echo"}}, nil)

	talimat := string(buildAgentFile(a, PackageRegistry{}))

	require.Contains(t, talimat, "PROJECT_DIR")
	require.Contains(t, talimat, ProjectDir)
}

/*
 * BASH KAPALIYKEN KLASÖRLÜ BETİK DE YAZILMAZ VE ANLATILMAZ.
 *
 * `scriptsFor` tek kapı; "yeni yetenek açmıyor" iddiasının tek kanıtı bu.
 */
func TestKlasor_BashKapaliyken(t *testing.T) {
	a := agentIle([]ScriptSpec{
		{Name: "01-baslat", Content: "echo", Folder: "node-24"},
	}, []FolderSpec{{Name: "node-24", Description: "kampanya"}})
	a.AllowBash = false

	files, err := BuildConfigFiles(ProviderSpec{Kind: "openrouter", Slug: "or"}, a, "m", PackageRegistry{})
	require.NoError(t, err)

	for _, f := range files {
		require.NotContains(t, f.Path, "node-24")
	}

	talimat := string(buildAgentFile(a, PackageRegistry{}))
	require.NotContains(t, talimat, "node-24")
	require.NotContains(t, talimat, "Kullanabileceğin betikler")
}
