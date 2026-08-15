package sandbox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

/*
 * Eksik imaj mesajı.
 *
 * İmaj otomatik indirilmiyor; kullanıcının ne yapacağını bu metin söylüyor ve
 * doğru olmak zorunda. Yanlış tarif, kullanıcıyı çalışmayan bir komuta
 * yönlendirir: hazır imajla kuran biri hiçbir şey derlemiyor, kaynaktan kuran
 * biri de bir şey çekemez.
 */

// Sürümlü ve UZAK bir etiket: kullanıcı hazır imajla kurmuş demektir, çekmeli.
func TestImageHint_UzakSurumluImajCekilir(t *testing.T) {
	hint := imageHint("ghcr.io/kurum/agent-coder-runner:node-24.13.0")

	require.Contains(t, hint, "docker pull")
	require.NotContains(t, hint, "make runner", "hazır imajla kuran kişi derlemiyor")
}

// Sürümlü ama YEREL bir etiket: kaynaktan kurulum; o sürümün imajı henüz
// derlenmemiş olabilir.
func TestImageHint_YerelSurumluImajDerlenir(t *testing.T) {
	hint := imageHint("agent-coder/opencode-runner:node-24.13.0")

	require.Contains(t, hint, "make runner")
	require.NotContains(t, hint, "docker pull")
}

// Sürümsüz taban imaj: her zaman `make runner`.
func TestImageHint_SurumsuzTabanImajDerlenir(t *testing.T) {
	require.Contains(t, imageHint("agent-coder/opencode-runner:latest"), "make runner")
	require.Contains(t, imageHint("ghcr.io/kurum/agent-coder-runner:latest"), "make runner")
}

/*
SÜRÜM ÖNEKİ `runner.ImageFor` İLE AYNI OLMAK ZORUNDA.

Sabit burada BİLEREK tekrarlanıyor: sandbox paketi runner'ı import edemez
(runner zaten bu paketi kullanıyor, ters bağımlılık döngü olurdu). Bilerek
yapılan tekrar da tekrardır — biri değişip diğeri kalırsa mesaj sessizce
yanlış tarifi verir.

Eşleşmenin uçtan uca kanıtı `EnsureImage` testinde; buradaki, sabitin kendisi
değişirse haber veren en ucuz kayıt.
*/
func TestNodeTagPrefix_DegismezKayit(t *testing.T) {
	require.Equal(t, "node-", nodeTagPrefix,
		"runner.ImageFor bu önekle etiket üretiyor")
}
