package runs

import (
	"net/url"

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

	gorulen := map[string]bool{}
	var hostlar []string
	for _, a := range adresler {
		h := hostAdi(a)
		if h == "" || gorulen[h] {
			continue
		}
		gorulen[h] = true
		hostlar = append(hostlar, h)
	}
	return hostlar
}

// hostAdi, bir adresten host kısmını çıkarır. Ayrıştırılamayan veya host
// taşımayan değer için boş döner.
func hostAdi(adres string) string {
	if adres == "" {
		return ""
	}
	u, err := url.Parse(adres)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
