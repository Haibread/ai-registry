# AI Registry — Implementation Plan

Forward-looking roadmap for the API-first MCP + Agent registry with a public
user UI and an admin UI. See `CLAUDE.md` for conventions and constraints.

## 1. Goals & non-goals

**Goals**

- Single source of truth for internal/public MCP servers and AI agents.
- Catalog MCP servers and expose them via the registry's own `/api/v1` API.
- Generate A2A-compatible Agent Cards for every registered agent.
- Provide a public read-only UI and an admin-only CRUD UI.
- Be API-first: every UI action maps 1:1 to an API call.

**Non-goals (for now)**

- Hosting/executing MCP servers or agents, or proxying calls to them.
- Billing, quotas, multi-tenant isolation.
- Skills/Prompts registry (reserved for a later phase).

## 2. Domain model

Entities and their fields: see `server/internal/domain/` and
`server/migrations/`. The roles and the relationships that the code does **not**
make obvious:

- `Publisher` — org/team that owns entries; the unit RBAC is scoped to.
- `User` / `Group` / `RoleGrant` — principals and publisher-scoped RBAC; roles
  (Viewer/Editor/Reviewer/Admin) are granted to users or groups per publisher.
  Server Admin comes from the `realm_access.roles` claim or a local
  `is_server_admin` flag.
- `MCPServer` → `MCPServerVersion` (1-to-many) and `Agent` → `AgentVersion`
  (1-to-many). The parent holds identity + `visibility`/`status`; versions hold
  the per-release payload. New entries default to `private`; an Editor/Admin can
  flip to `public` only after an approved (published) version exists.
- **Versions are immutable once published** — metadata edits are forbidden after
  publish; a new publish creates a new version row. Consumers cache cards, so
  silent mutation is disallowed by design.
- **Agent Card** = projection of an `Agent` plus its latest published
  `AgentVersion` into the A2A `AgentCard` schema (a2aproject/a2a June 2025 shape),
  served per-agent and as a global card. The registry is a first-class A2A citizen.

## 3. API surface

API surface: see `server/api/openapi.yaml` (the OpenAPI 3.1 source of truth,
served live at `/openapi.yaml`). Conceptually it splits three ways: **public**
read-only endpoints expose only `public` entries; **admin** mutations are gated
by publisher-scoped RBAC (or Server Admin); **system** endpoints cover health,
metrics, and the spec/docs. Errors use `application/problem+json`.

## 4. Authentication & authorization

- **Single token authority.** The registry issues a session behind a `Secure;
  HttpOnly` cookie (BFF model); the SPA holds no token.
- **OIDC brokered server-side**: the registry is one confidential client.
  `GET /api/v1/auth/oidc/login` runs Authorization Code + PKCE; `…/callback`
  exchanges the code with the `client_secret`, validates the `id_token` against the
  JWKS, maps the identity to a `users` row, and snapshots claim group membership +
  the claim Server-Admin flag into the session. Sign-out also ends the IdP session
  (RP-initiated logout). The IdP token never reaches the browser.
- **Local accounts**: email + password login (`POST /api/v1/auth/login`) sets the
  same session cookie. Bootstrap admin seeded from config.
- **Authorization** is publisher-scoped RBAC: writes require Editor/Reviewer/Admin
  on the owning publisher, or Server Admin. Reviewer is the sole approver (Admin can
  do everything except approve; Server Admin is the break-glass exception). Claims
  carry group membership only.
- Public GETs are unauthenticated by default (feature flag to require auth) and
  rate-limited. CSRF via SameSite + double-submit token.

## 5. Shipped so far

All phases through the brokered-OIDC/session rework are shipped:

- **Backend skeleton** — Go + chi, config (env/YAML/default), structured logging,
  OTel; `/healthz`/`/readyz`/`/metrics`/`/openapi.yaml`; Postgres + `golang-migrate`;
  Docker + docker-compose.
- **MCP registry** — `mcp_servers` / `mcp_server_versions`, CRUD + public reads,
  typed `tools[]` field, structural JSONB validation, testcontainers integration
  tests.
