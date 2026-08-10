# opencode-runner — bir agent adımı için ayağa kalkan geçici container.
#
# Bu bir compose servisi DEĞİLDİR. Backend her agent adımı için bu imajdan
# yeni bir container başlatır, iş bitince container'ı ve volume'ünü siler.
#
# Debian tabanı bilinçli seçildi: opencode'un dağıttığı ikili dosyalar glibc'ye
# bağlı, Alpine'ın musl'ı ile çalışmaz.
FROM node:24-slim

# git      — repo klonlama ve diff üretimi
# ca-certs — OpenRouter'a HTTPS
# curl     — sağlık kontrolü ve hata ayıklama
# openssh  — ileride SSH remote desteği için
RUN apt-get update && apt-get install -y --no-install-recommends \
        git \
        ca-certificates \
        curl \
        openssh-client \
    && rm -rf /var/lib/apt/lists/*

# Sürüm pinlenir — opencode API'si sürümler arası değişebilir.
# Yükseltirken backend/internal/runner/opencode istemcisi de doğrulanmalı.
ARG OPENCODE_VERSION=1.18.15
RUN npm install -g "opencode-ai@${OPENCODE_VERSION}" \
    && npm cache clean --force \
    && opencode --version

# Container root olarak çalışmaz.
RUN useradd --create-home --uid 10001 --shell /bin/bash agent \
    && mkdir -p /work \
    && chown -R agent:agent /work

USER agent
ENV HOME=/home/agent

# Yapılandırma ve agent tanımı bu imaja GÖMÜLMEZ. Backend, her çalıştırmada
# container'ı oluşturur ama başlatmaz, dosyaları /home/agent/.config/opencode/
# altına kopyalar, sonra başlatır. Sebebi: agent tanımları veritabanından geliyor
# ve opencode onları yalnızca açılışta okuyor (spec 003'te ölçüldü).
#
# Dizin önceden oluşturulur ki kopyalama sahiplik sorunu yaşamasın.
#
# scripts/ — agent'a atanmış hazır kabuk betikleri (spec 012). Bunlar da
# çalıştırma anında kopyalanır; imaja gömülmezler. /work altında DEĞİL: orası
# klonlama hedefi ve boş olmak zorunda.
RUN mkdir -p /home/agent/.config/opencode/agents /home/agent/scripts

COPY --chmod=755 runner/entrypoint.sh /usr/local/bin/entrypoint.sh

WORKDIR /work
EXPOSE 4096

# Dışarıya port yayınlanmaz; backend izole Docker network'ü üzerinden erişir.
HEALTHCHECK --interval=3s --timeout=3s --start-period=5s --retries=20 \
    CMD curl -fsS "http://127.0.0.1:${OPENCODE_PORT:-4096}/global/health" || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
