# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project: AI Registry

A centralized registry for AI ecosystem artifacts:

- **MCP Registry** — catalog of Model Context Protocol servers, exposed via the
  registry's own `/api/v1` API.
- **Agent Registry** — catalog of AI agents, each publishing an A2A-compatible
  Agent Card.
- **(Planned)** Skills / Prompts registry.

> **In progress (ADR 0006 amendment, 2026-06-01):** authentication is being
> reworked to a **server-side OIDC broker** (single confidential client) with
> **HttpOnly cookie sessions**, and the MCP-registry-spec **`/v0` surface is
> being removed**. The sections below describe that target; the running code
> still serves `/v0` and the old (multi-issuer, public-PKCE) auth until the
> follow-up PRs land. See the
> [Amendment](docs/adr/0006-publisher-scoped-rbac.md#amendment--2026-06-01-brokered-oidc-registry-sessions-and-removal-of-v0).

### Core principles (non-negotiable)

1. **API-first.** Every capability is exposed via a versioned HTTP API. UIs are
   only clients of this API. No feature lives in the UI that is not also in the
   API.
2. **Two UIs, one API.**
   - *User UI*: read-only. Browse/search/view entries. No mutations.
   - *Admin UI*: CRUD for authorized principals. What a principal may mutate
     is governed by their role on the owning publisher (see #3).
3. **Writes require authorization (RBAC).** Creating, updating, publishing,
   or deleting a registry entry requires the appropriate role on the owning
   publisher — **Editor** to author, **Reviewer** to approve, **Admin** to
   manage — or global **Server Admin**. Unauthorized principals get 403 on
   any write endpoint. Roles are granted to users/groups in the registry
   (ADR 0006); claims carry group membership only.
4. **A2A compatibility.** Every agent MUST generate a Google A2A-compatible
   Agent Card (`/.well-known/agent-card.json` shape) from its stored metadata.
   *(The former MCP-registry-spec conformance + MCP-authorization-spec
   requirement was dropped with the `/v0` surface — ADR 0006 amendment,
   2026-06-01. MCP server metadata still follows the MCP `server.json` field
   shapes where stored, but the registry no longer exposes an MCP protocol
   surface or acts as an OAuth resource server.)*
5. **Every API endpoint MUST be documented in `server/api/openapi.yaml`.** This is
   non-negotiable. When you add a route to the router, you MUST add the
   corresponding path + operation to the spec in the same change. The spec is
   the source of truth and is served live at `/openapi.yaml`; it MUST stay in
   sync with the implementation at all times.

## Tech stack

- **Server**: Go, `chi` router, PostgreSQL, `pgx` for DB access (no ORM,
  hand-written SQL), `golang-migrate` for schema migrations.
- **Auth**: two login front doors, both ending in a **registry-issued session
  behind a `Secure; HttpOnly` cookie** (BFF). OIDC is **brokered server-side**:
  the registry is a single **confidential** OIDC client (Keycloak in dev) — the
  browser hits `/api/v1/auth/oidc/login`, the server runs the Authorization
  Code + PKCE flow, exchanges the code with its `client_secret`, maps the
  external identity to an internal `users` row, and the IdP token never reaches
  the browser. Local email + password login sets the same session cookie. The
  registry is the **single token authority** — no multi-issuer validation and
  no MCP wall (both went away with `/v0`). Claim group membership and the
  claim-based Server-Admin flag are **snapshotted into the session at login**.
  **Authorization** is publisher-scoped RBAC: users, groups, and role grants
  live in the registry (ADR 0006); claims carry group membership only. Server
  Admin comes from the `realm_access.roles` claim **or** a local
  `is_server_admin` flag (bootstrap admin). Hashed per-publisher API keys for
  machine-to-machine ops remain planned (Decision B). The SPA is **not** an
  OIDC client — no `oidc-client-ts`, no client secret in the browser, no
  NextAuth/Auth.js.
- **Frontend**: Vite + React Router v7 + TanStack Query v5 + TypeScript +
  shadcn/ui + Tailwind. A pure SPA served as static files from nginx. Public
  section for browsing; `/admin` section guarded by a `<RequireAuth>` wrapper.
  Auth is a registry session (HttpOnly cookie): `login()` redirects to the
  server's `/api/v1/auth/oidc/login`, the local form POSTs `/api/v1/auth/login`,
  and the SPA learns its identity + grants from `GET /api/v1/me`.
  Theme switching via a local `ThemeProvider` (no next-themes). Pages live
  in `src/pages/`.
- **OpenAPI**: hand-written OpenAPI 3.1 spec is the source of truth; server
  types and TS client are generated from it.
- **Dev infra**: docker-compose for Postgres + Keycloak + server + web
  (`docker-compose.yml` baseline + `docker-compose.dev.yml` overlay for
  local development; `docker-compose.ci.yml` is CI-only).
- **Deployment**: docker-compose for self-hosted single-host installs +
  a Helm chart for k8s with optional CNPG-managed Postgres. A dedicated
  `docker-compose.prod.yml` profile is parked post-0.4.0.
- **Observability**: OpenTelemetry (OTel) for all signals — traces, metrics,
  and logs. Use the Go `go.opentelemetry.io/otel` SDK in the server; export
  via OTLP (gRPC or HTTP). Every HTTP handler must be traced; DB calls must
  produce child spans. Structured logs must carry `trace_id` and `span_id`
  fields. Key business metrics (request counts, latency histograms, registry
  entry counts) must be emitted as OTel metrics.

## Repository layout (target)

```
/server/              # Go service
  /api/               # OpenAPI 3.1 spec (source of truth, embedded into binary)
  /cmd/server/        # entrypoint
  /internal/
    /http/            # chi router, handlers, middleware (auth, logging)
    /mcp/             # MCP server registry endpoints (under /api/v1)
    /agents/          # Agent registry + A2A card generation
    /auth/            # OIDC broker, session validation, RBAC guards
    /store/           # Postgres repositories
    /domain/          # entities, validation
    /observability/   # OTel setup: tracer, meter, logger providers
  /migrations/        # SQL migrations
/web/                 # Vite + React SPA (user + admin UI; nginx in prod)
/deploy/              # docker-compose, env examples
/deploy/helm/         # Helm chart for k8s
/docs/                # architecture notes, ADRs
PLAN.md               # phased implementation plan
CLAUDE.md             # this file
```

## Conventions

- **Branching**: feature work on descriptive feature branches
  (`feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `chore/<topic>`). Never
  push to `main` without an explicit request.
- **Commits**: conventional commits (`feat:`, `fix:`, `docs:`, `chore:`).
- **DB**: schema changes are forward-only migrations. Down migrations are
  maintained for local development convenience only — never rely on them in
  production. No ORM magic; explicit SQL.
- **Errors**: API errors follow RFC 7807 (`application/problem+json`).
- **IDs**: ULIDs for primary keys exposed via API; internal bigserial allowed.
- **Versioning**: registry entries are versioned (semver). A publish creates
  an immutable version row; metadata edits on a version are forbidden after
  publish.
- **Testing**: table-driven tests in Go; integration tests use a real Postgres
  via docker-compose or testcontainers. Web uses Playwright for e2e on the
  admin flows. **Every new piece of code must have tests.** Unit tests are
  required for business logic; integration tests for handlers and DB
  repositories. Do not open a PR without test coverage for the changed code.

## Configuration (non-negotiable)

Every configuration value MUST be settable in **all three** of the following
ways, with the listed precedence (highest wins):

1. **Environment variable** — `UPPER_SNAKE_CASE` (e.g. `DATABASE_URL`).
2. **Config file key** — `lower_snake_case` in a YAML file whose path is given
   by the `CONFIG_FILE` env var or a `--config` CLI flag (e.g. `database_url`).
3. **Built-in default** — hard-coded in `server/internal/config/config.go`.

Rules for implementors:

- Adding a new config value means adding it in all three places: the env-var
  reader, the YAML key reader, and the default. Never add an env-only or
  file-only knob.
- Env var always overrides the config file; the config file always overrides
  the default. This lets operators use a base config file and override specific
  values per-environment via env vars without touching the file.
- Config file format is **YAML**. The file is optional — if absent the server
  runs on env vars and defaults alone.
- Sensitive values (passwords, tokens, DSNs) SHOULD be supplied via env var
  or a secrets manager, not committed in a config file.
- All config keys (env and file) MUST be documented in `deploy/.env.example`
  with a comment explaining the value and its default.

## Security rules

- Write endpoints are enforced by middleware checking the caller's role on the
  owning publisher (Editor/Reviewer/Admin per the action) or global Server Admin
  (`realm_access.roles` contains `admin`, or local `is_server_admin`) — ADR 0006.
  Do not rely on the UI alone.
- All write endpoints require an authenticated registry session (HttpOnly
  cookie); read endpoints are public by default (configurable). State-changing
  requests are CSRF-protected (SameSite + double-submit token).
- CORS: admin UI origin and user UI origin allow-listed via env.
- Rate limit unauthenticated reads.
- Never log tokens or full Authorization headers.

## How to work in this repo (for Claude)

1. Read `PLAN.md` before starting any task — it defines the phased roadmap.
2. Prefer editing existing files over creating new ones.
3. When touching the API, update `server/api/openapi.yaml` first, then regenerate
   types, then implement the handler.
4. Keep A2A (Agent Card) compatibility: when in doubt, link to the relevant
   A2A spec section in the PR description. (MCP-registry-spec conformance is no
   longer a target — the `/v0` surface was removed, ADR 0006 amendment.)
5. Do not add features outside the current phase without asking.
6. **Always write tests** for every function, handler, or repository method you
   create or modify. No exceptions.
7. **Instrument with OTel**: every new handler gets a span; every new metric
   (counter, histogram) is registered in `/server/internal/observability/`. Use the
   existing tracer/meter from context — never create ad-hoc providers.

## Resolved implementation decisions

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| A | Server Admin source | `realm_access.roles[]` contains `"admin"` (snapshotted into the session at login) **or** local `users.is_server_admin` (ADR 0006 + 2026-06-01 amendment) | Keycloak default shape; local flag lets the bootstrap admin run with no IdP |
| B | API-key auth | Deferred post-0.4.0 | Phase 2-5 ship JWT-only; v0.4.0 ships ADR 0006 RBAC; hashed per-publisher API keys parked for a later minor (see PLAN.md and README) |
| C | `/v0/` wire format | **Removed** (ADR 0006 amendment, 2026-06-01) | The MCP-registry-spec surface is dropped; MCP servers are exposed only via `/api/v1`. Was a strict MCP-spec shape (`{ servers: […], metadata: {…} }` list / single object detail) |
| D | Integration test infra | testcontainers-go (postgres module) with snapshot isolation | No external dependency needed to run `go test` |
| E | `packages` JSONB validation | Structural: each entry must have `registryType`, `identifier`, `version`, `transport.type` | Matches MCP server.json spec; strict schema deferred |
| F | `capabilities` JSONB validation | Free-form valid JSON only | Structure varies by server; strict validation deferred |
| G | A2A spec version | `a2aproject/a2a` June 2025 shape | Pinned to avoid chasing a moving target; documented in `internal/agents/card.go` |
| H | Agent card endpoint | Per-agent `/agents/{ns}/{slug}/.well-known/agent-card.json` + global `/.well-known/agent-card.json` | Global makes the registry a first-class A2A citizen |
| I | Agent version lifecycle | Same draft→published→deprecated state machine as MCP servers | Consumers cache agent cards; silent mutation breaks them |
| J | `skills[]` validation | Structural: `id`, `name`, `description` required strings; `tags` required string array | Skills has a defined A2A schema; enforce at write time |
| K | `authentication` schemes allowlist | `Bearer`, `ApiKey`, `OAuth2`, `OpenIdConnect` | Arbitrary schemes can't be reliably introspected; add to allowlist explicitly |
| L | Authorization model | Publisher-scoped RBAC — roles (Viewer/Editor/Reviewer/Admin) granted to users or groups; **Reviewer is the sole approver** — a publisher Admin can do everything *except* approve (Server Admin is the break-glass exception); making an entry public requires an approved (published) version (ADR 0006) | Self-managed in-registry; separation of duties by default |
| M | Workspaces | **Removed** (ADR 0006) — resources are publisher-scoped again | Workspace layer (ADR 0001/0002) didn't earn its keep |
| N | Local accounts | Local email+password login alongside brokered OIDC; both set a registry session cookie (ADR 0006 + 2026-06-01 amendment) | Run without an external IdP; bootstrap admin seeded from config |
| O | Claim → authorization | Claims carry **group membership only**; roles are grants on users/groups (ADR 0006) | Only two principal types (user, group); no claim-to-role side channel |

## References

- MCP specification: https://modelcontextprotocol.io/
- MCP registry (reference impl): https://github.com/modelcontextprotocol/registry
- A2A protocol / Agent Card: https://a2a-protocol.org/
- OAuth 2.1 draft: https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/
