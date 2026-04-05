# AI Registry — Implementation Plan

Phased roadmap for building an API-first MCP + Agent registry with a user UI
and an admin UI. See `CLAUDE.md` for conventions and constraints.

## 1. Goals & non-goals

**Goals**

- Serve as the single source of truth for internal/public MCP servers and
  AI agents.
- Expose an MCP-spec-compatible registry API.
- Generate A2A-compatible Agent Cards for every registered agent.
- Provide a public read-only UI and an admin-only CRUD UI.
- Be API-first: every UI action maps 1:1 to an API call.

**Non-goals (for now)**

- Hosting/executing MCP servers or agents.
- Proxying calls to MCP servers.
- Billing, quotas, multi-tenant isolation.
- Skills/Prompts registry (reserved for a later phase).

## 2. Domain model

### 2.1 Common

- `Publisher` — org/team owning an entry. `{id, slug, name, contact, verified}`.
- `User` — principal (from OIDC). `{subject, email, roles[]}`.
  Role set: `viewer` (implicit, public), `admin`.

### 2.2 MCP Registry

- `MCPServer`
  - `id` (ULID), `namespace` (publisher slug), `name`, `slug`
  - `description`, `homepage_url`, `repository_url`, `license`
  - `status` (`draft` | `published` | `deprecated`)
  - `created_at`, `updated_at`
- `MCPServerVersion`
  - `id`, `server_id`, `version` (semver), `released_at`
  - `runtime` (`stdio` | `http` | `sse` | `streamable_http`)
  - `install` (JSON: package manager, command, args, env schema)
  - `capabilities` (JSON: tools[], resources[], prompts[] summaries)
  - `protocol_version` (MCP spec version supported)
  - `checksum`, `signature` (optional)
- Immutable once published; new publishes create new versions.

### 2.3 Agent Registry

- `Agent`
  - `id`, `namespace`, `name`, `slug`, `description`
  - `status`, `created_at`, `updated_at`
- `AgentVersion`
  - `id`, `agent_id`, `version`, `released_at`
  - `endpoint_url` (A2A base URL)
  - `skills` (JSON array, A2A skill objects)
  - `capabilities` (JSON: streaming, pushNotifications, stateTransitionHistory)
  - `authentication` (JSON: supported schemes)
  - `default_input_modes`, `default_output_modes`
  - `provider` (JSON: organization, url)
  - `documentation_url`, `icon_url`
  - `protocol_version` (A2A version)
- Agent Card = projection of `Agent` + latest published `AgentVersion` into
  the A2A `AgentCard` JSON schema, served at
  `/agents/{namespace}/{slug}/.well-known/agent-card.json`.

## 3. API surface (OpenAPI 3.1)

All endpoints under `/api/v1` unless noted. Responses use `application/json`;
errors use `application/problem+json`.

### 3.1 Public (read-only)

- `GET /api/v1/mcp/servers` — list, filter by `namespace`, `q`, `tag`.
- `GET /api/v1/mcp/servers/{ns}/{slug}` — server detail + latest version.
- `GET /api/v1/mcp/servers/{ns}/{slug}/versions` — list versions.
- `GET /api/v1/mcp/servers/{ns}/{slug}/versions/{version}` — specific version.
- `GET /api/v1/agents` — list.
- `GET /api/v1/agents/{ns}/{slug}` — agent detail.
- `GET /api/v1/agents/{ns}/{slug}/versions` / `/{version}`.
- `GET /agents/{ns}/{slug}/.well-known/agent-card.json` — A2A Agent Card.
- `GET /.well-known/oauth-protected-resource` — MCP-mandated resource metadata.

### 3.2 MCP-spec registry endpoints

