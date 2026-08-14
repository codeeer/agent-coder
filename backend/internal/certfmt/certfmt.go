// Package certfmt, kurumsal kök sertifikayı hangi biçimde gelirse gelsin PEM'e
// çevirir.
//
// NEDEN VAR: sertifikayı tüketen dört yerin dördü de YALNIZCA PEM okuyor —
// Node (`NODE_EXTRA_CA_CERTS`), git (`GIT_SSL_CAINFO`), curl
// (`CURL_CA_BUNDLE`) ve Go'nun güven havuzu. Buna karşılık kurumsal ekiplerin
// dağıttığı dosya çoğu zaman PEM DEĞİL: Windows'un dışa aktarma sihirbazı
// ikili (DER) ve zincir taşıyan (PKCS#7) biçimleri de üretiyor ve üçü de aynı
// `.crt` / `.cer` uzantısıyla geliyor. Yani kullanıcı elindekinin hangisi
// olduğunu bilmiyor.
//
// Bu yüzden çevirme bir kolaylık değil, ZORUNLULUK. Alternatifi kullanıcıya
// "önce şu openssl komutunu çalıştırın" demekti.
package certfmt

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// ErrNoCertificate, verilen içerikte hiç sertifika bulunamadı.
var ErrNoCertificate = errors.New("içerikte sertifika bulunamadı")

// pemBlockCertificate, PEM blok başlığındaki sertifika türü.
const pemBlockCertificate = "CERTIFICATE"

// ToPEM, ham içeriği normalleştirilmiş PEM metnine çevirir.
//
// Sırayla denenir: PEM → PKCS#7 (ikili) → DER → çıplak base64. İlk sertifika
// üreten kabul edilir; hiçbiri üretmezse ErrNoCertificate.
//
// SERTİFİKA DIŞINDAKİ HER BLOK ATILIR. Kullanıcının seçtiği dosya sertifikanın
// yanında özel anahtar da taşıyabilir (birçok araç ikisini tek dosyada verir);
// o anahtarın veritabanına yazılması kabul edilemez.
func ToPEM(raw []byte) (string, error) {
	certs, err := parse(raw)
	if err != nil {
		return "", err
	}
	if len(certs) == 0 {
		return "", ErrNoCertificate
	}

	var b strings.Builder
	for _, c := range certs {
		if err := pem.Encode(&b, &pem.Block{Type: pemBlockCertificate, Bytes: c.Raw}); err != nil {
			return "", fmt.Errorf("sertifika PEM olarak yazılamadı: %w", err)
		}
	}
	return b.String(), nil
}

// parse, ham içerikten sertifikaları çıkarır.
func parse(raw []byte) ([]*x509.Certificate, error) {
	if certs := fromPEM(raw); len(certs) > 0 {
		return certs, nil
	}
	// İkili biçimler. PKCS#7 önce denenir: bir PKCS#7 gövdesi DER olarak da
	// ayrıştırılabilecek bir SEQUENCE ile başlar, ters sırada denense zincir
	// tek sertifikaya düşerdi.
	if certs := fromPKCS7(raw); len(certs) > 0 {
		return certs, nil
	}
	if certs, err := x509.ParseCertificates(raw); err == nil && len(certs) > 0 {
		return certs, nil
	}
	// Başlıksız base64 — kullanıcı bir portaldan gövdeyi kopyaladığında böyle
	// geliyor. Çözülüp aynı ikili yollar bir kez daha denenir.
	if decoded, ok := decodeBase64(raw); ok {
		if certs := fromPKCS7(decoded); len(certs) > 0 {
			return certs, nil
		}
		if certs, err := x509.ParseCertificates(decoded); err == nil && len(certs) > 0 {
			return certs, nil
		}
	}
	return nil, ErrNoCertificate
}

