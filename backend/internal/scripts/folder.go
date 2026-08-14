package scripts

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrFolderNotFound, klasör yok.
	ErrFolderNotFound = errors.New("klasör bulunamadı")
	// ErrDuplicateFolder, aynı adda klasör zaten var.
	ErrDuplicateFolder = errors.New("bu adda bir klasör zaten var")
)

/*
Folder, bir kampanyanın script'lerini toplayan klasör.

NEDEN VAR: standart bir yükseltme (örn. Node 18 → 24) yedi adımdan oluşabiliyor
ve bu adımlar düz bir kütüphanede diğer kampanyaların script'leriyle
karışıyordu. Klasör üç şey veriyor: kampanyaya bir isim, tek hamlede
atanabilirlik, ve container'da kendi dizini.

SIRA TUTMAZ. Adımların sırası ADLARINDAN gelir (`01-`, `02-`). Ayrı bir sıra
alanı ikinci bir doğruluk kaynağı olurdu: container'a bakan biri dosya adlarına
göre başka bir sıra görebilirdi.
*/
type Folder struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	// Description agent'ın talimatına yazılır: KAMPANYANIN NE OLDUĞUNU model
	// buradan öğrenir. Tek tek script açıklamaları bunu anlatamaz.
	Description string `json:"description"`
	// ScriptCount listede gösterilir; ayrı sorgu değil, JOIN'den gelir.
	ScriptCount int       `json:"scriptCount"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Path, klasörün container içindeki dizini.
func (f Folder) Path() string { return Dir + "/" + f.Name }

// FolderInput, klasör oluşturma ve güncelleme alanları.
type FolderInput struct {
	Name        string
	Description string
}

/*
Validate, klasörün kaydedilmeye uygun olduğunu sınar.

Ad kuralı SCRIPT'İNKİYLE AYNI ve aynı fonksiyondan geliyor (`validName`):
ikisi de bir dosya sistemi adına dönüşüyor ve kural kopyalansaydı biri
gevşetildiğinde diğeri geride kalırdı.
*/
func (in FolderInput) Validate() error {
	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		return ErrMissingName
	case !validName(name):
		return ErrInvalidName
	}
	return nil
}
