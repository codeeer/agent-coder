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
RUNNER_IMAGE := agent-coder/opencode-runner:latest

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

.PHONY: restart
restart: down up ## Yeniden başlat

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

.PHONY: runner
runner: ## opencode-runner imajını build et (derleme bağlamı proje köküdür)
	docker build -f runner/Dockerfile -t $(RUNNER_IMAGE) .

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
test: test-backend test-frontend ## Tüm testleri çalıştır (entegrasyon hariç)

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

.PHONY: lint
lint: lint-backend lint-frontend ## Tüm linter'ları çalıştır

.PHONY: lint-backend
lint-backend: ## gofmt + go vet
	@$(GO_RUN) sh -c 'test -z "$$(gofmt -l .)" || { echo "gofmt gerekli:"; gofmt -l .; exit 1; }'
	$(GO_RUN) go vet ./...

.PHONY: lint-frontend
lint-frontend: ## eslint
	$(NODE_RUN) npm run lint

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
