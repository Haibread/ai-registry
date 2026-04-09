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
  - `visibility` (`private` | `public`) — new entries default to `private`;
    an admin must explicitly set `public` after validation/security review
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
  - `visibility` (`private` | `public`) — same gating as MCP servers
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

### 3.1 Public (read-only, `visibility=public` entries only)

- `GET /api/v1/mcp/servers` — list, filter by `namespace`, `q`, `tag`.
- `GET /api/v1/mcp/servers/{ns}/{slug}` — server detail + latest version.
- `GET /api/v1/mcp/servers/{ns}/{slug}/versions` — list versions.
- `GET /api/v1/mcp/servers/{ns}/{slug}/versions/{version}` — specific version.
- `GET /api/v1/agents` — list.
- `GET /api/v1/agents/{ns}/{slug}` — agent detail.
- `GET /api/v1/agents/{ns}/{slug}/versions` / `/{version}`.

Private entries are hidden from public GETs; admins see all entries via
the admin endpoints.
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
- Visibility: `POST /{ns}/{slug}:set-visibility` (toggle `private`/`public`).
- API keys: `POST/DELETE /api/v1/api-keys` — manage per-publisher API keys.
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
- Admin UI uses `oidc-client-ts` (PKCE public client) with the same IdP;
  access token stored in React context and passed as Bearer on API calls.
- Public GETs are unauthenticated by default; feature flag to require auth.
- **API-key auth**: alongside OIDC, support static API keys for
  machine-to-machine admin operations (CI/CD publish pipelines). API keys are
  stored hashed in Postgres, scoped per publisher, and checked via
  `Authorization: Bearer apikey_...` header. The middleware tries JWT first,
  falls back to API-key lookup.

## 5. Phased delivery

### Phase 0 — Repo scaffolding (this PR: docs only)
- `CLAUDE.md`, `PLAN.md`. No code.

### Phase 1 — Backend skeleton
- Go module, chi server, config via env, structured logging (zerolog/slog).
- `/healthz`, `/readyz`, `/metrics`, `/openapi.yaml` serving.
- Initial OpenAPI 3.1 stub.
- Postgres + migrations + first tables (`publishers`, `users`).
- Dockerfile + docker-compose (postgres, keycloak, server).

### Phase 2 — MCP registry MVP ✅
- Schema: `mcp_servers`, `mcp_server_versions`.
- CRUD handlers (admin-guarded) + public read endpoints.
- MCP-compat layer: `/v0/servers`, `/v0/servers/{id}`, `/v0/publish` — strict MCP registry wire format.
- JWT middleware: Keycloak JWKS, checks `realm_access.roles[]` contains `"admin"`.
- `packages` JSONB validation: structural check (registryType, identifier, version, transport.type required).
- `capabilities` JSONB: free-form valid JSON; strict schema deferred.
- Integration tests use testcontainers-go (postgres module, snapshot isolation); no external deps needed.
- `/.well-known/oauth-protected-resource` endpoint (MCP auth spec).

### Phase 3 — Agent registry + A2A cards ✅
- Schema: `agents`, `agent_versions`.
- CRUD + public reads. Same draft→published→deprecated lifecycle as MCP servers.
- Agent Card generator (`internal/agents/card.go`) targets `a2aproject/a2a` June 2025 spec.
- Per-agent card at `/agents/{ns}/{slug}/.well-known/agent-card.json`.
- Global registry card at `/.well-known/agent-card.json`.
- `skills[]` structural validation: `id`, `name`, `description`, `tags[]` required.
- `authentication` scheme allowlist: Bearer, ApiKey, OAuth2, OpenIdConnect.
- Integration tests (testcontainers, shared container) + unit tests for card generation and validation.

### Phase 4 — Web app (Vite + React SPA) ✅
- Vite + React Router v7 + TanStack Query v5 + shadcn/ui + Tailwind.
  Build from shadcn/ui primitives: sidebar layout, data tables, cards, forms.
  No third-party admin template — keep it lean and fully controlled.
- Public routes: `/`, `/mcp`, `/mcp/:ns/:slug`, `/agents`,
  `/agents/:ns/:slug`. Clean card-grid layout with search/filter bar.
- Admin routes: `/admin/*` guarded by `<RequireAuth>` (oidc-client-ts PKCE).
  Sidebar nav, data tables with inline actions, forms for publisher / MCP
  server / agent CRUD, visibility toggle, API-key management.
