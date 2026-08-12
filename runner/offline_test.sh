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
# ─── GÖZLEMLENEBİLİRLİK ─────────────────────────────────────────────────────
#
# Bu betik bir kez CI'da askıda kaldı: python imajının çekilmesinden sonra
# 21 dakika boyunca TEK SATIR çıktı üretmedi ve iş 6 saatlik varsayılana
# yaslandı. Nerede beklediği anlaşılamadı çünkü:
#   · container logları yalnızca HATA yolunda, o da `tail` ile dökülüyordu
#   · aşamalar arası zaman damgası yoktu
#   · motor çevrimiçi log basmıyordu
#
# Bu yüzden artık: iki container'ın çıktısı test boyunca CANLI akar (önekli),
# motor DEBUG seviyesinde ve `--print-logs` ile konuşur, her aşamanın önünde
# zaman damgası vardır ve çıkışta — başarı da olsa — motorun kendi log
# dosyaları ile iki container'ın durum/çıkış kodu dökülür.
#
# Testin NE ÖLÇTÜĞÜ değişmedi; yalnızca görünürlüğü arttı.
#
# Kullanım: runner/offline_test.sh [imaj]
set -euo pipefail

IMAJ="${1:-agent-coder/opencode-runner:latest}"
AG="agent-coder-offline-test"
DIZIN="$(mktemp -d)"

# Canlı log izleyicilerinin PID'leri; temizlikte durdurulurlar.
IZLE_MOTOR=""
IZLE_LLM=""

log() { printf '[offline-test %s] %s\n' "$(date +%T)" "$*"; }

