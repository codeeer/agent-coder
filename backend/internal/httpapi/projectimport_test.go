package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/bitbucket"
	"github.com/agent-coder/backend/internal/config"
	"github.com/agent-coder/backend/internal/gitprovider"
	"github.com/agent-coder/backend/internal/projects"
	"github.com/agent-coder/backend/internal/secrets"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * Gruptan toplu proje ekleme uçları (spec 021).
 *
 * SAHTE BITBUCKET kendi kodumuzu doğrular, Atlassian'ın gerçek yanıtını değil
 * (spec → Belirsizlikler). Kilitlenen şey bizim davranışımız: durum etiketleri,
 * mükerrer atlama, kısmi başarı, akış biçimi ve eşzamanlılık sınırı.
 */

// sahteBitbucket, verilen repository'leri tek sayfada döndüren sunucu.
func sahteBitbucket(t *testing.T, klonAdresleri map[string]string) *httptest.Server {
	t.Helper()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var values []map[string]any
		for slug, klon := range klonAdresleri {
			values = append(values, map[string]any{
				"slug": slug, "name": slug,
				"links": map[string]any{"clone": []map[string]string{
					{"href": klon, "name": "http"},
				}},
			})
		}
		json.NewEncoder(w).Encode(map[string]any{"isLastPage": true, "values": values})
	}))
	t.Cleanup(s.Close)
	return s
}

func importHandler(t *testing.T) (*Handler, *projects.Store, uuid.UUID) {
	t.Helper()

	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "projects", "git_providers")

	key := make([]byte, secrets.KeySize)
	cipher, hataC := secrets.NewCipher(base64.StdEncoding.EncodeToString(key))
	require.NoError(t, hataC)

	gpStore := gitprovider.NewStore(pool, cipher)
	gp, hataG := gpStore.Create(context.Background(), gitprovider.CreateInput{
		Type: gitprovider.TypeBitbucket, Name: "kurumsal",
		Username: "ahmet", Secret: "gizli",
	})
	require.NoError(t, hataG)

	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("SECRET_ENCRYPTION_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	cfg, err := config.Load()
	require.NoError(t, err)

	prStore := projects.NewStore(pool)
	h := NewHandler(Deps{
		Config:       cfg,
		Projects:     prStore,
		GitProviders: gpStore,
		RepoVerifier: projects.NewVerifier(),
	})
	t.Cleanup(h.Shutdown)

	return h, prStore, gp.ID
}

func postJSON(t *testing.T, h *Handler, yol string, govde any) *httptest.ResponseRecorder {
	t.Helper()

	b, err := json.Marshal(govde)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	h.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, yol, bytes.NewReader(b)))
	return rec
}

// ── Önizleme ────────────────────────────────────────────────────────────────

