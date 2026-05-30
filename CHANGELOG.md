# Changelog

All notable changes to this project are documented here.

## Unreleased

### 🔭 Publisher-scoped admin visibility + `GET /api/v1/me`

The admin list endpoints (`GET /api/v1/mcp/servers`, `GET /api/v1/agents`)
gained a `mine=true` query parameter that scopes the listing to the resources
the authenticated caller can manage (ADR 0006): Server Admins and global-grant
holders still see every publisher, an author sees only the publishers they hold
a role on — **including their own private and draft entries** — and a caller
with no grants sees nothing. This is how the admin UI keeps multiple authors
from seeing each other's resources.

A new `GET /api/v1/me` returns the caller's resolved identity and effective
role grants (per-publisher and global, plus `is_server_admin`), so the SPA can
gate the admin UI by role without trusting any client-side claim.

The **admin UI is now role-aware** (ADR 0006). The MCP/agent list pages default
to `mine=true`, so authors see only their own resources. Actions and navigation
are gated by a new `usePermissions` hook: `New` appears only for Editors; edit /
deprecate / submit need Editor on the resource's publisher; approve / reject
need Reviewer; visibility flips and the direct-delete escape hatch stay
Server-Admin-only; and the Server-Admin-only nav (Publishers, Groups, Users,
Global grants, Reports, Activity) is hidden from non-admins. The server still
enforces every write.

### ✍️ Publisher Editors can author resources

Creating an MCP server or agent (`POST /api/v1/mcp/servers`, `POST
/api/v1/agents`) no longer requires Server Admin — a publisher **Editor** may
author for their own publisher (Admin / Server Admin still satisfy it). The new
resource is created **private + draft**, so it only reaches the public catalog
once a version is published and an Admin flips visibility; the approval gate is
unchanged. Anonymous create attempts get 401, non-Editors 403.

### 🔧 Pre-commit hooks + CI lint gate

Added a root [`.pre-commit-config.yaml`](.pre-commit-config.yaml) — baseline
hygiene, gitleaks, gofmt, golangci-lint, eslint, tsc, helm lint/docs, hadolint,
actionlint — plus a `pre-commit` CI job that runs the hygiene hooks, gofmt,
golangci-lint, and actionlint (the rest are already covered by dedicated jobs).
This closes the gap where Go formatting and linting weren't gated in CI. A new
[`server/.golangci.yml`](server/.golangci.yml) pins the v2 standard linter set.
Cleared the findings it surfaced (unchecked `Close()` on three `defer`s, an
unused first-call result in an auth test) and gofmt-normalised the tree.
Stopped tracking the regenerated `web/tsconfig.tsbuildinfo` build cache.

### 🚀 Tag pushes now cut a GitHub Release

The `Publish` workflow gained a `github-release` job: on a `v*.*.*` tag it
cuts a GitHub Release whose body is the matching `CHANGELOG.md` section,
gated behind a successful image + chart publish. The step is idempotent —
an existing release (e.g. cut by hand) has its notes refreshed instead of
erroring — and omits `--latest` so `gh` picks "Latest" by semver. Closes the
gap where `v0.3.2` and `v0.3.3` shipped images/charts but no Release.

### 🧹 Remove the workspace layer (ADR 0006)

Workspaces are gone. Resources are scoped directly to their owning
publisher again, and authorization is publisher-scoped RBAC (roles
granted to users/groups) rather than a per-workspace Keycloak-group
binding. See [ADR 0006](docs/adr/0006-publisher-scoped-rbac.md); this
is the destructive second half (the additive RBAC + local-auth half
shipped earlier).

> **Breaking schema change.** Migration `000013_workspaces_remove`
> backfills `publisher_id` from the workspace link, flips it `NOT NULL`,
> restores the `(publisher_id, slug)` uniqueness key, drops
> `workspace_id`, and `DROP`s the `workspaces` table. It aborts with a
> friendly error if any resource still has a NULL `publisher_id` or a
> `(publisher_id, slug)` collision — resolve those before upgrading. The
> down migration recreates one `default` workspace per publisher.

- Removes the seven `/api/v1/publishers/{slug}/workspaces…` endpoints
  and the `Workspace` / `CreateWorkspaceRequest` / `WorkspaceList`
  schemas from [openapi.yaml](server/api/openapi.yaml); regenerates
  `web/src/lib/schema.d.ts`.
- Drops the `workspace` field from the MCP-server and agent create
  payloads, the workspace `<Select>` from both admin create forms, and
  the Workspaces section from the publisher detail page.
- Deletes the workspace store/handlers/domain types and the dead
  `RequireWorkspaceWrite` middleware; write/approve routes already moved
  to `RequirePublisherRole` in the previous PR.
- Removes the CI "no stray publisher_id reads" guard — `publisher_id` is
  the canonical column on `mcp_servers` / `agents` once more.

### 📐 OpenAPI server / agent status enum gains `deleted`

Follow-up to the previous entry. `MCPServer.status` and `Agent.status`
in [openapi.yaml](server/api/openapi.yaml) declared
`enum: [draft, published, deprecated]`, but the code also returns
`deleted` for soft-deleted tombstones (see
[server/internal/domain/mcp.go](server/internal/domain/mcp.go) —
`ServerStatus`, used by both entities). Public list responses filter
deleted parents out, so most callers never see one — but
`GetMCPServer` / `GetAgent` don't filter on status, so an admin hitting
`/api/v1/mcp/servers/{ns}/{slug}` after a delete can land on a deleted
record. The `/v0/servers?include_deleted=true` path is also explicit
about returning them.

- Add `deleted` to both `MCPServer.status` and `Agent.status`
  enums. Status query-param filters keep the three-value union
  intact — they're filter inputs and the list endpoints still don't
  accept `deleted`.
- Regenerate `web/src/lib/schema.d.ts` (CI gate from PR #72 enforces
  the sync).
- `StatusBadge` ([web/src/components/ui/badge.tsx](web/src/components/ui/badge.tsx)
  and `statusVariant` in
  [badge-variants.ts](web/src/components/ui/badge-variants.ts)) now
  accept `"deleted"` and render an outline-style tombstone with an
  `XCircle` icon — visually distinct from the muted `draft`, the
  green `published`, and the red `deprecated`. New unit test pins
  the variant mapping; the distinct-variants invariant is widened
  from 3 to 4 colours.

