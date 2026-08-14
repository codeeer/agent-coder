// Package scripts, agent'ların çalıştırabileceği hazır kabuk betiklerini yönetir.
//
// NEDEN VAR: bir agent adımı her seferinde yeniden karar verir. Keşifte doğru
// olan bu davranış, PROSEDÜRDE risktir — aynı akış iki kez çalıştığında iki
// farklı komut dizisi üretebilir. Betik zaten bunun için var: yazılmış, gözden
// geçirilmiş, her çalıştığında aynı.
//
// Sınır: model betiği NE ZAMAN çağıracağına karar verir, NE YAPACAĞINA betik
// karar verir.
//
// GÜVENLİK SINIRI: betikler yalnızca bash yetkisi AÇIK agent'lara kopyalanır ve
// yetki kurallarına hiç dokunulmaz (spec 012 K2/K3). Bash yetkisi olan bir agent
// betiği bugün de kendisi yazıp çalıştırabiliyor; buradaki kazanç yeni bir
// yetenek değil, çalıştırdığı metnin öngörülebilir olması.
package scripts

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Dir, betiklerin container içindeki dizini.
//
// Klonlanan deponun (`/work`) DIŞINDA: orası klonlama hedefi ve boş olmak
// zorunda; ayrıca bizim dosyalarımız kullanıcının diff'inde görünürdü
// (spec 012 K6).
const Dir = "/home/agent/scripts"

var (
	// ErrNotFound, kayıt yok.
	ErrNotFound = errors.New("betik bulunamadı")
	// ErrMissingName, ad zorunlu.
	ErrMissingName = errors.New("betik adı zorunlu")
	// ErrInvalidName, ad dosya adına uygun değil.
	ErrInvalidName = errors.New("betik adı yalnızca küçük harf, rakam ve - içerebilir")
	// ErrMissingContent, içerik zorunlu.
	ErrMissingContent = errors.New("betik içeriği zorunlu")
	// ErrDuplicateName, aynı ad ikinci kez.
	ErrDuplicateName = errors.New("bu adda bir betik zaten var")
)

// Script, kütüphanedeki tek bir betik.
//
// Gizli değer TAŞIMAZ ve şifrelenmez (spec 012 K5): içerik container içinde düz
// metin olarak duruyor ve agent onu okuyabiliyor. Şifrelemek yanlış bir güvenlik
// hissi verirdi. Gizli değerler betiğin içine değil, ortam değişkenine konur.
type Script struct {
	ID uuid.UUID `json:"id"`
	// Name doğrudan dosya adına dönüşür, o yüzden dar bir karakter kümesiyle
	// sınırlı: kullanıcının yazdığı ad ile sistemin kullandığı yol AYNI olmalı.
	Name string `json:"name"`
	// Description agent'ın talimat dosyasına yazılır — betiğin NE ZAMAN
	// çağrılacağını modele anlatan tek şey budur.
	Description string `json:"description"`
	Content     string `json:"content"`

	// FolderID nil ise script klasörsüz: bugünkü davranış, bugünkü yol.
	FolderID *uuid.UUID `json:"folderId"`
	// FolderName yolu üretmek için JOIN'den gelir; ayrı sorgu yapılmaz.
	FolderName string `json:"folderName,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// FileName, container'daki dosya adı. Uzantıyı kullanıcı değil sistem koyar.
func (s Script) FileName() string { return s.Name + ".sh" }

/*
Path, container içindeki mutlak yol.

TEK ÜRETİCİ. Klasörlü script kendi alt dizinine düşer; klasörsüz olan kökte
kalır ve bugünkü yolu birebir korur. İkinci bir yol üreticisi yazılsaydı
dosyanın yazıldığı yer ile talimatta anlatılan yer ayrışabilirdi — ve model
var olmayan bir yolu denerdi.
*/
func (s Script) Path() string {
	if s.FolderName == "" {
		return Dir + "/" + s.FileName()
	}
	return Dir + "/" + s.FolderName + "/" + s.FileName()
}

// Validate, kaydedilmeye uygun mu diye sınar.
func (s Script) Validate() error {
	name := strings.TrimSpace(s.Name)
	switch {
	case name == "":
		return ErrMissingName
	case !validName(name):
		return ErrInvalidName
	case strings.TrimSpace(s.Content) == "":
		return ErrMissingContent
	}
	return nil
}

// validName, adın dosya adı olarak güvenli olması.
//
// Sessiz dönüştürme YAPILMAZ, baştan reddedilir (MCP sunucu adındaki kararın
// aynısı): `my script` yazan kullanıcı talimatta `my_script.sh` görseydi neden
// tutmadığını anlamazdı. Nokta da yasak — `..` ile dizin dışına çıkmanın yolu
// hiç açılmasın.
func validName(s string) bool {
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !ok {
			return false
		}
	}
	return true
}
