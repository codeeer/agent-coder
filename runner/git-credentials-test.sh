#!/usr/bin/env bash
#
# git-credentials.sh testleri.
#
# NEDEN VAR: kimlik bilgisi kurulumu yalnızca "temiz" kullanıcı adı ve
# token'larda çalışıyordu. Kurumsal kurulumda kullanıcı adı e-posta (`@`) veya
# DOMAIN\kullanici (`\`), token ise `:` `/` `%` içerebiliyor — ve o anda elle
# yazılan URL biçimi bozuluyor, git kaydı eşleştiremiyor, klonlama
# "could not read Username" ile ölüyordu.
#
# Hata gerçek bir remote GEREKTİRMEDEN yakalanabilir: `git credential fill`
# store'dan okuduğunu aynen döndürür. Test tam olarak bunu sınar.
#
# Çalıştırma:  make test-runner-credentials
set -euo pipefail

KAYNAK="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/git-credentials.sh"
# shellcheck source=runner/git-credentials.sh
source "$KAYNAK"

gecen=0
kalan=0

# dogrula, verilen kullanıcı adı/token çiftini store'a yazar ve git'in aynen
# geri verdiğini sınar. Her durum İZOLE bir HOME kullanır: bir öncekinin kaydı
# sonrakini gölgelemesin.
dogrula() {
    local ad="$1" user="$2" token="$3" protokol="${4:-https}"
    local host="bitbucket.sirket.local:7990"

    HOME="$(mktemp -d)"
    export HOME

    git_kimlik_kur "$host" "$user" "$token" "$protokol"

    # `|| true` ZORUNLU: eşleşme olmadığında `git credential fill` sıfırdan
    # farklı çıkıyor ve `set -e` betiği İLK başarısız durumda öldürüyordu — geri
    # kalan durumlar hiç raporlanmıyordu. Kaç durumun kaldığını görmek, hangi
    # karakterin bozduğunu anlamanın tek yolu.
    local cikti
    cikti="$(printf 'protocol=%s\nhost=%s\n\n' "$protokol" "$host" | git credential fill 2>&1 || true)"

    # `|| true` burada da ZORUNLU: eşleşme yoksa `grep` sıfırdan farklı çıkar ve
    # `set -e` betiği yine öldürürdü. Başarısız durumda okunan değer BOŞ kalmalı,
    # test o boşluğu rapor edebilmeli.
    local okunan_user okunan_token
    okunan_user="$(grep '^username=' <<<"$cikti" | cut -d= -f2- || true)"
    okunan_token="$(grep '^password=' <<<"$cikti" | cut -d= -f2- || true)"

    if [[ "$okunan_user" == "$user" && "$okunan_token" == "$token" ]]; then
        printf '  ✔ %s\n' "$ad"
        gecen=$((gecen + 1))
    else
        printf '  ✘ %s\n' "$ad"
        # Token ekrana BASILMAZ; yalnızca eşleşip eşleşmediği ve uzunluğu.
        printf '      kullanıcı adı: beklenen %q, gelen %q\n' "$user" "$okunan_user"
        printf '      token        : %s (beklenen %d karakter, gelen %d)\n' \
            "$([[ "$okunan_token" == "$token" ]] && echo eşleşti || echo EŞLEŞMEDİ)" \
            "${#token}" "${#okunan_token}"
        kalan=$((kalan + 1))
    fi

    rm -rf "$HOME"
}

echo "Kullanıcı adı biçimleri:"
dogrula "sade"                "omer"                "basit-token-123"
dogrula "e-posta (@)"         "omer.x@example.com"  "basit-token-123"
dogrula "alan adı (\\)"        'DOMAIN\omer'         "basit-token-123"
dogrula "GitHub varsayılanı"  "x-access-token"      "basit-token-123"

echo
echo "Token biçimleri:"
dogrula "alfanumerik"         "omer"  "abcXYZ0123456789"
dogrula "iki nokta (:)"       "omer"  "tok:en123"
dogrula "eğik çizgi (/)"      "omer"  "tok/en/123"
dogrula "kuyruklu a (@)"      "omer"  "tok@en123"
dogrula "yüzde (%)"           "omer"  "tok%en123"
dogrula "hepsi bir arada"     'DOMAIN\omer'  'tok:en/123%ab@son'

echo
echo "Protokol:"
# `protocol=https` SABİT yazılıydı; http adreslerinde git hiçbir kayıt bulamıyor
# ve kimlik bilgisini HİÇ göndermiyordu. Kurum içi sunucular TLS'siz olabiliyor.
dogrula "https"               "omer"  "tok/en123"  "https"
dogrula "http (TLS'siz kurum)" "omer"  "tok/en123"  "http"

echo
echo "Ölçüm: $((gecen + kalan)) durum, $gecen geçti, $kalan kaldı"
[[ $kalan -eq 0 ]] || exit 1
