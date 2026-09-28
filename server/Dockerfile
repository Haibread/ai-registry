# syntax=docker/dockerfile:1
# Multi-stage build: compile in the Go toolchain image, ship in distroless.

# ── Build stage ───────────────────────────────────────────────────────────────
# Pin the builder to the native BUILDPLATFORM so multi-arch builds cross-compile
# with the Go toolchain (GOARCH=$TARGETARCH below) instead of running the whole
# toolchain under QEMU emulation — much faster and avoids emulation flakiness.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS builder

WORKDIR /src

# Cache module downloads separately from source code.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# TARGETOS/TARGETARCH are injected automatically by BuildKit per target platform.
# Declared WITHOUT defaults so the injected values win; for a plain `docker build`
# (no buildx platform) they are empty and `go build` targets the native host.
ARG TARGETOS
ARG TARGETARCH

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
# The nonroot variant runs as UID 65532; matches Helm podSecurityContext. USER
# restates it numerically so runAsNonRoot can verify it without a passwd lookup.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab

# OCI image labels. The publish workflow augments these with per-build
# labels (revision, created, version) via docker/metadata-action. These
# static labels are the fallback for local builds that don't go through CI.
# https://github.com/opencontainers/image-spec/blob/main/annotations.md
LABEL org.opencontainers.image.title="ai-registry-server" \
      org.opencontainers.image.description="AI Registry HTTP API (MCP + A2A agent cards)" \
      org.opencontainers.image.source="https://github.com/haibread/ai-registry" \
      org.opencontainers.image.url="https://github.com/haibread/ai-registry" \
      org.opencontainers.image.documentation="https://github.com/haibread/ai-registry/blob/main/README.md" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.vendor="haibread"

COPY --from=builder /out/server /server

USER 65532:65532

# Default port — overridden by HTTP_ADDR env var in production (e.g. :8080 in k8s).
EXPOSE 8081

# distroless has no shell or curl: the binary probes its own /healthz.
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD ["/server", "--healthcheck"]

ENTRYPOINT ["/server"]
