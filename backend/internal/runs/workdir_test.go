package runs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
)

/*
 * Yerleşim ayarı OKUNAMIYORSA çalıştırma düşmez, varsayılana düşer.
 *
 * Spec 025 H2: bu bir kolaylık ayarı, güvenlik kontrolü değil. Ayarın
 * gelmemesi (closure nil) ile geçersiz olması aynı sonucu vermeli — ikisi de
 * bugünkü davranış.
 */
func TestProjectDir_AyarYoksaVarsayilan(t *testing.T) {
	m := &Manager{}
	require.Equal(t, runner.WorkRoot,
		m.projectDir("https://git.sirket.local/takim/proje.git"))
}

func TestProjectDir_AyarGecersizseVarsayilan(t *testing.T) {
	m := &Manager{limits: Limits{
		WorkdirLayout: func() runner.WorkdirLayout { return "uydurma" },
	}}
	require.Equal(t, runner.WorkRoot,
		m.projectDir("https://git.sirket.local/takim/proje.git"))
}

func TestProjectDir_RepoYerlesimi(t *testing.T) {
	m := &Manager{limits: Limits{
		WorkdirLayout: func() runner.WorkdirLayout { return runner.LayoutRepo },
	}}
	require.Equal(t, "/work/proje",
		m.projectDir("https://git.sirket.local/takim/proje.git"))
}

/*
 * AYARIN DEĞİŞMESİ YENİDEN BAŞLATMA GEREKTİRMEZ.
 *
 * Closure olmasının tek sebebi bu; sabit bir alan olsaydı kullanıcı ayarı
 * değiştirdikten sonra sunucuyu yeniden başlatana kadar eski yerleşim
 * kullanılırdı ve bunu hiçbir yerde göremezdi.
 */
func TestProjectDir_AyarDegisimiSonrakiKosudaGecerli(t *testing.T) {
	yerlesim := runner.LayoutRoot
	m := &Manager{limits: Limits{
		WorkdirLayout: func() runner.WorkdirLayout { return yerlesim },
	}}
	const url = "https://git.sirket.local/takim/proje.git"

	require.Equal(t, runner.WorkRoot, m.projectDir(url))
	yerlesim = runner.LayoutRepo
	require.Equal(t, "/work/proje", m.projectDir(url))
}
