# syntax=docker/dockerfile:1
# Build context: repository root (docker-compose sets context: ..)

# ── Dependencies ──────────────────────────────────────────────────────────────
FROM node:22-alpine AS deps
WORKDIR /app
COPY web/package.json web/package-lock.json* ./
RUN npm ci

# ── Builder ───────────────────────────────────────────────────────────────────
FROM node:22-alpine AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY web/ .
# OpenAPI spec is required to generate the TypeScript client types.
COPY server/api/openapi.yaml /api/openapi.yaml
# Generate TypeScript types from the OpenAPI spec.
# OIDC configuration is NOT baked in here — the SPA fetches /config.json at
# runtime from the server, so a single image works across all environments.
RUN npm run generate
RUN npm run build

# ── Runtime (nginx) ───────────────────────────────────────────────────────────
FROM nginx:alpine AS runner
COPY --from=builder /app/dist /usr/share/nginx/html
COPY web/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
