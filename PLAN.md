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
- Users & roles: **delegated to the IdP** — no user/role endpoints in this API.

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

**Backend CRUD — complete.** `PATCH` and `DELETE` for MCP servers, agents,
and publishers are all implemented (see `router.go`) and covered by
handler-level tests against a real Postgres (testcontainers).

**Admin UI CRUD — complete.** Edit and delete actions for MCP servers,
agents, and publishers are wired into the admin detail pages
(`web/src/pages/admin/{mcp,agents,publishers}/detail.tsx`) with
confirmation dialogs.

**Out of scope — User & role management:**
User and role management is intentionally delegated to the identity provider
(Keycloak in dev, any OIDC-compliant IdP in production). The registry never
stores or manages users or roles itself — it only reads the `realm_access.roles`
claim from the JWT. Adding or removing the `admin` role is done in the IdP's
admin console. No `/api/v1/users` endpoint or `/admin/users` page will be built.

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

**TODO — Phase 5:**
- [ ] `POST /api/v1/api-keys`, `DELETE /api/v1/api-keys/{id}` — hashed API keys (per-publisher, machine-to-machine)
- [ ] API-key auth middleware (JWT-first, fallback to API-key lookup)
- [ ] Admin UI: API keys management page (`/admin/api-keys` — placeholder only today)
- [ ] Docker Compose prod profile (`deploy/docker-compose.prod.yml`)

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

### v0.2.2 — Coverage depth (next patch)

v0.2.1 backfilled the obvious surface-level gaps. v0.2.2 should push deeper
into the test pyramid where v0.2.1 only scratched the surface. Scope is
test-only — no shipping features in this release unless they fall out of
fixing a bug surfaced by the new tests.

**Web — admin depth**
- [ ] Interactive coverage on `admin/mcp/detail.tsx` and `admin/agents/detail.tsx`:
      per-version publish, deprecation, edit-in-place, status transitions, and
      the lifecycle stepper. Today these files only have render-and-link smoke
      tests.
- [ ] Real flow for `admin/api-keys.tsx` (currently a single `it.skip` waiting
      on Phase 5). Lifts as soon as the API-key endpoints land.
- [ ] Extract the shadcn/Radix Select jsdom shims (`hasPointerCapture`,
      `releasePointerCapture`, `scrollIntoView`) into `web/src/test/setup.ts`
      so individual test files stop re-declaring them.
- [ ] OIDC token refresh / expired-session paths in `AuthContext` —
      `accessTokenExpiring` event, silent-renew failure, logout-on-401.

