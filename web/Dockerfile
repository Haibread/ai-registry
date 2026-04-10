# syntax=docker/dockerfile:1
# Build context: repository root (docker-compose sets context: ..)

# ── Dependencies ──────────────────────────────────────────────────────────────
FROM node:25-alpine AS deps
WORKDIR /app
COPY web/package.json web/package-lock.json* ./
RUN npm ci

# ── Builder ───────────────────────────────────────────────────────────────────
FROM node:25-alpine AS builder
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
# Stored as a template; API_URL is substituted at container startup via envsubst
# so that a single image works across docker-compose and Kubernetes.
COPY web/nginx.conf /etc/nginx/conf.d/default.conf.template
EXPOSE 80
# Only substitute ${API_URL} — quoting the variable list prevents nginx's own
# $host, $remote_addr, $uri etc. from being wiped by envsubst.
CMD ["/bin/sh", "-c", "envsubst '${API_URL}' < /etc/nginx/conf.d/default.conf.template > /etc/nginx/conf.d/default.conf && exec nginx -g 'daemon off;'"]
