#!/usr/bin/env bash
#
# Bir ölçüm koşusu için temiz yakalama açar.
#
# BU BETİK BİR HATANIN KAYDIDIR. Önce elle yapılıyordu: mitmproxy durdurulur,
# akış dosyası taşınır, yeniden başlatılır. İki şey birden ters gitti:
#
#   1. `docker compose stop` sessizce BAŞARISIZ oldu — compose, hedef servis
#      başka olsa bile dosyanın TAMAMINI doğruluyor ve `depo` servisi zorunlu
#      bir değişken (`KANARYA_GIT_TOKEN`) istiyordu. Değişken yüklenmemişti.
#   2. mitmproxy durmadığı için açık dosya tanıtıcısını korudu ve TAŞINMIŞ
#      dosyaya yazmayı sürdürdü. Yeni koşunun trafiği "önceki koşu" adlı
#      dosyanın içine düştü.
#
# Sonuç: yakalama çalışıyor görünürken beklenen dosya hiç oluşmamıştı.
# Buradaki iki koruma bunu imkânsız kılıyor: değişkenler HER ZAMAN yükleniyor
# ve akış dosyasının varlığı koşu başlamadan ÖNCE doğrulanıyor.
set -euo pipefail

kok="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cikti="$kok/cikti"
etiket="${1:?kullanım: $0 <koşu-etiketi>}"

set -a
# shellcheck source=/dev/null
source "$cikti/kanaryalar.env"
set +a
export KOSU_ETIKETI="$etiket"

"$kok/yakala.sh" durdur >/dev/null 2>&1 || true
rm -f "$cikti/${etiket}.mitm" "$cikti/${etiket}.pcap" "$cikti/${etiket}-runner-ip.log"

docker compose -f "$kok/docker-compose.yml" up -d --force-recreate mitm

# DOĞRULAMA — bu adım olmadan sessiz başarısızlık tekrar mümkün olur.
echo "akış dosyası bekleniyor: cikti/${etiket}.mitm"
for _ in $(seq 1 30); do
    [[ -f "$cikti/${etiket}.mitm" ]] && break
    sleep 1
done
if [[ ! -f "$cikti/${etiket}.mitm" ]]; then
    echo "HATA: mitmproxy akış dosyasını açmadı — koşu BAŞLATILMAMALI." >&2
    docker logs sizinti-mitm 2>&1 | tail -20 >&2
    exit 1
fi

"$kok/yakala.sh" başlat "$etiket"

# Runner IP'lerini kaydeden izleyici — bypass analizinde atıf için şart.
pkill -f "izle.sh" 2>/dev/null || true
nohup "$kok/izle.sh" "${etiket}-runner-ip" > "$cikti/${etiket}-izle.out" 2>&1 &

echo "hazır — koşuyu şimdi başlatabilirsiniz (etiket: $etiket)"
