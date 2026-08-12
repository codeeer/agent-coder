# agent-coder — geliştirme komutları
#
# Go ve Node'un host'ta kurulu olması gerekmez; her şey container içinde çalışır.

SHELL := /bin/bash
.DEFAULT_GOAL := help

# --project-directory proje kökünü işaret eder: compose kökteki .env dosyasını
# okur ve compose dosyalarındaki göreli yollar kökten çözülür. Bu bayrak
# olmadan compose .env'i deploy/ altında arar ve tüm port ayarları sessizce
# varsayılana düşer.
COMPOSE      := docker compose --project-directory "$(CURDIR)" -f deploy/docker-compose.yml
COMPOSE_DEV  := $(COMPOSE) -f deploy/docker-compose.dev.yml
COMPOSE_GHCR := $(COMPOSE) -f deploy/docker-compose.ghcr.yml
RUNNER_IMAGE := agent-coder/opencode-runner:latest

# Sürümlü runner etiketleri için taban ad (etiketsiz) ve sürüm listesi.
# Liste backend'in içinde çünkü uç nokta onu `go:embed` ile okuyor; CI ve
# Makefile aynı dosyayı depo kökünden okur — tek kaynak.
RUNNER_REPO      := $(firstword $(subst :, ,$(RUNNER_IMAGE)))
NODE_VERSION_FILE := backend/internal/runner/node-versions.txt
# `\#` KAÇIRILMAK ZORUNDA: make'te kaçırılmamış bir `#` satırın geri kalanını
# yorum yapar ve `$(shell …)` kapanmadan kesilir.
NODE_VERSIONS    := $(shell grep -vE '^[[:space:]]*(\#|$$)' $(NODE_VERSION_FILE) 2>/dev/null)

# Yayınlanan imajlar (make quickstart). Kendi çatalınızda GHCR_OWNER'ı
# değiştirin; IMAGE_TAG ile belirli bir sürüme sabitleyebilirsiniz.
GHCR_OWNER   ?= codeeer
IMAGE_TAG    ?= latest
GHCR_RUNNER   := ghcr.io/$(GHCR_OWNER)/agent-coder-runner:$(IMAGE_TAG)
GHCR_BACKEND  := ghcr.io/$(GHCR_OWNER)/agent-coder-backend:$(IMAGE_TAG)
GHCR_FRONTEND := ghcr.io/$(GHCR_OWNER)/agent-coder-frontend:$(IMAGE_TAG)

# Üç değişken compose'a HER çağrıda verilmek zorunda: overlay'deki
# `${BACKEND_IMAGE:-…}` varsayılanları yalnızca `latest` içindir, IMAGE_TAG ile
# sürüm sabitleyen kurulumda sessizce yanlış etikete düşerdi.
GHCR_ENV := RUNNER_IMAGE=$(GHCR_RUNNER) BACKEND_IMAGE=$(GHCR_BACKEND) FRONTEND_IMAGE=$(GHCR_FRONTEND)

# Yalnızca ekrana yazdırmak için .env'den okunur. Servislerin gerçek port
# yapılandırması compose tarafından yapılır.
env_or = $(or $(shell grep -E '^$(1)=' .env 2>/dev/null | cut -d= -f2- | tr -d '[:space:]'),$(2))
FRONTEND_PORT := $(call env_or,FRONTEND_PORT,3000)
BACKEND_PORT  := $(call env_or,BACKEND_PORT,8080)

# Go ve Node araçlarını host kurulumu olmadan çalıştırmak için.
# Debian tabanlı golang imajı bilinçli: `go test -race` cgo ister, alpine'da gcc yok.
GO_RUN   := docker run --rm -v "$(CURDIR)/backend":/src -w /src -v agent-coder-gomod:/go/pkg/mod golang:1.25
NODE_RUN := docker run --rm -v "$(CURDIR)/frontend":/app -w /app node:24-alpine

.PHONY: help
help: ## Bu yardımı göster
	@echo "agent-coder — kullanılabilir komutlar:"
	@echo
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
	@echo

# ─── Ortam ──────────────────────────────────────────────────────────────────

.PHONY: env
env: ## .env dosyasını .env.example'dan oluştur (varsa dokunmaz)
	@if [ -f .env ]; then \
		echo ".env zaten var, dokunulmadı."; \
	else \
		cp .env.example .env; \
		key=$$(openssl rand -base64 32); \
		sed -i.bak "s|^SECRET_ENCRYPTION_KEY=.*|SECRET_ENCRYPTION_KEY=$$key|" .env && rm -f .env.bak; \
		pw=$$(openssl rand -hex 16); \
		sed -i.bak "s|^OPENCODE_SERVER_PASSWORD=.*|OPENCODE_SERVER_PASSWORD=$$pw|" .env && rm -f .env.bak; \
		echo ".env oluşturuldu."; \
		echo "  ✓ SECRET_ENCRYPTION_KEY üretildi (bu anahtarı kaybederseniz kayıtlı"; \
		echo "    kimlik bilgileri çözülemez — yedekleyin)"; \
		echo "  ✓ OPENCODE_SERVER_PASSWORD üretildi"; \
		echo "  → OPENROUTER_API_KEY değerini elle girin."; \
	fi

