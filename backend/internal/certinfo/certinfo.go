// Package certinfo, bir sertifikanın ekranda gösterilecek özetini çıkarır.
//
// NEDEN AYRI: ürünün en sert kuralı "ölçülmeyen hiçbir şey ekranda
// gösterilmez". Sertifikanın sahibi, imzalayanı ve bitiş tarihi tahmin
// edilmez, kullanıcıya sordurulmaz — sertifikanın KENDİSİNDEN okunur. Bu paket
// o okumayı yapan tek yerdir.
package certinfo

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"
)

// ErrNoCertificate, verilen PEM metninde sertifika yok.
var ErrNoCertificate = errors.New("içerikte sertifika bulunamadı")

// Info, tek bir sertifikanın ekranda görünen özeti.
type Info struct {
	// Subject, sertifikanın kime ait olduğu.
	Subject string `json:"subject"`
	// Issuer, sertifikayı kimin imzaladığı. Kök sertifikada Subject ile aynıdır.
	Issuer string `json:"issuer"`
	// NotAfter, geçerliliğin bittiği an.
	NotAfter time.Time `json:"notAfter"`
	// Expired, süresi dolmuş mu.
	//
	// Alan olarak taşınır, arayüzde hesaplanmaz: "dolmuş mu" sorusunun cevabı
	// hangi saate göre bakıldığına bağlı ve o saat sunucununkidir.
	Expired bool `json:"expired"`
	// SelfSigned, kendi kendini imzalamış mı — kök sertifikaların işareti.
	SelfSigned bool `json:"selfSigned"`
}

// Parse, PEM metnindeki her sertifika için özet üretir.
//
// Girdi normalleştirilmiş PEM olmalıdır (bkz. certfmt.ToPEM). Sertifika
// dışındaki bloklar yok sayılır.
func Parse(pemText string) ([]Info, error) {
	return parseAt(pemText, time.Now())
}

// parseAt, "şimdi"yi dışarıdan alır — süre dolması testte sabit bir ana göre
// sınanabilsin diye.
func parseAt(pemText string, now time.Time) ([]Info, error) {
	var out []Info
	rest := []byte(pemText)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			// Bozuk blok atlanır; geçerli olanlar yine gösterilir.
			continue
		}
		out = append(out, Info{
			Subject:    ad(c.Subject.CommonName, c.Subject.String()),
			Issuer:     ad(c.Issuer.CommonName, c.Issuer.String()),
			NotAfter:   c.NotAfter,
			Expired:    now.After(c.NotAfter),
			SelfSigned: c.Subject.String() == c.Issuer.String(),
		})
	}
	if len(out) == 0 {
		return nil, ErrNoCertificate
	}
	return out, nil
}

// ad, CN varsa onu, yoksa tam ayırt edici adı verir.
//
// CN zorunlu bir alan DEĞİL: yalnızca organizasyon adı taşıyan kurumsal
// sertifikalar var ve onlarda boş bir satır göstermek, bilgiyi hiç
// göstermemekten daha kötü olurdu.
func ad(cn, dn string) string {
	if cn != "" {
		return cn
	}
	return dn
}
