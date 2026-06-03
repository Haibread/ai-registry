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

- **Registry-issued bearer tokens.** The registry is the single token authority. Both login front doors mint a short-lived **access token** (Ed25519 JWT, sent as `Authorization: Bearer …`) plus a rotating **refresh token** (single-use, stored only as a SHA-256 hash; replaying a rotated one revokes the whole lineage). There is no cookie. OIDC is brokered server-side: the registry is a single confidential client that runs Authorization Code + PKCE at `/api/v1/auth/oidc/login`, exchanges the code with its `client_secret`, validates the `id_token`, maps groups + roles from configurable claim paths (`AUTH_GROUPS_CLAIM`, `OIDC_ROLES_CLAIM`, `OIDC_ADMIN_ROLE`), provisions a `users` row, and hands the SPA a one-time code that it swaps for tokens at `/api/v1/auth/oidc/exchange`; sign-out (`POST /api/v1/auth/logout`) revokes the refresh token and returns the IdP RP-initiated logout URL. Local email + password login (`POST /api/v1/auth/login`) mints the same token pair; clients refresh at `/api/v1/auth/refresh`. The signing key is set via `JWT_SIGNING_KEY` (a PEM Ed25519 key) or `JWT_SIGNING_SEED` (a high-entropy secret string the key is derived from deterministically, ≥ 32 chars — the PEM wins if both are set); an ephemeral key is generated in dev when neither is set. Public keys are published at `/.well-known/jwks.json`. Server Admin comes from the configured roles claim containing the admin role **or** a local `is_server_admin` flag (bootstrap admin). Keycloak runs in local dev via docker-compose with a pre-seeded realm.
- **Publisher-scoped RBAC** — roles (Viewer/Editor/Reviewer/Admin) granted to users or groups on a publisher: Editor authors, Reviewer approves, Admin manages, or global Server Admin. Write endpoints 403 without the role, independent of the UI. Access tokens carry **group membership only** (snapshotted at login from `AUTH_GROUPS_CLAIM`, default `groups`); per-publisher roles are resolved server-side from grants on every write. Admin list endpoints take `mine=true` to scope results to the caller's publishers (authors don't see each other's resources); `GET /api/v1/me` returns identity + effective grants for role-gating the UI. No cookie means **no CSRF surface** — the bearer header is never sent ambiently.
- **Change-approval workflow** — publisher Editors submit version edits or deletion requests that a Reviewer approves before they go live. The cross-publisher review queue is gated by the reviewer group (`AUTH_REVIEWER_GROUP`, default `registry-reviewers`). Discriminated 409 errors prevent stale-edit clobbering.

### Observability

- OpenTelemetry SDK for traces, metrics, and logs; OTLP export (gRPC or HTTP). Every HTTP handler is wrapped by `otelhttp.NewHandler`; DB calls produce child spans; structured logs carry `trace_id` and `span_id`.
- Business metrics (request counts, latency histograms, registry entry counts) exposed as OTel counters/histograms — contract-tested so regressions fail CI.
- OTel Collector config checked into `deploy/otel-collector-config.yaml`.

---

## Tech stack

- **Server** — Go + chi · pgx · PostgreSQL · golang-migrate · OpenTelemetry SDK. Exact versions: see `server/go.mod`.
- **Frontend** — Vite + React Router v7 + TanStack Query + TypeScript + shadcn/ui + Tailwind; Vitest and Playwright for tests. Exact versions: see `web/package.json`.
- **Infra** — docker-compose for local/self-hosted, Helm chart (optional CNPG Postgres) for k8s, Keycloak for local OIDC, OTel Collector.

The codebase splits into two top-level directories: `server/` (Go service) and `web/` (Vite + React SPA), with `deploy/` for compose profiles and the Helm chart.

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

The dev realm provisions users covering every authorization path. Dev realm users: see [`deploy/keycloak-realm-dev.json`](deploy/keycloak-realm-dev.json).

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

Every setting resolves from env var, then YAML config file, then built-in default (highest precedence first). All keys are documented in `deploy/.env.example` (and `deploy/config.example.yaml`); defaults live in `server/internal/config/config.go`.

---

## API surface

All operations live under `/api/v1/`. API surface: see `server/api/openapi.yaml` (served live at `/openapi.yaml`). Server types and the TypeScript client are generated from it.

---

## Quality gates

CI mechanically prevents drift between spec, code, and the A2A specification — enforcing the OpenAPI↔router bijection, A2A Agent Card schema conformance, write-authorization on every write route, and an OTel span per handler. CI contracts: see `.github/workflows/`.

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

[pre-commit](https://pre-commit.com) runs formatters, linters, and secret scanners before each commit; the same hooks run in CI. Install with `pre-commit install`; see [`.pre-commit-config.yaml`](.pre-commit-config.yaml) for the hooks and the tools they shell out to.

---

## Roadmap

The phased roadmap lives in [`PLAN.md`](./PLAN.md). Shipped: everything through publisher-scoped RBAC, local accounts, and the brokered-OIDC / registry-issued bearer-token (access + rotating refresh) rework (with the `/v0` surface removed). Remaining (tracked in `PLAN.md`): API-key (M2M) auth, a Skills / Prompts registry, federation, webhooks, and a dedicated production `docker-compose.prod.yml` profile.

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
