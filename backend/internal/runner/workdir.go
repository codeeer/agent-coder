package runner

import (
	"path"
	"strings"
)

/*
WorkRoot, çalışma ortamındaki sabit kök.

TEK TANIM. Değer `entrypoint.sh`'a `PROJECT_DIR` olarak geçiyor ve betik onu
okuyor; iki yerde sabit yazılsaydı biri değiştiğinde diğeri geride kalır ve
kullanıcının yazdığı script'ler var olmayan bir dizine bakardı.

Betikler bu değişkeni kullanır ve ÇALIŞMA DİZİNİNE GÜVENMEZ: bash aracını
hangi dizinde çalıştırdığı motorun kararı, bizim değil.
*/
const WorkRoot = "/work"

/*
WorkdirLayout, projenin kökün neresine açılacağı (spec 025).

NEDEN VAR: kurumsal ekiplerin elindeki runbook, CI işi ve yardımcı betikler
projenin `<kök>/<repo-adı>` altında durduğunu varsayıyor — çoğu CI aracı ve
geliştiricinin kendi makinesi böyle çalıştığı için. Agent'ın içinde proje
doğrudan kökte duruyordu ve o betikler elle uyarlanmak zorundaydı.

VARSAYILAN DEĞİŞMEDİ: tanımsız veya tanınmayan her değer `LayoutRoot`'tur.
*/
type WorkdirLayout string

const (
	// LayoutRoot, proje doğrudan kökün kendisine açılır: /work
	LayoutRoot WorkdirLayout = "root"
	// LayoutRepo, proje kökün altında repo adını taşıyan klasöre açılır:
	// /work/<repo-adı>
	LayoutRepo WorkdirLayout = "repo"
)

/*
ProjectDir, verilen yerleşim ve depo adresi için proje kökünü üretir.

SAF FONKSİYON ve tek karar noktası. Yol çalıştırma başına BİR KEZ burada
hesaplanır; hem ortam değişkeni hem agent'a verilen talimat metni aynı
değeri kullanır. İki yerde ayrı ayrı hesaplansaydı, biri değiştiğinde diğeri
geride kalır ve model ile betikler farklı yola bakardı — sessizce.

GÜVENSİZ AD KÖKE DÜŞER, hata vermez. Bu bir güvenlik kontrolü olduğu kadar
bir kolaylık ayarı: adres tuhaf diye çalıştırmayı düşürmek, kullanıcının
istemediği bir bedel. Ama ad kökün DIŞINA çıkamaz — depo adresini girebilen
biri aksi halde çalışma ortamında istediği yere yazabilirdi.
*/
func ProjectDir(layout WorkdirLayout, repoURL string) string {
	if layout != LayoutRepo {
		return WorkRoot
	}

	ad := repoAdi(repoURL)
	if ad == "" {
		return WorkRoot
	}

	dir := path.Join(WorkRoot, ad)

	/*
	 * SON KONTROL — `repoAdi` zaten ayraç reddediyor, bu ikinci katman.
	 *
	 * Tek bir doğrulamaya güvenmiyoruz: `repoAdi` ileride gevşetilirse
	 * (yeni bir adres biçimi eklenirken) buradaki kontrol hâlâ kökün
	 * dışına çıkışı durdurur. Yol üretimi tek yerde olduğu için bu kontrolün
	 * maliyeti de tek yerde.
	 */
	if path.Dir(dir) != WorkRoot {
		return WorkRoot
	}
	return dir
}

/*
repoAdi, depo adresinden klasör adını türetir. Türetemezse boş döner.

BİÇİM AYRIŞTIRICI YAZILMADI: `url.Parse` kısa SSH biçimini
(`git@host:proj/repo.git`) zaten çözemiyor — orada `git@host` geçerli bir
scheme değil. Bunun yerine adresin BİÇİMİNDEN bağımsız olarak son parça
alınıyor: önce son `/`, kalanda hâlâ `:` varsa son `:` (slash'sız kısa SSH
biçimi: `git@host:repo.git`). Üç adres ailesi de aynı kodla çözülüyor.
*/
func repoAdi(repoURL string) string {
	s := strings.TrimSpace(repoURL)
	s = strings.TrimRight(s, "/")
	if s == "" {
		return ""
	}

	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	// Port taşıyan adreslerde son `/` zaten geçildiği için buraya yalnızca
	// slash'sız kısa SSH biçimi düşer.
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		s = s[i+1:]
	}

	s = strings.TrimSuffix(s, ".git")

	if s == "" || s == "." || s == ".." {
		return ""
	}
	// Ayraç ve NUL: `repoAdi` son parçayı aldığı için `/` zaten kalmamalı,
	// ama ters ayraç ve NUL gelebilir ve ikisi de dizin adında işimiz değil.
	if strings.ContainsAny(s, "/\\\x00") {
		return ""
	}
	return s
}
