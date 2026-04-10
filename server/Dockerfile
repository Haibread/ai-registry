# syntax=docker/dockerfile:1
# Multi-stage build: compile in the Go toolchain image, ship in alpine.

# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache module downloads separately from source code
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.21

# wget is used by the Docker healthcheck in docker-compose.
RUN apk add --no-cache wget && \
    addgroup -S -g 1001 appgroup && \
    adduser -S -u 1001 -G appgroup appuser

COPY --from=builder /out/server /server

USER appuser
EXPOSE 8081

ENTRYPOINT ["/server"]
