/*
Package bitbucket, kurumsal Bitbucket (Data Center) kurulumlarından okuma yapar.

`gitprovider` erişim TANIMINI yönetir; bu paket o erişimle VERİ okur. İkisi
ayrı duruyor çünkü aynı pakete konsalardı sağlayıcı doğrulaması depo
listelemeye bağlanırdı.

BULUT KAPSAM DIŞI (spec 021): Bitbucket Cloud'un hiyerarşisi ve şeması farklı;
bulut adresleri burada ayrı bir hatayla reddedilir, yanlış uca gönderilmez.

ÖLÇÜLMEDİ: bu paketin varsaydığı yanıt biçimleri Atlassian belgelerine
dayanıyor, gerçek bir sunucuya karşı ölçüme değil. Bu yüzden yalnızca sürümler
arası değişmeyen alanlara bağlanılıyor ve hatalar ham yanıtı saklamıyor —
sorunu bildirecek kişinin elinde nedeni gösteren bir iz kalmalı.
*/
package bitbucket

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/agent-coder/backend/internal/gitprovider"
)

var (
	// ErrNotGroupURL, verilen adres bir grup adresi değil.
	ErrNotGroupURL = errors.New("bu bir Bitbucket grup adresi değil")

	// ErrCloudAddress, verilen adres Bitbucket Cloud'a ait.
	ErrCloudAddress = errors.New("bu adres Bitbucket Cloud'a ait")
)

// ornekAdres, hata mesajlarında gösterilen beklenen biçim.
//
// Mesajda ÖRNEK var çünkü "grup adresi değil" tek başına ne yapılacağını
// söylemiyor; kullanıcının elinde depo adresi de var ve ikisi birbirine
// benziyor.
const ornekAdres = "https://bitbucket.sirket.com/projects/ANAHTAR"

// GroupRef, çözülmüş bir grup adresi.
type GroupRef struct {
	// BaseURL, sunucunun kökü — context path DAHİL, sonda eğik çizgi YOK.
	BaseURL string

	// Key, grup anahtarı. Kişisel alanlarda `~KULLANICI` biçiminde.
	Key string
}

/*
ParseGroupURL, tarayıcıdan kopyalanan grup adresini çözer.

Kullanıcı ayrı ayrı "sunucu adresi" ve "grup anahtarı" girmiyor; elindeki tek
şey adres çubuğundaki metin. Dolayısıyla sondaki eğik çizgi, derine inen yol,
sorgu ve çapa tolere edilir.

TABAN ADRES `/projects/{ANAHTAR}` PARÇASINDAN ÖNCESİDİR. Kurumsal kurulumların
çoğu kökte değil bir alt yolda duruyor; sabit host varsayan bir ayrıştırma o
kurulumlarda API'yi yanlış yere sorar ve kullanıcı sebebi anlaşılmayan bir 404
alır.
*/
func ParseGroupURL(raw string) (GroupRef, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return GroupRef{}, fmt.Errorf("%w: adres boş", ErrNotGroupURL)
	}

	/*
	 * BULUT DENETİMİ ÖNCE. Bulut adresleri de `/projects/…` içerebiliyor;
	 * sıra ters olsaydı kullanıcı "bu yol kurumsal kurulumlar için" mesajı
	 * yerine anlamsız bir grup hatası alırdı (spec 021 H4).
	 */
	if gitprovider.IsCloudHost(raw) {
		return GroupRef{}, ErrCloudAddress
	}

	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return GroupRef{}, fmt.Errorf(
			"%w — beklenen biçim: %s", ErrNotGroupURL, ornekAdres)
	}

	parcalar := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parcalar {
		kisisel := strings.EqualFold(p, "users")
		if !kisisel && !strings.EqualFold(p, "projects") {
			continue
		}
		// Ayraçtan sonra anahtar gelmiyorsa bu bir grup adresi değil:
		// `/projects` tek başına grup listesi sayfası.
		if i+1 >= len(parcalar) || parcalar[i+1] == "" {
			break
		}
		return grupRef(u, parcalar[:i], parcalar[i+1], kisisel), nil
	}

	return GroupRef{}, fmt.Errorf("%w — beklenen biçim: %s", ErrNotGroupURL, ornekAdres)
}

/*
grupRef, ayraçtan önceki yolu taban adres, sonrasını anahtar yapar.

İLK eşleşme kullanılır: standart yerleşimde `/projects` context path'in hemen
ardından gelir. Sondan aramak, adı `projects` olan bir repository'de
(`…/projects/ODEME/repos/projects/browse`) yanlış anahtarı seçerdi.
*/
func grupRef(u *url.URL, onek []string, anahtar string, kisisel bool) GroupRef {
	taban := &url.URL{Scheme: u.Scheme, Host: u.Host}
	if len(onek) > 0 {
		taban.Path = "/" + strings.Join(onek, "/")
	}

	if kisisel {
		// Kişisel alanın proje anahtarı `~KULLANICI` biçiminde saklanıyor.
		anahtar = "~" + strings.ToUpper(anahtar)
	}
	return GroupRef{BaseURL: taban.String(), Key: anahtar}
}

// reposURL, grubun repository listesi ucu.
func (g GroupRef) reposURL() string {
	return strings.TrimRight(g.BaseURL, "/") +
		"/rest/api/1.0/projects/" + url.PathEscape(g.Key) + "/repos"
}
