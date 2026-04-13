# Changelog

All notable changes to this project are documented here.

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