- Generated TS API client from OpenAPI (openapi-typescript + openapi-fetch).
- Note: originally planned as Next.js; migrated to Vite SPA in Phase 6.

**TODO — Backend (missing endpoints):**
- [ ] `PATCH /api/v1/mcp/servers/{ns}/{slug}` — edit MCP server metadata
- [ ] `DELETE /api/v1/mcp/servers/{ns}/{slug}` — delete MCP server
- [ ] `PATCH /api/v1/agents/{ns}/{slug}` — edit agent metadata
- [ ] `DELETE /api/v1/agents/{ns}/{slug}` — delete agent
- [ ] `PATCH /api/v1/publishers/{slug}` — edit publisher
- [ ] `DELETE /api/v1/publishers/{slug}` — delete publisher
- [ ] `GET /api/v1/users`, `PATCH /api/v1/users/{sub}` — user & role management
- [ ] Update `/api/openapi.yaml` to reflect all current endpoints

**TODO — Admin UI (missing features):**
- [ ] Edit form for MCP servers (`/admin/mcp/[ns]/[slug]/edit`)
- [ ] Edit form for agents (`/admin/agents/[ns]/[slug]/edit`)
- [ ] Edit form for publishers (`/admin/publishers/[slug]/edit`)
- [ ] Delete actions for servers, agents, and publishers (with confirmation)
- [ ] Users & roles management page (`/admin/users`)

**TODO — Public UI (missing features):**
- [ ] Search bar wired to `?q=` on `/mcp` and `/agents` list pages
- [ ] Cursor-based pagination controls on list pages

### Phase 5 — Hardening
- Rate limiting ✅, CORS ✅, audit log ✅.
- Pagination cursors ✅, full-text search ✅ (Postgres `tsvector`).
- E2E tests (Playwright) for admin flows.
- Deployment manifests: docker-compose prod profile + Helm chart for k8s.

**TODO — Phase 5:**
- [ ] `POST /api/v1/api-keys`, `DELETE /api/v1/api-keys/{id}` — hashed API keys (per-publisher, machine-to-machine)
- [ ] API-key auth middleware (JWT-first, fallback to API-key lookup)
- [ ] Admin UI: API keys management page (`/admin/api-keys`)
- [ ] E2E tests with Playwright covering admin create / publish / deprecate flows
- [ ] Docker Compose prod profile (`deploy/docker-compose.prod.yml`)
- [ ] Helm chart (`deploy/helm/`)

### Phase 6 — Migrate web app from Next.js → Vite + React SPA ✅ COMPLETED

Migration is done. The web app is now a plain Vite + React SPA served by nginx.

Next.js is overkill: there is no SEO requirement, no static generation need, and
SSR adds complexity (hydration bugs, double fetches, Server Actions, middleware)
without meaningful benefit. The target stack is **Vite + React Router + TanStack
Query** — a plain SPA served as static files from nginx.

#### What stays the same
- All UI components (Radix UI, shadcn/ui, Tailwind CSS, Lucide)
- `openapi-fetch` / `openapi-typescript` generated client
- `next-themes` (framework-agnostic)
- All page structure and visual design

#### What changes

| Area | Before (Next.js) | After (Vite + React) |
|------|-----------------|----------------------|
| Routing | App Router file-based | React Router v7 |
| Data fetching | Server Components + `getPublicClient` | `useQuery` (TanStack Query) |
| Auth | NextAuth.js + middleware | `oidc-client-ts` + React context |
| Admin guard | `proxy.ts` middleware | `<RequireAuth>` wrapper component |
| Mutations | Server Actions | `useMutation` + `fetch` |
| Page metadata | `export const metadata` | `<title>` via React Router future flag or `react-helmet-async` |
| Dev proxy | `next.config.ts` rewrites | Vite `server.proxy` config |
| Production serving | Node.js (`next start`) | nginx static file server |
| Docker image | `node:22-alpine` + standalone Next.js | `nginx:alpine` (static files only) |

#### Step-by-step plan

**Step 1 — Scaffold** ✅
- [x] Vite + React + TypeScript project in `web/`
- [x] Tailwind CSS v4, postcss, tsconfig configured
- [x] `src/components/ui/`, `src/lib/` migrated (no Next.js deps)
- [x] Installed: `react-router-dom`, `@tanstack/react-query`, `oidc-client-ts`, `openapi-fetch`, `lucide-react`
- [x] Vite proxy for `/api/v1/*`, `/v0/*`, `/.well-known/*` → server
- [x] `openapi-typescript` regenerated `schema.d.ts`

