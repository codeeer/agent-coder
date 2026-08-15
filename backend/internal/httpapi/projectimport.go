package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"

	"github.com/agent-coder/backend/internal/bitbucket"
	"github.com/agent-coder/backend/internal/gitprovider"
	"github.com/agent-coder/backend/internal/projects"
)

/*
 * Kurumsal Bitbucket grubundan toplu proje ekleme (spec 021).
 *
 * İki uç, iki faz: önizleme hızlıdır (yalnızca sayfalı liste çağrısı), içe
 * aktarma yavaştır (her repository için `ls-remote`). Kullanıcı ikisinin
 * arasında seçim yapar.
 *
 * BULUT KAPSAM DIŞI. Bulut adresi ayrı bir mesajla reddedilir; kurumsal uca
 * gönderilip anlamsız bir hata üretmez (spec 021 H4).
 */

// esZamanliSinama, aynı anda kaç repository'nin sınanacağı.
//
// Yüz repository seri sınansaydı en kötü hâl dakikalarla ölçülürdü ve spec'in
// "makul sürede biter" kriterini karşılamazdı. Sınır var çünkü sınırsız
// paralellik hem sunucuyu hem yerel süreç tablosunu zorlar.
const esZamanliSinama = 8

type importPreviewRequest struct {
	GroupURL      string     `json:"groupUrl"`
	GitProviderID *uuid.UUID `json:"gitProviderId"`
}

// importRepo, önizlemede ve içe aktarma isteğinde taşınan repository.
type importRepo struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	CloneURL string `json:"cloneUrl"`
	Archived bool   `json:"archived"`

	// Status yalnızca önizlemede dolu: "new" | "already_registered".
	Status string `json:"status,omitempty"`
}

const (
	statusNew        = "new"
	statusRegistered = "already_registered"
)

type importPreviewResponse struct {
	Group struct {
		BaseURL string `json:"baseUrl"`
		Key     string `json:"key"`
	} `json:"group"`
	Repos []importRepo `json:"repos"`
}

// importPreview, grup adresindeki repository'leri durum etiketiyle listeler.
func (h *Handler) importPreview(w http.ResponseWriter, r *http.Request) {
	if h.deps.Projects == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	var req importPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}

	grup, err := bitbucket.ParseGroupURL(req.GroupURL)
	if err != nil {
		h.respondImportError(w, err)
		return
	}

	creds, err := h.bitbucketCreds(ctx, req.GitProviderID)
	if err != nil {
		h.respondImportError(w, err)
		return
	}

	repos, err := bitbucket.NewClient(nil).ListRepos(ctx, grup, creds)
	if err != nil {
		h.respondImportError(w, err)
		return
	}

	mevcut, err := h.deps.Projects.ExistingRepoURLs(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "kayıtlı projeler okunamadı")
		return
	}

	var resp importPreviewResponse
	resp.Group.BaseURL = grup.BaseURL
	resp.Group.Key = grup.Key
	resp.Repos = []importRepo{}

	for _, rp := range repos {
		durum := statusNew
		if _, kayitli := mevcut[projects.NormalizeRepoURL(rp.CloneURL)]; kayitli {
			durum = statusRegistered
		}
		resp.Repos = append(resp.Repos, importRepo{
			Slug: rp.Slug, Name: rp.Name, CloneURL: rp.CloneURL,
			Archived: rp.Archived, Status: durum,
		})
	}

	respondJSON(w, http.StatusOK, resp)
}

type importRunRequest struct {
	GitProviderID *uuid.UUID   `json:"gitProviderId"`
	Repos         []importRepo `json:"repos"`
}

// importSatir, akışta gönderilen tek satır.
//
// Ya bir repository'nin sonucu ya da en sonda özet. Tek tipte tutuluyor ki
// istemci satırları ayırt etmek için ayrı bir çerçeveye ihtiyaç duymasın.
type importSatir struct {
	Slug      string       `json:"slug,omitempty"`
	Name      string       `json:"name,omitempty"`
	Result    string       `json:"result,omitempty"` // created | skipped | failed
	ProjectID *uuid.UUID   `json:"projectId,omitempty"`
	Reason    string       `json:"reason,omitempty"`
	Summary   *importOzeti `json:"summary,omitempty"`
}

type importOzeti struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

