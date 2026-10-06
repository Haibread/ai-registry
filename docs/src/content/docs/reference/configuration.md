---
title: Configuration
description: How settings are given, and the ones a deployment has to set.
sidebar:
  order: 1
---

Every setting can be given as an environment variable, as a key in a YAML file
passed with `CONFIG_FILE` or `--config`, or left to its default — in that order
of precedence. Every key is documented in
[deploy/config.example.yaml](https://github.com/Haibread/ai-registry/blob/main/deploy/config.example.yaml) and
[deploy/.env.example](https://github.com/Haibread/ai-registry/blob/main/deploy/.env.example). The ones a real deployment has to
set:

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `DATABASE_URL` | yes | — | PostgreSQL connection string |
| `PUBLIC_BASE_URL` | yes | — | External URL of the deployment, used as the access-token issuer and in login redirects |
| `JWT_SIGNING_KEY` or `JWT_SIGNING_SEED` | yes | ephemeral key | PEM Ed25519 key, or a ≥ 32-char secret the key is derived from; without either, tokens survive neither a restart nor a second replica |
| `CORS_ALLOWED_ORIGINS` | when the SPA is on another origin | — | Comma-separated allowed origins |
| `AUTH_BOOTSTRAP_ADMIN_EMAIL` / `AUTH_BOOTSTRAP_ADMIN_PASSWORD` | no | — | Seeds a local Server Admin on first start |
| `OIDC_ISSUER`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `OIDC_REDIRECT_URL` | for OIDC login | — | Brokered OIDC client; leave empty for local login only |
| `OIDC_AUDIENCE` | no | — | Accept IdP-issued access tokens carrying this audience (machine clients) |
| `TRUSTED_PROXY_CIDR` | behind a proxy | — | CIDR of the trusted proxies; the client IP is the rightmost `X-Forwarded-For` hop outside it |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | no | — | OTLP endpoint; telemetry export is off when empty |