**Step 2 — Auth** ✅
- [x] `AuthProvider` in `src/auth/AuthContext.tsx` using `oidc-client-ts` `UserManager` with PKCE
- [x] `AuthCallback` component at `/auth/callback`
- [x] `accessToken`, `isAuthenticated`, `login()`, `logout()` exposed via context
- [x] `<RequireAuth>` component redirects to Keycloak if not authenticated
- [x] `automaticSilentRenew: true` for refresh
- [x] `AUTH_KEYCLOAK_SECRET` removed — public OIDC client, no secret needed

**Step 3 — API client** ✅
- [x] Single `useApiClient()` hook (public: no headers; authed: Bearer token)
- [x] All admin pages use `useApiClient()` + `useQuery` / `useMutation`
- [x] Server Actions replaced with `useMutation` + `fetch`

**Step 4 — Routing** ✅
- [x] React Router v7 `createBrowserRouter` in `src/main.tsx`
- [x] All routes defined (public, admin, auth callback)

**Step 5 — Convert pages** ✅
- [x] All pages converted to client components with `useQuery`
- [x] `next/link` → `react-router-dom` `<Link to=...>`
- [x] `usePathname`/`useRouter`/`useSearchParams` → React Router equivalents
- [x] `notFound()` / `redirect()` replaced with React Router primitives

**Step 6 — Production build** ✅
- [x] `web/nginx.conf` with `try_files $uri /index.html` + server proxy blocks
- [x] `web/Dockerfile`: `node:22-alpine` build stage → `nginx:alpine` serve stage
- [x] `AUTH_SECRET`, `AUTH_KEYCLOAK_SECRET`, `NEXTAUTH_URL` removed from docker-compose
- [x] `VITE_OIDC_ISSUER`, `VITE_OIDC_CLIENT_ID` added as build args

**Step 7 — Cleanup** ✅
- [x] Old Next.js `src/app/` directory removed
- [x] `next`, `next-auth`, `next-themes` removed from `package.json`
- [x] `CLAUDE.md` updated to reflect new stack
- [x] `PLAN.md` updated (this section)

#### Environment variable changes

| Variable | Before | After |
|----------|--------|-------|
| `AUTH_SECRET` | Required | Removed |
| `AUTH_KEYCLOAK_ID` | Required | → `VITE_KEYCLOAK_CLIENT_ID` |
| `AUTH_KEYCLOAK_SECRET` | Required | **Removed** (public OIDC client) |
| `AUTH_KEYCLOAK_ISSUER` | Required | → `VITE_KEYCLOAK_ISSUER` |
| `NEXTAUTH_URL` | Required | Removed |
| `API_URL` | Build-time + runtime | Nginx config (runtime only) |

#### Key risks & mitigations

| Risk | Mitigation |
|------|-----------|
| Keycloak requires `client_secret` for the existing client | Create a new Keycloak client with `Access Type: public` — no secret needed for PKCE |
| Token refresh gaps | `oidc-client-ts` `automaticSilentRenew` + `accessTokenExpiring` event handle this |
| CORS during dev (Vite proxy vs browser) | Vite `server.proxy` routes all `/api/v1/*` through Node — no CORS headers needed in dev |
| `/.well-known/*` paths | Nginx proxy block covers them in production; Vite proxy in dev |

### Phase 7 — Later
- Skills & Prompts registry (same pattern as MCP servers).
- Signed publishes (sigstore/cosign).
- Webhooks on publish events.
- Federation with the public MCP registry.

## 6. Resolved decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Namespacing | Publisher-scoped: `{namespace}/{slug}` |
| 2 | Private entries | Yes — `visibility` field (`private`/`public`). New entries default to `private`; admin/security team must approve before setting `public`. Public GETs only return `public` entries. |
| 3 | IdP for dev | Keycloak via docker-compose |
| 4 | Deployment target | Docker Compose **and** Helm chart for k8s |
| 5 | API-key auth | Yes — support both OIDC (interactive) and hashed API keys (machine-to-machine). Middleware tries JWT first, falls back to API-key. |
| 6 | UI template | shadcn/ui blocks (minimal) — build from primitives, no third-party admin template |

## 7. Definition of done (per phase)

- OpenAPI updated and served at `/openapi.yaml`.
- Migrations run cleanly up and down.
- Unit + integration tests pass in CI.
- Admin guard enforced on every mutating endpoint (verified by test).
- Docs: README section per new capability; ADR if a cross-cutting decision
  was made.
