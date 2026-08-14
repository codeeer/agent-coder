package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRegistry_HerVarsayilanKendiAraligindaOlmali, kayıt defterinin kendi
// tutarlılığını sınar.
//
// Bu test bir tanım hatasını yakalar: min/max'ı varsayılanla çelişen bir ayar
// eklenirse, kullanıcı ayarlar ekranını açtığı anda geçersiz bir değer görür ve
// hiçbir şeyi kaydedemez. Kod sabiti olduğu için testte yakalanması gerekir.
func TestRegistry_HerVarsayilanKendiAraligindaOlmali(t *testing.T) {
	for _, def := range Registry {
		t.Run(def.Key, func(t *testing.T) {
			require.NoError(t, Validate(def, def.Default),
				"varsayılan değer kendi doğrulamasından geçmeli")
		})
	}
}

func TestRegistry_ZorunluAlanlarDolu(t *testing.T) {
	for _, def := range Registry {
		t.Run(def.Key, func(t *testing.T) {
			require.NotEmpty(t, def.Key)
			require.NotEmpty(t, def.Group, "grup boş olamaz — arayüz başlığa göre gruplar")
			require.NotEmpty(t, def.Label, "etiket boş olamaz — arayüzde gösterilir")
			require.NotEmpty(t, def.Help, "açıklama boş olamaz — spec H7 gereği")
			// Opsiyonel ayarın varsayılanı boş olabilir: "kapalı" durumu
			// bir değerle değil, değerin yokluğuyla anlatılıyor.
			if !def.Optional {
				require.NotEmpty(t, def.Default, "zorunlu ayarın varsayılanı olmalı")
			}
			require.Contains(t, GroupLabels, def.Group, "grubun insan okunur karşılığı olmalı")
		})
	}
}

func TestLookup(t *testing.T) {
	def, ok := Lookup(KeyRunTimeoutMinutes)
	require.True(t, ok)
	require.Equal(t, "30", def.Default)
	require.Equal(t, KindInt, def.Kind)

	_, ok = Lookup("uydurma.anahtar")
	require.False(t, ok)
}

func TestValidate_TamSayi(t *testing.T) {
	def, _ := Lookup(KeyMaxConcurrentRuns) // 1..20

	require.NoError(t, Validate(def, "1"))
	require.NoError(t, Validate(def, "20"))
	require.NoError(t, Validate(def, "3"))

	require.ErrorIs(t, Validate(def, "0"), ErrOutOfRange)
	require.ErrorIs(t, Validate(def, "21"), ErrOutOfRange)
	require.ErrorIs(t, Validate(def, "-5"), ErrOutOfRange)
	require.ErrorIs(t, Validate(def, "abc"), ErrInvalidValue)
	require.ErrorIs(t, Validate(def, ""), ErrInvalidValue)
	require.ErrorIs(t, Validate(def, "3.5"), ErrInvalidValue)
}

func TestValidate_AralikMesajiKullaniciyaYardimEder(t *testing.T) {
	def, _ := Lookup(KeyMaxConcurrentRuns)

	err := Validate(def, "999")
	require.ErrorIs(t, err, ErrOutOfRange)
	// Kullanıcı ne yapması gerektiğini mesajdan anlamalı.
	require.Contains(t, err.Error(), "1")
	require.Contains(t, err.Error(), "20")
}

func TestValidate_Mantiksal(t *testing.T) {
	def := Definition{Key: "x", Kind: KindBool, Default: "true"}

	for _, v := range []string{"true", "false", "1", "0", "TRUE"} {
		require.NoError(t, Validate(def, v), "değer: %q", v)
	}
	require.ErrorIs(t, Validate(def, "evet"), ErrInvalidValue)
}

func TestValidate_Metin(t *testing.T) {
	def := Definition{Key: "x", Kind: KindText, Default: "a"}

	require.NoError(t, Validate(def, "bir değer"))
	require.ErrorIs(t, Validate(def, ""), ErrInvalidValue)
}