# ─── Temizlik: her çıkışta TÜM erişilebilir kanıtı dök ──────────────────────
#
# Sıra önemli: önce izleyiciler durdurulur, sonra loglar okunur, EN SON
# container'lar silinir. Ters sırada olsaydı silinmiş bir container'dan log
# okunmaya çalışılırdı.
temizle() {
  local kod=$?

  kill "$IZLE_MOTOR" "$IZLE_LLM" 2>/dev/null || true

  echo
  log "── motorun kendi log dosyaları ──"
  # Motor `--print-logs` ile zaten stderr'e basıyor ve yukarıda canlı aktı;
  # dosyalar yine de dökülüyor çünkü akış kopmuş olabilir (container erken
  # ölürse `docker logs -f` de biter).
  # BOŞ ÇIKTI BIRAKMA: dosya yoksa bunu söyle. Sessiz bir bölüm, "dökmeyi
  # denedim ve başaramadım" ile "dökülecek bir şey yoktu" arasındaki farkı
  # gizler — bu betiğin tamamı o farkı görünür kılmak için var.
  docker exec oc-runner-test sh -c '
    bulundu=0
    for f in /home/agent/.local/share/opencode/log/*.log; do
      [ -e "$f" ] || continue
      bulundu=1
      echo "== $f =="
      cat "$f"
    done
    [ "$bulundu" -eq 1 ] || echo "(log dosyası yok — motor --print-logs ile stderr'"'"'e yazıyor, yukarıdaki [motor] akışına bakın)"
  ' 2>/dev/null || log "(motor log dosyaları okunamadı — container yok olabilir)"

  log "── container durumları ──"
  for c in oc-runner-test oc-sahte-llm; do
    durum=$(docker inspect "$c" \
      --format '{{.State.Status}} exit={{.State.ExitCode}} oom={{.State.OOMKilled}}' 2>/dev/null) \
      || durum="(container yok)"
    log "  $c: $durum"
  done

  docker rm -f oc-sahte-llm oc-runner-test >/dev/null 2>&1 || true
  docker network rm "$AG" >/dev/null 2>&1 || true
  rm -rf "$DIZIN"

  log "çıkış kodu: $kod"
}
trap temizle EXIT

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
    # İstekler stdout'a yazılıyor (ISTEK-GELDI); http.server'ın kendi erişim
    # satırları gürültü olurdu.
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

log "imaj: $IMAJ"

docker network rm "$AG" >/dev/null 2>&1 || true
docker network create --internal "$AG" >/dev/null
log "izole ağ kuruldu (internal: true)"

# ─── Sahte sağlayıcı ────────────────────────────────────────────────────────
log "sahte sağlayıcı başlatılıyor (python:3.12-slim yerelde yoksa çekilecek)…"
docker run -d --name oc-sahte-llm --network "$AG" \
  -v "$DIZIN/sahte.py:/app/sahte.py:ro" python:3.12-slim python /app/sahte.py >/dev/null
{ docker logs -f oc-sahte-llm 2>&1 | sed 's/^/[sahte-llm] /'; } &
IZLE_LLM=$!
# `disown`: iş listesinden düşer, böylece temizlikte öldürüldüğünde kabuk
# "Terminated: 15" satırı basmaz. O satır gerçek bir hata gibi okunuyor ve
# bu betiğin amacı tam tersi — çıktıdaki her satır anlamlı olmalı.
disown "$IZLE_LLM" 2>/dev/null || true
log "sahte sağlayıcı ayakta (log akışı açıldı)"

# ─── Motor ──────────────────────────────────────────────────────────────────
#
# `--print-logs --log-level DEBUG`: motor ayrıntıyı stderr'e basar ve
# `docker logs -f` ile canlı akar. Sağlayıcı çözümleme ve sürücü yükleme
# adımları tek tek görünür — testin ölçtüğü şey tam olarak orası.
log "motor başlatılıyor (DEBUG seviyesinde)…"
docker run -d --name oc-runner-test --network "$AG" \
  -v "$DIZIN/opencode.json:/home/agent/.config/opencode/opencode.json:ro" \
  --entrypoint sh "$IMAJ" -c \
  'opencode serve --hostname 0.0.0.0 --port 4096 --print-logs --log-level DEBUG' >/dev/null
{ docker logs -f oc-runner-test 2>&1 | sed 's/^/[motor] /'; } &
IZLE_MOTOR=$!
disown "$IZLE_MOTOR" 2>/dev/null || true

# Motorun ayağa kalkmasını bekle.
#
# Döngü BAŞARISIZLIĞI YUTMAMALI. Öncesinde `for … done` sonrası koşulsuz
# "motor ayakta" yazılıyordu: motor hiç kalkmasa bile test devam ediyor,
# sonraki adımda sessizce asılıyordu.
log "health bekleniyor…"
hazir=0
for i in $(seq 1 40); do
  if docker exec oc-runner-test curl -fsS -m 3 \
      http://127.0.0.1:4096/global/health >/dev/null 2>&1; then
    hazir=1
    log "health geldi ($i/40)"
    break
  fi
  # Her denemeyi yazmak 40 satıra kadar çıkabilir ama sessiz aralık
  # bırakmamak daha değerli: askıda kalındığında kaçıncı denemede
  # olunduğu görünür.
  log "  health bekleniyor ($i/40)"
  sleep 1
done

if [ "$hazir" -ne 1 ]; then
  log "BAŞARISIZ: motor 40 sn içinde ayağa kalkmadı"
  exit 1
fi

# ─── Ağın gerçekten kapalı olduğu KANITLANIR ────────────────────────────────
# Yoksa test hiçbir şey ölçmez: sürücü koşu anında da inebilirdi.
log "ağın kapalı olduğu doğrulanıyor…"
if docker exec oc-runner-test curl -m 5 -sS https://registry.npmjs.org/ -o /dev/null 2>/dev/null; then
  log "BAŞARISIZ: ağ kapalı değil, test anlamsız"
  exit 1
fi
log "paket deposuna erişim yok (beklenen)"

# ─── Oturum ve mesaj ────────────────────────────────────────────────────────
#
# `-m/--max-time` ZORUNLU: zaman aşımsız bir curl, motor yanıt vermediğinde
# sonsuza kadar bekler ve testi askıda bırakır — bir kez öyle oldu.
log "oturum açılıyor…"
ID=$(docker exec oc-runner-test curl -fsS -m 20 -X POST http://127.0.0.1:4096/session \
       -H "Content-Type: application/json" -d '{}' | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')
if [ -z "$ID" ]; then
  log "BAŞARISIZ: oturum açılamadı"
  exit 1
fi
log "oturum açıldı: $ID"

log "mesaj gönderiliyor (en fazla 90 sn)…"
docker exec oc-runner-test curl -sS -X POST "http://127.0.0.1:4096/session/$ID/message" \
  -H "Content-Type: application/json" --max-time 90 -o /dev/null \
  -d '{"agent":"build","model":{"providerID":"sirket","modelID":"test-model"},
       "parts":[{"type":"text","text":"selam"}]}' 2>/dev/null || true
log "mesaj isteği bitti, sonuç değerlendiriliyor"

# ─── Ölçüm ──────────────────────────────────────────────────────────────────
if [ "$(docker logs oc-sahte-llm 2>&1 | grep -c ISTEK-GELDI)" -eq 0 ]; then
  log "BAŞARISIZ: istek sağlayıcıya ulaşmadı — sürücü yüklenememiş"
  exit 1
fi
log "istek sağlayıcıya ULAŞTI"

if docker exec oc-runner-test test -d /home/agent/.config/opencode/node_modules/@ai-sdk/openai-compatible; then
  log "sürücü imajdan geliyor"
else
  log "BAŞARISIZ: sürücü imajda yok"
  exit 1
fi

log "GEÇTİ — koşu sırasında paket deposuna hiç istek çıkmadı"
