# AI Registry

> A centralized, spec-compatible registry for AI ecosystem artifacts — **MCP servers** and **A2A agents** — with a clean public browse UI, an admin CRUD console, and a first-class HTTP API.

[![Lint](https://github.com/Haibread/ai-registry/actions/workflows/lint.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/lint.yml)
[![Quality](https://github.com/Haibread/ai-registry/actions/workflows/quality.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/quality.yml)
[![Docker](https://github.com/Haibread/ai-registry/actions/workflows/docker.yml/badge.svg)](https://github.com/Haibread/ai-registry/actions/workflows/docker.yml)

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
- **Instance-wide tags** — Server Admins curate a registry-wide tag vocabulary (`GET /api/v1/tags`, managed on `/admin/tags`: slug, display name, description, badge color, active flag). Publishers tick tags from it when creating a version (validated server-side, frozen with the published version); entries surface their latest published version's tags as colored chips and the catalog filters on `?tag=`. In-use tags are deactivated rather than deleted, so frozen versions keep resolving. The vocabulary can also be defined declaratively (`instance_tags` config key / `INSTANCE_TAGS` env JSON / Helm `api.instanceTags`): config-listed tags are reconciled on startup and become read-only in the UI/API until removed from the configuration.

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

- **Registry-issued bearer tokens.** The registry is the single token authority. Both login front doors mint a short-lived **access token** (Ed25519 JWT, sent as `Authorization: Bearer …`) plus a rotating **refresh token** (single-use, stored only as a SHA-256 hash; replaying a rotated one revokes the whole lineage). There is no cookie. OIDC is brokered server-side: the registry is a single confidential client that runs Authorization Code + PKCE at `/api/v1/auth/oidc/login`, exchanges the code with its `client_secret`, validates the `id_token`, maps email, groups + roles from configurable claim paths (`AUTH_EMAIL_CLAIM`, `AUTH_GROUPS_CLAIM`, `OIDC_ROLES_CLAIM`, `OIDC_ADMIN_ROLE`), provisions a `users` row, and hands the SPA a one-time code that it swaps for tokens at `/api/v1/auth/oidc/exchange`; sign-out (`POST /api/v1/auth/logout`) revokes the refresh token and returns the IdP RP-initiated logout URL. Local email + password login (`POST /api/v1/auth/login`) mints the same token pair; clients refresh at `/api/v1/auth/refresh`. The signing key is set via `JWT_SIGNING_KEY` (a PEM Ed25519 key) or `JWT_SIGNING_SEED` (a high-entropy secret string the key is derived from deterministically, ≥ 32 chars — the PEM wins if both are set); an ephemeral key is generated in dev when neither is set. Public keys are published at `/.well-known/jwks.json`. Server Admin comes from the configured roles claim containing the admin role **or** a local `is_server_admin` flag (bootstrap admin). Keycloak runs in local dev via docker-compose with a pre-seeded realm.
- **Machine-to-machine (opt-in).** For callers that cannot run the interactive browser login — a Kubernetes operator, a CI job — set `OIDC_AUDIENCE` to accept an IdP-issued access token (e.g. a Keycloak **service account** via `client_credentials`) directly as the bearer. It is verified offline against the broker JWKS (no per-request network call, no DB) and accepted only when its `aud` claim contains `OIDC_AUDIENCE` — so a token minted for another client in the same realm is never honoured (configure a Keycloak audience mapper to stamp this `aud`). The realm-admin role then confers Server Admin and the `groups` claim drives publisher-scoped RBAC, exactly as a brokered login (the caller carries no `users` row; authorization runs off its claims). Leave `OIDC_AUDIENCE` empty to disable the path entirely — only registry-issued tokens are accepted, and the IdP-less / break-glass deployment is unchanged. A registry-native, hashed per-publisher API key remains planned (see roadmap).
- **Publisher-scoped RBAC** — roles (Viewer/Editor/Reviewer/Admin) granted to users or groups on a publisher: Editor authors, Reviewer approves, Admin manages, or global Server Admin. Write endpoints 403 without the role, independent of the UI. Access tokens carry **group membership only** (snapshotted at login from `AUTH_GROUPS_CLAIM`, default `groups`); per-publisher roles are resolved server-side from grants on every write. Admin list endpoints take `mine=true` to scope results to the caller's publishers (authors don't see each other's resources); `GET /api/v1/me` returns identity + effective grants for role-gating the UI. No cookie means **no CSRF surface** — the bearer header is never sent ambiently.
- **Change-approval workflow** — publisher Editors submit version edits, deletion requests, and entry-level changes (visibility, deprecate, metadata edits) that a Reviewer approves before they take effect; an Editor's request is enqueued (`202`) rather than applied, while a Server Admin retains an immediate escape hatch. The cross-publisher review queue is gated by the reviewer group (`AUTH_REVIEWER_GROUP`, default `registry-reviewers`). Discriminated 409 errors prevent stale-edit clobbering.

### Observability

- OpenTelemetry SDK for traces, metrics, and logs; OTLP export (gRPC or HTTP). Every HTTP handler is wrapped by `otelhttp.NewHandler`; DB calls produce child spans; structured logs carry `trace_id` and `span_id`.
- Business metrics (request counts, latency histograms, registry entry counts) exposed as OTel counters/histograms — contract-tested so regressions fail CI.
- OTel Collector config checked into `deploy/otel-collector-config.yaml`.

---

## Tech stack

- **Server** — Go + chi · pgx · PostgreSQL · golang-migrate · OpenTelemetry SDK. Exact versions: see `server/go.mod`.
- **Frontend** — Vite + React Router v7 + TanStack Query + TypeScript + shadcn/ui + Tailwind; Vitest and Playwright for tests. Exact versions: see `web/package.json`.
- **Infra** — docker-compose for local/self-hosted, Helm chart (optional CNPG Postgres) for k8s, Keycloak for local OIDC, OTel Collector.

The codebase splits into two top-level directories: `server/` (Go service) and `web/` (Vite + React SPA), with a root `docker-compose.yml` (dev/prod profiles) and `deploy/` for the Helm chart and supporting config (realm import, bootstrap sample, OTel config).

---

## Quick start (local dev)

Prerequisites: Docker + Docker Compose.

```bash
git clone git@github.com:Haibread/ai-registry.git
cd ai-registry

# Brings up: Postgres, Keycloak (pre-seeded realm), server, and the web SPA
# served by the Vite dev server (hot reload).
docker compose --profile dev up -d --build
```

The compose file at the repo root ships the web frontend in two mutually
exclusive flavours, each behind a profile:

```bash
docker compose --profile dev  up -d --build   # Vite dev server, hot reload
docker compose --profile prod up -d --build   # nginx image built from web/Dockerfile
```

Add `--profile observability` (alongside `dev` or `prod`) for the OTel
Collector + Jaeger.

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

[pre-commit](https://pre-commit.com) runs formatters, linters, and secret scanners before each commit; the same hooks run in CI (the **Lint** workflow is exactly `pre-commit run --all-files`). Install with `pre-commit install`; see [`.pre-commit-config.yaml`](.pre-commit-config.yaml) for the hooks and the tools they shell out to.

### Continuous integration & releases

Five workflows live in [`.github/workflows/`](.github/workflows/), each with a single responsibility and a manual `workflow_dispatch` trigger:

| Workflow | Triggers | Does |
| --- | --- | --- |
| **Lint** ([`lint.yml`](.github/workflows/lint.yml)) | every PR + manual | runs the full pre-commit hook set (gitleaks, gofmt/vet/golangci-lint, eslint, tsc, helm lint, hadolint, actionlint, …) |
| **Quality** ([`quality.yml`](.github/workflows/quality.yml)) | every PR + manual | Go build + unit/integration tests + coverage floor, OpenAPI↔router & A2A contract suites, web build + unit tests, Helm render + kubeconform, Playwright + k6 e2e |
| **Docker** ([`docker.yml`](.github/workflows/docker.yml)) | every PR + manual (build only); push to `main` and `v*.*.*` tags (build + push + Trivy scan) | multi-arch server/web images to GHCR — `:latest`/`:main-<sha>` on `main`, `:<semver>` on a `v*` tag |
| **Helm publish** ([`helm-publish.yml`](.github/workflows/helm-publish.yml)) | push to `main` (dev version) and `chart-*` tags (release) | packages and pushes the chart to GHCR as an OCI artifact |
| **Release** ([`release.yml`](.github/workflows/release.yml)) | after a successful **Docker** run for a `v*` tag | cuts the GitHub Release with notes from the matching `CHANGELOG.md` section |

Tag conventions, decoupled so app images and the chart version independently:

- **`v1.2.3`** — publishes the server/web images at that version, then cuts the GitHub Release.
- **`chart-1.2.3`** — publishes the Helm chart at that version (its `appVersion` is whatever is committed in `Chart.yaml`).

---

## Roadmap

The phased roadmap lives in [`PLAN.md`](./PLAN.md). Shipped: everything through publisher-scoped RBAC, local accounts, and the brokered-OIDC / registry-issued bearer-token (access + rotating refresh) rework (with the `/v0` surface removed). Also shipped: opt-in M2M auth via directly-presented IdP service-account tokens (`OIDC_AUDIENCE`). Remaining (tracked in `PLAN.md`): registry-native hashed per-publisher API keys, a Skills / Prompts registry, federation, and webhooks.

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