### 📐 OpenAPI version-status enum matches the code

`MCPServerVersion.status` and `AgentVersion.status` in
[openapi.yaml](server/api/openapi.yaml) declared `enum: [draft,
published, deprecated]` but the server has always returned values
from `[active, deprecated, deleted]` (see
[server/internal/domain/mcp.go](server/internal/domain/mcp.go) —
`VersionStatus`). The spec and the wire were silently inconsistent
and generated TS types mislead the SPA: `version-history.tsx`
compares `v.status !== 'active'`, which was unreachable per the
declared types. The PR-#70 E2E test originally asserted
`status === 'published'` and had to be rewritten to use
`published_at`-truthy to work around the gap.

- Fix the two version schemas to declare the actual code shape.
- Regenerate `web/src/lib/schema.d.ts` (PR #72's CI gate now keeps
  this honest going forward).
- No backend or frontend code change required — the wire shape was
  already correct.

Not in this PR: `MCPServer.status` and `Agent.status` declare
`[draft, published, deprecated]` but the code also allows `deleted`
([domain.ServerStatus](server/internal/domain/mcp.go)). That's a
missing value rather than wrong values; deleted parents are filtered
out of list responses by default, so the SPA never sees them today.
Worth a separate small change.

### 🗂️ Workspace selector on MCP / agent create forms

The MCP and agent "new" forms only let you pick a publisher; the
target workspace was always hardcoded to `default`, which made the
per-workspace group binding model invisible from the most common
entry point. A publisher with two workspaces bound to different
Keycloak groups had no UI to send a new server / agent to anything
other than `default`.

- `CreateMCPServerRequest` and `CreateAgentRequest` gain an optional
  `workspace` field (slug under the same `namespace`). Omitting it
  preserves the legacy default-workspace fallback so existing API
  callers are unaffected.
- Handlers route through `ResolveWorkspace` when set, returning 422
  with a friendly message if the workspace doesn't exist under the
  publisher. The default fallback short-circuits
  `EnsureDefaultWorkspaceID` so picking an explicit workspace never
  lazily creates `default`.
- Admin UI: a `Workspace` Select sits below `Namespace` on both
  create forms. It lists the publisher's workspaces with their group
  bindings inline (`anthropic-labs — bound to anthropic-team`) so the
  admin can see what they're picking. Selecting a publisher clears
  any stale workspace pick.
- Tests: backend handler tests cover the routed-to-explicit-workspace
  path and the unknown-workspace 422; vitest cases cover the form
  forwarding the slug into the POST body.
- Vitest `testTimeout` bumped to 15s — the new dependent
  workspace-fetch chain pushed 4 admin-form tests past the 5s ceiling
  under parallel-suite load.

### 🔎 Diagnostics, contract guard, version sync

Three small, unrelated fixes bundled to reduce ongoing operator and
reviewer friction.

