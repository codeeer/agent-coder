package bitbucket

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Repository listesinin çekilmesi.
 *
 * SAHTE SUNUCU KENDİ KODUMUZU DOĞRULAR, Atlassian'ın gerçek yanıtını değil.
 * Elimizde ölçüm yapılacak bir kurumsal sunucu yok (spec 021 → Belirsizlikler);
 * varsayımımız yanlışsa bu sunucu aynı yanlışı tekrarlar. Bu yüzden testler
 * "gerçek sunucuda doğrulandı" kanıtı sayılmaz — kilitledikleri şey sayfalama
 * döngüsü, adres seçimi ve hata sınıflandırması gibi BİZE ait davranışlar.
 */

// repoJSON, belgelenmiş repository nesnesinin sınadığımız alanları.
func repoJSON(slug string, kloneler ...map[string]string) map[string]any {
	return map[string]any{
		"slug":  slug,
		"name":  slug + " adı",
		"links": map[string]any{"clone": kloneler},
	}
}

// sayfaRepo, sayfalama testinde kullanılan, klonlanabilir bir kayıt.
func sayfaRepo(i int) map[string]any {
	slug := fmt.Sprintf("depo-%02d", i)
	return repoJSON(slug, httpKlon("https://bb/scm/ODEME/"+slug+".git"))
}

func httpKlon(href string) map[string]string {
	return map[string]string{"href": href, "name": "http"}
}

func sunucu(t *testing.T, h http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s, NewClient(s.Client())
}

func grup(s *httptest.Server) GroupRef {
	return GroupRef{BaseURL: s.URL, Key: "ODEME"}
}

func TestListRepos_TekSayfa(t *testing.T) {
	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/api/1.0/projects/ODEME/repos", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]any{
			"isLastPage": true,
			"values": []any{
				repoJSON("api", httpKlon("https://bb/scm/ODEME/api.git")),
				repoJSON("web", httpKlon("https://bb/scm/ODEME/web.git")),
				repoJSON("job", httpKlon("https://bb/scm/ODEME/job.git")),
			},
		})
	})

	repos, err := c.ListRepos(context.Background(), grup(s), Credentials{})

	require.NoError(t, err)
	require.Len(t, repos, 3)
	require.Equal(t, "api", repos[0].Slug)
	require.Equal(t, "api adı", repos[0].Name)
	require.Equal(t, "https://bb/scm/ODEME/api.git", repos[0].CloneURL)
}

/*
 * SAYFALAMA TÜKENENE KADAR DEVAM EDER.
 *
 * Varsayılan sayfa boyutu 25. Tek çağrı yapan bir uygulama 30 repository'lik
 * gruptan 25'ini alır ve "25 proje eklendi" der — kimse hata görmez. Sessizce
 * eksik çalışan bir kod, gürültülü çöken bir koddan tehlikelidir; bu test onu
 * düşürmek için var.
 */
func TestListRepos_IkiSayfaBirlestirilir(t *testing.T) {
	var istekler []string

	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		istekler = append(istekler, r.URL.RawQuery)

		start, _ := strconv.Atoi(r.URL.Query().Get("start"))
		if start == 0 {
			var ilk []any
			for i := 0; i < 25; i++ {
				ilk = append(ilk, sayfaRepo(i))
			}
			json.NewEncoder(w).Encode(map[string]any{
				"isLastPage":    false,
				"nextPageStart": 25,
				"values":        ilk,
			})
			return
		}

		var ikinci []any
		for i := 25; i < 30; i++ {
			ikinci = append(ikinci, sayfaRepo(i))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"isLastPage": true,
			"values":     ikinci,
		})
	})

	repos, err := c.ListRepos(context.Background(), grup(s), Credentials{})

	require.NoError(t, err)
	require.Len(t, repos, 30, "ikinci sayfa alınmadıysa 25 kalır")
	require.Len(t, istekler, 2)
	require.Contains(t, istekler[1], "start=25", "nextPageStart kullanılmalı")
}

// nextPageStart ilerlemiyorsa döngü kırılır: bozuk bir sunucu yüzünden
// sonsuza kadar istek atmak, hiç yanıt vermemekten kötüdür.
func TestListRepos_IlerlemeyenSayfalamaDurur(t *testing.T) {
	var sayac int

	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		sayac++
		json.NewEncoder(w).Encode(map[string]any{
			"isLastPage":    false,
			"nextPageStart": 0, // hep aynı yer
			"values":        []any{repoJSON("api")},
		})
	})

	_, err := c.ListRepos(context.Background(), grup(s), Credentials{})

	require.Error(t, err)
	require.Less(t, sayac, 5, "döngü erken kırılmalı")
}

/*
 * Klonlama adresi: `links.clone` içinden HTTP olan seçilir.
 *
 * SSH seçilemez — runner HTTPS + token ile klonluyor ve SSH anahtarı yönetimi
 * bu ürünün kapsamında değil.
 */
func TestListRepos_HTTPKlonuSecilir(t *testing.T) {
	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"isLastPage": true,
			"values": []any{
				repoJSON("api",
					map[string]string{"href": "ssh://git@bb:7999/odeme/api.git", "name": "ssh"},
					httpKlon("https://bb/scm/ODEME/api.git"),
				),
			},
		})
	})

	repos, err := c.ListRepos(context.Background(), grup(s), Credentials{})

	require.NoError(t, err)
	require.Equal(t, "https://bb/scm/ODEME/api.git", repos[0].CloneURL)
}

