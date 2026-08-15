package runs

import (
	"github.com/agent-coder/backend/internal/hostlist"
	"github.com/agent-coder/backend/internal/runner"
)

/*
 * zorunluHostlar, ürünün çalışması için her hâlükârda gereken adresler.
 *
 * NEDEN TÜRETİLİYOR, elle yazılmıyor (spec 020 H4): kullanıcı LLM provider,
 * git repository ve registry adreslerini ayarlara zaten girmiş — oraya
 * eriştiğini bildiği için girmiş. Bir de whitelist'e yazmasını beklemek
 * gereksiz tekrar olur ve ilk kurulumu deneme yanılmaya çevirirdi.
 *
 * Motorun KENDİ ihtiyaçları burada değil: onlar opencode'a özgü ve
 * `internal/runner/opencode` sınırının içinde duruyor (AGENTS.md).
 *
 * Bozuk veya boş adres SESSİZCE atlanır: yapılandırılmamış bir alan yüzünden
 * çalıştırmayı reddetmek, kullanıcının hiç dokunmadığı bir yer yüzünden işi
 * durdurmak olurdu.
 */
func zorunluHostlar(
	provider runner.ProviderSpec,
	repo runner.RepoSpec,
	paketler runner.PackageRegistry,
	mcp []runner.MCPServerSpec,
) []string {
	adresler := []string{
		provider.BaseURL,
		repo.URL,
		paketler.NPMRegistry,
		paketler.MavenRegistry,
	}
	for _, s := range mcp {
		adresler = append(adresler, s.URL)
	}

	/*
		Adres → host çevrimi `hostlist`'te: aynı listeyi bir de "hangi kapılar
		açık" ekranı hesaplıyor (`httpapi/egress.go`). İki kopyayla yürüseydi
		biri değişip diğeri kalabilirdi ve ekran, kapının gerçekte izin
		verdiğinden başka bir şey gösterirdi.
	*/
	return hostlist.Hosts(adresler)
}
