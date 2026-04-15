# syntax=docker/dockerfile:1
# Multi-stage build: compile in the Go toolchain image, ship in distroless.

# ── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

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
# distroless/static is ~2 MB — no shell, no package manager, no OS utilities.
# The Go binary is fully static (CGO_ENABLED=0) so nothing else is needed.
# The nonroot variant runs as UID 65532; matches Helm podSecurityContext.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/server /server

# Default port — overridden by HTTP_ADDR env var in production (e.g. :8080 in k8s).
EXPOSE 8081

ENTRYPOINT ["/server"]
