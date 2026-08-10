# ─── Derleme ────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Bağımlılıklar önce indirilir; kaynak değişince bu katman önbellekten gelir.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/agent-coder/backend/internal/httpapi.Version=${VERSION}" \
    -o /out/server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# ─── Çalışma ────────────────────────────────────────────────────────────────
FROM alpine:3.21

# ca-certificates: sağlayıcılara HTTPS için
# tzdata: zaman damgalarının doğru diliminde olması için
# wget: healthcheck için
# git: proje eklerken depo erişimini `git ls-remote` ile doğrulamak için
RUN apk add --no-cache ca-certificates tzdata wget git \
    && adduser -D -u 10001 appuser

COPY --from=builder /out/server /usr/local/bin/server
COPY --from=builder /out/migrate /usr/local/bin/migrate

USER appuser
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
    CMD wget -qO- http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["/usr/local/bin/server"]
