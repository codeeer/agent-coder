#!/usr/bin/env bash
#
# Kurumsal kök sertifikayı Java'nın güven listesine tanıtır.
#
# NEDEN AYRI BİR ADIM: Java, diğer araçlar gibi bir "ek sertifika" ortam
# değişkeni okumuyor. Node `NODE_EXTRA_CA_CERTS`, git `GIT_SSL_CAINFO`, curl
# `CURL_CA_BUNDLE` ile hallolurken JDK yalnızca kendi keystore dosyasına
# bakıyor. O dosya imajın içinde ve root'a ait; sertifika ise koşu anında,
# container'a kopyalanmış bir dosya olarak geliyor (spec 017).
#
# Çözüm: JDK'nın güven listesinin bir KOPYASI agent'ın ev dizininde üretilir ve
# kurumsal sertifika ona eklenir. Ölçüldü — kopyalama ve `keytool` root
# gerektirmiyor.
#
# Kaynak dosya bu betiği source eden entrypoint tarafından kullanılır; testi de
# aynı dosyayı source ediyor ki mantık iki yerde kopyalanmasın.

readonly TRUSTSTORE_YOL="${HOME:-/home/agent}/kurumsal-truststore.jks"

# Java keystore'larının değişmez varsayılan parolası. Sır değil: dosya zaten
# yalnızca genel sertifika taşıyor ve parola JDK'nın kendi `cacerts`'inde de
# budur.
readonly TRUSTSTORE_PAROLA=changeit

# java_truststore_kur, sertifika varsa truststore üretir ve MAVEN_OPTS'u kurar.
#
# Sertifika yoksa HİÇBİR ŞEY YAPMAZ ve MAVEN_OPTS tanımlanmaz: kurumsal
# olmayan kurulumların davranışı birebir aynı kalmalı.
#
# $1 — sertifika dosyasının yolu (boş veya yoksa atlanır)
java_truststore_kur() {
    local sertifika="${1:-}"

    [[ -n "$sertifika" && -f "$sertifika" ]] || return 0
    command -v keytool >/dev/null 2>&1 || return 0

    local kaynak="${JAVA_HOME:-/opt/java/25}/lib/security/cacerts"
    if [[ ! -f "$kaynak" ]]; then
        echo "[runner] uyarı: JDK güven listesi bulunamadı ($kaynak)" >&2
        return 0
    fi

    # VARSAYILAN JDK'nın listesi temel alınır: kurumsal sertifika güvenilen
    # köklere EKLENİR, yerine geçmez. Genel sertifikalar geçerli kalır.
    # `cp` KAYNAĞIN İZİNLERİNİ KOPYALAR ve JDK'nın `cacerts` dosyası salt
    # okunur (`-r--r--r--`). Bu bir hatanın kaydı: kopya salt okunur kalınca
    # `keytool` sertifikayı belleğe ekliyor ama dosyaya yazamıyordu ve
    # "Certificate was added to keystore" satırının HEMEN ARDINDAN
    # "Permission denied" veriyordu — yanıltıcı bir çift mesaj.
    if ! cp "$kaynak" "$TRUSTSTORE_YOL"; then
        echo "[runner] uyarı: güven listesi kopyalanamadı" >&2
        return 0
    fi
    if ! chmod u+w "$TRUSTSTORE_YOL"; then
        echo "[runner] uyarı: güven listesi yazılabilir yapılamadı" >&2
        rm -f "$TRUSTSTORE_YOL"
        return 0
    fi

    if ! keytool -importcert -noprompt \
            -alias kurumsal \
            -file "$sertifika" \
            -keystore "$TRUSTSTORE_YOL" \
            -storepass "$TRUSTSTORE_PAROLA" >/dev/null 2>&1; then
        echo "[runner] uyarı: kurumsal sertifika Java güven listesine eklenemedi" >&2
        rm -f "$TRUSTSTORE_YOL"
        return 0
    fi

    # MAVEN_OPTS, JAVA_TOOL_OPTIONS DEĞİL.
    #
    # `JAVA_TOOL_OPTIONS` her JVM'i kapsardı ama her JVM başlangıcında stderr'e
    # "Picked up JAVA_TOOL_OPTIONS…" satırı basıyor ve o satır agent'ın okuduğu
    # HER araç çıktısına düşerdi. Spec 014'te npm'in `always-auth` uyarısı tam
    # olarak bu sebeple kaldırılmıştı.
    #
    # Bedeli: `mvn` ve `mvnw` kapsanır, doğrudan `java -jar` kapsanmaz.
    export MAVEN_OPTS="${MAVEN_OPTS:+$MAVEN_OPTS }-Djavax.net.ssl.trustStore=${TRUSTSTORE_YOL} -Djavax.net.ssl.trustStorePassword=${TRUSTSTORE_PAROLA}"
    echo "[runner] kurumsal sertifika Java güven listesine eklendi" >&2
}
