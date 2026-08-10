package llm

import (
	"context"
	"log/slog"
)

// Bootstrap, hiç sağlayıcı tanımlı değilse ortam değişkenindeki OpenRouter
// anahtarından bir sağlayıcı oluşturur.
//
// Spec 001'de `.env` anahtarı bir "yedek çözümleme" idi. Sağlayıcılar artık
// veritabanı satırı olduğu için o yaklaşım anlamını yitirdi; yerine bu geldi.
// Böylece mevcut kurulum hiçbir şey yapmadan çalışmaya devam eder ve anahtar
// arayüzde görünür hale gelir.
//
// Kural bilinçli olarak "tablo TAMAMEN boşsa": kullanıcı sağlayıcıyı silip
// yeniden başlatırsa geri gelir. Bunu istemeyen .env'deki değişkeni boşaltır.
func Bootstrap(ctx context.Context, store *Store, envKey string) error {
	if envKey == "" {
		return nil
	}

	count, err := store.Count(ctx)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	p, err := store.Create(ctx, CreateInput{
		Type:      TypeOpenRouter,
		Name:      "OpenRouter",
		Secret:    envKey,
		IsDefault: true,
	})
	if err != nil {
		return err
	}

	slog.InfoContext(ctx, "ortam değişkeninden OpenRouter sağlayıcısı oluşturuldu",
		"id", p.ID, "hint", p.Hint)
	return nil
}
