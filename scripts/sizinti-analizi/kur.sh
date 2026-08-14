#!/usr/bin/env bash
#
# Ölçüm düzeneğini ayağa kaldırır ve .env'e eklenecek satırları basar.
#
# Sıra önemli: mitmproxy ilk açılışta kendi CA'sını üretiyor, backend ise o
# sertifikayı AÇILIŞTA okuyor. Sertifika hazır olmadan backend yeniden
# başlatılırsa runner TLS hatalarıyla düşer.
set -euo pipefail

kok="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cikti="$kok/cikti"

mkdir -p "$cikti/certs"
# mitmproxy imajı uid 1000 ile koşuyor; yazma izni olmadan CA üretemez.
chmod 777 "$cikti" "$cikti/certs"

"$kok/kanarya-uret.sh"
"$kok/depo-uret.sh"

# Depo sunucusu basic auth parolasını compose değişkeni olarak alıyor.
set -a
# shellcheck source=/dev/null
source "$cikti/kanaryalar.env"
set +a

docker compose -f "$kok/docker-compose.yml" up -d

echo "mitmproxy CA'sı bekleniyor…"
ca="$cikti/certs/mitmproxy-ca-cert.pem"
for _ in $(seq 1 30); do
    [[ -s "$ca" ]] && break
    sleep 1
done
if [[ ! -s "$ca" ]]; then
    echo "HATA: CA üretilmedi. 'docker logs sizinti-mitm' bakın." >&2
    exit 1
fi

# Ölçülen sürüm rapora yazılacak — varsayılmayacak.
docker exec sizinti-mitm mitmdump --version 2>/dev/null | head -3 \
    > "$cikti/mitmproxy-surum.txt" || true

cat <<EOF

Düzenek hazır.

.env dosyasına şu iki satırı ekleyin, sonra backend'i yeniden başlatın:

  RUNNER_EXTRA_CA_CERT=$ca
  RUNNER_HTTP_PROXY=http://sizinti-mitm:8080

Yeniden başlatma:  make down && make up

Sertifika: $ca
Akışlar:   $cikti/akislar.mitm
EOF