const (
	sonucCreated = "created"
	sonucSkipped = "skipped"
	sonucFailed  = "failed"
)

/*
importRun, seçilen repository'leri sınayıp kaydeder ve sonuçları AKIŞ olarak
gönderir.

NDJSON: `EventSource` yalnızca GET yapabiliyor, oysa seçim listesi gövdede
geliyor. İş kaydı + ayrı bir olay ucu çifti, tek istek ömrü kadar yaşayacak bir
iş için gereksiz durum yaratırdı.

KISMİ BAŞARI GERİ ALINMAZ: biri düşerken diğerleri kaydedilmiş kalır. Bir hata
yüzünden tamamlanmış işi geri almak kullanıcının kaybını büyütmekten başka işe
yaramaz.
*/
func (h *Handler) importRun(w http.ResponseWriter, r *http.Request) {
	if h.deps.Projects == nil {
		respondError(w, http.StatusServiceUnavailable, "db_unavailable", "veritabanı hazır değil")
		return
	}
	ctx := r.Context()

	var req importRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_body", "gövde ayrıştırılamadı")
		return
	}
	if len(req.Repos) == 0 {
		respondError(w, http.StatusBadRequest, "no_repos", "içe aktarılacak repository seçilmedi")
		return
	}

	// Kimlik BİR KEZ çözülür: her repository için yeniden çözmek, N şifre
	// çözme ve N sorgu demekti.
	creds, err := h.repoCreds(ctx, req.GitProviderID)
	if err != nil {
		h.respondImportError(w, err)
		return
	}

	mevcut, err := h.deps.Projects.ExistingRepoURLs(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "kayıtlı projeler okunamadı")
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	enc := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)

	// yaz, satırları SIRAYA SOKAR: goroutine'ler paralel çalışıyor ama tek bir
	// ResponseWriter'a yazıyorlar.
	var yazKilidi sync.Mutex
	yaz := func(s importSatir) {
		yazKilidi.Lock()
		defer yazKilidi.Unlock()
		if err := enc.Encode(s); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}

	ozet := h.importEt(req, mevcut, func(rp importRepo) importSatir {
		return h.repoEkle(ctx, rp, req.GitProviderID, creds)
	}, yaz)
	yaz(importSatir{Summary: &ozet})

	slog.InfoContext(ctx, "gruptan içe aktarma bitti",
		"eklenen", ozet.Created, "atlanan", ozet.Skipped, "basarisiz", ozet.Failed)
}

/*
importEt, seçilenleri sınırlı eşzamanlılıkla işler ve özeti döner.

`isle` DIŞARIDAN geliyor: eşzamanlılık sınırının gerçekten uygulandığı ancak
işin ne yaptığını sayabilen bir testle ölçülebilir. Üretimde tek çağıran var
ve o `repoEkle`'yi veriyor.
*/
func (h *Handler) importEt(req importRunRequest, mevcut map[string]uuid.UUID,
	isle func(importRepo) importSatir, yaz func(importSatir)) importOzeti {

	var (
		sayacKilidi sync.Mutex
		ozet        importOzeti
		wg          sync.WaitGroup
	)
	kapi := make(chan struct{}, esZamanliSinama)

	/*
		Bu istekte üstlenilen adresler.

		`mevcut` döngüden ÖNCE okunan bir anlık görüntü: bu çağrıda oluşturulan
		kayıtları görmüyor. `projects.repo_url` üzerinde unique kısıt da yok,
		yani veritabanı ikinci kaydı reddetmiyor — aynı adrese normalize olan
		iki giriş (birebir aynı, ya da yalnızca büyük/küçük harf veya `.git`
		ile ayrılan) iki ayrı proje oluşturuyordu.

		Kilit gerekmiyor: eleme, goroutine'ler açılmadan bu sıralı döngüde
		yapılıyor.
	*/
	ustlenilen := make(map[string]struct{}, len(req.Repos))

	for _, rp := range req.Repos {
		norm := projects.NormalizeRepoURL(rp.CloneURL)

		// Zaten kayıtlı olan sınanmadan atlanır: hem gereksiz ağ trafiği hem
		// de mevcut kaydın erişimini değiştirme riski olurdu.
		if _, kayitli := mevcut[norm]; kayitli {
			sayacKilidi.Lock()
			ozet.Skipped++
			sayacKilidi.Unlock()
			yaz(importSatir{Slug: rp.Slug, Name: rp.Name, Result: sonucSkipped,
				Reason: "bu repository zaten kayıtlı"})
			continue
		}

		if _, tekrar := ustlenilen[norm]; tekrar {
			sayacKilidi.Lock()
			ozet.Skipped++
			sayacKilidi.Unlock()
			yaz(importSatir{Slug: rp.Slug, Name: rp.Name, Result: sonucSkipped,
				Reason: "bu adres seçimde birden fazla kez var"})
			continue
		}
		ustlenilen[norm] = struct{}{}

		wg.Add(1)
		go func(rp importRepo) {
			defer wg.Done()
			kapi <- struct{}{}
			defer func() { <-kapi }()

			satir := isle(rp)

			sayacKilidi.Lock()
			switch satir.Result {
			case sonucCreated:
				ozet.Created++
			case sonucFailed:
				ozet.Failed++
			}
			sayacKilidi.Unlock()

			yaz(satir)
		}(rp)
	}

	wg.Wait()
	return ozet
}

