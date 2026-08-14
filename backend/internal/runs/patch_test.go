package runs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Yamadan veri taşımayan ikili blokların ayıklanması.
 *
 * GERÇEK BİR HATANIN KAYDI: `mvn` çalıştıran bir koşuda `target/` altındaki
 * JAR'lar değişiklik sayıldı (deponun `.gitignore`'u yok). opencode'un ürettiği
 * diff bu dosyalar için yalnızca şunu yazıyor:
 *
 *     index 0000000..091cb04
 *     Binary files /dev/null and b/….jar differ
 *
 * Yani kısaltılmış index ve HİÇ YÜK YOK — `git diff --binary` çıktısı değil.
 * `git apply` bunu uygulayamıyor ve tek blok TÜM yamayı düşürüyordu: dokuz
 * dosyanın yedisi düzgün metin olduğu hâlde hiçbiri gönderilemiyordu.
 */

const metinBlogu = `diff --git a/src/app.go b/src/app.go
index 1111111..2222222 100644
--- a/src/app.go
+++ b/src/app.go
@@ -1,2 +1,3 @@
 package main
+// yeni satır
`

func TestBinariAyikla_YuksuzIkiliBlokAtilir(t *testing.T) {
	diff := metinBlogu + `diff --git a/target/app.jar b/target/app.jar
new file mode 100644
index 0000000..091cb04
Binary files /dev/null and b/target/app.jar differ
`

	temiz, atlanan := stripUnappliableBinary(diff)

	require.Equal(t, []string{"target/app.jar"}, atlanan)
	require.Contains(t, temiz, "src/app.go", "metin bloğu korunmalı")
	require.NotContains(t, temiz, "app.jar", "ikili blok çıkarılmalı")
}

/*
 * YÜK TAŞIYAN ikili blok KORUNUR.
 *
 * `git diff --binary` ile üretilmiş bir yama `GIT binary patch` bölümü taşır ve
 * `git apply` onu uygulayabilir. Onu da atmak, uygulanabilir bir değişikliği
 * sebepsiz çöpe atmak olurdu.
 */
func TestBinariAyikla_YukTasiyanIkiliKorunur(t *testing.T) {
	diff := `diff --git a/logo.png b/logo.png
index 1111111..2222222 100644
GIT binary patch
literal 12
TcmZQzU|?

literal 0
HcmV?d00001

`

	temiz, atlanan := stripUnappliableBinary(diff)

	require.Empty(t, atlanan)
	require.Equal(t, diff, temiz)
}

func TestBinariAyikla_YalnizcaMetinDegismez(t *testing.T) {
	temiz, atlanan := stripUnappliableBinary(metinBlogu)

	require.Empty(t, atlanan)
	require.Equal(t, metinBlogu, temiz)
}

// Yol BOŞLUK içerebilir — gerçek vakada öyleydi ("Jwt Authentication with
// Spring Boot 3.1/target/…"). `diff --git` satırını ayrıştırmak boşlukta
// belirsiz; ad "Binary files" satırından alınıyor.
func TestBinariAyikla_BosluklaYol(t *testing.T) {
	diff := `diff --git a/Jwt Authentication 3.1/target/x.jar b/Jwt Authentication 3.1/target/x.jar
new file mode 100644
index 0000000..091cb04
Binary files /dev/null and b/Jwt Authentication 3.1/target/x.jar differ
`

	_, atlanan := stripUnappliableBinary(diff)

	require.Equal(t, []string{"Jwt Authentication 3.1/target/x.jar"}, atlanan)
}

// Değiştirilen (yeni olmayan) ikili dosyada iki taraf da yazılı.
func TestBinariAyikla_DegistirilmisIkili(t *testing.T) {
	diff := `diff --git a/logo.png b/logo.png
index 1111111..2222222 100644
Binary files a/logo.png and b/logo.png differ
`

	temiz, atlanan := stripUnappliableBinary(diff)

	require.Equal(t, []string{"logo.png"}, atlanan)
	require.Empty(t, strings.TrimSpace(temiz))
}

func TestBinariAyikla_BosDiff(t *testing.T) {
	temiz, atlanan := stripUnappliableBinary("")
	require.Empty(t, atlanan)
	require.Empty(t, temiz)
}
