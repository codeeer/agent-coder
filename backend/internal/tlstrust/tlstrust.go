// Package tlstrust, ürünün KENDİ giden HTTPS çağrılarına kurumsal kök
// sertifikayı tanıtır.
//
// NEDEN GEREKLİ: sertifika, agent'ın çalıştığı container'a tanıtıldığında
// yalnızca agent'ın araçları kapsanıyor. Ürünün kendi çağrıları — model
// sağlayıcı doğrulaması, Jira, kod deposu erişimi, model kataloğu — backend
// sürecinden çıkıyor ve o süreç kurumsal sertifikayı bilmiyordu. SSL denetimi
// yapan bir ağda sonuç şuydu: agent çalışıyor ama model sağlayıcıya hiç
// ulaşılamıyor ve hiçbir yerde sebebin sertifika olduğu yazmıyor.
//
// NEDEN GLOBAL DEĞİL: `http.DefaultTransport`'un TLS ayarını değiştirmek
// yedi ayrı istemciyi tek hamlede kapsardı ama global durumu değiştirmek
// testlere sızıyor ve ayar değiştiğinde güvenli yenileme yapılamıyor. Bunun
// yerine taşıyıcı AÇIKÇA enjekte ediliyor.
//
// NEDEN HAZIR `*http.Client` VERMİYOR: böyle bir yardımcı vardı ve hiç
// kullanılmadı — kullanılamazdı. Giden çağrıyı yapan sekiz paketin
// (llm, mcp, catalog, credentials, gitprovider, jira, github, httpapi)
// kurucuları `rt http.RoundTripper` alıyor; hazır istemci alabilmeleri için
// bu paketi import etmeleri, yani yukarıdaki "açıkça enjekte et" kararını
// tersine çevirmeleri gerekirdi. Zaman aşımı da her çağrının kendi kararı
// (15 sn doğrulama, 30 sn Jira/GitHub, MCP'de ayardan). Taşıyıcı verilir,
// istemciyi çağıran kurar.
package tlstrust

import (
	"crypto/tls"
	"crypto/x509"
	"log/slog"
	"net/http"
	"sync"
)

// Trust, geçerli sertifikaya göre taşıyıcı üretir ve sertifika değişince
// kendini yeniler.
type Trust struct {
	pem func() string

	mu sync.RWMutex
	// son, en son kullanılan PEM. Değişip değişmediğini anlamanın ölçüsü.
	son string
	// tr, son PEM'e göre kurulmuş taşıyıcı.
	tr *http.Transport
}

// New, çözümleyiciden beslenen bir güven katmanı kurar.
//
// pem nil olabilir veya boş dönebilir; o durumda sistemin kendi güven havuzu
// kullanılır ve davranış bugünküyle birebir aynı olur.
func New(pem func() string) *Trust {
	return &Trust{pem: pem}
}

/*
 * RoundTripper, backend'in giden çağrılarında kullanılacak taşıyıcı.
 *
 * Sertifikayı ÇAĞRI BAŞINA okur. Pahalı görünüyor ama ayar bellekte tutulan
 * bir dizeden ibaret; asıl iş olan havuz kurulumu yalnızca dize DEĞİŞTİĞİNDE
 * yapılıyor. Karşılığında sertifika arayüzden değiştirildiğinde sonraki çağrı
 * yeni sertifikayı kullanıyor — yeniden başlatma gerekmiyor (spec 017 H5).
 */
func (t *Trust) RoundTripper() http.RoundTripper {
	return roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return t.current().RoundTrip(req)
	})
}

// current, geçerli PEM'e karşılık gelen taşıyıcıyı verir; gerekirse yeniden kurar.
func (t *Trust) current() *http.Transport {
	var istenen string
	if t.pem != nil {
		istenen = t.pem()
	}

	t.mu.RLock()
	if t.tr != nil && t.son == istenen {
		tr := t.tr
		t.mu.RUnlock()
		return tr
	}
	t.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()
	// Kilit beklenirken başkası kurmuş olabilir.
	if t.tr != nil && t.son == istenen {
		return t.tr
	}

	// Eski taşıyıcının boştaki bağlantıları kapatılır: yeni sertifikayla
	// kurulmuş olması gereken bir bağlantı havuzda kalmamalı.
	if t.tr != nil {
		t.tr.CloseIdleConnections()
	}

	t.son = istenen
	t.tr = build(istenen)
	return t.tr
}

/*
 * build, PEM'e göre taşıyıcı kurar.
 *
 * Sertifika SİSTEM HAVUZUNA EKLENİR, yerine geçmez: kurumsal sertifika
 * tanıtıldı diye genel sertifikaların geçersizleşmesi, kurumsal ağda bile
 * doğrudan erişilebilen adresleri kırardı.
 *
 * Sistem havuzu okunamazsa (nadir) boş bir havuzla devam EDİLMEZ — o, her
 * genel adresi bir anda güvenilmez yapardı. Bunun yerine sertifika
 * eklenmeden varsayılan davranışa dönülür.
 */
func build(pemText string) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if pemText == "" {
		return tr
	}

	havuz, err := x509.SystemCertPool()
	if err != nil || havuz == nil {
		slog.Warn("sistem sertifika havuzu okunamadı — kurumsal sertifika eklenmedi",
			"error", err)
		return tr
	}
	if !havuz.AppendCertsFromPEM([]byte(pemText)) {
		slog.Warn("kurumsal sertifika güven havuzuna eklenemedi")
		return tr
	}

	/*
	 * TLS ayarı DEĞİŞTİRİLİR, YERİNE KONMAZ.
	 *
	 * Bu bir hatanın kaydı: `Clone()` çağrısı HTTP/2'yi kurarken
	 * TLSClientConfig'i kendisi dolduruyor ve içine `NextProtos` yazıyor.
	 * Yapılandırmanın tamamı yeni bir `tls.Config` ile değiştirildiğinde o
	 * alan siliniyor ve backend'in TÜM giden çağrıları sessizce HTTP/1.1'e
	 * düşüyordu — sertifikayla hiç ilgisi olmayan bir yan etki.
	 */
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	tr.TLSClientConfig.RootCAs = havuz
	return tr
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