func TestValidate_Sertifika(t *testing.T) {
	def := Definition{Key: KeyCorporateCA, Kind: KindCertificate, Default: "", Optional: true}

	// Boş = özellik kapalı, hata değil.
	require.NoError(t, Validate(def, ""))

	require.ErrorIs(t, Validate(def, "bu bir sertifika degil"), ErrInvalidValue)
	require.ErrorIs(t, Validate(def, "-----BEGIN CERTIFICATE-----\nYnV6dWs=\n-----END CERTIFICATE-----"), ErrInvalidValue)

	pemMetin, err := os.ReadFile(filepath.Join("..", "certfmt", "testdata", "kok.pem"))
	require.NoError(t, err)
	require.NoError(t, Validate(def, string(pemMetin)))
}

/*
 * PEM'in satır sonları KIRPILMAYA dayanmalı.
 *
 * Service.Set değeri TrimSpace'ten geçiriyor; kırpma blok içine dokunursa
 * sertifika sessizce bozulur ve bunu ancak bir çalıştırma düştüğünde fark
 * ederdik.
 */
func TestValidate_SertifikaBoslukKirpmayaDayanir(t *testing.T) {
	def := Definition{Key: KeyCorporateCA, Kind: KindCertificate, Default: "", Optional: true}

	pemMetin, err := os.ReadFile(filepath.Join("..", "certfmt", "testdata", "kok.pem"))
	require.NoError(t, err)

	require.NoError(t, Validate(def, strings.TrimSpace("\n\n  "+string(pemMetin)+"  \n\n")))
}

func TestValidate_TanimsizTip(t *testing.T) {
	def := Definition{Key: "x", Kind: Kind("uydurma"), Default: "a"}
	require.ErrorIs(t, Validate(def, "x"), ErrInvalidValue)
}

// TestService_OkumaVarsayilanaDuser, veritabanı olmadan da okumanın çalıştığını
// doğrular: Load çağrılmamış bir servis varsayılanları döndürmeli.
func TestService_YuklenmemisServisVarsayilanDoner(t *testing.T) {
	s := &Service{override: map[string]string{}}

	require.Equal(t, 30, s.Int(KeyRunTimeoutMinutes))
	require.Equal(t, 3, s.Int(KeyMaxConcurrentRuns))
	require.Equal(t, 24, s.Int(KeyCatalogSyncHours))
	require.Equal(t, 0, s.Int("uydurma.anahtar"), "bilinmeyen anahtar sıfır döner, panik etmez")
}

func TestService_SapmaOkunur(t *testing.T) {
	s := &Service{override: map[string]string{KeyMaxConcurrentRuns: "7"}}

	require.Equal(t, 7, s.Int(KeyMaxConcurrentRuns))
	require.Equal(t, 30, s.Int(KeyRunTimeoutMinutes), "diğerleri varsayılanda kalmalı")
}

func TestService_BozukDegerVarsayilanaDuser(t *testing.T) {
	// Veritabanına elle bozuk bir değer yazılmış olabilir. Sistem durmamalı.
	s := &Service{override: map[string]string{KeyMaxConcurrentRuns: "bu-sayı-değil"}}
	require.Equal(t, 3, s.Int(KeyMaxConcurrentRuns))
}

func TestService_All(t *testing.T) {
	s := &Service{override: map[string]string{KeyMaxConcurrentRuns: "7"}}

	all := s.All()
	require.Len(t, all, len(Registry))

	byKey := map[string]Value{}
	for _, v := range all {
		byKey[v.Key] = v
	}

	require.Equal(t, "7", byKey[KeyMaxConcurrentRuns].Value)
	require.True(t, byKey[KeyMaxConcurrentRuns].IsCustom, "sapan ayar işaretlenmeli")

	require.Equal(t, "30", byKey[KeyRunTimeoutMinutes].Value)
	require.False(t, byKey[KeyRunTimeoutMinutes].IsCustom)

	// Tanım bilgisi arayüze taşınmalı — ekran kendini bundan çiziyor.
	require.NotEmpty(t, byKey[KeyRunTimeoutMinutes].Label)
	require.NotEmpty(t, byKey[KeyRunTimeoutMinutes].Help)
	require.Equal(t, "dakika", byKey[KeyRunTimeoutMinutes].Unit)
}

func TestService_SiralamaKayitDefteriyleAyni(t *testing.T) {
	s := &Service{override: map[string]string{}}

	all := s.All()
	for i, def := range Registry {
		require.Equal(t, def.Key, all[i].Key, "arayüz sırası kayıt defteriyle aynı olmalı")
	}
}