func TestImportPreview_DurumEtiketleri(t *testing.T) {
	h, store, gpID := importHandler(t)

	bb := sahteBitbucket(t, map[string]string{
		"api": "https://bb.sirket.com/scm/ODEME/api.git",
		"web": "https://bb.sirket.com/scm/ODEME/web.git",
	})

	// `web` ZATEN kayıtlı — üstelik farklı harf büyüklüğüyle. Normalizasyon
	// çalışmazsa bu kayıt "new" görünür ve kopya üretilirdi.
	_, err := store.Create(context.Background(), projects.Input{
		Name: "web", RepoURL: "https://BB.sirket.com/scm/odeme/web",
		DefaultBranch: "main",
	})
	require.NoError(t, err)

	rec := postJSON(t, h, "/api/projects/import/preview", map[string]any{
		"groupUrl":      bb.URL + "/projects/ODEME",
		"gitProviderId": gpID,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp importPreviewResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "ODEME", resp.Group.Key)

	durum := map[string]string{}
	for _, r := range resp.Repos {
		durum[r.Slug] = r.Status
	}
	require.Equal(t, statusNew, durum["api"])
	require.Equal(t, statusRegistered, durum["web"])
}

func TestImportPreview_SaglayiciSecilmemis(t *testing.T) {
	h, _, _ := importHandler(t)

	rec := postJSON(t, h, "/api/projects/import/preview", map[string]any{
		"groupUrl": "https://bb.sirket.com/projects/ODEME",
	})

	require.Equal(t, http.StatusPreconditionFailed, rec.Code)
	// Mesaj NE YAPILACAĞINI söylemeli.
	require.Contains(t, rec.Body.String(), "Ayarlar")
}

// ── Hata tablosu (spec 021) ─────────────────────────────────────────────────

func TestRespondImportError_SpecTablosu(t *testing.T) {
	h := &Handler{}

	durumlar := []struct {
		ad   string
		err  error
		kod  int
		code string
	}{
		{"bulut adresi", bitbucket.ErrCloudAddress, http.StatusBadRequest, "bitbucket_cloud"},
		{"grup adresi değil", bitbucket.ErrNotGroupURL, http.StatusBadRequest, "not_group_url"},
		{"grup yok", bitbucket.ErrGroupNotFound, http.StatusNotFound, "group_not_found"},
		{"yetki yok", bitbucket.ErrForbidden, http.StatusForbidden, "bitbucket_forbidden"},
		{"ulaşılamıyor", bitbucket.ErrUnreachable, http.StatusBadGateway, "bitbucket_unreachable"},
		{"bozuk yanıt", bitbucket.ErrBadResponse, http.StatusBadGateway, "bitbucket_bad_response"},
		{"erişim seçilmedi", errSaglayiciSecilmedi, http.StatusPreconditionFailed, "no_git_access"},
	}

	for _, d := range durumlar {
		t.Run(d.ad, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.respondImportError(rec, d.err)

			require.Equal(t, d.kod, rec.Code)

			var body ErrorBody
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, d.code, body.Error.Code)
			require.NotEmpty(t, body.Error.Message)
		})
	}
}

// Bulut mesajı kullanıcıyı suçlamıyor, yolun ne için olduğunu söylüyor (H4).
func TestRespondImportError_BulutMesajiYolAnlatir(t *testing.T) {
	rec := httptest.NewRecorder()
	(&Handler{}).respondImportError(rec, bitbucket.ErrCloudAddress)

	require.Contains(t, rec.Body.String(), "kurumsal")
}

// akisSatirlari, NDJSON gövdesini satırlara ayırır (özet satırı dahil).
func akisSatirlari(t *testing.T, govde string) []importSatir {
	t.Helper()

	var out []importSatir
	sc := bufio.NewScanner(strings.NewReader(govde))
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var s importSatir
		require.NoError(t, json.Unmarshal(sc.Bytes(), &s))
		out = append(out, s)
	}
	return out
}

// ── İçe aktarma ─────────────────────────────────────────────────────────────

/*
gitHTTPSunucu, gerçek bir git deposunu HTTP üzerinden sunar.

NEDEN `file://` DEĞİL: `projects.Input.Normalize` yalnızca http/https kabul
ediyor (ve bu doğru — kimlik akışımız HTTP üzerine kurulu). `file://` ile
yazılan bir test, kayıt yolunu hiç sınayamaz.

`git http-backend` bir alt komut DEĞİL, ayrı bir çalıştırılabilir; yeri
`git --exec-path` ile bulunuyor. CGI olarak koşturulduğunda tam bir akıllı
HTTP git sunucusu oluyor — `ls-remote --symref` dahil.
*/
func gitHTTPSunucu(t *testing.T, branch string) string {
	t.Helper()

	calistir := func(dizin string, arg ...string) string {
		t.Helper()
		cmd := exec.Command("git", arg...)
		cmd.Dir = dizin
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e.com",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e.com")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}

	// Çalışma kopyası: bir commit ve istenen branch.
	is := t.TempDir()
	calistir(is, "init", "-b", branch)
	calistir(is, "commit", "--allow-empty", "-m", "ilk")

	// Bare kopya sunulacak; HEAD kaynağın HEAD'ini izler.
	kok := t.TempDir()
	calistir(kok, "clone", "--bare", is, "depo.git")

	execPath := calistir(t.TempDir(), "--exec-path")
	srv := httptest.NewServer(&cgi.Handler{
		Path: filepath.Join(execPath, "git-http-backend"),
		Env: []string{
			"GIT_PROJECT_ROOT=" + kok,
			"GIT_HTTP_EXPORT_ALL=1",
		},
	})
	t.Cleanup(srv.Close)

	return srv.URL + "/depo.git"
}