/*
 * ADRESE GÖMÜLÜ KULLANICI ADI AYIKLANIR.
 *
 * Bitbucket klonlama adresini çoğu kurulumda `https://ahmet@sunucu/...`
 * biçiminde veriyor. `projects.Input.Normalize` gömülü kimlik taşıyan adresi
 * REDDEDİYOR — ayıklanmazsa içe aktarmanın tamamı, üstelik kullanıcının
 * anlamayacağı bir mesajla başarısız olurdu.
 */
func TestListRepos_GomuluKullaniciAdiAyiklanir(t *testing.T) {
	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"isLastPage": true,
			"values": []any{
				repoJSON("api", httpKlon("https://ahmet@bb.sirket.com/scm/ODEME/api.git")),
			},
		})
	})

	repos, err := c.ListRepos(context.Background(), grup(s), Credentials{})

	require.NoError(t, err)
	require.Equal(t, "https://bb.sirket.com/scm/ODEME/api.git", repos[0].CloneURL)
}

// HTTP klonu olmayan repository atlanır: kaydedilecek adres yok.
// Sessizce atlanmıyor — çağırana bildirilecek şekilde listede yer almıyor.
func TestListRepos_HTTPKlonuOlmayanAtlanir(t *testing.T) {
	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"isLastPage": true,
			"values": []any{
				repoJSON("ssh-only", map[string]string{
					"href": "ssh://git@bb:7999/odeme/x.git", "name": "ssh"}),
				repoJSON("api", httpKlon("https://bb/scm/ODEME/api.git")),
			},
		})
	})

	repos, err := c.ListRepos(context.Background(), grup(s), Credentials{})

	require.NoError(t, err)
	require.Len(t, repos, 1)
	require.Equal(t, "api", repos[0].Slug)
}

/*
 * `archived` alanı OLMAYAN yanıtta hiçbir repository arşivli sayılmaz.
 *
 * Alan sunucu sürümüne göre gelmeyebiliyor; yokluğunu "arşivli" saymak
 * kullanıcının bütün seçimini boşaltırdı (spec 021: kaynak bildirmiyorsa
 * hepsi seçili gelir).
 */
func TestListRepos_ArsivAlaniYoksaFalse(t *testing.T) {
	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"isLastPage": true,
			"values":     []any{repoJSON("api", httpKlon("https://bb/scm/O/api.git"))},
		})
	})

	repos, err := c.ListRepos(context.Background(), grup(s), Credentials{})

	require.NoError(t, err)
	require.False(t, repos[0].Archived)
}

func TestListRepos_ArsivAlaniOkunur(t *testing.T) {
	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		r1 := repoJSON("eski", httpKlon("https://bb/scm/O/eski.git"))
		r1["archived"] = true
		json.NewEncoder(w).Encode(map[string]any{
			"isLastPage": true, "values": []any{r1},
		})
	})

	repos, err := c.ListRepos(context.Background(), grup(s), Credentials{})

	require.NoError(t, err)
	require.True(t, repos[0].Archived)
}

// Kimlik bilgisi Basic Auth ile gider — Cloud'da app password, Server'da
// kişisel erişim anahtarı; ikisi de aynı başlıkla çalışıyor.
func TestListRepos_BasicAuthGonderilir(t *testing.T) {
	var kullanici, sir string
	var okundu bool

	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		kullanici, sir, okundu = r.BasicAuth()
		json.NewEncoder(w).Encode(map[string]any{"isLastPage": true, "values": []any{}})
	})

	_, err := c.ListRepos(context.Background(), grup(s),
		Credentials{Username: "ahmet", Secret: "gizli"})

	require.NoError(t, err)
	require.True(t, okundu)
	require.Equal(t, "ahmet", kullanici)
	require.Equal(t, "gizli", sir)
}

func TestListRepos_HataSiniflandirmasi(t *testing.T) {
	durumlar := []struct {
		kod int
		err error
	}{
		{http.StatusNotFound, ErrGroupNotFound},
		{http.StatusUnauthorized, ErrForbidden},
		{http.StatusForbidden, ErrForbidden},
		{http.StatusInternalServerError, ErrUnreachable},
		{http.StatusBadGateway, ErrUnreachable},
	}

	for _, d := range durumlar {
		t.Run(strconv.Itoa(d.kod), func(t *testing.T) {
			s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(d.kod)
			})

			_, err := c.ListRepos(context.Background(), grup(s), Credentials{})
			require.ErrorIs(t, err, d.err)
		})
	}
}

/*
 * AYRIŞTIRILAMAYAN GÖVDE SESSİZ BOŞ LİSTE DÖNMEZ.
 *
 * Gerçek bir sunucuda ölçüm yapamadığımız için en olası arıza budur: sürüm
 * farkı yüzünden beklemediğimiz bir gövde. Boş liste dönseydi kullanıcı
 * "grupta repository yok" görür ve sorunun bizde olduğunu asla anlamazdı.
 * Hata, gövdenin kısaltılmış halini TAŞIR — issue açan kişinin elinde iz kalsın.
 */
func TestListRepos_BozukGovdeHataVerir(t *testing.T) {
	s, c := sunucu(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>beklenmedik bir yanıt</html>`))
	})

	_, err := c.ListRepos(context.Background(), grup(s), Credentials{})

	require.ErrorIs(t, err, ErrBadResponse)
	require.Contains(t, err.Error(), "beklenmedik bir yanıt", "ham gövde izlenebilmeli")
}

// Sunucuya hiç ulaşılamadığında ağ hatası da sınıflandırılır.
func TestListRepos_UlasilamiyorsaSiniflandirilir(t *testing.T) {
	c := NewClient(http.DefaultClient)

	_, err := c.ListRepos(context.Background(),
		GroupRef{BaseURL: "http://127.0.0.1:1", Key: "ODEME"}, Credentials{})

	require.ErrorIs(t, err, ErrUnreachable)
}
