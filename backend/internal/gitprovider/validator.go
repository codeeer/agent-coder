package gitprovider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Validator, erişim bilgisinin gerçekten çalıştığını sınar.
type Validator struct {
	http *http.Client
}

// NewValidator yeni doğrulayıcı üretir.
// rt nil ise varsayılan taşıyıcı kullanılır (bkz. tlstrust).
func NewValidator(rt http.RoundTripper) *Validator {
	return &Validator{http: &http.Client{Timeout: 15 * time.Second, Transport: rt}}
}

// Validate, türe uygun doğrulama isteğini atar.
//
// Genel Git türünde doğrulama mümkün değildir: elimizde yalnızca bir depo
// adresi ve kimlik bilgisi vardır, sorgulanacak standart bir API ucu yoktur.
// Bu durumda ErrNotVerifiable döner ve çağıran kaydı yine de yapar (spec 002 H5).
func (v *Validator) Validate(ctx context.Context, p Provider, secret string) error {
	switch p.Type {
	case TypeGitHub:
		return v.validateGitHub(ctx, p, secret)
	case TypeBitbucket:
		return v.validateBitbucket(ctx, p, secret)
	case TypeGeneric:
		return ErrNotVerifiable
	default:
		return fmt.Errorf("%w: %q", ErrInvalidType, p.Type)
	}
}

func (v *Validator) validateGitHub(ctx context.Context, p Provider, secret string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.APIURL()+"/user", nil)
	if err != nil {
		return fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Accept", "application/vnd.github+json")

	return v.check(req)
}

/*
 * validateBitbucket, Cloud ve kurumsal Server/DC kurulumlarını ayırır.
 *
 * İkisinin API şeması FARKLI: Cloud'da `/2.0/user` var, Bitbucket Server'da yok
 * — orada şema `/rest/api/1.0/...` biçiminde. Tek uca gitmek, kendi sunucusunu
 * kullanan herkese 404 döndürüyordu ve 404 kimlik hatası sayıldığı için
 * kullanıcı doğru token'ını yanlış sanıp boşuna yeniliyordu.
 *
 * Ayrım TÜR ile değil ADRES ile yapılır: yeni bir sağlayıcı türü eklemek
 * kullanıcıyı "hangisini seçmeliyim" sorusuyla baş başa bırakırdı, oysa adres
 * zaten cevabı içeriyor.
 */
func (v *Validator) validateBitbucket(ctx context.Context, p Provider, secret string) error {
	if p.Username == "" {
		return ErrMissingUsername
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bitbucketProbe(p.APIURL()), nil)
	if err != nil {
		return fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	// İki modda da Basic Auth: Cloud'da kullanıcı adı + app password,
	// Server'da kullanıcı adı + kişisel erişim anahtarı (PAT) çifti çalışıyor.
	req.SetBasicAuth(p.Username, secret)
	req.Header.Set("Accept", "application/json")

	return v.check(req)
}

// bitbucketProbe, doğrulama için çağrılacak uç.
//
// Server tarafında `projects` seçildi: her kurulumda var, okuma yetkisi yeten
// en ucuz uç ve `limit=1` ile tek kayıt döner — doğrulama için bir kaydın
// gelmesi bile gerekmez, 200 yeterlidir.
func bitbucketProbe(apiURL string) string {
	if bitbucketCloud(apiURL) {
		return apiURL + "/user"
	}
	return apiURL + "/rest/api/1.0/projects?limit=1"
}

// bitbucketCloud, adresin Bitbucket Cloud'a ait olup olmadığı.
//
// Karşılaştırma HOST üzerinden yapılır, ham metin üzerinden değil:
// `https://bitbucket.sirket.local/api.bitbucket.org/x` gibi bir yol düz metin
// aramasıyla yanlışlıkla Cloud sayılır ve kurumsal kurulum yine yanlış uca
// giderdi. Adres ayrıştırılamazsa metin karşılaştırmasına düşülür — orada da
// yanılmak, hiç karar verememekten iyidir.
func bitbucketCloud(apiURL string) bool {
	// Adres boşsa APIURL() zaten Cloud varsayılanını vermiştir.
	if strings.TrimSpace(apiURL) == "" {
		return true
	}

	if u, err := url.Parse(apiURL); err != nil || u.Host == "" {
		return strings.Contains(apiURL, bitbucketCloudHost)
	}
	return IsCloudHost(apiURL)
}

const bitbucketCloudHost = "api.bitbucket.org"

/*
 * IsCloudHost, adresin Bitbucket Cloud'a ait olup olmadığını söyler.
 *
 * `bitbucketCloud`'dan AYRI ve dışa açık: orası API adresine bakıyor ve boş
 * adresi "Cloud varsayılanı" sayıyor; burası ise kullanıcının tarayıcıdan
 * kopyaladığı herhangi bir adrese bakıyor ve boşu Cloud saymıyor. İkisi aynı
 * host kuralını paylaşsın diye kural tek yerde.
 *
 * `api.bitbucket.org` ayrıca yazılmıyor — o zaten `bitbucket.org`'un alt alan
 * adı. Kullanıcı tarayıcıda `bitbucket.org` görüyor, API ise
 * `api.bitbucket.org` kullanıyor; ikisi de Cloud.
 *
 * Karşılaştırma HOST üzerinden, metin araması DEĞİL:
 * `https://bitbucket.sirket.local/api.bitbucket.org/x` düz metin aramasıyla
 * yanlışlıkla Cloud sayılırdı.
 */
func IsCloudHost(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return false
	}

	host := strings.ToLower(u.Hostname())
	const cloud = "bitbucket.org"
	return host == cloud || strings.HasSuffix(host, "."+cloud)
}

// check, doğrulama isteğinin sonucunu ortak kurallara göre yorumlar.
func (v *Validator) check(req *http.Request) error {
	resp, err := v.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return ErrInvalidSecret
	case resp.StatusCode == http.StatusNotFound:
		/*
		 * 404 bir KİMLİK hatası değil, ADRES hatasıdır.
		 *
		 * Sunucu isteği aldı ve "burada böyle bir uç yok" dedi — token'a
		 * bakmadı bile. ErrInvalidSecret'e eşlendiği sürece kullanıcı
		 * "erişim bilgisi doğrulanamadı" görüyor ve doğru token'ını
		 * boşuna yeniliyordu; asıl sorun yanlış adres ya da API şemasıydı.
		 *
		 * ErrInvalidSecret ile BİRLİKTE sarmalanmaz: respondGitError'da
		 * ErrInvalidSecret dalı önce geldiği için yine kimlik hatası
		 * raporlanırdı.
		 */
		return fmt.Errorf("%w: sunucu bu adreste API sunmuyor (404)", ErrInvalidBaseURL)
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: sunucu %d döndü", ErrUnreachable, resp.StatusCode)
	default:
		return fmt.Errorf("%w: beklenmeyen durum %d", ErrUnreachable, resp.StatusCode)
	}
}