/*
repoEkle, tek bir repository'yi sınayıp kaydeder.

SIRA ÖNEMLİ: önce varsayılan branch okunur, sonra kayıt yapılır. `DefaultBranch`
aynı çağrıda erişimi de sınadığı için ayrı bir doğrulama adımı yok.

BOŞ BRANCH GEÇİRİLMEZ. `projects.Input.Normalize` boş branch'i sessizce "main"
yapıyor; ona güvenmek, HEAD'i okunamayan bir depoyu yanlış branch'le kaydetmek
ve ürünün "ölçülmeyen yazılmaz" kuralını sessizce çiğnemek olurdu.
*/
func (h *Handler) repoEkle(ctx contextT, rp importRepo,
	providerID *uuid.UUID, creds *projects.Credentials) importSatir {

	satir := importSatir{Slug: rp.Slug, Name: rp.Name}

	if h.deps.RepoVerifier == nil {
		satir.Result = sonucFailed
		satir.Reason = "erişim sınaması yapılamıyor"
		return satir
	}

	branch, err := h.deps.RepoVerifier.DefaultBranch(ctx, rp.CloneURL, creds)
	if err != nil {
		satir.Result = sonucFailed
		satir.Reason = importHataMetni(err)
		return satir
	}

	in := projects.Input{
		Name:          rp.Name,
		RepoURL:       rp.CloneURL,
		DefaultBranch: branch,
		GitProviderID: providerID,
	}
	p, err := h.deps.Projects.Create(ctx, in)
	if err != nil {
		satir.Result = sonucFailed
		satir.Reason = importHataMetni(err)
		return satir
	}

	satir.Result = sonucCreated
	satir.ProjectID = &p.ID
	return satir
}

// importHataMetni, kullanıcıya gösterilecek kısa sebep.
func importHataMetni(err error) string {
	switch {
	case errors.Is(err, projects.ErrRepoAuth):
		return "depo erişimi reddedildi"
	case errors.Is(err, projects.ErrRepoUnreachable):
		return "depoya ulaşılamadı"
	case errors.Is(err, projects.ErrDefaultBranchUnknown):
		return "varsayılan branch okunamadı"
	case errors.Is(err, projects.ErrInvalidRepoURL):
		return "depo adresi geçersiz"
	case errors.Is(err, projects.ErrEmptyName):
		return "repository adı boş"
	default:
		return err.Error()
	}
}

/*
bitbucketCreds, Bitbucket API'sine gidecek kimlik bilgisini çözer.

Sağlayıcı SEÇİLMEK ZORUNDA: kurumsal bir Bitbucket'ın repository listesi
kimliksiz okunamaz ve "belki açıktır" diye denemek, kullanıcıya yetki hatası
yerine boş liste gösterme riski taşırdı.
*/
func (h *Handler) bitbucketCreds(ctx contextT, id *uuid.UUID) (bitbucket.Credentials, error) {
	if id == nil {
		return bitbucket.Credentials{}, errSaglayiciSecilmedi
	}
	if h.deps.GitProviders == nil {
		return bitbucket.Credentials{}, errSaglayiciSecilmedi
	}

	gp, err := h.deps.GitProviders.Get(ctx, *id)
	if err != nil {
		return bitbucket.Credentials{}, err
	}
	if gp.Type != gitprovider.TypeBitbucket {
		return bitbucket.Credentials{}, errBitbucketDegil
	}

	secret, err := h.deps.GitProviders.Reveal(ctx, gp.ID)
	if err != nil {
		return bitbucket.Credentials{}, err
	}
	return bitbucket.Credentials{Username: gp.Username, Secret: secret}, nil
}

