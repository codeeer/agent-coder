package runs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/runner"
)

/*
 * Zorunlu host'lar YAPILANDIRMADAN türetilir, elle yazılmaz.
 *
 * Spec 020 H4: kullanıcı LLM provider ve git repository adresini ayarlara zaten
 * girmiş — oraya eriştiğini bildiği için girmiş. Bir de whitelist'e yazmasını
 * beklemek gereksiz tekrar olurdu ve ilk kurulumu deneme yanılmaya çevirirdi.
 */
func TestZorunluHostlar_YapilandirmadanTuretilir(t *testing.T) {
	hostlar := zorunluHostlar(
		runner.ProviderSpec{BaseURL: "https://openrouter.ai/api/v1"},
		runner.RepoSpec{URL: "https://git.sirket.local/takim/proje.git"},
		runner.PackageRegistry{
			NPMRegistry:   "https://nexus.sirket.local:8081/repository/npm/",
			MavenRegistry: "https://nexus.sirket.local:8081/repository/maven/",
		},
		[]runner.MCPServerSpec{{URL: "https://mcp.ornek.com/sse"}},
	)

	require.Contains(t, hostlar, "openrouter.ai")
	require.Contains(t, hostlar, "git.sirket.local")
	require.Contains(t, hostlar, "nexus.sirket.local")
	require.Contains(t, hostlar, "mcp.ornek.com")
}

// Aynı host iki yerden geliyorsa bir kez döner — liste kalabalıklaşmasın.
func TestZorunluHostlar_Yinelenmez(t *testing.T) {
	hostlar := zorunluHostlar(
		runner.ProviderSpec{BaseURL: "https://nexus.sirket.local/v1"},
		runner.RepoSpec{URL: "https://nexus.sirket.local/repo.git"},
		runner.PackageRegistry{},
		nil,
	)

	require.Equal(t, []string{"nexus.sirket.local"}, hostlar)
}

// Boş ve bozuk adresler sessizce atlanır: yapılandırılmamış bir alan yüzünden
// çalıştırma reddedilmemeli.
func TestZorunluHostlar_BosVeBozukAtlanir(t *testing.T) {
	hostlar := zorunluHostlar(
		runner.ProviderSpec{BaseURL: ""},
		runner.RepoSpec{URL: "bu bir adres değil"},
		runner.PackageRegistry{},
		nil,
	)

	require.Empty(t, hostlar)
}