- **Agent registry + A2A cards** — `agents` / `agent_versions`, per-agent and global
  Agent Cards (a2a June 2025 shape), skills/auth-scheme validation.
- **Web app** — Vite + React Router v7 + TanStack Query v5 + shadcn/ui + Tailwind SPA
  served by nginx; public browse (search, filters, cursor pagination, namespace
  landing pages, per-entry activity feed) + role-aware admin CRUD with a
  publisher-scoped admin home (Overview, Members, Activity, Settings, switcher).
- **Hardening** — rate limiting, CORS, audit log, full-text search, Helm chart, E2E
  (Playwright), handler-level write-path coverage, OpenAPI↔router contract test, OTel
  span-emission tests, CI lint gate + pre-commit hooks.
- **Access control & change-approval** — `review_state` + `revision` workflow
  (submit → pending → approve/reject), discriminated 409 error model, review queue,
  pending-deletion flow.
- **Publisher-scoped RBAC + local accounts** — `users`/`groups`/`group_members`/
  `role_grants`; Editor authors, Reviewer is sole approver, going public needs an
  approved version; OIDC JIT-provisioning alongside local password accounts;
  bootstrap can seed groups + grants. (The earlier workspace layer was removed.)
- **Brokered OIDC + registry cookie sessions** — confidential-client OIDC broker,
  `sessions` table + HttpOnly cookie, single-issuer auth; the SPA dropped
  `oidc-client-ts`. The MCP-registry-spec `/v0` surface and the OAuth resource-server
  role were removed; MCP servers are exposed only via `/api/v1`.

See `CHANGELOG.md` for per-release detail.

## 6. Later

Not-yet-done future work, in no committed order:

- **API-key (M2M) auth** — hashed per-publisher API keys for machine-to-machine admin
  ops (CI/CD publish pipelines), checked via `Authorization: Bearer apikey_…`.
  Includes the real `/admin/api-keys` management page (currently a placeholder) and
  the `POST/DELETE /api/v1/api-keys` endpoints.
- **Production docker-compose profile** — a dedicated `docker-compose.prod.yml` for
  self-hosted single-host installs.
- **Skills & Prompts registry** — same pattern as MCP servers.
- **Signed publishes** — sigstore/cosign.
- **Webhooks** on publish events.
- **Federation** with the public MCP registry.
- **Multi-environment entries** — dev/staging/prod per entry, each with its own
  URL/transport/auth/version pin. Design note in `docs/future-multi-environment.md`;
  do not implement until revisited.
- **Review-workflow extensions** — forbid self-approval, notifications, SLA timers,
  bulk approval, reviewer comments, revision diff view in the admin UI.

## 7. Resolved decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Namespacing | Publisher-scoped: `{namespace}/{slug}` |
| 2 | Private entries | `visibility` field (`private`/`public`); new entries default `private`. Going public requires an approved (published) version. Public GETs return only `public` entries. |
| 3 | IdP for dev | Keycloak via docker-compose |
| 4 | Deployment target | Docker Compose **and** Helm chart for k8s |
| 5 | Identity & roles | *Authentication* via brokered OIDC **and** local password accounts, both behind a registry session cookie. *Authorization* is registry-managed publisher-scoped RBAC (`users`, `groups`, `role_grants`); Reviewer is the sole approver. Server Admin from `realm_access.roles` or a local `is_server_admin` flag. |
| 6 | UI template | shadcn/ui primitives (minimal) — no third-party admin template |
| 7 | MCP wire format | The MCP-registry-spec `/v0` surface was removed; MCP servers are exposed only via `/api/v1`. |

## 8. Definition of done (per phase)

- OpenAPI updated and served at `/openapi.yaml`.
- Migrations run cleanly up and down.
- Unit + integration tests pass in CI.
- Authorization guard enforced on every mutating endpoint (verified by test).
- Docs: README section per new capability; a design note if a cross-cutting
  decision was made.
