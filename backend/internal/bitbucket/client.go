package bitbucket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var (
	// ErrGroupNotFound, grup yok veya görünmüyor.
	ErrGroupNotFound = errors.New("grup bulunamadı")

	// ErrForbidden, erişim reddedildi.
	ErrForbidden = errors.New("erişim reddedildi")

	// ErrUnreachable, sunucuya ulaşılamadı veya sunucu arızalı yanıt verdi.
	ErrUnreachable = errors.New("sunucuya ulaşılamıyor")

	// ErrBadResponse, yanıt beklenen biçimde değil.
	ErrBadResponse = errors.New("sunucu beklenmedik bir yanıt verdi")
)

const (
	// sayfaBoyutu, tek istekte istenecek kayıt sayısı.
	//
	// Sunucunun varsayılanı 25; büyük bir grupta bu, gereksiz yere dört kat
	// istek demek. 100, Bitbucket'ın kabul ettiği üst sınıra yakın ve
	// tek istekte biten grup sayısını artırıyor.
	sayfaBoyutu = 100

	// azamiSayfa, sayfalama döngüsünün üst sınırı.
	//
	// Bozuk bir sunucu `isLastPage: false` deyip aynı yeri göstermeye devam
	// edebilir. Sonsuza kadar istek atmak, hiç yanıt vermemekten kötüdür.
	azamiSayfa = 200

	// govdeIzi, hata mesajına eklenen ham gövdenin azami uzunluğu.
	govdeIzi = 200
)

// Credentials, Bitbucket'a gidecek kimlik bilgisi.
type Credentials struct {
	Username string
	Secret   string
}

// Repo, gruptaki bir repository.
type Repo struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	CloneURL string `json:"cloneUrl"`
	Archived bool   `json:"archived"`
}

// Client, kurumsal Bitbucket'ın okuma uçlarını çağırır.
type Client struct {
	http *http.Client
}

// NewClient yeni istemci üretir.
func NewClient(h *http.Client) *Client {
	if h == nil {
		h = http.DefaultClient
	}
	return &Client{http: h}
}

// sayfa, sayfalı yanıtın çerçevesi.
type sayfa struct {
	IsLastPage    bool      `json:"isLastPage"`
	NextPageStart *int      `json:"nextPageStart"`
	Values        []hamRepo `json:"values"`
}

// hamRepo, yalnızca SÜRÜMLER ARASI DEĞİŞMEYEN alanlar.
//
// Gerçek bir sunucuda ölçüm yapamadığımız için yüzey bilerek dar tutuluyor:
// `slug`, `name` ve `links.clone` Bitbucket Server 4.x'ten beri aynı.
// `archived` daha yeni; yoksa `false` kalır ve bu doğru varsayımdır.
type hamRepo struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
	Links    struct {
		Clone []struct {
			Href string `json:"href"`
			Name string `json:"name"`
		} `json:"clone"`
	} `json:"links"`
}

/*
ListRepos, grubun TÜM repository'lerini döner.

SAYFALAMA TÜKENENE KADAR DEVAM EDER. Tek çağrı yapan bir uygulama 60
repository'lik gruptan 25'ini alır ve başarılı görünür; sessizce eksik çalışan
bir kod, gürültülü çöken bir koddan tehlikelidir.
*/
func (c *Client) ListRepos(ctx context.Context, g GroupRef, creds Credentials) ([]Repo, error) {
	var (
		hepsi []Repo
		start int
	)

	for i := 0; i < azamiSayfa; i++ {
		s, err := c.sayfaCek(ctx, g, creds, start)
		if err != nil {
			return nil, err
		}

		for _, h := range s.Values {
			r, ok := repoDonustur(h)
			if ok {
				hepsi = append(hepsi, r)
			}
		}

		if s.IsLastPage || s.NextPageStart == nil {
			return hepsi, nil
		}
		// İlerlemeyen sayfalama: sunucu son sayfa değil diyor ama aynı yeri
		// gösteriyor. Döngüyü kırmak, aynı isteği sonsuza kadar tekrarlamaktan
		// iyidir.
		if *s.NextPageStart <= start {
			return nil, fmt.Errorf("%w: sayfalama ilerlemiyor", ErrBadResponse)
		}
		start = *s.NextPageStart
	}

	return nil, fmt.Errorf("%w: sayfa sınırı aşıldı", ErrBadResponse)
}

func (c *Client) sayfaCek(ctx context.Context, g GroupRef, creds Credentials, start int) (sayfa, error) {
	adres := g.reposURL() + "?limit=" + strconv.Itoa(sayfaBoyutu)
	if start > 0 {
		adres += "&start=" + strconv.Itoa(start)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, adres, nil)
	if err != nil {
		return sayfa{}, fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	if creds.Username != "" || creds.Secret != "" {
		req.SetBasicAuth(creds.Username, creds.Secret)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return sayfa{}, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	govde, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return sayfa{}, ErrGroupNotFound
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return sayfa{}, ErrForbidden
	case resp.StatusCode >= 400:
		return sayfa{}, fmt.Errorf("%w: sunucu %d döndü", ErrUnreachable, resp.StatusCode)
	}

	var s sayfa
	if err := json.Unmarshal(govde, &s); err != nil {
		// Ham gövde mesaja GİRER: gerçek sunucuda ölçüm yapamadığımız için
		// sürüm farkını ancak kullanıcının bildirimi ortaya çıkaracak ve o
		// bildirimde nedeni gösteren bir iz olmalı.
		return sayfa{}, fmt.Errorf("%w: %s", ErrBadResponse, kisalt(string(govde)))
	}
	return s, nil
}

/*
repoDonustur, ham kaydı ürünün beklediği biçime çevirir.

HTTP klonlama adresi yoksa repository ATLANIR (ikinci dönüş `false`):
kaydedilecek bir adres yok ve SSH bu ürünün kapsamında değil — runner
HTTPS + token ile klonluyor.
*/
func repoDonustur(h hamRepo) (Repo, bool) {
	adres := httpKlonAdresi(h)
	if adres == "" {
		return Repo{}, false
	}
	return Repo{
		Slug:     h.Slug,
		Name:     h.Name,
		CloneURL: adres,
		Archived: h.Archived,
	}, true
}

// httpKlonAdresi, `links.clone` içinden http olanı seçer ve gömülü kullanıcı
// adını ayıklar.
func httpKlonAdresi(h hamRepo) string {
	for _, k := range h.Links.Clone {
		ad := strings.ToLower(k.Name)
		if ad != "http" && ad != "https" {
			continue
		}
		if temiz := kimliksiz(k.Href); temiz != "" {
			return temiz
		}
	}
	return ""
}

/*
kimliksiz, adresteki gömülü kullanıcı adını/parolayı atar.

Bitbucket klonlama adresini çoğu kurulumda `https://ahmet@sunucu/...`
biçiminde veriyor. `projects.Input.Normalize` gömülü kimlik taşıyan adresi
REDDEDİYOR; ayıklanmasaydı içe aktarmanın tamamı, üstelik kullanıcının
anlamayacağı bir mesajla başarısız olurdu.
*/
func kimliksiz(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	u.User = nil
	return u.String()
}

func kisalt(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > govdeIzi {
		return s[:govdeIzi] + "…"
	}
	return s
}
