// Package paging, liste uçlarının ortak sayfalama kurallarını tutar.
//
// Tek yerde durmasının sebebi: her listede ayrı ayrı yazılan sınır kontrolü
// er geç ayrışır — biri 50, biri 100 varsayılan kullanır, biri eksi offset'i
// kontrol etmeyi unutur. Sayfalama bir sunum ayrıntısı değil, veritabanını
// koruyan bir sınırdır.
package paging

// Varsayılan ve azami sayfa boyutu.
//
// Azami sınır kullanıcı için değil VERİTABANI için: `?limit=100000` isteyen bir
// çağrı tüm tabloyu belleğe çeker. Seçim ekranları (açılır listeler) bilinçli
// olarak Max'e kadar isteyebilir.
const (
	Default = 25
	Max     = 200
)

// Page, bir listenin sayfa penceresi.
type Page struct {
	Limit  int
	Offset int
}

// Clamp, gelen değerleri geçerli aralığa çeker.
//
// Geçersiz değer HATA DEĞİLDİR: liste uçları bir aracın değil bir insanın
// eliyle de çağrılıyor ve `?limit=abc` yüzünden 400 dönmek, listeyi hiç
// göstermemek demek olurdu. Bozuk değer varsayılana düşer.
func Clamp(limit, offset int) Page {
	if limit <= 0 || limit > Max {
		limit = Default
	}
	if offset < 0 {
		offset = 0
	}
	return Page{Limit: limit, Offset: offset}
}
