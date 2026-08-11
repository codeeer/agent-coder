#!/usr/bin/env bash
# Runner imajında düzeltme var mı — hatanın yaşandığı makinede çalıştırın.
set -uo pipefail
BE=$(docker ps --filter "name=backend" --format '{{.Names}}' | head -1)
[ -z "$BE" ] && { echo "backend container bulunamadı"; exit 1; }

IMAJ=$(docker inspect "$BE" --format '{{range .Config.Env}}{{println .}}{{end}}' | sed -n 's/^RUNNER_IMAGE=//p')
echo "backend'in kullandığı runner imajı: ${IMAJ:-(tanımsız)}"

docker image inspect "$IMAJ" >/dev/null 2>&1 || { echo "→ bu imaj YEREL olarak yok"; exit 1; }

REV=$(docker inspect "$IMAJ" --format '{{json .Config.Labels}}' \
  | python3 -c "import json,sys;print(json.load(sys.stdin).get('org.opencontainers.image.revision','(yerel derleme)')[:7])" 2>/dev/null)
echo "imajın commit'i: $REV"

if docker run --rm --entrypoint bash "$IMAJ" -c 'grep -q GIT_TERMINAL_PROMPT=0 /usr/local/bin/entrypoint.sh' 2>/dev/null; then
  echo "SONUÇ: düzeltme İMAJDA VAR ✓  (hata devam ediyorsa başka bir neden var)"
else
  echo "SONUÇ: düzeltme İMAJDA YOK ✗  → imajı tazeleyin"
  echo "   ghcr kullanıyorsanız : docker pull $IMAJ && make restart"
  echo "   yerel derliyorsanız  : make runner && make restart"
fi
