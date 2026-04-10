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
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.21

# wget is used by the docker-compose healthcheck.
# UID/GID 1000 matches the Helm chart's podSecurityContext (runAsUser: 1000).
RUN apk add --no-cache wget && \
    addgroup -S -g 1000 appgroup && \
    adduser -S -u 1000 -G appgroup appuser

COPY --from=builder /out/server /server

USER appuser

# Default port — overridden by HTTP_ADDR env var in production (e.g. :8080 in k8s).
EXPOSE 8081

ENTRYPOINT ["/server"]