.PHONY: check-env
check-env:
	@test -f .env || { echo "HATA: .env yok. Önce 'make env' çalıştırın."; exit 1; }
	@grep -q '^SECRET_ENCRYPTION_KEY=DEGISTIRIN' .env && { \
		echo "HATA: SECRET_ENCRYPTION_KEY hâlâ örnek değerde."; \
		echo "  Üretin ve .env içine yazın:  openssl rand -base64 32"; \
		exit 1; \
	} || true
	@# PUBLIC_HOST değiştirilmiş ama eski NEXT_PUBLIC_API_URL satırı duruyorsa,
	@# ikincisi birinciyi SESSİZCE ezer ve arayüz yanlış adrese bağlanır. Eski
	@# kurulumlardan gelen bir tuzak: o değişken bir zamanlar etkisizdi.
	@if grep -qE '^PUBLIC_HOST=' .env && ! grep -qE '^PUBLIC_HOST=(localhost|127\.0\.0\.1)\s*$$' .env \
		&& grep -qE '^NEXT_PUBLIC_API_URL=.+' .env; then \
		echo "UYARI: .env içinde hem PUBLIC_HOST hem NEXT_PUBLIC_API_URL dolu."; \
		echo "  NEXT_PUBLIC_API_URL önceliklidir; PUBLIC_HOST dikkate ALINMAZ."; \
		echo "  Ters vekil kullanmıyorsanız NEXT_PUBLIC_API_URL satırını silin."; \
	fi

# ─── Stack ──────────────────────────────────────────────────────────────────

.PHONY: up
up: check-env ## Tüm servisleri build edip başlat
	$(COMPOSE) up -d --build
	@echo
	@echo "  Arayüz : http://localhost:$(FRONTEND_PORT)"
	@echo "  API    : http://localhost:$(BACKEND_PORT)/health"

# quickstart, `up`ın hazır imajlı kardeşi: üç imaj da çekilir, hiçbir şey
# derlenmez. `make runner` adımına gerek kalmaz — kurulumun en uzun ve en sık
# atlanan adımı oydu.
.PHONY: quickstart
quickstart: check-env ## Hazır imajlarla başlat (hiçbir şey derlenmez — en hızlı yol)
	@echo "Hazır imajlar çekiliyor — hiçbir şey derlenmeyecek…"
	@# Runner AYRI çekilir: o bir compose servisi değil, backend'in çalışma
	@# anında başlattığı imaj. `compose pull` onu görmez, backend de kendisi
	@# indirmez (sandbox.EnsureImage yalnızca yerelde var mı diye bakar).
	docker pull $(GHCR_RUNNER)
	@# Sürümlü etiketler de çekilir: arayüzdeki sürüm seçicisi bunları
	@# listeliyor ve çekilmemiş bir sürüm seçmek koşuyu düşürürdü.
	@for s in $(NODE_VERSIONS); do \
		docker pull $(firstword $(subst :, ,$(GHCR_RUNNER))):node-$$s || \
			echo "  UYARI: node-$$s imajı çekilemedi — o sürüm seçilemez"; \
	done
	@$(GHCR_ENV) $(COMPOSE_GHCR) pull
	@# RUNNER_IMAGE .env'e YAZILIR, yalnızca bu komuta özel bırakılmaz.
	@# Aksi halde kullanıcı quickstart'tan sonra `make restart` çalıştırdığında
	@# hiç derlemediği yerel imaja geri döner ve kurulum sessizce bozulur.
	@sed -i.bak "s|^RUNNER_IMAGE=.*|RUNNER_IMAGE=$(GHCR_RUNNER)|" .env && rm -f .env.bak
	@echo "  .env → RUNNER_IMAGE=$(GHCR_RUNNER)"
	@$(GHCR_ENV) $(COMPOSE_GHCR) up -d
	@echo
	@echo "  Arayüz : http://localhost:$(FRONTEND_PORT)"
	@echo "  API    : http://localhost:$(BACKEND_PORT)/health"
	@# İmajın hangi commit'ten geldiği YAZDIRILIR.
	@#
	@# Bir kez yayınlanan imajlar main'in gerisinde kaldı ve kimse fark etmedi:
	@# `docker pull` "Image is up to date" diyordu — çünkü BAYAT olan registry'nin
	@# kendisiydi. Kullanıcının elindeki sürümü görebilmesi, o sessiz kaymanın
	@# tek panzehiri.
	@# `|| echo` YETMEZ: etiketi olmayan bir imajda `docker inspect` BAŞARILI
	@# olur ve boş satır döner; yedek değer ancak boşluk kontrolüyle devreye girer.
	@sha=$$(docker inspect $(GHCR_BACKEND) \
		--format '{{ index .Config.Labels "org.opencontainers.image.revision" }}' \
		2>/dev/null | cut -c1-7); \
	echo "  İmaj   : $${sha:-bilinmiyor} commit'inden"

