// Package mcp, agent'ların erişebileceği MCP sunucularını yönetir.
//
// MCP (Model Context Protocol), bir dil modelinin dış araçlara standart bir
// arayüzle erişmesini sağlar. Her yeni kaynak için ayrı bir istemci yazmak
// yerine tek bir protokol konuşulur.
//
// SORUMLULUK SINIRI: bu paket sunucu TANIMLARINI saklar ve doğrular. Araçları
// asıl çağıran, agent'ın içinde koştuğu çalıştırma motorudur — biz ona yalnızca
// hangi sunucuya nasıl bağlanacağını söyleriz (`internal/runner/config.go`).
package mcp

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Transport, sunucuya hangi taşımayla bağlanıldığı.
//
// Yerel (stdio) sunucular BİLİNÇLİ OLARAK yok (spec 011 K2): komutun çalıştırma
// imajının içinde olmasını gerektirirdi ve "yeni entegrasyon için kod yazma"
// sorununu çözmek yerine yer değiştirmiş olurduk.
type Transport string

const (
	// TransportHTTP, streamable HTTP taşıması — güncel MCP sunucularının çoğu.
	TransportHTTP Transport = "http"
	// TransportSSE, sunucu tarafı olaylarıyla eski taşıma.
	TransportSSE Transport = "sse"
)

// AllTransports, arayüzün seçenek listesi.
var AllTransports = []Transport{TransportHTTP, TransportSSE}

// Valid, taşımanın tanımlı olup olmadığı.
func (t Transport) Valid() bool {
	return t == TransportHTTP || t == TransportSSE
}

var (
	// ErrNotFound, kayıt yok.
	ErrNotFound = errors.New("MCP sunucusu bulunamadı")
	// ErrInvalidTransport, tanınmayan taşıma.
	ErrInvalidTransport = errors.New("geçersiz taşıma türü")
	// ErrMissingName, ad zorunlu.
	ErrMissingName = errors.New("sunucu adı zorunlu")
	// ErrMissingURL, adres zorunlu.
	ErrMissingURL = errors.New("sunucu adresi zorunlu")
	// ErrInvalidURL, adres ayrıştırılamıyor veya http/https değil.
	ErrInvalidURL = errors.New("sunucu adresi http veya https olmalı")
	// ErrInvalidName, ad araç adlandırmasına uygun değil.
	ErrInvalidName = errors.New("sunucu adı yalnızca harf, rakam, - ve _ içerebilir")
	// ErrDuplicateName, aynı ad ikinci kez.
	ErrDuplicateName = errors.New("bu adda bir sunucu zaten var")
	// ErrUnreachable, sunucuya bağlanılamadı.
	ErrUnreachable = errors.New("MCP sunucusuna ulaşılamadı")
	// ErrToolFailed, araç çağrısı başarısız.
	ErrToolFailed = errors.New("araç çağrısı başarısız")
)

// Server, tanımlı bir MCP sunucusu.
//
// Gizli değeri BİLİNÇLİ OLARAK taşımaz: bu tip API yanıtlarına ve loglara
// giriyor. Erişimin tek yolu `Store.Reveal`.
type Server struct {
	ID uuid.UUID `json:"id"`
	// Name, araç adlarının önekidir: `sentry` sunucusunun `issue` aracı
	// modele `sentry_issue` olarak görünür. Bu yüzden dar bir karakter
	// kümesiyle sınırlı.
	Name      string    `json:"name"`
	Transport Transport `json:"transport"`
	URL       string    `json:"url"`
	// Hint, kaydedilmiş erişim anahtarının son dört karakteri. Anahtar yoksa boş.
	Hint string `json:"hint"`
	// HasSecret, kimlik doğrulama tanımlı mı — bazı sunucular anahtarsız çalışır.
	HasSecret bool `json:"hasSecret"`
	// Tools, son doğrulamada sunucunun bildirdiği araç adları.
	//
	// Saklanıyor çünkü kullanıcı bir agent'a erişim verirken NEYE erişim
	// verdiğini görmeli; her ekran açılışında sunucuya gitmek hem yavaş hem de
	// sunucu geçici olarak kapalıysa listeyi boş gösterirdi.
	Tools     []string  `json:"tools"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

/*
ARAÇ DESENİ VE ORTAM DEĞİŞKENİ ADI BURADA ÜRETİLMEZ — ikisi de `runner`'ın işi
(`runner.MCPEnvVar`, `BuildPermissions`).

Bu paketin kendi `ToolPattern()` ve `EnvVar()` metotları vardı ve üretimde hiç
çağrılmıyordu; yalnızca kendi testleri tutuyordu. İkinci bir uygulama olarak
durmaları tehlikeliydi:

  - `ToolPattern()` adı temizleyip desen üretiyordu, canlı olan
    (`runner.mcpToolPattern`) temizlemiyor. Bugün fark görünmüyor çünkü
    `Validate` geçersiz adı zaten reddediyor — yani ayrışma açık değil, gizli.
  - `EnvVar()` ile `runner.MCPEnvVar` aynı dizeyi ÜRETMEK ZORUNDA: biri
    anahtarı container'ın ortamına yazıyor, diğeri yapılandırma dosyasından ona
    referans veriyor. Ayrıştıkları an agent, var olmayan bir değişkene bakar ve
    MCP sunucusu sessizce anahtarsız kalır.

İki tarafın uyumunu sağlayan şey ortak kod değil `Validate`: ad yalnızca
harf, rakam, `-` ve `_` içerebildiği için her iki uygulama da aynı sonucu
veriyor. Kuralın sahibi o.
*/

// Validate, kaydedilmeye uygun mu diye sınar.
func (s Server) Validate() error {
	name := strings.TrimSpace(s.Name)
	switch {
	case name == "":
		return ErrMissingName
	case !validName(name):
		return ErrInvalidName
	case !s.Transport.Valid():
		return fmt.Errorf("%w: %q", ErrInvalidTransport, s.Transport)
	case strings.TrimSpace(s.URL) == "":
		return ErrMissingURL
	}

	u, err := url.Parse(strings.TrimSpace(s.URL))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrInvalidURL
	}
	return nil
}

// validName, adın araç adlandırmasında güvenle kullanılabilir olması.
//
// Çalıştırma motoru izin verilmeyen karakterleri alt çizgiye çeviriyor; biz
// baştan reddediyoruz ki kullanıcının yazdığı ad ile modelin gördüğü araç adı
// AYNI olsun. Aksi halde `my.server` yazan kullanıcı yetki kuralında
// `my_server_*` görür ve neden tutmadığını anlamaz.
func validName(s string) bool {
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			return false
		}
	}
	return true
}
