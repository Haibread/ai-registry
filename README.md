# AI Registry

> A centralized, spec-compatible registry for AI ecosystem artifacts — **MCP servers** and **A2A agents** — with a clean public browse UI, an admin CRUD console, and a first-class HTTP API.

[![CI](https://github.com/Haibread/ai-registry/actions/workflows/ci.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/ci.yml)
[![E2E](https://github.com/Haibread/ai-registry/actions/workflows/e2e.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/e2e.yml)
[![Publish](https://github.com/Haibread/ai-registry/actions/workflows/publish.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/publish.yml)

AI Registry gives teams a single place to publish, discover, and evaluate the building blocks of the AI ecosystem. Every entry is:

- **Versioned** — immutable published versions, draft/deprecated lifecycle.
- **Spec-compatible** — MCP endpoints conform to the [Model Context Protocol](https://modelcontextprotocol.io/) registry shape; every agent emits a [Google A2A](https://a2a-protocol.org/) Agent Card at `/.well-known/agent-card.json`.
- **API-first** — the UIs are thin clients. Nothing lives in the UI that isn't in the API.
- **Observable** — every handler is traced, every DB call is a child span, every business metric is an OTel counter or histogram.

---

## Features

### MCP Registry

- Browse, search, filter, and inspect MCP servers by namespace, runtime (stdio / http / sse), ecosystem (npm / pypi / oci / …), verification status, and tags.
- First-class `tools[]` field on each version — the publisher-declared tool list, distinct from the MCP spec's `capabilities.tools` capability-negotiation flag. Tool cards render name, description, input schema, and annotations on the detail page.
- Strict `/v0/` endpoints pinned to the MCP registry wire format (top-level `servers`, `metadata.count`/`nextCursor`, RFC 7807 errors, RFC 3339 timestamps) and validated by a 40-test conformance suite.
- View/copy counters, freshness indicators, report-entry workflow.

### Agent Registry

- Browse agents by namespace, skills, and tags, each with a structured card and detail page.
- Auto-generated A2A Agent Cards at `/agents/{namespace}/{slug}/.well-known/agent-card.json`, plus a global `/.well-known/agent-card.json` that makes the registry itself a first-class A2A citizen.
- A2A schema-conformant: `skills[]` validated at write time (`id`, `name`, `description`, `tags`), `securitySchemes` restricted to an explicit allowlist (Bearer, ApiKey, OAuth2, OpenIdConnect).

### Two UIs, one API

- **Public UI** — read-only. Browse, search, detail pages, JSON inspect, copy endpoints. No auth required.
- **Admin UI** (`/admin`) — full CRUD, guarded by OIDC login. Publishers, MCP servers + versions, agents + versions, audit log, reports triage, feature-flag management.
- **Both UIs consume the same versioned HTTP API.** Zero client-only features.

### AuthN/AuthZ

- OAuth 2.1 / OIDC with PKCE (public client via [`oidc-client-ts`](https://github.com/authts/oidc-client-ts) — no client secret, no NextAuth/Auth.js).
- Keycloak in local dev via docker-compose with a pre-seeded realm.
- `realm_access.roles[]` contains `"admin"` → unlocks the admin scope. Write endpoints 403 without it, independent of the UI. Middleware-enforced, never UI-enforced.
- MCP-authorization-spec compatible (resource indicators, protected resource metadata).

### Observability

- OpenTelemetry SDK for traces, metrics, and logs; OTLP export (gRPC or HTTP).
- Every HTTP handler is wrapped by `otelhttp.NewHandler`. DB calls produce child spans. Structured logs carry `trace_id` and `span_id`.
- Business metrics (request counts, latency histograms, registry entry counts) exposed as OTel counters/histograms — contract-tested so regressions fail CI.
- OTel Collector config checked into `deploy/otel-collector-config.yaml`.

---

## Tech stack

**Server** — Go 1.25 · [chi](https://github.com/go-chi/chi) v5 · [pgx/v5](https://github.com/jackc/pgx) · PostgreSQL 16 · [golang-migrate](https://github.com/golang-migrate/migrate) · [jwt/v5](https://github.com/golang-jwt/jwt) · [oklog/ulid](https://github.com/oklog/ulid) · [testcontainers-go](https://github.com/testcontainers/testcontainers-go) · OpenTelemetry SDK + OTLP exporter

**Frontend** — [Vite](https://vitejs.dev/) · React 19 · [React Router v7](https://reactrouter.com/) · [TanStack Query v5](https://tanstack.com/query/v5) · TypeScript · [shadcn/ui](https://ui.shadcn.com/) + Radix · Tailwind v4 · [oidc-client-ts](https://github.com/authts/oidc-client-ts) · Vitest + React Testing Library · Playwright (e2e)

**Infra** — docker-compose (dev / ci / prod) · Helm chart with optional CNPG-managed PostgreSQL cluster, HTTPRoute, and Ingress · Keycloak for local OIDC · OTel Collector

**API spec** — Hand-written OpenAPI 3.1 at `server/api/openapi.yaml` (**59 operations**), embedded into the binary and served live at `/openapi.yaml`. Server types and the TypeScript client are generated from the spec. A bijection test ensures the router and spec never drift.

---

## Architecture at a glance

```
       ┌─────────────────┐   ┌─────────────────┐
       │   Public SPA    │   │    Admin SPA    │
       │   (read-only)   │   │ (/admin, auth)  │
       └────────┬────────┘   └────────┬────────┘
                │                     │
                │  HTTP/JSON (v1/v0)  │
                └──────────┬──────────┘
                           ▼
                  ┌─────────────────┐
                  │ Go server (chi) │
                  │   OpenAPI 3.1   │
                  │   OIDC · OTel   │
                  └────────┬────────┘
                           │
            ┌──────────────┼──────────────┐
            ▼              ▼              ▼
      ┌───────────┐  ┌───────────┐  ┌───────────┐
      │ Postgres  │  │ Keycloak  │  │   OTel    │
      │  + JSONB  │  │  (OIDC)   │  │ Collector │
      └───────────┘  └───────────┘  └───────────┘
```

Directory layout:

```
server/             Go service
├── api/            OpenAPI 3.1 spec + A2A agent-card JSON schema (embedded)
├── cmd/server/     Entrypoint
├── internal/
│   ├── http/       chi router, handlers, middleware (auth, logging, rate limit)
│   ├── mcp/        MCP registry endpoints + /v0/ wire-format layer
│   ├── agents/     Agent registry + A2A card generation
│   ├── auth/       OIDC/JWT validation, scopes, admin guard
│   ├── store/      Postgres repositories (pgx)
│   ├── domain/     Entities, validation
│   ├── bootstrap/  Seed-from-YAML with idempotent upsert + narrow tools backfill
│   └── observability/  OTel tracer, meter, logger providers
└── migrations/     SQL migrations (forward-only)

web/                Vite + React SPA (public + admin)
├── src/components/ shadcn/ui + feature components
├── src/pages/      React Router v7 routes
├── src/lib/        API client (generated from OpenAPI), utils
└── src/auth/       oidc-client-ts PKCE flow

deploy/             docker-compose profiles, Keycloak realm, OTel config
└── helm/ai-registry/  Kubernetes chart (optional CNPG cluster)
docs/               Architecture notes, ADRs
PLAN.md             Phased implementation roadmap
design.md           System architecture, observability, data & API, UI/UX
CLAUDE.md           Project non-negotiables (API-first, spec compat, OTel, etc.)
```

---

## Quick start (local dev)

Prerequisites: Docker + Docker Compose.

```bash
git clone git@github.com:Haibread/ai-registry.git
cd ai-registry

# Brings up: Postgres, Keycloak (pre-seeded realm), OTel Collector, server, web
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.dev.yml up -d --build
```

Then open:

| URL                          | What                                                |
| ---                          | ---                                                 |
| http://localhost:8080        | Public SPA (browse MCP servers + agents)            |
| http://localhost:8080/admin  | Admin SPA (sign in via Keycloak)                    |
| http://localhost:8081/openapi.yaml | Live OpenAPI 3.1 spec                         |
| http://localhost:8081/api/v1/mcp/servers | JSON API (versioned)                    |
| http://localhost:8081/v0/servers         | MCP-registry-spec wire format           |
| http://localhost:8081/.well-known/agent-card.json | Global A2A Agent Card          |
| http://localhost:8180/       | Keycloak (realm `ai-registry`)                      |

A sample admin user is provisioned by the dev realm. See `deploy/keycloak-realm-dev.json`.

### Seeding from a YAML bootstrap file

Point the server at a bootstrap file and it will upsert publishers, MCP servers, and agents on every boot:

```yaml
# deploy/bootstrap.example.yaml
publishers:
  - namespace: acme
    display_name: Acme Corp
mcp_servers:
  - namespace: acme
    slug: files
    name: Files Server
    # …
```

Bootstrap is idempotent. Existing versions are skipped — except for a narrow, documented exception that backfills the `tools[]` array when it has just been declared in the YAML.

---

## Configuration

Every setting is available in **all three** of these, with the listed precedence (highest wins):

1. **Environment variable** — `UPPER_SNAKE_CASE` (e.g. `DATABASE_URL`)
2. **YAML config file** — `lower_snake_case` key, path via `CONFIG_FILE` env or `--config` flag
3. **Built-in default** — `server/internal/config/config.go`

See `deploy/config.example.yaml` and `deploy/.env.example` for the full list. Sensitive values (DSNs, client secrets) should come from env or a secrets manager, not a committed file.

---

## API surface

59 operations across these tags:

| Tag          | Purpose                                                        |
| ---          | ---                                                            |
| `system`     | `/healthz`, `/readyz`, OpenAPI spec, global `.well-known/*`    |
| `publishers` | Namespace/publisher CRUD                                       |
| `mcp`        | MCP server + version CRUD, search, detail, view/copy, reports  |
| `agents`     | Agent + version CRUD, per-agent A2A card                       |
| `audit`      | Admin-only audit log                                           |
| `v0`         | Strict MCP-registry-spec-compatible read layer                 |

Versioned private API lives under `/api/v1/`; the spec-compatible wire-format layer lives under `/v0/`. Both are generated from the same OpenAPI document.

---

## Quality gates

The CI pipeline enforces a set of contracts that mechanically prevent drift between spec, code, and the MCP / A2A specifications:

- **OpenAPI ↔ router bijection** — every route in the chi router has an operation in `openapi.yaml` and vice versa. Extra or missing either side = build failure.
- **`/v0/` MCP wire-format conformance** — 40 tests pinning response shapes, cursor semantics, error envelopes, and RFC 3339 timestamps to the MCP registry spec.
- **A2A Agent Card JSON Schema** — `server/api/a2a-agent-card.schema.json` pins the a2a-project/a2a June 2025 shape; every emission is validated against it.
- **Admin-guard router contract** — every write endpoint requires `registry:admin`, independent of the UI.
- **OTel span emission contract** — every handler produces a span; drift fails CI.
- **Migration forward-apply + idempotency** — all 7 migrations apply cleanly on a fresh Postgres via testcontainers.
- **Public rate-limit wiring** — unauthenticated read endpoints are rate-limited by middleware, not handler code.
- **Web test suite** — 500+ Vitest + React Testing Library tests; Playwright e2e on admin flows.

Run the suites locally:

```bash
# Go unit + integration (testcontainers Postgres)
cd server && go test ./...

# Web unit + component
cd web && npm test

# Web e2e (Playwright)
cd web && npm run test:e2e
```

---

## Development workflow

- **Branching** — never push directly to `main`. Feature branches per task.
- **Commits** — [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `chore:`, `test:`).
- **Spec-first** — when touching the API, update `server/api/openapi.yaml` first, then regenerate types, then implement the handler.
- **Tests required** — every new function, handler, or repository method needs unit coverage. Handlers and repositories also need integration coverage.
- **OTel on every handler** — new handlers get a span via the existing tracer from context, never an ad-hoc provider.
- **Forward-only migrations** — down migrations exist for local dev convenience only; never rely on them in production.

See [`CLAUDE.md`](./CLAUDE.md) for the full set of non-negotiables.

---

## Roadmap

The phased roadmap lives in [`PLAN.md`](./PLAN.md). High-level status:

- **v0.1.x** — Foundation: Postgres schema, chi router, OIDC, MCP + agent CRUD, public browse UI, admin UI, bootstrap seeding. ✅
- **v0.2.x** — Observability + coverage depth. OTel traces/metrics/logs wired everywhere; contract tests for every CLAUDE.md non-negotiable; `/v0/` wire-format conformance; A2A schema conformance. ✅ (current release: **v0.2.2**)
- **v0.3.x** — Browse polish. Real MCP `tools[]` field end-to-end, card redesign, namespace landing pages, per-entry activity feed. 🚧 (Task 1 shipped)
- **v0.4.x and beyond** — Skills/prompts registry, federation, API-key auth (M2M), webhooks.

---

## Specifications referenced

- [Model Context Protocol](https://modelcontextprotocol.io/)
- [MCP registry reference implementation](https://github.com/modelcontextprotocol/registry)
- [Google A2A Protocol / Agent Card](https://a2a-protocol.org/)
- [OAuth 2.1 draft](https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/)
- [RFC 7807 — Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc7807)

---

## Status

Pre-1.0. The API is versioned (`/api/v1/`, `/v0/`) and the contract tests keep it honest, but breaking changes may still land on minor bumps before `v1.0.0`.

## License

License TBD — this repository does not yet ship a `LICENSE` file. Please open an issue if you'd like to use the code before one lands.