.PHONY: dev
dev: check-env ## Hot reload ile geliştirme modunda başlat
	$(COMPOSE_DEV) up -d --build
	@echo
	@echo "  Arayüz : http://localhost:$(FRONTEND_PORT)  (next dev)"
	@echo "  API    : http://localhost:$(BACKEND_PORT)/health  (air)"

.PHONY: down
down: ## Servisleri durdur (veri korunur)
	$(COMPOSE_DEV) down --remove-orphans

.PHONY: clean
clean: ## Servisleri durdur ve TÜM verileri sil
	$(COMPOSE_DEV) down -v --remove-orphans

# restart, kurulumun TÜRÜNÜ korur.
#
# Eskiden `down up` idi ve `up` kaynaktan derliyordu: quickstart ile hazır imaj
# çeken kullanıcı tek bir `make restart` sonrası, farkında olmadan yerel
# derlemeye geçiyordu. Yayınlanan imajdaki bir düzeltme böylece sessizce geri
# alınabiliyordu — çektiği imaj değil, kendi ağacındaki kod koşuyordu.
#
# Ayrımı .env'deki RUNNER_IMAGE söylüyor: quickstart oraya GHCR adresini yazar,
# yerel kurulumda `agent-coder/...` kalır. Ayrı bir bayrak eklenmedi; ikinci bir
# işaretçi er geç birinciyle çelişirdi.
.PHONY: restart
restart: ## Yeniden başlat (kurulum türü korunur: hazır imaj / kaynaktan)
	@if grep -qE '^RUNNER_IMAGE=ghcr\.io/' .env 2>/dev/null; then \
		echo "Hazır imajlı kurulum — kaynaktan derlenmeyecek."; \
		$(MAKE) --no-print-directory down; \
		$(MAKE) --no-print-directory quickstart; \
	else \
		$(MAKE) --no-print-directory down up; \
	fi

.PHONY: ps
ps: ## Servis durumları
	$(COMPOSE) ps

.PHONY: logs
logs: ## Tüm servislerin canlı logları
	$(COMPOSE) logs -f --tail=100

.PHONY: logs-backend
logs-backend: ## Sadece backend logları
	$(COMPOSE) logs -f --tail=100 backend

.PHONY: logs-frontend
logs-frontend: ## Sadece frontend logları
	$(COMPOSE) logs -f --tail=100 frontend

# ─── Runner imajı ───────────────────────────────────────────────────────────

# Varsayılan imaj + listedeki HER sürüm.
#
# Sürümlüler de derleniyor çünkü kaynaktan kuran kullanıcı da arayüzden sürüm
# seçebiliyor; yalnızca varsayılan derlenseydi seçim "imaj bulunamadı" ile
# düşerdi. Liste büyüdükçe süre uzar — bedeli bilinçli.
.PHONY: runner
runner: ## opencode-runner imajını build et (varsayılan + listedeki sürümler)
	docker build -f runner/Dockerfile -t $(RUNNER_IMAGE) .
	@for s in $(NODE_VERSIONS); do \
		echo "Node $$s için runner imajı derleniyor…"; \
		docker build -f runner/Dockerfile --build-arg NODE_VERSION=$$s \
			-t $(RUNNER_REPO):node-$$s . || exit 1; \
	done

# ─── Veritabanı ─────────────────────────────────────────────────────────────

.PHONY: psql
psql: ## Postgres kabuğunu aç
	$(COMPOSE) exec postgres psql -U $${POSTGRES_USER:-agentcoder} -d $${POSTGRES_DB:-agentcoder}

.PHONY: migrate
migrate: ## Migration'ları uygula
	$(COMPOSE) exec backend /usr/local/bin/migrate up

.PHONY: migrate-down
migrate-down: ## Son migration'ı geri al
	$(COMPOSE) exec backend /usr/local/bin/migrate down

.PHONY: migrate-status
migrate-status: ## Migration durumunu göster
	$(COMPOSE) exec backend /usr/local/bin/migrate status

# ─── Test ve kalite ─────────────────────────────────────────────────────────

