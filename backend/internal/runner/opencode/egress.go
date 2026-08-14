package opencode

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"sync"

	"github.com/agent-coder/backend/internal/hostlist"
	"github.com/agent-coder/backend/internal/netgate"
	"github.com/agent-coder/backend/internal/runner"
)

/*
 * engineHosts, opencode'un KENDİ çalışması için gereken adresler.
 *
 * Burada duruyorlar çünkü opencode'a özgüler — `internal/runner` sınırının
 * dışına sızmamaları gerekiyor (AGENTS.md). Motor değişirse bu liste de
 * değişir.
 *
 * ÖLÇÜLDÜ, tahmin değil (docs/veri-sizintisi-analizi.md, bulgu 4): opencode
 * her çalıştırmada models.opencode.ai'den 3,6 MB katalog çekiyor ve GitHub'dan
 * ripgrep ikilisini indiriyor. Bunlar kapalıyken motor hiç açılmaz.
 *
 * BİLİNEN GENİŞLİK: github.com tek bir indirme için açılıyor ama GitHub'ın
 * tamamını açıyor — kapı TLS açmadığı için yola değil yalnızca host'a
 * bakabiliyor. Bu sürümde kabul edildi; çözümü ripgrep'i imaja gömüp bu satırı
 * kaldırmak (spec 020 → "Sonraya bırakılan").
 */
var engineHosts = []string{
	"models.opencode.ai",
	"github.com",
	"release-assets.githubusercontent.com",
}

/*
 * egressAllow, bir çalıştırmanın izin listesini kurar.
 *
 * TUZAK — boş whitelist: spec 020'ye göre boş liste "kısıt yok" demektir.
 * Zorunlu adresler körlemesine eklenseydi liste boş olmaktan çıkar ve
 * kısıtsız olması gereken çalıştırma yalnızca o adreslerle sınırlanırdı;
 * üstelik kullanıcı hiçbir şey yazmadığı için sebebini hiç anlamazdı.
 * Bu yüzden zorunlular YALNIZCA kullanıcı bir şey yazmışsa eklenir.
 */
func egressAllow(spec runner.EgressSpec) ([]hostlist.Pattern, error) {
	desenler, err := hostlist.Parse(spec.AllowedHosts)
	if err != nil {
		return nil, err
	}
	if len(desenler) == 0 {
		return nil, nil
	}

	zorunlu := append(append([]string{}, spec.Required...), engineHosts...)
	for _, h := range zorunlu {
		if h == "" {
			continue
		}
		ek, err := hostlist.Parse(h)
		if err != nil {
			/*
			 * Ayrıştırılamayan zorunlu adres atlanır ama SESSİZ KALMAZ.
			 *
			 * Sessizliğin bedeli ölçüldü: tek parçalı bir depo adresi
			 * (`sizinti-depo`) ayrıştırılamadığı için listeye giremedi,
			 * klonlama reddedildi ve ortada yalnızca "klonlama başarısız"
			 * yazıyordu. Sebebini backend logunda aramak gerekti.
			 */
			slog.Warn("zorunlu izinli adres ayrıştırılamadı — bu adrese çıkış engellenecek",
				"host", h, "error", err)
			continue
		}
		desenler = append(desenler, ek...)
	}
	return desenler, nil
}

/*
 * upstreamAdresi, ayardaki proxy URL'ini `host:port` biçimine çevirir.
 *
 * Port yazılmamışsa şemanın varsayılanı kullanılır: kullanıcıyı `:80` yazmaya
 * zorlamak gereksiz bir tökezleme noktası olurdu ve hata mesajı da bunu
 * anlatmak zorunda kalırdı.
 */
func upstreamAdresi(proxyURL string) (string, error) {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return "", fmt.Errorf("çıkış proxy adresi ayrıştırılamadı: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("çıkış proxy adresi http:// veya https:// ile başlamalı")
	}
	if u.Hostname() == "" {
		return "", fmt.Errorf("çıkış proxy adresinde sunucu adı yok")
	}

	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	return net.JoinHostPort(u.Hostname(), port), nil
}

/*
 * denyBildirici, reddedilen çıkışları hem loga hem çalıştırma olay akışına yazar.
 *
 * İKİ SINIR birden gözetiliyor:
 *
 *   TEKRAR — bir agent aynı adrese onlarca kez gidebiliyor (paket yöneticileri
 *   yeniden dener). Her deneme ayrı satır olsaydı çalıştırma ekranı okunamaz
 *   hâle gelir ve asıl olaylar kaybolurdu. İlk denemede yazılır, sonra
 *   seyrekleşerek — ama kaç kez denendiği yazıldığı için bilgi kaybolmaz.
 *
 *   SIRALILIK — `runner.EventFunc` sözleşmesi emit'in birden çok goroutine'den
 *   çağrılmamasını şart koşuyor; kapı ise her isteği kendi goroutine'inde
 *   karşılıyor. Kilit emit çağrısını da kapsıyor.
 *
 * Log HER denemede yazılır: orada gürültü sorun değil, eksik kayıt sorundur.
 */
func denyBildirici(runID string, emit runner.EventFunc) func(string) {
	var mu sync.Mutex
	sayac := map[string]int{}

	return func(host string) {
		mu.Lock()
		defer mu.Unlock()

		sayac[host]++
		n := sayac[host]

		slog.Warn("sandbox çıkışı engellendi",
			"run_id", runID, "host", host, "deneme", n)

		if n != 1 && n != 10 && n%100 != 0 {
			return
		}

		mesaj := fmt.Sprintf(
			"çıkış engellendi: %s — erişim gerekiyorsa Ayarlar → Kurumsal ağ → "+
				"izinli domain listesine ekleyin", host)
		if n > 1 {
			mesaj = fmt.Sprintf("%s (%d. deneme)", mesaj, n)
		}
		emit(runner.Event{Level: runner.LevelWarn, Message: mesaj})
	}
}

// egressOturumuAc, çalıştırma için çıkış oturumu açar.
func (r *Runner) egressOturumuAc(req runner.Request, emit runner.EventFunc) (*netgate.Session, error) {
	if r.gate == nil {
		return nil, fmt.Errorf("çıkış kapısı yapılandırılmamış")
	}

	allow, err := egressAllow(req.Egress)
	if err != nil {
		return nil, err
	}
	upstream, err := upstreamAdresi(req.Egress.ProxyURL)
	if err != nil {
		return nil, err
	}

	return r.gate.Open(netgate.Run{
		ID:       req.RunID.String(),
		Upstream: upstream,
		Allow:    allow,
		OnDeny:   denyBildirici(req.RunID.String(), emit),
	})
}
