# opencode-runner — bir agent adımı için ayağa kalkan geçici container.
#
# Bu bir compose servisi DEĞİLDİR. Backend her agent adımı için bu imajdan
# yeni bir container başlatır, iş bitince container'ı ve volume'ünü siler.
#
# Debian tabanı bilinçli seçildi: opencode'un dağıttığı ikili dosyalar glibc'ye
# bağlı, Alpine'ın musl'ı ile çalışmaz.
#
# Node sürümü DIŞARIDAN verilebilir: agent'ın koşturduğu build komutları belirli
# bir sürüm isteyebiliyor. Sürüm KOŞU ANINDA İNDİRİLMEZ — desteklenen her sürüm
# için derleme anında ayrı bir imaj yayınlanır (`node-<sürüm>` etiketi) ve
# kullanıcı koşuyu başlatmadan önce seçer. Liste tek yerde:
# `backend/internal/runner/node-versions.txt`.
#
# Varsayılan `24`, yani KAYAN yama — bugünkü davranışın aynısı. Tam sürüme
# sabitlemek `latest` imajını bulunduğu yamadan aşağı çekerdi; sabit sürüm
# isteyen `node-<sürüm>` etiketini seçer.
ARG NODE_VERSION=24
FROM node:${NODE_VERSION}-slim

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

