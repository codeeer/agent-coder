#!/usr/bin/env bash
#
# İki katmanı da çözümler ve raporun ham sayılarını üretir.
#
# Katman 1 (vekil): ne gönderildi.
# Katman 2 (pcap):  vekile uğramadan çıkan var mı.
#
# İkincisi olmadan birincisinin sonucu yorumlanamaz: vekilde hiçbir şey
# görünmemesi, "hiçbir şey göndermedi" DEĞİL "vekilden geçmedi" anlamına da
# gelebilir.
set -euo pipefail

kok="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cikti="$kok/cikti"
etiket="${1:-yakalama}"

cp "$kok/ihrac.py" "$cikti/ihrac.py"

echo "── Katman 1: vekil dökümü ──────────────────────────────────────────"
adet="$(docker exec sizinti-mitm python3 /cikti/ihrac.py \
    "/cikti/${etiket}.mitm" "/cikti/${etiket}.jsonl" | tail -1)"
echo "akış sayısı: $adet"
echo

python3 "$kok/analiz.py" "$cikti/${etiket}.jsonl" "$cikti/kanaryalar.env" \
    | tee "$cikti/${etiket}-cozumleme.md"

echo
echo "── Katman 2: köprü paketleri (bypass kontrolü) ─────────────────────"
pcap="$cikti/${etiket}.pcap"
if [[ ! -s "$pcap" ]]; then
    echo "pcap yok: $pcap" >&2
    exit 1
fi

# TLS ClientHello'daki SNI, vekile uğramamış bağlantıların tek adı.
docker run --rm -v "$cikti:/cikti" nicolaka/netshoot:latest \
    tshark -r "/cikti/${etiket}.pcap" \
        -Y 'tls.handshake.type == 1' \
        -T fields -e ip.src -e ip.dst -e tls.handshake.extensions_server_name \
    2>/dev/null | sort | uniq -c | sort -rn \
    | tee "$cikti/${etiket}-sni.txt"

echo
echo "── TCP bağlantı kurulumları (kaynak → hedef:port) ──────────────────"
docker run --rm -v "$cikti:/cikti" nicolaka/netshoot:latest \
    tshark -r "/cikti/${etiket}.pcap" \
        -Y 'tcp.flags.syn == 1 && tcp.flags.ack == 0' \
        -T fields -e ip.src -e ip.dst -e tcp.dstport \
    2>/dev/null | sort | uniq -c | sort -rn \
    | tee "$cikti/${etiket}-baglantilar.txt"

echo
echo "Runner IP'leri (izle.sh kaydı):"
cat "$cikti/${etiket}-runner-ip.log" 2>/dev/null || echo "  kayıt yok"
