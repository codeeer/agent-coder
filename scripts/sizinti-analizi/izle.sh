#!/usr/bin/env bash
#
# Runner container'larının IP adreslerini kaydeder.
#
# NEDEN GEREKLİ: köprü yakalaması ağdaki HERKESİ görüyor — backend, postgres,
# frontend ve runner. "Bu bağlantıyı runner mı kurdu" sorusu ancak runner'ın o
# andaki IP'si bilinirse cevaplanabilir. Container geçici olduğu için IP koşu
# bittikten sonra sorulamaz; koşu SIRASINDA yakalanmak zorunda.
set -euo pipefail

kok="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cikti="$kok/cikti"
ag="${RUNNER_NETWORK:-agent-coder_internal}"
kayit="$cikti/${1:-runner-ip}.log"

mkdir -p "$cikti"
echo "izleniyor → $kayit  (durdurmak için Ctrl-C)"

gorulen=""
while true; do
    for ad in $(docker ps --format '{{.Names}}' --filter "network=$ag"); do
        case "$gorulen" in *"|$ad|"*) continue ;; esac
        ip="$(docker inspect "$ad" \
            --format "{{(index .NetworkSettings.Networks \"$ag\").IPAddress}}" 2>/dev/null || true)"
        imaj="$(docker inspect "$ad" --format '{{.Config.Image}}' 2>/dev/null || true)"
        [[ -z "$ip" ]] && continue
        printf '%s\t%s\t%s\t%s\n' "$(date -u +%FT%TZ)" "$ad" "$ip" "$imaj" | tee -a "$kayit"
        gorulen="${gorulen}|$ad|"
    done
    sleep 0.5
done
