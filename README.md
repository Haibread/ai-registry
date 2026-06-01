# AI Registry

> A centralized, spec-compatible registry for AI ecosystem artifacts — **MCP servers** and **A2A agents** — with a clean public browse UI, an admin CRUD console, and a first-class HTTP API.

[![CI](https://github.com/Haibread/ai-registry/actions/workflows/ci.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/ci.yml)
[![E2E](https://github.com/Haibread/ai-registry/actions/workflows/e2e.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/e2e.yml)
[![Publish](https://github.com/Haibread/ai-registry/actions/workflows/publish.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/publish.yml)

A single place to publish, discover, and evaluate AI ecosystem building blocks. Every entry is:

- **Versioned** — immutable published versions, draft/deprecated lifecycle.
- **Spec-aware** — MCP metadata follows the [Model Context Protocol](https://modelcontextprotocol.io/) `server.json` field shapes; every agent emits a [Google A2A](https://a2a-protocol.org/) Agent Card at `/.well-known/agent-card.json`.
- **API-first** — UIs are thin clients; nothing lives in the UI that isn't in the API.
- **Observable** — every handler traced, every DB call a child span, every business metric an OTel counter or histogram.

---

## Features

### MCP Registry

- Browse, search, filter, and inspect MCP servers by namespace, runtime (stdio / http / sse), ecosystem (npm / pypi / oci / …), verification status, and tags.
- First-class `tools[]` field per version — the publisher-declared tool list, distinct from the MCP spec's `capabilities.tools` negotiation flag. Tool cards render name, description, input schema, and annotations on the detail page.
- View/copy counters, freshness indicators, report-entry workflow.

### Agent Registry

- Browse agents by namespace, skills, and tags, each with a structured card and detail page.
- Auto-generated A2A Agent Cards at `/agents/{namespace}/{slug}/.well-known/agent-card.json`, plus a global `/.well-known/agent-card.json` making the registry itself a first-class A2A citizen.
- A2A schema-conformant: `skills[]` validated at write time (`id`, `name`, `description`, `tags`); `securitySchemes` restricted to an explicit allowlist (Bearer, ApiKey, OAuth2, OpenIdConnect).

### Two UIs, one API

- **Public UI** — read-only, no auth: browse, search, detail pages, JSON inspect, copy endpoints.
- **Admin UI** (`/admin`) — full CRUD, guarded by OIDC or local login: publishers (with role grants), groups, users, MCP servers + versions, agents + versions, audit log, reports triage, feature-flag management, and the change-approval review queue.
- **Publisher-scoped admin home** — a publisher switcher scopes the admin area to one publisher (Server Admin picks any; a member sees those they hold a role on). The selected publisher gets a scoped Overview (attention strip + counts + recent-activity timeline), a Members page (manage that publisher's role grants), and an Activity feed — backed by `GET /api/v1/publishers/{slug}/stats` and `/activity` (any publisher member; 403 otherwise). A Settings page lets a publisher **Admin** edit the publisher's name + contact via `PATCH /api/v1/publishers/{slug}` (publisher Admin or Server Admin); the slug is permanent and deletion stays Server-Admin-only.
- **Both UIs consume the same versioned HTTP API** — zero client-only features.

### AuthN/AuthZ

- **Registry-issued sessions (BFF).** Both login front doors end in a `Secure; HttpOnly` cookie session — the browser never holds a token. OIDC is brokered server-side: the registry is a single confidential client that runs Authorization Code + PKCE at `/api/v1/auth/oidc/login`, exchanges the code with its `client_secret`, validates the `id_token`, maps the identity to a `users` row, and triggers RP-initiated logout on sign-out so the IdP session ends too. Local email + password login (`POST /api/v1/auth/login`) sets the same cookie. Server Admin comes from `realm_access.roles[]` containing `"admin"` **or** a local `is_server_admin` flag (bootstrap admin). Keycloak runs in local dev via docker-compose with a pre-seeded realm.
- **Publisher-scoped RBAC** — roles (Viewer/Editor/Reviewer/Admin) granted to users or groups on a publisher: Editor authors, Reviewer approves, Admin manages, or global Server Admin. Write endpoints 403 without the role, independent of the UI. Claims carry **group membership only** via the configurable JWT claim path `AUTH_GROUPS_CLAIM` (default `groups`). Admin list endpoints take `mine=true` to scope results to the caller's publishers (authors don't see each other's resources); `GET /api/v1/me` returns identity + effective grants for role-gating the UI.
- **Change-approval workflow** — publisher Editors submit version edits or deletion requests that a Reviewer approves before they go live. The cross-publisher review queue is gated by the reviewer group (`AUTH_REVIEWER_GROUP`, default `registry-reviewers`). Discriminated 409 errors prevent stale-edit clobbering.

### Observability

- OpenTelemetry SDK for traces, metrics, and logs; OTLP export (gRPC or HTTP). Every HTTP handler is wrapped by `otelhttp.NewHandler`; DB calls produce child spans; structured logs carry `trace_id` and `span_id`.
- Business metrics (request counts, latency histograms, registry entry counts) exposed as OTel counters/histograms — contract-tested so regressions fail CI.
- OTel Collector config checked into `deploy/otel-collector-config.yaml`.

---

## Tech stack

**Server** — Go 1.25 · [chi](https://github.com/go-chi/chi) v5 · [pgx/v5](https://github.com/jackc/pgx) · PostgreSQL 18 · [golang-migrate](https://github.com/golang-migrate/migrate) · [jwt/v5](https://github.com/golang-jwt/jwt) · [oklog/ulid](https://github.com/oklog/ulid) · [testcontainers-go](https://github.com/testcontainers/testcontainers-go) · OpenTelemetry SDK + OTLP exporter

**Frontend** — [Vite](https://vitejs.dev/) · React 19 · [React Router v7](https://reactrouter.com/) · [TanStack Query v5](https://tanstack.com/query/v5) · TypeScript · [shadcn/ui](https://ui.shadcn.com/) + Radix · Tailwind v4 · Vitest + React Testing Library · Playwright (e2e)

**Infra** — docker-compose (`docker-compose.yml` baseline + `dev` overlay; `ci` overlay for CI only) · Helm chart with optional CNPG-managed PostgreSQL 18 cluster, HTTPRoute, and Ingress · Keycloak for local OIDC · OTel Collector. (A dedicated `docker-compose.prod.yml` profile is parked under v0.4.x.)

**API spec** — Hand-written OpenAPI 3.1 at `server/api/openapi.yaml` (**90 operations**), embedded into the binary and served live at `/openapi.yaml`. Server types and the TypeScript client are generated from it; a bijection test keeps router and spec from drifting.

---

## Architecture at a glance

```
       ┌─────────────────┐   ┌─────────────────┐
       │   Public SPA    │   │    Admin SPA    │
       │   (read-only)   │   │ (/admin, auth)  │
       └────────┬────────┘   └────────┬────────┘
                │                     │
                │    HTTP/JSON (v1)   │
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
│   ├── mcp/        MCP registry endpoints
│   ├── agents/     Agent registry + A2A card generation
│   ├── auth/       OIDC broker, session validation, RBAC guards
│   ├── store/      Postgres repositories (pgx)
│   ├── domain/     Entities, validation
│   ├── bootstrap/  Seed-from-YAML with idempotent upsert + narrow tools backfill
│   └── observability/  OTel tracer, meter, logger providers
└── migrations/     SQL migrations (forward-only)

web/                Vite + React SPA (public + admin)
├── src/components/ shadcn/ui + feature components
├── src/pages/      React Router v7 routes
├── src/lib/        API client (generated from OpenAPI), utils
└── src/auth/       session-cookie auth (login redirect + /api/v1/me)

deploy/             docker-compose profiles, Keycloak realm, OTel config
└── helm/ai-registry/  Kubernetes chart (optional CNPG cluster)
docs/               Architecture notes
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
| http://localhost:8081/.well-known/agent-card.json | Global A2A Agent Card          |
| http://localhost:8180/       | Keycloak (realm `ai-registry`)                      |

The dev realm provisions four users covering every authorization path. Full definition in [`deploy/keycloak-realm-dev.json`](deploy/keycloak-realm-dev.json).

| Username                  | Password   | Realm role | Groups                              | Exercises                                    |
| ---                       | ---        | ---        | ---                                 | ---                                          |
| `admin@example.com`       | `admin`    | `admin`    | —                                   | Full admin (every write path)                |
| `author@example.com`      | `author`   | —          | `anthropic-core`, `anthropic-labs`  | Editor authoring via a group role grant + submit-for-review |
| `reviewer@example.com`    | `reviewer` | —          | `registry-reviewers`                | Approve / reject in the change-approval queue |
| `user@example.com`        | `user`     | —          | —                                   | 403 baseline (no roles, no groups)           |

Claims carry only group membership; a group authorizes writes when it holds a role grant (e.g. Editor) on the target publisher. Grant roles to users or groups from the publisher detail page in the admin UI, or via the `/api/v1/publishers/{slug}/grants` API.

### Seeding from a YAML bootstrap file

Point the server at a bootstrap file and it upserts publishers, MCP servers, and agents on every boot:

```yaml
# deploy/bootstrap.example.yaml
publishers:
  - slug: acme
    name: Acme Corp
    verified: true

mcp_servers:
  - publisher: acme
    slug: files
    name: Files Server
    # …
```

Full reference in [`deploy/bootstrap.example.yaml`](deploy/bootstrap.example.yaml). Bootstrap seeds the catalog only (publishers, servers, agents) — role grants are managed via the admin API. It is idempotent: existing rows are skipped on re-runs, with one narrow documented exception — it backfills `tools[]` when newly declared in the YAML.

---

## Configuration

Every setting is available in **all three**, highest precedence first:

1. **Environment variable** — `UPPER_SNAKE_CASE` (e.g. `DATABASE_URL`)
2. **YAML config file** — `lower_snake_case` key, path via `CONFIG_FILE` env or `--config` flag
3. **Built-in default** — `server/internal/config/config.go`

Full list in `deploy/config.example.yaml` and `deploy/.env.example`. Sensitive values (DSNs, client secrets) should come from env or a secrets manager, not a committed file.

---

## API surface

90 operations across these tags:

| Tag          | Purpose                                                        |
| ---          | ---                                                            |
| `system`     | `/healthz`, `/readyz`, OpenAPI spec, global `.well-known/*`    |
| `auth`       | OIDC broker (`/auth/oidc/login`, `/callback`, logout) + local email + password login (`POST /api/v1/auth/login`); caller identity + effective grants (`GET /api/v1/me`) |
| `publishers` | Namespace/publisher CRUD                                       |
| `rbac`       | Groups, users, and publisher-scoped role grants               |
| `mcp`        | MCP server + version CRUD, search, detail, view/copy, reports, change-approval (submit / withdraw / approve / reject / deletion-request) |
| `agents`     | Agent + version CRUD, per-agent A2A card, change-approval (same shape as MCP) |
| `reports`    | Abuse / issue reports + admin triage                          |
| `review`     | Reviewer-only `GET /review-queue` listing pending versions and deletions |
| `audit`      | Admin-only audit log                                           |

All operations live under `/api/v1/` and are generated from the same OpenAPI document.

---

## Quality gates

CI enforces contracts that mechanically prevent drift between spec, code, and the MCP / A2A specifications:

- **OpenAPI ↔ router bijection** — every chi route has an operation in `openapi.yaml` and vice versa; extra or missing either side fails the build.
- **A2A Agent Card JSON Schema** — `server/api/a2a-agent-card.schema.json` pins the a2a-project/a2a June 2025 shape; every emission is validated against it.
- **Write-authorization router contract** — every write endpoint requires a publisher-scoped role (Editor/Reviewer/Admin) or Server Admin, never anonymous; an ungated write route fails CI.
- **OTel span emission contract** — every handler produces a span; drift fails CI.
- **Migration forward-apply + idempotency** — all 15 forward migrations apply cleanly on a fresh Postgres via testcontainers.
- **Public rate-limit wiring** — unauthenticated reads are rate-limited by middleware, not handler code.
- **Web test suite** — 580+ Vitest + React Testing Library tests; Playwright e2e on admin flows including the change-approval workflow.

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

- **Branching** — never push directly to `main`; feature branches per task.
- **Commits** — [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, `chore:`, `test:`).
- **Spec-first** — when touching the API, update `server/api/openapi.yaml` first, then regenerate types, then implement the handler.
- **Tests required** — every new function, handler, or repository method needs unit coverage; handlers and repositories also need integration coverage.
- **OTel on every handler** — new handlers get a span via the existing tracer from context, never an ad-hoc provider.
- **Forward-only migrations** — down migrations exist for local dev convenience only; never rely on them in production.

See [`CLAUDE.md`](./CLAUDE.md) for the full set of non-negotiables.

### Pre-commit hooks

[pre-commit](https://pre-commit.com) runs formatters, linters, and secret scanners before each commit; the same hooks (minus those covered by dedicated jobs) run in CI. Install the framework, enable the git hook, then install the tools the `language: system` hooks shell out to:

```bash
# framework + git hook (run once per clone)
pipx install pre-commit            # or: brew install pre-commit
pre-commit install

# tools the hooks invoke (must be on PATH)
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
go install github.com/norwoodj/helm-docs/cmd/helm-docs@latest
# gofmt ships with the Go toolchain; helm is already required for the chart.
# hadolint: see https://github.com/hadolint/hadolint (or `brew install hadolint`).
# The web hooks (eslint, tsc) need `npm ci` run in web/ first.
```

Run every hook across the whole tree:

```bash
pre-commit run --all-files
```

---

## Roadmap

The phased roadmap lives in [`PLAN.md`](./PLAN.md). Shipped: everything through publisher-scoped RBAC, local accounts, and the brokered-OIDC / HttpOnly-cookie-session rework (with the `/v0` surface removed). Remaining (tracked in `PLAN.md`): API-key (M2M) auth, a Skills / Prompts registry, federation, webhooks, and a dedicated production `docker-compose.prod.yml` profile.

---

## Specifications referenced

- [Model Context Protocol](https://modelcontextprotocol.io/)
- [MCP registry reference implementation](https://github.com/modelcontextprotocol/registry)
- [Google A2A Protocol / Agent Card](https://a2a-protocol.org/)
- [OAuth 2.1 draft](https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/)
- [RFC 7807 — Problem Details for HTTP APIs](https://www.rfc-editor.org/rfc/rfc7807)

---

## Status

Pre-1.0. The API is versioned (`/api/v1/`) and contract-tested, but breaking changes may still land on minor bumps before `v1.0.0`.

## License

[MIT](LICENSE) © 2026 The AI Registry Authors.
