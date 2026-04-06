# CLAUDE.md

Guidance for Claude Code when working in this repository.

## Project: AI Registry

A centralized registry for AI ecosystem artifacts:

- **MCP Registry** — catalog of Model Context Protocol servers (exposed via an
  MCP-spec-compatible API).
- **Agent Registry** — catalog of AI agents, each publishing an A2A-compatible
  Agent Card.
- **(Planned)** Skills / Prompts registry.

### Core principles (non-negotiable)

1. **API-first.** Every capability is exposed via a versioned HTTP API. UIs are
   only clients of this API. No feature lives in the UI that is not also in the
   API.
2. **Two UIs, one API.**
   - *User UI*: read-only. Browse/search/view entries. No mutations.
   - *Admin UI*: full CRUD. Only authenticated admins can mutate.
3. **All writes go through admins.** Creation, update, publishing, and deletion
   of any registry entry is restricted to admin principals (via UI or API).
   Non-admins get 403 on any write endpoint.
4. **Spec compatibility.**
   - MCP endpoints MUST conform to the MCP specification
     (https://modelcontextprotocol.io/). Authentication MUST follow the MCP
     authorization spec (OAuth 2.1 / OIDC with PKCE, resource indicators,
     protected resource metadata).
   - Every agent MUST generate a Google A2A-compatible Agent Card
     (`/.well-known/agent-card.json` shape) from its stored metadata.

## Tech stack

- **Backend**: Go, `chi` router, PostgreSQL, `sqlc` or `pgx` for DB access,
  `golang-migrate` for schema migrations.
- **Auth**: OAuth2 / OIDC (external IdP, Keycloak in dev via docker-compose).
  JWT access tokens validated via JWKS. MCP-compatible. Also supports hashed
  API keys for machine-to-machine admin operations.
- **Frontend**: Next.js (App Router) + TypeScript + shadcn/ui + Tailwind.
  One Next.js app with a public section and an `/admin` section guarded by
  OIDC (NextAuth / Auth.js).
- **OpenAPI**: hand-written OpenAPI 3.1 spec is the source of truth; server
  types and TS client are generated from it.
- **Dev infra**: docker-compose for Postgres + Keycloak + backend + web.
- **Deployment**: docker-compose (dev + prod profiles) + Helm chart for k8s.

## Repository layout (target)

```
/api/                 # OpenAPI 3.1 spec (source of truth)
/backend/             # Go service
  /cmd/server/        # entrypoint
  /internal/
    /http/            # chi router, handlers, middleware (auth, logging)
    /mcp/             # MCP registry endpoints + MCP protocol surface
    /agents/          # Agent registry + A2A card generation
    /auth/            # OIDC/JWT validation, scopes, admin guard
    /store/           # Postgres repositories
    /domain/          # entities, validation
  /migrations/        # SQL migrations
/web/                 # Next.js app (user + admin UI)
/deploy/              # docker-compose, env examples
/deploy/helm/         # Helm chart for k8s
/docs/                # architecture notes, ADRs
PLAN.md               # phased implementation plan
CLAUDE.md             # this file
```

## Conventions

- **Branching**: feature work on `claude/ai-registry-setup-KMC3l` for initial
  setup; subsequent features on descriptive branches. Never push to `main`
  without explicit request.
- **Commits**: conventional commits (`feat:`, `fix:`, `docs:`, `chore:`).
- **DB**: every schema change is a forward + down migration. No ORM magic;
  explicit SQL.
- **Errors**: API errors follow RFC 7807 (`application/problem+json`).
- **IDs**: ULIDs for primary keys exposed via API; internal bigserial allowed.
- **Versioning**: registry entries are versioned (semver). A publish creates
  an immutable version row; metadata edits on a version are forbidden after
  publish.
- **Testing**: table-driven tests in Go; integration tests use a real Postgres
  via docker-compose or testcontainers. Web uses Playwright for e2e on the
  admin flows.

## Security rules

- Admin-only endpoints are enforced by middleware checking the `registry:admin`
  scope / role claim on the JWT. Do not rely on the UI alone.
- All write endpoints require a valid bearer token; read endpoints are public
  by default (configurable).
- CORS: admin UI origin and user UI origin allow-listed via env.
- Rate limit unauthenticated reads.
- Never log tokens or full Authorization headers.

## How to work in this repo (for Claude)

1. Read `PLAN.md` before starting any task — it defines the phased roadmap.
2. Prefer editing existing files over creating new ones.
3. When touching the API, update `/api/openapi.yaml` first, then regenerate
   types, then implement the handler.
4. Keep MCP and A2A compatibility: when in doubt, link to the relevant spec
   section in the PR description.
5. Do not add features outside the current phase without asking.

## References

- MCP specification: https://modelcontextprotocol.io/
- MCP registry (reference impl): https://github.com/modelcontextprotocol/registry
- A2A protocol / Agent Card: https://a2a-protocol.org/
- OAuth 2.1 draft: https://datatracker.ietf.org/doc/draft-ietf-oauth-v2-1/
