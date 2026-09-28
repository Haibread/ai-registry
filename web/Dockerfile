# syntax=docker/dockerfile:1
# Build context: repository root (docker-compose sets context: ..)

# ── Builder ───────────────────────────────────────────────────────────────────
# Pin to the native BUILDPLATFORM: the Vite output is static and
# architecture-independent, so the (heavy) node build runs once on the host arch
# and never under QEMU emulation. The per-arch runtime nginx layer below pulls
# the matching architecture automatically.
FROM --platform=$BUILDPLATFORM node:24-alpine AS builder
WORKDIR /app

COPY web/package.json web/package-lock.json* ./
# BuildKit cache mount keeps the npm cache between builds without adding a layer.
RUN --mount=type=cache,target=/root/.npm npm ci

COPY web/ .
# OpenAPI spec is required to generate the TypeScript client types.
# Copy to /server/api/openapi.yaml so the relative path used by `npm run generate`
# (../server/api/openapi.yaml from /app) matches the local repo layout.
COPY server/api/openapi.yaml /server/api/openapi.yaml
# Generate TypeScript types from the OpenAPI spec, then build the SPA.
# OIDC configuration is NOT baked in here — the SPA fetches /config.json at
# runtime from the server, so a single image works across all environments.
RUN npm run generate && npm run build \
 && mkdir /app/conf.d

# ── Runtime (nginx) ───────────────────────────────────────────────────────────
FROM nginxinc/nginx-unprivileged:alpine AS runner

# The entrypoint renders /etc/nginx/templates/ into conf.d at startup, so conf.d
# must be writable by the runtime user. The base image owns it as uid 101 and
# the build runs as 101, which can replace it but not chown it: swap it for an
# empty directory copied in with the runtime ownership (this also drops the
# stock default.conf server block).
RUN rm -rf /etc/nginx/conf.d
COPY --from=builder --chown=65532:65532 /app/conf.d/ /etc/nginx/conf.d/

COPY --from=builder /app/dist /usr/share/nginx/html

# The unprivileged nginx entrypoint runs envsubst on files in /etc/nginx/templates/
# and writes the result to /etc/nginx/conf.d/ at container startup.
COPY web/nginx.conf /etc/nginx/templates/default.conf.template

# Only substitute ${API_URL}. Without this, envsubst would also wipe nginx's
# own variables ($host, $remote_addr, $proxy_add_x_forwarded_for, etc.).
ENV NGINX_ENVSUBST_TEMPLATE_VARS=API_URL

# UID/GID above 10000 rather than the base image's 101, so the container user
# never aliases a host account (Trivy KSV-0020/KSV-0021). nginx writes its pid
# and temp files under /tmp, so conf.d is the only path it needs to own.
USER 65532:65532

EXPOSE 8080