- **Richer 403 detail on workspace / reviewer middleware**
  ([server/internal/auth/middleware.go](server/internal/auth/middleware.go)).
  The previous `"Insufficient permissions: workspace group membership
  required"` body was identical whether the workspace was admin-only,
  the user's JWT lacked the group, or the configured group was empty.
  Now the detail field names the required group ("Writes to this
  workspace require membership in Keycloak group `\"anthropic-core\"`.")
  and distinguishes the admin-only / empty-group cases from a
  group-mismatch. Two new unit tests pin the wording so a future edit
  can't silently re-collapse them.
- **CI lock on the generated TS schema.** `web/src/lib/schema.d.ts`
  was gitignored, so PR reviewers couldn't see openapi.yaml changes
  flow through to the SPA's typed surface. The file is now tracked
  and CI runs `npm run generate && git diff --exit-status -- src/lib/schema.d.ts`
  so any edit to `openapi.yaml` that forgets the regenerate step
  fails the build. (Bootstrap commit ships the current generated
  file; subsequent PRs touching the spec will show the schema diff.)
- **Version-string sync.** `web/package.json`,
  `web/package-lock.json`, and `deploy/helm/ai-registry/Chart.yaml`
  were all frozen at `0.1.0` while git tags reached `v0.3.3`.
  Bumped to `0.3.3` so `helm show chart` / `npm pack` /
  `package.json` no longer mislead. The publish workflow continues
  to override these from the git tag at build time
  ([.github/workflows/publish.yml](.github/workflows/publish.yml)),
  so the bump only affects local-from-source flows.

### 🔑 Dev realm refreshed for Phase 7

`deploy/keycloak-realm-dev.json` predated the workspaces / change-approval
work and only shipped an `admin` realm role — no groups, no
group-membership mapper, no demo non-admin users. That meant the Phase 7
authoring and review paths returned `403` out of the box because JWTs
never carried a `groups` claim.

- Seed four Keycloak groups matching `deploy/bootstrap.example.yaml`:
  `anthropic-core`, `anthropic-labs`, `openai-platform`, plus the
  `registry-reviewers` reviewer group (default
  `AUTH_REVIEWER_GROUP`).
- Add the `oidc-group-membership-mapper` (bare names, `full.path: false`)
  to the `ai-registry-web` and `ai-registry-cli` clients so access
  tokens actually carry `groups[]`.
- Add `author@example.com` (member of `anthropic-core` +
  `anthropic-labs`) and `reviewer@example.com` (member of
  `registry-reviewers`). `admin@example.com` and `user@example.com`
  are unchanged and stay as the admin / 403-baseline reference cases.
- README dev-stack section now lists all four users, their passwords,
  and what each one exercises.

Dev only — production realm setup is still the operator's job per
[ADR 0002](docs/adr/0002-workspace-group-binding.md) until 0002-F4 (Keycloak
reconciler) lands.

## v0.3.3 — 2026-05-25

Two workstreams shipped together as the chunky tail of v0.3.x:

1. **Phase 7 access-control + change-approval bundle plus an admin UI
   polish sweep.** Server-side work landed across PRs #28–#32; UI
   polish landed in PR #37.
2. **Project-audit follow-ups (PRs #52–#59 + #62).** A four-front
   audit (server, web, infra/config, docs) produced a P0/P1
   punch-list; the high-impact findings shipped as a batch of small,
   surgical PRs, capped by the workspaces-finalise migration (#62)
   that dropped the legacy `publisher_id` FK from resource tables.

> **Breaking schema change.** Migration `000011_workspaces_finalise`
> drops `publisher_id` from `mcp_servers`/`agents` and swaps slug
> uniqueness to `(workspace_id, slug)`. Operators upgrading from
> v0.3.2 must let the prior image's boot-time backfill run once
> before applying `000011` — the up migration aborts with a friendly
> *workspace backfill not complete* error otherwise.

### 🏢 Workspaces under publishers

A new `workspaces` entity groups MCP servers and agents under each
publisher and binds each set to a Keycloak group whose members can
author content (no group → admin-only). See [ADR 0001](docs/adr/0001-workspaces-under-publishers.md)
for the design and [ADR 0002](docs/adr/0002-workspace-group-binding.md)
for the auth model.

- New `workspaces` table; two-step migration creates one `default`
  workspace per existing publisher and pivots resources onto it. The
  follow-up finalising migration (`000011`, shipped 2026-05-14 — see
  the "Workspaces finalise" entry below) drops the legacy `publisher_id`
  FK on MCP servers / agents. Forward-only — down migrations are
  dev-only.
- Hierarchical API:
  `GET /api/v1/publishers/{p}/workspaces`,
  `POST /api/v1/publishers/{p}/workspaces`,
  `GET/PATCH/DELETE /api/v1/publishers/{p}/workspaces/{w}`,
  `GET .../workspaces/{w}/servers`, `.../agents`.
- `RequireWorkspaceWrite` middleware: write endpoints require the
  caller's JWT `groups` claim to include the workspace's `group_name`
  (or the `admin` realm role). Configurable claim path
  (`AUTH_GROUPS_CLAIM`, default `groups`).
- Admin UI: workspace section on the publisher detail page, with
  expandable rows showing the MCPs and agents scoped to each
  workspace, plus a modal Edit dialog for renaming or rebinding.
- Bootstrap: optional top-level `workspaces:` list and per-entry
  `workspace:` reference field. Validation rejects unknown
  publisher / workspace refs up front. `group_name` is applied on
  first creation only so re-runs don't silently overwrite operator
  edits. Example YAML now seeds four demo workspaces and pins ten
  entries to them so the UI demo is populated out of the box.

### ✅ Change-approval workflow

A draft → pending review → published lifecycle that lets non-admin
group members propose changes that a global reviewer group approves
before they go live. See [ADR 0003](docs/adr/0003-change-approval-workflow.md).

- New `review_state` column on MCP / agent versions, orthogonal to
  `status` / `published_at`. States: `none`, `pending_review`,
  `rejected`. A monotonic `revision` counter tracks edits across the
  version's lifetime so concurrent edits surface a discriminated 409
  (`review-revision-mismatch`) instead of clobbering each other.
- New endpoints (per resource kind):
  `POST .../versions/{v}/submit`, `.../withdraw`, `.../approve`,
  `.../reject`, plus `POST .../deletion-request` for proposing an
  entry deletion. The reviewer-only `GET /api/v1/review-queue`
  surfaces every pending item across the registry.
- Reviewer authorisation via `RequireReviewer` middleware;
  configurable via `AUTH_REVIEWER_GROUP` (default `registry-reviewers`).
- RFC 7807 error model uses discriminated `type` URIs:
  `review-state-mismatch`, `review-revision-mismatch`,
  `review-already-pending`, `already-published`. The admin UI maps
  each to a friendly error message inline.
- Admin UI: `/admin/review` queue page with approve / reject (with
  required reason) actions, a per-version history table on the entry
  detail pages with submit / withdraw / resubmit controls, a
  `RequestDeletionButton` on every entry, and a live-pinging count
  badge on the sidebar.

### 🎨 Admin UI polish

A coordinated polish pass over the admin section (PR #37) once the
new workflow surfaces had landed:

- Mobile hamburger drawer (Esc-key dismiss, body scroll lock,
  auto-close on navigation); the desktop sidebar is `hidden md:block`
  and the drawer reuses `AdminSidebar`.
- Loading skeletons on the queue, workspaces, and versions sections;
  toasts (sonner) on every change-approval mutation (submit, withdraw,
  approve, reject, request deletion) and on workspace CRUD; inline
  form-level error placement next to submit buttons. The review-queue
  badge cache is invalidated alongside change-approval toasts so the
  sidebar count stays in sync.
- Workspace edit form lives in a modal dialog (Esc to close, body
  scroll lock, `aria-modal`) instead of pushing the table down.
- Table-row primary actions (Edit, Manage) promoted from `ghost` to
  `outline` with leading icons; `DeleteButton` quieted to outline
  with destructive text so the visual hierarchy across the row stops
  collapsing into "two labels and one filled red button".
- List tables hide low-priority columns on small viewports and
  surface the slug inline under the name where the dedicated column
  is hidden; page headers wrap.

### 🧭 Project-audit follow-ups (PRs #52–#59)

A four-front audit (server, web, infra/config, docs) surfaced a P0/P1
punch-list. The high-impact items shipped as small, surgical PRs;
the deferred items are documented inline in the relevant PR
descriptions.

- **PLAN refresh** (#52) — mark v0.2.2 / v0.3.0 / v0.3.1 / v0.3.2 as
  shipped; PLAN was lagging behind the actual release state.
- **Doc + dead-code cleanup** (#54) — flip ADRs 0001/0002/0003 from
  `Proposed` → `Accepted`, drop the stale `next-themes` row from
  PLAN's Phase 6 migration table, prune dead Renovate rules
  (`next`, `eslint-config-next`, `next-auth`, `autoprefixer`,
  `tailwindcss-animate`), bump `@types/node` pin from `^22.0.0` →
  `^24.0.0` (CI runs Node 24), delete the unused
  `compatibility-info.tsx` component.
- **Config-layer fixes** (#55) — `PUBLIC_BASE_URL` and
  `BOOTSTRAP_FILE` were bypassing the config layer (read directly
  via `os.Getenv` from handlers and main), violating CLAUDE.md's
  env+YAML+default mandate. Both are now reachable via env, YAML
  (`http.public_base_url`, `bootstrap_file`), and a built-in
  default. The `--bootstrap-file` CLI flag still wins. The
  `OAuthProtectedResource` handler also dropped its silent
  `localhost:8081` fallback — empty `PublicBaseURL` now returns
  HTTP 500, mirroring `GlobalAgentCard`. 8 new tests pin the
  three-place rule.
- **Helm CNPG postgres bump** (#56) — `cnpg.postgresVersion: "16"`
  → `"18"`. Closes the version drift with the docker-compose stack,
  which moved to `postgres:18-alpine` in PR #41. `pg-probe` snippets
  in `docs/runbook.md` updated to match.
- **ADR 0004 backfill** (#57) — Phase 6 (Next.js → Vite migration)
  was a cross-cutting decision but never got an ADR. `docs/adr/0004-vite-spa-migration.md`
  captures the rationale, alternatives considered, and historical
  implementation steps so the decision survives the next "why
  aren't we on Next.js?" question.
- **OTel uplift** (#58) — three observability test gaps closed:
  - `router_otel_test.go` only pinned spans for 4 hand-picked routes;
    new `router_otel_walk_test.go` enumerates every chi-registered
    route and asserts every request lands inside an `otelhttp` span,
    so a future router change that drops instrumentation on any
    real handler fails CI.
  - `internal/observability/` was at 0.0% coverage; new
    `observability_test.go` pins log-level mapping, trace_id /
    span_id injection, and metric-instrument registration.
  - `internal/problem/` was at 0.0% coverage; new tests pin the
    RFC 7807 wire shape, `omitempty` semantics, and slug-as-URL
    contract.
  - Bonus: `handlers/config.go` (the SPA's `/config.json` bootstrap)
    gained its first tests covering the auth_storage coercion and
    the empty-issuer dev-boot case.
- **P2 quality fixes** (#59) — three small surgical fixes:
  - **Counter drift on delete.** `CreateServer` / `CreateAgent`
    incremented `MCPServersTotal` / `AgentsTotal`; the matching
    delete handlers never decremented. The OTel `UpDownCounter`
    monotonically inflated. Fixed.
  - **Audit metadata silent-drop.** A `json.Marshal` failure in
    `store.LogAuditEvent` dropped metadata without a log entry;
    now we `slog.Warn` and continue with `metadata=NULL`.
  - **`flag.Parse` error swallow.** `_ = fs.Parse(os.Args[1:])` is
    now a structured `slog.Warn` so log aggregators see flag
    typos.

Deferred (documented in the audit synthesis but not shipped):
`DisallowUnknownFields` rollout (no `additionalProperties: false`
in the OpenAPI spec — needs per-endpoint risk analysis first),
rate-limiter time-based janitor (bigger change), per-handler
internal child spans, eager markdown chunk on detail pages
(`React.lazy` deferral).

### 🏗️ Workspaces finalise (ADR 0001 Step 3, PR #62)

The finalising migration designed alongside the original workspaces
rollout but parked at the time. After this PR, MCP servers and agents
no longer carry `publisher_id`; the owning publisher is reached via
`workspaces.publisher_id` through a single JOIN. See ADR 0001's
"Status note (2026-05-14)" for the post-shipping context.

- New migration `000011_workspaces_finalise.{up,down}.sql`. The up
  migration is gated by a `DO $$ … RAISE EXCEPTION` block: if any row
  still has `NULL workspace_id`, it aborts with a friendly
  *workspace backfill not complete* error so operators know to redeploy
  the prior image once and let the boot-time backfill run before
  retrying.
- Slug uniqueness is now **per-workspace**: `UNIQUE(workspace_id,
  slug)`. Two workspaces under one publisher may each expose a
  resource with the same slug.
- Every store query that previously read `s.publisher_id` /
  `a.publisher_id` is rewritten to JOIN through workspaces;
  `mcp_servers.publisher_id` and `agents.publisher_id` are dropped.
- `CreateMCPServerParams.PublisherID` and
  `CreateAgentParams.PublisherID` are removed. Handlers now resolve
  the workspace via the newly-exported
  `DB.EnsureDefaultWorkspaceID(publisherID)` before calling create.
- The boot-time `BackfillWorkspaces` helper and its `cmd/server/main.go`
  caller are removed — after the NOT NULL constraint lands the
  function is a no-op by construction, and its UPDATE queries
  referenced the dropped column.
- Wire-level `publisher_id` fields on MCP server / agent API
  responses remain populated (derived through the JOIN); the OpenAPI
  spec is unchanged.
- CI gains a `grep` guard that fails the build if a future change
  reintroduces `s.publisher_id` / `a.publisher_id` / INSERTs of
  `publisher_id` into the resource tables.

## v0.3.2

Helm-chart-only patch release. Four fixes that unblock a fresh
`cnpg.enabled=true` install; no server/web/API changes.

### 🐘 CNPG superuser secret is auto-created again

The Cluster resource set `spec.superuserSecret.name`, which CNPG
interprets as "the user is providing this secret" and suppresses
auto-generation. A fresh install therefore left the backend pod stuck
in `CreateContainerConfigError` with `secret
"<cluster>-superuser" not found`.

- `superuserSecret` removed from `templates/cnpg-cluster.yaml`. CNPG
  now auto-generates `<clusterName>-superuser` as intended.
- Unused `cnpg.superuserSecretName` value and helper branch deleted;
  the helper always returns `<clusterName>-superuser`.

### 🎯 DATABASE_URL targets the actual database

CNPG's auto-generated superuser `uri` hardcodes `dbname=*` (wildcard),
so even once the secret existed the server crashed on start with
`database "*" does not exist`.

- Backend deployment now builds `DATABASE_URL` from the superuser
  secret's `username` + `password`, the CNPG `-rw` service, and
  `cnpg.initdb.database`. The `uri` key is no longer consumed.
- Scheme is `postgres://` — `golang-migrate` registers its driver
  under `postgres` and fails on `postgresql://` with
  `unknown driver postgresql`.

### 🚪 Ingress disabled by default

`ingress.enabled` now defaults to `false`, matching the existing
`httpRoute.enabled=false` and `cnpg.enabled=false` defaults.
Operators opt in to the networking path they actually use instead of
discovering a stray Ingress resource on first install.

## v0.3.1

Security bugfix release. Four high-severity findings from an internal
security review are fixed; no feature changes. All API shapes stable.

### 🔐 JWT audience binding (H1)

The JWT validator now enforces the `aud` claim when `OIDC_AUDIENCE` is
set. Previously the server accepted any token minted by the configured
issuer, even one intended for a different client on the same realm —
a straight violation of the MCP authorization spec (OAuth 2.1
resource indicators).

- `auth.Validator` takes an `audience` string; when non-empty it is
  passed to `jwt.WithAudience` during parse.
- `OIDC_AUDIENCE` wired through env (`OIDC_AUDIENCE`), YAML
  (`auth.oidc_audience`), and defaults — per the CLAUDE.md config
  rule. Example + `.env.example` updated.
- Keycloak dev realm (`deploy/keycloak-realm-dev.json`) now emits
  tokens with `aud=ai-registry-server` via an inline
  `oidc-audience-mapper` on both `ai-registry-web` and
  `ai-registry-cli` clients.
- Docker Compose (`dev`, prod, CI) and Helm default
  `OIDC_AUDIENCE=ai-registry-server`.
- Tests: reject tokens missing `aud`, reject tokens with wrong `aud`,
  accept tokens with matching `aud`, and the audience check is
  skipped when `OIDC_AUDIENCE` is empty (dev-only escape hatch).

### 🔒 SPA token storage defaults to sessionStorage (H2)

Access and refresh tokens were previously held in `localStorage`,
meaning any XSS on the admin UI could exfiltrate them and reuse them
across tabs indefinitely. The SPA now defaults to `sessionStorage`
(scoped to a single tab, cleared on close), and `localStorage` is an
opt-in chosen by the server.

- `GET /config.json` returns a new `auth_storage` field
  (`"session"` | `"local"`, default `"session"`). The server rejects
  any other value and falls back to `"session"`.
- `oidc-client-ts` `UserManager` is constructed with a
  `WebStorageStateStore` backed by the chosen store.
- The Playwright-friendly `"local"` mode is still available for E2E
  because `storageState()` only captures `localStorage`. CI compose
  sets `AUTH_STORAGE=local`; no production deployment should.
- `AUTH_STORAGE` wired through env + YAML + default per CLAUDE.md.
- Tests: defaults to sessionStorage, honours `auth_storage=local`
  when served, coerces unknown values back to `"session"`.

### 🛰 Trusted-proxy gate on reporter IPs (H3)

`POST /reports` (user bug/abuse reports) was honouring
`X-Forwarded-For` from every client, letting anyone forge the
`reporter_ip` column. The endpoint now goes through the existing
`middleware.ClientIP` helper, which only accepts XFF / X-Real-IP
from peers inside `TRUSTED_PROXY_CIDR`.

- `middleware.ClientIP` was exported so handlers share the same
  trust policy as the rate-limit middleware.
- `ReportHandlers` takes a `*net.IPNet` at construction; `nil`
  disables proxy trust entirely (safe default).
- The ad-hoc `reporterIP` helper was deleted.
- Tests: XFF ignored when no trusted proxy configured; XFF honoured
  only when the remote peer is inside the configured CIDR.

### 🌐 CORS never allows credentials (H4)

Our API is bearer-only — no cookies — so echoing
`Access-Control-Allow-Credentials: true` was a latent footgun in
case a future change ever added cookie auth. The middleware now
guarantees the header is never set, and wildcard origins emit a
literal `*` instead of reflecting the request `Origin`.

- `slices.Contains(allowedOrigins, "*")` → `Allow-Origin: *`, no
  `Vary`.
- Non-wildcard match → `Allow-Origin: <origin>` + `Vary: Origin`.
- No code path sets `Allow-Credentials`, and a regression test pins
  the invariant.

### 🧪 Verification

- Unit tests: 9 Go packages green (`go test -count=1 ./...`), 539/540
  Vitest (`web` unit + component), `tsc --noEmit` clean, ESLint 0
  warnings.
- End-to-end against a fresh Keycloak re-import: admin token →
  `GET /api/v1/stats` 200; non-admin token → 403; anonymous → 401;
  issued access tokens carry `aud=ai-registry-server` and
  `realm_access.roles`.

## v0.3.0

Browse-polish release. Three of the four v0.3.0 tasks from `PLAN.md`
land here (Task 2's card redesign was delivered ahead of schedule in
v0.2.x and only needed an icon-tile polish this cycle) plus the
bootstrap + audit-log work needed to make the new activity feed
interesting on a fresh stack. Zero breaking changes.

### ✨ MCP tools become a first-class field (Task 1)

MCP clients negotiate `capabilities.tools` as a boolean feature flag
(`{listChanged: bool}`), NOT a tool list — the actual list is only
returned at runtime via `tools/list`. The registry was previously
reading the capabilities flag as if it were a list, which silently
under-counted servers that advertised tools. v0.3.0 introduces a typed
`tools[]` field on `mcp_server_versions` so the registry can display
tool counts and metadata offline, and ends the semantic collision
with the spec's capabilities flag.

- Migration `000007_mcp_tools` adds `tools JSONB NOT NULL DEFAULT '[]'`
  to `mcp_server_versions`. Additive — no backfill needed.
- `domain.MCPTool` struct + `domain.ValidateTools` (non-empty name,
  unique within array, optional `description` / `input_schema` /
  `annotations`). Empty array is valid.
- Store, handler, and OpenAPI all carry the new field end-to-end.
  `POST /api/v1/mcp/servers/{ns}/{slug}/versions` accepts `tools` and
  validates via `ValidateTools`. The `/v0/` spec-shaped endpoints are
  unchanged.
- Bootstrap: `MCPVersionSpec.Tools` YAML field, with realistic tools
  populated for 7 versions across 6 servers (filesystem, computer-use,
  github, web-search, postgres, kubernetes) so local dev has data.
- New **Tools tab** on the MCP server detail page: one card per tool
  (name + description + annotation badges + collapsible `input_schema`
  viewer), with an empty state referencing the spec's runtime
  `tools/list` path. Tab label shows count (`Tools (3)`) when
  populated.
- MCP card chip rewired to `lv.tools.length`, hides when absent or
  empty. Regression test: `capabilities.tools: {listChanged: true}`
  alone does NOT render the chip.
- Admin new-server form: JSON textarea for declaring tools when
  creating the first version. Client + server both re-validate.

### 🗂 Namespace landing pages (Task 3)

Every publisher now has a scoped landing page for each catalogue half:
`/mcp/{namespace}` and `/agents/{namespace}`. Until now the only way
to see "everything by this publisher" was the flat list filtered via a
query string — now it's a first-class route that can be linked to,
bookmarked, and crawled.

- New pages fetch the publisher header (`GET /api/v1/publishers/{slug}`)
  and the filtered list (`GET /api/v1/mcp/servers?namespace=X` /
  `GET /api/v1/agents?namespace=X`) in parallel; three distinct states
  (loading skeleton, 404 when the publisher doesn't exist, empty-state
  when the publisher exists with zero entries of that kind).
- Namespace chip on every card, detail-page breadcrumbs, and the
  publisher-row link now point at the path-param URLs instead of
  `?namespace=X` query strings. Filter behaviour on the flat lists is
  preserved — existing e2e pagination tests pass unchanged.
- 10 new Vitest cases covering render / loading / empty / 404 /
  links-out across both namespace pages. Playwright `coverage-public`
  gains 5 new smoke tests: seeded entries appear, private-MCP is
  hidden, detail-page link works, unknown-namespace 404 renders, chip
  navigation from the flat list lands on the new route.

### 📜 Per-entry activity feed + admin audit page (Task 4)

Every MCP server and agent detail page now shows a privacy-scrubbed
lifecycle log: creations, publishes, visibility changes,
deprecations. The new admin `/audit` page is the full-fidelity view
with actor-identity columns and filters, so operators can drill from
the global log into a single entry's history and back. Both surfaces
share one backing endpoint per resource kind.

- **Public endpoints** `GET /api/v1/mcp/servers/{ns}/{slug}/activity`
  and the agents equivalent. Project from `audit_log` filtered by
  `(resource_type, resource_id)`, apply a privacy scrub (drop
  `actor_subject` / `actor_email`; metadata key allowlist: `from`,
  `to`, `visibility`, `reason`, `version`, `field`), and drop draft
  `*version.created` events so the public feed only shows
  lifecycle-relevant actions. Cursor pagination on
  `(created_at, id) DESC`. Rate-limited through the same per-IP bucket
  as the other public reads.
- **Admin `/audit` page**: filterable full-fidelity view of the audit
  log with actor identity (subject + email + role) and per-row
  drill-down links to the affected resource. Filter by resource type
  to narrow the feed; cursor paginates the same way.
- **Bootstrap** now emits synthetic audit events
  (`actor_subject = system:bootstrap`,
  `actor_email = bootstrap@ai-registry.local`,
  `metadata.source = "bootstrap"`) for publisher / server / version /
  agent / agent-version first-time inserts so a freshly-brought-up
  stack has realistic activity to render. Re-running the bootstrap is
  idempotent — it does not double-emit.
- **Layout**: the publisher README now renders at full container width
  directly under the short description (above the tabs) on MCP + agent
  detail pages, so the narrative content is always visible regardless
  of which tab the reader has open. Old `ActivityStrip` component
  renamed to `EngagementStrip` to free the "Activity" name for the
  lifecycle feed.
- **Tests**: new Playwright `activity` project exercises admin +
  public surfaces end-to-end, including a wire-level assertion that
  the public endpoint never leaks `actor_subject` / `actor_email` /
  `client_ip` / `user_agent` / `internal_note`. Vitest gains the
  `ActivityFeed` component suite (loading / empty / populated /
  load-more / privacy scrub / per-resource endpoint selection) and the
  `admin/audit` page suite. Bootstrap test covers audit emission shape
  + idempotency.

### 💅 UX polish

- **Card icon tile** — a small rounded identity anchor (`Boxes` for
  MCP servers, `Bot` for agents) renders before the name on both
  catalogue cards. Long names truncate with ellipsis instead of
  pushing the right-side badge cluster off-card. The rest of each
  card — version/status cluster, runtime/ecosystem chips, tools row,
  description, transport block, footer — is byte-for-byte unchanged.
- **Pointer cursors** on the Button, Tabs, and Select primitives so
  every interactive surface in the UI gets the hand cursor on hover.
  Previously only a handful of ad-hoc components set it.

### ⚠️ Upgrade notes

No breaking API changes. The `tools` field is additive. Namespace
URLs become first-class — existing bookmarks pointing at
`?namespace=X` query strings continue to work on the flat list pages.
The `audit_log` table is unchanged; bootstrap's synthetic events
reuse the existing shape with a sentinel `source = "bootstrap"`
metadata marker so they can be filtered out by operators who don't
want them in analytics.

**Full changelog:** `v0.2.2...v0.3.0`

## v0.2.2

Coverage-depth release. Zero user-visible feature changes — this patch
closes the test pyramid gaps called out in `PLAN.md` § v0.2.2, plus one
bundle-size win for first-time public visitors and the Node-20 → Node-24
Actions migration ahead of GitHub's June 2026 force-cut. Every
non-negotiable rule in `CLAUDE.md` (API-first, spec compatibility, OTel
instrumentation, admin-only writes) now has a mechanical contract test
enforcing it in CI.

### 🧪 Protocol & spec conformance (server)

- **`/v0/` MCP wire-format conformance suite** — 40 tests pinning the
  response shape to the MCP registry spec (top-level `servers` key,
  `metadata.count`/`nextCursor`, single-object detail, `_meta`, package
  `registryType`/`identifier`/`version`/`transport.type`, error envelope
  shape, RFC 3339 timestamps). No more `t.Skip` gaps — the old dead
  package-shape skip now fails loudly on an empty seeder.
- **A2A Agent Card JSON Schema conformance** — `server/api/a2a-agent-card.schema.json`
  pins the a2a-project/a2a **June 2025** shape (CLAUDE.md decision G) as
  a machine-checkable schema, embedded alongside `openapi.yaml` via
  `go:embed`. New handler tests validate every per-agent and global card
  emission, catching regressions like `defaultInputModes` going nil or
  a `securitySchemes` type outside the decision-K allow-list.
  Misconfiguration path is also covered: unset `PUBLIC_BASE_URL` must
  return `application/problem+json` 500, never silently advertise
  localhost.
- **`openapi.yaml` ↔ router bijection** — `router_contract_test.go`
  walks every chi route and every documented path/operation and fails
  if either side drifts. The allow-list is one line (`/config.json`)
  with a comment explaining why it's spec-exempt.
- **Admin-guard router contract** — `router_admin_guard_test.go`
  enumerates every `POST`/`PUT`/`PATCH`/`DELETE` route via `chi.Walk`,
  subtracts the public-writes allow-list (`view`, `copy`, `reports`),
  and asserts each remaining route returns 401 without a token. A
  sibling test identity-compares middleware chains to catch the other
  direction (an accidental `RequireAdmin` on a public telemetry route).
  This is the mechanical enforcement of CLAUDE.md's non-negotiable
  rule #3: *"All writes go through admins."*
- **OTel span emission contract** — `router_otel_test.go` installs a
  tracetest `SpanRecorder` as the global provider, fires DB-free public
  routes through the fully-wrapped (`otelhttp.NewHandler`) production
  router, and asserts every request produces a span carrying both
  method and status-code semantic-convention attributes. If anyone ever
  replaces `otelhttp.NewHandler` with a bare mux in `NewRouter`, the
  test fails immediately — the exact bug CLAUDE.md warns about.
- **Migration forward-apply + idempotency** — a fresh testcontainers
  Postgres, `Migrate()` twice, assert every core table and a sample of
  per-migration columns (`featured`, `tags`, `verified`, `readme`,
  `view_count`, `copy_count`) exist.
- **Public rate-limit wiring test** — proves `RouterDeps.PublicRateLimitRPM`
  actually reaches the per-IP bucket (3 requests at limit=2 → third
  gets 429) and that `0` maps to the documented 1000-rpm default, not
  to "reject everything".

To make the contract tests possible, `NewRouter` was split into
`buildMux()` + `NewRouterForTest()` so `chi.Walk` can descend into the
raw `*chi.Mux` without the `otelhttp` wrapper in the way. Production
`NewRouter` still returns the fully-wrapped handler.

### 🧪 Coverage depth (web)

- **Interactive admin-detail coverage** — `admin/mcp/detail.tsx` and
  `admin/agents/detail.tsx` gained 25 tests between them covering the
  LifecycleStepper Deprecated transition, DeprecateButton confirm
  accept/decline, edit-form cancel, delete confirm (with navigate
  assertion) and decline, visibility-mutation failure surfacing, the
  published-only deprecate guard, and the A2A `/.well-known/agent-card.json`
  link href (CLAUDE.md decision H: a cached URL regression silently
  breaks every A2A client).
- **OIDC token lifecycle in `AuthContext`** — 4 new tests capture the
  `addUserLoaded` / `addUserUnloaded` / initial-hydration / unmount
  cleanup paths on the `UserManager.events` subscription. The silent
  cleanup bug (fresh arrow-fn on unmount becomes a no-op) is now
  gated.
- **Radix Select jsdom shims** centralised in `src/test/setup.ts` —
  `hasPointerCapture`, `releasePointerCapture`, `scrollIntoView`.
  Individual test files stop re-declaring them in `beforeEach`.
- **Admin-page coverage floor is verifiable** — the stale
  `"src/pages/**"` exclusion in `vitest.config.ts` hid admin pages
  from the coverage report entirely. Narrowed to public user pages
  only; every admin page now reports ≥86% statements (lowest:
  `mcp/detail.tsx` at 86.4%; highest: 100%), comfortably above the
  v0.2.2 DoD floor of 80%.

Vitest is now **64 files / 490 passing / 1 skipped** (the skipped test
is the `admin/api-keys.tsx` interactive flow, blocked on Phase 5 API-key
endpoints per `PLAN.md`).

### 🔧 CI gates

- **Named conformance suite step** in `ci.yml` re-runs the `/v0/`, A2A,
  OpenAPI-contract, and admin-guard tests with `-v` so their names
  appear in the CI log. A silent deletion or rename now surfaces as a
  CI failure instead of quietly reducing coverage.
- **Go coverage floor at 70%** — `go tool cover -func` against the
  aggregated profile, floor-checked in CI. Current number: 72.2%. The
  floor catches regressions from silent test deletions without gating
  normal development on a moving target.
- **Node 24 readiness** — all third-party actions across `ci.yml`,
  `e2e.yml`, and `publish.yml` bumped to their Node-24 majors
  (`checkout@v5`, `setup-node@v5`, `setup-go@v6`, `upload-artifact@v5`,
  `setup-helm@v5`, `setup-buildx-action@v4`). The Docker action suite
  and `upload-artifact@v5` still bundle Node 20; `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24`
  is set in `publish.yml` + `e2e.yml` as the documented interim escape
  hatch ahead of the June 2, 2026 force-cut.
- **Playwright HTML report upload fix** — CI reporter was `github`
  only, so `upload-artifact` in `e2e.yml` had no `playwright-report/`
  to grab. Now emits both `github` annotations and an HTML report.

### 🚀 Performance

- **Lazy-loaded admin bundle.** All 13 admin pages are now
  `React.lazy()` with a single `Suspense` boundary inside `RequireAuth`.
  First-time public visitors no longer pay for the admin surface
  (forms, editors, bulk actions).
  **Main bundle: 729 KB → 207 KB (gzip: 215 KB → 55 KB).** The vite
  "chunk larger than 500 kB" warning is gone.
- **Long-lived vendor chunks.** `vite.config.ts` `manualChunks` splits
  `react`/`react-dom`/`react-router`, `@tanstack/react-query`,
  `oidc-client-ts`, and the `react-markdown` + `remark`/`rehype` chain
  into dedicated vendor chunks so app-code changes no longer bust
  their long-term browser caches.

### 🐛 Fixes

- **`any`-free web codebase.** The v0.2.1 unblock commit had temporarily
  dimmed `no-explicit-any` to `warn`. v0.2.2 reverts that downgrade
  and fixes every underlying site: hook call sites branch on path so
  openapi-fetch's literal-string typing survives the ternary; related
  / version views use the generated `MCPServer`/`Agent` schema types;
  test mocks are typed against the schema (which surfaced two fixture
  drifts — `status: 'active'` → `'published'`, `runtime: 'python'` →
  a valid transport enum value); `(globalThis as any)` → `vi.stubGlobal`.
- **React Fast Refresh compliance.** Split `cva` variants out of
  `button.tsx`/`badge.tsx` into dedicated `*-variants.ts` files so
  each component module only exports components —
  `react-refresh/only-export-components` clean.
- **Test-fixture drift.** Several MCP fixtures had bogus `runtime`
  values (`'node'`, `'python'`) hidden behind `as MCPServer` casts.
  The MCP `runtime` field is the **transport mechanism** (`stdio`,
  `http`, `sse`, `streamable_http`), not a language. Replaced with
  valid enum values and added comments pointing to
  `server/internal/domain/mcp.go`.
- **Dependabot bumps.** `vite ^6.2.5 → ^6.4.2`, `vitest` +
  `@vitest/coverage-v8 ^2.1.9 → ^3.2.4`, `esbuild ^0.25.0` override.
  Closes the two web advisories; the two Go advisories were test-only
  transitives of `testcontainers-go` and were dismissed as `not_used`.

### ⚠️ Upgrade notes

No schema changes. No breaking API changes. No config changes.
Operators do not need to touch anything to adopt v0.2.2.

**Full changelog:** `v0.2.1...v0.2.2`

## v0.2.1

Coverage backfill release. No user-visible feature changes — the focus is on
filling in test gaps left by the v0.2.0 sprint and tightening one piece of
operator config that showed up under load.

### 🧪 Tests added

- **Server (Go):** new handler tests for `view_count` / `copy_count` event
  recording on both MCP servers and agents, and parity tests for
  `PATCH /v0/servers/{ns}/{slug}/versions/{version}/status`. Store-level tests
  for the matching repository methods.
- **Web (Vitest):** ~18 new test files covering every admin page (`new` /
  `list` / `detail` for publishers, MCP servers, and agents), the admin
  dashboard, layout, and api-keys placeholder, plus shared components
  (server-card, agent-card, theme-toggle, delete-button, deprecate-button,
  raw-json-viewer, install-command, activity-strip, related-entries,
  section-header). Vitest run is now 64 files / 473 passing / 1 skipped
  (Phase 5 api-keys flow).
- **Web (Playwright):** new `coverage-admin.spec.ts` and `coverage-public.spec.ts`
  suites — bulk actions, publish-via-UI through the new-form flow, and a
  22-server pagination walkthrough on the public MCP list. Full Playwright
  suite is now 50 tests across 7 projects, all green.

### 🔧 Server

- **Configurable public rate limit.** The per-IP budget for unauthenticated
  reads on `/api/v1` is now driven by `PUBLIC_RATE_LIMIT_RPM` (env) /
  `http.public_rate_limit_rpm` (YAML), defaulting to **1000 req/min** (was a
  hard-coded 100). Documented in `deploy/.env.example`. The previous limit
  was easy to trip from a browser SPA or the e2e suite under normal use.

### 🐛 Fixes

- Playwright `testMatch` regexes were unanchored and silently pulled
  `coverage-admin.spec.ts` into the `admin` project (and similarly for
  `public`), causing duplicate runs and project-config mismatches. Now
  anchored with `(^|\/)admin\.spec\.ts$`.
- A handful of public-page locators were ambiguous (`getByText(slug)` matched
  both the Name and the Namespace/Slug cell; `getByLabel('Search')` matched
  checkbox aria-labels). Switched to role-based locators with `exact: true`.

### ⚠️ Upgrade notes

No schema changes. No breaking API changes. Operators running behind the
default rate limit will see the public budget rise from 100 to 1000 req/min
per IP — pin `PUBLIC_RATE_LIMIT_RPM=100` if you want the old behaviour.

**Full changelog:** `v0.2.0...v0.2.1`

## v0.2.0

Major UX overhaul of the public browse experience, plus new admin workflow tooling and a richer server API.

### ✨ Highlights

- **Redesigned detail pages** for MCP servers and agents — new Connection card surfaces endpoint URL, transport, protocol version and authentication at a glance, with tabs for Overview / Installation / Versions / JSON (MCP) and Overview / Skills / Connect / Versions / JSON (agents).
- **Version history** with inline diffs between published versions.
- **MCP client config generator** — copy-paste configs for Claude Desktop, Cursor, Windsurf, and other MCP hosts.
- **Agent client snippet generator** — multi-language connection snippets with per-scheme auth guidance.
- **README rendering** on every detail page.
- **Report an entry** dialog for takedown / correction requests.

### 📄 New pages

- **`/explore`** — cross-entity search and discovery.
- **`/publishers/:slug`** — publisher profile pages.
- **`/getting-started`** — MCP + A2A onboarding walkthrough.
- **`/changelog`** — public feed of recently published / updated entries.
- **Homepage rewrite** with a protocol explainer and featured entries.

### 🛠 Admin workflow

- **Bulk actions** — multi-select publish / unpublish / feature / delete on admin lists.
- **Lifecycle stepper** — visual draft → published → deprecated state machine.
- **Reports triage queue** for user-submitted reports.
- **`PATCH` / `DELETE`** endpoints (and delete buttons) for MCP servers, agents and publishers.

### 🔌 API

- **Reports API** — full CRUD for user-submitted reports.
- **Public changelog API** — feed of recent changes.
- **View / copy event tracking** exposed as `view_count` / `copy_count` on every entry.
- **New filters** on listing endpoints: `featured`, `verified`, `tags`, `transport`.
- **New fields** on entries: `featured`, `verified`, `tags[]`, `readme`, engagement counts.

### 🐛 Fixes

- Admin UI no longer breaks when a session expires mid-navigation.
- Several e2e test flakes fixed and CI pipelines stabilized.
- Dev deployment (docker-compose) regressions fixed.

### ⚠️ Upgrade notes

Five new database migrations (`000002` → `000006`) must be applied before rolling out the new server binary. No breaking API changes — all new fields are additive.

**Full changelog:** `v0.1.4...v0.2.0`
