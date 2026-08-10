package scripts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScriptValidate(t *testing.T) {
	ok := Script{Name: "upgrade-deps", Content: "#!/bin/bash\n"}

	t.Run("geçerli", func(t *testing.T) {
		require.NoError(t, ok.Validate())
	})

	t.Run("ad zorunlu", func(t *testing.T) {
		s := ok
		s.Name = "  "
		require.ErrorIs(t, s.Validate(), ErrMissingName)
	})

	t.Run("içerik zorunlu", func(t *testing.T) {
		s := ok
		s.Content = "\n  \n"
		require.ErrorIs(t, s.Validate(), ErrMissingContent)
	})

	/*
	 * Ad doğrudan DOSYA ADINA dönüşüyor. Sessizce dönüştürmek yerine baştan
	 * reddediliyor: `my script` yazan kullanıcı talimatta `my_script.sh` görseydi
	 * neden tutmadığını anlamazdı.
	 *
	 * Nokta da yasak — `..` ile dizin dışına çıkmanın yolu hiç açılmasın.
	 */
	t.Run("ad dosya adına uygun olmalı", func(t *testing.T) {
		for _, name := range []string{
			"my script", "Upgrade", "upgrade.sh", "../etc/passwd", "up/grade", "üst",
		} {
			s := ok
			s.Name = name
			require.ErrorIs(t, s.Validate(), ErrInvalidName, "kabul edilmemeliydi: %q", name)
		}
	})
}

func TestScriptPath(t *testing.T) {
	s := Script{Name: "upgrade-deps"}

	require.Equal(t, "upgrade-deps.sh", s.FileName(), "uzantıyı kullanıcı değil sistem koyar")
	require.Equal(t, "/home/agent/scripts/upgrade-deps.sh", s.Path())

	// Klonlanan deponun DIŞINDA: orası boş olmak zorunda ve bizim dosyalarımız
	// kullanıcının diff'ine karışırdı (spec 012 K6).
	require.False(t, strings.HasPrefix(s.Path(), "/work"))
}

/*
 * Windows satır sonları.
 *
 * Yapıştırılan bir betikte `\r` kalırsa shebang satırı `#!/bin/bash\r` olarak
 * okunuyor ve kabuk "command not found" diyor — kullanıcının EKRANDA GÖREMEDİĞİ
 * bir karakter yüzünden. Hata ayıklaması en pahalı sorunlardan biri, o yüzden
 * kayıt anında temizleniyor.
 */
func TestNormalizeContent(t *testing.T) {
	require.Equal(t, "#!/bin/bash\necho a\n", normalizeContent("#!/bin/bash\r\necho a"))
	require.Equal(t, "a\nb\n", normalizeContent("a\rb\r"))
	require.Equal(t, "a\n", normalizeContent("a\n"), "zaten düzgünse dokunulmaz")
	require.Equal(t, "", normalizeContent(""), "boş içerik doğrulamada yakalanır")
}
