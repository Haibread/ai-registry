# Changelog

All notable changes to this project are documented here.

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
