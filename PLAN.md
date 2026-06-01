# AI Registry — Implementation Plan

Phased roadmap for building an API-first MCP + Agent registry with a user UI
and an admin UI. See `CLAUDE.md` for conventions and constraints.

> **In progress (ADR 0006 amendment, 2026-06-01):** auth is moving to a
> server-side OIDC broker (single confidential client) + HttpOnly cookie
> sessions, and the MCP-registry-spec **`/v0` surface is being removed**.
> Sections that describe `/v0` or the old multi-issuer / public-PKCE auth are
> historical until [Phase 9](#phase-9--brokered-oidc--registry-sessions-remove-v0-adr-0006-amendment)
> lands. See the
> [Amendment](docs/adr/0006-publisher-scoped-rbac.md#amendment--2026-06-01-brokered-oidc-registry-sessions-and-removal-of-v0).

## 1. Goals & non-goals

**Goals**

- Serve as the single source of truth for internal/public MCP servers and
  AI agents.
- Catalog MCP servers and expose them via the registry's own `/api/v1` API.
  *(Was: an MCP-spec-compatible `/v0` API — removed, ADR 0006 amendment.)*
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

### 3.2 MCP-spec registry endpoints — REMOVED (ADR 0006 amendment, 2026-06-01)

The `/v0/*` surface (the strict MCP-registry-spec wire format — `GET
/v0/servers`, `GET /v0/servers/{id}`, `POST /v0/publish`) is being **removed**.
MCP servers are catalogued and served only through `/api/v1/mcp/*` (§3.1, §3.3).
The registry no longer mirrors the MCP registry API shape or acts as an OAuth
resource server.

### 3.3 Admin (publisher-scoped RBAC or Server Admin — ADR 0006)

- Publishers: `POST/PATCH/DELETE /api/v1/publishers[...]`.
- MCP: `POST /api/v1/mcp/servers`, `PATCH /{ns}/{slug}`,
  `POST /{ns}/{slug}/versions`, `POST /{ns}/{slug}/versions/{v}:publish`,
  `POST /{ns}/{slug}:deprecate`.
- Agents: symmetric endpoints.
- Visibility: `POST /{ns}/{slug}:set-visibility` (toggle `private`/`public`).
- API keys: `POST/DELETE /api/v1/api-keys` — manage per-publisher API keys.
- Users, groups & roles: **registry-managed** (ADR 0006) — `GET/POST
  /api/v1/users`, `/api/v1/groups[...]`, per-publisher
  `/api/v1/publishers/{slug}/grants` and global `/api/v1/grants`.
  Authentication is OIDC or local password.

### 3.4 System

- `GET /healthz`, `GET /readyz`, `GET /metrics` (Prometheus).
- `GET /openapi.yaml`, `GET /docs` (Swagger UI / Scalar).

## 4. Authentication & authorization

- **Single token authority** (ADR 0006 amendment, 2026-06-01): the registry
  issues a session behind a `Secure; HttpOnly` cookie; there is no multi-issuer
  validation and no MCP wall (both went away with `/v0`). OIDC is **brokered
  server-side** — the registry is one **confidential** client that runs the
  Authorization Code + PKCE flow at `/api/v1/auth/oidc/login` →
  `/api/v1/auth/oidc/callback`, maps the IdP identity to an internal `users`
  row, and snapshots claim group membership + the claim Server-Admin flag into
  the session. Local email+password login sets the same cookie.
- **Authorization** is publisher-scoped RBAC (ADR 0006): writes require
  Editor / Reviewer / Admin on the owning publisher, or Server Admin
  (`realm_access.roles` contains `admin`, or local `is_server_admin`). Roles
  are granted to users/groups; claims carry group membership only.
- Admin UI holds **no token**: `login()` redirects to the server's
  `/api/v1/auth/oidc/login`, the local form POSTs `/api/v1/auth/login`, and the
  SPA reads its identity + grants from `GET /api/v1/me`. The session rides in an
  HttpOnly cookie (not JS-readable); no `oidc-client-ts`, no Bearer header.
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

**Backend CRUD — complete.** `PATCH` and `DELETE` for MCP servers, agents,
and publishers are all implemented (see `router.go`) and covered by
handler-level tests against a real Postgres (testcontainers).

**Admin UI CRUD — complete.** Edit and delete actions for MCP servers,
agents, and publishers are wired into the admin detail pages
(`web/src/pages/admin/{mcp,agents,publishers}/detail.tsx`) with
confirmation dialogs.

**User & role management — now registry-managed (ADR 0006):**
Superseded by ADR 0006. *Authentication* stays delegated to the IdP (OIDC)
and is joined by local email+password accounts; *authorization* is managed in
the registry via `users`, `groups`, and per-publisher `role_grants`, exposed
through `/api/v1/users`, `/api/v1/groups`, and the grants endpoints (+ admin
pages). Server Admin comes from `realm_access.roles` or a local
`is_server_admin` flag. (This reverses the original "no user table / IdP-only"
stance.)

**Public UI — complete.** Search (`?q=`), namespace/status filters, cursor-based "Load more" pagination, and empty-state handling are all implemented on both `/mcp` and `/agents` list pages.

### Phase 5 — Hardening
- Rate limiting ✅, CORS ✅, audit log ✅.
- Pagination cursors ✅, full-text search ✅ (Postgres `tsvector`).
- E2E tests (Playwright) for admin flows ✅ (`web/e2e/admin.spec.ts`,
  `admin-stats.spec.ts`, `public.spec.ts`).
- Helm chart ✅ (`deploy/helm/ai-registry/`).
- Handler-level tests for write paths ✅ — every `POST`/`PATCH`/`DELETE`
  route on publishers, MCP servers, and agents has dedicated coverage
  in `internal/http/handlers/*_test.go` (testcontainers Postgres).

**Parked from Phase 5 (deferred beyond 0.4.0):**

These items were originally listed as Phase 5 TODOs but were not
attempted before Phase 5 closed. v0.4.0 ships the ADR 0006
authorization epic (publisher-scoped RBAC + local accounts + the
publisher-scoped admin home), so the items below now slip to a later
minor (post-0.4.0; see [README — Roadmap](README.md) and CLAUDE.md
Decision B):

- `POST /api/v1/api-keys`, `DELETE /api/v1/api-keys/{id}` — hashed API
  keys (per-publisher, machine-to-machine).
- API-key auth middleware (JWT-first, fallback to API-key lookup).
- Admin UI: real API-keys management page (the `/admin/api-keys`
  route ships as a placeholder today; the test file has a single
  `it.skip` waiting on the endpoints).
- Dedicated `deploy/docker-compose.prod.yml` profile for self-hosted
  production single-host installs.

### Phase 6 — Migrate web app from Next.js → Vite + React SPA ✅ COMPLETED

Migration is done. The web app is now a plain Vite + React SPA served by nginx.

Next.js is overkill: there is no SEO requirement, no static generation need, and
SSR adds complexity (hydration bugs, double fetches, Server Actions, middleware)
without meaningful benefit. The target stack is **Vite + React Router + TanStack
Query** — a plain SPA served as static files from nginx.

#### What stays the same
- All UI components (Radix UI, shadcn/ui, Tailwind CSS, Lucide)
- `openapi-fetch` / `openapi-typescript` generated client
- All page structure and visual design

(`next-themes` was originally listed here but was dropped along with
the rest of the Next.js stack — theme switching now lives in a local
`ThemeProvider` (`web/src/components/providers.tsx`).)

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

### v0.2.2 — Coverage depth ✅ SHIPPED

v0.2.1 backfilled the obvious surface-level gaps. v0.2.2 pushed deeper
into the test pyramid where v0.2.1 only scratched the surface. Scope was
test-only — no shipping features in this release unless they fell out
of fixing a bug surfaced by the new tests. See CHANGELOG `v0.2.2` for
the full coverage / CI / performance summary.

**What landed**

- *Web admin depth.* Interactive coverage on `admin/mcp/detail.tsx` and
  `admin/agents/detail.tsx` (per-version publish, deprecation,
  edit-in-place, status transitions, lifecycle stepper). Shared
  shadcn/Radix Select jsdom shims (`hasPointerCapture`,
  `releasePointerCapture`, `scrollIntoView`) extracted into
  `web/src/test/setup.ts`. OIDC token-refresh / expired-session paths
  in `AuthContext` (`accessTokenExpiring`, silent-renew failure,
  logout-on-401).
- *Server protocol & spec conformance.* Migration forward-and-idempotent
  apply tests against a fresh testcontainers Postgres. `/v0/` MCP
  wire-format conformance suite. A2A Agent Card schema conformance
  against the pinned a2a-project June 2025 schema (decision G).
  `openapi.yaml` ↔ router contract test (bijection enforced via
  `chi.Walk`). Router-level test for `PublicRateLimitRPM` wiring.
- *Server write paths.* Error-branch coverage on every `POST` / `PATCH`
  / `DELETE` handler (RFC 7807 shapes, 409 conflicts, 422 validation,
  403 admin-guard short-circuits).

**Carried forward**

- *OTel span emission tests.* v0.2.2 landed a focused 4-route span
  smoke test (`router_otel_test.go`); the broader "every handler"
  coverage came later in PR #58 (audit follow-up) via
  `router_otel_walk_test.go` which enumerates every chi-registered
  route and asserts each request lands inside an `otelhttp` span.
- *`admin/api-keys.tsx` real flow.* Still a single `it.skip` —
  unblocks once the API-keys endpoints land in v0.4.x.

**Definition of done (met at release)**

- Coverage report shows no admin page below 80 % statement coverage.
- `/v0/` and A2A conformance suites are in CI and gating.
- `openapi.yaml` ↔ router contract test is in CI and gating.
- "Every handler has at least one OTel span assertion" — partially met
  in v0.2.2 (focused smoke test), fully met in PR #58.

### v0.3.0 — Browse polish ✅ SHIPPED

v0.2.x was a coverage sprint. v0.3.0 was the first release that actually
changed what users see — four additive UX wins on the public browse
experience, zero breaking changes, zero new non-negotiables. All four
tasks shipped (see CHANGELOG `v0.3.0`).

Features accepted into scope (refused / deferred items tracked in
`docs/future-multi-environment.md` and in session notes):

- Per-entry activity feed on detail pages
- Namespace landing pages (`/mcp/{namespace}` and `/agents/{namespace}`)
- Card redesign (aligned tag row, status pill, inline metadata strip)
- Tool/skill count on list cards

Out of scope for v0.3.0 — each has its own reason and stays parked until
we decide otherwise:

- **Runtime usage / call-count metrics** — belongs on the API gateway
  once we have one. Do not fake it with copy_count.
- **Computed "registry score"** — the composite-score design wasn't
  landed. Do not ship a half-baked number.
- **Multi-environment entries** — design note in
  `docs/future-multi-environment.md`; do not implement until we
  revisit deliberately.
- **Access requests / grants / policies / publisher approval queue** —
  out of charter; we're a catalog, not an enterprise control plane.

Ordering is deliberate: low-risk UX wins first so we can ship each one
independently and gather feedback before committing to the activity-feed
work. Each item below is a self-contained task; the user validates each
before the next one starts.

**Task 1 — Real MCP tools field end-to-end** ✅ **SHIPPED**

Originally scoped as a lightweight chip reading `capabilities.tools[]` from
the free-form capabilities JSONB. During implementation we discovered the
MCP spec uses `capabilities.tools` as a capability-negotiation flag
(`{listChanged: bool}`), NOT a tool list — the actual list is only returned
at runtime via `tools/list`. Option C ("real typed `tools[]` field stored
in DB") was chosen so the registry can display tool counts and metadata
offline, and to end the semantic collision with the spec's capabilities
flag.

Shipped surface:
- [x] Migration `000007_mcp_tools` adds `tools JSONB NOT NULL DEFAULT '[]'`
      to `mcp_server_versions`. Additive, no backfill needed.
- [x] `domain.MCPTool` struct + `domain.ValidateTools` (non-empty name,
      unique within array, optional `description` / `input_schema` /
      `annotations`). Allows empty array so servers that simply don't
      declare tools are valid.
- [x] Store layer: `LatestMCPVersion.Tools` raw field, lateral sub-select
      adds `v.tools` to all three server read paths, `CreateMCPServerVersion`
      accepts `Tools` and defaults to `[]` when omitted. Integration-test
      coverage via `TestMCPServerVersion_ToolsRoundTrip` (6 read paths) and
      `TestMCPServerVersion_ToolsDefaultEmptyArray`.
- [x] Handler: `POST /api/v1/mcp/servers/{ns}/{slug}/versions` accepts
      `tools`, validates via `ValidateTools`, persists; `serverToResponse`
      projects `tools` onto `latest_version` defaulting to `[]`. New tests:
      `TestMCPHandler_CreateVersion_WithTools`,
      `TestMCPHandler_CreateVersion_InvalidTools`,
      `TestMCPHandler_GetServer_IncludesToolsOnLatestVersion`.
- [x] OpenAPI: new `MCPTool` schema component; `tools` field added to
      `MCPServerLatestVersion`, `MCPServerVersion`, and
      `CreateMCPServerVersionRequest`. Capabilities description rewritten
      to call out the distinction explicitly. v0 spec endpoints
      unchanged — they stay strictly MCP-spec shaped.
- [x] Bootstrap: `MCPVersionSpec.Tools` YAML field + validation at load
      time. Sample YAML populated with realistic tools for 7 versions
      across 6 servers (filesystem, computer-use, github, web-search,
      postgres, kubernetes) so local dev has data to render.
- [x] Agent card chip unchanged (already uses typed `skills.length`).
      MCP card chip rewired: `toolCount = lv?.tools?.length ?? 0`, chip
      hides when absent or empty. Regression test confirms
      `capabilities.tools: {listChanged: true}` alone does NOT render the
      chip (new test: "ignores capabilities.tools").
- [x] MCP server detail page: new Tools tab between Installation and
      Versions. Renders one card per tool (name + description +
      annotation badges + collapsible input_schema viewer), with an
      empty state referencing the spec's runtime `tools/list` path.
      Tab label shows count (`Tools (3)`) when populated.
- [x] Admin new-server form: JSON textarea for declaring tools when
      creating the first version. Client-side parse + array check
      returns inline errors before the POST; backend re-validates via
      `ValidateTools`.
- [x] Utility cleanup: `countMcpTools` helper and its test block removed
      from `web/src/lib/utils.ts` / `utils.test.ts` — the typed field
      replaces the shape-guessing heuristic entirely.

**Task 2 — Card redesign** ✅ **SHIPPED**

The aligned-layout work landed incrementally across v0.2.x and the
v0.3.0 polish cycle. The current `server-card.tsx` /
`agent-card.tsx` use `StatusBadge` from `badge-variants.ts` for the
lifecycle pill, an icon tile in the header, a chip row for
runtime/ecosystem/tools, and a footer freshness strip. Whole-card
focus is achieved via the `after:absolute after:inset-0` trick on the
title `<Link>`. Further alignment (consolidating status + transport
into a single tag row, pushing version into the bottom strip) was
considered and explicitly deferred — the existing layout is good
enough and the proposed refactor would invalidate downstream test
fixtures for marginal UX gain.

**Task 3 — Namespace landing pages** ✅ **SHIPPED**

`/mcp/:namespace` and `/agents/:namespace` are first-class routes
(`web/src/pages/mcp/namespace.tsx`,
`web/src/pages/agents/namespace.tsx`). The pages fetch the publisher
header and the filtered list in parallel, distinguish 404
("namespace doesn't exist") from empty-state ("publisher exists with
zero entries of this kind"), and have breadcrumbs + namespace chips
on every card pointing at them. Vitest covers render / loading /
empty / 404 / links-out; Playwright `coverage-public` covers the
end-to-end smokes.

**Task 4 — Per-entry activity feed** ✅ **SHIPPED**

`GET /api/v1/mcp/servers/{ns}/{slug}/activity` (and the agents
equivalent) project from `audit_log`, drop `actor_subject` /
`actor_email`, allowlist a small set of metadata keys (`from`, `to`,
`visibility`, `reason`, `version`, `field`), and rate-limit on the
public bucket. Detail pages render the privacy-scrubbed feed under
the tabs; the admin `/audit` page is the full-fidelity drill-down
view. Bootstrap emits synthetic events with
`metadata.source = "bootstrap"` so a fresh stack has realistic
activity. Wire-level Playwright assertions pin the privacy scrub.

**v0.3.0 release artefacts**
- CHANGELOG `v0.3.0` (Tasks 1, 3, 4 + UX polish) — already published.
- v0.3.1 (security bugfix release: JWT audience binding,
  sessionStorage default, trusted-proxy reporter IPs, CORS
  no-credentials).
- v0.3.2 (Helm-only patch: CNPG superuser secret, `DATABASE_URL`
  database name, ingress default off).

### Phase 7 — Access control & change-approval workflow ✅

Three sequenced ADRs designed how non-admin users author content and
how changes are reviewed before going live. All three sub-phases
shipped (PRs #28 → #32) plus an admin UI polish sweep (PR #37) that
made the new surfaces usable end-to-end.

**Phase 7.1 — Workspaces under publishers ✅** — **superseded by
[ADR 0006](docs/adr/0006-publisher-scoped-rbac.md)** (workspace layer removed;
resources are publisher-scoped again)
([ADR 0001](docs/adr/0001-workspaces-under-publishers.md))

- New `workspaces` entity between publishers and resources.
- Three-step migration: schema (`000008`) + Go-side backfill creating
  one `default` workspace per publisher; finalising migration
  (`000011`, PR #62, 2026-05-14) drops `publisher_id` from resources
  and swaps the slug unique key to `(workspace_id, slug)`. Slug
  uniqueness is now per-workspace.
- Hierarchical URLs `/v0/publishers/{p}/workspaces/{w}/servers/{s}`
  with HTTP 301 redirects from legacy paths.
- Bootstrap loader gained a `workspaces:` top-level list and a
  per-entry `workspace:` ref so seed data demonstrates the feature.
- Auth model unchanged in this phase (delivered in 7.2).

**Phase 7.2 — Workspace OIDC group binding ✅** — **superseded by
[ADR 0006](docs/adr/0006-publisher-scoped-rbac.md)** (replaced by
publisher-scoped role grants)
([ADR 0002](docs/adr/0002-workspace-group-binding.md))

- `workspaces.group_name` (1:1, nullable; `NULL` = admin-only).
- `KeycloakClaims.Groups` + `RequireWorkspaceWrite` middleware.
- Configurable `AUTH_GROUPS_CLAIM` (default `groups`).
- Manual Keycloak setup; reconciler ("operator") deferred to F4.

**Phase 7.3 — Change-approval workflow ✅** (reviewer authorization gate
amended by [ADR 0006](docs/adr/0006-publisher-scoped-rbac.md); workflow
unchanged)
([ADR 0003](docs/adr/0003-change-approval-workflow.md))

- New `review_state` column orthogonal to existing `status` /
  `published_at`.
- `revision` counter monotonic across the version's lifetime,
  PR-style continuous editing, discriminated 409 error model
  (`review-state-mismatch`, `review-revision-mismatch`,
  `review-already-pending`, `already-published`).
- One global reviewer group `registry-reviewers` (configurable via
  `AUTH_REVIEWER_GROUP`).
- Pending deletion flow on entries.
- Admin UI: `/admin/review` queue page, per-version history table
  with submit / withdraw / resubmit, request-deletion button on
  entries, live-pinging review queue badge on the sidebar, toasts
  on every mutation, modal Edit dialog for workspace settings.

#### Phase 7 backlog (deferred items from the ADRs)

From [ADR 0002](docs/adr/0002-workspace-group-binding.md):

- **0002-F1.** Per-resource-type group binding via Keycloak client
  roles.
- **0002-F2.** List members of a workspace's group via Keycloak Admin
  API.
- **0002-F3.** Many-to-many workspace↔group binding.
- **0002-F4.** Keycloak reconciler ("operator"). Pull-forward
  triggers: workspace count ≳ 50, or self-service workspace creation.
- **0002-F5.** SCIM provisioning.

From [ADR 0003](docs/adr/0003-change-approval-workflow.md):

- **0003-F1.** Per-resource-type or per-workspace reviewer groups.
- **0003-F2.** Forbid self-approval (`submitted_by != reviewed_by`).
- **0003-F3.** Notifications on submission/approval/rejection/deletion.
- **0003-F4.** SLA timers on `pending_review`.
- **0003-F5.** Bulk approval.
- **0003-F6.** Reviewer comments / discussion thread.
- **0003-F7.** Diff view in the admin UI between revisions.
- **0003-F8.** Cleanup of long-abandoned `rejected` versions.

### Phase 8 — Publisher-scoped RBAC, local accounts, remove workspaces ✅
([ADR 0006](docs/adr/0006-publisher-scoped-rbac.md))

- **Remove workspaces**: resources go back to publisher-scoped
  (`publisher_id` restored and `NOT NULL`, `(publisher_id, slug)`
  uniqueness restored, `workspace_id` + `workspaces` dropped in migration
  `000013`; the `/v0/` wire format is publisher-scoped again with no
  workspace segment).
- **RBAC**: `users`, `groups`, `group_members`, `role_grants` tables; roles
  Viewer / Editor / Reviewer / Admin granted to users or groups per publisher
  (Editor and Reviewer independent). Server Admin via claim or local flag.
- **Identity**: OIDC (JIT-provisioned) **and** local email+password accounts,
  handled together; the registry signs local tokens and validates both
  issuers, with local tokens walled off the MCP surface. Bootstrap admin
  seeded from config.
- **Claims** carry group membership only (`AUTH_GROUPS_CLAIM`); the
  reviewer-group seed (`AUTH_REVIEWER_GROUP`) becomes a group + global
  Reviewer grant.
- Two-step migration (`000012` additive, `000013` finalise), shipped as two
  PRs. Supersedes ADR 0001/0002, amends ADR 0003.

### Phase 9 — Brokered OIDC + registry sessions; remove `/v0` (ADR 0006 amendment)

Per the [2026-06-01 amendment](docs/adr/0006-publisher-scoped-rbac.md#amendment--2026-06-01-brokered-oidc-registry-sessions-and-removal-of-v0).
**Accepted, not yet implemented.**

- **OIDC broker**: a single confidential client; `GET /api/v1/auth/oidc/login`
  → IdP (Authorization Code + PKCE + state/nonce) → `…/callback` (code exchange
  with `client_secret`, `id_token` validation, identity → `users` mapping).
- **Registry sessions**: a `sessions` table + opaque `Secure; HttpOnly;
  SameSite=Lax` cookie; CSRF via SameSite + double-submit token. Claim groups +
  claim Server-Admin snapshotted at login.
- **Single-issuer middleware**: drop the multi-issuer validator, the
  request-path IdP JWKS, `IssuerKind`, the MCP wall, and the registry-signed
  local bearer token + its signing key + the local JWKS endpoint. `POST
  /api/v1/auth/login` sets the cookie instead of returning `access_token`.
- **Remove `/v0`**: delete the MCP-registry-spec routes / handlers / tests and
  the `/.well-known/oauth-protected-resource` endpoint.
- **Frontend collapse**: remove `oidc-client-ts` / `UserManager` / the
  `/auth/callback` route / sessionStorage tokens; `login()` redirects, identity
  comes from `GET /api/v1/me`.
- **Config + OpenAPI**: add `AUTH_OIDC_CLIENT_SECRET` + session-cookie settings,
  drop `AUTH_LOCAL_SIGNING_KEY`; delete the `/v0` paths and add the OIDC
  login / callback / logout operations + a session-cookie security scheme.

### Phase 10 — Later
- Skills & Prompts registry (same pattern as MCP servers).
- Signed publishes (sigstore/cosign).
- Webhooks on publish events.
- Federation with the public MCP registry.
- **Multi-environment entries** (dev/staging/prod per entry, each with
  its own URL/transport/auth/version pin). Design note + open questions
  parked in `docs/future-multi-environment.md` — do not implement until
  we revisit deliberately.

## 6. Resolved decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Namespacing | Publisher-scoped: `{namespace}/{slug}` |
| 2 | Private entries | Yes — `visibility` field (`private`/`public`). New entries default to `private`; admin/security team must approve before setting `public`. Public GETs only return `public` entries. |
| 3 | IdP for dev | Keycloak via docker-compose |
| 4 | Deployment target | Docker Compose **and** Helm chart for k8s |
| 5 | API-key auth | Yes — support both OIDC (interactive) and hashed API keys (machine-to-machine). Middleware tries JWT first, falls back to API-key. |
| 6 | UI template | shadcn/ui blocks (minimal) — build from primitives, no third-party admin template |
| 7 | User & role management | **Superseded by ADR 0006.** *Authentication* delegated to the IdP (OIDC) **and** local password accounts; *authorization* is registry-managed — `users`, `groups`, and per-publisher `role_grants`. Server Admin from `realm_access.roles` or a local `is_server_admin` flag. `/api/v1/users`, `/api/v1/groups`, and grants endpoints + admin pages exist. *(Reverses the original "no user table / IdP-only" decision.)* |

## 7. Definition of done (per phase)

- OpenAPI updated and served at `/openapi.yaml`.
- Migrations run cleanly up and down.
- Unit + integration tests pass in CI.
- Admin guard enforced on every mutating endpoint (verified by test).
- Docs: README section per new capability; ADR if a cross-cutting decision
  was made.