/*
Kayıt yolu: varsayılan branch DEPODAN okunur ve projeye yazılır.

`repoEkle` doğrudan çağrılıyor: listeleme yolundan geçilseydi `file://` adresi
klonlama adresi süzgecine takılırdı (orası yalnızca http kabul ediyor, ki bu
doğru). Sınanan şey listeleme değil, KAYIT.
*/
func TestRepoEkle_VarsayilanBranchDepodanYazilir(t *testing.T) {
	h, store, _ := importHandler(t)

	satir := h.repoEkle(context.Background(),
		importRepo{Slug: "api", Name: "API", CloneURL: gitHTTPSunucu(t, "develop")}, nil, nil)

	require.Equal(t, sonucCreated, satir.Result, satir.Reason)
	require.NotNil(t, satir.ProjectID)

	p, err := store.Get(context.Background(), *satir.ProjectID)
	require.NoError(t, err)
	require.Equal(t, "develop", p.DefaultBranch, "branch uydurulmamalı, depodan gelmeli")
	require.Equal(t, "API", p.Name, "ad kaynaktan alınır, türetilmez")
}

// Erişilemeyen depo EKLENMEZ ve sebebi yazılır.
func TestRepoEkle_ErisilemeyenDepoEklenmez(t *testing.T) {
	h, _, _ := importHandler(t)

	satir := h.repoEkle(context.Background(),
		importRepo{Slug: "yok", Name: "yok", CloneURL: "https://127.0.0.1:1/yok.git"}, nil, nil)

	require.Equal(t, sonucFailed, satir.Result)
	require.NotEmpty(t, satir.Reason)
	require.Nil(t, satir.ProjectID)
}

/*
Akış biçimi, kısmi başarı ve mükerrer atlama bir arada.

Üç repository gönderiliyor: biri zaten kayıtlı, ikisi erişilemez. Kayıtlı olan
SINANMADAN atlanmalı; diğerleri ayrı ayrı düşmeli ve özet üçünü de saymalı.
*/
func TestImportRun_AkisVeKismiBasari(t *testing.T) {
	h, store, gpID := importHandler(t)

	_, err := store.Create(context.Background(), projects.Input{
		Name: "kayitli", RepoURL: "https://bb.sirket.com/scm/ODEME/kayitli.git",
		DefaultBranch: "main",
	})
	require.NoError(t, err)

	rec := postJSON(t, h, "/api/projects/import", map[string]any{
		"gitProviderId": gpID,
		"repos": []map[string]any{
			{"slug": "kayitli", "name": "kayitli",
				"cloneUrl": "https://bb.sirket.com/scm/ODEME/kayitli.git"},
			{"slug": "a", "name": "a", "cloneUrl": "https://127.0.0.1:1/scm/ODEME/a.git"},
			{"slug": "b", "name": "b", "cloneUrl": "https://127.0.0.1:1/scm/ODEME/b.git"},
		},
	})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "x-ndjson")

	var (
		satirlar []importSatir
		ozet     *importOzeti
	)
	sc := bufio.NewScanner(strings.NewReader(rec.Body.String()))
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var s importSatir
		require.NoError(t, json.Unmarshal(sc.Bytes(), &s))
		if s.Summary != nil {
			ozet = s.Summary
			continue
		}
		satirlar = append(satirlar, s)
	}

	require.Len(t, satirlar, 3, "her repository için bir satır")
	require.NotNil(t, ozet, "sonda özet satırı olmalı")
	require.Equal(t, 0, ozet.Created)
	require.Equal(t, 1, ozet.Skipped)
	require.Equal(t, 2, ozet.Failed)

	for _, s := range satirlar {
		if s.Slug == "kayitli" {
			require.Equal(t, sonucSkipped, s.Result)
			require.Contains(t, s.Reason, "zaten kayıtlı")
		}
	}
}

