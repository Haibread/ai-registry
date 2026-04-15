# Changelog

All notable changes to this project are documented here.

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
