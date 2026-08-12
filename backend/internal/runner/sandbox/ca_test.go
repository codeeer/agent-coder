package sandbox

import "testing"

/*
 * Kurumsal CA bağlaması.
 *
 * İki şey sınanıyor ve ikisi de güvenlik gereksinimi:
 *   1. Tanımlanmadıysa HİÇBİR bağlama yapılmaz — varsayılan davranış
 *      kullanıcının haberi olmadan değişmez.
 *   2. Tanımlandıysa bağlama SALT OKUNUR olur. Yazılabilir bir sertifika
 *      deposu, çalıştırılan agent'ın güven zincirini değiştirebilmesi
 *      demektir.
 */
func TestCABind(t *testing.T) {
	t.Run("tanımsızsa bağlama yok", func(t *testing.T) {
		if got := caBind(""); got != nil {
			t.Fatalf("boş yol için bağlama üretildi: %v", got)
		}
	})

	t.Run("tanımlıysa salt okunur bağlanır", func(t *testing.T) {
		got := caBind("/etc/pki/kurum.pem")
		if len(got) != 1 {
			t.Fatalf("bağlama sayısı = %d, beklenen 1: %v", len(got), got)
		}
		want := "/etc/pki/kurum.pem:" + ExtraCACertPath + ":ro"
		if got[0] != want {
			t.Fatalf("bağlama = %q, beklenen %q", got[0], want)
		}
	})
}
