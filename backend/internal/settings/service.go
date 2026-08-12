package settings

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrUnknownKey, kayıt defterinde olmayan anahtar.
	ErrUnknownKey = errors.New("bilinmeyen ayar")

	// ErrOutOfRange, değer izin verilen aralığın dışında.
	ErrOutOfRange = errors.New("değer izin verilen aralıkta değil")

	// ErrInvalidValue, değer ayarın tipine uymuyor.
	ErrInvalidValue = errors.New("değer bu ayarın tipine uymuyor")
)

// Value, bir ayarın arayüze verilen hali: tanım + mevcut değer + sapma bilgisi.
type Value struct {
	Definition
	Value string `json:"value"`
	// IsCustom, değerin varsayılandan farklı olup olmadığı.
	IsCustom bool `json:"isCustom"`
}

// Service, ayarları okur ve yazar.
//
// Okuma çok sık yapılır (her çalıştırma kontrolünde), bu yüzden değerler bellekte
// önbelleklenir. Yazma önbelleği tazeler — böylece değişiklik sunucu yeniden
// başlatılmadan geçerli olur (spec 003 H7).
type Service struct {
	pool *pgxpool.Pool

	mu       sync.RWMutex
	override map[string]string // yalnızca varsayılandan sapanlar
}

// NewService yeni servis üretir. Kullanmadan önce Load çağrılmalıdır.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, override: map[string]string{}}
}

// Load, veritabanındaki sapmaları belleğe alır.
func (s *Service) Load(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return fmt.Errorf("ayarlar okunamadı: %w", err)
	}
	defer rows.Close()

	loaded := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return fmt.Errorf("ayar taranamadı: %w", err)
		}
		// Kayıt defterinden kaldırılmış bir anahtar veritabanında kalmış olabilir;
		// yok sayılır, silinmez (kod geri alınırsa değeri kaybolmasın).
		if _, ok := Lookup(k); ok {
			loaded[k] = v
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("ayarlar okunamadı: %w", err)
	}

	s.mu.Lock()
	s.override = loaded
	s.mu.Unlock()
	return nil
}

// raw, anahtarın geçerli ham değerini döner (sapma varsa o, yoksa varsayılan).
func (s *Service) raw(key string) (string, Definition, bool) {
	def, ok := Lookup(key)
	if !ok {
		return "", Definition{}, false
	}

	s.mu.RLock()
	v, custom := s.override[key]
	s.mu.RUnlock()

	if !custom {
		return def.Default, def, true
	}
	return v, def, true
}

// Int, tam sayı ayarını döner.
//
// Bilinmeyen anahtar veya bozuk değer durumunda VARSAYILANA düşer ve panik etmez:
// bir ayar hatası yüzünden çalışan sistemin durması, yanlış değerle çalışmasından
// daha kötü olurdu. Doğrulama zaten yazma anında yapılıyor.
func (s *Service) Int(key string) int {
	v, def, ok := s.raw(key)
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		n, _ = strconv.Atoi(def.Default)
	}
	return n
}

// Bool, mantıksal ayarını döner.
func (s *Service) Bool(key string) bool {
	v, def, ok := s.raw(key)
	if !ok {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		b, _ = strconv.ParseBool(def.Default)
	}
	return b
}

// Text, metin ayarını döner.
func (s *Service) Text(key string) string {
	v, _, _ := s.raw(key)
	return v
}

// All, arayüz için tüm ayarları kayıt defteri sırasıyla döner.
func (s *Service) All() []Value {
	out := make([]Value, 0, len(Registry))
	for _, def := range Registry {
		s.mu.RLock()
		v, custom := s.override[def.Key]
		s.mu.RUnlock()

		if !custom {
			v = def.Default
		}
		out = append(out, Value{Definition: def, Value: v, IsCustom: custom})
	}
	return out
}

