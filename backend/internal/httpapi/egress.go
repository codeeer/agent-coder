package httpapi

import (
	"context"
	"net/http"
	"net/url"

	"github.com/agent-coder/backend/internal/hostlist"
	"github.com/agent-coder/backend/internal/paging"
	"github.com/agent-coder/backend/internal/settings"
)

/*
 * Çıkış denetiminin durumu (spec 020 H4).
 *
 * Bu uç KAYDETMEZ. Proxy ve izinli domain listesi diğer bütün ayarlarla aynı
 * yerden yazılıyor (`PUT /api/settings/...`); buraya ikinci bir yazma yolu
 * koymak aynı değerin iki ayrı doğrulamadan geçmesi demekti — sertifikadaki
 * kararın aynısı.
 *
 * VAR OLMA SEBEBİ: ürün, kullanıcı yazmasa da bazı adreslere izin veriyor
 * (LLM provider, git repository, registry, motorun kendi ihtiyaçları).
 * Kullanıcının bilmediği açık bir kapı bırakılmaz — bu uç o kapıları
 * gösteriyor. Liste UYDURULMAZ, gerçek yapılandırmadan türetilir.
 */

type egressProxyInfo struct {
	// Source: "settings" | "env" | "none" — sertifikadaki üç durumun aynısı.
	Source string `json:"source"`
	// Host, proxy'nin adresi (host:port). Kaynak "none" ise boş.
	Host string `json:"host"`
}

type egressAllowed struct {
	// Engine, çalıştırma motorunun kendi çalışması için gerekenler.
	Engine []string `json:"engine"`
	// Providers, tanımlı LLM sağlayıcıların adresleri.
	Providers []string `json:"providers"`
	// Repositories, tanımlı projelerin depo adresleri.
	Repositories []string `json:"repositories"`
	// Registries, tanımlıysa paket deposu adresleri.
	Registries []string `json:"registries"`
}

type egressResponse struct {
	Proxy egressProxyInfo `json:"proxy"`
	// AlwaysAllowed, kullanıcı yazmasa da izinli olan adresler.
	AlwaysAllowed egressAllowed `json:"alwaysAllowed"`

	/*
	 * InternalHosts, proxy'ye UĞRAMADAN gidilen adresler (spec 026).
	 *
	 * `AlwaysAllowed`'ın yanında ama ONUN PARÇASI DEĞİL: o liste "kullanıcı
	 * yazmasa da izinli" der, bu liste izinle hiç ilgilenmez. İç içe
	 * konsaydı ekran ikisini aynı soruya cevap sanırdı.
	 *
	 * Bu adresler kurumsal proxy'nin kaydından ve denetiminden çıkıyor;
	 * ekranda görünmeleri tam da bu yüzden gerekli.
	 */
	InternalHosts []string `json:"internalHosts"`
}

// egressStatus, çıkış denetiminin çözülmüş durumunu döner.
func (h *Handler) egressStatus(w http.ResponseWriter, r *http.Request) {
	out := egressResponse{
		Proxy:         egressProxyInfo{Source: "none"},
		AlwaysAllowed: egressAllowed{},
	}

	// Ayar kazanır, ortam değişkeni yedekte kalır (spec 017 kalıbı).
	ayar := ""
	if h.deps.Settings != nil {
		ayar = h.deps.Settings.Text(settings.KeyEgressProxy)
	}
	switch {
	case ayar != "":
		out.Proxy = egressProxyInfo{Source: "settings", Host: proxyHostu(ayar)}
	case h.deps.EgressProxyEnv != "":
		out.Proxy = egressProxyInfo{Source: "env", Host: proxyHostu(h.deps.EgressProxyEnv)}
	}

	/*
	 * Kurum içi domain'ler AYRIŞTIRILARAK gösteriliyor (spec 026).
	 *
	 * Ham metni bölüp göstermek daha kolaydı ama ekran o zaman kapının
	 * gerçekte kullandığı listeyi değil, kullanıcının yazdığı metni
	 * gösterirdi — yorum satırları ve atlanan girdiler dahil. Bu dosyanın
	 * kuralı: "Liste UYDURULMAZ, gerçek yapılandırmadan türetilir."
	 */
	if h.deps.Settings != nil {
		if desenler, err := hostlist.Parse(
			h.deps.Settings.Text(settings.KeyInternalHosts)); err == nil {
			out.InternalHosts = hostlist.Strings(desenler)
		}
	}

	if h.deps.EngineHosts != nil {
		out.AlwaysAllowed.Engine = h.deps.EngineHosts()
	}
	out.AlwaysAllowed.Providers = h.saglayiciHostlari(r.Context())
	out.AlwaysAllowed.Repositories = h.depoHostlari(r.Context())
	out.AlwaysAllowed.Registries = h.registryHostlari()

	respondJSON(w, http.StatusOK, out)
}

func (h *Handler) saglayiciHostlari(ctx context.Context) []string {
	if h.deps.LLMProviders == nil {
		return nil
	}
	list, err := h.deps.LLMProviders.List(ctx)
	if err != nil {
		return nil
	}
	adresler := make([]string, 0, len(list))
	for _, p := range list {
		adresler = append(adresler, p.BaseURL)
	}
	return hostlist.Hosts(adresler)
}

func (h *Handler) depoHostlari(ctx context.Context) []string {
	if h.deps.Projects == nil {
		return nil
	}
	/*
	 * Sayfalama sınırı BİLEREK yüksek: bu liste "hangi kapılar açık"
	 * sorusunun cevabı ve eksik göstermek, kullanıcının bilmediği bir kapı
	 * bırakmak demek. Proje sayısı bu ölçeğe yaklaşırsa liste zaten
	 * okunamaz olur ve ayrı bir tasarım gerekir.
	 */
	list, _, err := h.deps.Projects.List(ctx, paging.Page{Limit: 500})
	if err != nil {
		return nil
	}
	adresler := make([]string, 0, len(list))
	for _, p := range list {
		adresler = append(adresler, p.RepoURL)
	}
	return hostlist.Hosts(adresler)
}

func (h *Handler) registryHostlari() []string {
	if h.deps.Settings == nil {
		return nil
	}
	return hostlist.Hosts([]string{
		h.deps.Settings.Text(settings.KeyNPMRegistry),
		h.deps.Settings.Text(settings.KeyMavenRegistry),
	})
}

/*
 * proxyHostu, proxy adresinden host:port kısmını çıkarır.
 *
 * Tam URL göstermek gereksiz gürültü: şema hep http(s) ve yol yok.
 *
 * `hostlist.Host` DEĞİL, çünkü ikisi farklı iş yapıyor: orası izin kararına
 * giren adı üretir ve portu atar — izinli bir domain'e tüm portlar açık
 * olduğu için port oraya girmemeli. Burası ise yalnızca ekrana yazılan bir
 * etiket ve portu göstermek gerekiyor; ayrıştırılamayan değer de gizlenmeyip
 * olduğu gibi yazılıyor, kullanıcı ayara ne girdiğini görebilsin.
 */
func proxyHostu(adres string) string {
	u, err := url.Parse(adres)
	if err != nil || u.Host == "" {
		return adres
	}
	return u.Host
}
