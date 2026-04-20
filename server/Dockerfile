# syntax=docker/dockerfile:1
# Multi-stage build: compile in the Go toolchain image, ship in distroless.

# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache module downloads separately from source code.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# TARGETOS/TARGETARCH are set automatically by BuildKit for multi-platform builds.
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Build metadata — injected at link time so `server --version` (and the
# startup log / OTel service.version attribute) reflect the exact image
# content. Defaults keep `docker build .` functional without requiring the
# caller to pass anything; CI overrides with real values.
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
      -ldflags="-s -w \
        -X main.Version=${VERSION} \
        -X main.Commit=${COMMIT} \
        -X main.BuildTime=${BUILD_TIME}" \
      -o /out/server ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
# distroless/static is ~2 MB — no shell, no package manager, no OS utilities.
# The Go binary is fully static (CGO_ENABLED=0) so nothing else is needed.
# The nonroot variant runs as UID 65532; matches Helm podSecurityContext.
FROM gcr.io/distroless/static-debian12:nonroot

# OCI image labels. The publish workflow augments these with per-build
# labels (revision, created, version) via docker/metadata-action. These
# static labels are the fallback for local builds that don't go through CI.
# https://github.com/opencontainers/image-spec/blob/main/annotations.md
LABEL org.opencontainers.image.title="ai-registry-server" \
      org.opencontainers.image.description="AI Registry HTTP API (MCP + A2A agent cards)" \
      org.opencontainers.image.source="https://github.com/haibread/ai-registry" \
      org.opencontainers.image.url="https://github.com/haibread/ai-registry" \
      org.opencontainers.image.documentation="https://github.com/haibread/ai-registry/blob/main/README.md" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.vendor="haibread"

COPY --from=builder /out/server /server

# Default port — overridden by HTTP_ADDR env var in production (e.g. :8080 in k8s).
EXPOSE 8081

ENTRYPOINT ["/server"]
