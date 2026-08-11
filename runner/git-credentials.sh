#!/usr/bin/env bash
#
# Git kimlik bilgisi kurulumu.
#
# entrypoint.sh tarafından source edilir; ayrıca git-credentials-test.sh
# doğrudan bu dosyayı source edip sınar. Mantık entrypoint'in içinde kalsaydı
# test onu kopyalamak zorunda kalır, kopya da er geç ayrışırdı.

# git_host_cikar, https adresinden host'u (varsa port'uyla) çıkarır.
#
# Eşleşme olmazsa girdiyi olduğu gibi döndürür; çağıran bunu "https değil"
# kontrolü olarak kullanır.
git_host_cikar() {
    sed -E 's#^https?://([^/]+)/.*#\1#' <<<"$1"
}

# git_protokol_cikar, adresin şemasını (http|https) döndürür.
#
# SABİT `https` YAZILAMAZ: host çıkarımı `https?` kabul ediyor, yani `http://`
# ile başlayan bir adres de buraya kadar geliyor. Kimlik bilgisi `protocol=https`
# olarak kaydedilirse git, `http://` klonlamasında `protocol=http` arar, hiçbir
# kayıt bulamaz ve kimlik bilgisi HİÇ GÖNDERİLMEZ — sunucuda Authorization
# başlığının hiç görünmediği ölçüldü. Kurum içi sunucular TLS'siz olabiliyor.
git_protokol_cikar() {
    sed -E 's#^(https?)://.*#\1#' <<<"$1"
}

# git_kimlik_kur, kimlik bilgisini git'in kendi credential store'una yazar.
#
# NEDEN `git credential approve` — ELLE YAZILMIYOR:
#
# Önceki hal `.git-credentials` dosyasına URL biçiminde elle satır yazıyordu:
#     printf 'https://%s:%s@%s\n' "$user" "$token" "$host"
#
# Kullanıcı adı veya token URL'de anlamı olan bir karakter içerdiği anda bu
# satır bozuluyordu — ve kurumsal kurulumlarda bu istisna değil kural:
# kullanıcı adı e-posta ise `@`, DOMAIN\kullanici ise `\`, token'da `:` `/` `%`.
#
# Ölçüldü: user="omer.x@example.com", token="tok:en/123%ab" için üretilen satır
#     https:...@example.com:tok:en/123%ab@bitbucket.sirket.local
# oluyor. git bunu o host için bir kayıt olarak EŞLEŞTİREMİYOR, sessizce yok
# sayıyor ve kullanıcı adı sormaya çalışıyor. Terminal olmadığı için de şu
# hatayla ölüyor:
#     fatal: could not read Username for '...': No such device or address
#
# Yani arıza yalnızca "temiz" kullanıcı adı ve token'larda görünmüyordu.
#
# `git credential approve` protokolü key=value satırlarından oluşur; değerlerin
# içindeki özel karakterlerin ayrıştırmada hiçbir anlamı yoktur. Store dosyasına
# yazarken percent-encoding'i git'in kendisi yapar — doğru kaçırma kuralını
# bilen taraf odur.
git_kimlik_kur() {
    local host="$1" user="$2" token="$3" protokol="${4:-https}"

    # Dosya yalnızca bu container'da yaşar ve iş bitince container'la silinir;
    # yine de başkasına okutulmaz.
    umask 077

    git config --global credential.helper store
    printf 'protocol=%s\nhost=%s\nusername=%s\npassword=%s\n\n' \
        "$protokol" "$host" "$user" "$token" | git credential approve
}