# ─── Java ve Maven ──────────────────────────────────────────────────────────
#
# NEDEN BURADA, opencode'dan ÖNCE: Docker katmanları sırayla önbelleklenir.
# opencode sürümü aşağıda değiştiğinde bu katman geçerli kalır ve ~500MB'lık
# JDK indirmesi tekrarlanmaz. Ters sırada her opencode yükseltmesi JDK'ları da
# yeniden indirirdi.
#
# İKİ SÜRÜM BİRDEN: Java sürümü projeden projeye değişiyor ve hangisinin
# kullanılacağı agent'a TALİMATLA söyleniyor (spec 018 H1) — koşuda seçici yok.
# Ayrı imaj etiketi yapılsaydı etiket ekseni ikiye çıkar, beş ayrı yer (etiket
# kurgusu, Makefile, CI, doğrulama, arayüz) bunu öğrenmek zorunda kalırdı.
ARG JDK_DEFAULT=25
ARG JDK_ALT=17
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends gnupg; \
    mkdir -p /etc/apt/keyrings; \
    curl -fsSL https://packages.adoptium.net/artifactory/api/gpg/key/public \
        | gpg --dearmor -o /etc/apt/keyrings/adoptium.gpg; \
    . /etc/os-release; \
    echo "deb [signed-by=/etc/apt/keyrings/adoptium.gpg] https://packages.adoptium.net/artifactory/deb ${VERSION_CODENAME} main" \
        > /etc/apt/sources.list.d/adoptium.list; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        "temurin-${JDK_ALT}-jdk" "temurin-${JDK_DEFAULT}-jdk"; \
    apt-get purge -y --auto-remove gnupg; \
    rm -rf /var/lib/apt/lists/*; \
    \
    # MİMARİDEN BAĞIMSIZ SABİT YOL. Temurin'in kurulum dizini mimariye göre
    # değişiyor (`…-jdk-amd64` / `…-jdk-arm64`); agent'ın talimatına yazılacak
    # yol her iki mimaride de aynı olmak zorunda.
    mkdir -p /opt/java; \
    ln -s "/usr/lib/jvm/temurin-${JDK_ALT}-jdk-$(dpkg --print-architecture)" "/opt/java/${JDK_ALT}"; \
    ln -s "/usr/lib/jvm/temurin-${JDK_DEFAULT}-jdk-$(dpkg --print-architecture)" "/opt/java/${JDK_DEFAULT}"; \
    "/opt/java/${JDK_ALT}/bin/java" -version; \
    "/opt/java/${JDK_DEFAULT}/bin/java" -version

# Maven APT'TAN DEĞİL, sürümü sabitlenmiş tarball'dan.
#
# Debian'ın `maven` paketi `default-jdk`'yı bağımlılık olarak sürüklüyor: imaja
# üçüncü bir JDK girer ve hangi Java'nın kullanıldığı belirsizleşirdi. Sürümün
# sabit olması ayrıca imajı tekrarlanabilir kılıyor (opencode'daki kararın
# aynısı).
ARG MAVEN_VERSION=3.9.16
ARG MAVEN_SHA512=831a8591fe20c8243b1dbe7d71e3244f31d1665b0804b2e825e38cbbe5ce0cafb8338851f90780735568773e0a6cd07bbec107cda0b896b008b861075358b6f6
RUN set -eux; \
    curl -fsSL -o /tmp/maven.tar.gz \
        "https://dlcdn.apache.org/maven/maven-3/${MAVEN_VERSION}/binaries/apache-maven-${MAVEN_VERSION}-bin.tar.gz"; \
    echo "${MAVEN_SHA512}  /tmp/maven.tar.gz" | sha512sum -c -; \
    mkdir -p /opt/maven; \
    tar -xzf /tmp/maven.tar.gz -C /opt/maven --strip-components=1; \
    rm /tmp/maven.tar.gz; \
    ln -s /opt/maven/bin/mvn /usr/local/bin/mvn

ENV JAVA_HOME=/opt/java/25
ENV PATH="${JAVA_HOME}/bin:/opt/maven/bin:${PATH}"

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
# `.m2` BURADA AÇILIR — bir hatanın kaydı.
#
# Dizin önceden açılmadığında Docker'ın tar kopyası onu ROOT sahipliğiyle
# yaratıyor: `settings.xml` sorunsuz yazılıyor ama Maven kendi yerel deposunu
# (`~/.m2/repository`) içine açamıyor ve gerçek bir koşuda
# `AccessDeniedException` ile düşüyor. Dosyanın yazılmış olması dizinin
# kullanılabilir olduğu anlamına gelmiyordu.
RUN mkdir -p /home/agent/.config/opencode/agents /home/agent/scripts /home/agent/.m2

# Motorun paket deposuna çıkmasını engelleyen ayar — YALNIZCA MOTOR İÇİN.
#
# Sürücüler yukarıda kuruldu; yine de motor kendi `package.json`'ını yazıp
# kurulumu tetikleyebiliyor. `offline` ile npm ağa HİÇ çıkmaz, yalnızca yerel
# önbelleğe ve mevcut node_modules'e bakar. Kurulu olan bir bağımlılık için bu
# bir no-op; kurulu olmayan için hızlı ve net bir hata.
#
# NEDEN ENV DEĞİL .npmrc: bayrak `ENV NPM_CONFIG_OFFLINE=true` olarak
# konmuştu ve AGENT'IN KABUĞUNA DA GEÇİYORDU. Sonuç ölçüldü: agent kullanıcının
# deposunda `npm install` çalıştırdığında `ENOTCACHED` alıyordu — imajda
# önbellek de temizlendiği için kaçış yoktu. Yani kapalı ağ düzeltmesi,
# agent'ın bağımlılık kurmasını her ortamda imkânsız kılmıştı.
#
# `.npmrc` motorun ÇALIŞMA DİZİNİNDE: npm proje yapılandırmasını cwd'den
# yukarı doğru arar. Motor burada kurulum yapıyor, agent ise /work altında —
# ölçüldü: bu dizinde `offline=true`, /work altında `false`.
#
# TLS DOĞRULAMASI KAPATILMIYOR. `strict-ssl=false` burada işi "çözerdi" ama
# çözmüş olmazdı: imaj başka kurumlarda da koşuyor ve doğrulamayı kaldırmak
# sorunu görünmez yapar, ortadan kaldırmaz. Kurumsal CA'nın yolu
# RUNNER_EXTRA_CA_CERT (bkz. README).
RUN printf 'offline=true\n' > /home/agent/.config/opencode/.npmrc

# Bunlar imaj geneli kalabilir: gürültü kırpıyorlar, hiçbir şeyi engellemiyorlar.
ENV NPM_CONFIG_AUDIT=false \
    NPM_CONFIG_FUND=false

COPY --chmod=755 runner/entrypoint.sh /usr/local/bin/entrypoint.sh
# entrypoint tarafından source edilir; testi de aynı dosyayı source ediyor ki
# mantık iki yerde kopyalanmasın.
COPY --chmod=755 runner/git-credentials.sh /usr/local/bin/git-credentials.sh
COPY --chmod=755 runner/java-truststore.sh /usr/local/bin/java-truststore.sh

WORKDIR /work
EXPOSE 4096

# Dışarıya port yayınlanmaz; backend izole Docker network'ü üzerinden erişir.
HEALTHCHECK --interval=3s --timeout=3s --start-period=5s --retries=20 \
    CMD curl -fsS "http://127.0.0.1:${OPENCODE_PORT:-4096}/global/health" || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
