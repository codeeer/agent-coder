package runner

import (
	"context"
	"errors"
)

/*
 * Bağımlılık önbelleği yönetimi (spec 027 H3).
 *
 * ARAYÜZ OLARAK BURADA DURUYOR ki HTTP katmanı çalışma ortamının iç bilgisini
 * — volume adları, container içi yollar, Docker — hiç görmesin. Bu paketin
 * sınır kuralı: `internal/runner/` sızmaz. Dışarıya çıkan tek şey "maven" ve
 * "npm" kimlikleri ile bayt sayıları.
 */

// ErrUnknownCache, istenen önbellek kimliği tanımlı değil.
var ErrUnknownCache = errors.New("bilinmeyen önbellek")

/*
ErrCacheBusy, önbellek şu anda bir çalışma ortamı tarafından bağlı.

Çağıranın ayırt etmesi gerekiyor: kullanıcıya söylenecek şey "bakım
başarısız" değil, "şu an yapılamaz".
*/
var ErrCacheBusy = errors.New("önbellek şu anda kullanılıyor")

// CacheID, bir bağımlılık önbelleğinin kimliği.
type CacheID string

const (
	CacheMaven CacheID = "maven"
	CacheNPM   CacheID = "npm"
)

// CacheInfo, bir önbelleğin kullanıcıya gösterilecek durumu.
type CacheInfo struct {
	ID    CacheID `json:"id"`
	Label string  `json:"label"`

	/*
	 * SizeBytes, önbelleğin bayt cinsinden boyutu.
	 *
	 * BAYT OLARAK TAŞINIYOR, biçimlenmiş metin olarak değil: biçimlendirme
	 * arayüzün işi ve temizleme onayındaki "ne kadar yer boşalacak" sayısı da
	 * buradan besleniyor.
	 *
	 * `Used` yanlışsa bu alan ANLAMSIZDIR — okunmamalı.
	 */
	SizeBytes int64 `json:"sizeBytes"`

	/*
	 * Used, önbelleğin hiç kullanılıp kullanılmadığı.
	 *
	 * "Henüz kullanılmadı" ile "boş" AYRI ŞEYLER (spec 027 H3). Tek bir sayı
	 * taşınsaydı arayüz hiç çalıştırılmamış bir önbelleği "0 B" diye gösterir
	 * ve kullanıcı onu boşaltılmış sanırdı.
	 */
	Used bool `json:"used"`
}

// VerifyResult, bütünlük taramasının sonucu.
type VerifyResult struct {
	Checked      int `json:"checked"`
	Mismatched   int `json:"mismatched"`
	Unverifiable int `json:"unverifiable"`
	Removed      int `json:"removed"`
}

// CacheAdmin, bağımlılık önbelleklerini yönetir.
//
// Runner uygulaması bunu sağlar; sağlamayan bir uygulamada bakım uçları
// kapalıdır (arayüz isteğe bağlı).
type CacheAdmin interface {
	// CacheStatus, tanımlı bütün önbelleklerin durumunu döner.
	CacheStatus(ctx context.Context) ([]CacheInfo, error)

	// ClearCache, önbelleği boşaltır ve boşalan bayt sayısını döner.
	//
	// Önbellek bağlıysa ErrCacheBusy döner — çağıran bunu "şu an yapılamaz"
	// diye çevirir.
	ClearCache(ctx context.Context, id CacheID) (int64, error)

	/*
	 * VerifyCache, önbelleği tarar ve özetiyle UYUŞMAYAN artefaktları siler.
	 *
	 * Özeti okunamayan artefakt silinmez, "denetlenemedi" sayılır: bozuk bir
	 * özet dosyası yüzünden sağlam bir artefaktı silmek, doğrulamayı
	 * düzeltmesi gereken sorunun kaynağına çevirirdi (spec 027 H5).
	 */
	VerifyCache(ctx context.Context, id CacheID) (VerifyResult, error)
}