// repoCreds, klonlama sınaması için kimlik bilgisini çözer.
func (h *Handler) repoCreds(ctx contextT, id *uuid.UUID) (*projects.Credentials, error) {
	if id == nil {
		return nil, errSaglayiciSecilmedi
	}
	if h.deps.GitProviders == nil {
		return nil, errSaglayiciSecilmedi
	}

	gp, err := h.deps.GitProviders.Get(ctx, *id)
	if err != nil {
		return nil, err
	}
	secret, err := h.deps.GitProviders.Reveal(ctx, gp.ID)
	if err != nil {
		return nil, err
	}

	kullanici := gp.Username
	if kullanici == "" {
		kullanici = "x-access-token"
	}
	return &projects.Credentials{Username: kullanici, Secret: secret}, nil
}

var (
	errSaglayiciSecilmedi = errors.New("git erişimi seçilmedi")
	errBitbucketDegil     = errors.New("seçilen erişim Bitbucket değil")
)

/*
respondImportError, içe aktarma hatalarını spec 021'in hata tablosuna göre
yanıtlar.

Her mesaj NE YAPILACAĞINI söyler. "Grup bulunamadı" tek başına kullanıcıyı
yanlış yere baktırabilir — adres de yetki de sebep olabilir, ikisi de yazılır.
*/
func (h *Handler) respondImportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, bitbucket.ErrCloudAddress):
		respondError(w, http.StatusBadRequest, "bitbucket_cloud",
			"Bu adres Bitbucket Cloud'a ait. Gruptan toplu ekleme yalnızca "+
				"kendi sunucunuzda çalışan kurumsal Bitbucket için geçerli.")

	case errors.Is(err, bitbucket.ErrNotGroupURL):
		respondError(w, http.StatusBadRequest, "not_group_url", err.Error())

	case errors.Is(err, bitbucket.ErrGroupNotFound):
		respondError(w, http.StatusNotFound, "group_not_found",
			"Bu grup bulunamadı. Adres yanlış olabilir ya da seçilen erişimin "+
				"bu grubu görme yetkisi olmayabilir.")

	case errors.Is(err, bitbucket.ErrForbidden):
		respondError(w, http.StatusForbidden, "bitbucket_forbidden",
			"Bitbucket erişimi reddetti. Ayarlar → Git repository'ler bölümünden "+
				"kullanıcı adını ve erişim anahtarını kontrol edin.")

	case errors.Is(err, bitbucket.ErrUnreachable):
		respondError(w, http.StatusBadGateway, "bitbucket_unreachable",
			"Bitbucket sunucusuna ulaşılamıyor. Adres ve ağ erişimi kontrol edilmeli.")

	case errors.Is(err, bitbucket.ErrBadResponse):
		// Ham yanıt mesajda KALIR: gerçek bir kurumsal sunucuda ölçüm
		// yapamadığımız için sürüm farkını ancak kullanıcının bildirimi
		// ortaya çıkaracak (spec 021 → Belirsizlikler).
		respondError(w, http.StatusBadGateway, "bitbucket_bad_response", err.Error())

	case errors.Is(err, errSaglayiciSecilmedi):
		respondError(w, http.StatusPreconditionFailed, "no_git_access",
			"Önce bir Bitbucket erişimi tanımlayın: Ayarlar → Git repository'ler.")

	case errors.Is(err, errBitbucketDegil):
		respondError(w, http.StatusBadRequest, "not_bitbucket_provider",
			"Seçilen git erişimi Bitbucket türünde değil.")

	case errors.Is(err, gitprovider.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "git erişimi bulunamadı")

	default:
		slog.Error("gruptan içe aktarma başarısız", "error", err)
		respondError(w, http.StatusInternalServerError, "internal_error",
			fmt.Sprintf("içe aktarma yapılamadı: %v", err))
	}
}
