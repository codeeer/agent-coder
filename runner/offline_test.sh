#!/usr/bin/env bash
# Kapalı ağ testi — sağlayıcı sürücüsü imajdan geliyor mu?
#
# NEDEN VAR: sürücüler bir zamanlar KOŞU ANINDA paket deposundan iniyordu.
# SSL denetimi yapan kurumsal ağlarda bu istek düşüyor, air-gapped ortamda
# ise hiç mümkün değil (spec 003, 2026-08-12 kararı). Bu betik, koşu
# sırasında paket deposuna hiç çıkılmadığını KANITLAR.
#
# Yöntem: `internal: true` bir Docker ağı — DNS bile çözülmez. İçinde sahte
# bir OpenAI-uyumlu sunucu koşar. İsteğin o sunucuya ULAŞMASI, sürücünün
# yüklenebildiği anlamına gelir; ulaşmazsa sürücü yok demektir.
#
# KAPSAM: bu test MOTORUN davranışını ölçer, agent'ınkini değil. Agent
# kullanıcının deposunda `npm install` çalıştırabilir ve bir paket deposu
# tanımlıysa oraya çıkar — bu bilinçli. Motorun çevrimdışı kalması ise
# `~/.config/opencode/.npmrc` ile o dizine kapsanmıştır.
#
# Kullanım: runner/offline_test.sh [imaj]
set -euo pipefail

IMAJ="${1:-agent-coder/opencode-runner:latest}"
AG="agent-coder-offline-test"
DIZIN="$(mktemp -d)"
trap 'docker rm -f oc-sahte-llm oc-runner-test >/dev/null 2>&1 || true
      docker network rm "$AG" >/dev/null 2>&1 || true
      rm -rf "$DIZIN"' EXIT

log() { printf '[offline-test] %s\n' "$*"; }

cat > "$DIZIN/sahte.py" <<'PY'
import json, http.server
class H(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", 0)))
        print("ISTEK-GELDI", flush=True)
        b = json.dumps({"id":"c1","object":"chat.completion","created":0,
            "model":"test-model","choices":[{"index":0,"finish_reason":"stop",
            "message":{"role":"assistant","content":"TAMAM"}}],
            "usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}).encode()
        self.send_response(200); self.send_header("Content-Type","application/json")
        self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def do_GET(self):
        b = json.dumps({"object":"list","data":[{"id":"test-model","object":"model"}]}).encode()
        self.send_response(200); self.send_header("Content-Type","application/json")
        self.send_header("Content-Length", str(len(b))); self.end_headers(); self.wfile.write(b)
    def log_message(self, *a): pass
http.server.HTTPServer(("0.0.0.0", 8099), H).serve_forever()
PY

cat > "$DIZIN/opencode.json" <<'JSON'
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "sirket": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "sirket",
      "options": { "apiKey": "sk-test", "baseURL": "http://oc-sahte-llm:8099/v1" },
      "models": { "test-model": {} }
    }
  }
}
JSON

docker network rm "$AG" >/dev/null 2>&1 || true
docker network create --internal "$AG" >/dev/null
log "izole ağ kuruldu (internal: true)"

docker run -d --name oc-sahte-llm --network "$AG" \
  -v "$DIZIN/sahte.py:/app/sahte.py:ro" python:3.12-slim python /app/sahte.py >/dev/null
docker run -d --name oc-runner-test --network "$AG" \
  -v "$DIZIN/opencode.json:/home/agent/.config/opencode/opencode.json:ro" \
  --entrypoint sh "$IMAJ" -c 'opencode serve --hostname 0.0.0.0 --port 4096' >/dev/null

for _ in $(seq 1 40); do
  docker exec oc-runner-test curl -fsS http://127.0.0.1:4096/global/health >/dev/null 2>&1 && break
  sleep 1
done
log "motor ayakta"

# Ağın gerçekten kapalı olduğu KANITLANIR; yoksa test hiçbir şey ölçmez.
if docker exec oc-runner-test curl -m 5 -sS https://registry.npmjs.org/ -o /dev/null 2>/dev/null; then
  log "HATA: ağ kapalı değil, test anlamsız"; exit 1
fi
log "paket deposuna erişim yok (beklenen)"

ID=$(docker exec oc-runner-test curl -fsS -X POST http://127.0.0.1:4096/session \
       -H "Content-Type: application/json" -d '{}' | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
docker exec oc-runner-test curl -sS -X POST "http://127.0.0.1:4096/session/$ID/message" \
  -H "Content-Type: application/json" --max-time 90 -o /dev/null \
  -d '{"agent":"build","model":{"providerID":"sirket","modelID":"test-model"},
       "parts":[{"type":"text","text":"selam"}]}' 2>/dev/null || true

if [ "$(docker logs oc-sahte-llm 2>&1 | grep -c ISTEK-GELDI)" -eq 0 ]; then
  log "BAŞARISIZ: istek sağlayıcıya ulaşmadı — sürücü yüklenememiş"
  docker logs oc-runner-test 2>&1 | tail -20
  exit 1
fi
log "istek sağlayıcıya ULAŞTI"

if docker exec oc-runner-test test -d /home/agent/.config/opencode/node_modules/@ai-sdk/openai-compatible; then
  log "sürücü imajdan geliyor"
else
  log "BAŞARISIZ: sürücü imajda yok"; exit 1
fi

log "GEÇTİ — koşu sırasında paket deposuna hiç istek çıkmadı"