**Server — protocol & spec conformance**
- [ ] OTel span emission tests: every HTTP handler must produce a span with
      the documented attributes (per CLAUDE.md "every new handler gets a
      span"). Use the OTel test SDK / in-memory exporter.
- [ ] Migration tests: forward apply of every numbered migration against a
      fresh testcontainers Postgres, plus idempotency (running `Migrate`
      twice must be a no-op).
- [ ] `/v0/` MCP wire-format conformance suite — assert the exact response
      shape from the MCP registry spec (`{servers, metadata: {count, nextCursor}}`,
      single-object detail, `_meta`, `packages[].registryType`, etc.).
- [ ] A2A Agent Card schema conformance — validate the per-agent
      `/.well-known/agent-card.json` and the global card against the pinned
      a2a-project June 2025 schema (decision G).
- [ ] `openapi.yaml` ↔ handler contract test: every documented path/operation
      must have a matching route, and every registered route must be
      documented. Catches drift between spec and implementation.
- [ ] Router-level test for `PublicRateLimitRPM` wiring — a test request
      loop that proves the env/YAML value reaches the per-IP bucket and
      changes the cutoff.

**Server — write paths**
- [ ] Audit every `POST` / `PATCH` / `DELETE` handler for untested error
      branches (RFC 7807 problem responses, 409 conflicts, 422 validation,
      403 admin-guard short-circuits). Today happy paths and 404s are well
      covered, error branches less so.

**Definition of done for v0.2.2**
- Coverage report shows no admin page below 80 % statement coverage.
- Every handler has at least one OTel span assertion.
- `/v0/` and A2A conformance suites are in CI and gating.
- `openapi.yaml` ↔ router contract test is in CI and gating.

### v0.3.0 — Browse polish (next minor)

v0.2.x was a coverage sprint. v0.3.0 is the first release that actually
changes what users see — four additive UX wins on the public browse
experience, zero breaking changes, zero new non-negotiables.

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

**Task 2 — Card redesign**
- [ ] Refactor `server-card.tsx` and `agent-card.tsx` to the aligned
      layout: icon + title + publisher row, description, tag row
      (status pill + transport/visibility + category tags), inline
      metadata strip at the bottom (tool/skill count + version +
      freshness).
- [ ] Status pill uses colour from the lifecycle state (`draft`,
      `published`, `deprecated`); reuse `badge-variants.ts` rather
      than adding a new primitive.
- [ ] Card is fully keyboard-focusable and the whole card is the link
      target (today some children compete for click). Verify with an
      a11y smoke test: `getByRole('link', { name: /.../ })` reaches
      the detail page.
- [ ] Update existing card tests; add a snapshot-free DOM test for the
      new metadata strip so it's enforced structurally, not visually.
- [ ] No API change. Pure CSS/Tailwind + component surgery.

**Task 3 — Namespace landing pages**
- [ ] New web route `/mcp/:namespace` and `/agents/:namespace`. URLs
      already match the slug pattern we publish today.
- [ ] New page components that call `GET /api/v1/mcp/servers?namespace=X`
      (and the agents equivalent) — the server-side filter already
      exists, no new endpoint needed.
- [ ] Page header shows the publisher behind the namespace via
      `GET /api/v1/publishers/{slug}` (call in parallel with the list
      fetch; render skeleton until both resolve).
- [ ] Empty-state copy when the namespace has zero entries of the
      requested kind (vs. the namespace not existing — those are
      different, render a 404 for the latter).
- [ ] Breadcrumbs: `Home › MCP Servers › {namespace}` so users can
      escape back to the flat list.
- [ ] Link to the namespace page from every card's publisher row and
      from the detail-page publisher row.
- [ ] Vitest coverage for the new page (render, loading, empty, 404,
      links out).
- [ ] Playwright smoke: land on `/mcp/{seeded-ns}`, assert the seeded
      entries appear, assert a link to a detail page works.

**Task 4 — Per-entry activity feed (biggest, ships last)**
- [ ] New public endpoint
      `GET /v0/mcp/servers/{ns}/{slug}/activity` (and the agents
      equivalent) that projects from `audit_log` filtered by
      `resource_type` + `resource_id`, returning a trimmed public
      view: `{id, action, created_at, actor_display_name}`. Must NOT
      expose `actor_email` or `actor_subject`; show a coarse
      "Publisher" / "Admin" label instead, derived from metadata.
- [ ] Rate-limit the new endpoint on the same per-IP bucket as other
      public reads.
- [ ] OpenAPI entry for both endpoints. Router contract test catches
      drift (already in place from v0.2.2).
- [ ] Handler tests: empty, populated, cursor pagination, unknown
      resource returns 404, privacy-scrub assertion (actor_email /
      actor_subject MUST NOT appear in the response body).
- [ ] Web: new "Activity" section on MCP + agent detail pages,
      rendered under the existing tabs (not inside them — it's
      cross-version). Paginated, loads 10 entries at a time with a
      "Show more" button. Reuses the existing date/time formatting
      from the version history component.
- [ ] Make the same feed filterable on the existing admin `/audit`
      page so the admin view can drill from the global log into a
      single entry's history (and vice versa).
- [ ] Vitest coverage for the new section (loading, empty, populated,
      load-more, privacy scrub — "actor_email" substring MUST NOT
      appear in the DOM).

**Cross-cutting chores**
- [ ] `CHANGELOG.md` entry with one section per task.
- [ ] Include the already-shipped pointer-cursor fix
      (`button-variants.ts`, `tabs.tsx`, `select.tsx`) under a "UX
      polish" sub-section of the changelog. It was an out-of-band
      patch but users will notice it on this release.
- [ ] OpenAPI + TS types regenerated.
- [ ] Coverage floors stay green.

**Definition of done for v0.3.0**
- All four task groups land behind per-task validation gates.
- No admin page drops below the 80 % floor set in v0.2.2.
- Go coverage floor stays ≥ 70 % after the new handler lands.
- `openapi.yaml` is in sync with the new activity endpoints, verified
  by the existing contract test.
- Playwright smoke exercises at least one namespace landing page and
  one detail page with activity visible.
- Changelog + git tag + GitHub release published.

### Phase 7 — Later
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
| 7 | User & role management | Fully delegated to the IdP (Keycloak or any OIDC provider). The registry reads `realm_access.roles` from the JWT but never stores or manages users or roles. No `/api/v1/users` endpoint or admin users page. |

## 7. Definition of done (per phase)

- OpenAPI updated and served at `/openapi.yaml`.
- Migrations run cleanly up and down.
- Unit + integration tests pass in CI.
- Admin guard enforced on every mutating endpoint (verified by test).
- Docs: README section per new capability; ADR if a cross-cutting decision
  was made.
