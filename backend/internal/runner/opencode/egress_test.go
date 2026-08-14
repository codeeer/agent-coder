package opencode

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/hostlist"
	"github.com/agent-coder/backend/internal/runner"
)

/*
 * İzin listesinin kurulmasındaki TUZAK ve testi.
 *
 * Boş whitelist "kısıt yok" demektir (spec 020). Zorunlu adresler körlemesine
 * eklenseydi boş liste boş olmaktan çıkardı ve kısıtsız olması gereken bir
 * çalıştırma yalnızca o üç beş adresle sınırlanırdı — üstelik kullanıcı hiçbir
 * şey yazmadığı için sebebini hiç anlamazdı.
 */
func TestEgressAllow_BosWhitelistKisitUygulanmaz(t *testing.T) {
	desenler, err := egressAllow(runner.EgressSpec{
		ProxyURL: "http://proxy:8080",
		Required: []string{"openrouter.ai", "github.com"},
	})
	require.NoError(t, err)
	require.Empty(t, desenler, "boş whitelist kısıtsızlıktır")

	// Boş desen listesi her host'u geçirir — kural burada da tutmalı.
	require.True(t, hostlist.Match(desenler, "herhangi.com"))
}

// Whitelist doluyken zorunlu adresler EKLENİR: kullanıcı sağlayıcı ve
// repository adresini ayarlara zaten girmiş, ayrıca yazmasını beklemek
// gereksiz tekrar olurdu (spec 020 H4).
func TestEgressAllow_DoluWhitelisteZorunlularEklenir(t *testing.T) {
	desenler, err := egressAllow(runner.EgressSpec{
		ProxyURL:     "http://proxy:8080",
		AllowedHosts: "repo1.maven.org",
		Required:     []string{"openrouter.ai", "git.sirket.local"},
	})
	require.NoError(t, err)

	require.True(t, hostlist.Match(desenler, "repo1.maven.org"), "kullanıcının satırı")
	require.True(t, hostlist.Match(desenler, "openrouter.ai"), "provider zorunlu izinli")
	require.True(t, hostlist.Match(desenler, "git.sirket.local"), "repository zorunlu izinli")
	require.False(t, hostlist.Match(desenler, "baska.com"), "listede olmayan kapalı")
}

/*
 * Motorun kendi adresleri de eklenir.
 *
 * ÖLÇÜLDÜ (docs/veri-sizintisi-analizi.md, bulgu 4): opencode her çalıştırmada
 * models.opencode.ai'den katalog çekiyor ve GitHub'dan bir yardımcı program
 * indiriyor. Bunlar engellenirse motor hiç açılmaz — kullanıcı da sebebini
 * bulmak için deneme yanılmaya mahkûm kalırdı.
 */
func TestEgressAllow_MotorunKendiAdresleriEklenir(t *testing.T) {
	desenler, err := egressAllow(runner.EgressSpec{
		ProxyURL:     "http://proxy:8080",
		AllowedHosts: "repo1.maven.org",
	})
	require.NoError(t, err)

	require.True(t, hostlist.Match(desenler, "models.opencode.ai"))
	require.True(t, hostlist.Match(desenler, "github.com"))
	require.True(t, hostlist.Match(desenler, "release-assets.githubusercontent.com"))
}

func TestEgressAllow_GecersizSatirHataDoner(t *testing.T) {
	_, err := egressAllow(runner.EgressSpec{
		ProxyURL:     "http://proxy:8080",
		AllowedHosts: "https://bozuk.com",
	})
	require.Error(t, err)
}

// Denetim açıkken proxy değişkenleri KAPIYI gösterir, kurumsal proxy'yi değil:
// whitelist kapıda uygulanıyor, doğrudan kurumsal proxy'ye gidilseydi liste
// hiçbir şey yapmazdı.
func TestBuildEnv_DenetimAcikkenKapiyiGosterir(t *testing.T) {
	env := buildEnv(runner.Request{
		Egress: runner.EgressSpec{ProxyURL: "http://kurumsal:3128"},
	}, "http://backend:8090")

	require.Equal(t, "http://backend:8090", env["HTTPS_PROXY"])
	require.Equal(t, "http://backend:8090", env["https_proxy"])
	require.Contains(t, env["MAVEN_OPTS"], "-Dhttps.proxyHost=backend")
	require.NotContains(t, env["HTTPS_PROXY"], "kurumsal")
}

// Denetim kapalıyken hiçbir proxy değişkeni yazılmaz — bugünkü davranış aynen
// sürmeli.
func TestBuildEnv_DenetimKapaliykenDegiskenYazilmaz(t *testing.T) {
	env := buildEnv(runner.Request{}, "")

	require.NotContains(t, env, "HTTPS_PROXY")
	require.NotContains(t, env, "NO_PROXY")
	require.NotContains(t, env, "MAVEN_OPTS")
}

/*
 * Tekrarlı ret olay akışını BOĞMAZ.
 *
 * Bir agent aynı adrese onlarca kez deneyebiliyor (paket yöneticileri yeniden
 * dener). Her deneme ayrı satır olsaydı çalıştırma ekranı okunamaz hâle gelir
 * ve asıl olaylar kaybolurdu. Buna karşılık ret'in HİÇ görünmemesi de olmaz —
 * kullanıcı whitelist'e ne ekleyeceğini oradan öğreniyor.
 */
func TestDenyBildirici_TekrarAkisiBogmaz(t *testing.T) {
	var olaylar []runner.Event
	bildir := denyBildirici("k1", func(e runner.Event) { olaylar = append(olaylar, e) })

	for i := 0; i < 50; i++ {
		bildir("yasak.com")
	}

	require.NotEmpty(t, olaylar, "ret hiç görünmemezlik edemez")
	require.Less(t, len(olaylar), 5, "50 deneme için 50 satır yazılmamalı")
	require.Contains(t, olaylar[0].Message, "yasak.com")
	require.Equal(t, runner.LevelWarn, olaylar[0].Level)
}

// Farklı host'lar ayrı ayrı bildirilir — biri diğerini bastırmamalı.
func TestDenyBildirici_FarkliHostlarAyriBildirilir(t *testing.T) {
	var olaylar []runner.Event
	bildir := denyBildirici("k1", func(e runner.Event) { olaylar = append(olaylar, e) })

	bildir("bir.com")
	bildir("iki.com")

	require.Len(t, olaylar, 2)
}

// Uyarı, kullanıcıya NE YAPACAĞINI söylemeli: whitelist'e eklemek.
func TestDenyBildirici_MesajYapilacagiSoyler(t *testing.T) {
	var olaylar []runner.Event
	bildir := denyBildirici("k1", func(e runner.Event) { olaylar = append(olaylar, e) })
	bildir("yasak.com")

	require.Contains(t, olaylar[0].Message, "izinli domain")
}

func TestUpstreamAdresi(t *testing.T) {
	adres, err := upstreamAdresi("http://proxy.sirket.local:8080")
	require.NoError(t, err)
	require.Equal(t, "proxy.sirket.local:8080", adres)

	// Port yazılmamışsa şemanın varsayılanı kullanılır — kullanıcıyı port
	// yazmaya zorlamak gereksiz bir tökezleme noktası olurdu.
	adres, err = upstreamAdresi("http://proxy.sirket.local")
	require.NoError(t, err)
	require.Equal(t, "proxy.sirket.local:80", adres)

	_, err = upstreamAdresi("proxy.sirket.local:8080")
	require.Error(t, err, "şemasız adres reddedilmeli")
}
