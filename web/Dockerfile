# syntax=docker/dockerfile:1
# Build context: repository root (docker-compose sets context: ..)

# ── Builder ───────────────────────────────────────────────────────────────────
FROM node:24-alpine AS builder
WORKDIR /app

COPY web/package.json web/package-lock.json* ./
# BuildKit cache mount keeps the npm cache between builds without adding a layer.
RUN --mount=type=cache,target=/root/.npm npm ci

COPY web/ .
# OpenAPI spec is required to generate the TypeScript client types.
COPY server/api/openapi.yaml /api/openapi.yaml
# Generate TypeScript types from the OpenAPI spec, then build the SPA.
# OIDC configuration is NOT baked in here — the SPA fetches /config.json at
# runtime from the server, so a single image works across all environments.
RUN npm run generate && npm run build

# ── Runtime (nginx) ───────────────────────────────────────────────────────────
FROM nginxinc/nginx-unprivileged:alpine AS runner

# Remove the default server block shipped with the image.
RUN rm /etc/nginx/conf.d/default.conf

COPY --from=builder /app/dist /usr/share/nginx/html

# The unprivileged nginx entrypoint runs envsubst on files in /etc/nginx/templates/
# and writes the result to /etc/nginx/conf.d/ at container startup.
COPY web/nginx.conf /etc/nginx/templates/default.conf.template

# Only substitute ${API_URL}. Without this, envsubst would also wipe nginx's
# own variables ($host, $remote_addr, $proxy_add_x_forwarded_for, etc.).
ENV NGINX_ENVSUBST_TEMPLATE_VARS=API_URL

EXPOSE 8080
