# ─── Bağımlılıklar ──────────────────────────────────────────────────────────
FROM node:24-alpine AS deps

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund

# ─── Derleme ────────────────────────────────────────────────────────────────
FROM node:24-alpine AS builder

WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .

# NEXT_PUBLIC_* değişkenleri derleme anında gömülür — build-arg olarak gelmeli.
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
ENV NEXT_TELEMETRY_DISABLED=1

RUN npm run build

# ─── Çalışma ────────────────────────────────────────────────────────────────
FROM node:24-alpine AS runner

WORKDIR /app
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN addgroup -g 1001 -S nodejs && adduser -S nextjs -u 1001

# standalone çıktısı yalnızca gerçekten kullanılan node_modules'ü içerir.
COPY --from=builder /app/public ./public
COPY --from=builder --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=builder --chown=nextjs:nodejs /app/.next/static ./.next/static

USER nextjs
EXPOSE 3000
ENV PORT=3000
ENV HOSTNAME=0.0.0.0

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
    CMD wget -qO- http://127.0.0.1:3000/ >/dev/null || exit 1

CMD ["node", "server.js"]