// TestValidate_OpsiyonelMetinBosOlabilir — "kapalı" bir yapılandırma hata değil.
func TestValidate_OpsiyonelMetinBosOlabilir(t *testing.T) {
	opsiyonel, ok := Lookup(KeyNPMRegistry)
	require.True(t, ok)
	require.NoError(t, Validate(opsiyonel, ""), "opsiyonel ayar boş bırakılabilmeli")

	zorunlu, ok := Lookup(KeyReportTimezone)
	require.True(t, ok)
	require.Error(t, Validate(zorunlu, ""), "zorunlu metin ayarı hâlâ boş kabul etmemeli")
}

/*
 * TestValidate_KayitDefteriAdresi — kimlik bilgisi adrese GÖMÜLEMEZ.
 *
 * `putSetting` ayarın değerini log'a yazıyor; `https://kullanici:token@…`
 * biçiminde bir adres token'ı düz metin olarak loga ve veritabanına
 * düşürürdü. Aynı kural projelerin depo adresinde de var.
 */
func TestValidate_KayitDefteriAdresi(t *testing.T) {
	def, ok := Lookup(KeyNPMRegistry)
	require.True(t, ok)

	require.NoError(t, Validate(def, "https://nexus.sirket.local/repository/npm-group/"))
	require.NoError(t, Validate(def, "http://nexus.local:8081/repository/npm/"))

	require.Error(t, Validate(def, "nexus.sirket.local"), "şema zorunlu")
	require.Error(t, Validate(def, "ftp://nexus.sirket.local"), "yalnızca http(s)")
	require.Error(t, Validate(def, "https://"), "sunucu adı zorunlu")

	err := Validate(def, "https://kullanici:gizlitoken@nexus.sirket.local/repository/npm/")
	require.Error(t, err, "kimlik gömülü adres reddedilmeli")
	require.NotContains(t, err.Error(), "gizlitoken", "hata mesajı sırrı tekrarlamamalı")
}

/*
 * Proxy adresi doğrulaması — kayıt defteri adresiyle aynı gerekçe.
 *
 * `putSetting` değeri loga yazıyor; `http://kullanici:parola@proxy:8080`
 * biçiminde bir adres parolayı düz metin olarak loga ve veritabanına
 * düşürürdü. Spec 020 bu yüzden kimlik doğrulaması isteyen proxy'leri
 * kapsam dışı bıraktı.
 */
func TestValidate_ProxyAdresi(t *testing.T) {
	def, ok := Lookup(KeyEgressProxy)
	require.True(t, ok)

	require.NoError(t, Validate(def, "http://proxy.sirket.local:8080"))
	require.NoError(t, Validate(def, ""), "boş bırakmak denetimi kapatır")

	require.Error(t, Validate(def, "proxy.sirket.local:8080"), "şema zorunlu")
	require.Error(t, Validate(def, "ftp://proxy.sirket.local"), "yalnızca http(s)")
	require.Error(t, Validate(def, "http://"), "sunucu adı zorunlu")

	err := Validate(def, "http://kullanici:gizliparola@proxy.sirket.local:8080")
	require.Error(t, err, "kimlik gömülü adres reddedilmeli")
	require.NotContains(t, err.Error(), "gizliparola", "hata mesajı sırrı tekrarlamamalı")
}

// Whitelist doğrulaması hostlist paketine devrediliyor: ayarda kabul edilen bir
// satırın kapıda tutmaması, kullanıcının listede sandığı ama olmayan bir izin
// demektir.
func TestValidate_IzinliHostlar(t *testing.T) {
	def, ok := Lookup(KeyAllowedHosts)
	require.True(t, ok)

	require.NoError(t, Validate(def, "ornek.com\n*.npmjs.org\n# yorum\n"))
	require.NoError(t, Validate(def, ""), "boş liste kısıt uygulanmadığı anlamına gelir")

	err := Validate(def, "ornek.com\nhttps://bozuk.com")
	require.Error(t, err, "URL yazılan satır reddedilmeli")
	require.Contains(t, err.Error(), "2", "hata kaçıncı satır olduğunu söylemeli")
}
