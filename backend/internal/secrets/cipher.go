// Package secrets, kimlik bilgilerini veritabanına yazmadan önce şifreler.
//
// Şifreleme uygulama tarafında yapılır (veritabanı tarafında değil) ki anahtar
// hiçbir zaman SQL sorgusuna, dolayısıyla sorgu loglarına girmesin. Veritabanı
// dökümünü ele geçiren biri şifreleme anahtarı olmadan hiçbir şey okuyamaz.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

const (
	// KeySize, AES-256 için gereken anahtar uzunluğu.
	KeySize = 32

	// currentVersion, şifreli blob'un ilk baytı. İleride anahtar döndürmek
	// gerekirse eski blob'lar sürüm baytından tanınıp eski anahtarla çözülebilir.
	currentVersion byte = 0x01
)

var (
	// ErrKeyMissing, şifreleme anahtarı hiç verilmediğinde döner.
	ErrKeyMissing = errors.New("şifreleme anahtarı tanımlı değil")

	// ErrKeySize, anahtar çözüldüğünde 32 bayt değilse döner.
	ErrKeySize = errors.New("şifreleme anahtarı 32 bayt olmalı")

	// ErrMalformed, şifreli blob bozuk veya çok kısa olduğunda döner.
	ErrMalformed = errors.New("şifreli veri bozuk")

	// ErrVersion, blob bilinmeyen bir sürüm baytı taşıdığında döner.
	ErrVersion = errors.New("şifreli veri sürümü desteklenmiyor")

	// ErrDecrypt, çözme başarısız olduğunda döner — yanlış anahtar veya
	// veriyle oynanmış olması. İkisi bilinçli olarak ayrılmaz: saldırgana
	// hangisinin geçerli olduğu bilgisi verilmez.
	ErrDecrypt = errors.New("şifreli veri çözülemedi")
)

// Cipher, AES-256-GCM ile şifreleme ve çözme yapar.
//
// Blob düzeni: [sürüm:1][nonce:12][şifreli metin + doğrulama etiketi:16]
// GCM kimliği doğrulanmış şifreleme sağlar; veriyle oynanırsa çözme başarısız olur.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher, base64 kodlu bir anahtardan Cipher üretir.
//
// Anahtar eksik veya yanlış uzunluktaysa hata döner — sessizce zayıf bir moda
// düşmez, çünkü şifrelenmediğini fark etmeden çalışan bir sistem en kötü sonuçtur.
// Anahtar üretmek için: openssl rand -base64 32
func NewCipher(base64Key string) (*Cipher, error) {
	if base64Key == "" {
		return nil, ErrKeyMissing
	}

	key, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("şifreleme anahtarı base64 değil: %w", err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("%w (verilen: %d bayt)", ErrKeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher oluşturulamadı: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm oluşturulamadı: %w", err)
	}

	return &Cipher{aead: aead}, nil
}

// Encrypt, düz metni şifreler ve saklanmaya hazır blob döner.
//
// Her çağrıda yeni bir rastgele nonce üretilir; aynı girdi her seferinde
// farklı çıktı verir.
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce üretilemedi: %w", err)
	}

	// Kapasiteyi baştan ayırıp sürüm ve nonce'u önüne yazıyoruz; Seal
	// şifreli metni bu dilimin sonuna ekler.
	out := make([]byte, 0, 1+len(nonce)+len(plaintext)+c.aead.Overhead())
	out = append(out, currentVersion)
	out = append(out, nonce...)

	return c.aead.Seal(out, nonce, plaintext, nil), nil
}

// EncryptString, Encrypt'in metin kolaylık sarmalayıcısı.
func (c *Cipher) EncryptString(plaintext string) ([]byte, error) {
	return c.Encrypt([]byte(plaintext))
}

// Decrypt, Encrypt ile üretilmiş blob'u çözer.
//
// Blob'un tek biti bile değişmişse GCM doğrulaması başarısız olur ve
// ErrDecrypt döner.
func (c *Cipher) Decrypt(blob []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	if len(blob) < 1+nonceSize+c.aead.Overhead() {
		return nil, ErrMalformed
	}
	if blob[0] != currentVersion {
		return nil, fmt.Errorf("%w: 0x%02x", ErrVersion, blob[0])
	}

	nonce := blob[1 : 1+nonceSize]
	ciphertext := blob[1+nonceSize:]

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Asıl hata bilinçli olarak sarmalanmıyor: yanlış anahtar ile
		// oynanmış veri arasındaki fark dışarı sızmasın.
		return nil, ErrDecrypt
	}
	return plaintext, nil
}

// DecryptString, Decrypt'in metin kolaylık sarmalayıcısı.
func (c *Cipher) DecryptString(blob []byte) (string, error) {
	plaintext, err := c.Decrypt(blob)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Mask, bir gizli değerin arayüzde gösterilebilecek ipucunu üretir:
// yalnızca son 4 karakter. Değer çok kısaysa hiçbir şey açığa çıkarmaz.
func Mask(secret string) string {
	const visible = 4
	if len(secret) <= visible {
		return ""
	}
	return secret[len(secret)-visible:]
}
