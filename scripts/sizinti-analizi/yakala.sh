#!/usr/bin/env bash
#
# İKİNCİ ÖLÇÜM KATMANI — köprü üzerinde ham paket yakalama.
#
# NEDEN GEREKLİ: vekil trafiği YÖNLENDİRİR, MECBUR ETMEZ. Vekil ortam
# değişkenlerini yok sayan bir istemci (ya da sertifika sabitleyen bir kod
# yolu) doğrudan dışarı çıkar ve mitmproxy dökümünde HİÇ GÖRÜNMEZ. Yalnız
# vekile bakılsaydı böyle bir kaçış "sızıntı yok" gibi okunurdu.
#
# Docker ağının köprü arayüzü, Docker Desktop'ın Linux VM'inde duruyor;
# `--net=host` ile açılan ayrıcalıklı bir container o isim uzayına giriyor.
# Runner'ın netns'ine bağlanmaya göre üstünlüğü: container doğmadan ÖNCE
# başlatılabiliyor, yani ilk paketler kaçmıyor.
#
# GÖREMEDİĞİ ŞEY: container içi DNS (127.0.0.11) — o adres netns'e özel.
# Ad çözümlemesi yerine TLS SNI ve hedef IP ölçülüyor; bypass sorusu için
# ikisi de yeterli.
set -euo pipefail

kok="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cikti="$kok/cikti"
ag="${RUNNER_NETWORK:-agent-coder_internal}"
ad="sizinti-tcpdump"

komut="${1:-}"

case "$komut" in
başlat|baslat|start)
    etiket="${2:-yakalama}"
    mkdir -p "$cikti"
    chmod 777 "$cikti"

    kimlik="$(docker network inspect "$ag" --format '{{.Id}}')"
    kopru="br-${kimlik:0:12}"

    docker rm -f "$ad" >/dev/null 2>&1 || true
    docker run -d --name "$ad" --net=host --privileged \
        -v "$cikti:/cikti" \
        nicolaka/netshoot:latest \
        tcpdump -i "$kopru" -n -s 0 -U -w "/cikti/${etiket}.pcap" \
        >/dev/null

    sleep 2
    if ! docker ps --format '{{.Names}}' | grep -qx "$ad"; then
        echo "HATA: yakalama başlamadı:" >&2
        docker logs "$ad" >&2 || true
        exit 1
    fi
    echo "yakalama başladı — arayüz $kopru, dosya cikti/${etiket}.pcap"
    ;;

durdur|stop)
    docker stop "$ad" >/dev/null 2>&1 || true
    docker rm "$ad" >/dev/null 2>&1 || true
    echo "yakalama durdu"
    ls -la "$cikti"/*.pcap 2>/dev/null || true
    ;;

*)
    echo "kullanım: $0 {başlat <etiket>|durdur}" >&2
    exit 1
    ;;
esac