// fromPEM, PEM bloklarından sertifikaları toplar.
//
// Ayrıştırılamayan bir CERTIFICATE bloğu SESSİZCE ATLANMAZ diye düşünülebilir
// ama atlanır: dosyada bir bozuk blok varken diğerlerinin geçerli olması
// mümkün ve kullanıcıya "hiç sertifika yok" demek yanlış olurdu. Hiçbiri
// ayrıştırılamazsa zaten boş dönülür ve çağıran hata üretir.
func fromPEM(raw []byte) []*x509.Certificate {
	var out []*x509.Certificate
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return out
		}
		switch block.Type {
		case pemBlockCertificate:
			if c, err := x509.ParseCertificate(block.Bytes); err == nil {
				out = append(out, c)
			}
		case "PKCS7":
			// PKCS#7 metin biçiminde de dağıtılıyor.
			out = append(out, fromPKCS7(block.Bytes)...)
		}
		// Diğer her blok (özel anahtar dahil) atılır.
	}
}

// decodeBase64, boşlukları temizleyip base64 çözer.
func decodeBase64(raw []byte) ([]byte, bool) {
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, string(raw))
	if clean == "" {
		return nil, false
	}
	decoded, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, false
	}
	return decoded, true
}

/*
 * PKCS#7 okuma.
 *
 * Standart kütüphanede PKCS#7 ayrıştırıcısı YOK ve bunun için bir bağımlılık
 * eklenmedi (plan 017: Değerlendirilen alternatifler). İhtiyaç duyulan şey dar
 * ve sabit: SignedData içindeki sertifika kümesi.
 *
 * İMZA DOĞRULANMAZ, İÇERİK YORUMLANMAZ. Burası bir güvenlik denetimi değil,
 * bir biçim dönüştürücüsü: kullanıcının zaten güvenmeye karar verdiği kök
 * sertifikayı kapsayıcısından çıkarır. Güven kararı sertifikayı ayara
 * yazmakla verilir.
 *
 * RFC 2315 §9.1:
 *   ContentInfo ::= SEQUENCE { contentType OID, content [0] EXPLICIT ANY }
 *   SignedData  ::= SEQUENCE { version, digestAlgorithms SET, contentInfo,
 *                              certificates [0] IMPLICIT OPTIONAL,
 *                              crls [1] IMPLICIT OPTIONAL, signerInfos SET }
 */

// oidSignedData, 1.2.840.113549.1.7.2.
var oidSignedData = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 7, 2}

type contentInfo struct {
	ContentType asn1.ObjectIdentifier
	Content     asn1.RawValue `asn1:"explicit,optional,tag:0"`
}

// signedData, alanların TAMAMINI taşır.
//
// Eksik bırakılamaz: encoding/asn1, struct alanları bittiğinde dizide veri
// kalırsa "trailing data" hatası veriyor. Okunmayan alanlar da RawValue olarak
// karşılanmak zorunda.
type signedData struct {
	Version          int
	DigestAlgorithms asn1.RawValue
	ContentInfo      asn1.RawValue
	Certificates     asn1.RawValue `asn1:"optional,tag:0"`
	CRLs             asn1.RawValue `asn1:"optional,tag:1"`
	SignerInfos      asn1.RawValue
}

// fromPKCS7, PKCS#7 gövdesindeki sertifikaları döndürür. Gövde PKCS#7 değilse
// boş döner — hata değil, "bu biçim değilmiş" demektir.
func fromPKCS7(der []byte) []*x509.Certificate {
	var ci contentInfo
	if _, err := asn1.Unmarshal(der, &ci); err != nil {
		return nil
	}
	if !ci.ContentType.Equal(oidSignedData) || len(ci.Content.Bytes) == 0 {
		return nil
	}

	var sd signedData
	if _, err := asn1.Unmarshal(ci.Content.Bytes, &sd); err != nil {
		return nil
	}
	if len(sd.Certificates.Bytes) == 0 {
		return nil
	}

	// [0] IMPLICIT içeriği, arka arkaya eklenmiş DER sertifikalardır.
	certs, err := x509.ParseCertificates(sd.Certificates.Bytes)
	if err != nil {
		return nil
	}
	return certs
}