// Set, bir ayarı değiştirir. Değer doğrulanır ve önbellek hemen tazelenir.
//
// Varsayılana eşit bir değer verilirse sapma kaydı SİLİNİR — böylece kod
// güncellendiğinde yeni varsayılan bu ayarda da geçerli olur.
func (s *Service) Set(ctx context.Context, key, value string) error {
	def, ok := Lookup(key)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}

	value = strings.TrimSpace(value)
	if err := Validate(def, value); err != nil {
		return err
	}

	if value == def.Default {
		return s.Reset(ctx, key)
	}

	const q = `
		INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`

	if _, err := s.pool.Exec(ctx, q, key, value); err != nil {
		return fmt.Errorf("ayar kaydedilemedi: %w", err)
	}

	s.mu.Lock()
	s.override[key] = value
	s.mu.Unlock()
	return nil
}

// Reset, ayarı varsayılanına döndürür.
func (s *Service) Reset(ctx context.Context, key string) error {
	if _, ok := Lookup(key); !ok {
		return fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM settings WHERE key = $1`, key); err != nil {
		return fmt.Errorf("ayar sıfırlanamadı: %w", err)
	}

	s.mu.Lock()
	delete(s.override, key)
	s.mu.Unlock()
	return nil
}

// Validate, bir değerin tanıma uygunluğunu sınar.
// Kayıt defterinin kendi tutarlılığını test etmek için de kullanılır.
func Validate(def Definition, value string) error {
	switch def.Kind {
	case KindInt:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%w: tam sayı bekleniyor", ErrInvalidValue)
		}
		if def.Min != nil && n < *def.Min {
			return rangeError(def)
		}
		if def.Max != nil && n > *def.Max {
			return rangeError(def)
		}
		return nil

	case KindBool:
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%w: doğru/yanlış bekleniyor", ErrInvalidValue)
		}
		return nil

	case KindText:
		// Opsiyonel ayarda boş, "kapalı" demektir — hata değil.
		if value == "" {
			if def.Optional {
				return nil
			}
			return fmt.Errorf("%w: boş olamaz", ErrInvalidValue)
		}
		return validateText(def, value)

	default:
		return fmt.Errorf("%w: tanımsız tip %q", ErrInvalidValue, def.Kind)
	}
}

// rangeError, kullanıcıya aralığı söyleyen hata üretir.
func rangeError(def Definition) error {
	switch {
	case def.Min != nil && def.Max != nil:
		return fmt.Errorf("%w: %d ile %d arasında olmalı", ErrOutOfRange, *def.Min, *def.Max)
	case def.Min != nil:
		return fmt.Errorf("%w: en az %d olmalı", ErrOutOfRange, *def.Min)
	case def.Max != nil:
		return fmt.Errorf("%w: en fazla %d olmalı", ErrOutOfRange, *def.Max)
	default:
		return ErrOutOfRange
	}
}

/*
 * validateText, metin ayarlarına anahtar bazında ek kural uygular.
 *
 * Kayıt defteri her ayara özel doğrulama taşımıyor (Kind + Min/Max yeterliydi)
 * ama paket deposu adresi bir istisna: kimlik bilgisi gömülü bir URL hem
 * veritabanına hem LOG'a düz metin olarak düşerdi — `putSetting` değeri
 * loglar. Aynı kural projelerin depo adresinde de var.
 */
func validateText(def Definition, value string) error {
	switch def.Key {
	case KeyNPMRegistry:
		u, err := url.Parse(value)
		if err != nil {
			return fmt.Errorf("%w: adres ayrıştırılamadı", ErrInvalidValue)
		}
		if u.Scheme != "https" && u.Scheme != "http" {
			return fmt.Errorf("%w: https:// ile başlamalı", ErrInvalidValue)
		}
		if u.Host == "" {
			return fmt.Errorf("%w: sunucu adı yok", ErrInvalidValue)
		}
		if u.User != nil {
			return fmt.Errorf("%w: adres kullanıcı adı/parola içermemeli — "+
				"kimlik doğrulama bölümünü kullanın", ErrInvalidValue)
		}
	}
	return nil
}