.PHONY: test
test: test-backend test-frontend test-runner ## Tüm testleri çalıştır (entegrasyon hariç)

.PHONY: test-integration
test-integration: ## Gerçek Postgres'e karşı entegrasyon testleri (stack ayakta olmalı)
	@docker network inspect agent-coder_internal >/dev/null 2>&1 \
		|| { echo "HATA: stack ayakta değil. Önce 'make up' çalıştırın."; exit 1; }
	@# Ayrı test veritabanı: geliştirme verisi testlerden etkilenmesin.
	@$(COMPOSE) exec -T postgres psql -U $${POSTGRES_USER:-agentcoder} -d postgres \
		-c "CREATE DATABASE agentcoder_test" >/dev/null 2>&1 || true
	docker run --rm --network agent-coder_internal \
		-v "$(CURDIR)/backend":/src -w /src -v agent-coder-gomod:/go/pkg/mod \
		-e TEST_DATABASE_URL="postgres://$${POSTGRES_USER:-agentcoder}:$(call env_or,POSTGRES_PASSWORD,agentcoder_dev)@postgres:5432/agentcoder_test?sslmode=disable" \
		golang:1.25 env CGO_ENABLED=1 go test ./... -race -count=1 -p 1
	@# -p 1: test paketleri tek veritabanını paylaştığı için sırayla çalışır.
	@# Paralel çalışırlarsa şemayı aynı anda kurmaya çalışıp çakışırlar.

.PHONY: test-backend
test-backend: ## Go testleri (race detector ile)
	$(GO_RUN) env CGO_ENABLED=1 go test ./... -race -count=1

.PHONY: test-frontend
test-frontend: ## Frontend birim testleri ve tip kontrolü
	$(NODE_RUN) npm run test
	$(NODE_RUN) npm run typecheck

# Runner'ın kabuk mantığı da test edilir: git kimlik bilgisi kurulumu yalnızca
# "temiz" kullanıcı adı/token'larda çalışıyordu ve arıza ancak kurumsal bir
# kurulumda, klonlama anında görülüyordu.
#
# Runner İMAJINDA değil, backend testleriyle aynı golang imajında koşar: test
# edilen şey bash + git davranışı ve o her ikisinde de aynı. Runner imajına
# bağlansaydı `make test`, imajı hiç derlememiş biri için düşerdi — testin
# çalışması için dakikalarca derleme beklemek, testin çalıştırılmamasına yol açar.
.PHONY: test-runner
test-runner: ## Runner kabuk testleri (git kimlik bilgisi kurulumu)
	docker run --rm -v "$(CURDIR)/runner":/r:ro golang:1.25 bash /r/git-credentials-test.sh

.PHONY: lint
lint: lint-backend lint-frontend lint-shell ## Tüm linter'ları çalıştır

.PHONY: lint-backend
lint-backend: ## gofmt + go vet
	@$(GO_RUN) sh -c 'test -z "$$(gofmt -l .)" || { echo "gofmt gerekli:"; gofmt -l .; exit 1; }'
	$(GO_RUN) go vet ./...

.PHONY: lint-frontend
lint-frontend: ## eslint
	$(NODE_RUN) npm run lint

.PHONY: lint-shell
lint-shell: ## Kabuk betiklerini shellcheck ile denetle
	docker run --rm -v "$(CURDIR)":/mnt -w /mnt koalaman/shellcheck-alpine:stable \
		shellcheck runner/entrypoint.sh runner/git-credentials.sh runner/git-credentials-test.sh

.PHONY: fmt
fmt: ## Go kodunu biçimlendir
	$(GO_RUN) gofmt -w .

.PHONY: tidy
tidy: ## go.mod / go.sum düzenle
	$(GO_RUN) go mod tidy

.PHONY: smoke
smoke: ## Uçtan uca duman testi — GERÇEK Docker + GERÇEK model (para harcar)
	@docker network inspect agent-coder_internal >/dev/null 2>&1 \
		|| { echo "HATA: stack ayakta değil. Önce 'make up' çalıştırın."; exit 1; }
	@docker image inspect $(RUNNER_IMAGE) >/dev/null 2>&1 \
		|| { echo "HATA: runner imajı yok. Önce 'make runner' çalıştırın."; exit 1; }
	docker run --rm --network agent-coder_internal \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v "$(CURDIR)/backend":/src -w /src -v agent-coder-gomod:/go/pkg/mod \
		-e SMOKE_TEST_API_KEY="$(call env_or,OPENROUTER_API_KEY,)" \
		-e RUNNER_IMAGE="$(RUNNER_IMAGE)" \
		-e RUNNER_NETWORK=agent-coder_internal \
		golang:1.25 go test ./internal/runner/opencode/... -count=1 -v -timeout 20m
