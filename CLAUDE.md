# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project

AI Registry — a centralized catalog for AI ecosystem artifacts (MCP servers,
A2A agents, more planned), exposed via a versioned HTTP API with two SPA
clients (read-only user UI, authenticated admin UI).

## Tech stack

- **Server**: Go, `chi` router, PostgreSQL, `pgx` (no ORM, hand-written SQL),
  `golang-migrate` for migrations.
- **Frontend**: Vite + React Router v7 + TanStack Query v5 + TypeScript +
  shadcn/ui + Tailwind. Pure SPA served as static files by nginx. ESLint for
  style/correctness; `tsc -b` (via `npm run build`) is the type gate.
- **Auth**: registry-issued Ed25519 JWT access token + rotating refresh token,
  returned in the response body and sent as `Authorization: Bearer` (no
  cookie). Interactive login is either local email+password or server-brokered
  OIDC. Authorization is publisher-scoped RBAC enforced in server middleware.
- **OpenAPI**: hand-written OpenAPI 3.1 spec is the source of truth; server
  types and the TS client are generated from it.
- **Observability**: OpenTelemetry for traces, metrics, and logs, exported via
  OTLP. Handlers are traced; DB calls produce child spans; structured logs
  carry `trace_id` / `span_id`.
- **Dev infra**: a single root `docker-compose.yml` with `dev` (Vite hot
  reload) and `prod` (nginx built from `web/Dockerfile`) profiles, plus an
  `observability` profile. CI runs no compose: Postgres and Keycloak are
  GitHub Actions service containers; the server and web SPA run as native host
  processes. Helm chart for k8s under `deploy/helm/`.

## Repository layout

- `server/` — Go service. `internal/{http,mcp,agents,auth,bootstrap,config,domain,store,observability,problem}`,
  `api/` (embedded OpenAPI spec), `migrations/`, `cmd/`.
- `web/` — the SPA (pages in `src/pages/`).
- `deploy/` — ops (compose files, Helm, example configs).
- `docs/`, `design.md` — design notes.

## Non-negotiables

1. **API-first.** Every capability lives in the versioned HTTP API; UIs are
   only clients. No feature in the UI that isn't in the API.
2. **OpenAPI stays in sync.** When you add or change a route, update
   `server/api/openapi.yaml` in the same change, then regenerate types.
3. **Tests for everything.** Table-driven unit tests for business logic;
   integration tests (testcontainers Postgres) for handlers and repositories;
   Playwright e2e for admin flows. No PR without coverage for changed code.
4. **Instrument with OTel.** New handlers get a span; new metrics are
   registered under `server/internal/observability/`. Use the tracer/meter
   from context — never create ad-hoc providers.
5. **Writes are authorized server-side.** Write endpoints require a valid
   bearer token and the right role on the owning publisher (or Server Admin);
   never rely on the UI alone. Reads are public by default (configurable).

## Conventions

- **Branching**: feature branches (`feat/`, `fix/`, `docs/`, `chore/<topic>`).
  Never push to `main` without an explicit request.
- **Commits**: conventional commits.
- **DB**: forward-only migrations; explicit SQL, no ORM. Down migrations are
  for local convenience only.
- **Errors**: RFC 7807 (`application/problem+json`).
- **IDs**: ULIDs for API-exposed primary keys; internal bigserial allowed.
- **Versioning**: registry entries are semver-versioned; a publish creates an
  immutable version row (no metadata edits after publish).
- **Never bump app/chart versions** unless explicitly asked.

## Configuration

Every config value MUST be settable three ways, highest precedence wins:

1. **Env var** — `UPPER_SNAKE_CASE` (e.g. `DATABASE_URL`), no app-name prefix.
2. **Config file key** — `lower_snake_case` in a YAML file (`CONFIG_FILE` env
   or `--config` flag).
3. **Built-in default** — in `server/internal/config/config.go`.

A new value goes in all three places and is documented in
`deploy/config.example.yaml`. Sensitive values (passwords, DSNs, tokens) come
from env/secrets, not a committed file.

## Security

- Never log tokens or full `Authorization` headers.
- CORS origins allow-listed via env; rate-limit unauthenticated reads.
- No CSRF middleware by design — credentials live in the `Authorization`
  header, never an ambient cookie.

## How to work here

1. API change → update `openapi.yaml` first, regenerate types, then implement.
2. Don't add features outside the current scope without asking.

## References

- MCP: https://modelcontextprotocol.io/
- A2A / Agent Card: https://a2a-protocol.org/
