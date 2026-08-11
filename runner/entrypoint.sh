#!/usr/bin/env bash
#
# Runner container'ının giriş noktası:
#   1. (varsa) hedef repo'yu /work içine klonlar
#   2. opencode'u headless HTTP sunucu olarak başlatır
#
# Yapılandırma (opencode.json ve agent tanımı) container BAŞLATILMADAN ÖNCE
# backend tarafından /home/agent/.config/opencode/ altına kopyalanır; burada
# üretilmez. Sebebi: opencode agent tanımlarını yalnızca açılışta okur.
set -euo pipefail

# shellcheck source=runner/git-credentials.sh
source /usr/local/bin/git-credentials.sh

readonly WORKDIR=/work
readonly PORT="${OPENCODE_PORT:-4096}"

# Kimlik bilgisi eşleşmezse git kullanıcı adı SORMAYA çalışıyor; terminal
# olmadığı için "could not read Username ... No such device or address" ile
# ölüyor ve bu mesaj asıl nedeni gizliyor. Sorma kapatılınca git hemen ve
# anlaşılır biçimde ("terminal prompts disabled") düşer.
export GIT_TERMINAL_PROMPT=0

log() { printf '[runner] %s\n' "$*" >&2; }

# URL'deki kimlik bilgilerini loglamadan önce maskeler.
mask_url() { sed -E 's#(://)[^/@]+@#\1***@#' <<<"$1"; }

die() { log "HATA: $*"; exit 1; }

[[ -n "${AGENT_CODER_PROVIDER_KEY:-}" ]] || die "sağlayıcı anahtarı tanımlı değil"

if [[ -n "${REPO_URL:-}" ]]; then
    # Kimlik bilgisi remote URL'e GÖMÜLMEZ — .git/config içinde kalıcı olur ve
    # sızabilir. Bunun yerine credential store kullanılır; dosya yalnızca bu
    # container'da yaşar ve iş bitince container'la birlikte silinir.
    if [[ -n "${GIT_TOKEN:-}" ]]; then
        host="$(git_host_cikar "$REPO_URL")"
        [[ "$host" != "$REPO_URL" ]] || die "REPO_URL bir https adresi olmalı"

        # GitHub token'ı kullanıcı adı istemez; Bitbucket ve genel Git ister.
        user="${GIT_USERNAME:-x-access-token}"

        # Kimlik bilgisi remote URL'e gömülmez; store'a git'in kendi kaçırma
        # kurallarıyla yazılır (bkz. git-credentials.sh).
        git_kimlik_kur "$host" "$user" "$GIT_TOKEN"
    fi

    git config --global user.name "${GIT_AUTHOR_NAME:-Agent Coder}"
    git config --global user.email "${GIT_AUTHOR_EMAIL:-agent-coder@localhost}"
    git config --global --add safe.directory "$WORKDIR"
    git config --global advice.detachedHead false

    log "repo klonlanıyor: $(mask_url "$REPO_URL") (branch: ${REPO_BRANCH:-varsayılan})"

    clone_args=(--depth "${GIT_CLONE_DEPTH:-1}")
    [[ -n "${REPO_BRANCH:-}" ]] && clone_args+=(--branch "$REPO_BRANCH")

    git clone "${clone_args[@]}" "$REPO_URL" "$WORKDIR" 2>&1 \
        | sed -E 's#(://)[^/@]+@#\1***@#' \
        || die "klonlama başarısız"

    log "klonlandı: $(git -C "$WORKDIR" rev-parse --short HEAD)"
fi

cd "$WORKDIR"

log "opencode serve başlatılıyor (port ${PORT})"
exec opencode serve --hostname 0.0.0.0 --port "$PORT"
