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

# ─── Sağlayıcı sürücüleri ───────────────────────────────────────────────────
#
# NEDEN BURADA: motor, yerleşik olmayan bir sağlayıcı (litellm /
# openai_compatible) ilk kullanıldığında sürücü paketini ve kendi eklentisini
# KOŞU ANINDA paket deposundan indirmeye çalışıyor. SSL denetimi yapan
# kurumsal ağlarda bu istek düşüyor ve sağlayıcı hiç ayağa kalkmıyor; mesaj
# sağlayıcıya çıkmadan "500 UnknownError" olarak yansıyordu (spec 003,
# 2026-08-12 kararı). Air-gapped ortamlarda ise koşu hiç mümkün değildi.
#
# HEDEF DİZİN ÖLÇÜLDÜ: motor paketleri YAPILANDIRMA dizinine kuruyor —
# `$HOME/.config/opencode/node_modules`. Global npm kökü değil; oraya
# kurmak işe yaramıyor.
#
# Sürümler `runner/package.json` + `package-lock.json` ile sabit ve `npm ci`
# ile kuruluyor: kayan `latest` kullanılsaydı imaj tekrarlanabilir olmaz ve
# koşu anında değişen bir bağımlılık riski kalırdı.
#
# `--omit=dev --ignore-scripts`: derleme betikleri çalıştırılmıyor; bu
# paketlerin hiçbirinin kuruluma özel adımı yok ve betik çalıştırmak
# tedarik zinciri yüzeyini gereksiz büyütür.
COPY runner/package.json runner/package-lock.json /home/agent/.config/opencode/
RUN cd /home/agent/.config/opencode \
    && npm ci --omit=dev --ignore-scripts --no-audit --no-fund \
    && npm cache clean --force \
    && chown -R agent:agent /home/agent/.config

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

# Motorun paket deposuna çıkmasını engelleyen ayar.
#
# Sürücüler yukarıda kuruldu; yine de motor kendi `package.json`'ını yazıp
# kurulumu tetikleyebiliyor. `--offline` ile npm ağa HİÇ çıkmaz, yalnızca
# yerel önbelleğe ve mevcut node_modules'e bakar. Kurulu olan bir bağımlılık
# için bu bir no-op; kurulu olmayan için hızlı ve net bir hata.
#
# TLS DOĞRULAMASI KAPATILMIYOR. `strict-ssl=false` burada işi "çözerdi" ama
# çözmüş olmazdı: imaj başka kurumlarda da koşuyor ve doğrulamayı kaldırmak
# sorunu görünmez yapar, ortadan kaldırmaz. Kurumsal CA'nın yolu
# RUNNER_EXTRA_CA_CERT (bkz. README).
ENV NPM_CONFIG_OFFLINE=true \
    NPM_CONFIG_AUDIT=false \
    NPM_CONFIG_FUND=false

COPY --chmod=755 runner/entrypoint.sh /usr/local/bin/entrypoint.sh
# entrypoint tarafından source edilir; testi de aynı dosyayı source ediyor ki
# mantık iki yerde kopyalanmasın.
COPY --chmod=755 runner/git-credentials.sh /usr/local/bin/git-credentials.sh

WORKDIR /work
EXPOSE 4096

# Dışarıya port yayınlanmaz; backend izole Docker network'ü üzerinden erişir.
HEALTHCHECK --interval=3s --timeout=3s --start-period=5s --retries=20 \
    CMD curl -fsS "http://127.0.0.1:${OPENCODE_PORT:-4096}/global/health" || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