Mirror the MCP registry API shape
(https://github.com/modelcontextprotocol/registry):

- `GET /v0/servers` — MCP registry discovery, cursor-paginated.
- `GET /v0/servers/{id}` — canonical MCP server record.
- `POST /v0/publish` — admin only, publish/update a server version.

These are a thin compatibility layer over `/api/v1/mcp/*`.

### 3.3 Admin (JWT with `registry:admin` scope)

- Publishers: `POST/PATCH/DELETE /api/v1/publishers[...]`.
- MCP: `POST /api/v1/mcp/servers`, `PATCH /{ns}/{slug}`,
  `POST /{ns}/{slug}/versions`, `POST /{ns}/{slug}/versions/{v}:publish`,
  `POST /{ns}/{slug}:deprecate`.
- Agents: symmetric endpoints.
- Users & roles: `GET/PATCH /api/v1/users`.

### 3.4 System

- `GET /healthz`, `GET /readyz`, `GET /metrics` (Prometheus).
- `GET /openapi.yaml`, `GET /docs` (Swagger UI / Scalar).

## 4. Authentication & authorization

- External IdP (Keycloak in dev). Backend validates JWTs via JWKS.
- Token claims required for admin writes: `scope` includes `registry:admin`
  OR `roles` contains `admin`.
- **MCP-compatibility**: implement the MCP authorization spec
  - Serve `/.well-known/oauth-protected-resource` advertising the IdP as
    authorization server.
  - Accept `resource` parameter per RFC 8707.
  - Require PKCE on any OAuth flow we initiate.
- Admin UI uses Auth.js (NextAuth) with the same IdP; session stores the
  access token used for API calls.
- Public GETs are unauthenticated by default; feature flag to require auth.

## 5. Phased delivery

### Phase 0 — Repo scaffolding (this PR: docs only)
- `CLAUDE.md`, `PLAN.md`. No code.

### Phase 1 — Backend skeleton
- Go module, chi server, config via env, structured logging (zerolog/slog).
- `/healthz`, `/readyz`, `/metrics`, `/openapi.yaml` serving.
- Initial OpenAPI 3.1 stub.
- Postgres + migrations + first tables (`publishers`, `users`).
- Dockerfile + docker-compose (postgres, keycloak, backend).

### Phase 2 — MCP registry MVP
- Schema: `mcp_servers`, `mcp_server_versions`.
- CRUD handlers (admin-guarded) + public read endpoints.
- MCP-compat layer: `/v0/servers`, `/v0/servers/{id}`, `/v0/publish`.
- JWT middleware + scope check.
- Validation of `install`/`capabilities` against JSON schema.
- Table-driven + integration tests.

### Phase 3 — Agent registry + A2A cards
- Schema: `agents`, `agent_versions`.
- CRUD + public reads.
- Agent Card generator → `/.well-known/agent-card.json` per agent,
  validated against the A2A JSON schema.
- Tests: card conforms to A2A schema for every fixture.

### Phase 4 — Web app (Next.js)
- Next.js App Router + shadcn/ui + Tailwind.
- Public routes: `/`, `/mcp`, `/mcp/[ns]/[slug]`, `/agents`,
  `/agents/[ns]/[slug]`.
- Admin routes: `/admin/*` guarded by Auth.js (OIDC).
- Forms for publisher / MCP server / agent CRUD.
- Generated TS API client from OpenAPI.

### Phase 5 — Hardening
- Rate limiting, CORS, audit log table (`who did what when`).
- Pagination cursors, full-text search (Postgres `tsvector`).
- E2E tests (Playwright) for admin flows.
- Deployment manifests (compose prod profile; optional k8s later).

### Phase 6 — Later
- Skills & Prompts registry (same pattern as MCP servers).
- Signed publishes (sigstore/cosign).
- Webhooks on publish events.
- Federation with the public MCP registry.

## 6. Open questions (to confirm before Phase 1)

1. Namespacing: single global namespace, or one publisher per entry (chosen
   default: publisher-scoped, `{namespace}/{slug}`)?
2. Who is allowed to read private entries — is "private" a status we need
   from day one, or can everything public be public?
3. IdP choice for dev: Keycloak (assumed) vs Dex vs Ory Hydra.
4. Deployment target (compose only? k8s manifests needed?).
5. Do we need API-key auth alongside OIDC for machine-to-machine admin
   publishes, or is a service-account OIDC flow sufficient?

## 7. Definition of done (per phase)

- OpenAPI updated and served at `/openapi.yaml`.
- Migrations run cleanly up and down.
- Unit + integration tests pass in CI.
- Admin guard enforced on every mutating endpoint (verified by test).
- Docs: README section per new capability; ADR if a cross-cutting decision
  was made.