/*
UÇTAN UCA: önizleme → içe aktarma, ikisi de HTTP uçlarından.

Sahte Bitbucket, gerçek bir git HTTP sunucusunun adresini klonlama adresi
olarak veriyor. Böylece zincirin tamamı koşuyor: grup adresi çözülüyor, liste
sayfalanıyor, klonlama adresi seçiliyor, erişim sınanıyor, varsayılan branch
depodan okunuyor ve proje kaydediliyor.
*/
func TestImport_UctanUca(t *testing.T) {
	h, store, gpID := importHandler(t)

	klon := gitHTTPSunucu(t, "release/2026")
	bb := sahteBitbucket(t, map[string]string{"api": klon})

	// 1) Önizleme: repository yeni görünmeli.
	rec := postJSON(t, h, "/api/projects/import/preview", map[string]any{
		"groupUrl": bb.URL + "/projects/ODEME", "gitProviderId": gpID,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var onizleme importPreviewResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &onizleme))
	require.Len(t, onizleme.Repos, 1)
	require.Equal(t, statusNew, onizleme.Repos[0].Status)

	// 2) İçe aktarma: önizlemeden gelen kayıt aynen gönderiliyor.
	rec = postJSON(t, h, "/api/projects/import", map[string]any{
		"gitProviderId": gpID, "repos": onizleme.Repos,
	})
	require.Equal(t, http.StatusOK, rec.Code)

	var olusan *uuid.UUID
	for _, satir := range akisSatirlari(t, rec.Body.String()) {
		if satir.Result == sonucCreated {
			olusan = satir.ProjectID
		}
	}
	require.NotNil(t, olusan, "proje oluşmalı: %s", rec.Body.String())

	p, err := store.Get(context.Background(), *olusan)
	require.NoError(t, err)
	require.Equal(t, "release/2026", p.DefaultBranch, "branch depodan okunmalı")
	require.NotNil(t, p.GitProviderID, "proje, listelemede kullanılan erişime bağlanmalı")
	require.Equal(t, gpID, *p.GitProviderID)
}

func TestImportRun_BosSecimReddedilir(t *testing.T) {
	h, _, gpID := importHandler(t)

	rec := postJSON(t, h, "/api/projects/import", map[string]any{
		"gitProviderId": gpID, "repos": []any{},
	})

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

/*
EŞZAMANLILIK SINIRI ÖLÇÜLÜR.

Sınırsız olsaydı yüz repository yüz eşzamanlı `git` süreci demekti. Test,
aynı anda kaç `repoEkle`'nin koştuğunu sayıyor ve tepe değerin sınırı
aşmadığını doğruluyor.
*/
func TestImportEt_EsZamanlilikSiniriAsilmaz(t *testing.T) {
	h := &Handler{}

	var (
		anlik atomic.Int32
		tepe  atomic.Int32
		basla = make(chan struct{})
	)

	var repos []importRepo
	for i := 0; i < 40; i++ {
		repos = append(repos, importRepo{Slug: "d", Name: "d", CloneURL: "https://x/y.git"})
	}

	// İş, aynı anda kaç kopyasının koştuğunu sayıyor ve HEPSİ girene kadar
	// bekliyor. Sınır uygulanmasaydı 40'ı birden girer ve tepe 40 olurdu.
	isle := func(importRepo) importSatir {
		n := anlik.Add(1)
		for {
			eski := tepe.Load()
			if n <= eski || tepe.CompareAndSwap(eski, n) {
				break
			}
		}
		<-basla
		anlik.Add(-1)
		return importSatir{Result: sonucFailed}
	}

	bitti := make(chan importOzeti, 1)
	go func() {
		bitti <- h.importEt(importRunRequest{Repos: repos},
			map[string]uuid.UUID{}, isle, func(importSatir) {})
	}()

	// Kapıya sığanlar girdi; sınır aşılmadığını görmek için serbest bırak.
	require.Eventually(t, func() bool { return anlik.Load() == esZamanliSinama },
		2*time.Second, 5*time.Millisecond, "kapı dolmalı")
	close(basla)

	ozet := <-bitti

	require.Equal(t, 40, ozet.Failed, "hiçbir iş kaybolmamalı")
	require.LessOrEqual(t, int(tepe.Load()), esZamanliSinama,
		"aynı anda sınırdan fazla iş koşmamalı")
}
