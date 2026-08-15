package runs

import "strings"

/*
 * stripUnappliableBinary, yamadan UYGULANAMAZ ikili blokları ayıklar ve
 * atlanan dosyaların adlarını döner.
 *
 * NEDEN VAR — gerçek bir hatanın kaydı: `mvn` çalıştıran bir koşuda `target/`
 * altındaki JAR'lar değişiklik sayıldı (deponun `.gitignore`'u yoktu).
 * opencode'un ürettiği diff bu dosyalar için yalnızca
 *
 *     index 0000000..091cb04
 *     Binary files /dev/null and b/….jar differ
 *
 * yazıyor: kısaltılmış index ve HİÇ YÜK YOK. Bu, `git diff --binary` çıktısı
 * değil; `git apply` onu uygulayamıyor ve
 *
 *     cannot apply binary patch … without full index line
 *
 * ile düşüyordu. Tek blok TÜM yamayı iptal ediyordu: dokuz dosyanın yedisi
 * düzgün metin olduğu hâlde hiçbiri gönderilemiyordu.
 *
 * ATMAK VERİ KAYBI DEĞİL: o blokta uygulanacak bir şey zaten yok. Ama sessiz
 * de kalınmıyor — atlananların adı çağırana dönüyor ve kullanıcıya yazılıyor.
 *
 * YÜK TAŞIYAN İKİLİ KORUNUR: `GIT binary patch` bölümü olan bir blok
 * uygulanabilir ve ona dokunulmuyor.
 */
func stripUnappliableBinary(diff string) (string, []string) {
	if diff == "" {
		return "", nil
	}

	var (
		korunan  []string
		atlanan  []string
		blok     []string
		ikili    bool
		yukVar   bool
		ikiliAdi string
	)

	bitir := func() {
		if len(blok) == 0 {
			return
		}
		if ikili && !yukVar {
			atlanan = append(atlanan, ikiliAdi)
		} else {
			korunan = append(korunan, blok...)
		}
		blok, ikili, yukVar, ikiliAdi = nil, false, false, ""
	}

	for _, satir := range strings.Split(diff, "\n") {
		if strings.HasPrefix(satir, "diff --git ") {
			bitir()
		}
		blok = append(blok, satir)

		switch {
		case strings.HasPrefix(satir, "Binary files ") && strings.HasSuffix(satir, " differ"):
			ikili = true
			ikiliAdi = ikiliDosyaAdi(satir)
		case strings.HasPrefix(satir, "GIT binary patch"):
			yukVar = true
		}
	}
	bitir()

	return strings.Join(korunan, "\n"), atlanan
}

/*
 * ikiliDosyaAdi, "Binary files X and b/YOL differ" satırından yolu çıkarır.
 *
 * `diff --git a/… b/…` satırı AYRIŞTIRILMIYOR: yol boşluk içerdiğinde iki
 * tarafın nerede bittiği belirsiz oluyor ve gerçek vakada tam olarak öyleydi
 * ("Jwt Authentication with Spring Boot 3.1/target/…"). Bu satırda ise hedef
 * her zaman sonda ve " differ" ile bitiyor.
 */
func ikiliDosyaAdi(satir string) string {
	s := strings.TrimSuffix(strings.TrimPrefix(satir, "Binary files "), " differ")

	// Ayraç " and " DEĞİL " and b/": hedef tarafın öneki ayracın parçası
	// sayılmazsa dizi yolun İÇİNDE de eşleşiyor ve yol kırpılıyor —
	// "a/Search and Replace/app.jar and b/Search and Replace/app.jar"
	// satırından geriye "Replace/app.jar" kalıyordu.
	if i := strings.LastIndex(s, " and b/"); i >= 0 {
		return s[i+len(" and b/"):]
	}

	// Hedefte "b/" öneki yoksa (silinen dosyada git "/dev/null" yazar) eski
	// davranış: son " and " ayracı.
	if i := strings.LastIndex(s, " and "); i >= 0 {
		s = s[i+len(" and "):]
	}
	return strings.TrimPrefix(s, "b/")
}
